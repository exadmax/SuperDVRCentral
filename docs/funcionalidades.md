# Mapa de funcionalidades

Inventário de tudo que o sistema faz, de onde vem e onde está o código. Para o passo a passo de cada área, siga os links
para os outros documentos de `docs/`.

Legenda de **Precisa do hub**: ✅ só funciona com o hub no ar · ➖ funciona sem ele (com dados de exemplo ou direto nas APIs públicas).

## 1. Visão geral

```
 TV / celular / PC ── navegador ──►  http://192.168.0.10/  ──►  Hub Go (server/)  ──►  SQLite, RTSP, Google, Google Routes,
                                                                   serve src/ + /api/*       Steam, PSN, Xbox, RetroAchievements
```

| Parte | Pasta | Tecnologia |
|---|---|---|
| Painel web (único para todas as telas) | `src/` | HTML + JS clássico (sem módulos ES, roda também em navegadores de TV), CSS |
| Hub (servidor + API) | `server/` | Go 1.26, biblioteca padrão + `modernc.org/sqlite`, `gortsplib`, `mediacommon`, `golang.org/x/net` e, para o E-mail, `go-imap/v2`, `go-message` e `go-sasl` (veja [licencas.md](licencas.md)) |

## 2. Páginas e funcionalidades do painel

| Página | Arquivo | Atalho | Precisa do hub | O que faz |
|---|---|---|---|---|
| **Início** | `index.html` + `js/core/app.js` | — | ➖ | Painel passivo: relógio, selo de feriado, clima, agenda, trânsito, rodízio de câmeras, atalhos e rodapé |
| **Geladeira e Dispensa** | `estoque.html` + `js/pages/estoque.js` | Geladeira | ✅ | CRUD de itens com quantidade/peso, validade e local; filtros e busca |
| **Serviços** | `contatos.html` + `js/pages/contatos.js` | Serviços | ✅ | CRUD de profissionais (eletricista, encanador...); sincroniza com Google Planilhas |
| **Câmeras** | `camaras.html` + `js/pages/camaras.js` | Câmeras | ✅ | Imagem ao vivo de cada câmera e lista de gravações (assistir/baixar) |
| **Vídeos** | `videos.html` + `js/pages/videos.js` | Vídeos | ✅ | Lista a pasta de vídeos; tocar no navegador, abrir no VLC, copiar link |
| **Livros** | `livros.html` + `js/pages/livros.js` | Livros | ✅ | PDFs e EPUBs no servidor: envio pelo painel, categoria e dono, leitor de EPUB; pode ser desligada na Config |
| **Omega** | `omega.html` + `js/omega/*.js` | Omega | ✅ | Assistente por voz com cena 3D (three.js): câmera, trânsito, agenda, estoque e serviços (consulta, adicionar, remover) e perguntas à IA; em TVs abre `omega-tv.html` (GIF, caixa lateral de perguntas e microfone, sem three.js) ([omega.md](omega.md)) |
| **Config** | `config.html` + `js/pages/config.js` | Config | ✅ | CPU/memória do servidor, botões de energia e configuração das gravações |
| Jogos (externo) | — | Jogos | ➖ | Abre o WebRCade (porta 8080) |
| Música (externo) | — | Música | ➖ | Abre o Navidrome (porta 4533) |

As páginas avulsas usam `js/pages/boot.js`, que lê `<body data-page="...">`, desenha o cabeçalho com botão de voltar, detecta o hub
(`/api/health`) e chama o `mount()` do módulo correspondente em `CD.pages`.

### 2.1 Tela inicial (Início)

