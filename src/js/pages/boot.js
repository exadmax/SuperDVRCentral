(function (CD) {
  // Página avulsa (estoque.html, contatos.html), no mesmo molde da antiga página de Conquistas: cabeçalho com voltar
  // ao painel, detecta o hub e monta o módulo indicado em <body data-page="...">.
  const U = CD.util, I = CD.icons, t = CD.i18n.t;

  const cfg = U.deepMerge(JSON.parse(JSON.stringify(window.CD_SETTINGS || {})), window.CD_SETTINGS_LOCAL);
  CD.state = { cfg: cfg, hub: '' };

  async function boot() {
    const page = CD.pages[document.body.getAttribute('data-page')];
    document.getElementById('pg').innerHTML =
      '<header class="pg-top"><a class="pg-back" href="index.html" title="' + U.esc(t('pg.back')) + '">' + I.svg('back', 'currentColor', 22, 2) + '</a>' +
      '<img class="pg-logo" src="assets/img/logo-128.png" alt=""><div class="pg-title"><h1>' + U.esc(t(page.labelKey)) + '</h1>' +
      '<div class="pg-sub">Omega Diogo · ' + U.esc(t(page.subKey)) + '</div></div></header><main id="page"></main>';

    // O hub é o servidor que entregou a página 
    const base = /^https?:/.test(location.protocol) ? location.origin : '';
    if (base) { try { await U.getJson(base + '/api/health', 3000); CD.state.hub = base; } catch (e) { CD.state.hub = ''; } }
    if (CD.state.hub) { try { await CD.hubApi.settings.loadPanel(cfg); } catch (e) { /* hub antigo: vale o padrão */ } }
    document.title = t(page.labelKey) + ' · Central Diogo';
    page.mount(document.getElementById('page'));

    document.addEventListener('keydown', e => {
      if (e.keyCode === 10009) location.href = 'index.html'; // Voltar do controle Samsung (com janela aberta, ela fecha primeiro)
    });
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot); else boot();
})(window.CD = window.CD || {});
