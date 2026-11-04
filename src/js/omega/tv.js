(function (CD) {
  // Omega na TV (omega-tv.html). Sem three.js: um GIF no centro, uma caixa lateral estilo menu de Mega Drive e o microfone como
  // única entrada (a TV não tem teclado). A caixa tem duas telas: o MENU com as perguntas mais comuns e a MENSAGEM, que mostra a
  // resposta uma página por vez, com texto surgindo letra a letra. Controle remoto: ▲▼ escolhem, OK pergunta/avança, ◀▶ trocam de
  // página, Voltar fecha (menu → caixa fechada → página inicial). Pedido ao hub: o mesmo POST /api/omega/ask da página do PC.
  const U = CD.util, esc = U.esc, t = CD.i18n.t;
  const HISTORY = 6;        // mensagens anteriores enviadas ao hub (contexto da conversa)
  const TYPE_MS = 26;       // intervalo entre letras do texto
  const IDLE_MS = 60e3;     // sem tocar no controle por este tempo: a caixa fecha e sobra só o GIF
  const LISTEN_MS = 15e3;   // microfone aberto sem ninguém falar: desliga
  const CHUNK = 120;        // tamanho máximo (em letras) de uma página de texto
  const GIF = 'assets/img/omega.gif', GIF_FALLBACK = 'assets/img/omega.webp';

  const cfg = U.deepMerge(JSON.parse(JSON.stringify(window.CD_SETTINGS || {})), window.CD_SETTINGS_LOCAL);
  CD.state = { cfg: cfg, hub: '' };

  const $ = id => document.getElementById(id);
  let voice = null, history = [], busy = false;
  let view = 'menu';                     // 'menu' | 'msg' | 'closed'
  let items = [], cursor = 0;            // itens do menu (o 0 é o microfone) e o selecionado
  let pages = [], page = 0;              // respostas: [{text, warn}]
  let typeTimer = 0, typeFull = '', typeAt = 0, idleTimer = 0, listenTimer = 0, camTimer = 0;

  // ---------- interface ----------
  const micIcon = () => '<svg viewBox="0 0 24 24" width="100%" height="100%" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3"/></svg>';

  function shell() {
    $('tv').innerHTML =
      '<div class="tv-brand"><b>OMEGA</b><span>' + esc(t('om.sub')) + '</span></div>' +
      '<div class="tv-stage" id="tv-stage" data-mode="idle">' +
      '<img class="tv-gif" id="tv-gif" alt="Omega" src="' + GIF + '">' +
      '<img class="tv-cam" id="tv-cam" alt="" hidden>' +
      '<div class="tv-heard" id="tv-heard"></div>' +
      '<button type="button" class="tv-mic" id="tv-mic" title="' + esc(t('om.mic')) + '">' + micIcon() + '</button>' +
      '</div>' +
      '<div class="tv-tab" id="tv-tab">&#9664; ' + esc(t('om.tv.tab')) + '</div>' +
      '<aside class="tv-box" id="tv-box"></aside>';
    const g = $('tv-gif');
    g.onerror = () => { g.onerror = null; g.src = GIF_FALLBACK; };    // enquanto não houver omega.gif, usa a imagem parada
  }

  function mode(m) { $('tv-stage').setAttribute('data-mode', m); }
  function heard(text, interim) { const el = $('tv-heard'); el.textContent = text ? '“' + text + '”' : ''; el.className = 'tv-heard' + (interim ? ' interim' : ''); }
  const levelText = l => t('om.tr.level.' + l);

  // ---------- caixa lateral ----------
  function setView(v) {
    view = v;
    $('tv').className = 'tv' + (v === 'closed' ? '' : ' open');
    if (v === 'menu') drawMenu(); else if (v === 'msg') drawMsg();
    poke();
  }

  function poke() {
    clearTimeout(idleTimer);
    idleTimer = setTimeout(() => { if (!busy && !(voice && voice.active())) { stopType(); setView('closed'); } }, IDLE_MS);
  }

  function drawMenu() {
    stopType();
    const listening = voice && voice.active();
    $('tv-box').innerHTML = '<div class="tv-h"><span>' + esc(t('om.tv.menu')) + '</span></div><div class="tv-list">' +
      items.map((it, i) => '<div class="tv-it' + (i === cursor ? ' sel' : '') + (it.mic ? ' mic' : '') + '" data-i="' + i + '">' +
        (it.mic ? (listening ? esc(t('om.tv.listening')) : esc(t('om.tv.speak'))) : esc(it.label)) + '</div>').join('') +
      '</div><div class="tv-f">' + esc(t('om.tv.hint.menu')) + '</div>';
    const sel = $('tv-box').querySelector('.sel');
    if (sel && sel.scrollIntoView) sel.scrollIntoView({ block: 'nearest' });
  }

  function drawMsg() {
    const p = pages[page] || { text: '' };
    $('tv-box').innerHTML = '<div class="tv-h"><span>OMEGA</span><span>' + (pages.length > 1 ? (page + 1) + '/' + pages.length : '') + '</span></div>' +
      '<div class="tv-msg' + (p.warn ? ' warn' : '') + '" id="tv-msg"></div><div class="tv-f">' + esc(t(pages.length > 1 ? 'om.tv.hint.msg' : 'om.tv.hint.single')) + '</div>';
    typeText(p.text);
  }

  // texto surgindo letra a letra; chamar de novo (ou OK) completa de uma vez
  function typeText(text) {
    stopType();
    typeFull = String(text || ''); typeAt = 0;
    const el = $('tv-msg');
    if (!el) return;
    el.classList.add('typing');
    typeTimer = setInterval(() => {
      typeAt += 2;
      if (typeAt >= typeFull.length) { stopType(); el.textContent = typeFull; return; }
      el.textContent = typeFull.slice(0, typeAt);
    }, TYPE_MS);
  }
  function stopType() {
    clearInterval(typeTimer); typeTimer = 0;
    const el = $('tv-msg');
    if (el) { el.classList.remove('typing'); if (typeFull) el.textContent = typeFull; }
  }
  const typing = () => !!typeTimer;

  function show(list) {            // list: [{text, warn}]; mostra a primeira página
    pages = list.length ? list : [{ text: '' }]; page = 0;
    setView('msg');
  }
  function turn(d) {
    const n = page + d;
    if (n < 0 || n >= pages.length) return false;
    page = n; drawMsg(); poke();
    return true;
  }

  // ---------- controle remoto ----------
  function onKey(e) {
    const k = e.keyCode;
    const back = k === 27 || k === 10009 || k === 461 || k === 8;   // Esc, Voltar Samsung, Voltar LG, Backspace
    if (k === 403 || k === 77) { e.preventDefault(); toggleMic(); return; }  // botão vermelho do controle, ou "m"
    if (view === 'closed') {
      if (back) { e.preventDefault(); if (camShown()) hideCam(); else location.href = 'index.html'; }
      else if (k === 13 || (k >= 37 && k <= 40)) { e.preventDefault(); setView(k === 39 && pages.length ? 'msg' : 'menu'); }
      return;
    }
    poke();
    if (view === 'menu') {
      if (k === 38) move(-1); else if (k === 40) move(1); else if (k === 13) choose();
      else if (k === 39 && pages.length) setView('msg');
      else if (back) setView('closed');
      else return;
      e.preventDefault();
      return;
    }
    // tela de mensagem
    if (k === 37) turn(-1);
    else if (k === 39) { if (typing()) stopType(); else turn(1); }
    else if (k === 13) { if (typing()) stopType(); else if (!turn(1)) setView('menu'); }
    else if (back) setView('menu');
    else return;
    e.preventDefault();
  }

  function move(d) {
    cursor = (cursor + d + items.length) % items.length;
    drawMenu();
  }
  function choose() {
    const it = items[cursor];
    if (!it) return;
    if (it.mic) toggleMic(); else ask(it.say);
  }

  // ---------- pedido ao hub ----------
  async function ask(text) {
    text = String(text || '').trim();
    if (!text || busy) return;
    if (!CD.state.hub) { show([{ text: t('om.nohub'), warn: true }]); return; }
    busy = true; hideCam();
    heard(text); mode('thinking');
    show([{ text: t('om.tv.thinking') }]);
    let res;
    try {
      const r = await fetch(CD.state.hub + '/api/omega/ask', { method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: text, history: history.slice(-HISTORY) }) });
      res = await r.json().catch(() => null);
      if (!r.ok) throw new Error(CD.i18n.srv(res && res.error) || t('err.http', { n: r.status }));
    } catch (e) {
      busy = false; mode('error');
      show([{ text: e.message || t('err.offline'), warn: true }]);
      return;
    }
    history.push({ role: 'user', content: text });
    const list = chunks(res.say), spoken = [res.say];
    for (const a of res.actions || []) {
      try { const r = await run(a); r.pages.forEach(p => list.push(p)); if (r.say) spoken.push(r.say); }
      catch (e) { list.push({ text: e.message, warn: true }); }
    }
    const full = spoken.filter(Boolean).join(' ');
    history.push({ role: 'assistant', content: full });
    if (history.length > HISTORY * 2) history = history.slice(-HISTORY * 2);
    busy = false; mode('idle');
    show(list);
    if (full && voice && CD.omegaVoice.canSpeak()) { mode('speaking'); await voice.speak(full); mode('idle'); }
  }

  // quebra um texto longo em páginas curtas, nas pontuações (a TV é lida de longe: pouco texto por vez)
  function chunks(text) {
    const out = [], parts = String(text || '').match(/[^.!?。;]+[.!?。;]*\s*/g) || [];
    let cur = '';
    parts.forEach(s => {
      s = s.trim();
      if (!s) return;
      if (cur && (cur + ' ' + s).length > CHUNK) { out.push({ text: cur }); cur = s; } else cur = cur ? cur + ' ' + s : s;
    });
    if (cur) out.push({ text: cur });
    return out;
  }

  // ---------- ações (cada uma vira páginas de texto; a câmera troca o GIF por uma imagem) ----------
  function run(a) {
    if (a.type === 'camera') return showCamera(a);
    if (a.type === 'traffic') return traffic(a);
    if (a.type === 'agenda') return agenda(a);
    if (a.type === 'list') return Promise.resolve(list(a));
    return Promise.resolve({ pages: [] });
  }

  // lista pronta do hub (estoque, serviços): título e uma página por linha; a frase falada já vem no `say`
  function list(a) {
    const l = a.list;
    if (!l || !l.rows) return { pages: [] };
    const out = [{ text: l.title, warn: l.rows.some(r => r.warn) }];
    l.rows.forEach(r => out.push({ text: r.text + (r.sub ? ' · ' + r.sub : ''), warn: !!r.warn }));
    if (l.more > 0) out.push({ text: t('om.ag.more', { n: l.more }) });
    return { pages: out };
  }

  // sem vídeo: o navegador da TV tem um só decodificador. A imagem da câmera é atualizada a cada 2 s
  // Só troca o GIF depois que a primeira imagem chega; se a câmera não responde, o GIF fica e a página avisa "sem imagem".
  function showCamera(a) {
    const title = t('om.cam.title', { n: a.name || a.id }), img = $('tv-cam');
    const url = () => CD.state.hub + '/api/cameras/' + encodeURIComponent(a.id) + '/snapshot.jpg?t=' + Date.now();
    hideCam();
    return new Promise(resolve => {
      let done = false;
      const finish = ok => {
        if (done) return;
        done = true;
        if (ok) { img.hidden = false; $('tv-gif').hidden = true; camTimer = setInterval(() => { const p = new Image(); p.onload = () => { img.src = p.src; }; p.src = url(); }, 2000); }
        resolve({ pages: [{ text: ok ? title : title + ' · ' + t('om.cam.err'), warn: !ok }], say: '' });
      };
      const probe = new Image();
      probe.onload = () => { img.src = probe.src; finish(true); };
      probe.onerror = () => finish(false);
      setTimeout(() => finish(false), 10000);
      probe.src = url();
    });
  }
  const camShown = () => !$('tv-cam').hidden;
  function hideCam() {
    clearInterval(camTimer); camTimer = 0;
    const img = $('tv-cam');
    if (img.hidden) return;
    img.hidden = true; img.removeAttribute('src'); $('tv-gif').hidden = false;
  }

  async function traffic(a) {
    let person = null, demo = false;
    try {
      const j = await U.getJson(CD.state.hub + '/api/traffic?ask=' + encodeURIComponent(a.person), 25000);
      if (j.configured) person = (j.people || []).find(p => p.id === a.person) || null;
      else demo = true;
    } catch (e) { demo = true; }
    if (demo) person = { id: a.person, label: a.label || a.person, result: CD.trafficApi.demo().p1 };
    const r = person && person.result, label = (person && person.label) || a.label || a.person;
    const ok = r && typeof r.min === 'number' && r.min > 0;
    if (!ok) {
      const why = r && r.reason ? CD.i18n.srv(r.reason) : r && r.manual ? t('om.tr.manual') : t('om.tr.none');    // home office, fora do horário...
      const msg = t('om.tr.sayMsg', { n: label, msg: /[.!?。]$/.test(why) ? why : why + '.' });
      return { pages: [{ text: msg, warn: true }], say: msg };
    }
    const leave = leaveAt(r.arriveBy, r.min), out = [], say = [];
    out.push({ text: t('om.tr.say', { n: label, m: r.min, l: levelText(r.level) }), warn: r.level === 'congestionado' });
    if (r.delay > 0) out.push({ text: t('om.tr.delay', { m: r.delay }), warn: true });
    if (leave) out.push({ text: t('om.tr.leave', { h: leave }) + '.' });
    if (demo) out.push({ text: t('om.demoSay') });
    out.forEach(p => say.push(p.text));
    return { pages: out, say: say.join(' ') };
  }

  function leaveAt(arriveBy, minutes) {
    if (!arriveBy) return '';
    const p = arriveBy.split(':').map(Number), now = new Date();
    const arrive = new Date(now.getFullYear(), now.getMonth(), now.getDate(), p[0], p[1]);
    return arrive > now ? U.hhmm(new Date(arrive.getTime() - minutes * 60000)) : '';
  }

  async function agenda(a) {
    const day = new Date();
    if (a.day === 'tomorrow') day.setDate(day.getDate() + 1);
    let model;
    try { model = await CD.calendar.load(cfg, CD.state.hub); } catch (e) { model = { demo: true, sources: [] }; }
    const evs = CD.calendar.eventsOn(model, day), when = a.day === 'tomorrow' ? t('om.ag.tomorrow') : t('om.ag.today'), rows = 8;
    if (!evs.length) { const m = t('om.ag.none'); return { pages: [{ text: m }], say: m }; }
    const out = [{ text: CD.i18n.n('om.ag.count', evs.length, { n: evs.length, d: when }) + '.' }];
    evs.slice(0, rows).forEach(e => out.push({ text: (e.allDay ? t('om.ag.allday') : U.hhmm(e.start)) + ' · ' + e.title }));
    if (evs.length > rows) out.push({ text: t('om.ag.more', { n: evs.length - rows }) });
    if (model.demo) out.push({ text: t('om.demoSay') });
    const items5 = evs.slice(0, 5).map(e => (e.allDay ? '' : t('om.ag.at', { h: U.hhmm(e.start) }) + ', ') + e.title);
    return { pages: out, say: CD.i18n.n('om.ag.count', evs.length, { n: evs.length, d: when }) + ': ' + items5.join('; ') + (evs.length > 5 ? '; ' + t('om.ag.more', { n: evs.length - 5 }) : '') + '.' +
      (model.demo ? ' ' + t('om.demoSay') : '') };
  }

  // ---------- microfone (aperta, fala um pedido, desliga) ----------
  function toggleMic() {
    if (!voice || !CD.omegaVoice.canListen()) { show([{ text: t('om.tv.mic.unsupported'), warn: true }]); return; }
    if (voice.active()) { stopListening(); return; }
    if (voice.start()) {
      voice.hush(); heard(''); mode('listening');
      $('tv-mic').classList.add('on');
      clearTimeout(listenTimer); listenTimer = setTimeout(stopListening, LISTEN_MS);
      if (view === 'menu') drawMenu();
    }
  }
  function stopListening() {
    clearTimeout(listenTimer);
    if (voice && voice.active()) voice.stop();
    $('tv-mic').classList.remove('on');
    if (!busy) mode('idle');
    if (view === 'menu') drawMenu();
  }

  // ---------- perguntas mais comuns ----------
  const plain = s => { s = s.replace(/^(ok\s+)?[oôó]mega\s*[,，:：]?\s*/i, ''); return s.charAt(0).toUpperCase() + s.slice(1); };   // tira "Omega, " do rótulo

  async function buildItems() {
    const q = [];
    try {
      const cams = await CD.hubApi.cameras.list();
      const c = (cams || []).find(Boolean);
      if (c) q.push(t('om.sug.cam', { n: c.name || c.id }));
    } catch (e) { /* sem câmeras */ }
    try {
      const j = await U.getJson(CD.state.hub + '/api/settings/traffic', 8000);
      (j.people || []).slice(0, 2).forEach(p => q.push(t('om.sug.traffic', { n: p.label })));
    } catch (e) { /* sem trânsito */ }
    q.push(t('om.sug.agenda'), t('om.sug.tomorrow'), t('om.sug.stock'), t('om.sug.fridge'), t('om.sug.who'));
    return q;
  }
  function setItems(q) {
    items = [{ mic: true }].concat(q.map(s => ({ say: s, label: plain(s) })));
    cursor = Math.min(cursor, items.length - 1);
    if (view === 'menu') drawMenu();
  }

  // ---------- início ----------
  async function boot() {
    document.title = 'Omega · DVR';
    shell();
    setItems([t('om.sug.agenda'), t('om.sug.tomorrow'), t('om.sug.stock'), t('om.sug.fridge'), t('om.sug.who')]);   // já mostra algo enquanto o hub responde
    cursor = 1;
    setView('menu');

    voice = CD.omegaVoice.create({ lang: CD.i18n.locale(), noWake: true,
      onWake: () => {}, onInterim: s => heard(s, true), onState: () => {},
      onCommand: s => { stopListening(); ask(s); },
      onSpeak: on => mode(on ? 'speaking' : 'idle'),
      onError: e => { stopListening(); show([{ text: t(e === 'mic-denied' ? 'om.mic.denied' : 'om.tv.mic.error'), warn: true }]); } });

    document.addEventListener('keydown', onKey);
    $('tv-box').addEventListener('click', e => {
      const it = e.target.closest && e.target.closest('.tv-it');
      if (it) { cursor = Number(it.getAttribute('data-i')) || 0; choose(); return; }
      if (view === 'msg' && e.target.closest && e.target.closest('#tv-msg')) { if (typing()) stopType(); else if (!turn(1)) setView('menu'); }
    });
    $('tv-tab').addEventListener('click', () => setView('menu'));
    $('tv-mic').addEventListener('click', toggleMic);
    $('tv-stage').addEventListener('click', e => { if (view !== 'closed' && !(e.target.closest && e.target.closest('#tv-mic'))) { stopType(); setView('closed'); } });

    const base = /^https?:/.test(location.protocol) ? location.origin : '';
    if (base) { try { await U.getJson(base + '/api/health', 3000); CD.state.hub = base; } catch (e) { CD.state.hub = ''; } }
    if (!CD.state.hub) { show([{ text: t('om.nohub'), warn: true }]); return; }
    try { await CD.hubApi.settings.loadPanel(cfg); } catch (e) { /* hub antigo: vale o padrão */ }
    setItems(await buildItems());
    try { voice.setVoice((await CD.hubApi.ai.settings()).voice || ''); } catch (e) { /* hub antigo: voz automática */ }
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot); else boot();
})(window.CD = window.CD || {});
