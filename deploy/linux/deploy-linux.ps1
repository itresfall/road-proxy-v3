param(
  [Parameter(Mandatory = $true)]
  [string]$Target,

  [ValidateSet("amd64", "arm64")]
  [string]$Arch = "amd64",

  [string]$SourceDir = "",
  [string]$RemoteDir = "/opt/road-proxy-v3",
  [string]$ServiceUser = "road",
  [string]$ServiceGroup = "road",
  [bool]$CreateServiceUser = $true,
  [switch]$InstallSystemd,
  [switch]$Start,
  [switch]$Restart,
  [switch]$IncludeVoice,
  [switch]$WhatIfOnly,
  [string]$SSH = "ssh",
  [string]$SCP = "scp"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)

if ([string]::IsNullOrWhiteSpace($SourceDir)) {
  $SourceDir = Join-Path $repoRoot "build/linux/$Arch"
}
$SourceDir = (Resolve-Path -LiteralPath $SourceDir).Path

foreach ($name in @("road-server", "road-client", "road-proxy")) {
  $path = Join-Path $SourceDir $name
  if (-not (Test-Path -LiteralPath $path)) {
    throw "missing linux build artifact: $path. Run ./scripts/build-linux.ps1 -Arch $Arch first."
  }
}

$stamp = Get-Date -Format "yyyyMMddHHmmss"
$remoteTemp = "/tmp/road-proxy-v3-deploy-$stamp"

function Invoke-Remote {
  param([string]$Command)
  if ($WhatIfOnly) {
    Write-Host "DRY-RUN ssh $Target $Command"
    return
  }
  & $SSH $Target $Command
  if ($LASTEXITCODE -ne 0) {
    throw "ssh command failed: $Command"
  }
}

function Copy-ToRemote {
  param([string]$Path, [string]$Destination)
  if ($WhatIfOnly) {
    Write-Host "DRY-RUN scp -r $Path ${Target}:$Destination"
    return
  }
  & $SCP -r $Path "${Target}:$Destination"
  if ($LASTEXITCODE -ne 0) {
    throw "scp failed: $Path -> ${Target}:$Destination"
  }
}

Write-Host "Preparing remote temp directory: ${Target}:$remoteTemp"
Invoke-Remote "rm -rf '$remoteTemp' && mkdir -p '$remoteTemp'"

Write-Host "Copying ROAD linux build from $SourceDir"
Get-ChildItem -LiteralPath $SourceDir -Force | ForEach-Object {
  Copy-ToRemote $_.FullName "$remoteTemp/"
}

$installParts = @()
if ($CreateServiceUser) {
  $installParts += "if ! getent group '$ServiceGroup' >/dev/null 2>&1; then sudo groupadd --system '$ServiceGroup'; fi"
  $installParts += "if ! id -u '$ServiceUser' >/dev/null 2>&1; then sudo useradd --system --home '$RemoteDir' --shell /usr/sbin/nologin --gid '$ServiceGroup' '$ServiceUser'; fi"
}

$installParts += @(
  "sudo mkdir -p '$RemoteDir'",
  "sudo cp -a '$remoteTemp/.' '$RemoteDir/'",
  "(sudo chmod +x '$RemoteDir/road-proxy' '$RemoteDir/road-server' '$RemoteDir/road-client' '$RemoteDir/plugin-studio' '$RemoteDir/voice-server' '$RemoteDir/deploy/linux/firewall-ufw.sh' 2>/dev/null || true)",
  "sudo chown -R '$($ServiceUser):$($ServiceGroup)' '$RemoteDir'",
  "rm -rf '$remoteTemp'"
)
$installCommand = $installParts -join " && "

Write-Host "Installing to $RemoteDir"
Invoke-Remote $installCommand

if ($InstallSystemd) {
  Write-Host "Installing systemd service templates"
  Invoke-Remote "sudo cp '$RemoteDir/deploy/systemd/road-server.service' /etc/systemd/system/road-server.service && sudo systemctl daemon-reload && sudo systemctl enable road-server.service"
  if ($IncludeVoice) {
    Invoke-Remote "sudo cp '$RemoteDir/deploy/systemd/voice-server.service' /etc/systemd/system/voice-server.service && sudo systemctl daemon-reload && sudo systemctl enable voice-server.service"
  }
}

if ($Restart) {
  Write-Host "Restarting services"
  Invoke-Remote "sudo systemctl restart road-server.service"
  if ($IncludeVoice) {
    Invoke-Remote "sudo systemctl restart voice-server.service"
  }
} elseif ($Start) {
  Write-Host "Starting services"
  Invoke-Remote "sudo systemctl start road-server.service"
  if ($IncludeVoice) {
    Invoke-Remote "sudo systemctl start voice-server.service"
  }
}

Write-Host "Deploy complete: ${Target}:$RemoteDir"
