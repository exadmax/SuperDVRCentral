// Livros (PDF e EPUB): o hub guarda os arquivos numa pasta do servidor (padrão ~/Livros), aceita envio pelo painel e entrega
// cada arquivo pelo mesmo endereço do painel. O EPUB é um zip de páginas HTML: o hub o abre aqui mesmo e serve os capítulos
// um a um, assim o leitor do painel é só um <iframe> (sem biblioteca JS e sem depender da internet).
// Categoria e dono de cada livro ficam num .livros.json dentro da pasta; as listas de categorias e donos e o liga/desliga
// da página são da Config (settings_panel.go).
package main

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type BooksConfig struct {
	Dir   string `json:"dir"`   // pasta dos livros no servidor (padrão ~/Livros); vazio desliga a página
	MaxMB int    `json:"maxMB"` // tamanho máximo de um arquivo enviado (padrão 300)
}

type bookItem struct {
	Name     string `json:"name"`               // nome do arquivo
	Size     int64  `json:"size"`               // bytes
	Time     int64  `json:"time"`               // modificação, segundos Unix
	Type     string `json:"type"`               // "pdf" ou "epub"
	Category string `json:"category,omitempty"` // categoria (da lista da Config; texto livre se a lista mudou depois)
	Owner    string `json:"owner,omitempty"`    // dono do livro (da lista da Config)
}

// bookMeta é o que o hub guarda de cada livro além do arquivo.
type bookMeta struct {
	Category string `json:"category,omitempty"`
	Owner    string `json:"owner,omitempty"`
}

const (
	metaFile     = ".livros.json"
	maxEpubEntry = 50 << 20 // maior arquivo que o hub entrega de dentro de um EPUB
)

func bookType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "pdf"
	case ".epub":
		return "epub"
	}
	return ""
}

// bookNameOK: o nome vindo da URL nunca vira caminho livre (só um arquivo PDF/EPUB direto da pasta, sem ponto no começo).
func bookNameOK(name string) bool {
	return name != "" && len(name) <= 200 && filepath.Base(name) == name && !strings.ContainsAny(name, `/\`) &&
		!strings.HasPrefix(name, ".") && bookType(name) != ""
}

var badNameChars = regexp.MustCompile(`[\x00-\x1f\x7f/\\:*?"<>|]`)

// cleanBookName deixa o nome enviado seguro para virar arquivo; devolve "" se não for PDF/EPUB.
func cleanBookName(s string) string {
	s = badNameChars.ReplaceAllString(s, "_")
	s = strings.Trim(strings.TrimSpace(s), ".")
	if r := []rune(s); len(r) > 150 { // corta o nome, mantendo a extensão
		ext := filepath.Ext(s)
		s = string(r[:150-len([]rune(ext))]) + ext
	}
	if bookType(s) == "" {
		return ""
	}
	ext := filepath.Ext(s)
	return strings.TrimSpace(strings.TrimSuffix(s, ext)) + strings.ToLower(ext)
}

// cleanLabel valida o texto de uma categoria ou de um dono: até 40 caracteres, numa linha só.
func cleanLabel(s, what string) (string, error) {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > 40 {
		return "", fmt.Errorf("%s: no máximo 40 caracteres", what)
	}
	return s, nil
}

func readBookMeta(dir string) map[string]bookMeta {
	m := map[string]bookMeta{}
	if b, err := os.ReadFile(filepath.Join(dir, metaFile)); err == nil {
		if err := json.Unmarshal(b, &m); err != nil {
			log.Printf("livros: %s inválido (ignorado): %v", metaFile, err)
			m = map[string]bookMeta{}
		}
	}
	return m
}

func writeBookMeta(dir string, m map[string]bookMeta) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, metaFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, metaFile))
}

func listBooks(dir string) ([]bookItem, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []bookItem{}, nil // ainda sem nenhum envio: a pasta nasce no primeiro upload
	}
	if err != nil {
		return nil, err
	}
	meta := readBookMeta(dir)
	out := []bookItem{}
	for _, e := range entries {
		if e.IsDir() || !bookNameOK(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		mt := meta[e.Name()]
		out = append(out, bookItem{Name: e.Name(), Size: info.Size(), Time: info.ModTime().Unix(), Type: bookType(e.Name()), Category: mt.Category, Owner: mt.Owner})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time }) // mais recentes primeiro
	return out, nil
}

