// Configurações editáveis pela página Config: câmeras, agendas (iCal), rotas do trânsito e atalhos do menu.
// Ficam em hub.settings.json, ao lado do hub.config.json, e valem mais que ele. Enquanto uma seção não for
// editada, continua valendo o que está no hub.config.json (ou, no menu, o padrão do painel), então nada muda sozinho.
// As alterações entram em vigor na hora, sem reiniciar o hub. Senhas e chaves nunca voltam para o painel: a leitura
// devolve a URL com o segredo mascarado e, ao salvar, URL/chave em branco significa "manter a atual".
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MenuLink é um atalho do rodapé do painel (mesmo formato de links em settings.example.js).
type MenuLink struct {
	Label string `json:"label"`
	Sub   string `json:"sub,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Port  int    `json:"port,omitempty"`
	Path  string `json:"path"`
	Alert string `json:"alert,omitempty"` // "estoque": o atalho avisa o que está vencendo
}

type hubSettings struct {
	Cameras   *[]CameraConfig   `json:"cameras,omitempty"`
	Calendars *[]CalendarConfig `json:"calendars,omitempty"`
	Traffic   *TrafficConfig    `json:"traffic,omitempty"`
	Menu      *[]MenuLink       `json:"menu,omitempty"`
	Panel     *PanelSettings    `json:"panel,omitempty"` // tela inicial, feriados e locais do estoque (settings_panel.go)
	Hub       *hubOptions       `json:"hub,omitempty"`   // opções do próprio hub (pastas, Drive, planilha, jogos...): valem após reiniciar
}

func loadSettings(path string) hubSettings {
	var s hubSettings
	b, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(b, &s); err != nil {
		log.Printf("%s inválido (ignorado): %v", path, err)
		return hubSettings{}
	}
	return s
}

// applyTo sobrepõe ao hub.config.json as seções que já foram editadas.
func (s hubSettings) applyTo(cfg *Config) {
	if s.Cameras != nil {
		cfg.Cameras = *s.Cameras
	}
	if s.Calendars != nil {
		cfg.Calendars = *s.Calendars
	}
	if s.Traffic != nil {
		cfg.Traffic = *s.Traffic
		cfg.Traffic.normalize()
	}
	if s.Hub != nil {
		s.Hub.applyTo(cfg)
	}
}

type settingsMgr struct {
	path string
	cfg  *Config // configuração em uso (já com o que foi editado)
	db   *store  // pode ser nil (banco indisponível)
	cams *cameras
	rec  *recMgr
	cal  *calendars
	tr   *traffic

	lmu  sync.RWMutex // protege lids (lido pelo banco a cada item gravado)
	lids []string     // ids dos locais do estoque em uso

	mu   sync.Mutex // serializa as gravações
	st   hubSettings
	boot hubOptions // opções do hub como estavam quando ele subiu: diferença para as atuais = "aguardando reinício"
}

func newSettingsMgr(path string, st hubSettings, cfg *Config, db *store, cams *cameras, rec *recMgr, cal *calendars, tr *traffic) *settingsMgr {
	m := &settingsMgr{path: path, st: st, cfg: cfg, db: db, cams: cams, rec: rec, cal: cal, tr: tr, boot: hubOptsFrom(cfg)}
	m.setLocais(m.currentLocais())
	return m
}

func (m *settingsMgr) saveLocked() error {
	b, err := json.MarshalIndent(m.st, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil { // tem senhas e chaves
		return err
	}
	return os.Rename(tmp, m.path)
}

func (m *settingsMgr) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings/cameras", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, m.camerasView()) })
	mux.HandleFunc("PUT /api/settings/cameras", m.write(m.putCameras))
	mux.HandleFunc("GET /api/settings/calendars", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, m.calendarsView()) })
	mux.HandleFunc("PUT /api/settings/calendars", m.write(m.putCalendars))
	mux.HandleFunc("GET /api/settings/traffic", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, m.trafficView()) })
	mux.HandleFunc("PUT /api/settings/traffic", m.write(m.putTraffic))
	mux.HandleFunc("GET /api/settings/menu", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, m.menuView()) })
	mux.HandleFunc("PUT /api/settings/menu", m.write(m.putMenu))
	m.registerPanel(mux)
}

// write embrulha um PUT: confere a origem e o cabeçalho de confirmação (como as outras gravações do painel), lê o JSON e responde.
func (m *settingsMgr) write(f func(body []byte) (interface{}, error)) http.HandlerFunc {
	chk := &store{origins: m.cfg.AllowedOrigins}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-CD-Action") != "confirm" {
			writeError(w, &apiError{http.StatusMethodNotAllowed, "use o cabeçalho X-CD-Action: confirm"})
			return
		}
		if err := chk.checkWrite(r); err != nil {
			writeError(w, err)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<17))
		if err != nil {
			writeError(w, badRequest("não foi possível ler o pedido"))
			return
		}
		m.mu.Lock()
		v, err := f(body)
		m.mu.Unlock()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, v)
	}
}

// ---------- utilidades ----------

// isMasked: a URL é a versão mascarada devolvida na leitura (o painel a reenviou sem editar): vale como "manter a atual".
func isMasked(u string) bool { return strings.Contains(u, ":*****@") || strings.Contains(u, "/…") }

// maskURL esconde o segredo de uma URL: a senha de um RTSP, ou o caminho (token) de um iCal.
func maskURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	if _, has := u.User.Password(); has {
		return u.Scheme + "://" + u.User.Username() + ":*****@" + u.Host + u.Path
	}
	if strings.HasPrefix(u.Scheme, "http") {
		tail := u.Path
		if len(tail) > 12 {
			tail = tail[len(tail)-12:]
		}
		return u.Scheme + "://" + u.Host + "/…" + tail
	}
	return u.Scheme + "://" + u.Host + u.Path
}

func slug(name string) string {
	s := strings.ToLower(strings.ReplaceAll(recName(name, ""), "_", "-"))
	if s == "" {
		return "item"
	}
	return s
}

func uniqueID(base string, used map[string]bool) string {
	id := base
	for i := 2; used[id]; i++ {
		id = base + "-" + strconv.Itoa(i)
	}
	used[id] = true
	return id
}

func trimLen(s string, max int, what string) (string, error) {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > max {
		return "", badRequest("%s: no máximo %d caracteres", what, max)
	}
	return s, nil
}

func decodeBody(body []byte, v interface{}) error {
	if err := json.Unmarshal(body, v); err != nil {
		return badRequest("pedido inválido: %v", err)
	}
	return nil
}

// ---------- câmeras ----------

type camView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"` // mascarada na leitura; em branco ao salvar = manter a atual
}

