# AGENTS.md — SuperDVRCentral

Regras para todo agente (Claude Code, Codex, OpenCode, Gemini CLI) que trabalha neste repositório. **Leia só o que a tarefa pede**:
este arquivo é o mapa, não abra o repositório inteiro.

## O projeto
Painel da casa (câmeras, gravações, agenda, trânsito, estoque...) servido por um **hub em Go** com **SQLite**. Duas interfaces:
**TV** (`src/`, a atual) e **PC** (`src/pc/`, assistente "Jarvis" com chat, voz e memória, a partir da M1). IA local pelo **Ollama**.
Reconhecimento facial (M7) num serviço à parte (`facesvc/`). Sem login: é a interface da casa, disponível na hora, só na rede local.
Plano: `docs/MILESTONES.md`. Levantamento e decisões: `docs/superdvr/levantamento-e-milestones.md`.

## Mapa
| Onde | O quê |
|---|---|
| `server/main.go` | Configuração (`Config`), rotas, início do hub |
| `server/db.go` | SQLite: `migrations` (só acrescente no fim), `store`, estoque e serviços |
| `server/cameras.go`, `recorder.go`, `motion.go` | RTSP → fMP4 sem decodificar; gravação; movimento pelo tamanho dos quadros |
| `server/settings.go`, `settings_panel.go` | O que a página Config edita (`hub.settings.json`) |
| `server/calendars.go`, `traffic.go`, `books.go`, `videos.go`, `sheet.go`, `drive.go`, `games*.go` | Uma área por arquivo |
| `server/sysinfo*.go`, `power.go` | CPU/memória; desligar display manager e hub |
| `server/ai/`, `server/faces/` (a criar) | Agente/LLM/memória (M2–M5); cadastro e eventos de rostos (M7) |
| `src/*.html`, `src/js/{core,services,widgets,pages,i18n}` | Interface TV: JS **clássico** (sem módulos ES, roda em navegador de TV) |
| `src/pc/` (a criar) | Interface PC (HUD do JARVIS) |
| `docs/funcionalidades.md` | Inventário de rotas e funções (consulte a seção, não o arquivo todo) |

Ignore `docs/design/` (mockups de 3 MB cada) e `docs/img/`.

## Regras
1. **Escopo:** mexa só nos arquivos que a tarefa cita (ou que o código cita a partir deles). Não refatore o que a tarefa não pede.
2. **Go primeiro:** todo código de servidor em Go. O hub fica **sem CGO** (`CGO_ENABLED=0`); CGO só no `facesvc/`. Dependência nova só se a
   biblioteca padrão não resolver, com licença GPL/MIT/Apache/BSD.
3. **Banco:** só SQLite (`modernc.org/sqlite`). Mudança de esquema = nova entrada no fim de `migrations`.
4. **Memória:** o hub deve continuar leve (alvo 400 MB). Modelos (LLM, voz, rosto) rodam fora do processo do hub.
5. **Windows e Linux:** código de sistema com sufixo `_windows.go` / `_linux.go` / `_other.go`. Teste nos dois quando possível.
6. **Textos da interface** em `src/js/i18n/pt.js` primeiro, depois `en`, `es`, `zh`; rode `node scripts/check-i18n.js`.
7. **Comentários e docs em português**, no estilo do código vizinho. Identificadores como já estão.
8. **Segredos** (chaves, senhas de câmera) só no servidor; nunca em `src/`, logs ou respostas da API (mascarar).
9. **Antes de entregar:** `cd server && gofmt -l . && go vet ./... && go test ./...` limpos. Termine com uma linha de resumo.
10. Não faça commit nem push a menos que a tarefa peça (o orquestrador faz).

## Comandos
```bash
cd server && go run .                 # hub em http://localhost:8080 (painel em ../src)
cd server && go test ./...            # testes
node scripts/check-i18n.js            # traduções
```
