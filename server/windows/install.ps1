# Instalador do DVR para Windows, em janela (sem terminal). Abra pelo Instalar.cmd.
# Pergunta como o DVR será usado:
#   servidor : roda como tarefa do sistema, sobe com o Windows e atende a rede da casa (precisa de administrador; porta 80 ou 8080)
#   pessoal  : roda na sua sessão, só neste PC (localhost:8080), sem abrir nada na rede
# Banco, configuração e livros ficam na pasta de dados e são mantidos ao reinstalar ou atualizar.
# Parâmetros (uso avançado/automação): -Modo servidor|pessoal  -Silencioso  -Desinstalar [-ApagarDados]  -Pasta <dir>  -Porta <n>  -SemRegistro
param(
  [ValidateSet('servidor', 'pessoal')][string]$Modo,
  [switch]$Executar,
  [switch]$Desinstalar,
  [switch]$ApagarDados,
  [switch]$Silencioso,
  [string]$Pasta,
  [int]$Porta,
  [switch]$SemRegistro
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms, System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()

$Aqui = $PSScriptRoot
$Tarefa = 'DVRHub'
$Regra = 'DVR (hub)'
$ChaveReg = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\DVR'
$Exe = 'dvr-hub.exe'
$VersaoRe = '^\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$'

function Test-Admin {
  ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-Destino([string]$modo) {
  if ($Pasta) { return $Pasta }
  if ($modo -eq 'servidor') { return (Join-Path $env:ProgramData 'DVR') }
  return (Join-Path $env:LOCALAPPDATA 'DVR')
}

function Get-Atalho([string]$modo) {
  if ($modo -eq 'servidor') { return (Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\DVR.url') }
  return (Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\DVR.url')
}

function Write-Texto([string]$caminho, [string]$texto, [bool]$bom = $false) {
  [IO.File]::WriteAllText($caminho, $texto, (New-Object Text.UTF8Encoding $bom))
}

# Instalação existente: lê o registro (HKLM do modo servidor, HKCU do pessoal). Devolve $null se não houver.
function Find-Instalacao {
  foreach ($raiz in @('HKLM:', 'HKCU:')) {
    $k = Join-Path $raiz $ChaveReg
    if (Test-Path $k) {
      $p = Get-ItemProperty $k
      if ($p.InstallLocation -and (Test-Path $p.InstallLocation)) {
        $ver = ''
        $vf = Join-Path $p.InstallLocation 'VERSION'
        if (Test-Path $vf) { $ver = (Get-Content $vf -Raw).Trim() }
        $m = 'pessoal'
        if ($raiz -eq 'HKLM:') { $m = 'servidor' }
        return [pscustomobject]@{ Pasta = $p.InstallLocation; Modo = $m; Versao = $ver; Raiz = $raiz }
      }
    }
  }
  return $null
}

function Stop-Hub([string]$destino) {
  Stop-ScheduledTask -TaskName $Tarefa -ErrorAction SilentlyContinue
  $alvo = Join-Path $destino $Exe
  Get-Process -Name ($Exe -replace '\.exe$', '') -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $alvo } | Stop-Process -Force -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 800
}

function Test-PortaLivre([int]$p) {
  return -not (Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue)
}

function Set-Prop($obj, [string]$nome, $valor) {
  if ($obj.PSObject.Properties[$nome]) { $obj.$nome = $valor } else { $obj | Add-Member -NotePropertyName $nome -NotePropertyValue $valor }
}

function Install-Central([string]$modo, [scriptblock]$log) {
  $D = Get-Destino $modo
  $exeOrigem = Join-Path $Aqui 'hub-windows-amd64.exe'
  if (-not (Test-Path $exeOrigem) -or -not (Test-Path (Join-Path $Aqui 'www'))) {
    throw 'Não encontrei os arquivos do pacote. Extraia o .zip inteiro e abra o Instalar.cmd de dentro da pasta extraída.'
  }
  if ($modo -eq 'servidor' -and -not $SemRegistro -and -not (Test-Admin)) { throw 'O modo servidor precisa ser executado como administrador.' }

  # Versão: o pacote de release (arquivo RELEASE) manda; sem ele, cada instalação soma 1 ao terceiro número.
  $velha = '1.0.0'
  $vf = Join-Path $D 'VERSION'
  if (Test-Path $vf) { $velha = (Get-Content $vf -Raw).Trim() }
  if ($velha -notmatch $VersaoRe) { throw "O arquivo $vf está inválido ('$velha'). Esperado algo como 1.0.1." }
  $p = (($velha -split '-')[0]) -split '\.'
  $nova = '{0}.{1}.{2}' -f $p[0], $p[1], ([int]$p[2] + 1)
  $rf = Join-Path $Aqui 'RELEASE'
  if (Test-Path $rf) {
    $nova = (Get-Content $rf -Raw).Trim()
    if ($nova -notmatch $VersaoRe) { throw "Arquivo RELEASE inválido: '$nova'." }
  }

  & $log 'Parando o DVR, se estiver rodando...'
  Stop-Hub $D

  & $log "Copiando arquivos para $D ..."
  New-Item -ItemType Directory -Force -Path $D | Out-Null
  Copy-Item $exeOrigem (Join-Path $D $Exe) -Force
  Unblock-File (Join-Path $D $Exe)
  $www = Join-Path $D 'www'
  if (Test-Path $www) { Remove-Item $www -Recurse -Force }
  Copy-Item (Join-Path $Aqui 'www') $www -Recurse -Force
  Copy-Item (Join-Path $Aqui 'install.ps1') (Join-Path $D 'install.ps1') -Force
  Write-Texto (Join-Path $D 'Desinstalar.cmd') "@echo off`r`ncd /d `"%~dp0`"`r`nstart `"`" powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"%~dp0install.ps1`" -Desinstalar`r`n"

  # Configuração: só na primeira instalação. Depois disso vale o arquivo do usuário (e a página Config do painel).
  $cfgPath = Join-Path $D 'hub.config.json'
  $porta = $Porta
  if (-not (Test-Path $cfgPath)) {
    if (-not $porta) {
      if ($modo -eq 'servidor') { $porta = 80 } else { $porta = 8080 }
      while (-not (Test-PortaLivre $porta) -and $porta -lt 8100) {
        if ($porta -eq 80) { $porta = 8080 } else { $porta++ }
      }
    }
    & $log "Criando a configuração (porta $porta) ..."
    $cfg = Get-Content (Join-Path $Aqui 'hub.config.example.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($modo -eq 'servidor') { Set-Prop $cfg 'listen' ":$porta" } else { Set-Prop $cfg 'listen' "127.0.0.1:$porta" }
    Set-Prop $cfg 'staticDir' './www'
    $grav = Join-Path $D 'Gravacoes'
    if ($cfg.PSObject.Properties['recordings']) { Set-Prop $cfg.recordings 'dir' $grav } else { Set-Prop $cfg 'recordings' ([pscustomobject]@{ dir = $grav }) }
    if ($modo -eq 'servidor') {
      # a tarefa roda como SYSTEM: sem pasta pessoal, então livros e vídeos ficam na pasta de dados
      Set-Prop $cfg 'books' ([pscustomobject]@{ dir = (Join-Path $D 'Livros') })
      Set-Prop $cfg 'videos' ([pscustomobject]@{ dir = (Join-Path $D 'Videos') })
    }
    Write-Texto $cfgPath ($cfg | ConvertTo-Json -Depth 20)
  } else {
    & $log 'Configuração existente mantida.'
    $cfgAtual = Get-Content $cfgPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($cfgAtual.listen -match ':(\d+)$') { $porta = [int]$Matches[1] }
  }

  Write-Texto (Join-Path $D 'VERSION') $nova
  Add-Content -Path (Join-Path $D 'install.log') -Value ('{0}  V{1} -> V{2}  ({3})' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $velha, $nova, $modo)

  if (-not $SemRegistro) {
    & $log 'Registrando a inicialização automática ...'
    Unregister-ScheduledTask -TaskName $Tarefa -Confirm:$false -ErrorAction SilentlyContinue
    $acao = New-ScheduledTaskAction -Execute (Join-Path $D $Exe) -Argument ('-config "{0}"' -f $cfgPath) -WorkingDirectory $D
    $conf = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -MultipleInstances IgnoreNew `
      -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 5 -RestartInterval (New-TimeSpan -Minutes 1)
    if ($modo -eq 'servidor') {
      $gatilho = New-ScheduledTaskTrigger -AtStartup
      $princ = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    } else {
      $eu = "$env:USERDOMAIN\$env:USERNAME"
      $gatilho = New-ScheduledTaskTrigger -AtLogOn -User $eu
      $princ = New-ScheduledTaskPrincipal -UserId $eu -LogonType Interactive -RunLevel Limited
    }
    Register-ScheduledTask -TaskName $Tarefa -Action $acao -Trigger $gatilho -Principal $princ -Settings $conf -Description 'DVR: painel da casa' -Force | Out-Null

    if ($modo -eq 'servidor') {
      & $log "Liberando a porta $porta no firewall (só redes privadas e de domínio) ..."
      Remove-NetFirewallRule -DisplayName $Regra -ErrorAction SilentlyContinue
      New-NetFirewallRule -DisplayName $Regra -Direction Inbound -Action Allow -Protocol TCP -LocalPort $porta -Profile Private, Domain | Out-Null
    }

    $url = "http://127.0.0.1:$porta/"
    $atalho = Get-Atalho $modo
    New-Item -ItemType Directory -Force -Path (Split-Path $atalho) | Out-Null
    [IO.File]::WriteAllText($atalho, "[InternetShortcut]`r`nURL=$url`r`n", [Text.Encoding]::ASCII)

    # entrada em "Aplicativos instalados" do Windows
    $raiz = 'HKCU:'
    if ($modo -eq 'servidor') { $raiz = 'HKLM:' }
    $k = Join-Path $raiz $ChaveReg
    New-Item -Path $k -Force | Out-Null
    Set-ItemProperty $k DisplayName 'DVR'
    Set-ItemProperty $k DisplayVersion $nova
    Set-ItemProperty $k Publisher 'Diogo'
    Set-ItemProperty $k InstallLocation $D
    Set-ItemProperty $k UninstallString ('"{0}"' -f (Join-Path $D 'Desinstalar.cmd'))
    Set-ItemProperty $k NoModify 1 -Type DWord
    Set-ItemProperty $k NoRepair 1 -Type DWord

    & $log 'Iniciando o DVR ...'
    Start-ScheduledTask -TaskName $Tarefa
  }

  $url = "http://127.0.0.1:$porta/"
  if (-not $SemRegistro) {
    & $log 'Aguardando o DVR responder ...'
    $ok = $false
    for ($i = 0; $i -lt 30 -and -not $ok; $i++) {
      Start-Sleep -Seconds 1
      try { $ok = (Invoke-WebRequest -Uri ($url + 'api/health') -UseBasicParsing -TimeoutSec 2).StatusCode -eq 200 } catch { }
    }
    if ($ok) { & $log "O DVR está no ar em $url" }
    else { & $log "O DVR foi iniciado, mas ainda não respondeu em $url. Espere um pouco e abra o endereço; se persistir, veja o arquivo hub.log em $D." }
  }

  & $log "Versão instalada: V$nova (era V$velha)"
  return [pscustomobject]@{ Url = $url; Versao = $nova; Modo = $modo; Pasta = $D; Porta = $porta }
}

function Uninstall-Central([bool]$apagar, [scriptblock]$log) {
  $inst = Find-Instalacao
  if (-not $inst -and $Pasta -and (Test-Path (Join-Path $Pasta $Exe))) { $inst = [pscustomobject]@{ Pasta = $Pasta; Modo = 'pessoal'; Raiz = 'HKCU:' } }
  if (-not $inst) { throw 'Não encontrei uma instalação do DVR neste Windows.' }
  if ($inst.Modo -eq 'servidor' -and -not $SemRegistro -and -not (Test-Admin)) { throw 'Desinstalar o modo servidor precisa de administrador.' }
  $D = $inst.Pasta
  & $log 'Parando o DVR ...'
  Stop-Hub $D
  if (-not $SemRegistro) {
    Unregister-ScheduledTask -TaskName $Tarefa -Confirm:$false -ErrorAction SilentlyContinue
    Remove-NetFirewallRule -DisplayName $Regra -ErrorAction SilentlyContinue
    Remove-Item (Get-Atalho $inst.Modo) -Force -ErrorAction SilentlyContinue
    Remove-Item (Join-Path $inst.Raiz $ChaveReg) -Recurse -Force -ErrorAction SilentlyContinue
  }
  & $log 'Removendo os arquivos do programa ...'
  Remove-Item (Join-Path $D $Exe), (Join-Path $D 'www') -Recurse -Force -ErrorAction SilentlyContinue
  # install.ps1 e Desinstalar.cmd (ou a pasta toda) saem logo depois que esta janela fechar
  if ($apagar) {
    & $log 'Apagando também os dados (banco, configuração, livros) ...'
    $limpa = 'rd /s /q "{0}"' -f $D
  } else {
    $limpa = 'del /f /q "{0}\install.ps1" "{0}\Desinstalar.cmd"' -f $D
  }
  Start-Process cmd.exe -ArgumentList ('/c ping -n 4 127.0.0.1 >nul & ' + $limpa) -WindowStyle Hidden
  if ($apagar) { & $log 'Dados removidos.' } else { & $log "Seus dados foram mantidos em $D" }
}

function Restart-ComoAdmin([string]$extra) {
  $a = '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}" {1}' -f $PSCommandPath, $extra
  if ($Pasta) { $a += ' -Pasta "{0}"' -f $Pasta }
  if ($Porta) { $a += ' -Porta {0}' -f $Porta }
  try {
    Start-Process powershell.exe -Verb RunAs -ArgumentList $a
    return $true
  } catch {
    [System.Windows.Forms.MessageBox]::Show('A permissão de administrador não foi concedida. Nada foi alterado.', 'DVR', 'OK', 'Information') | Out-Null
    return $false
  }
}

# ---------- sem janela (automação) ----------
if ($Silencioso) {
  $logConsole = { param($m) Write-Host $m }
  if ($Desinstalar) { Uninstall-Central $ApagarDados.IsPresent $logConsole; exit 0 }
  if (-not $Modo) { Write-Host 'Informe -Modo servidor ou -Modo pessoal.'; exit 2 }
  $r = Install-Central $Modo $logConsole
  exit 0
}

# ---------- janela ----------
$fonte = New-Object Drawing.Font('Segoe UI', 10)
$form = New-Object Windows.Forms.Form
$form.Text = 'DVR'
$form.ClientSize = New-Object Drawing.Size(580, 440)
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.Font = $fonte

function New-Rotulo([string]$texto, [int]$x, [int]$y, [int]$w, [int]$h, [bool]$negrito = $false) {
  $l = New-Object Windows.Forms.Label
  $l.Text = $texto; $l.Location = New-Object Drawing.Point($x, $y); $l.Size = New-Object Drawing.Size($w, $h)
  if ($negrito) { $l.Font = New-Object Drawing.Font('Segoe UI', 14, [Drawing.FontStyle]::Bold) }
  return $l
}

# Página 1: escolha do modo
$pg1 = New-Object Windows.Forms.Panel
$pg1.Size = $form.ClientSize
$pg1.Controls.Add((New-Rotulo 'DVR' 24 18 530 34 $true))
$pg1.Controls.Add((New-Rotulo 'Como você vai usar o DVR neste Windows?' 24 60 530 26))

$rbServidor = New-Object Windows.Forms.RadioButton
$rbServidor.Text = 'Servidor da casa'
$rbServidor.Font = New-Object Drawing.Font('Segoe UI', 10, [Drawing.FontStyle]::Bold)
$rbServidor.Location = New-Object Drawing.Point(30, 100); $rbServidor.Size = New-Object Drawing.Size(500, 26)
$pg1.Controls.Add($rbServidor)
$pg1.Controls.Add((New-Rotulo "Este computador fica ligado e atende a TV, os celulares e os outros aparelhos da casa. Liga sozinho com o Windows, mesmo sem ninguém conectado. Pede permissão de administrador e libera o acesso só nas redes privadas." 52 128 490 66))

$rbPessoal = New-Object Windows.Forms.RadioButton
$rbPessoal.Text = 'Meu computador (recomendado para experimentar)'
$rbPessoal.Font = New-Object Drawing.Font('Segoe UI', 10, [Drawing.FontStyle]::Bold)
$rbPessoal.Location = New-Object Drawing.Point(30, 206); $rbPessoal.Size = New-Object Drawing.Size(500, 26)
$rbPessoal.Checked = $true
$pg1.Controls.Add($rbPessoal)
$pg1.Controls.Add((New-Rotulo "Só você, neste computador. Abre sozinho quando você entra no Windows e não fica acessível pela rede. Não precisa de administrador." 52 234 490 50))

$lblExiste = New-Rotulo '' 24 300 530 44
$lblExiste.ForeColor = [Drawing.Color]::FromArgb(0, 90, 160)
$pg1.Controls.Add($lblExiste)

$btInstalar = New-Object Windows.Forms.Button
$btInstalar.Text = 'Instalar'; $btInstalar.Size = New-Object Drawing.Size(120, 36); $btInstalar.Location = New-Object Drawing.Point(300, 388)
$btCancelar = New-Object Windows.Forms.Button
$btCancelar.Text = 'Cancelar'; $btCancelar.Size = New-Object Drawing.Size(120, 36); $btCancelar.Location = New-Object Drawing.Point(430, 388)
$btRemover = New-Object Windows.Forms.Button
$btRemover.Text = 'Desinstalar'; $btRemover.Size = New-Object Drawing.Size(120, 36); $btRemover.Location = New-Object Drawing.Point(24, 388)
$btRemover.Visible = $false
$pg1.Controls.AddRange(@($btInstalar, $btCancelar, $btRemover))

# Página 2: andamento
$pg2 = New-Object Windows.Forms.Panel
$pg2.Size = $form.ClientSize; $pg2.Visible = $false
$lblTitulo2 = New-Rotulo 'Instalando...' 24 18 530 34 $true
$pg2.Controls.Add($lblTitulo2)
$barra = New-Object Windows.Forms.ProgressBar
$barra.Style = 'Marquee'; $barra.Location = New-Object Drawing.Point(24, 64); $barra.Size = New-Object Drawing.Size(530, 18)
$pg2.Controls.Add($barra)
$caixa = New-Object Windows.Forms.TextBox
$caixa.Multiline = $true; $caixa.ReadOnly = $true; $caixa.ScrollBars = 'Vertical'
$caixa.Location = New-Object Drawing.Point(24, 96); $caixa.Size = New-Object Drawing.Size(530, 160)
$pg2.Controls.Add($caixa)

# Resultado (aparece quando a instalação termina bem): endereço
$pnlRes = New-Object Windows.Forms.Panel
$pnlRes.Location = New-Object Drawing.Point(24, 264); $pnlRes.Size = New-Object Drawing.Size(530, 110); $pnlRes.Visible = $false
$lblEnd = New-Object Windows.Forms.Label
$lblEnd.Location = New-Object Drawing.Point(0, 0); $lblEnd.Size = New-Object Drawing.Size(90, 24); $lblEnd.Text = 'Endereço:'
$lnkEnd = New-Object Windows.Forms.LinkLabel
$lnkEnd.Location = New-Object Drawing.Point(90, 0); $lnkEnd.Size = New-Object Drawing.Size(430, 24)
$pnlRes.Controls.AddRange(@($lblEnd, $lnkEnd))
$pg2.Controls.Add($pnlRes)
$btAbrir = New-Object Windows.Forms.Button
$btAbrir.Text = 'Abrir o DVR'; $btAbrir.Size = New-Object Drawing.Size(180, 36); $btAbrir.Location = New-Object Drawing.Point(180, 388); $btAbrir.Visible = $false
$btFechar = New-Object Windows.Forms.Button
$btFechar.Text = 'Fechar'; $btFechar.Size = New-Object Drawing.Size(120, 36); $btFechar.Location = New-Object Drawing.Point(430, 388); $btFechar.Enabled = $false
$pg2.Controls.AddRange(@($btAbrir, $btFechar))

$form.Controls.AddRange(@($pg1, $pg2))

$script:urlFinal = ''
$logJanela = { param($m) $caixa.AppendText($m + "`r`n"); [Windows.Forms.Application]::DoEvents() }

function Show-Andamento([string]$titulo) {
  $pg1.Visible = $false; $pg2.Visible = $true
  $lblTitulo2.Text = $titulo
  [Windows.Forms.Application]::DoEvents()
}

function Invoke-Instalar([string]$modo) {
  Show-Andamento 'Instalando...'
  try {
    $r = Install-Central $modo $logJanela
    $script:urlFinal = $r.Url
    $lblTitulo2.Text = 'Instalação concluída'
    $lnkEnd.Text = $r.Url
    $pnlRes.Visible = $true
    $btAbrir.Visible = $true
  } catch {
    $lblTitulo2.Text = 'Não foi possível instalar'
    & $logJanela ('ERRO: ' + $_.Exception.Message)
  }
  $barra.Style = 'Continuous'; $barra.Value = 100
  $btFechar.Enabled = $true
}

function Invoke-Desinstalar {
  $c = [Windows.Forms.MessageBox]::Show('Remover o DVR deste computador?', 'DVR', 'OKCancel', 'Question')
  if ($c -ne 'OK') { return }
  $apagar = ([Windows.Forms.MessageBox]::Show("Apagar também os seus dados (banco, configuração, livros)?`r`n`r`nSim: apaga tudo.`r`nNão: remove o programa e mantém os dados.", 'DVR', 'YesNo', 'Warning', 'Button2') -eq 'Yes')
  Show-Andamento 'Desinstalando...'
  try {
    Uninstall-Central $apagar $logJanela
    $lblTitulo2.Text = 'Desinstalação concluída'
  } catch {
    $lblTitulo2.Text = 'Não foi possível desinstalar'
    & $logJanela ('ERRO: ' + $_.Exception.Message)
  }
  $barra.Style = 'Continuous'; $barra.Value = 100
  $btFechar.Enabled = $true
}

$existente = Find-Instalacao
if ($existente) {
  $v = ''
  if ($existente.Versao) { $v = ' (versão ' + $existente.Versao + ')' }
  $lblExiste.Text = "Já existe uma instalação em modo $($existente.Modo)$v. Instalar atualiza o programa e mantém seus dados."
  if ($existente.Modo -eq 'servidor') { $rbServidor.Checked = $true } else { $rbPessoal.Checked = $true }
  $btInstalar.Text = 'Atualizar'
  $btRemover.Visible = $true
}

$btCancelar.Add_Click({ $form.Close() })
$btFechar.Add_Click({ $form.Close() })
$btAbrir.Add_Click({ Start-Process $script:urlFinal })
$lnkEnd.Add_LinkClicked({ Start-Process $script:urlFinal })
$btInstalar.Add_Click({
  $modo = 'pessoal'
  if ($rbServidor.Checked) { $modo = 'servidor' }
  if ($modo -eq 'servidor' -and -not (Test-Admin)) {
    if (Restart-ComoAdmin '-Modo servidor -Executar') { $form.Close() }
    return
  }
  Invoke-Instalar $modo
})
$btRemover.Add_Click({
  if ($existente.Modo -eq 'servidor' -and -not (Test-Admin)) {
    if (Restart-ComoAdmin '-Desinstalar') { $form.Close() }
    return
  }
  Invoke-Desinstalar
})

$form.Add_Shown({
  $form.Activate()
  if ($Desinstalar) { Invoke-Desinstalar }
  elseif ($Executar -and $Modo) { Invoke-Instalar $Modo }
})

[void]$form.ShowDialog()
