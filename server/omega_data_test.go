package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"dvr/hub/ai"
)

// testOmegaDB é o testOmega com um banco de verdade, com o estoque e os serviços de exemplo. Hoje é 2026-11-04.
func testOmegaDB(t *testing.T, aiCfg ai.Config) (*omega, *http.ServeMux) {
	t.Helper()
	o, mux := testOmega(t, aiCfg)
	s, _ := newTestAPI(t)
	o.db = s
	for _, b := range []string{
		`{"nome":"Leite","quantidade":3,"validade":"2026-11-02","local":"geladeira"}`,
		`{"nome":"Iogurte","quantidade":6,"validade":"2026-11-06","local":"geladeira"}`,
		`{"nome":"Frango","pesoKg":1.5,"validade":"2027-02-01","local":"freezer"}`,
		`{"nome":"Arroz","quantidade":2,"local":"dispensa"}`,
		`{"nome":"Ovos","quantidade":12,"validade":"2026-12-20","local":"geladeira"}`,
		`{"nome":"Ovos de codorna","quantidade":30,"validade":"2026-12-20","local":"dispensa"}`,
	} {
		if _, err := s.itensAPI().create([]byte(b)); err != nil {
			t.Fatal(err)
		}
	}
	for _, b := range []string{
		`{"nome":"João Silva","tipoServico":"Eletricista","contato":"11 99999-0001","servico":"Trocou o chuveiro"}`,
		`{"nome":"Marcos","tipoServico":"Encanador","contato":"11 99999-0002"}`,
		`{"nome":"Pedro","tipoServico":"Eletricista","contato":"11 99999-0003"}`,
	} {
		if _, err := s.contatosAPI().create([]byte(b)); err != nil {
			t.Fatal(err)
		}
	}
	return o, mux
}

func TestOmegaEstoqueVencidos(t *testing.T) {
	_, mux := testOmegaDB(t, ai.Config{Provider: "off"})
	rep := ask(t, mux, "Omega, tem algo vencido no estoque?")
	if rep.Source != "rule" || !strings.Contains(rep.Say, "1 item vencido") || !strings.Contains(rep.Say, "Leite (vencido há 2 dias)") ||
		!strings.Contains(rep.Say, "1 vence") || len(rep.Actions) != 1 || rep.Actions[0].Type != "list" {
		t.Fatalf("%+v", rep)
	}
	rows := rep.Actions[0].List.Rows
	if len(rows) != 2 || !rows[0].Warn || rows[0].Text != "Leite" || rows[1].Warn || rows[1].Text != "Iogurte" {
		t.Fatalf("linhas: %+v", rows)
	}
	// só no freezer não há nada vencido nem perto de vencer
	rep = ask(t, mux, "Omega, o que está vencendo no freezer?")
	if !strings.HasPrefix(rep.Say, "Nenhum item vencido no estoque em freezer") || len(rep.Actions) != 0 {
		t.Fatalf("freezer: %+v", rep)
	}
}

func TestOmegaEstoqueConsulta(t *testing.T) {
	_, mux := testOmegaDB(t, ai.Config{Provider: "off"})
	rep := ask(t, mux, "Omega, o que tenho no estoque?") // não é agenda
	if rep.Source != "rule" || len(rep.Actions) != 1 || rep.Actions[0].Type != "list" || len(rep.Actions[0].List.Rows) != 6 || !strings.Contains(rep.Say, "6 itens") {
		t.Fatalf("tudo: %+v", rep)
	}
	rep = ask(t, mux, "Omega, o que tem na geladeira?")
	if !strings.Contains(rep.Say, "3 itens em geladeira") || strings.Contains(rep.Say, "Frango") {
		t.Fatalf("geladeira: %q", rep.Say)
	}
	rep = ask(t, mux, "Omega, tem ovo no estoque?") // singular casa com "Ovos"; "Ovos de codorna" também tem a palavra
	if !strings.Contains(rep.Say, "2 itens") || !strings.Contains(rep.Say, "Ovos, 12 un, geladeira") {
		t.Fatalf("ovo: %q", rep.Say)
	}
	rep = ask(t, mux, "Omega, tem frango na geladeira?")
	if !strings.HasPrefix(rep.Say, "Não encontrei frango no estoque em geladeira") || len(rep.Actions) != 0 {
		t.Fatalf("frango: %+v", rep)
	}
	rep = ask(t, mux, "Omega, quanto de frango tem no freezer?")
	if !strings.Contains(rep.Say, "Frango, 1,5 kg") {
		t.Fatalf("peso: %q", rep.Say)
	}
}

