package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func newTestAPI(t *testing.T, origins ...string) (*store, http.Handler) {
	t.Helper()
	s, err := openStore(filepath.Join(t.TempDir(), "teste.db"), origins)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	mux := http.NewServeMux()
	s.register(mux)
	return s, mux
}

// do envia um pedido JSON e devolve status e corpo decodificado (mapa, lista ou nil).
func do(t *testing.T, h http.Handler, method, path, body string, hdr ...string) (int, interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var v interface{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatalf("%s %s: resposta não é JSON: %q", method, path, rec.Body.String())
		}
	}
	return rec.Code, v
}

func obj(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("esperava objeto JSON, veio %#v", v)
	}
	return m
}

func TestEstoqueCRUD(t *testing.T) {
	_, h := newTestAPI(t)

	// lista vazia é [] e não null
	if code, v := do(t, h, "GET", "/api/estoque", ""); code != 200 || len(v.([]interface{})) != 0 {
		t.Fatalf("lista vazia: %d %#v", code, v)
	}

	code, v := do(t, h, "POST", "/api/estoque", `{"nome":"  Leite  ","quantidade":2,"validade":"2026-10-05","local":"Geladeira"}`)
	if code != 201 {
		t.Fatalf("criar: %d %#v", code, v)
	}
	leite := obj(t, v)
	id := int64(leite["id"].(float64))
	if leite["nome"] != "Leite" || leite["local"] != "geladeira" || leite["quantidade"].(float64) != 2 || leite["pesoKg"] != nil || leite["validade"] != "2026-10-05" {
		t.Fatalf("item criado inesperado: %#v", leite)
	}

	// peso sem quantidade, sem validade
	if code, v = do(t, h, "POST", "/api/estoque", `{"nome":"Frango","pesoKg":1.5,"local":"freezer"}`); code != 201 {
		t.Fatalf("criar por kg: %d %#v", code, v)
	}
	// sem validade vai para o fim da lista
	if code, v = do(t, h, "POST", "/api/estoque", `{"nome":"Arroz","quantidade":1,"pesoKg":5,"validade":"2026-09-01","local":"dispensa"}`); code != 201 {
		t.Fatalf("criar arroz: %d %#v", code, v)
	}
	code, v = do(t, h, "GET", "/api/estoque", "")
	list := v.([]interface{})
	if code != 200 || len(list) != 3 {
		t.Fatalf("lista: %d %#v", code, v)
	}
	if obj(t, list[0])["nome"] != "Arroz" || obj(t, list[1])["nome"] != "Leite" || obj(t, list[2])["nome"] != "Frango" {
		t.Fatalf("ordem por validade (sem validade por último): %#v", list)
	}

	// atualizar (inclui limpar a validade)
	code, v = do(t, h, "PUT", "/api/estoque/"+itoa(id), `{"nome":"Leite integral","quantidade":1,"validade":"","local":"geladeira"}`)
	up := obj(t, v)
	if code != 200 || up["nome"] != "Leite integral" || up["quantidade"].(float64) != 1 || up["validade"] != nil {
		t.Fatalf("atualizar: %d %#v", code, v)
	}
	if code, v = do(t, h, "GET", "/api/estoque/"+itoa(id), ""); code != 200 || obj(t, v)["nome"] != "Leite integral" {
		t.Fatalf("ler um: %d %#v", code, v)
	}

	// excluir
	if code, _ = do(t, h, "DELETE", "/api/estoque/"+itoa(id), ""); code != 204 {
		t.Fatalf("excluir: %d", code)
	}
	if code, _ = do(t, h, "GET", "/api/estoque/"+itoa(id), ""); code != 404 {
		t.Fatalf("ler excluído deveria ser 404, veio %d", code)
	}
	if code, _ = do(t, h, "DELETE", "/api/estoque/"+itoa(id), ""); code != 404 {
		t.Fatalf("excluir de novo deveria ser 404, veio %d", code)
	}
	if code, _ = do(t, h, "PUT", "/api/estoque/"+itoa(id), `{"nome":"x","quantidade":1,"local":"freezer"}`); code != 404 {
		t.Fatalf("atualizar inexistente deveria ser 404, veio %d", code)
	}
}

