(function (CD) {
  // Voz do Omega no navegador: escuta contínua com a palavra de ativação "Omega" (Web Speech API: Chrome e Edge), fala com a
  // síntese de voz do próprio sistema e mede o nível do microfone para o núcleo 3D reagir.
  // Atenção: no Chrome o reconhecimento de fala envia o áudio para o serviço do Google; o Edge usa o da Microsoft.
  const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
  const WAKE = /\b(ok\s+)?(o|ô|ó)mega\b[,.!:]?\s*/i;
  const AWAIT_MS = 8000;      // depois de "Omega" sozinho, quanto tempo espera o pedido

  function create(opts) {
    // opts: lang, onWake(), onInterim(text), onCommand(text), onState(listening:boolean), onSpeak(on:boolean), onError(msg),
    //       noWake (opcional): dispensa a palavra "Omega" (usado pela TV, onde o microfone é apertado para falar)
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
      if (opts.noWake) { opts.onCommand(txt); return; }   // TV: quem apertou o microfone já chamou; tudo o que ouvir é o pedido
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

    // A voz vem da Config (nome exato); sem ela, a escolha automática é sempre a mesma para a mesma lista de vozes.
    let voiceName = opts.voiceName || '';
    function setVoice(name) { voiceName = String(name || ''); }

    function speak(text) {
      if (!window.speechSynthesis || !text) return Promise.resolve();
      // a lista de vozes carrega depois da página: sem esperar por ela a primeira fala saía com a voz padrão do sistema
      return voicesReady().then(() => new Promise(resolve => {
        speechSynthesis.cancel();
        const u = new SpeechSynthesisUtterance(text);
        u.lang = opts.lang; u.rate = 1.05; u.pitch = 0.9;
        const v = findVoice(speechSynthesis.getVoices(), voiceName, opts.lang);
        if (v) { u.voice = v; u.lang = v.lang || opts.lang; }
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
      }));
    }

    function hush() { if (window.speechSynthesis) speechSynthesis.cancel(); }

    return { start: start, stop: stop, speak: speak, hush: hush, level: level, setVoice: setVoice, active: () => enabled, speaking: () => speaking };
  }

  // ---------- vozes de síntese (compartilhado com a página Config) ----------
  const sameLang = (v, lang) => (v.lang || '').toLowerCase().replace('_', '-').indexOf(lang.slice(0, 2).toLowerCase()) === 0;

  // Resolve quando a lista de vozes do navegador está carregada (ou, no máximo, em VOICES_WAIT ms: há navegadores sem nenhuma voz).
  const VOICES_WAIT = 1500;
  function voicesReady() {
    return new Promise(resolve => {
      if (!window.speechSynthesis) return resolve([]);
      if (speechSynthesis.getVoices().length) return resolve(speechSynthesis.getVoices());
      const end = () => { speechSynthesis.removeEventListener('voiceschanged', end); clearTimeout(tm); resolve(speechSynthesis.getVoices()); };
      const tm = setTimeout(end, VOICES_WAIT);
      speechSynthesis.addEventListener('voiceschanged', end);
    });
  }

  // Voz pelo nome configurado; se não existir neste aparelho, a escolha automática: do idioma, "natural/neural" primeiro, depois por nome
  // (ordem fixa, para não mudar de voz a cada abertura da página).
  function findVoice(vs, name, lang) {
    if (name) { const exact = vs.find(v => v.name === name || v.voiceURI === name); if (exact) return exact; }
    const full = lang.toLowerCase();
    const rank = v => (/natural|neural|online/i.test(v.name) ? 0 : 2) + ((v.lang || '').toLowerCase().replace('_', '-') === full ? 0 : 1);
    const same = vs.filter(v => sameLang(v, lang)).sort((a, b) => rank(a) - rank(b) || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
    return same[0] || null;
  }

  // Fala uma frase de teste com a voz dada (aba IA da Config).
  function preview(name, lang, text) {
    if (!window.speechSynthesis) return;
    voicesReady().then(vs => {
      speechSynthesis.cancel();
      const u = new SpeechSynthesisUtterance(text);
      const v = findVoice(vs, name, lang);
      u.lang = (v && v.lang) || lang; u.rate = 1.05; u.pitch = 0.9;
      if (v) u.voice = v;
      speechSynthesis.speak(u);
    });
  }

  CD.omegaVoice = {
    create: create, canListen: () => !!SR, canSpeak: () => !!window.speechSynthesis,
    // vozes do aparelho, as do idioma da interface primeiro: [{name, lang, same}]
    list: function (lang) {
      return voicesReady().then(vs => vs.map(v => ({ name: v.name, lang: v.lang, same: sameLang(v, lang) }))
        .sort((a, b) => (b.same - a.same) || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0)));
    },
    preview: preview
  };
})(window.CD = window.CD || {});
