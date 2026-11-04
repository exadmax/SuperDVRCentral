package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Conversa com o LLM no formato OpenAI (POST {base}/chat/completions), que a Groq e o Ollama (/v1) falam igual. Sem streaming:
// o pedido volta inteiro, com o texto e as chamadas de ferramenta.
const (
	// llmTimeout é o teto de uma resposta do LLM (a Groq responde em 1–3 s; um modelo local grande pode levar mais).
	llmTimeout = 30 * time.Second
	// llmErrSnippet é quanto da mensagem de erro do provedor entra no erro devolvido.
	llmErrSnippet = 200
	// llmMaxBody limita a resposta lida do provedor.
	llmMaxBody = 1 << 20
)

// groqBaseURL é a raiz da API OpenAI-compatível da Groq (variável só para os testes).
var groqBaseURL = "https://api.groq.com/openai/v1"

// Message é uma mensagem da conversa. Role: system | user | assistant | tool.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // só em assistant
	ToolCallID string     `json:"tool_call_id,omitempty"` // só em tool: a chamada que esta mensagem responde
	Name       string     `json:"name,omitempty"`
}

// Tool é uma função que o modelo pode chamar. Parameters é o JSON Schema dos argumentos.
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// MarshalJSON escreve a ferramenta no formato OpenAI: {"type":"function","function":{...}}.
func (t Tool) MarshalJSON() ([]byte, error) {
	params := t.Parameters
	if len(params) == 0 {
		params = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return json.Marshal(map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  params,
		},
	})
}

// ToolCall é uma chamada de ferramenta pedida pelo modelo. Arguments é o JSON dos argumentos, como o modelo escreveu.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type wireToolCall struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// MarshalJSON escreve a chamada no formato OpenAI (para reenviar o histórico ao modelo).
func (c ToolCall) MarshalJSON() ([]byte, error) {
	w := wireToolCall{ID: c.ID, Type: "function"}
	w.Function.Name, w.Function.Arguments = c.Name, c.Arguments
	return json.Marshal(w)
}

// UnmarshalJSON lê a chamada no formato OpenAI.
func (c *ToolCall) UnmarshalJSON(b []byte) error {
	var w wireToolCall
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	c.ID, c.Name, c.Arguments = w.ID, w.Function.Name, w.Function.Arguments
	return nil
}

// ChatRequest é o pedido ao LLM. O modelo sai da Config; Temperature 0 é enviada como 0.
type ChatRequest struct {
	Messages    []Message
	Tools       []Tool
	Temperature float64
	MaxTokens   int // 0 = o padrão do provedor
}

// ChatReply é a resposta do LLM: o texto e as ferramentas que ele quer chamar (pode vir só um dos dois).
type ChatReply struct {
	Content   string
	ToolCalls []ToolCall
}

// Complete faz uma pergunta ao LLM do provedor configurado. client nil usa um cliente próprio; o pedido tem no máximo llmTimeout.
// Os erros são em português e nunca trazem a chave da Groq.
func Complete(ctx context.Context, cfg Config, client *http.Client, req ChatRequest) (ChatReply, error) {
	cfg = cfg.WithDefaults()
	base, model, key, err := llmTarget(ctx, cfg)
	if err != nil {
		return ChatReply{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: llmTimeout}
	}
	ctx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()

	body := map[string]any{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
		"stream":      false,
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
		body["tool_choice"] = "auto"
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return ChatReply{}, fmt.Errorf("montar pedido ao LLM: %w", err)
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return ChatReply{}, fmt.Errorf("montar pedido ao LLM: %w", err)
	}
	hr.Header.Set("Content-Type", "application/json")
	if key != "" {
		hr.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := client.Do(hr)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ChatReply{}, fmt.Errorf("%s não respondeu em %s", providerName(cfg.Provider), llmTimeout)
		}
		return ChatReply{}, hideKey(fmt.Errorf("%s fora do alcance: %v", providerName(cfg.Provider), err), key)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, llmMaxBody))
	if err != nil {
		return ChatReply{}, fmt.Errorf("ler resposta de %s: %v", providerName(cfg.Provider), err)
	}
	if resp.StatusCode/100 != 2 {
		return ChatReply{}, hideKey(fmt.Errorf("%s respondeu %d: %s", providerName(cfg.Provider), resp.StatusCode, errSnippet(raw)), key)
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return ChatReply{}, fmt.Errorf("resposta inválida de %s: %v", providerName(cfg.Provider), err)
	}
	if len(out.Choices) == 0 {
		return ChatReply{}, fmt.Errorf("%s respondeu sem nenhuma mensagem", providerName(cfg.Provider))
	}
	m := out.Choices[0].Message
	return ChatReply{Content: strings.TrimSpace(m.Content), ToolCalls: m.ToolCalls}, nil
}

// llmTarget escolhe a raiz da API, o modelo e a chave pelo provedor.
func llmTarget(ctx context.Context, cfg Config) (base, model, key string, err error) {
	switch cfg.Provider {
	case ProviderGroq:
		if cfg.GroqKey == "" {
			return "", "", "", errors.New("chave da Groq não definida (informe na página Config)")
		}
		model = cfg.GroqModel
		if model == "" {
			model = DefaultGroqModel
		}
		return groqBaseURL, model, cfg.GroqKey, nil
	case ProviderOllama:
		model = cfg.Model
		if model == "" {
			rec, _ := Recommend(EffectiveMB(llmHardware().get(ctx)))
			if len(rec) == 0 {
				return "", "", "", errors.New("nenhum modelo do Ollama definido (escolha um na página Config)")
			}
			model = rec[0].Name
		}
		return cfg.OllamaURL + "/v1", model, "", nil
	default:
		return "", "", "", errors.New("IA desligada na configuração")
	}
}

// llmHW guarda a detecção da GPU usada para escolher o modelo do Ollama quando a Config não diz qual.
var (
	llmHWOnce sync.Once
	llmHW     *hwCache
)

func llmHardware() *hwCache {
	llmHWOnce.Do(func() {
		if llmHW == nil {
			llmHW = &hwCache{detect: DetectGPUs}
		}
	})
	return llmHW
}

func providerName(p string) string {
	switch p {
	case ProviderGroq:
		return "Groq"
	case ProviderOllama:
		return "Ollama"
	}
	return p
}

// errSnippet tira a mensagem do corpo de erro ({"error":{"message":...}} ou {"error":"..."}) e corta em llmErrSnippet caracteres.
func errSnippet(raw []byte) string {
	var e struct {
		Error json.RawMessage `json:"error"`
	}
	msg := strings.TrimSpace(string(raw))
	if json.Unmarshal(raw, &e) == nil && len(e.Error) > 0 {
		var obj struct {
			Message string `json:"message"`
		}
		var s string
		if json.Unmarshal(e.Error, &obj) == nil && obj.Message != "" {
			msg = obj.Message
		} else if json.Unmarshal(e.Error, &s) == nil && s != "" {
			msg = s
		}
	}
	msg = strings.Join(strings.Fields(msg), " ")
	if r := []rune(msg); len(r) > llmErrSnippet {
		msg = string(r[:llmErrSnippet]) + "…"
	}
	if msg == "" {
		msg = "sem detalhes"
	}
	return msg
}

// hideKey troca a chave por *** caso o provedor (ou a pilha de rede) a repita na mensagem.
func hideKey(err error, key string) error {
	if key == "" || !strings.Contains(err.Error(), key) {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), key, "***"))
}
