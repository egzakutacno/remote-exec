# AI Remote Execution — GitHub One-Line Installer
# Usage: $u='https://TUNNEL_URL'; irm https://raw.githubusercontent.com/egzakutacno/remote-exec/main/install.ps1 | iex
# Or:     $env:REMOTE_EXEC_SERVER='https://TUNNEL_URL'; irm https://raw.githubusercontent.com/egzakutacno/remote-exec/main/install.ps1 | iex

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$ServerUrl = if ($u) { $u } elseif ($env:REMOTE_EXEC_SERVER) { $env:REMOTE_EXEC_SERVER } else { $null }
if (-not $ServerUrl) {
    Write-Host "ERROR: Set `$u or `$env:REMOTE_EXEC_SERVER with the server URL." -ForegroundColor Red
    Write-Host 'Usage: $u="https://TUNNEL_URL"; irm https://raw.githubusercontent.com/egzakutacno/remote-exec/main/install.ps1 | iex' -ForegroundColor Yellow
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
    Write-Host "[1/4] Downloading agent.exe..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $agentPath = "$InstallDir\agent.exe"
    Invoke-WebRequest -Uri "$ServerUrl/agent.exe" -OutFile $agentPath -UseBasicParsing
    if (-not (Test-Path $agentPath)) { throw "Download failed" }
    Write-Host "       agent.exe $((Get-Item $agentPath).Length) bytes" -ForegroundColor Gray

    Write-Host "[2/4] Config + register..." -ForegroundColor Yellow
    $configJson = @"
{
    "server_url": "$ServerUrl",
    "machine_id": "$MachineId",
    "api_key": "$ApiKey",
    "name": "$MachineName",
    "poll_wait": 60,
    "max_timeout": 90,
    "log_file": "$($InstallDir.Replace('\','\\'))\\agent.log"
}
"@
    $configPath = "$InstallDir\agent.json"
    [System.IO.File]::WriteAllText($configPath, $configJson, [System.Text.UTF8Encoding]::new($false))
    Write-Host "       config saved" -ForegroundColor Gray

    $body = @{ name = $MachineName; api_key = $ApiKey; hostname = $env:COMPUTERNAME; metadata = "{}" } | ConvertTo-Json
    try {
        $reg = Invoke-RestMethod -Uri "$ServerUrl/api/v1/agent/register" -Method POST -Body $body -ContentType "application/json"
        $realMid = $reg.machine_id
        Write-Host "       registered: $realMid" -ForegroundColor Gray
        if ($realMid -ne $MachineId) {
            $configJson = $configJson.Replace("`"machine_id`": `"$MachineId`"", "`"machine_id`": `"$realMid`"")
            [System.IO.File]::WriteAllText($configPath, $configJson, [System.Text.UTF8Encoding]::new($false))
            $MachineId = $realMid
        }
    } catch {
        Write-Host "       WARNING: registration failed (agent retries on poll)" -ForegroundColor Yellow
    }

    Write-Host "[3/4] Windows Service..." -ForegroundColor Yellow
    if (-NOT ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]"Administrator")) {
        Write-Host "ERROR: Run PowerShell as Administrator!" -ForegroundColor Red; exit 1
    }

    $svcName = "RemoteExec-$MachineId"
    $old = Get-Service -Name $svcName -ErrorAction SilentlyContinue
    if ($old) { Stop-Service $svcName -Force -ErrorAction SilentlyContinue; sc.exe delete $svcName; Start-Sleep 2 }

    $binCmd = "`"$agentPath`" --config `"$configPath`""
    $result = sc.exe create $svcName binPath= $binCmd start= auto 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "       sc.exe failed, trying New-Service..." -ForegroundColor Gray
        New-Service -Name $svcName -BinaryPathName $binCmd -DisplayName "Remote Exec ($MachineName)" -StartupType Automatic
    }
    sc.exe description $svcName "AI Remote Execution Agent - $MachineName" 2>&1 | Out-Null

    Write-Host "[4/4] Starting... " -ForegroundColor Yellow -NoNewline
    Start-Service $svcName -ErrorAction SilentlyContinue
    Start-Sleep 4
    $svc = Get-Service -Name $svcName -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status -eq "Running") {
        Write-Host "RUNNING" -ForegroundColor Green
    } else {
        Write-Host "CHECK LOG" -ForegroundColor Yellow
    }

    Write-Host "`n  DONE. Machine: $MachineId  Service: $svcName`n" -ForegroundColor Green
    Write-Host "  Logs: Get-Content $InstallDir\agent.log" -ForegroundColor Gray
    Write-Host "  Uninstall: sc.exe delete $svcName" -ForegroundColor Gray
    Write-Host ""
} catch {
    Write-Host "`nFAILED: $_" -ForegroundColor Red
    Write-Host $_.ScriptStackTrace -ForegroundColor Gray
    exit 1
}
