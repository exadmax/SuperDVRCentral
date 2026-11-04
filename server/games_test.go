package main

import "testing"

func TestIsoMinutes(t *testing.T) {
	cases := map[string]int{"PT12H34M5S": 754, "PT45M": 45, "PT3H": 180, "P1DT2H": 1560, "PT30S": 0, "": 0, "lixo": 0}
	for in, want := range cases {
		if got := isoMinutes(in); got != want {
			t.Errorf("isoMinutes(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestNormName(t *testing.T) {
	if normName("God of War Ragnarök Trophies") != normName("God of War Ragnarök") {
		t.Error("sufixo Trophies deveria ser ignorado")
	}
	if normName("Marvel's Spider-Man 2") != "marvelsspiderman2" {
		t.Errorf("normName: %q", normName("Marvel's Spider-Man 2"))
	}
}

func TestRaDate(t *testing.T) {
	if got := raDate("2024-05-01 21:10:33"); got != "2024-05-01T21:10:33Z" {
		t.Errorf("raDate = %q", got)
	}
	if raDate("") != "" || raDate("ontem") != "" {
		t.Error("datas inválidas devem virar vazio")
	}
}

func TestPctAndRecent(t *testing.T) {
	if pct(1, 3) != 33.3 || pct(5, 0) != 0 || pct(10, 10) != 100 {
		t.Errorf("pct: %v %v %v", pct(1, 3), pct(5, 0), pct(10, 10))
	}
	r := sortRecent([]Unlock{{Date: "2024-01-01T00:00:00Z"}, {Date: "2025-01-01T00:00:00Z"}, {Date: "2023-01-01T00:00:00Z"}}, 2)
	if len(r) != 2 || r[0].Date[:4] != "2025" || r[1].Date[:4] != "2024" {
		t.Errorf("sortRecent: %+v", r)
	}
}
