# Changelog

All notable release-facing changes should be recorded in this file.

This project uses SemVer for tagged releases. Development builds use `0.2.0-dev`
unless `ROAD_VERSION` is set during build.

## v0.2.0 - 2026-08-05

- Added named multi-port UDP targets and matching client listeners while
  preserving legacy single-target plugin compatibility.
- Added the field-tested Sons Of The Forest dedicated-server profile for UDP
  ports `8766`, `9700`, and `27016`, plus a public acceptance document.
- Added optional Windows `pktmon` packet metadata capture to Plugin Studio.
  It falls back to socket snapshots when unavailable and deletes raw temporary
  captures after producing aggregate size, timing, port, and RakNet signals.
- Hardened publication hygiene: packet captures and local environment files are
  ignored, raw local test logs are not source artifacts, and incomplete draft
  profiles were removed from release inputs.

## v0.1.0 - 2026-05-15

- Initial public release of ROAD Proxy v3 as a WebSocket-backed TCP/UDP tunnel
  for direct/LAN/local IP:port game and service traffic.
- Added the all-in-one `road-proxy` menu with Engine/Server, Client, Public
  Server Wizard, Settings, and Exit flows.
- Added Settings for language selection, base client/server config editing,
  normal server/client auth toggles, config path discovery, and optional app
  restart after save.
- Added UTF-8 locale support with English and Turkish locale files.
- Added Public Server Wizard for TryCloudflare, named Cloudflare tunnel login +
  DNS route mode, and existing Cloudflare tunnel-token mode.
- Added generated ROAD token auth for public wizard configs, client-side token
  prompts, and clearer `401` / `403` auth diagnostics.
- Added Host/Origin allowlists, proxy-aware client IP handling, connection
  limits, per-IP limits, rate limiting, and private/public plugin API control.
- Added the embedded dependency-free dashboard and control API for status,
  sessions, plugins, UDP diagnostics, security posture, and endpoint hints.
- Added JSON runtime plugins for Minecraft Java, Minecraft Bedrock UDP, DDNet,
  GZDoom UDP, Lethal Company direct UDP, and Sven Co-op UDP.
- Added compatibility profile contribution docs so community game profiles can
  be added through `compat-profiles/*.json`.
- Added Plugin Studio for Windows/Linux process scanning, compatibility profile
  matching, multi-phase capture metadata, topology hints, and generated plugin
  drafts.
- Added UDP diagnostics including metrics, jitter/max-gap/loss estimates,
  `udp-check`, UDP recorder metadata, oversized datagram drops, MTU warnings,
  reconnect continuity tests, and peer-broadcast risk docs.
- Added GZDoom guidance for 3+ player sessions: use `-netmode 1` and keep UDP
  peer broadcast disabled.
- Added Lethal Company direct/LAN profile notes and marked it experimental
  because direct LAN physics/object desync can happen without ROAD.
- Added release packaging for Windows amd64/arm64 and Linux amd64/arm64 with
  checksum files, release plugin filtering, and GitHub Actions release upload.
- Added AGPL-3.0-only licensing, `SECURITY.md`, `CONTRIBUTING.md`, public
  deployment security docs, and release checklist docs.
