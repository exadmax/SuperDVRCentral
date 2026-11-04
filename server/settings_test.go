package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dvr/hub/ai"
)

func testSettings(t *testing.T) (*settingsMgr, *http.ServeMux, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{CalendarCacheMinutes: 5, TrafficCacheMinutes: 10}
	cfg.Cameras = []CameraConfig{{ID: "garagem", Name: "Garagem", URL: "rtsp://admin:segredo@10.0.0.1:554/cam"}}
	cfg.Traffic.Home = "Casa"
	cfg.Traffic.Key = "k"
	cfg.Traffic.Cassia = Person{Days: map[string][]Leg{ // formato antigo: vira a pessoa "cassia"
		"1": {{Name: "A", To: "Rua A"}}, "4": {{Name: "A", To: "Rua A"}}, "3": {{Home: true}},
	}}
	cams := newCameras(cfg.Cameras)
	rec := newRecMgr(RecordingsConfig{Dir: filepath.Join(dir, "rec")}, filepath.Join(dir, "rec.json"), cams)
	m := newSettingsMgr(filepath.Join(dir, "hub.settings.json"), hubSettings{}, &cfg, nil, cams, rec, newCalendars(cfg), newTraffic(cfg))
	mux := http.NewServeMux()
	m.register(mux)
	return m, mux, dir
}