func (m *settingsMgr) camerasView() []camView {
	out := []camView{}
	for _, c := range m.cams.all() {
		out = append(out, camView{ID: c.cfg.ID, Name: c.cfg.Name, URL: maskURL(c.cfg.URL)})
	}
	return out
}

func (m *settingsMgr) putCameras(body []byte) (interface{}, error) {
	var in []camView
	if err := decodeBody(body, &in); err != nil {
		return nil, err
	}
	if len(in) > 16 {
		return nil, badRequest("no máximo 16 câmeras")
	}
	cur := map[string]CameraConfig{}
	for _, c := range m.cams.all() {
		cur[c.cfg.ID] = c.cfg
	}
	used := map[string]bool{}
	for _, c := range in { // ids já existentes mantêm o seu lugar
		if _, ok := cur[c.ID]; ok {
			used[c.ID] = true
		}
	}
	next := []CameraConfig{}
	seen := map[string]bool{}
	for _, c := range in {
		name, err := trimLen(c.Name, 40, "nome da câmera")
		if err != nil {
			return nil, err
		}
		if name == "" {
			return nil, badRequest("dê um nome a cada câmera")
		}
		old, exists := cur[c.ID]
		if exists && seen[c.ID] {
			return nil, badRequest("câmera repetida")
		}
		cc := CameraConfig{ID: c.ID, Name: name, URL: strings.TrimSpace(c.URL)}
		if !exists {
			cc.ID = uniqueID(slug(name), used)
		} else {
			seen[c.ID] = true
			if cc.URL == "" || isMasked(cc.URL) {
				cc.URL = old.URL
			}
		}
		if cc.URL == "" {
			return nil, badRequest("%s: informe o endereço RTSP da câmera", name)
		}
		if u, err := url.Parse(cc.URL); err != nil || (u.Scheme != "rtsp" && u.Scheme != "rtsps") || u.Host == "" {
			return nil, badRequest("%s: o endereço deve começar com rtsp://", name)
		}
		if len(cc.URL) > 500 {
			return nil, badRequest("%s: endereço longo demais", name)
		}
		next = append(next, cc)
	}
	m.st.Cameras = &next
	if err := m.saveLocked(); err != nil {
		log.Printf("salvar %s: %v", m.path, err)
		return nil, &apiError{http.StatusInternalServerError, "não foi possível gravar as configurações no servidor"}
	}
	m.cams.replace(next)
	m.rec.apply() // câmeras novas já começam a gravar conforme a chave geral; as removidas fecham o arquivo
	return m.camerasView(), nil
}