// uniqueBookName acrescenta " (2)", " (3)"... se já existir um livro com esse nome.
func uniqueBookName(dir, name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s (%d)%s", base, i, ext)
	}
}

// writeAllowed: gravar exige o cabeçalho X-CD-Action (um formulário de outra página não consegue enviá-lo sem preflight) e,
// se o navegador informar a origem, que seja o próprio hub ou uma origem listada em allowedOrigins.
func writeAllowed(r *http.Request, origins []string) error {
	if r.Header.Get("X-CD-Action") != "confirm" {
		return errors.New("use o cabeçalho X-CD-Action: confirm")
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	if u, err := url.Parse(origin); err == nil && strings.EqualFold(u.Host, r.Host) {
		return nil
	}
	for _, o := range origins {
		if strings.EqualFold(strings.TrimRight(o, "/"), origin) {
			return nil
		}
	}
	log.Printf("livros: gravação recusada: Origin %q diferente de Host %q (para liberar, use allowedOrigins em hub.config.json)", origin, r.Host)
	return errors.New("origem não permitida para gravar")
}

// looksLikeBook confere o começo do arquivo enviado (e, no EPUB, se abre como zip com container.xml).
func looksLikeBook(path, typ string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 1024)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if typ == "pdf" {
		return strings.Contains(string(head), "%PDF-")
	}
	if len(head) < 4 || string(head[:4]) != "PK\x03\x04" {
		return false
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if zf.Name == "META-INF/container.xml" {
			return true
		}
	}
	return false
}

type booksMgr struct {
	cfg     BooksConfig
	origins []string
	enabled func() bool // liga/desliga da Config (nil = sempre ligado)
	mu      sync.Mutex  // serializa o .livros.json e a gravação/exclusão de arquivos
}

func newBooks(cfg BooksConfig, origins []string, enabled func() bool) *booksMgr {
	if cfg.MaxMB <= 0 {
		cfg.MaxMB = 300
	}
	return &booksMgr{cfg: cfg, origins: origins, enabled: enabled}
}

func (b *booksMgr) on() bool { return b.enabled == nil || b.enabled() }

// setMeta grava a categoria e o dono de um livro (vazios apagam a entrada); quem chama já segura b.mu.
func (b *booksMgr) setMeta(name string, m bookMeta) error {
	all := readBookMeta(b.cfg.Dir)
	if m == (bookMeta{}) {
		delete(all, name)
	} else {
		all[name] = m
	}
	return writeBookMeta(b.cfg.Dir, all)
}

