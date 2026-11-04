# Câmeras ao vivo (`camaras.html`)

Página com a imagem ao vivo das câmeras de casa, cada uma com o nome em cima. Abre pelo atalho **Câmeras** do painel.

## Como funciona
- O **hub** (`server/cameras.go`) abre o RTSP da câmera, pega o vídeo **H.264** e o entrega ao navegador como MP4 fragmentado contínuo em
  `GET /api/cameras/{id}/stream`. Não recodifica (quase sem CPU) e **não precisa de ffmpeg**.
- A página (`src/js/pages/camaras.js`) toca com Media Source Extensions, sem plugin. Sem áudio (fica mudo).
- A conexão com a câmera só existe enquanto alguém assiste (fecha 15 s depois do último espectador) e é **compartilhada**: várias telas
  abertas usam uma sessão RTSP só por câmera. Com a aba em segundo plano, a página para o vídeo.
- `GET /api/cameras` lista `id` e `name`. A **URL com a senha fica só no `hub.config.json`** (no servidor); o navegador nunca a recebe.

## Rodízio na tela inicial
No painel principal, o **card do meio (Clima) alterna com as câmeras**: Clima → Garagem → Quintal → Sala de Estar → Clima..., **1 minuto cada**
(a ordem é a da lista `cameras` do hub). Etiquetas embaixo do vídeo mostram onde está o rodízio. O Clima continua no cabeçalho.
- Só roda com o hub no ar e se houver câmeras configuradas; sem hub (ou em navegador sem Media Source, como TVs antigas) fica só o Clima.
- Muda o tempo em `src/config/settings.js`: `cameras: { rotateSeconds: 60 }`. Para testar rápido: `index.html?rotate=15`.
- No modo noturno e com a aba escondida o vídeo para e a conexão com a câmera fecha. O vídeo fica num elemento próprio que sobrevive aos
  redesenhos do painel (a cada 30 s), então não reinicia (`src/js/widgets/cameras.js`; o player é `src/js/services/cameraPlayer.js`).

## Configurar (`hub.config.json`)
```json
"cameras": [
  {"id": "garagem", "name": "Garagem", "url": "rtsp://admin:SENHA@192.168.0.21:554/cam/realmonitor?channel=1&subtype=1"},
  {"id": "quintal", "name": "Quintal", "url": "rtsp://admin:SENHA@192.168.0.22:554/cam/realmonitor?channel=1&subtype=1"},
  {"id": "sala-de-estar", "name": "Sala de Estar", "url": "rtsp://192.168.0.23:554/live/mpeg4"}
]
```
A ordem da lista é a ordem na página. Caracteres especiais da senha vão codificados na URL (`@` = `%40`, `!` = `%21`).

## Câmeras Mibo Smart (Intelbras)
- **Use `subtype=1`** (substream): é H.264. O `subtype=0` das iM7 é **H.265**, que o hub não repassa ao navegador.
- Usuário `admin`; a senha é a que o app Mibo Smart definiu para a câmera (o RTSP só aceita a senha nova se ela foi trocada no app).
  Com login errado várias vezes seguidas a câmera bloqueia por alguns minutos (403).
- Alguns modelos (ex.: servidor "Hankvision") entregam H.264 em `rtsp://IP:554/live/mpeg4`, **sem senha**; só o substream principal
  (2304x1296) existe nesses, então consomem mais banda da rede.
- Reserve os IPs das câmeras no roteador (DHCP fixo), senão a integração quebra quando o IP muda.

## Gravações (1 arquivo por hora)
O hub grava o mesmo vídeo H.264 que já repassa ao navegador (sem recodificar, sem ffmpeg) em `server/recorder.go`:
- Arquivos `NOME_DA_CAMERA_AAAAMMDDHH.mp4` (ex.: `Sala_de_Estar_2026100114.mp4`), 1 por câmera por hora, na pasta
  **`~/securityCam`** (pasta pessoal de quem roda o hub) (campo `"recordings": {"dir": "..."}` do `hub.config.json`; o hub cria a pasta). O arquivo novo começa
  no primeiro quadro-chave depois da virada da hora. Se o hub reiniciar na mesma hora, o arquivo seguinte leva `_2`, `_3`...
- **Tudo se configura na página Config**, por câmera (grava em `recordings.state.json`, ao lado do `hub.config.json`). A chave geral
  **Gravar as câmeras** vem **desligada**. Para cada câmera:
  - liga/desliga;
  - **Modo**: *Contínuo* (grava o tempo todo) ou *Só com movimento*;
  - **Sensibilidade** 1–5 (só no modo movimento);
  - **Guardar por** N dias (padrão 7; 1 a 365);
  - **Limite de espaço** em GB (0 = sem limite): passou disso, o hub apaga as gravações mais antigas **daquela câmera**.
  A página mostra por câmera o estado (gravando, aguardando movimento, erro) e os arquivos/GB em uso.
- **Modo movimento** (`server/motion.go`): não decodifica o vídeo. Compara o tamanho de cada quadro P com a média recente da câmera
  (parado = quadros minúsculos; movimento = quadros grandes) e dispara com 3 de 8 quadros acima do limite. Grava o grupo de quadros
  desde o quadro-chave anterior ao movimento (alguns segundos de pré-roll) e continua 15 s depois do último movimento. Hora sem movimento
  não cria arquivo; o arquivo da hora contém só os trechos com movimento, um depois do outro. Custo: uns poucos números por câmera.
  Luz ligando/desligando, TV e sombras podem disparar; baixe a sensibilidade se gravar demais.
- **Retenção**: a cada 10 min (e ao mudar a configuração) o hub apaga as gravações vencidas e aplica o limite de GB. Só mexe em
  arquivos com o nome acima; nunca em outros arquivos da pasta.
