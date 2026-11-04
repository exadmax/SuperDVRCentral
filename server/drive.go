// Cópia diária das gravações para o Google Drive (rclone, já instalado e configurado no servidor).
// Uma vez por dia (padrão 3:00) o hub envia as gravações novas para <remote>/<AAAA-MM-DD>/<NOME_DA_CAMERA>/arquivo.mp4, onde o dia é o
// da gravação. Arquivo maior que o limite (driveSync.maxFileMB) é dividido em partes (..._parte1.mp4, _parte2...) nos limites dos trechos
// do MP4 fragmentado, sem recodificar e sem ffmpeg; cada parte toca sozinha. As gravações locais continuam seguindo a retenção da página Config.
// O que já foi enviado fica em drive.state.json (ao lado do hub.config.json), então nada sobe duas vezes.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type DriveSyncConfig struct {
	Enabled   bool   `json:"enabled"`
	Remote    string `json:"remote"`    // destino do rclone, ex.: "GDrive:CameraSeguranca"
	Hour      int    `json:"hour"`      // hora do dia (0-23) em que a cópia roda; padrão 3
	MaxFileMB int    `json:"maxFileMB"` // maior arquivo enviado inteiro; acima disso é dividido em partes (0 = nunca divide; padrão 1024)
	Rclone    string `json:"rclone"`    // executável (padrão "rclone")
}

const (
	driveMinAge      = 5 * time.Minute // arquivo mexido há menos que isso ainda pode estar sendo gravado
	driveMaxAttempts = 3               // tentativas por rodada (a rodada falha se o envio der erro)
	driveRetryWindow = 22 * time.Hour  // só tenta até este tempo depois do horário marcado: 3:00 -> 1:00 do dia seguinte
	driveFileLimit   = 3 * time.Hour   // tempo máximo para enviar um arquivo
	driveStartDelay  = 5 * time.Minute // espera ao subir o hub antes da primeira conferência da hora marcada
)

// driveRetryDelays: espera antes da 2ª e da 3ª tentativa, contada do início da anterior. Espaçadas para cobrir uma queda de internet
// de algumas horas sem gastar as tentativas em minutos.
var driveRetryDelays = []time.Duration{time.Hour, 4 * time.Hour}

type driveState struct {
	Cycle       time.Time        `json:"cycle"`    // horário marcado da rodada em curso (a das 3:00 de um dia)
	Attempts    int              `json:"attempts"` // tentativas já feitas nessa rodada
	LastAttempt time.Time        `json:"lastAttempt"`
	LastOK      bool             `json:"lastOK"`
	LastMsg     string           `json:"lastMsg"`
	Uploaded    map[string]int64 `json:"uploaded"` // nome do arquivo original -> tamanho enviado
}

type driveSync struct {
	cfg       DriveSyncConfig
	statePath string
	m         *recMgr
	now       func() time.Time
	upload    func(ctx context.Context, src, dest string) error        // rclone copyto (trocável nos testes)
	check     func(ctx context.Context) error                          // rclone mkdir da pasta raiz: falha cedo se o rclone não está pronto
	list      func(ctx context.Context, path string) ([]string, error) // subpastas de <destino>/<path>
	purge     func(ctx context.Context, path string) error             // apaga <destino>/<path> com o conteúdo (vai para a lixeira do Drive)
	rmdir     func(ctx context.Context, path string) error             // remove <destino>/<path> só se estiver vazia
	maxBytes  int64

	mu       sync.Mutex
	st       driveState
	running  bool
	progress string
	scope    string // nomes das câmeras da rodada em andamento ("" = todas)
}

