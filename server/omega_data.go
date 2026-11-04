package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"dvr/hub/ai"
)

// Estoque da cozinha e serviços (profissionais) no Omega. As consultas e as alterações usam as mesmas regras de validação das páginas
// (itensAPI e contatosAPI). Pedidos simples de leitura são resolvidos por regra, sem LLM; com o LLM, estas funções são ferramentas
// cujo resultado volta ao modelo, que então fala (omegaMaxRounds limita as idas e vindas).
const (
	omegaSoonDays    = 7  // "a vencer": itens que vencem nesta quantidade de dias
	omegaMaxRows     = 8  // linhas do painel; o resto vira "e mais N"
	omegaMaxSayNames = 5  // nomes lidos em voz alta
	omegaMaxToolData = 40 // itens devolvidos ao LLM numa consulta
	omegaMaxToolText = 200
	omegaMaxRounds   = 4 // chamadas ao LLM por pedido, contando as de ferramenta
)

// omegaRow é uma linha do painel de lista; omegaList, o painel.
type omegaRow struct {
	Text string `json:"text"`
	Sub  string `json:"sub,omitempty"`
	Warn bool   `json:"warn,omitempty"`
}

type omegaList struct {
	Title string     `json:"title"`
	Rows  []omegaRow `json:"rows"`
	More  int        `json:"more,omitempty"`
}

// omegaOutcome é o resultado de uma consulta ou alteração: a frase padrão (a que as regras falam), o JSON que a ferramenta devolve
// ao LLM e o painel a mostrar (se houver).
type omegaOutcome struct {
	Say    string
	Data   string
	Action *omegaAction
}

