(function (CD) {
  // Parte editável da página Config: câmeras, agendas (iCal), rotas do trânsito (origem/destino) e atalhos do menu, mais o
  // modo compatibilidade. Tudo é guardado no hub (hub.settings.json) e vale na hora, sem reiniciar. Senhas e chaves nunca
  // voltam do hub: o campo mostra a versão mascarada e, deixado em branco, mantém o valor atual.
  // Esta área só é redesenhada por ações do usuário (nunca pelo recarregamento periódico da página), para não perder o que está sendo digitado.
  const U = CD.util, esc = U.esc, t = CD.i18n.t;
  const ICONS = ['gamepad', 'fridge', 'phone', 'camera', 'music', 'film', 'folder', 'gear', 'home', 'link', 'box', 'clock', 'mail', 'user', 'pin', 'trophy',
    'umbrella', 'search', 'book', 'desktop'].map(id => [id, t('ico.' + id)]);
  const DAYS = [1, 2, 3, 4, 5, 6, 0].map(d => [d, t('date.daysShort')[d]]);
  const HEX = /^#[0-9a-fA-F]{6}$/;

  const fld = (label, inner, cls) => '<label class="fld' + (cls ? ' ' + cls : '') + '"><span>' + esc(label) + '</span>' + inner + '</label>';
  const inp = (k, v, ph, extra) => '<input data-k="' + k + '" value="' + esc(v) + '"' + (ph ? ' placeholder="' + esc(ph) + '"' : '') + ' autocomplete="off" spellcheck="false"' + (extra || '') + '>';
  const btn = (a, label, cls, dis) => '<button type="button" class="btn ' + (cls || 'ghost') + '" data-a="' + a + '"' + (dis ? ' disabled' : '') + '>' + esc(label) + '</button>';

  CD.cfgUi = { fld: fld, inp: inp, btn: btn, ICONS: ICONS };       // usado também por configPanel.js

  const TAB = { cameras: 'cameras', calendars: 'agendas', traffic: 'agendas', menu: 'menu' };

  function mount(root) {
    root.innerHTML = '<div class="cfg" id="ce"></div>';
    const box = root.firstChild;
    const ed = { cameras: null, calendars: null, traffic: null, menu: null, menuCustom: false };
    const msgs = {}, busy = {};

    const say = (sec, text, bad) => { msgs[sec] = text ? { text: text, bad: !!bad } : null; };
    const note = sec => msgs[sec] ? '<div class="cfg-msg' + (msgs[sec].bad ? ' cfg-err' : '') + '">' + esc(msgs[sec].text) + '</div>' : '';
    const card = (sec, title, intro, body) => '<div class="cfg-card" data-tab="' + TAB[sec] + '" data-sec="' + sec + '"><h2>' + esc(title) + '</h2><div class="cfg-hint">' + intro + '</div>' + body + note(sec) + '</div>';

    // ---------- câmeras ----------
    const keepBlank = '  (' + t('ce.keepBlank') + ')';
    function camRow(c, i) {
      return '<div class="cfg-item" data-i="' + i + '"><div class="cfg-grid">' +
        fld(t('ce.name'), inp('name', c.name, t('ce.cam.namePh'))) +
        fld(t('ce.cam.rtsp'), inp('url', c.url, c.masked ? c.masked + keepBlank : 'rtsp://usuario:senha@192.168.0.10:554/cam/realmonitor?channel=1&subtype=1', ' type="text"'), 'wide') +
        '</div><div class="cfg-ia">' + btn('del:cameras:' + i, t('c.remove'), 'ghost') + '</div></div>';
    }
    function camerasCard() {
      if (!ed.cameras) return card('cameras', t('cfg.tab.cameras'), t('c.loading'), '');
      return card('cameras', t('cfg.tab.cameras'), t('ce.cam.intro'),
        ed.cameras.map(camRow).join('') + '<div class="cfg-bar2">' + btn('add:cameras', t('ce.cam.add')) + btn('save:cameras', t('ce.cam.save'), '', busy.cameras) + '</div>');
    }

    // ---------- agendas ----------
    function calRow(c, i) {
      const hex = HEX.test(c.color) ? c.color : '#B48CFF';      // cor não sólida (degradê) não cabe no seletor: só muda se o usuário escolher outra
      return '<div class="cfg-item" data-i="' + i + '"><div class="cfg-grid">' +
        fld(t('ce.name'), inp('label', c.label, t('ce.cal.namePh'))) +
        fld(t('ce.color'), '<input type="color" data-k="color" data-touched="' + (c.colorTouched ? 1 : 0) + '" value="' + hex + '">') +
        fld(t('ce.cal.url'), inp('url', c.url, c.masked ? c.masked + keepBlank : 'https://calendar.google.com/calendar/ical/.../basic.ics'), 'wide') +
        '</div><div class="cfg-ia">' + btn('del:calendars:' + i, t('c.remove'), 'ghost') + '</div></div>';
    }
    function calendarsCard() {
      if (!ed.calendars) return card('calendars', t('ce.cal.title'), t('c.loading'), '');
      return card('calendars', t('ce.cal.title'), t('ce.cal.intro'),
        ed.calendars.map(calRow).join('') + '<div class="cfg-bar2">' + btn('add:calendars', t('ce.cal.add')) + btn('save:calendars', t('ce.cal.save'), '', busy.calendars) + '</div>');
    }

    // ---------- rotas do trânsito ----------
    function routeRow(p, r, pi, ri) {
      const days = DAYS.map(d => '<label><input type="checkbox" data-day="' + d[0] + '"' + (r.days.indexOf(d[0]) >= 0 ? ' checked' : '') + '>' + esc(d[1]) + '</label>').join('');
      return '<div class="cfg-item" data-r="' + ri + '"><div class="cfg-grid">' +
        fld(t('ce.route.name'), inp('name', r.name, t('ce.route.namePh'))) +
        fld(t('ce.route.after'), inp('after', r.after, 'HH:MM', ' type="time"')) +
        (r.home ? '' : fld(t('ce.route.from'), inp('from', r.from, t('ce.route.fromPh')), 'wide') + fld(t('ce.route.to'), inp('to', r.to, t('ce.route.toPh')), 'wide')) +
        '</div><label class="cfg-chk"><input type="checkbox" data-k="home"' + (r.home ? ' checked' : '') + ' data-redraw="1">' + esc(t('ce.route.home')) + '</label>' +
        '<div class="cfg-days">' + days + '</div>' +
        '<div class="cfg-ia">' + btn('del:route:' + pi + ':' + ri, t('ce.route.remove'), 'ghost') + '</div></div>';
    }
    // Regras de consulta da API de rotas (exemplos de uso comuns):
    //  • ondemand — só consulta quando alguém toca em “Ver trânsito” (vale por 10 min); não gasta cota sozinho.
    //  • window   — consulta sozinho só entre dois horários; antes mostra “Disponível às…”, depois mostra o último resultado e some no horário de esconder.
    //  • always   — consulta sempre que o painel atualiza (respeitando o intervalo de cache do hub).
    const MODES = [['ondemand', t('ce.mode.ondemand')], ['window', t('ce.mode.window')], ['always', t('ce.mode.always')]];
    const MODE_HELP = { ondemand: t('ce.modeHelp.ondemand'), window: t('ce.modeHelp.window'), always: t('ce.modeHelp.always') };
    function personBlock(p, pi) {
      const mode = p.mode || 'always';
      return '<div class="cfg-cam" data-p="' + pi + '"><div class="cfg-sub">' + t('ce.person', { n: pi + 1 }) + '</div>' +
        '<div class="cfg-grid">' + fld(t('ce.person.name'), inp('label', p.label, t('ce.person.namePh'))) + fld(t('ce.person.arrive'), inp('arriveBy', p.arriveBy, 'HH:MM', ' type="time"')) + '</div>' +
        '<div class="cfg-grid">' + fld(t('ce.person.when'), '<select data-k="mode" data-redraw="1">' + MODES.map(m => '<option value="' + m[0] + '"' + (m[0] === mode ? ' selected' : '') + '>' + esc(m[1]) + '</option>').join('') + '</select>', 'wide') +
        (mode === 'window' ? fld(t('ce.person.winFrom'), inp('windowFrom', p.windowFrom, 'HH:MM', ' type="time"')) + fld(t('ce.person.winTo'), inp('windowTo', p.windowTo, 'HH:MM', ' type="time"')) +
          fld(t('ce.person.hideAfter'), inp('showUntil', p.showUntil, 'HH:MM', ' type="time"')) : '') + '</div>' +
        '<div class="cfg-hint">' + MODE_HELP[mode] + '</div>' +
        p.routes.map((r, ri) => routeRow(p, r, pi, ri)).join('') +
        '<div class="cfg-bar2">' + btn('add:route:' + pi, t('ce.route.add')) + btn('del:person:' + pi, t('ce.person.remove', { name: p.label || t('ce.person.fallback') }), 'ghost') + '</div></div>';
    }
    function trafficCard() {
      const tr = ed.traffic;
      if (!tr) return card('traffic', t('ce.tr.title'), t('c.loading'), '');
      return card('traffic', t('ce.tr.title'), t('ce.tr.intro'),
        '<div class="cfg-grid">' + fld(t('ce.tr.home'), '<input data-t="home" value="' + esc(tr.home) + '" placeholder="' + esc(t('ce.tr.homePh')) + '" autocomplete="off">', 'wide') +
        fld(t('ce.tr.key'), '<input data-t="key" type="password" value="' + esc(tr.key) + '" placeholder="' + esc(tr.keySet ? t('ce.tr.keySet') : t('ce.tr.keyPaste')) + '" autocomplete="new-password">', 'wide') + '</div>' +
        tr.people.map(personBlock).join('') + '<div class="cfg-bar2">' + btn('add:person', t('ce.person.add')) + btn('save:traffic', t('ce.tr.save'), '', busy.traffic) + '</div>');
    }

    // ---------- menu ----------
    function menuRow(l, i, n) {
      const icons = ICONS.some(x => x[0] === l.icon) || !l.icon ? ICONS : ICONS.concat([[l.icon, l.icon]]);
      const locked = !l.port && l.path === '/config.html';
      return '<div class="cfg-item" data-i="' + i + '"><div class="cfg-grid">' +
        fld(t('ce.name'), inp('label', l.label, t('ce.menu.namePh'))) + fld(t('ce.menu.desc'), inp('sub', l.sub, t('ce.menu.descPh'))) +
        fld(t('ce.menu.icon'), '<select data-k="icon">' + icons.map(x => '<option value="' + esc(x[0]) + '"' + (x[0] === (l.icon || 'link') ? ' selected' : '') + '>' + esc(x[1]) + '</option>').join('') + '</select>') +
        fld(t('ce.menu.port'), inp('port', l.port || '', 'Ex.: 4533', ' type="number" min="1" max="65535"')) +
        fld(t('ce.menu.path'), inp('path', l.path, '/estoque.html'), 'wide') + '</div>' +
        '<div class="cfg-ia">' + btn('up:menu:' + i, '↑', 'ghost', i === 0) + btn('down:menu:' + i, '↓', 'ghost', i === n - 1) +
        '<span class="grow cfg-hint">' + (locked ? t('ce.menu.locked') : '') + '</span>' + btn('del:menu:' + i, t('c.remove'), 'ghost', locked) + '</div></div>';
    }
    function menuCard() {
      if (!ed.menu) return card('menu', t('ce.menu.title'), t('c.loading'), '');
      return card('menu', t('ce.menu.title'), t('ce.menu.intro') + ' ' + (ed.menuCustom ? t('ce.menu.custom') : t('ce.menu.isDefault')),
        ed.menu.map((l, i) => menuRow(l, i, ed.menu.length)).join('') + '<div class="cfg-bar2">' + btn('add:menu', t('ce.menu.add')) + btn('save:menu', t('ce.menu.save'), '', busy.menu) +
        (ed.menuCustom ? btn('reset:menu', t('ce.menu.reset')) : '') + '</div>');
    }

    // ---------- idioma (vale só para este aparelho) ----------
    function langCard() {
      const I18N = CD.i18n, pref = I18N.pref();
      const opts = [['auto', t('ce.lang.auto', { name: I18N.LANGS.filter(l => l.id === I18N.detected())[0].name })]].concat(I18N.LANGS.map(l => [l.id, l.name]));
      return '<div class="cfg-card" data-tab="idioma" data-sec="idioma"><h2>' + t('ce.lang.title') + '</h2>' +
        '<div class="cfg-hint">' + t('ce.lang.intro') + '</div>' +
        '<div class="cfg-a"><div class="cfg-at"><b>' + t('ce.lang.label') + '</b><span>' + esc(t('ce.lang.current', { name: I18N.LANGS.filter(l => l.id === I18N.lang())[0].name })) + '</span></div>' +
        '<select class="cfg-sel" id="lang-sel">' + opts.map(o => '<option value="' + o[0] + '"' + (o[0] === pref ? ' selected' : '') + '>' + esc(o[1]) + '</option>').join('') + '</select></div></div>';
    }

    // ---------- modo compatibilidade ----------
    function compatCard() {
      const C = CD.compat, m = C.mode();
      const opts = [['auto', t('ce.compat.auto')], ['on', t('ce.compat.on')], ['off', t('ce.compat.off')]];
      return '<div class="cfg-card" data-tab="compat" data-sec="compat"><h2>' + t('ce.compat.title') + '</h2>' +
        '<div class="cfg-hint">' + t('ce.compat.intro') + '</div>' +
        '<div class="cfg-a"><div class="cfg-at"><b>' + t('ce.compat.mode') + '</b><span id="compat-st">' + esc(status()) + '</span></div>' +
        '<select class="cfg-sel" id="compat-sel">' + opts.map(o => '<option value="' + o[0] + '"' + (o[0] === m ? ' selected' : '') + '>' + esc(o[1]) + '</option>').join('') + '</select></div></div>';
    }
    function status() {
      const C = CD.compat;
      return (C.active() ? t('ce.compat.stOn') : t('ce.compat.stOff')) + ' ' + t('ce.compat.tv', { v: C.detected() ? t('c.yes') : t('c.no') });
    }

    function draw() {
      if (!CD.state.hub) { box.innerHTML = langCard() + compatCard(); return; }
      box.innerHTML = camerasCard() + calendarsCard() + trafficCard() + menuCard() + langCard() + compatCard();
    }

    // ---------- DOM -> estado ----------
    const val = (item, k) => { const e = item.querySelector('[data-k="' + k + '"]'); return !e ? '' : e.type === 'checkbox' ? e.checked : e.value; };
    function collect() {
      const sec = n => box.querySelectorAll('[data-sec="' + n + '"] > .cfg-item');
      if (ed.cameras) sec('cameras').forEach((it, i) => { if (ed.cameras[i]) { ed.cameras[i].name = val(it, 'name'); ed.cameras[i].url = val(it, 'url'); } });
      if (ed.calendars) sec('calendars').forEach((it, i) => {
        const c = ed.calendars[i]; if (!c) return;
        c.label = val(it, 'label'); c.url = val(it, 'url');
        const col = it.querySelector('[data-k="color"]');
        if (col && col.getAttribute('data-touched') === '1') { c.color = col.value; c.colorTouched = true; }
      });
      if (ed.traffic) {
        const t = ed.traffic, h = box.querySelector('[data-t="home"]'), k = box.querySelector('[data-t="key"]');
        if (h) t.home = h.value;
        if (k) t.key = k.value;
        box.querySelectorAll('.cfg-cam[data-p]').forEach(pb => {
          const p = t.people[+pb.getAttribute('data-p')]; if (!p) return;
          p.label = val(pb, 'label'); p.arriveBy = val(pb, 'arriveBy'); p.mode = val(pb, 'mode');
          if (p.mode === 'window') {                      // ao escolher a janela pela primeira vez, sugere a da manhã
            p.windowFrom = pb.querySelector('[data-k="windowFrom"]') ? val(pb, 'windowFrom') : (p.windowFrom || '06:00');
            p.windowTo = pb.querySelector('[data-k="windowTo"]') ? val(pb, 'windowTo') : (p.windowTo || '08:00');
            p.showUntil = pb.querySelector('[data-k="showUntil"]') ? val(pb, 'showUntil') : (p.showUntil || '09:00');
          }
          pb.querySelectorAll('.cfg-item[data-r]').forEach(it => {
            const r = p.routes[+it.getAttribute('data-r')]; if (!r) return;
            r.name = val(it, 'name'); r.after = val(it, 'after'); r.home = val(it, 'home');
            if (!r.home) { r.from = val(it, 'from'); r.to = val(it, 'to'); }
            r.days = []; it.querySelectorAll('[data-day]').forEach(d => { if (d.checked) r.days.push(+d.getAttribute('data-day')); });
          });
        });
      }
      if (ed.menu) sec('menu').forEach((it, i) => {
        const l = ed.menu[i]; if (!l) return;
        l.label = val(it, 'label'); l.sub = val(it, 'sub'); l.icon = val(it, 'icon'); l.port = parseInt(val(it, 'port'), 10) || 0; l.path = val(it, 'path');
      });
    }

    // ---------- hub -> estado ----------
    const fromCams = l => l.map(c => ({ id: c.id, name: c.name, masked: c.url, url: '' }));
    const fromCals = l => l.map(c => ({ who: c.who, label: c.label, color: c.color, masked: c.url, url: '', colorTouched: false }));
    function fromTraffic(t) {
      t.key = '';
      t.people.forEach(p => { p.mode = p.onDemand ? 'ondemand' : (p.windowFrom ? 'window' : 'always'); p.arriveBy = p.arriveBy || ''; p.windowFrom = p.windowFrom || ''; p.windowTo = p.windowTo || ''; p.showUntil = p.showUntil || ''; });
      t.people.forEach(p => p.routes.forEach(r => { r.name = r.name || ''; r.from = r.from || ''; r.to = r.to || ''; r.after = r.after || ''; }));
      return t;
    }
    function fromMenu(j) {
      ed.menuCustom = Array.isArray(j.links);
      const src = ed.menuCustom ? j.links : ((CD.state.cfg && CD.state.cfg.links) || []);
      ed.menu = src.map(l => ({ label: l.label || '', sub: l.sub || '', icon: l.icon || 'link', port: l.port || 0, path: l.path || '/', alert: l.alert || '' }));
    }

    async function load() {
      if (!CD.state.hub) return draw();
      draw();
      const A = CD.hubApi.settings;
      try { ed.cameras = fromCams(await A.get('cameras')); } catch (e) { say('cameras', e.message, true); ed.cameras = []; }
      try { ed.calendars = fromCals(await A.get('calendars')); } catch (e) { say('calendars', e.message, true); ed.calendars = []; }
      try { ed.traffic = fromTraffic(await A.get('traffic')); } catch (e) { say('traffic', e.message, true); }
      try { fromMenu(await A.get('menu')); } catch (e) { say('menu', e.message, true); }
      draw();
    }

    async function save(sec) {
      collect();
      const A = CD.hubApi.settings;
      let payload;
      if (sec === 'cameras') payload = ed.cameras.filter(c => c.id || c.name.trim() || c.url.trim()).map(c => ({ id: c.id || '', name: c.name, url: c.url }));
      else if (sec === 'calendars') payload = ed.calendars.filter(c => c.who || c.label.trim() || c.url.trim()).map(c => ({ who: c.who || '', label: c.label, color: c.color, url: c.url }));
      else if (sec === 'traffic') payload = { home: ed.traffic.home, key: ed.traffic.key, people: ed.traffic.people.map(p => ({
        id: p.id, label: p.label, arriveBy: p.arriveBy, routes: p.routes, onDemand: p.mode === 'ondemand',
        windowFrom: p.mode === 'window' ? p.windowFrom : '', windowTo: p.mode === 'window' ? p.windowTo : '', showUntil: p.mode === 'window' ? p.showUntil : '' })) };
      else payload = { links: ed.menu.map(l => ({ label: l.label, sub: l.sub, icon: l.icon, port: l.port, path: l.path, alert: l.alert })) };
      busy[sec] = true; say(sec, t('c.saving')); draw();
      try {
        const r = await A.save(sec, payload);
        if (sec === 'cameras') ed.cameras = fromCams(r);
        else if (sec === 'calendars') ed.calendars = fromCals(r);
        else if (sec === 'traffic') ed.traffic = fromTraffic(r);
        else fromMenu(r);
        say(sec, t('ce.saved'));
      } catch (e) { say(sec, e.message, true); }
      busy[sec] = false; draw();
    }

    async function resetMenu() {
      if (!confirm(t('ce.menu.resetAsk'))) return;
      busy.menu = true; say('menu', t('c.saving')); draw();
      try { fromMenu(await CD.hubApi.settings.save('menu', { links: null })); say('menu', t('ce.menu.resetDone')); } catch (e) { say('menu', e.message, true); }
      busy.menu = false; draw();
    }

    function act(a) {
      const p = a.split(':'), op = p[0], what = p[1];
      if (op === 'save') return save(what);
      if (op === 'reset') return resetMenu();
      collect();
      if (op === 'add') {
        say(what === 'route' || what === 'person' ? 'traffic' : what, '');
        if (what === 'cameras') ed.cameras.push({ id: '', name: '', masked: '', url: '' });
        else if (what === 'calendars') ed.calendars.push({ who: '', label: '', color: '#B48CFF', masked: '', url: '', colorTouched: true });
        else if (what === 'person') ed.traffic.people.push({ id: '', label: '', arriveBy: '', mode: 'always', onDemand: false, windowFrom: '', windowTo: '', showUntil: '', routes: [] });
        else if (what === 'route') ed.traffic.people[+p[2]].routes.push({ name: '', from: '', to: '', home: false, after: '', days: [1, 2, 3, 4, 5] });
        else ed.menu.push({ label: '', sub: '', icon: 'link', port: 0, path: '/', alert: '' });
        return draw();
      }
      if (op === 'up' || op === 'down') {
        const i = +p[2], j = op === 'up' ? i - 1 : i + 1;
        if (j >= 0 && j < ed.menu.length) { const t = ed.menu[i]; ed.menu[i] = ed.menu[j]; ed.menu[j] = t; }
        say('menu', t('ce.menu.moved'));
        return draw();
      }
      if (op === 'del') {
        let row, sec = what, fresh = false;
        if (what === 'cameras') { row = ed.cameras[+p[2]]; fresh = !row.id; }
        else if (what === 'calendars') { row = ed.calendars[+p[2]]; fresh = !row.who; }
        else if (what === 'person') { sec = 'traffic'; row = ed.traffic.people[+p[2]]; fresh = !row.id; row.name = row.label; }
        else if (what === 'route') { sec = 'traffic'; row = ed.traffic.people[+p[2]].routes[+p[3]]; }
        else row = ed.menu[+p[2]];
        const nome = (row.name || row.label || '').trim() || t('ce.thisItem');
        if (!(fresh && !(row.name || row.label || row.url || '').trim()) && !confirm(t('ce.removeAsk', { name: nome }))) return;
        if (what === 'cameras') ed.cameras.splice(+p[2], 1);
        else if (what === 'calendars') ed.calendars.splice(+p[2], 1);
        else if (what === 'person') ed.traffic.people.splice(+p[2], 1);
        else if (what === 'route') ed.traffic.people[+p[2]].routes.splice(+p[3], 1);
        else ed.menu.splice(+p[2], 1);
        draw();                                           // a tela volta a refletir a lista antes de salvar
        if (fresh) { say(sec, ''); return draw(); }      // linha nova, ainda não salva: some sem falar com o hub
        return save(sec);                                 // remover vale na hora (junto com o que mais estiver editado na seção)
      }
    }

    box.addEventListener('click', e => {
      const b = e.target.closest && e.target.closest('[data-a]');
      if (b && !b.disabled) act(b.getAttribute('data-a'));
    });
    box.addEventListener('change', e => {
      const t = e.target;
      if (t.id === 'lang-sel') { CD.i18n.setLang(t.value); return; }
      if (t.id === 'compat-sel') { CD.compat.setMode(t.value); const s = box.querySelector('#compat-st'); if (s) s.textContent = status(); return; }
      if (t.getAttribute && t.getAttribute('data-k') === 'color') t.setAttribute('data-touched', '1');
      if (t.hasAttribute && t.hasAttribute('data-redraw')) { collect(); draw(); }    // home office troca os campos de origem/destino
    });

    load();
  }

  CD.configEdit = { mount: mount };
})(window.CD = window.CD || {});
