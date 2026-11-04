(function (CD) {
  // Card rotativo da tela inicial: no lugar do Clima (coluna do meio) entram as câmeras, uma por vez (o app.js troca a cada minuto).
  // O painel se redesenha a cada 30 s (innerHTML); por isso o vídeo vive num elemento próprio que é recolocado no novo
  // #camslot a cada redesenho (na mesma tarefa, então o navegador não pausa o vídeo).
  const esc = CD.util.esc, t = CD.i18n.t;
  let box = null;                                           // { id, el, player }

  // Câmera que ocupa o lugar do clima agora (midIdx 0 = Clima), ou null
  function cur(s) {
    if (!s.cams || !s.cams.length || !CD.cameraPlayer.supported()) return null;
    return s.midIdx > 0 && s.midIdx <= s.cams.length ? s.cams[s.midIdx - 1] : null;
  }

  function slot(s) {
    const c = cur(s);
    const names = [t('wx.short')].concat(s.cams.map(x => x.name));
    return '<section class="col camcol"><div class="h2">' + esc(c.name) + ' <span class="tag">' + t('cam.liveTag') + '</span></div><div id="camslot"></div>' +
      '<div class="cam-dots">' + names.map((n, i) => '<span class="cam-dot' + (i === s.midIdx ? ' on' : '') + '">' + esc(n) + '</span>').join('') + '</div>' + CD.widgets.thumbs(s) + '</section>';
  }

  function dispose() {
    if (!box) return;
    box.player.stop();
    if (box.el.parentNode) box.el.parentNode.removeChild(box.el);
    box = null;
  }

  function create(c) {
    const el = document.createElement('div');
    el.className = 'camv';
    el.innerHTML = '<video muted autoplay playsinline></video><div class="cam-st">' + t('cam.connecting') + '</div>';
    const video = el.firstChild, st = el.lastChild;
    const player = CD.cameraPlayer.create(video, CD.hubApi.cameras.streamUrl(c.id), (kind, text) => {
      st.textContent = text;
      st.className = 'cam-st' + (kind === 'play' ? ' off' : kind === 'err' ? ' err' : '');
    });
    box = { id: c.id, el: el, player: player };
    player.start();
  }

  // Chamado depois de cada redesenho do painel: liga, troca ou desliga o vídeo conforme o que está na tela
  function sync(s) {
    const c = document.hidden ? null : cur(s), host = document.getElementById('camslot');
    if (!c || !host) return dispose();                      // clima na tela, modo noturno ou aba escondida: fecha a conexão
    if (box && box.id !== c.id) dispose();
    if (!box) create(c);
    host.appendChild(box.el);
  }

  document.addEventListener('visibilitychange', () => { if (CD.state) sync(CD.state); });

  CD.widgets = CD.widgets || {};
  CD.widgets.cameraCur = cur;
  CD.widgets.cameraSlot = slot;
  CD.widgets.cameraSync = sync;
})(window.CD = window.CD || {});
