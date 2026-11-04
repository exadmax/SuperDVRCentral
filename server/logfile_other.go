//go:build !windows

package main

// initLogFile: fora do Windows o log fica no terminal ou no journal do systemd.
func initLogFile(dir string) {}