| Função | Descrição | Fonte de dados | Código |
|---|---|---|---|
| Relógio e data | Hora grande e data por extenso em pt-BR; redesenha a cada 30 s | relógio local | `core/app.js`, `core/util.js` |
| Selo de feriado | Feriado nacional/estadual/municipal (cor por tipo), "Fim de semana" ou "Dia útil" | BrasilAPI + `config/holidays-sp.js` | `widgets/holidays.js`, `services/holidaysApi.js` |
| Clima | Temperatura atual, previsão do dia, alerta "leve guarda-chuva", mín/máx de amanhã | Open-Meteo (sem chave) | `widgets/weather.js`, `services/weatherApi.js` |
| Agenda do dia | Eventos do casal por cor/pessoa, próximo evento com contagem regressiva, rolagem automática se longa; suporta recorrência iCal (RRULE, EXDATE, COUNT, `nº` dia da semana) | Google Calendar (iCal) via `/api/ics` | `widgets/agenda.js`, `services/calendar.js` |
| Trânsito | Tempo até o trabalho de cada pessoa cadastrada (automático na janela configurada, ou sob demanda por 10 min via "Ver trânsito"), nível livre/moderado/congestionado, horário de saída, mini-mapa do trajeto | Google Routes via `/api/`{text, history}` → `{say, actions[camera|traffic|agenda|list], source, provider}`` | `widgets/map.js`, `services/trafficApi.js` |
| Rodízio Clima ↔ Câmeras | O card do meio alterna Clima e cada câmera (60 s cada; `?rotate=15` acelera para teste). Para no modo noturno e com a aba escondida | `/api/cameras` | `widgets/cameras.js`, `services/cameraPlayer.js` |
| Atalhos | Rodapé com os links; o da Geladeira vira alerta | `settings.js` (`links`) | `widgets/links.js` |
| Alerta de validades | "1 vencido · 3 a vencer" / "Tudo em dia" no atalho Geladeira | `/api/estoque` | `widgets/expiring.js`, `core/stock.js` |
| Rodapé | "Atualizado às HH:MM · V1.0.x" e uso de CPU/memória do hub | `/api/health`, `/api/system` | `widgets/holidays.js` (`footer`, `sysText`) |
| Modo noturno | Das 23h às 5h (configurável): tela escura com clima e eventos de amanhã; botão para desligar até o fim da noite | relógio | `widgets/night.js`, `core/app.js` |
| Anti burn-in | Desloca o conteúdo ±4 px a cada 3 min | — | `core/app.js` (`shift`) |
| Escala TV/celular | Palco fixo 1920×1080 escalado para caber; coluna única se largura < 900 px ou retrato | — | `core/app.js` (`fit`) |
| Controle remoto | **OK** atualiza tudo; **Voltar** (tecla 10009 do controle Samsung) fecha janelas e volta ao painel | — | `core/app.js` |
| Funcionamento offline | Sem internet usa o último clima em cache (`localStorage`) e marca como desatualizado | — | `core/app.js`, `core/util.js` |
| Sem configuração | Agenda e trânsito mostram dados de **exemplo** com a etiqueta "exemplo" | — | `services/calendar.js`, `services/`{text, history}` → `{say, actions[camera|traffic|agenda|list], source, provider}`Api.js` |

Intervalos de atualização: clima a cada `refreshMinutes` (15), trânsito 5 min, agenda 5 min, feriados 6 h, estoque 5 min,
sistema 10 s, tela 30 s.

### 2.2 Geladeira e Dispensa
- Campos: nome, quantidade e/ou peso (kg), validade, local (`freezer`, `geladeira`, `dispensa`). Detalhes e regras em
  [estoque-contatos.md](estoque-contatos.md).
- Lista ordenada pela validade, filtro por local, busca por nome, formulário em janela (`core/ui.js`, sem `<dialog>` nativo).
- Cor da validade: vermelho (vencido), âmbar (até `estoque.soonDays` dias, padrão 3), verde.

### 2.3 Serviços
- Campos: nome, tipo de serviço, contato (vira link `tel:`/`mailto:`/site), indicação, serviço, obs.
- Filtros por tipo, busca em todos os campos, tipos já usados sugeridos no formulário.
- Sincronização opcional com uma aba do Google Planilhas (planilha é a fonte; edições locais são sobrescritas).

### 2.4 Câmeras
- Vídeo ao vivo (H.264, MSE, sem áudio), um bloco por câmera com o nome em cima. Para quando a aba fica em segundo plano.
- Aba **Gravações**: lista por dia com filtro por câmera, **Assistir** (player com barra de busca) e **Baixar**.
- Detalhes, configuração das câmeras Intelbras e limites em [cameras.md](cameras.md).

