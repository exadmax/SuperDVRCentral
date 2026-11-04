package ai

import (
	"sort"
	"strings"
)

// Model é um modelo do Ollama no catálogo. MinMB é a memória de GPU para ele rodar inteiro nela (pesos em Q4 + contexto padrão);
// a interface compara com a memória efetiva para marcar "cabe / não cabe".
type Model struct {
	Name  string `json:"name"`  // nome no Ollama (ollama pull <name>)
	Label string `json:"label"` // nome para a interface
	MinMB int    `json:"minMB"`
	Note  string `json:"note"`
	Tools bool   `json:"tools"` // aceita tool calling
}

// catalog segue a tabela "Modelos por memória de vídeo" do docs/MILESTONES.md, do menor para o maior.
var catalog = []Model{
	{Name: "qwen3:1.7b", Label: "Qwen3 1.7B", MinMB: 2048, Note: "Roda até em CPU; respostas curtas", Tools: true},
	{Name: "llama3.2:3b", Label: "Llama 3.2 3B", MinMB: 3072, Note: "Leve; bom em CPU e GPU pequena", Tools: true},
	{Name: "qwen3:4b", Label: "Qwen3 4B", MinMB: 4096, Note: "Iris Xe e similares (Vulkan)", Tools: true},
	{Name: "llama3.1:8b", Label: "Llama 3.1 8B", MinMB: 6144, Note: "Equilíbrio com tool calling", Tools: true},
	{Name: "qwen3:8b", Label: "Qwen3 8B", MinMB: 6144, Note: "Equilíbrio com tool calling", Tools: true},
	{Name: "qwen3:14b", Label: "Qwen3 14B", MinMB: 11264, Note: "Mais preciso; pede 12 GB", Tools: true},
	{Name: "gpt-oss:20b", Label: "gpt-oss 20B", MinMB: 14336, Note: "O mesmo que o JARVIS usava na Groq", Tools: true},
	{Name: "qwen3:30b", Label: "Qwen3 30B", MinMB: 22528, Note: "Sobra para whisper e rostos com 24 GB+", Tools: true},
}

// Faixas da recomendação, pela memória efetiva (EffectiveMB). O limite de cima de cada faixa é inclusivo, como na tabela ("até 6 GB",
// "6–12 GB"): uma placa de 6 GB (6144 MB) fica em "até 6 GB" e uma de 12 GB em "6–12 GB". A Iris Xe com 24 GB de RAM tem ~12150 MB
// compartilhados, efetiva ~6075 MB, e cai em "até 6 GB" (qwen3:4b), que é o que funciona na prática. Abaixo de cpuBandMB a GPU não
// comporta nem o menor modelo e a recomendação é a de CPU. A tabela não tem faixa 20–24 GB: ela fica com a de 12–20 GB.
const (
	cpuBandMB  = 2048
	band6GBMB  = 6 * 1024
	band12GBMB = 12 * 1024
	band24GBMB = 24 * 1024
)

var bands = []struct {
	maxMB  int // inclusivo; o último é "sem teto"
	models []string
}{
	{cpuBandMB - 1, []string{"qwen3:1.7b", "llama3.2:3b"}},
	{band6GBMB, []string{"qwen3:4b", "llama3.2:3b"}},
	{band12GBMB, []string{"qwen3:8b", "llama3.1:8b"}},
	{band24GBMB - 1, []string{"gpt-oss:20b", "qwen3:14b"}},
	{1 << 30, []string{"qwen3:30b", "gpt-oss:20b"}},
}

// defaultTags é a tag que o Ollama baixa quando o nome vem sem tag (ou com ":latest").
var defaultTags = map[string]string{
	"qwen3":    "8b",
	"llama3.2": "3b",
	"llama3.1": "8b",
	"gpt-oss":  "20b",
}

// Recommend devolve os modelos sugeridos para a memória efetiva (os que cabem primeiro, na ordem da tabela) e o catálogo inteiro.
func Recommend(effectiveMB int) (recommended []Model, all []Model) {
	all = append([]Model(nil), catalog...)
	for _, b := range bands {
		if effectiveMB <= b.maxMB {
			for _, name := range b.models {
				if m, ok := lookup(name); ok {
					recommended = append(recommended, m)
				}
			}
			break
		}
	}
	// Na faixa 12–20 GB o gpt-oss:20b só cabe a partir de 14 GB: com 12 GB o qwen3:14b passa à frente.
	if effectiveMB >= cpuBandMB {
		sort.SliceStable(recommended, func(i, j int) bool {
			return recommended[i].MinMB <= effectiveMB && recommended[j].MinMB > effectiveMB
		})
	}
	return recommended, all
}

// Fits diz se o modelo está no catálogo (known) e se cabe inteiro na GPU com essa memória efetiva (fits; em CPU, 0, nunca cabe).
// Reconhece o nome base: "qwen3:8b", "qwen3:8b-q4_K_M", "library/qwen3:8b" e "qwen3" (= qwen3:latest = qwen3:8b) são o mesmo.
func Fits(modelName string, effectiveMB int) (known bool, fits bool) {
	m, ok := lookup(baseName(modelName))
	if !ok {
		return false, false
	}
	return true, effectiveMB > 0 && effectiveMB >= m.MinMB
}

func lookup(name string) (Model, bool) {
	for _, m := range catalog {
		if m.Name == name {
			return m, true
		}
	}
	return Model{}, false
}

// baseName reduz um nome do Ollama a "família:tamanho".
func baseName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	family, tag, _ := strings.Cut(name, ":")
	tag, _, _ = strings.Cut(tag, "-")
	if tag == "" || tag == "latest" {
		tag = defaultTags[family]
	}
	return family + ":" + tag
}
