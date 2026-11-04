package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Um trecho do dia: de "from" (padrão: casa) até "to". "after" = a partir de que horas vale (HH:MM).
// "home": true = não há deslocamento (home office).
type Leg struct {
	Name  string `json:"name"`
	To    string `json:"to"`
	From  string `json:"from"`
	After string `json:"after"`
	Home  bool   `json:"home"`
}

type Person struct {
	ID        string           `json:"id,omitempty"`    // identificador (diogo, cassia...); usado na API
	Label     string           `json:"label,omitempty"` // nome exibido no painel
	ArriveBy  string           `json:"arriveBy"`
	OnDemand  bool             `json:"onDemand"`  // só consulta quando o painel pede (clique em "Ver trânsito")
	Window    []string         `json:"window"`    // ["06:00","08:00"]: única faixa em que consulta a API
	ShowUntil string           `json:"showUntil"` // depois disso o card some ("Só pela manhã")
	Days      map[string][]Leg `json:"days"`      // "0" domingo ... "6" sábado
}

type route struct {
	Min      int          `json:"min"`
	Normal   int          `json:"normal"`
	Delay    int          `json:"delay"`
	Level    string       `json:"level"`
	Points   [][2]float64 `json:"points"`
	ArriveBy string       `json:"arriveBy"`
	Name     string       `json:"name"`
	OnDemand bool         `json:"onDemand,omitempty"` // o painel só mostra depois de "Ver trânsito"
}

type cached struct {
	r  *route
	at time.Time
}

type traffic struct {
	mu     sync.Mutex
	cfg    Config
	client *http.Client
	cache  map[string]*cached
}

func newTraffic(cfg Config) *traffic {
	cfg.Traffic.normalize()
	tr := &http.Transport{}
	if cfg.ForceIPv4 { // chaves com restrição de IP costumam liberar só o IPv4 público
		d := &net.Dialer{Timeout: 10 * time.Second}
		tr.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) { return d.DialContext(ctx, "tcp4", addr) }
	}
	return &traffic{cfg: cfg, client: &http.Client{Timeout: 20 * time.Second, Transport: tr}, cache: map[string]*cached{}}
}

func hm(s string) int {
	var h, m int
	fmt.Sscanf(s, "%d:%d", &h, &m)
	return h*60 + m
}

func level(min, normal int) (int, string) {
	delay := min - normal
	ratio := 0.0
	if normal > 0 {
		ratio = float64(delay) / float64(normal)
	}
	switch {
	case ratio < 0.1:
		return delay, "livre"
	case ratio < 0.3:
		return delay, "moderado"
	}
	return delay, "congestionado"
}

// reduz para ~100 pontos: o mini-mapa não precisa de mais
func sample(pts [][2]float64) [][2]float64 {
	if len(pts) == 0 {
		return pts
	}
	step := len(pts)/100 + 1
	var out [][2]float64
	for i := 0; i < len(pts); i += step {
		out = append(out, pts[i])
	}
	return append(out, pts[len(pts)-1])
}

func secs(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "s"), 64)
	return v
}

// polyline codificada do Google (precisão 1e5)
func decodePolyline(s string) [][2]float64 {
	var out [][2]float64
	idx, lat, lon := 0, 0, 0
	next := func() int {
		shift, result := 0, 0
		for idx < len(s) {
			b := int(s[idx]) - 63
			idx++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			return ^(result >> 1)
		}
		return result >> 1
	}
	for idx < len(s) {
		lat += next()
		lon += next()
		out = append(out, [2]float64{float64(lat) / 1e5, float64(lon) / 1e5})
	}
	return out
}

// Google Routes API: origem e destino por endereço, tempo com trânsito atual.
func (t *traffic) fetch(from, to string) (*route, error) {
	wp := func(a string) map[string]interface{} { return map[string]interface{}{"address": a} }
	body, _ := json.Marshal(map[string]interface{}{
		"origin": wp(from), "destination": wp(to), "travelMode": "DRIVE",
		"routingPreference": "TRAFFIC_AWARE", "languageCode": "pt-BR", "regionCode": "BR",
	})
	req, _ := http.NewRequest("POST", "https://routes.googleapis.com/directions/v2:computeRoutes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", t.cfg.Traffic.Key)
	req.Header.Set("X-Goog-FieldMask", "routes.duration,routes.staticDuration,routes.polyline.encodedPolyline")
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var j struct {
		Routes []struct {
			Duration       string `json:"duration"`
			StaticDuration string `json:"staticDuration"`
			Polyline       struct {
				Encoded string `json:"encodedPolyline"`
			} `json:"polyline"`
		} `json:"routes"`
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&j)
	if resp.StatusCode != 200 || len(j.Routes) == 0 {
		return nil, fmt.Errorf("Google Routes HTTP %d %s: %s", resp.StatusCode, j.Error.Status, j.Error.Message)
	}
	rt := j.Routes[0]
	r := &route{Min: int(secs(rt.Duration)/60 + 0.5), Normal: int(secs(rt.StaticDuration)/60 + 0.5)}
	r.Delay, r.Level = level(r.Min, r.Normal)
	r.Points = sample(decodePolyline(rt.Polyline.Encoded))
	return r, nil
}

