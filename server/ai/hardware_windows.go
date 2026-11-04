package ai

import (
	"context"
	"os/exec"
	"syscall"
	"time"
)

// Tempo máximo do PowerShell (a primeira execução depois do boot é a mais lenta).
const powershellTimeout = 15 * time.Second

// createNoWindow evita a janela de console piscando quando o hub roda fora de um terminal.
const createNoWindow = 0x08000000

// winVideoScript junta num comando só (uma execução) a RAM total, as placas do Win32_VideoController e o qwMemorySize de cada
// subchave da classe de vídeo {4d36e968-...} no registro, em JSON.
const winVideoScript = `$ErrorActionPreference='SilentlyContinue'
$ram=(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory
$reg=@(Get-ChildItem 'HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}' | ForEach-Object { $p=Get-ItemProperty $_.PSPath; if ($p.DriverDesc) { [pscustomobject]@{Name=$p.DriverDesc; Mem=$p.'HardwareInformation.qwMemorySize'} } })
$gpus=@(Get-CimInstance Win32_VideoController | ForEach-Object { [pscustomobject]@{Name=$_.Name; AdapterRAM=$_.AdapterRAM; PNPDeviceID=$_.PNPDeviceID} })
[pscustomobject]@{TotalRAM=$ram; GPUs=$gpus; Reg=$reg} | ConvertTo-Json -Depth 4 -Compress`

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// detectGPUs: nvidia-smi quando existe (memória exata); senão PowerShell/CIM + registro.
func detectGPUs(ctx context.Context) []GPU {
	if gpus := runNvidiaSMI(ctx); len(gpus) > 0 {
		return gpus
	}
	ctx, cancel := context.WithTimeout(ctx, powershellTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", winVideoScript)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseWinVideo(out)
}
