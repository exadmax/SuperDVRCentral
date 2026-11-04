package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
)

func TestRecName(t *testing.T) {
	cases := map[string]string{"Sala de Estar": "Sala_de_Estar", "Garagem": "Garagem", "Câmera  Cozinha!": "Camera_Cozinha", "!!!": "x"}
	for in, want := range cases {
		if got := recName(in, "x"); got != want {
			t.Errorf("recName(%q) = %q, queria %q", in, got, want)
		}
	}
}

func testMgr(t *testing.T, now *time.Time) (*recMgr, *camera) {
	t.Helper()
	cs := newCameras([]CameraConfig{{ID: "sala", Name: "Sala de Estar", URL: "rtsp://x/y"}})
	m := newRecMgr(RecordingsConfig{Dir: filepath.Join(t.TempDir(), "securityCam")}, filepath.Join(t.TempDir(), "state.json"), cs)
	m.now = func() time.Time { return *now }
	return m, cs.list[0]
}

func setCam(m *recMgr, id string, f func(*recCamSet)) {
	m.mu.Lock()
	cs := m.set.Cameras[id]
	f(&cs)
	m.set.Cameras[id] = cs
	m.mu.Unlock()
}

func frame(t *testing.T, key bool, n int) *fmp4.Sample {
	t.Helper()
	nal := make([]byte, n)
	nal[0] = 0x41
	if key {
		nal[0] = 0x65
	}
	s, err := fmp4.NewSampleH264(0, [][]byte{nal})
	if err != nil {
		t.Fatal(err)
	}
	s.Duration = 6000
	return s
}

func feedFrame(t *testing.T, c *camera, key bool, n int) { c.rec.sample(frame(t, key, n), key) }

func TestRecorderHourlyFiles(t *testing.T) {
	now := time.Date(2026, 10, 1, 14, 59, 0, 0, time.Local)
	m, c := testMgr(t, &now)

	c.rec.setInit([]byte("INIT"))
	feedFrame(t, c, true, 100) // chave geral desligada: nada é gravado
	feedFrame(t, c, true, 100)
	if names, _ := os.ReadDir(m.dir); len(names) != 0 {
		t.Fatal("gravou com a chave geral desligada")
	}

	m.set.Enabled = true
	feedFrame(t, c, false, 50) // sem quadro-chave não começa nada
	feedFrame(t, c, true, 100)
	feedFrame(t, c, false, 50)
	now = now.Add(2 * time.Minute) // virou a hora
	feedFrame(t, c, true, 100)     // fecha o grupo das 14h
	if got := c.rec.current(); got != "Sala_de_Estar_2026100114.mp4" {
		t.Fatalf("arquivo = %q", got)
	}
	feedFrame(t, c, false, 50)
	now = now.Add(5 * time.Second)
	feedFrame(t, c, true, 100) // fecha o grupo das 15h: arquivo novo
	if got := c.rec.current(); got != "Sala_de_Estar_2026100115.mp4" {
		t.Fatalf("arquivo após a virada = %q", got)
	}
	if got := c.rec.writing(now); got != "Sala_de_Estar_2026100115.mp4" {
		t.Fatalf("writing = %q", got)
	}
	c.rec.close()

	for _, n := range []string{"Sala_de_Estar_2026100114.mp4", "Sala_de_Estar_2026100115.mp4"} {
		b, err := os.ReadFile(filepath.Join(m.dir, n))
		if err != nil || !bytes.HasPrefix(b, []byte("INIT")) || !bytes.Contains(b, []byte("moof")) {
			t.Errorf("%s inválido (err=%v, %d bytes)", n, err, len(b))
		}
	}
	list, err := m.files()
	if err != nil || len(list) != 2 || list[0].Name != "Sala_de_Estar_2026100115.mp4" || list[0].Camera != "Sala de Estar" {
		t.Fatalf("lista = %+v (%v)", list, err)
	}

	// reinício do hub na mesma hora: não mistura dois vídeos no arquivo existente
	c.rec.setInit([]byte("INIT"))
	feedFrame(t, c, true, 100)
	feedFrame(t, c, true, 100)
	if got := c.rec.current(); got != "Sala_de_Estar_2026100115_2.mp4" {
		t.Fatalf("arquivo após reinício = %q", got)
	}
	c.rec.close()
}