func omegaData(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func omegaFail(msg string) omegaOutcome {
	return omegaOutcome{Say: "Não foi possível, Comandante: " + msg + ".", Data: omegaData(map[string]any{"ok": false, "erro": msg})}
}

// omegaDBFail traduz o erro do banco: os de validação (apiError) vão como estão; os demais ficam só no log.
func omegaDBFail(err error) omegaOutcome {
	var ae *apiError
	if errors.As(err, &ae) {
		return omegaFail(ae.msg)
	}
	log.Printf("omega: banco de dados: %v", err)
	return omegaFail("erro ao acessar o banco de dados")
}

var omegaNoDB = omegaFail("o banco de dados está indisponível")

func omegaPlural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func omegaNum(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', -1, 64), ".", ",", 1)
}

func omegaTrunc(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

// ---- busca por palavras ----

// omegaStem tira o plural simples (eletricistas, encanadores, limões, pães) para casar "ovos" com "ovo".
func omegaStem(w string) string {
	switch {
	case len(w) > 4 && (strings.HasSuffix(w, "res") || strings.HasSuffix(w, "zes")):
		return w[:len(w)-2]
	case len(w) > 3 && (strings.HasSuffix(w, "oes") || strings.HasSuffix(w, "aes")):
		return w[:len(w)-3] + "ao"
	case len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss"):
		return w[:len(w)-1]
	}
	return w
}

func omegaStems(s string) []string {
	words := strings.Fields(omegaNormalize(s))
	for i, w := range words {
		words[i] = omegaStem(w)
	}
	return words
}

// omegaWordsIn diz se todas as palavras de query aparecem em text (sem acento, maiúsculas nem plural). Query vazia casa com tudo.
func omegaWordsIn(query, text string) bool {
	have := map[string]bool{}
	for _, w := range omegaStems(text) {
		have[w] = true
	}
	for _, w := range omegaStems(query) {
		if !have[w] {
			return false
		}
	}
	return true
}

// ---- estoque ----

func (o *omega) locais() []omegaItem {
	list := defaultLocais
	if o.db != nil && o.db.locais != nil {
		list = o.db.locais()
	}
	out := make([]omegaItem, len(list))
	for i, id := range list {
		out[i] = omegaItem{id, strings.NewReplacer("-", " ", "_", " ").Replace(id)}
	}
	return out
}

func (o *omega) items() ([]Item, error) {
	v, err := o.db.itensAPI().list()
	if err != nil {
		return nil, err
	}
	return v.([]Item), nil
}

// today é a data de hoje à meia-noite (UTC só para a conta de dias; a validade não tem fuso).
func (o *omega) today() time.Time {
	n := o.now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// omegaDaysLeft: dias até a validade (negativo = vencido). ok=false se o item não tem validade.
func omegaDaysLeft(it Item, today time.Time) (int, bool) {
	if it.Validade == nil {
		return 0, false
	}
	d, err := time.Parse("2006-01-02", *it.Validade)
	if err != nil {
		return 0, false
	}
	return int(d.Sub(today).Hours() / 24), true
}

func omegaValidity(days int) string {
	switch {
	case days < -1:
		return fmt.Sprintf("vencido há %d dias", -days)
	case days == -1:
		return "venceu ontem"
	case days == 0:
		return "vence hoje"
	case days == 1:
		return "vence amanhã"
	}
	return fmt.Sprintf("vence em %d dias", days)
}

func omegaQty(it Item) string {
	var p []string
	if it.Quantidade != nil {
		p = append(p, omegaNum(*it.Quantidade)+" un")
	}
	if it.PesoKg != nil {
		p = append(p, omegaNum(*it.PesoKg)+" kg")
	}
	return strings.Join(p, " e ")
}

// omegaItemSay descreve o item para falar: "Leite, 2 un, geladeira, vence em 3 dias".
func omegaItemSay(it Item, today time.Time, withLocal bool) string {
	p := []string{it.Nome, omegaQty(it)}
	if withLocal {
		p = append(p, it.Local)
	}
	if d, ok := omegaDaysLeft(it, today); ok {
		p = append(p, omegaValidity(d))
	}
	return strings.Join(p, ", ")
}

func omegaItemData(it Item, today time.Time) map[string]any {
	m := map[string]any{"id": it.ID, "nome": it.Nome, "local": it.Local}
	if it.Quantidade != nil {
		m["quantidade"] = *it.Quantidade
	}
	if it.PesoKg != nil {
		m["peso_kg"] = *it.PesoKg
	}
	if it.Validade != nil {
		m["validade"] = *it.Validade
		if d, ok := omegaDaysLeft(it, today); ok {
			m["situacao"] = omegaValidity(d)
		}
	}
	return m
}

func omegaItemsData(items []Item, today time.Time) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for i, it := range items {
		if i == omegaMaxToolData {
			break
		}
		out = append(out, omegaItemData(it, today))
	}
	return out
}

// omegaItemRows monta o painel: vencidos em destaque.
func omegaItemRows(title string, items []Item, today time.Time) *omegaAction {
	l := &omegaList{Title: title, Rows: []omegaRow{}}
	for i, it := range items {
		if i == omegaMaxRows {
			l.More = len(items) - omegaMaxRows
			break
		}
		row := omegaRow{Text: it.Nome, Sub: omegaQty(it)}
		if d, ok := omegaDaysLeft(it, today); ok {
			row.Sub += " · " + omegaValidity(d)
			row.Warn = d < 0
		}
		l.Rows = append(l.Rows, row)
	}
	return &omegaAction{Type: "list", List: l}
}

// stockQuery lista o estoque, podendo filtrar por parte do nome e por local.
func (o *omega) stockQuery(query, local string) omegaOutcome {
	if o.db == nil {
		return omegaNoDB
	}
	all, err := o.items()
	if err != nil {
		return omegaDBFail(err)
	}
	query = strings.TrimSpace(query)
	local = strings.ToLower(strings.TrimSpace(local))
	var hit []Item
	for _, it := range all {
		if (local == "" || it.Local == local) && omegaWordsIn(query, it.Nome) {
			hit = append(hit, it)
		}
	}
	today := o.today()
	where := ""
	if local != "" {
		where = " em " + local
	}
	data := omegaData(map[string]any{"total": len(hit), "itens": omegaItemsData(hit, today)})
	if len(hit) == 0 {
		say := "O estoque está vazio, Comandante."
		switch {
		case query != "":
			say = "Não encontrei " + query + " no estoque" + where + ", Comandante."
		case local != "":
			say = "Não há nada em " + local + ", Comandante."
		}
		return omegaOutcome{Say: say, Data: data}
	}
	var names []string
	for i, it := range hit {
		if i == omegaMaxSayNames {
			names = append(names, fmt.Sprintf("e mais %d", len(hit)-omegaMaxSayNames))
			break
		}
		names = append(names, omegaItemSay(it, today, local == ""))
	}
	say := "Encontrei " + omegaPlural(len(hit), "item", "itens") + where + ": " + strings.Join(names, "; ") + "."
	title := "Estoque"
	switch {
	case query != "":
		title += " · " + query
	case local != "":
		title += " · " + local
	}
	return omegaOutcome{Say: say, Data: data, Action: omegaItemRows(title, hit, today)}
}

// stockExpired separa o que já venceu do que vence nos próximos days dias.
func (o *omega) stockExpired(days int, local string) omegaOutcome {
	if o.db == nil {
		return omegaNoDB
	}
	if days <= 0 || days > 365 {
		days = omegaSoonDays
	}
	all, err := o.items()
	if err != nil {
		return omegaDBFail(err)
	}
	local = strings.ToLower(strings.TrimSpace(local))
	today := o.today()
	var gone, soon []Item
	for _, it := range all {
		d, ok := omegaDaysLeft(it, today)
		if !ok || local != "" && it.Local != local {
			continue
		}
		switch {
		case d < 0:
			gone = append(gone, it)
		case d <= days:
			soon = append(soon, it)
		}
	}
	data := omegaData(map[string]any{"dias_considerados": days, "vencidos": omegaItemsData(gone, today), "a_vencer": omegaItemsData(soon, today)})
	list := func(items []Item) string {
		var p []string
		for i, it := range items {
			if i == omegaMaxSayNames {
				p = append(p, fmt.Sprintf("e mais %d", len(items)-omegaMaxSayNames))
				break
			}
			d, _ := omegaDaysLeft(it, today)
			p = append(p, it.Nome+" ("+omegaValidity(d)+")")
		}
		return strings.Join(p, "; ")
	}
	where := ""
	if local != "" {
		where = " em " + local
	}
	switch {
	case len(gone) == 0 && len(soon) == 0:
		return omegaOutcome{Say: fmt.Sprintf("Nenhum item vencido no estoque%s, Comandante, e nada vence nos próximos %d dias.", where, days), Data: data}
	case len(gone) == 0:
		return omegaOutcome{Say: fmt.Sprintf("Nada vencido%s, Comandante, mas %s nos próximos %d dias: %s.", where,
			omegaPlural(len(soon), "item vence", "itens vencem"), days, list(soon)), Data: data, Action: omegaItemRows("Estoque · a vencer", soon, today)}
	}
	say := fmt.Sprintf("Há %s%s, Comandante: %s.", omegaPlural(len(gone), "item vencido", "itens vencidos"), where, list(gone))
	if len(soon) > 0 {
		say += fmt.Sprintf(" E %s nos próximos %d dias.", omegaPlural(len(soon), "vence", "vencem"), days)
	}
	return omegaOutcome{Say: say, Data: data, Action: omegaItemRows("Estoque · validade", append(gone, soon...), today)}
}

// stockAdd guarda um item novo (a validação é a mesma da página Estoque).
func (o *omega) stockAdd(it Item) omegaOutcome {
	if o.db == nil {
		return omegaNoDB
	}
	body, _ := json.Marshal(it)
	v, err := o.db.itensAPI().create(body)
	if err != nil {
		return omegaDBFail(err)
	}
	saved := v.(Item)
	say := "Anotado, Comandante: " + omegaItemSay(saved, o.today(), true) + "."
	return omegaOutcome{Say: say, Data: omegaData(map[string]any{"ok": true, "item": omegaItemData(saved, o.today())})}
}

// stockRemove tira um item do estoque (por id ou pelo nome). Com qty menor que a quantidade guardada, só baixa a quantidade.
func (o *omega) stockRemove(id int64, nome string, qty *float64) omegaOutcome {
	if o.db == nil {
		return omegaNoDB
	}
	all, err := o.items()
	if err != nil {
		return omegaDBFail(err)
	}
	var found []Item
	for _, it := range all {
		if id > 0 && it.ID == id || id <= 0 && strings.TrimSpace(nome) != "" && omegaWordsIn(nome, it.Nome) {
			found = append(found, it)
		}
	}
	today := o.today()
	switch {
	case len(found) == 0:
		return omegaFail("não encontrei esse item no estoque")
	case len(found) > 1:
		var names []string
		for _, it := range found {
			names = append(names, omegaItemSay(it, today, true))
		}
		return omegaOutcome{
			Say:  "Há vários itens parecidos, Comandante: " + strings.Join(names, "; ") + ". Qual deles?",
			Data: omegaData(map[string]any{"ok": false, "erro": "mais de um item combina; pergunte qual", "itens": omegaItemsData(found, today)}),
		}
	}
	it := found[0]
	if qty != nil && *qty > 0 {
		if it.Quantidade == nil {
			return omegaFail(it.Nome + " é controlado por peso, não por quantidade")
		}
		if *qty < *it.Quantidade {
			left := *it.Quantidade - *qty
			it.Quantidade = &left
			body, _ := json.Marshal(it)
			v, err := o.db.itensAPI().update(it.ID, body)
			if err != nil {
				return omegaDBFail(err)
			}
			saved := v.(Item)
			return omegaOutcome{
				Say:  fmt.Sprintf("Baixei %s de %s, Comandante; restam %s.", omegaNum(*qty), saved.Nome, omegaNum(left)),
				Data: omegaData(map[string]any{"ok": true, "item": omegaItemData(saved, today)}),
			}
		}
	}
	if err := o.db.itensAPI().remove(it.ID); err != nil {
		return omegaDBFail(err)
	}
	return omegaOutcome{Say: "Removi " + it.Nome + " do estoque, Comandante.", Data: omegaData(map[string]any{"ok": true, "removido": omegaItemData(it, today)})}
}

// ---- serviços (profissionais) ----

func (o *omega) contacts() ([]Contato, error) {
	v, err := o.db.contatosAPI().list()
	if err != nil {
		return nil, err
	}
	return v.([]Contato), nil
}

func omegaContactData(c Contato) map[string]any {
	return map[string]any{"id": c.ID, "nome": c.Nome, "tipo_servico": c.TipoServico, "contato": c.Contato, "indicacao": omegaTrunc(c.Indicacao, omegaMaxToolText),
		"servico": omegaTrunc(c.Servico, omegaMaxToolText), "obs": omegaTrunc(c.Obs, omegaMaxToolText)}
}

// serviceSearch procura profissionais pelo nome, tipo de serviço, o que fazem, indicação ou observação; quem casa no nome ou no tipo vem primeiro.
func (o *omega) serviceSearch(query string) omegaOutcome {
	if o.db == nil {
		return omegaNoDB
	}
	all, err := o.contacts()
	if err != nil {
		return omegaDBFail(err)
	}
	query = strings.TrimSpace(query)
	var first, rest []Contato
	for _, c := range all {
		switch {
		case omegaWordsIn(query, c.Nome+" "+c.TipoServico):
			first = append(first, c)
		case omegaWordsIn(query, c.Nome+" "+c.TipoServico+" "+c.Servico+" "+c.Indicacao+" "+c.Obs):
			rest = append(rest, c)
		}
	}
	hit := append(first, rest...)
	data := make([]map[string]any, 0, len(hit))
	for i, c := range hit {
		if i == omegaMaxToolData {
			break
		}
		data = append(data, omegaContactData(c))
	}
	out := omegaOutcome{Data: omegaData(map[string]any{"total": len(hit), "servicos": data})}
	if len(hit) == 0 {
		out.Say = "Não há serviços cadastrados, Comandante."
		if query != "" {
			out.Say = "Não encontrei nenhum profissional para " + query + ", Comandante."
		}
		return out
	}
	var names []string
	for i, c := range hit {
		if i == omegaMaxSayNames {
			names = append(names, fmt.Sprintf("e mais %d", len(hit)-omegaMaxSayNames))
			break
		}
		s := c.Nome + ", " + c.TipoServico
		if len(hit) <= 2 && c.Contato != "" {
			s += ", " + c.Contato
		}
		names = append(names, s)
	}
	out.Say = "Encontrei " + omegaPlural(len(hit), "profissional", "profissionais")
	if query != "" {
		out.Say += " para " + query
	}
	out.Say += ": " + strings.Join(names, "; ") + "."
	title := "Serviços"
	if query != "" {
		title += " · " + query
	}
	l := &omegaList{Title: title, Rows: []omegaRow{}}
	for i, c := range hit {
		if i == omegaMaxRows {
			l.More = len(hit) - omegaMaxRows
			break
		}
		l.Rows = append(l.Rows, omegaRow{Text: c.Nome + " · " + c.TipoServico, Sub: c.Contato})
	}
	out.Action = &omegaAction{Type: "list", List: l}
	return out
}

// serviceAdd cadastra um profissional; não repete quem já existe com o mesmo nome e tipo.
func (o *omega) serviceAdd(c Contato) omegaOutcome {
	if o.db == nil {
		return omegaNoDB
	}
	all, err := o.contacts()
	if err != nil {
		return omegaDBFail(err)
	}
	for _, x := range all {
		if omegaNormalize(x.Nome) == omegaNormalize(c.Nome) && omegaNormalize(x.TipoServico) == omegaNormalize(c.TipoServico) {
			return omegaOutcome{Say: x.Nome + " já está cadastrado como " + x.TipoServico + ", Comandante.",
				Data: omegaData(map[string]any{"ok": false, "erro": "já cadastrado", "servico": omegaContactData(x)})}
		}
	}
	body, _ := json.Marshal(c)
	v, err := o.db.contatosAPI().create(body)
	if err != nil {
		return omegaDBFail(err)
	}
	saved := v.(Contato)
	return omegaOutcome{Say: "Cadastrei " + saved.Nome + " como " + saved.TipoServico + ", Comandante.", Data: omegaData(map[string]any{"ok": true, "servico": omegaContactData(saved)})}
}

// serviceRemove apaga um profissional (por id ou pelo nome).
func (o *omega) serviceRemove(id int64, nome string) omegaOutcome {
	if o.db == nil {
		return omegaNoDB
	}
	all, err := o.contacts()
	if err != nil {
		return omegaDBFail(err)
	}
	var found []Contato
	for _, c := range all {
		if id > 0 && c.ID == id || id <= 0 && strings.TrimSpace(nome) != "" && omegaWordsIn(nome, c.Nome) {
			found = append(found, c)
		}
	}
	switch {
	case len(found) == 0:
		return omegaFail("não encontrei esse profissional")
	case len(found) > 1:
		var names []string
		var data []map[string]any
		for _, c := range found {
			names = append(names, c.Nome+" ("+c.TipoServico+")")
			data = append(data, omegaContactData(c))
		}
		return omegaOutcome{Say: "Há vários profissionais parecidos, Comandante: " + strings.Join(names, "; ") + ". Qual deles?",
			Data: omegaData(map[string]any{"ok": false, "erro": "mais de um profissional combina; pergunte qual", "servicos": data})}
	}
	c := found[0]
	if err := o.db.contatosAPI().remove(c.ID); err != nil {
		return omegaDBFail(err)
	}
	return omegaOutcome{Say: "Removi " + c.Nome + " dos serviços, Comandante.", Data: omegaData(map[string]any{"ok": true, "removido": omegaContactData(c)})}
}

// ---- ferramentas do LLM ----

var omegaDataToolNames = map[string]bool{"stock_list": true, "stock_expired": true, "stock_add": true, "stock_remove": true,
	"services_search": true, "service_add": true, "service_remove": true}

func omegaHasDataCall(calls []ai.ToolCall) bool {
	for _, c := range calls {
		if omegaDataToolNames[c.Name] {
			return true
		}
	}
	return false
}

func omegaProp(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

func omegaSchema(required []string, props map[string]any) json.RawMessage {
	if required == nil {
		required = []string{}
	}
	b, _ := json.Marshal(map[string]any{"type": "object", "properties": props, "required": required})
	return b
}

// dataTools: as ferramentas de estoque e serviços (nenhuma se o banco não abriu).
func (o *omega) dataTools() []ai.Tool {
	if o.db == nil {
		return nil
	}
	local := map[string]any{"type": "string", "description": "id do local do estoque", "enum": omegaIDs(o.locais())}
	return []ai.Tool{
		{Name: "stock_list", Description: "Consulta o estoque da cozinha (geladeira, freezer, dispensa). Sem filtros lista tudo; com query procura o produto.",
			Parameters: omegaSchema(nil, map[string]any{"query": omegaProp("string", "parte do nome do produto, ex.: leite"), "local": local})},
		{Name: "stock_expired", Description: "Diz o que está vencido no estoque e o que vence nos próximos dias.",
			Parameters: omegaSchema(nil, map[string]any{"days": omegaProp("integer", fmt.Sprintf("considerar 'a vencer' até estes dias (padrão %d)", omegaSoonDays)), "local": local})},
		{Name: "stock_add", Description: "Guarda um item novo no estoque. Informe a quantidade (unidades) e/ou o peso em kg.",
			Parameters: omegaSchema([]string{"nome", "local"}, map[string]any{"nome": omegaProp("string", "nome do produto"), "local": local,
				"quantidade": omegaProp("number", "unidades"), "peso_kg": omegaProp("number", "peso em kg"), "validade": omegaProp("string", "AAAA-MM-DD; omita se não tiver")})},
		{Name: "stock_remove", Description: "Tira um item do estoque. Com quantidade, só baixa essa quantidade (some o item se acabar). Use o id de stock_list ou o nome.",
			Parameters: omegaSchema(nil, map[string]any{"id": omegaProp("integer", "id do item"), "nome": omegaProp("string", "nome do produto, se não souber o id"),
				"quantidade": omegaProp("number", "quanto baixar; omita para remover o item inteiro")})},
		{Name: "services_search", Description: "Procura nos serviços cadastrados (profissionais e prestadores: eletricista, encanador...). Sem query lista todos.",
			Parameters: omegaSchema(nil, map[string]any{"query": omegaProp("string", "tipo de serviço, nome ou assunto, ex.: eletricista, chuveiro")})},
		{Name: "service_add", Description: "Cadastra um profissional nos serviços.",
			Parameters: omegaSchema([]string{"nome", "tipo_servico"}, map[string]any{"nome": omegaProp("string", "nome do profissional"), "tipo_servico": omegaProp("string", "ex.: eletricista"),
				"contato": omegaProp("string", "telefone, e-mail ou site"), "servico": omegaProp("string", "o que ele faz ou fez"), "indicacao": omegaProp("string", "quem indicou"), "obs": omegaProp("string", "observações")})},
		{Name: "service_remove", Description: "Apaga um profissional dos serviços (por id ou nome).",
			Parameters: omegaSchema(nil, map[string]any{"id": omegaProp("integer", "id do profissional"), "nome": omegaProp("string", "nome, se não souber o id")})},
	}
}

// dataPrompt: o trecho do prompt do sistema sobre estoque e serviços.
func (o *omega) dataPrompt() string {
	if o.db == nil {
		return ""
	}
	return "\nVocê também cuida do estoque da cozinha e dos serviços (profissionais cadastrados). " +
		"Use stock_list e stock_expired para consultar o estoque, stock_add e stock_remove para alterá-lo, e services_search, service_add e service_remove para os serviços. " +
		"Só adicione ou remova quando o Comandante pedir, e nunca diga que fez sem chamar a ferramenta. " +
		"Se faltar um dado (local, quantidade ou peso, tipo de serviço), pergunte em vez de inventar; se mais de um item combinar com o que ele quer remover, pergunte qual. " +
		"A validade vai no formato AAAA-MM-DD: converta datas ditas por extenso a partir da data de agora. " +
		"Locais do estoque (id): " + strings.Join(omegaIDs(o.locais()), "; ") + "."
}

// omegaArgs são os argumentos de uma ferramenta; os modelos às vezes mandam número como texto, então os leitores aceitam os dois.
type omegaArgs map[string]any

func (a omegaArgs) str(k string) string {
	switch v := a[k].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return omegaNum(v)
	}
	return ""
}

func (a omegaArgs) num(k string) *float64 {
	switch v := a[k].(type) {
	case float64:
		return &v
	case string:
		if f, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(v), ",", ".", 1), 64); err == nil {
			return &f
		}
	}
	return nil
}

