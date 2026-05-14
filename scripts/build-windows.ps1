param(
  [string]$OutputRoot = "build/windows"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot

function Get-RoadCommit {
  if (-not [string]::IsNullOrWhiteSpace($env:ROAD_COMMIT)) {
    return $env:ROAD_COMMIT
  }
  if (-not (Test-Path -LiteralPath (Join-Path $repoRoot ".git"))) {
    return "unknown"
  }
  try {
    $commit = & git -C $repoRoot rev-parse --verify --short HEAD 2>$null
    if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($commit)) {
      return $commit.Trim()
    }
  }
  catch {
    return "unknown"
  }
  return "unknown"
}

$version = if ([string]::IsNullOrWhiteSpace($env:ROAD_VERSION)) { "0.1.0-dev" } else { $env:ROAD_VERSION }
$commit = Get-RoadCommit
$buildDate = if ([string]::IsNullOrWhiteSpace($env:ROAD_BUILD_DATE)) { (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ") } else { $env:ROAD_BUILD_DATE }
$ldflags = "-X road-proxy-v3/internal/version.Version=$version -X road-proxy-v3/internal/version.Commit=$commit -X road-proxy-v3/internal/version.BuildDate=$buildDate"

$commands = @(
  @{ Name = "road-proxy.exe"; Path = "./cmd/road" },
  @{ Name = "road-server.exe"; Path = "./cmd/server" },
  @{ Name = "road-client.exe"; Path = "./cmd/client" },
  @{ Name = "plugin-studio.exe"; Path = "./cmd/plugin-studio" },
  @{ Name = "voice-server.exe"; Path = "./cmd/voice-server" }
)

Push-Location $repoRoot
try {
  $oldGOOS = $env:GOOS
  $oldGOARCH = $env:GOARCH
  $oldCGO = $env:CGO_ENABLED

  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  $env:CGO_ENABLED = "0"

  if (-not (Test-Path -LiteralPath $OutputRoot)) {
    [void](New-Item -ItemType Directory -Path $OutputRoot -Force)
  }

  foreach ($cmd in $commands) {
    $name = $cmd.Name
    $path = $cmd.Path
    $outPath = Join-Path $OutputRoot $name
    Write-Host "Building windows/amd64 -> $name"
    go build -ldflags $ldflags -o $outPath $path
    if ($LASTEXITCODE -ne 0) {
      throw "build failed: windows/amd64 $name"
    }
  }

  foreach ($asset in @("configs", "plugins", "locales", "docs", "compat-profiles", "deploy")) {
    $dest = Join-Path $OutputRoot $asset
    if (Test-Path -LiteralPath $dest) {
      Remove-Item -LiteralPath $dest -Recurse -Force
    }
    Copy-Item -Path ".\$asset" -Destination $OutputRoot -Recurse -Force
  }

  foreach ($doc in @("README.md", "CHANGELOG.md", "LICENSE", "SECURITY.md", "CONTRIBUTING.md")) {
    if (Test-Path -LiteralPath $doc) {
      Copy-Item -LiteralPath $doc -Destination $OutputRoot -Force
    }
  }

  Write-Host ""
  Write-Host "Windows build complete: $OutputRoot"
}
finally {
  $env:GOOS = $oldGOOS
  $env:GOARCH = $oldGOARCH
  $env:CGO_ENABLED = $oldCGO
  Pop-Location
}
