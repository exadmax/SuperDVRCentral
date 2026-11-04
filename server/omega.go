package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"dvr/hub/ai"
)

// Omega é o assistente de voz/texto da casa, no estilo J.A.R.V.I.S. A interface manda a frase (já transcrita) e recebe o que falar e
// as ações a executar (mostrar câmera, trânsito ou agenda). Pedidos comuns são resolvidos por regras, sem gastar o LLM; o resto vai
// ao provedor configurado (Groq ou Ollama), com as ações oferecidas como ferramentas.
const (
	omegaMaxText     = 500      // caracteres do pedido e de cada mensagem do histórico
	omegaMaxHistory  = 6        // mensagens do histórico enviadas ao LLM (as últimas)
	omegaMaxBody     = 16 << 10 // bytes do corpo do pedido
	omegaRateLimit   = 20       // pedidos por omegaRateWindow, somando todos os clientes (a Groq cobra por uso)
	omegaRateWindow  = time.Minute
	omegaTemperature = 0.4  // respostas variadas, mas sem inventar
	omegaMaxTokens   = 1024 // inclui o raciocínio dos modelos que pensam antes de responder (gpt-oss, qwen3)
	omegaErrReason   = 120  // caracteres do motivo de erro mostrado ao usuário
	omegaDayToday    = "today"
	omegaDayTomorrow = "tomorrow"
	omegaSayFallback = "Perdão, Comandante, não tenho uma boa resposta para isso." // o modelo não disse nada nem chamou ferramenta
)

type omega struct {
	cams   *cameras
	tr     *traffic
	db     *store // estoque e serviços (nil = banco indisponível)
	aiCfg  func() ai.Config
	st     *store       // só para checkWrite (origem)
	client *http.Client // nil = o padrão do ai.Complete
	now    func() time.Time

	mu   sync.Mutex
	hits []time.Time // horários dos pedidos dentro da janela do limite de taxa
}

// omegaAction é uma ação que a interface executa. Só os campos do tipo vêm preenchidos.
type omegaAction struct {
	Type   string     `json:"type"` // camera | traffic | agenda | list
	ID     string     `json:"id,omitempty"`
	Name   string     `json:"name,omitempty"`
	Person string     `json:"person,omitempty"`
	Label  string     `json:"label,omitempty"`
	Day    string     `json:"day,omitempty"`  // today | tomorrow
	List   *omegaList `json:"list,omitempty"` // lista pronta (estoque, serviços)
}

type omegaReply struct {
	Say      string        `json:"say"`
	Actions  []omegaAction `json:"actions"`
	Source   string        `json:"source"`   // rule | llm | none
	Provider string        `json:"provider"` // groq | ollama | off
}

type omegaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type omegaItem struct{ id, name string } // câmera ou pessoa, como o Omega conhece

func newOmega(cams *cameras, tr *traffic, db *store, aiCfg func() ai.Config, origins []string) *omega {
	return &omega{cams: cams, tr: tr, db: db, aiCfg: aiCfg, st: &store{origins: origins}, now: time.Now}
}

// register registra POST /api/omega/ask: {"text": "...", "history": [{"role","content"}]} -> omegaReply.
func (o *omega) register(mux *http.ServeMux) {
	mux.HandleFunc("/api/omega/ask", o.ask)
}

