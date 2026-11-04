package main

// Subsistema "Conquistas": junta PSN, Xbox, Steam e RetroAchievements num formato único para o dashboard (src/jogos.html).
// As chaves/tokens ficam só no hub. As consultas rodam em segundo plano (a cada games.cacheMinutes) e o
// resultado é gravado em disco, então reiniciar o hub não refaz centenas de chamadas às APIs.

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type GamesConfig struct {
	CacheMinutes int    `json:"cacheMinutes"` // intervalo entre atualizações completas (padrão 360 = 6h)
	StateFile    string `json:"stateFile"`    // tokens renovados (PSN/Xbox) + cache dos dados
	Steam        struct {
		Key  string `json:"key"`  // https://steamcommunity.com/dev/apikey
		User string `json:"user"` // URL personalizada (seu-usuario) ou SteamID64
	} `json:"steam"`
	PSN struct {
		NPSSO string `json:"npsso"` // cookie npsso de https://ca.account.sony.com/api/v1/ssocookie (logado)
		User  string `json:"user"`  // só para exibição (a consulta usa a conta do npsso)
	} `json:"psn"`
	Xbox struct {
		ClientID     string `json:"clientId"`     // opcional: app do Azure; vazio = cliente público do app do Xbox
		ClientSecret string `json:"clientSecret"` // opcional (apps "confidenciais")
		Gamertag     string `json:"gamertag"`
	} `json:"xbox"`
	Retro struct {
		Key  string `json:"key"` // https://retroachievements.org/settings (Web API Key)
		User string `json:"user"`
	} `json:"retro"`
}

// ---------- formato normalizado (o que o painel recebe) ----------

type Game struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Image      string  `json:"image,omitempty"`
	System     string  `json:"system,omitempty"` // PS5, Xbox 360, SNES...
	Earned     int     `json:"earned"`
	Total      int     `json:"total"`
	Pct        float64 `json:"pct"`
	LastPlayed string  `json:"lastPlayed,omitempty"` // RFC3339
	PlayMin    int     `json:"playMin,omitempty"`
	Award      string  `json:"award,omitempty"` // platina | 100% | masterizado | zerado
	Score      int     `json:"score,omitempty"` // gamerscore / pontos RA obtidos
	ScoreTotal int     `json:"scoreTotal,omitempty"`
	Grades     *Grades `json:"grades,omitempty"` // PSN: troféus por tipo
}

type Grades struct {
	Platinum int `json:"platinum"`
	Gold     int `json:"gold"`
	Silver   int `json:"silver"`
	Bronze   int `json:"bronze"`
}

type Unlock struct {
	Game   string  `json:"game"`
	Name   string  `json:"name"`
	Desc   string  `json:"desc,omitempty"`
	Image  string  `json:"image,omitempty"`
	Date   string  `json:"date"`
	Kind   string  `json:"kind,omitempty"`   // platinum/gold/silver/bronze (PSN)
	Points int     `json:"points,omitempty"` // gamerscore / pontos RA
	Rarity float64 `json:"rarity,omitempty"` // % dos jogadores que têm
}

type Platform struct {
	ID         string                 `json:"id"` // psn | xbox | steam | retro
	Name       string                 `json:"name"`
	Account    string                 `json:"account"`
	Avatar     string                 `json:"avatar,omitempty"`
	ProfileURL string                 `json:"profileUrl,omitempty"`
	Configured bool                   `json:"configured"`
	Error      string                 `json:"error,omitempty"`
	Stale      bool                   `json:"stale,omitempty"`      // erro na última consulta: estes são os dados anteriores
	NeedsLogin bool                   `json:"needsLogin,omitempty"` // Xbox: falta conectar a conta Microsoft
	UpdatedAt  string                 `json:"updatedAt,omitempty"`
	Stats      map[string]interface{} `json:"stats"`
	Games      []Game                 `json:"games"`
	Recent     []Unlock               `json:"recent"`
}

// ---------- estado persistido ----------

type gamesState struct {
	PSNRefresh  string               `json:"psnRefresh,omitempty"`
	XboxRefresh string               `json:"xboxRefresh,omitempty"`
	SteamAch    map[string]steamAch  `json:"steamAch,omitempty"` // appid -> conquistas (refeito só quando o jogo é jogado de novo)
	Data        map[string]*Platform `json:"data,omitempty"`
}

type games struct {
	mu         sync.Mutex
	cfg        GamesConfig
	path       string
	st         gamesState
	client     *http.Client
	refreshing bool
	lastRun    time.Time
}

func newGames(cfg Config, cfgPath string) *games {
	gc := cfg.Games
	if gc.CacheMinutes <= 0 {
		gc.CacheMinutes = 360
	}
	if gc.StateFile == "" {
		gc.StateFile = "games.state.json"
	}
	p := gc.StateFile
	if !filepath.IsAbs(p) { // relativo à pasta do hub.config.json
		p = filepath.Join(filepath.Dir(cfgPath), p)
	}
	g := &games{cfg: gc, path: p, client: &http.Client{Timeout: 30 * time.Second}}
	if b, err := os.ReadFile(p); err == nil {
		if err := json.Unmarshal(b, &g.st); err != nil {
			log.Printf("jogos: %s inválido, ignorando: %v", p, err)
		}
	}
	if g.st.Data == nil {
		g.st.Data = map[string]*Platform{}
	}
	if g.st.SteamAch == nil {
		g.st.SteamAch = map[string]steamAch{}
	}
	return g
}

func (g *games) save() {
	b, err := json.Marshal(g.st)
	if err != nil {
		return
	}
	tmp := g.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err == nil {
		err = os.Rename(tmp, g.path)
		if err != nil {
			log.Printf("jogos: não gravou %s: %v", g.path, err)
		}
	}
}