func newDriveSync(cfg DriveSyncConfig, statePath string, m *recMgr) *driveSync {
	if cfg.Rclone == "" {
		cfg.Rclone = "rclone"
	}
	if cfg.Hour < 0 || cfg.Hour > 23 {
		cfg.Hour = 3
	}
	if cfg.MaxFileMB == 0 {
		cfg.MaxFileMB = 1024
	}
	d := &driveSync{cfg: cfg, statePath: statePath, m: m, now: time.Now, st: driveState{Uploaded: map[string]int64{}}}
	d.maxBytes = int64(cfg.MaxFileMB) << 20
	d.upload = func(ctx context.Context, src, dest string) error {
		return d.rclone(ctx, "copyto", "--transfers", "1", "--retries", "3", "--low-level-retries", "10", "--buffer-size", "4M", "--drive-chunk-size", "32M", src, dest)
	}
	d.check = func(ctx context.Context) error { return d.rclone(ctx, "mkdir", cfg.Remote) }
	at := func(path string) string { return strings.TrimRight(cfg.Remote, "/") + "/" + path }
	d.list = func(ctx context.Context, path string) ([]string, error) {
		out, err := d.rcloneOut(ctx, "lsf", "--dirs-only", at(path))
		var names []string
		for _, l := range strings.Split(out, "\n") {
			if l = strings.TrimSuffix(strings.TrimSpace(l), "/"); l != "" {
				names = append(names, l)
			}
		}
		return names, err
	}
	d.purge = func(ctx context.Context, path string) error { return d.rclone(ctx, "purge", at(path)) }
	d.rmdir = func(ctx context.Context, path string) error { return d.rclone(ctx, "rmdir", at(path)) } // só remove pasta vazia
	if b, err := os.ReadFile(statePath); err == nil {
		if err := json.Unmarshal(b, &d.st); err != nil {
			log.Printf("drive: %s inválido: %v", statePath, err)
		}
		if d.st.Uploaded == nil {
			d.st.Uploaded = map[string]int64{}
		}
	}
	return d
}

func (d *driveSync) rclone(ctx context.Context, args ...string) error {
	_, err := d.rcloneOut(ctx, args...)
	return err
}

// rcloneOut roda o rclone e devolve o que ele escreveu na saída padrão.
func (d *driveSync) rcloneOut(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, d.cfg.Rclone, args...)
	cmd.Env = append(os.Environ(), "GOMEMLIMIT=96MiB") // o rclone divide o limite de memória do serviço com o hub
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if i := strings.LastIndex(msg, "\n"); i >= 0 && len(msg) > 400 {
			msg = msg[i+1:]
		}
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), fmt.Errorf("rclone %s: %s", args[0], msg)
	}
	return out.String(), nil
}

func (d *driveSync) save() {
	b, err := json.MarshalIndent(d.st, "", "  ")
	if err == nil {
		tmp := d.statePath + ".tmp"
		if err = os.WriteFile(tmp, b, 0o644); err == nil {
			err = os.Rename(tmp, d.statePath)
		}
	}
	if err != nil {
		log.Printf("drive: salvar %s: %v", d.statePath, err)
	}
}

// start: confere a cada minuto se já passou da hora marcada. Sem estado anterior (primeira instalação) a primeira cópia é na próxima
// hora marcada, não na hora em que o hub subiu; se o hub estava desligado na hora marcada, ele copia ao voltar.
func (d *driveSync) start() {
	if !d.cfg.Enabled || d.cfg.Remote == "" {
		return
	}
	d.mu.Lock()
	if d.st.LastAttempt.IsZero() { // primeira instalação: nada atrasado para recuperar
		d.st.LastAttempt, d.st.Cycle, d.st.Attempts = d.now(), d.latestTrigger(d.now()), driveMaxAttempts
		d.save()
	}
	d.mu.Unlock()
	log.Printf("drive: cópia diária das gravações para %s às %02d:00", d.cfg.Remote, d.cfg.Hour)
	go func() {
		time.Sleep(driveStartDelay) // depois de reiniciar o hub, dá tempo de ajustar o que será enviado antes de uma rodada atrasada começar
		for {
			if d.due() {
				d.run()
			}
			time.Sleep(time.Minute)
		}
	}()
}

// latestTrigger: o último horário marcado (driveSync.hour) que já passou, que identifica a rodada em curso.
func (d *driveSync) latestTrigger(now time.Time) time.Time {
	t := time.Date(now.Year(), now.Month(), now.Day(), d.cfg.Hour, 0, 0, 0, now.Location())
	if now.Before(t) {
		t = t.AddDate(0, 0, -1)
	}
	return t
}

