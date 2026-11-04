//go:build !linux && !windows

package ai

import (
	"context"
	"os/exec"
)

func hideWindow(*exec.Cmd) {}

// detectGPUs: fora do Windows e do Linux só o nvidia-smi, se houver.
func detectGPUs(ctx context.Context) []GPU {
	return runNvidiaSMI(ctx)
}
