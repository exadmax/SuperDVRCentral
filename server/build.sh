#!/usr/bin/env bash
# Compila o hub para Linux (4 arquiteturas) e Windows (amd64), binários estáticos sem dependências. Uso: ./build.sh
set -e
cd "$(dirname "$0")"
mkdir -p dist
for t in amd64 386 arm64 arm; do
  GOOS=linux GOARCH=$t CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "dist/hub-linux-$t" .
done
# Windows: sem janela de console (o hub roda em segundo plano, iniciado pela tarefa agendada do instalador)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H=windowsgui" -o dist/hub-windows-amd64.exe .
ls -lh dist
