(function (CD) {
  const U = CD.util, t = CD.i18n.t;

  function countdown(ms) {
    const m = Math.round(ms / 60000);
    if (m <= 0) return t('cd.now');
    if (m < 60) return t('cd.min', { n: m });
    const h = Math.floor(m / 60), r = m % 60;
    return r ? t('cd.hm', { h: h, r: r, m: U.pad(r) }) : t('cd.h', { h: h });
  }

  function tagHtml(cal) {
    if (cal.demo) return '<span class="tag">' + t('tag.demo') + '</span>';
    return cal.stale ? '<span class="tag warn">' + t('tag.cache') + '</span>' : '';
  }

  function row(e, now) {
    const past = !e.allDay && e.end <= now;
    return '<div class="ev' + (past ? ' past' : '') + '">' +
      (e.allDay ? '<span class="ev-time allday">' + t('ev.allday') + '</span>' : '<span class="ev-time num">' + U.hhmm(e.start) + '</span>') +
      '<span class="ev-title">' + U.esc(e.title) + '</span></div>';
  }

  // Um bloco por agenda (Casal, Pessoa 1, Pessoa 2...), na ordem configurada; agendas sem eventos ficam de fora
  function groups(s, events, now) {
    return CD.calendar.calendars(s.cal).map(c => {
      const list = events.filter(e => e.cal.id === c.id);
      if (!list.length) return '';
      return '<div class="grp"><div class="grp-h">' + U.dot(c) + '<span>' + U.esc(CD.i18n.user(c.label)) + '</span><span class="grp-n">' + list.length + '</span></div>' +
        list.map(e => row(e, now)).join('') + '</div>';
    }).join('');
  }

  function render(s) {
    const now = s.now, events = CD.calendar.eventsOn(s.cal, now);
    const next = events.find(e => !e.allDay && e.end > now);
    const rest = events.filter(e => e !== next);
    let html = '<section class="col"><div class="ag-head"><div class="h2">' + t('ag.today') + ' ' + tagHtml(s.cal) + '</div>' +
      '<div class="muted">' + CD.i18n.n('ag.events', events.length) + '</div></div>';
    if (!events.length) return html + '<div class="empty">' + t('ag.none') + '</div></section>';
    if (next) {
      html += '<div class="next"><div class="next-l"><div class="next-time num">' + U.hhmm(next.start) + '</div>' +
        '<div class="next-title">' + U.esc(next.title) + '</div>' +
        '<div class="who">' + U.dot(next.cal, true) + '<span>' + U.esc(CD.i18n.user(next.cal.label)) + '</span></div></div>' +
        '<div class="badge">' + countdown(next.start - now) + '</div></div>';
    }
    html += '<div class="evlist" id="evlist">' + groups(s, rest, now) + '</div></section>';
    return html;
  }

  CD.widgets = CD.widgets || {};
  CD.widgets.agenda = render;
})(window.CD = window.CD || {});
