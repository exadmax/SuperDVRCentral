package ai

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io/fs"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Tempo máximo de cada comando externo da detecção. Passou disso, a GPU fica de fora (lista vazia), sem derrubar o hub.
const (
	nvidiaSMITimeout = 5 * time.Second
	detectTimeout    = 20 * time.Second // teto da detecção inteira (o PowerShell frio leva 1–3 s)
)

// Fabricantes em GPU.Vendor.
const (
	VendorNVIDIA = "nvidia"
	VendorAMD    = "amd"
	VendorIntel  = "intel"
	VendorOther  = "other"
)

// GPU é uma placa de vídeo vista pelo sistema. Integrada não tem memória dedicada: usa a compartilhada (metade da RAM, regra do Windows,
// adotada também no Linux para dar o mesmo número nos dois).
type GPU struct {
	Name        string `json:"name"`
	Vendor      string `json:"vendor"` // nvidia | amd | intel | other
	Integrated  bool   `json:"integrated"`
	DedicatedMB int    `json:"dedicatedMB"`
	SharedMB    int    `json:"sharedMB"`
}

// DetectGPUs lista as GPUs da máquina (implementação em hardware_<sistema>.go). Falha em qualquer etapa devolve o que deu para ver,
// ou lista vazia; nunca erro.
func DetectGPUs(ctx context.Context) []GPU {
	ctx, cancel := context.WithTimeout(ctx, detectTimeout)
	defer cancel()
	return detectGPUs(ctx)
}

// EffectiveMB é a memória que conta para escolher o modelo: a maior dedicada entre as GPUs; sem nenhuma dedicada, metade da
// compartilhada da integrada (ela disputa com o sistema); sem GPU, 0 (CPU). GPUs "other" (adaptador básico, telas virtuais) não contam.
func EffectiveMB(gpus []GPU) int {
	best, shared := 0, 0
	for _, g := range gpus {
		if g.Vendor == VendorOther {
			continue
		}
		if g.DedicatedMB > best {
			best = g.DedicatedMB
		}
		if g.Integrated && g.SharedMB > shared {
			shared = g.SharedMB
		}
	}
	if best > 0 {
		return best
	}
	return shared / 2
}

// runNvidiaSMI pergunta nome e memória das placas NVIDIA. Sem nvidia-smi (ou sem driver) devolve nil.
func runNvidiaSMI(ctx context.Context) []GPU {
	bin, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, nvidiaSMITimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--query-gpu=name,memory.total", "--format=csv,noheader,nounits")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseNvidiaSMI(string(out))
}

// parseNvidiaSMI lê linhas "NVIDIA GeForce RTX 3060, 12288" (memória em MiB).
func parseNvidiaSMI(out string) []GPU {
	var gpus []GPU
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		i := strings.LastIndex(line, ",")
		if i < 0 {
			continue
		}
		mb, err := strconv.Atoi(strings.TrimSpace(line[i+1:]))
		if err != nil {
			continue
		}
		gpus = append(gpus, GPU{Name: strings.TrimSpace(line[:i]), Vendor: VendorNVIDIA, DedicatedMB: mb})
	}
	return gpus
}

const mib = 1 << 20

// Placas dedicadas da Intel (Arc A/B). As demais Intel (Iris Xe, UHD, a "Arc Graphics" do Meteor Lake) são integradas.
var intelDiscrete = regexp.MustCompile(`(?i)\barc(\(tm\))?\s+[ab]\d`)

// Placas dedicadas da AMD pelo nome. As demais Radeon (Radeon Graphics, Vega 8, 780M) são APUs integradas.
var amdDiscrete = regexp.MustCompile(`(?i)\b(rx|pro|firepro|vega\s*(56|64)|radeon\s+vii)\b`)

// pnpVendor tira o fabricante do PNPDeviceID ("PCI\VEN_8086&DEV_9A49...").
var pnpVendor = regexp.MustCompile(`(?i)VEN_([0-9a-f]{4})`)

// vendorFromPCI traduz o ID de fabricante PCI ("8086", "0x8086", "VEN_8086").
func vendorFromPCI(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	id = strings.TrimPrefix(strings.TrimPrefix(id, "0x"), "ven_")
	switch id {
	case "10de":
		return VendorNVIDIA
	case "1002", "1022":
		return VendorAMD
	case "8086":
		return VendorIntel
	}
	return VendorOther
}

// isIntegrated decide pelo fabricante e pelo nome (o Windows não diz se a placa é integrada).
func isIntegrated(vendor, name string) bool {
	switch vendor {
	case VendorIntel:
		return !intelDiscrete.MatchString(name)
	case VendorAMD:
		return !amdDiscrete.MatchString(name)
	}
	return false
}

// winVideoReport é a saída JSON do comando PowerShell de hardware_windows.go.
type winVideoReport struct {
	TotalRAM int64             `json:"TotalRAM"`
	GPUs     flexList[winCtrl] `json:"GPUs"`
	Reg      flexList[winReg]  `json:"Reg"`
}

type winCtrl struct {
	Name        string `json:"Name"`
	AdapterRAM  int64  `json:"AdapterRAM"`
	PNPDeviceID string `json:"PNPDeviceID"`
}

// winReg é uma subchave da classe de vídeo no registro; Mem é HardwareInformation.qwMemorySize (QWORD; alguns drivers gravam binário).
type winReg struct {
	Name string          `json:"Name"`
	Mem  json.RawMessage `json:"Mem"`
}

