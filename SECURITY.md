# Segurança

## Modelo de ameaça
O hub é feito para uma **rede doméstica confiável**. Ele **não tem login**, o CORS é aberto e quem alcança a porta consegue ver as
câmeras, baixar e apagar gravações, mudar a configuração (inclusive chaves e endereços) e usar o botão que desativa o hub.

- **Não exponha o hub à internet** (nem por redirecionamento de porta). Para acesso remoto use uma VPN (Tailscale, WireGuard) ou um proxy
  reverso com autenticação e HTTPS na frente.
- Senhas das câmeras, chaves de API e endereços secretos de agenda ficam em `hub.config.json` e `hub.settings.json` no servidor, em texto
  (arquivos com permissão restrita ao dono). Não os versione nem os compartilhe; o `.gitignore` já os exclui.
- Gravações de câmeras são dados pessoais. Avise quem frequenta a casa e respeite a legislação local (no Brasil, a LGPD).
- A Config permite informar endereços (iCal, RTSP, planilha) que o **servidor** acessa. Em uma rede confiável isso é esperado; não dê
  acesso à Config a quem não deveria alcançar sua rede interna.

## Reportar uma vulnerabilidade
Abra um *security advisory* privado no GitHub (aba **Security** do repositório) em vez de uma issue pública. Descreva o problema e
como reproduzi-lo.
