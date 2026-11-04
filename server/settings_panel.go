// Mais configurações editáveis pela página Config (settings.go):
//   - painel: localização do clima, modo noturno, rodízio das câmeras, aviso de validade, atualização, página Vídeos,
//     feriados locais e locais do estoque. O painel busca em /api/settings/panel e vale na hora (em até 5 min nas telas abertas);
//   - hub: pastas, cópia para o Drive, planilha de profissionais, jogos e tempos de cache. Essas só valem depois de reiniciar o hub
//     (o painel mostra "aguardando reinício" e um botão que reinicia pelo systemd).
package main

import (
	"log"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// ---------- painel ----------

type LocationSettings struct {
	City     string  `json:"city"`
	State    string  `json:"state"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Timezone string  `json:"timezone"`
}

type NightSettings struct {
	From  int `json:"from"`
	Until int `json:"until"`
}

type CamerasSettings struct {
	RotateSeconds int `json:"rotateSeconds"`
}

type EstoqueSettings struct {
	SoonDays int `json:"soonDays"`
}

type VideosPanelSettings struct {
	Port int    `json:"port"`
	Path string `json:"path"`
}

type LocalHoliday struct {
	Date string `json:"date"` // MM-DD
	Name string `json:"name"`
	Type string `json:"type"` // municipal | estadual | nacional
}

// EstoqueLocal é um lugar onde se guardam itens (Geladeira, Freezer, Dispensa...).
type EstoqueLocal struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Color string `json:"color"`
	Icon  string `json:"icon"`
}

// BooksPanelSettings: página Livros (liga/desliga e as listas de categorias e de donos que o formulário de envio oferece).
type BooksPanelSettings struct {
	Enabled    bool     `json:"enabled"`
	Categories []string `json:"categories"`
	Owners     []string `json:"owners"`
}

// PanelSettings: cada grupo ausente (nil) significa "não editado": vale o padrão do painel (settings.js).
type PanelSettings struct {
	Location       *LocationSettings    `json:"location,omitempty"`
	Night          *NightSettings       `json:"night,omitempty"`
	Cameras        *CamerasSettings     `json:"cameras,omitempty"`
	Estoque        *EstoqueSettings     `json:"estoque,omitempty"`
	RefreshMinutes int                  `json:"refreshMinutes,omitempty"`
	Videos         *VideosPanelSettings `json:"videos,omitempty"`
	LocalHolidays  *[]LocalHoliday      `json:"localHolidays,omitempty"`
	Locais         *[]EstoqueLocal      `json:"locais,omitempty"`
	Books          *BooksPanelSettings  `json:"books,omitempty"`
}

var (
	tzRe    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+\-]*(/[A-Za-z0-9_+\-]+)*$`)
	colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	iconRe  = regexp.MustCompile(`^[a-z0-9\-]{1,20}$`)
)

func (m *settingsMgr) registerPanel(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings/panel", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		p := m.st.Panel
		m.mu.Unlock()
		if p == nil {
			p = &PanelSettings{}
		}
		writeJSON(w, p)
	})
	mux.HandleFunc("PUT /api/settings/panel", m.write(m.putPanel))
	mux.HandleFunc("GET /api/settings/hub", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		v := m.hubView()
		m.mu.Unlock()
		writeJSON(w, v)
	})
	mux.HandleFunc("PUT /api/settings/hub", m.write(m.putHub))
	mux.HandleFunc("POST /api/settings/hub/restart", m.write(m.restartHub))
}

// localIDs: ids válidos dos locais do estoque (usado pelo banco ao gravar um item).
func (m *settingsMgr) localIDs() []string {
	m.lmu.RLock()
	defer m.lmu.RUnlock()
	return append([]string(nil), m.lids...)
}

func (m *settingsMgr) setLocais(l []EstoqueLocal) {
	ids := make([]string, 0, len(l))
	for _, x := range l {
		ids = append(ids, x.ID)
	}
	m.lmu.Lock()
	m.lids = ids
	m.lmu.Unlock()
}

func (m *settingsMgr) currentLocais() []EstoqueLocal {
	if m.st.Panel != nil && m.st.Panel.Locais != nil {
		return *m.st.Panel.Locais
	}
	out := []EstoqueLocal{}
	for _, id := range defaultLocais {
		out = append(out, EstoqueLocal{ID: id})
	}
	return out
}

