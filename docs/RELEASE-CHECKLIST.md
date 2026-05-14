# Release Checklist

Use this checklist before publishing a GitHub release.

## Preflight

- Confirm `README.md`, `CHANGELOG.md`, `LICENSE`, `SECURITY.md`, and
  `CONTRIBUTING.md` are current.
- Confirm `docs/PROJECT-ROADMAP.md` has no release-blocking `{ }` items.
- Confirm experimental profiles are clearly marked as experimental.
- Confirm no tokens, local logs, temporary configs, or private hostnames are
  staged for release.

## Validation

Run from the repository root:

```powershell
go test ./...
go run ./cmd/road validate --all-configs
go run ./cmd/road validate-plugin plugins/gzdoom-udp/plugin.json
go run ./cmd/road validate-plugin plugins/ddnet-udp/plugin.json
```

Build packages:

```powershell
$env:ROAD_VERSION = "0.1.0"
./scripts/build-cross.ps1 -Package
```

Smoke-test a fresh package extraction:

```powershell
$tmp = Join-Path $env:TEMP "road-release-smoke"
Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $tmp | Out-Null
Expand-Archive .\build\release\road-proxy-v3_0.1.0_windows_amd64.zip -DestinationPath $tmp
Push-Location $tmp
.\road-proxy.exe --version
.\road-proxy.exe validate --all-configs
Pop-Location
```

## Git Release

Recommended flow:

```powershell
git status --short
git add .
git commit -m "Prepare ROAD Proxy v3 0.1.0"
git tag v0.1.0
git push
git push origin v0.1.0
```

The GitHub Actions workflow should build release artifacts on tag push. Compare
uploaded artifact checksums against local `build/release/*.sha256` if publishing
the locally built files.

## Release Notes

Release notes should state:

- Supported platforms: Windows amd64, Linux amd64, Linux arm64.
- macOS is not an official target.
- ROAD is not a VPN or Steam/EOS relay replacement.
- Public deployment should use token auth and local-only reverse-proxy binding.
- Lethal Company direct UDP is experimental/community-validation.

