(function (CD) {
  const pad = n => String(n).padStart(2, '0');

  CD.util = {
    pad,
    hhmm: d => pad(d.getHours()) + ':' + pad(d.getMinutes()),
    dayKey: d => d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()),
    startOfDay: d => new Date(d.getFullYear(), d.getMonth(), d.getDate()),
    addDays: (d, n) => new Date(d.getFullYear(), d.getMonth(), d.getDate() + n),
    diaSemana: d => CD.i18n.dayName(d),
    dataExtenso: d => CD.i18n.longDate(d),
    dataCurta: d => CD.i18n.shortDate(d),
    esc: s => String(s == null ? '' : s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c])),
    dot: (cal, big) => '<span class="wdot' + (big ? ' big' : '') + '" style="background:' + cal.color + '"></span>',
    deepMerge: function merge(a, b) {
      if (!b) return a;
      Object.keys(b).forEach(k => {
        if (b[k] && typeof b[k] === 'object' && !Array.isArray(b[k]) && a[k] && typeof a[k] === 'object') merge(a[k], b[k]);
        else a[k] = b[k];
      });
      return a;
    },
    cache: {
      get(k) { try { return JSON.parse(localStorage.getItem('cd:' + k)); } catch (e) { return null; } },
      set(k, v) { try { localStorage.setItem('cd:' + k, JSON.stringify(v)); } catch (e) { /* cheio/bloqueado */ } }
    },
    async fetchWithTimeout(url, ms) {
      const ctl = typeof AbortController !== 'undefined' ? new AbortController() : null;
      const t = setTimeout(() => ctl && ctl.abort(), ms || 12000);
      try {
        const r = await fetch(url, ctl ? { signal: ctl.signal } : undefined);
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r;
      } finally { clearTimeout(t); }
    },
    async getJson(url, ms) { return (await CD.util.fetchWithTimeout(url, ms)).json(); },
    async getText(url, ms) { return (await CD.util.fetchWithTimeout(url, ms)).text(); }
  };
})(window.CD = window.CD || {});
