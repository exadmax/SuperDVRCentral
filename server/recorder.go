// Gravação das câmeras: o hub grava o mesmo vídeo H.264 que já repassa ao navegador (sem recodificar, sem ffmpeg) em arquivos
// MP4 de 1 hora, um por câmera: <NOME_DA_CAMERA>_AAAAMMDDHH.mp4, na pasta recordings.dir (padrão ~/securityCam).
// Cada câmera tem a sua configuração (página Config, guardada em recordings.state.json ao lado do hub.config.json):
// liga/desliga, modo (contínuo ou só com movimento, ver motion.go), dias de retenção e limite de espaço em GB.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4/seekablebuffer"
)

type RecordingsConfig struct {
	Dir string `json:"dir"` // pasta das gravações no servidor (padrão ~/securityCam)
}

const (
	recContinuous  = "continuous"
	recMotion      = "motion"
	recDefaultDays = 7
	recDefaultSens = 3
	recMaxPending  = 1500 // teto de quadros guardados até o próximo quadro-chave (memória)
	recSweepEvery  = 10 * time.Minute
	recWriteFresh  = 30 * time.Second // "gravando agora" = escreveu no arquivo há menos que isso
)

type recCamSet struct {
	Enabled       bool   `json:"enabled"`       // grava esta câmera (vale se a chave geral estiver ligada)
	Mode          string `json:"mode"`          // "continuous" ou "motion" (só trechos com movimento)
	RetentionDays int    `json:"retentionDays"` // 1 a 365
	MaxGB         int    `json:"maxGB"`         // 0 = sem limite; passou disso, apaga as mais antigas desta câmera
	Sensitivity   int    `json:"sensitivity"`   // 1 a 5 (só no modo movimento)
	DriveOff      bool   `json:"driveOff"`      // true = não envia esta câmera ao Google Drive nas rodadas automáticas (drive.go); padrão: envia
}

type recSettings struct {
	Enabled bool                 `json:"enabled"` // chave geral: desligada, nada é gravado
	Cameras map[string]recCamSet `json:"cameras"`
}

func defaultCamSet() recCamSet {
	return recCamSet{Enabled: true, Mode: recContinuous, RetentionDays: recDefaultDays, Sensitivity: recDefaultSens}
}

func (c recCamSet) fixed() recCamSet {
	if c.Mode != recMotion {
		c.Mode = recContinuous
	}
	if c.RetentionDays < 1 {
		c.RetentionDays = recDefaultDays
	}
	if c.RetentionDays > 365 {
		c.RetentionDays = 365
	}
	if c.MaxGB < 0 {
		c.MaxGB = 0
	}
	if c.Sensitivity < 1 || c.Sensitivity > 5 {
		c.Sensitivity = recDefaultSens
	}
	return c
}

var recGB int64 = 1 << 30 // bytes em 1 GB (variável só para os testes)

var recFileRe = regexp.MustCompile(`^(.+?)_(\d{10})(?:_(\d+))?\.mp4$`)

var recAccents = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "ê", "e", "è", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i", "ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "ù", "u", "û", "u", "ü", "u", "ç", "c",
	"Á", "A", "À", "A", "Â", "A", "Ã", "A", "É", "E", "Ê", "E", "Í", "I", "Ó", "O", "Ô", "O", "Õ", "O", "Ú", "U", "Ç", "C")

// recName transforma o nome da câmera em algo seguro para nome de arquivo: "Sala de Estar" -> "Sala_de_Estar".
func recName(name, id string) string {
	var b strings.Builder
	for _, r := range recAccents.Replace(name) {
		if r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			b.WriteRune(r)
		} else if s := b.String(); s != "" && !strings.HasSuffix(s, "_") {
			b.WriteByte('_')
		}
	}
	if s := strings.Trim(b.String(), "_"); s != "" {
		return s
	}
	return id
}

// ---- gerente (configurações, retenção, HTTP) ----

type recMgr struct {
	dir       string
	statePath string
	cams      *cameras
	now       func() time.Time

	mu  sync.RWMutex
	set recSettings
}

func newRecMgr(cfg RecordingsConfig, statePath string, cs *cameras) *recMgr {
	if cfg.Dir == "" {
		cfg.Dir = filepath.Join(homeDir(), "securityCam")
	}
	m := &recMgr{dir: cfg.Dir, statePath: statePath, cams: cs, now: time.Now, set: recSettings{Cameras: map[string]recCamSet{}}}
	if b, err := os.ReadFile(statePath); err == nil {
		if err := json.Unmarshal(b, &m.set); err != nil {
			log.Printf("gravações: %s inválido: %v", statePath, err)
		}
	}
	m.set = m.clean(m.set)
	attach := func(c *camera) { c.rec = &recorder{c: c, m: m, waitKey: true} }
	for _, c := range cs.all() {
		attach(c)
	}
	cs.onAdd = attach
	return m
}