// ---------- agendas (iCal) ----------

type calView struct {
	Who   string `json:"who"`
	Label string `json:"label"`
	Color string `json:"color"`
	URL   string `json:"url"`
}

func (m *settingsMgr) calendarsView() []calView {
	out := []calView{}
	for _, c := range m.cal.sources() {
		out = append(out, calView{Who: c.Who, Label: c.Label, Color: c.Color, URL: maskURL(c.URL)})
	}
	return out
}

func (m *settingsMgr) putCalendars(body []byte) (interface{}, error) {
	var in []calView
	if err := decodeBody(body, &in); err != nil {
		return nil, err
	}
	if len(in) > 12 {
		return nil, badRequest("no máximo 12 agendas")
	}
	cur := map[string]CalendarConfig{}
	used := map[string]bool{}
	for _, c := range m.cal.sources() {
		cur[c.Who] = c
	}
	for _, c := range in {
		if _, ok := cur[c.Who]; ok {
			used[c.Who] = true
		}
	}
	next := []CalendarConfig{}
	seen := map[string]bool{}
	for _, c := range in {
		label, err := trimLen(c.Label, 40, "nome da agenda")
		if err != nil {
			return nil, err
		}
		if label == "" {
			return nil, badRequest("dê um nome a cada agenda")
		}
		color, err := trimLen(c.Color, 120, "cor")
		if err != nil {
			return nil, err
		}
		if color == "" {
			color = "#B48CFF"
		}
		old, exists := cur[c.Who]
		cc := CalendarConfig{Who: c.Who, Label: label, Color: color, URL: strings.TrimSpace(c.URL)}
		if !exists {
			cc.Who = uniqueID(slug(label), used)
		} else {
			if seen[c.Who] {
				return nil, badRequest("agenda repetida")
			}
			seen[c.Who] = true
			if cc.URL == "" || isMasked(cc.URL) {
				cc.URL = old.URL
			}
		}
		if cc.URL == "" {
			return nil, badRequest("%s: informe o endereço iCal (.ics)", label)
		}
		if u, err := url.Parse(cc.URL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return nil, badRequest("%s: o endereço deve começar com https://", label)
		}
		if !exists || old.URL != cc.URL { // endereço novo: confere se de fato é um iCal antes de guardar
			if _, err := m.cal.fetch(cc.URL); err != nil {
				return nil, badRequest("%s: não consegui ler essa agenda (%v)", label, err)
			}
		}
		next = append(next, cc)
	}
	m.st.Calendars = &next
	if err := m.saveLocked(); err != nil {
		log.Printf("salvar %s: %v", m.path, err)
		return nil, &apiError{http.StatusInternalServerError, "não foi possível gravar as configurações no servidor"}
	}
	m.cal.setSources(next)
	return m.calendarsView(), nil
}

// ---------- rotas do trânsito ----------

// routeView é uma rota como o painel de configuração a enxerga: um trecho (origem -> destino) que vale em certos dias.
type routeView struct {
	Name  string `json:"name"`
	From  string `json:"from"`  // vazio = a casa
	To    string `json:"to"`    // destino
	Home  bool   `json:"home"`  // sem deslocamento (home office)
	After string `json:"after"` // a partir de que horas vale (HH:MM); vazio = o dia todo
	Days  []int  `json:"days"`  // 0 domingo ... 6 sábado
}

type personView struct {
	ID         string      `json:"id"`
	Label      string      `json:"label"`
	ArriveBy   string      `json:"arriveBy"`
	OnDemand   bool        `json:"onDemand"`   // só consulta quando o painel pede ("Ver trânsito")
	WindowFrom string      `json:"windowFrom"` // só consulta a API entre estes horários (vazio = o dia todo)
	WindowTo   string      `json:"windowTo"`
	ShowUntil  string      `json:"showUntil"` // depois disso o cartão some
	Routes     []routeView `json:"routes"`
}

