// Entrega do painel (a pasta staticDir). Duas coisas, pensadas para máquina e rede lentas:
//   - gzip: texto (js, css, html, json, svg) é comprimido uma vez e guardado em memória (teto de staticCacheMax); depois só se envia o pronto.
//   - cache: as páginas .html saem com ?v=VERSÃO nos scripts e estilos, e esses endereços versionados ganham cache de um ano. Cada instalação muda
//     a versão, então o navegador baixa de novo só o que mudou de versão. Sem VERSION ("dev") nada é reescrito e tudo continua com no-cache.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	staticGzipMax   = 2 << 20 // arquivos maiores passam direto: o gzip de um .wasm de vários MB não compensa a memória
	staticCacheMax  = 8 << 20 // teto da memória usada pelas cópias comprimidas
	staticVerTTL    = 5 * time.Second
	staticImmutable = "public, max-age=31536000, immutable"
)

var staticCompressible = map[string]bool{".html": true, ".js": true, ".css": true, ".json": true, ".svg": true, ".txt": true, ".md": true, ".xml": true}

// Pastas cujo conteúdo muda só com a versão. config/ fica de fora (é editada à mão e precisa valer na hora) e vendor/ também
// (as bibliotecas de terceiros descobrem a própria pasta pelo endereço do script).
var (
	staticVersionedDirs = []string{"/js/", "/css/", "/assets/"}
	staticAssetRe       = regexp.MustCompile(`((?:src|href)=")((?:js|css|assets)/[^"?#]+)(")`)
)

type staticEntry struct {
	key   string // modificação|tamanho|versão: muda quando o arquivo ou a versão muda
	gz    []byte
	plain []byte // só nas páginas .html reescritas (o resto, sem gzip, vai direto do disco)
	etag  string
	mod   time.Time
}

func (e *staticEntry) bytes() int { return len(e.gz) + len(e.plain) }

type staticHandler struct {
	root  http.Dir
	files http.Handler
	ver   func() string

	mu      sync.Mutex
	entries map[string]*staticEntry
	size    int
	verAt   time.Time
	verVal  string
}

func newStaticHandler(dir string, ver func() string) *staticHandler {
	root := http.Dir(dir)
	return &staticHandler{root: root, files: http.FileServer(root), ver: ver, entries: map[string]*staticEntry{}}
}

// version devolve a versão instalada, relida no máximo a cada staticVerTTL (o arquivo VERSION muda a cada instalação).
func (s *staticHandler) version() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.verAt.IsZero() || time.Since(s.verAt) > staticVerTTL {
		s.verVal, s.verAt = s.ver(), time.Now()
	}
	return s.verVal
}

func (s *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		s.files.ServeHTTP(w, r)
		return
	}
	p := path.Clean("/" + r.URL.Path)
	if p == "/" {
		p = "/index.html"
	}
	ext := strings.ToLower(path.Ext(p))
	ver := s.version()
	s.cacheHeader(w, r, p, ext, ver)
	if !staticCompressible[ext] {
		s.files.ServeHTTP(w, r)
		return
	}
	rewrite := ext == ".html" && ver != "dev"
	gz := acceptsGzip(r)
	if !rewrite && !gz {
		s.files.ServeHTTP(w, r)
		return
	}
	f, err := s.root.Open(p)
	if err != nil {
		s.files.ServeHTTP(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() || st.Size() > staticGzipMax {
		s.files.ServeHTTP(w, r)
		return
	}
	e := s.entry(p, f, st, ver, rewrite)
	if e == nil {
		s.files.ServeHTTP(w, r)
		return
	}
	h := w.Header()
	h.Set("Vary", "Accept-Encoding")
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		ct = "text/plain; charset=utf-8"
	}
	h.Set("Content-Type", ct)
	if gz {
		h.Set("Content-Encoding", "gzip")
		h.Set("ETag", strings.TrimSuffix(e.etag, `"`)+`-gzip"`)
		http.ServeContent(w, r, p, e.mod, bytes.NewReader(e.gz))
		return
	}
	h.Set("ETag", e.etag)
	http.ServeContent(w, r, p, e.mod, bytes.NewReader(e.plain))
}

// cacheHeader dá cache de um ano ao endereço versionado da versão instalada. Qualquer outro fica com o no-cache que o withCORS já pôs.
func (s *staticHandler) cacheHeader(w http.ResponseWriter, r *http.Request, p, ext, ver string) {
	if ver == "dev" || ext == ".html" || r.URL.Query().Get("v") != ver {
		return
	}
	for _, d := range staticVersionedDirs {
		if strings.HasPrefix(p, d) {
			w.Header().Set("Cache-Control", staticImmutable)
			return
		}
	}
}

func (s *staticHandler) entry(p string, f io.Reader, st fs.FileInfo, ver string, rewrite bool) *staticEntry {
	key := strconv.FormatInt(st.ModTime().UnixNano(), 36) + "|" + strconv.FormatInt(st.Size(), 36) + "|" + ver
	s.mu.Lock()
	if e := s.entries[p]; e != nil && e.key == key {
		s.mu.Unlock()
		return e
	}
	s.mu.Unlock()
	raw, err := io.ReadAll(io.LimitReader(f, staticGzipMax+1))
	if err != nil || len(raw) > staticGzipMax {
		return nil
	}
	if rewrite {
		raw = staticVersioned(raw, ver)
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write(raw)
	if zw.Close() != nil {
		return nil
	}
	sum := sha1.Sum(raw)
	e := &staticEntry{key: key, gz: buf.Bytes(), etag: `"` + hex.EncodeToString(sum[:8]) + `"`, mod: st.ModTime()}
	if rewrite {
		e.plain = raw
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old := s.entries[p]; old != nil {
		s.size -= old.bytes()
		delete(s.entries, p)
	}
	if s.size+e.bytes() > staticCacheMax { // estourou o teto: recomeça (o painel inteiro comprimido cabe com folga)
		s.entries, s.size = map[string]*staticEntry{}, 0
	}
	s.entries[p] = e
	s.size += e.bytes()
	return e
}

// staticVersioned põe ?v=VERSÃO nos scripts, estilos e imagens da página (ver staticAssetRe).
func staticVersioned(html []byte, ver string) []byte {
	return staticAssetRe.ReplaceAll(html, []byte("${1}${2}?v="+ver+"${3}"))
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		tok, q, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(tok), "gzip") {
			continue
		}
		if _, v, ok := strings.Cut(strings.ReplaceAll(q, " ", ""), "q="); ok {
			if n, err := strconv.ParseFloat(v, 64); err == nil && n == 0 {
				return false
			}
		}
		return true
	}
	return false
}
