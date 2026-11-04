(function (CD) {
  const U = CD.util;
  // Fixos nacionais: garantem o funcionamento offline. A BrasilAPI acrescenta os móveis (Carnaval, Páscoa, Corpus Christi).
  const FIXOS = [
    ['01-01', 'Confraternização Universal'], ['04-21', 'Tiradentes'], ['05-01', 'Dia do Trabalho'],
    ['09-07', 'Independência do Brasil'], ['10-12', 'Nossa Sra. Aparecida'], ['11-02', 'Finados'],
    ['11-15', 'Proclamação da República'], ['11-20', 'Consciência Negra'], ['12-25', 'Natal']
  ];

  async function nacionaisApi(year) {
    const key = 'holidays-' + year;
    const hit = U.cache.get(key);
    if (hit) return hit;
    try {
      const j = await U.getJson('https://brasilapi.com.br/api/feriados/v1/' + year, 10000);
      const list = j.map(x => ({ date: x.date, name: x.name.replace('Nossa Senhora Aparecida', 'Nossa Sra. Aparecida'), type: 'nacional' }));
      U.cache.set(key, list);
      return list;
    } catch (e) { return []; }
  }

  async function forYear(year) {
    const map = {};
    FIXOS.forEach(f => { map[year + '-' + f[0]] = { date: year + '-' + f[0], name: f[1], type: 'nacional' }; });
    (await nacionaisApi(year)).forEach(x => { map[x.date] = x; });
    ((CD.state && CD.state.cfg && CD.state.cfg.localHolidays) || window.CD_LOCAL_HOLIDAYS || []).forEach(x => {
      const date = year + '-' + x.date;
      if (!map[date]) map[date] = { date: date, name: x.name, type: x.type };
    });
    return Object.keys(map).sort().map(k => map[k]);
  }

  // Lista do ano atual + próximo (para "faltam N dias" na virada do ano)
  async function load(now) {
    const y = now.getFullYear();
    return (await forYear(y)).concat(await forYear(y + 1));
  }

  function today(list, now) { return list.find(h => h.date === U.dayKey(now)) || null; }

  function next(list, now) {
    const t = U.dayKey(now);
    const h = list.find(x => x.date > t);
    if (!h) return null;
    const p = h.date.split('-').map(Number);
    const date = new Date(p[0], p[1] - 1, p[2]);
    return { name: h.name, type: h.type, date: date, days: Math.round((date - U.startOfDay(now)) / 86400000) };
  }

  CD.holidaysApi = { load: load, today: today, next: next };
})(window.CD = window.CD || {});
