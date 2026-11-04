package main

// Steam Web API (https://developer.valvesoftware.com/wiki/Steam_Web_API).
// O perfil e os "detalhes de jogos" precisam estar públicos na Steam para listar jogos e conquistas.

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"sync"
)

const steamAPI = "https://api.steampowered.com"

type steamAch struct {
	LastPlayed int64         `json:"lastPlayed"` // rtime_last_played quando foi consultado
	Total      int           `json:"total"`
	Unlocked   []steamUnlock `json:"unlocked,omitempty"`
}

type steamUnlock struct {
	Name string `json:"n"`
	Desc string `json:"d,omitempty"`
	Time int64  `json:"t"`
}

var steamID64 = regexp.MustCompile(`^\d{17}$`)

func (g *games) steamGet(path string, q url.Values, out interface{}) error {
	q.Set("key", g.cfg.Steam.Key)
	return g.getJSON(steamAPI+path+"?"+q.Encode(), nil, out)
}

func (g *games) fetchSteam() (*Platform, error) {
	id := g.cfg.Steam.User
	if !steamID64.MatchString(id) {
		var r struct {
			Response struct {
				Success int    `json:"success"`
				SteamID string `json:"steamid"`
			} `json:"response"`
		}
		if err := g.steamGet("/ISteamUser/ResolveVanityURL/v1/", url.Values{"vanityurl": {id}}, &r); err != nil {
			return nil, err
		}
		if r.Response.Success != 1 {
			return nil, fmt.Errorf("usuário Steam %q não encontrado (use a URL personalizada do perfil ou o SteamID64)", id)
		}
		id = r.Response.SteamID
	}

	var sum struct {
		Response struct {
			Players []struct {
				Name       string `json:"personaname"`
				Avatar     string `json:"avatarfull"`
				ProfileURL string `json:"profileurl"`
				Created    int64  `json:"timecreated"`
			} `json:"players"`
		} `json:"response"`
	}
	if err := g.steamGet("/ISteamUser/GetPlayerSummaries/v2/", url.Values{"steamids": {id}}, &sum); err != nil {
		return nil, err
	}
	p := &Platform{Account: g.cfg.Steam.User, Stats: map[string]interface{}{}}
	if len(sum.Response.Players) > 0 {
		pl := sum.Response.Players[0]
		p.Account, p.Avatar, p.ProfileURL = pl.Name, pl.Avatar, pl.ProfileURL
	}

	var lvl struct {
		Response struct {
			Level int `json:"player_level"`
		} `json:"response"`
	}
	if g.steamGet("/IPlayerService/GetSteamLevel/v1/", url.Values{"steamid": {id}}, &lvl) == nil {
		p.Stats["level"] = lvl.Response.Level
	}

	var owned struct {
		Response struct {
			Count int `json:"game_count"`
			Games []struct {
				AppID      int    `json:"appid"`
				Name       string `json:"name"`
				Playtime   int    `json:"playtime_forever"` // minutos
				LastPlayed int64  `json:"rtime_last_played"`
			} `json:"games"`
		} `json:"response"`
	}
	q := url.Values{"steamid": {id}, "include_appinfo": {"1"}, "include_played_free_games": {"1"}}
	if err := g.steamGet("/IPlayerService/GetOwnedGames/v1/", q, &owned); err != nil {
		return nil, err
	}
	if owned.Response.Count == 0 {
		return nil, fmt.Errorf("nenhum jogo visível: deixe \"Detalhes de jogos\" como Público na privacidade do perfil Steam")
	}

	// Conquistas: só de jogos já jogados, e só quando o jogo foi jogado depois da última consulta.
	type job struct {
		appid string
		last  int64
	}
	var jobs []job
	for _, gm := range owned.Response.Games {
		k := strconv.Itoa(gm.AppID)
		if c, ok := g.st.SteamAch[k]; gm.Playtime > 0 && (!ok || c.LastPlayed != gm.LastPlayed) {
			jobs = append(jobs, job{k, gm.LastPlayed})
		}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			a, err := g.steamAchievements(id, j.appid)
			if err != nil {
				return // tenta de novo na próxima atualização
			}
			a.LastPlayed = j.last
			mu.Lock()
			g.st.SteamAch[j.appid] = a
			mu.Unlock()
		}(j)
	}
	wg.Wait()

	var recent []Unlock
	earnedAll, played, perfect, minutes := 0, 0, 0, 0
	for _, gm := range owned.Response.Games {
		k := strconv.Itoa(gm.AppID)
		a := g.st.SteamAch[k]
		gg := Game{
			ID: k, Name: gm.Name, System: "PC",
			Image:   "https://cdn.cloudflare.steamstatic.com/steam/apps/" + k + "/header.jpg",
			PlayMin: gm.Playtime, LastPlayed: unixISO(gm.LastPlayed),
			Earned: len(a.Unlocked), Total: a.Total,
		}
		gg.Pct = pct(gg.Earned, gg.Total)
		if gg.Total > 0 && gg.Earned == gg.Total {
			gg.Award = "100%"
			perfect++
		}
		if gm.Playtime > 0 {
			played++
		}
		minutes += gm.Playtime
		earnedAll += gg.Earned
		for _, u := range a.Unlocked {
			recent = append(recent, Unlock{Game: gm.Name, Name: u.Name, Desc: u.Desc, Date: unixISO(u.Time), Image: gg.Image})
		}
		p.Games = append(p.Games, gg)
	}
	p.Recent = sortRecent(recent, 30)
	p.Stats["games"] = owned.Response.Count
	p.Stats["played"] = played
	p.Stats["achievements"] = earnedAll
	p.Stats["perfect"] = perfect
	p.Stats["hours"] = minutes / 60
	return p, nil
}

func (g *games) steamAchievements(steamid, appid string) (steamAch, error) {
	var r struct {
		PlayerStats struct {
			Success      bool `json:"success"`
			Achievements []struct {
				Achieved    int    `json:"achieved"`
				UnlockTime  int64  `json:"unlocktime"`
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"achievements"`
		} `json:"playerstats"`
	}
	err := g.steamGet("/ISteamUserStats/GetPlayerAchievements/v1/", url.Values{"steamid": {steamid}, "appid": {appid}, "l": {"brazilian"}}, &r)
	if e, ok := err.(*apiErr); ok && e.status == 400 {
		return steamAch{}, nil // "Requested app has no stats": jogo sem conquistas
	}
	if err != nil {
		return steamAch{}, err
	}
	a := steamAch{Total: len(r.PlayerStats.Achievements)}
	for _, x := range r.PlayerStats.Achievements {
		if x.Achieved == 1 {
			a.Unlocked = append(a.Unlocked, steamUnlock{x.Name, x.Description, x.UnlockTime})
		}
	}
	return a, nil
}