func (a omegaArgs) int(k string) int64 {
	if f := a.num(k); f != nil {
		return int64(*f)
	}
	return 0
}

// runDataTool executa a ferramenta pelo nome. ok=false: o nome não é de estoque nem de serviços.
func (o *omega) runDataTool(name, rawArgs string) (omegaOutcome, bool) {
	if !omegaDataToolNames[name] {
		return omegaOutcome{}, false
	}
	a := omegaArgs{}
	if s := strings.TrimSpace(rawArgs); s != "" && json.Unmarshal([]byte(s), &a) != nil {
		return omegaFail("argumentos inválidos"), true
	}
	switch name {
	case "stock_list":
		return o.stockQuery(a.str("query"), a.str("local")), true
	case "stock_expired":
		return o.stockExpired(int(a.int("days")), a.str("local")), true
	case "stock_add":
		it := Item{Nome: a.str("nome"), Local: a.str("local"), Quantidade: a.num("quantidade"), PesoKg: a.num("peso_kg")}
		if v := a.str("validade"); v != "" {
			it.Validade = &v
		}
		return o.stockAdd(it), true
	case "stock_remove":
		return o.stockRemove(a.int("id"), a.str("nome"), a.num("quantidade")), true
	case "services_search":
		return o.serviceSearch(a.str("query")), true
	case "service_add":
		return o.serviceAdd(Contato{Nome: a.str("nome"), TipoServico: a.str("tipo_servico"), Contato: a.str("contato"),
			Servico: a.str("servico"), Indicacao: a.str("indicacao"), Obs: a.str("obs")}), true
	}
	return o.serviceRemove(a.int("id"), a.str("nome")), true
}

