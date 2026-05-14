# Release Checklist

Use this checklist before publishing a GitHub release.

## Preflight

- Confirm `README.md`, `CHANGELOG.md`, `LICENSE`, `SECURITY.md`, and
  `CONTRIBUTING.md` are current.
- Confirm `docs/PROJECT-ROADMAP.md` has no release-blocking open `{ }` items.
  Long-term experimental items may remain open when clearly marked non-blocking.
- Confirm experimental profiles are clearly marked as experimental.
- Confirm no tokens, local logs, temporary configs, or private hostnames are
  staged for release.
- Confirm repository Actions settings allow workflow read/write permissions so
  tag CI can create GitHub Release assets.

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
$env:ROAD_VERSION = "v0.1.0"
./scripts/build-cross.ps1 -Package -IncludeWindowsArm64
```

Smoke-test a fresh package extraction:

```powershell
$tmp = Join-Path $env:TEMP "road-release-smoke"
Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $tmp | Out-Null
Expand-Archive .\build\release\road-proxy-v3_v0.1.0_windows_amd64.zip -DestinationPath $tmp
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

After the repository is public, do not force-move release tags. Use a new
version tag for follow-up releases.

## Release Notes

Release notes should state:

- Supported platforms: Windows amd64, Linux amd64, Linux arm64.
- Experimental artifacts: Windows arm64.
- macOS is not an official target.
- ROAD is not a VPN or Steam/EOS relay replacement.
- Public deployment should use token auth and local-only reverse-proxy binding.
- Lethal Company direct UDP is experimental/community-validation.

Release assets:

- Do not commit `build/` outputs to the repository.
- Tag CI must publish `build/release/*.zip` and `build/release/*.sha256` as
  GitHub Release assets.
- Keep Actions artifacts only as CI diagnostics; the public download path is
  the GitHub Releases page.