### 2.5 Vídeos
- Lista a pasta de vídeos do servidor (mais recentes primeiro), com tamanho, data e indicação de legenda.
- **Tocar aqui** (mp4/webm/mov/mkv, legenda `.srt`/`.vtt`), **Abrir no VLC** (intent no Android, `vlc-x-callback` no iOS, `.m3u` no PC), **Copiar link**, **Link direto**.
- Os arquivos são entregues pelo nginx em `:8080/videos/`; o hub só lista os nomes.

### 2.5.1 Livros (PDF e EPUB)
- Os arquivos ficam na pasta `~/Livros` do servidor (`books.dir`); `books.maxMB` limita o tamanho de cada envio (padrão 300 MB). Mais recentes primeiro.
- **Enviar livro**: um ou mais `.pdf`/`.epub` de uma vez, com **categoria** e **dono** (listas da Config). O hub confere o conteúdo (PDF começa com `%PDF-`; EPUB é um zip com `container.xml`) e acrescenta “(2)” a nomes repetidos.
- Filtros por categoria (chips), por dono e busca. O lápis troca categoria e dono; a lixeira exclui o arquivo.
- **PDF** abre numa aba do navegador. **EPUB** abre no leitor da página: o hub abre o zip (`archive/zip`), devolve o sumário e serve cada capítulo, e a página mostra o capítulo num `<iframe sandbox="allow-same-origin">`. O hub manda uma política (CSP) que proíbe scripts do livro. O último capítulo lido fica no `localStorage` do aparelho.
- Categoria e dono de cada livro ficam em `.livros.json` dentro da pasta; as listas de categorias e donos e o **liga/desliga** ficam na Config > Livros (`panel.books`). Desligada, o atalho some do menu e a API responde 503 (lista/envio) ou 404 (arquivos).

### 2.6 Config
- Medidores: CPU e memória do hub e da máquina (atualiza a cada 5 s).
- **Gravação das câmeras**: chave geral e, por câmera: liga/desliga, modo (contínuo ou só movimento), sensibilidade 1–5, guardar por N dias, limite em GB.
- **A Config tem subseções (abas)**: Servidor · Câmeras · Agendas e rotas · Painel · Menu · Integrações · Idioma · Compatibilidade · Ajuda. Tudo que era fixo em arquivo agora se edita aqui e fica em `hub.settings.json` (ao lado do `hub.config.json`, que continua valendo para o que ainda não foi editado). Segredos (senhas, chaves, tokens) voltam mascarados; campo em branco = manter.
  - **Câmeras**: cadastro (nome + endereço RTSP), gravação por câmera, pasta das gravações e envio ao Drive (ligar, destino do rclone, hora, tamanho máximo).
  - **Agendas e rotas**: agendas iCal e **pessoas do trânsito** (adicionar/remover), cada uma com nome, horário de chegada, rotas (origem, destino, dias, "a partir de") e a **regra de consulta da API**: *só quando eu pedir* (botão "Ver trânsito", como o Diogo), *automático numa janela de horário* com "esconder depois de" (como a Pessoa 2) ou *sempre*.
  - **Painel** (vale na hora, até 5 min nas telas abertas): localização/clima e intervalo de atualização, modo noturno, tempo do rodízio das câmeras, dias do aviso de validade, **locais do estoque** (um local só sai se estiver vazio), endereço da página Vídeos e feriados locais.
  - **Livros**: liga/desliga a página e define as listas de categorias e de donos (uma por linha; vale na hora).
  - **Menu**: atalhos do rodapé (o da Config não pode ser removido).
  - **Integrações**: planilha de profissionais, contas de jogos (Steam, PSN, Xbox, RetroAchievements), pasta de vídeos, tempos de cache e IPv4. **Essas opções do hub só valem depois de reiniciar o hub**: a tela mostra "aguardando reinício" e um botão que reinicia pelo systemd.
  - **Ajuda**: manual completo de cada configuração, com busca e índice (`js/pages/configHelp.js`); cada aba tem o link "Ajuda desta aba". Ao mudar uma opção, atualize o texto desse arquivo.
  - **Idioma** (só no aparelho): português, inglês, espanhol ou chinês (`js/core/i18n.js`, `js/i18n/`). *Automático* segue o idioma do navegador; a escolha fica em `localStorage` (`cd:lang`) e também vale `?lang=en`. Traduz a interface, datas/números, os nomes de fábrica do menu/locais do estoque, os feriados nacionais e as mensagens conhecidas do hub; o que o usuário cadastrou fica como foi escrito.
  - **Compatibilidade** (só no aparelho): em navegador de TV (user agent: Samsung/Tizen, LG/webOS, Chromecast...) ou forçado, a página Câmeras mostra uma câmera por vez (`js/core/compat.js`).
  - Ainda fica em arquivo: `listen`/`staticDir`/`dbPath`/`allowedOrigins` e o executável do rclone.