type trafficView struct {
	Home   string       `json:"home"`
	KeySet bool         `json:"keySet"`
	Key    string       `json:"key,omitempty"` // só na gravação; em branco = manter
	People []personView `json:"people"`
}

var weekOrder = []int{1, 2, 3, 4, 5, 6, 0}

func personToView(p Person) personView {
	out := personView{ID: p.ID, Label: p.Label, ArriveBy: p.ArriveBy, OnDemand: p.OnDemand, ShowUntil: p.ShowUntil, Routes: []routeView{}}
	if len(p.Window) == 2 {
		out.WindowFrom, out.WindowTo = p.Window[0], p.Window[1]
	}
	idx := map[Leg]int{}
	for _, d := range weekOrder {
		for _, l := range p.Days[strconv.Itoa(d)] {
			i, ok := idx[l]
			if !ok {
				i = len(out.Routes)
				idx[l] = i
				out.Routes = append(out.Routes, routeView{Name: l.Name, From: l.From, To: l.To, Home: l.Home, After: l.After, Days: []int{}})
			}
			out.Routes[i].Days = append(out.Routes[i].Days, d)
		}
	}
	return out
}

func (m *settingsMgr) trafficView() trafficView {
	tc := m.tr.config()
	v := trafficView{Home: tc.Home, KeySet: tc.Key != "", People: []personView{}}
	for _, p := range tc.People {
		v.People = append(v.People, personToView(p))
	}
	return v
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func validHM(s string) bool {
	t, err := time.Parse("15:04", s)
	return err == nil && t.Format("15:04") == s
}

// viewToPerson refaz a pessoa a partir da tela (nome, horários, rotas por dia).
func viewToPerson(v personView) (Person, error) {
	label, err := trimLen(v.Label, 30, "nome da pessoa")
	if err != nil {
		return Person{}, err
	}
	if label == "" {
		return Person{}, badRequest("dê um nome a cada pessoa")
	}
	p := Person{ID: v.ID, Label: label, OnDemand: v.OnDemand, Days: map[string][]Leg{}}
	for _, h := range []struct{ v, what string }{{v.ArriveBy, "horário de chegada"}, {v.WindowFrom, "início da janela"}, {v.WindowTo, "fim da janela"}, {v.ShowUntil, "\"esconder depois de\""}} {
		if h.v != "" && !validHM(h.v) {
			return p, badRequest("%s: %s deve ser HH:MM", label, h.what)
		}
	}
	if (v.WindowFrom == "") != (v.WindowTo == "") {
		return p, badRequest("%s: preencha o início e o fim da janela (ou deixe os dois em branco)", label)
	}
	p.ArriveBy, p.ShowUntil = v.ArriveBy, v.ShowUntil
	if v.WindowFrom != "" {
		p.Window = []string{v.WindowFrom, v.WindowTo}
	}
	for _, r := range v.Routes {
		name, err := trimLen(r.Name, 40, "nome da rota")
		if err != nil {
			return p, err
		}
		from, err := trimLen(r.From, 200, "origem")
		if err != nil {
			return p, err
		}
		to, err := trimLen(r.To, 200, "destino")
		if err != nil {
			return p, err
		}
		after := strings.TrimSpace(r.After)
		if after != "" && !validHM(after) {
			return p, badRequest("%s: o horário \"a partir de\" deve ser HH:MM", orDefault(name, "rota"))
		}
		if !r.Home && (to == "" || name == "") {
			return p, badRequest("%s: toda rota precisa de nome e destino", label)
		}
		if len(r.Days) == 0 {
			return p, badRequest("%s: marque ao menos um dia da semana", orDefault(name, "rota"))
		}
		leg := Leg{Name: name, To: to, From: from, After: after, Home: r.Home}
		if r.Home {
			leg = Leg{Home: true, Name: name, After: after}
		}
		seen := map[int]bool{}
		for _, d := range r.Days {
			if d < 0 || d > 6 || seen[d] {
				continue
			}
			seen[d] = true
			p.Days[strconv.Itoa(d)] = append(p.Days[strconv.Itoa(d)], leg)
		}
	}
	for _, legs := range p.Days { // dentro do dia, em ordem de horário: vale o último trecho cujo "a partir de" já passou
		sort.SliceStable(legs, func(i, j int) bool { return hm(legs[i].After) < hm(legs[j].After) })
	}
	return p, nil
}

func (m *settingsMgr) putTraffic(body []byte) (interface{}, error) {
	var in trafficView
	if err := decodeBody(body, &in); err != nil {
		return nil, err
	}
	if len(in.People) > 8 {
		return nil, badRequest("no máximo 8 pessoas")
	}
	tc := m.tr.config()
	home, err := trimLen(in.Home, 200, "endereço de casa")
	if err != nil {
		return nil, err
	}
	tc.Home = home
	if k := strings.TrimSpace(in.Key); k != "" {
		if len(k) > 200 {
			return nil, badRequest("chave longa demais")
		}
		tc.Key = k
	}
	cur := map[string]bool{}
	used := map[string]bool{}
	for _, p := range tc.People {
		cur[p.ID] = true
	}
	for _, pv := range in.People {
		if cur[pv.ID] {
			used[pv.ID] = true
		}
	}
	tc.People, tc.Diogo, tc.Cassia = nil, Person{}, Person{}
	seen := map[string]bool{}
	for _, pv := range in.People {
		p, err := viewToPerson(pv)
		if err != nil {
			return nil, err
		}
		if cur[pv.ID] {
			if seen[pv.ID] {
				return nil, badRequest("pessoa repetida")
			}
			seen[pv.ID] = true
		} else {
			p.ID = uniqueID(slug(p.Label), used)
		}
		tc.People = append(tc.People, p)
	}
	m.st.Traffic = &tc
	if err := m.saveLocked(); err != nil {
		log.Printf("salvar %s: %v", m.path, err)
		return nil, &apiError{http.StatusInternalServerError, "não foi possível gravar as configurações no servidor"}
	}
	m.tr.setConfig(tc)
	return m.trafficView(), nil
}

// ---------- menu ----------

// menuView: links == nil significa que o menu ainda é o padrão do painel (settings.js); o painel usa o dele.
func (m *settingsMgr) menuView() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.st.Menu == nil {
		return map[string]interface{}{"links": nil}
	}
	return map[string]interface{}{"links": *m.st.Menu}
}

