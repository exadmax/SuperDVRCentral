(function (CD) {
  // Página "Serviços" (contatos.html): profissionais e prestadores de serviço da casa (eletricista, encanador...), guardados no hub.
  const U = CD.util, I = CD.icons, esc = U.esc, t = CD.i18n.t;

  // Sugestões para o campo "tipo de serviço" (somadas aos tipos já cadastrados); a lista vem do idioma em uso
  const tipos0 = () => t('con.types');

  const tipoKey = t => t.trim().toLowerCase();
  const cap = t => t.trim().charAt(0).toUpperCase() + t.trim().slice(1);        // só a exibição: "chaveiro" -> "Chaveiro"
  const byName = (a, b) => a.tipoServico.localeCompare(b.tipoServico, CD.i18n.locale()) || a.nome.localeCompare(b.nome, CD.i18n.locale());
  // O campo "contato" aceita telefone, e-mail ou site; vira link conforme o que for (vários números juntos ficam só como texto)
  function contatoLink(c) {
    const t = c.trim();
    if (/^[^\s@\/]+@[^\s@\/]+\.[^\s@\/]+$/.test(t)) return { href: 'mailto:' + t, icon: 'mail' };
    if (/^(https?:\/\/|www\.)\S+$/i.test(t)) return { href: /^www\./i.test(t) ? 'https://' + t : t, icon: 'link', blank: true };
    if (/^[\d\s()+\-.]{8,}$/.test(t)) return { href: 'tel:' + t.replace(/[^\d+]/g, ''), icon: 'phone', wa: whatsappUrl(t) };
    return { icon: 'phone' };
  }
  // Link do WhatsApp (wa.me) para um número: com "+" ou "00" já vem com o país; sem isso, 10-11 dígitos (DDD + número) são do Brasil (55).
  // Números sem DDD ou com tamanho estranho (vários números juntos) ficam sem botão.
  function whatsappUrl(t) {
    let d = t.replace(/\D/g, ''), min = 12;
    if (/^\s*\+/.test(t) || /^\s*00/.test(t)) { d = d.replace(/^0+/, ''); min = 8; }        // já tem o código do país
    else { d = d.replace(/^0+/, ''); if (d.length === 10 || d.length === 11) d = '55' + d; }
    return d.length >= min && d.length <= 15 ? 'https://wa.me/' + d : '';
  }

  function mount(root) {
    const api = CD.hubApi.contatos;
    const st = { items: null, error: '', filter: '', q: '', confirmId: 0 };
    const $ = s => root.querySelector(s);

    root.innerHTML = '<section class="pg">' +
      '<div class="pg-sum" id="sum"></div>' +
      '<div class="pg-tools"><nav class="chips" id="chips"></nav>' +
      '<label class="search">' + I.svg('search', 'currentColor', 26, 2) + '<input type="search" id="q" placeholder="' + esc(t('con.search')) + '" autocomplete="off"></label>' +
      '<button type="button" class="btn" data-act="new">' + I.svg('plus', 'currentColor', 26, 2.2) + '<span>' + t('con.new') + '</span></button></div>' +
      '<div class="pg-list cards" id="list"></div><div id="modal"></div></section>';

    function all() { return st.items || []; }

    // Tipos já usados, sem repetir por maiúsculas/minúsculas, com a grafia da primeira ocorrência
    function tipos() {
      const seen = {}, out = [];
      all().forEach(c => { const k = tipoKey(c.tipoServico); if (!seen[k]) { seen[k] = 1; out.push({ key: k, label: cap(c.tipoServico) }); } });
      return out.sort((a, b) => a.label.localeCompare(b.label, CD.i18n.locale()));
    }

    function renderChips() {
      const count = k => all().filter(c => !k || tipoKey(c.tipoServico) === k).length;
      const chips = [{ key: '', label: t('c.all') }].concat(tipos());
      if (st.filter && !chips.some(c => c.key === st.filter)) st.filter = '';        // o último do tipo foi excluído
      $('#chips').innerHTML = chips.map(c => '<button type="button" class="chip' + (st.filter === c.key ? ' on' : '') + '" data-filter="' + esc(c.key) + '">' +
        esc(c.label) + '<b>' + count(c.key) + '</b></button>').join('');
      $('#sum').innerHTML = all().length ? '<span class="spill ok">' + CD.i18n.n('con.count', all().length) + '</span>' : '';
    }

    function contatoHtml(c, nome) {
      const l = contatoLink(c), inner = I.svg(l.icon, 'currentColor', 26, 2) + '<span class="num">' + esc(c) + '</span>';
      const main = l.href ? '<a class="cc-tel" href="' + esc(l.href) + '"' + (l.blank ? ' target="_blank" rel="noopener"' : '') + '>' + inner + '</a>'
        : '<div class="cc-tel">' + inner + '</div>';
      if (!l.wa) return main;
      const label = t('con.wa', { name: esc(nome) });
      return '<div class="cc-contact">' + main + '<a class="cc-wa" href="' + esc(l.wa) + '" target="_blank" rel="noopener" title="' + label + '" aria-label="' + label + '">' +
        I.svg('whatsapp', 'currentColor', 22, 2) + '<span>WhatsApp</span></a></div>';
    }

    function card(c) {
      if (st.confirmId === c.id) {
        return '<article class="cc confirm"><div class="cc-name">' + t('c.confirmDelete', { name: esc(c.nome) }) + '</div><div class="rw-meta">' + t('c.irreversible') + '</div>' +
          '<div class="cc-act"><button type="button" class="btn danger" data-act="del" data-id="' + c.id + '">' + t('c.delete') + '</button>' +
          '<button type="button" class="btn ghost" data-act="cancel">' + t('c.cancel') + '</button></div></article>';
      }
      return '<article class="cc"><div class="cc-top"><div class="cc-name">' + esc(c.nome) + '</div><span class="ctag">' + esc(cap(c.tipoServico)) + '</span></div>' +
        (c.contato ? contatoHtml(c.contato, c.nome) : '') +
        (c.servico ? '<div class="cc-note">' + esc(c.servico) + '</div>' : '') +
        (c.indicacao ? '<div class="cc-line">' + I.svg('user', 'currentColor', 22, 2) + '<span>' + esc(c.indicacao) + '</span></div>' : '') +
        (c.obs ? '<div class="cc-line">' + I.svg('alert', 'currentColor', 22, 2) + '<span>' + esc(c.obs) + '</span></div>' : '') +
        '<div class="cc-act"><button type="button" class="ib" data-act="edit" data-id="' + c.id + '" title="' + esc(t('c.edit')) + '" aria-label="' + esc(t('c.edit') + ' ' + c.nome) + '">' + I.svg('edit', 'currentColor', 28, 2) + '</button>' +
        '<button type="button" class="ib danger" data-act="ask" data-id="' + c.id + '" title="' + esc(t('c.delete')) + '" aria-label="' + esc(t('c.delete') + ' ' + c.nome) + '">' + I.svg('trash', 'currentColor', 28, 2) + '</button></div></article>';
    }

    function renderList() {
      const el = $('#list');
      let h = st.error ? '<div class="banner err"><span>' + esc(st.error) + '</span><button type="button" class="btn ghost" data-act="retry">' + t('c.retry') + '</button></div>' : '';
      if (!st.items) { el.innerHTML = h || '<div class="skel" style="height:190px"></div><div class="skel" style="height:190px"></div>'; return; }
      const q = st.q.trim().toLowerCase();
      const list = st.items.filter(c => (!st.filter || tipoKey(c.tipoServico) === st.filter) &&
        (!q || [c.nome, c.tipoServico, c.contato, c.indicacao, c.servico, c.obs].join(' ').toLowerCase().indexOf(q) >= 0));
      if (!list.length) h += '<div class="empty">' + (st.items.length ? t('con.notfound') : t('con.empty')) + '</div>';
      else h += list.map(card).join('');
      el.innerHTML = h;
    }

    function renderAll() { renderChips(); renderList(); }

    function changed() { st.items.sort(byName); renderAll(); }

    async function load() {
      st.error = '';
      try { st.items = await api.list(); } catch (e) { st.error = e.message; }
      renderAll();
    }

    function form(c) {
      const cur = c || { nome: '', tipoServico: st.filter ? (tipos().filter(x => x.key === st.filter)[0] || {}).label || '' : '', contato: '', indicacao: '', servico: '', obs: '' };
      const sugestoes = [], seen = {};
      tipos().map(x => x.label).concat(tipos0()).forEach(x => { if (!seen[tipoKey(x)]) { seen[tipoKey(x)] = 1; sugestoes.push(x); } });
      const body = CD.ui.field(t('con.f.name'), '<input name="nome" type="text" maxlength="120" autocomplete="off" value="' + esc(cur.nome) + '">') +
        CD.ui.field(t('con.f.type'), '<input name="tipoServico" type="text" maxlength="60" list="tipos-dl" autocomplete="off" placeholder="' + esc(t('con.f.typePh')) + '" value="' + esc(cur.tipoServico) + '">') +
        '<datalist id="tipos-dl">' + sugestoes.map(x => '<option value="' + esc(x) + '"></option>').join('') + '</datalist>' +
        CD.ui.field(t('con.f.contact'), '<input name="contato" type="text" maxlength="250" autocomplete="off" placeholder="' + esc(t('con.f.contactPh')) + '" value="' + esc(cur.contato) + '">', t('c.optional')) +
        CD.ui.field(t('con.f.ref'), '<input name="indicacao" type="text" maxlength="500" autocomplete="off" placeholder="' + esc(t('con.f.refPh')) + '" value="' + esc(cur.indicacao) + '">', t('c.optional')) +
        CD.ui.field(t('con.f.service'), '<textarea name="servico" rows="3" maxlength="2000" placeholder="' + esc(t('con.f.servicePh')) + '">' + esc(cur.servico) + '</textarea>', t('c.optional')) +
        CD.ui.field(t('con.f.notes'), '<textarea name="obs" rows="2" maxlength="2000">' + esc(cur.obs) + '</textarea>', t('c.optional'));

      CD.ui.modal($('#modal'), {
        title: c ? t('con.edit') : t('con.new'), body: body,
        read(f) {
          const el = f.elements, d = {
            nome: el.nome.value.trim(), tipoServico: el.tipoServico.value.trim(), contato: el.contato.value.trim(),
            indicacao: el.indicacao.value.trim(), servico: el.servico.value.trim(), obs: el.obs.value.trim()
          };
          if (!d.nome) throw new Error(t('con.e.name'));
          if (!d.tipoServico) throw new Error(t('con.e.type'));
          return d;
        },
        async save(data) {
          if (c) { const r = await api.update(c.id, data); st.items[st.items.indexOf(c)] = r; }
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
      const c = all().filter(x => x.id === id)[0];
      if (act === 'new') form(null);
      else if (act === 'edit' && c) form(c);
      else if (act === 'ask' && c) { st.confirmId = id; renderList(); }
      else if (act === 'cancel') { st.confirmId = 0; renderList(); }
      else if (act === 'retry') { st.items = null; renderList(); load(); }
      else if (act === 'del' && c) {
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
  CD.pages.contatos = { labelKey: 'page.contatos.label', subKey: 'page.contatos.sub', icon: 'phone', mount: mount };
})(window.CD = window.CD || {});
