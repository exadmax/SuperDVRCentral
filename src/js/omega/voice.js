(function (CD) {
  // Voz do Omega no navegador: escuta contínua com a palavra de ativação "Omega" (Web Speech API: Chrome e Edge), fala com a
  // síntese de voz do próprio sistema e mede o nível do microfone para o núcleo 3D reagir.
  // Atenção: no Chrome o reconhecimento de fala envia o áudio para o serviço do Google; o Edge usa o da Microsoft.
  const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
  const WAKE = /\b(ok\s+)?(o|ô|ó)mega\b[,.!:]?\s*/i;
  const AWAIT_MS = 8000;      // depois de "Omega" sozinho, quanto tempo espera o pedido

  function create(opts) {
    // opts: lang, onWake(), onInterim(text), onCommand(text), onState(listening:boolean), onSpeak(on:boolean), onError(msg)
    let rec = null, enabled = false, speaking = false, awaitUntil = 0, restartTimer = 0;
    let audioCtx = null, analyser = null, micStream = null, buf = null;

    function newRec() {
      const r = new SR();
      r.lang = opts.lang; r.continuous = true; r.interimResults = true; r.maxAlternatives = 1;
      r.onresult = e => {
        let interim = '';
        for (let i = e.resultIndex; i < e.results.length; i++) {
          const txt = e.results[i][0].transcript;
          if (e.results[i].isFinal) handleFinal(txt); else interim += txt;
        }
        if (interim) opts.onInterim(interim);
      };
      r.onerror = e => {
        if (e.error === 'not-allowed' || e.error === 'service-not-allowed') { enabled = false; opts.onError('mic-denied'); }
        else if (e.error !== 'no-speech' && e.error !== 'aborted') opts.onError(e.error);
      };
      r.onend = () => { opts.onState(false); if (enabled && !speaking) restartTimer = setTimeout(startRec, 250); };
      r.onstart = () => opts.onState(true);
      return r;
    }

    function startRec() {
      clearTimeout(restartTimer);
      if (!enabled || speaking) return;
      try { rec = rec || newRec(); rec.start(); } catch (e) { /* já estava ouvindo */ }
    }

    function handleFinal(txt) {
      txt = String(txt || '').trim();
      if (!txt) return;
      const m = WAKE.exec(txt);
      if (m) {
        const rest = txt.slice(m.index + m[0].length).trim();
        if (rest.split(/\s+/).length >= 2) { awaitUntil = 0; opts.onCommand(rest); }
        else { awaitUntil = Date.now() + AWAIT_MS; opts.onWake(); }
        return;
      }
      if (Date.now() < awaitUntil) { awaitUntil = 0; opts.onCommand(txt); }
    }

    async function micLevel() {
      if (analyser) return;
      try {
        micStream = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true } });
        audioCtx = new (window.AudioContext || window.webkitAudioContext)();
        analyser = audioCtx.createAnalyser(); analyser.fftSize = 512;
        audioCtx.createMediaStreamSource(micStream).connect(analyser);
        buf = new Uint8Array(analyser.fftSize);
      } catch (e) { /* sem nível: o reconhecimento pode funcionar mesmo assim */ }
    }

    function level() {
      if (!analyser) return 0;
      analyser.getByteTimeDomainData(buf);
      let s = 0;
      for (let i = 0; i < buf.length; i++) { const v = (buf[i] - 128) / 128; s += v * v; }
      return Math.min(1, Math.sqrt(s / buf.length) * 4);
    }

    function start() {
      if (!SR) return false;
      enabled = true; micLevel(); startRec();
      return true;
    }

    function stop() {
      enabled = false; awaitUntil = 0; clearTimeout(restartTimer);
      if (rec) try { rec.abort(); } catch (e) { /* já parado */ }
      if (micStream) micStream.getTracks().forEach(t => t.stop());
      if (audioCtx) audioCtx.close();
      micStream = audioCtx = analyser = null;
      opts.onState(false);
    }

    // escolhe uma voz do idioma (prefere as "naturais"/neurais do Edge e do Windows)
    function pickVoice() {
      const vs = (window.speechSynthesis && speechSynthesis.getVoices()) || [];
      const base = opts.lang.slice(0, 2).toLowerCase();
      const same = vs.filter(v => (v.lang || '').toLowerCase().replace('_', '-').indexOf(base) === 0);
      return same.find(v => /natural|neural|online/i.test(v.name) && v.lang.toLowerCase() === opts.lang.toLowerCase()) ||
        same.find(v => v.lang.toLowerCase() === opts.lang.toLowerCase()) || same[0] || null;
    }

    function speak(text) {
      return new Promise(resolve => {
        if (!window.speechSynthesis || !text) return resolve();
        speechSynthesis.cancel();
        const u = new SpeechSynthesisUtterance(text);
        u.lang = opts.lang; u.rate = 1.05; u.pitch = 0.9;
        const v = pickVoice();
        if (v) u.voice = v;
        const done = () => {
          if (!speaking) return;
          speaking = false; opts.onSpeak(false);
          if (enabled) startRec();
          resolve();
        };
        u.onend = done; u.onerror = done;
        speaking = true; opts.onSpeak(true);
        if (rec) try { rec.abort(); } catch (e) { /* para não ouvir a própria voz */ }
        speechSynthesis.speak(u);
      });
    }

    function hush() { if (window.speechSynthesis) speechSynthesis.cancel(); }

    if (window.speechSynthesis) speechSynthesis.onvoiceschanged = () => { /* carrega a lista de vozes */ };
    return { start: start, stop: stop, speak: speak, hush: hush, level: level, active: () => enabled, speaking: () => speaking };
  }

  CD.omegaVoice = { create: create, canListen: () => !!SR, canSpeak: () => !!window.speechSynthesis };
})(window.CD = window.CD || {});
