// Câmeras ao vivo: o hub lê o RTSP (H.264) de cada câmera e entrega ao navegador como MP4 fragmentado contínuo
// (/api/cameras/{id}/stream), que a página toca com Media Source Extensions. O hub só repassa o vídeo (sem recodificar),
// então não gasta CPU nem precisa de ffmpeg. A URL da câmera (com a senha) fica só no hub.config.json: o navegador nunca a vê.
// A conexão com a câmera só existe enquanto alguém assiste (fecha 15 s depois do último espectador), compartilhada entre todos,
// ou enquanto a gravação (recorder.go) está ligada para ela.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4/seekablebuffer"
	"github.com/pion/rtp"
)

type CameraConfig struct {
	ID   string `json:"id"`   // usado na URL da API
	Name string `json:"name"` // nome exibido em cima da imagem
	URL  string `json:"url"`  // rtsp://usuario:senha@ip:554/cam/realmonitor?channel=1&subtype=1 (senha com %40 no lugar de @ etc.)
}

const (
	camTimescale   = 90000            // relógio RTP do H.264
	camIdleClose   = 15 * time.Second // sem espectadores, derruba a conexão com a câmera
	camRetry       = 4 * time.Second
	camMaxGOPParts = 900 // teto do cache do último grupo de quadros (memória limitada)
)

type camViewer struct {
	ch      chan []byte
	gotInit bool // já recebeu o segmento de inicialização
	synced  bool // já recebeu um quadro-chave (só então passa a receber os quadros seguintes)
}

type camera struct {
	cfg CameraConfig

	mu      sync.Mutex
	viewers map[*camViewer]struct{}
	running bool
	gen     int // identifica a sessão atual; quadros de sessões antigas são descartados
	stop    context.CancelFunc
	idle    *time.Timer
	initSeg []byte
	codec   string   // "avc1.64001F": o navegador precisa dele para abrir o MP4
	gop     [][]byte // do último quadro-chave até agora: quem chega no meio já começa com imagem
	lastErr string
	wantRec bool      // gravação ligada: mantém a conexão mesmo sem espectadores
	rec     *recorder // gravação em arquivos de 1 hora (recorder.go)

	snapMu sync.Mutex // um ffmpeg por vez por câmera
	snapAt time.Time
	snap   []byte // último JPEG (miniatura da tela inicial)
}

type cameras struct {
	mu    sync.RWMutex // a lista muda quando a página Config adiciona ou remove câmeras (settings.go)
	list  []*camera
	byID  map[string]*camera
	onAdd func(*camera) // chamado para cada câmera nova, antes de ela aparecer na lista (liga o gravador)
}

func newCamera(cc CameraConfig) *camera {
	return &camera{cfg: cc, viewers: map[*camViewer]struct{}{}}
}

func newCameras(cfgs []CameraConfig) *cameras {
	cs := &cameras{byID: map[string]*camera{}}
	for _, cc := range cfgs {
		if cc.ID == "" || cc.URL == "" {
			continue
		}
		if cc.Name == "" {
			cc.Name = cc.ID
		}
		c := newCamera(cc)
		cs.list = append(cs.list, c)
		cs.byID[cc.ID] = c
	}
	return cs
}

// all devolve uma cópia da lista atual (seguro para percorrer enquanto a lista muda).
func (cs *cameras) all() []*camera {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return append([]*camera(nil), cs.list...)
}

func (cs *cameras) get(id string) *camera {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.byID[id]
}

// replace troca o conjunto de câmeras em funcionamento: as iguais seguem como estão (sem derrubar vídeo nem gravação),
// as novas ou alteradas são criadas e as que saíram são desligadas.
func (cs *cameras) replace(cfgs []CameraConfig) {
	cs.mu.Lock()
	old := cs.byID
	cs.list, cs.byID = nil, map[string]*camera{}
	for _, cc := range cfgs {
		if cc.ID == "" || cc.URL == "" || cs.byID[cc.ID] != nil {
			continue
		}
		if cc.Name == "" {
			cc.Name = cc.ID
		}
		c := old[cc.ID]
		if c != nil && c.cfg == cc {
			delete(old, cc.ID)
		} else {
			c = newCamera(cc)
			if cs.onAdd != nil {
				cs.onAdd(c)
			}
		}
		cs.list = append(cs.list, c)
		cs.byID[cc.ID] = c
	}
	cs.mu.Unlock()
	for _, c := range old { // sobrou: removida ou substituída por uma versão nova
		c.shutdown()
	}
}

