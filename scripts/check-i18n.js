#!/usr/bin/env node
// Confere as traduções do painel (src/js/i18n/): rode depois de acrescentar ou mudar um texto.
//   node scripts/check-i18n.js
// Verifica: (1) pt, en, es e zh têm as mesmas chaves (chinês só precisa da forma .other dos plurais); (2) os {parâmetros}
// de cada texto batem entre os idiomas; (3) toda chave usada no código (t('...'), n('...')) existe; (4) o manual da Config
// (help.*.js) tem as mesmas seções e a mesma estrutura de tabelas/listas nos quatro idiomas. Sai com código 1 se algo falhar.
const fs = require('fs'), path = require('path'), vm = require('vm');

const SRC = path.join(__dirname, '..', 'src');
const LANGS = ['pt', 'en', 'es', 'zh'];
const read = f => fs.readFileSync(path.join(SRC, f), 'utf8');
let bad = 0;
const fail = m => { bad++; console.log('ERRO: ' + m); };

// Carrega o núcleo + dicionários + manual como o navegador faria e devolve os dicionários internos
const sb = { navigator: { language: 'pt-BR' }, location: { search: '', pathname: '/', hash: '' }, localStorage: { getItem() { return null; }, setItem() {}, removeItem() {} },
  document: { documentElement: { setAttribute() {} }, readyState: 'complete', querySelector() { return null; }, addEventListener() {} } };
sb.window = sb;
vm.createContext(sb);
vm.runInContext(read('js/core/i18n.js').replace('CD.i18n = {', 'CD.i18n = { __dicts: dicts, __helps: helps,'), sb);
LANGS.forEach(l => vm.runInContext(read('js/i18n/' + l + '.js'), sb));
vm.runInContext(read('js/i18n/srv.js'), sb);
LANGS.forEach(l => vm.runInContext(read('js/i18n/help.' + l + '.js'), sb));
const D = sb.CD.i18n.__dicts, H = sb.CD.i18n.__helps;

// ---- 1. mesmas chaves ----
const fam = k => k.replace(/\.(one|other)$/, '');
const families = d => new Set(Object.keys(d).map(fam));
const ref = families(D.pt);
LANGS.slice(1).forEach(l => {
  const s = families(D[l]);
  ref.forEach(k => { if (!s.has(k)) fail(l + ' sem a chave ' + k); });
  s.forEach(k => { if (!ref.has(k)) fail(l + ' tem chave que o pt não tem: ' + k); });
  Object.keys(D.pt).forEach(k => {
    if (/\.one$/.test(k) && l !== 'zh' && (D[l][k] == null || D[l][k.replace(/\.one$/, '.other')] == null)) fail(l + ' plural incompleto: ' + fam(k));
  });
});
['date.days', 'date.daysShort', 'date.months', 'date.monthsShort', 'con.types'].forEach(k => LANGS.forEach(l => {
  if (!Array.isArray(D[l][k]) || D[l][k].length !== D.pt[k].length) fail(l + ': ' + k + ' deveria ter ' + D.pt[k].length + ' itens');
}));

// ---- 2. {parâmetros} iguais (o chinês pode escrever a hora em outra forma: cd.hm usa {r}) ----
const ph = s => (typeof s === 'string' ? (s.match(/\{\w+\}/g) || []).sort().join(',') : '');
Object.keys(D.pt).forEach(k => LANGS.slice(1).forEach(l => {
  const v = D[l][k];
  if (v == null || (l === 'zh' && (/\.one$/.test(k) || k === 'cd.hm'))) return;
  if (ph(D.pt[k]) !== ph(v)) fail('parâmetros de ' + k + ' (' + l + '): pt=' + ph(D.pt[k]) + ' ' + l + '=' + ph(v));
}));

// ---- 3. chaves usadas no código ----
function walk(dir, out) {
  fs.readdirSync(dir, { withFileTypes: true }).forEach(e => {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out); else if (/\.js$/.test(e.name) && !/[\\/]i18n[\\/]/.test(p)) out.push(p);
  });
  return out;
}
const used = new Set(), dyn = new Set();
walk(path.join(SRC, 'js'), []).forEach(f => {
  const s = fs.readFileSync(f, 'utf8');
  for (const m of s.matchAll(/\b(?:t|tt|i18n\.t)\('([^']+)'(\s*\+)?/g)) (m[2] ? dyn : used).add(m[1]);
  for (const m of s.matchAll(/\.n\('([^']+)'/g)) used.add(m[1] + '.other');
});
used.forEach(k => { if (D.pt[k] == null) fail('chave usada no código e ausente no pt: ' + k); });
// chaves montadas com + (prefixo + valor): precisam estar listadas aqui
const DYNAMIC = {
  'wx.period.': ['morning', 'afternoon', 'night'], 'cfg.tab.': ['servidor', 'cameras', 'agendas', 'painel', 'menu', 'integracoes', 'idioma', 'compat', 'ajuda'],
  'ico.': ['gamepad', 'fridge', 'phone', 'camera', 'music', 'film', 'folder', 'gear', 'home', 'link', 'box', 'clock', 'mail', 'user', 'pin', 'trophy', 'umbrella', 'search'],
  'hol.type.': ['municipal', 'estadual', 'nacional'], 'pill.holiday.': ['municipal', 'estadual', 'nacional']
};
dyn.forEach(p => { if (!DYNAMIC[p]) fail('chave dinâmica sem lista de conferência (scripts/check-i18n.js): ' + p); else DYNAMIC[p].forEach(x => { if (D.pt[p + x] == null) fail('falta ' + p + x); }); });
for (const m of read('js/core/i18n.js').matchAll(/'((?:def|h)\.[a-z.]+)'/g)) if (D.pt[m[1]] == null) fail('i18n.js usa ' + m[1] + ' sem tradução');

// ---- 4. manual da Config ----
const TAGS = ['<tr>', '<li>', '<h3>', '<code>', '<table', '<ul>', '<ol>', '<p>', '<b>', '<i>', '<div', '<span'];
const count = (h, t) => h.split(t).length - 1;
LANGS.slice(1).forEach(l => {
  if (H[l].length !== H.pt.length) fail('help.' + l + ' tem ' + H[l].length + ' seções; o pt tem ' + H.pt.length);
  H.pt.forEach((s, i) => {
    const o = H[l][i];
    if (!o || o.id !== s.id) { fail('help.' + l + ': seção ' + i + ' deveria ser "' + s.id + '"'); return; }
    TAGS.forEach(t => { if (count(s.html, t) !== count(o.html, t)) fail('help.' + l + ' / ' + s.id + ': ' + t + ' pt=' + count(s.html, t) + ' ' + l + '=' + count(o.html, t)); });
  });
});

console.log(bad ? bad + ' problema(s).' : 'OK: ' + Object.keys(D.pt).length + ' chaves e ' + H.pt.length + ' seções de ajuda consistentes em ' + LANGS.join(', ') + '.');
process.exit(bad ? 1 : 0);
