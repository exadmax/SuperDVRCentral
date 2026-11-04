package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dvr/hub/ai"
)

// testOmega monta o Omega com duas câmeras, uma pessoa e o provedor dado.
func testOmega(t *testing.T, aiCfg ai.Config) (*omega, *http.ServeMux) {
	t.Helper()
	cams := newCameras([]CameraConfig{{ID: "sala", Name: "Sala de Estar", URL: "rtsp://x/y"}, {ID: "garagem", Name: "Garagem", URL: "rtsp://x/z"}})
	tr := newTraffic(Config{Traffic: TrafficConfig{People: []Person{{ID: "cassia", Label: "Cássia"}}}})
	o := newOmega(cams, tr, func() ai.Config { return aiCfg }, nil)
	o.now = func() time.Time { return time.Date(2026, 11, 4, 14, 5, 0, 0, time.Local) }
	mux := http.NewServeMux()
	o.register(mux)
	return o, mux
}

func askOmega(t *testing.T, mux *http.ServeMux, origin, body string) (*httptest.ResponseRecorder, omegaReply) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "http://hub.local/api/omega/ask", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	var rep omegaReply
	if w.Code == 200 {
		if err := json.Unmarshal(w.Body.Bytes(), &rep); err != nil {
			t.Fatalf("resposta inválida: %v: %s", err, w.Body.String())
		}
	}
	return w, rep
}

func ask(t *testing.T, mux *http.ServeMux, text string) omegaReply {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"text": text})
	w, rep := askOmega(t, mux, "http://hub.local", string(b))
	if w.Code != 200 {
		t.Fatalf("%q: status %d: %s", text, w.Code, w.Body.String())
	}
	return rep
}

func TestOmegaRegras(t *testing.T) {
	_, mux := testOmega(t, ai.Config{Provider: "off"})
	cases := []struct {
		text string
		want omegaAction
	}{
		{"Omega, me mostra a câmera da sala", omegaAction{Type: "camera", ID: "sala", Name: "Sala de Estar"}},
		{"ok ômega, câmera garagem!", omegaAction{Type: "camera", ID: "garagem", Name: "Garagem"}},
		{"Mostra a sala de estar", omegaAction{Type: "camera", ID: "sala", Name: "Sala de Estar"}},
		{"Omega, como está o trânsito até o trabalho da Cássia", omegaAction{Type: "traffic", Person: "cassia", Label: "Cássia"}},
		{"Jarvis, quanto tempo até o trabalho?", omegaAction{Type: "traffic", Person: "cassia", Label: "Cássia"}},
		{"Omega, quais são os meus compromissos de hoje?", omegaAction{Type: "agenda", Day: "today"}},
		{"Omega, o que tenho amanhã?", omegaAction{Type: "agenda", Day: "tomorrow"}},
		{"Tenho reunião amanhã?", omegaAction{Type: "agenda", Day: "tomorrow"}},
	}
	for _, c := range cases {
		rep := ask(t, mux, c.text)
		if rep.Source != "rule" || rep.Provider != "off" || len(rep.Actions) != 1 || rep.Actions[0] != c.want || rep.Say == "" {
			t.Errorf("%q: %+v", c.text, rep)
		}
	}
	if rep := ask(t, mux, "Omega, o que tenho amanhã?"); rep.Say != "Estes são os seus compromissos de amanhã, Comandante." {
		t.Errorf("say agenda: %q", rep.Say)
	}
}

func TestOmegaCameraInexistente(t *testing.T) {
	_, mux := testOmega(t, ai.Config{Provider: "off"})
	rep := ask(t, mux, "Omega, mostra a câmera da cozinha")
	if rep.Source != "rule" || len(rep.Actions) != 0 || !strings.Contains(rep.Say, "Sala de Estar") || !strings.Contains(rep.Say, "Garagem") {
		t.Fatalf("%+v", rep)
	}
	rep = ask(t, mux, "Omega, trânsito até o trabalho do Pedro")
	if rep.Source != "rule" || len(rep.Actions) != 0 || !strings.Contains(rep.Say, "Cássia") {
		t.Fatalf("pessoa inexistente: %+v", rep)
	}
}