func (m *settingsMgr) putPanel(body []byte) (interface{}, error) {
	var in PanelSettings
	if err := decodeBody(body, &in); err != nil {
		return nil, err
	}
	if in.Location != nil {
		l := in.Location
		var err error
		if l.City, err = trimLen(l.City, 40, "cidade"); err != nil {
			return nil, err
		}
		if l.State, err = trimLen(l.State, 40, "estado"); err != nil {
			return nil, err
		}
		l.Timezone = strings.TrimSpace(l.Timezone)
		if l.City == "" {
			return nil, badRequest("informe a cidade")
		}
		if l.Lat < -90 || l.Lat > 90 || l.Lon < -180 || l.Lon > 180 || (l.Lat == 0 && l.Lon == 0) {
			return nil, badRequest("latitude/longitude inválidas (ex.: -23,5505 e -46,6333)")
		}
		if !tzRe.MatchString(l.Timezone) {
			return nil, badRequest("fuso horário inválido (ex.: America/Sao_Paulo)")
		}
	}
	if n := in.Night; n != nil && (n.From < 0 || n.From > 23 || n.Until < 0 || n.Until > 23) {
		return nil, badRequest("as horas do modo noturno vão de 0 a 23")
	}
	if c := in.Cameras; c != nil && (c.RotateSeconds < 5 || c.RotateSeconds > 3600) {
		return nil, badRequest("o rodízio das câmeras deve ficar entre 5 e 3600 segundos")
	}
	if e := in.Estoque; e != nil && (e.SoonDays < 0 || e.SoonDays > 365) {
		return nil, badRequest("dias de aviso de validade: de 0 a 365")
	}
	if in.RefreshMinutes != 0 && (in.RefreshMinutes < 5 || in.RefreshMinutes > 240) {
		return nil, badRequest("a atualização do clima deve ficar entre 5 e 240 minutos")
	}
	if v := in.Videos; v != nil {
		v.Path = strings.TrimSpace(v.Path)
		if v.Port < 1 || v.Port > 65535 {
			return nil, badRequest("porta dos vídeos inválida")
		}
		if !strings.HasPrefix(v.Path, "/") || len(v.Path) > 200 || strings.ContainsAny(v.Path, " \t\r\n") {
			return nil, badRequest("o caminho dos vídeos deve começar com / (ex.: /videos/)")
		}
		if !strings.HasSuffix(v.Path, "/") {
			v.Path += "/"
		}
	}
	if in.LocalHolidays != nil {
		if len(*in.LocalHolidays) > 40 {
			return nil, badRequest("no máximo 40 feriados locais")
		}
		for i := range *in.LocalHolidays {
			h := &(*in.LocalHolidays)[i]
			var err error
			if h.Name, err = trimLen(h.Name, 60, "nome do feriado"); err != nil {
				return nil, err
			}
			h.Date = strings.TrimSpace(h.Date)
			if _, err := time.Parse("01-02", h.Date); err != nil || len(h.Date) != 5 {
				return nil, badRequest("feriado %q: data deve ser MM-DD (ex.: 01-25)", h.Name)
			}
			if h.Name == "" {
				return nil, badRequest("dê um nome a cada feriado")
			}
			if h.Type != "municipal" && h.Type != "estadual" && h.Type != "nacional" {
				h.Type = "municipal"
			}
		}
	}
	if in.Locais != nil {
		if err := m.checkLocais(in.Locais); err != nil {
			return nil, err
		}
	}
	if in.Books != nil {
		var err error
		if in.Books.Categories, err = cleanNames(in.Books.Categories, 30, "categorias"); err != nil {
			return nil, err
		}
		if in.Books.Owners, err = cleanNames(in.Books.Owners, 20, "donos"); err != nil {
			return nil, err
		}
	}

	p := m.st.Panel
	if p == nil {
		p = &PanelSettings{}
	}
	if in.Location != nil {
		p.Location = in.Location
	}
	if in.Night != nil {
		p.Night = in.Night
	}
	if in.Cameras != nil {
		p.Cameras = in.Cameras
	}
	if in.Estoque != nil {
		p.Estoque = in.Estoque
	}
	if in.RefreshMinutes != 0 {
		p.RefreshMinutes = in.RefreshMinutes
	}
	if in.Videos != nil {
		p.Videos = in.Videos
	}
	if in.LocalHolidays != nil {
		p.LocalHolidays = in.LocalHolidays
	}
	if in.Locais != nil {
		p.Locais = in.Locais
	}
	if in.Books != nil {
		p.Books = in.Books
	}
	m.st.Panel = p
	if err := m.saveLocked(); err != nil {
		return nil, saveFailed(m.path, err)
	}
	if in.Locais != nil {
		m.setLocais(*in.Locais)
	}
	return p, nil
}

