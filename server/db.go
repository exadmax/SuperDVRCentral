// Banco SQLite do hub: estoque da cozinha (geladeira, freezer, dispensa) e contatos importantes.
// Driver em Go puro (modernc.org/sqlite), sem CGO: o hub continua compilando para todas as arquiteturas Linux.
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// Cada entrada roda uma vez, em ordem (PRAGMA user_version guarda quantas já foram aplicadas).
// Para mudar o esquema depois, acrescente uma migração nova; nunca edite as antigas.
var migrations = []string{
	`CREATE TABLE itens (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT NOT NULL,
		quantidade REAL CHECK (quantidade IS NULL OR quantidade >= 0),
		peso_kg REAL CHECK (peso_kg IS NULL OR peso_kg >= 0),
		validade TEXT,
		local TEXT NOT NULL CHECK (local IN ('freezer', 'geladeira', 'dispensa')),
		criado_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		atualizado_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		CHECK (quantidade IS NOT NULL OR peso_kg IS NOT NULL)
	);
	CREATE INDEX itens_validade ON itens (validade);
	CREATE TABLE contatos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT NOT NULL,
		tipo_servico TEXT NOT NULL,
		telefone TEXT NOT NULL,
		endereco TEXT NOT NULL DEFAULT '',
		nota TEXT NOT NULL DEFAULT '',
		criado_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		atualizado_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	);`,
	// Contatos viram profissionais: telefone -> contato (telefone, e-mail ou site), nota -> servico (o que será feito),
	// novo campo indicacao (quem indicou / observações); o endereço, que ninguém usava, vai junto para o serviço.
	`ALTER TABLE contatos RENAME COLUMN telefone TO contato;
	ALTER TABLE contatos ADD COLUMN indicacao TEXT NOT NULL DEFAULT '';
	ALTER TABLE contatos ADD COLUMN servico TEXT NOT NULL DEFAULT '';
	UPDATE contatos SET servico = trim(nota || CASE WHEN endereco <> '' THEN char(10) || 'Endereço: ' || endereco ELSE '' END);
	ALTER TABLE contatos DROP COLUMN endereco;
	ALTER TABLE contatos DROP COLUMN nota;`,
	// Observações do profissional (coluna "Obs" da planilha).
	`ALTER TABLE contatos ADD COLUMN obs TEXT NOT NULL DEFAULT '';`,
	// Os locais do estoque deixam de ser fixos (freezer/geladeira/dispensa): sai o CHECK da coluna local; quem valida agora é o hub.
	`CREATE TABLE itens_novo (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT NOT NULL,
		quantidade REAL CHECK (quantidade IS NULL OR quantidade >= 0),
		peso_kg REAL CHECK (peso_kg IS NULL OR peso_kg >= 0),
		validade TEXT,
		local TEXT NOT NULL,
		criado_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		atualizado_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		CHECK (quantidade IS NOT NULL OR peso_kg IS NOT NULL)
	);
	INSERT INTO itens_novo (id, nome, quantidade, peso_kg, validade, local, criado_em, atualizado_em)
		SELECT id, nome, quantidade, peso_kg, validade, local, criado_em, atualizado_em FROM itens;
	DROP TABLE itens;
	ALTER TABLE itens_novo RENAME TO itens;
	CREATE INDEX itens_validade ON itens (validade);`,
}

type store struct {
	db      *sql.DB         // única conexão, para gravar (e para o que precisa ler o que acabou de gravar numa transação)
	rdb     *sql.DB         // até readConns conexões só de leitura: no modo WAL elas não esperam a gravação (nem a gravação espera por elas)
	locais  func() []string // ids dos locais do estoque em uso (nil = os padrão)
	origins []string        // origens extras com permissão de gravar (além do próprio endereço do hub)
}

const readConns = 3

func openStore(path string, origins []string) (*store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // uso doméstico: uma conexão basta para gravar, serializa as gravações e poupa memória
	s := &store{db: db, rdb: db, origins: origins}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if path != "" && !strings.Contains(path, ":memory:") { // banco em memória é por conexão: nele a leitura usa a mesma conexão
		rdb, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=query_only(1)")
		if err != nil {
			db.Close()
			return nil, err
		}
		rdb.SetMaxOpenConns(readConns)
		s.rdb = rdb
	}
	return s, nil
}

// Close encerra as conexões de gravação e de leitura.
func (s *store) Close() error {
	err := s.db.Close()
	if s.rdb != s.db {
		if e := s.rdb.Close(); err == nil {
			err = e
		}
	}
	return err
}