// ---- regras (leitura, sem LLM) ----

// omegaMutations são os verbos que mudam algo: esses pedidos vão ao LLM, nunca às regras de consulta.
var omegaMutations = map[string]bool{}

// omegaDataFiller são palavras de pedido de estoque/serviço que não nomeiam o produto nem o tipo de serviço.
var omegaDataFiller = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`adiciona adicione adicionar acrescenta acrescente acrescentar coloca coloque colocar cadastra cadastre cadastrar
		anota anote anotar compra comprei comprar guarda guarde guardar remove remova remover tira tire tirar apaga apague apagar exclui exclua
		excluir deleta delete deletar joga jogue jogar acabou acabaram usei consumi gastei cancela cancele cancelar`) {
		omegaMutations[w] = true
	}
	for _, w := range strings.Fields(`estoque geladeira freezer congelador dispensa despensa tem tenho temos ha algum alguma alguns algumas algo
		coisa coisas item itens produto produtos guardado guardados guardada guardadas sobrando sobrou resta restam existe existem lista listar
		diz diga fala fale falar saber sabe quero quer gostaria queria preciso precisa confere confira conferir verifica verifique checa checar
		onde cadastrado cadastrados cadastrada cadastradas servicos profissional profissionais prestador prestadores vencido vencida vencidos
		vencidas vencendo vence vencem validade estragado estragados estragando expirado expirados proximo proximos dias dia semana quais
		telefone contato numero quem`) {
		omegaDataFiller[w] = true
	}
}

