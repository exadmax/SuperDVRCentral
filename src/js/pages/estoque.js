(function (CD) {
  // Página "Geladeira e Dispensa" (estoque.html): CRUD dos itens guardados no hub (SQLite).
  const U = CD.util, I = CD.icons, S = CD.stock, esc = U.esc, t = CD.i18n.t;

  const sortKey = it => it.validade || '9999-99-99';                        // sem validade vai para o fim
  const byExpiry = (a, b) => sortKey(a) < sortKey(b) ? -1 : sortKey(a) > sortKey(b) ? 1 : a.nome.localeCompare(b.nome, CD.i18n.locale());

  function mount(root) {
    const api = CD.hubApi.estoque;
    const st = { items: null, error: '', filter: 'todos', q: '', confirmId: 0 };
    const $ = s => root.querySelector(s);

    root.innerHTML = '<section class="pg">' +
      '<div class="pg-sum" id="sum"></div>' +
      '<div class="pg-tools"><nav class="chips" id="chips"></nav>' +
      '<label class="search">' + I.svg('search', 'currentColor', 26, 2) + '<input type="search" id="q" placeholder="' + esc(t('est.search')) + '" autocomplete="off"></label>' +
      '<button type="button" class="btn" data-act="new">' + I.svg('plus', 'currentColor', 26, 2.2) + '<span>' + t('est.new') + '</span></button></div>' +
      '<div class="pg-list" id="list"></div><div id="modal"></div></section>';

    function all() { return st.items || []; }

    function renderSummary() {
      const now = new Date(), soon = S.soonDays();
      let late = 0, warn = 0;
      all().forEach(it => { const x = S.expiry(it, now, soon); if (x.lvl === 'late') late++; else if (x.lvl === 'soon') warn++; });
      let h = '';
      if (late) h += '<span class="spill late">' + CD.i18n.n('stock.expired', late) + '</span>';
      if (warn) h += '<span class="spill soon">' + CD.i18n.n('est.soon', warn) + '</span>';
      if (!late && !warn && all().length) h += '<span class="spill ok">' + t('est.allok') + '</span>';
      $('#sum').innerHTML = h;
    }

    function renderChips() {
      const count = id => all().filter(it => id === 'todos' || it.local === id).length;
      const chips = [{ id: 'todos', label: t('c.all'), color: '' }].concat(S.LOCAIS);
      $('#chips').innerHTML = chips.map(c => '<button type="button" class="chip' + (st.filter === c.id ? ' on' : '') + '" data-filter="' + c.id + '">' +
        (c.color ? '<i style="background:' + c.color + '"></i>' : '') + esc(CD.i18n.user(c.label)) + '<b>' + count(c.id) + '</b></button>').join('');
    }

    function row(it, now, soon) {
      if (st.confirmId === it.id) {
        return '<article class="rw confirm"><div class="rw-main"><div class="rw-name">' + t('c.confirmDelete', { name: esc(it.nome) }) + '</div>' +
          '<div class="rw-meta">' + t('c.irreversible') + '</div></div><div class="rw-act">' +
          '<button type="button" class="btn danger" data-act="del" data-id="' + it.id + '">' + t('c.delete') + '</button>' +
          '<button type="button" class="btn ghost" data-act="cancel">' + t('c.cancel') + '</button></div></article>';
      }
      const L = S.local(it.local), x = S.expiry(it, now, soon);
      return '<article class="rw ' + x.lvl + '"><div class="rw-main"><div class="rw-name">' + esc(it.nome) + '</div>' +
        '<div class="rw-meta"><span class="loc" style="color:' + L.color + '">' + I.svg(L.icon, L.color, 22, 2) + esc(CD.i18n.user(L.label)) + '</span>' +
        '<span class="qty">' + esc(S.qty(it)) + '</span></div></div>' +
        '<div class="rw-exp ' + x.lvl + '"><b>' + esc(x.text) + '</b>' + (it.validade ? '<span>' + S.dataBr(it.validade) + '</span>' : '') + '</div>' +
        '<div class="rw-act"><button type="button" class="ib" data-act="edit" data-id="' + it.id + '" title="' + esc(t('c.edit')) + '" aria-label="' + esc(t('c.edit') + ' ' + it.nome) + '">' + I.svg('edit', 'currentColor', 28, 2) + '</button>' +
        '<button type="button" class="ib danger" data-act="ask" data-id="' + it.id + '" title="' + esc(t('c.delete')) + '" aria-label="' + esc(t('c.delete') + ' ' + it.nome) + '">' + I.svg('trash', 'currentColor', 28, 2) + '</button></div></article>';
    }

    function renderList() {
      const el = $('#list');
      let h = st.error ? '<div class="banner err"><span>' + esc(st.error) + '</span><button type="button" class="btn ghost" data-act="retry">' + t('c.retry') + '</button></div>' : '';
      if (!st.items) { el.innerHTML = h || '<div class="skel" style="height:96px"></div><div class="skel" style="height:96px"></div><div class="skel" style="height:96px"></div>'; return; }
      const q = st.q.trim().toLowerCase(), now = new Date(), soon = S.soonDays();
      const list = st.items.filter(it => (st.filter === 'todos' || it.local === st.filter) && (!q || it.nome.toLowerCase().indexOf(q) >= 0));
      if (!list.length) {
        h += '<div class="empty">' + (st.items.length ? t('est.notfound') : t('est.empty')) + '</div>';
      } else {
        h += list.map(it => row(it, now, soon)).join('');
      }
      el.innerHTML = h;
    }

    function renderAll() { renderSummary(); renderChips(); renderList(); }

    function changed() { st.items.sort(byExpiry); renderAll(); }

    async function load() {
      st.error = '';
      try { st.items = await api.list(); }
      catch (e) { st.error = e.message; }
      renderAll();
    }

    function form(it) {
      const cur = it || { nome: '', quantidade: null, pesoKg: null, validade: null, local: st.filter !== 'todos' ? st.filter : 'geladeira' };
      const val = v => v == null ? '' : esc(v);
      const body = CD.ui.field(t('est.f.name'), '<input name="nome" type="text" maxlength="120" autocomplete="off" value="' + esc(cur.nome) + '">') +
        '<div class="row2">' +
        CD.ui.field(t('est.f.qty'), '<input name="quantidade" type="number" inputmode="decimal" min="0" step="any" value="' + val(cur.quantidade) + '">', t('est.f.qtyHint')) +
        CD.ui.field(t('est.f.weight'), '<input name="pesoKg" type="number" inputmode="decimal" min="0" step="any" value="' + val(cur.pesoKg) + '">', t('est.f.weightHint')) + '</div>' +
        CD.ui.field(t('est.f.expiry'), '<input name="validade" type="date" value="' + val(cur.validade) + '">', t('c.optional')) +
        '<div class="fld"><span>' + t('est.f.place') + '</span><div class="seg">' + S.LOCAIS.map(l => '<label class="seg-i"><input type="radio" name="local" value="' + l.id + '"' +
          (cur.local === l.id ? ' checked' : '') + '><span>' + I.svg(l.icon, l.color, 26, 2) + esc(CD.i18n.user(l.label)) + '</span></label>').join('') + '</div></div>';

      CD.ui.modal($('#modal'), {
        title: it ? t('est.edit') : t('est.new'), body: body,
        read(f) {
          const el = f.elements, nome = el.nome.value.trim();
          const n = v => { v = String(v).trim().replace(',', '.'); return v === '' ? null : Number(v); };
          const q = n(el.quantidade.value), p = n(el.pesoKg.value);
          if (!nome) throw new Error(t('est.e.name'));
          if (q === null && p === null) throw new Error(t('est.e.qtyOrWeight'));
          if ((q !== null && !(q >= 0)) || (p !== null && !(p >= 0))) throw new Error(t('est.e.number'));
          return { nome: nome, quantidade: q, pesoKg: p, validade: el.validade.value || null, local: el.local.value };
        },
        async save(data) {
          if (it) { const r = await api.update(it.id, data); st.items[st.items.indexOf(it)] = r; }
          else st.items.push(await api.create(data));
          st.error = ''; changed();
        }
      });
    }

    root.addEventListener('click', async e => {
      const b = e.target.closest && e.target.closest('[data-act],[data-filter]');
      if (!b) return;
      if (b.hasAttribute('data-filter')) { st.filter = b.getAttribute('data-filter'); st.confirmId = 0; renderChips(); renderList(); return; }
      const act = b.getAttribute('data-act'), id = +b.getAttribute('data-id');
      const it = all().filter(x => x.id === id)[0];
      if (act === 'new') form(null);
      else if (act === 'edit' && it) form(it);
      else if (act === 'ask' && it) { st.confirmId = id; renderList(); }
      else if (act === 'cancel') { st.confirmId = 0; renderList(); }
      else if (act === 'retry') { st.items = null; renderList(); load(); }
      else if (act === 'del' && it) {
        b.disabled = true;
        try { await api.remove(id); st.items = st.items.filter(x => x.id !== id); st.error = ''; }
        catch (x) { st.error = x.message; }
        st.confirmId = 0; changed();
      }
    });
    $('#q').addEventListener('input', e => { st.q = e.target.value; st.confirmId = 0; renderList(); });

    renderAll();
    load();
  }

  CD.pages = CD.pages || {};
  CD.pages.estoque = { labelKey: 'page.estoque.label', subKey: 'page.estoque.sub', icon: 'fridge', mount: mount };
})(window.CD = window.CD || {});
