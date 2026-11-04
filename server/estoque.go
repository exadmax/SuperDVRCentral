// Estoque da cozinha: itens da geladeira, do freezer e da dispensa (CRUD em /api/estoque).
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Item guarda quantidade (unidades) e/ou peso em kg: pelo menos um dos dois é obrigatório.
type Item struct {
	ID           int64    `json:"id"`
	Nome         string   `json:"nome"`
	Quantidade   *float64 `json:"quantidade"`
	PesoKg       *float64 `json:"pesoKg"`
	Validade     *string  `json:"validade"` // AAAA-MM-DD; nulo = sem validade
	Local        string   `json:"local"`    // freezer | geladeira | dispensa
	CriadoEm     string   `json:"criadoEm"`
	AtualizadoEm string   `json:"atualizadoEm"`
}

const itemCols = "id, nome, quantidade, peso_kg, validade, local, criado_em, atualizado_em"

// Locais padrão; a página Config pode trocar a lista (settings.go) e o store passa a consultar a lista atual.
var defaultLocais = []string{"geladeira", "freezer", "dispensa"}

func (s *store) localOK(id string) bool {
	list := defaultLocais
	if s.locais != nil {
		list = s.locais()
	}
	for _, l := range list {
		if l == id {
			return true
		}
	}
	return false
}

func (it *Item) clean(localOK func(string) bool) error {
	it.Nome = strings.TrimSpace(it.Nome)
	if it.Nome == "" {
		return badRequest("informe o nome do item")
	}
	if utf8.RuneCountInString(it.Nome) > 120 {
		return badRequest("nome muito longo (máximo 120 caracteres)")
	}
	it.Local = strings.ToLower(strings.TrimSpace(it.Local))
	if !localOK(it.Local) {
		return badRequest("local inválido (veja os locais cadastrados em Config → Painel)")
	}
	if it.Quantidade == nil && it.PesoKg == nil {
		return badRequest("informe a quantidade e/ou o peso em kg")
	}
	for _, n := range []struct {
		v    *float64
		nome string
	}{{it.Quantidade, "quantidade"}, {it.PesoKg, "peso"}} {
		if n.v != nil && (*n.v < 0 || *n.v > 1e6) {
			return badRequest("%s deve estar entre 0 e 1.000.000", n.nome)
		}
	}
	if it.Validade != nil {
		v := strings.TrimSpace(*it.Validade)
		if v == "" {
			it.Validade = nil
		} else if _, err := time.Parse("2006-01-02", v); err != nil {
			return badRequest("validade inválida (use AAAA-MM-DD)")
		} else {
			it.Validade = &v
		}
	}
	return nil
}

func scanItem(r interface{ Scan(...interface{}) error }) (Item, error) {
	var it Item
	var q, p sql.NullFloat64
	var v sql.NullString
	if err := r.Scan(&it.ID, &it.Nome, &q, &p, &v, &it.Local, &it.CriadoEm, &it.AtualizadoEm); err != nil {
		return it, err
	}
	if q.Valid {
		it.Quantidade = &q.Float64
	}
	if p.Valid {
		it.PesoKg = &p.Float64
	}
	if v.Valid {
		it.Validade = &v.String
	}
	return it, nil
}

func (s *store) itensAPI() crud {
	return crud{
		list: func() (interface{}, error) {
			rows, err := s.rdb.Query("SELECT " + itemCols + " FROM itens ORDER BY validade IS NULL, validade, nome COLLATE NOCASE")
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			out := []Item{} // [] (e não null) quando vazio
			for rows.Next() {
				it, err := scanItem(rows)
				if err != nil {
					return nil, err
				}
				out = append(out, it)
			}
			return out, rows.Err()
		},
		get: func(id int64) (interface{}, error) { return s.getItem(id) },
		create: func(body []byte) (interface{}, error) {
			var it Item
			if err := json.Unmarshal(body, &it); err != nil {
				return nil, badRequest("JSON inválido")
			}
			if err := it.clean(s.localOK); err != nil {
				return nil, err
			}
			res, err := s.db.Exec("INSERT INTO itens (nome, quantidade, peso_kg, validade, local) VALUES (?, ?, ?, ?, ?)",
				it.Nome, it.Quantidade, it.PesoKg, it.Validade, it.Local)
			if err != nil {
				return nil, err
			}
			id, _ := res.LastInsertId()
			return s.getItem(id)
		},
		update: func(id int64, body []byte) (interface{}, error) {
			var it Item
			if err := json.Unmarshal(body, &it); err != nil {
				return nil, badRequest("JSON inválido")
			}
			if err := it.clean(s.localOK); err != nil {
				return nil, err
			}
			res, err := s.db.Exec(`UPDATE itens SET nome = ?, quantidade = ?, peso_kg = ?, validade = ?, local = ?,
				atualizado_em = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = ?`,
				it.Nome, it.Quantidade, it.PesoKg, it.Validade, it.Local, id)
			if err != nil {
				return nil, err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return nil, errNotFound
			}
			return s.getItem(id)
		},
		remove: func(id int64) error {
			res, err := s.db.Exec("DELETE FROM itens WHERE id = ?", id)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return errNotFound
			}
			return nil
		},
	}
}

func (s *store) getItem(id int64) (interface{}, error) {
	it, err := scanItem(s.rdb.QueryRow("SELECT "+itemCols+" FROM itens WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	return it, err
}