func (m *settingsMgr) putMenu(body []byte) (interface{}, error) {
	var in struct {
		Links *[]MenuLink `json:"links"`
	}
	if err := decodeBody(body, &in); err != nil {
		return nil, err
	}
	if in.Links == nil { // volta ao padrão do painel
		m.st.Menu = nil
	} else {
		if len(*in.Links) > 24 {
			return nil, badRequest("no máximo 24 atalhos")
		}
		next := []MenuLink{}
		hasConfig := false
		for _, l := range *in.Links {
			label, err := trimLen(l.Label, 24, "nome do atalho")
			if err != nil {
				return nil, err
			}
			sub, err := trimLen(l.Sub, 40, "descrição do atalho")
			if err != nil {
				return nil, err
			}
			if label == "" {
				return nil, badRequest("dê um nome a cada atalho")
			}
			path := strings.TrimSpace(l.Path)
			if path == "" {
				path = "/"
			}
			if !strings.HasPrefix(path, "/") || len(path) > 300 || strings.ContainsAny(path, " \t\r\n") {
				return nil, badRequest("%s: o caminho deve começar com / (ex.: /estoque.html)", label)
			}
			if l.Port < 0 || l.Port > 65535 {
				return nil, badRequest("%s: porta inválida", label)
			}
			if l.Alert != "" && l.Alert != "estoque" {
				l.Alert = ""
			}
			icon := strings.TrimSpace(l.Icon)
			if len(icon) > 20 || strings.ContainsAny(icon, " <>\"'") {
				icon = "link"
			}
			if l.Port == 0 && path == "/config.html" {
				hasConfig = true
			}
			next = append(next, MenuLink{Label: label, Sub: sub, Icon: icon, Port: l.Port, Path: path, Alert: l.Alert})
		}
		if !hasConfig {
			return nil, badRequest("o atalho da Config (/config.html) não pode ser removido: é por ele que se volta a esta tela")
		}
		m.st.Menu = &next
	}
	if err := m.saveLocked(); err != nil {
		log.Printf("salvar %s: %v", m.path, err)
		return nil, &apiError{http.StatusInternalServerError, "não foi possível gravar as configurações no servidor"}
	}
	if m.st.Menu == nil {
		return map[string]interface{}{"links": nil}, nil
	}
	return map[string]interface{}{"links": *m.st.Menu}, nil
}
