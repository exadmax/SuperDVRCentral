package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testEpub(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	add("mimetype", "application/epub+zip")
	add("META-INF/container.xml", `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)
	add("OEBPS/content.opf", `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="2.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Livro de Teste</dc:title></metadata>
<manifest><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="c1" href="cap%201.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="text/cap2.xhtml" media-type="application/xhtml+xml"/><item id="js" href="x.js" media-type="text/javascript"/></manifest>
<spine toc="ncx"><itemref idref="c1"/><itemref idref="c2"/></spine></package>`)
	add("OEBPS/toc.ncx", `<?xml version="1.0"?><ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"><navMap><navPoint id="a"><navLabel><text>Primeiro</text></navLabel><content src="cap%201.xhtml"/><navPoint id="b"><navLabel><text>Segundo</text></navLabel><content src="text/cap2.xhtml#sec"/></navPoint></navPoint></navMap></ncx>`)
	add("OEBPS/cap 1.xhtml", "<html><body><p>um</p></body></html>")
	add("OEBPS/text/cap2.xhtml", "<html><body><p>dois</p></body></html>")
	add("OEBPS/x.js", "alert(1)")
	zw.Close()
	return buf.Bytes()
}

func TestCleanBookName(t *testing.T) {
	cases := map[string]string{
		"Dom Casmurro.PDF":     "Dom Casmurro.pdf",
		"../../etc/passwd.pdf": ".._.._etc_passwd.pdf",
		"a:b*c?.epub":          "a_b_c_.epub",
		"foto.jpg":             "",
		"  .pdf":               "",
		".oculto.pdf":          "oculto.pdf",
	}
	for in, want := range cases {
		got := cleanBookName(in)
		if want == ".._.._etc_passwd.pdf" { // só importa que não sobre separador
			if strings.ContainsAny(got, `/\`) || got == "" {
				t.Errorf("cleanBookName(%q) = %q", in, got)
			}
			continue
		}
		if got != want {
			t.Errorf("cleanBookName(%q) = %q, queria %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "a/b.pdf", `a\b.pdf`, ".x.pdf", "x.txt", "../x.pdf"} {
		if bookNameOK(bad) {
			t.Errorf("bookNameOK(%q) deveria ser falso", bad)
		}
	}
}

func TestBooksAPI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Livros") // a pasta ainda não existe
	mux := http.NewServeMux()
	on := true
	newBooks(BooksConfig{Dir: dir, MaxMB: 1}, nil, func() bool { return on }).register(mux)
	do := func(method, url string, body []byte, hdr map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, url, bytes.NewReader(body))
		for k, v := range hdr {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	ok := map[string]string{"X-CD-Action": "confirm"}

	if w := do("GET", "/api/books", nil, nil); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("lista vazia: %d %q", w.Code, w.Body.String())
	}
	pdf := []byte("%PDF-1.4\nconteudo")
	if w := do("PUT", "/api/books/file/livro.pdf", pdf, nil); w.Code != http.StatusForbidden {
		t.Errorf("sem X-CD-Action deveria ser 403, veio %d", w.Code)
	}
	if w := do("PUT", "/api/books/file/livro.pdf", pdf, map[string]string{"X-CD-Action": "confirm", "Origin": "http://outro.site"}); w.Code != http.StatusForbidden {
		t.Errorf("origem de outro site deveria ser 403, veio %d", w.Code)
	}
	if w := do("PUT", "/api/books/file/livro.pdf", pdf, ok); w.Code != http.StatusCreated {
		t.Fatalf("envio: %d %s", w.Code, w.Body.String())
	}
	var it bookItem
	w := do("PUT", "/api/books/file/livro.pdf", pdf, ok) // mesmo nome de novo
	json.Unmarshal(w.Body.Bytes(), &it)
	if it.Name != "livro (2).pdf" {
		t.Errorf("nome repetido deveria virar 'livro (2).pdf', veio %q", it.Name)
	}
	if w := do("PUT", "/api/books/file/falso.pdf", []byte("isto nao e pdf"), ok); w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("PDF falso: %d", w.Code)
	}
	if w := do("PUT", "/api/books/file/falso.epub", []byte("PK\x03\x04lixo"), ok); w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("EPUB falso: %d", w.Code)
	}
	if w := do("PUT", "/api/books/file/foto.jpg", pdf, ok); w.Code != http.StatusBadRequest {
		t.Errorf("extensão errada: %d", w.Code)
	}
	if w := do("PUT", "/api/books/file/grande.pdf", append([]byte("%PDF-"), make([]byte, 2<<20)...), ok); w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("acima do limite: %d", w.Code)
	}
	epub := testEpub(t)
	if w := do("PUT", "/api/books/file/Teste.epub?category=Fic%C3%A7%C3%A3o&owner=Ana", epub, ok); w.Code != http.StatusCreated {
		t.Fatalf("envio epub: %d %s", w.Code, w.Body.String())
	} else if !strings.Contains(w.Body.String(), `"category":"Ficção"`) || !strings.Contains(w.Body.String(), `"owner":"Ana"`) {
		t.Errorf("categoria/dono não voltaram: %s", w.Body.String())
	}
	if w := do("PUT", "/api/books/file/Longo.pdf?owner="+strings.Repeat("a", 41), pdf, ok); w.Code != http.StatusBadRequest {
		t.Errorf("dono longo demais: %d", w.Code)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".envio-") || strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("sobrou arquivo temporário %s", e.Name())
		}
	}

	var list []bookItem
	json.Unmarshal(do("GET", "/api/books", nil, nil).Body.Bytes(), &list)
	if len(list) != 3 {
		t.Fatalf("esperava 3 livros, veio %+v", list)
	}
	for _, b := range list {
		if (b.Name == "Teste.epub") != (b.Category == "Ficção" && b.Owner == "Ana") {
			t.Errorf("categoria/dono errados na lista: %+v", b)
		}
	}
	// trocar categoria e dono
	pj := func(body string) *httptest.ResponseRecorder {
		return do("PATCH", "/api/books/file/livro.pdf", []byte(body), ok)
	}
	if w := pj(`{"category":"  Técnico  ","owner":"Bia"}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"category":"Técnico"`) {
		t.Errorf("patch: %d %s", w.Code, w.Body.String())
	}
	if w := do("PATCH", "/api/books/file/livro.pdf", []byte(`{}`), nil); w.Code != http.StatusForbidden {
		t.Errorf("patch sem cabeçalho: %d", w.Code)
	}
	if w := do("PATCH", "/api/books/file/nao-existe.pdf", []byte(`{}`), ok); w.Code != http.StatusNotFound {
		t.Errorf("patch de livro inexistente: %d", w.Code)
	}
	if w := pj(`{"category":""}`); w.Code != 200 {
		t.Errorf("limpar categoria: %d", w.Code)
	}

	if w := do("GET", "/api/books/file/livro.pdf", nil, nil); w.Code != 200 || w.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "inline") {
		t.Errorf("baixar pdf: %d %v", w.Code, w.Header())
	}
	if w := do("GET", "/api/books/file/..%2F..%2Fx.pdf", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("nome com caminho deveria ser 404, veio %d", w.Code)
	}

	// EPUB: sumário e arquivos de dentro
	var toc epubTOC
	w = do("GET", "/api/books/epub/Teste.epub/toc", nil, nil)
	if w.Code != 200 {
		t.Fatalf("toc: %d %s", w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &toc)
	if toc.Title != "Livro de Teste" || len(toc.Chapters) != 2 || toc.Chapters[0].Href != "OEBPS/cap 1.xhtml" || toc.Chapters[0].Title != "Primeiro" ||
		toc.Chapters[1].Href != "OEBPS/text/cap2.xhtml" || toc.Chapters[1].Title != "Segundo" {
		t.Errorf("toc inesperado: %+v", toc)
	}
	w = do("GET", "/api/books/epub/Teste.epub/x/OEBPS/cap%201.xhtml", nil, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<p>um</p>") || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") ||
		!strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Errorf("capítulo: %d %v %q", w.Code, w.Header(), w.Body.String())
	}
	for _, p := range []string{"OEBPS/x.js", "OEBPS/nao-existe.xhtml", "META-INF"} {
		if w := do("GET", "/api/books/epub/Teste.epub/x/"+p, nil, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s deveria ser 404, veio %d", p, w.Code)
		}
	}
	if w := do("GET", "/api/books/epub/livro.pdf/toc", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("toc de PDF: %d", w.Code)
	}

	// desligado na Config: nada responde
	on = false
	if w := do("GET", "/api/books", nil, nil); w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "desativada") {
		t.Errorf("lista com a página desligada: %d %s", w.Code, w.Body.String())
	}
	for _, u := range []string{"/api/books/file/livro.pdf", "/api/books/epub/Teste.epub/toc"} {
		if w := do("GET", u, nil, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s desligado deveria ser 404, veio %d", u, w.Code)
		}
	}
	if w := do("PUT", "/api/books/file/outro.pdf", pdf, ok); w.Code != http.StatusServiceUnavailable {
		t.Errorf("envio desligado: %d", w.Code)
	}
	on = true

	// exclusão
	if w := do("DELETE", "/api/books/file/livro.pdf", nil, nil); w.Code != http.StatusForbidden {
		t.Errorf("excluir sem cabeçalho: %d", w.Code)
	}
	if w := do("DELETE", "/api/books/file/livro.pdf", nil, ok); w.Code != http.StatusNoContent {
		t.Errorf("excluir: %d", w.Code)
	}
	if w := do("DELETE", "/api/books/file/livro.pdf", nil, ok); w.Code != http.StatusNotFound {
		t.Errorf("excluir de novo: %d", w.Code)
	}
	if _, ok := readBookMeta(dir)["livro.pdf"]; ok {
		t.Errorf("a categoria do livro excluído ficou no .livros.json")
	}
}
