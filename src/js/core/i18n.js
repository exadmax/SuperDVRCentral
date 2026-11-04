(function (CD) {
  // Internacionalização: português (padrão e reserva), inglês, espanhol e chinês simplificado.
  // Cada idioma é um arquivo em js/i18n/ que se registra com CD.i18n.add(id, { chave: texto }).
  // A escolha vale só para este aparelho (fica no armazenamento do navegador); "auto" segue o idioma do navegador.
  // Scripts clássicos, sem Intl: o painel roda em navegadores de TV antigos, por isso datas e números são montados aqui.
  const LANGS = [
    { id: 'pt', name: 'Português', locale: 'pt-BR', html: 'pt-BR' },
    { id: 'en', name: 'English', locale: 'en-US', html: 'en' },
    { id: 'es', name: 'Español', locale: 'es-ES', html: 'es' },
    { id: 'zh', name: '中文', locale: 'zh-CN', html: 'zh-CN' }
  ];
  const KEY = 'cd:lang';
  const dicts = {}, helps = {};

  const known = id => LANGS.some(l => l.id === id);
  const info = id => LANGS.filter(l => l.id === id)[0] || LANGS[0];
  const pad = n => (n < 10 ? '0' : '') + n;

  function stored() {
    try { const v = localStorage.getItem(KEY); return known(v) ? v : ''; } catch (e) { return ''; }
  }
  function store(id) {
    try { if (known(id)) localStorage.setItem(KEY, id); else localStorage.removeItem(KEY); } catch (e) { /* bloqueado: vale até recarregar */ }
  }

  // Idioma do navegador (a primeira preferência que o painel tenha); sem correspondência, português
  function detect() {
    const prefs = (navigator.languages && navigator.languages.length ? navigator.languages : [navigator.language || navigator.userLanguage || '']);
    for (let i = 0; i < prefs.length; i++) {
      const id = String(prefs[i] || '').toLowerCase().slice(0, 2);
      if (known(id)) return id;
    }
    return 'pt';
  }

  // Endereço atual sem o ?lang=... (a escolha já fica guardada; senão o parâmetro desfaria uma troca feita depois)
  const noLangUrl = () => location.pathname + (location.search || '').replace(/([?&])lang=[\w-]*&?/, '$1').replace(/[?&]$/, '') + (location.hash || '');

  // ?lang=en troca e guarda o idioma deste aparelho; ?lang=auto volta a seguir o navegador
  (function fromUrl() {
    const m = /[?&]lang=([\w-]+)/.exec(location.search || '');
    if (!m) return;
    const id = m[1].toLowerCase().slice(0, 2);
    if (known(id)) store(id); else if (m[1] === 'auto') store('');
    try { history.replaceState(null, '', noLangUrl()); } catch (e) { /* sem history: setLang cuida do endereço */ }
  })();

  const cur = stored() || detect();

  function add(id, dict) { dicts[id] = Object.assign(dicts[id] || {}, dict); }
  function addHelp(id, sections) { helps[id] = sections; }

  function fill(s, p) { return p ? s.replace(/\{(\w+)\}/g, (m, k) => (p[k] == null ? m : p[k])) : s; }

  // Texto da chave no idioma atual (reserva: português; sem nada, a própria chave). {nome} é trocado pelos parâmetros.
  function t(key, p) {
    let v = dicts[cur] && dicts[cur][key];
    if (v == null) v = dicts.pt && dicts.pt[key];
    if (v == null) return key;
    return typeof v === 'string' ? fill(v, p) : v;
  }

  // Plural: chave.one para 1 e chave.other para o resto; o chinês só tem .other. {n} já vem preenchido.
  function n(key, count, p) {
    const d = dicts[cur] || {};
    const k = count === 1 && d[key + '.one'] != null ? key + '.one' : key + '.other';
    return t(k, Object.assign({ n: count }, p));
  }

  // ---------- números e datas ----------
  const num = (x, d) => Number(x).toFixed(d || 0).replace('.', t('num.dec'));

  function fmt(d, pattern) {
    return pattern.replace('YYYY', d.getFullYear()).replace('MM', pad(d.getMonth() + 1)).replace('DD', pad(d.getDate()))
      .replace('HH', pad(d.getHours())).replace('mm', pad(d.getMinutes()));
  }
  const dayName = d => t('date.days')[d.getDay()];
  const longDate = d => fill(t('date.long'), { dow: dayName(d), d: d.getDate(), month: t('date.months')[d.getMonth()] });
  const shortDate = d => fill(t('date.short'), { d: d.getDate(), mon: t('date.monthsShort')[d.getMonth()] });
  const dateNum = d => fmt(d, t('date.num'));
  const dateTime = d => fmt(d, t('date.dm')) + ' ' + fmt(d, 'HH:mm');
  // "2026-10-05" (AAAA-MM-DD, como o hub guarda a validade) no formato do idioma
  function isoDate(v) {
    const p = String(v).split('-');
    return dateNum(new Date(+p[0], +p[1] - 1, +p[2]));
  }

  // ---------- textos que o usuário pode ter editado ----------
  // Nomes de fábrica (menu, locais do estoque, agendas de exemplo) valem em qualquer idioma; se o usuário os renomeou, aparece o que ele escreveu.
  const DEFAULTS = {
    'Jogos': 'def.games', 'Geladeira': 'def.fridge', 'Freezer': 'def.freezer', 'Dispensa': 'def.pantry', 'Serviços': 'def.services',
    'Câmeras': 'def.cameras', 'Música': 'def.music', 'Vídeos': 'def.videos', 'Config': 'def.config', 'Itens e validades': 'def.fridge.sub',
    'Profissionais': 'def.services.sub', 'Ao vivo': 'def.cameras.sub', 'Abrir no VLC': 'def.videos.sub', 'Servidor': 'def.config.sub', 'Casal': 'def.couple',
    'Livros': 'def.books', 'PDF e EPUB': 'def.books.sub'
  };
  function user(text) {
    if (cur === 'pt' || !text || !DEFAULTS[text]) return text;
    return t(DEFAULTS[text]);
  }

  // Feriados: o nome vem da BrasilAPI ou do cadastro local; os conhecidos são traduzidos pelo nome sem acento/maiúscula
  const norm = s => {
    s = String(s).toLowerCase();
    if (s.normalize) s = s.normalize('NFD').replace(/[̀-ͯ]/g, '');
    return s.replace(/[^a-z0-9]+/g, ' ').trim();
  };
  const HOLIDAYS = {
    'confraternizacao universal': 'h.newyear', 'confraternizacao mundial': 'h.newyear', 'carnaval': 'h.carnival', 'sexta feira santa': 'h.goodfriday',
    'paixao de cristo': 'h.goodfriday', 'pascoa': 'h.easter', 'tiradentes': 'h.tiradentes', 'dia do trabalho': 'h.labor', 'dia do trabalhador': 'h.labor',
    'corpus christi': 'h.corpus', 'independencia do brasil': 'h.independence', 'nossa sra aparecida': 'h.aparecida', 'nossa senhora aparecida': 'h.aparecida',
    'finados': 'h.allsouls', 'proclamacao da republica': 'h.republic', 'consciencia negra': 'h.blackawareness', 'dia da consciencia negra': 'h.blackawareness',
    'natal': 'h.christmas', 'aniversario de sao paulo': 'h.spbirthday', 'revolucao constitucionalista': 'h.spconst'
  };
  function holiday(name) {
    if (cur === 'pt') return name;
    const k = HOLIDAYS[norm(name)];
    return k ? t(k) : name;
  }

  // Mensagens que o hub devolve em português (erros e avisos): as conhecidas são traduzidas; as outras aparecem como vieram.
  // Cada item: [expressão sobre o texto original, chave]; os grupos da expressão viram {1}, {2}...
  const SRV = [];
  function srvAdd(list) { list.forEach(x => SRV.push(x)); }
  function srv(msg) {
    if (cur === 'pt' || !msg) return msg;
    const s = String(msg);
    for (let i = 0; i < SRV.length; i++) {
      const m = SRV[i][0].exec(s);
      if (m) return t(SRV[i][1], { 1: m[1], 2: m[2], 3: m[3] });
    }
    return s;
  }

  const setLang = id => { store(id); const u = noLangUrl(); if (u !== location.pathname + (location.search || '') + (location.hash || '')) location.replace(u); else location.reload(); };

  document.documentElement.setAttribute('lang', info(cur).html);
  // <title data-i18n="chave"> traduz o título da aba
  function title() {
    const el = document.querySelector('title[data-i18n]');
    if (el) document.title = t(el.getAttribute('data-i18n'));
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', title); else title();

  CD.i18n = {
    LANGS: LANGS, add: add, addHelp: addHelp, t: t, n: n, num: num, fmt: fmt, dayName: dayName, longDate: longDate, shortDate: shortDate,
    dateNum: dateNum, dateTime: dateTime, isoDate: isoDate, user: user, holiday: holiday, srv: srv, srvAdd: srvAdd, title: title,
    lang: () => cur, locale: () => info(cur).locale, pref: () => stored() || 'auto', detected: detect, setLang: setLang,
    help: () => helps[cur] || helps.pt || []
  };
})(window.CD = window.CD || {});
