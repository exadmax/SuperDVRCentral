package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")
)

func ft(f syscall.Filetime) float64 { return float64(uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime)) } // 100 ns

func readSys() (s sysSample, err error) {
	var idle, kern, user syscall.Filetime
	if r, _, e := kernel32.NewProc("GetSystemTimes").Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kern)), uintptr(unsafe.Pointer(&user))); r == 0 {
		return s, e
	}
	s.cpuIdle, s.cpuTotal = ft(idle), ft(kern)+ft(user) // o tempo de kernel já inclui o ocioso

	var ms struct {
		length, load                               uint32
		total, avail, pageTotal, pageAvail, vTotal uint64
		vAvail, extAvail                           uint64
	}
	ms.length = uint32(unsafe.Sizeof(ms))
	if r, _, e := kernel32.NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&ms))); r == 0 {
		return s, e
	}
	s.memTotal, s.memAvail = ms.total, ms.avail

	var created, exit, k, u syscall.Filetime
	self, _ := syscall.GetCurrentProcess()
	if err = syscall.GetProcessTimes(self, &created, &exit, &k, &u); err == nil {
		s.appCPU = (ft(k) + ft(u)) / 1e7
	}
	var pm struct {
		cb, pageFaults                   uint32
		peakWS, ws, qpPeakP, qpP, qnPeak uintptr
		qnP, pagefile, peakPagefile      uintptr
	}
	pm.cb = uint32(unsafe.Sizeof(pm))
	if r, _, _ := psapi.NewProc("GetProcessMemoryInfo").Call(uintptr(self), uintptr(unsafe.Pointer(&pm)), uintptr(pm.cb)); r != 0 {
		s.appRSS = uint64(pm.ws)
	}
	return s, nil
}