// flexList aceita lista ou objeto único: o ConvertTo-Json do PowerShell 5.1 às vezes desembrulha listas de um item.
type flexList[T any] []T

func (l *flexList[T]) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		return nil
	}
	if strings.HasPrefix(s, "[") {
		var v []T
		err := json.Unmarshal(b, &v)
		*l = v
		return err
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*l = flexList[T]{v}
	return nil
}

// regBytes lê o qwMemorySize: número, ou lista de bytes (REG_BINARY, little-endian).
func regBytes(raw json.RawMessage) int64 {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var n int64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var bs []byte
	var ints []int
	if json.Unmarshal(raw, &ints) == nil {
		for _, v := range ints {
			bs = append(bs, byte(v))
		}
		for len(bs) < 8 {
			bs = append(bs, 0)
		}
		return int64(binary.LittleEndian.Uint64(bs[:8]))
	}
	return 0
}

// parseWinVideo monta as GPUs a partir do Win32_VideoController + registro. AdapterRAM é uint32 (trava em 4 GB) e, na Intel, é uma
// reserva fictícia (a Iris Xe diz 2 GB): por isso integrada fica com DedicatedMB 0 e SharedMB = RAM/2; dedicada usa o qwMemorySize do
// registro e só cai no AdapterRAM como último recurso.
func parseWinVideo(b []byte) []GPU {
	var rep winVideoReport
	if json.Unmarshal(b, &rep) != nil {
		return nil
	}
	regMem := map[string]int64{}
	for _, r := range rep.Reg {
		if m := regBytes(r.Mem); m > regMem[r.Name] {
			regMem[r.Name] = m
		}
	}
	sharedMB := int(rep.TotalRAM / 2 / mib)
	var gpus []GPU
	for _, c := range rep.GPUs {
		g := GPU{Name: strings.TrimSpace(c.Name), Vendor: VendorOther}
		if m := pnpVendor.FindStringSubmatch(c.PNPDeviceID); m != nil {
			g.Vendor = vendorFromPCI(m[1])
		}
		g.Integrated = isIntegrated(g.Vendor, g.Name)
		switch {
		case g.Integrated:
			g.SharedMB = sharedMB
		case regMem[c.Name] > 0:
			g.DedicatedMB = int(regMem[c.Name] / mib)
		case g.Vendor != VendorOther:
			g.DedicatedMB = int(c.AdapterRAM / mib)
		}
		gpus = append(gpus, g)
	}
	return gpus
}

// parseMemTotalMB lê o MemTotal do /proc/meminfo (em kB) e devolve MB.
func parseMemTotalMB(meminfo string) int {
	for _, line := range strings.Split(meminfo, "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "MemTotal:" {
			kb, _ := strconv.Atoi(f[1])
			return kb / 1024
		}
	}
	return 0
}

// amdAPUVRAMMB: abaixo disso o "VRAM" da AMD no Linux é a reserva de uma APU (512 MB por padrão), e ela é tratada como integrada.
// Reserva maior configurada na BIOS é memória separada de fato e conta como dedicada.
const amdAPUVRAMMB = 2048

var drmCard = regexp.MustCompile(`^card\d+$`)

// detectDRM varre /sys/class/drm/card*/device (fsys é a raiz "/", trocável nos testes). NVIDIA fica de fora quando skipNVIDIA
// (já veio do nvidia-smi, com memória); sem ele aparece com memória 0 (desconhecida).
func detectDRM(fsys fs.FS, skipNVIDIA bool) []GPU {
	entries, err := fs.ReadDir(fsys, "sys/class/drm")
	if err != nil {
		return nil
	}
	memMB := 0
	if b, err := fs.ReadFile(fsys, "proc/meminfo"); err == nil {
		memMB = parseMemTotalMB(string(b))
	}
	read := func(p string) string {
		b, _ := fs.ReadFile(fsys, p)
		return strings.TrimSpace(string(b))
	}
	var gpus []GPU
	for _, e := range entries {
		if !drmCard.MatchString(e.Name()) {
			continue // card0-HDMI-A-1 e afins são conectores
		}
		dev := "sys/class/drm/" + e.Name() + "/device/"
		vid := read(dev + "vendor")
		if vid == "" {
			continue
		}
		g := GPU{Vendor: vendorFromPCI(vid)}
		if g.Vendor == VendorNVIDIA && skipNVIDIA {
			continue
		}
		driver, pciID := "", ""
		for _, l := range strings.Split(read(dev+"uevent"), "\n") {
			if k, v, ok := strings.Cut(l, "="); ok {
				switch k {
				case "DRIVER":
					driver = v
				case "PCI_ID":
					pciID = v
				}
			}
		}
		g.Name = strings.TrimSpace(strings.ToUpper(g.Vendor[:1]) + g.Vendor[1:] + " " + pciID)
		if driver != "" {
			g.Name += " (" + driver + ")"
		}
		vram, _ := strconv.ParseInt(read(dev+"mem_info_vram_total"), 10, 64)
		vramMB := int(vram / mib)
		switch {
		case g.Vendor == VendorIntel:
			g.Integrated = true
		case g.Vendor == VendorAMD && vramMB < amdAPUVRAMMB:
			g.Integrated = true
		default:
			g.DedicatedMB = vramMB
		}
		if g.Integrated {
			g.SharedMB = memMB / 2
		}
		gpus = append(gpus, g)
	}
	return gpus
}
