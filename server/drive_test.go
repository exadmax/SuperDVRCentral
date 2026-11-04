package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4/seekablebuffer"
)

// fakeMP4 monta um arquivo no formato do recorder.go: "init" (ftyp+moov de mentira) e n trechos moof/mdat de payloadSize bytes cada.
func fakeMP4(t *testing.T, n, payloadSize int) ([]byte, int) {
	t.Helper()
	box := func(typ string, body []byte) []byte {
		b := make([]byte, 8, 8+len(body))
		b[3] = byte(8 + len(body))
		copy(b[4:], typ)
		return append(b, body...)
	}
	init := append(box("ftyp", []byte("isom")), box("moov", []byte("0123456789"))...)
	out := append([]byte{}, init...)
	var base uint64
	partLen := 0
	for i := 0; i < n; i++ {
		s := frame(t, true, payloadSize)
		var buf seekablebuffer.Buffer
		part := fmp4.Part{SequenceNumber: uint32(i + 1), Tracks: []*fmp4.PartTrack{{ID: 1, BaseTime: base, Samples: []*fmp4.Sample{s}}}}
		if err := part.Marshal(&buf); err != nil {
			t.Fatal(err)
		}
		out = append(out, buf.Bytes()...)
		partLen = len(buf.Bytes())
		base += uint64(s.Duration)
	}
	return out, partLen
}

func TestSplitMP4(t *testing.T) {
	data, partLen := fakeMP4(t, 5, 1000)
	dir := t.TempDir()
	src := filepath.Join(dir, "Sala_2026100110.mp4")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	initLen := len(data) - 5*partLen
	// cabem 2 trechos por parte -> 3 partes (2 + 2 + 1)
	parts, err := splitMP4(src, dir, "Sala_2026100110", int64(initLen+2*partLen))
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("partes = %v, queria 3", parts)
	}
	if filepath.Base(parts[0]) != "Sala_2026100110_parte1.mp4" || filepath.Base(parts[2]) != "Sala_2026100110_parte3.mp4" {
		t.Errorf("nomes inesperados: %v", parts)
	}
	var total int
	for i, p := range parts {
		b, _ := os.ReadFile(p)
		if !bytes.HasPrefix(b, data[:initLen]) {
			t.Errorf("parte %d sem o init", i+1)
		}
		if len(b) > initLen+2*partLen {
			t.Errorf("parte %d passou do limite: %d", i+1, len(b))
		}
		total += len(b) - initLen
		var ps fmp4.Parts
		if err := ps.Unmarshal(b[initLen:]); err != nil {
			t.Fatalf("parte %d não é MP4 fragmentado válido: %v", i+1, err)
		}
		if got := ps[0].Tracks[0].BaseTime; got != 0 {
			t.Errorf("parte %d começa no tempo %d, queria 0", i+1, got)
		}
		if i < 2 && ps[1].Tracks[0].BaseTime != 6000 {
			t.Errorf("parte %d: segundo trecho em %d, queria 6000", i+1, ps[1].Tracks[0].BaseTime)
		}
	}
	if total != 5*partLen {
		t.Errorf("os trechos somam %d bytes, queria %d (algo se perdeu)", total, 5*partLen)
	}
}

func TestSplitMP4TruncatedAndOversize(t *testing.T) {
	data, partLen := fakeMP4(t, 3, 1000)
	dir := t.TempDir()
	src := filepath.Join(dir, "a_2026100110.mp4")
	os.WriteFile(src, data[:len(data)-100], 0o644)       // último trecho cortado no meio
	parts, err := splitMP4(src, dir, "a_2026100110", 10) // limite menor que um trecho: 1 trecho por parte
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 {
		t.Fatalf("partes = %d, queria 2 (o trecho cortado é descartado)", len(parts))
	}
	_ = partLen
	if _, err := splitMP4(filepath.Join(dir, "nao_existe.mp4"), dir, "x", 10); err == nil {
		t.Error("arquivo inexistente deveria falhar")
	}
}

