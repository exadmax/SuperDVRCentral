# SuperDVRCentral: levantamento e milestones

> Data: 03/11/2026. Fontes lidas: **DVRCentral** (`exadmax/DVRCentral`, cópia local em `centraldiogo`, commit `04e8720`),
> **O-J.A.R.V.I.S.** (`exadmax/O-J.A.R.V.I.S`) e **face_recognition** (`exadmax/face_recognition`, fork do `ageitgey/face_recognition`).

## 0. Resumo e decisões

| Tema | Decisão proposta |
|---|---|
| Base | Cópia do DVRCentral **sem o Diogows** (feita, ver §1). Hub em Go, **SQLite** (`modernc.org/sqlite`, sem CGO) continua como banco único |
| Interfaces | **TV** = o painel atual (`src/`, JS clássico, roda em navegador de TV). **PC** = nova interface "Jarvis" (HUD + voz + chat), servida pelo mesmo hub |
| Linguagem | Tudo que for servidor vai para **Go**: o `app.py`/`server.js` (chat e agente) e o `backend/main.py` (ferramentas, voz, memória) do JARVIS somem. O que fica fora do Go: o HUD (HTML/JS no navegador) e os **modelos** (LLM, voz, rosto), que rodam em motores nativos |
| LLM | **Ollama no lugar da Groq: sim, dá** (ver §5). O JARVIS já fala o protocolo OpenAI (`/chat/completions` com streaming e tool calling), que o Ollama também expõe. Groq fica como alternativa opcional, configurável |
| Rosto | Porte **conceitual** do face_recognition (ele é um embrulho Python do dlib): detector + extrator de "assinatura" de 128 números + comparação por distância, em Go. Inferência num **processo à parte** com GPU, para o hub continuar sem CGO (ver §4) |
| Ordem | Do básico ao avançado: fundação → interface PC → chat local → ferramentas da casa → voz → memória → ações de sistema → **reconhecimento facial (extra)** |

## 1. O que já foi feito nesta cópia

Cópia dos arquivos versionados do DVRCentral (mais as alterações locais pendentes de `.gitignore` e `server/windows/install.ps1`), **sem** binários,
banco, `server/dist/` e `server/demo/`. Depois, retirada total do **Diogows** (a área de trabalho web):

| Removido | O que era |
|---|---|
| `server/desk*.go` (32 arquivos, incluindo testes) | Usuários/login, Explorer, terminal e sudo, calendário por usuário, chat, e-mail IMAP/SMTP/OAuth, navegador interno, VMs/DOS, perfil, ícones de sites, atualizações de componentes, **planta da casa com TVs Samsung** (`desk_home.go`) |
| `server/office.go` (+ teste) | Cabeçalhos COOP/COEP do LibreOffice em WebAssembly |
| `src/desktop.html`, `office.html`, `emu.html`, `emulator.html`, `src/js/desktop/`, `src/js/office/`, `src/css/desktop.css` | Telas do Diogows |
| `src/vendor/` (JS Paint, js-dos, Webamp, ZetaJS) | Componentes de terceiros só do Diogows (≈ 880 arquivos) |
| `docs/desktop.md`, `email.md`, `aplicativos.md`, `emuladores.md`, `atualizacoes.md`, `novidades.md`, `index.html`, `apresentacao.html` e as imagens `desktop-*` | Documentação e site do DVRCentral ligados ao Diogows |
| Dependências Go `go-imap/v2`, `go-message`, `go-sasl` | Só o e-mail usava (`go mod tidy`) |

Ajustes: `main.go` (sem `newDesk`, sem `protectSettings`, sem isolamento do Office), `db.go` (migrações `desk*` saíram do fim da lista:
banco novo não ganha as tabelas `desk_*`), `install.sh` (sem `--desk-shell`), `hub.config.example.json` (sem bloco `desk`), instalador Windows
(sem a senha inicial do `admin`), atalho "Diogows" do menu, chaves de tradução `def.desktop*`, `SECURITY.md`, `README.md`, `licencas.md` e `funcionalidades.md`.

