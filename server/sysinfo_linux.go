package main

import (
	"bytes"
	"errors"
	"os"
	"strconv"
	"strings"
)

const clockTicks = 100 // USER_HZ: fixo em 100 no Linux

func readSys() (s sysSample, err error) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return s, err
	}
	// "cpu  user nice system idle iowait irq softirq steal ..." (em ticks, todos os núcleos somados)
	if line, _, _ := strings.Cut(string(b), "\n"); strings.HasPrefix(line, "cpu ") {
		f := strings.Fields(line)[1:]
		for i := 0; i < len(f) && i < 8; i++ {
			v, _ := strconv.ParseFloat(f[i], 64)
			s.cpuTotal += v
			if i == 3 || i == 4 { // idle + iowait
				s.cpuIdle += v
			}
		}
	}

	if b, err = os.ReadFile("/proc/meminfo"); err != nil {
		return s, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, _ := strings.Cut(line, ":")
		n, _ := strconv.ParseUint(strings.Fields(v + " 0")[0], 10, 64) // kB
		switch k {
		case "MemTotal":
			s.memTotal = n << 10
		case "MemAvailable":
			s.memAvail = n << 10
		}
	}

	// /proc/self/stat: o nome do processo vem entre parênteses e pode ter espaços, então corta no último ")".
	if b, err = os.ReadFile("/proc/self/stat"); err != nil {
		return s, err
	}
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return s, errors.New("/proc/self/stat inesperado")
	}
	f := strings.Fields(string(b[i+1:]))
	if len(f) > 12 { // f[0] é o campo 3 (estado): utime = campo 14, stime = campo 15
		ut, _ := strconv.ParseFloat(f[11], 64)
		st, _ := strconv.ParseFloat(f[12], 64)
		s.appCPU = (ut + st) / clockTicks
	}
	if b, err = os.ReadFile("/proc/self/statm"); err == nil {
		if f := strings.Fields(string(b)); len(f) > 1 {
			pages, _ := strconv.ParseUint(f[1], 10, 64)
			s.appRSS = pages * uint64(os.Getpagesize())
		}
	}
	return s, nil
}
