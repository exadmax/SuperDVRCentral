(function (CD) {
  // Cliente da API CRUD do hub (/api/estoque, /api/contatos). Erros viram Error com a mensagem do servidor.
  async function call(method, path, body, headers) {
    const t = CD.i18n.t, base = CD.state && CD.state.hub;
    if (!base) throw new Error(t('err.nohub'));
    const opt = { method: method };
    if (headers) opt.headers = headers;
    if (body !== undefined) { opt.headers = Object.assign({ 'Content-Type': 'application/json' }, headers); opt.body = JSON.stringify(body); }
    const ctl = typeof AbortController !== 'undefined' ? new AbortController() : null;
    if (ctl) opt.signal = ctl.signal;
    const timer = setTimeout(() => ctl && ctl.abort(), 12000);
    try {
      const r = await fetch(base + path, opt);
      if (r.status === 204) return null;
      let j = null;
      try { j = await r.json(); } catch (e) { /* corpo vazio ou não JSON */ }
      if (!r.ok) throw new Error(CD.i18n.srv(j && j.error) || t('err.http', { n: r.status }));
      return j;
    } catch (e) {
      if (e && e.name === 'AbortError') throw new Error(t('err.timeout'));
      if (e instanceof TypeError) throw new Error(t('err.offline'));
      throw e;
    } finally { clearTimeout(timer); }
  }

  function resource(path) {
    return {
      list() { return call('GET', path); },
      create(obj) { return call('POST', path, obj); },
      update(id, obj) { return call('PUT', path + '/' + id, obj); },
      remove(id) { return call('DELETE', path + '/' + id); }
    };
  }

  CD.hubApi = {
    estoque: resource('/api/estoque'), contatos: resource('/api/contatos'),
    // câmeras ao vivo: a lista vem do hub (a URL com a senha nunca sai de lá); o vídeo é um MP4 contínuo em /stream
    // Config: uso de CPU/memória e os botões que param o desktop / o hub (o cabeçalho X-CD-Action evita disparo por outra página)
    system() { return call('GET', '/api/system'); },
    power(what) { return call('POST', '/api/admin/' + what, undefined, { 'X-CD-Action': 'confirm' }); },
    // câmeras, agendas (iCal), rotas do trânsito e atalhos do menu: editados na página Config e guardados no hub (segredos ficam mascarados)
    settings: {
      // Configurações da tela inicial editadas na Config (localização, modo noturno, rodízio, validade, vídeos, feriados locais, locais do estoque):
      // mescla na configuração em uso. O que não foi editado continua com o padrão do settings.js.
      async loadPanel(cfg) {
        const j = await call('GET', '/api/settings/panel');
        ['location', 'night', 'cameras', 'estoque', 'videos', 'books'].forEach(k => { if (j[k]) cfg[k] = Object.assign(cfg[k] || {}, j[k]); });
        if (j.refreshMinutes) cfg.refreshMinutes = j.refreshMinutes;
        if (j.localHolidays) cfg.localHolidays = j.localHolidays;
        if (j.locais && CD.stock) CD.stock.setLocais(j.locais);
        return j;
      },
      restartHub() { return call('POST', '/api/settings/hub/restart', undefined, { 'X-CD-Action': 'confirm' }); },
      get(section) { return call('GET', '/api/settings/' + section); },
      save(section, obj) { return call('PUT', '/api/settings/' + section, obj, { 'X-CD-Action': 'confirm' }); }
    },
    videos: { list() { return call('GET', '/api/videos'); } },
    // livros (PDF/EPUB): lista, envio (corpo cru do arquivo, com progresso), categoria/dono, exclusão e leitor de EPUB (capítulos servidos pelo hub)
    books: {
      list() { return call('GET', '/api/books'); },
      setMeta(name, o) { return call('PATCH', '/api/books/file/' + encodeURIComponent(name), o, { 'X-CD-Action': 'confirm' }); },
      remove(name) { return call('DELETE', '/api/books/file/' + encodeURIComponent(name), undefined, { 'X-CD-Action': 'confirm' }); },
      fileUrl(name, download) { return CD.state.hub + '/api/books/file/' + encodeURIComponent(name) + (download ? '?download=1' : ''); },
      toc(name) { return call('GET', '/api/books/epub/' + encodeURIComponent(name) + '/toc'); },
      chapterUrl(name, href) { return CD.state.hub + '/api/books/epub/' + encodeURIComponent(name) + '/x/' + href.split('/').map(encodeURIComponent).join('/'); },
      upload(file, meta, onProgress) {
        const t = CD.i18n.t, base = CD.state && CD.state.hub;
        return new Promise((resolve, reject) => {
          if (!base) return reject(new Error(t('err.nohub')));
          const x = new XMLHttpRequest();
          x.open('PUT', base + '/api/books/file/' + encodeURIComponent(file.name) + '?category=' + encodeURIComponent(meta.category || '') + '&owner=' + encodeURIComponent(meta.owner || ''));
          x.setRequestHeader('X-CD-Action', 'confirm');
          x.setRequestHeader('Content-Type', 'application/octet-stream');
          if (x.upload && onProgress) x.upload.onprogress = e => { if (e.lengthComputable) onProgress(e.loaded / e.total); };
          x.onerror = () => reject(new Error(t('err.offline')));
          x.onload = () => {
            let j = null;
            try { j = JSON.parse(x.responseText); } catch (e) { /* corpo vazio ou não JSON */ }
            if (x.status >= 200 && x.status < 300) resolve(j);
            else reject(new Error(CD.i18n.srv(j && j.error) || t('err.http', { n: x.status })));
          };
          x.send(file);
        });
      }
    },
    // gravações das câmeras: arquivos de 1 hora no servidor; as configurações (liga/desliga, câmeras, retenção) ficam na página Config
    recordings: {
      list() { return call('GET', '/api/recordings'); },
      settings() { return call('GET', '/api/recordings/settings'); },
      save(obj) { return call('PUT', '/api/recordings/settings', obj, { 'X-CD-Action': 'confirm' }); },
      remove(name) { return call('DELETE', '/api/recordings/file/' + encodeURIComponent(name), undefined, { 'X-CD-Action': 'confirm' }); },
      fileUrl(name, download) { return CD.state.hub + '/api/recordings/file/' + encodeURIComponent(name) + (download ? '?download=1' : ''); },
      // cópia das gravações para o Google Drive: estado e envio imediato (ids das câmeras; vazio = todas)
      drive() { return call('GET', '/api/recordings/drive'); },
      driveRun(ids) { return call('POST', '/api/recordings/drive/run', { cameras: ids || [] }, { 'X-CD-Action': 'confirm' }); }
    },
    cameras: { list() { return call('GET', '/api/cameras'); }, streamUrl(id) { return CD.state.hub + '/api/cameras/' + encodeURIComponent(id) + '/stream'; } }
  };
})(window.CD = window.CD || {});
