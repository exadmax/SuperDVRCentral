# SuperDVRCentral

Versão do **DVRCentral / Central Diogo** para quem tem **GPU e um servidor mais potente**. Parte do mesmo painel da casa
(interface **TV**, a atual), sem a área de trabalho web (Diogows), e vai ganhar uma interface **PC** no estilo **J.A.R.V.I.S.**
(assistente por voz com LLM local via Ollama) e **reconhecimento facial** nas câmeras — tudo o que for possível em **Go** e com base **SQLite**.
Plano: **[docs/MILESTONES.md](docs/MILESTONES.md)** (tarefas em `tao.tasks.json`, time em `tao.json`, regras dos agentes em [AGENTS.md](AGENTS.md)); levantamento em [docs/superdvr/levantamento-e-milestones.md](docs/superdvr/levantamento-e-milestones.md).

> O texto abaixo descreve a base herdada do DVRCentral (interface TV) e ainda usa os nomes originais (`centraldiogo-hub`, `/opt/centraldiogo`).


Painel de informações da casa: agenda, clima, trânsito, feriados, câmeras ao vivo, estoque da cozinha, lista de profissionais, vídeos e biblioteca de livros (PDF e EPUB).
Roda em qualquer navegador (TV, celular, tablet ou computador), tudo a partir de um único código web em `src/`,
servido por um **hub** em Go que fica em um servidor Linux da casa (pode ser um mini-PC ou um PC antigo, com 2 GB de RAM).

> ⚠️ **Uso doméstico, sem login no painel.** O hub foi pensado para a rede da sua casa e o painel **não tem autenticação**: quem alcança a porta vê as câmeras
> e a Config. **Não exponha à internet.** Para acesso de fora, use uma VPN (Tailscale, WireGuard). Veja [SECURITY.md](SECURITY.md).