// checkLocais valida a lista de locais do estoque; um local só pode sair se não houver itens nele.
func (m *settingsMgr) checkLocais(list *[]EstoqueLocal) error {
	if len(*list) == 0 || len(*list) > 12 {
		return badRequest("o estoque precisa de 1 a 12 locais")
	}
	cur := map[string]bool{}
	for _, l := range m.currentLocais() {
		cur[l.ID] = true
	}
	used := map[string]bool{}
	for _, l := range *list {
		if cur[l.ID] {
			used[l.ID] = true
		}
	}
	keep := map[string]bool{}
	for i := range *list {
		l := &(*list)[i]
		var err error
		if l.Label, err = trimLen(l.Label, 20, "nome do local"); err != nil {
			return err
		}
		if l.Label == "" {
			return badRequest("dê um nome a cada local")
		}
		if !colorRe.MatchString(l.Color) {
			l.Color = "#4DA3FF"
		}
		if !iconRe.MatchString(l.Icon) {
			l.Icon = "box"
		}
		if cur[l.ID] {
			if keep[l.ID] {
				return badRequest("local repetido")
			}
			keep[l.ID] = true
		} else {
			l.ID = uniqueID(slug(l.Label), used)
			keep[l.ID] = true
		}
	}
	if m.db == nil {
		return nil
	}
	for id := range cur {
		if keep[id] {
			continue
		}
		var n int
		if err := m.db.db.QueryRow("SELECT COUNT(*) FROM itens WHERE local = ?", id).Scan(&n); err != nil {
			return &apiError{http.StatusInternalServerError, "não foi possível conferir os itens do estoque"}
		}
		if n > 0 {
			return badRequest("ainda há %d item(ns) em %q: mova ou apague antes de remover o local", n, id)
		}
	}
	return nil
}

// cleanNames valida uma lista de nomes (categorias, donos): sem vazios, até 40 caracteres, sem repetir (maiúsculas não contam).
func cleanNames(in []string, max int, what string) ([]string, error) {
	if len(in) > max {
		return nil, badRequest("no máximo %d itens na lista", max)
	}
	out, seen := []string{}, map[string]bool{}
	for _, n := range in {
		n, err := cleanLabel(n, what)
		if err != nil {
			return nil, badRequest("%v", err)
		}
		if n == "" {
			continue
		}
		if k := strings.ToLower(n); seen[k] {
			return nil, badRequest("%q aparece mais de uma vez na lista", n)
		} else {
			seen[k] = true
		}
		out = append(out, n)
	}
	return out, nil
}

// booksEnabled: a página Livros está ligada? (padrão: sim, até alguém desligar na Config)
func (m *settingsMgr) booksEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.st.Panel == nil || m.st.Panel.Books == nil || m.st.Panel.Books.Enabled
}

func saveFailed(path string, err error) error {
	log.Printf("salvar %s: %v", path, err)
	return &apiError{http.StatusInternalServerError, "não foi possível gravar as configurações no servidor"}
}

// ---------- hub ----------

type hubOptions struct {
	Recordings           RecordingsConfig `json:"recordings"`
	Videos               VideosConfig     `json:"videos"`
	DriveSync            DriveSyncConfig  `json:"driveSync"`
	Sheet                SheetConfig      `json:"sheet"`
	Games                GamesConfig      `json:"games"`
	CalendarCacheMinutes int              `json:"calendarCacheMinutes"`
	TrafficCacheMinutes  int              `json:"trafficCacheMinutes"`
	ForceIPv4            bool             `json:"forceIPv4"`
}

func hubOptsFrom(c *Config) hubOptions {
	return hubOptions{Recordings: c.Recordings, Videos: c.Videos, DriveSync: c.DriveSync, Sheet: c.ProfissionaisSheet, Games: c.Games,
		CalendarCacheMinutes: c.CalendarCacheMinutes, TrafficCacheMinutes: c.TrafficCacheMinutes, ForceIPv4: c.ForceIPv4}
}