func TestEstoqueValidacao(t *testing.T) {
	_, h := newTestAPI(t)
	ruins := map[string]string{
		"sem nome":            `{"quantidade":1,"local":"freezer"}`,
		"nome em branco":      `{"nome":"   ","quantidade":1,"local":"freezer"}`,
		"nome longo":          `{"nome":"` + strings.Repeat("a", 121) + `","quantidade":1,"local":"freezer"}`,
		"local inválido":      `{"nome":"x","quantidade":1,"local":"varanda"}`,
		"sem local":           `{"nome":"x","quantidade":1}`,
		"sem quantidade e kg": `{"nome":"x","local":"freezer"}`,
		"quantidade negativa": `{"nome":"x","quantidade":-1,"local":"freezer"}`,
		"peso negativo":       `{"nome":"x","pesoKg":-0.5,"local":"freezer"}`,
		"quantidade enorme":   `{"nome":"x","quantidade":1e9,"local":"freezer"}`,
		"validade inválida":   `{"nome":"x","quantidade":1,"validade":"31/12/2026","local":"freezer"}`,
		"data impossível":     `{"nome":"x","quantidade":1,"validade":"2026-02-30","local":"freezer"}`,
		"json quebrado":       `{"nome":`,
		"tipo errado":         `{"nome":"x","quantidade":"muitos","local":"freezer"}`,
	}
	for nome, body := range ruins {
		code, v := do(t, h, "POST", "/api/estoque", body)
		if code != 400 {
			t.Errorf("%s: esperava 400, veio %d", nome, code)
			continue
		}
		if msg, _ := obj(t, v)["error"].(string); msg == "" {
			t.Errorf("%s: 400 sem mensagem de erro", nome)
		}
	}
	// zero é válido (acabou, mas o registro fica)
	if code, v := do(t, h, "POST", "/api/estoque", `{"nome":"Ovos","quantidade":0,"local":"geladeira"}`); code != 201 {
		t.Errorf("quantidade 0 deveria ser aceita: %d %#v", code, v)
	}
}

func TestContatosCRUD(t *testing.T) {
	_, h := newTestAPI(t)
	code, v := do(t, h, "POST", "/api/contatos", `{"nome":"João Silva","tipoServico":"Eletricista","contato":"(11) 98888-7777","indicacao":"Indicado pela Ana","servico":"Trocar o quadro de luz"}`)
	if code != 201 {
		t.Fatalf("criar: %d %#v", code, v)
	}
	c := obj(t, v)
	id := itoa(int64(c["id"].(float64)))
	if c["nome"] != "João Silva" || c["tipoServico"] != "Eletricista" || c["indicacao"] != "Indicado pela Ana" || c["servico"] != "Trocar o quadro de luz" {
		t.Fatalf("contato criado inesperado: %#v", c)
	}
	// contato, indicação e serviço são opcionais
	if code, v = do(t, h, "POST", "/api/contatos", `{"nome":"Ana","tipoServico":"encanadora","contato":"ana@exemplo.com"}`); code != 201 {
		t.Fatalf("criar mínimo: %d %#v", code, v)
	}
	code, v = do(t, h, "GET", "/api/contatos", "")
	list := v.([]interface{})
	if code != 200 || len(list) != 2 || obj(t, list[0])["nome"] != "João Silva" { // ordena por tipo: Eletricista < encanadora
		t.Fatalf("lista: %d %#v", code, v)
	}
	code, v = do(t, h, "PUT", "/api/contatos/"+id, `{"nome":"João S.","tipoServico":"Eletricista","contato":"11988887777","indicacao":"","servico":""}`)
	if code != 200 || obj(t, v)["nome"] != "João S." || obj(t, v)["servico"] != "" {
		t.Fatalf("atualizar: %d %#v", code, v)
	}
	if code, _ = do(t, h, "DELETE", "/api/contatos/"+id, ""); code != 204 {
		t.Fatalf("excluir: %d", code)
	}
	if code, _ = do(t, h, "GET", "/api/contatos/"+id, ""); code != 404 {
		t.Fatalf("excluído deveria dar 404, veio %d", code)
	}
}