func testDrive(t *testing.T, now time.Time, maxMB int) (*driveSync, *[]string, string) {
	t.Helper()
	cur := now
	m, _ := testMgr(t, &cur)
	var sent []string
	d := newDriveSync(DriveSyncConfig{Enabled: true, Remote: "GDrive:CameraSeguranca", Hour: 3, MaxFileMB: maxMB}, filepath.Join(t.TempDir(), "drive.state.json"), m)
	d.now = func() time.Time { return cur }
	d.check = func(ctx context.Context) error { return nil }
	d.list = func(ctx context.Context, path string) ([]string, error) { return nil, nil }
	d.purge = func(ctx context.Context, p string) error { t.Errorf("purge inesperado: %s", p); return nil }
	d.rmdir = func(ctx context.Context, p string) error { t.Errorf("rmdir inesperado: %s", p); return nil }
	d.upload = func(ctx context.Context, src, dest string) error {
		if _, err := os.Stat(src); err != nil {
			return err
		}
		sent = append(sent, dest)
		return nil
	}
	return d, &sent, m.dir
}

func writeRec(t *testing.T, dir, name string, size int, mod time.Time) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, bytes.Repeat([]byte{1}, size), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(p, mod, mod)
}

func TestDriveRun(t *testing.T) {
	now := time.Date(2026, 10, 4, 3, 0, 0, 0, time.Local)
	d, sent, dir := testDrive(t, now, 1024)
	old := now.Add(-2 * time.Hour)
	writeRec(t, dir, "Sala_de_Estar_2026100320.mp4", 100, old)
	writeRec(t, dir, "Garagem_2026100323.mp4", 100, old)
	writeRec(t, dir, "Garagem_2026100323_2.mp4", 100, old)
	writeRec(t, dir, "Garagem_2026100402.mp4", 100, now.Add(-30*time.Minute)) // hora 02 terminou às 03:00, sem escrita há 30 min: vai
	writeRec(t, dir, "Garagem_2026100403.mp4", 100, now)                      // hora em andamento: fica para depois
	writeRec(t, dir, "outro.txt", 100, old)

	ok, msg := d.runOnce()
	if !ok {
		t.Fatalf("falhou: %s", msg)
	}
	want := []string{
		"GDrive:CameraSeguranca/2026-10-03/Garagem/Garagem_2026100323.mp4",
		"GDrive:CameraSeguranca/2026-10-03/Garagem/Garagem_2026100323_2.mp4",
		"GDrive:CameraSeguranca/2026-10-04/Garagem/Garagem_2026100402.mp4",
		"GDrive:CameraSeguranca/2026-10-03/Sala_de_Estar/Sala_de_Estar_2026100320.mp4",
	}
	if strings.Join(*sent, "\n") != strings.Join(want, "\n") {
		t.Errorf("enviados:\n%s\nqueria:\n%s", strings.Join(*sent, "\n"), strings.Join(want, "\n"))
	}
	*sent = nil
	if ok, msg := d.runOnce(); !ok || len(*sent) != 0 || msg != "nada novo para enviar" {
		t.Errorf("segunda rodada: ok=%v msg=%q enviados=%v", ok, msg, *sent)
	}
	os.Remove(filepath.Join(dir, "Sala_de_Estar_2026100320.mp4")) // a retenção apagou: sai do estado
	d.forget()
	if _, ok := d.st.Uploaded["Sala_de_Estar_2026100320.mp4"]; ok {
		t.Error("arquivo apagado do disco deveria sair do estado")
	}
}

func TestDriveRunSplitsAndRetries(t *testing.T) {
	now := time.Date(2026, 10, 4, 3, 0, 0, 0, time.Local)
	d, sent, dir := testDrive(t, now, 1)
	data, partLen := fakeMP4(t, 4, 600<<10) // ~2,4 MB com limite de 1 MB: 1 trecho por parte
	_ = partLen
	os.WriteFile(filepath.Join(dir, "x"), nil, 0) // dir existe
	os.MkdirAll(dir, 0o755)
	p := filepath.Join(dir, "Sala_de_Estar_2026100310.mp4")
	os.WriteFile(p, data, 0o644)
	os.Chtimes(p, now.Add(-time.Hour), now.Add(-time.Hour))

	fail := true
	up := d.upload
	d.upload = func(ctx context.Context, src, dest string) error {
		if fail && strings.HasSuffix(dest, "_parte2.mp4") {
			return errors.New("sem rede")
		}
		return up(ctx, src, dest)
	}
	ok, msg := d.runOnce()
	if ok || !strings.Contains(msg, "sem rede") {
		t.Fatalf("queria falha por rede, veio ok=%v msg=%q", ok, msg)
	}
	if _, done := d.st.Uploaded["Sala_de_Estar_2026100310.mp4"]; done {
		t.Fatal("arquivo com parte que falhou não pode ser marcado como enviado")
	}
	if _, err := os.Stat(filepath.Join(dir, ".drive-tmp")); err == nil {
		t.Error("pasta temporária deveria ter sido limpa")
	}
	fail = false
	*sent = nil
	if ok, msg := d.runOnce(); !ok {
		t.Fatalf("segunda tentativa falhou: %s", msg)
	}
	if len(*sent) != 4 || !strings.HasSuffix((*sent)[0], "/2026-10-03/Sala_de_Estar/Sala_de_Estar_2026100310_parte1.mp4") {
		t.Errorf("enviados: %v", *sent)
	}
}

