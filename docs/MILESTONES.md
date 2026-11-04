# Milestones

Um milestone por vez, do básico ao avançado. Cada um termina com algo que o dono vê funcionando. As tarefas de cada milestone estão em
`tao.tasks.json` (uma sprint por milestone, `S1` = M0 ... `S9` = M8) e rodam pelo **TeamAgent Orchestrator** com o time de `tao.json`.
Levantamento e motivos das decisões: [superdvr/levantamento-e-milestones.md](superdvr/levantamento-e-milestones.md).

## Decisões do dono (03/11/2026)

| Tema | Decisão |
|---|---|
| Servidor | **Windows e Linux**. Máquina de teste: Windows 11, Intel Iris Xe (sem NVIDIA, 11,9 GB de memória de vídeo compartilhada), 23,7 GB de RAM |
| Login | **Nenhum**, nas duas interfaces: é a interface da casa e precisa estar disponível na hora. Proteção = rede local (VPN para acesso de fora) |
| Nome | `centraldiogo` / "Central Diogo" vira **`dvr`** / **"DVR"** (módulo, binário, serviço, pastas, banco, textos) |
| Banco | Começa novo; migração remove as tabelas `desk_*` que vierem de um banco do DVRCentral |
| IA | Ollama local, com **opções de modelo conforme a memória de vídeo** detectada (tabela abaixo); Groq opcional; instalar o Ollama na máquina de teste só quando a M2 precisar |
| Rosto | YuNet (detecção) + SFace (assinatura de 128 números) com ONNX Runtime; **conferir as versões oficiais antes de implementar** (tarefa T035) |

### Modelos por memória de vídeo (ponto de partida; a T007 transforma em catálogo no código)

| Memória disponível para a GPU | Sugestão de modelo (Ollama) | Observação |
|---|---|---|
| Sem GPU utilizável / CPU | `qwen3:1.7b`, `llama3.2:3b` | Lento; Groq é a alternativa rápida |
| até 6 GB | `qwen3:4b`, `llama3.2:3b` | Iris Xe e similares (Vulkan) caem aqui na prática |
| 6–12 GB | `qwen3:8b`, `llama3.1:8b` | Bom equilíbrio com tool calling |
| 12–20 GB | `gpt-oss:20b`, `qwen3:14b` | `gpt-oss:20b` é o mesmo que o JARVIS usava na Groq |
| 24 GB ou mais | `qwen3:30b`, `gpt-oss:20b` com contexto maior | Sobra para whisper e rostos ao mesmo tempo |

Memória **compartilhada** (GPU integrada) conta pela metade na recomendação: ela disputa com o sistema. Em GPU Intel/AMD o Ollama usa **Vulkan**;
conferir com `ollama ps` se o modelo foi para a GPU (sem driver adequado ele cai em CPU sem avisar). Para ONNX Runtime (rostos):
CUDA em NVIDIA, **DirectML ou OpenVINO** em Intel/AMD no Windows, CPU como reserva.

## Time

| Agente (`tao.json`) | Cliente / nível | Fica com |
|---|---|---|
| `orquestrador` | Claude / medium | Planejar, quebrar tarefas, responder dúvidas do time |
| `arquiteto` | Claude / senior | Peças de risco: cliente de IA, ferramentas, memória, ações de sistema, `facesvc`, pipeline de vídeo |
| `eng-go` | Codex / medium | Servidor Go: rotas, banco, configuração, instaladores |
| `eng-front` | Claude / medium | Interface TV (`src/`) e PC (`src/pc/`), traduções da interface |
| `junior` | OpenCode / junior | Renomeações, documentação, tarefas mecânicas |
| `revisor` | Gemini / medium | Revisão (o orquestrador escolhe sempre um cliente diferente de quem fez) |
| `revisor-opencode` | OpenCode / medium | Revisão reserva |

## M0 — Fundação do DVR (sprint S1)
Pronto quando: o hub sobe com o nome **DVR** (`dvr-hub`, `dvr.db`, `/opt/dvr`, `%ProgramData%\DVR`); um banco antigo perde as tabelas `desk_*`;
a Config tem a aba **IA** com a GPU detectada, a memória de vídeo, os modelos sugeridos e o teste de conexão com o Ollama; o CI roda build,
vet, testes (Windows e Linux) e traduções em cada PR.

| Área | Dono |
|---|---|
| Renomear hub, instaladores, CI, migração, bloco `ai`, GPU/catálogo | `eng-go` |
| Aba IA da Config | `eng-front` |
| Renomear interface e documentos | `junior` |

## M1 — Interface PC sem IA (S2)
Pronto quando: `/pc/` abre o HUD do JARVIS servido pelo hub (sem Python, Node nem Electron), com câmeras ao vivo, agenda, clima, trânsito,
estoque a vencer e o estado do servidor (inclusive GPU); a TV continua igual e as duas se alcançam por atalho.

## M2 — Chat local com Ollama (S3)
Pronto quando: conversar com o Jarvis no HUD com resposta em streaming, 100% local, com o modelo escolhido entre os que cabem na GPU;
Groq funciona como alternativa configurada na Config.

## M3 — Ferramentas da casa e do JARVIS (S4)
Pronto quando: "o que vence essa semana?", "qual o telefone do eletricista?", "mostra a câmera da garagem", notícias, moedas e lembretes
funcionam; o que altera dados pede confirmação.

## M4 — Voz (S5)
Pronto quando: falar "Jarvis, ..." e ouvir a resposta com voz pt-BR do Piper; com whisper.cpp ligado, nada sai de casa.

## M5 — Memória (S6)
Pronto quando: o Jarvis lembra de uma preferência dita dias antes; as memórias aparecem e podem ser apagadas no HUD.

## M6 — Ações de sistema e extras (S7)
Pronto quando: processos, energia e comandos permitidos rodam só com token, confirmação e registro; visualizador 3D e modo programador
funcionam com o modelo local.

## M7 — Reconhecimento facial (S8, o extra)
Pronto quando: pessoas são cadastradas com fotos e consentimento; uma pessoa cadastrada que aparece numa câmera é reconhecida em poucos
segundos (análise só durante movimento); a TV mostra o selo, o Jarvis avisa por voz e responde "quem passou na garagem hoje?"; eventos de
desconhecidos expiram e uma pessoa pode ser apagada com todos os dados (LGPD).

## M8 — Empacotamento (S9)
Pronto quando: os instaladores de Windows e Linux detectam a GPU e oferecem Ollama, Piper, ffmpeg e `facesvc`; há documentação de hardware
mínimo e release no GitHub.

## Como o time gasta menos tokens
- `AGENTS.md` é o mapa do repositório (com `CLAUDE.md` e `GEMINI.md` apontando para ele): o agente não precisa explorar.
- Cada tarefa diz **quais arquivos** mexer e **como conferir** (comando de teste); o critério "pronto quando" fica no campo `done`.
- `.ignore` tira mockups de 3 MB e imagens das buscas dos agentes.
- Tarefas pequenas (um PR cada), nível do modelo conforme o risco (`junior` para o mecânico, `senior` só para o que é difícil).
- Sprints futuras ficam como **propostas**: só viram prontas quando a anterior fecha, para ninguém trabalhar em cima de código que ainda não existe.
