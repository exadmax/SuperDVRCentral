package main

import "testing"

// Exemplo da documentação do Google: (38.5,-120.2) (40.7,-120.95) (43.252,-126.453)
func TestDecodePolyline(t *testing.T) {
	p := decodePolyline("_p~iF~ps|U_ulLnnqC_mqNvxq`@")
	want := [][2]float64{{38.5, -120.2}, {40.7, -120.95}, {43.252, -126.453}}
	if len(p) != 3 {
		t.Fatalf("pontos: %d", len(p))
	}
	for i := range want {
		if d := p[i][0] - want[i][0]; d > 1e-6 || d < -1e-6 {
			t.Errorf("lat %d: %v != %v", i, p[i][0], want[i][0])
		}
		if d := p[i][1] - want[i][1]; d > 1e-6 || d < -1e-6 {
			t.Errorf("lon %d: %v != %v", i, p[i][1], want[i][1])
		}
	}
}

func TestLevel(t *testing.T) {
	if _, l := level(40, 32); l != "moderado" {
		t.Errorf("40/32 = %s", l)
	}
	if _, l := level(28, 27); l != "livre" {
		t.Errorf("28/27 = %s", l)
	}
	if _, l := level(60, 30); l != "congestionado" {
		t.Errorf("60/30 = %s", l)
	}
}
