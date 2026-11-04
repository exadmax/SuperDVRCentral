// Botões do painel para aliviar a máquina: desligar o desktop (display manager) e desativar o próprio hub.
// Só no Linux com systemd. O hub roda sem root, então a permissão vem de uma regra do polkit que o install.sh grava
// (somente "stop" dessas unidades, somente para o usuário do serviço). Nada é desativado no boot: o desktop volta no
// próximo reinício e o hub volta com "sudo systemctl start centraldiogo-hub".
package main

import (
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

const dmUnit = "display-manager.service" // alias do lightdm/gdm3/sddm instalado

// desktopActive diz se o display manager está rodando agora.
func desktopActive() bool {
	return runtime.GOOS == "linux" && exec.Command("systemctl", "is-active", "--quiet", dmUnit).Run() == nil
}

// stopUnit pede ao systemd para parar a unidade, sem esperar (parar o próprio hub mata este processo).
func stopUnit(unit string) error {
	return exec.Command("systemctl", "--no-block", "stop", unit).Run()
}

// powerHandler: POST com o cabeçalho X-CD-Action: confirm (um formulário de outra página não consegue enviá-lo sem preflight).
func powerHandler(m *sysMon, unit string, isHub bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-CD-Action") != "confirm" {
			http.Error(w, `{"error":"use POST com X-CD-Action: confirm"}`, http.StatusMethodNotAllowed)
			return
		}
		if runtime.GOOS != "linux" {
			http.Error(w, `{"error":"disponível só no Linux do servidor"}`, http.StatusNotImplemented)
			return
		}
		if err := stopUnit(unit); err != nil {
			log.Printf("parar %s: %v", unit, err)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"o servidor não permitiu (falta a regra do polkit: rode o install.sh de novo)"}`))
			return
		}
		log.Printf("parado pelo painel: %s", unit)
		if !isHub {
			go func() { time.Sleep(2 * time.Second); m.sample() }() // reflete o desktop desligado na próxima leitura
		}
		writeJSON(w, map[string]bool{"ok": true})
	}
}
