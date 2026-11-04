#!/usr/bin/env bash
# Gera dist/centraldiogo.tgz (Linux) e dist/centraldiogo-windows.zip (Windows) com tudo que cada sistema precisa (roda no seu PC).
# Uso: ./package.sh [vX.Y.Z]   (com a versão, o pacote vira uma release: o install.sh grava essa versão em VERSION)
set -e
VER="${1#v}"
cd "$(dirname "$0")"
./build.sh >/dev/null
P=dist/pkg; rm -rf $P; mkdir -p $P
cp dist/hub-linux-* install.sh centraldiogo-hub.service $P/
# config de produção: porta 80 e painel em ./www (o arquivo de origem não é alterado).
# Por padrão parte do hub.config.example.json (versionado). O hub.config.json local tem URLs privadas das agendas: só use
# CONFIG=hub.config.json ./package.sh para um pacote pessoal, que NUNCA deve ser publicado em release.
CONFIG="${CONFIG:-hub.config.example.json}"
sed -e 's#"listen": *"[^"]*"#"listen": ":80"#' -e 's#"staticDir": *"[^"]*"#"staticDir": "./www"#' "$CONFIG" > $P/hub.config.json
[ -n "$VER" ] && echo "$VER" > $P/RELEASE
tar -C ../src -czf $P/www.tgz .
tar -C $P -czf dist/centraldiogo.tgz .

# Windows: zip com o instalador em janela (windows/), o hub, o painel em ./www e a configuração de exemplo
W=dist/winpkg; rm -rf $W; mkdir -p $W/www
cp dist/hub-windows-amd64.exe windows/install.ps1 windows/Instalar.cmd windows/LEIA-ME.txt hub.config.example.json $W/
[ -n "$VER" ] && echo "$VER" > $W/RELEASE
tar -C ../src -cf - . | tar -C $W/www -xf -
PY=python3; command -v $PY >/dev/null || PY=python
rm -f dist/centraldiogo-windows.zip
(cd $W && $PY -m zipfile -c ../centraldiogo-windows.zip *)
ls -lh dist/centraldiogo.tgz dist/centraldiogo-windows.zip