func TestDriveDue(t *testing.T) {
	at := func(day, h, m int) time.Time { return time.Date(2026, 10, day, h, m, 0, 0, time.Local) }
	cur := at(4, 2, 59)
	d, _, _ := testDrive(t, cur, 0)
	d.now = func() time.Time { return cur }
	d.st.Cycle, d.st.Attempts, d.st.LastOK = at(3, 3, 0), 1, true // rodada de ontem concluída
	d.st.LastAttempt = at(3, 3, 5)
	if d.due() {
		t.Error("antes das 3:00 não roda")
	}
	cur = at(4, 3, 0)
	if !d.due() {
		t.Error("às 3:00 começa a rodada nova")
	}
	// 1ª tentativa falha às 3:00
	d.st.Cycle, d.st.Attempts, d.st.LastOK, d.st.LastAttempt = at(4, 3, 0), 1, false, at(4, 3, 0)
	cur = at(4, 3, 59)
	if d.due() {
		t.Error("a 2ª tentativa espera 1 h")
	}
	cur = at(4, 4, 0)
	if !d.due() {
		t.Error("2ª tentativa 1 h depois")
	}
	// 2ª falha às 4:00: a 3ª espera 4 h
	d.st.Attempts, d.st.LastAttempt = 2, at(4, 4, 0)
	cur = at(4, 7, 59)
	if d.due() {
		t.Error("a 3ª tentativa espera 4 h")
	}
	cur = at(4, 8, 0)
	if !d.due() {
		t.Error("3ª tentativa 4 h depois")
	}
	// 3 tentativas gastas: para
	d.st.Attempts, d.st.LastAttempt = 3, at(4, 8, 0)
	cur = at(4, 20, 0)
	if d.due() {
		t.Error("no máximo 3 tentativas por rodada")
	}
	// prazo: depois de 1:00 do dia seguinte não tenta mais, mesmo com tentativas sobrando
	d.st.Attempts, d.st.LastAttempt = 2, at(4, 20, 0)
	cur = at(5, 0, 59)
	if !d.due() {
		t.Error("ainda dentro do prazo (até 1:00)")
	}
	cur = at(5, 1, 0)
	if d.due() {
		t.Error("passou de 1:00 do dia seguinte: sem nova tentativa")
	}
	cur = at(5, 3, 0)
	if !d.due() {
		t.Error("às 3:00 seguinte começa outra rodada")
	}
	// hub desligado na hora marcada: ao voltar (dentro do prazo) faz a rodada atrasada
	d.st.Cycle, d.st.Attempts, d.st.LastOK = at(5, 3, 0), 1, true
	cur = at(6, 10, 0)
	if !d.due() {
		t.Error("rodada perdida deve ser feita ao voltar")
	}
	cur = at(7, 1, 30) // a rodada de 6/10 passou do prazo, mas a de 7/10 ainda não começou
	d.st.Cycle = at(6, 3, 0)
	if d.due() {
		t.Error("rodada vencida e a nova ainda não é hora")
	}
}

