# AI Remote Execution — GitHub One-Line Installer
# Usage: $u='https://TUNNEL_URL'; irm https://raw.githubusercontent.com/ruter/remote-exec/main/install.ps1 | iex

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$ServerUrl = if ($u) { $u } elseif ($env:REMOTE_EXEC_SERVER) { $env:REMOTE_EXEC_SERVER } else { $null }
if (-not $ServerUrl) {
    Write-Host "ERROR: Set `$u variable with server URL first." -ForegroundColor Red
    Write-Host 'Usage: $u="https://TUNNEL_URL"; irm https://raw.githubusercontent.com/ruter/remote-exec/main/install.ps1 | iex' -ForegroundColor Yellow
    exit 1
}

$InstallDir = if ($env:REMOTE_EXEC_DIR) { $env:REMOTE_EXEC_DIR } else { "C:\remote-exec-agent" }
$MachineName = if ($env:REMOTE_EXEC_NAME) { $env:REMOTE_EXEC_NAME } else { $env:COMPUTERNAME }

$guid = [guid]::NewGuid().ToString().Replace("-", "")
$randomHex = -join ((48..57) + (97..102) | Get-Random -Count 16 | ForEach-Object { [char]$_ })
$ApiKey = $guid + $randomHex

$MachineId = ($MachineName.ToLower() -replace '[^a-z0-9]', '-').Trim('-')
$suffix = -join ((48..57) + (97..102) | Get-Random -Count 4 | ForEach-Object { [char]$_ })
$MachineId = "$MachineId-$suffix"

Write-Host "`n  AI Remote Execution — Installer`n" -ForegroundColor Cyan
Write-Host "  Server:    $ServerUrl" -ForegroundColor Gray
Write-Host "  Machine:   $MachineId ($MachineName)" -ForegroundColor Gray
Write-Host "  Install:   $InstallDir`n" -ForegroundColor Gray

try {
    Write-Host "[1/4] Downloading agent..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $agentPath = "$InstallDir\agent.exe"
    Invoke-WebRequest -Uri "$ServerUrl/agent.exe" -OutFile $agentPath -UseBasicParsing
    if (-not (Test-Path $agentPath)) { throw "Download failed" }
    Write-Host "       agent.exe OK" -ForegroundColor Gray

    Write-Host "[2/4] Config + register..." -ForegroundColor Yellow
    $config = @{
        server_url  = $ServerUrl; machine_id = $MachineId; api_key = $ApiKey
        name        = $MachineName; poll_wait = 60; max_timeout = 90
        log_file    = "$InstallDir\agent.log"
    }
    $configPath = "$InstallDir\agent.json"
    $config | ConvertTo-Json -Depth 5 | Out-File $configPath -Encoding utf8

    $body = @{ name = $MachineName; api_key = $ApiKey; hostname = $env:COMPUTERNAME; metadata = "{}" } | ConvertTo-Json
    try {
        $reg = Invoke-RestMethod -Uri "$ServerUrl/api/v1/agent/register" -Method POST -Body $body -ContentType "application/json"
        Write-Host "       registered: $($reg.machine_id)" -ForegroundColor Gray
    } catch {
        Write-Host "       WARNING: registration failed (will retry)" -ForegroundColor Yellow
    }

    Write-Host "[3/4] Windows Service..." -ForegroundColor Yellow
    if (-NOT ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]"Administrator")) {
        Write-Host "ERROR: Run as Administrator!" -ForegroundColor Red; exit 1
    }

    $svcName = "RemoteExec-$MachineId"
    $old = Get-Service -Name $svcName -ErrorAction SilentlyContinue
    if ($old) { Stop-Service $svcName -Force -ErrorAction SilentlyContinue; sc.exe delete $svcName | Out-Null; Start-Sleep 2 }

    sc.exe create $svcName binPath= "`"$agentPath`" --config `"$configPath`" --console" start= auto | Out-Null
    sc.exe description $svcName "AI Remote Execution Agent ($MachineName)" | Out-Null
    sc.exe start $svcName | Out-Null

    Write-Host "[4/4] Starting... " -ForegroundColor Yellow -NoNewline
    Start-Sleep 3
    $svc = Get-Service -Name $svcName -ErrorAction SilentlyContinue
    if ($svc.Status -eq "Running") { Write-Host "RUNNING" -ForegroundColor Green } else { Write-Host "CHECK LOG" -ForegroundColor Yellow }

    Write-Host "`n  DONE. Machine: $MachineId  Service: $svcName`n" -ForegroundColor Green
    Write-Host "  Logs:  Get-Content $agentPath\..\agent.log -Wait" -ForegroundColor Gray
} catch {
    Write-Host "`nFAILED: $_`n" -ForegroundColor Red; exit 1
}