func put(mux *http.ServeMux, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("PUT", path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CD-Action", "confirm")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func TestSettingsCameras(t *testing.T) {
	m, mux, dir := testSettings(t)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/settings/cameras", nil))
	if strings.Contains(w.Body.String(), "segredo") || !strings.Contains(w.Body.String(), "*****") {
		t.Fatalf("a senha deve sair mascarada: %s", w.Body)
	}
	// mantém a existente (URL em branco), acrescenta uma nova
	w = put(mux, "/api/settings/cameras", `[{"id":"garagem","name":"Garagem","url":""},{"name":"Sala de Estar","url":"rtsp://10.0.0.2/live"}]`)
	if w.Code != 200 {
		t.Fatalf("PUT: %d %s", w.Code, w.Body)
	}
	if len(m.cams.all()) != 2 || m.cams.get("sala-de-estar") == nil || m.cams.get("garagem").cfg.URL != "rtsp://admin:segredo@10.0.0.1:554/cam" {
		t.Fatalf("lista inesperada: %+v", m.cams.all())
	}
	if m.cams.get("sala-de-estar").rec == nil {
		t.Fatal("câmera nova sem gravador")
	}
	if _, err := os.Stat(filepath.Join(dir, "hub.settings.json")); err != nil {
		t.Fatal("não gravou o arquivo")
	}
	// remove a garagem
	w = put(mux, "/api/settings/cameras", `[{"id":"sala-de-estar","name":"Sala de Estar","url":""}]`)
	if w.Code != 200 || m.cams.get("garagem") != nil || len(m.cams.all()) != 1 {
		t.Fatalf("remoção: %d %s", w.Code, w.Body)
	}
	if w = put(mux, "/api/settings/cameras", `[{"name":"X","url":"http://x"}]`); w.Code != 400 {
		t.Fatalf("URL que não é rtsp deveria falhar: %d", w.Code)
	}
}

func TestSettingsNeedsConfirm(t *testing.T) {
	_, mux, _ := testSettings(t)
	r := httptest.NewRequest("PUT", "/api/settings/menu", strings.NewReader(`{"links":null}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code == 200 {
		t.Fatal("sem X-CD-Action não pode gravar")
	}
}

func TestSettingsTrafficRoundTrip(t *testing.T) {
	m, mux, _ := testSettings(t)
	v := m.trafficView()
	if len(v.People) != 1 || v.People[0].ID != "cassia" || len(v.People[0].Routes) != 2 || len(v.People[0].Routes[0].Days) != 2 {
		t.Fatalf("agrupamento por rota: %+v", v.People)
	}
	b, _ := json.Marshal(v)
	if w := put(mux, "/api/settings/traffic", string(b)); w.Code != 200 {
		t.Fatalf("PUT: %d %s", w.Code, w.Body)
	}
	got := m.tr.config()
	c := got.People[0]
	if got.Key != "k" || c.ID != "cassia" || len(c.Days["1"]) != 1 || !c.Days["3"][0].Home || c.Days["4"][0].To != "Rua A" {
		t.Fatalf("rotas mudaram no vai-e-volta: %+v", got)
	}
	// rota sem destino é recusada
	bad := `{"home":"Casa","people":[{"id":"cassia","label":"Cássia","routes":[{"name":"X","to":"","days":[1]}]}]}`
	if w := put(mux, "/api/settings/traffic", bad); w.Code != 400 {
		t.Fatalf("rota sem destino deveria falhar: %d", w.Code)
	}
}

func TestSettingsMenu(t *testing.T) {
	m, mux, _ := testSettings(t)
	if w := put(mux, "/api/settings/menu", `{"links":[{"label":"Vídeos","path":"/videos.html"}]}`); w.Code != 400 {
		t.Fatalf("menu sem Config deveria falhar: %d", w.Code)
	}
	ok := `{"links":[{"label":"Config","path":"/config.html","icon":"gear"},{"label":"Música","port":4533,"path":"/"}]}`
	if w := put(mux, "/api/settings/menu", ok); w.Code != 200 || m.st.Menu == nil || len(*m.st.Menu) != 2 {
		t.Fatalf("PUT menu: %d", w.Code)
	}
	if w := put(mux, "/api/settings/menu", `{"links":null}`); w.Code != 200 || m.st.Menu != nil {
		t.Fatal("links:null deveria voltar ao padrão")
	}
}

func TestSettingsTrafficAddRemovePerson(t *testing.T) {
	m, mux, _ := testSettings(t)
	body := `{"home":"Casa","people":[{"id":"cassia","label":"Cássia","routes":[{"name":"A","to":"Rua A","days":[1]}]},` +
		`{"label":"João Pedro","onDemand":true,"windowFrom":"06:00","windowTo":"08:00","showUntil":"09:00","routes":[{"name":"Escola","to":"Rua B","days":[2,3]}]}]}`
	if w := put(mux, "/api/settings/traffic", body); w.Code != 200 {
		t.Fatalf("PUT: %d %s", w.Code, w.Body)
	}
	p := m.tr.config().People
	if len(p) != 2 || p[1].ID != "joao-pedro" || !p[1].OnDemand || len(p[1].Window) != 2 || p[1].ShowUntil != "09:00" {
		t.Fatalf("pessoas: %+v", p)
	}
	// remove a primeira
	if w := put(mux, "/api/settings/traffic", `{"home":"Casa","people":[{"id":"joao-pedro","label":"João Pedro","routes":[{"name":"Escola","to":"Rua B","days":[2]}]}]}`); w.Code != 200 {
		t.Fatalf("remoção: %d %s", w.Code, w.Body)
	}
	if p = m.tr.config().People; len(p) != 1 || p[0].ID != "joao-pedro" {
		t.Fatalf("depois de remover: %+v", p)
	}
	// janela pela metade é recusada
	if w := put(mux, "/api/settings/traffic", `{"people":[{"label":"X","windowFrom":"06:00","routes":[]}]}`); w.Code != 400 {
		t.Fatalf("janela incompleta deveria falhar: %d", w.Code)
	}
}

func TestSettingsPanel(t *testing.T) {
	m, mux, _ := testSettings(t)
	ok := `{"location":{"city":"Campinas","state":"SP","lat":-22.9,"lon":-47.06,"timezone":"America/Sao_Paulo"},"night":{"from":22,"until":6},` +
		`"cameras":{"rotateSeconds":30},"estoque":{"soonDays":5},"refreshMinutes":20,"videos":{"port":8080,"path":"/filmes"},` +
		`"localHolidays":[{"date":"01-25","name":"Aniversário","type":"municipal"}]}`
	if w := put(mux, "/api/settings/panel", ok); w.Code != 200 {
		t.Fatalf("PUT: %d %s", w.Code, w.Body)
	}
	p := m.st.Panel
	if p.Location.City != "Campinas" || p.Night.From != 22 || p.Videos.Path != "/filmes/" || len(*p.LocalHolidays) != 1 {
		t.Fatalf("painel: %+v", p)
	}
	for name, bad := range map[string]string{
		"hora":    `{"night":{"from":25,"until":6}}`,
		"lat":     `{"location":{"city":"X","lat":0,"lon":0,"timezone":"America/Sao_Paulo"}}`,
		"rodízio": `{"cameras":{"rotateSeconds":1}}`,
		"data":    `{"localHolidays":[{"date":"13-40","name":"x","type":"municipal"}]}`,
		"locais":  `{"locais":[]}`,
	} {
		if w := put(mux, "/api/settings/panel", bad); w.Code != 400 {
			t.Errorf("%s deveria falhar: %d", name, w.Code)
		}
	}
	// um merge parcial não apaga o que já estava
	put(mux, "/api/settings/panel", `{"estoque":{"soonDays":7}}`)
	if m.st.Panel.Location == nil || m.st.Panel.Estoque.SoonDays != 7 {
		t.Fatal("merge parcial")
	}
}

func TestSettingsLocais(t *testing.T) {
	m, mux, _ := testSettings(t)
	w := put(mux, "/api/settings/panel", `{"locais":[{"id":"geladeira","label":"Geladeira","color":"#4DA3FF","icon":"fridge"},{"label":"Garagem","color":"nope","icon":"box"}]}`)
	if w.Code != 200 {
		t.Fatalf("PUT: %d %s", w.Code, w.Body)
	}
	ids := m.localIDs()
	if len(ids) != 2 || ids[1] != "garagem" {
		t.Fatalf("ids: %v", ids)
	}
	if (&store{locais: m.localIDs}).localOK("freezer") || !(&store{locais: m.localIDs}).localOK("garagem") {
		t.Fatal("validação de local")
	}
}

func TestSettingsHub(t *testing.T) {
	m, mux, _ := testSettings(t)
	m.cfg.Games.Steam.Key = "SEGREDO"
	m.boot = hubOptsFrom(m.cfg)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/settings/hub", nil))
	if strings.Contains(w.Body.String(), "SEGREDO") || !strings.Contains(w.Body.String(), `"steamKeySet":true`) {
		t.Fatalf("segredo deve vir só como configurado: %s", w.Body)
	}
	body := `{"recordingsDir":"/dados/cam","videosDir":"/dados/videos","driveEnabled":true,"driveRemote":"GDrive:Cam","driveHour":4,"driveMaxFileMB":512,` +
		`"sheetUrl":"https://docs.google.com/x","sheetMinutes":10,"gamesCacheMinutes":120,"steamUser":"fulano","calendarCacheMinutes":5,"trafficCacheMinutes":10,"forceIPv4":true}`
	if w := put(mux, "/api/settings/hub", body); w.Code != 200 || !strings.Contains(w.Body.String(), `"pending":true`) {
		t.Fatalf("PUT: %d %s", w.Code, w.Body)
	}
	if m.st.Hub.Games.Steam.Key != "SEGREDO" || m.st.Hub.DriveSync.Hour != 4 || m.st.Hub.Recordings.Dir != "/dados/cam" {
		t.Fatalf("opções: %+v", m.st.Hub)
	}
	var cfg Config
	m.st.applyTo(&cfg)
	if cfg.Recordings.Dir != "/dados/cam" || cfg.Games.Steam.User != "fulano" {
		t.Fatal("applyTo")
	}
	if w := put(mux, "/api/settings/hub", `{"driveEnabled":true,"driveRemote":"semdoispontos","driveHour":3,"calendarCacheMinutes":5,"trafficCacheMinutes":10}`); w.Code != 400 {
		t.Fatalf("remote inválido deveria falhar: %d", w.Code)
	}
	if w := put(mux, "/api/settings/hub", `{"clearSecrets":["steamKey"],"calendarCacheMinutes":5,"trafficCacheMinutes":10}`); w.Code != 200 || m.st.Hub.Games.Steam.Key != "" {
		t.Fatalf("clearSecrets: %d", w.Code)
	}
}

// A migração que solta o CHECK de "local" precisa preservar os itens já guardados.
func TestMigrationKeepsItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	s, err := openStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.db.Exec("PRAGMA user_version = 3") // simula banco na versão anterior
	s.db.Exec("DROP TABLE itens")
	s.db.Exec(`CREATE TABLE itens (id INTEGER PRIMARY KEY AUTOINCREMENT, nome TEXT NOT NULL, quantidade REAL, peso_kg REAL, validade TEXT,
		local TEXT NOT NULL CHECK (local IN ('freezer','geladeira','dispensa')), criado_em TEXT NOT NULL DEFAULT 'x', atualizado_em TEXT NOT NULL DEFAULT 'x')`)
	s.db.Exec("INSERT INTO itens (nome, quantidade, local) VALUES ('Leite', 2, 'geladeira'), ('Frango', 1, 'freezer')")
	s.Close()
	s, err = openStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM itens").Scan(&n)
	if _, err := s.db.Exec("INSERT INTO itens (nome, quantidade, local) VALUES ('Tinta', 1, 'garagem')"); err != nil || n != 2 {
		t.Fatalf("itens=%d err=%v", n, err)
	}
}

func TestSettingsBooks(t *testing.T) {
	m, mux, _ := testSettings(t)
	if !m.booksEnabled() {
		t.Fatal("a página Livros deve vir ligada por padrão")
	}
	w := put(mux, "/api/settings/panel", `{"books":{"enabled":true,"categories":["  Romance ","","Técnico"],"owners":["Ana","Bia"]}}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"categories":["Romance","Técnico"]`) {
		t.Fatalf("salvar listas: %d %s", w.Code, w.Body)
	}
	for _, bad := range []string{
		`{"books":{"enabled":true,"categories":["A","a"],"owners":[]}}`,                           // repetida
		`{"books":{"enabled":true,"categories":[],"owners":["` + strings.Repeat("x", 41) + `"]}}`, // longa demais
	} {
		if w := put(mux, "/api/settings/panel", bad); w.Code != 400 {
			t.Errorf("deveria recusar %s: %d", bad, w.Code)
		}
	}
	if w := put(mux, "/api/settings/panel", `{"books":{"enabled":false,"categories":[],"owners":[]}}`); w.Code != 200 || m.booksEnabled() {
		t.Fatalf("desligar: %d enabled=%v", w.Code, m.booksEnabled())
	}
}

func TestSettingsAI(t *testing.T) {
	m, mux, _ := testSettings(t)
	get := func() map[string]interface{} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/settings/ai", nil))
		var v map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &v)
		return v
	}
	if v := get(); v["provider"] != "ollama" || v["ollamaURL"] != ai.DefaultOllamaURL || v["groqKeySet"] != false {
		t.Fatalf("padrões: %v", v)
	}
	if w := put(mux, "/api/settings/ai", `{"provider":"groq","groqModel":"openai/gpt-oss-20b"}`); w.Code != 400 {
		t.Fatalf("groq sem chave deveria falhar: %d", w.Code)
	}
	if w := put(mux, "/api/settings/ai", `{"provider":"ollama","ollamaURL":"ftp://x"}`); w.Code != 400 {
		t.Fatalf("URL inválida deveria falhar: %d", w.Code)
	}
	w := put(mux, "/api/settings/ai", `{"provider":"groq","model":"qwen3:4b","groqKey":"gsk_SEGREDO"}`)
	if w.Code != 200 || strings.Contains(w.Body.String(), "SEGREDO") {
		t.Fatalf("gravar: %d %s", w.Code, w.Body.String())
	}
	if v := get(); v["groqKeySet"] != true || v["model"] != "qwen3:4b" || strings.Contains(fmt.Sprint(v), "SEGREDO") {
		t.Fatalf("leitura não pode trazer a chave: %v", v)
	}
	// reenviar sem a chave mantém a atual
	if w := put(mux, "/api/settings/ai", `{"provider":"groq","model":"qwen3:8b"}`); w.Code != 200 || m.aiConfig().GroqKey != "gsk_SEGREDO" {
		t.Fatalf("manter chave: %d", w.Code)
	}
	if w := put(mux, "/api/settings/ai", `{"provider":"off","model":"qwen3:8b","clearGroqKey":true}`); w.Code != 200 || m.aiConfig().GroqKey != "" || m.st.AI.Provider != "off" {
		t.Fatalf("apagar chave: %d", w.Code)
	}
	var cfg Config
	m.st.applyTo(&cfg)
	if cfg.AI.Provider != "off" || cfg.AI.Model != "qwen3:8b" {
		t.Fatalf("applyTo: %+v", cfg.AI)
	}
}
