# Central Diogo — Especificação de design

## Contexto
Painel de informações da casa, exibido no navegador da **TV (1920×1080), uso doméstico**, lido a 2–3 metros de distância ("10-foot UI"). Celular e tablet usam o mesmo painel web, responsivo a partir do mesmo layout.
Não é interativo no dia a dia: fica ligado como painel passivo. Controle remoto só para trocar de tela/atualizar.

## Princípios
- Legível de longe: texto mínimo 28px, números principais 72–120px.
- Uma olhada responde: "o que tenho hoje? vou levar guarda-chuva? quanto tempo até o trabalho? é feriado?"
- Identidade visual baseada no logotipo **Omega Diogo** (arquivo do logo anexo): fundo azul-marinho quase preto, neon ciano e verde com degradê e leve glow. Nome do app: **Omega Diogo**.
- Paleta (extraída do logo, ajustar por amostragem): fundo #0B1220, cartões #121B2D, texto #EAF6FF, ciano primário #00C8FF, verde secundário #3DFF7A, degradê de destaque ciano→verde (esquerda→direita, 135deg), alerta âmbar #FFB020, feriado/erro #FF6B6B.
- Uso das cores: ciano para informação (hora, clima, títulos), verde para "ok/livre/bom" (trânsito fluindo, sem chuva), degradê ciano→verde em bordas de cartão, barras e no anel de destaque do próximo evento. Glow discreto (blur 12–20px, opacidade 30%) só em elementos de destaque, nunca no texto corrido.
- Motivo gráfico: anel/arco em linhas finas duplas inspirado no ômega do logo, usado como moldura do relógio e como indicador de progresso (ex.: tempo até o próximo evento).
- Tema escuro obrigatório, sem branco puro (evita brilho e burn-in).
- Fonte sans-serif geométrica em caixa alta para títulos, como no logo (Montserrat/Poppins), pesos 500/600/700; números em Montserrat para combinar.
- Cantos arredondados 20px, espaçamento 24px, sem sombras pesadas.
- Anti burn-in: deslocamento de 1–2px do conteúdo a cada poucos minutos, sem elementos fixos brilhantes.
- Sem interação por toque; foco visível (borda azul 4px) para navegação por setas do controle.

## Layout (1920×1080, grade de 12 colunas, margem 48px)
**Barra superior (altura 120px)**
- Esquerda: hora grande (HH:MM) e data por extenso ("Quarta, 23 de setembro").
- Centro: selo de feriado quando houver (nacional / estadual / municipal, com cor por tipo) ou "Dia útil".
- Direita: clima atual (ícone + temperatura) e cidade.

**Coluna esquerda (5/12) — Agenda de hoje**
- Lista cronológica de eventos: horário, título, quem (pessoa 1 / pessoa 2 / ambos, com bolinha de cor por pessoa).
- Próximo evento destacado (cartão maior, contagem regressiva "em 1h20").
- Estado vazio: "Nada agendado hoje".
- Máx. 6 itens visíveis; rolagem automática lenta se houver mais.

**Coluna central (4/12) — Clima do dia**
- Temperatura atual grande, mín/máx, sensação térmica.
- Faixa por hora (manhã, tarde, noite) com ícone, temperatura e % de chuva.
- Alertas em destaque âmbar: "Chuva às 16h — leve guarda-chuva".
- Nascer/pôr do sol e umidade em linha secundária.

**Coluna direita (3/12) — Deslocamento**
- Dois cartões: "Pessoa 1 → trabalho" e "Pessoa 2 → trabalho".
- Cada cartão: tempo estimado com trânsito (grande), tempo normal, diferença colorida (verde/âmbar/vermelho), hora de saída sugerida.
- Mini-mapa da rota com camada de trânsito (verde/amarelo/vermelho).

**Rodapé (altura 60px)**
- Última atualização, indicador de conexão, próximo feriado ("Faltam 12 dias para ...").

## Estados
- Carregando (esqueleto cinza pulsando), sem internet (faixa âmbar com dados em cache), erro por widget (cartão isolado com mensagem curta, o resto continua).
- Modo noturno (após 23h): brilho reduzido, mostra só hora, clima e agenda de amanhã.

## Responsivo
- Celular: coluna única, ordem Barra superior → Agenda → Clima → Deslocamento.
- Web/tablet: 2 colunas.

## Entregáveis esperados do design
1. Tela principal TV 1920×1080 (estado normal, dia útil).
2. Variação com feriado e com alerta de chuva.
3. Variação modo noturno.
4. Versão celular (390×844).
5. Guia de estilo: cores, tipografia, cartões, ícones de clima, estados.