- **Modo compatibilidade** (último cartão, vale só para o aparelho): em navegador de TV (detectado pelo user agent: Tizen, webOS, Chromecast etc.; ou forçado), a página Câmeras mostra uma câmera por vez, com botões para trocar (`js/core/compat.js`).
- **Liberar memória**: *Desligar o desktop* (display manager) e *Desativar a central* (para o hub). Exigem dois toques (botão arma por 5 s, o segundo confirma) e só funcionam no Linux com a regra do polkit instalada pelo `install.sh`.


## 3. API do hub

Todas as rotas respondem JSON (exceto vídeo/arquivos) com `Cache-Control: no-store` e CORS aberto (o hub só é acessível na rede de casa).

| Método e caminho | Função | Código |
|---|---|---|
| `GET /api/health` | `{ok, time, version}` — usado pelo painel para detectar o hub | `main.go` |
| `GET /api/version` | `{version, installedAt}` do arquivo `VERSION` (`dev` fora do servidor) | `main.go`, `version.go` |
| `GET /api/ics` | Texto iCal de cada agenda configurada (com cache e fallback "stale") | `calendars.go` |
| `GET /api/`{text, history}` → `{say, actions[camera|traffic|agenda|list], source, provider}`[?ask=id,id]` | `people: [{id, label, onDemand, result}]` (tempo, nível, trajeto codificado); quem é "só quando eu pedir" só vem com `?ask=` (`?diogo=1` e as chaves `diogo`/`pessoa2` continuam por compatibilidade) | `traffic.go` |
| `GET /api/games[?refresh=1]` | Conquistas/troféus Steam, PSN, Xbox, RetroAchievements (subsistema sem tela; ver [jogos.md](jogos.md)) | `games*.go` |
| `GET /api/games/xbox-login` · `POST` `{url}` | URL de login Microsoft · conclui a autorização do Xbox | `main.go`, `games_xbox.go` |
| `GET/POST /api/estoque` · `GET/PUT/DELETE /api/estoque/{id}` | CRUD de itens | `db.go`, `estoque.go` |
| `GET/POST /api/contatos` · `GET/PUT/DELETE /api/contatos/{id}` | CRUD de profissionais | `db.go`, `contatos.go` |
| `GET /api/cameras` | `[{id, name}]` (a URL RTSP com senha nunca sai do hub) | `cameras.go` |
| `GET /api/cameras/{id}/stream` | MP4 fragmentado contínuo (cabeçalho `X-Codec`) | `cameras.go` |
| `GET/PUT /api/settings/{cameras,calendars,`{text, history}` → `{say, actions[camera|traffic|agenda|list], source, provider}`,menu,panel,hub}` | Lê/grava o que a página Config edita; PUT exige `X-CD-Action: confirm`; segredos mascarados na leitura. `panel` mescla por grupo; `hub` só vale após reiniciar | `settings.go`, `settings_panel.go` |
| `POST /api/settings/hub/restart` | Reinicia o hub (systemd sobe de novo); exige `X-CD-Action: confirm` | `settings_panel.go` |
| `GET /api/recordings` | Lista dos arquivos de gravação | `recorder.go` |
| `GET /api/recordings/file/{nome}[?download=1]` | Arquivo MP4 com suporte a Range | `recorder.go` |
| `GET /api/recordings/settings` | Estado e configuração das gravações, por câmera | `recorder.go` |
| `PUT /api/recordings/settings` | Altera a configuração (exige `X-CD-Action: confirm`) | `recorder.go` |
| `GET /api/recordings/drive` | Estado da cópia diária para o Drive | `drive.go` |
| `POST /api/recordings/drive/run` | Copia para o Drive agora; corpo opcional `{"cameras":[ids]}` (exige `X-CD-Action: confirm`) | `drive.go` |
| `POST /api/recordings/drive/skip` | Marca como já enviadas as pendentes mais velhas que N dias (exige `X-CD-Action: confirm`) | `drive.go` |
| `GET /api/videos` | Lista da pasta de vídeos | `videos.go` |
| `GET /api/videos/sub?name=` | Legenda do vídeo em WebVTT (converte SRT) | `videos.go` |
| `GET /api/books` | Lista dos livros (`name`, `size`, `time`, `type`, `category`, `owner`) | `books.go` |
| `PUT /api/books/file/{nome}?category=&owner=` | Envia um livro (corpo cru do arquivo; exige `X-CD-Action: confirm`) | `books.go` |
| `PATCH /api/books/file/{nome}` | Troca categoria e dono, corpo `{"category","owner"}` (exige `X-CD-Action: confirm`) | `books.go` |
| `GET /api/books/file/{nome}` | Entrega o PDF (inline) ou EPUB; `?download=1` baixa | `books.go` |
| `DELETE /api/books/file/{nome}` | Exclui o livro (exige `X-CD-Action: confirm`) | `books.go` |
| `GET /api/books/epub/{nome}/toc` | Título e capítulos do EPUB | `books.go` |
| `GET /api/books/epub/{nome}/x/{arquivo}` | Arquivo de dentro do EPUB (HTML, CSS, imagens, fontes; sem scripts) | `books.go` |
| `GET /api/system` | CPU/memória da máquina e do hub, `control`, `desktop` | `sysinfo.go` |
| `POST /api/admin/desktop-off` | Para o display manager (exige `X-CD-Action: confirm`) | `power.go` |
| `GET /api/ai/hardware` | GPUs (nome, fabricante, integrada, memória dedicada/compartilhada), memória efetiva para um modelo, modelos sugeridos e catálogo | `ai/hardware*.go`, `ai/catalog.go`, `ai/routes.go` |
| `POST /api/omega/ask` | `{text, history}` → `{say, actions[camera\|traffic\|agenda\|list], source, provider}`; origem do hub, 20/min | `omega.go`, `omega_data.go`, `ai/llm.go` |
| `GET /api/ai/status` | Provedor ativo; Ollama alcançável, versão e modelos instalados (cabe/não cabe na GPU); Groq configurada (nunca devolve a chave) | `ai/status.go` |
| `GET` / `PUT /api/settings/ai` | Aba IA da Config: provedor, endereço do Ollama, modelo, modelo e chave da Groq (`groqKeySet`; chave vazia = manter, `clearGroqKey` apaga), nome pelo qual o Omega chama o usuário (`userName`; vazio = sem tratamento) e voz de síntese (`voice`; vazio = automática) | `settings_ai.go` |
| `POST /api/admin/hub-off` | Para o serviço do hub (exige `X-CD-Action: confirm`) | `power.go` |
| `/` | Arquivos estáticos do painel (`staticDir`), com `Cache-Control: no-cache` | `main.go` |