// clean deixa uma configuração completa e válida para todas as câmeras do hub (câmeras que sumiram do hub.config.json saem).
func (m *recMgr) clean(s recSettings) recSettings {
	cams := map[string]recCamSet{}
	for _, c := range m.cams.all() {
		cs, ok := s.Cameras[c.cfg.ID]
		if !ok {
			cs = defaultCamSet()
		}
		cams[c.cfg.ID] = cs.fixed()
	}
	s.Cameras = cams
	return s
}

func (m *recMgr) camSet(id string) recCamSet {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if cs, ok := m.set.Cameras[id]; ok {
		return cs
	}
	return defaultCamSet()
}

// want: a câmera deve gravar agora? (chave geral e da câmera ligadas) e com qual configuração.
func (m *recMgr) want(id string) (recCamSet, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cs, ok := m.set.Cameras[id]
	if !ok {
		cs = defaultCamSet()
	}
	return cs, m.set.Enabled && cs.Enabled
}

// apply liga ou desliga a gravação de cada câmera conforme as configurações atuais.
func (m *recMgr) apply() {
	for _, c := range m.cams.all() {
		_, on := m.want(c.cfg.ID)
		c.setRecording(on)
	}
}

func (m *recMgr) save() error {
	m.mu.RLock()
	b, err := json.MarshalIndent(m.set, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	tmp := m.statePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.statePath)
}

// start liga o que estiver configurado e passa a apagar as gravações vencidas.
func (m *recMgr) start() {
	m.apply()
	go func() {
		for {
			m.sweep()
			time.Sleep(recSweepEvery)
		}
	}()
}

type recFile struct {
	Name      string `json:"name"`
	Camera    string `json:"camera"`              // nome da câmera como aparece no arquivo
	Size      int64  `json:"size"`                // bytes
	Start     int64  `json:"start"`               // início da hora gravada, segundos Unix
	Recording bool   `json:"recording,omitempty"` // escrevendo neste momento
}

// camByPrefix liga o prefixo do nome do arquivo à câmera (id); arquivos de câmeras que saíram do hub não têm dono.
func (m *recMgr) camByPrefix() map[string]string {
	out := map[string]string{}
	for _, c := range m.cams.all() {
		out[recName(c.cfg.Name, c.cfg.ID)] = c.cfg.ID
	}
	return out
}

func (m *recMgr) writingNames() map[string]bool {
	out := map[string]bool{}
	for _, c := range m.cams.all() {
		if n := c.rec.writing(m.now()); n != "" {
			out[n] = true
		}
	}
	return out
}

func (m *recMgr) files() ([]recFile, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []recFile{}, nil
		}
		return nil, err
	}
	open := m.writingNames()
	out := []recFile{}
	for _, e := range entries {
		g := recFileRe.FindStringSubmatch(e.Name())
		if e.IsDir() || g == nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		t, err := time.ParseInLocation("2006010215", g[2], time.Local)
		if err != nil {
			continue
		}
		out = append(out, recFile{Name: e.Name(), Camera: strings.ReplaceAll(g[1], "_", " "), Size: info.Size(), Start: t.Unix(), Recording: open[e.Name()]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start != out[j].Start {
			return out[i].Start > out[j].Start
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// sweep apaga as gravações vencidas (retenção da câmera) e, se a câmera tem limite em GB, as mais antigas até caber.
// Só mexe em arquivos com o nome deste sistema, e nunca no que está aberto para gravação.
func (m *recMgr) sweep() int {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return 0
	}
	byPrefix := m.camByPrefix()
	open := map[string]bool{}
	for _, c := range m.cams.all() {
		if n := c.rec.current(); n != "" {
			open[n] = true
		}
	}
	type fileInfo struct {
		name string
		size int64
		mod  time.Time
	}
	perCam := map[string][]fileInfo{} // id da câmera ("" = sem dono) -> arquivos
	for _, e := range entries {
		g := recFileRe.FindStringSubmatch(e.Name())
		if e.IsDir() || g == nil || open[e.Name()] {
			continue
		}
		if info, err := e.Info(); err == nil {
			id := byPrefix[g[1]]
			perCam[id] = append(perCam[id], fileInfo{e.Name(), info.Size(), info.ModTime()})
		}
	}
	remove := func(name string) bool {
		if err := os.Remove(filepath.Join(m.dir, name)); err != nil {
			return false
		}
		return true
	}
	n := 0
	for id, files := range perCam {
		cs := defaultCamSet()
		if id != "" {
			cs = m.camSet(id)
		}
		cutoff := m.now().Add(-time.Duration(cs.RetentionDays) * 24 * time.Hour)
		sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) }) // mais antigos primeiro
		var total int64
		kept := files[:0:0]
		for _, f := range files {
			if f.mod.Before(cutoff) && remove(f.name) {
				n++
				continue
			}
			total += f.size
			kept = append(kept, f)
		}
		if cs.MaxGB > 0 {
			limit := int64(cs.MaxGB) * recGB
			for _, f := range kept {
				if total <= limit {
					break
				}
				if remove(f.name) {
					total -= f.size
					n++
				}
			}
		}
	}
	if n > 0 {
		log.Printf("gravações: %d arquivo(s) apagado(s) (retenção ou limite de espaço)", n)
	}
	return n
}

type recCamStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	recCamSet
	Recording bool   `json:"recording"`       // escrevendo no arquivo agora
	Motion    bool   `json:"motion"`          // movimento detectado agora (modo movimento)
	UsedBytes int64  `json:"usedBytes"`       // espaço das gravações desta câmera
	Files     int    `json:"files"`           // quantidade de arquivos desta câmera
	Error     string `json:"error,omitempty"` // último erro de gravação
}

func (m *recMgr) status() map[string]interface{} {
	m.mu.RLock()
	master := m.set.Enabled
	m.mu.RUnlock()
	list, _ := m.files()
	byPrefix := m.camByPrefix()
	used, count := map[string]int64{}, map[string]int{}
	var total int64
	for _, f := range list {
		id := byPrefix[strings.ReplaceAll(f.Camera, " ", "_")]
		used[id] += f.Size
		count[id]++
		total += f.Size
	}
	now := m.now()
	cams := make([]recCamStatus, 0, 4)
	for _, c := range m.cams.all() {
		id := c.cfg.ID
		cams = append(cams, recCamStatus{ID: id, Name: c.cfg.Name, recCamSet: m.camSet(id), Recording: c.rec.writing(now) != "",
			Motion: c.rec.motionActive(now), UsedBytes: used[id], Files: count[id], Error: c.rec.err()})
	}
	return map[string]interface{}{"enabled": master, "dir": m.dir, "cameras": cams, "files": len(list), "usedBytes": total}
}

// recCamPatch: só os campos presentes mudam.
type recCamPatch struct {
	Enabled       *bool   `json:"enabled"`
	Mode          *string `json:"mode"`
	RetentionDays *int    `json:"retentionDays"`
	MaxGB         *int    `json:"maxGB"`
	Sensitivity   *int    `json:"sensitivity"`
	DriveOff      *bool   `json:"driveOff"`
}

func (p recCamPatch) validate() string {
	switch {
	case p.Mode != nil && *p.Mode != recContinuous && *p.Mode != recMotion:
		return "Modo inválido (use contínuo ou movimento)."
	case p.RetentionDays != nil && (*p.RetentionDays < 1 || *p.RetentionDays > 365):
		return "A retenção vai de 1 a 365 dias."
	case p.MaxGB != nil && (*p.MaxGB < 0 || *p.MaxGB > 100000):
		return "O limite de espaço vai de 0 (sem limite) a 100000 GB."
	case p.Sensitivity != nil && (*p.Sensitivity < 1 || *p.Sensitivity > 5):
		return "A sensibilidade vai de 1 a 5."
	}
	return ""
}

func (p recCamPatch) apply(c recCamSet) recCamSet {
	if p.Enabled != nil {
		c.Enabled = *p.Enabled
	}
	if p.Mode != nil {
		c.Mode = *p.Mode
	}
	if p.RetentionDays != nil {
		c.RetentionDays = *p.RetentionDays
	}
	if p.MaxGB != nil {
		c.MaxGB = *p.MaxGB
	}
	if p.Sensitivity != nil {
		c.Sensitivity = *p.Sensitivity
	}
	if p.DriveOff != nil {
		c.DriveOff = *p.DriveOff
	}
	return c
}

