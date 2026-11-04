(function (CD) {
  // Caminho do mini-mapa: pontos lat/lon -> "M x,y L x,y" num viewBox 400x84
  function path(points) {
    if (!points || points.length < 2) return null;
    const step = Math.max(1, Math.floor(points.length / 70));
    const pts = points.filter((_, i) => i % step === 0 || i === points.length - 1);
    const lats = pts.map(p => p[0]), lons = pts.map(p => p[1]);
    const minLa = Math.min.apply(null, lats), maxLa = Math.max.apply(null, lats);
    const minLo = Math.min.apply(null, lons), maxLo = Math.max.apply(null, lons);
    const W = 352, H = 52, dLa = (maxLa - minLa) || 1e-6, dLo = (maxLo - minLo) || 1e-6;
    const k = Math.min(W / dLo, H / dLa);           // mantém proporção
    const ox = 24 + (W - dLo * k) / 2, oy = 16 + (H - dLa * k) / 2;
    const xy = pts.map(p => [ox + (p[1] - minLo) * k, oy + (maxLa - p[0]) * k]);
    return { d: xy.map((p, i) => (i ? 'L' : 'M') + p[0].toFixed(1) + ',' + p[1].toFixed(1)).join(' '), a: xy[0], b: xy[xy.length - 1] };
  }

  // Exemplo (do design) quando não há chave/coordenadas
  function demo() {
    const mk = (min, normal, level, d) => ({ min: min, normal: normal, delay: min - normal, level: level, demo: true, mock: d });
    return {
      p1: mk(42, 32, 'moderado', 'M24,60 C90,60 90,22 170,22 S260,50 380,40'),
      p2: mk(28, 25, 'livre', 'M24,30 C100,30 110,58 190,58 S280,26 380,34')
    };
  }

  CD.trafficApi = { path: path, demo: demo };
})(window.CD = window.CD || {});