// shutdown encerra a conexão com a câmera, desconecta quem assistia e fecha o arquivo de gravação.
func (c *camera) shutdown() {
	c.mu.Lock()
	c.wantRec = false
	if c.idle != nil {
		c.idle.Stop()
		c.idle = nil
	}
	if c.running {
		c.running = false
		c.stop()
	}
	c.resetLocked()
	for v := range c.viewers {
		c.removeLocked(v)
	}
	c.mu.Unlock()
	if c.rec != nil {
		c.rec.close()
	}
}

// ---- espectadores ----

// push entrega uma mensagem ao espectador sem bloquear; quem não acompanha (fila cheia) é desconectado e o navegador reconecta.
func (c *camera) push(v *camViewer, b []byte) {
	select {
	case v.ch <- b:
	default:
		c.removeLocked(v)
	}
}

func (c *camera) removeLocked(v *camViewer) {
	if _, ok := c.viewers[v]; ok {
		delete(c.viewers, v)
		close(v.ch)
	}
}

// attach registra um espectador e já entrega o que existe (init e último grupo de quadros).
func (c *camera) attach() *camViewer {
	v := &camViewer{ch: make(chan []byte, 256)}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.idle != nil {
		c.idle.Stop()
		c.idle = nil
	}
	c.viewers[v] = struct{}{}
	if c.initSeg != nil {
		v.gotInit = true
		c.push(v, c.initSeg)
		if len(c.gop) > 0 {
			v.synced = true
			for _, p := range c.gop {
				c.push(v, p)
			}
		}
	}
	return v
}

func (c *camera) startLocked() {
	if !c.running {
		c.running = true
		c.gen++
		ctx, cancel := context.WithCancel(context.Background())
		c.stop = cancel
		go c.run(ctx, c.gen)
	}
}

func (c *camera) subscribe() *camViewer {
	v := c.attach()
	c.mu.Lock()
	c.startLocked()
	c.mu.Unlock()
	return v
}

// idleLocked agenda o fim da conexão quando não sobrou espectador nem gravação.
func (c *camera) idleLocked() {
	if len(c.viewers) == 0 && !c.wantRec && c.running && c.idle == nil {
		c.idle = time.AfterFunc(camIdleClose, func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.idle = nil
			if len(c.viewers) == 0 && !c.wantRec && c.running {
				c.running = false
				c.stop()
				c.resetLocked()
			}
		})
	}
}

func (c *camera) unsubscribe(v *camViewer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(v)
	c.idleLocked()
}

// setRecording liga ou desliga a gravação: ligada, a conexão com a câmera fica aberta mesmo sem ninguém assistindo.
func (c *camera) setRecording(on bool) {
	c.mu.Lock()
	c.wantRec = on
	if on {
		if c.idle != nil {
			c.idle.Stop()
			c.idle = nil
		}
		c.startLocked()
	} else {
		c.idleLocked()
	}
	init := c.initSeg
	c.mu.Unlock()
	if c.rec == nil {
		return
	}
	if !on {
		c.rec.close()
	} else if init != nil {
		c.rec.ensureInit(init)
	}
}

func (c *camera) resetLocked() {
	c.initSeg, c.codec, c.gop = nil, "", nil
	for v := range c.viewers {
		v.gotInit, v.synced = false, false
	}
}

func (c *camera) setInit(gen int, init []byte, codec string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running || gen != c.gen {
		return
	}
	c.initSeg, c.codec, c.lastErr = init, codec, ""
	for v := range c.viewers {
		if !v.gotInit {
			v.gotInit = true
			c.push(v, init)
		}
	}
}

func (c *camera) pushFrame(gen int, frag []byte, key bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running || gen != c.gen {
		return
	}
	if key {
		c.gop = append(c.gop[:0], frag)
	} else if len(c.gop) > 0 && len(c.gop) < camMaxGOPParts {
		c.gop = append(c.gop, frag)
	}
	for v := range c.viewers {
		if !v.gotInit {
			continue
		}
		if !v.synced {
			if !key {
				continue
			}
			v.synced = true
		}
		c.push(v, frag)
	}
}

// ---- conexão com a câmera ----

