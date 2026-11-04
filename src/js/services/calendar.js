(function (CD) {
  const U = CD.util;
  const DOW = { SU: 0, MO: 1, TU: 2, WE: 3, TH: 4, FR: 5, SA: 6 };

  // ---------- iCal ----------
  function unfold(txt) { return txt.replace(/\r?\n[ \t]/g, '').split(/\r?\n/); }

  function parseDt(val) {
    const m = /^(\d{4})(\d{2})(\d{2})(?:T(\d{2})(\d{2})(\d{2})(Z)?)?$/.exec(val);
    if (!m) return null;
    if (!m[4]) return { date: new Date(+m[1], +m[2] - 1, +m[3]), allDay: true };
    if (m[7]) return { date: new Date(Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6])), allDay: false };
    // Horário com TZID: assumido no fuso do aparelho (uso doméstico, mesmo fuso).
    return { date: new Date(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6]), allDay: false };
  }

  function parseIcs(txt) {
    const events = [];
    let cur = null;
    unfold(txt).forEach(line => {
      if (line === 'BEGIN:VEVENT') { cur = { ex: [] }; return; }
      if (line === 'END:VEVENT') { if (cur && cur.start) events.push(cur); cur = null; return; }
      if (!cur) return;
      const i = line.indexOf(':');
      if (i < 0) return;
      const name = line.slice(0, i).split(';')[0].toUpperCase(), val = line.slice(i + 1);
      if (name === 'DTSTART') { const p = parseDt(val); if (p) { cur.start = p.date; cur.allDay = p.allDay; } }
      else if (name === 'DTEND') { const p = parseDt(val); if (p) cur.end = p.date; }
      else if (name === 'SUMMARY') cur.title = val.replace(/\\,/g, ',').replace(/\\;/g, ';').replace(/\\n/gi, ' ').replace(/\\\\/g, '\\');
      else if (name === 'UID') cur.uid = val;
      else if (name === 'STATUS') cur.status = val.toUpperCase();
      else if (name === 'RRULE') cur.rrule = val;
      else if (name === 'EXDATE') val.split(',').forEach(v => { const p = parseDt(v); if (p) cur.ex.push(U.dayKey(p.date)); });
      else if (name === 'RECURRENCE-ID') { const p = parseDt(val); if (p) cur.recId = U.dayKey(p.date); }
    });
    return events.filter(e => e.status !== 'CANCELLED');
  }

  // ---------- recorrência (DAILY / WEEKLY / MONTHLY / YEARLY) ----------
  function parseRule(r) {
    const o = {};
    r.split(';').forEach(kv => { const p = kv.split('='); o[p[0]] = p[1]; });
    return {
      freq: o.FREQ, interval: +(o.INTERVAL || 1), count: o.COUNT ? +o.COUNT : 0,
      until: o.UNTIL ? parseDt(o.UNTIL).date : null,
      byday: o.BYDAY ? o.BYDAY.split(',') : null, bymonthday: o.BYMONTHDAY ? +o.BYMONTHDAY.split(',')[0] : null
    };
  }
  const monday = d => { const x = U.startOfDay(d); x.setDate(x.getDate() - ((x.getDay() + 6) % 7)); return x; };

  function nthWeekdayMatch(day, spec) {         // ex.: "1MO", "-1FR"
    const m = /^(-?\d+)([A-Z]{2})$/.exec(spec);
    if (!m || DOW[m[2]] !== day.getDay()) return false;
    const n = +m[1], dom = day.getDate();
    if (n > 0) return Math.ceil(dom / 7) === n;
    const last = new Date(day.getFullYear(), day.getMonth() + 1, 0).getDate();
    return Math.ceil((last - dom + 1) / 7) === -n;
  }

  function occursOn(ev, rule, day) {
    const base = U.startOfDay(ev.start);
    if (day < base) return false;
    if (rule.until && day > rule.until) return false;
    const diff = Math.round((day - base) / 86400000);
    if (rule.freq === 'DAILY') return diff % rule.interval === 0;
    if (rule.freq === 'WEEKLY') {
      const weeks = Math.round((monday(day) - monday(base)) / (7 * 86400000));
      const days = rule.byday ? rule.byday.map(x => DOW[x.slice(-2)]) : [ev.start.getDay()];
      return weeks % rule.interval === 0 && days.indexOf(day.getDay()) >= 0;
    }
    if (rule.freq === 'MONTHLY') {
      const months = (day.getFullYear() - base.getFullYear()) * 12 + day.getMonth() - base.getMonth();
      if (months % rule.interval !== 0) return false;
      return rule.byday ? rule.byday.some(s => nthWeekdayMatch(day, s)) : day.getDate() === (rule.bymonthday || base.getDate());
    }
    if (rule.freq === 'YEARLY') {
      return (day.getFullYear() - base.getFullYear()) % rule.interval === 0 && day.getMonth() === base.getMonth() && day.getDate() === base.getDate();
    }
    return false;
  }

  function countOk(ev, rule, day) {                // COUNT: conta ocorrências até o dia
    if (!rule.count) return true;
    let n = 0;
    for (let d = U.startOfDay(ev.start); d <= day; d = U.addDays(d, 1)) {
      if (occursOn(ev, rule, d) && ++n > rule.count) return false;
    }
    return true;
  }

  // Ocorrências do evento que tocam o dia [00:00, 24:00)
  function onDay(ev, day, overridden) {
    const dayEnd = U.addDays(day, 1), dur = (ev.end || ev.start) - ev.start;
    if (!ev.rrule) {
      const end = ev.end || ev.start;
      const hit = ev.start < dayEnd && (end > day || (end.getTime() === ev.start.getTime() && ev.start >= day));
      return hit ? [{ start: ev.start, end: end }] : [];
    }
    const rule = parseRule(ev.rrule), key = U.dayKey(day);
    if (ev.ex.indexOf(key) >= 0 || (overridden && overridden[ev.uid + '|' + key])) return [];
    if (!occursOn(ev, rule, day) || !countOk(ev, rule, day)) return [];
    const s = new Date(day.getFullYear(), day.getMonth(), day.getDate(), ev.start.getHours(), ev.start.getMinutes());
    return [{ start: s, end: new Date(s.getTime() + dur) }];
  }

  // ---------- fontes ----------
  async function fetchSource(cfg, src) {
    const key = 'ics-' + src.who;
    const url = cfg.calendar.proxy ? cfg.calendar.proxy + encodeURIComponent(src.url) : src.url;
    try {
      const txt = await U.getText(url, 20000);
      if (txt.indexOf('BEGIN:VCALENDAR') < 0) throw new Error('não é iCal');
      U.cache.set(key, txt);
      return { who: src.who, events: parseIcs(txt), stale: false };
    } catch (e) {
      const c = U.cache.get(key);
      if (c) return { who: src.who, events: parseIcs(c), stale: true };
      throw e;
    }
  }

  // Via hub: um JSON com o texto iCal de cada agenda (o hub guarda as URLs secretas e resolve o CORS)
  async function loadFromHub(hub) {
    const key = 'ics-hub';
    try {
      const j = await U.getJson(hub + '/api/ics', 25000);
      const model = { demo: false, stale: j.sources.some(x => x.stale),
        sources: j.sources.map(x => ({ who: x.who, label: x.label, color: x.color, events: parseIcs(x.text), stale: !!x.stale })) };
      U.cache.set(key, j);
      return model;
    } catch (e) {
      const c = U.cache.get(key);
      if (!c) throw e;
      return { demo: false, stale: true, sources: c.sources.map(x => ({ who: x.who, label: x.label, color: x.color, events: parseIcs(x.text), stale: true })) };
    }
  }

  async function load(cfg, hub) {
    if (hub) return loadFromHub(hub);
    const srcs = (cfg.calendar.sources || []).filter(s => s.url);
    if (!srcs.length) return { demo: true, sources: [] };
    const res = await Promise.all(srcs.map(s => fetchSource(cfg, s).catch(() => ({ who: s.who, events: [], stale: true, failed: true }))));
    return { demo: false, sources: res, stale: res.some(r => r.stale) };
  }

  // ---------- categorias (uma por agenda) ----------
  const GRAD = 'linear-gradient(135deg,#4DA3FF 50%,#33E19A 50%)';
  const COLORS = { casal: GRAD, ambos: GRAD };
  const cap = w => (w || '').charAt(0).toUpperCase() + (w || '').slice(1);
  const meta = src => ({ id: src.who, label: src.label || cap(src.who), color: src.color || COLORS[src.who] || '#B48CFF' });

  const t = CD.i18n.t;
  const DEMO = [meta({ who: 'casal' }), meta({ who: 'pessoa1', label: t('demo.person', { n: 1 }), color: '#4DA3FF' }), meta({ who: 'pessoa2', label: t('demo.person', { n: 2 }), color: '#33E19A' })];
  const calendars = model => model.demo ? DEMO : model.sources.map(meta);

  // Eventos do dia; cada evento carrega a agenda de origem (cal) para o painel agrupar por categoria
  function eventsOn(model, day) {
    day = U.startOfDay(day);
    if (model.demo) return demoEvents(day);
    const out = [];
    model.sources.forEach(src => {
      const cal = meta(src), seen = {}, overridden = {};
      src.events.forEach(e => { if (e.recId) overridden[e.uid + '|' + e.recId] = 1; });
      src.events.forEach(e => {
        onDay(e, day, overridden).forEach(o => {
          const k = (e.title || '') + '|' + o.start.getTime();
          if (seen[k]) return;
          seen[k] = 1;
          out.push({ title: e.title || t('ev.untitled'), start: o.start, end: o.end, allDay: !!e.allDay, cal: cal });
        });
      });
    });
    return out.sort((a, b) => (b.allDay - a.allDay) || (a.start - b.start));
  }

  function demoEvents(day) {
    const at = (h, m) => new Date(day.getFullYear(), day.getMonth(), day.getDate(), h, m);
    const e = (h, m, title, cal) => ({ title: title, start: at(h, m), end: at(h + 1, m), allDay: false, cal: DEMO[cal] });
    return [e(9, 30, t('demo.ev1'), 1), e(11, 0, t('demo.ev2'), 1), e(14, 0, t('demo.ev3'), 2),
      e(16, 0, t('demo.ev4'), 1), e(19, 0, t('demo.ev5'), 1), e(20, 30, t('demo.ev6'), 0)];
  }

  CD.calendar = { load: load, eventsOn: eventsOn, calendars: calendars, parseIcs: parseIcs };
})(window.CD = window.CD || {});