func TestContatosValidacao(t *testing.T) {
	_, h := newTestAPI(t)
	ruins := map[string]string{
		"sem nome":        `{"tipoServico":"x"}`,
		"sem tipo":        `{"nome":"x"}`,
		"serviço longo":   `{"nome":"x","tipoServico":"x","servico":"` + strings.Repeat("n", 2001) + `"}`,
		"indicação longa": `{"nome":"x","tipoServico":"x","indicacao":"` + strings.Repeat("n", 501) + `"}`,
	}
	for nome, body := range ruins {
		if code, _ := do(t, h, "POST", "/api/contatos", body); code != 400 {
			t.Errorf("%s: esperava 400, veio %d", nome, code)
		}
	}
}

func TestSQLInjectionEhTextoComum(t *testing.T) {
	_, h := newTestAPI(t)
	nome := `Robert'); DROP TABLE itens;--`
	body, _ := json.Marshal(map[string]interface{}{"nome": nome, "quantidade": 1, "local": "dispensa"})
	if code, _ := do(t, h, "POST", "/api/estoque", string(body)); code != 201 {
		t.Fatalf("criar: %d", code)
	}
	code, v := do(t, h, "GET", "/api/estoque", "")
	list := v.([]interface{})
	if code != 200 || len(list) != 1 || obj(t, list[0])["nome"] != nome {
		t.Fatalf("o nome deveria ser gravado como texto: %d %#v", code, v)
	}
}

func TestProtecaoDeOrigem(t *testing.T) {
	_, h := newTestAPI(t, "http://localhost:5173/")
	novo := `{"nome":"x","quantidade":1,"local":"freezer"}`

	// httptest usa Host "example.com"
	casos := []struct {
		nome, origin string
		want         int
	}{
		{"sem Origin (curl, scripts)", "", 201},
		{"mesma origem do hub", "http://example.com", 201},
		{"origem listada em allowedOrigins", "http://localhost:5173", 201},
		{"site de fora", "http://malicioso.example", 403},
		{"origin null (iframe isolado, file://)", "null", 403},
		{"mesmo host, outra porta", "http://example.com:8080", 403},
	}
	for _, c := range casos {
		hdr := []string{}
		if c.origin != "" {
			hdr = []string{"Origin", c.origin}
		}
		if code, _ := do(t, h, "POST", "/api/estoque", novo, hdr...); code != c.want {
			t.Errorf("POST %s: esperava %d, veio %d", c.nome, c.want, code)
		}
	}

	// o corpo precisa ser JSON (um <form> de outro site não consegue)
	req := httptest.NewRequest("POST", "/api/estoque", strings.NewReader(novo))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 415 {
		t.Errorf("text/plain: esperava 415, veio %d", rec.Code)
	}

	// PUT/DELETE de fora também caem; leitura continua liberada
	if code, _ := do(t, h, "DELETE", "/api/estoque/1", "", "Origin", "http://malicioso.example"); code != 403 {
		t.Errorf("DELETE de fora: esperava 403, veio %d", code)
	}
	if code, _ := do(t, h, "PUT", "/api/estoque/1", novo, "Origin", "http://malicioso.example"); code != 403 {
		t.Errorf("PUT de fora: esperava 403, veio %d", code)
	}
	if code, _ := do(t, h, "GET", "/api/estoque", "", "Origin", "http://malicioso.example"); code != 200 {
		t.Errorf("GET com outra origem: esperava 200, veio %d", code)
	}
}

