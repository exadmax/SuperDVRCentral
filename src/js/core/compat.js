(function (CD) {
  // Modo compatibilidade: navegadores de TV (Samsung/Tizen, LG/webOS, Chromecast, etc.) costumam ter um único decodificador de
  // vídeo, então não aguentam várias câmeras ao vivo ao mesmo tempo. Nesse modo a página Câmeras mostra uma câmera por vez.
  // "auto" liga sozinho quando o navegador parece de TV; a escolha vale só para este aparelho (fica no armazenamento do navegador).
  const KEY = 'cd:compat';
  const TV = /Tizen|SMART-?TV|SmartTV|Web0S|WebOS|NetCast|HbbTV|VIDAA|Viera|BRAVIA|AquosBrowser|CrKey|AppleTV|Roku|Hisense|NetTV|Opera TV/i;

  function mode() {
    try { const m = localStorage.getItem(KEY); return m === 'on' || m === 'off' ? m : 'auto'; } catch (e) { return 'auto'; }
  }
  function setMode(m) {
    try { if (m === 'on' || m === 'off') localStorage.setItem(KEY, m); else localStorage.removeItem(KEY); } catch (e) { /* bloqueado: vale até recarregar */ }
  }
  const detected = () => TV.test(navigator.userAgent || '');
  const active = () => { const m = mode(); return m === 'on' || (m === 'auto' && detected()); };

  CD.compat = { mode: mode, setMode: setMode, detected: detected, active: active };
})(window.CD = window.CD || {});
