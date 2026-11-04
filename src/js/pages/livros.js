(function (CD) {
  // Página "Livros" (livros.html): PDFs e EPUBs guardados no servidor (pasta ~/Livros do hub), com envio pelo painel, categoria e dono.
  // PDF abre no visualizador do navegador; EPUB abre no leitor da página (o hub abre o arquivo e entrega um capítulo por vez num <iframe>).
  // As listas de categorias e de donos, e o liga/desliga da página, vêm da Config (aba Livros).
  const U = CD.util, I = CD.icons, esc = U.esc, t = CD.i18n.t;

  const norm = s => String(s || '').trim().toLowerCase();
  const title = n => n.replace(/\.(pdf|epub)$/i, '');
  const size = n => n >= 1073741824 ? CD.i18n.num(n / 1073741824, 1) + ' GB' : n >= 1048576 ? Math.round(n / 1048576) + ' MB' : Math.max(1, Math.round(n / 1024)) + ' KB';
  const typeOf = n => /\.epub$/i.test(n) ? 'epub' : /\.pdf$/i.test(n) ? 'pdf' : '';

  // Categorias e donos oferecidos nos formulários: o que a Config definiu (ou as categorias padrão do idioma) mais o que já existe nos livros
  function lists(items) {
    const b = CD.state.cfg.books || {};
    const pick = (base, used) => {
      const out = [], seen = {};
      base.concat(used).forEach(x => { x = String(x || '').trim(); if (x && !seen[norm(x)]) { seen[norm(x)] = 1; out.push(x); } });
      return out;
    };
    return {
      cats: pick(b.categories && b.categories.length ? b.categories : t('liv.cats'), (items || []).map(x => x.category)),
      owners: pick(b.owners || [], (items || []).map(x => x.owner))
    };
  }

  const options = (list, cur) => '<option value="">' + esc(t('liv.none')) + '</option>' +
    list.concat(cur && list.map(norm).indexOf(norm(cur)) < 0 ? [cur] : []).map(x => '<option value="' + esc(x) + '"' + (norm(x) === norm(cur) ? ' selected' : '') + '>' + esc(x) + '</option>').join('');

  // Leitor de EPUB: sumário e capítulos vêm do hub; o capítulo aparece num iframe sem scripts (sandbox sem allow-scripts, e o hub ainda manda uma política que proíbe scripts) e o último lido fica lembrado
  function reader(book) {
    const api = CD.hubApi.books, key = 'cd:epub:' + book.name;
    const ov = document.createElement('div');
    ov.className = 'rd';
    ov.innerHTML = '<div class="rd-top"><div class="rd-name">' + esc(title(book.name)) + '</div>' +
      '<select class="rd-sel" aria-label="' + esc(t('liv.contents')) + '"></select>' +
      '<button type="button" class="vbtn" data-a="prev">‹ ' + esc(t('liv.prev')) + '</button><button type="button" class="vbtn" data-a="next">' + esc(t('liv.next')) + ' ›</button>' +
      '<button type="button" class="vbtn" data-a="x">' + esc(t('c.close')) + '</button></div>' +
      '<iframe class="rd-frame" sandbox="allow-same-origin" title="' + esc(title(book.name)) + '"></iframe><div class="rd-msg">' + esc(t('liv.opening')) + '</div>';
    const frame = ov.querySelector('iframe'), sel = ov.querySelector('select'), msg = ov.querySelector('.rd-msg');
    let toc = null, at = 0;
    function show(i) {
      if (!toc) return;
      at = Math.max(0, Math.min(toc.chapters.length - 1, i));
      frame.src = api.chapterUrl(book.name, toc.chapters[at].href);
      sel.value = String(at);
      try { localStorage.setItem(key, String(at)); } catch (e) { /* sem armazenamento */ }
      ov.querySelector('[data-a="prev"]').disabled = at === 0;
      ov.querySelector('[data-a="next"]').disabled = at === toc.chapters.length - 1;
    }
    function close() { document.removeEventListener('keydown', onKey); frame.removeAttribute('src'); ov.remove(); }
    function onKey(e) {
      if (e.key === 'Escape' || e.keyCode === 10009) close();
      else if (e.key === 'ArrowRight') show(at + 1);
      else if (e.key === 'ArrowLeft') show(at - 1);
    }
    document.addEventListener('keydown', onKey);
    ov.addEventListener('click', e => {
      const b = e.target.closest && e.target.closest('[data-a]');
      if (!b) return;
      const a = b.getAttribute('data-a');
      if (a === 'x') close(); else if (a === 'prev') show(at - 1); else if (a === 'next') show(at + 1);
    });
    sel.addEventListener('change', () => show(+sel.value));
    document.body.appendChild(ov);
    api.toc(book.name).then(r => {
      toc = r; msg.remove();
      sel.innerHTML = r.chapters.map((c, i) => '<option value="' + i + '">' + esc(c.title || t('liv.chapter', { n: i + 1 })) + '</option>').join('');
      let start = 0;
      try { start = parseInt(localStorage.getItem(key), 10) || 0; } catch (e) { /* ok */ }
      show(start);
    }, e => { msg.textContent = e.message || t('liv.openErr'); msg.className = 'rd-msg err'; });
  }

  function mount(root) {
    const api = CD.hubApi.books;
    const st = { items: null, error: '', cat: '', owner: '', q: '', confirm: '' };
    const $ = s => root.querySelector(s);
    const off = () => CD.state.cfg.books && CD.state.cfg.books.enabled === false;

    root.innerHTML = '<section class="pg">' +
      '<div class="pg-sum" id="sum"></div>' +
      '<div class="pg-tools"><nav class="chips" id="chips"></nav>' +
      '<select class="own-sel" id="own" aria-label="' + esc(t('liv.f.owner')) + '"></select>' +
      '<label class="search">' + I.svg('search', 'currentColor', 26, 2) + '<input type="search" id="q" placeholder="' + esc(t('liv.search')) + '" autocomplete="off"></label>' +
      '<button type="button" class="btn" data-act="new">' + I.svg('plus', 'currentColor', 26, 2.2) + '<span>' + t('liv.upload') + '</span></button></div>' +
      '<div class="pg-list cards" id="list"></div><div id="modal"></div></section>';

    const all = () => st.items || [];

    function renderChips() {
      const count = k => all().filter(b => !k || (k === '\u0000' ? !b.category : norm(b.category) === k)).length;
      const seen = {}, chips = [{ key: '', label: t('c.all') }];
      lists(all()).cats.forEach(c => { if (count(norm(c))) { seen[norm(c)] = 1; chips.push({ key: norm(c), label: c }); } });
      if (count('\u0000')) chips.push({ key: '\u0000', label: t('liv.uncat') });
      if (st.cat && !chips.some(c => c.key === st.cat)) st.cat = '';
      $('#chips').innerHTML = chips.map(c => '<button type="button" class="chip' + (st.cat === c.key ? ' on' : '') + '" data-cat="' + esc(c.key) + '">' + esc(c.label) + '<b>' + count(c.key) + '</b></button>').join('');
      const owners = lists(all()).owners.filter(o => all().some(b => norm(b.owner) === norm(o)));
      if (st.owner && owners.map(norm).indexOf(st.owner) < 0) st.owner = '';
      const own = $('#own');
      own.style.display = owners.length ? '' : 'none';
      own.innerHTML = '<option value="">' + esc(t('liv.allOwners')) + '</option>' + owners.map(o => '<option value="' + esc(norm(o)) + '"' + (st.owner === norm(o) ? ' selected' : '') + '>' + esc(o) + '</option>').join('');
      $('#sum').innerHTML = all().length ? '<span class="spill ok">' + CD.i18n.n('liv.count', all().length) + '</span>' : '';
    }

    function card(b) {
      if (st.confirm === b.name) {
        return '<article class="cc confirm"><div class="cc-name">' + t('c.confirmDelete', { name: esc(title(b.name)) }) + '</div><div class="rw-meta">' + t('c.irreversible') + '</div>' +
          '<div class="cc-act"><button type="button" class="btn danger" data-act="del" data-n="' + esc(b.name) + '">' + t('c.delete') + '</button>' +
          '<button type="button" class="btn ghost" data-act="cancel">' + t('c.cancel') + '</button></div></article>';
      }
      const ep = b.type === 'epub', dl = esc(api.fileUrl(b.name, true));
      return '<article class="cc"><div class="cc-top"><div class="cc-name">' + esc(title(b.name)) + '</div><span class="ctag alt">' + esc(b.type.toUpperCase()) + '</span></div>' +
        (b.category ? '<div><span class="ctag">' + esc(b.category) + '</span></div>' : '') +
        (b.owner ? '<div class="cc-line">' + I.svg('user', 'currentColor', 22, 2) + '<span>' + esc(b.owner) + '</span></div>' : '') +
        '<div class="cc-line">' + size(b.size) + ' · ' + CD.i18n.dateNum(new Date(b.time * 1000)) + '</div>' +
        '<div class="bk-act">' + (ep ? '<button type="button" class="vbtn main" data-act="read" data-n="' + esc(b.name) + '">' + t('liv.read') + '</button>'
          : '<a class="vbtn main" href="' + esc(api.fileUrl(b.name)) + '" target="_blank" rel="noopener">' + t('liv.open') + '</a>') +
        '<a class="vbtn" href="' + dl + '" download>' + t('liv.download') + '</a></div>' +
        '<div class="cc-act"><button type="button" class="ib" data-act="edit" data-n="' + esc(b.name) + '" title="' + esc(t('c.edit')) + '" aria-label="' + esc(t('c.edit') + ' ' + title(b.name)) + '">' + I.svg('edit', 'currentColor', 28, 2) + '</button>' +
        '<button type="button" class="ib danger" data-act="ask" data-n="' + esc(b.name) + '" title="' + esc(t('c.delete')) + '" aria-label="' + esc(t('c.delete') + ' ' + title(b.name)) + '">' + I.svg('trash', 'currentColor', 28, 2) + '</button></div></article>';
    }

    function renderList() {
      const el = $('#list');
      $('.pg-tools').style.display = off() ? 'none' : '';        // desligada na Config: sem busca nem botão de envio
      if (off()) { el.innerHTML = '<div class="empty">' + esc(t('liv.off')) + '</div>'; return; }
      let h = st.error ? '<div class="banner err"><span>' + esc(st.error) + '</span><button type="button" class="btn ghost" data-act="retry">' + t('c.retry') + '</button></div>' : '';
      if (!st.items) { el.innerHTML = h || '<div class="skel" style="height:190px"></div><div class="skel" style="height:190px"></div>'; return; }
      const q = norm(st.q);
      const list = st.items.filter(b => (!st.cat || (st.cat === '\u0000' ? !b.category : norm(b.category) === st.cat)) && (!st.owner || norm(b.owner) === st.owner) &&
        (!q || [b.name, b.category, b.owner].join(' ').toLowerCase().indexOf(q) >= 0));
      if (!list.length) h += '<div class="empty">' + (st.items.length ? t('liv.notfound') : t('liv.empty')) + '</div>';
      else h += list.map(card).join('');
      el.innerHTML = h;
    }

    function renderAll() { renderChips(); renderList(); }

    async function load() {
      st.error = '';
      if (off()) { st.items = []; renderAll(); return; }
      try { st.items = await api.list(); } catch (e) { st.error = e.message; }
      renderAll();
    }

    // Enviar (um ou mais arquivos, todos com a mesma categoria e o mesmo dono) ou editar a categoria e o dono de um livro
    function form(b) {
      const l = lists(all());
      const body = (b ? '' : CD.ui.field(t('liv.f.file'), '<input name="files" type="file" multiple accept=".pdf,.epub,application/pdf,application/epub+zip">')) +
        CD.ui.field(t('liv.f.category'), '<select name="category">' + options(l.cats, b ? b.category : st.cat && st.cat !== '\u0000' ? (l.cats.filter(c => norm(c) === st.cat)[0] || '') : '') + '</select>', t('c.optional')) +
        CD.ui.field(t('liv.f.owner'), '<select name="owner">' + options(l.owners, b ? b.owner : st.owner ? (l.owners.filter(o => norm(o) === st.owner)[0] || '') : '') + '</select>', t('c.optional'));
      CD.ui.modal($('#modal'), {
        title: b ? t('liv.edit') + ': ' + title(b.name) : t('liv.upload'), body: body, submit: b ? t('c.save') : t('liv.upload'),
        read(f) {
          const el = f.elements, d = { category: el.category.value, owner: el.owner.value };
          if (!b) {
            d.files = Array.prototype.slice.call(el.files.files || []);
            if (!d.files.length) throw new Error(t('liv.e.file'));
            const bad = d.files.filter(x => !typeOf(x.name))[0];
            if (bad) throw new Error(t('liv.e.type', { name: bad.name }));
          }
          return d;
        },
        async save(d) {
          if (b) { const r = await api.setMeta(b.name, { category: d.category, owner: d.owner }); st.items[st.items.indexOf(b)] = r; }
          else {
            const btn = $('#modal [type=submit]');
            for (let i = 0; i < d.files.length; i++) {
              const f = d.files[i];
              const prog = p => { if (btn) btn.textContent = t('liv.sending', { i: i + 1, n: d.files.length, p: Math.round(p * 100) }); };
              prog(0);
              try { st.items.unshift(await api.upload(f, d, prog)); }
              catch (e) { renderAll(); throw new Error(t('liv.fail', { name: f.name, msg: e.message })); }
            }
          }
          st.error = ''; renderAll();
        }
      });
    }

    root.addEventListener('click', async e => {
      const b = e.target.closest && e.target.closest('[data-act],[data-cat]');
      if (!b) return;
      if (b.hasAttribute('data-cat')) { st.cat = b.getAttribute('data-cat'); st.confirm = ''; renderChips(); renderList(); return; }
      const act = b.getAttribute('data-act'), n = b.getAttribute('data-n');
      const book = all().filter(x => x.name === n)[0];
      if (act === 'new') form(null);
      else if (act === 'edit' && book) form(book);
      else if (act === 'read' && book) reader(book);
      else if (act === 'ask' && book) { st.confirm = n; renderList(); }
      else if (act === 'cancel') { st.confirm = ''; renderList(); }
      else if (act === 'retry') { st.items = null; renderList(); load(); }
      else if (act === 'del' && book) {
        b.disabled = true;
        try { await api.remove(n); st.items = st.items.filter(x => x !== book); st.error = ''; }
        catch (x) { st.error = x.message; }
        st.confirm = ''; renderAll();
      }
    });
    $('#q').addEventListener('input', e => { st.q = e.target.value; st.confirm = ''; renderList(); });
    $('#own').addEventListener('change', e => { st.owner = e.target.value; st.confirm = ''; renderList(); });

    if (!CD.state.hub) { $('#list').innerHTML = '<div class="empty">' + esc(t('liv.nohub')) + '</div>'; return; }
    renderAll();
    load();
  }

  CD.pages = CD.pages || {};
  CD.pages.livros = { labelKey: 'page.livros.label', subKey: 'page.livros.sub', icon: 'book', mount: mount };
})(window.CD = window.CD || {});
