package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListVideos(t *testing.T) {
	d := t.TempDir()
	for _, n := range []string{"Filme.mp4", "Filme.srt", "outro.MKV", "leiame.txt"} {
		os.WriteFile(filepath.Join(d, n), []byte("x"), 0o644)
	}
	os.Mkdir(filepath.Join(d, "pasta.mp4"), 0o755)
	l, err := listVideos(d)
	if err != nil || len(l) != 2 {
		t.Fatalf("esperava 2 vídeos, veio %v (%v)", l, err)
	}
	for _, v := range l {
		if v.Name == "Filme.mp4" && v.Sub != "Filme.srt" {
			t.Errorf("legenda não casou: %+v", v)
		}
		if v.Name == "outro.MKV" && v.Sub != "" {
			t.Errorf("legenda inesperada: %+v", v)
		}
	}
}

func TestSrtToVTT(t *testing.T) {
	got := srtToVTT([]byte("\xef\xbb\xbf1\r\n00:00:01,500 --> 00:00:03,000\r\nOl\xe1\r\n"))
	want := "WEBVTT\n\n1\n00:00:01.500 --> 00:00:03.000\nOl\u00e1\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
