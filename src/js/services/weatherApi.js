(function (CD) {
  // Open-Meteo: gratuito, sem chave.
  const PERIODS = [
    { id: 'morning', from: 6, to: 11, at: 9 },
    { id: 'afternoon', from: 12, to: 17, at: 15 },
    { id: 'night', from: 18, to: 23, at: 21 }
  ];
  const r = n => Math.round(n);

  async function fetchWeather(cfg) {
    const L = cfg.location;
    const url = 'https://api.open-meteo.com/v1/forecast?latitude=' + L.lat + '&longitude=' + L.lon +
      '&timezone=' + encodeURIComponent(L.timezone) + '&forecast_days=2' +
      '&current=temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,is_day' +
      '&hourly=temperature_2m,precipitation_probability,weather_code' +
      '&daily=temperature_2m_max,temperature_2m_min,sunrise,sunset';
    return parse(await CD.util.getJson(url), new Date());
  }

  function parse(j, now) {
    const h = j.hourly, d = j.daily, c = j.current;
    const hourNow = now.getHours();
    const periods = PERIODS.map(p => {
      let pop = 0;
      for (let i = p.from; i <= p.to; i++) pop = Math.max(pop, h.precipitation_probability[i] || 0);
      return { id: p.id, temp: r(h.temperature_2m[p.at]), pop: pop, code: h.weather_code[p.at] };
    });
    // Primeira hora (de agora até 22h) com chance de chuva >= 60%
    let rain = null;
    for (let i = Math.max(hourNow, 6); i <= 22; i++) {
      if ((h.precipitation_probability[i] || 0) >= 60) { rain = { hour: i, pop: h.precipitation_probability[i] }; break; }
    }
    return {
      now: { temp: r(c.temperature_2m), feels: r(c.apparent_temperature), humidity: r(c.relative_humidity_2m), code: c.weather_code, isDay: !!c.is_day },
      min: r(d.temperature_2m_min[0]), max: r(d.temperature_2m_max[0]),
      sunrise: d.sunrise[0].slice(11, 16), sunset: d.sunset[0].slice(11, 16),
      tomorrow: { min: r(d.temperature_2m_min[1]), max: r(d.temperature_2m_max[1]) },
      periods: periods, rain: rain
    };
  }

  CD.weatherApi = { fetch: fetchWeather, parse: parse };
})(window.CD = window.CD || {});