### Proteções de escrita
- `/api/estoque` e `/api/contatos`: exigem `Content-Type: application/json` e, se o navegador mandar `Origin`, ele precisa ser o do próprio hub ou estar em `allowedOrigins` (`db.go: checkWrite`).
- Gravações e botões de energia: exigem o cabeçalho `X-CD-Action: confirm`, que um formulário de outro site não consegue enviar sem preflight.
- Corpos de requisição limitados (64 KB); nomes de arquivo vindos da URL nunca viram caminho livre (validados por regex/lista).


## 4. Rotinas em segundo plano do hub

| Rotina | Frequência | O que faz | Código |
|---|---|---|---|
| Atualização de jogos | ao iniciar e a cada `games.cacheMinutes` (360) | Consulta as 4 plataformas e grava cache/tokens em `games.state.json` | `games.go: loop/refresh` |
| Sincronização da planilha | ao iniciar e a cada `profissionaisSheet.minutes` (5) | Baixa o CSV, cria/atualiza/apaga profissionais; não altera nada se a planilha falhar ou vier vazia | `sheet.go: sheetLoop` |
| Amostragem do sistema | 3 s | Lê CPU/memória (Linux `/proc`, Windows API); consulta o desktop a cada 4ª amostra | `sysinfo.go: loop` |
| Varredura das gravações | 10 min e a cada mudança de configuração | Apaga gravações vencidas e aplica o limite de GB (só arquivos com o padrão de nome) | `recorder.go: sweep` |
| Sessão RTSP das câmeras | sob demanda | Abre a câmera quando há espectador ou gravação; fecha 15 s após o último; reconecta em 4 s se cair | `cameras.go: run/session` |
| Detecção de movimento | por quadro | Compara o tamanho do quadro P com a média; dispara com 3 de 8 quadros acima do limite; pré-roll até o quadro-chave e 15 s de cauda | `motion.go` |
| Rotação de arquivo | virada da hora | Fecha o MP4 e abre o seguinte no primeiro quadro-chave | `recorder.go: rotateLocked` |
| Cópia para o Google Drive | diária às 3:00 (`driveSync.hour`); confere a cada minuto | Envia as gravações novas (das câmeras com o envio ligado na página Config) com rclone para `CameraSeguranca/AAAA-MM-DD/CAMERA/`; divide arquivos acima de `maxFileMB`; apaga do Drive o que passou da retenção da câmera (a da página Config); até 3 tentativas se falhar (1 h e 4 h depois), até 1:00 do dia seguinte | `drive.go: start/run` |

