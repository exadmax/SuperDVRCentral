package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeOllama responde /api/version e /api/tags como o Ollama 0.40.
func fakeOllama(t *testing.T, tags string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			w.Write([]byte(`{"version":"0.40.14"}`))
		case "/api/tags":
			w.Write([]byte(tags))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

const twoModels = `{"models":[
 {"name":"qwen3:4b","size":2620788019,"details":{"parameter_size":"4.0B"}},
 {"name":"gpt-oss:20b","size":13780173839,"details":{"parameter_size":"20.9B"}}]}`

var irisXeGPUs = []GPU{{Name: "Intel(R) Iris(R) Xe Graphics", Vendor: VendorIntel, Integrated: true, SharedMB: 12150}}

func TestStatusOllamaUp(t *testing.T) {
	srv := fakeOllama(t, twoModels)
	st := Status(context.Background(), Config{OllamaURL: srv.URL + "/", Model: "qwen3:4b"}, srv.Client(), irisXeGPUs)
	if st.Provider != ProviderOllama || st.Ollama == nil || st.Groq != nil {
		t.Fatalf("st = %+v", st)
	}
	o := st.Ollama
	if !o.Reachable || o.Version != "0.40.14" || o.URL != srv.URL || o.EffectiveMB != 6075 || !o.ModelInstalled {
		t.Fatalf("ollama = %+v", o)
	}
	if len(o.Models) != 2 {
		t.Fatalf("models = %+v", o.Models)
	}
	q, g := o.Models[0], o.Models[1]
	if q.Name != "qwen3:4b" || !q.Known || !q.Fits || q.ParameterSize != "4.0B" || q.Size != 2620788019 {
		t.Errorf("qwen3:4b = %+v", q)
	}
	if g.Name != "gpt-oss:20b" || !g.Known || g.Fits {
		t.Errorf("gpt-oss:20b = %+v", g)
	}
	if st.Message != "" {
		t.Errorf("message = %q", st.Message)
	}
}

func TestStatusOllamaNoModels(t *testing.T) {
	srv := fakeOllama(t, `{"models":[]}`)
	st := Status(context.Background(), Config{OllamaURL: srv.URL}, srv.Client(), irisXeGPUs)
	if !st.Ollama.Reachable || len(st.Ollama.Models) != 0 || !strings.Contains(st.Message, "ollama pull qwen3:4b") {
		t.Fatalf("st = %+v / %+v", st, st.Ollama)
	}
	// models sai como [] (não null) para a interface.
	b, _ := json.Marshal(st)
	if !strings.Contains(string(b), `"models":[]`) {
		t.Fatalf("json = %s", b)
	}

	st = Status(context.Background(), Config{OllamaURL: srv.URL, Model: "qwen3:8b"}, srv.Client(), irisXeGPUs)
	if st.Ollama.ModelInstalled || !strings.Contains(st.Message, `"ollama pull qwen3:8b"`) {
		t.Fatalf("modelo ausente: %+v", st)
	}
}

func TestStatusOllamaDown(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // porta fechada: conexão recusada
	st := Status(context.Background(), Config{Provider: "OLLAMA", OllamaURL: url}, nil, nil)
	if st.Ollama == nil || st.Ollama.Reachable {
		t.Fatalf("st = %+v", st)
	}
	want := "Ollama não encontrado em " + url + ": instale em https://ollama.com/download ou confira o endereço"
	if st.Message != want {
		t.Fatalf("message = %q", st.Message)
	}
}

func TestStatusGroq(t *testing.T) {
	// Cliente que falha o teste se alguém tentar a rede.
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Errorf("chamada de rede inesperada: %s", r.URL)
		return nil, http.ErrHandlerTimeout
	})}
	st := Status(context.Background(), Config{Provider: ProviderGroq, GroqKey: "gsk_segredo123"}, client, nil)
	if st.Provider != ProviderGroq || st.Groq == nil || !st.Groq.Configured || st.Groq.Model != DefaultGroqModel || st.Ollama != nil {
		t.Fatalf("st = %+v", st)
	}
	b, _ := json.Marshal(st)
	if strings.Contains(string(b), "gsk_segredo123") {
		t.Fatalf("chave vazou: %s", b)
	}
	st = Status(context.Background(), Config{Provider: ProviderGroq, GroqModel: "llama-3.3-70b-versatile"}, client, nil)
	if st.Groq.Configured || st.Groq.Model != "llama-3.3-70b-versatile" || st.Message == "" {
		t.Fatalf("sem chave: %+v", st)
	}
}

func TestStatusOff(t *testing.T) {
	st := Status(context.Background(), Config{Provider: ProviderOff}, nil, nil)
	if st.Provider != ProviderOff || st.Ollama != nil || st.Groq != nil {
		t.Fatalf("st = %+v", st)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
