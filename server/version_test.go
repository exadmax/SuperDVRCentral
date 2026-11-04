package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadVersion(t *testing.T) {
	casos := []struct {
		nome, conteudo string
		existe         bool
		want           string
	}{
		{"sem arquivo (desenvolvimento)", "", false, "dev"},
		{"versão normal", "1.0.1\n", true, "1.0.1"},
		{"com espaços e CRLF", "  2.3.10 \r\n", true, "2.3.10"},
		{"vazio", "", true, "dev"},
		{"formato errado", "V1.0.1", true, "dev"},
		{"faltando o terceiro número", "1.0", true, "dev"},
		{"lixo", "1.0.1; rm -rf /", true, "dev"},
	}
	for _, c := range casos {
		dir := t.TempDir()
		if c.existe {
			if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(c.conteudo), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got, at := readVersion(dir)
		if got != c.want {
			t.Errorf("%s: esperava %q, veio %q", c.nome, c.want, got)
		}
		if (got == "dev") != (at == "") {
			t.Errorf("%s: installedAt=%q não combina com a versão %q", c.nome, at, got)
		}
		if at != "" {
			if _, err := time.Parse(time.RFC3339, at); err != nil {
				t.Errorf("%s: installedAt fora do formato RFC 3339: %q", c.nome, at)
			}
		}
	}
}

func TestReadVersionAcompanhaOArquivo(t *testing.T) {
	// o install.sh troca o arquivo com o hub rodando: a próxima consulta já mostra a versão nova
	dir := t.TempDir()
	p := filepath.Join(dir, "VERSION")
	os.WriteFile(p, []byte("1.0.1\n"), 0o644)
	if v, _ := readVersion(dir); v != "1.0.1" {
		t.Fatalf("esperava 1.0.1, veio %q", v)
	}
	os.WriteFile(p, []byte("1.0.2\n"), 0o644)
	if v, _ := readVersion(dir); v != "1.0.2" {
		t.Fatalf("esperava 1.0.2 após a troca, veio %q", v)
	}
}
