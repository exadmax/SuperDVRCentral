package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeLLM sobe um servidor OpenAI-compatível que devolve status e corpo fixos e guarda o último pedido.
func fakeLLM(t *testing.T, status int, reply string) (*httptest.Server, *http.Request, *map[string]any) {
	t.Helper()
	got := &http.Request{}
	body := &map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = *r.Clone(context.Background())
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	return srv, got, body
}

func TestCompleteTexto(t *testing.T) {
	srv, got, body := fakeLLM(t, 200, `{"choices":[{"message":{"role":"assistant","content":"  Às ordens, Comandante. "}}]}`)
	cfg := Config{Provider: "ollama", OllamaURL: srv.URL, Model: "qwen3:8b"}
	rep, err := Complete(context.Background(), cfg, nil, ChatRequest{Messages: []Message{{Role: "user", Content: "oi"}}, Temperature: 0.3})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Content != "Às ordens, Comandante." || len(rep.ToolCalls) != 0 {
		t.Fatalf("resposta: %+v", rep)
	}
	if got.URL.Path != "/v1/chat/completions" || got.Header.Get("Authorization") != "" {
		t.Fatalf("pedido: %s auth=%q", got.URL.Path, got.Header.Get("Authorization"))
	}
	if (*body)["model"] != "qwen3:8b" {
		t.Fatalf("modelo: %v", (*body)["model"])
	}
	if _, ok := (*body)["tools"]; ok {
		t.Fatal("sem ferramentas não deve mandar tools")
	}
}

func TestCompleteToolCall(t *testing.T) {
	srv, _, body := fakeLLM(t, 200, `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[
		{"id":"call_1","type":"function","function":{"name":"show_camera","arguments":"{\"camera_id\":\"sala\"}"}}]}}]}`)
	cfg := Config{Provider: "ollama", OllamaURL: srv.URL, Model: "qwen3:8b"}
	tools := []Tool{{Name: "show_camera", Description: "mostra", Parameters: json.RawMessage(`{"type":"object","properties":{"camera_id":{"type":"string","enum":["sala"]}},"required":["camera_id"]}`)}}
	rep, err := Complete(context.Background(), cfg, nil, ChatRequest{Messages: []Message{{Role: "user", Content: "sala"}}, Tools: tools})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Content != "" || len(rep.ToolCalls) != 1 || rep.ToolCalls[0] != (ToolCall{ID: "call_1", Name: "show_camera", Arguments: `{"camera_id":"sala"}`}) {
		t.Fatalf("resposta: %+v", rep)
	}
	if (*body)["tool_choice"] != "auto" {
		t.Fatalf("tool_choice: %v", (*body)["tool_choice"])
	}
	ts, _ := (*body)["tools"].([]any)
	if len(ts) != 1 {
		t.Fatalf("tools: %v", (*body)["tools"])
	}
	fn := ts[0].(map[string]any)["function"].(map[string]any)
	if ts[0].(map[string]any)["type"] != "function" || fn["name"] != "show_camera" || fn["parameters"] == nil {
		t.Fatalf("ferramenta: %v", ts[0])
	}
}

func TestCompleteGroq401(t *testing.T) {
	srv, got, body := fakeLLM(t, 401, `{"error":{"message":"Invalid API Key gsk_SEGREDO","type":"invalid_request_error"}}`)
	old := groqBaseURL
	groqBaseURL = srv.URL
	defer func() { groqBaseURL = old }()
	_, err := Complete(context.Background(), Config{Provider: "groq", GroqKey: "gsk_SEGREDO"}, nil, ChatRequest{Messages: []Message{{Role: "user", Content: "oi"}}})
	if err == nil {
		t.Fatal("esperava erro")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Invalid API Key") || strings.Contains(err.Error(), "gsk_SEGREDO") {
		t.Fatalf("erro: %v", err)
	}
	if got.Header.Get("Authorization") != "Bearer gsk_SEGREDO" || got.URL.Path != "/chat/completions" {
		t.Fatalf("pedido: %s %q", got.URL.Path, got.Header.Get("Authorization"))
	}
	if (*body)["model"] != DefaultGroqModel {
		t.Fatalf("modelo padrão: %v", (*body)["model"])
	}
}

func TestCompleteSemProvedor(t *testing.T) {
	_, err := Complete(context.Background(), Config{Provider: "off"}, nil, ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), "desligada") {
		t.Fatalf("off: %v", err)
	}
	_, err = Complete(context.Background(), Config{Provider: "groq"}, nil, ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), "chave da Groq") {
		t.Fatalf("groq sem chave: %v", err)
	}
}

func TestCompleteOllamaModeloPelaGPU(t *testing.T) {
	srv, _, body := fakeLLM(t, 200, `{"choices":[{"message":{"content":"ok"}}]}`)
	llmHW = &hwCache{detect: func(context.Context) []GPU { return nil }} // sem GPU: faixa de CPU
	if _, err := Complete(context.Background(), Config{Provider: "ollama", OllamaURL: srv.URL}, nil, ChatRequest{}); err != nil {
		t.Fatal(err)
	}
	rec, _ := Recommend(0)
	if (*body)["model"] != rec[0].Name {
		t.Fatalf("modelo: %v, esperava %s", (*body)["model"], rec[0].Name)
	}
}