func (m *recMgr) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/recordings/settings", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, m.status()) })
	// PUT com X-CD-Action: confirm (como os botões de energia): um formulário de outra página não consegue enviá-lo sem preflight.
	mux.HandleFunc("PUT /api/recordings/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-CD-Action") != "confirm" {
			jsonError(w, http.StatusMethodNotAllowed, "use o cabeçalho X-CD-Action: confirm")
			return
		}
		var in struct {
			Enabled *bool                  `json:"enabled"`
			Cameras map[string]recCamPatch `json:"cameras"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil {
			jsonError(w, http.StatusBadRequest, "JSON inválido")
			return
		}
		for _, p := range in.Cameras {
			if msg := p.validate(); msg != "" {
				jsonError(w, http.StatusBadRequest, msg)
				return
			}
		}
		m.mu.Lock()
		next := recSettings{Enabled: m.set.Enabled, Cameras: map[string]recCamSet{}}
		for k, v := range m.set.Cameras {
			next.Cameras[k] = v
		}
		if in.Enabled != nil {
			next.Enabled = *in.Enabled
		}
		for id, p := range in.Cameras {
			if cur, ok := next.Cameras[id]; ok { // ids desconhecidos são ignorados
				next.Cameras[id] = p.apply(cur)
			}
		}
		m.set = m.clean(next)
		m.mu.Unlock()
		if err := m.save(); err != nil {
			log.Printf("gravações: salvar %s: %v", m.statePath, err)
			jsonError(w, http.StatusInternalServerError, "Não foi possível salvar a configuração no servidor.")
			return
		}
		m.apply()
		go m.sweep()
		writeJSON(w, m.status())
	})
	mux.HandleFunc("GET /api/recordings", func(w http.ResponseWriter, r *http.Request) {
		list, err := m.files()
		if err != nil {
			jsonError(w, http.StatusServiceUnavailable, "Não foi possível ler a pasta das gravações do servidor.")
			return
		}
		writeJSON(w, list)
	})
	mux.HandleFunc("GET /api/recordings/file/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !recFileRe.MatchString(name) || filepath.Base(name) != name { // o nome da URL nunca vira caminho livre
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(m.dir, name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		http.NewResponseController(w).SetWriteDeadline(time.Now().Add(30 * time.Minute)) // o WriteTimeout (60 s) cortaria vídeos grandes
		w.Header().Set("Content-Type", "video/mp4")
		if r.URL.Query().Get("download") == "1" {
			w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		}
		http.ServeContent(w, r, name, info.ModTime(), f)
	})
	// DELETE com X-CD-Action: confirm; nunca apaga o arquivo que está sendo gravado.
	mux.HandleFunc("DELETE /api/recordings/file/{name}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-CD-Action") != "confirm" {
			jsonError(w, http.StatusMethodNotAllowed, "use o cabeçalho X-CD-Action: confirm")
			return
		}
		name := r.PathValue("name")
		if !recFileRe.MatchString(name) || filepath.Base(name) != name {
			http.NotFound(w, r)
			return
		}
		if m.writingNames()[name] {
			jsonError(w, http.StatusConflict, "Esta gravação está em andamento e não pode ser excluída agora.")
			return
		}
		if err := os.Remove(filepath.Join(m.dir, name)); err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			log.Printf("gravações: excluir %s: %v", name, err)
			jsonError(w, http.StatusInternalServerError, "Não foi possível excluir a gravação no servidor.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ---- gravador de uma câmera ----

type recorder struct {
	c *camera
	m *recMgr

	mu        sync.Mutex
	init      []byte
	f         *os.File
	w         *bufio.Writer
	name      string // arquivo aberto (só o nome)
	hour      string // AAAAMMDDHH do arquivo aberto
	lastWrite time.Time
	seq       uint32
	base      uint64 // tempo (relógio de 90 kHz) do próximo trecho dentro do arquivo
	lastErr   string

	// Grupo de quadros atual (do último quadro-chave até agora). Só é escrito se gopRec: sempre no modo contínuo; no modo
	// movimento, quando há movimento no grupo (assim o trecho já começa no quadro-chave anterior ao movimento: pré-roll).
	gop     []*fmp4.Sample
	gopRec  bool
	gopHour string
	waitKey bool // depois de (re)conectar, descarta quadros até o próximo quadro-chave
	det     motionDet
}

func (r *recorder) current() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.name
}

// writing: o arquivo em que escreveu há pouco ("" se parado, ex.: modo movimento sem movimento).
func (r *recorder) writing(now time.Time) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f != nil && now.Sub(r.lastWrite) < recWriteFresh {
		return r.name
	}
	return ""
}

func (r *recorder) motionActive(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.det.active(now)
}

func (r *recorder) err() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastErr
}

// setInit recebe o segmento de inicialização de uma nova sessão com a câmera.
func (r *recorder) setInit(init []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushLocked()
	if r.f != nil && !bytes.Equal(r.init, init) { // parâmetros do vídeo mudaram: não dá para continuar o mesmo arquivo
		r.closeLocked()
	}
	r.init = init
	r.waitKey = true
}

// ensureInit só preenche o init se o gravador ainda não tem (ligou a gravação com a câmera já aberta por um espectador).
func (r *recorder) ensureInit(init []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.init == nil {
		r.init = init
	}
}

func (r *recorder) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushLocked()
	r.closeLocked()
	r.det.reset()
	r.waitKey = true
}

// sample recebe cada quadro da câmera.
func (r *recorder) sample(s *fmp4.Sample, key bool) {
	cs, on := r.m.want(r.c.cfg.ID)
	if !on {
		r.close()
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.init == nil {
		return
	}
	now := r.m.now()
	motion := false
	if cs.Mode == recMotion {
		motion = r.det.feed(len(s.Payload), key, now, cs.Sensitivity)
	} else {
		r.det.reset()
	}
	if key {
		r.waitKey = false
		r.flushLocked()
		r.gop = []*fmp4.Sample{s}
		r.gopHour = now.Format("2006010215")
		r.gopRec = cs.Mode != recMotion || motion
		return
	}
	if r.waitKey {
		return
	}
	r.gop = append(r.gop, s)
	if motion {
		r.gopRec = true
	}
	if len(r.gop) >= recMaxPending {
		r.flushLocked()
	}
}

func (r *recorder) rotateLocked(hour string) {
	r.closeLocked()
	if err := os.MkdirAll(r.m.dir, 0o755); err != nil {
		r.lastErr = "pasta de gravação: " + err.Error()
		return
	}
	base := recName(r.c.cfg.Name, r.c.cfg.ID) + "_" + hour
	name := base + ".mp4"
	for i := 2; ; i++ { // reinício do hub na mesma hora: não mistura dois vídeos no mesmo arquivo
		if _, err := os.Stat(filepath.Join(r.m.dir, name)); os.IsNotExist(err) {
			break
		}
		name = fmt.Sprintf("%s_%d.mp4", base, i)
	}
	f, err := os.OpenFile(filepath.Join(r.m.dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		r.lastErr = "criar " + name + ": " + err.Error()
		log.Printf("gravações: %s: %s", r.c.cfg.ID, r.lastErr)
		return
	}
	w := bufio.NewWriterSize(f, 64<<10)
	if _, err := w.Write(r.init); err != nil {
		f.Close()
		r.lastErr = err.Error()
		return
	}
	r.f, r.w, r.name, r.hour, r.seq, r.base, r.lastErr = f, w, name, hour, 0, 0, ""
}

// flushLocked escreve o grupo de quadros atual como um único trecho, se ele deve ser gravado. O arquivo só é criado aqui,
// então no modo movimento horas sem movimento não geram arquivo.
func (r *recorder) flushLocked() {
	samples := r.gop
	r.gop = nil
	if len(samples) == 0 || !r.gopRec {
		return
	}
	if r.f == nil || r.gopHour != r.hour {
		r.rotateLocked(r.gopHour)
	}
	if r.f == nil {
		return
	}
	var dur uint64
	for _, s := range samples {
		dur += uint64(s.Duration)
	}
	r.seq++
	var buf seekablebuffer.Buffer
	part := fmp4.Part{SequenceNumber: r.seq, Tracks: []*fmp4.PartTrack{{ID: 1, BaseTime: r.base, Samples: samples}}}
	if err := part.Marshal(&buf); err == nil {
		_, err = r.w.Write(buf.Bytes())
		if err == nil {
			err = r.w.Flush()
		}
		if err != nil {
			r.lastErr = "gravar " + r.name + ": " + err.Error()
			log.Printf("gravações: %s: %s", r.c.cfg.ID, r.lastErr)
			r.closeLocked()
			return
		}
	}
	r.base += dur
	r.lastWrite = r.m.now()
}

func (r *recorder) closeLocked() {
	if r.f == nil {
		return
	}
	r.w.Flush()
	r.f.Close()
	r.f, r.w, r.name, r.hour = nil, nil, "", ""
}
