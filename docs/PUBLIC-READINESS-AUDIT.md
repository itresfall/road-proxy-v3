# Public Readiness Audit

Date: 2026-05-14
Scope: source code, docs, configs, plugins, compatibility profiles, CI, build scripts, and release packaging.

## Automated Checks

Completed on Windows/PowerShell from the repository root:

```powershell
go test ./...
go test -race ./...
go vet ./...
go run ./cmd/road validate --all-configs
./scripts/build-windows.ps1
./scripts/build-linux.ps1
./scripts/build-cross.ps1 -Package -IncludeWindowsArm64
```

Result: passed.

Race test environment: Windows race testing requires CGO and a C compiler. This
machine was prepared with MSYS2 UCRT64 GCC, then `go test -race ./...` passed.

## Fixed Findings

### F-001: Diagnostic bundle could leak secrets

Risk: `road-proxy diagnostic-bundle` collected config and log files into a zip without redacting tokens. Users could accidentally share ROAD auth tokens, bearer headers, API keys, passwords, or client secrets.

Fix: diagnostic bundle file collection now redacts sensitive JSON keys, common text/log token patterns, and user-home paths in metadata. Unit coverage verifies that config/log secrets are not present in generated zips and that home paths are masked.

### F-002: Stale legacy research notes were public-confusing

Risk: old `MIT-RAPORU` notes were stale, mojibake-heavy, and did not match current ROAD scope or AGPL release posture.

Fix: removed those legacy docs from the public release tree.

### F-003: Changelog still described the release as unreleased

Risk: `CHANGELOG.md` still used a `0.1.0-dev - Unreleased` heading while the project is preparing `v0.1.0` release assets.

Fix: changed the release heading to `v0.1.0 - 2026-05-14` and added public-readiness and diagnostic-redaction notes.

### F-004: Release package examples did not match tag asset names

Risk: release docs used `0.1.0` examples while tag CI uses `github.ref_name`, producing `v0.1.0` asset names.

Fix: aligned README and release checklist examples with `v0.1.0` asset names.

### F-005: Literal escape/mojibake artifacts remained in docs

Risk: Markdown contained literal `` `r`n `` fragments and mojibake from earlier edits, which looks unprofessional and can confuse contributors.

Fix: cleaned the affected changelog, I18N, and TODO text.

### F-006: GitHub Release creation depends on repository settings

Risk: the workflow has job-level `contents: write`, but GitHub repository Actions settings can still block release creation if workflow permissions are restricted.

Fix: documented the required read/write Actions permission in the release checklist and README.

### F-007: Repository map under-described Plugin Studio

Risk: README still called Plugin Studio a Windows process scanner even though Linux scanning exists.

Fix: updated the repository map to Windows/Linux.

### F-008: Local release packaging could leave stale assets

Risk: repeated local `scripts/build-cross.ps1 -Package` runs could leave old ROAD zip/checksum files under `build/release`. CI starts from a clean workspace, but manual release verification could accidentally inspect or upload stale files.

Fix: packaging now removes existing `road-proxy-v3_*.zip` and matching `.sha256` files before writing the current package set.

## Residual Risks

- Default example server configs bind data/control to `0.0.0.0` for LAN/dev convenience and do not enable auth. Internet-facing users must use Public Server Wizard or `configs/server-cloudflare-local.json` with a real token.
- `configs/server-cloudflare-local.json` uses `env:ROAD_WS_TOKEN`; if the environment variable is empty, auth is disabled. This is documented, but operators must set the variable.
- `trust_proxy_headers=true` is safe only behind Cloudflare or another trusted reverse proxy. Direct exposure with spoofable proxy headers is not supported.
- UDP compatibility remains game-dependent. ROAD can prove transport behavior with `udp-check`, DDNet, and Sven Co-op baselines, but games that embed peer addresses or rely on official Steam/EOS relay traffic may still need protocol-specific work.
- Windows arm64 builds are produced as experimental artifacts and still need runtime testing on real hardware.
- GitHub Release publishing should be verified after tag CI runs in the private repo before flipping visibility to public.

## Public Readiness Verdict

Ready for a private final release run and GitHub Release verification.

Do not make the repository public until:

1. `go test ./...`, `go vet ./...`, config validation, and release packaging pass after the final commit.
2. The `v0.1.0` tag workflow creates GitHub Release assets successfully.
3. The GitHub Release page contains zip + sha256 assets for Windows amd64, Windows arm64 experimental, Linux amd64, and Linux arm64.
4. Repository visibility, Issues, Discussions, and private vulnerability reporting settings are set intentionally.
