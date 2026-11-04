(function (CD) {
  // Miniaturas das câmeras (um JPEG do hub, atualizado a cada 5 s) sob os períodos do Clima e também sob a câmera que ocupa o meio.
  // O painel se redesenha por innerHTML; por isso a última imagem de cada câmera fica guardada aqui e é reaplicada no redesenho.
  const esc = CD.util.esc, EVERY = 5000;
  const last = {};                                          // id -> URL da última imagem carregada
  let failed = 0, timer = 0;

  const url = id => CD.state.hub + '/api/cameras/' + encodeURIComponent(id) + '/snapshot.jpg';

  function render(s) {
    if (!s.hub || !s.cams || !s.cams.length || failed >= 3) return '';
    return '<div class="thumbs">' + s.cams.map(c =>
      '<div class="thumb"><div class="thumb-i">' + (last[c.id] ? '<img alt="" data-cam="' + esc(c.id) + '" src="' + esc(last[c.id]) + '">' : '<img alt="" data-cam="' + esc(c.id) + '" hidden>') + '</div>' +
      '<div class="thumb-n">' + esc(c.name) + '</div></div>').join('') + '</div>';
  }

  function load(c) {
    const u = url(c.id) + '?t=' + Date.now(), im = new Image();
    im.onload = () => {
      failed = 0; last[c.id] = u;
      document.querySelectorAll('img[data-cam="' + c.id + '"]').forEach(el => { el.src = u; el.hidden = false; });
    };
    im.onerror = () => { failed++; };
    im.src = u;
  }

  function tick() {
    const s = CD.state;
    if (document.hidden || !s.hub || !document.querySelector('.thumbs')) return;
    if (failed >= 3 && Date.now() % 60000 > EVERY) return;   // sem ffmpeg/câmera: tenta de novo só uma vez por minuto
    s.cams.forEach(load);
  }

  function start() { if (!timer) { timer = setInterval(tick, EVERY); tick(); } }
  document.addEventListener('DOMContentLoaded', () => setTimeout(start, 3000));
  document.addEventListener('visibilitychange', tick);

  CD.widgets = CD.widgets || {};
  CD.widgets.thumbs = render;
})(window.CD = window.CD || {});