func (o hubOptions) applyTo(c *Config) {
	c.Recordings, c.Videos, c.DriveSync, c.ProfissionaisSheet, c.Games = o.Recordings, o.Videos, o.DriveSync, o.Sheet, o.Games
	c.CalendarCacheMinutes, c.TrafficCacheMinutes, c.ForceIPv4 = o.CalendarCacheMinutes, o.TrafficCacheMinutes, o.ForceIPv4
}

func (m *settingsMgr) hubCurrent() hubOptions {
	if m.st.Hub != nil {
		return *m.st.Hub
	}
	return m.boot
}

// hubView é a forma que a tela enxerga: segredos só aparecem como "configurado" (…Set) e, ao salvar, em branco = manter.
type hubView struct {
	RecordingsDir string `json:"recordingsDir"`
	VideosDir     string `json:"videosDir"`

	DriveEnabled   bool   `json:"driveEnabled"`
	DriveRemote    string `json:"driveRemote"`
	DriveHour      int    `json:"driveHour"`
	DriveMaxFileMB int    `json:"driveMaxFileMB"`

	SheetURL     string `json:"sheetUrl"`
	SheetMinutes int    `json:"sheetMinutes"`

	GamesCacheMinutes int      `json:"gamesCacheMinutes"`
	SteamUser         string   `json:"steamUser"`
	SteamKey          string   `json:"steamKey,omitempty"`
	SteamKeySet       bool     `json:"steamKeySet"`
	PSNUser           string   `json:"psnUser"`
	PSNNpsso          string   `json:"psnNpsso,omitempty"`
	PSNNpssoSet       bool     `json:"psnNpssoSet"`
	XboxGamertag      string   `json:"xboxGamertag"`
	XboxClientID      string   `json:"xboxClientId"`
	XboxSecret        string   `json:"xboxClientSecret,omitempty"`
	XboxSecretSet     bool     `json:"xboxClientSecretSet"`
	RetroUser         string   `json:"retroUser"`
	RetroKey          string   `json:"retroKey,omitempty"`
	RetroKeySet       bool     `json:"retroKeySet"`
	ClearSecrets      []string `json:"clearSecrets,omitempty"` // só na gravação: nomes dos segredos a apagar (ex.: "steamKey")

	CalendarCacheMinutes int  `json:"calendarCacheMinutes"`
	TrafficCacheMinutes  int  `json:"trafficCacheMinutes"`
	ForceIPv4            bool `json:"forceIPv4"`

	Pending    bool `json:"pending"`    // há mudanças salvas que só valem depois de reiniciar o hub
	CanRestart bool `json:"canRestart"` // o hub roda sob o systemd e pode se reiniciar
}

func canRestart() bool { return os.Getenv("INVOCATION_ID") != "" }

func (m *settingsMgr) hubView() hubView {
	o := m.hubCurrent()
	g := o.Games
	return hubView{
		RecordingsDir: o.Recordings.Dir, VideosDir: o.Videos.Dir,
		DriveEnabled: o.DriveSync.Enabled, DriveRemote: o.DriveSync.Remote, DriveHour: o.DriveSync.Hour, DriveMaxFileMB: o.DriveSync.MaxFileMB,
		SheetURL: o.Sheet.URL, SheetMinutes: o.Sheet.Minutes,
		GamesCacheMinutes: g.CacheMinutes,
		SteamUser:         g.Steam.User, SteamKeySet: g.Steam.Key != "",
		PSNUser: g.PSN.User, PSNNpssoSet: g.PSN.NPSSO != "",
		XboxGamertag: g.Xbox.Gamertag, XboxClientID: g.Xbox.ClientID, XboxSecretSet: g.Xbox.ClientSecret != "",
		RetroUser: g.Retro.User, RetroKeySet: g.Retro.Key != "",
		CalendarCacheMinutes: o.CalendarCacheMinutes, TrafficCacheMinutes: o.TrafficCacheMinutes, ForceIPv4: o.ForceIPv4,
		Pending:    m.st.Hub != nil && !reflect.DeepEqual(*m.st.Hub, m.boot),
		CanRestart: canRestart(),
	}
}

func cleanPath(s, what string) (string, error) {
	s, err := trimLen(s, 300, what)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(s, "\x00\r\n") {
		return "", badRequest("%s: caracteres inválidos", what)
	}
	return s, nil
}

