package main

import (
	"strings"
	"testing"
)

const csvPlanilha = `"Porfissional/Empresa","Serviço","Contato","Referência","O que vai fazer","Obs","",""
"Afonso","Alarme","11 95733-8888","Eliana","Trocar segredo","Não usar rede Enel","",""
"Ana","Encanadora","1133334444","","","","",""
"","","","","","","",""
"Sem serviço","","1","","","","",""
"Afonso","alarme","11 95733-8888","dup","x","","",""
`

func TestParseSheet(t *testing.T) {
	list, err := parseSheet(strings.NewReader(csvPlanilha))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("esperava 2 profissionais (sem linha em branco, sem serviço vazio e sem duplicado), veio %d: %#v", len(list), list)
	}
	a := list[0]
	if a.Nome != "Afonso" || a.TipoServico != "Alarme" || a.Contato != "11 95733-8888" || a.Indicacao != "Eliana" || a.Servico != "Trocar segredo" || a.Obs != "Não usar rede Enel" {
		t.Fatalf("colunas mapeadas errado: %#v", a)
	}
	// página de login do Google (planilha privada) não pode ser tomada por dados
	if _, err := parseSheet(strings.NewReader("<!DOCTYPE html><html><body>login</body></html>")); err == nil {
		t.Error("HTML deveria ser recusado")
	}
}

func TestSyncContatos(t *testing.T) {
	s, _ := newTestAPI(t)
	l1, _ := parseSheet(strings.NewReader(csvPlanilha))
	if a, u, r, err := s.syncContatos(l1); err != nil || a != 2 || u != 0 || r != 0 {
		t.Fatalf("primeira sync: %d/%d/%d %v", a, u, r, err)
	}
	if a, u, r, err := s.syncContatos(l1); err != nil || a+u+r != 0 {
		t.Fatalf("repetir não deve mudar nada: %d/%d/%d %v", a, u, r, err)
	}
	l2, _ := parseSheet(strings.NewReader(strings.Replace(csvPlanilha, `"Ana","Encanadora"`, `"Zé","Chaveiro"`, 1)))
	l2[0].Obs = "mudou"
	if a, u, r, err := s.syncContatos(l2); err != nil || a != 1 || u != 1 || r != 1 {
		t.Fatalf("sync com mudanças: %d/%d/%d %v", a, u, r, err)
	}
	if _, _, _, err := s.syncContatos(nil); err == nil {
		t.Error("lista vazia não pode apagar o banco")
	}
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM contatos").Scan(&n)
	if n != 2 {
		t.Errorf("esperava 2 linhas, tem %d", n)
	}
}
