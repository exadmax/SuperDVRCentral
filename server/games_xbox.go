package main

// Xbox Live: conta Microsoft -> token de usuário Xbox -> XSTS -> APIs de perfil/títulos/conquistas.
// A página do GDK (Achievements Manager) é o lado do jogo; para ler o perfil de um jogador usamos os mesmos
// serviços do Xbox Live que o app do Xbox usa (titlehub, achievements, profile).
//
// Sem app no Azure: por padrão o login usa o cliente público do próprio app do Xbox (o mesmo de várias
// ferramentas open source). Com games.xbox.clientId preenchido, usa o seu app do Azure.
// Autorizar (uma vez): botão "Conectar Xbox" no dashboard, ou  ./hub -config hub.config.json -xbox-login

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	msAuthorize   = "https://login.live.com/oauth20_authorize.srf"
	msToken       = "https://login.live.com/oauth20_token.srf"
	msRedirect    = "https://login.live.com/oauth20_desktop.srf"
	xblScopeAzure = "Xboxlive.signin Xboxlive.offline_access"
	xboxAppClient = "000000004C12AE6F" // cliente público do app do Xbox
	xblScopeApp   = "service::user.auth.xboxlive.com::MBI_SSL"
)

func (g *games) xboxClient() (id, scope string) {
	if g.cfg.Xbox.ClientID != "" {
		return g.cfg.Xbox.ClientID, xblScopeAzure
	}
	return xboxAppClient, xblScopeApp
}

func (g *games) xboxLoginURL() string {
	id, scope := g.xboxClient()
	q := url.Values{"client_id": {id}, "response_type": {"code"}, "scope": {scope}, "redirect_uri": {msRedirect}, "prompt": {"select_account"}}
	return msAuthorize + "?" + q.Encode()
}

// Aceita o endereço final colado (…oauth20_desktop.srf?code=…) ou só o código. Grava o refresh token e testa.
func (g *games) xboxAuthorize(pasted string) (*xblAuth, error) {
	pasted = strings.TrimSpace(pasted)
	code := pasted
	if u, err := url.Parse(pasted); err == nil && strings.Contains(pasted, "?") {
		if e := u.Query().Get("error"); e != "" {
			return nil, fmt.Errorf("a Microsoft recusou: %s %s", e, u.Query().Get("error_description"))
		}
		code = u.Query().Get("code")
	}
	if code == "" {
		return nil, fmt.Errorf("não encontrei o código (code=...) no endereço colado")
	}
	_, refresh, err := g.msToken(url.Values{"grant_type": {"authorization_code"}, "code": {code}})
	if err != nil || refresh == "" {
		return nil, fmt.Errorf("código inválido ou expirado (cole logo após o login): %v", err)
	}
	g.mu.Lock()
	g.st.XboxRefresh = refresh
	g.save()
	g.mu.Unlock()
	return g.xboxLogin()
}

type xblAuth struct {
	header string // "XBL3.0 x=uhs;token"
	xuid   string
	gtg    string
}

func (g *games) msToken(form url.Values) (access, refresh string, err error) {
	id, scope := g.xboxClient()
	form.Set("client_id", id)
	form.Set("scope", scope)
	form.Set("redirect_uri", msRedirect)
	if g.cfg.Xbox.ClientSecret != "" {
		form.Set("client_secret", g.cfg.Xbox.ClientSecret)
	}
	var t struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
	}
	if err = g.postForm(msToken, form, nil, &t); err != nil {
		return
	}
	return t.Access, t.Refresh, nil
}

func (g *games) xblPost(u string, body interface{}, out interface{}) error {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", u, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-xbl-contract-version", "1")
	return g.do(req, out)
}

