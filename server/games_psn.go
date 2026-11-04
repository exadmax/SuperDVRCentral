package main

// PlayStation Network: mesmo fluxo da biblioteca psnawp (https://pypi.org/project/psnawp/), em Go.
// npsso (cookie de login da Sony, ~2 meses) -> código OAuth -> access/refresh token (o refresh fica no estado do hub).

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	psnAuth     = "https://ca.account.sony.com/api/authz/v3/oauth/"
	psnAPI      = "https://m.np.playstation.com/api/"
	psnClientID = "09515159-7237-4370-9b40-3806e67c0891" // app oficial do PlayStation (o mesmo usado pela psnawp)
	psnBasic    = "Basic MDk1MTUxNTktNzIzNy00MzcwLTliNDAtMzgwNmU2N2MwODkxOnVjUGprYTV0bnRCMktxc1A="
	psnRedirect = "com.scee.psxandroid.scecompcall://redirect"
	psnScope    = "psn:mobile.v2.core psn:clientapp"
)

type psnToken struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}

func (g *games) psnLogin() (string, error) {
	form := url.Values{"token_format": {"jwt"}}
	if g.st.PSNRefresh != "" {
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", g.st.PSNRefresh)
		form.Set("scope", psnScope)
		var t psnToken
		if err := g.postForm(psnAuth+"token", form, map[string]string{"Authorization": psnBasic}, &t); err == nil && t.Access != "" {
			if t.Refresh != "" {
				g.st.PSNRefresh = t.Refresh
			}
			return t.Access, nil
		}
		g.st.PSNRefresh = "" // expirou: volta para o npsso
	}

	q := url.Values{"access_type": {"offline"}, "client_id": {psnClientID}, "response_type": {"code"}, "scope": {psnScope}, "redirect_uri": {psnRedirect}}
	req, _ := http.NewRequest("GET", psnAuth+"authorize?"+q.Encode(), nil)
	req.Header.Set("Cookie", "npsso="+g.cfg.PSN.NPSSO)
	noRedirect := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc, _ := url.Parse(resp.Header.Get("Location"))
	code := ""
	if loc != nil {
		code = loc.Query().Get("code")
	}
	if code == "" {
		return "", fmt.Errorf("npsso inválido ou expirado: gere outro em https://ca.account.sony.com/api/v1/ssocookie (logado na PSN) e atualize games.psn.npsso")
	}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", psnRedirect)
	var t psnToken
	if err := g.postForm(psnAuth+"token", form, map[string]string{"Authorization": psnBasic}, &t); err != nil {
		return "", err
	}
	g.st.PSNRefresh = t.Refresh
	return t.Access, nil
}

// "PT12H34M5S" -> minutos
var isoDur = regexp.MustCompile(`^P(?:(\d+)D)?T?(?:(\d+)H)?(?:(\d+)M)?(?:[\d.]+S)?$`)

func isoMinutes(s string) int {
	m := isoDur.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n := func(x string) int { v, _ := strconv.Atoi(x); return v }
	return n(m[1])*1440 + n(m[2])*60 + n(m[3])
}

func normName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return strings.TrimSuffix(strings.TrimSuffix(b.String(), "trophies"), "trophy")
}

type psnTitle struct {
	Service    string `json:"npServiceName"`
	CommID     string `json:"npCommunicationId"`
	Name       string `json:"trophyTitleName"`
	Icon       string `json:"trophyTitleIconUrl"`
	Platform   string `json:"trophyTitlePlatform"`
	Progress   int    `json:"progress"`
	Defined    Grades `json:"definedTrophies"`
	Earned     Grades `json:"earnedTrophies"`
	LastUpdate string `json:"lastUpdatedDateTime"`
}

func (gr Grades) sum() int { return gr.Platinum + gr.Gold + gr.Silver + gr.Bronze }

