# Omega (assistente por voz)

Página `omega.html`, atalho **Omega** no menu. Simula uma conversa com um assistente no estilo J.A.R.V.I.S.: você fala (ou digita)
“Omega, …” e ele responde falando e mostrando o resultado em painéis 3D (three.js) ao redor de um núcleo de partículas que reage à voz.

**Só para computador e celular.** Em navegadores de TV (Tizen, webOS e outros reconhecidos por `js/core/compat.js`) o atalho some do
menu e a página mostra um aviso: falta microfone, WebGL confiável e desempenho para a cena 3D.

## O que ele entende hoje

| Pedido (exemplos) | O que acontece |
|---|---|
| “Omega, me mostra a câmera da sala” | Painel com a câmera ao vivo (o mesmo vídeo da página Câmeras) |
| “Omega, como está o trânsito até o trabalho da Cássia?” | Painel com minutos, nível, atraso, horário de saída e o trajeto; fala o resumo |
| “Omega, quais os compromissos de hoje?” (ou “de amanhã”) | Painel com a agenda do dia e a lista falada (até 5 itens) |
| “Omega, tem algo vencido no estoque?” | Fala o que já venceu e o que vence nos próximos 7 dias; painel com a lista (vencidos em vermelho) |
| “Omega, o que tem na geladeira?” / “tem ovo no estoque?” | Lista o estoque (todo, de um local ou de um produto) com quantidade, local e validade |
| “Omega, guarda 2 queijos na geladeira, vencem dia 10 de dezembro” | Grava o item no estoque (precisa de local e de quantidade e/ou peso; se faltar, ele pergunta) |
| “Omega, tira o iogurte” / “usei 2 ovos” | Remove o item, ou só baixa a quantidade; se mais de um item combinar, pergunta qual |
| “Omega, tem eletricista nos serviços?” / “preciso de um encanador” | Procura nos **Serviços** (profissionais) por tipo, nome ou assunto; painel com nome, tipo e contato |
| “Omega, cadastra o João eletricista, telefone …” / “apaga o Marcos dos serviços” | Cadastra ou apaga um profissional (não repete nome + tipo já cadastrados) |
| Qualquer outra pergunta | Resposta curta da IA (Groq ou Ollama, conforme a aba **IA** da Config) |

Pedidos de câmera, trânsito, agenda e **consultas** ao estoque e aos serviços são reconhecidos **por regra no hub** (rápido e sem
gastar a IA). O resto vai para o modelo, que também pode escolher mostrar câmera, trânsito ou agenda e, para estoque e serviços, usar as
ferramentas `stock_list`, `stock_expired`, `stock_add`, `stock_remove`, `services_search`, `service_add` e `service_remove`: o hub executa a
ferramenta (com a mesma validação das páginas Estoque e Serviços), devolve o resultado ao modelo e ele responde (até 4 idas e vindas por
pedido). **Adicionar e remover só funcionam com a IA ligada**; sem ela o Omega responde que não conseguiu falar com a IA. Os nomes vêm do
que está cadastrado na Config (câmeras, pessoas do trânsito, locais do estoque); acentos e plural não importam (“cassia” = “Cássia”,
“ovo” = “Ovos”).

## Como usar

- **Microfone** (botão à esquerda): escuta contínua. Diga “Omega” e o pedido na mesma frase, ou “Omega” sozinho e o pedido em
  seguida (8 s). Usa a Web Speech API do navegador: funciona no **Chrome** e no **Edge** (no Chrome o áudio passa pelo serviço do
  Google; no Edge, pelo da Microsoft). O painel de navegador embutido do Claude bloqueia o microfone: abra no Chrome/Edge.
- **Fala**: síntese de voz do próprio sistema, no idioma do aparelho (prefere as vozes “naturais” do Edge/Windows). 🔊/🔇 liga e desliga.
- **Webcam** (opcional): mostra a prévia no canto e serve de sensor de presença: depois de 90 s sem movimento, quando alguém aparece
  o Omega cumprimenta. A imagem não sai do aparelho.
- **Texto**: digite na barra de baixo ou toque numa sugestão.
- **Painéis**: até 3 ao mesmo tempo; arrastar gira a cena, clicar num painel traz para a frente, **Esc** ou ✕ limpa.

## Servidor

`POST /api/omega/ask` (`server/omega.go`), corpo `{"text": "...", "history": [...]}` (até 6 mensagens anteriores), resposta
`{"say", "actions": [{"type": "camera"|"traffic"|"agenda"|"list", ...}], "source": "rule"|"llm"|"none", "provider"}`
(`list` = painel pronto do estoque ou dos serviços: `{"title", "rows": [{"text", "sub", "warn"}], "more"}`). Exige a origem do próprio
hub (a rota gasta cota da Groq), limite de 20 pedidos por minuto. Cliente de IA em `server/ai/llm.go` (formato OpenAI, Groq ou Ollama).

## Código

| Arquivo | O quê |
|---|---|
| `src/omega.html`, `src/css/omega.css` | Página e HUD |
| `src/js/omega/scene.js` | Cena three.js: núcleo, anéis, painéis holográficos, arrastar/clicar |
| `src/js/omega/voice.js` | Reconhecimento com palavra de ativação, síntese de voz, nível do microfone |
| `src/js/omega/omega.js` | Conversa com o hub, executa as ações, webcam/presença, sugestões |
| `src/vendor/three/` | three.js r128 (MIT) |
| `server/omega.go`, `server/ai/llm.go` | Entendimento do pedido (regras + IA com ferramentas) |
| `server/omega_data.go` | Estoque e serviços: regras de consulta, ferramentas do LLM e as alterações |

## Próximos passos (milestones)

Voz local com Piper/whisper (M4), memória (M5) e mais ferramentas (M3: lembretes, notícias) entram no Omega.