func TestRecorderMotionMode(t *testing.T) {
	now := time.Date(2026, 10, 1, 10, 0, 0, 0, time.Local)
	m, c := testMgr(t, &now)
	m.set.Enabled = true
	setCam(m, "sala", func(cs *recCamSet) { cs.Mode = recMotion })
	c.rec.setInit([]byte("INIT"))

	step := func(key bool, n int) { now = now.Add(66 * time.Millisecond); feedFrame(t, c, key, n) }
	feedFrame(t, c, true, 3000)
	for i := 0; i < 60; i++ { // cena parada: aprende o normal e não grava nada
		step(false, 300)
	}
	step(true, 3000)
	for i := 0; i < 20; i++ {
		step(false, 300)
	}
	step(true, 3000)
	if names, _ := os.ReadDir(m.dir); len(names) != 0 {
		t.Fatalf("gravou sem movimento: %v", names)
	}

	for i := 0; i < 10; i++ { // movimento: quadros P grandes
		step(false, 8000)
	}
	if !c.rec.motionActive(now) {
		t.Fatal("não detectou o movimento")
	}
	step(true, 3000) // fecha o grupo com movimento: agora existe arquivo (começando no quadro-chave anterior)
	if c.rec.current() == "" {
		t.Fatal("não gravou o grupo com movimento")
	}

	for i := 0; i < 300; i++ { // ~20 s parado: passa o pós-roll (quadro-chave a cada 30 quadros, como a câmera)
		step(i%30 == 0, map[bool]int{true: 3000, false: 300}[i%30 == 0])
	}
	step(true, 3000)
	step(true, 3000)
	if c.rec.motionActive(now) {
		t.Fatal("movimento continua ativo depois do pós-roll")
	}
	if w := c.rec.writing(now.Add(recWriteFresh + time.Second)); w != "" {
		t.Fatalf("ainda escrevendo parado: %q", w)
	}
	c.rec.close()
	if names, _ := os.ReadDir(m.dir); len(names) != 1 {
		t.Fatalf("esperava 1 arquivo, tem %d", len(names))
	}
}

func TestMotionDetector(t *testing.T) {
	var d motionDet
	now := time.Now()
	for i := 0; i < 100; i++ {
		if d.feed(400, false, now, 3) {
			t.Fatal("movimento em cena parada")
		}
	}
	if d.feed(9000, false, now, 3) { // um pico isolado não dispara
		t.Fatal("pico isolado disparou")
	}
	for i := 0; i < 5; i++ {
		d.feed(400, false, now, 3)
	}
	var hit bool
	for i := 0; i < 5; i++ {
		hit = d.feed(9000, false, now, 3)
	}
	if !hit {
		t.Fatal("não detectou movimento sustentado")
	}
	if d.feed(100, true, now.Add(motionPostRoll+time.Second), 3) {
		t.Fatal("movimento ativo depois do pós-roll")
	}
}

func TestSweepRetentionAndLimit(t *testing.T) {
	now := time.Now()
	m, c := testMgr(t, &now)
	os.MkdirAll(m.dir, 0o755)
	mk := func(n string, size int, age time.Duration) {
		os.WriteFile(filepath.Join(m.dir, n), bytes.Repeat([]byte("x"), size), 0o644)
		when := now.Add(-age)
		os.Chtimes(filepath.Join(m.dir, n), when, when)
	}
	day := 24 * time.Hour
	mk("Sala_de_Estar_2026092001.mp4", 10, 8*day) // vencida (7 dias)
	mk("Sala_de_Estar_2026100101.mp4", 10, day)
	mk("Garagem_2026092001.mp4", 10, 8*day) // câmera sem dono no hub: retenção padrão (7 dias)
	mk("foto.mp4", 10, 30*day)              // não é gravação: intocado
	if n := m.sweep(); n != 2 {
		t.Fatalf("apagou %d, queria 2", n)
	}
	for _, gone := range []string{"Sala_de_Estar_2026092001.mp4", "Garagem_2026092001.mp4"} {
		if _, err := os.Stat(filepath.Join(m.dir, gone)); err == nil {
			t.Errorf("não apagou %s", gone)
		}
	}
	for _, kept := range []string{"Sala_de_Estar_2026100101.mp4", "foto.mp4"} {
		if _, err := os.Stat(filepath.Join(m.dir, kept)); err != nil {
			t.Errorf("apagou %s", kept)
		}
	}

	// retenção própria da câmera
	setCam(m, "sala", func(cs *recCamSet) { cs.RetentionDays = 1 })
	mk("Sala_de_Estar_2026093001.mp4", 10, 2*day)
	if n := m.sweep(); n != 1 {
		t.Fatalf("retenção por câmera: apagou %d, queria 1", n)
	}

	// limite de espaço: apaga as mais antigas até caber
	recGB = 25
	defer func() { recGB = 1 << 30 }()
	setCam(m, "sala", func(cs *recCamSet) { cs.RetentionDays = 7; cs.MaxGB = 1 })
	mk("Sala_de_Estar_2026100102.mp4", 10, 4*time.Hour)
	mk("Sala_de_Estar_2026100103.mp4", 10, 3*time.Hour)
	mk("Sala_de_Estar_2026100104.mp4", 10, 2*time.Hour)
	// 4 arquivos de 10 bytes (um de 1 dia, três de horas) = 40 > 25: apaga os 2 mais antigos
	if n := m.sweep(); n != 2 {
		t.Fatalf("limite: apagou %d, queria 2", n)
	}
	if _, err := os.Stat(filepath.Join(m.dir, "Sala_de_Estar_2026100104.mp4")); err != nil {
		t.Error("apagou o mais novo")
	}
	_ = c
}

