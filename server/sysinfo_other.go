//go:build !linux && !windows

package main

import "errors"

func readSys() (sysSample, error) { return sysSample{}, errors.New("sem leitura de CPU/memória nesta plataforma") }