// Atualiza ao iniciar (se o cache estiver velho) e depois a cada cacheMinutes.
func (g *games) loop() {
	every := time.Duration(g.cfg.CacheMinutes) * time.Minute
	oldest := time.Now()
	for _, p := range g.st.Data {
		if t, err := time.Parse(time.RFC3339, p.UpdatedAt); err == nil && t.Before(oldest) {
			oldest = t
		}
	}
	if len(g.st.Data) == 0 || time.Since(oldest) > every {
		g.refresh()
	}
	for range time.Tick(every) {
		g.refresh()
	}
}

type fetcher struct {
	id, name, account string
	configured        bool
	run               func() (*Platform, error)
}

func (g *games) fetchers() []fetcher {
	c := g.cfg
	return []fetcher{
		{"psn", "PlayStation", c.PSN.User, c.PSN.NPSSO != "", g.fetchPSN},
		{"xbox", "Xbox", c.Xbox.Gamertag, c.Xbox.Gamertag != "", g.fetchXbox},
		{"steam", "Steam", c.Steam.User, c.Steam.Key != "" && c.Steam.User != "", g.fetchSteam},
		{"retro", "RetroAchievements", c.Retro.User, c.Retro.Key != "" && c.Retro.User != "", g.fetchRetro},
	}
}

func (g *games) refresh() {
	g.mu.Lock()
	if g.refreshing {
		g.mu.Unlock()
		return
	}
	g.refreshing, g.lastRun = true, time.Now()
	g.mu.Unlock()

	var wg sync.WaitGroup
	results := make([]*Platform, 4)
	errs := make([]error, 4)
	for i, f := range g.fetchers() {
		if !f.configured {
			continue
		}
		wg.Add(1)
		go func(i int, f fetcher) {
			defer wg.Done()
			defer func() { // uma API mudando de formato não derruba o hub
				if r := recover(); r != nil {
					errs[i] = fmt.Errorf("falha inesperada: %v", r)
				}
			}()
			results[i], errs[i] = f.run()
		}(i, f)
	}
	wg.Wait()

	g.mu.Lock()
	defer g.mu.Unlock()
	for i, f := range g.fetchers() {
		if !f.configured {
			continue
		}
		if errs[i] != nil {
			log.Printf("jogos %s: %v", f.id, errs[i])
			if old := g.st.Data[f.id]; old != nil && len(old.Games) > 0 {
				old.Stale, old.Error = true, errs[i].Error()
			} else {
				g.st.Data[f.id] = &Platform{Error: errs[i].Error()}
			}
			continue
		}
		p := results[i]
		p.UpdatedAt = time.Now().Format(time.RFC3339)
		g.st.Data[f.id] = p
	}
	g.refreshing = false
	g.save()
}

func (g *games) get(force bool) map[string]interface{} {
	g.mu.Lock()
	started := false
	if force && !g.refreshing && time.Since(g.lastRun) > 5*time.Minute { // "Atualizar" no painel, no máx. a cada 5 min
		go g.refresh() // refresh() ignora chamadas enquanto outra estiver rodando
		started = true
	}
	out := make([]*Platform, 0, 4)
	for _, f := range g.fetchers() {
		p := Platform{Stats: map[string]interface{}{}, Games: []Game{}, Recent: []Unlock{}}
		if d := g.st.Data[f.id]; d != nil {
			p = *d
		}
		p.ID, p.Name, p.Configured = f.id, f.name, f.configured
		if p.Account == "" {
			p.Account = f.account
		}
		if !f.configured {
			p.Error = ""
		}
		if p.Games == nil {
			p.Games = []Game{}
		}
		if p.Recent == nil {
			p.Recent = []Unlock{}
		}
		if p.Stats == nil {
			p.Stats = map[string]interface{}{}
		}
		p.NeedsLogin = f.id == "xbox" && f.configured && g.st.XboxRefresh == ""
		out = append(out, &p)
	}
	res := map[string]interface{}{"platforms": out, "refreshing": g.refreshing || started}
	g.mu.Unlock()
	return res
}

// ---------- utilidades HTTP ----------

type apiErr struct {
	status int
	body   string
}

func (e *apiErr) Error() string { return fmt.Sprintf("HTTP %d: %s", e.status, e.body) }

func (g *games) do(req *http.Request, out interface{}) error {
	if req.Header.Get("User-Agent") == "" { // o RetroAchievements (Cloudflare) recusa o User-Agent padrão do Go
		req.Header.Set("User-Agent", "CentralDiogo/1.0 (hub caseiro)")
	}
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "pt-BR")
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		s := strings.TrimSpace(string(b))
		if strings.HasPrefix(s, "<") {
			s = "resposta HTML (bloqueio ou página de erro)"
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			s = "acesso negado (chave/token inválido?) " + s
		}
		if len(s) > 200 {
			s = s[:200]
		}
		return &apiErr{resp.StatusCode, s}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(b, out)
}

func (g *games) getJSON(u string, headers map[string]string, out interface{}) error {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return g.do(req, out)
}

func (g *games) postForm(u string, form url.Values, headers map[string]string, out interface{}) error {
	req, err := http.NewRequest("POST", u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return g.do(req, out)
}

func pct(earned, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(int(float64(earned)*1000/float64(total))) / 10
}

func sortRecent(u []Unlock, n int) []Unlock {
	sort.Slice(u, func(i, j int) bool { return u[i].Date > u[j].Date })
	if len(u) > n {
		u = u[:n]
	}
	return u
}

func unixISO(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}