func (c *camera) run(ctx context.Context, gen int) {
	var lastLog string
	var lastLogAt time.Time
	for ctx.Err() == nil {
		err := c.session(ctx, gen)
		if ctx.Err() != nil {
			return
		}
		c.mu.Lock()
		if gen == c.gen {
			c.lastErr = err.Error()
			c.resetLocked()
		}
		c.mu.Unlock()
		if msg := err.Error(); msg != lastLog || time.Since(lastLogAt) > 10*time.Minute { // câmera fora do ar não enche o log
			lastLog, lastLogAt = msg, time.Now()
			log.Printf("câmera %s: %v (nova tentativa em %s)", c.cfg.ID, err, camRetry)
		}
		select {
		case <-ctx.Done():
		case <-time.After(camRetry):
		}
	}
}

func (c *camera) session(ctx context.Context, gen int) error {
	u, err := base.ParseURL(c.cfg.URL)
	if err != nil {
		return errors.New("URL RTSP inválida no hub.config.json")
	}
	proto := gortsplib.ProtocolTCP // TCP: sem perda de pacotes no Wi-Fi
	cl := gortsplib.Client{Scheme: u.Scheme, Host: u.Host, Protocol: &proto, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	if err := cl.Start(); err != nil {
		return err
	}
	defer cl.Close()

	desc, _, err := cl.Describe(u)
	if err != nil {
		return err
	}
	var forma *format.H264
	medi := desc.FindFormat(&forma)
	if medi == nil {
		return errors.New("a câmera não entrega H.264 neste endereço (use o substream: subtype=1)")
	}
	dec, err := forma.CreateDecoder()
	if err != nil {
		return err
	}
	if _, err = cl.Setup(desc.BaseURL, medi, 0, 0); err != nil {
		return err
	}

	var (
		sps, pps           = forma.SPS, forma.PPS
		haveInit, havePrev bool
		firstPTS, prevPTS  int64
		seq                uint32
	)
	cl.OnPacketRTP(medi, forma, func(pkt *rtp.Packet) {
		pts, ok := cl.PacketPTS(medi, pkt)
		if !ok {
			return
		}
		au, err := dec.Decode(pkt)
		if err != nil { // "mais pacotes": o quadro ainda está chegando
			return
		}
		nalus := make([][]byte, 0, len(au))
		for _, n := range au {
			if len(n) == 0 {
				continue
			}
			switch h264.NALUType(n[0] & 0x1F) {
			case h264.NALUTypeSPS:
				sps = append([]byte(nil), n...)
			case h264.NALUTypePPS:
				pps = append([]byte(nil), n...)
			case h264.NALUTypeAccessUnitDelimiter: // o init já leva SPS/PPS
			default:
				nalus = append(nalus, n)
			}
		}
		if len(nalus) == 0 {
			return
		}
		if !haveInit {
			if len(sps) < 4 || len(pps) == 0 {
				return
			}
			init, err := buildInit(sps, pps)
			if err != nil {
				log.Printf("câmera %s: init: %v", c.cfg.ID, err)
				return
			}
			c.setInit(gen, init, fmt.Sprintf("avc1.%02X%02X%02X", sps[1], sps[2], sps[3]))
			if c.rec != nil {
				c.rec.setInit(init)
			}
			haveInit, firstPTS = true, pts
		}
		dur := int64(camTimescale / 15) // duração do primeiro quadro: estimativa
		if havePrev && pts > prevPTS {
			dur = pts - prevPTS
		}
		if dur > 2*camTimescale {
			dur = 2 * camTimescale
		}
		prevPTS, havePrev = pts, true

		sample, err := fmp4.NewSampleH264(0, nalus)
		if err != nil {
			return
		}
		sample.Duration = uint32(dur)
		seq++
		var buf seekablebuffer.Buffer
		part := fmp4.Part{SequenceNumber: seq, Tracks: []*fmp4.PartTrack{{ID: 1, BaseTime: uint64(pts - firstPTS), Samples: []*fmp4.Sample{sample}}}}
		if err := part.Marshal(&buf); err != nil {
			return
		}
		c.pushFrame(gen, buf.Bytes(), !sample.IsNonSyncSample)
		if c.rec != nil {
			c.rec.sample(sample, !sample.IsNonSyncSample)
		}
	})

	if _, err = cl.Play(nil); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cl.Wait() }()
	select {
	case <-ctx.Done():
		return nil
	case err := <-done:
		return err
	}
}