## 5. Mapa do código

### 5.1 Hub (`server/`)

| Arquivo | Responsabilidade |
|---|---|
| `main.go` | Config (`hub.config.json`), rotas, CORS, servidor de arquivos, flag `-xbox-login` |
| `db.go` | Abertura do SQLite, migrações versionadas (`PRAGMA user_version`), CRUD genérico `crud`, `checkWrite` |
| `estoque.go` / `contatos.go` | Validação (`clean`) e SQL de itens e profissionais |
| `sheet.go` | Parser do CSV (cabeçalhos tolerantes), sincronização `syncContatos`, laço `sheetLoop` |
| `calendars.go` | Baixa as URLs iCal com cache, devolve a última boa se o Google falhar |
| ``{text, history}` → `{say, actions[camera|traffic|agenda|list], source, provider}`.go` | Google Routes: janelas de consulta, cache, nível de trânsito, polilinha decodificada e amostrada |
| `cameras.go` | Sessão RTSP compartilhada, MP4 fragmentado, cache do último GOP, `stream` HTTP |
| `recorder.go` | Gerente de gravações (`recMgr`), gravador por câmera, retenção, API de configuração e arquivos |
| `drive.go` | Cópia diária das gravações para o Google Drive (rclone), divisão de MP4 grande em partes, estado em `drive.state.json` |
| `motion.go` | Detector de movimento por tamanho de quadro |
| `videos.go` | Listagem de vídeos e conversão SRT → VTT |
| `books.go` | Livros PDF/EPUB: lista, envio, categoria/dono, exclusão e leitor de EPUB |
| `sysinfo.go` + `_linux/_windows/_other.go` | Amostragem de CPU/memória por plataforma |
| `power.go` | Parar desktop/hub via `systemctl` (polkit) |
| `games.go`, `games_psn.go`, `games_xbox.go`, `games_steam.go`, `games_retro.go` | Agregador de conquistas e um cliente por plataforma |
| `version.go` | Lê o arquivo `VERSION` |
| `install.sh`, `build.sh`, `package.sh`, `dvr-hub.service` | Compilar (4 arquiteturas Linux), empacotar e instalar como serviço |

### 5.2 Painel (`src/`)

