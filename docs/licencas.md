# Licenças de terceiros

O SuperDVRCentral (derivado do Central Diogo / DVRCentral) é distribuído sob a **GNU GPL v3** ([LICENSE](../LICENSE)). Este documento lista os
componentes de terceiros e a licença de cada um. Este texto é um inventário técnico, **não é aconselhamento jurídico**.

Regra do projeto para componentes novos: **GPL ou MIT** (ou equivalente permissivo), sem alterar o código de terceiros além do estritamente necessário e sempre
registrando a mudança no próprio arquivo ou aqui.

## No navegador (painel)

O painel (`src/`) não embute bibliotecas de terceiros: só fontes (Inter e Poppins, licença SIL OFL 1.1) em `src/assets/fonts/`.

## No servidor (Go)

Todas as dependências do hub estão em `server/go.mod` (SQLite em Go puro `modernc.org/sqlite`, RTSP `gortsplib`/`mediacommon`/`pion`, `golang.org/x/net`).
Os binários são estáticos (`CGO_ENABLED=0`).

## Atualizar um componente

- **Bibliotecas Go:** `cd server && go get <módulo>@<versão> && go mod tidy && go vet ./... && go test ./...`.
- Atualize este arquivo.