func (o *omega) ask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, &apiError{http.StatusMethodNotAllowed, "use POST"})
		return
	}
	if err := o.st.checkWrite(r); err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Text    string     `json:"text"`
		History []omegaMsg `json:"history"`
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, omegaMaxBody))
	if err != nil {
		writeError(w, badRequest("pedido grande demais"))
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, badRequest("JSON inválido: %v", err))
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeError(w, badRequest("diga alguma coisa ao Omega"))
		return
	}
	if len([]rune(text)) > omegaMaxText {
		writeError(w, badRequest("pedido: no máximo %d caracteres", omegaMaxText))
		return
	}
	if !o.allow() {
		writeError(w, &apiError{http.StatusTooManyRequests, "muitos pedidos ao Omega; tente de novo em um minuto"})
		return
	}

	cfg := o.aiCfg().WithDefaults()
	cams, people := o.cameraList(), o.peopleList()
	norm := omegaNormalize(text)
	for _, rule := range []func() (omegaReply, bool){
		func() (omegaReply, bool) { return o.dataRules(norm) }, // estoque/serviços citados explicitamente vêm antes (ex.: "o que tenho no estoque" não é agenda)
		func() (omegaReply, bool) { return omegaRules(norm, cams, people) },
		func() (omegaReply, bool) { return o.serviceTypeRule(norm) },
	} {
		if rep, ok := rule(); ok {
			rep.Provider = cfg.Provider
			writeJSON(w, rep)
			return
		}
	}

	rep := omegaReply{Source: "llm", Provider: cfg.Provider, Actions: []omegaAction{}}
	msgs, tools := o.messages(text, req.History, cams, people), omegaTools(cams, people, o.dataTools())
	var (
		out      ai.ChatReply
		dataSays []string // o que as ferramentas de estoque/serviços fizeram, caso o modelo fique mudo ou a IA caia no meio
	)
	for round := 1; ; round++ {
		out, err = ai.Complete(r.Context(), cfg, o.client, ai.ChatRequest{Messages: msgs, Tools: tools, Temperature: omegaTemperature, MaxTokens: omegaMaxTokens})
		if err != nil {
			break
		}
		rep.Actions = append(rep.Actions, omegaToolActions(out.ToolCalls, cams, people)...)
		if round >= omegaMaxRounds || !omegaHasDataCall(out.ToolCalls) {
			break
		}
		// ferramentas de dados: o hub executa e devolve o resultado ao modelo, que então responde (toda chamada precisa de resposta)
		msgs = append(msgs, ai.Message{Role: "assistant", Content: out.Content, ToolCalls: out.ToolCalls})
		for _, c := range out.ToolCalls {
			res := `{"ok":true}`
			if oc, ok := o.runDataTool(c.Name, c.Arguments); ok {
				res = oc.Data
				dataSays = append(dataSays, oc.Say)
				if oc.Action != nil {
					rep.Actions = append(rep.Actions, *oc.Action)
				}
			}
			msgs = append(msgs, ai.Message{Role: "tool", ToolCallID: c.ID, Name: c.Name, Content: res})
		}
	}
	if err != nil {
		if cfg.Provider != ai.ProviderOff {
			log.Printf("omega: %v", err)
		}
		rep.Source = "none"
		rep.Say = "Não consegui falar com a IA agora: " + shortReason(err.Error())
		if len(dataSays) > 0 { // já alterou algo: o usuário precisa saber
			rep.Say = strings.Join(dataSays, " ")
		}
		writeJSON(w, rep)
		return
	}
	rep.Say = omegaClean(out.Content)
	if rep.Say == "" {
		says := dataSays
		for _, a := range rep.Actions {
			if s := omegaActionSay(a); s != "" {
				says = append(says, s)
			}
		}
		rep.Say = strings.Join(says, " ")
	}
	if rep.Say == "" {
		rep.Say = omegaSayFallback
	}
	writeJSON(w, rep)
}

// allow aplica o limite de taxa: no máximo omegaRateLimit pedidos em omegaRateWindow.
func (o *omega) allow() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	now := o.now()
	keep := o.hits[:0]
	for _, t := range o.hits {
		if now.Sub(t) < omegaRateWindow {
			keep = append(keep, t)
		}
	}
	o.hits = keep
	if len(o.hits) >= omegaRateLimit {
		return false
	}
	o.hits = append(o.hits, now)
	return true
}

func (o *omega) cameraList() []omegaItem {
	if o.cams == nil {
		return nil
	}
	var out []omegaItem
	for _, c := range o.cams.all() {
		out = append(out, omegaItem{c.cfg.ID, c.cfg.Name})
	}
	return out
}

func (o *omega) peopleList() []omegaItem {
	if o.tr == nil {
		return nil
	}
	var out []omegaItem
	for _, p := range o.tr.config().People {
		out = append(out, omegaItem{p.ID, p.Label})
	}
	return out
}

// ---- normalização e regras ----

