// Hub da DVR: serve o painel (pasta src/) e expõe /api/* para TV, celular e web.
// Guarda as URLs secretas do calendário e a chave da TomTom fora dos aparelhos e resolve o CORS do Google.
// Apenas biblioteca padrão: binário pequeno e pouca memória (feito para máquinas com 2 GB).
package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dvr/hub/ai"
)

type Config struct {
	Listen               string           `json:"listen"`
	StaticDir            string           `json:"staticDir"`
	Calendars            []CalendarConfig `json:"calendars"`
	Traffic              TrafficConfig    `json:"traffic"`
	CalendarCacheMinutes int              `json:"calendarCacheMinutes"`
	TrafficCacheMinutes  int              `json:"trafficCacheMinutes"` // intervalo entre consultas dentro da janela
	ForceIPv4            bool             `json:"forceIPv4"`
	Games                GamesConfig      `json:"games"`
	DBPath               string           `json:"dbPath"` // banco SQLite (estoque e contatos); relativo à pasta do hub.config.json
	ProfissionaisSheet   SheetConfig      `json:"profissionaisSheet"`
	Videos               VideosConfig     `json:"videos"`         // pasta dos vídeos servida pelo nginx em /videos/ (lista da página Vídeos)
	Books                BooksConfig      `json:"books"`          // pasta dos livros PDF/EPUB (página Livros: lista, envio e leitor)
	Cameras              []CameraConfig   `json:"cameras"`        // câmeras ao vivo (RTSP H.264); a URL com a senha nunca sai do hub
	Recordings           RecordingsConfig `json:"recordings"`     // pasta das gravações das câmeras (liga/desliga e retenção: página Config)
	DriveSync            DriveSyncConfig  `json:"driveSync"`      // cópia diária das gravações para o Google Drive via rclone (drive.go)
	AllowedOrigins       []string         `json:"allowedOrigins"` // origens extras com permissão de gravar (ex.: painel em dev em outra porta)
	AI                   ai.Config        `json:"ai"`             // provedor de IA (Ollama local ou Groq) e modelo; a chave da Groq nunca sai do hub
}

type CalendarConfig struct {
	Who   string `json:"who"`   // identificador
	Label string `json:"label"` // nome exibido no painel
	Color string `json:"color"` // cor (CSS) da categoria
	URL   string `json:"url"`
}

type TrafficConfig struct {
	Key    string   `json:"key"`  // chave do Google Routes API
	Home   string   `json:"home"` // endereço de saída padrão
	People []Person `json:"people"`
	Diogo  Person   `json:"diogo"`  // formato antigo: vira o primeiro item de People (normalize)
	Cassia Person   `json:"cassia"` // idem
}

type SheetConfig struct {
	URL     string `json:"url"`     // CSV da aba de profissionais (Google Planilhas); vazio desliga a sincronização
	Minutes int    `json:"minutes"` // intervalo entre sincronizações (padrão 5)
}

// homeDir é a pasta pessoal de quem roda o hub (base dos padrões de gravações e vídeos).
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return "."
}

func loadConfig(path string) Config {
	c := Config{Listen: ":8080", StaticDir: "../src", CalendarCacheMinutes: 5, TrafficCacheMinutes: 10, ForceIPv4: true, DBPath: "dvr.db", Videos: VideosConfig{Dir: filepath.Join(homeDir(), "Videos")}, Books: BooksConfig{Dir: filepath.Join(homeDir(), "Livros")}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Primeira execução: sobe com os padrões e tudo (câmeras, agendas, trânsito...) se configura pela página Config.
		log.Printf("%s não existe: usando os padrões. Abra http://<este-servidor>%s/config.html para configurar (ou copie hub.config.example.json).", path, c.Listen)
		return c
	}
	if err != nil {
		log.Fatalf("não foi possível ler %s: %v", path, err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		log.Fatalf("hub.config.json inválido: %v", err)
	}
	c.Traffic.normalize()
	return c
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

// CORS aberto: o hub só é acessível dentro da rede de casa.
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") { // painel sempre atualizado após um deploy
			w.Header().Set("Cache-Control", "no-cache")
		}
		h.ServeHTTP(w, r)
	})
}

