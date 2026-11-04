# Conquistas (subsistema de jogos)

> **A página de Conquistas foi retirada do painel** (o atalho e `jogos.html` não existem mais). O backend continua no
> hub (`server/games*.go`, `GET /api/games`), sem tela. Para recuperar a página, ela está no histórico do git
> (commit `b8f576a`: `src/jogos.html`, `src/js/games/`, `src/css/games.css`). As instruções abaixo que falam do
> dashboard valem só se a página voltar.

Dashboard (removido) com jogos, conquistas e troféus de:

| Plataforma | Conta | Como o hub consulta |
|---|---|---|
| PlayStation | `seu-usuario` | mesmo fluxo da [psnawp](https://pypi.org/project/psnawp/): cookie **npsso** → token OAuth → APIs de troféus e tempo de jogo |
| Xbox | `SeuGamertag` | login Microsoft (cliente do app do Xbox, sem Azure) → token Xbox Live/XSTS → titlehub, achievements e profile |
| Steam | `seu-usuario` (SteamID64) | [Steam Web API](https://developer.valvesoftware.com/wiki/Steam_Web_API) com chave |
| RetroAchievements | `seu-usuario` | [Web API](https://api-docs.retroachievements.org) com chave |

Tudo passa pelo hub (`server/games*.go`): as chaves ficam em `hub.config.json` (bloco `games`) e os tokens
renovados + o cache dos dados em `games.state.json` (mesma pasta, permissão 600). O hub atualiza ao iniciar e a cada
`cacheMinutes` (padrão 6h); o botão **Atualizar** do dashboard força uma nova consulta (no máx. a cada 5 min).
Plataforma sem chave aparece com dados de **exemplo**.

Endpoint: `GET /api/games` (`?refresh=1` para forçar).

## Configurar (no `hub.config.json` do servidor)

```json
"games": {
  "cacheMinutes": 360,
  "stateFile": "games.state.json",
  "steam": { "key": "", "user": "seu-usuario" },
  "psn":   { "npsso": "", "user": "seu-usuario" },
  "xbox":  { "clientId": "", "clientSecret": "", "gamertag": "SeuGamertag" },
  "retro": { "key": "", "user": "seu-usuario" }
}
```

### Steam
A Valve só libera a chave para contas com o **autenticador móvel do Steam Guard** ativo (app Steam no celular →
Steam Guard). Sem chave não há alternativa: a lista de jogos da comunidade agora exige login.
1. Com o autenticador ativo, gere a chave em https://steamcommunity.com/dev/apikey (domínio: qualquer, ex. `servidor.local`).
2. No perfil Steam → Editar perfil → Privacidade: **Detalhes de jogos = Público** (senão a lista vem vazia).
3. `user` aceita a URL personalizada (`seu-usuario`) ou o SteamID64 (`76561198000000000`).

A primeira atualização consulta as conquistas de cada jogo já jogado (pode levar alguns minutos numa biblioteca
grande); depois só reconsulta jogos jogados desde a última vez.

### RetroAchievements
Entre em https://retroachievements.org/settings → **Keys** → copie a *Web API Key* para `retro.key`.

### PlayStation (npsso)
1. Entre em https://www.playstation.com com a conta `seu-usuario`.
2. Na mesma aba abra https://ca.account.sony.com/api/v1/ssocookie e copie o valor de `npsso` (64 caracteres).
3. Coloque em `psn.npsso`. O hub troca por um refresh token (~2 meses) e o renova sozinho; quando expirar, o
   cartão da PSN mostra o erro e basta repetir estes passos.

### Xbox (uma vez, sem Azure)
A página do GDK (*Achievements Manager*) é a API do lado do jogo; para ler o **seu** perfil o hub usa os serviços
do Xbox Live com login da sua conta Microsoft. Por padrão o login usa o cliente público do próprio app do Xbox
(não precisa registrar nada no Azure; `xbox.clientId` fica vazio).

1. Abra o dashboard **pelo hub** (`http://192.168.0.10/jogos.html`). O cartão do Xbox mostra **Conectar**.
2. Clique em **Entrar na Microsoft** e entre com a conta do Xbox (`SeuGamertag`).
3. No fim abre uma página **em branco** (`login.live.com/oauth20_desktop.srf?code=...`). Copie o endereço inteiro
   da barra, cole no campo do cartão e clique **Conectar** (o código vale poucos minutos).

O refresh token vai para `games.state.json` e é renovado sozinho. Alternativa pelo terminal do servidor:
`./hub -config hub.config.json -xbox-login`. Se preferir um app próprio do Azure, preencha `xbox.clientId`
(cliente público com redirect `https://login.live.com/oauth20_desktop.srf`).

Depois de editar o `hub.config.json`: `sudo systemctl restart centraldiogo-hub`.

## Limitações
- Horas jogadas: só Steam e PS4/PS5 informam (Xbox e RetroAchievements não).
- Xbox 360: conquistas por jogo vêm como resumo; a lista de "recentes" do Xbox cobre jogos de Xbox One em diante.
- Steam: conquistas recentes não trazem ícone (usa a capa do jogo).
- O gráfico "Atividade" usa a data da última sessão de cada jogo, não o histórico completo.