// omegaWakeWords são as palavras de ativação tiradas do começo do pedido (já normalizadas; as mais longas primeiro).
var omegaWakeWords = []string{"ok omega", "ola omega", "oi omega", "omega", "ok jarvis", "jarvis"}

// omegaNormalize deixa o pedido em minúsculas, sem acento, sem pontuação e sem a palavra de ativação.
func omegaNormalize(s string) string {
	s = recAccents.Replace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	s = strings.Join(strings.Fields(b.String()), " ")
	for again := true; again; {
		again = false
		for _, w := range omegaWakeWords {
			if s == w {
				return ""
			}
			if strings.HasPrefix(s, w+" ") {
				s, again = s[len(w)+1:], true
				break
			}
		}
	}
	return s
}

// omegaFiller são palavras que não nomeiam câmera nem pessoa: o que sobra depois delas é tratado como um nome pedido.
var omegaFiller = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a o as os um uma uns umas de da do das dos e em na no nas nos ao aos pra pro para por com
		me mim eu meu minha seu sua voce favor porfavor ok oi ola ai la aqui ta esta estao ficou fica vai como qual quais que quanto quantos
		mostra mostre mostrar mostrando exibe exiba exibir abre abra abrir ver veja coloca coloque bota poe liga ligue tela imagem video vivo
		camera cameras cam cams transito tempo trabalho servico escritorio ate chegar caminho rota leva levar demora demorar minutos
		hoje agora amanha manha tarde noite rapido rapidinho entao`) {
		omegaFiller[w] = true
	}
}

func hasWord(words []string, w ...string) bool {
	for _, x := range words {
		for _, y := range w {
			if x == y {
				return true
			}
		}
	}
	return false
}

// omegaMatch acha o item citado no pedido: nome inteiro (3 pontos), id (2) ou palavras do nome (1 cada). ok=false se nenhum casou.
func omegaMatch(norm string, words []string, items []omegaItem) (omegaItem, bool) {
	best, bestScore := omegaItem{}, 0
	padded := " " + norm + " "
	for _, it := range items {
		score := 0
		name := omegaNormalize(it.name)
		if name != "" && strings.Contains(padded, " "+name+" ") {
			score += 3
		}
		if id := omegaNormalize(strings.NewReplacer("-", " ", "_", " ").Replace(it.id)); id != "" && strings.Contains(padded, " "+id+" ") {
			score += 2
		}
		for _, nw := range strings.Fields(name) {
			if (len(nw) >= 3 || isDigits(nw)) && !omegaFiller[nw] && hasWord(words, nw) {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = it, score
		}
	}
	return best, bestScore > 0
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// omegaNamed diz se sobrou no pedido alguma palavra que parece um nome (fora das palavras de enchimento).
func omegaNamed(words []string) bool {
	for _, w := range words {
		if !omegaFiller[w] {
			return true
		}
	}
	return false
}

// omegaRules resolve os pedidos comuns sem LLM. ok=false: o pedido vai ao LLM.
func omegaRules(norm string, cams, people []omegaItem) (omegaReply, bool) {
	words := strings.Fields(norm)
	padded := " " + norm + " "
	camWord := hasWord(words, "camera", "cameras", "cam", "cams")
	trafficAsk := hasWord(words, "transito") ||
		hasWord(words, "trabalho", "servico", "escritorio") && hasWord(words, "tempo", "ate", "chegar", "caminho", "demora", "leva")
	agendaAsk := hasWord(words, "compromisso", "compromissos", "agenda", "evento", "eventos", "reuniao", "reunioes") ||
		strings.Contains(padded, " o que tenho ") || strings.Contains(padded, " o que eu tenho ")
	showVerb := len(words) > 0 && hasWord(words[:1], "mostra", "mostre", "mostrar", "exibe", "exiba", "abre", "abra", "ver", "me")

	switch {
	case camWord:
		return omegaCameraRule(norm, words, cams, true), true
	case trafficAsk:
		return omegaTrafficRule(norm, words, people), true
	case agendaAsk:
		day := omegaDayToday
		if hasWord(words, "amanha") {
			day = omegaDayTomorrow
		}
		return omegaRuleReply(omegaAction{Type: "agenda", Day: day}), true
	case showVerb:
		if _, ok := omegaMatch(norm, words, cams); ok {
			return omegaCameraRule(norm, words, cams, false), true
		}
	}
	return omegaReply{}, false
}

func omegaRuleReply(a omegaAction) omegaReply {
	return omegaReply{Say: omegaActionSay(a), Actions: []omegaAction{a}, Source: "rule"}
}

func omegaCameraRule(norm string, words []string, cams []omegaItem, explicit bool) omegaReply {
	if len(cams) == 0 {
		return omegaReply{Say: "Não há câmeras configuradas, Comandante.", Actions: []omegaAction{}, Source: "rule"}
	}
	c, ok := omegaMatch(norm, words, cams)
	if !ok && explicit && len(cams) == 1 && !omegaNamed(words) {
		c, ok = cams[0], true
	}
	if !ok {
		return omegaReply{Say: "Não encontrei essa câmera, Comandante. Disponíveis: " + omegaNames(cams) + ".", Actions: []omegaAction{}, Source: "rule"}
	}
	return omegaRuleReply(omegaAction{Type: "camera", ID: c.id, Name: c.name})
}

func omegaTrafficRule(norm string, words []string, people []omegaItem) omegaReply {
	if len(people) == 0 {
		return omegaReply{Say: "Não há rotas de trânsito configuradas, Comandante.", Actions: []omegaAction{}, Source: "rule"}
	}
	p, ok := omegaMatch(norm, words, people)
	if !ok && len(people) == 1 && !omegaNamed(words) {
		p, ok = people[0], true
	}
	if !ok {
		return omegaReply{Say: "De quem, Comandante? Tenho as rotas de: " + omegaNames(people) + ".", Actions: []omegaAction{}, Source: "rule"}
	}
	return omegaRuleReply(omegaAction{Type: "traffic", Person: p.id, Label: p.name})
}

func omegaNames(items []omegaItem) string {
	var n []string
	for _, it := range items {
		n = append(n, it.name)
	}
	return strings.Join(n, ", ")
}

// omegaActionSay é a frase padrão de cada ação (quando a regra resolve ou o modelo chamou a ferramenta sem dizer nada).
func omegaActionSay(a omegaAction) string {
	switch a.Type {
	case "camera":
		return "Exibindo a câmera " + a.Name + ", Comandante."
	case "traffic":
		return "Consultando o trânsito até o trabalho de " + a.Label + ", Comandante."
	case "agenda":
		if a.Day == omegaDayTomorrow {
			return "Estes são os seus compromissos de amanhã, Comandante."
		}
		return "Estes são os seus compromissos de hoje, Comandante."
	}
	return ""
}

// ---- LLM ----

func (o *omega) messages(text string, history []omegaMsg, cams, people []omegaItem) []ai.Message {
	msgs := []ai.Message{{Role: "system", Content: omegaSystemPrompt(o.now(), cams, people) + o.dataPrompt()}}
	var hist []ai.Message
	for _, m := range history {
		c := strings.TrimSpace(m.Content)
		if (m.Role != "user" && m.Role != "assistant") || c == "" {
			continue
		}
		if r := []rune(c); len(r) > omegaMaxText {
			c = string(r[:omegaMaxText])
		}
		hist = append(hist, ai.Message{Role: m.Role, Content: c})
	}
	if len(hist) > omegaMaxHistory {
		hist = hist[len(hist)-omegaMaxHistory:]
	}
	msgs = append(msgs, hist...)
	return append(msgs, ai.Message{Role: "user", Content: text})
}

var (
	omegaWeekdays = []string{"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"}
	omegaMonths   = []string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}
)

func omegaSystemPrompt(now time.Time, cams, people []omegaItem) string {
	var b strings.Builder
	b.WriteString("Você é Omega, o assistente da casa no estilo J.A.R.V.I.S.: formal, com um toque de humor seco. Chame o usuário de \"Comandante\". ")
	b.WriteString("Responda em português do Brasil com no máximo 2 frases curtas, próprias para serem faladas em voz alta, sem markdown, listas nem emojis. ")
	b.WriteString("Quando o pedido for mostrar uma câmera, o trânsito até o trabalho ou a agenda, use a ferramenta correspondente. ")
	b.WriteString("Não invente câmeras, pessoas nem compromissos.\n")
	fmt.Fprintf(&b, "Agora: %s, %d de %s de %d, %02d:%02d.\n", omegaWeekdays[now.Weekday()], now.Day(), omegaMonths[now.Month()-1], now.Year(), now.Hour(), now.Minute())
	b.WriteString("Câmeras (id: nome): ")
	b.WriteString(omegaListPrompt(cams))
	b.WriteString("\nPessoas com rota de trânsito (id: nome): ")
	b.WriteString(omegaListPrompt(people))
	return b.String()
}

func omegaListPrompt(items []omegaItem) string {
	if len(items) == 0 {
		return "nenhuma"
	}
	var s []string
	for _, it := range items {
		s = append(s, it.id+": "+it.name)
	}
	return strings.Join(s, "; ")
}

// omegaTools monta as ferramentas; a que teria lista vazia fica de fora. extra: as de estoque e serviços (omega.dataTools).
func omegaTools(cams, people []omegaItem, extra []ai.Tool) []ai.Tool {
	var tools []ai.Tool
	if len(cams) > 0 {
		tools = append(tools, ai.Tool{Name: "show_camera", Description: "Mostra na tela a imagem ao vivo de uma câmera da casa.",
			Parameters: omegaEnumSchema("camera_id", "id da câmera", omegaIDs(cams))})
	}
	if len(people) > 0 {
		tools = append(tools, ai.Tool{Name: "show_traffic", Description: "Mostra o trânsito e o tempo até o trabalho de uma pessoa.",
			Parameters: omegaEnumSchema("person_id", "id da pessoa", omegaIDs(people))})
	}
	tools = append(tools, ai.Tool{Name: "show_agenda", Description: "Mostra os compromissos da agenda de hoje ou de amanhã.",
		Parameters: omegaEnumSchema("day", "today (hoje) ou tomorrow (amanhã)", []string{omegaDayToday, omegaDayTomorrow})})
	return append(tools, extra...)
}

func omegaIDs(items []omegaItem) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.id
	}
	return ids
}

func omegaEnumSchema(prop, desc string, enum []string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			prop: map[string]any{"type": "string", "description": desc, "enum": enum},
		},
		"required": []string{prop},
	})
	return b
}

// omegaToolActions converte as chamadas de ferramenta em ações, conferindo os ids; chamada inválida é ignorada.
func omegaToolActions(calls []ai.ToolCall, cams, people []omegaItem) []omegaAction {
	find := func(items []omegaItem, id string) (omegaItem, bool) {
		for _, it := range items {
			if it.id == id {
				return it, true
			}
		}
		return omegaItem{}, false
	}
	actions := []omegaAction{}
	for _, c := range calls {
		var args map[string]string
		if json.Unmarshal([]byte(c.Arguments), &args) != nil {
			continue
		}
		switch c.Name {
		case "show_camera":
			if it, ok := find(cams, args["camera_id"]); ok {
				actions = append(actions, omegaAction{Type: "camera", ID: it.id, Name: it.name})
			}
		case "show_traffic":
			if it, ok := find(people, args["person_id"]); ok {
				actions = append(actions, omegaAction{Type: "traffic", Person: it.id, Label: it.name})
			}
		case "show_agenda":
			if d := args["day"]; d == omegaDayToday || d == omegaDayTomorrow {
				actions = append(actions, omegaAction{Type: "agenda", Day: d})
			}
		}
	}
	return actions
}

// omegaClean tira marcas de markdown que escapem do modelo (a frase vai para a síntese de voz).
var omegaMarkdown = strings.NewReplacer("**", "", "__", "", "`", "", "#", "")

func omegaClean(s string) string {
	return strings.Join(strings.Fields(omegaMarkdown.Replace(s)), " ")
}

func shortReason(s string) string {
	if r := []rune(s); len(r) > omegaErrReason {
		return string(r[:omegaErrReason]) + "…"
	}
	return s
}
