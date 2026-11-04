(function (CD) {
  // Página "Câmeras" (camaras.html): imagem ao vivo das câmeras de casa (o player fica em services/cameraPlayer.js) e a aba
  // Gravações, com os arquivos de 1 hora que o hub grava (liga/desliga e retenção: página Config). Só conecta enquanto visível.
  const U = CD.util, I = CD.icons, esc = U.esc, t = CD.i18n.t;

  function size(n) { return n >= 1073741824 ? CD.i18n.num(n / 1073741824, 1) + ' GB' : Math.max(1, Math.round(n / 1048576)) + ' MB'; }
  const two = n => (n < 10 ? '0' : '') + n;
  const hhmm = d => two(d.getHours()) + ':' + two(d.getMinutes());

  // Player embutido de uma gravação (mesmo molde da página Vídeos).
  function player(f) {
    const ov = document.createElement('div'), d = new Date(f.start * 1000);
    ov.className = 'vp';
    ov.innerHTML = '<div class="vp-top"><div class="vp-name">' + esc(f.camera + ' · ' + CD.i18n.dateNum(d) + ' ' + hhmm(d)) + '</div>' +
      '<a class="vbtn" href="' + esc(CD.hubApi.recordings.fileUrl(f.name, true)) + '">' + t('cam.rec.download') + '</a><button type="button" class="vbtn" data-a="x">' + t('c.close') + '</button></div>' +
      '<video controls autoplay playsinline></video><div class="vp-msg">' + t('cam.rec.hint') + '</div>';
    const video = ov.querySelector('video'), msg = ov.querySelector('.vp-msg');
    video.src = CD.hubApi.recordings.fileUrl(f.name);
    video.addEventListener('error', () => { msg.textContent = t('cam.rec.playErr'); msg.className = 'vp-msg err'; });
    function close() {
      document.removeEventListener('keydown', onKey);
      video.pause(); video.removeAttribute('src'); video.load();
      ov.remove();
    }
    function onKey(e) { if (e.key === 'Escape' || e.keyCode === 10009) close(); }
    document.addEventListener('keydown', onKey);
    ov.querySelector('[data-a="x"]').addEventListener('click', close);
    document.body.appendChild(ov);
    video.play().catch(() => { /* autoplay barrado: o usuário aperta play */ });
  }

  function mount(root) {
    root.innerHTML = '<section class="pg"><div class="cams-bar"><button type="button" class="vbtn on" data-v="live">' + t('cam.tab.live') + '</button>' +
      '<button type="button" class="vbtn" data-v="recs">' + t('cam.tab.recs') + '</button></div>' +
      '<div class="cams" id="cams"></div><div class="recs" id="recs" hidden></div></section>';
    const box = root.querySelector('#cams'), recsBox = root.querySelector('#recs'), cards = [];
    let view = 'live', recCam = '', selMode = false, picked = {}, sel = 0;

    function note(msg) { box.innerHTML = '<div class="empty">' + esc(msg) + '</div>'; }

    function card(c) {
      const el = document.createElement('article');
      el.className = 'cam';
      el.innerHTML = '<header class="cam-h"><h2>' + esc(c.name) + '</h2>' +
        '<button type="button" class="ib" title="' + esc(t('cam.fullscreen')) + '" aria-label="' + esc(t('cam.fullscreen') + ': ' + c.name) + '">' + I.svg('expand', 'currentColor', 24, 2) + '</button></header>' +
        '<div class="cam-v"><video muted autoplay playsinline></video><div class="cam-s">' + t('cam.connecting') + '</div></div>';
      const video = el.querySelector('video'), status = el.querySelector('.cam-s');
      const p = CD.cameraPlayer.create(video, CD.hubApi.cameras.streamUrl(c.id), (kind, text) => {
        status.textContent = text;
        status.className = 'cam-s' + (kind === 'play' ? ' off' : kind === 'err' ? ' err' : '');
      });
      el.querySelector('.ib').addEventListener('click', () => {
        const d = document;
        if (d.fullscreenElement || d.webkitFullscreenElement) { (d.exitFullscreen || d.webkitExitFullscreen).call(d); return; }
        const f = el.requestFullscreen || el.webkitRequestFullscreen;
        if (f) f.call(el);
      });
      cards.push({ el: el, p: p, on: false });
      return el;
    }

    // Modo compatibilidade (TV): uma câmera por vez, porque o navegador não decodifica vários vídeos ao mesmo tempo.
    const compat = () => !!(CD.compat && CD.compat.active()) && cards.length > 1;

    // Liga só o que deve estar no ar: tudo no modo normal, ou apenas a câmera escolhida no modo compatibilidade (as outras fecham antes).
    function sync() {
      const visible = !document.hidden && view === 'live', one = compat();
      const want = cards.map((k, i) => visible && (!one || i === sel));
      cards.forEach((k, i) => { k.el.hidden = one && i !== sel; if (!want[i] && k.on) { k.p.stop(); k.on = false; } });
      cards.forEach((k, i) => { if (want[i] && !k.on) { k.p.start(); k.on = true; } });
    }

    function picker(list) {
      const bar = document.createElement('div');
      bar.className = 'recs-f cams-pick';
      bar.innerHTML = list.map((c, i) => '<button type="button" class="vbtn' + (i === sel ? ' on' : '') + '" data-pick="' + i + '">' + esc(c.name) + '</button>').join('') +
        '<span class="cfg-md">' + t('cam.compatNote') + '</span>';
      bar.addEventListener('click', e => {
        const b = e.target.closest && e.target.closest('[data-pick]');
        if (!b) return;
        sel = +b.getAttribute('data-pick');
        bar.querySelectorAll('[data-pick]').forEach(x => x.classList.toggle('on', x === b));
        sync();
      });
      return bar;
    }

    function watch() {
      document.addEventListener('visibilitychange', sync);
      window.addEventListener('pagehide', () => cards.forEach(k => { k.p.stop(); k.on = false; }));
    }

    // ---- gravações ----
    async function loadRecs() {
      if (!CD.state.hub) { recsBox.innerHTML = '<div class="empty">' + t('c.nohub') + '</div>'; return; }
      recsBox.innerHTML = '<div class="empty">' + t('c.loading') + '</div>';
      let list, st = null;
      try { list = await CD.hubApi.recordings.list(); } catch (e) { recsBox.innerHTML = '<div class="empty">' + esc(e.message) + '</div>'; return; }
      try { st = await CD.hubApi.recordings.settings(); } catch (e) { /* só o aviso de "desligada" some */ }
      if (view !== 'recs') return;
      const cams = []; list.forEach(f => { if (cams.indexOf(f.camera) < 0) cams.push(f.camera); });
      if (recCam && cams.indexOf(recCam) < 0) recCam = '';
      const shown = list.filter(f => !recCam || f.camera === recCam);
      let html = '';
      if (st && !st.enabled) html += '<div class="cfg-msg">' + t('cam.rec.off', { link: '<a href="config.html">' + t('def.config') + '</a>' }) + '</div>';
      else if (st) html += '<div class="cfg-md">' + t('cam.rec.stat', { files: st.files, size: size(st.usedBytes), link: '<a href="config.html">' + t('def.config') + '</a>' }) + '</div>';
      if (cams.length > 1) {
        html += '<div class="recs-f"><button type="button" class="vbtn' + (recCam ? '' : ' on') + '" data-cam="">' + t('cam.rec.allCams') + '</button>' +
          cams.map(c => '<button type="button" class="vbtn' + (recCam === c ? ' on' : '') + '" data-cam="' + esc(c) + '">' + esc(c) + '</button>').join('') + '</div>';
      }
      // o arquivo que está sendo gravado não pode ser excluído (o hub recusa); fora da seleção
      const deletable = shown.filter(f => !f.recording);
      Object.keys(picked).forEach(n => { if (!deletable.some(f => f.name === n)) delete picked[n]; });
      const nPick = Object.keys(picked).length;
      if (shown.length) {
        html += '<div class="recs-f"><button type="button" class="vbtn' + (selMode ? ' on' : '') + '" data-sel="mode">' + (selMode ? t('cam.sel.cancel') : t('cam.sel.mode')) + '</button>';
        if (selMode) {
          html += '<button type="button" class="vbtn" data-sel="all">' + (nPick === deletable.length && nPick ? t('cam.sel.clear') : t('cam.sel.all')) + '</button>' +
            '<button type="button" class="vbtn danger" data-sel="del"' + (nPick ? '' : ' disabled') + '>' + t('cam.sel.del') + (nPick ? ' (' + nPick + ')' : '') + '</button>';
        }
        html += '</div>';
      }
      if (!shown.length) html += '<div class="empty">' + t('cam.rec.none') + '</div>';
      let day = '';
      shown.forEach((f, i) => {
        const d = new Date(f.start * 1000), key = CD.i18n.longDate(d);
        if (key !== day) { day = key; html += '<div class="rec-day">' + esc(key) + '</div>'; }
        const end = new Date((f.start + 3600) * 1000);
        html += '<article class="vid">' +
          (selMode ? '<input type="checkbox" class="rec-ck" data-pick="' + i + '" aria-label="' + esc(t('cam.sel.pick')) + '"' + (f.recording ? ' disabled' : '') + (picked[f.name] ? ' checked' : '') + '>' : '') +
          '<div class="vid-main"><div class="vid-name">' + esc(f.camera) + (f.recording ? '<span class="rec-live">' + t('cam.rec.live') + '</span>' : '') + '</div>' +
          '<div class="vid-meta">' + CD.i18n.dateNum(d) + ' · ' + hhmm(d) + ' – ' + hhmm(end) + ' · ' + size(f.size) + '</div></div><div class="vid-act">' +
          '<button type="button" class="vbtn main" data-play="' + i + '">' + t('cam.rec.watch') + '</button>' +
          '<a class="vbtn" href="' + esc(CD.hubApi.recordings.fileUrl(f.name, true)) + '">' + t('cam.rec.download') + '</a>' +
          (f.recording ? '' : '<button type="button" class="vbtn danger" data-del="' + i + '">' + t('c.delete') + '</button>') + '</div></article>';
      });
      recsBox.innerHTML = html;

      async function remove(files, what) {
        if (!files.length || !confirm(t('cam.rec.confirm', { what: what }))) return;
        let fail = 0, last = '';
        for (const f of files) {
          try { await CD.hubApi.recordings.remove(f.name); delete picked[f.name]; } catch (e) { fail++; last = e.message; }
        }
        if (!Object.keys(picked).length) selMode = false;
        await loadRecs();
        if (fail) { const m = document.createElement('div'); m.className = 'cfg-msg'; m.textContent = t('cam.rec.failDel', { n: fail, err: last }); recsBox.insertBefore(m, recsBox.firstChild); }
      }
      recsBox.onclick = e => {
        const b = e.target.closest && e.target.closest('button');
        if (!b) return;
        if (b.hasAttribute('data-cam')) { recCam = b.getAttribute('data-cam'); loadRecs(); }
        else if (b.hasAttribute('data-play')) player(shown[+b.getAttribute('data-play')]);
        else if (b.hasAttribute('data-del')) { const f = shown[+b.getAttribute('data-del')]; remove([f], t('cam.rec.thisOne')); }
        else if (b.hasAttribute('data-sel')) {
          const a = b.getAttribute('data-sel');
          if (a === 'mode') { selMode = !selMode; picked = {}; loadRecs(); }
          else if (a === 'all') { const all = nPick === deletable.length && nPick; picked = {}; if (!all) deletable.forEach(f => { picked[f.name] = 1; }); loadRecs(); }
          else if (a === 'del') remove(deletable.filter(f => picked[f.name]), t('cam.rec.nSelected', { n: nPick }));
        }
      };
      recsBox.onchange = e => {
        const c = e.target;
        if (!c.hasAttribute || !c.hasAttribute('data-pick')) return;
        const f = shown[+c.getAttribute('data-pick')];
        if (c.checked) picked[f.name] = 1; else delete picked[f.name];
        loadRecs();
      };
    }

    function show(v) {
      view = v;
      root.querySelectorAll('[data-v]').forEach(b => b.classList.toggle('on', b.getAttribute('data-v') === v));
      box.hidden = v !== 'live'; recsBox.hidden = v !== 'recs';
      const pk = root.querySelector('.cams-pick'); if (pk) pk.hidden = v !== 'live';
      sync();
      if (v === 'recs') loadRecs();
    }
    root.querySelector('.cams-bar').addEventListener('click', e => {
      const b = e.target.closest && e.target.closest('[data-v]');
      if (b) show(b.getAttribute('data-v'));
    });

    async function load() {
      if (!CD.state.hub) { note(t('cam.nohub')); return; }
      let list;
      try { list = await CD.hubApi.cameras.list(); } catch (e) { note(e.message); return; }
      if (!list || !list.length) { note(t('cam.none')); return; }
      box.innerHTML = '';
      list.forEach(c => box.appendChild(card(c)));
      if (compat()) box.parentNode.insertBefore(picker(list), box);
      sync();
      watch();
    }
    load();
  }

  CD.pages = CD.pages || {};
  CD.pages.camaras = { labelKey: 'page.camaras.label', subKey: 'page.camaras.sub', icon: 'camera', mount: mount };
})(window.CD = window.CD || {});
