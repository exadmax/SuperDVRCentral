(function (CD) {
  const I = CD.icons, esc = CD.util.esc, t = CD.i18n.t;

  // Janela de formulário dentro da aba. Sem <dialog>/confirm() nativos: nem todo navegador de TV os suporta.
  //   opt.title, opt.body (HTML dos campos), opt.submit (texto do botão),
  //   opt.read(form) -> dados (ou lança Error com a mensagem de validação), opt.save(dados) -> Promise (erro aparece no formulário)
  function modal(host, opt) {
    const label = opt.submit || t('c.save');
    host.innerHTML = '<div class="modal"><form class="dlg" novalidate>' +
      '<div class="dlg-h"><h2>' + esc(opt.title) + '</h2><button type="button" class="ib" data-close="1" aria-label="' + esc(t('c.close')) + '">' + I.svg('x', 'currentColor', 30, 2) + '</button></div>' +
      opt.body + '<div class="dlg-err" role="alert"></div>' +
      '<div class="dlg-f"><button type="button" class="btn ghost" data-close="1">' + t('c.cancel') + '</button><button type="submit" class="btn">' + esc(label) + '</button></div></form></div>';
    const back = host.firstChild, form = back.firstChild, err = form.querySelector('.dlg-err'), save = form.querySelector('[type=submit]');

    function close() {
      document.removeEventListener('keydown', onKey, true);
      if (CD.ui.current === api) CD.ui.current = null;
      host.innerHTML = '';
    }
    // Esc (teclado) e Voltar (controle Samsung, 10009) fecham a janela em vez de sair da aba
    function onKey(e) { if (e.keyCode === 27 || e.keyCode === 10009) { e.stopPropagation(); close(); } }
    const api = { close: close };

    document.addEventListener('keydown', onKey, true);
    back.addEventListener('mousedown', e => { if (e.target === back) close(); });
    form.addEventListener('click', e => { if (e.target.closest && e.target.closest('[data-close]')) close(); });
    form.addEventListener('submit', async e => {
      e.preventDefault();
      err.textContent = '';
      let data;
      try { data = opt.read(form); } catch (x) { err.textContent = x.message; return; }
      save.disabled = true; save.textContent = t('c.saving');
      try { await opt.save(data); close(); }
      catch (x) { err.textContent = x.message; save.disabled = false; save.textContent = label; }
    });
    CD.ui.current = api;
    const first = form.querySelector('input:not([type=radio]),textarea');
    if (first) first.focus();
    return api;
  }

  const field = (label, control, hint) => '<label class="fld"><span>' + esc(label) + (hint ? ' <em>' + esc(hint) + '</em>' : '') + '</span>' + control + '</label>';

  CD.ui = { modal: modal, field: field, current: null };
})(window.CD = window.CD || {});