func (g *games) xboxLogin() (*xblAuth, error) {
	if g.st.XboxRefresh == "" { // -xbox-login pode ter gravado o token com o hub rodando
		var disk gamesState
		if b, err := os.ReadFile(g.path); err == nil && json.Unmarshal(b, &disk) == nil {
			g.st.XboxRefresh = disk.XboxRefresh
		}
	}
	if g.st.XboxRefresh == "" {
		return nil, fmt.Errorf("Xbox ainda não conectado: use o botão Conectar Xbox no dashboard")
	}
	access, refresh, err := g.msToken(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {g.st.XboxRefresh}})
	if err != nil {
		return nil, fmt.Errorf("login Microsoft expirou, conecte o Xbox de novo: %v", err)
	}
	if refresh != "" {
		g.st.XboxRefresh = refresh // a Microsoft troca o refresh token a cada uso
	}

	var user struct {
		Token string `json:"Token"`
	}
	// App do Azure usa o prefixo "d="; o token do cliente do Xbox (MBI_SSL) é aceito com "t=" (ou puro, conforme a conta)
	prefixes := []string{"t=", "d=", ""}
	if g.cfg.Xbox.ClientID != "" {
		prefixes = []string{"d="}
	}
	for _, pre := range prefixes {
		err = g.xblPost("https://user.auth.xboxlive.com/user/authenticate", map[string]interface{}{
			"RelyingParty": "http://auth.xboxlive.com", "TokenType": "JWT",
			"Properties": map[string]string{"AuthMethod": "RPS", "SiteName": "user.auth.xboxlive.com", "RpsTicket": pre + access},
		}, &user)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("token Xbox: %v", err)
	}
	var xsts struct {
		Token         string `json:"Token"`
		DisplayClaims struct {
			Xui []struct {
				Uhs string `json:"uhs"`
				Xid string `json:"xid"`
				Gtg string `json:"gtg"`
			} `json:"xui"`
		} `json:"DisplayClaims"`
	}
	err = g.xblPost("https://xsts.auth.xboxlive.com/xsts/authorize", map[string]interface{}{
		"RelyingParty": "http://xboxlive.com", "TokenType": "JWT",
		"Properties": map[string]interface{}{"UserTokens": []string{user.Token}, "SandboxId": "RETAIL"},
	}, &xsts)
	if err != nil {
		return nil, fmt.Errorf("XSTS (a conta tem perfil Xbox?): %v", err)
	}
	if len(xsts.DisplayClaims.Xui) == 0 {
		return nil, fmt.Errorf("XSTS sem dados do usuário")
	}
	x := xsts.DisplayClaims.Xui[0]
	return &xblAuth{header: "XBL3.0 x=" + x.Uhs + ";" + xsts.Token, xuid: x.Xid, gtg: x.Gtg}, nil
}

func (a *xblAuth) h(contract string) map[string]string {
	return map[string]string{"Authorization": a.header, "x-xbl-contract-version": contract, "Accept": "application/json"}
}

var xboxDevices = map[string]string{"XboxSeries": "Xbox Series", "XboxOne": "Xbox One", "Xbox360": "Xbox 360", "PC": "PC", "Win32": "PC", "Mobile": "Mobile"}

