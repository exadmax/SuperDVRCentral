package ai

import (
	"slices"
	"testing"
)

func names(ms []Model) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return out
}

func TestRecommend(t *testing.T) {
	irisXe := EffectiveMB([]GPU{{Name: "Intel(R) Iris(R) Xe Graphics", Vendor: VendorIntel, Integrated: true, SharedMB: 12150}})
	if irisXe != 6075 {
		t.Fatalf("Iris Xe efetiva = %d", irisXe)
	}
	cases := []struct {
		eff  int
		want []string
	}{
		{0, []string{"qwen3:1.7b", "llama3.2:3b"}},
		{1024, []string{"qwen3:1.7b", "llama3.2:3b"}},
		{4096, []string{"qwen3:4b", "llama3.2:3b"}},
		{irisXe, []string{"qwen3:4b", "llama3.2:3b"}},
		{6144, []string{"qwen3:4b", "llama3.2:3b"}}, // "até 6 GB" é inclusivo
		{8192, []string{"qwen3:8b", "llama3.1:8b"}},
		{12288, []string{"qwen3:8b", "llama3.1:8b"}},  // "6–12 GB" é inclusivo
		{13000, []string{"qwen3:14b", "gpt-oss:20b"}}, // gpt-oss ainda não cabe: o que cabe vem antes
		{16384, []string{"gpt-oss:20b", "qwen3:14b"}},
		{24576, []string{"qwen3:30b", "gpt-oss:20b"}},
	}
	for _, c := range cases {
		rec, all := Recommend(c.eff)
		if got := names(rec); !slices.Equal(got, c.want) {
			t.Errorf("%d MB: %v, want %v", c.eff, got, c.want)
		}
		if len(all) != len(catalog) {
			t.Errorf("all com %d modelos", len(all))
		}
	}
	rec, _ := Recommend(irisXe)
	if k, f := Fits(rec[0].Name, irisXe); rec[0].Name != "qwen3:4b" || !k || !f {
		t.Fatalf("Iris Xe: %s known=%v fits=%v", rec[0].Name, k, f)
	}
}

func TestRecommendDoesNotAliasCatalog(t *testing.T) {
	_, all := Recommend(0)
	all[0].Name = "mexido"
	if catalog[0].Name == "mexido" {
		t.Fatal("Recommend devolveu o catálogo sem copiar")
	}
}

func TestFits(t *testing.T) {
	cases := []struct {
		name        string
		eff         int
		known, fits bool
	}{
		{"qwen3:8b", 8192, true, true},
		{"qwen3:8b-q4_K_M", 8192, true, true},
		{"QWEN3:8B", 4096, true, false},
		{"qwen3", 8192, true, true},         // = qwen3:latest = qwen3:8b
		{"qwen3:latest", 4096, true, false}, // idem
		{"registry.ollama.ai/library/gpt-oss:20b", 16384, true, true},
		{"gpt-oss:20b", 12288, true, false},
		{"qwen3:4b", 6075, true, true},
		{"qwen3:1.7b", 0, true, false}, // CPU: nada cabe na GPU
		{"mistral:7b", 24576, false, false},
		{"", 8192, false, false},
	}
	for _, c := range cases {
		k, f := Fits(c.name, c.eff)
		if k != c.known || f != c.fits {
			t.Errorf("Fits(%q, %d) = %v,%v; want %v,%v", c.name, c.eff, k, f, c.known, c.fits)
		}
	}
}