func TestRecordingsHTTP(t *testing.T) {
	now := time.Now()
	m, c := testMgr(t, &now)
	os.MkdirAll(m.dir, 0o755)
	fresh := "Sala_de_Estar_2026100101.mp4"
	os.WriteFile(filepath.Join(m.dir, fresh), []byte("x"), 0o644)

	mux := http.NewServeMux()
	m.register(mux)
	do := func(method, url, body string, hdr map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, url, strings.NewReader(body))
		for k, v := range hdr {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	ok := map[string]string{"X-CD-Action": "confirm"}
	if w := do("PUT", "/api/recordings/settings", `{"enabled":true}`, nil); w.Code != 405 {
		t.Errorf("PUT sem X-CD-Action = %d", w.Code)
	}
	for _, bad := range []string{`{"cameras":{"sala":{"retentionDays":0}}}`, `{"cameras":{"sala":{"mode":"x"}}}`, `{"cameras":{"sala":{"sensitivity":9}}}`, `{"cameras":{"sala":{"maxGB":-1}}}`} {
		if w := do("PUT", "/api/recordings/settings", bad, ok); w.Code != 400 {
			t.Errorf("%s = %d, queria 400", bad, w.Code)
		}
	}
	w := do("PUT", "/api/recordings/settings", `{"enabled":true,"cameras":{"sala":{"mode":"motion","retentionDays":3,"maxGB":20,"sensitivity":4},"fantasma":{"enabled":false}}}`, ok)
	cs, on := m.want("sala")
	if w.Code != 200 || !on || cs.Mode != recMotion || cs.RetentionDays != 3 || cs.MaxGB != 20 || cs.Sensitivity != 4 || len(m.set.Cameras) != 1 {
		t.Fatalf("PUT = %d %s set=%+v", w.Code, w.Body.String(), m.set)
	}
	if _, err := os.Stat(m.statePath); err != nil {
		t.Error("não salvou o estado")
	}
	if w := do("PUT", "/api/recordings/settings", `{"cameras":{"sala":{"enabled":false}}}`, ok); w.Code != 200 {
		t.Fatal("PUT desligando a câmera falhou")
	}
	if cs, on := m.want("sala"); on || cs.Mode != recMotion { // desligar mantém o resto
		t.Errorf("want após desligar = %+v %v", cs, on)
	}
	c.setRecording(false)

	st := do("GET", "/api/recordings/settings", "", nil).Body.String()
	if !strings.Contains(st, `"mode":"motion"`) || !strings.Contains(st, `"retentionDays":3`) {
		t.Errorf("status sem os campos da câmera: %s", st)
	}

	if w := do("GET", "/api/recordings/file/"+fresh, "", nil); w.Code != 200 || w.Body.String() != "x" || w.Header().Get("Content-Type") != "video/mp4" {
		t.Errorf("arquivo = %d %q", w.Code, w.Body.String())
	}
	for _, bad := range []string{"..%2Fhub.config.json", "foto.mp4", "x.txt", "..%2F..%2Fetc%2Fpasswd"} {
		if w := do("GET", "/api/recordings/file/"+bad, "", nil); w.Code != 404 {
			t.Errorf("%s = %d, queria 404", bad, w.Code)
		}
	}
}