func (g *games) fetchXbox() (*Platform, error) {
	a, err := g.xboxLogin()
	if err != nil {
		return nil, err
	}
	p := &Platform{Account: a.gtg, Stats: map[string]interface{}{}}
	if p.Account == "" {
		p.Account = g.cfg.Xbox.Gamertag
	}
	p.ProfileURL = "https://www.xbox.com/pt-BR/play/user/" + url.PathEscape(p.Account)

	var prof struct {
		ProfileUsers []struct {
			Settings []struct {
				ID    string `json:"id"`
				Value string `json:"value"`
			} `json:"settings"`
		} `json:"profileUsers"`
	}
	if g.getJSON("https://profile.xboxlive.com/users/xuid("+a.xuid+")/profile/settings?settings=Gamertag,Gamerscore,GameDisplayPicRaw", a.h("3"), &prof) == nil && len(prof.ProfileUsers) > 0 {
		for _, s := range prof.ProfileUsers[0].Settings {
			switch s.ID {
			case "Gamerscore":
				n, _ := strconv.Atoi(s.Value)
				p.Stats["gamerscore"] = n
			case "GameDisplayPicRaw":
				p.Avatar = s.Value
			case "Gamertag":
				p.Account = s.Value
			}
		}
	}

	var th struct {
		Titles []struct {
			TitleID     string   `json:"titleId"`
			Name        string   `json:"name"`
			Image       string   `json:"displayImage"`
			Devices     []string `json:"devices"`
			Achievement struct {
				Current      int     `json:"currentAchievements"`
				Total        int     `json:"totalAchievements"`
				CurrentScore int     `json:"currentGamerscore"`
				TotalScore   int     `json:"totalGamerscore"`
				Progress     float64 `json:"progressPercentage"`
			} `json:"achievement"`
			History struct {
				Last string `json:"lastTimePlayed"`
			} `json:"titleHistory"`
		} `json:"titles"`
	}
	u := "https://titlehub.xboxlive.com/users/xuid(" + a.xuid + ")/titles/titlehistory/decoration/Achievement,Image?maxItems=1000"
	if err := g.getJSON(u, a.h("2"), &th); err != nil {
		return nil, err
	}
	earned, perfect := 0, 0
	for _, t := range th.Titles {
		ac := t.Achievement
		if ac.Total == 0 && ac.TotalScore == 0 && ac.Current == 0 {
			continue // apps (Netflix, YouTube...) e jogos sem conquistas
		}
		var sys []string
		for _, d := range t.Devices {
			if n, ok := xboxDevices[d]; ok && !strings.Contains(strings.Join(sys, ","), n) {
				sys = append(sys, n)
			}
		}
		gm := Game{ID: t.TitleID, Name: t.Name, Image: t.Image, System: strings.Join(sys, " / "), Earned: ac.Current, Total: ac.Total,
			Pct: ac.Progress, Score: ac.CurrentScore, ScoreTotal: ac.TotalScore, LastPlayed: t.History.Last}
		if ac.Total == 0 && ac.TotalScore > 0 { // Xbox 360 às vezes só informa gamerscore
			gm.Pct = pct(ac.CurrentScore, ac.TotalScore)
		}
		if gm.Pct >= 100 {
			gm.Award = "100%"
			perfect++
		}
		earned += ac.Current
		p.Games = append(p.Games, gm)
	}
	p.Stats["games"] = len(p.Games)
	p.Stats["achievements"] = earned
	p.Stats["perfect"] = perfect

	var rec struct {
		Achievements []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Progression struct {
				Time string `json:"timeUnlocked"`
			} `json:"progression"`
			Titles []struct {
				Name string `json:"name"`
			} `json:"titleAssociations"`
			Media []struct {
				URL string `json:"url"`
			} `json:"mediaAssets"`
			Rewards []struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"rewards"`
			Rarity struct {
				Pct float64 `json:"currentPercentage"`
			} `json:"rarity"`
		} `json:"achievements"`
	}
	u = "https://achievements.xboxlive.com/users/xuid(" + a.xuid + ")/achievements?unlockedOnly=true&orderBy=UnlockTime&maxItems=30"
	if g.getJSON(u, a.h("2"), &rec) == nil {
		for _, r := range rec.Achievements {
			un := Unlock{Name: r.Name, Desc: r.Description, Date: r.Progression.Time, Rarity: r.Rarity.Pct}
			if len(r.Titles) > 0 {
				un.Game = r.Titles[0].Name
			}
			if len(r.Media) > 0 {
				un.Image = r.Media[0].URL
			}
			for _, rw := range r.Rewards {
				if rw.Type == "Gamerscore" {
					un.Points, _ = strconv.Atoi(rw.Value)
				}
			}
			p.Recent = append(p.Recent, un)
		}
	}
	p.Recent = sortRecent(p.Recent, 30)
	return p, nil
}

// Login interativo (terminal): abre a URL, entra na conta Microsoft e cola o endereço final.
func xboxLoginCLI(cfg Config, cfgPath string) {
	g := newGames(cfg, cfgPath)
	fmt.Println("1) Abra no navegador e entre com a conta do Xbox (" + g.cfg.Xbox.Gamertag + "):")
	fmt.Println("\n   " + g.xboxLoginURL() + "\n")
	fmt.Println("2) Ao terminar, a página fica em branco. Copie o endereço inteiro da barra (começa com " + msRedirect + "?code=...) e cole aqui:")
	fmt.Print("> ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	a, err := g.xboxAuthorize(line)
	if err != nil {
		fmt.Println("Falhou:", err)
		os.Exit(1)
	}
	fmt.Printf("OK: conectado como %s. O hub usa o token na próxima atualização (ou clique em Atualizar no painel).\n", a.gtg)
}