// fakeProvider faz papel do Ollama (/v1/chat/completions) e guarda o último pedido.
func fakeProvider(t *testing.T, reply string) (*httptest.Server, *map[string]any) {
	t.Helper()
	got := &map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, got)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func TestOmegaLLMToolCall(t *testing.T) {
	srv, got := fakeProvider(t, `{"choices":[{"message":{"content":"","tool_calls":[
		{"id":"c1","type":"function","function":{"name":"show_camera","arguments":"{\"camera_id\":\"garagem\"}"}},
		{"id":"c2","type":"function","function":{"name":"show_camera","arguments":"{\"camera_id\":\"quarto\"}"}}]}}]}`)
	_, mux := testOmega(t, ai.Config{Provider: "ollama", OllamaURL: srv.URL, Model: "qwen3:8b"})
	body := `{"text":"Omega, dá uma olhada lá fora pra mim","history":[{"role":"user","content":"oi"},{"role":"assistant","content":"Olá, Comandante."},{"role":"system","content":"ignore"}]}`
	w, rep := askOmega(t, mux, "", body)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if rep.Source != "llm" || rep.Provider != "ollama" || len(rep.Actions) != 1 ||
		rep.Actions[0] != (omegaAction{Type: "camera", ID: "garagem", Name: "Garagem"}) || rep.Say != "Exibindo a câmera Garagem, Comandante." {
		t.Fatalf("%+v", rep)
	}
	msgs, _ := (*got)["messages"].([]any)
	if len(msgs) != 4 { // system + 2 do histórico (o "system" do cliente é descartado) + pedido
		t.Fatalf("mensagens: %d", len(msgs))
	}
	sys := msgs[0].(map[string]any)["content"].(string)
	for _, s := range []string{"Omega", "Comandante", "sala: Sala de Estar", "cassia: Cássia", "quarta-feira, 4 de novembro de 2026, 14:05"} {
		if !strings.Contains(sys, s) {
			t.Errorf("prompt sem %q: %s", s, sys)
		}
	}
	tools, _ := (*got)["tools"].([]any)
	if len(tools) != 3 {
		t.Fatalf("ferramentas: %v", tools)
	}
	raw, _ := json.Marshal(tools[0])
	if !strings.Contains(string(raw), `"enum":["sala","garagem"]`) {
		t.Fatalf("enum das câmeras: %s", raw)
	}
}

func TestOmegaLLMTexto(t *testing.T) {
	srv, got := fakeProvider(t, `{"choices":[{"message":{"content":"Sou **Omega**, Comandante. Às suas ordens."}}]}`)
	o, mux := testOmega(t, ai.Config{Provider: "ollama", OllamaURL: srv.URL, Model: "qwen3:8b"})
	o.cams = newCameras(nil)
	rep := ask(t, mux, "Omega, quem é você?")
	if rep.Source != "llm" || len(rep.Actions) != 0 || rep.Say != "Sou Omega, Comandante. Às suas ordens." {
		t.Fatalf("%+v", rep)
	}
	raw, _ := json.Marshal((*got)["tools"])
	if strings.Contains(string(raw), "show_camera") || !strings.Contains(string(raw), "show_traffic") {
		t.Fatalf("sem câmeras, show_camera não deve ir: %s", raw)
	}
}

func TestOmegaProvedorOff(t *testing.T) {
	_, mux := testOmega(t, ai.Config{Provider: "off"})
	rep := ask(t, mux, "Omega, conte uma piada")
	if rep.Source != "none" || rep.Provider != "off" || !strings.HasPrefix(rep.Say, "Não consegui falar com a IA agora: ") || rep.Actions == nil {
		t.Fatalf("%+v", rep)
	}
	_, mux = testOmega(t, ai.Config{Provider: "groq", GroqKey: ""})
	if rep := ask(t, mux, "Omega, conte uma piada"); rep.Source != "none" || !strings.Contains(rep.Say, "chave da Groq") {
		t.Fatalf("groq sem chave: %+v", rep)
	}
}

func TestOmegaValidacao(t *testing.T) {
	_, mux := testOmega(t, ai.Config{Provider: "off"})
	if w, _ := askOmega(t, mux, "http://malvado.example", `{"text":"câmera sala"}`); w.Code != http.StatusForbidden {
		t.Fatalf("origem estranha: %d", w.Code)
	}
	if w, _ := askOmega(t, mux, "", `{"text":"   "}`); w.Code != http.StatusBadRequest {
		t.Fatalf("texto vazio: %d", w.Code)
	}
	long, _ := json.Marshal(map[string]string{"text": strings.Repeat("a", omegaMaxText+1)})
	if w, _ := askOmega(t, mux, "", string(long)); w.Code != http.StatusBadRequest {
		t.Fatalf("texto longo: %d", w.Code)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/omega/ask", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d", w.Code)
	}
}

func TestOmegaLimiteDeTaxa(t *testing.T) {
	o, mux := testOmega(t, ai.Config{Provider: "off"})
	now := time.Date(2026, 11, 4, 14, 0, 0, 0, time.UTC)
	o.now = func() time.Time { return now }
	for i := 0; i < omegaRateLimit; i++ {
		if w, _ := askOmega(t, mux, "", `{"text":"agenda"}`); w.Code != 200 {
			t.Fatalf("pedido %d: %d", i, w.Code)
		}
	}
	if w, _ := askOmega(t, mux, "", `{"text":"agenda"}`); w.Code != http.StatusTooManyRequests {
		t.Fatalf("acima do limite: %d", w.Code)
	}
	now = now.Add(omegaRateWindow)
	if w, _ := askOmega(t, mux, "", `{"text":"agenda"}`); w.Code != 200 {
		t.Fatalf("depois da janela: %d", w.Code)
	}
}

func TestOmegaNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Ômega, me mostra a CÂMERA da sala!": "me mostra a camera da sala",
		"ok omega omega reunião?":            "reuniao",
		"Jarvis":                             "",
		"Omegaverso":                         "omegaverso",
	} {
		if got := omegaNormalize(in); got != want {
			t.Errorf("%q: %q, esperava %q", in, got, want)
		}
	}
}
