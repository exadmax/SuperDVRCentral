(function (CD) {
  // Cena 3D do Omega (three.js r128): um núcleo de partículas que respira conforme o estado (ouvindo, pensando, falando), anéis
  // orbitando e painéis holográficos que entram na cena com o resultado de cada pedido (câmera ao vivo, trânsito, agenda).
  // Arrastar gira a cena; clicar num painel traz para a frente (de novo: volta); Esc ou "limpar" tira todos.
  const MODE_COLOR = { idle: 0x4DA3FF, listening: 0x33E19A, thinking: 0xB48CFF, speaking: 0xFFB020, error: 0xFF6B6B };
  const MAX_PANELS = 3;
  const SLOTS = [[3.9, 0.55, 0], [-3.9, 0.55, 0], [0, -2.75, 0.8]];  // onde os painéis param (ao redor do núcleo)
  const PANEL_H = 2.5;

  const VERT = [
    'uniform float uTime; uniform float uAmp; uniform float uSize; attribute float aSeed; varying float vAlpha;',
    'void main() {',
    '  vec3 p = position;',
    '  float w = sin(aSeed * 12.0 + uTime * 1.7) * 0.5 + sin(p.y * 3.0 + uTime * 2.3) * 0.5;',
    '  p *= 1.0 + w * (0.035 + uAmp * 0.32);',
    '  vec4 mv = modelViewMatrix * vec4(p, 1.0);',
    '  gl_PointSize = uSize * (1.0 + uAmp * 0.8) * (300.0 / -mv.z);',
    '  vAlpha = 0.3 + 0.7 * fract(aSeed * 7.13);',
    '  gl_Position = projectionMatrix * mv;',
    '}'].join('\n');
  const FRAG = [
    'uniform vec3 uColor; varying float vAlpha;',
    'void main() {',
    '  float d = length(gl_PointCoord - 0.5);',
    '  if (d > 0.5) discard;',
    '  gl_FragColor = vec4(uColor, smoothstep(0.5, 0.0, d) * vAlpha);',
    '}'].join('\n');

  function create(canvas) {
    const renderer = new THREE.WebGLRenderer({ canvas: canvas, antialias: true, alpha: true });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(55, 1, 0.1, 100);
    camera.position.set(0, 0, 10);
    const world = new THREE.Group();          // o que gira ao arrastar
    scene.add(world);

    // ---------- núcleo de partículas (esfera de Fibonacci) ----------
    const N = 3600, pos = new Float32Array(N * 3), seed = new Float32Array(N);
    for (let i = 0; i < N; i++) {
      const y = 1 - (i / (N - 1)) * 2, r = Math.sqrt(1 - y * y), th = i * 2.399963;
      const k = 1.55 + (Math.random() - 0.5) * 0.12;
      pos[i * 3] = Math.cos(th) * r * k; pos[i * 3 + 1] = y * k; pos[i * 3 + 2] = Math.sin(th) * r * k;
      seed[i] = Math.random();
    }
    const coreGeo = new THREE.BufferGeometry();
    coreGeo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
    coreGeo.setAttribute('aSeed', new THREE.BufferAttribute(seed, 1));
    const uni = { uTime: { value: 0 }, uAmp: { value: 0 }, uSize: { value: 0.055 }, uColor: { value: new THREE.Color(MODE_COLOR.idle) } };
    const coreMat = new THREE.ShaderMaterial({ uniforms: uni, vertexShader: VERT, fragmentShader: FRAG, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending });
    const core = new THREE.Points(coreGeo, coreMat);
    world.add(core);

    // anéis
    const rings = [2.15, 2.55, 2.95].map((r, i) => {
      const g = new THREE.TorusGeometry(r, 0.006 + i * 0.002, 6, 160);
      const m = new THREE.MeshBasicMaterial({ color: MODE_COLOR.idle, transparent: true, opacity: 0.45 - i * 0.1 });
      const mesh = new THREE.Mesh(g, m);
      mesh.rotation.set(Math.PI / 2 + (i - 1) * 0.5, i * 0.7, 0);
      world.add(mesh);
      return mesh;
    });

    // poeira de fundo
    const dustGeo = new THREE.BufferGeometry(), D = 700, dpos = new Float32Array(D * 3);
    for (let i = 0; i < D * 3; i++) dpos[i] = (Math.random() - 0.5) * 40;
    dustGeo.setAttribute('position', new THREE.BufferAttribute(dpos, 3));
    const dust = new THREE.Points(dustGeo, new THREE.PointsMaterial({ color: 0x4DA3FF, size: 0.04, transparent: true, opacity: 0.35, depthWrite: false }));
    scene.add(dust);

    // ---------- estado ----------
    let mode = 'idle', level = 0, amp = 0, target = new THREE.Color(MODE_COLOR.idle);
    let yaw = 0, pitch = 0, vyaw = 0.0015, vpitch = 0, dragging = false, lastX = 0, lastY = 0, moved = 0;
    const panels = [];
    let focused = null, raf = 0, t0 = performance.now(), alive = true;

    function setMode(m) { mode = MODE_COLOR[m] !== undefined ? m : 'idle'; target.setHex(MODE_COLOR[mode]); }
    function setLevel(v) { level = Math.max(0, Math.min(1, v || 0)); }

    // ---------- painéis ----------
    function frameLines(w, h, color) {
      const c = 0.22, x = w / 2, y = h / 2, v = [];
      [[-x, y, 1, -1], [x, y, -1, -1], [x, -y, -1, 1], [-x, -y, 1, 1]].forEach(q => {   // cantoneiras
        v.push(q[0], q[1], 0, q[0] + q[2] * c, q[1], 0, q[0], q[1], 0, q[0], q[1] + q[3] * c, 0);
      });
      const g = new THREE.BufferGeometry();
      g.setAttribute('position', new THREE.Float32BufferAttribute(v, 3));
      return new THREE.LineSegments(g, new THREE.LineBasicMaterial({ color: color, transparent: true, opacity: 0.95 }));
    }

    function titleTexture(text, color) {
      const cv = document.createElement('canvas'); cv.width = 1024; cv.height = 96;
      const x = cv.getContext('2d');
      x.font = '600 52px Poppins, Inter, sans-serif'; x.fillStyle = color; x.textBaseline = 'middle';
      x.fillText(String(text).toUpperCase(), 8, 50);
      const tex = new THREE.CanvasTexture(cv); tex.minFilter = THREE.LinearFilter;
      return tex;
    }

    // add({kind, title, texture, aspect, color, onDispose}) → painel; a textura pode ser de vídeo ou de canvas
    function add(opt) {
      const color = opt.color || '#4DA3FF', h = PANEL_H, w = h * (opt.aspect || 16 / 9);
      const g = new THREE.Group();
      const back = new THREE.Mesh(new THREE.PlaneGeometry(w + 0.12, h + 0.12), new THREE.MeshBasicMaterial({ color: 0x06121f, transparent: true, opacity: 0.55, depthWrite: false }));
      back.position.z = -0.01;
      const face = new THREE.Mesh(new THREE.PlaneGeometry(w, h), new THREE.MeshBasicMaterial({ map: opt.texture, transparent: true, opacity: 0, depthWrite: false }));
      const fr = frameLines(w + 0.12, h + 0.12, color);
      const tt = new THREE.Mesh(new THREE.PlaneGeometry(w, w * 96 / 1024), new THREE.MeshBasicMaterial({ map: titleTexture(opt.title || '', color), transparent: true, depthWrite: false }));
      tt.position.set(0, h / 2 + 0.06 + w * 48 / 1024, 0);
      g.add(back, face, fr, tt);
      g.position.set(0, 0, -8); g.scale.setScalar(0.01);
      const p = { kind: opt.kind, key: opt.key || '', group: g, face: face, title: tt, color: color, born: performance.now(), onDispose: opt.onDispose,
        retitle(text) { const old = tt.material.map; tt.material.map = titleTexture(text, color); tt.material.needsUpdate = true; old.dispose(); } };
      g.userData.panel = p;
      scene.add(g);
      // mesmo assunto de novo (ex.: a mesma câmera) substitui o painel antigo
      if (p.key) panels.filter(x => x.key === p.key).forEach(remove);
      panels.unshift(p);
      while (panels.length > MAX_PANELS) remove(panels[panels.length - 1]);
      focused = null;
      layout();
      return p;
    }

    function layout() { panels.forEach((p, i) => { p.slot = SLOTS[i] || SLOTS[SLOTS.length - 1]; }); }

    function remove(p) {
      const i = panels.indexOf(p);
      if (i >= 0) panels.splice(i, 1);
      if (focused === p) focused = null;
      scene.remove(p.group);
      p.group.traverse(o => {
        if (o.geometry) o.geometry.dispose();
        if (o.material) { if (o.material.map) o.material.map.dispose(); o.material.dispose(); }
      });
      if (p.onDispose) try { p.onDispose(); } catch (e) { /* já encerrado */ }
      layout();
    }

    function clear() { panels.slice().forEach(remove); }

    // ---------- interação ----------
    const ray = new THREE.Raycaster(), ndc = new THREE.Vector2();
    function pick(ev) {
      const r = canvas.getBoundingClientRect();
      ndc.set(((ev.clientX - r.left) / r.width) * 2 - 1, -((ev.clientY - r.top) / r.height) * 2 + 1);
      ray.setFromCamera(ndc, camera);
      const hit = ray.intersectObjects(panels.map(p => p.face), false)[0];
      return hit ? hit.object.parent.userData.panel : null;
    }
    canvas.addEventListener('pointerdown', e => { dragging = true; moved = 0; lastX = e.clientX; lastY = e.clientY; canvas.setPointerCapture(e.pointerId); });
    canvas.addEventListener('pointermove', e => {
      if (!dragging) return;
      const dx = e.clientX - lastX, dy = e.clientY - lastY;
      moved += Math.abs(dx) + Math.abs(dy);
      vyaw = dx * 0.004; vpitch = dy * 0.003;
      lastX = e.clientX; lastY = e.clientY;
    });
    canvas.addEventListener('pointerup', e => {
      dragging = false;
      if (moved < 6) { const p = pick(e); focused = p && p !== focused ? p : null; }
    });

    // ---------- laço ----------
    function resize() {
      const w = canvas.clientWidth || window.innerWidth, h = canvas.clientHeight || window.innerHeight;
      renderer.setSize(w, h, false);
      camera.aspect = w / h;
      // telas estreitas: afasta a câmera para os painéis laterais caberem
      camera.position.z = w / h < 1.2 ? 15 : 10;
      camera.updateProjectionMatrix();
    }
    window.addEventListener('resize', resize);
    resize();

    const tmp = new THREE.Vector3();
    function frame(now) {
      if (!alive) return;
      raf = requestAnimationFrame(frame);
      if (document.hidden) return;
      const t = (now - t0) / 1000;
      // amplitude: nível do microfone ao ouvir, oscilação ao falar, giro ao pensar
      const want = mode === 'listening' ? 0.15 + level * 1.4 : mode === 'speaking' ? 0.35 + Math.abs(Math.sin(t * 9)) * 0.45 * (0.6 + level) :
        mode === 'thinking' ? 0.25 : 0.05 + level * 0.3;
      amp += (want - amp) * 0.12;
      uni.uTime.value = t; uni.uAmp.value = amp;
      uni.uColor.value.lerp(target, 0.06);
      rings.forEach((r, i) => { r.material.color.lerp(target, 0.05); r.rotation.z += (0.002 + i * 0.0015) * (mode === 'thinking' ? 4 : 1); });
      core.rotation.y += mode === 'thinking' ? 0.02 : 0.0035;
      if (!dragging) { vyaw += (0.0012 - vyaw) * 0.02; vpitch *= 0.92; }
      yaw += vyaw; pitch = Math.max(-0.5, Math.min(0.5, pitch + vpitch));
      world.rotation.set(pitch, yaw, 0);
      dust.rotation.y = yaw * 0.2;
      core.material.opacity = 1;

      panels.forEach((p, i) => {
        const f = focused === p;
        const s = f ? [0, 0, 3.4] : p.slot;
        tmp.set(s[0], s[1] + Math.sin(t * 0.8 + i) * 0.06, s[2]);
        p.group.position.lerp(tmp, 0.08);
        const sc = f ? 1.55 : (focused ? 0.75 : 1);
        p.group.scale.lerp(tmp.set(sc, sc, sc), 0.1);
        p.group.lookAt(camera.position);
        const age = (now - p.born) / 1000;
        p.face.material.opacity = Math.min(1, age * 1.6) * (age < 0.7 ? 0.55 + Math.random() * 0.45 : 1); // tremida holográfica ao surgir
      });
      renderer.render(scene, camera);
    }
    raf = requestAnimationFrame(frame);

    function dispose() {
      alive = false; cancelAnimationFrame(raf); clear();
      window.removeEventListener('resize', resize);
      renderer.dispose();
    }

    return { setMode: setMode, setLevel: setLevel, add: add, remove: remove, clear: clear, dispose: dispose, panels: () => panels.slice() };
  }

  // Textura de vídeo para os painéis (câmera ao vivo, webcam)
  function videoTexture(video) {
    const t = new THREE.VideoTexture(video);
    t.minFilter = THREE.LinearFilter; t.magFilter = THREE.LinearFilter;
    return t;
  }

  // Textura desenhada num canvas 2D; draw(ctx, w, h) desenha; devolve {texture, redraw}
  function canvasTexture(w, h, draw) {
    const cv = document.createElement('canvas'); cv.width = w; cv.height = h;
    const ctx = cv.getContext('2d');
    const tex = new THREE.CanvasTexture(cv); tex.minFilter = THREE.LinearFilter;
    function redraw(fn) { ctx.clearRect(0, 0, w, h); (fn || draw)(ctx, w, h); tex.needsUpdate = true; }
    redraw();
    return { texture: tex, redraw: redraw, canvas: cv };
  }

  CD.omegaScene = { create: create, videoTexture: videoTexture, canvasTexture: canvasTexture,
    supported: () => { try { const c = document.createElement('canvas'); return !!(window.WebGLRenderingContext && (c.getContext('webgl') || c.getContext('experimental-webgl'))); } catch (e) { return false; } } };
})(window.CD = window.CD || {});
