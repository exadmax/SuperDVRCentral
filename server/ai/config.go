// Package ai reúne o que o hub faz com IA: o provedor de LLM (Ollama local ou Groq), a detecção da GPU e o catálogo de modelos
// que cabem nela. Os modelos rodam fora do processo do hub (no Ollama); aqui só ficam a configuração e as chamadas HTTP.
package ai

import "strings"

// Provedores aceitos em Config.Provider.
const (
	ProviderOllama = "ollama"
	ProviderGroq   = "groq"
	ProviderOff    = "off"

	DefaultOllamaURL = "http://127.0.0.1:11434"
)

// Config é o bloco "ai" do hub.config.json (e da página Config). A chave da Groq só existe no servidor: a API devolve apenas se ela está definida.
type Config struct {
	Provider  string `json:"provider,omitempty"`  // ollama (padrão) | groq | off
	OllamaURL string `json:"ollamaURL,omitempty"` // padrão DefaultOllamaURL
	Model     string `json:"model,omitempty"`     // modelo do Ollama; vazio = o primeiro sugerido para a GPU
	GroqKey   string `json:"groqKey,omitempty"`
	GroqModel string `json:"groqModel,omitempty"`
	UserName  string `json:"userName,omitempty"` // como o Omega chama o usuário; vazio = sem tratamento ("Olá, tudo bem?")
	Voice     string `json:"voice,omitempty"`    // nome da voz de síntese do navegador usada pelo Omega; vazio = escolha automática
}

// WithDefaults devolve a configuração com os padrões preenchidos e o provedor normalizado.
func (c Config) WithDefaults() Config {
	c.Provider = strings.ToLower(strings.TrimSpace(c.Provider))
	switch c.Provider {
	case ProviderOllama, ProviderGroq, ProviderOff:
	default:
		c.Provider = ProviderOllama
	}
	c.OllamaURL = strings.TrimRight(strings.TrimSpace(c.OllamaURL), "/")
	if c.OllamaURL == "" {
		c.OllamaURL = DefaultOllamaURL
	}
	c.Model = strings.TrimSpace(c.Model)
	c.GroqModel = strings.TrimSpace(c.GroqModel)
	c.UserName = strings.Join(strings.Fields(c.UserName), " ")
	c.Voice = strings.TrimSpace(c.Voice)
	return c
}