func main() {
	cfgPath := flag.String("config", "hub.config.json", "caminho do arquivo de configuração")
	xboxLogin := flag.Bool("xbox-login", false, "autoriza a conta Xbox (uma vez) e sai")
	flag.Parse()
	initLogFile(filepath.Dir(*cfgPath))
	cfg := loadConfig(*cfgPath)
	// Câmeras, agendas, rotas e menu editados na página Config ficam em hub.settings.json e valem mais que o hub.config.json.
	settingsPath := filepath.Join(filepath.Dir(*cfgPath), "hub.settings.json")
	saved := loadSettings(settingsPath)
	saved.applyTo(&cfg)
	if *xboxLogin {
		xboxLoginCLI(cfg, *cfgPath)
		return
	}

	cal := newCalendars(cfg)
	tr := newTraffic(cfg)
	gm := newGames(cfg, *cfgPath)
	go gm.loop()

	// Se o banco não abrir, o resto do hub (agenda, clima, jogos) segue no ar e /api/estoque e /api/contatos respondem 503.
	if !filepath.IsAbs(cfg.DBPath) { // relativo à pasta do hub.config.json, como o stateFile dos jogos
		cfg.DBPath = filepath.Join(filepath.Dir(*cfgPath), cfg.DBPath)
	}
	db, err := openStore(cfg.DBPath, cfg.AllowedOrigins)
	if err != nil {
		log.Printf("banco de dados %s: %v", cfg.DBPath, err)
		db = nil
	}
	sm := newSettingsMgr(settingsPath, saved, &cfg, db, nil, nil, nil, nil)
	if db != nil {
		db.locais = sm.localIDs
	}

	// Versão instalada: arquivo VERSION ao lado do hub.config.json (o install.sh soma 1 ao terceiro número a cada instalação)
	verDir := filepath.Dir(*cfgPath)

	mux := http.NewServeMux()
	db.register(mux)
	cams := newCameras(cfg.Cameras)
	cams.register(mux)
	rec := newRecMgr(cfg.Recordings, filepath.Join(filepath.Dir(*cfgPath), "recordings.state.json"), cams)
	rec.register(mux)
	rec.start()
	ds := newDriveSync(cfg.DriveSync, filepath.Join(filepath.Dir(*cfgPath), "drive.state.json"), rec)
	ds.register(mux)
	ds.start()
	sm.cams, sm.rec, sm.cal, sm.tr = cams, rec, cal, tr
	sm.register(mux)
	ai.Register(mux, sm.aiConfig)
	newOmega(cams, tr, sm.aiConfig, cfg.AllowedOrigins).register(mux)
	sys := newSysMon()
	go sys.loop()
	mux.HandleFunc("/api/videos", videosHandler(cfg.Videos))
	mux.HandleFunc("/api/videos/sub", subHandler(cfg.Videos))
	newBooks(cfg.Books, cfg.AllowedOrigins, sm.booksEnabled).register(mux)
	if db != nil && cfg.ProfissionaisSheet.URL != "" {
		go db.sheetLoop(cfg.ProfissionaisSheet.URL, time.Duration(cfg.ProfissionaisSheet.Minutes)*time.Minute)
	}
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		v, _ := readVersion(verDir)
		writeJSON(w, map[string]interface{}{"ok": true, "time": time.Now().Format(time.RFC3339), "version": v})
	})
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		v, at := readVersion(verDir)
		writeJSON(w, map[string]interface{}{"version": v, "installedAt": at})
	})
	mux.HandleFunc("/api/system", sys.handler)
	mux.HandleFunc("/api/admin/desktop-off", powerHandler(sys, dmUnit, false))
	mux.HandleFunc("/api/admin/hub-off", powerHandler(sys, "dvr-hub.service", true))
	mux.HandleFunc("/api/ics", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, cal.get()) })
	mux.HandleFunc("/api/traffic", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		ask := map[string]bool{}
		for _, id := range strings.Split(q.Get("ask"), ",") { // pessoas que o painel pediu agora (as "só quando eu pedir")
			ask[strings.TrimSpace(id)] = true
		}
		if q.Get("diogo") == "1" { // formato antigo
			ask["diogo"] = true
		}
		writeJSON(w, tr.get(ask))
	})
	mux.HandleFunc("/api/games", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, gm.get(r.URL.Query().Get("refresh") == "1"))
	})
	// Conectar o Xbox pelo dashboard: GET devolve a URL de login; POST {"url": "<endereço final colado>"}
	mux.HandleFunc("/api/games/xbox-login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, map[string]string{"url": gm.xboxLoginURL()})
			return
		}
		var body struct {
			URL string `json:"url"`
		}
		json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
		a, err := gm.xboxAuthorize(body.URL)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		go gm.refresh()
		writeJSON(w, map[string]interface{}{"ok": true, "gamertag": a.gtg})
	})
	mux.Handle("/", newStaticHandler(cfg.StaticDir, func() string { v, _ := readVersion(verDir); return v }))

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ver, _ := readVersion(verDir)
	if ver != "dev" {
		ver = "V" + ver
	}
	log.Printf("DVR hub %s em %s (painel: %s)", ver, cfg.Listen, cfg.StaticDir)
	log.Fatal(srv.ListenAndServe())
}
