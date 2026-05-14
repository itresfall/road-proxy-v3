# Changelog

All notable release-facing changes should be recorded in this file.

This project uses SemVer for tagged releases. Development builds use `0.1.0-dev`
unless `ROAD_VERSION` is set during build.

## 0.1.0-dev - Unreleased

- Added GNU Affero General Public License v3.0 (`AGPL-3.0-only`) licensing for
  public open-source distribution.
- Added `SECURITY.md`, `CONTRIBUTING.md`, and `docs/RELEASE-CHECKLIST.md` for
  public release readiness.
- Added release package inclusion for security and contribution documents.
- Added Public Server Wizard local data/control port selection and wired the
  chosen data origin into TryCloudflare and named Cloudflare tunnel configs.
- Added named Cloudflare tunnel ingress generation for both ROAD data `/ws` and
  control/dashboard routes.
- Added shared-token WebSocket authentication back to the data plane and
  applied the same token model to control API data endpoints.
- Added server-side Host and Origin allowlists, proxy-aware client IP handling,
  global connection limits, per-IP connection limits, and per-IP rate limits for
  public deployments.
- Added `configs/server-cloudflare-local.json` as the recommended
  Cloudflare/Nginx local-only preset.
- Added `docs/PUBLIC-DEPLOYMENT-SECURITY.md` for Cloudflare Access, WAF,
  local-only binding, token auth, and rate-limit guidance.
- Added private/public plugin API mode through `control.plugin_api_public`.
- Added runtime warnings for plugins with `udp_peer_broadcast=true`.
- Expanded `/dashboard` into an embedded Control Deck with Overview, Sessions,
  Plugins, UDP Diagnostics, Security, and API tabs while keeping it dependency
  free and inside the Go binary.
- Changed authenticated dashboard behavior so the static shell can load and ask
  for a token in the browser while `/api/*` data remains protected.
- Expanded `/api/info` with safe runtime and security posture fields for the
  dashboard.
- Fixed interactive client mode so manually entered endpoints must expose a
  valid ROAD plugin profile before the client starts. This prevents bad domains
  from silently falling back to the stale `minecraft` client config.
- Added authenticated plugin-profile discovery for client menu fetches, using
  client auth headers when configured.
- Marked the Lethal Company direct UDP profile as community-validation /
  experimental instead of a release blocker, because object/physics desync can
  occur on direct LAN without ROAD.
- Clarified Lethal Company direct UDP plugin notes so direct-LAN physics desync
  is not misattributed to ROAD without baseline evidence.
- Added shared build metadata support for all main binaries.
- Added `--version` output to `road-proxy`, `road-server`, `road-client`,`r`n  and `plugin-studio`.
- Added Windows, Linux amd64, and optional Linux arm64 release packaging through
  `scripts/build-cross.ps1 -Package`.
- Added SHA256 checksum files for release archives.
- Added GitHub Actions CI for tests, Windows build smoke, Linux build smoke, and
  tag-based release package artifacts.
- Added repository QA tests that load real config JSON files and built-in plugin
  profiles.
- Split the interactive `cmd/road` entrypoint into focused menu, server flow,
  client flow, runtime file, WebSocket URL, and port-check modules.
- Consolidated TCP/UDP port ownership checks and Windows `netstat` parsing.
- Moved default config path checks for server/client binaries into `internal/app`.
- Added UTF-8 locale infrastructure with `internal/i18n`, `locales/en.json`, and
  `locales/tr.json`.
- Moved interactive `cmd/road` user-facing text to key-based locale lookups and
  removed mojibake strings from that package.
- Moved `cmd/plugin-studio` UI text, compatibility notes, and launch hints to
  key-based locale lookups.
- Moved command-level flag and fatal error text for `road-server` and`r`n  `road-client` to locale lookups.
- Added locale QA coverage to catch missing Turkish keys for the English
  template.
- Added `docs/I18N.md` and included `locales/` in build and release outputs.
- Added `road-proxy validate` and `road-proxy validate-plugin` commands.
- Added client `server_ws_url` validation during config loading.
- Added interactive client menu repair flow for invalid `server_ws_url` values.
- Added `road-proxy generate-config` for base config plus profile override
  generation.
- Moved Plugin Studio compatibility profiles from Go code into
  `compat-profiles/*.json`.
- Added compatibility profile schema, ROAD scope fields, match confidence, and
  match reasons for Plugin Studio reports.
- Added Plugin Studio process identity signals: executable path, executable
  SHA256, Steam AppID detection, and match reasons for those signals. Steam
  AppID remains an identity hint only, not a Steam relay/lobby support claim.
- Added non-interactive Plugin Studio flags for process selection, capture
  duration, network/port overrides, plugin name override, UDP peer broadcast,
  and force overwrite.
- Added optional Plugin Studio multi-phase capture for lobby, connect, ingame,
  and disconnect phases, plus phase port role classification in reports.
