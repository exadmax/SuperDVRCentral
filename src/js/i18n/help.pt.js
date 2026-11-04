(function (CD) {
  // Manual da aba Ajuda da Config, em português (idioma padrão). Os outros idiomas ficam em help.en.js, help.es.js e help.zh.js.
  // O texto é HTML fixo; atualize aqui (e nos outros idiomas) quando uma opção mudar.
  const tip = t => '<div class="help-tip"><b>Dica:</b> ' + t + '</div>';
  const warn = t => '<div class="help-warn"><b>Atenção:</b> ' + t + '</div>';

  CD.i18n.addHelp('pt', [
    { id: 'comecar', title: 'Como a Config funciona', html: `
      <p>A Config é onde se ajusta tudo o que antes ficava escondido em arquivos: câmeras, agendas, trânsito, menu, cidade do clima, feriados e as contas de integração. Ela só funciona <b>aberta pelo endereço do servidor da casa</b> (o hub), porque é ele quem guarda as configurações.</p>
      <h3>As abas</h3>
      <table class="help-t"><tr><th>Aba</th><th>Para que serve</th></tr>
        <tr><td>Servidor</td><td>Uso de CPU e memória, e os botões para liberar memória.</td></tr>
        <tr><td>Câmeras</td><td>Cadastro das câmeras, gravação, pasta das gravações e envio ao Google Drive.</td></tr>
        <tr><td>Agendas e rotas</td><td>Agendas (iCal) e o trânsito de cada pessoa.</td></tr>
        <tr><td>Painel</td><td>Cidade e clima, modo noturno, rodízio das câmeras, validade, locais do estoque, vídeos e feriados locais.</td></tr>
        <tr><td>Livros</td><td>Liga ou desliga a página Livros e define as categorias e os donos dos livros.</td></tr>
        <tr><td>Menu</td><td>Os botões de atalho do rodapé da tela inicial.</td></tr>
        <tr><td>Integrações</td><td>Planilha de profissionais, contas de jogos, pasta de vídeos, tempos de cache e rede.</td></tr>
        <tr><td>Idioma</td><td>Idioma da interface: português, inglês, espanhol ou chinês (vale só no aparelho).</td></tr>
        <tr><td>Compatibilidade</td><td>Modo para TVs que só tocam um vídeo por vez (vale só no aparelho).</td></tr>
        <tr><td>Ajuda</td><td>Esta página.</td></tr></table>
      <h3>Salvar</h3>
      <ul><li>Cada cartão tem o seu botão <b>Salvar</b>. Salvar um cartão não salva os outros.</li>
        <li>Se o hub recusar um valor, aparece uma mensagem <span class="cfg-err">em vermelho</span> no próprio cartão e <b>o que você digitou continua lá</b> para corrigir.</li>
        <li>Botões <b>Remover</b> valem na hora (pedem confirmação). Linhas novas que você ainda não salvou somem sem falar com o servidor.</li></ul>
      <h3>Quando a mudança vale</h3>
      <table class="help-t"><tr><th>O que você mudou</th><th>Quando vale</th></tr>
        <tr><td>Câmeras, agendas, trânsito, menu</td><td>Na hora, no servidor. As telas já abertas atualizam o menu e o painel em até 5 minutos (ou ao recarregar).</td></tr>
        <tr><td>Aba Painel (cidade, noturno, rodízio, validade, locais, vídeos, feriados)</td><td>Na hora; telas abertas pegam em até 5 minutos.</td></tr>
        <tr><td>Pasta das gravações, Drive, planilha, jogos, pasta de vídeos, cache, IPv4</td><td><b>Só depois de reiniciar o hub.</b> A tela mostra uma faixa amarela “aguardando reinício” com o botão <b>Reiniciar o hub agora</b>.</td></tr>
        <tr><td>Idioma e modo compatibilidade</td><td>Na hora, só neste aparelho (a tela recarrega ao trocar o idioma).</td></tr></table>
      <h3>Onde fica guardado</h3>
      <p>Tudo o que você edita fica em <code>hub.settings.json</code>, na mesma pasta do <code>hub.config.json</code> no servidor. O arquivo de configuração original continua valendo para tudo o que você ainda não editou. Para <b>voltar uma seção ao original</b>, apague a chave dela em <code>hub.settings.json</code> (ou o arquivo todo) e reinicie o hub. Faça uma cópia do <code>hub.settings.json</code> como backup.</p>
      <h3>Segredos (senhas, chaves, tokens)</h3>
      <p>Senhas de câmeras, endereços secretos de agenda e chaves de API <b>nunca voltam para a tela</b>: aparecem mascaradas (<code>*****</code>) ou como “configurada”. Deixar o campo <b>em branco significa manter o valor atual</b>. Para trocar, digite o novo valor. Alguns campos têm a opção “apagar a configurada”.</p>
      ${warn('o hub é aberto à rede de casa e <b>não tem login</b>: qualquer pessoa conectada ao seu Wi-Fi consegue abrir a Config. Não exponha o hub à internet.')}` },

    { id: 'servidor', title: 'Servidor', html: `
      <h3>Medidores</h3>
      <p>Mostram o uso de CPU e memória do próprio hub e da máquina inteira, atualizados a cada 5 segundos. Se a memória da máquina estiver quase cheia, o painel pode ficar lento: use “Liberar memória”.</p>
      <h3>Liberar memória</h3>
      <ul><li><b>Desligar o desktop</b>: para o ambiente gráfico do servidor (tela, navegador, áudio do desktop). Não afeta o hub, o painel nem as câmeras. Volta sozinho no próximo reinício da máquina.</li>
        <li><b>Desativar a central</b>: para o hub. O painel, as câmeras e a agenda saem do ar até alguém rodar no servidor <code>sudo systemctl start centraldiogo-hub</code>.</li></ul>
      <p>Os dois botões pedem <b>dois toques</b>: o primeiro arma o botão (“Confirmar?”, por 5 segundos) e o segundo executa. Só funcionam no servidor Linux, com a regra do polkit instalada pelo <code>install.sh</code>.</p>` },

    { id: 'cameras', title: 'Câmeras: cadastro', html: `
      <p>Em <b>Câmeras → Câmeras</b> ficam as câmeras que aparecem na página Câmeras e no rodízio da tela inicial.</p>
      <h3>Adicionar</h3>
      <ol><li>Clique em <b>+ Adicionar câmera</b>.</li>
        <li><b>Nome</b>: como aparece em cima da imagem (ex.: Garagem). Também vira o nome dos arquivos de gravação.</li>
        <li><b>Endereço RTSP</b>: o endereço de vídeo da câmera (veja os exemplos abaixo).</li>
        <li>Clique em <b>Salvar câmeras</b>. A câmera já aparece, sem reiniciar.</li></ol>
      <h3>Endereços RTSP</h3>
      <table class="help-t"><tr><th>Tipo</th><th>Endereço</th></tr>
        <tr><td>Intelbras / Dahua (Mibo Smart, iM)</td><td><code>rtsp://admin:SENHA@192.168.0.10:554/cam/realmonitor?channel=1&amp;subtype=1</code></td></tr>
        <tr><td>Câmera sem senha (ex.: modelos “Hankvision”)</td><td><code>rtsp://192.168.0.11:554/live/mpeg4</code></td></tr></table>
      <ul><li><b>Use o substream (<code>subtype=1</code>)</b>: ele é H.264, que o hub repassa ao navegador. O <code>subtype=0</code> de várias câmeras é H.265 e <b>não funciona</b>. O hub avisa “a câmera não entrega H.264 neste endereço”.</li>
        <li>Usuário costuma ser <code>admin</code> e a senha é a definida no app da câmera (Mibo Smart). Errar a senha várias vezes bloqueia a câmera por alguns minutos.</li>
        <li><b>Caracteres especiais na senha</b> vão codificados: <code>@</code> = <code>%40</code>, <code>!</code> = <code>%21</code>, <code>#</code> = <code>%23</code>, <code>:</code> = <code>%3A</code>, <code>/</code> = <code>%2F</code>.</li>
        <li>Reserve o IP da câmera no roteador (DHCP fixo). Se o IP mudar, a câmera para de funcionar até você corrigir o endereço aqui.</li>
        <li>Sem áudio: o vídeo ao vivo é sempre mudo.</li></ul>
      <h3>Editar e remover</h3>
      <ul><li>Para <b>renomear</b>, mude o nome e salve. Atenção: os arquivos novos passam a usar o novo nome.</li>
        <li>Para <b>trocar a senha/endereço</b>, digite o endereço completo novo. Em branco mantém o atual (o campo mostra o atual mascarado).</li>
        <li>A <b>ordem</b> da lista é a ordem na página Câmeras e no rodízio.</li>
        <li><b>Remover</b> desliga a câmera na hora e fecha o arquivo de gravação. As gravações antigas dela ficam na pasta, mas sem dono: passam a seguir a retenção padrão (7 dias) e depois são apagadas. Baixe antes o que quiser guardar.</li>
        <li>Máximo de 16 câmeras.</li></ul>
      ${tip('uma câmera <b>nova</b> começa a gravar sozinha se a chave geral “Gravar as câmeras” estiver ligada. Desligue-a no cartão de gravação se não quiser.')}` },

    { id: 'gravacao', title: 'Câmeras: gravação', html: `
      <p>O hub grava o mesmo vídeo que mostra ao vivo (sem recodificar), em <b>1 arquivo MP4 por hora e por câmera</b>, na pasta de gravações. Os arquivos se chamam <code>NOME_AAAAMMDDHH.mp4</code>. Para assistir, use <b>Câmeras → Gravações</b> (Assistir, Baixar, Excluir e seleção em lote).</p>
      <h3>Chave geral</h3>
      <p><b>Gravar as câmeras</b> liga ou desliga tudo. Vem desligada por padrão. Ligada, o hub mantém a conexão com as câmeras 24 horas por dia.</p>
      <h3>Por câmera</h3>
      <table class="help-t"><tr><th>Opção</th><th>O que faz</th></tr>
        <tr><td>Liga/desliga</td><td>Grava ou não esta câmera (vale se a chave geral estiver ligada).</td></tr>
        <tr><td>Modo: Contínuo</td><td>Grava o tempo todo.</td></tr>
        <tr><td>Modo: Só com movimento</td><td>Grava só quando detecta movimento, com alguns segundos antes e 15 segundos depois. Hora sem movimento não cria arquivo.</td></tr>
        <tr><td>Sensibilidade (1 a 5)</td><td>Só no modo movimento. Mais alta pega movimentos pequenos, mas dispara com sombra, TV e mudança de luz. Se gravar demais, baixe.</td></tr>
        <tr><td>Guardar por</td><td>Gravações mais velhas que isso são apagadas sozinhas (1 a 365 dias; padrão 7).</td></tr>
        <tr><td>Limite de espaço</td><td>Passou desse total em GB, o hub apaga as mais antigas <b>daquela câmera</b>. “Sem limite” não apaga por tamanho.</td></tr></table>
      <p>Debaixo do nome de cada câmera aparece o estado: <i>gravando agora</i>, <i>aguardando movimento</i>, <i>iniciando…</i> ou a mensagem de erro, mais a quantidade de arquivos e o espaço usado.</p>
      <h3>Pasta das gravações</h3>
      <p>No cartão <b>Gravações: pasta e Google Drive</b>. O padrão é <code>~/securityCam (pasta pessoal de quem roda o hub)</code>. O hub cria a pasta se não existir. <b>Vale depois de reiniciar o hub.</b> Trocar a pasta não move os arquivos antigos.</p>
      ${tip('a limpeza (retenção e limite de GB) roda a cada 10 minutos e só mexe em arquivos com o nome desse sistema, nunca em outros arquivos da pasta.')}` },

    { id: 'drive', title: 'Câmeras: envio ao Google Drive', html: `
      <p>Todo dia o hub copia as gravações novas para o Google Drive usando o <b>rclone</b>. As gravações locais continuam seguindo a retenção da câmera; a cópia no Drive também respeita os mesmos dias.</p>
      <h3>Antes de ligar</h3>
      <ol><li>O <code>rclone</code> precisa estar instalado no servidor e já ligado ao Google Drive (<code>rclone config</code> cria um “remote”, por exemplo <code>GDrive</code>).</li>
        <li>Teste no servidor: <code>rclone lsd GDrive:</code> deve listar suas pastas do Drive.</li></ol>
      <h3>Campos (cartão “Gravações: pasta e Google Drive”)</h3>
      <table class="help-t"><tr><th>Campo</th><th>O que colocar</th></tr>
        <tr><td>Copiar as gravações para o Google Drive todo dia</td><td>Liga o envio automático.</td></tr>
        <tr><td>Destino no rclone</td><td>No formato <code>REMOTE:Pasta</code>, ex.: <code>GDrive:CameraSeguranca</code>. Dentro dela o hub cria uma subpasta por dia e por câmera. Ao ligar, o formato é conferido.</td></tr>
        <tr><td>Hora do envio automático</td><td>Hora cheia em que a rodada começa (ex.: 03:00). Se falhar, tenta de novo até 3 vezes dentro de uma janela de cerca de 22 horas.</td></tr>
        <tr><td>Maior arquivo enviado inteiro (MB)</td><td>Arquivos maiores são divididos em partes, cada uma tocando sozinha. 0 usa o padrão de 1024 MB.</td></tr></table>
      <p>Tudo isso <b>só vale depois de reiniciar o hub</b>.</p>
      <h3>No dia a dia (aba Câmeras, cartão “Google Drive”)</h3>
      <ul><li><b>Sincronizar tudo</b>: envia agora o que estiver pendente de todas as câmeras.</li>
        <li><b>Sincronizar</b> (dentro de cada câmera): envia só aquela câmera.</li>
        <li>A chave <b>Enviar ao Google Drive</b> de cada câmera tira essa câmera das rodadas automáticas.</li>
        <li>O cartão mostra a última mensagem, o número de tentativas e o horário da próxima rodada.</li></ul>` },

    { id: 'agendas', title: 'Agendas (iCal)', html: `
      <p>As agendas aparecem na tela inicial (próximo evento, eventos do dia, modo noturno). Cada agenda é um calendário do Google lido pelo <b>endereço secreto iCal</b>.</p>
      <h3>Onde pegar o endereço</h3>
      <ol><li>No <b>Google Agenda</b> (no computador), clique nos três pontinhos da agenda &rarr; <b>Configurações e compartilhamento</b>.</li>
        <li>Role até <b>Integrar agenda</b>.</li>
        <li>Copie o <b>Endereço secreto no formato iCal</b> (termina em <code>basic.ics</code>).</li></ol>
      ${warn('esse endereço dá acesso de leitura à agenda inteira. Não compartilhe. Se vazar, gere outro em “Redefinir” no mesmo lugar.')}
      <h3>Adicionar</h3>
      <ol><li><b>+ Adicionar agenda</b>.</li>
        <li><b>Nome</b>: aparece nos eventos (ex.: Casal, Fulano, Plantões).</li>
        <li><b>Cor</b>: a bolinha dos eventos dessa agenda. Agendas antigas com degradê só trocam de cor se você escolher outra.</li>
        <li><b>Endereço iCal</b>: cole o endereço secreto.</li>
        <li><b>Salvar agendas</b>. O hub <b>abre o endereço antes de guardar</b>: se não for um iCal válido, mostra o erro e não salva.</li></ol>
      <h3>Detalhes</h3>
      <ul><li>A ordem da lista é a ordem dos blocos na tela inicial. Agendas sem eventos no dia ficam de fora.</li>
        <li>Remover apaga a agenda do painel (não mexe no Google Agenda).</li>
        <li>O hub guarda a agenda por alguns minutos (cache em Integrações → “Agendas: guardar por”). Se o Google falhar, mantém a última versão boa.</li>
        <li>Máximo de 12 agendas.</li></ul>` },

    { id: 'transito', title: 'Trânsito', html: `
      <p>O cartão “Deslocamento” da tela inicial mostra o tempo de cada pessoa até o destino, com o nível (livre, moderado, congestionado), a hora de sair e um mini-mapa. Os dados vêm da <b>Google Routes API</b>.</p>
      <h3>Passo 1: chave e casa</h3>
      <ul><li><b>Chave da API Google Routes</b>: crie no Google Cloud Console um projeto, ative a <b>Routes API</b>, ative o faturamento (há uma cota gratuita mensal) e gere uma chave. Cole no campo; ela fica só no servidor. Em branco mantém a atual.</li>
        <li>Se você restringiu a chave por <b>endereço IP</b>, ligue em Integrações a opção “Usar só IPv4”, porque o Google costuma liberar só o IPv4 público.</li>
        <li><b>Endereço de casa</b>: origem padrão de todas as rotas. Escreva completo, com número e cidade (ex.: <code>R. Exemplo, 100, São Paulo, SP</code>). Sem chave ou sem endereço de casa o painel mostra dados de <b>exemplo</b>.</li></ul>
      <h3>Passo 2: pessoas</h3>
      <p>Cada pessoa vira um cartão na tela inicial. <b>+ Adicionar pessoa</b> (até 8) e <b>Remover</b> no fim do bloco dela.</p>
      <ul><li><b>Nome da pessoa</b>: título do cartão.</li>
        <li><b>Precisa chegar às</b>: com isso o cartão calcula “Saia às HH:MM” (hora de chegada menos o tempo do trajeto). Some quando a hora já passou.</li></ul>
      <h3>Quando consultar a API de rotas</h3>
      <p>É a regra que mais gasta ou poupa cota. Escolha por pessoa:</p>
      <table class="help-t"><tr><th>Opção</th><th>Como funciona</th><th>Use quando</th></tr>
        <tr><td><b>Só quando eu pedir</b></td><td>O cartão mostra o botão <b>Ver trânsito</b>. Ao tocar, o hub consulta a API e o resultado fica na tela por 10 minutos. Toques repetidos em até 5 minutos reaproveitam a consulta.</td><td>Horário variável; quer poupar cota .</td></tr>
        <tr><td><b>Automático numa janela de horário</b></td><td>O hub só consulta dentro da janela (ex.: 06:00 às 08:00). Antes dela o cartão diz “Disponível às 06:00”. Depois da janela mostra o último resultado, sem nova consulta. Passando de <b>“Esconder o cartão depois das”</b> (ex.: 09:00) o cartão mostra “Só pela manhã”.</td><td>Rotina fixa de manhã .</td></tr>
        <tr><td><b>Sempre</b></td><td>Consulta a cada atualização do painel, respeitando o intervalo em Integrações (“Trânsito: consultar no máximo a cada”, padrão 10 min).</td><td>Poucas pessoas e cota sobrando.</td></tr></table>
      <h3>Passo 3: rotas da pessoa</h3>
      <p>Uma rota é um trecho que vale em certos dias da semana. <b>+ Adicionar rota</b>:</p>
      <table class="help-t"><tr><th>Campo</th><th>Para que serve</th></tr>
        <tr><td>Nome da rota</td><td>Aparece no cartão: “Fulano → Escritório”.</td></tr>
        <tr><td>Origem</td><td>Em branco = o endereço de casa. Preencha para trechos que não saem de casa (ex.: do trabalho para a escola).</td></tr>
        <tr><td>Destino</td><td>Endereço completo de chegada.</td></tr>
        <tr><td>A partir de</td><td>Hora (opcional) em que esta rota passa a valer no dia. Serve para trocar de trecho durante o dia.</td></tr>
        <tr><td>Home office</td><td>Marque nos dias sem deslocamento: o cartão mostra “Home office hoje” e não consulta a API. Os campos de origem e destino somem.</td></tr>
        <tr><td>Dias (Seg…Dom)</td><td>Marque os dias em que a rota vale. É preciso marcar pelo menos um.</td></tr></table>
      <h3>Exemplos</h3>
      <ul><li><b>Trabalho de segunda a sexta</b>: uma rota “Escritório”, destino X, dias Seg–Sex.</li>
        <li><b>Escolas e dias diferentes</b>: rota “Escola A” nas segundas e quintas, rota “Escola B” nas terças e sextas, e uma rota “Home office” (marcando a caixa) na quarta.</li>
        <li><b>Ida e volta</b>: rota “Ida” (destino trabalho) e rota “Volta” com origem = trabalho, destino = casa e <i>A partir de</i> 17:00. Depois das 17h vale a “Volta”.</li></ul>
      <p>Se uma pessoa não tem nenhuma rota no dia, o cartão mostra “Sem deslocamento hoje”.</p>
      ${tip('clique em <b>Salvar rotas</b> para guardar chave, endereço de casa e todas as pessoas de uma vez. Remover uma pessoa ou uma rota salva na hora.')}` },

    { id: 'painel', title: 'Painel (tela inicial)', html: `
      <p>Ajustes de como a tela inicial se comporta. Valem na hora para todos os aparelhos (TV, celular, navegador): as telas abertas pegam a mudança em até 5 minutos.</p>
      <h3>Localização e clima</h3>
      <ul><li><b>Cidade/Estado</b>: texto do cabeçalho.</li>
        <li><b>Latitude e Longitude</b>: usadas para buscar o clima (Open-Meteo, sem chave). Use ponto como separador (ex.: <code>-23.5505</code> e <code>-46.6333</code>). Dica: no Google Maps, clique com o botão direito no local e copie as coordenadas.</li>
        <li><b>Fuso horário</b>: nome IANA, ex.: <code>America/Sao_Paulo</code>, <code>America/Manaus</code>.</li>
        <li><b>Atualizar o clima a cada</b>: de 5 a 240 minutos (padrão 15).</li></ul>
      <h3>Modo noturno</h3>
      <p>Das horas “Começa às” até “Termina às” a tela fica escura, só com o clima e os eventos de amanhã (menos brilho, evita queimar a TV). Pode cruzar a meia-noite (23:00 até 05:00). Na tela existe um botão para desligar o modo noturno até o fim da noite atual.</p>
      <h3>Câmeras na tela inicial</h3>
      <p><b>Segundos por câmera</b> (5 a 3600). No meio da tela inicial o Clima alterna com cada câmera por este tempo (Clima → câmera 1 → câmera 2 → Clima…). Sem câmeras cadastradas, fica só o Clima. Para testar rápido, abra <code>index.html?rotate=10</code>.</p>
      <h3>Aviso de validade</h3>
      <p><b>Avisar com quantos dias de antecedência</b> (0 a 365; padrão 3). Itens que vencem em até esse prazo ficam em destaque âmbar; vencidos ficam em vermelho. O atalho “Geladeira” do menu também usa isso para avisar o que está vencendo.</p>
      <h3>Locais do estoque</h3>
      <p>Os lugares onde os itens são guardados (Geladeira, Freezer, Dispensa…). Aparecem como abas e como opção ao cadastrar um item em Geladeira e Dispensa.</p>
      <ul><li><b>+ Adicionar local</b>: nome, cor e ícone. Salve em “Salvar”.</li>
        <li>Renomear não perde os itens. A ordem é a da lista.</li>
        <li><b>Remover</b> só funciona se o local estiver <b>vazio</b>; se tiver itens, o hub avisa quantos são. Mova ou apague os itens antes.</li>
        <li>Precisa de pelo menos 1 local (máximo 12).</li></ul>
      <h3>Página Vídeos (endereço de reprodução)</h3>
      <p>Os arquivos de vídeo são entregues por um servidor web da máquina (nginx). <b>Porta</b> (padrão 8080) e <b>Caminho</b> (padrão <code>/videos/</code>) dizem onde o navegador busca o arquivo para tocar ou abrir no VLC. A pasta que o hub lista fica em Integrações.</p>
      <h3>Feriados locais</h3>
      <p>Complementam os feriados nacionais (que vêm da BrasilAPI). Informe <b>dia e mês</b> (o ano é ignorado: repete todo ano), o <b>nome</b> e o <b>tipo</b> (municipal, estadual, nacional), que muda a cor do selo na tela inicial. Se você nunca editou, valem os de São Paulo (aniversário da cidade e 9 de julho). A lista que você salva <b>substitui</b> a padrão.</p>` },

    { id: 'livros', title: 'Livros (PDF e EPUB)', html: `
      <p>A página <b>Livros</b> guarda PDFs e EPUBs no servidor (pasta <code>~/Livros</code>) e deixa enviar arquivos novos pelo próprio painel. Cada livro pode ter uma <b>categoria</b> e um <b>dono</b>.</p>
      <table class="help-t"><tr><th>Opção</th><th>O que faz</th></tr>
        <tr><td>Mostrar a página Livros</td><td>Desligada, o atalho some do menu e o servidor deixa de listar, entregar e receber livros. Os arquivos continuam na pasta; ao ligar de novo, tudo volta.</td></tr>
        <tr><td>Categorias</td><td>Uma por linha. Viram filtros na página e opções ao enviar ou editar um livro. Tirar uma categoria da lista <b>não</b> apaga o livro nem a categoria que ele já tem.</td></tr>
        <tr><td>Donos</td><td>Uma por linha (de quem é o livro). O filtro de dono aparece na página quando algum livro tem dono.</td></tr></table>
      <p>Vale na hora; telas abertas pegam a mudança em até 5 minutos.</p>
      <h3>Na página Livros</h3>
      <ul><li><b>Enviar livro</b>: escolha um ou mais arquivos .pdf ou .epub (até 300 MB cada), a categoria e o dono. Nome repetido ganha “(2)”.</li>
        <li><b>PDF</b> abre numa aba do navegador. <b>EPUB</b> abre no leitor da página: as setas ← → ou os botões trocam de capítulo, e o último capítulo lido fica lembrado neste aparelho.</li>
        <li>O lápis troca categoria e dono; a lixeira exclui o arquivo do servidor (não dá para desfazer).</li></ul>
    ` },
    { id: 'menu', title: 'Menu de atalhos', html: `
      <p>São os botões do rodapé da tela inicial. Você pode adicionar, remover, renomear e reordenar.</p>
      <h3>Campos de cada atalho</h3>
      <table class="help-t"><tr><th>Campo</th><th>O que é</th></tr>
        <tr><td>Nome</td><td>Título do botão (até 24 letras).</td></tr>
        <tr><td>Descrição</td><td>Texto pequeno embaixo (opcional).</td></tr>
        <tr><td>Ícone</td><td>Escolha na lista.</td></tr>
        <tr><td>Porta</td><td><b>Vazia</b>: o atalho abre uma página do próprio painel/hub. <b>Preenchida</b>: abre outro serviço da mesma máquina nessa porta (ex.: 4533 para o Navidrome).</td></tr>
        <tr><td>Caminho</td><td>Começa com <code>/</code>. Exemplos: <code>/estoque.html</code>, <code>/camaras.html</code>, <code>/videos.html</code>, <code>/</code>. Pode usar <code>{host}</code> para o endereço da máquina (ex.: <code>/jogos/play/?feed={host}:8080/roms/feed.json</code>).</td></tr></table>
      <h3>Páginas do painel disponíveis</h3>
      <p><code>/estoque.html</code> (Geladeira), <code>/contatos.html</code> (Serviços), <code>/camaras.html</code> (Câmeras), <code>/videos.html</code> (Vídeos), <code>/livros.html</code> (Livros), <code>/config.html</code> (Config).</p>
      <h3>Reordenar, remover, restaurar</h3>
      <ul><li>Use as setas <b>↑</b> e <b>↓</b> e clique em <b>Salvar menu</b>.</li>
        <li><b>Remover</b> salva na hora.</li>
        <li>O atalho da <b>Config</b> não pode ser removido, para você não perder o acesso a esta tela.</li>
        <li><b>Voltar ao menu padrão</b> descarta suas mudanças e volta ao menu original do painel.</li>
        <li>Na primeira vez, a lista mostra o menu padrão; ao salvar, ele vira personalizado para todos os aparelhos.</li>
        <li>Os nomes de fábrica (Jogos, Geladeira, Câmeras…) são traduzidos para o idioma em uso; um nome que você digitou aparece exatamente como escrito.</li></ul>` },

    { id: 'integracoes', title: 'Integrações', html: `
      <p>Opções do próprio hub. <b>Todas só valem depois de reiniciar o hub</b>: depois de salvar, aparece a faixa amarela com o botão <b>Reiniciar o hub agora</b> (o painel e as câmeras ficam fora do ar por alguns segundos). Se o botão não aparecer, reinicie no servidor: <code>sudo systemctl restart centraldiogo-hub</code>.</p>
      <h3>Planilha de profissionais</h3>
      <p>A página Serviços (profissionais: eletricista, encanador…) pode ser alimentada por uma planilha do Google.</p>
      <ol><li>Na planilha, deixe uma aba com os cabeçalhos abaixo e <b>publique na web</b> (Arquivo &rarr; Compartilhar &rarr; Publicar na web) ou use o formato <code>https://docs.google.com/spreadsheets/d/ID/gviz/tq?tqx=out:csv&amp;sheet=NOME_DA_ABA</code>.</li>
        <li>Cole o endereço em <b>Endereço do CSV</b>. Em branco desliga a sincronização.</li>
        <li><b>Sincronizar a cada</b>: minutos entre leituras (padrão 5).</li></ol>
      <p>Cabeçalhos reconhecidos (sem diferenciar acento ou maiúscula): <code>Profissional/Empresa</code> (ou Nome), <code>Serviço</code> (ou Tipo), <code>Contato</code> (ou Telefone), <code>Referência</code> (ou Indicação), <code>O que vai fazer</code>, <code>Obs</code>.</p>
      ${warn('a planilha é a <b>fonte da verdade</b>: linhas novas entram, mudanças atualizam e <b>linhas que saírem da planilha são apagadas</b> da página Serviços.')}
      <h3>Jogos e conquistas</h3>
      <p>Lista jogos e conquistas das suas contas. <b>Atualizar a cada</b>: minutos entre atualizações completas (padrão 360 = 6 horas). Cada serviço:</p>
      <table class="help-t"><tr><th>Serviço</th><th>O que preencher</th></tr>
        <tr><td>Steam</td><td><b>Usuário</b>: a URL personalizada do perfil ou o SteamID64. <b>Chave da API</b>: gere em <code>steamcommunity.com/dev/apikey</code>. O perfil precisa ter os detalhes de jogos públicos.</td></tr>
        <tr><td>PlayStation</td><td><b>Usuário</b>: só para exibição. <b>Cookie npsso</b>: logado na sua conta PSN, abra <code>ca.account.sony.com/api/v1/ssocookie</code> e copie o valor de <code>npsso</code>. Ele vale por cerca de 2 meses: se aparecer “npsso inválido ou expirado”, gere outro.</td></tr>
        <tr><td>Xbox</td><td><b>Gamertag</b>. Client ID e secret são opcionais (só se você usar um app seu no Azure). Depois de salvar e reiniciar, autorize a conta uma vez: botão <b>Conectar Xbox</b> do painel de jogos, ou no servidor <code>./hub -config hub.config.json -xbox-login</code>.</td></tr>
        <tr><td>RetroAchievements</td><td><b>Usuário</b> e <b>Chave da API</b> (em <code>retroachievements.org/settings</code>, “Web API Key”).</td></tr></table>
      <p>Os campos de chave ficam em branco depois de salvos e mostram “configurada”. Marque “apagar a configurada” para remover uma chave.</p>
      <h3>Vídeos, cache e rede</h3>
      <ul><li><b>Pasta dos vídeos no servidor</b>: a pasta que a página Vídeos lista (padrão <code>~/Videos</code>). Em branco desliga a lista. O endereço de reprodução fica em Painel.</li>
        <li><b>Agendas: guardar por</b>: minutos que o hub reaproveita a agenda antes de ler o Google de novo (padrão 5).</li>
        <li><b>Trânsito: consultar no máximo a cada</b>: intervalo mínimo entre consultas à API (padrão 10). Aumente para gastar menos cota.</li>
        <li><b>Usar só IPv4</b>: necessário se a chave do Google tem restrição de IP.</li></ul>` },

    { id: 'idioma', title: 'Idioma', html: `
      <p>A interface (tela inicial, Geladeira e Dispensa, Serviços, Câmeras, Vídeos e a própria Config, inclusive esta Ajuda) existe em <b>quatro idiomas</b>: Português, English, Español e 中文 (chinês simplificado).</p>
      <table class="help-t"><tr><th>Opção</th><th>Efeito</th></tr>
        <tr><td>Automático</td><td>Segue o idioma do navegador do aparelho. Se ele não for um dos quatro, usa português.</td></tr>
        <tr><td>Português, English, Español, 中文</td><td>Fixa o idioma escolhido neste aparelho.</td></tr></table>
      <ul><li>A escolha vale <b>só para o aparelho em que você a fez</b> (fica guardada no navegador dele): a TV pode ficar em português e o celular em inglês. Para a TV, abra a Config na própria TV.</li>
        <li>Ao trocar, a tela recarrega. Também dá para forçar pelo endereço: <code>index.html?lang=en</code> (<code>pt</code>, <code>en</code>, <code>es</code>, <code>zh</code>); <code>?lang=auto</code> volta ao automático.</li>
        <li><b>O que é traduzido</b>: textos da interface, datas e números (formato do idioma), os nomes de fábrica do menu (Jogos, Geladeira, Câmeras…), dos locais do estoque (Geladeira, Freezer, Dispensa) e os feriados nacionais.</li>
        <li><b>O que não é traduzido</b>: o que você mesmo cadastrou (nome de câmera, agenda, item, profissional, atalho renomeado, feriado local) e os eventos do Google Agenda. Os avisos de erro enviados pelo servidor são traduzidos quando conhecidos; os demais aparecem como o servidor os escreveu (em português).</li>
        <li>A cidade do clima, o fuso horário e os feriados vêm da sua localização, não do idioma: trocar de idioma não muda a cidade.</li></ul>` },

    { id: 'compat', title: 'Modo compatibilidade (TVs)', html: `
      <p>Navegadores de TV (Samsung/Tizen, LG/webOS, Chromecast e similares) normalmente têm <b>um único decodificador de vídeo</b> e não conseguem mostrar várias câmeras ao vivo ao mesmo tempo. No modo compatibilidade a página Câmeras mostra <b>uma câmera por vez</b>, com botões com os nomes para trocar. As outras ficam desconectadas.</p>
      <table class="help-t"><tr><th>Opção</th><th>Efeito</th></tr>
        <tr><td>Automático</td><td>Liga sozinho quando o navegador parece ser de TV. O texto abaixo diz se uma TV foi detectada.</td></tr>
        <tr><td>Sempre ligado</td><td>Força uma câmera por vez (útil em um tablet ou PC fraco).</td></tr>
        <tr><td>Sempre desligado</td><td>Todas as câmeras ao mesmo tempo, mesmo em TV.</td></tr></table>
      <ul><li>A escolha vale <b>só para o aparelho em que você a fez</b> (fica guardada no navegador dele). Para a TV, abra a Config na própria TV.</li>
        <li>Com uma única câmera cadastrada, nada muda.</li>
        <li>A tela inicial já mostra uma câmera por vez no rodízio, então não é afetada.</li></ul>` },

    { id: 'problemas', title: 'Solução de problemas', html: `
      <table class="help-t"><tr><th>Sintoma</th><th>O que verificar</th></tr>
        <tr><td>“Sem o hub: abra o painel pelo endereço do servidor da casa”</td><td>Abra pelo endereço do servidor (ex.: <code>http://192.168.0.10/config.html</code>), não por um arquivo ou por outro endereço.</td></tr>
        <tr><td>Câmera diz “Conectando…” ou “câmera indisponível”</td><td>IP/porta/senha corretos? A câmera está ligada e na rede? Senha com caractere especial sem codificar? Errou a senha várias vezes (bloqueio de alguns minutos)? Teste o mesmo endereço no VLC.</td></tr>
        <tr><td>“A câmera não entrega H.264”</td><td>Use o substream (<code>subtype=1</code>). O fluxo principal costuma ser H.265.</td></tr>
        <tr><td>Imagem some só na TV</td><td>Ligue o <b>Modo compatibilidade</b> (aba Compatibilidade).</td></tr>
        <tr><td>Miniaturas das câmeras não aparecem</td><td>Elas precisam do <code>ffmpeg</code> instalado no servidor (<code>sudo apt install ffmpeg</code>).</td></tr>
        <tr><td>“não consegui ler essa agenda”</td><td>O endereço iCal está errado, foi redefinido no Google ou o servidor está sem internet. Copie o endereço secreto de novo.</td></tr>
        <tr><td>Trânsito mostra “exemplo”</td><td>Falta a chave da API ou o endereço de casa, ou o hub não conseguiu falar com o Google.</td></tr>
        <tr><td>Trânsito com etiqueta “cache”</td><td>A última consulta falhou e o painel mostra a anterior. Veja chave, cota/faturamento do Google e a opção “só IPv4”.</td></tr>
        <tr><td>Cartão do trânsito diz “Disponível às…” ou “Só pela manhã”</td><td>É a regra de janela da pessoa. Ajuste a janela, ou troque para “Sempre”/“Só quando eu pedir”.</td></tr>
        <tr><td>Mudei uma opção de Integrações e nada mudou</td><td>Essas opções só valem depois de reiniciar o hub (faixa amarela na tela).</td></tr>
        <tr><td>Mudei o Painel e a TV não mudou</td><td>Espere até 5 minutos ou recarregue a tela da TV.</td></tr>
        <tr><td>A tela abriu em outro idioma</td><td>Em <b>Config → Idioma</b>, escolha o idioma do aparelho. O padrão “Automático” segue o navegador.</td></tr>
        <tr><td>Algumas mensagens de erro continuam em português</td><td>São mensagens do servidor ainda sem tradução. Elas só mudam de idioma quando constam da lista conhecida do painel.</td></tr>
        <tr><td>“origem não permitida para gravar”</td><td>Você abriu a Config por um endereço diferente do hub (ex.: porta de desenvolvimento). Abra pelo endereço do servidor.</td></tr>
        <tr><td>“Este hub é antigo e não tem estas opções”</td><td>O servidor ainda está com uma versão antiga do hub. Faça o deploy da versão nova.</td></tr>
        <tr><td>Não consigo remover um local do estoque</td><td>Ainda há itens nele. Mova ou apague os itens e tente de novo.</td></tr>
        <tr><td>Envio ao Drive falha</td><td>Rode no servidor <code>rclone lsd REMOTE:</code>; se pedir login, refaça <code>rclone config</code>. Veja a mensagem de erro no cartão “Google Drive”.</td></tr></table>
      <h3>Voltar ao original</h3>
      <p>Para desfazer tudo o que foi feito pela Config, pare o hub, apague (ou renomeie) <code>hub.settings.json</code> no servidor e inicie de novo: valem de novo o <code>hub.config.json</code> e os padrões do painel.</p>` }
  ]);
})(window.CD = window.CD || {});
