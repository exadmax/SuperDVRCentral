package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRoutesHardware(t *testing.T) {
	var calls atomic.Int32
	detect := func(context.Context) []GPU {
		calls.Add(1)
		return irisXeGPUs
	}
	mux := http.NewServeMux()
	register(mux, func() Config { return Config{Provider: ProviderOff} }, detect, nil)

	for range 3 {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ai/hardware", nil))
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("code %d %s", rec.Code, rec.Body)
		}
		var hw HardwareInfo
		if err := json.Unmarshal(rec.Body.Bytes(), &hw); err != nil {
			t.Fatal(err)
		}
		if len(hw.GPUs) != 1 || hw.EffectiveMB != 6075 || len(hw.Recommended) == 0 || hw.Recommended[0].Name != "qwen3:4b" || len(hw.All) != len(catalog) {
			t.Fatalf("hw = %+v", hw)
		}
		for _, k := range []string{`"gpus"`, `"effectiveMB"`, `"recommended"`, `"all"`, `"dedicatedMB"`, `"sharedMB"`, `"minMB"`} {
			if !strings.Contains(rec.Body.String(), k) {
				t.Errorf("falta %s em %s", k, rec.Body)
			}
		}
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("detecção rodou %d vezes (devia ficar em cache)", n)
	}
}

func TestRoutesHardwareNoGPU(t *testing.T) {
	mux := http.NewServeMux()
	register(mux, func() Config { return Config{} }, func(context.Context) []GPU { return nil }, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ai/hardware", nil))
	if !strings.Contains(rec.Body.String(), `"gpus":[]`) || !strings.Contains(rec.Body.String(), `"effectiveMB":0`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestRoutesStatus(t *testing.T) {
	srv := fakeOllama(t, twoModels)
	cfg := Config{OllamaURL: srv.URL}
	mux := http.NewServeMux()
	register(mux, func() Config { return cfg }, func(context.Context) []GPU { return irisXeGPUs }, srv.Client())

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ai/status", nil))
	var st StatusInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("code %d %s %v", rec.Code, rec.Body, err)
	}
	if st.Provider != ProviderOllama || st.Ollama == nil || !st.Ollama.Reachable || len(st.Ollama.Models) != 2 || st.Ollama.EffectiveMB != 6075 {
		t.Fatalf("st = %s", rec.Body)
	}

	// A configuração é relida a cada chamada.
	cfg = Config{Provider: ProviderGroq, GroqKey: "gsk_x"}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ai/status", nil))
	if !strings.Contains(rec.Body.String(), `"configured":true`) || strings.Contains(rec.Body.String(), "gsk_x") {
		t.Fatalf("groq = %s", rec.Body)
	}
}

func TestRoutesMethod(t *testing.T) {
	mux := http.NewServeMux()
	register(mux, func() Config { return Config{} }, func(context.Context) []GPU { return nil }, nil)
	for _, p := range []string{"/api/ai/hardware", "/api/ai/status"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, p, nil))
		var e map[string]string
		if rec.Code != http.StatusMethodNotAllowed || json.Unmarshal(rec.Body.Bytes(), &e) != nil || e["error"] == "" {
			t.Fatalf("%s: %d %s", p, rec.Code, rec.Body)
		}
	}
}
