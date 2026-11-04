// Versão instalada do hub. O install.sh grava "MAIOR.MENOR.PATCH" no arquivo VERSION (ao lado do hub.config.json)
// e soma 1 ao PATCH a cada instalação concluída (ex.: 1.0.1 -> 1.0.2). O hub lê o arquivo a cada consulta, então a versão
// nova aparece assim que o install.sh a grava.
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var versionRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// readVersion devolve a versão gravada em dir/VERSION e quando foi instalada (RFC 3339).
// Sem arquivo (ou com conteúdo inválido, ex.: rodando o hub em desenvolvimento) devolve "dev".
func readVersion(dir string) (version, installedAt string) {
	p := filepath.Join(dir, "VERSION")
	b, err := os.ReadFile(p)
	v := strings.TrimSpace(string(b))
	if err != nil || !versionRe.MatchString(v) {
		return "dev", ""
	}
	if st, err := os.Stat(p); err == nil {
		installedAt = st.ModTime().Format(time.RFC3339)
	}
	return v, installedAt
}
