package main

import "testing"

// Uma câmera "viva" só para testar a distribuição: running=true e gen=1 fazem o hub aceitar os quadros.
func liveCamera() *camera {
	return &camera{cfg: CameraConfig{ID: "t", Name: "T", URL: "rtsp://x"}, viewers: map[*camViewer]struct{}{}, running: true, gen: 1}
}

func drain(v *camViewer) (out []string) {
	for {
		select {
		case b, ok := <-v.ch:
			if !ok {
				return
			}
			out = append(out, string(b))
		default:
			return
		}
	}
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCameraEspectadorQueChegaNoMeioRecebeInitEUltimoGrupo(t *testing.T) {
	c := liveCamera()
	c.setInit(1, []byte("init"), "avc1.640028")
	c.pushFrame(1, []byte("K1"), true)
	c.pushFrame(1, []byte("p1"), false)
	c.pushFrame(1, []byte("K2"), true) // novo grupo: o cache recomeça aqui
	c.pushFrame(1, []byte("p2"), false)

	v := c.attach()
	if got, want := drain(v), []string{"init", "K2", "p2"}; !eq(got, want) {
		t.Fatalf("recebeu %v, esperado %v", got, want)
	}
	c.pushFrame(1, []byte("p3"), false)
	if got := drain(v); !eq(got, []string{"p3"}) {
		t.Fatalf("depois de sincronizado deve seguir recebendo: %v", got)
	}
}

func TestCameraEspectadorAntesDoInitEsperaQuadroChave(t *testing.T) {
	c := liveCamera()
	v := c.attach()
	c.setInit(1, []byte("init"), "avc1.640028")
	c.pushFrame(1, []byte("p0"), false) // quadro comum: ainda não dá para começar
	c.pushFrame(1, []byte("K1"), true)
	c.pushFrame(1, []byte("p1"), false)
	if got, want := drain(v), []string{"init", "K1", "p1"}; !eq(got, want) {
		t.Fatalf("recebeu %v, esperado %v", got, want)
	}
}

func TestCameraEspectadorLentoEDesconectado(t *testing.T) {
	c := liveCamera()
	c.setInit(1, []byte("init"), "avc1.640028")
	slow := c.attach()
	for i := 0; i < cap(slow.ch)+10; i++ {
		c.pushFrame(1, []byte("K"), true)
	}
	c.mu.Lock()
	_, still := c.viewers[slow]
	c.mu.Unlock()
	if still {
		t.Fatal("espectador com a fila cheia deveria ter sido desconectado")
	}
}

func TestCameraIgnoraQuadrosDeSessaoAntiga(t *testing.T) {
	c := liveCamera()
	c.gen = 2 // a sessão 1 já foi substituída
	c.setInit(1, []byte("init"), "x")
	c.pushFrame(1, []byte("K"), true)
	if c.initSeg != nil || len(c.gop) != 0 {
		t.Fatal("dados de sessão antiga não podem entrar")
	}
}