Conferido: `go build`, `go vet` e `go test ./...` passam; `node scripts/check-i18n.js` OK (632 chaves × 4 idiomas); o instalador PowerShell não tem erro de sintaxe.

> **Atenção:** um banco já usado pelo DVRCentral (com `user_version` 11 e as tabelas `desk_*`; restaram 4 migrações) **abre normalmente** (as migrações que restam já
> foram aplicadas), mas as tabelas `desk_*` ficam lá, órfãs. O SuperDVRCentral deve começar com banco novo ou ganhar uma migração de limpeza (M0).

> **Para resgatar depois:** `desk_home.go` (planta da casa + estado das TVs Samsung) é um bom candidato a voltar como **ferramenta do Jarvis**
> ("Jarvis, desliga a TV da sala"). Está no histórico do DVRCentral.

## 2. Levantamento: DVRCentral (o que fica)

**Pilha:** hub Go 1.26 (biblioteca padrão + `modernc.org/sqlite`, `gortsplib`/`mediacommon`/`pion` para RTSP e `golang.org/x/net`), sem CGO,
pensado para 2 GB de RAM (o serviço limita o hub a 400 MB). Painel em HTML/JS clássico (sem módulos ES, para navegador de TV), 4 idiomas.
Sem login: só para a rede de casa.

| Área | O que faz | Código (servidor / painel) | Vira ferramenta do Jarvis? |
|---|---|---|---|
| Câmeras ao vivo | RTSP H.264 → fMP4 para o navegador, **sem decodificar** (sem ffmpeg) | `cameras.go` / `camaras.html`, `widgets/cameras.js`, `services/cameraPlayer.js` | "mostra a câmera da garagem" |
| Gravações | 1 MP4 por hora e por câmera, contínuo ou por movimento, retenção por dias/GB | `recorder.go` | "o que foi gravado hoje à tarde?" |
| Movimento | Detecção **sem decodificar** (tamanho dos quadros P em relação à média) | `motion.go` | **Gatilho do reconhecimento facial** (§4) |
| Cópia para o Drive | Envio diário das gravações via rclone | `drive.go` | — |
| Agenda | iCal do Google (RRULE, EXDATE...) | `calendars.go` / `widgets/agenda.js` | "o que tenho amanhã?" |
| Trânsito | Google Routes por pessoa e dia da semana | `traffic.go` / `widgets/map.js` | "quanto tempo até o trabalho?" |
| Clima / feriados | Open-Meteo e BrasilAPI (no navegador) | `services/weatherApi.js`, `holidaysApi.js` | Precisa de versão Go para o agente |
| Geladeira e Dispensa | CRUD com validade (SQLite `itens`) | `db.go` / `estoque.html` | "o que está vencendo?", "acabou o leite" |
| Serviços | Profissionais, sincroniza com Google Planilhas | `db.go`, `sheet.go` / `contatos.html` | "qual o telefone do eletricista?" |
| Vídeos / Livros | Lista pasta de vídeos; PDF/EPUB com envio e leitor | `videos.go`, `books.go` | "abre o livro X" |
| Config | Configuração pela tela (`hub.settings.json`), CPU/memória, desligar display manager e hub | `settings*.go`, `sysinfo*.go`, `power.go` / `config.html` | "como está o servidor?" |
| Conquistas de jogos | Steam/PSN/Xbox/RetroAchievements, sem tela | `games*.go` | Opcional |
| Modo noturno, idiomas | 23h–5h; pt/en/es/zh | `widgets/night.js`, `core/i18n.js` | — |

API completa (31 rotas) em [../funcionalidades.md](../funcionalidades.md). Banco: tabelas `itens` (estoque) e `profissionais`; configuração em JSON.

**O que reaproveitar direto no Jarvis:** os *handlers* já devolvem JSON pronto (`/api/ics`, `/api/traffic`, `/api/estoque`, `/api/contatos`,
`/api/cameras`, `/api/recordings`, `/api/system`...). As ferramentas do agente podem chamar as mesmas funções Go **dentro do processo**, sem HTTP.

## 3. Levantamento: O-J.A.R.V.I.S. (o que portar)

