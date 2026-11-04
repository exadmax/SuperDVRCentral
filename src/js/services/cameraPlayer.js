(function (CD) {
  // Player de câmera ao vivo: toca o MP4 contínuo do hub (/api/cameras/{id}/stream) com Media Source Extensions.
  // Usado pela página Câmeras (camaras.html) e pelo card rotativo da tela inicial.
  const t = CD.i18n.t;
  const MS = window.ManagedMediaSource || window.MediaSource;
  const LIVE_LAG = 1.2;      // se a imagem atrasar mais que isso (s), pula para o "agora"
  const KEEP_BEHIND = 6;     // segundos já vistos que ficam na memória
  const RETRY_MS = 4000;

  const u32 = (b, o) => ((b[o] << 24) | (b[o + 1] << 16) | (b[o + 2] << 8) | b[o + 3]) >>> 0;
  function concat(a, b) {
    if (!a.length) return b;
    const out = new Uint8Array(a.length + b.length);
    out.set(a, 0); out.set(b, a.length);
    return out;
  }

  // Toca um stream fMP4 no <video>. onState(tipo, texto): 'load' conectando, 'play' no ar, 'err' erro (tenta de novo sozinho).
  function player(video, url, onState) {
    let gen = 0, ctl = null, ms = null, sb = null, objUrl = '', timer = 0, watchdog = 0, lastData = 0;
    let pending = new Uint8Array(0), group = [], queue = [];

    function cleanup() {
      gen++;
      clearTimeout(timer); clearInterval(watchdog);
      if (ctl) { try { ctl.abort(); } catch (e) { /* já encerrado */ } ctl = null; }
      try { video.pause(); } catch (e) { /* sem vídeo */ }
      video.removeAttribute('src');
      try { video.load(); } catch (e) { /* sem vídeo */ }
      if (objUrl) { try { URL.revokeObjectURL(objUrl); } catch (e) { /* já liberado */ } objUrl = ''; }
      ms = sb = null; pending = new Uint8Array(0); group = []; queue = [];
    }

    function fail(my, msg) {
      if (my !== gen) return;
      cleanup();
      onState('err', msg);
      timer = setTimeout(start, RETRY_MS);
    }

    function tidy() {
      if (!sb || sb.updating || !video.buffered.length) return;
      const start = video.buffered.start(0), end = video.buffered.end(video.buffered.length - 1);
      if (video.currentTime < start || end - video.currentTime > LIVE_LAG) video.currentTime = Math.max(start, end - 0.4);
      if (video.currentTime - start > KEEP_BEHIND * 2) { try { sb.remove(start, video.currentTime - KEEP_BEHIND); } catch (e) { /* tenta na próxima */ } }
    }

    function pump(my) {
      if (!sb || sb.updating || !queue.length || my !== gen) return;
      try { sb.appendBuffer(queue.shift()); }
      catch (e) {
        if (e && e.name === 'QuotaExceededError' && video.buffered.length) {
          try { sb.remove(video.buffered.start(0), Math.max(video.buffered.start(0), video.currentTime - 1)); } catch (x) { /* segue */ }
        } else fail(my, t('cp.playErr'));
      }
    }

    // O servidor manda caixas MP4 inteiras (init = ftyp+moov; quadro = moof+mdat); junta as caixas e entrega cada grupo completo.
    function feed(my, chunk) {
      pending = concat(pending, chunk);
      while (pending.length >= 8) {
        const size = u32(pending, 0);
        if (size < 8 || size > (1 << 24)) throw new Error(t('cp.badStream'));
        if (pending.length < size) break;
        const type = String.fromCharCode(pending[4], pending[5], pending[6], pending[7]);
        group.push(pending.slice(0, size));
        pending = pending.subarray(size);
        if (type === 'moov' || type === 'mdat') {
          let n = 0; group.forEach(g => { n += g.length; });
          const joined = new Uint8Array(n); let o = 0;
          group.forEach(g => { joined.set(g, o); o += g.length; });
          group = [];
          queue.push(joined);
        }
      }
      if (queue.length > 300) queue = queue.slice(-60);            // navegador não acompanha: descarta o atrasado
      pump(my);
    }

    async function start() {
      cleanup();
      const my = gen;
      if (!MS || !window.fetch) { onState('err', t('cp.noLive')); return; }
      onState('load', t('cam.connecting'));
      try {
        ctl = window.AbortController ? new AbortController() : null;
        const r = await fetch(url, ctl ? { signal: ctl.signal, cache: 'no-store' } : { cache: 'no-store' });
        if (my !== gen) return;
        if (!r.ok) throw new Error(CD.i18n.srv(((await r.text()) || '').trim()) || t('cp.httpErr', { n: r.status }));
        if (!r.body || !r.body.getReader) throw new Error(t('cp.noStream'));
        const type = 'video/mp4; codecs="' + r.headers.get('X-Codec') + '"';
        if (!MS.isTypeSupported(type)) throw new Error(t('cp.noCodec', { type: type }));

        ms = new MS();
        objUrl = URL.createObjectURL(ms);
        if (window.ManagedMediaSource && MS === window.ManagedMediaSource) video.disableRemotePlayback = true;
        video.src = objUrl;
        await new Promise(res => ms.addEventListener('sourceopen', res, { once: true }));
        if (my !== gen) return;
        sb = ms.addSourceBuffer(type);
        try { sb.mode = 'sequence'; } catch (e) { /* o padrão serve */ }
        sb.addEventListener('updateend', () => { tidy(); pump(my); });
        sb.addEventListener('error', () => fail(my, t('cp.playErr')));

        lastData = Date.now();
        watchdog = setInterval(() => { if (Date.now() - lastData > 15000) fail(my, t('cp.stalled')); }, 5000);
        const reader = r.body.getReader();
        for (;;) {
          const { done, value } = await reader.read();
          if (my !== gen) return;
          if (done) throw new Error(t('cp.closed'));
          lastData = Date.now();
          feed(my, value);
          if (video.paused) { const p = video.play(); if (p && p.catch) p.catch(() => { /* espera o navegador liberar */ }); }
        }
      } catch (e) {
        if (my !== gen) return;
        fail(my, (e && e.message) || t('err.offline'));
      }
    }

    video.addEventListener('playing', () => onState('play', ''));
    return { start: start, stop: cleanup };
  }

  CD.cameraPlayer = {
    supported: () => !!(MS && window.fetch && window.ReadableStream),
    create: player
  };
})(window.CD = window.CD || {});
