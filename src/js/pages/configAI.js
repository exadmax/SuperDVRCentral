(function (CD) {
  // Config → IA: a GPU do servidor e quanto de memória de vídeo sobra para um modelo, os modelos sugeridos para ela, o provedor
  // (Ollama local, Groq ou desligado) e o teste de conexão. Dados de /api/ai/hardware e /api/ai/status; gravação em /api/settings/ai.
  // A chave da Groq nunca volta do hub: o campo fica em branco e "configurada" aparece no lugar.
  const U = CD.util, esc = U.esc, ui = CD.cfgUi, t = CD.i18n.t;
  const fld = ui.fld, btn = ui.btn;

  const gb = mb => (mb / 1024).toLocaleString(CD.i18n.locale ? CD.i18n.locale() : undefined, { maximumFractionDigits: 1 }) + ' GB';
  const input = (f, v, o) => {
    o = o || {};
    return '<input data-f="' + f + '" type="' + (o.type || 'text') + '" value="' + esc(v == null ? '' : v) + '"' + (o.ph ? ' placeholder="' + esc(o.ph) + '"' : '') +
      ' autocomplete="' + (o.type === 'password' ? 'new-password' : 'off') + '" spellcheck="false"' + (o.list ? ' list="' + o.list + '"' : '') + '>';
  };
  const select = (f, v, opts) => '<select data-f="' + f + '">' + opts.map(o => '<option value="' + esc(o[0]) + '"' + (String(o[0]) === String(v) ? ' selected' : '') + '>' + esc(o[1]) + '</option>').join('') + '</select>';

  function mount(root) {
    const st = { hw: null, status: null, cfg: null, hwErr: '', msg: null, testing: false, busy: false };

    function cardHW() {
      let body;
      if (st.hwErr) body = '<div class="cfg-msg cfg-err">' + esc(st.hwErr) + '</div>';
      else if (!st.hw) body = '<div class="cfg-msg">' + t('c.loading') + '</div>';
      else {
        const gpus = st.hw.gpus || [];
        body = (gpus.length ? gpus.map(g => '<div class="cfg-row"><b>' + esc(g.name) + '</b> · ' +
          (g.dedicatedMB ? t('ai.hw.dedicated', { v: gb(g.dedicatedMB) }) : '') + (g.dedicatedMB && g.sharedMB ? ' · ' : '') +
          (g.sharedMB ? t('ai.hw.shared', { v: gb(g.sharedMB) }) : '') + (g.integrated ? ' · ' + t('ai.hw.integrated') : '') + '</div>').join('')
          : '<div class="cfg-row">' + t('ai.hw.none') + '</div>') +
          '<div class="cfg-row">' + t('ai.hw.effective', { v: gb(st.hw.effectiveMB || 0) }) + '</div>' +
          '<div class="cfg-sub">' + t('ai.hw.recommended') + '</div><div class="cfg-row">' +
          (st.hw.recommended || []).map(m => '<code>' + esc(m.name) + '</code>' + (m.note ? ' <span class="cfg-hint">' + esc(m.note) + '</span>' : '')).join('<br>') + '</div>';
      }
      return '<div class="cfg-card" data-tab="ia" data-card="aihw"><h2>' + t('ai.hw.title') + '</h2><div class="cfg-hint">' + t('ai.hw.hint') + '</div>' + body + '</div>';
    }

    function statusBlock() {
      const s = st.status;
      if (st.testing) return '<div class="cfg-msg">' + t('ai.testing') + '</div>';
      if (!s) return '';
      if (s.provider === 'off') return '<div class="cfg-msg">' + t('ai.st.off') + '</div>';
      if (s.provider === 'groq') {
        const g = s.groq || {};
        return '<div class="cfg-msg' + (g.configured ? '' : ' cfg-err') + '">' + (g.configured ? t('ai.st.groqOk', { m: g.model || '—' }) : t('ai.st.groqNoKey')) + '</div>';
      }
      const o = s.ollama || {};
      if (!o.reachable) return '<div class="cfg-msg cfg-err">' + esc(s.message ? CD.i18n.srv(s.message) : t('ai.st.down')) + '</div><div class="cfg-hint">' + t('ai.install') + '</div>';
      const models = o.models || [];
      return '<div class="cfg-msg">' + t('ai.st.ollamaOk', { v: o.version || '?', n: models.length }) + '</div>' +
        (models.length ? '<div class="cfg-row">' + models.map(m => '<code>' + esc(m.name) + '</code> ' +
          (m.known ? (m.fits ? '✅ ' + t('ai.fits') : '⚠️ ' + t('ai.noFit')) : '')).join('<br>') + '</div>'
          : '<div class="cfg-hint">' + t('ai.noModels', { m: esc(((st.hw && st.hw.recommended) || [])[0] ? st.hw.recommended[0].name : 'qwen3:4b') }) + '</div>');
    }

    function cardProvider() {
      const c = st.cfg;
      if (!c) return '<div class="cfg-card" data-tab="ia" data-card="aicfg"><h2>' + t('ai.cfg.title') + '</h2><div class="cfg-msg">' + t('c.loading') + '</div></div>';
      const names = ((st.hw && st.hw.all) || []).map(m => m.name).concat(((st.status && st.status.ollama && st.status.ollama.models) || []).map(m => m.name));
      const m = st.msg;
      return '<div class="cfg-card" data-tab="ia" data-card="aicfg"><h2>' + t('ai.cfg.title') + '</h2>' +
        '<div class="cfg-grid">' +
        fld(t('ai.cfg.provider'), select('provider', c.provider, [['ollama', t('ai.prov.ollama')], ['groq', t('ai.prov.groq')], ['off', t('ai.prov.off')]])) +
        fld(t('ai.cfg.ollamaURL'), input('ollamaURL', c.ollamaURL, { ph: 'http://127.0.0.1:11434' }), 'wide') +
        fld(t('ai.cfg.model'), input('model', c.model, { ph: t('ai.cfg.modelAuto'), list: 'ai-models' }), 'wide') +
        '<datalist id="ai-models">' + names.filter((n, i) => names.indexOf(n) === i).map(n => '<option value="' + esc(n) + '">').join('') + '</datalist>' +
        '<div class="cfg-sub wide">Groq</div>' +
        fld(t('ai.cfg.groqModel'), input('groqModel', c.groqModel, { ph: 'openai/gpt-oss-20b' }), 'wide') +
        '<div class="fld wide"><span>' + t('ai.cfg.groqKey') + '</span>' + input('groqKey', '', { type: 'password', ph: c.groqKeySet ? t('cp.secretSet') : t('cp.secretPaste') }) +
        (c.groqKeySet ? '<label class="cfg-chk"><input type="checkbox" data-f="clearGroqKey">' + esc(t('cp.secretClear')) + '</label>' : '') + '</div>' +
        '</div>' + statusBlock() +
        (m ? '<div class="cfg-msg' + (m.bad ? ' cfg-err' : '') + '">' + esc(m.text) + '</div>' : '') +
        '<div class="cfg-bar2">' + btn('aitest', t('ai.test'), '', st.testing) + btn('aisave', t('c.save'), '', st.busy) + '</div></div>';
    }

    function draw() {
      if (!CD.state.hub) { root.innerHTML = '<div class="empty" data-tab="ia">' + t('c.nohub') + '</div>'; return; }
      root.innerHTML = cardHW() + cardProvider();
    }

    function read() {
      const o = {}, el = root.querySelector('[data-card="aicfg"]');
      if (el) el.querySelectorAll('[data-f]').forEach(e => { o[e.getAttribute('data-f')] = e.type === 'checkbox' ? e.checked : e.value.trim(); });
      return o;
    }

    async function test() {
      st.testing = true; draw();
      try { st.status = await CD.hubApi.ai.status(); } catch (e) { st.status = null; st.msg = { text: e.message, bad: true }; }
      st.testing = false; draw();
    }

    async function save() {
      const body = read(); // antes de redesenhar: o desenho volta os campos para o que está salvo
      const refill = () => Object.keys(body).forEach(f => {   // recusado: devolve o que foi digitado aos campos
        const e = root.querySelector('[data-card="aicfg"] [data-f="' + f + '"]');
        if (e) { if (e.type === 'checkbox') e.checked = body[f]; else e.value = body[f]; }
      });
      st.busy = true; st.msg = { text: t('c.saving') }; draw(); refill();
      try {
        st.cfg = await CD.hubApi.ai.save(body);
        st.msg = { text: t('ai.saved') };
      } catch (e) { st.msg = { text: e.message, bad: true }; }
      st.busy = false; draw();
      if (st.msg.bad) refill(); else test();
    }

    root.addEventListener('click', e => {
      const b = e.target.closest && e.target.closest('[data-a]');
      if (!b) return;
      const act = b.getAttribute('data-a');
      if (act === 'aitest') test();
      else if (act === 'aisave') save();
    });

    async function load() {
      if (!CD.state.hub) { draw(); return; }
      draw();
      try { st.cfg = await CD.hubApi.ai.settings(); } catch (e) { st.msg = { text: e.message, bad: true }; }
      draw();
      try { st.hw = await CD.hubApi.ai.hardware(); } catch (e) { st.hwErr = e.message; }
      draw();
      test();
    }
    load();
  }

  CD.configAI = { mount: mount };
})(window.CD = window.CD || {});
