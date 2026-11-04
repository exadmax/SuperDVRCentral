# Geladeira e Dispensa + Serviços

Duas páginas do painel, cada uma com a sua jornada, abertas pelos itens **Geladeira** e **Serviços** do menu de atalhos
no rodapé do Início (junto de Jogos, Música e Vídeos). Têm cadastro completo (criar, ver, editar, excluir)
guardado em um banco **SQLite** no hub. Funcionam no navegador da TV, do celular e do computador: todos falam com o mesmo hub,
então o que se cadastra em um aparelho aparece nos outros.

- `estoque.html`: Geladeira e Dispensa
- `contatos.html`: Serviços

## Geladeira e Dispensa

| Campo | Regra |
|---|---|
| Nome | obrigatório (até 120 caracteres) |
| Quantidade e/ou peso (kg) | **pelo menos um** dos dois; aceita decimais; zero é válido |
| Validade | opcional (`AAAA-MM-DD`); sem validade o item vai para o fim da lista |
| Local | `freezer`, `geladeira` ou `dispensa` |

- A lista vem ordenada pela validade (o que vence primeiro no topo), com filtro por local e busca pelo nome.
- Cores da validade: **vermelho** = vencido, **âmbar** = vence em até `estoque.soonDays` dias (padrão 3, em
  `src/config/settings.js`), verde = dentro do prazo.
- O item **Geladeira** do menu do Início avisa sem precisar abrir a página: fica vermelho ou âmbar com o resumo
  ("1 vencido · 3 a vencer"), ou mostra "Tudo em dia". O aviso vem de `alert: 'estoque'` em `links` (`settings.example.js`)
  e só aparece com o hub ligado e itens cadastrados; senão o item é um atalho comum.

## Serviços

| Campo | Regra |
|---|---|
| Nome | obrigatório |
| Tipo de serviço | obrigatório (eletricista, encanador...); o campo sugere os tipos já usados |
| Contato | opcional; telefone, e-mail ou site (vira link para ligar, escrever ou abrir) |
| Indicação | opcional; quem indicou, se recomenda ou não |
| Serviço | opcional; o que o profissional vai fazer (ou fez) |
| Obs | opcional; observações |

### Sincronização com o Google Planilhas

Se `profissionaisSheet.url` estiver no `hub.config.json`, a planilha é a **fonte**: o hub lê a aba em CSV ao ligar e a cada
`minutes` (padrão 5), cria os profissionais novos, atualiza os alterados e **apaga os que saíram da planilha**. Edições feitas
no painel são sobrescritas na próxima leitura, então edite pela planilha. Se o Google não responder ou a planilha vier
vazia, nada é alterado (o motivo vai para o log do hub).

- A planilha precisa estar com acesso "Qualquer pessoa com o link" (Leitor).
- URL: `https://docs.google.com/spreadsheets/d/<ID>/gviz/tq?tqx=out:csv&sheet=<nome da aba>`.
- Colunas pelo cabeçalho: Profissional/Empresa, Serviço, Contato, Referência, O que vai fazer, Obs (a ordem não importa).
- Um profissional é identificado por nome + serviço; linhas sem nome ou sem serviço são ignoradas.

Os tipos viram filtros (Eletricista, Encanador...) e a busca procura em todos os campos.

## Navegação

- As páginas seguem o mesmo molde da tela de Início: botão de voltar ao painel no topo e, na TV, o **Voltar** do
  controle (com uma janela de cadastro aberta, ele fecha a janela primeiro).
- Em telas grandes (TV, monitor) o conteúdo é ampliado para leitura à distância; no celular a lista vira coluna única.
- Endereços diretos: `http://192.168.0.10/estoque.html` e `http://192.168.0.10/contatos.html`.

## Banco de dados

- Arquivo `centraldiogo.db` ao lado do `hub.config.json` (em produção: `/opt/centraldiogo/centraldiogo.db`), criado e
  atualizado sozinho na primeira execução. O caminho muda em `dbPath` no `hub.config.json`.
- O `install.sh` **não mexe** no banco: reinstalar o hub preserva os dados.
- Driver em Go puro (`modernc.org/sqlite`, sem CGO), então `server/build.sh` continua gerando os 4 binários Linux.
  Compilar exige Go 1.26 ou mais novo. O binário ficou uns 5 MB maior.
- Se o banco não abrir, o resto do hub (agenda, clima, jogos) segue no ar e as duas páginas mostram o erro.
- **Backup** (o SQLite usa modo WAL, então não copie só o `.db` com o hub rodando):
  ```bash
  sudo systemctl stop centraldiogo-hub
  cp -a /opt/centraldiogo/centraldiogo.db* ~/backup-centraldiogo/
  sudo systemctl start centraldiogo-hub
  ```

## API (no hub)

