package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type calSource struct {
	Who   string `json:"who"`
	Label string `json:"label"`
	Color string `json:"color"`
	Text  string `json:"text"`
	Stale bool   `json:"stale"`
}

type calendars struct {
	mu      sync.Mutex
	cfg     Config
	ttl     time.Duration
	client  *http.Client
	fetched time.Time
	data    []calSource
}

func newCalendars(cfg Config) *calendars {
	return &calendars{cfg: cfg, ttl: time.Duration(cfg.CalendarCacheMinutes) * time.Minute, client: &http.Client{Timeout: 20 * time.Second}}
}

func (c *calendars) fetch(url string) (string, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if !strings.Contains(string(b[:min(len(b), 200)]), "BEGIN:VCALENDAR") {
		return "", fmt.Errorf("resposta não é iCal")
	}
	return string(b), nil
}

// Devolve o iCal bruto de cada agenda (o painel faz o parse). Em caso de erro, mantém o último bom.
func (c *calendars) get() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil || time.Since(c.fetched) > c.ttl {
		next := make([]calSource, 0, len(c.cfg.Calendars))
		for _, s := range c.cfg.Calendars {
			text, err := c.fetch(s.URL)
			if err != nil {
				log.Printf("agenda %q: %v", s.Who, err)
				for _, old := range c.data { // mantém a última versão boa desta agenda
					if old.Who == s.Who {
						old.Stale = true
						next = append(next, old)
					}
				}
				continue
			}
			next = append(next, calSource{Who: s.Who, Label: s.Label, Color: s.Color, Text: text})
		}
		c.data, c.fetched = next, time.Now()
	}
	return map[string]interface{}{"sources": c.data}
}

// setSources troca as agendas em uso (página Config) e descarta o cache para a próxima leitura já trazer a lista nova.
func (c *calendars) setSources(list []CalendarConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.Calendars = list
	c.data = nil
}

func (c *calendars) sources() []CalendarConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]CalendarConfig(nil), c.cfg.Calendars...)
}