func TestOmegaServicosRegras(t *testing.T) {
	_, mux := testOmegaDB(t, ai.Config{Provider: "off"})
	rep := ask(t, mux, "Omega, quais serviços eu tenho?")
	if rep.Source != "rule" || !strings.Contains(rep.Say, "3 profissionais") || len(rep.Actions) != 1 || len(rep.Actions[0].List.Rows) != 3 {
		t.Fatalf("todos: %+v", rep)
	}
	rep = ask(t, mux, "Omega, tem algum eletricista nos serviços?")
	if !strings.Contains(rep.Say, "2 profissionais para eletricista") || strings.Contains(rep.Say, "Marcos") {
		t.Fatalf("eletricista: %q", rep.Say)
	}
	rep = ask(t, mux, "Omega, preciso de um encanador") // sem a palavra "serviços": o tipo cadastrado basta
	if rep.Source != "rule" || !strings.Contains(rep.Say, "Marcos, Encanador, 11 99999-0002") {
		t.Fatalf("encanador: %+v", rep)
	}
	rep = ask(t, mux, "Omega, quem trocou o chuveiro nos serviços?")
	if !strings.Contains(rep.Say, "João Silva") {
		t.Fatalf("busca no texto do serviço: %q", rep.Say)
	}
	rep = ask(t, mux, "Omega, tem pintor nos serviços?")
	if !strings.HasPrefix(rep.Say, "Não encontrei nenhum profissional para pintor") {
		t.Fatalf("pintor: %q", rep.Say)
	}
}

func TestOmegaAlterarVaiAoLLM(t *testing.T) {
	_, mux := testOmegaDB(t, ai.Config{Provider: "off"})
	for _, text := range []string{"Omega, adiciona leite na geladeira", "Omega, remove o iogurte do estoque", "Omega, apaga o eletricista João dos serviços"} {
		if rep := ask(t, mux, text); rep.Source != "none" { // provedor desligado: não virou consulta
			t.Errorf("%q: %+v", text, rep)
		}
	}
}

// seqProvider responde com cada resposta da lista, na ordem, e guarda os corpos recebidos.
func seqProvider(t *testing.T, replies ...string) (*httptest.Server, func() []map[string]any) {
	t.Helper()
	var mu sync.Mutex
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		json.Unmarshal(b, &m)
		mu.Lock()
		n := len(got)
		got = append(got, m)
		mu.Unlock()
		if n >= len(replies) {
			n = len(replies) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, replies[n])
	}))
	t.Cleanup(srv.Close)
	return srv, func() []map[string]any { mu.Lock(); defer mu.Unlock(); return append([]map[string]any(nil), got...) }
}

func TestOmegaLLMAdicionaEstoque(t *testing.T) {
	srv, reqs := seqProvider(t,
		`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"stock_add","arguments":"{\"nome\":\"Queijo\",\"local\":\"geladeira\",\"quantidade\":2,\"validade\":\"2026-12-10\"}"}}]}}]}`,
		`{"choices":[{"message":{"content":"Pronto, Comandante, guardei o queijo."}}]}`)
	o, mux := testOmegaDB(t, ai.Config{Provider: "ollama", OllamaURL: srv.URL, Model: "qwen3:8b"})
	rep := ask(t, mux, "Omega, guarda 2 queijos na geladeira, vencem dia 10 de dezembro")
	if rep.Source != "llm" || rep.Say != "Pronto, Comandante, guardei o queijo." {
		t.Fatalf("%+v", rep)
	}
	items, _ := o.items()
	var found *Item
	for i := range items {
		if items[i].Nome == "Queijo" {
			found = &items[i]
		}
	}
	if found == nil || found.Local != "geladeira" || found.Quantidade == nil || *found.Quantidade != 2 || found.Validade == nil || *found.Validade != "2026-12-10" {
		t.Fatalf("item gravado: %+v", found)
	}
	got := reqs()
	if len(got) != 2 {
		t.Fatalf("chamadas ao LLM: %d", len(got))
	}
	msgs := got[1]["messages"].([]any) // 2º pedido: system, usuário, assistant com a chamada, resposta da ferramenta
	if len(msgs) != 4 {
		t.Fatalf("mensagens: %d", len(msgs))
	}
	if sys := msgs[0].(map[string]any)["content"].(string); !strings.Contains(sys, "stock_add") || !strings.Contains(sys, "geladeira; freezer; dispensa") {
		t.Errorf("prompt: %s", sys)
	}
	tool := msgs[3].(map[string]any)
	if tool["role"] != "tool" || tool["tool_call_id"] != "c1" || !strings.Contains(tool["content"].(string), `"ok":true`) {
		t.Errorf("resposta da ferramenta: %v", tool)
	}
	if raw, _ := json.Marshal(got[0]["tools"]); !strings.Contains(string(raw), "stock_remove") || !strings.Contains(string(raw), "services_search") {
		t.Errorf("ferramentas: %s", raw)
	}
}

func TestOmegaLLMErroDeValidacaoVoltaAoModelo(t *testing.T) {
	srv, reqs := seqProvider(t,
		`{"choices":[{"message":{"tool_calls":[{"id":"c1","type":"function","function":{"name":"stock_add","arguments":"{\"nome\":\"Queijo\",\"local\":\"geladeira\"}"}}]}}]}`,
		`{"choices":[{"message":{"content":"Quantas unidades, Comandante?"}}]}`)
	o, mux := testOmegaDB(t, ai.Config{Provider: "ollama", OllamaURL: srv.URL, Model: "qwen3:8b"})
	before, _ := o.items()
	rep := ask(t, mux, "Omega, guarda queijo na geladeira")
	if rep.Say != "Quantas unidades, Comandante?" {
		t.Fatalf("%+v", rep)
	}
	if after, _ := o.items(); len(after) != len(before) {
		t.Fatal("gravou item sem quantidade nem peso")
	}
	msgs := reqs()[1]["messages"].([]any)
	if c := msgs[len(msgs)-1].(map[string]any)["content"].(string); !strings.Contains(c, "quantidade") || !strings.Contains(c, `"ok":false`) {
		t.Errorf("o modelo precisa ver o erro: %s", c)
	}
}