- Ligada, a gravação mantém a conexão com a câmera aberta 24 h (mesmo sem ninguém assistindo) e compartilha a sessão com a imagem ao vivo.
- Em **Câmeras → Gravações** a lista (por dia, com filtro por câmera) tem **Assistir** (player do navegador, com barra para pular) e **Baixar**.
  API: `GET /api/recordings`, `GET /api/recordings/file/{nome}` (com Range), `GET|PUT /api/recordings/settings`.
- A qualidade é a do endereço RTSP da câmera (`subtype=1` = substream, baixa resolução). Ver o espaço: ~0,3–1 GB/dia por câmera no substream.

## Cópia diária para o Google Drive (rclone)
Todo dia às **3:00** o hub (`server/drive.go`) envia as gravações novas para o Google Drive usando o `rclone` já instalado no servidor:
- Destino: `GDrive:CameraSeguranca/AAAA-MM-DD/NOME_DA_CAMERA/arquivo.mp4`. O dia é o da gravação (a pasta do dia é criada se não existir).
  Ex.: `CameraSeguranca/2026-10-03/Sala_de_Estar/Sala_de_Estar_2026100314.mp4`.
- "Novas" = ainda não enviadas e já fechadas (a hora acabou e ninguém escreve no arquivo há 5 min). O que já subiu fica em `drive.state.json`
  (ao lado do `hub.config.json`), então nada é enviado duas vezes. As gravações locais seguem a retenção da página Config; a cópia não apaga nada.
- **Arquivo maior que `maxFileMB`** (padrão 1024) é dividido em `..._parte1.mp4`, `_parte2.mp4`... nos limites dos trechos do MP4, sem
  recodificar; cada parte toca sozinha. Arquivo que falhou no meio não é marcado como enviado e é refeito inteiro na próxima tentativa.
- **Limpeza no Drive**: depois de enviar, apaga do Drive o que passou da **mesma retenção da página Config** ("Guardar por N dias", por câmera; padrão 7), ou seja, o Drive e o servidor guardam o mesmo período. A cada rodada remove `CameraSeguranca/AAAA-MM-DD/NOME_DA_CAMERA` quando o dia é mais velho que a retenção daquela câmera e a pasta do dia quando fica vazia. Só mexe em pastas com nome de data e, dentro delas, com nome de câmera; qualquer outra coisa na `CameraSeguranca` é ignorada. As pastas vão para a **lixeira do Drive** (recuperáveis por 30 dias). Gravação já vencida que ainda está no disco não é enviada.
- **Erro**: a rodada faz no máximo **3 tentativas**, a 2ª 1 h depois da 1ª e a 3ª 4 h depois da 2ª, e só até **1:00 do dia seguinte** (2 h antes da próxima rodada); depois disso espera a das 3:00. A tela mostra "Tentativa 2 de 3" e a hora da próxima. Se o hub estava desligado às 3:00, faz a rodada atrasada ao voltar (5 min depois de subir), dentro desse prazo.
- Config (`hub.config.json`): `"driveSync": {"enabled": true, "remote": "GDrive:CameraSeguranca", "hour": 3, "maxFileMB": 1024}`.
- **Página Config → Google Drive**: **Sincronizar tudo** e, em cada câmera, **Sincronizar** enviam agora só as gravações novas (diferencial: o que já subiu não sobe de novo). Mostra o que falta por câmera, o andamento e o último resultado. Envio de só uma câmera não conta como a rodada das 3:00 e não apaga nada do Drive.
- **Interruptor por câmera** ("Enviar ao Google Drive", ao lado do botão): desligado, a câmera fica fora da rodada das 3:00 e do **Sincronizar tudo**; só vai ao Drive se você clicar no **Sincronizar** da própria câmera. Ligado por padrão (campo `driveOff` em `recordings.state.json`). A limpeza por retenção no Drive vale para todas, ligadas ou não.
- API (escrita exige `X-CD-Action: confirm`): `GET /api/recordings/drive` (estado, pendências por câmera, tentativas); `POST /api/recordings/drive/run` (envia agora; corpo opcional `{"cameras":["id"]}`); `POST /api/recordings/drive/skip` (`{"olderThanDays":2}` marca como já enviadas, sem enviar, as pendentes mais velhas que isso, para pular um acúmulo antigo; o arquivo fica no servidor até a retenção).
- Ver o resultado: `journalctl -u centraldiogo-hub | grep drive:`.

### Configurar o rclone no servidor (uma vez)
O remote `GDrive` precisa de um token da sua conta Google. O servidor não tem navegador, então o login é feito neste PC por um túnel SSH:
```bash
ssh -i ~/.ssh/centraldiogo_deploy -L 53682:127.0.0.1:53682 usuario@192.168.0.10 'rclone config reconnect GDrive: --auto-confirm'
```
Abra no navegador do PC o link `http://127.0.0.1:53682/auth?state=...` que o comando imprime, entre com a conta Google e clique em Permitir.
Conferir: `ssh usuario@192.168.0.10 'rclone lsd GDrive:'`. Opcional: criar um *client ID* próprio no Google Cloud (`rclone config`) para não
dividir a cota da chave pública do rclone. O serviço do hub roda como o usuário definido no `install.sh`, então usa `~/.config/rclone/rclone.conf` desse usuário.

## Limites
- Só H.264 (H.265 mostra o erro "não entrega H.264 neste endereço").
- Precisa do hub; aberto fora dele (ex.: só `python -m http.server`) a página avisa que está sem o hub.
- Navegadores sem Media Source (iPhone antigo) mostram um aviso em vez da imagem.
