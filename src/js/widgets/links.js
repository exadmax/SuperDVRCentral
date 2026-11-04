(function (CD) {
  // Atalhos para os outros serviços da máquina (WebRCade, Navidrome, vídeos) e para as páginas do próprio hub.
  // Só aparecem quando existe um endereço-base: o hub configurado ou o servidor que entregou a página.
  function base(s) {
    if (s.hub) return s.hub;
    return /^https?:/.test(location.protocol) ? location.origin : '';
  }

  function href(l, b) {
    let u;
    try { u = new URL(b); } catch (e) { return null; }
    const host = u.protocol + '//' + u.hostname;                 // mesmo host, sem porta
    const origin = l.port ? host + ':' + l.port : u.origin;
    return origin + l.path.replace('{host}', host);
  }

  function render(s) {
    const b = base(s), booksOff = s.cfg.books && s.cfg.books.enabled === false;
    const list = (s.cfg.links || []).filter(l => !(booksOff && /\/livros\.html$/.test(l.path)));       // página Livros desligada na Config
    if (!b || !list.length) return '';
    return '<nav class="links">' + list.map(l => {
      const h = href(l, b);
      if (!h) return '';
      const a = l.alert === 'estoque' ? CD.widgets.expiring(s) : null;     // item que avisa vencidos/perto de vencer
      return '<a class="lk' + (a ? ' ' + a.lvl : '') + '" target="_top" href="' + CD.util.esc(h) + '">' + CD.icons.svg(a ? a.icon : l.icon, a ? a.color : '#4DA3FF', 36, 1.8) +
        '<span class="lk-t"><b>' + CD.util.esc(CD.i18n.user(l.label)) + '</b><span>' + CD.util.esc(a ? a.sub : CD.i18n.user(l.sub || '')) + '</span></span></a>';
    }).join('') + '</nav>';
  }

  CD.widgets = CD.widgets || {};
  CD.widgets.links = render;
})(window.CD = window.CD || {});
