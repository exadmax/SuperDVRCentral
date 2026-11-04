package ai

import (
	"testing"
	"testing/fstest"
)

// Saída real do winVideoScript num notebook com Iris Xe e 24 GB de RAM (AdapterRAM de 2 GB é reserva fictícia).
const irisXeJSON = `{"TotalRAM":25479716864,"GPUs":[{"Name":"Intel(R) Iris(R) Xe Graphics","AdapterRAM":2147479552,"PNPDeviceID":"PCI\\VEN_8086&DEV_9A49&SUBSYS_C195144D&REV_01\\3&11583659&0&10"}],"Reg":[{"Name":"Intel(R) Iris(R) Xe Graphics","Mem":null}]}`

func TestParseWinVideoIrisXe(t *testing.T) {
	gpus := parseWinVideo([]byte(irisXeJSON))
	if len(gpus) != 1 {
		t.Fatalf("gpus = %+v", gpus)
	}
	want := GPU{Name: "Intel(R) Iris(R) Xe Graphics", Vendor: VendorIntel, Integrated: true, DedicatedMB: 0, SharedMB: 12149}
	if gpus[0] != want {
		t.Fatalf("got %+v, want %+v", gpus[0], want)
	}
	if eff := EffectiveMB(gpus); eff != 6074 {
		t.Fatalf("efetiva = %d", eff)
	}
}

func TestParseWinVideoDedicated(t *testing.T) {
	// Intel + NVIDIA + AMD sem nvidia-smi: registro em QWORD na NVIDIA, binário na AMD; Arc sem registro cai no AdapterRAM.
	js := `{"TotalRAM":34359738368,
	 "GPUs":[
	  {"Name":"Intel(R) UHD Graphics","AdapterRAM":1073741824,"PNPDeviceID":"PCI\\VEN_8086&DEV_9BC4"},
	  {"Name":"NVIDIA GeForce RTX 3060 Laptop GPU","AdapterRAM":4293918720,"PNPDeviceID":"PCI\\VEN_10DE&DEV_2520"},
	  {"Name":"AMD Radeon RX 6800","AdapterRAM":4293918720,"PNPDeviceID":"PCI\\VEN_1002&DEV_73BF"},
	  {"Name":"AMD Radeon(TM) Graphics","AdapterRAM":536870912,"PNPDeviceID":"PCI\\VEN_1002&DEV_1638"},
	  {"Name":"Intel(R) Arc(TM) A770 Graphics","AdapterRAM":4293918720,"PNPDeviceID":"PCI\\VEN_8086&DEV_56A0"},
	  {"Name":"Microsoft Basic Display Adapter","AdapterRAM":0,"PNPDeviceID":"ROOT\\BasicDisplay"}],
	 "Reg":[{"Name":"NVIDIA GeForce RTX 3060 Laptop GPU","Mem":6442450944},
	        {"Name":"AMD Radeon RX 6800","Mem":[0,0,0,0,4,0,0,0]}]}`
	gpus := parseWinVideo([]byte(js))
	want := []GPU{
		{Name: "Intel(R) UHD Graphics", Vendor: VendorIntel, Integrated: true, SharedMB: 16384},
		{Name: "NVIDIA GeForce RTX 3060 Laptop GPU", Vendor: VendorNVIDIA, DedicatedMB: 6144},
		{Name: "AMD Radeon RX 6800", Vendor: VendorAMD, DedicatedMB: 16384},
		{Name: "AMD Radeon(TM) Graphics", Vendor: VendorAMD, Integrated: true, SharedMB: 16384},
		{Name: "Intel(R) Arc(TM) A770 Graphics", Vendor: VendorIntel, DedicatedMB: 4095},
		{Name: "Microsoft Basic Display Adapter", Vendor: VendorOther},
	}
	if len(gpus) != len(want) {
		t.Fatalf("gpus = %+v", gpus)
	}
	for i := range want {
		if gpus[i] != want[i] {
			t.Errorf("[%d] got %+v, want %+v", i, gpus[i], want[i])
		}
	}
	if eff := EffectiveMB(gpus); eff != 16384 {
		t.Fatalf("efetiva = %d", eff)
	}
}

