package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// hardwareCacheTTL: a GPU não muda com o hub ligado; a detecção roda de novo de tempos em tempos só para pegar driver novo.
const hardwareCacheTTL = 10 * time.Minute

// HardwareInfo é a resposta de GET /api/ai/hardware.
type HardwareInfo struct {
	GPUs        []GPU   `json:"gpus"`
	EffectiveMB int     `json:"effectiveMB"`
	Recommended []Model `json:"recommended"`
	All         []Model `json:"all"`
}

// Register registra as rotas da IA:
//
//	GET /api/ai/hardware  GPUs, memória efetiva e modelos sugeridos (detecção em cache por hardwareCacheTTL)
//	GET /api/ai/status    estado do provedor configurado (StatusInfo)
//
// cfg é lido a cada chamada, para valer o que a página Config acabou de salvar. A primeira detecção começa aqui, em segundo plano,
// sem segurar o início do hub.
func Register(mux *http.ServeMux, cfg func() Config) {
	register(mux, cfg, DetectGPUs, nil)
}

func register(mux *http.ServeMux, cfg func() Config, detect func(context.Context) []GPU, client *http.Client) {
	hw := &hwCache{detect: detect}
	hw.refresh()

	mux.HandleFunc("/api/ai/hardware", func(w http.ResponseWriter, r *http.Request) {
		if !onlyGET(w, r) {
			return
		}
		gpus := hw.get(r.Context())
		eff := EffectiveMB(gpus)
		rec, all := Recommend(eff)
		if rec == nil {
			rec = []Model{}
		}
		writeJSON(w, http.StatusOK, HardwareInfo{GPUs: gpus, EffectiveMB: eff, Recommended: rec, All: all})
	})
	mux.HandleFunc("/api/ai/status", func(w http.ResponseWriter, r *http.Request) {
		if !onlyGET(w, r) {
			return
		}
		c := cfg().WithDefaults()
		var gpus []GPU
		if c.Provider == ProviderOllama {
			gpus = hw.get(r.Context())
		}
		writeJSON(w, http.StatusOK, Status(r.Context(), c, client, gpus))
	})
}

func onlyGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método não permitido"})
	return false
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// hwCache guarda a última detecção. Vencida, quem pedir recebe a anterior enquanto outra roda em segundo plano; só a primeira
// chamada espera (até a detecção acabar ou a requisição desistir).
type hwCache struct {
	detect func(context.Context) []GPU

	mu      sync.Mutex
	gpus    []GPU
	at      time.Time
	pending chan struct{}
}

// refresh dispara a detecção se nenhuma estiver rodando e devolve o canal que fecha quando ela termina.
func (c *hwCache) refresh() chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending != nil {
		return c.pending
	}
	ch := make(chan struct{})
	c.pending = ch
	go func() {
		g := c.detect(context.Background()) // DetectGPUs já tem o próprio teto
		if g == nil {
			g = []GPU{}
		}
		c.mu.Lock()
		c.gpus, c.at, c.pending = g, time.Now(), nil
		c.mu.Unlock()
		close(ch)
	}()
	return ch
}

func (c *hwCache) get(ctx context.Context) []GPU {
	c.mu.Lock()
	gpus, at := c.gpus, c.at
	c.mu.Unlock()
	if !at.IsZero() {
		if time.Since(at) > hardwareCacheTTL {
			c.refresh()
		}
		return gpus
	}
	select {
	case <-c.refresh():
	case <-ctx.Done():
		return []GPU{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gpus
}
