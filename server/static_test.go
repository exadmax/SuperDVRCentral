package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const staticTestPage = `<!doctype html><link rel="stylesheet" href="css/main.css"><link rel="icon" href="assets/img/logo.png">
<script src="js/app.js"></script><script src="config/settings.js"></script><script src="vendor/lib/lib.js"></script><script src="js/q.js?x=1"></script>
<a href="outra.html">outra</a>`

func staticTestSite(t *testing.T, ver string) (*staticHandler, string) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.html":          staticTestPage,
		"js/app.js":           strings.Repeat("console.log('ola');\n", 200),
		"css/main.css":        strings.Repeat("body{margin:0}\n", 100),
		"config/settings.js":  "window.X=1;",
		"vendor/lib/lib.js":   "var lib=1;",
		"assets/img/logo.png": "\x89PNG-fake",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return newStaticHandler(dir, func() string { return ver }), dir
}

func staticGet(h http.Handler, target string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func gunzipBody(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("corpo não é gzip: %v", err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStaticGzip(t *testing.T) {
	h, _ := staticTestSite(t, "dev")
	rec := staticGet(h, "/js/app.js", map[string]string{"Accept-Encoding": "gzip, deflate"})
	if rec.Code != 200 || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("esperava gzip, veio %d %q", rec.Code, rec.Header().Get("Content-Encoding"))
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Errorf("faltou Vary: Accept-Encoding")
	}
	if got := gunzipBody(t, rec); !strings.HasPrefix(got, "console.log('ola');") {
		t.Errorf("conteúdo descomprimido errado: %.40q", got)
	}
	if rec.Body.Len() >= 200*len("console.log('ola');\n") {
		t.Errorf("o gzip não encolheu o arquivo (%d bytes)", rec.Body.Len())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") && !strings.Contains(ct, "text/plain") {
		t.Errorf("tipo inesperado: %q", ct)
	}

	plain := staticGet(h, "/js/app.js", nil)
	if plain.Header().Get("Content-Encoding") != "" || !strings.HasPrefix(plain.Body.String(), "console.log") {
		t.Errorf("sem Accept-Encoding deveria vir o arquivo puro")
	}
	if no := staticGet(h, "/js/app.js", map[string]string{"Accept-Encoding": "gzip;q=0"}); no.Header().Get("Content-Encoding") != "" {
		t.Errorf("gzip;q=0 recusa gzip")
	}
	if png := staticGet(h, "/assets/img/logo.png", map[string]string{"Accept-Encoding": "gzip"}); png.Header().Get("Content-Encoding") != "" {
		t.Errorf("imagem não deve ser comprimida")
	}
	if miss := staticGet(h, "/nao-existe.js", map[string]string{"Accept-Encoding": "gzip"}); miss.Code != 404 {
		t.Errorf("arquivo ausente deveria dar 404, deu %d", miss.Code)
	}
}

func TestStaticHTMLVersioned(t *testing.T) {
	h, _ := staticTestSite(t, "1.2.3")
	rec := staticGet(h, "/", nil) // "/" é o index.html
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`href="css/main.css?v=1.2.3"`, `src="js/app.js?v=1.2.3"`, `href="assets/img/logo.png?v=1.2.3"`} {
		if !strings.Contains(body, want) {
			t.Errorf("faltou %s em %s", want, body)
		}
	}
	for _, keep := range []string{`src="config/settings.js"`, `src="vendor/lib/lib.js"`, `src="js/q.js?x=1"`, `href="outra.html"`} {
		if !strings.Contains(body, keep) {
			t.Errorf("não deveria reescrever %s", keep)
		}
	}
	if rec.Header().Get("Cache-Control") == staticImmutable {
		t.Errorf("a página HTML nunca pode ter cache longo")
	}

	// mesma página comprimida, com ETag próprio e 304 em seguida
	gz := staticGet(h, "/index.html", map[string]string{"Accept-Encoding": "gzip"})
	if !strings.Contains(gunzipBody(t, gz), `src="js/app.js?v=1.2.3"`) {
		t.Errorf("a versão comprimida também deve ser reescrita")
	}
	etag := gz.Header().Get("ETag")
	if !strings.HasSuffix(etag, `-gzip"`) {
		t.Fatalf("ETag do gzip deveria terminar em -gzip: %q", etag)
	}
	if again := staticGet(h, "/index.html", map[string]string{"Accept-Encoding": "gzip", "If-None-Match": etag}); again.Code != http.StatusNotModified {
		t.Errorf("com o mesmo ETag deveria dar 304, deu %d", again.Code)
	}
	if plainTag := rec.Header().Get("ETag"); plainTag == "" || plainTag == etag {
		t.Errorf("ETag puro e comprimido devem ser diferentes: %q %q", plainTag, etag)
	}
}

func TestStaticCacheHeaders(t *testing.T) {
	h, _ := staticTestSite(t, "1.2.3")
	cases := []struct {
		url       string
		immutable bool
	}{
		{"/js/app.js?v=1.2.3", true},
		{"/css/main.css?v=1.2.3", true},
		{"/assets/img/logo.png?v=1.2.3", true},
		{"/js/app.js?v=0.0.1", false},          // versão velha: revalida
		{"/js/app.js", false},                  // sem versão: revalida
		{"/config/settings.js?v=1.2.3", false}, // config/ é editada à mão
		{"/vendor/lib/lib.js?v=1.2.3", false},
	}
	for _, c := range cases {
		rec := staticGet(h, c.url, map[string]string{"Accept-Encoding": "gzip"})
		if got := rec.Header().Get("Cache-Control") == staticImmutable; got != c.immutable {
			t.Errorf("%s: cache longo=%v, esperava %v", c.url, got, c.immutable)
		}
	}
	if miss := staticGet(h, "/js/sumiu.js?v=1.2.3", nil); miss.Code != 404 || miss.Header().Get("Cache-Control") == staticImmutable {
		t.Errorf("404 não pode ter cache longo (código %d, cache %q)", miss.Code, miss.Header().Get("Cache-Control"))
	}

	dev, _ := staticTestSite(t, "dev")
	if rec := staticGet(dev, "/js/app.js?v=dev", nil); rec.Header().Get("Cache-Control") == staticImmutable {
		t.Errorf("sem versão instalada (dev) não pode ter cache longo")
	}
	if body := staticGet(dev, "/", nil).Body.String(); strings.Contains(body, "?v=") && !strings.Contains(body, "js/q.js?x=1") {
		t.Errorf("em dev a página não deve ser reescrita")
	} else if strings.Contains(body, "app.js?v=") {
		t.Errorf("em dev a página não deve ser reescrita: %s", body)
	}
}

func TestStaticInvalidatesOnChange(t *testing.T) {
	h, dir := staticTestSite(t, "1.2.3")
	hdr := map[string]string{"Accept-Encoding": "gzip"}
	if got := gunzipBody(t, staticGet(h, "/js/app.js", hdr)); !strings.Contains(got, "ola") {
		t.Fatalf("conteúdo inicial errado")
	}
	p := filepath.Join(dir, "js", "app.js")
	if err := os.WriteFile(p, []byte("var novo = 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}
	if got := gunzipBody(t, staticGet(h, "/js/app.js", hdr)); got != "var novo = 1;" {
		t.Errorf("depois de mudar o arquivo, o gzip em memória precisa ser refeito; veio %q", got)
	}
}

func TestStaticCacheLimitAndTraversal(t *testing.T) {
	h, dir := staticTestSite(t, "dev")
	hdr := map[string]string{"Accept-Encoding": "gzip"}
	big := make([]byte, staticGzipMax+10) // acima do limite: passa direto, sem gzip
	if err := os.WriteFile(filepath.Join(dir, "js", "grande.js"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := staticGet(h, "/js/grande.js", hdr); rec.Code != 200 || rec.Header().Get("Content-Encoding") != "" {
		t.Errorf("arquivo grande deveria sair puro (código %d, encoding %q)", rec.Code, rec.Header().Get("Content-Encoding"))
	}

	secret := filepath.Join(filepath.Dir(dir), "segredo.js")
	if err := os.WriteFile(secret, []byte("SEGREDO"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"/../segredo.js", "/js/../../segredo.js", "/%2e%2e/segredo.js"} {
		rec := staticGet(h, u, hdr)
		body := rec.Body.String()
		if strings.Contains(body, "SEGREDO") {
			t.Errorf("%s vazou arquivo fora da pasta", u)
		}
		if rec.Header().Get("Content-Encoding") == "gzip" && strings.Contains(gunzipBodyOrEmpty(rec), "SEGREDO") {
			t.Errorf("%s vazou arquivo fora da pasta (gzip)", u)
		}
	}

	h.mu.Lock()
	h.entries, h.size = map[string]*staticEntry{}, staticCacheMax-1 // quase no teto: o próximo arquivo faz recomeçar
	h.mu.Unlock()
	staticGet(h, "/js/app.js", hdr)
	h.mu.Lock()
	n, size := len(h.entries), h.size
	h.mu.Unlock()
	if n != 1 || size >= staticCacheMax {
		t.Errorf("ao estourar o teto o cache deveria recomeçar com 1 item pequeno (itens %d, tamanho %d)", n, size)
	}
}

func gunzipBodyOrEmpty(rec *httptest.ResponseRecorder) string {
	zr, err := gzip.NewReader(strings.NewReader(rec.Body.String()))
	if err != nil {
		return ""
	}
	b, _ := io.ReadAll(zr)
	return string(b)
}
