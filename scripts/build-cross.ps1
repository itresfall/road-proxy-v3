param(
  [string]$OutputRoot = "build/release",
  [switch]$IncludeWindowsArm64,
  [switch]$IncludeLinuxArm64 = $true,
  [switch]$Package
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
$safeVersion = $version -replace '[^A-Za-z0-9._-]', '-'

$targets = @(
  @{ GOOS = "windows"; GOARCH = "amd64" },
  @{ GOOS = "linux"; GOARCH = "amd64" }
)
if ($IncludeWindowsArm64) {
  $targets += @{ GOOS = "windows"; GOARCH = "arm64" }
}
if ($IncludeLinuxArm64) {
  $targets += @{ GOOS = "linux"; GOARCH = "arm64" }
}

$commands = @(
  @{ Name = "road-proxy"; Path = "./cmd/road" },
  @{ Name = "road-server"; Path = "./cmd/server" },
  @{ Name = "road-client"; Path = "./cmd/client" },
  @{ Name = "plugin-studio"; Path = "./cmd/plugin-studio" }
)

Push-Location $repoRoot
try {
  $oldGOOS = $env:GOOS
  $oldGOARCH = $env:GOARCH
  $oldCGO = $env:CGO_ENABLED

  foreach ($target in $targets) {
    $goos = $target.GOOS
    $goarch = $target.GOARCH
    $outDir = Join-Path $OutputRoot "$goos-$goarch"
    if (Test-Path -LiteralPath $outDir) {
      Remove-Item -LiteralPath $outDir -Recurse -Force
    }
    [void](New-Item -ItemType Directory -Path $outDir -Force)

    Write-Host ""
    Write-Host "Building target: $goos/$goarch"

    $env:GOOS = $goos
    $env:GOARCH = $goarch
    # Prefer static-ish builds for cross-target portability.
    $env:CGO_ENABLED = "0"
    foreach ($cmd in $commands) {
      $name = $cmd.Name
      $path = $cmd.Path

      $exeSuffix = ""
      if ($goos -eq "windows") {
        $exeSuffix = ".exe"
      }
      $outPath = Join-Path $outDir "$name$exeSuffix"
      Write-Host "  -> $name"
      go build -ldflags $ldflags -o $outPath $path
      if ($LASTEXITCODE -ne 0) {
        throw "build failed for $goos/$goarch $name"
      }
    }

    foreach ($asset in @("configs", "plugins", "locales", "docs", "compat-profiles", "deploy")) {
      $dest = Join-Path $outDir $asset
      if (Test-Path -LiteralPath $dest) {
        Remove-Item -LiteralPath $dest -Recurse -Force
      }
      Copy-Item -Path ".\$asset" -Destination $outDir -Recurse -Force
    }

    $scriptsDest = Join-Path $outDir "scripts"
    if (Test-Path -LiteralPath $scriptsDest) {
      Remove-Item -LiteralPath $scriptsDest -Recurse -Force
    }
    Copy-Item -Path ".\scripts" -Destination $outDir -Recurse -Force

    foreach ($doc in @("README.md", "CHANGELOG.md", "LICENSE", "SECURITY.md", "CONTRIBUTING.md")) {
      if (Test-Path -LiteralPath $doc) {
        Copy-Item -LiteralPath $doc -Destination $outDir -Force
      }
    }

    @(
      "version=$version",
      "commit=$commit",
      "build_date=$buildDate",
      "target=$goos/$goarch"
    ) | Set-Content -LiteralPath (Join-Path $outDir "VERSION.txt") -Encoding ASCII

    if ($Package) {
      $zipName = "road-proxy-v3_${safeVersion}_${goos}_${goarch}.zip"
      $zipPath = Join-Path $OutputRoot $zipName
      if (Test-Path -LiteralPath $zipPath) {
        Remove-Item -LiteralPath $zipPath -Force
      }
      Compress-Archive -Path (Join-Path $outDir "*") -DestinationPath $zipPath -Force
      $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath
      "$($hash.Hash)  $zipName" | Set-Content -LiteralPath "$zipPath.sha256" -Encoding ASCII
      Write-Host "  packaged: $zipName"
    }
  }

  Write-Host ""
  Write-Host "Cross build complete: $OutputRoot"
}
finally {
  $env:GOOS = $oldGOOS
  $env:GOARCH = $oldGOARCH
  $env:CGO_ENABLED = $oldCGO
  Pop-Location
}
