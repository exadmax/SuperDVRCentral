(function (CD) {
  // Regras de estoque compartilhadas pela página Geladeira e Dispensa e pelo aviso de validades no menu da tela inicial.
  const t = CD.i18n.t;
  const LOCAIS = [
    { id: 'geladeira', label: 'Geladeira', color: '#4DA3FF', icon: 'fridge' },
    { id: 'freezer', label: 'Freezer', color: '#7EE7F2', icon: 'snow' },
    { id: 'dispensa', label: 'Dispensa', color: '#C89BFF', icon: 'box' }
  ];

  // Locais vindos do hub (Config → Painel): troca a lista no mesmo array, que as páginas já usam
  function setLocais(list) {
    LOCAIS.length = 0;
    (list || []).forEach(l => LOCAIS.push({ id: l.id, label: l.label || l.id, color: l.color || '#4DA3FF', icon: l.icon || 'box' }));
  }

  const parseDate = s => { const p = String(s).split('-'); return new Date(+p[0], +p[1] - 1, +p[2]); };

  // Dias até vencer (negativo = já venceu); null quando o item não tem validade
  function daysLeft(it, now) {
    if (!it.validade) return null;
    return Math.round((parseDate(it.validade) - CD.util.startOfDay(now)) / 86400000);
  }

  // Situação da validade: lvl = 'none' | 'ok' | 'soon' | 'late', com o texto curto para mostrar
  function expiry(it, now, soonDays) {
    const d = daysLeft(it, now);
    if (d === null) return { lvl: 'none', d: null, text: t('stock.noexp') };
    if (d < 0) return { lvl: 'late', d: d, text: CD.i18n.n('stock.overdue', -d) };
    if (d === 0) return { lvl: 'soon', d: d, text: t('stock.today') };
    if (d === 1) return { lvl: 'soon', d: d, text: t('stock.tomorrow') };
    return { lvl: d <= soonDays ? 'soon' : 'ok', d: d, text: CD.i18n.n('stock.inDays', d) };
  }

  function num(n) { return String(+Number(n).toFixed(3)).replace('.', t('num.dec')); }

  function qty(it) {
    const p = [];
    if (it.quantidade != null) p.push(t('stock.units', { n: num(it.quantidade) }));
    if (it.pesoKg != null) p.push(t('stock.kg', { n: num(it.pesoKg) }));
    return p.join(' · ');
  }

  const dataBr = v => CD.i18n.isoDate(v);

  CD.stock = {
    LOCAIS: LOCAIS, setLocais: setLocais,
    local: id => LOCAIS.filter(l => l.id === id)[0] || LOCAIS[0],
    localLabel: l => CD.i18n.user(l.label),
    soonDays: () => Math.max(0, +((CD.state && CD.state.cfg.estoque && CD.state.cfg.estoque.soonDays) || 3)),
    daysLeft: daysLeft, expiry: expiry, qty: qty, num: num, dataBr: dataBr
  };
})(window.CD = window.CD || {});