func TestMetodosEIds(t *testing.T) {
	_, h := newTestAPI(t)
	for _, c := range []struct {
		method, path string
		want         int
	}{
		{"PATCH", "/api/estoque/1", 405},
		{"POST", "/api/estoque/1", 405}, // criar não aceita id
		{"PUT", "/api/estoque", 405},    // atualizar exige id
		{"DELETE", "/api/estoque", 405},
		{"GET", "/api/estoque/abc", 400},
		{"GET", "/api/estoque/0", 400},
		{"GET", "/api/estoque/-3", 400},
		{"GET", "/api/estoque/999", 404},
		{"GET", "/api/contatos/999", 404},
	} {
		if code, _ := do(t, h, c.method, c.path, ""); code != c.want {
			t.Errorf("%s %s: esperava %d, veio %d", c.method, c.path, c.want, code)
		}
	}
}

func TestPersisteEntreAberturas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.db")
	s, err := openStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.register(mux)
	if code, _ := do(t, mux, "POST", "/api/contatos", `{"nome":"Zé","tipoServico":"Chaveiro","contato":"1144445555"}`); code != 201 {
		t.Fatalf("criar: %d", code)
	}
	s.Close()

	// reabrir roda as migrações de novo sem perder nem duplicar nada
	s2, err := openStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	mux2 := http.NewServeMux()
	s2.register(mux2)
	code, v := do(t, mux2, "GET", "/api/contatos", "")
	if list := v.([]interface{}); code != 200 || len(list) != 1 || obj(t, list[0])["nome"] != "Zé" {
		t.Fatalf("dados perdidos ao reabrir: %d %#v", code, v)
	}
}

func TestBancoIndisponivelResponde503(t *testing.T) {
	var s *store // openStore falhou no main
	mux := http.NewServeMux()
	s.register(mux)
	for _, p := range []string{"/api/estoque", "/api/contatos"} {
		code, v := do(t, mux, "GET", p, "")
		if code != 503 || obj(t, v)["error"] == "" {
			t.Errorf("%s: esperava 503 com mensagem, veio %d %#v", p, code, v)
		}
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// Banco que veio do DVRCentral: tem as tabelas do Diogows e user_version 11 (mais migrações do que este hub conhece).
func TestBancoDoDVRCentralPerdeTabelasDoDiogows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "antigo.db")
	s, err := openStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO itens (nome, quantidade, local) VALUES ('Leite', 2, 'geladeira');
		CREATE TABLE desk_users (id INTEGER PRIMARY KEY, nome TEXT);
		CREATE TABLE desk_sessions (id INTEGER PRIMARY KEY);
		CREATE TABLE desk_home (id INTEGER PRIMARY KEY);
		PRAGMA user_version = 11;`); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = openStore(path, nil)
	if err != nil {
		t.Fatalf("banco do DVRCentral deveria abrir: %v", err)
	}
	defer s.Close()
	var desk, itens, v int
	s.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND substr(name, 1, 5) = 'desk_'").Scan(&desk)
	s.db.QueryRow("SELECT count(*) FROM itens").Scan(&itens)
	s.db.QueryRow("PRAGMA user_version").Scan(&v)
	if desk != 0 || itens != 1 || v != len(migrations) {
		t.Fatalf("esperava 0 tabelas desk_*, 1 item e versão %d; veio %d, %d e %d", len(migrations), desk, itens, v)
	}
}

// Banco de uma versão mais nova do hub (sem as tabelas do Diogows): não é migrado para trás.
func TestBancoMaisNovoQueOHubEhRecusado(t *testing.T) {
	path := filepath.Join(t.TempDir(), "novo.db")
	s, err := openStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.db.Exec("PRAGMA user_version = 99")
	s.Close()
	if s, err := openStore(path, nil); err == nil {
		s.Close()
		t.Fatal("esperava erro para banco mais novo que o hub")
	}
}
