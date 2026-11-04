package main

import (
	"log"
	"os"
	"path/filepath"
)

// initLogFile: no Windows o hub roda sem console (tarefa agendada), então o log vai para hub.log ao lado da configuração.
// O arquivo recomeça quando passa de 5 MB.
func initLogFile(dir string) {
	p := filepath.Join(dir, "hub.log")
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if st, err := os.Stat(p); err == nil && st.Size() > 5<<20 {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	if f, err := os.OpenFile(p, flags, 0o600); err == nil {
		log.SetOutput(f)
	}
}
