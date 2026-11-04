package main

// RetroAchievements Web API (https://api-docs.retroachievements.org). Chave em retroachievements.org/settings.

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	raAPI   = "https://retroachievements.org/API/"
	raMedia = "https://media.retroachievements.org"
)

func (g *games) raGet(endpoint string, q url.Values, out interface{}) error {
	q.Set("y", g.cfg.Retro.Key)
	q.Set("u", g.cfg.Retro.User)
	return g.getJSON(raAPI+endpoint+"?"+q.Encode(), nil, out)
}

// datas do RA vêm como "2024-05-01 21:10:33" (UTC) ou ISO
func raDate(s string) string {
	if s == "" {
		return ""
	}
	for _, f := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05.000000Z"} {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func raImg(p string) string {
	if p == "" || strings.HasPrefix(p, "http") {
		return p
	}
	return raMedia + p
}

func (g *games) fetchRetro() (*Platform, error) {
	var sum struct {
		User        string      `json:"User"`
		UserPic     string      `json:"UserPic"`
		TotalPoints int         `json:"TotalPoints"`
		TotalTrue   int         `json:"TotalTruePoints"`
		Softcore    int         `json:"TotalSoftcorePoints"`
		Rank        interface{} `json:"Rank"`
		MemberSince string      `json:"MemberSince"`
	}
	if err := g.raGet("API_GetUserSummary.php", url.Values{"g": {"0"}, "a": {"0"}}, &sum); err != nil {
		return nil, err
	}
	if sum.User == "" {
		return nil, fmt.Errorf("usuário %q não encontrado ou chave inválida", g.cfg.Retro.User)
	}
	p := &Platform{Account: sum.User, Avatar: raImg(sum.UserPic), ProfileURL: "https://retroachievements.org/user/" + url.PathEscape(sum.User),
		Stats: map[string]interface{}{"points": sum.TotalPoints, "truePoints": sum.TotalTrue, "softcorePoints": sum.Softcore}}
	if r, err := strconv.Atoi(fmt.Sprint(sum.Rank)); err == nil && r > 0 {
		p.Stats["rank"] = r
	}

	type prog struct {
		GameID      int    `json:"GameID"`
		Title       string `json:"Title"`
		ImageIcon   string `json:"ImageIcon"`
		ConsoleName string `json:"ConsoleName"`
		Max         int    `json:"MaxPossible"`
		Awarded     int    `json:"NumAwarded"`
		AwardedHC   int    `json:"NumAwardedHardcore"`
		LastDate    string `json:"MostRecentAwardedDate"`
		Award       string `json:"HighestAwardKind"`
	}
	var all []prog
	for off := 0; off < 5000; off += 500 {
		var page struct {
			Total   int    `json:"Total"`
			Results []prog `json:"Results"`
		}
		if err := g.raGet("API_GetUserCompletionProgress.php", url.Values{"c": {"500"}, "o": {strconv.Itoa(off)}}, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Results...)
		if len(page.Results) < 500 || len(all) >= page.Total {
			break
		}
	}

	earned, mastered, beaten := 0, 0, 0
	for _, x := range all {
		n := x.Awarded
		if x.AwardedHC > n {
			n = x.AwardedHC
		}
		gm := Game{ID: strconv.Itoa(x.GameID), Name: x.Title, Image: raImg(x.ImageIcon), System: x.ConsoleName,
			Earned: n, Total: x.Max, Pct: pct(n, x.Max), LastPlayed: raDate(x.LastDate)}
		switch x.Award {
		case "mastered":
			gm.Award = "masterizado"
			mastered++
		case "completed":
			gm.Award = "100%"
			mastered++
		case "beaten-hardcore", "beaten-softcore":
			gm.Award = "zerado"
			beaten++
		}
		earned += n
		p.Games = append(p.Games, gm)
	}
	p.Stats["games"] = len(all)
	p.Stats["achievements"] = earned
	p.Stats["perfect"] = mastered
	p.Stats["beaten"] = beaten

	var rec []struct {
		Date      string `json:"Date"`
		Title     string `json:"Title"`
		Desc      string `json:"Description"`
		Points    int    `json:"Points"`
		BadgeURL  string `json:"BadgeURL"`
		GameTitle string `json:"GameTitle"`
	}
	if err := g.raGet("API_GetUserRecentAchievements.php", url.Values{"m": {strconv.Itoa(90 * 24 * 60)}}, &rec); err == nil {
		for _, r := range rec {
			p.Recent = append(p.Recent, Unlock{Game: r.GameTitle, Name: r.Title, Desc: r.Desc, Image: raImg(r.BadgeURL), Date: raDate(r.Date), Points: r.Points})
		}
	}
	p.Recent = sortRecent(p.Recent, 30)
	return p, nil
}