## Sumário
- [Funcionalidades](#funcionalidades)
- [Arquitetura](#arquitetura)
- [Estrutura do repositório](#estrutura-do-repositório)
- [Começando](#começando)
- [Configuração](#configuração)
- [Idiomas](#idiomas)
- [Implantação no servidor](#implantação-no-servidor)
- [Versão instalada](#versão-instalada)
- [Testes](#testes)
- [Documentação](#documentação)
- [Licença](#licença)

## Funcionalidades

| Área | O que faz | Página |
|---|---|---|
| **Agenda** | Eventos do dia do casal (Google Calendar via iCal), próximo evento com contagem regressiva | Início |
| **Clima** | Temperatura, previsão e aviso de guarda-chuva (Open-Meteo, sem chave) | Início |
| **Trânsito** | Tempo de deslocamento de cada pessoa cadastrada, horário de saída e mini-mapa | Início |
| **Feriados** | Selo nacional / estadual / municipal, ou "Fim de semana" / "Dia útil" | Início |
| **Câmeras** | Imagem ao vivo (RTSP H.264, sem ffmpeg); no Início alternam com o Clima a cada 1 min | `camaras.html` |
| **Gravações** | 1 arquivo MP4 por hora e por câmera, contínuo ou só com movimento, com retenção por dias e GB | Câmeras → Gravações |
| **Geladeira e Dispensa** | Itens com quantidade/peso, validade e local; avisa o que está vencendo | `estoque.html` |
| **Serviços** | Profissionais (eletricista, encanador...), com sincronização opcional com Google Planilhas | `contatos.html` |
| **Vídeos** | Lista a pasta de vídeos do servidor; toca no navegador ou abre no VLC | `videos.html` |
| **Livros** | PDFs e EPUBs no servidor, com envio pelo painel, categoria, dono e leitor de EPUB (desligável na Config) | `livros.html` |
| **Config** | CPU/memória do servidor; câmeras, agendas, trânsito, menu, painel, livros e integrações em abas, com ajuda embutida; desligar desktop/hub | `config.html` |
| **Modo noturno** | Das 23h às 5h: tela escura com clima e eventos de amanhã | Início |
| **Idiomas** | Português, English, Español e 中文, escolhido por aparelho (ou automático, pelo navegador) | Config → Idioma |

O inventário completo, com fontes de dados, endpoints e arquivos de código, está em **[docs/funcionalidades.md](docs/funcionalidades.md)**.

## Arquitetura

```
TV / celular / PC ─►  navegador  ─►  http://192.168.0.10/  ─►  Hub Go (server/)
                                                                 ├─ serve o painel (src/)
                                                       ├─ /api/*  agenda, trânsito, estoque, serviços, câmeras, vídeos, livros, sistema
                                                       ├─ SQLite  (centraldiogo.db)
                                                       └─ guarda as chaves e senhas (nunca vão para os aparelhos)
```

- **Hub** (`server/`, Go 1.26, sem CGO): roda no Linux na porta 80, só com biblioteca padrão mais SQLite em Go puro e RTSP.
  Pensado para máquinas de 2 GB (o serviço limita a memória a 400 MB). Os livros enviados ficam numa pasta do servidor (`~/Livros`), fora do banco. Guarda as URLs iCal, a chave do Google Routes, as senhas das câmeras
  e resolve o CORS do Google.
- **Painel** (`src/`): HTML e JS clássicos (sem módulos ES, para funcionar também em navegadores de TV). Aberto pelo hub em qualquer navegador.
- Para abrir na **TV**, use o navegador dela em `http://192.168.0.10/` (e na Config ligue o *Modo compatibilidade* se as câmeras não carregarem). Para mudar o painel basta reenviar o hub.
- **Sem hub**, o painel ainda abre e mostra clima e feriados; agenda e trânsito aparecem com dados de **exemplo** (etiqueta "exemplo")
  e as páginas de estoque, serviços, câmeras, vídeos, livros e config avisam que precisam do hub.
- O hub é aberto à rede de casa, **sem login**. Não exponha à internet.

## Estrutura do repositório

| Pasta | Conteúdo |
|---|---|
| `src/` | Painel web: `*.html`, `css/`, `js/{core,services,widgets,pages,i18n}`, `config/`, `assets/` |
| `server/` | Hub em Go, testes, `hub.config.example.json`, scripts de build/instalação e unidade systemd |
| `docs/` | Documentação por área e design de referência (`docs/design/`) |
| `scripts/` | Índice de build e empacotamento ([`scripts/README.md`](scripts/README.md)) e o verificador de traduções (`check-i18n.js`) |

## Baixar e instalar
Pacotes prontos nas **[Releases](https://github.com/exadmax/DVRCentral/releases/latest)** (a página de apresentação tem a [guia de download](https://exadmax.github.io/DVRCentral/#baixar)):
- **Windows** (`centraldiogo-windows.zip`): extraia e abra o `Instalar.cmd`. Uma **janela** pergunta se este PC será o *servidor da casa* (tarefa do sistema, sobe com o Windows, atende a rede) ou de *uso pessoal* (só `localhost`, sem administrador). Dados em `%ProgramData%\CentralDiogo` (servidor) ou `%LocalAppData%\CentralDiogo` (pessoal), mantidos ao atualizar. Desinstale em Aplicativos do Windows.
- **Linux** (`centraldiogo.tgz`): `tar xzf centraldiogo.tgz && sudo bash install.sh`.
Limite no Windows: os botões da Config que param o desktop e o hub (`systemctl`) são só do Linux.

## Começando

### Só o painel (dados de exemplo)
```bash
python -m http.server 5173 --directory src
```
Abra http://localhost:5173. TV/web usa um palco 1920×1080 escalado; celular ou retrato usa coluna única.
Esse modo não tem hub, então só vale para ver o layout.

### Com o hub (tudo funcionando)
Requer **Go 1.26+**.
```bash
cd server
go run .
```
Abra http://localhost:8080 e vá em **Config**: câmeras, agendas, trânsito, menu, cidade e integrações se configuram pela própria tela
(há uma aba **Ajuda** com o passo a passo). Sem `hub.config.json` o hub sobe com os padrões; se preferir configurar por arquivo,
copie `hub.config.example.json` para `hub.config.json` e rode `go run . -config hub.config.json`.
O painel fica em `../src` (rode a partir da pasta `server/`). O banco `centraldiogo.db` e o `hub.settings.json` (o que você edita na Config) são criados ao lado do `hub.config.json`.

Para o painel em outra porta falar com o hub (ex.: `python -m http.server 5173`), acrescente
`"allowedOrigins": ["http://localhost:5173"]` ao `hub.config.json`, senão as gravações são recusadas.

## Configuração

### No hub: `server/hub.config.json` (ignorado pelo git)
Referência completa em `server/hub.config.example.json`.

| Campo | Para quê |
|---|---|
| `listen`, `staticDir` | Porta (`:80` em produção) e pasta do painel |
| `calendars[]` | URLs iCal secretas do Google Calendar (`who`, `label`, `color`, `url`) |
| `traffic` | Chave do Google Routes, endereço de casa e trajetos de cada pessoa por dia da semana |
| `cameras[]` | `id`, `name` e URL RTSP (com senha; **use `subtype=1`**, H.264) |
| `recordings.dir` | Pasta das gravações (padrão `~/securityCam`) |
| `videos.dir` | Pasta de vídeos (padrão `~/Videos`) |
| `books.dir`, `books.maxMB` | Pasta dos livros (padrão `~/Livros`) e tamanho máximo de cada envio (padrão 300 MB) |
| `profissionaisSheet` | CSV do Google Planilhas e intervalo de sincronização (vazio desliga) |
| `games` | Chaves de Steam, PSN, Xbox e RetroAchievements (subsistema sem tela) |
| `dbPath`, `allowedOrigins`, `forceIPv4` | Banco SQLite, origens extras para gravar, uso de IPv4 nas consultas externas |

Depois de editar: `sudo systemctl restart centraldiogo-hub`.

### No painel: `src/config/settings.js` (opcional, ignorado pelo git)
É mesclado sobre `settings.example.js`; coloque só o que mudar.
```js
window.CD_SETTINGS_LOCAL = {
  cameras: { rotateSeconds: 60 },                 // tempo de cada câmera no rodízio
  estoque: { soonDays: 3 },                       // "a vencer" = até N dias
  night: { from: 23, until: 5 }                   // modo noturno
};
```
Também define a cidade, os atalhos do rodapé (`links`) e a porta/pasta dos vídeos (`videos`).

## Idiomas

O painel e as páginas (inclusive a Config e o manual da aba Ajuda) existem em **Português (pt)**, **English (en)**, **Español (es)** e **中文 (zh, chinês simplificado)**.

- **Como escolher**: em **Config → Idioma**. Vale **só para aquele aparelho** (fica guardado no navegador dele): a TV pode ficar em português e o celular em inglês.
  A opção *Automático* (padrão) segue o idioma do navegador e, se ele não for um dos quatro, usa português. Também dá para forçar pelo endereço:
  `index.html?lang=en` (`pt`, `en`, `es`, `zh`); `?lang=auto` volta ao automático.
- **O que é traduzido**: textos da interface, datas e números no formato de cada idioma, os nomes de fábrica do menu, dos locais do estoque e das agendas de exemplo,
  os feriados nacionais e os avisos de erro do hub que são conhecidos. **O que você cadastrou** (nome de câmera, agenda, item, profissional, atalho renomeado, feriado local)
  e os eventos do Google Agenda aparecem exatamente como foram escritos. Os avisos de erro do hub ainda não conhecidos aparecem em português.
- **Onde fica**: `src/js/core/i18n.js` (núcleo: `CD.i18n.t`, plurais, datas, escolha do idioma) e `src/js/i18n/` (`pt.js`, `en.js`, `es.js`, `zh.js`, o manual `help.<idioma>.js`
  e `srv.js`, que traduz mensagens do hub reconhecendo o texto original em português).
- **Mudou ou criou um texto?** Escreva-o primeiro em `pt.js` (é a referência e o idioma de reserva), traduza nos outros três arquivos e rode
  `node scripts/check-i18n.js`: ele aponta chave faltando, `{parâmetro}` diferente, chave usada no código sem tradução e diferença de estrutura no manual.
- **Outro idioma**: acrescente-o em `LANGS` (`i18n.js`), crie `src/js/i18n/<id>.js` e `help.<id>.js` com as mesmas chaves e inclua os `<script>` nas páginas `*.html`.

## Implantação no servidor

No PC de desenvolvimento:
```bash
cd server
./package.sh        # gera dist/centraldiogo.tgz (4 binários Linux + painel + instalador)
```
No servidor Linux, após copiar e extrair o pacote:
```bash
sudo bash install.sh           # instala em /opt/centraldiogo, libera a porta 80 e ativa o serviço
bash install.sh --check        # só mostra a versão instalada e a próxima (sem sudo, sem alterar nada)
```
O `install.sh` **não toca no banco, na pasta de livros**: reinstalar preserva estoque, serviços e livros. Ele também grava a regra do polkit que permite
à página Config parar o desktop e o hub. Comandos úteis: `systemctl status centraldiogo-hub`, `journalctl -u centraldiogo-hub -f`.

**Backup do banco** (modo WAL: não copie só o `.db` com o hub rodando):
```bash
sudo systemctl stop centraldiogo-hub
cp -a /opt/centraldiogo/centraldiogo.db* ~/backup-centraldiogo/
sudo systemctl start centraldiogo-hub
```

## Versão instalada
- As versões publicadas seguem SemVer (`vMAIOR.MENOR.PATCH`) e saem como **Releases** do GitHub, com o pacote e os binários prontos. Política em **[docs/versionamento.md](docs/versionamento.md)**.
  Num pacote de release, o `install.sh` grava no servidor a versão da própria release; o contador abaixo vale só para pacotes montados do código.
- O servidor guarda a versão no arquivo `VERSION` (`/opt/centraldiogo/VERSION`), formato `MAIOR.MENOR.PATCH`, por exemplo `1.0.1`.
- Cada `sudo bash install.sh` **concluído** soma 1 ao terceiro número e registra a linha em `install.log`. Instalação que falha no meio
  não muda a versão. Sem o arquivo, a primeira instalação vira `V1.0.1`. Para subir o primeiro ou o segundo número, edite o `VERSION` à mão.
- Para conferir: `bash install.sh --check`, `curl localhost/api/version` (o `/api/health` também traz a versão) ou o rodapé do painel
  ("Atualizado às 21:30 · V1.0.1").
- Fora do servidor (`go run`, sem `VERSION`) o hub informa `dev`.

## Testes
```bash
cd server && go test ./...
```
Cobrem CRUD e validação de estoque/serviços, proteção de origem, sincronização da planilha, câmeras e gravador (retenção, movimento),
vídeos e legendas, livros (envio, categoria/dono, leitor de EPUB, liga/desliga), trânsito, versão e métricas do sistema. O painel não tem testes automatizados; só há o verificador das traduções:
```bash
node scripts/check-i18n.js
```

## Documentação

| Documento | Assunto |
|---|---|
| [docs/funcionalidades.md](docs/funcionalidades.md) | **Mapa de todas as funcionalidades**, API, rotinas e código |
| [docs/cameras.md](docs/cameras.md) | Câmeras ao vivo, rodízio, gravações, modelos Intelbras |
| [docs/licencas.md](docs/licencas.md) | Componentes de terceiros embutidos e suas licenças |
| [docs/estoque-contatos.md](docs/estoque-contatos.md) | Geladeira, Serviços, planilha, banco, API e segurança; páginas Vídeos e Livros |
| [docs/jogos.md](docs/jogos.md) | Subsistema de conquistas (sem tela) |
| [docs/spec-design.md](docs/spec-design.md) | Especificação visual (TV 1920×1080) |
| [docs/decisoes.md](docs/decisoes.md) | Decisões em aberto |
| `docs/design/` | Mockups HTML e guia de estilo |

## Licença
Distribuído sob a **GNU GPL v3** (veja [LICENSE](LICENSE)). Você pode usar, estudar, modificar e redistribuir; versões modificadas
distribuídas devem continuar sob a mesma licença e com o código aberto.

## Contribuindo
Issues e pull requests são bem-vindos. Rode `cd server && go vet ./... && go test ./...` antes de enviar. O painel usa JS e CSS clássicos
(sem módulos ES e sem recursos novos de CSS) para funcionar em navegadores de TV; mantenha esse cuidado. Nunca inclua endereços reais,
chaves, senhas ou dados da sua casa em exemplos, testes ou capturas de tela.

## Aviso sobre integrações
Steam, PlayStation, Xbox, RetroAchievements, Google (Agenda, Routes, Planilhas, Drive) e BrasilAPI são serviços de terceiros, com termos
próprios. As integrações de PlayStation (cookie `npsso`) e Xbox (cliente público do app do Xbox) usam acesso **não oficial** e podem
parar de funcionar ou violar os termos do serviço: use por sua conta e risco. Este projeto não é afiliado a nenhuma dessas empresas.