func (m *settingsMgr) putHub(body []byte) (interface{}, error) {
	var in hubView
	if err := decodeBody(body, &in); err != nil {
		return nil, err
	}
	o := m.hubCurrent()
	var err error
	if o.Recordings.Dir, err = cleanPath(in.RecordingsDir, "pasta das gravações"); err != nil {
		return nil, err
	}
	if o.Videos.Dir, err = cleanPath(in.VideosDir, "pasta dos vídeos"); err != nil {
		return nil, err
	}
	d := &o.DriveSync // o executável do rclone não é editável aqui
	d.Enabled, d.Remote = in.DriveEnabled, strings.TrimSpace(in.DriveRemote)
	if d.Enabled && !strings.Contains(d.Remote, ":") {
		return nil, badRequest("destino do Drive deve ser no formato NOME:Pasta (ex.: GDrive:CameraSeguranca)")
	}
	if in.DriveHour < 0 || in.DriveHour > 23 {
		return nil, badRequest("hora do envio ao Drive: de 0 a 23")
	}
	if in.DriveMaxFileMB < 0 || in.DriveMaxFileMB > 1<<20 {
		return nil, badRequest("tamanho máximo por arquivo inválido")
	}
	d.Hour, d.MaxFileMB = in.DriveHour, in.DriveMaxFileMB

	o.Sheet.URL = strings.TrimSpace(in.SheetURL)
	if o.Sheet.URL != "" && !strings.HasPrefix(o.Sheet.URL, "https://") && !strings.HasPrefix(o.Sheet.URL, "http://") {
		return nil, badRequest("o endereço da planilha deve começar com https://")
	}
	if in.SheetMinutes < 0 || in.SheetMinutes > 1440 {
		return nil, badRequest("intervalo da planilha: de 1 a 1440 minutos")
	}
	o.Sheet.Minutes = in.SheetMinutes
	if o.Sheet.URL != "" && o.Sheet.Minutes == 0 {
		o.Sheet.Minutes = 5
	}

	if in.GamesCacheMinutes < 0 || in.GamesCacheMinutes > 10080 {
		return nil, badRequest("intervalo dos jogos: até 10080 minutos")
	}
	g := &o.Games
	g.CacheMinutes = in.GamesCacheMinutes
	for _, f := range []struct {
		dst  *string
		src  string
		what string
	}{{&g.Steam.User, in.SteamUser, "usuário Steam"}, {&g.PSN.User, in.PSNUser, "usuário PSN"}, {&g.Xbox.Gamertag, in.XboxGamertag, "gamertag"},
		{&g.Xbox.ClientID, in.XboxClientID, "client id do Xbox"}, {&g.Retro.User, in.RetroUser, "usuário RetroAchievements"}} {
		if *f.dst, err = trimLen(f.src, 80, f.what); err != nil {
			return nil, err
		}
	}
	for _, f := range []struct {
		dst  *string
		src  string
		name string
	}{{&g.Steam.Key, in.SteamKey, "steamKey"}, {&g.PSN.NPSSO, in.PSNNpsso, "psnNpsso"}, {&g.Xbox.ClientSecret, in.XboxSecret, "xboxClientSecret"}, {&g.Retro.Key, in.RetroKey, "retroKey"}} {
		if v := strings.TrimSpace(f.src); v != "" {
			if len(v) > 500 {
				return nil, badRequest("segredo longo demais")
			}
			*f.dst = v
		}
		for _, c := range in.ClearSecrets {
			if c == f.name {
				*f.dst = ""
			}
		}
	}

	if in.CalendarCacheMinutes < 1 || in.CalendarCacheMinutes > 1440 || in.TrafficCacheMinutes < 1 || in.TrafficCacheMinutes > 1440 {
		return nil, badRequest("os tempos de cache vão de 1 a 1440 minutos")
	}
	o.CalendarCacheMinutes, o.TrafficCacheMinutes, o.ForceIPv4 = in.CalendarCacheMinutes, in.TrafficCacheMinutes, in.ForceIPv4

	m.st.Hub = &o
	if err := m.saveLocked(); err != nil {
		return nil, saveFailed(m.path, err)
	}
	return m.hubView(), nil
}

// restartHub encerra o processo; o systemd (Restart=always) sobe o hub de novo em poucos segundos com as opções novas.
func (m *settingsMgr) restartHub(_ []byte) (interface{}, error) {
	if !canRestart() {
		return nil, &apiError{http.StatusNotImplemented, "reinício automático só no servidor Linux (systemd); reinicie o hub à mão"}
	}
	go func() {
		time.Sleep(700 * time.Millisecond)
		os.Exit(0)
	}()
	return map[string]bool{"ok": true}, nil
}