**Pilha atual (pesada):** três processos e três linguagens. Python FastAPI (`backend/main.py`, 113 KB, porta 8000) para ferramentas, voz e memória;
Python ou Node (`app.py` 93 KB / `server.js` 60 KB, porta 8755) para chat/agente e arquivos estáticos; Electron para a janela; ChromaDB + MiniLM
(sentence-transformers/PyTorch ou ONNX) para a memória; Piper ou Edge TTS para a voz; Three.js r128 para o globo 3D. O `.exe` empacotado tem ~260 MB.
Pensado para **Windows, uso pessoal** (`/exec` roda comandos do shell, `/power` desliga o PC). Licença MIT.

| Componente | Hoje | No SuperDVRCentral (Go) |
|---|---|---|
| HUD (`public/index.html`, `app.js` 100 KB, `styles.css` 56 KB, Three.js) | Servido pelo Python/Node | **Mantém em HTML/JS**, servido pelo hub em `/pc/`. É a interface PC. Three.js (MIT) volta como vendor só dela |
| Chat com streaming (`/api/chat`, SSE) | `app.py` → Groq (`openai/gpt-oss-120b`, `gpt-oss-20b`, `qwen3.8-27b`) | Pacote Go `ai`: cliente **OpenAI-compatível** (Ollama por padrão, Groq opcional), SSE com os mesmos eventos (`start`, `delta`, `tool`, `done`) para o HUD mudar pouco |
| Agente: detecção por regex (`detect_tool`) + tool calling (14 ferramentas) | Python | Go: mesmas regras de atalho por regex (respostas rápidas sem LLM) + `tools` no formato OpenAI |
| Personalidade (`system_prompt.txt`) | Arquivo | Arquivo embutido + editável na Config |
| Clima (`/weather`, cidade fixa São João do Paraíso-MG) | wttr.in/Open-Meteo | Unificar com o clima do painel (Open-Meteo, cidade da Config) |
| Notícias (`/news`, `/news/brasil`) | RSS G1 / Google News | Go `encoding/xml` |
| Moedas e ações (`/currencies`, `/stocks`) | APIs públicas | Go `net/http` |
| Agenda/lembretes (`agenda.json`) | Arquivo JSON | **Tabela SQLite** `lembretes`, junto com a agenda iCal da casa |
| Memória (`memory.py`: ChromaDB, 10 mil memórias, arquivamento após 90 dias, bloqueio de senhas) | Chroma + MiniLM | **SQLite** (`memorias` com vetor em BLOB) + embeddings pelo **Ollama** (`nomic-embed-text` ou `all-minilm`); busca por cosseno em Go (10 mil vetores = milissegundos). Regras de `memory.py` portadas |
| Voz de saída (`/tts`) | Piper (Python) ou Edge TTS | **Piper como executável** chamado pelo Go (sem Python), vozes `.onnx` pt-BR (cadu, faber). Edge TTS opcional (protocolo WebSocket, dá para portar) |
| Voz de entrada / wake word "Jarvis" | **Web Speech API do navegador** (Chrome manda o áudio para o Google) | Fase 1: igual. Fase 2: **whisper.cpp** (servidor local com GPU), áudio vai do navegador para o hub. Fica tudo dentro de casa |
| Sistema (`/sysinfo`, `/processes`, `/kill`, `/exec`, `/power`) | Windows, com token `X-Jarvis-Token` e lista negra | `sysinfo.go` já existe. `exec`/`kill`/`power` passam a valer **para o servidor**: desligados por padrão, só com token, lista de comandos permitidos e confirmação |
| Visualizador 3D e "modo programador" (`/visualize-3d`, `/generate-code`) | LLM gera HTML | Mesma ideia, LLM local; abre em iframe com sandbox |
| Electron, `launcher.py`, PyInstaller, `installer.iss` | Empacotamento Windows | **Sai.** A interface PC é uma página do hub (dá para "instalar" como PWA) |

**Ganho de memória esperado:** de Python + Node + Electron + PyTorch/Chroma (centenas de MB a mais de 1 GB) para **um binário Go** (dezenas de MB)
mais os motores de modelo (Ollama, Piper, whisper), que só ocupam memória quando estão em uso e podem ficar na GPU.