func omegaHasMutation(words []string) bool {
	for _, w := range words {
		if omegaMutations[w] {
			return true
		}
	}
	return false
}

// omegaQuery junta as palavras do pedido que não são enchimento nem palavras já usadas (o nome do local, por exemplo).
func omegaQuery(words []string, skip map[string]bool) string {
	var q []string
	for _, w := range words {
		if !omegaFiller[w] && !omegaDataFiller[w] && !skip[w] {
			q = append(q, w)
		}
	}
	return strings.Join(q, " ")
}

func (o *omega) dataReply(oc omegaOutcome) omegaReply {
	rep := omegaReply{Say: oc.Say, Actions: []omegaAction{}, Source: "rule"}
	if oc.Action != nil {
		rep.Actions = append(rep.Actions, *oc.Action)
	}
	return rep
}

// dataRules resolve as consultas ao estoque e aos serviços que citam estas palavras de forma explícita. ok=false: segue para as outras regras.
func (o *omega) dataRules(norm string) (omegaReply, bool) {
	words := strings.Fields(norm)
	if o.db == nil || omegaHasMutation(words) || hasWord(words, "camera", "cameras", "cam", "cams", "transito") {
		return omegaReply{}, false
	}
	expiry := hasWord(words, "vencido", "vencida", "vencidos", "vencidas", "vencendo", "vence", "vencem", "validade", "estragado", "estragados", "estragando", "expirado", "expirados")
	local, hasLocal := omegaMatch(norm, words, o.locais())
	skip := map[string]bool{}
	if hasLocal {
		for _, w := range strings.Fields(omegaNormalize(local.name + " " + strings.NewReplacer("-", " ", "_", " ").Replace(local.id))) {
			skip[w] = true
		}
	}
	stock := expiry || hasLocal || hasWord(words, "estoque", "geladeira", "freezer", "congelador", "dispensa", "despensa")
	if stock {
		if expiry {
			return o.dataReply(o.stockExpired(0, local.id)), true
		}
		return o.dataReply(o.stockQuery(omegaQuery(words, skip), local.id)), true
	}
	if hasWord(words, "servicos", "profissional", "profissionais", "prestador", "prestadores") {
		return o.dataReply(o.serviceSearch(omegaQuery(words, nil))), true
	}
	return omegaReply{}, false
}

// serviceTypeRule: o pedido cita um tipo de serviço cadastrado ("preciso de um eletricista"): lista os profissionais desse tipo.
func (o *omega) serviceTypeRule(norm string) (omegaReply, bool) {
	words := strings.Fields(norm)
	if o.db == nil || omegaHasMutation(words) {
		return omegaReply{}, false
	}
	all, err := o.contacts()
	if err != nil {
		return omegaReply{}, false
	}
	types := map[string]bool{}
	for _, c := range all {
		for _, w := range omegaStems(c.TipoServico) {
			if len(w) >= 5 && !omegaFiller[w] && !omegaDataFiller[w] {
				types[w] = true
			}
		}
	}
	var q []string
	for _, w := range words {
		if types[omegaStem(w)] {
			q = append(q, w)
		}
	}
	if len(q) == 0 {
		return omegaReply{}, false
	}
	return o.dataReply(o.serviceSearch(strings.Join(q, " "))), true
}