func TestParseWinVideoSingleObjectAndGarbage(t *testing.T) {
	js := `{"TotalRAM":8589934592,"GPUs":{"Name":"Intel(R) HD Graphics 620","AdapterRAM":1073741824,"PNPDeviceID":"PCI\\VEN_8086&DEV_5916"},"Reg":null}`
	gpus := parseWinVideo([]byte(js))
	if len(gpus) != 1 || gpus[0].SharedMB != 4096 || !gpus[0].Integrated {
		t.Fatalf("gpus = %+v", gpus)
	}
	if g := parseWinVideo([]byte("Get-CimInstance : erro")); g != nil {
		t.Fatalf("lixo virou %+v", g)
	}
}

func TestParseNvidiaSMI(t *testing.T) {
	out := "NVIDIA GeForce RTX 3060, 12288\r\nNVIDIA RTX A4000, 16376\n\n[N/A], [N/A]\n"
	gpus := parseNvidiaSMI(out)
	want := []GPU{
		{Name: "NVIDIA GeForce RTX 3060", Vendor: VendorNVIDIA, DedicatedMB: 12288},
		{Name: "NVIDIA RTX A4000", Vendor: VendorNVIDIA, DedicatedMB: 16376},
	}
	if len(gpus) != 2 || gpus[0] != want[0] || gpus[1] != want[1] {
		t.Fatalf("got %+v", gpus)
	}
	if parseNvidiaSMI("") != nil {
		t.Fatal("vazio deveria dar nil")
	}
}

func TestDetectDRM(t *testing.T) {
	fsys := fstest.MapFS{
		"proc/meminfo":                                   {Data: []byte("MemTotal:       16303580 kB\nMemFree: 1 kB\n")},
		"sys/class/drm/card0/device/vendor":              {Data: []byte("0x8086\n")},
		"sys/class/drm/card0/device/uevent":              {Data: []byte("DRIVER=i915\nPCI_ID=8086:9A49\n")},
		"sys/class/drm/card0-HDMI-A-1/status":            {Data: []byte("connected\n")},
		"sys/class/drm/card1/device/vendor":              {Data: []byte("0x1002\n")},
		"sys/class/drm/card1/device/uevent":              {Data: []byte("DRIVER=amdgpu\nPCI_ID=1002:73BF\n")},
		"sys/class/drm/card1/device/mem_info_vram_total": {Data: []byte("17163091968\n")},
		"sys/class/drm/card2/device/vendor":              {Data: []byte("0x1002\n")},
		"sys/class/drm/card2/device/mem_info_vram_total": {Data: []byte("536870912\n")},
		"sys/class/drm/card3/device/vendor":              {Data: []byte("0x10de\n")},
		"sys/class/drm/renderD128/device/vendor":         {Data: []byte("0x8086\n")},
	}
	gpus := detectDRM(fsys, true)
	want := []GPU{
		{Name: "Intel 8086:9A49 (i915)", Vendor: VendorIntel, Integrated: true, SharedMB: 7960},
		{Name: "Amd 1002:73BF (amdgpu)", Vendor: VendorAMD, DedicatedMB: 16368},
		{Name: "Amd", Vendor: VendorAMD, Integrated: true, SharedMB: 7960},
	}
	if len(gpus) != len(want) {
		t.Fatalf("gpus = %+v", gpus)
	}
	for i := range want {
		if gpus[i] != want[i] {
			t.Errorf("[%d] got %+v, want %+v", i, gpus[i], want[i])
		}
	}
	if g := detectDRM(fsys, false); len(g) != 4 || g[3].Vendor != VendorNVIDIA || g[3].DedicatedMB != 0 {
		t.Fatalf("sem nvidia-smi: %+v", g)
	}
	if g := detectDRM(fstest.MapFS{}, false); g != nil {
		t.Fatalf("sem /sys: %+v", g)
	}
}

func TestEffectiveMB(t *testing.T) {
	cases := []struct {
		gpus []GPU
		want int
	}{
		{nil, 0},
		{[]GPU{{Vendor: VendorOther, DedicatedMB: 8192}}, 0},
		{[]GPU{{Vendor: VendorIntel, Integrated: true, SharedMB: 12150}}, 6075},
		{[]GPU{{Vendor: VendorIntel, Integrated: true, SharedMB: 16384}, {Vendor: VendorNVIDIA, DedicatedMB: 8192}}, 8192},
		{[]GPU{{Vendor: VendorNVIDIA, DedicatedMB: 8192}, {Vendor: VendorNVIDIA, DedicatedMB: 24576}}, 24576},
	}
	for i, c := range cases {
		if got := EffectiveMB(c.gpus); got != c.want {
			t.Errorf("[%d] = %d, want %d", i, got, c.want)
		}
	}
}