func (b *booksMgr) register(mux *http.ServeMux) {
	// unavailable responde com mensagem (para a tela mostrar); usable só diz se pode seguir (arquivos respondem 404 simples)
	unavailable := func(w http.ResponseWriter) bool {
		if !b.on() {
			jsonError(w, http.StatusServiceUnavailable, "A página Livros está desativada (Config > Livros).")
			return true
		}
		if b.cfg.Dir == "" {
			jsonError(w, http.StatusServiceUnavailable, "Pasta de livros não configurada no hub (campo \"books.dir\").")
			return true
		}
		return false
	}
	usable := func() bool { return b.on() && b.cfg.Dir != "" }

	mux.HandleFunc("GET /api/books", func(w http.ResponseWriter, r *http.Request) {
		if unavailable(w) {
			return
		}
		list, err := listBooks(b.cfg.Dir)
		if err != nil {
			log.Printf("livros: ler %s: %v", b.cfg.Dir, err)
			jsonError(w, http.StatusServiceUnavailable, "Não foi possível ler a pasta de livros do servidor.")
			return
		}
		writeJSON(w, list)
	})

	// Envio: corpo cru do arquivo em PUT /api/books/file/{nome}?category=...&owner=...; o nome final ganha " (2)" se já existir.
	mux.HandleFunc("PUT /api/books/file/{name}", func(w http.ResponseWriter, r *http.Request) {
		if unavailable(w) {
			return
		}
		if err := writeAllowed(r, b.origins); err != nil {
			jsonError(w, http.StatusForbidden, err.Error())
			return
		}
		name := cleanBookName(r.PathValue("name"))
		if name == "" {
			jsonError(w, http.StatusBadRequest, "Envie um arquivo PDF ou EPUB.")
			return
		}
		var meta bookMeta
		var err error
		if meta.Category, err = cleanLabel(r.URL.Query().Get("category"), "categoria"); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if meta.Owner, err = cleanLabel(r.URL.Query().Get("owner"), "dono"); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		max := int64(b.cfg.MaxMB) << 20
		if r.ContentLength > max {
			jsonError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("O arquivo passa do limite de %d MB.", b.cfg.MaxMB))
			return
		}
		rc := http.NewResponseController(w)
		rc.SetReadDeadline(time.Now().Add(30 * time.Minute)) // o painel pode estar numa rede lenta
		rc.SetWriteDeadline(time.Now().Add(30 * time.Minute))
		if err := os.MkdirAll(b.cfg.Dir, 0o755); err != nil {
			log.Printf("livros: criar %s: %v", b.cfg.Dir, err)
			jsonError(w, http.StatusInternalServerError, "Não foi possível criar a pasta de livros no servidor.")
			return
		}
		tmp, err := os.CreateTemp(b.cfg.Dir, ".envio-*")
		if err != nil {
			log.Printf("livros: arquivo temporário em %s: %v", b.cfg.Dir, err)
			jsonError(w, http.StatusInternalServerError, "Não foi possível gravar o livro no servidor.")
			return
		}
		defer os.Remove(tmp.Name()) // some se o envio falhar; depois do rename não existe mais
		_, err = io.Copy(tmp, http.MaxBytesReader(w, r.Body, max))
		tmp.Close()
		if err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				jsonError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("O arquivo passa do limite de %d MB.", b.cfg.MaxMB))
			} else {
				jsonError(w, http.StatusBadRequest, "O envio foi interrompido.")
			}
			return
		}
		if !looksLikeBook(tmp.Name(), bookType(name)) {
			jsonError(w, http.StatusUnsupportedMediaType, "O arquivo não parece ser um "+strings.ToUpper(bookType(name))+" válido.")
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		name = uniqueBookName(b.cfg.Dir, name)
		final := filepath.Join(b.cfg.Dir, name)
		if err := os.Rename(tmp.Name(), final); err != nil {
			log.Printf("livros: gravar %s: %v", name, err)
			jsonError(w, http.StatusInternalServerError, "Não foi possível gravar o livro no servidor.")
			return
		}
		os.Chmod(final, 0o644)
		info, err := os.Stat(final)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "Não foi possível gravar o livro no servidor.")
			return
		}
		if err := b.setMeta(name, meta); err != nil {
			log.Printf("livros: gravar %s: %v", metaFile, err)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(bookItem{Name: name, Size: info.Size(), Time: info.ModTime().Unix(), Type: bookType(name), Category: meta.Category, Owner: meta.Owner})
	})

	// Troca a categoria e o dono de um livro já enviado: PATCH com {"category": "...", "owner": "..."} (vazio = sem categoria/dono).
	mux.HandleFunc("PATCH /api/books/file/{name}", func(w http.ResponseWriter, r *http.Request) {
		if unavailable(w) {
			return
		}
		if err := writeAllowed(r, b.origins); err != nil {
			jsonError(w, http.StatusForbidden, err.Error())
			return
		}
		name := r.PathValue("name")
		if !bookNameOK(name) {
			http.NotFound(w, r)
			return
		}
		var in bookMeta
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<12)).Decode(&in); err != nil {
			jsonError(w, http.StatusBadRequest, "Pedido inválido.")
			return
		}
		var err error
		if in.Category, err = cleanLabel(in.Category, "categoria"); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if in.Owner, err = cleanLabel(in.Owner, "dono"); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		info, err := os.Stat(filepath.Join(b.cfg.Dir, name))
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		if err := b.setMeta(name, in); err != nil {
			log.Printf("livros: gravar %s: %v", metaFile, err)
			jsonError(w, http.StatusInternalServerError, "Não foi possível gravar a categoria e o dono no servidor.")
			return
		}
		writeJSON(w, bookItem{Name: name, Size: info.Size(), Time: info.ModTime().Unix(), Type: bookType(name), Category: in.Category, Owner: in.Owner})
	})

	mux.HandleFunc("GET /api/books/file/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !usable() || !bookNameOK(name) {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(b.cfg.Dir, name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		http.NewResponseController(w).SetWriteDeadline(time.Now().Add(30 * time.Minute))
		ct := "application/pdf"
		if bookType(name) == "epub" {
			ct = "application/epub+zip"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		disp := "inline"
		if r.URL.Query().Get("download") == "1" || bookType(name) == "epub" {
			disp = "attachment"
		}
		w.Header().Set("Content-Disposition", disp+"; filename*=UTF-8''"+url.PathEscape(name))
		http.ServeContent(w, r, name, info.ModTime(), f)
	})

	mux.HandleFunc("DELETE /api/books/file/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !usable() {
			http.NotFound(w, r)
			return
		}
		if err := writeAllowed(r, b.origins); err != nil {
			jsonError(w, http.StatusForbidden, err.Error())
			return
		}
		name := r.PathValue("name")
		if !bookNameOK(name) {
			http.NotFound(w, r)
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		if err := os.Remove(filepath.Join(b.cfg.Dir, name)); err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			log.Printf("livros: excluir %s: %v", name, err)
			jsonError(w, http.StatusInternalServerError, "Não foi possível excluir o livro no servidor.")
			return
		}
		if _, ok := readBookMeta(b.cfg.Dir)[name]; ok {
			if err := b.setMeta(name, bookMeta{}); err != nil {
				log.Printf("livros: gravar %s: %v", metaFile, err)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Leitor de EPUB: sumário (capítulos na ordem de leitura) e os arquivos de dentro do zip.
	mux.HandleFunc("GET /api/books/epub/{name}/toc", func(w http.ResponseWriter, r *http.Request) {
		zr, ok := b.openEpub(w, r)
		if !ok {
			return
		}
		defer zr.Close()
		toc, err := readEpubTOC(&zr.Reader)
		if err != nil {
			jsonError(w, http.StatusUnprocessableEntity, "Não foi possível abrir este EPUB.")
			return
		}
		writeJSON(w, toc)
	})
	mux.HandleFunc("GET /api/books/epub/{name}/x/{file...}", func(w http.ResponseWriter, r *http.Request) {
		zr, ok := b.openEpub(w, r)
		if !ok {
			return
		}
		defer zr.Close()
		serveEpubEntry(w, r, &zr.Reader, r.PathValue("file"))
	})
}

func (b *booksMgr) openEpub(w http.ResponseWriter, r *http.Request) (*zip.ReadCloser, bool) {
	name := r.PathValue("name")
	if !b.on() || b.cfg.Dir == "" || !bookNameOK(name) || bookType(name) != "epub" {
		http.NotFound(w, r)
		return nil, false
	}
	zr, err := zip.OpenReader(filepath.Join(b.cfg.Dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			jsonError(w, http.StatusUnprocessableEntity, "Não foi possível abrir este EPUB.")
		}
		return nil, false
	}
	return zr, true
}

// ---- EPUB ----

type epubChapter struct {
	Href  string `json:"href"`  // caminho dentro do zip (a página monta /api/books/epub/<nome>/x/<href>)
	Title string `json:"title"` // do sumário do livro; vazio se o livro não traz
}

type epubTOC struct {
	Title    string        `json:"title"`
	Chapters []epubChapter `json:"chapters"`
}

type opfPackage struct {
	Metadata struct {
		Title []string `xml:"title"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID    string `xml:"id,attr"`
			Href  string `xml:"href,attr"`
			Media string `xml:"media-type,attr"`
			Props string `xml:"properties,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Itemrefs []struct {
			IDRef  string `xml:"idref,attr"`
			Linear string `xml:"linear,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

type ncxPoint struct {
	Text    string `xml:"navLabel>text"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Points []ncxPoint `xml:"navPoint"`
}

func readZipFile(zr *zip.Reader, name string) ([]byte, error) {
	f, err := zr.Open(name) // zip.Reader é um fs.FS: recusa "..", "/" no começo etc.
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 8<<20))
}