func (g *games) fetchPSN() (*Platform, error) {
	tok, err := g.psnLogin()
	if err != nil {
		return nil, err
	}
	h := map[string]string{"Authorization": "Bearer " + tok}

	var sum struct {
		AccountID string `json:"accountId"`
		Level     int    `json:"trophyLevel"`
		Progress  int    `json:"progress"`
		Tier      int    `json:"tier"`
		Earned    Grades `json:"earnedTrophies"`
	}
	if err := g.getJSON(psnAPI+"trophy/v1/users/me/trophySummary", h, &sum); err != nil {
		return nil, err
	}
	p := &Platform{Account: g.cfg.PSN.User, Stats: map[string]interface{}{
		"level": sum.Level, "levelProgress": sum.Progress, "tier": sum.Tier, "grades": sum.Earned, "achievements": sum.Earned.sum(),
	}}
	var prof struct {
		OnlineID string `json:"onlineId"`
		Avatars  []struct {
			Size string `json:"size"`
			URL  string `json:"url"`
		} `json:"avatars"`
	}
	if g.getJSON(psnAPI+"userProfile/v1/internal/users/"+sum.AccountID+"/profiles", h, &prof) == nil {
		if prof.OnlineID != "" {
			p.Account = prof.OnlineID
		}
		for _, a := range prof.Avatars {
			p.Avatar = a.URL // o último é o maior
		}
	}
	p.ProfileURL = "https://psnprofiles.com/" + url.PathEscape(p.Account)

	var titles []psnTitle
	for off := 0; off < 5000; off += 800 {
		var page struct {
			Titles []psnTitle `json:"trophyTitles"`
			Total  int        `json:"totalItemCount"`
		}
		if err := g.getJSON(psnAPI+"trophy/v1/users/me/trophyTitles?limit=800&offset="+strconv.Itoa(off), h, &page); err != nil {
			return nil, err
		}
		titles = append(titles, page.Titles...)
		if len(page.Titles) < 800 || len(titles) >= page.Total {
			break
		}
	}

	// Tempo de jogo (só PS4/PS5): casado com os troféus pelo nome
	type played struct {
		Name     string `json:"name"`
		Image    string `json:"imageUrl"`
		Duration string `json:"playDuration"`
		Last     string `json:"lastPlayedDateTime"`
		Category string `json:"category"`
	}
	var plays []played
	for off := 0; off < 2000; off += 200 {
		var page struct {
			Titles []played `json:"titles"`
			Total  int      `json:"totalItemCount"`
		}
		if g.getJSON(psnAPI+"gamelist/v2/users/me/titles?categories=ps4_game,ps5_native_game&limit=200&offset="+strconv.Itoa(off), h, &page) != nil {
			break
		}
		plays = append(plays, page.Titles...)
		if len(page.Titles) < 200 || len(plays) >= page.Total {
			break
		}
	}
	byName := map[string]*played{}
	minutes := 0
	for i := range plays {
		minutes += isoMinutes(plays[i].Duration)
		byName[normName(plays[i].Name)] = &plays[i]
	}

	platinums := 0
	for _, t := range titles {
		gm := Game{ID: t.CommID, Name: t.Name, Image: t.Icon, System: strings.ReplaceAll(t.Platform, ",", " / "),
			Earned: t.Earned.sum(), Total: t.Defined.sum(), LastPlayed: t.LastUpdate, Grades: &Grades{}}
		*gm.Grades = t.Earned
		gm.Pct = float64(t.Progress)
		if t.Earned.Platinum > 0 {
			gm.Award = "platina"
			platinums++
		} else if t.Progress == 100 {
			gm.Award = "100%"
		}
		if pl := byName[normName(t.Name)]; pl != nil {
			gm.PlayMin = isoMinutes(pl.Duration)
			if pl.Last > gm.LastPlayed {
				gm.LastPlayed = pl.Last
			}
		}
		p.Games = append(p.Games, gm)
	}
	p.Stats["games"] = len(titles)
	p.Stats["perfect"] = platinums
	p.Stats["hours"] = minutes / 60

	p.Recent = g.psnRecent(h, titles)
	return p, nil
}

// Troféus recentes: detalhes dos 6 jogos com atividade mais recente (a lista vem ordenada por lastUpdatedDateTime)
func (g *games) psnRecent(h map[string]string, titles []psnTitle) []Unlock {
	if len(titles) > 6 {
		titles = titles[:6]
	}
	type trophy struct {
		ID     int    `json:"trophyId"`
		Earned bool   `json:"earned"`
		Date   string `json:"earnedDateTime"`
		Type   string `json:"trophyType"`
		Rate   string `json:"trophyEarnedRate"`
		Name   string `json:"trophyName"`
		Detail string `json:"trophyDetail"`
		Icon   string `json:"trophyIconUrl"`
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	var out []Unlock
	for _, t := range titles {
		wg.Add(1)
		go func(t psnTitle) {
			defer wg.Done()
			suffix := t.CommID + "/trophyGroups/all/trophies?npServiceName=" + t.Service
			var mine, defs struct {
				Trophies []trophy `json:"trophies"`
			}
			if g.getJSON(psnAPI+"trophy/v1/users/me/npCommunicationIds/"+suffix, h, &mine) != nil ||
				g.getJSON(psnAPI+"trophy/v1/npCommunicationIds/"+suffix, h, &defs) != nil {
				return
			}
			info := map[int]trophy{}
			for _, d := range defs.Trophies {
				info[d.ID] = d
			}
			mu.Lock()
			defer mu.Unlock()
			for _, x := range mine.Trophies {
				if !x.Earned {
					continue
				}
				d := info[x.ID]
				rate, _ := strconv.ParseFloat(x.Rate, 64)
				out = append(out, Unlock{Game: t.Name, Name: d.Name, Desc: d.Detail, Image: d.Icon, Date: x.Date, Kind: x.Type, Rarity: rate})
			}
		}(t)
	}
	wg.Wait()
	return sortRecent(out, 30)
}