// due: hora de começar a rodada, ou de tentar de novo uma que falhou. Cada rodada (a das 3:00 de um dia) faz no máximo driveMaxAttempts
// tentativas, espaçadas por driveRetryDelays, e só até driveRetryWindow depois do horário marcado (1:00 do dia seguinte, antes da próxima).
func (d *driveSync) due() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	t := d.latestTrigger(now)
	if d.running || !now.Before(t.Add(driveRetryWindow)) {
		return false
	}
	if !d.st.Cycle.Equal(t) { // rodada ainda não começou (inclui o hub que estava desligado na hora marcada)
		return true
	}
	if d.st.LastOK || d.st.Attempts >= driveMaxAttempts || d.st.Attempts < 1 {
		return false
	}
	return now.Sub(d.st.LastAttempt) >= driveRetryDelays[d.st.Attempts-1]
}

type drivePending struct {
	name, path, camera, day string
	start                   time.Time // início da hora gravada
	size                    int64
}

// pending: gravações fechadas (a hora acabou e ninguém mexe nelas há alguns minutos) que ainda não foram enviadas, das mais antigas às mais novas.
// only (prefixo do nome de arquivo das câmeras): se não for nil, só dessas câmeras; nil = as automáticas, ou seja, todas menos as que têm
// o envio ao Drive desligado na página Config.
func (d *driveSync) pending(only map[string]bool) []drivePending {
	var off map[string]bool
	if only == nil {
		off = map[string]bool{}
		for prefix, id := range d.m.camByPrefix() {
			if d.m.camSet(id).DriveOff {
				off[prefix] = true
			}
		}
	}
	entries, err := os.ReadDir(d.m.dir)
	if err != nil {
		return nil
	}
	now := d.now()
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []drivePending
	for _, e := range entries {
		g := recFileRe.FindStringSubmatch(e.Name())
		if e.IsDir() || g == nil || (only != nil && !only[g[1]]) || off[g[1]] {
			continue
		}
		if _, done := d.st.Uploaded[e.Name()]; done {
			continue
		}
		t, err := time.ParseInLocation("2006010215", g[2], time.Local)
		info, ierr := e.Info()
		if err != nil || ierr != nil || info.Size() == 0 || now.Before(t.Add(time.Hour)) || now.Sub(info.ModTime()) < driveMinAge {
			continue
		}
		day := t.Format("2006-01-02")
		if day < d.cutoffDay(g[1]) { // já passou da retenção da câmera: a limpeza do Drive apagaria na hora
			continue
		}
		out = append(out, drivePending{name: e.Name(), path: filepath.Join(d.m.dir, e.Name()), camera: g[1], day: day, start: t, size: info.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func (d *driveSync) setProgress(s string) {
	d.mu.Lock()
	d.progress = s
	d.mu.Unlock()
}

// run envia o que está pendente. Uma rodada por vez; falha em um arquivo não impede os outros.
// ids: ids das câmeras a enviar; vazio = todas (é a rodada completa, que também apaga do Drive o que passou da retenção).
// Rodada só de algumas câmeras (botão da página Config) não conta como a rodada do dia: a das 3:00 continua valendo.
func (d *driveSync) run(ids ...string) (started bool) {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return false
	}
	d.running = true
	started0 := d.now() // a rodada vale a partir de quando começou: uma rodada longa que passa das 3:00 não "consome" a das 3:00, que então só envia o diferencial
	var only map[string]bool
	var names []string
	if len(ids) > 0 {
		only = map[string]bool{}
		for _, c := range d.m.cams.all() {
			for _, id := range ids {
				if c.cfg.ID == id {
					only[recName(c.cfg.Name, c.cfg.ID)] = true
					names = append(names, c.cfg.Name)
				}
			}
		}
	}
	d.scope = strings.Join(names, ", ")
	if only == nil { // rodada completa: conta como tentativa da rodada atual (salvo já, para um hub que reinicia no meio não tentar sem fim)
		if t := d.latestTrigger(started0); !d.st.Cycle.Equal(t) {
			d.st.Cycle, d.st.Attempts = t, 0
		}
		d.st.Attempts++
		d.st.LastAttempt, d.st.LastOK = started0, false
		d.save()
	}
	d.mu.Unlock()
	go func() {
		ok, msg := d.runOnceFor(only)
		d.mu.Lock()
		d.running, d.progress, d.scope = false, "", ""
		if only == nil {
			d.st.LastOK = ok
		} else {
			msg = strings.Join(names, ", ") + ": " + msg
		}
		d.st.LastMsg = msg
		d.save()
		d.mu.Unlock()
		log.Printf("drive: %s", msg)
	}()
	return true
}

func (d *driveSync) runOnce() (bool, string) { return d.runOnceFor(nil) }

func (d *driveSync) runOnceFor(only map[string]bool) (bool, string) {
	files := d.pending(only)
	ctx := context.Background()
	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	err := d.check(cctx)
	cancel()
	if err != nil {
		return false, err.Error()
	}
	tmp := filepath.Join(d.m.dir, ".drive-tmp")
	defer os.RemoveAll(tmp)
	var sent, failed int
	var bytesSent int64
	var lastErr string
	for i, f := range files {
		d.setProgress(fmt.Sprintf("%d/%d %s", i+1, len(files), f.name))
		n, err := d.sendFile(ctx, f, tmp)
		if err != nil {
			failed++
			lastErr = f.name + ": " + err.Error()
			log.Printf("drive: %s", lastErr)
			continue
		}
		sent++
		bytesSent += n
		d.mu.Lock()
		d.st.Uploaded[f.name] = f.size
		d.save()
		d.mu.Unlock()
	}
	d.forget()
	msg := "nada novo para enviar"
	if len(files) > 0 {
		msg = fmt.Sprintf("%d arquivo(s) enviado(s) (%.1f MB)", sent, float64(bytesSent)/(1<<20))
	}
	if failed > 0 {
		msg = fmt.Sprintf("%s, %d com erro (último: %s)", msg, failed, lastErr)
	}
	removed, perr := 0, error(nil)
	if only == nil { // a limpeza do Drive é da rodada completa
		removed, perr = d.prune(ctx)
	}
	if removed > 0 {
		msg += fmt.Sprintf("; %d pasta(s) de câmera/dia fora da retenção apagada(s) do Drive", removed)
	}
	if perr != nil {
		msg += "; limpeza do Drive: " + perr.Error()
	}
	return failed == 0 && perr == nil, msg
}

// retentionDays: a mesma retenção da página Config ("Guardar por N dias") da câmera dona da pasta/arquivo (prefixo do nome); câmera que
// saiu do hub usa o padrão, como a limpeza local.
func (d *driveSync) retentionDays(prefix string) int {
	cs := defaultCamSet()
	if id, ok := d.m.camByPrefix()[prefix]; ok {
		cs = d.m.camSet(id)
	}
	return cs.RetentionDays
}

// cutoffDay: dia (AAAA-MM-DD) antes do qual as gravações dessa câmera já passaram da retenção.
func (d *driveSync) cutoffDay(prefix string) string {
	return d.now().AddDate(0, 0, -d.retentionDays(prefix)).Format("2006-01-02")
}

var (
	driveDayRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	driveCamRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`) // forma dos nomes que o recorder.go cria (recName)
)

// prune apaga do Drive as gravações que passaram da retenção da câmera (a mesma da página Config). Só mexe em
// <destino>/AAAA-MM-DD/<Camera>: pastas de dia com nome exatamente de data e, dentro delas, pastas com nome de câmera; qualquer outra coisa
// (arquivos soltos, outras pastas suas) é ignorada. As pastas vão para a lixeira do Drive, e a pasta do dia some quando fica vazia.
// Devolve quantas pastas de câmera/dia foram apagadas.
func (d *driveSync) prune(ctx context.Context) (int, error) {
	if i := strings.Index(d.cfg.Remote, ":"); i < 0 || strings.Trim(d.cfg.Remote[i+1:], "/") == "" {
		return 0, errors.New("o destino precisa ser uma pasta (ex.: GDrive:CameraSeguranca), não a raiz do Drive")
	}
	listDirs := func(path string) ([]string, error) {
		lctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		return d.list(lctx, path)
	}
	days, err := listDirs("")
	if err != nil {
		return 0, err
	}
	minDays := defaultCamSet().RetentionDays // dias mais novos que a menor retenção (de qualquer câmera) não têm o que apagar
	for _, c := range d.m.cams.all() {
		if r := d.m.camSet(c.cfg.ID).RetentionDays; r < minDays {
			minDays = r
		}
	}
	minCutoff := d.now().AddDate(0, 0, -minDays).Format("2006-01-02")
	n := 0
	for _, day := range days {
		if _, err := time.Parse("2006-01-02", day); err != nil || !driveDayRe.MatchString(day) || day >= minCutoff {
			continue
		}
		cams, err := listDirs(day)
		if err != nil {
			return n, err
		}
		left := 0
		for _, cam := range cams {
			if !driveCamRe.MatchString(cam) || day >= d.cutoffDay(cam) {
				left++
				continue
			}
			pctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
			err := d.purge(pctx, day+"/"+cam)
			cancel()
			if err != nil {
				return n, err
			}
			n++
		}
		if left == 0 { // vazia (pode ainda ter arquivo solto: o rmdir recusa e a pasta fica)
			rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			if err := d.rmdir(rctx, day); err != nil {
				log.Printf("drive: pasta %s não removida: %v", day, err)
			}
			cancel()
		}
	}
	return n, nil
}

// forget tira do estado os arquivos que a retenção já apagou do disco.
func (d *driveSync) forget() {
	entries, err := os.ReadDir(d.m.dir)
	if err != nil {
		return
	}
	have := map[string]bool{}
	for _, e := range entries {
		have[e.Name()] = true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for name := range d.st.Uploaded {
		if !have[name] {
			delete(d.st.Uploaded, name)
		}
	}
}

// sendFile envia um arquivo (inteiro ou em partes) e devolve os bytes enviados.
func (d *driveSync) sendFile(ctx context.Context, f drivePending, tmp string) (int64, error) {
	dir := strings.TrimRight(d.cfg.Remote, "/") + "/" + f.day + "/" + f.camera + "/"
	put := func(path, name string) error {
		cctx, cancel := context.WithTimeout(ctx, driveFileLimit)
		defer cancel()
		return d.upload(cctx, path, dir+name)
	}
	if d.maxBytes <= 0 || f.size <= d.maxBytes {
		return f.size, put(f.path, f.name)
	}
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return 0, err
	}
	parts, err := splitMP4(f.path, tmp, strings.TrimSuffix(f.name, ".mp4"), d.maxBytes)
	if err != nil {
		return 0, fmt.Errorf("dividir: %w", err)
	}
	var total int64
	for _, p := range parts {
		err := put(p, filepath.Base(p))
		if st, e := os.Stat(p); e == nil {
			total += st.Size()
		}
		os.Remove(p) // libera o espaço já
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// ---- divisão do MP4 fragmentado ----

type mp4Box struct {
	typ  string
	size int64 // total, com o cabeçalho
	hdr  []byte
}

func readBoxHeader(r *bufio.Reader) (mp4Box, error) {
	h := make([]byte, 8)
	if _, err := io.ReadFull(r, h); err != nil {
		return mp4Box{}, err
	}
	b := mp4Box{typ: string(h[4:8]), size: int64(binary.BigEndian.Uint32(h)), hdr: h}
	if b.size == 1 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return mp4Box{}, err
		}
		b.hdr = append(h, ext...)
		b.size = int64(binary.BigEndian.Uint64(ext))
	}
	if b.size < int64(len(b.hdr)) {
		return mp4Box{}, errors.New("caixa MP4 inválida (" + b.typ + ")")
	}
	return b, nil
}

// splitMP4 divide um MP4 fragmentado (init + pares moof/mdat, como o recorder.go grava) em arquivos de no máximo maxBytes, sempre entre
// dois trechos. Cada parte leva o init e tem o tempo reiniciado em 0. Devolve os caminhos <base>_parte1.mp4, _parte2.mp4...
// Um trecho sozinho maior que o limite vai inteiro numa parte. Um trecho cortado no fim do arquivo (hub desligado no meio) é descartado.
func splitMP4(src, dstDir, base string, maxBytes int64) ([]string, error) {
	f, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	remaining := st.Size()
	r := bufio.NewReaderSize(f, 256<<10)

	var init []byte
	var first []byte // cabeçalho + conteúdo do primeiro moof
	for first == nil {
		b, err := readBoxHeader(r)
		if err != nil {
			return nil, fmt.Errorf("sem trechos de vídeo: %w", err)
		}
		rest := b.size - int64(len(b.hdr))
		if b.typ == "moof" {
			body := make([]byte, rest)
			if _, err := io.ReadFull(r, body); err != nil {
				return nil, err
			}
			first = append(b.hdr, body...)
			remaining -= b.size
			break
		}
		body := make([]byte, rest)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, err
		}
		init = append(init, b.hdr...)
		init = append(init, body...)
		remaining -= b.size
	}

	var out []string
	var w *bufio.Writer
	var wf *os.File
	var cur int64
	var offset uint64 // BaseTime do primeiro trecho da parte atual
	closePart := func() error {
		if wf == nil {
			return nil
		}
		err := w.Flush()
		if cerr := wf.Close(); err == nil {
			err = cerr
		}
		wf = nil
		return err
	}
	defer closePart()
	open := func() error {
		if err := closePart(); err != nil {
			return err
		}
		p := filepath.Join(dstDir, fmt.Sprintf("%s_parte%d.mp4", base, len(out)+1))
		nf, err := os.Create(p)
		if err != nil {
			return err
		}
		wf, w, cur, offset = nf, bufio.NewWriterSize(nf, 256<<10), int64(len(init)), 0
		out = append(out, p)
		_, err = w.Write(init)
		return err
	}

	moof := first
	for {
		mh, err := readBoxHeader(r) // o mdat que acompanha o moof
		if err != nil || mh.typ != "mdat" {
			break // fim do arquivo (ou lixo): o último trecho sem mdat é descartado
		}
		mdatBody := mh.size - int64(len(mh.hdr))
		remaining -= int64(len(mh.hdr))
		if remaining < mdatBody { // mdat cortado
			break
		}
		remaining -= mdatBody
		if wf == nil || (cur+int64(len(moof))+mh.size > maxBytes && cur > int64(len(init))) {
			if err := open(); err != nil {
				return out, err
			}
		}
		base, ok := tfdtBase(moof)
		if cur == int64(len(init)) {
			offset = base
		}
		if ok && base >= offset {
			setTfdtBase(moof, base-offset)
		}
		if _, err := w.Write(moof); err != nil {
			return out, err
		}
		if _, err := w.Write(mh.hdr); err != nil {
			return out, err
		}
		if _, err := io.CopyN(w, r, mdatBody); err != nil {
			return out, err
		}
		cur += int64(len(moof)) + mh.size

		nh, err := readBoxHeader(r)
		if err != nil || nh.typ != "moof" {
			break
		}
		body := make([]byte, nh.size-int64(len(nh.hdr)))
		if _, err := io.ReadFull(r, body); err != nil {
			break
		}
		moof = append(nh.hdr, body...)
		remaining -= nh.size
	}
	if err := closePart(); err != nil {
		return out, err
	}
	if len(out) == 0 {
		return nil, errors.New("nenhum trecho de vídeo completo")
	}
	return out, nil
}

// tfdtPos acha o campo "tempo de decodificação" (tfdt dentro de moof>traf) e devolve onde ele começa e se tem 64 bits.
func tfdtPos(moof []byte) (pos int, wide, ok bool) {
	walk := func(b []byte, start, end int, f func(typ string, at, size int) bool) {
		for at := start; at+8 <= end; {
			size := int(binary.BigEndian.Uint32(b[at:]))
			if size < 8 || at+size > end {
				return
			}
			if f(string(b[at+4:at+8]), at, size) {
				return
			}
			at += size
		}
	}
	walk(moof, 8, len(moof), func(typ string, at, size int) bool {
		if typ != "traf" {
			return false
		}
		walk(moof, at+8, at+size, func(typ string, at, size int) bool {
			if typ == "tfdt" && size >= 16 {
				wide = moof[at+8] == 1
				pos, ok = at+12, !wide || size >= 20
				return true
			}
			return false
		})
		return ok
	})
	return
}

func tfdtBase(moof []byte) (uint64, bool) {
	pos, wide, ok := tfdtPos(moof)
	if !ok {
		return 0, false
	}
	if wide {
		return binary.BigEndian.Uint64(moof[pos:]), true
	}
	return uint64(binary.BigEndian.Uint32(moof[pos:])), true
}

func setTfdtBase(moof []byte, v uint64) {
	pos, wide, ok := tfdtPos(moof)
	if !ok {
		return
	}
	if wide {
		binary.BigEndian.PutUint64(moof[pos:], v)
	} else {
		binary.BigEndian.PutUint32(moof[pos:], uint32(v))
	}
}

// ---- HTTP ----

type drivePend struct {
	Files int   `json:"files"` // gravações fechadas ainda não enviadas
	Bytes int64 `json:"bytes"`
}

func (d *driveSync) status() map[string]interface{} {
	pend := map[string]drivePend{} // por id da câmera: o que falta enviar
	byPrefix := d.m.camByPrefix()
	all := map[string]bool{} // inclui as câmeras com o envio desligado: o botão Sincronizar delas mostra o que enviaria
	for prefix := range byPrefix {
		all[prefix] = true
	}
	for _, f := range d.pending(all) {
		id := byPrefix[f.camera]
		p := pend[id]
		p.Files++
		p.Bytes += f.size
		pend[id] = p
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	next := d.now()
	trigger := time.Date(next.Year(), next.Month(), next.Day(), d.cfg.Hour, 0, 0, 0, next.Location())
	if !next.Before(trigger) {
		trigger = trigger.AddDate(0, 0, 1)
	}
	out := map[string]interface{}{"enabled": d.cfg.Enabled && d.cfg.Remote != "", "remote": d.cfg.Remote, "hour": d.cfg.Hour,
		"maxFileMB": d.cfg.MaxFileMB, "running": d.running, "progress": d.progress, "scope": d.scope, "uploaded": d.sentCount(), "skipped": len(d.st.Uploaded) - d.sentCount(),
		"pending": pend, "lastOK": d.st.LastOK, "lastMsg": d.st.LastMsg, "nextRun": trigger.Format(time.RFC3339)}
	if !d.st.LastAttempt.IsZero() {
		out["lastAttempt"] = d.st.LastAttempt.Format(time.RFC3339)
	}
	out["maxAttempts"] = driveMaxAttempts
	if t := d.latestTrigger(d.now()); d.st.Cycle.Equal(t) {
		out["attempts"] = d.st.Attempts
		if !d.st.LastOK && d.st.Attempts >= 1 && d.st.Attempts < driveMaxAttempts && !d.running {
			if at := d.st.LastAttempt.Add(driveRetryDelays[d.st.Attempts-1]); at.Before(t.Add(driveRetryWindow)) {
				out["nextRetry"] = at.Format(time.RFC3339)
			}
		}
	}
	return out
}

func (d *driveSync) register(mux *http.ServeMux) {
	d.registerSkip(mux)
	mux.HandleFunc("GET /api/recordings/drive", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, d.status()) })
	// POST com X-CD-Action: confirm: envia agora, sem esperar a hora marcada. Corpo opcional {"cameras": ["id", ...]}: só essas câmeras
	// (sem corpo ou lista vazia = todas, como a rodada das 3:00).
	mux.HandleFunc("POST /api/recordings/drive/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-CD-Action") != "confirm" {
			jsonError(w, http.StatusMethodNotAllowed, "use o cabeçalho X-CD-Action: confirm")
			return
		}
		if !d.cfg.Enabled || d.cfg.Remote == "" {
			jsonError(w, http.StatusConflict, "A cópia para o Drive está desligada (driveSync no hub.config.json).")
			return
		}
		var in struct {
			Cameras []string `json:"cameras"`
		}
		if r.ContentLength != 0 {
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil && err != io.EOF {
				jsonError(w, http.StatusBadRequest, "JSON inválido")
				return
			}
		}
		known := map[string]bool{}
		for _, c := range d.m.cams.all() {
			known[c.cfg.ID] = true
		}
		for _, id := range in.Cameras {
			if !known[id] {
				jsonError(w, http.StatusBadRequest, "Câmera desconhecida: "+id)
				return
			}
		}
		if !d.run(in.Cameras...) {
			jsonError(w, http.StatusConflict, "Já há uma cópia em andamento.")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(d.status())
	})
}

// sentCount: arquivos realmente enviados (no estado, os pulados têm tamanho 0). Chamar com d.mu travado.
func (d *driveSync) sentCount() int {
	n := 0
	for _, sz := range d.st.Uploaded {
		if sz > 0 {
			n++
		}
	}
	return n
}

// skip marca como "já enviadas" (sem enviar) as gravações pendentes com mais de days dias, das câmeras em only (nil = todas), para não
// subir um acúmulo antigo. Os arquivos continuam no servidor até a retenção apagar. Devolve quantos foram pulados.
func (d *driveSync) skip(days int, only map[string]bool) int {
	cutoff := d.now().Add(-time.Duration(days) * 24 * time.Hour)
	files := d.pending(only)
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for _, f := range files {
		if f.start.Before(cutoff) {
			d.st.Uploaded[f.name] = 0
			n++
		}
	}
	if n > 0 {
		d.save()
	}
	return n
}

func (d *driveSync) registerSkip(mux *http.ServeMux) {
	// POST com X-CD-Action: confirm e {"olderThanDays": N, "cameras": [...]}: marca como já enviadas (sem enviar) as gravações pendentes
	// com mais de N dias, para pular um acúmulo antigo. Não roda durante uma cópia.
	mux.HandleFunc("POST /api/recordings/drive/skip", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-CD-Action") != "confirm" {
			jsonError(w, http.StatusMethodNotAllowed, "use o cabeçalho X-CD-Action: confirm")
			return
		}
		var in struct {
			OlderThanDays int      `json:"olderThanDays"`
			Cameras       []string `json:"cameras"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil || in.OlderThanDays < 0 || in.OlderThanDays > 365 {
			jsonError(w, http.StatusBadRequest, "Informe olderThanDays (0 a 365).")
			return
		}
		only, bad := d.prefixes(in.Cameras)
		if bad != "" {
			jsonError(w, http.StatusBadRequest, "Câmera desconhecida: "+bad)
			return
		}
		d.mu.Lock()
		busy := d.running
		d.mu.Unlock()
		if busy {
			jsonError(w, http.StatusConflict, "Há uma cópia em andamento; espere terminar.")
			return
		}
		n := d.skip(in.OlderThanDays, only)
		writeJSON(w, map[string]interface{}{"skipped": n})
	})
}

// prefixes traduz ids de câmeras para os prefixos de nome de arquivo (nil se ids está vazio = todas). O segundo valor é o primeiro id desconhecido.
func (d *driveSync) prefixes(ids []string) (map[string]bool, string) {
	if len(ids) == 0 {
		return nil, ""
	}
	byID := map[string]string{}
	for _, c := range d.m.cams.all() {
		byID[c.cfg.ID] = recName(c.cfg.Name, c.cfg.ID)
	}
	out := map[string]bool{}
	for _, id := range ids {
		p, ok := byID[id]
		if !ok {
			return nil, id
		}
		out[p] = true
	}
	return out, ""
}