// epubPath resolve um href relativo ao arquivo base (ambos dentro do zip), sem o #trecho e já decodificado.
func epubPath(baseFile, href string) string {
	if i := strings.IndexByte(href, '#'); i >= 0 {
		href = href[:i]
	}
	if u, err := url.PathUnescape(href); err == nil {
		href = u
	}
	return path.Clean(path.Join(path.Dir(baseFile), href))
}

var (
	tagRe    = regexp.MustCompile(`<[^>]*>`)
	navLinks = regexp.MustCompile(`(?is)<a\s[^>]*href\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)
)

func readEpubTOC(zr *zip.Reader) (*epubTOC, error) {
	cb, err := readZipFile(zr, "META-INF/container.xml")
	if err != nil {
		return nil, err
	}
	var cont struct {
		Roots []struct {
			Path string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal(cb, &cont); err != nil || len(cont.Roots) == 0 {
		return nil, errors.New("container.xml inválido")
	}
	opfPath := path.Clean(cont.Roots[0].Path)
	ob, err := readZipFile(zr, opfPath)
	if err != nil {
		return nil, err
	}
	var pkg opfPackage
	if err := xml.Unmarshal(ob, &pkg); err != nil {
		return nil, err
	}
	byID := map[string]string{}
	titles := map[string]string{} // caminho no zip -> título do sumário
	for _, it := range pkg.Manifest.Items {
		byID[it.ID] = epubPath(opfPath, it.Href)
	}
	for _, it := range pkg.Manifest.Items {
		file := epubPath(opfPath, it.Href)
		switch {
		case it.Media == "application/x-dtbncx+xml":
			if nb, err := readZipFile(zr, file); err == nil {
				var ncx struct {
					Points []ncxPoint `xml:"navMap>navPoint"`
				}
				if xml.Unmarshal(nb, &ncx) == nil {
					var walk func(ps []ncxPoint)
					walk = func(ps []ncxPoint) {
						for _, p := range ps {
							if k := epubPath(file, p.Content.Src); titles[k] == "" {
								titles[k] = strings.TrimSpace(p.Text)
							}
							walk(p.Points)
						}
					}
					walk(ncx.Points)
				}
			}
		case strings.Contains(it.Props, "nav"): // EPUB 3: <nav epub:type="toc">
			if nb, err := readZipFile(zr, file); err == nil {
				for _, m := range navLinks.FindAllStringSubmatch(string(nb), -1) {
					k := epubPath(file, m[1])
					if t := strings.TrimSpace(tagRe.ReplaceAllString(m[2], "")); t != "" && titles[k] == "" {
						titles[k] = t
					}
				}
			}
		}
	}
	toc := &epubTOC{Chapters: []epubChapter{}}
	if len(pkg.Metadata.Title) > 0 {
		toc.Title = strings.TrimSpace(pkg.Metadata.Title[0])
	}
	for _, ref := range pkg.Spine.Itemrefs {
		if href := byID[ref.IDRef]; href != "" && ref.Linear != "no" {
			toc.Chapters = append(toc.Chapters, epubChapter{Href: href, Title: titles[href]})
		}
	}
	if len(toc.Chapters) == 0 {
		return nil, errors.New("EPUB sem capítulos")
	}
	return toc, nil
}

// serveEpubEntry entrega um arquivo de dentro do EPUB. O conteúdo é de terceiros e roda no mesmo endereço do painel: a política
// abaixo (e o iframe com sandbox no painel) impede qualquer script do livro.
func serveEpubEntry(w http.ResponseWriter, r *http.Request, zr *zip.Reader, file string) {
	file = path.Clean(file)
	f, err := zr.Open(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil || st.IsDir() || st.Size() > maxEpubEntry {
		http.NotFound(w, r)
		return
	}
	var ct string
	switch strings.ToLower(path.Ext(file)) {
	case ".xhtml", ".html", ".htm":
		ct = "text/html; charset=utf-8"
	case ".css":
		ct = "text/css; charset=utf-8"
	case ".svg":
		ct = "image/svg+xml"
	case ".ttf":
		ct = "font/ttf"
	case ".otf":
		ct = "font/otf"
	case ".woff":
		ct = "font/woff"
	case ".woff2":
		ct = "font/woff2"
	default:
		ct = mime.TypeByExtension(path.Ext(file))
	}
	if ct == "" || strings.Contains(ct, "javascript") {
		http.NotFound(w, r) // scripts e tipos desconhecidos não são entregues
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; media-src 'self'")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	io.Copy(w, f)
}
