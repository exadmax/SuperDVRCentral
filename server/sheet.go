// Sincroniza os profissionais (tabela contatos) com uma aba do Google Planilhas publicada como CSV.
// A planilha é a fonte: linhas novas entram, mudanças atualizam, linhas que saíram da planilha são apagadas.
package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Nomes de cabeçalho aceitos para cada campo (comparados sem acento, em minúsculas e sem espaços extras).
var sheetCols = map[string][]string{
	"nome":      {"profissional/empresa", "porfissional/empresa", "profissional", "empresa", "nome"},
	"tipo":      {"servico", "tipo de servico", "tipo"},
	"contato":   {"contato", "telefone"},
	"indicacao": {"referencia", "indicacao"},
	"servico":   {"o que vai fazer", "o que fazer"},
	"obs":       {"obs", "observacao", "observacoes"},
}

var semAcento = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ç", "c")

func normHeader(s string) string {
	return strings.Join(strings.Fields(semAcento.Replace(strings.ToLower(s))), " ")
}

// parseSheet lê o CSV e devolve os contatos válidos; linhas em branco ou inválidas são ignoradas.
func parseSheet(r io.Reader) ([]Contato, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV inválido: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("planilha vazia")
	}
	idx := map[string]int{}
	for i, h := range rows[0] {
		h = normHeader(h)
		for field, names := range sheetCols {
			for _, n := range names {
				if _, dup := idx[field]; h == n && !dup {
					idx[field] = i
				}
			}
		}
	}
	if _, ok := idx["nome"]; !ok {
		return nil, fmt.Errorf("cabeçalho sem a coluna do profissional (a planilha está com acesso por link?)")
	}
	if _, ok := idx["tipo"]; !ok {
		return nil, fmt.Errorf("cabeçalho sem a coluna Serviço")
	}
	get := func(row []string, f string) string {
		if i, ok := idx[f]; ok && i < len(row) {
			return row[i]
		}
		return ""
	}
	var out []Contato
	seen := map[string]bool{}
	for n, row := range rows[1:] {
		c := Contato{Nome: get(row, "nome"), TipoServico: get(row, "tipo"), Contato: get(row, "contato"),
			Indicacao: get(row, "indicacao"), Servico: get(row, "servico"), Obs: get(row, "obs")}
		if strings.TrimSpace(c.Nome) == "" && strings.TrimSpace(c.TipoServico) == "" {
			continue // linha em branco
		}
		if err := c.clean(); err != nil {
			log.Printf("planilha de profissionais, linha %d ignorada: %v", n+2, err)
			continue
		}
		if k := contatoKey(c); !seen[k] {
			seen[k] = true
			out = append(out, c)
		}
	}
	return out, nil
}

func contatoKey(c Contato) string {
	return strings.ToLower(c.Nome) + "|" + strings.ToLower(c.TipoServico)
}

// syncContatos aplica a lista da planilha ao banco, numa transação só.
func (s *store) syncContatos(list []Contato) (added, updated, removed int, err error) {
	if len(list) == 0 {
		return 0, 0, 0, fmt.Errorf("a planilha não trouxe nenhum profissional; nada foi alterado")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT " + contatoCols + " FROM contatos")
	if err != nil {
		return
	}
	have := map[string]Contato{}
	for rows.Next() {
		var c Contato
		if err = rows.Scan(&c.ID, &c.Nome, &c.TipoServico, &c.Contato, &c.Indicacao, &c.Servico, &c.Obs, &c.CriadoEm, &c.AtualizadoEm); err != nil {
			rows.Close()
			return
		}
		have[contatoKey(c)] = c
	}
	rows.Close()
	for _, c := range list {
		k := contatoKey(c)
		old, ok := have[k]
		delete(have, k)
		switch {
		case !ok:
			_, err = tx.Exec("INSERT INTO contatos (nome, tipo_servico, contato, indicacao, servico, obs) VALUES (?, ?, ?, ?, ?, ?)",
				c.Nome, c.TipoServico, c.Contato, c.Indicacao, c.Servico, c.Obs)
			added++
		case old.Nome != c.Nome || old.TipoServico != c.TipoServico || old.Contato != c.Contato || old.Indicacao != c.Indicacao || old.Servico != c.Servico || old.Obs != c.Obs:
			_, err = tx.Exec(`UPDATE contatos SET nome = ?, tipo_servico = ?, contato = ?, indicacao = ?, servico = ?, obs = ?,
				atualizado_em = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = ?`, c.Nome, c.TipoServico, c.Contato, c.Indicacao, c.Servico, c.Obs, old.ID)
			updated++
		}
		if err != nil {
			return
		}
	}
	for _, old := range have {
		if _, err = tx.Exec("DELETE FROM contatos WHERE id = ?", old.ID); err != nil {
			return
		}
		removed++
	}
	err = tx.Commit()
	return
}

func (s *store) syncSheetOnce(client *http.Client, url string) (added, updated, removed int, err error) {
	resp, err := client.Get(url)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, 0, fmt.Errorf("o Google respondeu %s", resp.Status)
	}
	list, err := parseSheet(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return 0, 0, 0, err
	}
	return s.syncContatos(list)
}

// sheetLoop sincroniza ao ligar e depois a cada intervalo (padrão 5 min); erros só vão para o log e o banco fica como está.
func (s *store) sheetLoop(url string, every time.Duration) {
	if every < time.Minute {
		every = 5 * time.Minute
	}
	client := &http.Client{Timeout: 30 * time.Second}
	for {
		a, u, r, err := s.syncSheetOnce(client, url)
		if err != nil {
			log.Printf("sincronização da planilha de profissionais: %v", err)
		} else if a+u+r > 0 {
			log.Printf("planilha de profissionais sincronizada: %d novos, %d atualizados, %d removidos", a, u, r)
		}
		time.Sleep(every)
	}
}