func buildInit(sps, pps []byte) ([]byte, error) {
	var buf seekablebuffer.Buffer
	init := fmp4.Init{Tracks: []*fmp4.InitTrack{{ID: 1, TimeScale: camTimescale, Codec: &fmp4.CodecH264{SPS: sps, PPS: pps}}}}
	if err := init.Marshal(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---- HTTP ----

func (cs *cameras) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/cameras", func(w http.ResponseWriter, r *http.Request) {
		list := cs.all()
		out := make([]map[string]string, 0, len(list))
		for _, c := range list {
			out = append(out, map[string]string{"id": c.cfg.ID, "name": c.cfg.Name})
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("GET /api/cameras/{id}/stream", cs.stream)
	mux.HandleFunc("GET /api/cameras/{id}/snapshot.jpg", cs.snapshot)
}

func (cs *cameras) stream(w http.ResponseWriter, r *http.Request) {
	c := cs.get(r.PathValue("id"))
	if c == nil {
		http.Error(w, "câmera não encontrada", http.StatusNotFound)
		return
	}
	v := c.subscribe()
	defer c.unsubscribe(v)

	var first []byte
	select {
	case b, ok := <-v.ch:
		if !ok {
			http.Error(w, "conexão encerrada", http.StatusServiceUnavailable)
			return
		}
		first = b
	case <-time.After(15 * time.Second):
		c.mu.Lock()
		msg := c.lastErr
		c.mu.Unlock()
		if msg == "" {
			msg = "sem imagem"
		}
		http.Error(w, "câmera indisponível: "+msg, http.StatusServiceUnavailable)
		return
	case <-r.Context().Done():
		return
	}
	c.mu.Lock()
	codec := c.codec
	c.mu.Unlock()

	h := w.Header()
	h.Set("Content-Type", "video/mp4")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Codec", codec)
	h.Set("Access-Control-Expose-Headers", "X-Codec")
	rc := http.NewResponseController(w)
	write := func(b []byte) bool {
		rc.SetWriteDeadline(time.Now().Add(15 * time.Second)) // o WriteTimeout do servidor (60 s) cortaria o vídeo contínuo
		if _, err := w.Write(b); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !write(first) {
		return
	}
	for {
		select {
		case b, ok := <-v.ch:
			if !ok || !write(b) {
				return
			}
		case <-time.After(20 * time.Second): // a câmera parou de mandar imagem: o navegador reconecta
			return
		case <-r.Context().Done():
			return
		}
	}
}

// ---- miniatura (JPEG) ----

const camSnapTTL = 4 * time.Second // várias telas pedindo ao mesmo tempo dividem o mesmo JPEG

// snapshot devolve um JPEG pequeno do último quadro-chave. O hub não decodifica vídeo: quem faz isso é o ffmpeg do
// sistema (apt install ffmpeg), chamado só aqui. Sem ffmpeg a resposta é 501 e a tela inicial simplesmente esconde as miniaturas.
func (cs *cameras) snapshot(w http.ResponseWriter, r *http.Request) {
	c := cs.get(r.PathValue("id"))
	if c == nil {
		http.Error(w, "câmera não encontrada", http.StatusNotFound)
		return
	}
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		http.Error(w, "ffmpeg não instalado no servidor", http.StatusNotImplemented)
		return
	}
	c.snapMu.Lock()
	defer c.snapMu.Unlock()
	if c.snap == nil || time.Since(c.snapAt) > camSnapTTL {
		jpg, err := c.grabJPEG(r.Context(), bin)
		if err != nil {
			http.Error(w, "sem imagem: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		c.snap, c.snapAt = jpg, time.Now()
	}
	h := w.Header()
	h.Set("Content-Type", "image/jpeg")
	h.Set("Cache-Control", "private, max-age=30") // a URL leva um carimbo de tempo; refazer a tela não baixa de novo
	w.Write(c.snap)
}

func (c *camera) grabJPEG(ctx context.Context, bin string) ([]byte, error) {
	v := c.subscribe()
	defer c.unsubscribe(v)
	var in bytes.Buffer
	parts := 0
	timeout := time.After(10 * time.Second)
	for parts < 2 { // segmento de inicialização + quadro-chave
		select {
		case b, ok := <-v.ch:
			if !ok {
				return nil, errors.New("conexão encerrada")
			}
			in.Write(b)
			parts++
		case <-timeout:
			return nil, errors.New("câmera indisponível")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, bin, "-loglevel", "error", "-f", "mp4", "-i", "pipe:0", "-frames:v", "1",
		"-vf", "scale=320:-2", "-q:v", "6", "-f", "mjpeg", "pipe:1")
	cmd.Stdin = &in
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil || out.Len() == 0 {
		return nil, errors.New("falha ao gerar a imagem")
	}
	return out.Bytes(), nil
}
