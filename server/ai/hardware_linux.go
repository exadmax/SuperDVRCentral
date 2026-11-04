package ai

import (
	"context"
	"os"
	"os/exec"
)

func hideWindow(*exec.Cmd) {}

// detectGPUs: NVIDIA pelo nvidia-smi (quando existe) e as demais pelo /sys/class/drm (amdgpu informa a VRAM; Intel é integrada).
func detectGPUs(ctx context.Context) []GPU {
	nv := runNvidiaSMI(ctx)
	return append(nv, detectDRM(os.DirFS("/"), len(nv) > 0)...)
}