func (s *store) migrate() error {
	var v int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	for i := v; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migração %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// register expõe /api/estoque e /api/contatos. Funciona com s == nil (responde 503 em vez de derrubar o hub).
func (s *store) register(mux *http.ServeMux) {
	mux.HandleFunc("/api/estoque", s.handler("/api/estoque", s.itensAPI()))
	mux.HandleFunc("/api/estoque/", s.handler("/api/estoque", s.itensAPI()))
	mux.HandleFunc("/api/contatos", s.handler("/api/contatos", s.contatosAPI()))
	mux.HandleFunc("/api/contatos/", s.handler("/api/contatos", s.contatosAPI()))
}

// ---------- CRUD genérico sobre JSON ----------

// apiError vira resposta HTTP com a mensagem em JSON ({"error": "..."}) para o painel mostrar.
type apiError struct {
	status int
	msg    string
}

func (e *apiError) Error() string { return e.msg }

func badRequest(format string, a ...interface{}) error {
	return &apiError{http.StatusBadRequest, fmt.Sprintf(format, a...)}
}

var errNotFound = &apiError{http.StatusNotFound, "registro não encontrado"}

type crud struct {
	list   func() (interface{}, error)
	get    func(id int64) (interface{}, error)
	create func(body []byte) (interface{}, error)
	update func(id int64, body []byte) (interface{}, error)
	remove func(id int64) error
}

func writeStatus(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var ae *apiError
	if !errors.As(err, &ae) {
		log.Printf("erro no banco de dados: %v", err)
		ae = &apiError{http.StatusInternalServerError, "erro interno ao acessar o banco de dados"}
	}
	writeStatus(w, ae.status, map[string]string{"error": ae.msg})
}

func (s *store) handler(prefix string, c crud) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil {
			writeError(w, &apiError{http.StatusServiceUnavailable, "banco de dados indisponível (veja o log do hub)"})
			return
		}
		var id int64
		rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
		hasID := rest != ""
		if hasID {
			var err error
			if id, err = strconv.ParseInt(rest, 10, 64); err != nil || id <= 0 {
				writeError(w, badRequest("id inválido"))
				return
			}
		}
		var body []byte
		switch {
		case r.Method == http.MethodGet:
		case (r.Method == http.MethodPost && !hasID) || (r.Method == http.MethodPut && hasID) || (r.Method == http.MethodDelete && hasID):
			if err := s.checkWrite(r); err != nil {
				writeError(w, err)
				return
			}
			if r.Method != http.MethodDelete {
				var err error
				if body, err = io.ReadAll(io.LimitReader(r.Body, 1<<16)); err != nil {
					writeError(w, badRequest("não foi possível ler o pedido"))
					return
				}
			}
		default:
			w.Header().Set("Allow", "GET, POST, PUT, DELETE")
			writeError(w, &apiError{http.StatusMethodNotAllowed, "método não permitido"})
			return
		}

		var (
			v      interface{}
			err    error
			status = http.StatusOK
		)
		switch {
		case r.Method == http.MethodGet && !hasID:
			v, err = c.list()
		case r.Method == http.MethodGet:
			v, err = c.get(id)
		case r.Method == http.MethodPost:
			v, err = c.create(body)
			status = http.StatusCreated
		case r.Method == http.MethodPut:
			v, err = c.update(id, body)
		default:
			err = c.remove(id)
			status = http.StatusNoContent
		}
		if err != nil {
			writeError(w, err)
			return
		}
		writeStatus(w, status, v)
	}
}

// checkWrite protege as gravações de páginas de outros sites abertas no navegador de alguém da casa:
// exige JSON (um <form> comum não consegue enviar) e, se o navegador informar a origem, que seja o próprio
// hub ou uma origem listada em allowedOrigins. Clientes sem cabeçalho Origin (curl, scripts) passam.
func (s *store) checkWrite(r *http.Request) error {
	if ct := r.Header.Get("Content-Type"); r.Method != http.MethodDelete && !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		return &apiError{http.StatusUnsupportedMediaType, "envie o corpo como application/json"}
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	if u, err := url.Parse(origin); err == nil && strings.EqualFold(u.Host, r.Host) {
		return nil
	}
	for _, o := range s.origins {
		if strings.EqualFold(strings.TrimRight(o, "/"), origin) {
			return nil
		}
	}
	log.Printf("gravação recusada: Origin %q diferente de Host %q (para liberar, use allowedOrigins em hub.config.json)", origin, r.Host)
	return &apiError{http.StatusForbidden, "origem não permitida para gravar"}
}
