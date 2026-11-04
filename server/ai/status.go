package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Tempos máximos das perguntas ao Ollama. Ele roda na mesma máquina ou na rede local: se não respondeu nisso, está fora do ar.
const (
	ollamaTimeout     = 3 * time.Second
	ollamaDownloadURL = "https://ollama.com/download"

	// DefaultGroqModel é o modelo da Groq quando a configuração não diz outro (o mesmo que o JARVIS usava).
	DefaultGroqModel = "openai/gpt-oss-20b"
)

// StatusInfo é o estado do provedor de IA para a página Config. Só um dos blocos vem preenchido, conforme o provedor; a chave da Groq
// nunca aparece, só se está definida.
type StatusInfo struct {
	Provider string        `json:"provider"` // ollama | groq | off
	Ollama   *OllamaStatus `json:"ollama,omitempty"`
	Groq     *GroqStatus   `json:"groq,omitempty"`
	Message  string        `json:"message,omitempty"` // o que fazer, em português, quando algo falta
}

// OllamaStatus diz se o Ollama respondeu, a versão e os modelos instalados (cada um marcado pelo catálogo).
type OllamaStatus struct {
	URL            string           `json:"url"`
	Reachable      bool             `json:"reachable"`
	Version        string           `json:"version,omitempty"`
	Model          string           `json:"model,omitempty"` // o configurado; vazio = o primeiro sugerido
	ModelInstalled bool             `json:"modelInstalled"`
	EffectiveMB    int              `json:"effectiveMB"`
	Models         []InstalledModel `json:"models"`
}

// InstalledModel é um modelo baixado no Ollama.
type InstalledModel struct {
	Name          string `json:"name"`
	Size          int64  `json:"size"` // bytes em disco
	ParameterSize string `json:"parameterSize,omitempty"`
	Known         bool   `json:"known"` // está no catálogo
	Fits          bool   `json:"fits"`  // cabe inteiro na GPU
}

// GroqStatus diz só se há chave e qual modelo: nenhuma chamada de rede é feita aqui.
type GroqStatus struct {
	Configured bool   `json:"configured"`
	Model      string `json:"model"`
}

// Status consulta o provedor configurado. client nil usa um cliente próprio; cada pergunta ao Ollama tem ollamaTimeout.
func Status(ctx context.Context, cfg Config, client *http.Client, gpus []GPU) StatusInfo {
	cfg = cfg.WithDefaults()
	st := StatusInfo{Provider: cfg.Provider}
	switch cfg.Provider {
	case ProviderOff:
		st.Message = "IA desligada na configuração"
	case ProviderGroq:
		st.Groq = &GroqStatus{Configured: cfg.GroqKey != "", Model: cfg.GroqModel}
		if st.Groq.Model == "" {
			st.Groq.Model = DefaultGroqModel
		}
		if !st.Groq.Configured {
			st.Message = "Chave da Groq não definida: crie uma em https://console.groq.com/keys e informe na Config"
		}
	case ProviderOllama:
		st.Ollama, st.Message = ollamaStatus(ctx, cfg, client, gpus)
	}
	return st
}

func ollamaStatus(ctx context.Context, cfg Config, client *http.Client, gpus []GPU) (*OllamaStatus, string) {
	if client == nil {
		client = &http.Client{Timeout: ollamaTimeout}
	}
	eff := EffectiveMB(gpus)
	o := &OllamaStatus{URL: cfg.OllamaURL, Model: cfg.Model, EffectiveMB: eff, Models: []InstalledModel{}}

	var ver struct {
		Version string `json:"version"`
	}
	if err := getJSON(ctx, client, cfg.OllamaURL+"/api/version", &ver); err != nil {
		return o, fmt.Sprintf("Ollama não encontrado em %s: instale em %s ou confira o endereço", cfg.OllamaURL, ollamaDownloadURL)
	}
	o.Reachable, o.Version = true, ver.Version

	var tags struct {
		Models []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			Details struct {
				ParameterSize string `json:"parameter_size"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := getJSON(ctx, client, cfg.OllamaURL+"/api/tags", &tags); err != nil {
		return o, "O Ollama respondeu, mas não listou os modelos: " + err.Error()
	}
	for _, m := range tags.Models {
		known, fits := Fits(m.Name, eff)
		o.Models = append(o.Models, InstalledModel{Name: m.Name, Size: m.Size, ParameterSize: m.Details.ParameterSize, Known: known, Fits: fits})
		if cfg.Model != "" && (m.Name == cfg.Model || m.Name == cfg.Model+":latest") {
			o.ModelInstalled = true
		}
	}

	rec, _ := Recommend(eff)
	suggest := ""
	if len(rec) > 0 {
		suggest = rec[0].Name
	}
	switch {
	case cfg.Model != "" && !o.ModelInstalled:
		return o, fmt.Sprintf("O modelo %s não está instalado: rode \"ollama pull %s\"", cfg.Model, cfg.Model)
	case len(o.Models) == 0 && suggest != "":
		return o, fmt.Sprintf("Nenhum modelo instalado: para esta máquina, rode \"ollama pull %s\"", suggest)
	}
	return o, ""
}

// getJSON faz um GET com ollamaTimeout e decodifica a resposta.
func getJSON(ctx context.Context, client *http.Client, url string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, ollamaTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