func TestDriveRunCountsAttempts(t *testing.T) {
	now := time.Date(2026, 10, 4, 3, 0, 0, 0, time.Local)
	d, _, _ := testDrive(t, now, 1024)
	d.check = func(ctx context.Context) error { return errors.New("sem rede") }
	wait := func() {
		for i := 0; i < 200; i++ {
			d.mu.Lock()
			r := d.running
			d.mu.Unlock()
			if !r {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("rodada não terminou")
	}
	if !d.run() {
		t.Fatal("não iniciou")
	}
	wait()
	if d.st.Attempts != 1 || d.st.LastOK || !d.st.Cycle.Equal(now) || !strings.Contains(d.st.LastMsg, "sem rede") {
		t.Errorf("estado após falha: %+v", d.st)
	}
	d.run("sala") // só uma câmera: não conta como tentativa da rodada
	wait()
	if d.st.Attempts != 1 {
		t.Errorf("rodada parcial contou como tentativa: %d", d.st.Attempts)
	}
	d.check = func(ctx context.Context) error { return nil }
	d.now = func() time.Time { return now.Add(time.Hour) }
	d.run()
	wait()
	if d.st.Attempts != 2 || !d.st.LastOK {
		t.Errorf("2ª tentativa: %+v", d.st)
	}
}

func TestDriveSingleCameraAndSkip(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.Local)
	cs := newCameras([]CameraConfig{{ID: "sala", Name: "Sala de Estar", URL: "rtsp://x/y"}, {ID: "garagem", Name: "Garagem", URL: "rtsp://x/z"}})
	cur := now
	m := newRecMgr(RecordingsConfig{Dir: filepath.Join(t.TempDir(), "securityCam")}, filepath.Join(t.TempDir(), "state.json"), cs)
	m.now = func() time.Time { return cur }
	var sent []string
	d := newDriveSync(DriveSyncConfig{Enabled: true, Remote: "GDrive:CameraSeguranca", Hour: 3}, filepath.Join(t.TempDir(), "drive.state.json"), m)
	d.now = func() time.Time { return cur }
	d.check = func(ctx context.Context) error { return nil }
	d.upload = func(ctx context.Context, src, dest string) error { sent = append(sent, dest); return nil }
	d.list = func(ctx context.Context, path string) ([]string, error) { return nil, nil }
	old, recent := now.Add(-3*24*time.Hour), now.Add(-3*time.Hour)
	writeRec(t, m.dir, "Sala_de_Estar_2026100112.mp4", 100, old)
	writeRec(t, m.dir, "Sala_de_Estar_2026100409.mp4", 100, recent)
	writeRec(t, m.dir, "Garagem_2026100112.mp4", 100, old)
	writeRec(t, m.dir, "Garagem_2026100409.mp4", 100, recent)

	only, bad := d.prefixes([]string{"garagem"})
	if bad != "" || len(only) != 1 || !only["Garagem"] {
		t.Fatalf("prefixes: %v %q", only, bad)
	}
	if _, bad := d.prefixes([]string{"nada"}); bad != "nada" {
		t.Errorf("câmera desconhecida deveria ser recusada: %q", bad)
	}
	// só a Garagem
	if ok, msg := d.runOnceFor(only); !ok || len(sent) != 2 || strings.Contains(strings.Join(sent, " "), "Sala") {
		t.Errorf("só Garagem: ok=%v msg=%q enviados=%v", ok, msg, sent)
	}
	// pula o que tem mais de 2 dias (só da Sala; a Garagem já foi) e envia o resto
	sent = nil
	if n := d.skip(2, nil); n != 1 {
		t.Errorf("pulados = %d, queria 1 (Sala de 01/10)", n)
	}
	if ok, _ := d.runOnce(); !ok || len(sent) != 1 || !strings.Contains(sent[0], "Sala_de_Estar_2026100409.mp4") {
		t.Errorf("depois de pular: enviados=%v", sent)
	}
	st := d.status()
	if st["skipped"] != 1 || st["uploaded"] != 3 {
		t.Errorf("status: uploaded=%v skipped=%v", st["uploaded"], st["skipped"])
	}
}

func TestDrivePrune(t *testing.T) {
	// retenção da página Config: Sala de Estar 3 dias, as demais (padrão) 7. Hoje 11/10 -> corte da Sala 08/10, das outras 04/10.
	now := time.Date(2026, 10, 11, 3, 0, 0, 0, time.Local)
	d, sent, dir := testDrive(t, now, 1024)
	setCam(d.m, "sala", func(c *recCamSet) { c.RetentionDays = 3 })

	tree := map[string][]string{
		"":           {"2026-10-03", "2026-10-06", "2026-10-09", "2026-09-01", "Fotos", "2026-10-03-backup", "2026-13-45"},
		"2026-10-03": {"Sala_de_Estar", "Garagem"},
		"2026-10-06": {"Sala_de_Estar", "Garagem"},
		"2026-10-09": {"Sala_de_Estar", "Garagem"},
		"2026-09-01": {"Garagem", "pasta com espaço"},
	}
	var purged, rmdirs []string
	d.list = func(ctx context.Context, path string) ([]string, error) { return tree[path], nil }
	d.purge = func(ctx context.Context, p string) error { purged = append(purged, p); return nil }
	d.rmdir = func(ctx context.Context, p string) error { rmdirs = append(rmdirs, p); return nil }
	// gravação de 8 dias atrás ainda no disco: não vale subir para uma pasta que seria apagada agora mesmo
	writeRec(t, dir, "Garagem_2026100310.mp4", 100, now.Add(-8*24*time.Hour))
	writeRec(t, dir, "Garagem_2026100610.mp4", 100, now.Add(-5*24*time.Hour))       // Garagem guarda 7 dias: sobe
	writeRec(t, dir, "Sala_de_Estar_2026100610.mp4", 100, now.Add(-5*24*time.Hour)) // Sala guarda 3 dias: já venceu, não sobe

	ok, msg := d.runOnce()
	if !ok || !strings.Contains(msg, "4 pasta(s)") {
		t.Fatalf("ok=%v msg=%q", ok, msg)
	}
	want := "2026-10-03/Sala_de_Estar,2026-10-03/Garagem,2026-10-06/Sala_de_Estar,2026-09-01/Garagem" // na ordem da listagem
	if got := strings.Join(purged, ","); got != want {
		t.Errorf("apagadas: %s\nqueria:   %s", got, want)
	}
	// 10-03 ficou sem nada (as duas câmeras venceram) e 10-06 e 10-09 ainda têm a Garagem/Sala dentro da retenção;
	// 09-01 ainda tem "pasta com espaço", que não é nome de câmera do sistema: não é apagada e segura a pasta do dia
	if got := strings.Join(rmdirs, ","); got != "2026-10-03" {
		t.Errorf("pastas de dia removidas: %q, queria só 2026-10-03", got)
	}
	if len(*sent) != 1 || !strings.Contains((*sent)[0], "2026-10-06/Garagem/") {
		t.Errorf("enviados: %v", *sent)
	}

	purged = nil
	d.cfg.Remote = "GDrive:"
	if _, err := d.prune(context.Background()); err == nil || len(purged) != 0 {
		t.Errorf("destino na raiz do Drive deve ser recusado: err=%v purged=%v", err, purged)
	}
}

func TestDriveCameraOff(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.Local)
	cs := newCameras([]CameraConfig{{ID: "sala", Name: "Sala de Estar", URL: "rtsp://x/y"}, {ID: "garagem", Name: "Garagem", URL: "rtsp://x/z"}})
	m := newRecMgr(RecordingsConfig{Dir: filepath.Join(t.TempDir(), "securityCam")}, filepath.Join(t.TempDir(), "state.json"), cs)
	m.now = func() time.Time { return now }
	var sent []string
	d := newDriveSync(DriveSyncConfig{Enabled: true, Remote: "GDrive:CameraSeguranca", Hour: 3}, filepath.Join(t.TempDir(), "drive.state.json"), m)
	d.now = func() time.Time { return now }
	d.check = func(ctx context.Context) error { return nil }
	d.upload = func(ctx context.Context, src, dest string) error { sent = append(sent, dest); return nil }
	d.list = func(ctx context.Context, path string) ([]string, error) { return nil, nil }
	recent := now.Add(-3 * time.Hour)
	writeRec(t, m.dir, "Sala_de_Estar_2026100409.mp4", 100, recent)
	writeRec(t, m.dir, "Garagem_2026100409.mp4", 100, recent)
	setCam(m, "sala", func(c *recCamSet) { c.DriveOff = true })

	// rodada automática / "Sincronizar tudo": só a Garagem
	if ok, _ := d.runOnce(); !ok || len(sent) != 1 || !strings.Contains(sent[0], "/Garagem/") {
		t.Fatalf("automática: enviados=%v", sent)
	}
	// o status ainda mostra o que a Sala enviaria (botão Sincronizar da câmera)
	if p := d.status()["pending"].(map[string]drivePend); p["sala"].Files != 1 {
		t.Errorf("pendências da Sala desligada: %+v", p)
	}
	// botão da própria câmera: envia mesmo desligada
	sent = nil
	only, _ := d.prefixes([]string{"sala"})
	if ok, _ := d.runOnceFor(only); !ok || len(sent) != 1 || !strings.Contains(sent[0], "/Sala_de_Estar/") {
		t.Errorf("botão da câmera: enviados=%v", sent)
	}
}