| Camada | Arquivos | Exporta |
|---|---|---|
| Núcleo | `core/i18n.js`, `core/util.js`, `core/icons.js`, `core/ui.js`, `core/stock.js`, `core/app.js` | `CD.i18n` (`t`, `n` plural, datas, `user`, `holiday`, `srv`), `CD.util`, `CD.icons`, `CD.ui` (modal/campos), `CD.stock` (regras de validade), `CD.state` + laço principal |
| Idiomas | `i18n/pt.js`, `en.js`, `es.js`, `zh.js`, `srv.js` (mensagens do hub), `help.pt.js`/`help.en.js`/`help.es.js`/`help.zh.js` (manual da Config) | `CD.i18n.add(id, {...})`, `CD.i18n.addHelp(id, [...])` |
| Serviços | `services/weatherApi.js`, `holidaysApi.js`, ``{text, history}` → `{say, actions[camera|traffic|agenda|list], source, provider}`Api.js`, `calendar.js`, `hubApi.js`, `cameraPlayer.js` | `CD.weatherApi`, `CD.holidaysApi`, `CD.trafficApi`, `CD.calendar`, `CD.hubApi`, `CD.cameraPlayer` |
| Widgets (Início) | `widgets/agenda.js`, `weather.js`, `map.js`, `holidays.js` (selo + rodapé), `expiring.js`, `links.js`, `cameras.js`, `night.js` | `CD.widgets.*` |
| Páginas | `pages/boot.js`, `estoque.js`, `contatos.js`, `camaras.js`, `videos.js`, `config.js` | `CD.pages.*` (cada uma com `labelKey`, `subKey` (chaves do dicionário), `icon`, `mount`) |
| Configuração | `config/settings.example.js` (padrões), `config/settings.js` (local, ignorado pelo git), `config/holidays-sp.js` | `CD_SETTINGS`, `CD_SETTINGS_LOCAL`, `CD_LOCAL_HOLIDAYS` |
| Estilo | `css/main.css`, `css/pages.css` | — |

`hubApi.js` é o único ponto que fala com o hub nas páginas: `estoque`, `contatos` (CRUD), `cameras`, `recordings`, `videos`, `system`, `power`.
Erros viram `Error` com a mensagem do servidor (traduzida por `CD.i18n.srv` quando é uma das conhecidas em `i18n/srv.js`); há timeout de 12 s.

### 5.3 Plataformas e ferramentas

| Item | Função |
|---|---|

## 6. Configuração

| Arquivo | Onde | O que guarda |
|---|---|---|
| `hub.config.json` | servidor (`/opt/dvr/`) | Porta, pasta do painel, agendas iCal, trânsito, câmeras (RTSP com senha), chaves de jogos, planilha, pastas de vídeos/gravações, `dbPath`, `allowedOrigins`. **Nunca versionar** (está no `.gitignore`) |
| `hub.config.example.json` | repositório | Modelo para copiar |
| `settings.js` | `src/config/` | Local, cidade, atalhos, `cameras.rotateSeconds`, `estoque.soonDays`, `videos`, modo noturno |
| `VERSION` | servidor | `MAIOR.MENOR.PATCH`; o `install.sh` soma 1 ao último número |
| `dvr.db` | servidor | SQLite (WAL) com `itens` e `contatos`; o `install.sh` não mexe |
| `recordings.state.json` | servidor | Configuração das gravações por câmera |
| `drive.state.json` | servidor | Gravações já copiadas para o Drive e resultado da última rodada |
| `games.state.json` | servidor | Cache e tokens dos jogos (permissão 600) |

## 7. Testes

34 testes em Go (`cd server && go test ./...`): estoque/contatos/CRUD e proteção de origem (`store_test.go`), planilha (`sheet_test.go`),
câmeras (`cameras_test.go`), gravador, retenção e movimento (`recorder_test.go`), vídeos e legendas (`videos_test.go`), trânsito
(``{text, history}` → `{say, actions[camera|traffic|agenda|list], source, provider}`_test.go`), versão (`version_test.go`), sistema (`sysinfo_test.go`) e jogos (`games_test.go`). O painel não tem testes automatizados; as traduções têm um verificador (`node scripts/check-i18n.js`: chaves iguais nos quatro idiomas, parâmetros, chaves usadas no código e estrutura do manual).
