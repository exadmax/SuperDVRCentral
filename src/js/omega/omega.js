(function (CD) {
  // Página Omega (omega.html): simulação de conversa com um assistente no estilo J.A.R.V.I.S. Só para PC/celular: TVs (Tizen,
  // webOS...) recebem um aviso, porque não têm microfone, WebGL confiável nem desempenho para a cena 3D.
  // Fluxo: voz ("Omega, ...") ou texto → POST /api/omega/ask (o hub entende o pedido: regra rápida ou IA) → {say, actions} →
  // a página mostra cada ação num painel 3D (câmera ao vivo, trânsito, agenda) e fala a resposta.
  const U = CD.util, esc = U.esc, t = CD.i18n.t;
  const HISTORY = 6;          // mensagens anteriores enviadas ao hub (contexto da conversa)
  const PRESENCE_IDLE = 90e3; // sem movimento na webcam por este tempo: o próximo movimento ganha um cumprimento
  const LEVEL_COLOR = { livre: '#33E19A', moderado: '#FFB020', congestionado: '#FF6B6B' };

  const cfg = U.deepMerge(JSON.parse(JSON.stringify(window.CD_SETTINGS || {})), window.CD_SETTINGS_LOCAL);
  CD.state = { cfg: cfg, hub: '' };

  let scene = null, voice = null, history = [], busy = false, muted = false, lastPresence = 0, selfStream = null, presTimer = 0;
  const $ = id => document.getElementById(id);

  // ---------- interface ----------
  function shell() {
    document.body.innerHTML =
      '<div class="om">' +
      '<canvas id="om-gl" class="om-gl"></canvas>' +
      '<header class="om-top"><a class="om-back" href="index.html" title="' + esc(t('pg.back')) + '">' + CD.icons.svg('back', 'currentColor', 22, 2) + '</a>' +
      '<div class="om-brand"><b>OMEGA</b><span>' + esc(t('om.sub')) + '</span></div>' +
      '<div class="om-chips" id="om-status"></div></header>' +
      '<div class="om-sub" id="om-sub"><div class="om-heard" id="om-heard"></div><div class="om-said" id="om-said"></div></div>' +
      '<aside class="om-log" id="om-log"></aside>' +
      '<div class="om-sug" id="om-sug"></div>' +
      '<form class="om-ask" id="om-ask" autocomplete="off">' +
      '<button type="button" class="om-btn" id="om-mic" title="' + esc(t('om.mic')) + '">' + micIcon() + '</button>' +
      '<input id="om-in" maxlength="500" placeholder="' + esc(t('om.placeholder')) + '" spellcheck="false">' +
      '<button type="submit" class="om-btn om-send" title="' + esc(t('om.send')) + '">➤</button>' +
      '<button type="button" class="om-btn" id="om-cam" title="' + esc(t('om.webcam')) + '">' + camIcon() + '</button>' +
      '<button type="button" class="om-btn" id="om-mute" title="' + esc(t('om.mute')) + '">🔊</button>' +
      '<button type="button" class="om-btn" id="om-clear" title="' + esc(t('om.clear')) + '">✕</button>' +
      '</form>' +
      '<video id="om-self" class="om-self" muted playsinline hidden></video>' +
      '</div>';
  }
  const micIcon = () => '<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3"/></svg>';
  const camIcon = () => '<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="6" width="13" height="12" rx="2"/><path d="M16 10l5-3v10l-5-3z"/></svg>';

  function notice(title, text) {
    document.body.innerHTML = '<div class="om om-notice"><div><h1>OMEGA</h1><h2>' + esc(title) + '</h2><p>' + esc(text) + '</p>' +
      '<a class="om-btn om-wide" href="index.html">' + esc(t('pg.back')) + '</a></div></div>';
  }

  function status(items) {
    $('om-status').innerHTML = items.map(i => '<span class="om-chip' + (i.on ? ' on' : '') + (i.bad ? ' bad' : '') + '">' + esc(i.text) + '</span>').join('');
  }
  const st = { mic: false, cam: false, ai: '', aiBad: false };
  function drawStatus() {
    status([{ text: st.ai || t('om.ai.checking'), on: !st.aiBad && !!st.ai, bad: st.aiBad },
      { text: st.mic ? t('om.mic.on') : t('om.mic.off'), on: st.mic }, { text: st.cam ? t('om.cam.on') : t('om.cam.off'), on: st.cam }]);
  }

  function heard(text, interim) { const el = $('om-heard'); el.textContent = text ? '“' + text + '”' : ''; el.classList.toggle('interim', !!interim); }
  function said(text) { $('om-said').textContent = text || ''; }

  function log(who, text) {
    const el = $('om-log');
    el.insertAdjacentHTML('afterbegin', '<div class="om-msg ' + who + '"><b>' + (who === 'me' ? esc(t('om.you')) : 'Omega') + '</b>' + esc(text) + '</div>');
    while (el.children.length > 12) el.removeChild(el.lastChild);
  }

  // ---------- pedido ao hub ----------
  async function ask(text) {
    text = String(text || '').trim();
    if (!text || busy) return;
    if (!CD.state.hub) { reply(t('om.nohub')); return; }
    busy = true;
    heard(text); said(''); log('me', text);
    scene.setMode('thinking');
    let res;
    try {
      const r = await fetch(CD.state.hub + '/api/omega/ask', { method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: text, history: history.slice(-HISTORY) }) });
      res = await r.json().catch(() => null);
      if (!r.ok) throw new Error(CD.i18n.srv(res && res.error) || t('err.http', { n: r.status }));
    } catch (e) {
      busy = false; scene.setMode('error');
      await reply(e.message || t('err.offline'));
      return;
    }
    history.push({ role: 'user', content: text });
    const extra = [];
    for (const a of res.actions || []) {
      try { const s = await run(a); if (s) extra.push(s); } catch (e) { extra.push(e.message); }
    }
    const full = [res.say].concat(extra).filter(Boolean).join(' ');
    history.push({ role: 'assistant', content: full });
    if (history.length > HISTORY * 2) history = history.slice(-HISTORY * 2);
    busy = false;
    await reply(full);
  }

  async function reply(text) {
    said(text); log('om', text);
    if (muted || !voice || !CD.omegaVoice.canSpeak()) { scene.setMode(voice && voice.active() ? 'listening' : 'idle'); return; }
    await voice.speak(text);
    scene.setMode(voice.active() ? 'listening' : 'idle');
  }

  // ---------- ações ----------
  function run(a) {
    if (a.type === 'camera') return showCamera(a);
    if (a.type === 'traffic') return showTraffic(a);
    if (a.type === 'agenda') return showAgenda(a);
    if (a.type === 'list') return showList(a);
    return null;
  }

  // Lista pronta vinda do hub (estoque, serviços): {title, rows: [{text, sub, warn}], more}. A frase falada já vem no `say`.
  function showList(a) {
    const l = a.list;
    if (!l || !l.rows) return '';
    const warn = l.rows.some(r => r.warn), H = 64, color = warn ? '#FF6B6B' : '#4DA3FF';
    const c = CD.omegaScene.canvasTexture(1280, 720, (x, w, h) => {
      x.fillStyle = 'rgba(6,18,31,0.92)'; x.fillRect(0, 0, w, h);
      x.fillStyle = '#8FB7E0'; x.font = '500 40px Inter, sans-serif'; x.fillText(trim(x, l.title, w - 100), 50, 70);
      l.rows.forEach((r, i) => {
        const y = 150 + i * (H + 8);
        x.fillStyle = r.warn ? '#FF6B6B' : '#4DA3FF'; x.fillRect(50, y - 40, 8, 52);
        x.font = '500 30px Inter, sans-serif';
        const sub = trim(x, r.sub || '', 560), sw = x.measureText(sub).width;
        x.fillStyle = r.warn ? '#FF9B9B' : '#8FB7E0'; x.fillText(sub, w - 50 - sw, y - 2);
        x.fillStyle = '#E8F1FB'; x.font = '500 42px Inter, sans-serif'; x.fillText(trim(x, r.text, w - 190 - sw), 80, y);
      });
      if (l.more > 0) { x.fillStyle = '#8FB7E0'; x.font = '500 34px Inter, sans-serif'; x.fillText(t('om.ag.more', { n: l.more }), 80, 150 + l.rows.length * (H + 8)); }
    });
    scene.add({ kind: 'list', key: 'ls:' + l.title, title: l.title, texture: c.texture, aspect: 16 / 9, color: color });
    return '';
  }

  function showCamera(a) {
    const title = t('om.cam.title', { n: a.name || a.id });
    if (!CD.cameraPlayer.supported()) return showSnapshot(a, title);
    const video = document.createElement('video');
    video.muted = true; video.playsInline = true; video.crossOrigin = 'anonymous';
    let panel = null;
    const pl = CD.cameraPlayer.create(video, CD.hubApi.cameras.streamUrl(a.id), (kind, msg) => {
      if (!panel) return;
      if (kind === 'err') panel.retitle(title + ' · ' + (msg || t('om.cam.err')));
      else if (kind === 'play') panel.retitle(title + ' · ' + t('om.live'));
    });
    panel = scene.add({ kind: 'camera', key: 'cam:' + a.id, title: title + ' · ' + t('om.connecting'), texture: CD.omegaScene.videoTexture(video), aspect: 16 / 9, color: '#33E19A',
      onDispose: () => { pl.stop(); } });
    pl.start();
    return '';
  }

  // sem Media Source (alguns navegadores): imagem parada da câmera, atualizada a cada 2 s
  function showSnapshot(a, title) {
    const c = CD.omegaScene.canvasTexture(1280, 720, (x, w, h) => { x.fillStyle = '#06121f'; x.fillRect(0, 0, w, h); });
    let timer = 0;
    const img = new Image();
    img.onload = () => c.redraw(x => x.drawImage(img, 0, 0, 1280, 720));
    const load = () => { img.src = CD.state.hub + '/api/cameras/' + encodeURIComponent(a.id) + '/snapshot.jpg?t=' + Date.now(); };
    load(); timer = setInterval(load, 2000);
    scene.add({ kind: 'camera', key: 'cam:' + a.id, title: title, texture: c.texture, aspect: 16 / 9, color: '#33E19A', onDispose: () => clearInterval(timer) });
    return '';
  }

  async function showTraffic(a) {
    let person = null, demo = false;
    try {
      const j = await U.getJson(CD.state.hub + '/api/traffic?ask=' + encodeURIComponent(a.person), 25000);
      if (j.configured) person = (j.people || []).find(p => p.id === a.person) || null;
      else demo = true;
    } catch (e) { demo = true; }
    if (demo) person = { id: a.person, label: a.label || a.person, result: CD.trafficApi.demo().p1 };
    const r = person && person.result;
    const label = (person && person.label) || a.label || a.person;
    const ok = r && typeof r.min === 'number' && r.min > 0;
    const color = ok ? LEVEL_COLOR[r.level] || '#4DA3FF' : '#FFB020';
    const leave = ok ? leaveAt(r.arriveBy, r.min) : '';
    const why = !ok ? (r && r.reason ? CD.i18n.srv(r.reason) : r && r.manual ? t('om.tr.manual') : t('om.tr.none')) : '';  // home office, fora do horário...
    const c = CD.omegaScene.canvasTexture(1280, 720, (x, w, h) => {
      x.fillStyle = 'rgba(6,18,31,0.92)'; x.fillRect(0, 0, w, h);
      x.fillStyle = '#8FB7E0'; x.font = '500 44px Inter, sans-serif'; x.fillText(r && r.name ? r.name : t('om.tr.work'), 60, 90);
      if (!ok) { x.fillStyle = '#FFB020'; x.font = '600 56px Inter, sans-serif'; x.fillText(why, 60, 360); return; }
      x.fillStyle = color; x.font = '700 200px Poppins, Inter, sans-serif'; x.fillText(String(r.min), 60, 330);
      const wmin = x.measureText(String(r.min)).width;
      x.font = '600 64px Inter, sans-serif'; x.fillText('min', 80 + wmin, 330);
      x.fillStyle = '#E8F1FB'; x.font = '500 46px Inter, sans-serif';
      x.fillText(t('om.tr.level.' + r.level) + (r.delay > 0 ? ' · +' + r.delay + ' min' : ''), 60, 410);
      if (leave) { x.fillStyle = '#8FB7E0'; x.fillText(t('om.tr.leave', { h: leave }), 60, 480); }
      const p = r.demo ? { d: r.mock } : (r.points ? CD.trafficApi.path(r.points) : null);
      if (p && p.d && window.Path2D) {     // trajeto (viewBox 400x84 do mini-mapa) esticado na parte de baixo
        x.save(); x.translate(40, 520); x.scale((w - 80) / 400, 170 / 84);
        x.strokeStyle = color; x.lineWidth = 2.2; x.lineCap = 'round'; x.shadowColor = color; x.shadowBlur = 6;
        x.stroke(new Path2D(p.d)); x.restore();
      }
      if (demo) { x.fillStyle = '#FFB020'; x.font = '500 34px Inter, sans-serif'; x.fillText(t('om.demo'), w - 260, 90); }
    });
    scene.add({ kind: 'traffic', key: 'tr:' + a.person, title: t('om.tr.title', { n: label }), texture: c.texture, aspect: 16 / 9, color: color });
    if (!ok) return t('om.tr.sayMsg', { n: label, msg: /[.!?。]$/.test(why) ? why : why + '.' });
    return t('om.tr.say', { n: label, m: r.min, l: t('om.tr.level.' + r.level) }) + (r.delay > 0 ? ' ' + t('om.tr.delay', { m: r.delay }) : '') +
      (leave ? ' ' + t('om.tr.leave', { h: leave }) + '.' : '') + (demo ? ' ' + t('om.demoSay') : '');
  }

  function leaveAt(arriveBy, minutes) {
    if (!arriveBy) return '';
    const p = arriveBy.split(':').map(Number), now = new Date();
    const arrive = new Date(now.getFullYear(), now.getMonth(), now.getDate(), p[0], p[1]);
    return arrive > now ? U.hhmm(new Date(arrive.getTime() - minutes * 60000)) : '';
  }

  async function showAgenda(a) {
    const day = new Date();
    if (a.day === 'tomorrow') day.setDate(day.getDate() + 1);
    let model;
    try { model = await CD.calendar.load(cfg, CD.state.hub); } catch (e) { model = { demo: true, sources: [] }; }
    const evs = CD.calendar.eventsOn(model, day);
    const when = a.day === 'tomorrow' ? t('om.ag.tomorrow') : t('om.ag.today');
    const now = Date.now(), H = 64, rows = Math.min(evs.length, 8);
    const c = CD.omegaScene.canvasTexture(1280, 720, (x, w, h) => {
      x.fillStyle = 'rgba(6,18,31,0.92)'; x.fillRect(0, 0, w, h);
      x.fillStyle = '#8FB7E0'; x.font = '500 40px Inter, sans-serif';
      x.fillText(CD.i18n.longDate ? CD.i18n.longDate(day) : day.toLocaleDateString(), 50, 70);
      if (model.demo) { x.fillStyle = '#FFB020'; x.font = '500 32px Inter, sans-serif'; x.fillText(t('om.demo'), w - 240, 70); }
      if (!evs.length) { x.fillStyle = '#E8F1FB'; x.font = '600 56px Inter, sans-serif'; x.fillText(t('om.ag.none'), 50, 360); return; }
      evs.slice(0, rows).forEach((e, i) => {
        const y = 150 + i * (H + 8), past = !e.allDay && e.end && e.end.getTime() < now && a.day !== 'tomorrow';
        const col = /^#/.test(e.cal && e.cal.color) ? e.cal.color : '#4DA3FF';
        x.globalAlpha = past ? 0.45 : 1;
        x.fillStyle = col; x.fillRect(50, y - 40, 8, 52);
        x.fillStyle = '#8FB7E0'; x.font = '600 40px Inter, sans-serif'; x.fillText(e.allDay ? t('om.ag.allday') : U.hhmm(e.start), 80, y);
        x.fillStyle = '#E8F1FB'; x.font = '500 42px Inter, sans-serif'; x.fillText(trim(x, e.title, w - 330), 260, y);
        x.globalAlpha = 1;
      });
      if (evs.length > rows) { x.fillStyle = '#8FB7E0'; x.font = '500 34px Inter, sans-serif'; x.fillText(t('om.ag.more', { n: evs.length - rows }), 80, 150 + rows * (H + 8)); }
    });
    scene.add({ kind: 'agenda', key: 'ag:' + a.day, title: t('om.ag.title', { d: when }), texture: c.texture, aspect: 16 / 9, color: '#B48CFF' });
    if (!evs.length) return t('om.ag.none');
    const items = evs.slice(0, 5).map(e => (e.allDay ? '' : t('om.ag.at', { h: U.hhmm(e.start) }) + ', ') + e.title);
    return CD.i18n.n('om.ag.count', evs.length, { n: evs.length, d: when }) + ': ' + items.join('; ') + (evs.length > 5 ? '; ' + t('om.ag.more', { n: evs.length - 5 }) : '') + '.' +
      (model.demo ? ' ' + t('om.demoSay') : '');
  }

  function trim(x, s, max) {
    s = String(s || '');
    if (x.measureText(s).width <= max) return s;
    while (s.length > 1 && x.measureText(s + '…').width > max) s = s.slice(0, -1);
    return s + '…';
  }

  // ---------- microfone ----------
  function toggleMic() {
    if (!CD.omegaVoice.canListen()) { reply(t('om.mic.unsupported')); return; }
    if (voice.active()) { voice.stop(); st.mic = false; scene.setMode('idle'); }
    else if (voice.start()) { st.mic = true; scene.setMode('listening'); said(t('om.mic.hint')); }
    $('om-mic').classList.toggle('on', voice.active());
    drawStatus();
  }

  // ---------- webcam (opcional): prévia no canto e presença ----------
  async function toggleCam() {
    const v = $('om-self');
    if (selfStream) {
      selfStream.getTracks().forEach(x => x.stop()); selfStream = null; clearInterval(presTimer);
      v.hidden = true; st.cam = false; $('om-cam').classList.remove('on'); drawStatus(); return;
    }
    try {
      selfStream = await navigator.mediaDevices.getUserMedia({ video: { width: 320, height: 240, facingMode: 'user' } });
    } catch (e) { reply(t('om.cam.denied')); return; }
    v.srcObject = selfStream; v.hidden = false; v.play();
    st.cam = true; $('om-cam').classList.add('on'); drawStatus();
    lastPresence = Date.now();
    const cv = document.createElement('canvas'); cv.width = 32; cv.height = 24;
    const x = cv.getContext('2d', { willReadFrequently: true });
    let prev = null;
    presTimer = setInterval(() => {
      if (v.readyState < 2) return;
      x.drawImage(v, 0, 0, 32, 24);
      const d = x.getImageData(0, 0, 32, 24).data;
      if (prev) {
        let diff = 0;
        for (let i = 0; i < d.length; i += 4) diff += Math.abs(d[i] - prev[i]) + Math.abs(d[i + 1] - prev[i + 1]);
        const moving = diff / (32 * 24) > 18;
        if (moving) {
          if (Date.now() - lastPresence > PRESENCE_IDLE && !busy && !(voice && voice.speaking())) greet();
          lastPresence = Date.now();
        }
        if (scene) scene.setLevel(moving ? 0.4 : (voice ? voice.level() : 0));
      }
      prev = d;
    }, 700);
  }

  function greet() {
    const h = new Date().getHours();
    reply(t(h < 12 ? 'om.greet.morning' : h < 18 ? 'om.greet.afternoon' : 'om.greet.evening'));
  }

  // ---------- sugestões ----------
  async function suggestions() {
    const sug = [];
    try {
      const cams = await CD.hubApi.cameras.list();
      const c = (cams || []).find(Boolean);
      if (c) sug.push(t('om.sug.cam', { n: c.name || c.id }));
    } catch (e) { /* sem câmeras */ }
    try {
      const j = await U.getJson(CD.state.hub + '/api/settings/traffic', 8000);
      const p = (j.people || [])[0];
      if (p) sug.push(t('om.sug.traffic', { n: p.label }));
    } catch (e) { /* sem trânsito */ }
    sug.push(t('om.sug.agenda'), t('om.sug.stock'), t('om.sug.who'));
    $('om-sug').innerHTML = sug.map(s => '<button type="button" class="om-chip on" data-say="' + esc(s) + '">' + esc(s) + '</button>').join('');
  }

  async function aiStatus() {
    try {
      const s = await CD.hubApi.ai.status();
      if (s.provider === 'groq') { st.ai = 'Groq · ' + ((s.groq && s.groq.model) || ''); st.aiBad = !(s.groq && s.groq.configured); }
      else if (s.provider === 'ollama') { st.ai = 'Ollama' + (s.ollama && s.ollama.model ? ' · ' + s.ollama.model : ''); st.aiBad = !(s.ollama && s.ollama.reachable); }
      else { st.ai = t('om.ai.off'); st.aiBad = true; }
    } catch (e) { st.ai = t('om.ai.unknown'); st.aiBad = true; }
    drawStatus();
  }

  // ---------- início ----------
  async function boot() {
    if (CD.compat && CD.compat.detected()) { notice(t('om.tv.title'), t('om.tv.text')); return; }
    if (!window.THREE || !CD.omegaScene.supported()) { notice(t('om.gl.title'), t('om.gl.text')); return; }
    document.title = 'Omega · DVR';
    shell();
    scene = CD.omegaScene.create($('om-gl'));
    voice = CD.omegaVoice.create({ lang: CD.i18n.locale(), onWake: () => { scene.setMode('listening'); said(t('om.wake')); if (!muted) voice.speak(t('om.wakeReply')); },
      onInterim: s => heard(s, true), onCommand: s => ask(s), onState: () => {}, onSpeak: on => scene.setMode(on ? 'speaking' : (voice && voice.active() ? 'listening' : 'idle')),
      onError: e => { if (e === 'mic-denied') { st.mic = false; drawStatus(); reply(t('om.mic.denied')); } } });
    setInterval(() => { if (voice.active() && !selfStream) scene.setLevel(voice.level()); }, 80);
    drawStatus();

    $('om-ask').addEventListener('submit', e => { e.preventDefault(); const i = $('om-in'); const v = i.value; i.value = ''; ask(v); });
    $('om-mic').addEventListener('click', toggleMic);
    $('om-cam').addEventListener('click', toggleCam);
    $('om-mute').addEventListener('click', () => { muted = !muted; $('om-mute').textContent = muted ? '🔇' : '🔊'; if (muted) voice.hush(); });
    $('om-clear').addEventListener('click', () => { scene.clear(); said(''); heard(''); });
    $('om-sug').addEventListener('click', e => { const b = e.target.closest && e.target.closest('[data-say]'); if (b) ask(b.getAttribute('data-say')); });
    document.addEventListener('keydown', e => { if (e.key === 'Escape') scene.clear(); });

    const base = /^https?:/.test(location.protocol) ? location.origin : '';
    if (base) { try { await U.getJson(base + '/api/health', 3000); CD.state.hub = base; } catch (e) { CD.state.hub = ''; } }
    if (!CD.state.hub) { st.ai = t('om.nohub'); st.aiBad = true; drawStatus(); said(t('om.nohub')); return; }
    try { await CD.hubApi.settings.loadPanel(cfg); } catch (e) { /* hub antigo: vale o padrão */ }
    aiStatus(); suggestions();
    said(t('om.hello'));
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot); else boot();
})(window.CD = window.CD || {});