- Added Plugin Studio socket-snapshot `packet_fingerprint` metadata for flow
  direction, tick frequency, burst/streak stats, and TCP handshake estimates.
  Packet size is explicitly reported as unavailable without packet capture.
- Added Plugin Studio `topology` heuristics for server/host, client-to-server,
  peer-to-peer candidate, mixed, and unknown capture classification.
- Added payload fingerprint spike and capture replay draft documents.
- Added Linux Plugin Studio runtime scanning through `ss`, `lsof`, and `/proc`
  fallbacks, with process metadata read from `/proc/<pid>`.
- Added Windows Plugin Studio endpoint scanning through PowerShell NetTCPIP
  cmdlets, keeping `netstat` as fallback for older or restricted systems.
- Added Plugin Studio `port_selection` reports with selected port, selection
  reason, and rejected candidate ports.
- Added Plugin Studio `unknown-game-report.json` output for unmatched processes.
- Added deterministic UDP impairment tests for RakNet-style metrics covering
  packet loss, packet reorder, duplicate packets, burst jitter, max gap, and
  loss percentage.
- Added a repeatable UDP-over-WebSocket head-of-line measurement test and
  documentation for same-session UDP burst backlog behavior.
- Added a QUIC/WebTransport spike document and kept WebSocket as the default
  transport while framing QUIC DATAGRAM as a future opt-in experiment.
- Added UDP reply source policy for plugins: `strict`, `same_ip`, and legacy
  `any`, with e2e coverage for alternate source port replies.
- Added UDP datagram truncation guards that drop payloads larger than
  `buffer_size` instead of forwarding silently truncated data, plus one-time
  conservative MTU warnings for datagrams above 1200 bytes.
- Added an e2e UDP reconnect continuity risk test and documentation showing
  that target-visible UDP source ports can change after session reconnect.
- Added a UDP peer broadcast slow-peer measurement test and async/drop policy
  design document.
- Added LAN discovery, broadcast, and multicast analysis documenting that ROAD
  remains a direct unicast IP:port proxy, not a LAN broadcast emulator.
- Added payload embedded IP/port adapter hook design for future
  protocol-aware, opt-in address rewrite support.
- Added multi-client config generation through `road-proxy generate-config`
  with `--client-instances` and `--client-start-port`.
- Added `/api/ping` on data/control planes and `road-proxy ping` for
  lightweight ROAD server RTT measurement without starting a game.
- Added aggregate UDP jitter, max-gap, RakNet-style loss, reorder, and duplicate
  counters to `/api/stats`.
- Added per-plugin `/api/stats.plugins` counters for active/total sessions,
  bytes, errors, and UDP health metrics.
- Added per-session stats and `/api/sessions` for active session inspection,
  including plugin, transport, remote/target addresses, tx/rx bytes, age, idle
  time, and UDP health counters.
- Added `/dashboard`, a no-build local control dashboard for clients, plugins,
  ports, traffic, errors, ping, and active sessions.
- Added config-driven JSON logging for server/client runtime logs through
  `logging.format: "json"`, with `text` remaining the default.
- Added `road-proxy diagnostic-bundle` to collect configs, plugin definitions,
  logs, version metadata, and a netstat/ss snapshot into a support zip.
- Added config-driven UDP metadata recording through `udp_record`, writing
  JSONL direction/timing/size/session metadata without storing payload bytes.
- Added UDP recorder and replay/analysis draft documentation.
- Added Linux deployment helper, systemd service templates, and Linux
  deployment documentation under `deploy/` and `docs/LINUX-DEPLOYMENT.md`.
- Added `-WhatIfOnly` dry-run output to the Linux deploy helper.
- Added Windows NSSM service guidance, Windows/Linux firewall helpers,
  deployment presets, a `roadctl` operations plan, and Docker evaluation docs.
- Build outputs now include `deploy/` alongside configs, plugins, locales,
  docs, and compatibility profiles.
- Added GZDoom UDP and Lethal Company direct UDP acceptance documents.
- Split the large Plugin Studio entrypoint into focused flow, capture,
  fingerprint, topology, port analysis, report, scanner, signals, input, and
  type files.
- Changed generated known-profile plugin drafts to reference profile notes by
  `notes_source` instead of copying compatibility notes into multiple maintained
  places.
- Expanded plugin schema with optional compatibility metadata: status, tested
  players, known ports, launch args, notes, and last verified date.
- Documented config/plugin system decisions in `docs/CONFIG-PLUGIN-SYSTEM.md`.
- Build scripts now copy `docs/` into platform output folders.
- Kept ROAD scope explicit: direct/LAN/local TCP or UDP port traffic only, not a
  full VPN or Steam/EOS relay replacement.
- Documented current GZDoom and Lethal Company direct UDP profile behavior.
