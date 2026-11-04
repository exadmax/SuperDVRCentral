(function (CD) {
  const U = CD.util, tt = CD.i18n.t;
  const COLOR = { livre: '#33E19A', moderado: '#FFB020', congestionado: '#FF6B6B' };
  const STOPS = {
    livre: ['#33E19A', '#33E19A'],
    moderado: ['#33E19A', '#FFB020'],
    congestionado: ['#33E19A', '#FFB020', '#FF6B6B']
  };

  function leaveAt(now, arriveBy, minutes) {
    if (!arriveBy) return null;
    const p = arriveBy.split(':').map(Number);
    const arrive = new Date(now.getFullYear(), now.getMonth(), now.getDate(), p[0], p[1]);
    if (arrive <= now) return null;                       // já passou do horário de chegada
    return U.hhmm(new Date(arrive.getTime() - minutes * 60000));
  }

  function mini(id, r) {
    const p = r.demo ? null : CD.trafficApi.path(r.points);
    const d = r.demo ? r.mock : (p && p.d);
    if (!d) return '';
    const stops = STOPS[r.level], last = COLOR[r.level];
    const a = r.demo ? [24, 45] : p.a, b = r.demo ? [380, 40] : p.b;
    return '<div class="mini"><svg viewBox="0 0 400 84" width="100%" height="84" preserveAspectRatio="none" style="display:block;position:absolute;left:0;top:0">' +
      '<defs><linearGradient id="g' + id + '" x1="0" y1="0" x2="1" y2="0">' +
      stops.map((c, i) => '<stop offset="' + Math.round(i / (stops.length - 1) * 100) + '%" stop-color="' + c + '"/>').join('') +
      '</linearGradient></defs><path d="' + d + '" fill="none" stroke="url(#g' + id + ')" stroke-width="6" stroke-linecap="round" stroke-linejoin="round"/>' +
      '<circle cx="' + a[0] + '" cy="' + a[1] + '" r="6" fill="#0B1220" stroke="#4DA3FF" stroke-width="3"/>' +
      '<circle cx="' + b[0] + '" cy="' + b[1] + '" r="6" fill="' + last + '"/></svg></div>';
  }

  const PALETTE = ['#4DA3FF', '#33E19A', '#B48CFF', '#FFB020', '#FF6B6B', '#7EE7F2', '#C89BFF'];     // uma cor por pessoa, na ordem da lista
  function dotStyle(who, i) { return ' style="background:' + PALETTE[i % PALETTE.length] + '"'; }

  function title(who, label, r, i) {
    return '<div class="t-title"><span class="wdot"' + dotStyle(who, i || 0) + '></span><span>' + label + (r && r.name ? ' → ' + CD.util.esc(r.name) : '') + '</span></div>';
  }

  function card(s, who, label, r, i) {
    // Pessoa "só quando eu pedir": desligada até clicar em "Ver trânsito"; volta a desligar depois de alguns minutos
    if (r && r.onDemand && !r.off && !r.manual && Date.now() > ((s.asked || {})[who] || 0)) r = { manual: true, name: r.name };
    if (!r) return '<div class="tcard">' + title(who, label, null, i) + '<div class="sub">' + tt('tr.nodata') + '</div></div>';
    if (r.off) return '<div class="tcard">' + title(who, label, null, i) + '<div class="t-off">' + CD.util.esc(CD.i18n.srv(r.reason)) + '</div></div>';
    if (r.manual) {
      return '<div class="tcard">' + title(who, label, r, i) + '<button class="tbtn" data-traffic="' + CD.util.esc(who) + '">' +
        (s.trafficBusy ? tt('tr.querying') : tt('tr.view')) + '</button></div>';
    }
    const leave = leaveAt(s.now, r.arriveBy, r.min);
    const badge = r.delay <= 0 ? tt('tr.free') : tt('tr.delay', { n: r.delay });
    return '<div class="tcard">' + title(who, label, r, i) +
      '<div class="t-main"><span class="num t-min">' + tt('tr.min', { n: r.min }) + '</span><span class="lvl ' + r.level + '">' + badge + '</span></div>' +
      '<div class="sub sm">' + tt('tr.normal', { n: r.normal }) + (leave ? tt('tr.leave', { t: leave }) : '') + '</div>' + mini(who, r) + '</div>';
  }

  function render(s) {
    const t = s.traffic;
    if (!t) return '<section class="col"><div class="h2">' + tt('tr.title') + '</div><div class="skel" style="height:240px"></div><div class="skel" style="height:240px"></div></section>';
    const tag = t.demo ? ' <span class="tag">' + tt('tag.demo') + '</span>' : (t.stale ? ' <span class="tag warn">' + tt('tag.cache') + '</span>' : '');
    return '<section class="col"><div class="h2">' + tt('tr.title') + tag + '</div>' +
      (t.people || []).map((p, i) => card(s, p.id, CD.util.esc(p.label), p.result, i)).join('') + '</section>';
  }

  CD.widgets = CD.widgets || {};
  CD.widgets.map = render;
})(window.CD = window.CD || {});
