# Licenças de terceiros

O SuperDVRCentral (derivado do DVR / DVRCentral) é distribuído sob a **GNU GPL v3** ([LICENSE](../LICENSE)). Este documento lista os
componentes de terceiros e a licença de cada um. Este texto é um inventário técnico, **não é aconselhamento jurídico**.

Regra do projeto para componentes novos: **GPL ou MIT** (ou equivalente permissivo), sem alterar o código de terceiros além do estritamente necessário e sempre
registrando a mudança no próprio arquivo ou aqui.

## No navegador (painel)

O painel (`src/`) usa fontes Inter e Poppins (licença SIL OFL 1.1) em `src/assets/fonts/` e uma biblioteca:

| Componente | Onde | Versão | Licença | Alterações |
|---|---|---|---|---|
| **three.js** — https://github.com/mrdoob/three.js | `src/vendor/three/three.min.js` (só a página Omega) | r128 | **MIT** (cabeçalho do arquivo) | Nenhuma ([LEIA-ME](../src/vendor/three/LEIA-ME.md), com o SHA-256) |

## No servidor (Go)

Todas as dependências do hub estão em `server/go.mod` (SQLite em Go puro `modernc.org/sqlite`, RTSP `gortsplib`/`mediacommon`/`pion`, `golang.org/x/net`).
Os binários são estáticos (`CGO_ENABLED=0`).

## Atualizar um componente

- **Bibliotecas Go:** `cd server && go get <módulo>@<versão> && go mod tidy && go vet ./... && go test ./...`.
- Atualize este arquivo.
