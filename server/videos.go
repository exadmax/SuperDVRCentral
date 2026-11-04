// Lista de vídeos: o hub só lê os nomes da pasta de vídeos (a mesma que o nginx serve em /videos/) para a página Vídeos
// montar os botões "Abrir no VLC". O arquivo em si é entregue pelo nginx (com Accept-Ranges), nunca passa pelo hub.
package main

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type VideosConfig struct {
	Dir string `json:"dir"` // pasta dos vídeos no servidor (padrão ~/Videos); vazio desliga a lista
}

type videoItem struct {
	Name string `json:"name"`          // nome do arquivo (a página monta a URL em /videos/<name>)
	Size int64  `json:"size"`          // bytes
	Time int64  `json:"time"`          // modificação, segundos Unix
	Sub  string `json:"sub,omitempty"` // legenda com o mesmo nome (.srt, .vtt, .ass), se existir
}

var videoExts = map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true, ".m4v": true, ".ts": true, ".wmv": true}
var subExts = map[string]bool{".srt": true, ".vtt": true, ".ass": true}

func listVideos(dir string) ([]videoItem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	subs := map[string]string{} // nome sem extensão (minúsculo) -> arquivo da legenda
	for _, e := range entries {
		if ext := strings.ToLower(filepath.Ext(e.Name())); !e.IsDir() && subExts[ext] {
			subs[strings.ToLower(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))] = e.Name()
		}
	}
	out := []videoItem{}
	for _, e := range entries {
		if e.IsDir() || !videoExts[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		v := videoItem{Name: e.Name(), Size: info.Size(), Time: info.ModTime().Unix()}
		v.Sub = subs[strings.ToLower(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))]
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time }) // mais recentes primeiro
	return out, nil
}

func videosHandler(cfg VideosConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Dir == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"Pasta de vídeos não configurada no hub (campo \"videos.dir\")."}`))
			return
		}
		list, err := listVideos(cfg.Dir)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"Não foi possível ler a pasta de vídeos do servidor."}`))
			return
		}
		writeJSON(w, list)
	}
}

var srtStamp = regexp.MustCompile(`(\d\d:\d\d:\d\d),(\d\d\d)`)

// srtToVTT converte legenda SRT em WebVTT (o <track> do navegador só lê VTT); aceita UTF-8 ou Latin-1 (comum em .srt antigos).
func srtToVTT(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}) // BOM
	var s string
	if utf8.Valid(b) {
		s = string(b)
	} else {
		r := make([]rune, len(b))
		for i, c := range b {
			r[i] = rune(c)
		}
		s = string(r)
	}
	s = strings.TrimPrefix(strings.ReplaceAll(s, "\r\n", "\n"), "\ufeff")
	return "WEBVTT\n\n" + srtStamp.ReplaceAllString(s, "$1.$2")
}

// subHandler entrega a legenda do vídeo ?name=... como WebVTT, pelo mesmo endereço do painel (o <track> exige mesma origem).
// Só serve a legenda que a listagem associou àquele vídeo: o nome vindo da URL nunca vira caminho de arquivo.
func subHandler(cfg VideosConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := listVideos(cfg.Dir)
		if cfg.Dir == "" || err != nil {
			http.Error(w, "pasta de vídeos indisponível", http.StatusServiceUnavailable)
			return
		}
		name := r.URL.Query().Get("name")
		for _, v := range list {
			if v.Name != name || v.Sub == "" {
				continue
			}
			ext := strings.ToLower(filepath.Ext(v.Sub))
			b, err := os.ReadFile(filepath.Join(cfg.Dir, v.Sub))
			if err != nil || (ext != ".srt" && ext != ".vtt") { // .ass o <track> não lê
				break
			}
			w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
			if ext == ".srt" {
				w.Write([]byte(srtToVTT(b)))
			} else {
				w.Write(b)
			}
			return
		}
		http.NotFound(w, r)
	}
}
