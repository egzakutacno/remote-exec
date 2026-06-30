param(
    [Parameter(Mandatory=$true)]
    [string]$ServerUrl,

    [Parameter(Mandatory=$true)]
    [string]$MachineId,

    [Parameter(Mandatory=$false)]
    [string]$ApiKey,

    [Parameter(Mandatory=$false)]
    [string]$Name = $env:COMPUTERNAME,

    [Parameter(Mandatory=$false)]
    [string]$InstallDir = "C:\remote-exec-agent"
)

$ErrorActionPreference = "Stop"

Write-Host "=== AI Remote Execution — Windows Agent Deploy ===" -ForegroundColor Cyan
Write-Host "Server:  $ServerUrl"
Write-Host "Machine: $MachineId"
Write-Host "Install: $InstallDir"
Write-Host ""

if (-not (Test-Path "$PSScriptRoot\agent.exe")) {
    Write-Error "agent.exe not found in script directory. Build it first."
    exit 1
}

Write-Host "[1/4] Creating install directory..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

Write-Host "[2/4] Copying agent binary..."
Copy-Item -Force "$PSScriptRoot\agent.exe" "$InstallDir\agent.exe"

Write-Host "[3/4] Creating agent config..."
if (-not $ApiKey) {
    $guid = [guid]::NewGuid().ToString()
    $ApiKey = $guid.Replace("-", "")
}

$config = @{
    server_url  = $ServerUrl
    machine_id  = $MachineId
    api_key     = $ApiKey
    name        = $Name
    poll_wait   = 60
    max_timeout = 90
    log_file    = "$InstallDir\agent.log"
}

$config | ConvertTo-Json -Depth 3 | Out-File -FilePath "$InstallDir\agent.json" -Encoding utf8

Write-Host "[4/4] Creating Windows service..."
$serviceName = "RemoteExec-$MachineId"

$existing = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($existing) {
    Stop-Service $serviceName -Force
    sc.exe delete $serviceName
    Start-Sleep -Seconds 2
}

$binPath = "`"$InstallDir\agent.exe`" --config `"$InstallDir\agent.json`" --console"

New-Service `
    -Name $serviceName `
    -BinaryPathName $binPath `
    -DisplayName "AI Remote Exec Agent ($Name)" `
    -Description "AI Remote Execution System — Windows Agent" `
    -StartupType Automatic

Start-Service $serviceName

Write-Host ""
Write-Host "=== DEPLOY COMPLETE ===" -ForegroundColor Green
Write-Host ""

Write-Host "Config saved: $InstallDir\agent.json"
Write-Host "Service:      $serviceName"
Write-Host ""
Write-Host "API Key:      $ApiKey"
Write-Host ""
Write-Host "Register this machine on the server:"
Write-Host "  curl -X POST '$ServerUrl/api/v1/agent/register' -H 'Content-Type: application/json' -d '{`"name`":`"$Name`",`"api_key`":`"$ApiKey`",`"hostname`":`"$env:COMPUTERNAME`"}'"
Write-Host ""
Write-Host "Check logs:"
Write-Host "  Get-Content $InstallDir\agent.log -Wait"
