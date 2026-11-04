// Detecção de movimento leve: não decodifica o vídeo. No H.264, quadros P de uma cena parada são minúsculos e crescem muito
// quando algo se mexe; basta comparar o tamanho de cada quadro com a média recente da própria câmera. Custo: alguns números por câmera.
package main

import (
	"math/bits"
	"time"
)

const (
	motionWarmup   = 30               // quadros P para aprender o "normal" da câmera antes de detectar
	motionPostRoll = 15 * time.Second // continua gravando depois do último movimento
	motionFloor    = 1500.0           // bytes acima da média: ruído abaixo disso nunca conta como movimento
)

// motionFactor: quantas vezes acima da média um quadro P precisa estar (sensibilidade 1 = menos sensível ... 5 = mais).
var motionFactor = [6]float64{0, 3.2, 2.6, 2.0, 1.6, 1.3}

type motionDet struct {
	ema    float64   // tamanho médio de um quadro P "parado"
	n      int       // quadros P vistos
	recent uint8     // últimos 8 quadros P: 1 = acima do limite
	until  time.Time // movimento considerado ativo até aqui
}

func (d *motionDet) reset() { *d = motionDet{} }

// feed recebe cada quadro e diz se há movimento agora. Quadros-chave (sempre grandes) não entram na conta.
func (d *motionDet) feed(size int, key bool, now time.Time, sens int) bool {
	if sens < 1 || sens > 5 {
		sens = 3
	}
	if key {
		return now.Before(d.until)
	}
	sz := float64(size)
	if d.n < motionWarmup {
		if d.n == 0 {
			d.ema = sz
		} else {
			d.ema += (sz - d.ema) * 0.1
		}
		d.n++
		return false
	}
	thr := d.ema * motionFactor[sens]
	if min := d.ema + motionFloor; thr < min {
		thr = min
	}
	hit := sz > thr
	d.recent <<= 1
	if hit {
		d.recent |= 1
		d.ema += (sz - d.ema) * 0.002 // movimento prolongado (luz acesa, TV) vira o novo normal em alguns minutos
	} else {
		d.ema += (sz - d.ema) * 0.02
	}
	if bits.OnesCount8(d.recent) >= 3 { // 3 de 8 quadros: ignora um pico isolado
		d.until = now.Add(motionPostRoll)
	}
	return now.Before(d.until)
}

func (d *motionDet) active(now time.Time) bool { return now.Before(d.until) }
