(function (CD) {
  // Config → Ajuda: manual completo de cada configuração, dentro da própria tela, no idioma em uso.
  // Cada seção é um cartão (com busca e índice no topo). O texto fica em js/i18n/help.<idioma>.js; atualize lá quando uma opção mudar.
  const U = CD.util, esc = U.esc, t = CD.i18n.t;


  const norm = s => String(s).toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '');

  function mount(root) {
    const SECTIONS = CD.i18n.help();
    root.innerHTML = '<div class="help-top" data-tab="ajuda"><input type="search" id="help-q" placeholder="' + esc(t('help.search')) + '" autocomplete="off" spellcheck="false">' +
      '<div class="help-toc" id="help-toc">' + SECTIONS.map(s => '<button type="button" class="vbtn" data-go="' + s.id + '">' + esc(s.title) + '</button>').join('') + '</div>' +
      '<div class="cfg-hint" id="help-none" hidden>' + t('help.none') + '</div></div>' +
      SECTIONS.map(s => '<div class="cfg-card help" data-tab="ajuda" id="help-' + s.id + '"><h2>' + esc(s.title) + '</h2><div class="help-body">' + s.html + '</div></div>').join('');

    const cards = SECTIONS.map(s => ({ s: s, el: root.querySelector('#help-' + s.id), text: norm(s.title + ' ' + s.html.replace(/<[^>]+>/g, ' ')) }));
    const q = root.querySelector('#help-q'), none = root.querySelector('#help-none'), toc = root.querySelector('#help-toc');

    function filter() {
      const words = norm(q.value).split(/\s+/).filter(Boolean);
      let shown = 0;
      cards.forEach(c => {
        const ok = words.every(w => c.text.indexOf(w) >= 0);
        c.el.hidden = !ok;
        if (ok) shown++;
      });
      toc.querySelectorAll('[data-go]').forEach(b => { b.hidden = !!words.length && root.querySelector('#help-' + b.getAttribute('data-go')).hidden; });
      none.hidden = shown > 0;
    }
    q.addEventListener('input', filter);
    toc.addEventListener('click', e => {
      const b = e.target.closest && e.target.closest('[data-go]');
      if (b) { const el = root.querySelector('#help-' + b.getAttribute('data-go')); if (el && el.scrollIntoView) el.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
    });
    // Link “Ajuda desta aba” (config.js): abre a seção certa
    root.go = id => {
      q.value = ''; filter();
      const el = root.querySelector('#help-' + id);
      if (el && el.scrollIntoView) el.scrollIntoView({ block: 'start' });
    };
  }

  CD.configHelp = { mount: mount };
})(window.CD = window.CD || {});
