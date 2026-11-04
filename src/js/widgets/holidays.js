(function (CD) {
  const U = CD.util, t = CD.i18n.t;

  // Selo central do cabeçalho: feriado (cor por tipo), fim de semana ou dia útil
  function pill(s) {
    const h = CD.holidaysApi.today(s.holidays, s.now);
    if (h) return '<div class="pill hol ' + h.type + '"><span class="pdot"></span><span>' + t('pill.holiday.' + h.type, { name: U.esc(CD.i18n.holiday(h.name)) }) + '</span></div>';
    const dow = s.now.getDay();
    return '<div class="pill">' + (dow === 0 || dow === 6 ? t('pill.weekend') : t('pill.weekday')) + '</div>';
  }

  const nf = CD.i18n.num;
  function bytes(b) { return b >= 1073741824 ? nf(b / 1073741824, 1) + ' GB' : Math.max(1, Math.round(b / 1048576)) + ' MB'; }

  // Uso de CPU/memória do hub e da máquina (vem de /api/system; atualizado no lugar por CD.widgets.sysText)
  function sysText(y) {
    if (!y) return '';
    return t('sys.text', { appCpu: nf(y.appCpuPercent, 0), appMem: bytes(y.appMem), appMemPct: nf(y.appMemPercent, 1),
      cpu: nf(y.cpuPercent, 0), cores: y.cores, memUsed: bytes(y.memUsed), memTotal: bytes(y.memTotal), memPct: nf(y.memPercent, 0) });
  }

  function footer(s) {
    const n = CD.holidaysApi.next(s.holidays, s.now);
    let nextTxt = '';
    if (n) {
      const name = U.esc(CD.i18n.holiday(n.name));
      nextTxt = n.days === 1 ? t('hol.tomorrow', { name: name }) : t('hol.until', { n: n.days, name: name, date: U.dataCurta(n.date) });
    }
    const conn = s.online && !s.wxStale
      ? '<span class="ok"><span class="cdot"></span>' + t('conn.ok') + '</span>'
      : '<span class="bad"><span class="cdot"></span>' + t('conn.off') + '</span>';
    const night = s.nightSkipped ? '<button type="button" class="nightbtn" data-night="on">' + t('night.on') + '</button>' : '';   // desligado à mão nesta noite
    const ver = s.version ? ' · ' + (/^\d/.test(s.version) ? 'V' : '') + U.esc(s.version) : '';      // versão instalada no hub (V1.0.1; "dev" fora do servidor)
    return '<footer class="foot"><span>' + (s.updatedAt ? t('foot.updated', { t: U.hhmm(s.updatedAt) }) : t('c.loading')) + ver + '</span><span id="sysinfo" class="sysinfo">' + U.esc(sysText(s.sys)) + '</span>' + conn + night + '<span>' + nextTxt + '</span></footer>';
  }

  CD.widgets = CD.widgets || {};
  CD.widgets.holidayPill = pill;
  CD.widgets.footer = footer;
  CD.widgets.sysText = sysText;
})(window.CD = window.CD || {});