// Trecho que vale agora para a pessoa (o último cujo "after" já passou).
func legNow(p Person, now time.Time) *Leg {
	var cur *Leg
	legs := p.Days[strconv.Itoa(int(now.Weekday()))]
	for i := range legs {
		if legs[i].After == "" || hm(legs[i].After) <= now.Hour()*60+now.Minute() {
			cur = &legs[i]
		}
	}
	return cur
}

func minutes(t time.Time) int { return t.Hour()*60 + t.Minute() }

func off(reason string) map[string]interface{} {
	return map[string]interface{}{"off": true, "reason": reason}
}

// Chamado com t.mu travado. "asked" = o painel pediu esta pessoa agora (só vale para onDemand).
func (t *traffic) forPerson(p Person, now time.Time, asked bool, stale *bool) interface{} {
	leg := legNow(p, now)
	if leg == nil {
		return off("Sem deslocamento hoje")
	}
	if leg.Home {
		return off("Home office hoje")
	}
	ttl, cacheOnly := time.Duration(t.cfg.TrafficCacheMinutes)*time.Minute, false
	switch {
	case p.OnDemand && !asked:
		return map[string]interface{}{"manual": true, "name": leg.Name}
	case p.OnDemand:
		ttl = 5 * time.Minute // cliques repetidos não gastam cota
	case len(p.Window) == 2:
		m := minutes(now)
		if m < hm(p.Window[0]) {
			return off("Disponível às " + p.Window[0])
		}
		if p.ShowUntil != "" && m >= hm(p.ShowUntil) {
			return off("Só pela manhã")
		}
		cacheOnly = m >= hm(p.Window[1]) // depois da janela: mostra o último resultado, sem nova chamada
	}
	from := leg.From
	if from == "" {
		from = t.cfg.Traffic.Home
	}
	key := from + "|" + leg.To
	c := t.cache[key]
	if !cacheOnly && (c == nil || now.Sub(c.at) > ttl) {
		r, err := t.fetch(from, leg.To)
		if err != nil {
			log.Printf("trânsito %s: %v", leg.Name, err)
			*stale = true
		} else {
			c = &cached{r: r, at: now}
			t.cache[key] = c
		}
	}
	if c == nil {
		return off("Sem dados")
	}
	out := *c.r
	out.ArriveBy, out.Name, out.OnDemand = p.ArriveBy, leg.Name, p.OnDemand
	return &out
}

// normalize leva o formato antigo (campos diogo e cassia) para a lista People e dá id/nome a quem não tiver.
func (tc *TrafficConfig) normalize() {
	if len(tc.People) == 0 {
		if len(tc.Diogo.Days) > 0 {
			d := tc.Diogo
			d.ID, d.Label = "diogo", "Diogo"
			tc.People = append(tc.People, d)
		}
		if len(tc.Cassia.Days) > 0 {
			c := tc.Cassia
			c.ID, c.Label = "cassia", "Cássia"
			tc.People = append(tc.People, c)
		}
	}
	tc.Diogo, tc.Cassia = Person{}, Person{}
	for i := range tc.People {
		if tc.People[i].ID == "" {
			tc.People[i].ID = slug(tc.People[i].Label)
		}
		if tc.People[i].Label == "" {
			tc.People[i].Label = tc.People[i].ID
		}
	}
}

// ask: ids das pessoas "só quando eu pedir" que o painel quer consultar agora.
func (t *traffic) get(ask map[string]bool) map[string]interface{} {
	tc := t.config()
	if tc.Key == "" || tc.Home == "" {
		return map[string]interface{}{"configured": false}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now, stale := time.Now(), false
	people := make([]map[string]interface{}, 0, len(tc.People))
	out := map[string]interface{}{"configured": true}
	for _, p := range tc.People {
		r := t.forPerson(p, now, ask[p.ID], &stale)
		people = append(people, map[string]interface{}{"id": p.ID, "label": p.Label, "onDemand": p.OnDemand, "result": r})
		out[p.ID] = r // formato antigo (diogo, cassia): painéis ainda não atualizados continuam funcionando
	}
	out["people"], out["stale"] = people, stale
	return out
}

// setConfig troca as rotas e a chave em uso (página Config) e descarta o cache de rotas já consultadas.
func (t *traffic) setConfig(tc TrafficConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tc.normalize()
	t.cfg.Traffic = tc
	t.cache = map[string]*cached{}
}

func (t *traffic) config() TrafficConfig {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cfg.Traffic
}
