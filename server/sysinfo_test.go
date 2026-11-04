package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSysMon(t *testing.T) {
	m := newSysMon()
	x := 0
	for end := time.Now().Add(300 * time.Millisecond); time.Now().Before(end); x++ { // gasta CPU para o delta aparecer
	}
	m.sample()
	st := m.cur
	if st.Cores < 1 || st.MemTotal == 0 || st.AppMem == 0 || st.MemUsed == 0 || st.MemUsed > st.MemTotal {
		t.Fatalf("leitura inválida: %+v", st)
	}
	if st.AppCPUPercent <= 0 || st.AppCPUPercent > 100 || st.CPUPercent <= 0 {
		t.Fatalf("CPU inválida: %+v", st)
	}
	t.Logf("%+v", st)
}

func TestRatioPct(t *testing.T) {
	if ratioPct(1, 3) != 33.33 || ratioPct(5, 0) != 0 || ratioPct(9, 3) != 100 || ratioPct(-1, 3) != 0 {
		t.Fatal("pct")
	}
}

func TestPowerRecusaSemCabecalho(t *testing.T) {
	h := powerHandler(newSysMon(), "x.service", false)
	for _, m := range []string{"GET", "POST"} { // GET nunca; POST só com X-CD-Action
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(m, "/api/admin/desktop-off", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s sem cabeçalho devia dar 405, deu %d", m, rec.Code)
		}
	}
}
