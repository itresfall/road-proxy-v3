param(
  [string]$InstallDir = "C:\road-proxy-v3",
  [string]$NSSM = "nssm.exe",
  [string]$ServiceName = "road-server",
  [string]$Config = "configs\server.json",
  [ValidateSet("server", "voice")]
  [string]$Mode = "server"
)

$ErrorActionPreference = "Stop"

$binary = if ($Mode -eq "voice") { "voice-server.exe" } else { "road-server.exe" }
$displayName = if ($Mode -eq "voice") { "ROAD Voice Server" } else { "ROAD Proxy Server" }
$serviceBinary = Join-Path $InstallDir $binary

if (-not (Test-Path -LiteralPath $serviceBinary)) {
  throw "missing binary: $serviceBinary"
}

& $NSSM install $ServiceName $serviceBinary "-config" $Config
if ($LASTEXITCODE -ne 0) {
  throw "nssm install failed"
}

& $NSSM set $ServiceName AppDirectory $InstallDir
& $NSSM set $ServiceName DisplayName $displayName
& $NSSM set $ServiceName Start SERVICE_AUTO_START
& $NSSM set $ServiceName AppStdout (Join-Path $InstallDir "logs\$ServiceName.out.log")
& $NSSM set $ServiceName AppStderr (Join-Path $InstallDir "logs\$ServiceName.err.log")

Write-Host "Installed NSSM service: $ServiceName"
Write-Host "Start it with: Start-Service $ServiceName"