## 4. Levantamento: face_recognition (o "extra")

O repositório é o `ageitgey/face_recognition` (MIT): um **embrulho Python fino** sobre o **dlib** (C++). Não há algoritmo próprio a portar, só o fluxo:

| Função Python | O que faz | Modelo dlib |
|---|---|---|
| `face_locations` | Acha rostos (HOG no CPU ou CNN na GPU) | `mmod_human_face_detector` |
| `face_landmarks` | 5 ou 68 pontos do rosto (alinhamento) | `shape_predictor_5/68` |
| `face_encodings` | Gera a "assinatura" do rosto: **128 números** | `dlib_face_recognition_resnet_model_v1` |
| `compare_faces` / `face_distance` | Distância euclidiana; mesmo rosto se < **0,6** | — |
| Exemplos `facerec_ipcamera_knn.py`, `face_recognition_knn.py` | Câmera IP + KNN com várias fotos por pessoa | — |

**Como trazer para Go (opções):**

| Opção | Prós | Contras |
|---|---|---|
| **A. ONNX Runtime + modelos YuNet (detecção) e SFace (128-d)**, num serviço Go à parte (`facesvc`) | GPU via CUDA; modelos pequenos e de licença livre (YuNet MIT, SFace Apache-2.0); mesmo conceito (128 números + distância) | Precisa de CGO **só no `facesvc`** |
| B. `go-face` (dlib via CGO) | Mesmos modelos do face_recognition, resultados iguais | CGO + dlib para compilar; quase só CPU |
| C. Manter o Python como microserviço | Zero porte | Contraria "tudo em Go" e o objetivo de memória |

**Recomendação: A.** O hub continua **sem CGO** e leve. O `facesvc` (Go + ONNX Runtime com GPU) recebe quadros e devolve rostos e assinaturas.
Evitar os modelos pré-treinados da InsightFace (ArcFace "buffalo"): a licença deles é só para uso não comercial.

**Fluxo na câmera (sem decodificar à toa):**

```
RTSP ─► hub (cameras.go) ─► motion.go detecta movimento (sem decodificar)
                               │ só então
                               ▼
              decodifica 2–5 quadros/s do substream (ffmpeg com NVDEC, processo à parte)
                               ▼
              facesvc: YuNet acha rostos ─► alinha ─► SFace gera 128 números
                               ▼
              hub compara com os rostos cadastrados (SQLite) ─► "Maria (0,31)" ou "desconhecido"
                               ▼
              evento + foto recortada (SQLite + pasta) ─► TV (selo) / Jarvis ("Comandante, Maria chegou")
```

O decodificador é a peça nova: hoje o hub **não decodifica vídeo** (repassa H.264). Não existe decodificador H.264 em Go puro maduro; o caminho
prático é o **ffmpeg** como processo filho com aceleração da NVIDIA, ligado só enquanto há movimento.

**Dados (SQLite):** `pessoas` (id, nome), `rostos` (pessoa, assinatura em BLOB de 128 floats, foto de origem, qualidade), `rosto_eventos`
(câmera, horário, pessoa ou desconhecido, distância, recorte). Várias fotos por pessoa melhoram a precisão (vizinho mais próximo, como no exemplo KNN).

**LGPD:** rosto é **dado biométrico sensível**. Cadastro só com consentimento, retenção curta dos eventos de desconhecidos, tudo local, opção de apagar a pessoa e os dados dela.

## 5. Ollama no lugar da Groq

**Dá, e com pouca mudança.** O JARVIS chama `https://api.groq.com/openai/v1/chat/completions`, o formato da OpenAI, com `stream` e `tools`.
O Ollama expõe o **mesmo formato** em `http://<servidor>:11434/v1/chat/completions`, com streaming e tool calling (versões recentes), sem chave.
Trocar é mudar a URL base e o nome do modelo. Além disso:

- **Mesmo modelo disponível:** o JARVIS usa `openai/gpt-oss-20b` na Groq; o Ollama tem `gpt-oss:20b` (cerca de 14–16 GB de VRAM). Para GPU menor:
  `qwen3:8b` ou `llama3.1:8b` (cerca de 6–8 GB), ambos com tool calling.
