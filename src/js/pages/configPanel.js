(function (CD) {
  // Config → Painel e Integrações: o que antes ficava fixo em arquivos.
  //  • Painel (vale na hora): localização do clima, modo noturno, rodízio das câmeras, aviso de validade, locais do estoque,
  //    página Vídeos e feriados locais. O painel guarda tudo no hub (/api/settings/panel); TV e celular buscam de lá.
  //  • Hub (vale depois de reiniciar o hub): pastas, cópia para o Drive, planilha de profissionais, jogos, tempos de cache e rede
  //    (/api/settings/hub). Segredos nunca voltam do hub: o campo fica em branco e “configurada” aparece ao lado.
  // Cada cartão é redesenhado sozinho (não o conjunto), para não perder o que está sendo digitado nos outros.
  const U = CD.util, esc = U.esc, ui = CD.cfgUi, t = CD.i18n.t;
  const fld = ui.fld, btn = ui.btn;

  const HOURS = Array.from({ length: 24 }, (_, h) => [h, String(h).padStart(2, '0') + ':00']);
  const HTYPES = ['municipal', 'estadual', 'nacional'].map(id => [id, t('hol.type.' + id)]);

  const input = (f, v, o) => {
    o = o || {};
    return '<input data-f="' + f + '" type="' + (o.type || 'text') + '" value="' + esc(v == null ? '' : v) + '"' + (o.ph ? ' placeholder="' + esc(o.ph) + '"' : '') +
      (o.attr ? ' ' + o.attr : '') + ' autocomplete="' + (o.type === 'password' ? 'new-password' : 'off') + '" spellcheck="false">';
  };
  const num = (f, v, min, max, step) => input(f, v, { type: 'number', attr: 'min="' + min + '" max="' + max + '"' + (step ? ' step="' + step + '"' : '') });
  const select = (f, v, opts) => '<select data-f="' + f + '">' + opts.map(o => '<option value="' + esc(o[0]) + '"' + (String(o[0]) === String(v) ? ' selected' : '') + '>' + esc(o[1]) + '</option>').join('') + '</select>';
  const check = (f, v, label) => '<label class="cfg-chk"><input type="checkbox" data-f="' + f + '"' + (v ? ' checked' : '') + '>' + esc(label) + '</label>';

  function read(el) {
    const o = {};
    el.querySelectorAll('[data-f]').forEach(e => { o[e.getAttribute('data-f')] = e.type === 'checkbox' ? e.checked : e.value; });
    return o;
  }
  const int = (v, d) => { const n = parseInt(v, 10); return isNaN(n) ? d : n; };
  const flt = (v, d) => { const n = parseFloat(String(v).replace(',', '.')); return isNaN(n) ? d : n; };

  function mount(root) {
    root.innerHTML = '';
    const cfg = CD.state.cfg;
    const st = { hub: null, locais: null, hol: null }, msgs = {}, busy = {};
    const say = (id, text, bad) => { msgs[id] = text ? { text: text, bad: !!bad } : null; };

    // ---------- esqueleto dos cartões ----------
    function card(id, tab, title, hint, body, save) {
      const m = msgs[id];
      return '<div class="cfg-card" data-tab="' + tab + '" data-card="' + id + '"><h2>' + esc(title) + '</h2>' + (hint ? '<div class="cfg-hint">' + hint + '</div>' : '') + body +
        (m ? '<div class="cfg-msg' + (m.bad ? ' cfg-err' : '') + '">' + esc(m.text) + '</div>' : '') +
        (save === false ? '' : '<div class="cfg-bar2">' + (save || '') + btn('save:' + id, t('c.save'), '', busy[id]) + '</div>') + '</div>';
    }
    const grid = html => '<div class="cfg-grid">' + html + '</div>';

    // ---------- Painel ----------
    function cLoc() {
      const l = cfg.location || {};
      return card('loc', 'painel', t('cp.loc.title'), t('cp.loc.hint'),
        grid(fld(t('cp.loc.city'), input('city', l.city)) + fld(t('cp.loc.state'), input('state', l.state)) +
          fld(t('cp.loc.lat'), num('lat', l.lat, -90, 90, 'any')) + fld(t('cp.loc.lon'), num('lon', l.lon, -180, 180, 'any')) +
          fld(t('cp.loc.tz'), input('timezone', l.timezone, { ph: 'America/Sao_Paulo' })) +
          fld(t('cp.loc.refresh'), num('refreshMinutes', cfg.refreshMinutes || 15, 5, 240))));
    }
    function cNight() {
      const n = cfg.night || { from: 23, until: 5 };
      return card('night', 'painel', t('cp.night.title'), t('cp.night.hint'),
        grid(fld(t('cp.night.from'), select('from', n.from, HOURS)) + fld(t('cp.night.until'), select('until', n.until, HOURS))));
    }
    function cRot() {
      return card('rot', 'painel', t('cp.rot.title'), t('cp.rot.hint'),
        grid(fld(t('cp.rot.secs'), num('rotateSeconds', (cfg.cameras || {}).rotateSeconds || 60, 5, 3600))));
    }
    function cSoon() {
      return card('soon', 'painel', t('cp.soon.title'), t('cp.soon.hint'),
        grid(fld(t('cp.soon.days'), num('soonDays', (cfg.estoque || {}).soonDays == null ? 3 : cfg.estoque.soonDays, 0, 365))));
    }
    function cVid() {
      const v = cfg.videos || {};
      return card('vid', 'painel', t('cp.vid.title'), t('cp.vid.hint'),
        grid(fld(t('cp.vid.port'), num('port', v.port || 8080, 1, 65535)) + fld(t('ce.menu.path'), input('path', v.path || '/videos/', { ph: '/videos/' }))));
    }

    function localRow(l, i) {
      return '<div class="cfg-item" data-r="' + i + '"><div class="cfg-grid">' + fld(t('ce.name'), input('label', l.label, { ph: t('cp.locais.namePh') })) +
        fld(t('ce.color'), '<input type="color" data-f="color" value="' + esc(l.color || '#4DA3FF') + '">') +
        fld(t('ce.menu.icon'), select('icon', l.icon || 'box', ui.ICONS)) + '</div><div class="cfg-ia">' + btn('del:locais:' + i, t('c.remove'), 'ghost') + '</div></div>';
    }
    function cLocais() {
      return card('locais', 'painel', t('cp.locais.title'), t('cp.locais.hint'),
        st.locais.map(localRow).join(''), btn('add:locais', t('cp.locais.add')));
    }

    // Livros: liga/desliga a página e define as listas de categorias e de donos (uma por linha); vale na hora
    function cBooks() {
      const b = cfg.books || {}, cats = b.categories && b.categories.length ? b.categories : t('liv.cats');
      const area = (f, v, ph) => '<textarea data-f="' + f + '" rows="7" placeholder="' + esc(ph) + '" spellcheck="false">' + esc((v || []).join('\n')) + '</textarea>';
      return card('books', 'livros', t('cp.books.title'), t('cp.books.hint'),
        grid('<div class="wide">' + check('enabled', b.enabled !== false, t('cp.books.enable')) + '</div>' +
          fld(t('cp.books.cats'), area('categories', cats, t('cp.books.catsPh'))) + fld(t('cp.books.owners'), area('owners', b.owners, t('cp.books.ownersPh')))));
    }

    function holRow(h, i) {
      return '<div class="cfg-item" data-r="' + i + '"><div class="cfg-grid">' + fld(t('cp.hol.date'), input('date', h.date ? '2000-' + h.date : '', { type: 'date' })) +
        fld(t('ce.name'), input('name', h.name, { ph: t('cp.hol.namePh') })) + fld(t('cp.hol.type'), select('type', h.type || 'municipal', HTYPES)) +
        '</div><div class="cfg-ia">' + btn('del:hol:' + i, t('c.remove'), 'ghost') + '</div></div>';
    }
    function cHol() {
      return card('hol', 'painel', t('cp.hol.title'), t('cp.hol.hint'),
        st.hol.map(holRow).join(''), btn('add:hol', t('cp.hol.add')));
    }

    // ---------- Hub ----------
    function pendingBanner() {
      const h = st.hub;
      if (!h || !h.pending) return '';
      return '<div class="cfg-banner"><span>' + t('cp.pending') + (h.canRestart ? '' : ' ' + t('cp.pendingManual')) + '</span>' +
        (h.canRestart ? btn('restart', busy.restart ? t('cp.restarting') : t('cp.restartNow'), '', busy.restart) : '') + '</div>';
    }
    const secret = (f, label, set, ph) => '<div class="fld wide"><span>' + esc(label) + '</span>' +
      input(f, '', { type: 'password', ph: set ? t('cp.secretSet') : ph || t('cp.secretPaste') }) +
      (set ? '<label class="cfg-chk"><input type="checkbox" data-clear="' + f + '">' + esc(t('cp.secretClear')) + '</label>' : '') + '</div>';

    function cDrive() {
      const h = st.hub;
      return card('hubdrive', 'cameras', t('cp.drive.title'), t('cp.drive.hint'),
        grid(fld(t('cp.drive.dir'), input('recordingsDir', h.recordingsDir, { ph: '/home/usuario/securityCam' }), 'wide') +
          '<div class="wide">' + check('driveEnabled', h.driveEnabled, t('cp.drive.enable')) + '</div>' +
          fld(t('cp.drive.remote'), input('driveRemote', h.driveRemote, { ph: 'GDrive:CameraSeguranca' }), 'wide') +
          fld(t('cp.drive.hour'), select('driveHour', h.driveHour, HOURS)) +
          fld(t('cp.drive.maxMB'), num('driveMaxFileMB', h.driveMaxFileMB, 0, 1048576))));
    }
    function cSheet() {
      const h = st.hub;
      return card('hubsheet', 'integracoes', t('cp.sheet.title'), t('cp.sheet.hint'),
        grid(fld(t('cp.sheet.url'), input('sheetUrl', h.sheetUrl, { ph: 'https://docs.google.com/spreadsheets/d/ID/gviz/tq?tqx=out:csv&sheet=profissionais' }), 'wide') +
          fld(t('cp.sheet.min'), num('sheetMinutes', h.sheetMinutes || 5, 1, 1440))));
    }
    function cGames() {
      const h = st.hub;
      return card('hubgames', 'integracoes', t('cp.games.title'), t('cp.games.hint'),
        grid(fld(t('cp.games.min'), num('gamesCacheMinutes', h.gamesCacheMinutes || 360, 1, 10080), 'wide') +
          '<div class="cfg-sub wide">Steam</div>' + fld(t('cp.games.steamUser'), input('steamUser', h.steamUser)) + secret('steamKey', t('cp.games.apiKey'), h.steamKeySet) +
          '<div class="cfg-sub wide">PlayStation</div>' + fld(t('cp.games.psnUser'), input('psnUser', h.psnUser)) + secret('psnNpsso', t('cp.games.npsso'), h.psnNpssoSet) +
          '<div class="cfg-sub wide">Xbox</div>' + fld('Gamertag', input('xboxGamertag', h.xboxGamertag)) + fld(t('cp.games.clientId'), input('xboxClientId', h.xboxClientId)) +
          secret('xboxClientSecret', t('cp.games.clientSecret'), h.xboxClientSecretSet) +
          '<div class="cfg-sub wide">RetroAchievements</div>' + fld(t('cp.games.retroUser'), input('retroUser', h.retroUser)) + secret('retroKey', t('cp.games.apiKey'), h.retroKeySet)));
    }
    function cMisc() {
      const h = st.hub;
      return card('hubmisc', 'integracoes', t('cp.misc.title'), t('cp.restartNote'),
        grid(fld(t('cp.misc.videosDir'), input('videosDir', h.videosDir, { ph: '/home/usuario/Videos' }), 'wide') +
          fld(t('cp.misc.calCache'), num('calendarCacheMinutes', h.calendarCacheMinutes, 1, 1440)) +
          fld(t('cp.misc.trafficCache'), num('trafficCacheMinutes', h.trafficCacheMinutes, 1, 1440)) +
          '<div class="wide">' + check('forceIPv4', h.forceIPv4, t('cp.misc.ipv4')) + '</div>'));
    }

    // ---------- desenho ----------
    const R = { loc: cLoc, night: cNight, rot: cRot, soon: cSoon, vid: cVid, locais: cLocais, hol: cHol, books: cBooks,
      hubdrive: cDrive, hubsheet: cSheet, hubgames: cGames, hubmisc: cMisc };
    const HUBCARDS = ['hubdrive', 'hubsheet', 'hubgames', 'hubmisc'];
    const ORDER = ['loc', 'night', 'rot', 'soon', 'locais', 'vid', 'hol', 'books', 'hubdrive', 'hubsheet', 'hubgames', 'hubmisc'];

    function cardEl(id) { return root.querySelector('[data-card="' + id + '"]'); }
    function redraw(id) {
      const el = cardEl(id);
      if (!el || (HUBCARDS.indexOf(id) >= 0 && !st.hub)) return;
      const t = document.createElement('div');
      t.innerHTML = R[id]();
      el.parentNode.replaceChild(t.firstChild, el);
    }
    function drawBanner() { const b = root.querySelector('#cp-banner'); if (b) b.innerHTML = pendingBanner(); }
    function drawAll() {
      if (!CD.state.hub) { root.innerHTML = ''; return; }
      root.innerHTML = '<div id="cp-banner"></div>' + ORDER.filter(id => HUBCARDS.indexOf(id) < 0 || st.hub).map(id => R[id]()).join('');
      drawBanner();
    }

    // ---------- listas (locais e feriados) ----------
    function collectList(kind) {
      const el = cardEl(kind), rows = kind === 'locais' ? st.locais : st.hol;
      if (!el) return;
      el.querySelectorAll('.cfg-item').forEach((it, i) => {
        const o = read(it), r = rows[i];
        if (!r) return;
        if (kind === 'locais') { r.label = o.label; r.color = o.color; r.icon = o.icon; }
        else { r.date = (o.date || '').slice(5); r.name = o.name; r.type = o.type; }
      });
    }

    // ---------- salvar ----------
    // Mostra a mensagem do cartão sem redesenhá-lo (assim um erro de validação não apaga o que foi digitado)
    function showMsg(id) {
      const el = cardEl(id), x = msgs[id];
      if (!el) return;
      let m = el.querySelector('.cfg-msg');
      if (!x) { if (m) m.remove(); return; }
      if (!m) { m = document.createElement('div'); el.insertBefore(m, el.querySelector('.cfg-bar2')); }
      m.className = 'cfg-msg' + (x.bad ? ' cfg-err' : '');
      m.textContent = x.text;
    }
    function setBusy(id, on) {
      busy[id] = on;
      const el = cardEl(id), b = el && el.querySelector('[data-a="save:' + id + '"]');
      if (b) b.disabled = on;
    }
    function resyncLists() {
      st.locais = (CD.stock ? CD.stock.LOCAIS : []).map(l => Object.assign({}, l));
      st.hol = (cfg.localHolidays || window.CD_LOCAL_HOLIDAYS || []).map(h => Object.assign({}, h));
    }

    async function savePanel(id, payload, resync) {
      setBusy(id, true); say(id, t('c.saving')); showMsg(id);
      let ok = false;
      try {
        await CD.hubApi.settings.save('panel', payload);
        await CD.hubApi.settings.loadPanel(cfg);
        say(id, t('cp.saved')); ok = true;
      } catch (e) { say(id, e.message, true); }
      setBusy(id, false);
      if (ok || resync) { resyncLists(); redraw(id); } else showMsg(id);
    }

    function panelPayload(id) {
      const el = cardEl(id), o = el ? read(el) : {};
      if (id === 'loc') return { location: { city: o.city, state: o.state, lat: flt(o.lat, 0), lon: flt(o.lon, 0), timezone: o.timezone }, refreshMinutes: int(o.refreshMinutes, 15) };
      if (id === 'night') return { night: { from: int(o.from, 23), until: int(o.until, 5) } };
      if (id === 'rot') return { cameras: { rotateSeconds: int(o.rotateSeconds, 60) } };
      if (id === 'soon') return { estoque: { soonDays: int(o.soonDays, 3) } };
      if (id === 'vid') return { videos: { port: int(o.port, 8080), path: o.path } };
      if (id === 'books') { const lines = v => String(v || '').split(/\r?\n/).map(x => x.trim()).filter(Boolean); return { books: { enabled: !!o.enabled, categories: lines(o.categories), owners: lines(o.owners) } }; }
      collectList(id);
      if (id === 'locais') return { locais: st.locais.filter(l => l.id || (l.label || '').trim()).map(l => ({ id: l.id || '', label: l.label, color: l.color, icon: l.icon })) };
      return { localHolidays: st.hol.filter(h => h.date || (h.name || '').trim()).map(h => ({ date: h.date, name: h.name, type: h.type })) };
    }

    const NUMERIC = ['driveHour', 'driveMaxFileMB', 'sheetMinutes', 'gamesCacheMinutes', 'calendarCacheMinutes', 'trafficCacheMinutes'];
    async function saveHub(from) {
      const body = {}, clear = [];
      HUBCARDS.forEach(id => {
        const el = cardEl(id);
        if (!el) return;
        const o = read(el);
        Object.keys(o).forEach(k => { body[k] = NUMERIC.indexOf(k) >= 0 ? int(o[k], 0) : o[k]; });
        el.querySelectorAll('[data-clear]:checked').forEach(c => clear.push(c.getAttribute('data-clear')));
      });
      body.clearSecrets = clear;
      setBusy(from, true); say(from, t('c.saving')); showMsg(from);
      try {
        st.hub = await CD.hubApi.settings.save('hub', body);
        say(from, st.hub.pending ? t('cp.savedPending') : t('cp.savedOk'));
      } catch (e) { say(from, e.message, true); setBusy(from, false); showMsg(from); return; }
      setBusy(from, false);
      HUBCARDS.forEach(redraw); drawBanner();
    }

    async function restart() {
      if (!confirm(t('cp.restartAsk'))) return;
      busy.restart = true; drawBanner();
      try { await CD.hubApi.settings.restartHub(); } catch (e) { busy.restart = false; say('hubmisc', e.message, true); redraw('hubmisc'); drawBanner(); return; }
      const t0 = Date.now();
      const poll = async () => {
        try { await U.getJson(CD.state.hub + '/api/health', 2500); if (Date.now() - t0 > 2500) { busy.restart = false; st.hub = await CD.hubApi.settings.get('hub'); HUBCARDS.forEach(redraw); drawBanner(); return; } } catch (e) { /* ainda reiniciando */ }
        if (Date.now() - t0 > 90000) { busy.restart = false; drawBanner(); return; }
        setTimeout(poll, 2000);
      };
      setTimeout(poll, 2500);
    }

    // ---------- ações ----------
    root.addEventListener('click', async e => {
      const b = e.target.closest && e.target.closest('[data-a]');
      if (!b || b.disabled) return;
      const p = b.getAttribute('data-a').split(':'), op = p[0], what = p[1];
      if (op === 'restart') return restart();
      if (op === 'save') return HUBCARDS.indexOf(what) >= 0 ? saveHub(what) : savePanel(what, panelPayload(what));
      if (what !== 'locais' && what !== 'hol') return;
      collectList(what);
      const rows = what === 'locais' ? st.locais : st.hol;
      if (op === 'add') {
        rows.push(what === 'locais' ? { id: '', label: '', color: '#4DA3FF', icon: 'box' } : { date: '', name: '', type: 'municipal' });
        say(what, ''); return redraw(what);
      }
      if (op === 'del') {
        const i = +p[2], r = rows[i], fresh = what === 'locais' ? !r.id : false;
        const blank = !(r.label || r.name || r.date || '').trim();
        if (!blank && !confirm(t('ce.removeAsk', { name: r.label || r.name || t('ce.thisItem') }))) return;
        rows.splice(i, 1);
        say(what, ''); redraw(what);
        if (fresh || (what === 'hol' && blank)) return;
        return savePanel(what, panelPayload(what), true);   // remover vale na hora (se o local ainda tiver itens, o hub recusa e a lista volta)
      }
    });

    // ---------- carga ----------
    async function load() {
      if (!CD.state.hub) return drawAll();
      resyncLists();
      try { st.hub = await CD.hubApi.settings.get('hub'); } catch (e) { st.hub = null; }
      drawAll();
      if (!st.hub) root.insertAdjacentHTML('beforeend', '<div class="cfg-card" data-tab="integracoes"><h2>' + t('cp.old.title') + '</h2><div class="cfg-msg">' + t('cp.old.text') + '</div></div>');
    }
    load();
  }

  CD.configPanel = { mount: mount };
})(window.CD = window.CD || {});
