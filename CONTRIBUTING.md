# Contributing

ROAD Proxy v3 accepts focused contributions that preserve the project scope:
direct/LAN/local TCP or UDP port traffic over a WebSocket-backed proxy.

## Project Scope

In scope:

- Windows and Linux runtime fixes.
- Direct TCP/UDP game or local-service profiles.
- Plugin Studio improvements.
- Public deployment hardening.
- Tests, docs, and reproducible diagnostics.

Out of scope by default:

- Full VPN behavior.
- LAN broadcast/multicast emulation as a default mode.
- Steam/EOS lobby or relay replacement claims.
- macOS as an official maintained target.
- Closed-source forks presented as compatible ROAD releases.

macOS support can be accepted only from a contributor who can build and test it
on real macOS and provide runtime evidence.

## Development Setup

Requirements:

- Go 1.23 or newer.
- Windows PowerShell for Windows build scripts, or Bash for Linux native builds.

Common commands:

```powershell
go test ./...
go run ./cmd/road validate --all-configs
./scripts/build-windows.ps1
./scripts/build-linux.ps1
./scripts/build-cross.ps1 -Package
```

Linux native build:

```bash
./scripts/build-linux.sh auto
```

## Contribution Rules

- Keep changes small and reviewable.
- Add or update tests for behavior changes.
- Update docs when config, CLI, plugin schema, or deployment behavior changes.
- Keep default UDP behavior conservative: `udp_peer_broadcast=false`.
- Do not claim a game is supported unless direct test evidence exists.
- Mark uncertain game profiles as `experimental` or community-validation.
- Do not add secrets, tokens, logs with public IPs, or personal data.

## Plugin Profiles

Plugin profiles live in `plugins/<name>/plugin.json`.

For a new profile, include:

- Network and target port.
- Server/client config references.
- Compatibility status.
- Tested player count.
- Known ports.
- Launch notes.
- Direct/LAN/local scope notes.

If the game uses Steam/EOS transport, say so explicitly and avoid implying ROAD
can carry that traffic unless packet evidence proves a direct local port path.

## Pull Request Checklist

Before opening a PR:

- Run `go test ./...`.
- Run `go run ./cmd/road validate --all-configs`.
- Run at least the relevant platform build script.
- Update `CHANGELOG.md` for release-facing changes.
- Update `docs/PROJECT-ROADMAP.md` or `docs/PROJECT-TODO.md` if the change
  closes or creates roadmap work.

## License

By contributing, you agree that your contribution is licensed under
`AGPL-3.0-only`, matching the project license.