- **Embeddings pelo mesmo servidor** (`/api/embed`): substitui o ChromaDB + MiniLM da memória.
- **Privacidade:** nada sai de casa (hoje o texto vai para a Groq e, com Edge TTS, para a Microsoft).
- **O que se perde:** velocidade e tamanho do modelo. A Groq é muito rápida e roda o `gpt-oss-120b`; localmente depende da GPU.
  Por isso o cliente Go deve aceitar **as duas** (Ollama padrão, Groq opcional com chave), escolhidas na Config.
- **Escrever em Go:** usar HTTP puro (o protocolo é simples) ou a biblioteca oficial `github.com/ollama/ollama/api`; preferir o formato OpenAI
  para manter Groq e outros compatíveis.
- **A conferir no servidor de verdade:** a versão do Ollama instalada e qual modelo cabe na GPU (esta máquina de desenvolvimento não tem GPU nem Ollama instalados).

## 6. Arquitetura alvo

```
TV  ─► /            painel atual (src/)                ┐
PC  ─► /pc/         HUD Jarvis (src/pc/, Three.js)     ├─► Hub Go (server/) ── SQLite (superdvr.db)
                                                      │     ├─ API da casa (câmeras, agenda, estoque...)
                                                      │     ├─ ai/      agente, ferramentas, memória ──► Ollama :11434 (LLM + embeddings)
                                                      │     ├─ voz/     TTS e STT ──────────────────────► piper / whisper.cpp (processos)
                                                      │     └─ faces/   cadastro, comparação, eventos ──► facesvc (Go + ONNX/GPU) + ffmpeg/NVDEC
```

Princípios herdados do DVRCentral: um binário Go sem CGO para o hub, configuração pela tela, segredos só no servidor, sem expor à internet.
Os componentes de GPU ficam **fora do processo do hub**: se um modelo travar ou faltar GPU, o painel da TV continua no ar.

## 7. Milestones

Cada milestone termina com algo que funciona e pode ser usado. As ferramentas e telas sempre degradam bem quando falta GPU ou Ollama.

### M0. Fundação do SuperDVRCentral
- Renomear módulo, binário, serviço systemd, pastas (`centraldiogo` → `superdvr`), textos e porta padrão; manter compatibilidade de dados opcional.
- Migração de limpeza das tabelas `desk_*` antigas (ou exigir banco novo).
- Bloco `ai` na configuração (URL do Ollama, modelo, provedor opcional Groq) e aba na Config com teste de conexão.
- CI (build, vet, test, check-i18n) apontando para `exadmax/SuperDVRCentral`.
- **Pronto quando:** o hub sobe com o nome novo, testes no CI verdes, Config mostra "Ollama: conectado / modelo X".

### M1. Interface PC (sem IA)
- Trazer o HUD do JARVIS (`public/`) para `src/pc/`, servido pelo hub, sem Python nem Node.
- Os painéis do HUD mostram dados reais da casa via `hubApi`: câmeras ao vivo, agenda, clima, trânsito, estoque a vencer, CPU/GPU do servidor.
- Escolha TV × PC: endereço (`/` e `/pc/`) e atalho no menu.
- **Pronto quando:** abrir `/pc/` num PC mostra o HUD com o globo e os dados da casa; a TV continua igual.

### M2. Chat local com Ollama
- Pacote Go `ai`: cliente OpenAI-compatível com streaming; `/api/jarvis/chat` (SSE) no formato que o HUD já espera.
- Personalidade (`system_prompt.txt`), histórico da conversa, limite de taxa, lista de modelos do Ollama (`/api/tags`).
- Groq como alternativa (chave guardada no servidor, mascarada na API).
- **Pronto quando:** conversar com o Jarvis pelo HUD, com resposta em streaming, 100% local.