`/api/estoque` e `/api/contatos`, JSON, mesmo padrão nos dois:

| Método | Caminho | Resposta |
|---|---|---|
| GET | `/api/estoque` | lista (`[]` quando vazio) |
| GET | `/api/estoque/{id}` | um registro, ou 404 |
| POST | `/api/estoque` | 201 com o registro criado |
| PUT | `/api/estoque/{id}` | 200 com o registro atualizado (substitui todos os campos) |
| DELETE | `/api/estoque/{id}` | 204 |

Erros vêm como `{"error": "mensagem"}` com 400 (dado inválido), 404, 415 ou 503 (banco indisponível).

Item: `{"nome", "quantidade", "pesoKg", "validade", "local"}`.
Contato: `{"nome", "tipoServico", "contato", "indicacao", "servico", "obs"}`.

```bash
curl -X POST http://192.168.0.10/api/contatos -H 'Content-Type: application/json' \
  -d '{"nome":"João","tipoServico":"Eletricista","contato":"(11) 98888-7777"}'
```

### Segurança

O hub é aberto à rede de casa (sem login), como o resto do painel. As gravações têm uma proteção extra: exigem
`Content-Type: application/json` e, quando o navegador envia o cabeçalho `Origin`, ele precisa ser o próprio endereço do
hub. Assim uma página de outro site aberta no celular de alguém da casa não consegue alterar o estoque. `curl` e
scripts (sem `Origin`) passam. Para liberar outra origem, por exemplo o painel em desenvolvimento na porta 5173 falando
com o hub na 8080, use `"allowedOrigins": ["http://localhost:5173"]` no `hub.config.json`.

## Vídeos (`videos.html`)
Lista a pasta de vídeos do servidor (`~/Videos`, a mesma que o nginx serve em `:8080/videos/`) com os botões **Tocar aqui** (player embutido, só aparece para formatos que o navegador toca: mp4/webm/mov/mkv), **Abrir no VLC**,
**Copiar link** e **Link direto**. O hub só lista os nomes (`GET /api/videos`, `server/videos.go`; pasta em `videos.dir` do `hub.config.json`);
o arquivo vem do nginx, sem copiar nada. Legenda com o mesmo nome (`.srt`/`.vtt`/`.ass`) é carregada junto.
- Android: abre o VLC direto (intent). iPhone/iPad: `vlc-x-callback`. PC: baixa um `.m3u` que o VLC abre.
- Porta e caminho do nginx: `videos: { port: 8080, path: '/videos/' }` em `settings.js`.
- O player embutido decodifica no aparelho (o servidor só entrega o arquivo). A legenda `.srt` é convertida para WebVTT pelo hub (`GET /api/videos/sub?name=`). Se o navegador não conseguir tocar (codec x265/AC3), aparece o aviso para abrir no VLC.

## Livros (`livros.html`)
Biblioteca de **PDFs e EPUBs** guardados no servidor (pasta `~/Livros`, campo `books.dir` do `hub.config.json`; `books.maxMB` limita cada envio, padrão 300 MB).
Tudo passa pelo hub (`server/books.go`), sem nginx.
- **Enviar livro**: um ou mais `.pdf`/`.epub`, com **categoria** e **dono** escolhidos nas listas da Config. O hub confere o conteúdo (PDF começa com `%PDF-`; EPUB é um zip com `container.xml`) e acrescenta “(2)” a nomes repetidos. O envio é o corpo cru do arquivo (`PUT /api/books/file/{nome}?category=&owner=`) e exige o cabeçalho `X-CD-Action: confirm`.
- **Filtros e busca**: chips por categoria, seletor de dono e campo de busca. O lápis troca categoria e dono (`PATCH /api/books/file/{nome}`); a lixeira exclui o arquivo.
- **PDF** abre numa aba do navegador. **EPUB** abre no leitor da página: o hub abre o zip (`GET /api/books/epub/{nome}/toc` devolve os capítulos, `.../x/{arquivo}` entrega cada arquivo de dentro), e a página mostra o capítulo num `<iframe sandbox="allow-same-origin">`, sem scripts: o iframe não tem `allow-scripts` e o hub manda uma política (CSP) que proíbe scripts, então nada de um EPUB de terceiros roda no painel. As setas ← → trocam de capítulo e o último lido fica no `localStorage` do aparelho.
- **Categoria e dono** de cada livro ficam em `.livros.json` dentro da pasta dos livros. As **listas** de categorias e de donos, e o **liga/desliga**, ficam em **Config → Livros** (`panel.books`, vale na hora). Desligada, o atalho some do menu, a página avisa e a API responde 503 (lista e envio) ou 404 (arquivos); os arquivos continuam na pasta.
- A pasta nasce no primeiro envio. Reinstalar o hub (`install.sh`) não mexe nela.
