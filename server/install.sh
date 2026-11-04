#!/usr/bin/env bash
# Roda NO Linux, com sudo:  sudo bash install.sh
# Instala o hub em /opt/dvr, libera a porta 80 e ativa o serviço.
# Pacote de release (tem o arquivo RELEASE, gravado por package.sh a partir da tag vX.Y.Z): a instalação grava essa versão em VERSION.
# Pacote sem RELEASE (montado do código, sem tag): cada instalação concluída soma 1 ao terceiro número (VERSION: 1.0.1 -> 1.0.2).
# Só consultar a versão instalada, sem sudo e sem alterar nada:  bash install.sh --check
set -e
cd "$(dirname "$0")"
D=/opt/dvr

# Versão instalada = arquivo VERSION ("MAIOR.MENOR.PATCH"). Sem o arquivo ainda não há instalação versionada: parte de 1.0.0.
# Para subir o MAIOR ou o MENOR, edite o VERSION à mão (ex.: 1.1.0); as instalações seguintes continuam somando no terceiro.
OLD=1.0.0
[ -f "$D/VERSION" ] && OLD="$(tr -d '[:space:]' < "$D/VERSION")"
if ! echo "$OLD" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$'; then
  echo "!! $D/VERSION inválido: '$OLD' (esperado algo como 1.0.1). Corrija o arquivo e rode de novo."; exit 1
fi
BASE="${OLD%%-*}"   # sem o sufixo de pré-lançamento (-rc.1)
NEW="${BASE%.*}.$((10#${BASE##*.} + 1))"
# Versão da release empacotada (ver docs/versionamento.md): manda no número instalado.
if [ -f RELEASE ]; then
  REL="$(tr -d '[:space:]' < RELEASE)"
  echo "$REL" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$' || { echo "!! RELEASE inválido: '$REL'"; exit 1; }
  NEW="$REL"
fi

if [ "$1" = "--check" ]; then
  if [ -f "$D/VERSION" ]; then echo "versão instalada: V$OLD (desde $(date -r "$D/VERSION" '+%F %T'))"
  else echo "versão instalada: nenhuma registrada ainda"; fi
  if [ -f RELEASE ]; then echo "este pacote instala: V$NEW"; else echo "a próxima instalação será: V$NEW"; fi
  exit 0
fi

[ "$(id -u)" = 0 ] || { echo "rode com sudo: sudo bash install.sh"; exit 1; }
USER_RUN="${SUDO_USER:-root}"
case "$(uname -m)" in
  x86_64) A=amd64;; i?86) A=386;; aarch64|arm64) A=arm64;; armv6*|armv7*|arm) A=arm;;
  *) echo "arquitetura não suportada: $(uname -m)"; exit 1;;
esac
echo ">> arquitetura: $A"

# a porta 80 precisa estar livre (o nginx atual está na 8080)
if ss -ltn 2>/dev/null | awk '{print $4}' | grep -qE '[:.]80$' && ! systemctl is-active --quiet dvr-hub; then
  echo "!! Já existe algo escutando na porta 80:"; ss -ltnp | grep -E '[:.]80 ' || true; exit 1
fi

mkdir -p "$D/www"
install -m 755 "hub-linux-$A" "$D/hub"
# A configuração do servidor (hub.config.json e o hub.settings.json da página Config) é preservada: o pacote só instala a configuração
# na primeira instalação. Para trocar algo depois, use a página Config ou edite o arquivo no servidor.
if [ -f "$D/hub.config.json" ]; then
  cp "$D/hub.config.json" "$D/hub.config.json.bak"
  echo ">> hub.config.json existente mantido (cópia em hub.config.json.bak)"
else
  install -m 600 hub.config.json "$D/hub.config.json"
fi
rm -rf "$D/www" && mkdir "$D/www" && tar -xzf www.tgz -C "$D/www"
chown -R "$USER_RUN" "$D"
sed "s/__USER__/$USER_RUN/" dvr-hub.service > /etc/systemd/system/dvr-hub.service

# firewall: libera a 80 se houver ufw/firewalld ativo
if command -v ufw >/dev/null && ufw status | grep -q "Status: active"; then ufw allow 80/tcp; echo ">> ufw: porta 80 liberada"
elif command -v firewall-cmd >/dev/null && systemctl is-active --quiet firewalld; then firewall-cmd --permanent --add-port=80/tcp && firewall-cmd --reload; echo ">> firewalld: porta 80 liberada"
else echo ">> sem firewall ativo detectado (ufw/firewalld); nada a liberar"; fi

systemctl daemon-reload
systemctl enable dvr-hub
systemctl restart dvr-hub
sleep 1.5
systemctl is-active dvr-hub

# instalação concluída (o serviço subiu): registra a nova versão. O hub lê o arquivo, então /api/health já mostra a nova.
echo "$NEW" > "$D/VERSION.tmp" && mv "$D/VERSION.tmp" "$D/VERSION"
echo "$(date '+%F %T')  V$OLD -> V$NEW" >> "$D/install.log"
chown "$USER_RUN" "$D/VERSION" "$D/install.log"
echo ">> versão instalada: V$NEW (era V$OLD)"
curl -s localhost/api/health && echo && echo ">> pronto: http://$(hostname -I | awk '{print $1}')/"