### M3. Ferramentas da casa e do JARVIS
- Leitura primeiro: clima, agenda, trânsito, feriados, estoque, serviços, câmeras e gravações, sistema, vídeos e livros (funções Go do próprio hub).
- Portadas do JARVIS: notícias, moedas, ações, lembretes (SQLite).
- Escrita com confirmação: adicionar/baixar item do estoque, criar lembrete.
- Atalhos por regex para pedidos comuns (respondem sem chamar o LLM).
- **Pronto quando:** "Jarvis, o que vence essa semana?" e "mostra a câmera da garagem" funcionam.

### M4. Voz
- Saída: Piper como executável chamado pelo hub (`/api/jarvis/tts`), vozes pt-BR.
- Entrada: Web Speech do navegador (como hoje) e, como opção local, whisper.cpp na GPU (`/api/jarvis/stt`); palavra de ativação "Jarvis".
- **Pronto quando:** falar "Jarvis, que horas é minha próxima reunião?" e ouvir a resposta, sem serviço de fora (modo whisper).

### M5. Memória persistente
- Tabela `memorias` no SQLite com embeddings do Ollama; busca por cosseno em Go; regras do `memory.py` (o que vale lembrar, bloqueio de segredos,
  arquivamento após 90 dias, limite de 10 mil).
- Tela no HUD para ver e apagar memórias.
- **Pronto quando:** o Jarvis lembra de uma preferência dita dias antes.

### M6. Ações de sistema e extras do JARVIS
- `exec`, processos, `kill` e energia **no servidor**: desligados por padrão, token, lista de comandos permitidos, confirmação e log.
- Visualizador 3D e modo programador com o LLM local (iframe com sandbox).
- Opcional: resgatar a planta da casa e o controle das TVs Samsung como ferramenta.
- **Pronto quando:** as ações perigosas só rodam com token e confirmação, e ficam registradas.

### M7. Reconhecimento facial (extra)
- **M7a – cadastro:** `facesvc` (Go + ONNX Runtime, CUDA) com YuNet + SFace; tela de pessoas e fotos; assinaturas no SQLite; testes com as imagens
  de exemplo do face_recognition (mesma pessoa < limite, pessoas diferentes > limite).
- **M7b – ao vivo:** movimento → ffmpeg/NVDEC (2–5 quadros/s) → `facesvc` → comparação → evento com recorte; limite de câmeras e de quadros por segundo.
- **M7c – integração:** selo na TV ("Maria chegou"), aviso falado do Jarvis, pergunta "quem passou na garagem hoje?", retenção e exclusão (LGPD).
- **Pronto quando:** uma pessoa cadastrada aparece na câmera e o sistema a reconhece em poucos segundos, sem pesar no painel.

### M8. Empacotamento
- Instalador Linux com checagem de NVIDIA, CUDA, Ollama, Piper, whisper e ffmpeg; documentação de hardware mínimo; release no GitHub.

## 8. Riscos e decisões em aberto

> **Atualização 03/11/2026:** os itens 1, 2, 4 e 5 foram decididos pelo dono (Windows e Linux, máquina de teste com Intel Iris Xe, sem login,
> nome `dvr`, modelos conforme a memória de vídeo). Valem as decisões de [../MILESTONES.md](../MILESTONES.md); o texto abaixo fica como registro.

1. **GPU do servidor:** modelo e VRAM definem o LLM (gpt-oss:20b × qwen3:8b), se whisper e rostos rodam juntos, e quantas câmeras dá para analisar.
2. **Sistema do servidor:** o DVRCentral é Linux-first (systemd, polkit). Confirmar se o servidor potente é Linux ou Windows.
3. **CGO no `facesvc`:** aceito por ficar isolado do hub? (A alternativa é ainda mais pesada.)
4. **Nome técnico:** renomear `centraldiogo` em tudo (serviço, pastas, banco) ou manter os nomes internos para facilitar migrar dados do DVRCentral.
5. **Segurança:** a interface PC terá voz, memória e ações; o painel da TV não tem login. A interface PC deve ganhar **login** (simples, só ela), já que o Diogows saiu.
6. **Arquivos pessoais copiados:** `.claude/launch.json`, `.codex/agents/operador-infra-vovozinho.toml` e `.agents/skills/debian` vieram do DVRCentral;
   decidir se ficam neste repositório.
