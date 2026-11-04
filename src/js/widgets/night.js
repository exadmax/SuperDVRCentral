(function (CD) {
  const U = CD.util, I = CD.icons, t = CD.i18n.t;

  function render(s) {
    const w = s.weather;
    const tomorrow = CD.calendar.eventsOn(s.cal, U.addDays(s.now, 1)).slice(0, 4);
    const line = w
      ? I.weather(w.now.code, w.now.isDay, 'rgba(77,163,255,.55)', 40, 1.6) +
        '<div class="n-wx">' + t('night.wx', { t: w.now.temp, min: w.tomorrow.min, max: w.tomorrow.max }) + '</div>'
      : '';
    const list = tomorrow.length
      ? tomorrow.map(e => '<div class="n-ev"><span class="n-t num">' + (e.allDay ? t('night.allday') : U.hhmm(e.start)) + '</span>' +
        '<span class="n-dot" style="background:' + e.cal.color + '"></span><span class="n-title">' + U.esc(e.title) + '</span></div>').join('')
      : '<div class="n-ev"><span class="n-title" style="text-align:center;width:100%">' + t('night.none') + '</span></div>';

    return '<div class="night-wrap"><img class="n-logo" src="assets/img/logo-128.png" alt="">' +
      '<button type="button" class="n-off" data-night="off">' + t('night.off') + '</button>' +
      '<div class="n-col"><div class="n-clock"><div class="n-time num">' + U.hhmm(s.now) + '</div>' +
      '<div class="n-date">' + U.dataExtenso(s.now) + '</div></div>' +
      '<div class="n-line">' + line + '</div><div class="n-rule"></div>' +
      '<div class="n-tom"><div class="n-tom-h">' + t('night.tomorrow') + '</div>' + list + '</div></div></div>';
  }

  CD.widgets = CD.widgets || {};
  CD.widgets.night = render;
})(window.CD = window.CD || {});
