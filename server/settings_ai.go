package main

// Aba IA da página Config: provedor (Ollama local ou Groq), endereço do Ollama e modelos. A chave da Groq fica só no servidor:
// a leitura devolve apenas se ela está definida, e gravar com a chave vazia mantém a atual (mesma regra da chave do Google Routes).

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"dvr/hub/ai"
)

type aiView struct {
	Provider   string `json:"provider"`
	OllamaURL  string `json:"ollamaURL"`
	Model      string `json:"model"`
	GroqModel  string `json:"groqModel"`
	GroqKeySet bool   `json:"groqKeySet"`
	GroqKey    string `json:"groqKey,omitempty"`      // só na gravação: vazio = manter a atual
	ClearGroq  bool   `json:"clearGroqKey,omitempty"` // só na gravação: apaga a chave
}

// aiConfig é a configuração de IA em uso (lida pelas rotas /api/ai/*).
func (m *settingsMgr) aiConfig() ai.Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg.AI.WithDefaults()
}

func (m *settingsMgr) aiView() aiView {
	c := m.cfg.AI.WithDefaults()
	return aiView{Provider: c.Provider, OllamaURL: c.OllamaURL, Model: c.Model, GroqModel: c.GroqModel, GroqKeySet: c.GroqKey != ""}
}

func (m *settingsMgr) putAI(body []byte) (interface{}, error) {
	var in aiView
	if err := decodeBody(body, &in); err != nil {
		return nil, err
	}
	c := m.cfg.AI
	switch p := strings.ToLower(strings.TrimSpace(in.Provider)); p {
	case ai.ProviderOllama, ai.ProviderGroq, ai.ProviderOff:
		c.Provider = p
	default:
		return nil, badRequest("provedor de IA inválido (use ollama, groq ou off)")
	}
	raw, err := trimLen(in.OllamaURL, 200, "endereço do Ollama")
	if err != nil {
		return nil, err
	}
	if raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, badRequest("endereço do Ollama inválido (ex.: http://127.0.0.1:11434)")
		}
	}
	c.OllamaURL = raw
	if c.Model, err = trimLen(in.Model, 100, "modelo"); err != nil {
		return nil, err
	}
	if c.GroqModel, err = trimLen(in.GroqModel, 100, "modelo da Groq"); err != nil {
		return nil, err
	}
	if in.ClearGroq {
		c.GroqKey = ""
	} else if k := strings.TrimSpace(in.GroqKey); k != "" {
		if len(k) > 200 {
			return nil, badRequest("chave longa demais")
		}
		c.GroqKey = k
	}
	if c.Provider == ai.ProviderGroq && c.GroqKey == "" {
		return nil, badRequest("para usar a Groq, informe a chave")
	}
	prev := m.cfg.AI
	m.cfg.AI = c
	m.st.AI = &c
	if err := m.saveLocked(); err != nil {
		m.cfg.AI = prev
		log.Printf("salvar %s: %v", m.path, err)
		return nil, &apiError{http.StatusInternalServerError, "não foi possível gravar as configurações no servidor"}
	}
	return m.aiView(), nil
}