func TestOmegaLLMRemoveEServicos(t *testing.T) {
	srv, _ := seqProvider(t,
		`{"choices":[{"message":{"tool_calls":[
			{"id":"c1","type":"function","function":{"name":"stock_remove","arguments":"{\"nome\":\"iogurte\"}"}},
			{"id":"c2","type":"function","function":{"name":"service_remove","arguments":"{\"nome\":\"marcos\"}"}},
			{"id":"c3","type":"function","function":{"name":"service_add","arguments":"{\"nome\":\"Ana\",\"tipo_servico\":\"Diarista\",\"contato\":\"11 98888-0000\"}"}}]}}]}`,
		`{"choices":[{"message":{"content":""}}]}`) // modelo mudo no fim: a frase vem do que as ferramentas fizeram
	o, mux := testOmegaDB(t, ai.Config{Provider: "ollama", OllamaURL: srv.URL, Model: "qwen3:8b"})
	rep := ask(t, mux, "Omega, tira o iogurte, apaga o Marcos e cadastra a Ana diarista")
	for _, s := range []string{"Removi Iogurte do estoque", "Removi Marcos dos serviços", "Cadastrei Ana como Diarista"} {
		if !strings.Contains(rep.Say, s) {
			t.Errorf("sem %q: %q", s, rep.Say)
		}
	}
	items, _ := o.items()
	if len(items) != 5 {
		t.Errorf("itens: %d", len(items))
	}
	cs, _ := o.contacts()
	if len(cs) != 3 || cs[0].Nome != "Ana" && cs[1].Nome != "Ana" && cs[2].Nome != "Ana" {
		t.Errorf("contatos: %+v", cs)
	}
}

func TestOmegaEstoqueBaixarQuantidade(t *testing.T) {
	o, _ := testOmegaDB(t, ai.Config{Provider: "off"})
	run := func(name, args string) omegaOutcome {
		t.Helper()
		oc, ok := o.runDataTool(name, args)
		if !ok {
			t.Fatalf("%s: ferramenta desconhecida", name)
		}
		return oc
	}
	if oc := run("stock_remove", `{"nome":"leite","quantidade":1}`); !strings.Contains(oc.Say, "restam 2") {
		t.Errorf("baixa parcial: %q", oc.Say)
	}
	if oc := run("stock_remove", `{"nome":"leite","quantidade":"5"}`); !strings.Contains(oc.Say, "Removi Leite") { // pediu mais do que há: tira o item
		t.Errorf("baixa total: %q", oc.Say)
	}
	if oc := run("stock_remove", `{"nome":"ovos"}`); !strings.Contains(oc.Say, "vários") { // "Ovos" e "Ovos de codorna"
		t.Errorf("ambíguo: %q", oc.Say)
	}
	if oc := run("stock_remove", `{"nome":"frango","quantidade":1}`); !strings.Contains(oc.Say, "peso") {
		t.Errorf("item por peso: %q", oc.Say)
	}
	if oc := run("stock_remove", `{"nome":"pizza"}`); !strings.Contains(oc.Say, "não encontrei") {
		t.Errorf("inexistente: %q", oc.Say)
	}
	if oc := run("service_add", `{"nome":"joão silva","tipo_servico":"eletricista"}`); !strings.Contains(oc.Say, "já está cadastrado") {
		t.Errorf("duplicado: %q", oc.Say)
	}
	if oc := run("stock_add", `{"nome":"Sal","local":"garagem","quantidade":1}`); !strings.Contains(oc.Say, "local inválido") {
		t.Errorf("local inválido: %q", oc.Say)
	}
	if oc := run("stock_list", `not json`); !strings.Contains(oc.Say, "argumentos inválidos") {
		t.Errorf("json ruim: %q", oc.Say)
	}
	if _, ok := o.runDataTool("show_camera", `{}`); ok {
		t.Error("show_camera não é ferramenta de dados")
	}
}

func TestOmegaSemBanco(t *testing.T) {
	o, mux := testOmega(t, ai.Config{Provider: "off"})
	if o.dataTools() != nil || o.dataPrompt() != "" {
		t.Fatal("sem banco não há ferramentas de estoque")
	}
	// "estoque" sem banco cai no caminho normal (aqui, sem IA)
	if rep := ask(t, mux, "Omega, o que tem no estoque?"); rep.Source == "rule" && len(rep.Actions) != 0 {
		t.Fatalf("%+v", rep)
	}
	if oc := o.stockQuery("", ""); !strings.Contains(oc.Say, "indisponível") {
		t.Errorf("%q", oc.Say)
	}
}
