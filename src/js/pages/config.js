(function (CD) {
  // Página "Config" (config.html): uso de CPU/memória do servidor e dois botões para aliviá-lo — desligar o desktop da
  // máquina e desativar a própria central (hub). Nada disso é permanente: o desktop volta no próximo reinício.
  // Também configura a gravação das câmeras, por câmera: liga/desliga, modo (contínuo ou só com movimento), sensibilidade, dias de retenção e limite de GB.
  const U = CD.util, esc = U.esc, t = CD.i18n.t;

  const nf = CD.i18n.num;
  const bytes = b => b >= 1073741824 ? nf(b / 1073741824, 1) + ' GB' : Math.max(1, Math.round(b / 1048576)) + ' MB';

  function meter(label, pct, detail) {
    return '<div class="cfg-m"><div class="cfg-mh"><span>' + esc(label) + '</span><b>' + nf(pct, 0) + '%</b></div>' +
      '<div class="cfg-bar"><i style="width:' + Math.max(1, Math.min(100, pct)) + '%"></i></div><div class="cfg-md">' + esc(detail) + '</div></div>';
  }

  function mount(root) {
    // Subseções: cada cartão tem data-tab e só os da aba escolhida aparecem (o CSS esconde os outros; nada é redesenhado ao trocar de aba)
    const TABS = ['servidor', 'cameras', 'agendas', 'painel', 'livros', 'menu', 'integracoes', 'ia', 'idioma', 'compat', 'ajuda'].map(id => [id, t('cfg.tab.' + id)]);
    let tab = 'servidor';
    try { tab = (location.hash || '').slice(1) || sessionStorage.getItem('cd:cfgtab') || tab; } catch (e) { /* sem armazenamento */ }
    if (!TABS.some(t => t[0] === tab)) tab = 'servidor';
    root.innerHTML = '<section class="pg cfg-root" data-show="' + tab + '"><nav class="cams-bar cfg-tabs">' + TABS.map(t =>
      '<button type="button" class="vbtn' + (t[0] === tab ? ' on' : '') + '" data-tab-go="' + t[0] + '">' + t[1] + '</button>').join('') + '</nav><div class="cfg-helplink" id="helplink"></div>' +
      '<div class="cfg" id="cfg"></div><div id="cfgpanel" class="cfg-stack"></div><div id="cfgai" class="cfg-stack"></div><div id="cfgedit" class="cfg-stack"></div><div id="cfghelp" class="cfg-stack"></div></section>';
    const helpEl = root.querySelector('#cfghelp'), helpLink = root.querySelector('#helplink');
    // “Ajuda desta aba”: leva à seção da ajuda sobre o que está na tela
    const showHelpLink = () => { helpLink.innerHTML = tab === 'ajuda' ? '' : '<a href="#ajuda" data-help-go="' + tab + '">❓ ' + t('cfg.helpTab') + '</a>'; };
    function goTab(t) {
      tab = t;
      root.firstChild.setAttribute('data-show', tab);
      root.querySelectorAll('[data-tab-go]').forEach(x => x.classList.toggle('on', x.getAttribute('data-tab-go') === t));
      try { sessionStorage.setItem('cd:cfgtab', tab); } catch (x) { /* ok */ }
      showHelpLink();
    }
    showHelpLink();
    helpLink.addEventListener('click', e => {
      const a = e.target.closest && e.target.closest('[data-help-go]');
      if (!a) return;
      e.preventDefault();
      goTab('ajuda');
      if (helpEl.go) helpEl.go(a.getAttribute('data-help-go'));
    });
    root.querySelector('.cfg-tabs').addEventListener('click', e => {
      const b = e.target.closest && e.target.closest('[data-tab-go]');
      if (!b) return;
      goTab(b.getAttribute('data-tab-go'));
    });
    const box = root.querySelector('#cfg');
    if (CD.configEdit) CD.configEdit.mount(root.querySelector('#cfgedit'));   // câmeras, agendas, rotas, menu e modo compatibilidade (configEdit.js)
    if (CD.configHelp) CD.configHelp.mount(helpEl);                             // manual de cada configuração (configHelp.js)
    if (CD.configPanel) CD.configPanel.mount(root.querySelector('#cfgpanel')); // painel, estoque, feriados e opções do hub (configPanel.js)
    if (CD.configAI) CD.configAI.mount(root.querySelector('#cfgai'));          // GPU, modelos sugeridos e provedor de IA (configAI.js)
    let sys = null, rec = null, drv = null, msg = '', recMsg = '', armed = '', armTimer = 0;

    function draw() {
      if (!CD.state.hub) { box.innerHTML = '<div class="empty" data-tab="servidor">' + t('c.nohub') + '</div>'; return; }
      if (!sys) { box.innerHTML = '<div class="empty" data-tab="servidor">' + t('c.loading') + '</div>'; return; }
      const can = sys.control, linuxOnly = t('cfg.linuxOnly');
      box.innerHTML = recCard() + driveCard() +
        '<div class="cfg-card" data-tab="servidor"><h2>' + t('cfg.tab.servidor') + '</h2><div class="cfg-ms">' +
        meter(t('cfg.m.hubCpu'), sys.appCpuPercent, t('cfg.m.hubCpuD', { n: sys.cores })) +
        meter(t('cfg.m.hubMem'), sys.appMemPercent, t('cfg.m.hubMemD', { size: bytes(sys.appMem) })) +
        meter(t('cfg.m.pcCpu'), sys.cpuPercent, t('cfg.m.pcCpuD', { n: sys.cores })) +
        meter(t('cfg.m.pcMem'), sys.memPercent, t('cfg.m.pcMemD', { used: bytes(sys.memUsed), total: bytes(sys.memTotal) })) +
        '</div></div>' +
        '<div class="cfg-card" data-tab="servidor"><h2>' + t('cfg.free') + '</h2>' +
        action('desktop', t('cfg.desktop.title'), t('cfg.desktop.text'),
          can && sys.desktop, !can ? linuxOnly : !sys.desktop ? t('cfg.desktop.off') : '') +
        action('hub', t('cfg.hub.title'), t('cfg.hub.text'),
          can, !can ? linuxOnly : '', true) +
        (msg ? '<div class="cfg-msg">' + esc(msg) + '</div>' : '') + '</div>';
    }

    function sw(id, on, label) {
      return '<button type="button" class="cfg-sw' + (on ? ' on' : '') + '" role="switch" aria-checked="' + on + '" aria-label="' + esc(label) + '" data-sw="' + esc(id) + '"></button>';
    }

    function sel(cam, field, value, opts) {
      const have = opts.some(o => o[0] === value);
      return '<select class="cfg-sel" data-cam="' + esc(cam) + '" data-f="' + field + '">' + opts.map(o =>
        '<option value="' + o[0] + '"' + (o[0] === value ? ' selected' : '') + '>' + esc(o[1]) + '</option>').join('') +
        (have ? '' : '<option value="' + value + '" selected>' + esc(String(value)) + '</option>') + '</select>';
    }
    function row(title, text, control) {
      return '<div class="cfg-a"><div class="cfg-at"><b>' + esc(title) + '</b><span>' + esc(text) + '</span></div>' + control + '</div>';
    }

    const DAYS = [1, 2, 3, 5, 7, 10, 14, 30, 60].map(d => [d, CD.i18n.n('cfg.days', d)]);
    const GBS = [[0, t('cfg.noLimit')]].concat([5, 10, 20, 50, 100, 200, 500].map(g => [g, g + ' GB']));
    const SENS = [[1, t('cfg.sens.1')], [2, '2'], [3, t('cfg.sens.3')], [4, '4'], [5, t('cfg.sens.5')]];

    const when = iso => CD.i18n.dateTime(new Date(iso));
    const driveBusy = () => !!(drv && drv.running);
    function driveBtn(id, label) {
      return '<button type="button" class="btn ghost" data-drv="' + esc(id) + '"' + (driveBusy() ? ' disabled' : '') + '>' + esc(label) + '</button>';
    }
    // Linha "Google Drive" de uma câmera: o que falta enviar e o botão que envia só ela.
    function driveRow(c) {
      if (!drv || !drv.enabled) return '';
      const p = (drv.pending || {})[c.id] || { files: 0, bytes: 0 };
      const sending = driveBusy() && (!drv.scope || drv.scope.split(', ').indexOf(c.name) >= 0);
      const todo = p.files ? t('cfg.drv.todo', { n: p.files, size: bytes(p.bytes) }) : t('cfg.drv.nothing');
      const text = sending ? t('cfg.drv.sending') : c.driveOff ?
        t('cfg.drv.off', { todo: p.files ? t('cfg.drv.todoIfClick', { n: p.files, size: bytes(p.bytes) }) : todo }) : todo;
      return row(t('cfg.drv.row'), text, '<span class="cfg-ctl">' + driveBtn(c.id, t('cfg.drv.sync')) + sw('drv:' + c.id, !c.driveOff, t('cfg.drv.sendAria', { name: c.name })) + '</span>');
    }
    function driveCard() {
      if (!drv) return '';
      if (!drv.enabled) return '<div class="cfg-card" data-tab="cameras"><h2>Google Drive</h2><div class="cfg-msg">' + t('cfg.drv.disabled') + '</div></div>';
      const lines = [];
      if (drv.running) lines.push(t('cfg.drv.running') + (drv.scope ? ' (' + drv.scope + ')' : '') + (drv.progress ? ': ' + drv.progress : '…'));
      else if (drv.lastMsg) lines.push((drv.lastOK ? '' : t('cfg.drv.errPrefix')) + CD.i18n.srv(drv.lastMsg));
      if (!drv.running && drv.attempts && !drv.lastOK && drv.attempts < drv.maxAttempts)
        lines.push(t('cfg.drv.attempt', { n: drv.attempts, max: drv.maxAttempts }) + (drv.nextRetry ? t('cfg.drv.nextAt', { t: when(drv.nextRetry) }) : t('cfg.drv.noTime')));
      const text = t('cfg.drv.info', { folder: drv.remote.replace(/^.*:/, ''), hour: String(drv.hour).padStart(2, '0'), next: when(drv.nextRun), up: drv.uploaded }) +
        (drv.skipped ? t('cfg.drv.skipped', { n: drv.skipped }) : '') + '.';
      return '<div class="cfg-card" data-tab="cameras"><h2>Google Drive</h2>' + row(t('cfg.drv.all'), text, driveBtn('', t('cfg.drv.syncAll'))) +
        (lines.length ? '<div class="cfg-msg">' + lines.map(esc).join('<br>') + '</div>' : '') + '</div>';
    }

    function camBlock(c) {
      const motion = c.mode === 'motion';
      const state = !rec.enabled ? t('cfg.rec.stOff') : !c.enabled ? t('cfg.rec.stCamOff') : c.error ? CD.i18n.srv(c.error) :
        motion ? (c.motion ? t('cfg.rec.stMotion') : c.recording ? t('cfg.rec.stTail') : t('cfg.rec.stWait')) :
        c.recording ? t('cfg.rec.stNow') : t('cfg.rec.stStart');
      const cls = rec.enabled && c.enabled ? (c.error ? 'cfg-bad' : c.recording ? 'cfg-ok' : '') : '';
      return '<div class="cfg-cam"><div class="cfg-a"><div class="cfg-at"><b>' + esc(c.name) + '</b><span class="' + cls + '">' + esc(state) + ' · ' +
        t('cfg.rec.files', { n: c.files, size: bytes(c.usedBytes) }) + '</span></div>' + sw('cam:' + c.id, c.enabled, t('cfg.rec.recAria', { name: c.name })) + '</div>' + driveRow(c) +
        (c.enabled ? '<div class="cfg-opts">' +
          row(t('cfg.rec.mode'), motion ? t('cfg.rec.modeMotionD') : t('cfg.rec.modeContD'),
            sel(c.id, 'mode', c.mode, [['continuous', t('cfg.rec.cont')], ['motion', t('cfg.rec.motion')]])) +
          (motion ? row(t('cfg.rec.sens'), t('cfg.rec.sensD'), sel(c.id, 'sensitivity', c.sensitivity, SENS)) : '') +
          row(t('cfg.rec.keep'), t('cfg.rec.keepD'), sel(c.id, 'retentionDays', c.retentionDays, DAYS)) +
          row(t('cfg.rec.limit'), t('cfg.rec.limitD'), sel(c.id, 'maxGB', c.maxGB, GBS)) + '</div>' : '') + '</div>';
    }

    function recCard() {
      if (!rec) return '<div class="cfg-card" data-tab="cameras"><h2>' + t('cfg.rec.title') + '</h2><div class="cfg-msg">' + esc(recMsg || t('c.loading')) + '</div></div>';
      return '<div class="cfg-card" data-tab="cameras"><h2>' + t('cfg.rec.title') + '</h2>' +
        row(t('cfg.rec.master'), t(rec.enabled ? 'cfg.rec.on' : 'cfg.rec.offS') + t('cfg.rec.masterD', { dir: rec.dir, n: rec.files, size: bytes(rec.usedBytes) }),
          sw('master', rec.enabled, t('cfg.rec.master'))) +
        (rec.cameras || []).map(camBlock).join('') +
        (recMsg ? '<div class="cfg-msg">' + esc(recMsg) + '</div>' : '') + '</div>';
    }

    async function saveRec(patch) {
      recMsg = '';
      try { rec = await CD.hubApi.recordings.save(patch); } catch (e) { recMsg = e.message; }
      draw();
    }

    function action(id, title, text, enabled, why, danger) {
      const on = armed === id;
      return '<div class="cfg-a"><div class="cfg-at"><b>' + esc(title) + '</b><span>' + esc(why || text) + '</span></div>' +
        '<button type="button" class="btn' + (danger ? ' danger' : '') + (on ? '' : ' ghost') + '" data-act="' + id + '"' + (enabled ? '' : ' disabled') + '>' +
        (on ? t('cfg.confirm') : esc(title)) + '</button></div>';
    }

    async function load() {
      if (!CD.state.hub) return draw();
      try { sys = await CD.hubApi.system(); } catch (e) { /* mantém o anterior */ }
      try { rec = await CD.hubApi.recordings.settings(); } catch (e) { if (!rec) recMsg = e.message; }
      try { drv = await CD.hubApi.recordings.drive(); } catch (e) { /* hub antigo, sem a cópia para o Drive */ }
      draw();
    }

    // Primeiro toque arma o botão (5 s); o segundo executa.
    box.addEventListener('change', e => {
      const el = e.target;
      if (!el.hasAttribute || !el.hasAttribute('data-f')) return;
      const o = {}, f = el.getAttribute('data-f');
      o[f] = f === 'mode' ? el.value : +el.value;
      const cams = {}; cams[el.getAttribute('data-cam')] = o;
      saveRec({ cameras: cams });
    });
    box.addEventListener('click', async e => {
      const swEl = e.target.closest && e.target.closest('[data-sw]');
      if (swEl && rec) {
        const id = swEl.getAttribute('data-sw');
        if (id === 'master') saveRec({ enabled: !rec.enabled });
        else if (id.indexOf('drv:') === 0) { const c = (rec.cameras || []).filter(x => 'drv:' + x.id === id)[0]; if (c) { const o = {}; o[c.id] = { driveOff: !c.driveOff }; saveRec({ cameras: o }); } }
        else { const c = (rec.cameras || []).filter(x => 'cam:' + x.id === id)[0]; if (c) { const o = {}; o[c.id] = { enabled: !c.enabled }; saveRec({ cameras: o }); } }
        return;
      }
      const dv = e.target.closest && e.target.closest('[data-drv]');
      if (dv) {
        if (dv.disabled) return;
        const id = dv.getAttribute('data-drv');
        dv.disabled = true; recMsg = '';
        try { drv = await CD.hubApi.recordings.driveRun(id ? [id] : []); } catch (err) { recMsg = err.message; }
        draw();
        return;
      }
      const b = e.target.closest && e.target.closest('[data-act]');
      if (!b || b.disabled) return;
      const id = b.getAttribute('data-act');
      if (armed !== id) {
        armed = id; msg = ''; draw();
        clearTimeout(armTimer); armTimer = setTimeout(() => { armed = ''; draw(); }, 5000);
        return;
      }
      clearTimeout(armTimer); armed = ''; b.disabled = true;
      try {
        await CD.hubApi.power(id === 'hub' ? 'hub-off' : 'desktop-off');
        msg = id === 'hub' ? t('cfg.hub.done') : t('cfg.desktop.done');
        if (id === 'hub') { clearInterval(tick); sys = null; box.innerHTML = '<div class="cfg-card" data-tab="servidor"><h2>' + t('cfg.hub.doneTitle') + '</h2><div class="cfg-msg">' + esc(msg) + '</div></div>'; return; }
        setTimeout(load, 4000);
      } catch (err) { msg = err.message; }
      draw();
    });

    load();
    const tick = setInterval(() => { if (!document.hidden && !armed && !(document.activeElement && document.activeElement.tagName === 'SELECT')) load(); }, 5000);
  }

  CD.pages = CD.pages || {};
  CD.pages.config = { labelKey: 'def.config', subKey: 'page.config.sub', icon: 'gear', mount: mount };
})(window.CD = window.CD || {});
