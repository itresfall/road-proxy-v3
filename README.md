# ROAD Proxy v3

ROAD Proxy v3 is a WebSocket-backed TCP/UDP tunnel for games and local services. It uses JSON plugin profiles instead of native `.dll` or `.so` extensions.

The practical goal: run ROAD server near the real game host, run ROAD client near the player, and make the game talk to a local `127.0.0.1:<port>` endpoint.

## What It Does

- Tunnels TCP or UDP traffic over a WebSocket data plane.
- Routes traffic through named JSON plugin profiles.
- Provides separate binaries for server, client, menu, and Plugin Studio.
- Supports UDP session metrics for packet count, bytes, jitter, and RakNet-style loss estimates.
- Supports optional UDP peer broadcast, disabled by default.
- Keeps build scripts minimal: only Windows, Linux, and cross-build helpers live in `scripts/`.

## What It Is Not

- Not a full VPN or virtual LAN adapter.
- Not a LAN broadcast emulator.
- Not a Steam relay/lobby replacement.
- Not a protocol-aware payload rewriter unless a future adapter adds that explicitly.

## Quick Start

Requirements:

- Go 1.23 or newer for source builds.
- Windows amd64 or Linux amd64/arm64 for maintained runtime builds.
- Windows arm64 is available as an experimental cross-build artifact until real
  ARM hardware testing exists.
- macOS is not an official target; external tested PRs can add it later.

Build Windows binaries:

```powershell
./scripts/build-windows.ps1
./scripts/build-windows.ps1 -Arch arm64
```

Run the interactive menu:

```powershell
./build/windows/road-proxy.exe
```

Run server and client directly from source:

```powershell
go run ./cmd/server -config configs/server.json
go run ./cmd/client -config configs/client.json
```

Run server and client from Windows build output:

```powershell
./build/windows/road-server.exe -config configs/server.json
./build/windows/road-client.exe -config configs/client.json
```

Run tests:

```powershell
go test ./...
```

Validate configs and plugins:

```powershell
./build/windows/road-proxy.exe validate --server configs/server.json --client configs/client.json
./build/windows/road-proxy.exe validate --all-configs
./build/windows/road-proxy.exe validate-plugin plugins/gzdoom-udp/plugin.json
```

Generate a plain server/client config pair from base config plus a profile
override:

```powershell
./build/windows/road-proxy.exe generate-config --profile gzdoom
./build/windows/road-proxy.exe generate-config --profile ddnet
./build/windows/road-proxy.exe generate-config --profile lethal-company --out configs/.generated
./build/windows/road-proxy.exe generate-config --profile gzdoom --client-instances 2 --client-start-port 5031
```

Measure ROAD client-server HTTP RTT against the data plane or control API:

```powershell
./build/windows/road-proxy.exe ping --url ws://127.0.0.1:8080/ws --count 4
./build/windows/road-proxy.exe ping --url http://127.0.0.1:8081 --count 4
```

Run a synthetic UDP game-flow check without depending on a real game's netcode:

```powershell
./build/windows/road-proxy.exe udp-check server --listen 127.0.0.1:7777
./build/windows/road-proxy.exe udp-check client --target 127.0.0.1:25568 --players 3 --tickrate 30 --duration 2m --payload 256
```

See `docs/UDP-CHECK.md` for direct LAN, ROAD LAN, WSS/domain, and large-payload
test patterns.

DDNet is the recommended free UDP game baseline for 3-player ROAD validation:

```text
plugin: ddnet-udp
server config: configs/server-ddnet.json
client config: configs/client-ddnet.json
DDNet server target: 127.0.0.1:8303
ROAD client listen: 127.0.0.1:18303
```

See `docs/DDNET-ROAD-SETUP.md` and `docs/DDNET-UDP-ACCEPTANCE.md`.

Create a diagnostic zip for support/debugging:

```powershell
./build/windows/road-proxy.exe diagnostic-bundle --out diagnostics
```

## Build Scripts

Only build scripts are kept under `scripts/`:

```text
scripts/build-windows.ps1
scripts/build-linux.ps1
scripts/build-linux.sh
scripts/build-cross.ps1
```

Windows build:

```powershell
./scripts/build-windows.ps1
```

Linux cross build from PowerShell:

```powershell
./scripts/build-linux.ps1 -Arch amd64
./scripts/build-linux.ps1 -Arch arm64
```

Linux native build:

```bash
./scripts/build-linux.sh auto
./scripts/build-linux.sh amd64
./scripts/build-linux.sh arm64
```

Cross build:

```powershell
./scripts/build-cross.ps1
```

Build scripts also copy current `configs/`, `plugins/`, `locales/`, `docs/`,
`compat-profiles/`, and `deploy/` into the output folder.

## Versioning and Release Packages

ROAD uses SemVer for tagged releases. Local development builds default to
`0.1.0-dev`.

Check binary metadata:

```powershell
./build/windows/road-proxy.exe --version
./build/windows/road-server.exe --version
./build/windows/road-client.exe --version
./build/windows/plugin-studio.exe --version
```

Build scripts embed:

- `ROAD_VERSION`, default `0.1.0-dev`
- `ROAD_COMMIT`, default detected from `git rev-parse --short HEAD`
- `ROAD_BUILD_DATE`, default current UTC time

Example release build:

```powershell
$env:ROAD_VERSION = "0.1.0"
./scripts/build-cross.ps1 -Package
```

Release archive names use:

```text
road-proxy-v3_<version>_<os>_<arch>.zip
```

`scripts/build-cross.ps1 -Package` creates archive and checksum files under
`build/release/`. Package contents include binaries, `configs/`, `plugins/`,
`locales/`, `deploy/`, `scripts/`, `README.md`, `CHANGELOG.md`, `LICENSE`,
`SECURITY.md`, `CONTRIBUTING.md`, and `VERSION.txt`.

`build/` is local/CI output and should not be committed. Tagged CI runs publish
the zip and checksum files as GitHub Release assets.

Linux arm64 is included by default in cross builds. Windows arm64 is opt-in
because it is not runtime-tested yet.

```powershell
./scripts/build-cross.ps1 -Package -IncludeLinuxArm64:$false
./scripts/build-cross.ps1 -Package -IncludeWindowsArm64
```

## CI

The GitHub Actions workflow lives at:

```text
.github/workflows/ci.yml
```

It runs on push, pull request, and manual dispatch:

- `go test ./...`
- Linux amd64 build smoke with `scripts/build-linux.sh amd64`
- Linux arm64 cross-build smoke with `scripts/build-linux.sh arm64`
- Windows amd64 build smoke with `scripts/build-windows.ps1`
- Windows arm64 cross-build smoke with `scripts/build-windows.ps1 -Arch arm64`

On Git tags, CI also runs:

```powershell
./scripts/build-cross.ps1 -Package
```

and uploads release zip/checksum files as workflow artifacts.

## Language Files

The interactive `road-proxy` menu, Plugin Studio, and command-level binary
messages use UTF-8 locale files:

```text
locales/en.json
locales/tr.json
```

The active language is configured in `configs/app.json`. Missing translation
keys fall back to English. Translation rules are documented in:

```text
docs/I18N.md
```

## Linux Deployment

Linux deployment helpers live under `deploy/`, not `scripts/`.

```powershell
./scripts/build-linux.ps1 -Arch amd64
./deploy/linux/deploy-linux.ps1 -Target user@SERVER-IP -Arch amd64 -InstallSystemd -Restart
./deploy/linux/deploy-linux.ps1 -Target user@SERVER-IP -Arch amd64 -InstallSystemd -Restart -WhatIfOnly
```

Service templates:

```text
deploy/systemd/road-server.service
```

Detailed notes:

```text
docs/LINUX-DEPLOYMENT.md
docs/WINDOWS-SERVICE-NSSM.md
docs/FIREWALL.md
docs/DEPLOYMENT-PRESETS.md
docs/ROADCTL-PLAN.md
docs/DOCKER-EVALUATION.md
docs/RELEASE-CHECKLIST.md
```

## Core Flow

1. The game connects to the local ROAD client listener, for example `127.0.0.1:7777`.
2. ROAD client opens a WebSocket tunnel to ROAD server, for example `ws://host:8080/ws`.
3. ROAD client passes `plugin_name` to the server.
4. ROAD server loads `plugins/<name>/plugin.json`.
5. ROAD server forwards traffic to the plugin target, for example `127.0.0.1:7777` on the host machine.

Default data plane:

```text
0.0.0.0:8080/ws
```

Default control API:

```text
0.0.0.0:8081
```

## Compatibility Profiles

Runtime plugin profiles live in `plugins/<name>/plugin.json`.
Community compatibility profiles live in `compat-profiles/<id>.json`.

If ROAD is missing a game you tested, add a JSON profile under
`compat-profiles/` and open a PR. This is the intended community contribution
path: small, evidence-based game profiles are easier to review than broad code
changes. See `compat-profiles/README.md` and
`docs/COMPATIBILITY-CONTRIBUTING.md` for the required evidence, confidence
rules, and validation commands.

`plugins/` answers "how does ROAD forward this traffic at runtime?"
`compat-profiles/` answers "how does Plugin Studio recognize and document this
game?"

| Profile | Network | Host target | Client listen | Notes |
| --- | --- | --- | --- | --- |
| `ddnet-udp` | UDP | `127.0.0.1:8303` | usually `127.0.0.1:18303` | Recommended free UDP baseline for multiplayer validation. |
| `minecraft` | TCP | `127.0.0.1:25565` or LAN server IP | usually `127.0.0.1:25568` | Java edition style TCP forwarding. |
| `minecraft-bedrock-udp` | UDP | `127.0.0.1:19132` | usually `127.0.0.1:19133` | RakNet UDP. |
| `gzdoom-udp` | UDP | `127.0.0.1:5029` | `127.0.0.1:5029` | Use `-netmode 1`; keep peer broadcast off. |
| `lethal-company-udp` | UDP | `127.0.0.1:7777` | `127.0.0.1:7777` | Experimental community profile for direct/LAN/local port traffic, not Steam relay traffic. |
| `sven-coop-udp` | UDP | `127.0.0.1:27015` | usually `127.0.0.1:27015` | GoldSrc/Sven Co-op direct server traffic baseline. |

A plugin target is the real service on the server side. A client listen address is what the local player connects to.

## Plugin Studio

Plugin Studio watches a live Windows or Linux process, samples active sockets, recommends a network/port, and writes:

- `plugins/<plugin>/plugin.json`
- `plugins/<plugin>/studio-report.json`
- `configs/server-<plugin>.json`
- `configs/client-<plugin>.json`

Run it from source:

```powershell
go run ./cmd/plugin-studio
```

Run it from Windows build output:

```powershell
./build/windows/plugin-studio.exe
```

Run it from Linux build output:

```bash
./build/linux/amd64/plugin-studio
```

Non-interactive example:

```powershell
./build/windows/plugin-studio.exe --process "Lethal Company" --seconds 20 --plugin-name lethal-company-udp --force
```

Multi-phase non-interactive example:

```powershell
./build/windows/plugin-studio.exe --process "gzdoom" --multi-phase --phase-seconds 8 --plugin-name gzdoom-udp --force
```

Useful non-interactive flags:

- `--pid` or `--process`: choose the process to capture.
- `--multi-phase`, `--phase-seconds`: capture lobby/connect/ingame/disconnect phases and write phase roles.
- `--network tcp|udp`, `--target-host`, `--target-port`: override the target.
- `--client-listen-port`: override local client listen port.
- `--udp-peer-broadcast`: explicitly enable UDP peer broadcast.
- `--force`: overwrite an existing generated plugin draft.

Current behavior:

- Detects compatibility profiles for DDNet, GZDoom, Lethal Company direct UDP, Minecraft Bedrock, and Minecraft Java.
- Loads compatibility profiles from `compat-profiles/*.json`.
- Scans Windows sockets with PowerShell NetTCPIP cmdlets (`Get-NetTCPConnection`, `Get-NetUDPEndpoint`) and falls back to `netstat`.
- Reads Windows process metadata with PowerShell/CIM and falls back to `tasklist`.
- Scans Linux sockets with `ss -H -tunap`, then `lsof -nP -iTCP -iUDP`, then `/proc/net/*` plus `/proc/<pid>/fd` socket inode fallback.
- Reads Linux process metadata from `/proc/<pid>/comm`, `/proc/<pid>/exe`, and `/proc/<pid>/cmdline`.
- Can run interactively or in non-interactive mode with `--pid` / `--process`.
- Writes match confidence and match reasons into `studio-report.json`.
- Uses Steam AppID and executable SHA256 only as optional identity signals; they do not imply Steam lobby/relay support.
- Adds process path, executable SHA256, and detected Steam AppID to reports when Windows exposes that information.
- Writes `port_selection` into `studio-report.json`: selected network/port, selection reason, and rejected candidate ports.
- Optional multi-phase capture writes `multi_phase` into reports with `lobby`, `connect`, `ingame`, and `disconnect` captures.
- Multi-phase reports classify ports as `persistent`, `connect_only`, `game_only`, `lobby_only`, `disconnect_only`, `connect_and_game`, or `multi_phase`.
- Writes `packet_fingerprint` metadata from socket snapshots: flow direction, tick frequency, burst/streak stats, and TCP handshake estimates.
- Packet sizes are explicitly marked unavailable in this mode because current scanners do not capture packets or payloads.
- Writes `topology` heuristics to classify the capture as `server_or_host`, `client_to_server`, `peer_to_peer_candidate`, `mixed_or_unclear`, or `unknown`.
- Writes `unknown-game-report.json` for processes that do not match a compatibility profile.
- Prefers stable non-ephemeral game ports over noisy ephemeral TCP ports.
- Asks separately for server target port and local client listen port.
- Uses compatibility profiles as the source of truth for known profile notes; generated plugin drafts reference `notes_source` instead of copying those notes.
- Writes `compatibility` metadata into generated plugin drafts.
- Keeps `runtime.udp_peer_broadcast=false` by default for UDP profiles.
- Writes `runtime.udp_reply_policy` for UDP drafts when a profile provides one.
- Raw payload fingerprint and capture replay plans are documented in `docs/PLUGIN-STUDIO-PAYLOAD-FINGERPRINT-SPIKE.md` and `docs/PLUGIN-STUDIO-CAPTURE-REPLAY-DRAFT.md`.

## Manual Plugin Shape

Minimum UDP plugin example:

```json
{
  "schema_version": "v1",
  "name": "custom-game-udp",
  "version": "3.0.0",
  "target": {
    "network": "udp",
    "address": "127.0.0.1:7777"
  },
  "runtime": {
    "type": "json",
    "mode": "passthrough",
    "udp_peer_broadcast": false,
    "udp_reply_policy": "same_ip",
    "client_pipeline": [],
    "server_pipeline": []
  }
}
```

Schema reference:

```text
configs/plugin.schema.v1.json
```

## UDP Rules

1. Prefer the game's own server, packet-server, or client-server mode.
2. Keep `runtime.udp_peer_broadcast=false` unless packet captures prove clients must receive each other's client-origin packets.
3. Use `runtime.udp_reply_policy` to control accepted reply sources: `strict` accepts only exact target IP:port, `same_ip` accepts the target IP on any port, `any` preserves legacy accept-all behavior.
4. For GZDoom 3+ player sessions, use `-netmode 1`; peer broadcast is not the correct fix for that case.
5. Use one game client per machine first. If multiple local clients are needed, give each ROAD client a different listen port.
6. If a game embeds peer IP/port values inside payloads, ROAD may need a protocol-aware adapter.
7. ROAD drops UDP datagrams larger than configured `buffer_size` instead of forwarding silently truncated payloads.
8. UDP datagrams above 1200 bytes are allowed but logged once as conservative WAN/MTU risk.
9. ROAD does not emulate LAN discovery, subnet broadcast, or multicast. Prefer manual direct IP:port connect.

UDP diagnostics:

```text
docs/UDP-MULTIPLAYER-DIAGNOSIS.md
docs/UDP-WEBSOCKET-HOL-MEASUREMENT.md
docs/UDP-RECONNECT-CONTINUITY.md
docs/UDP-PEER-BROADCAST-SLOW-PEER.md
docs/UDP-LAN-DISCOVERY-BROADCAST-MULTICAST.md
docs/UDP-PAYLOAD-ADDRESS-ADAPTER-HOOK.md
docs/MULTI-CLIENT-INSTANCES.md
docs/QUIC-WEBTRANSPORT-SPIKE.md
docs/UDP-RECORDER.md
docs/UDP-REPLAY-ANALYSIS-DRAFT.md
docs/GZDOOM-UDP-ACCEPTANCE.md
docs/LETHAL-COMPANY-UDP-ACCEPTANCE.md
```

## GZDoom / FNaF Doom Reborn

Stable rule:

```text
-netmode 1
```

Host pattern:

```powershell
gzdoom.exe ... -host 3 -netmode 1
```

Client pattern:

```powershell
gzdoom.exe ... -join 127.0.0.1 -netmode 1
```

Reference files:

```text
plugins/gzdoom-udp/plugin.json
configs/server-gzdoom-linux.json
configs/client-gzdoom-p2.json
configs/client-gzdoom-p3.json
```

Important: `udp_peer_broadcast` should stay `false` for the fixed GZDoom setup. `udp_reply_policy=same_ip` is the intended profile setting.

## Lethal Company Direct UDP

The included experimental profile assumes a mod or direct/LAN mode publishes locally on UDP `7777`. It is intentionally shipped as a community-test profile: if it works for a setup, keep the logs; if it desyncs, compare against direct LAN before blaming ROAD.

Reference files:

```text
plugins/lethal-company-udp/plugin.json
configs/server-lethal-company.json
configs/client-lethal-company.json
```

Run directly:

```powershell
go run ./cmd/server -config configs/server-lethal-company.json
go run ./cmd/client -config configs/client-lethal-company.json
```

If the game uses Steam lobby/relay transport instead of direct/local UDP, this profile will not see the real traffic.

Known limitation: Lethal Company can show object/physics desync even on direct LAN without ROAD. Treat this profile as useful-but-experimental until community logs prove stable patterns.

## Security And Contributions

Security reporting and public deployment expectations are documented in:

```text
SECURITY.md
docs/PUBLIC-DEPLOYMENT-SECURITY.md
```

Contribution rules, supported scope, macOS policy, and PR expectations are in:

```text
CONTRIBUTING.md
```

Release preflight steps are in:

```text
docs/RELEASE-CHECKLIST.md
```

## Minecraft

Reference profiles and docs:

```text
plugins/minecraft/plugin.json
plugins/minecraft-bedrock-udp/plugin.json
docs/MINECRAFT-ACCEPTANCE.md
docs/MINECRAFT-BEDROCK-UDP-ACCEPTANCE.md
```

Run Java TCP profile directly:

```powershell
go run ./cmd/server -config configs/server.json
go run ./cmd/client -config configs/client.json
```

Run Bedrock UDP profile directly:

```powershell
go run ./cmd/server -config configs/server-udp.example.json
go run ./cmd/client -config configs/client-udp.example.json
```

## Config Files

Server config controls HTTP/WebSocket, control API, and enabled plugins:

```json
{
  "http": {
    "listen_addr": "0.0.0.0:8080",
    "ws_endpoint": "/ws"
  },
  "control": {
    "listen_addr": "0.0.0.0:8081"
  },
  "plugins": {
    "dir": "plugins",
    "enabled": ["lethal-company-udp"]
  },
  "logging": {
    "format": "text"
  }
}
```

Client config controls local listen and remote WebSocket URL:

```json
{
  "listen_network": "udp",
  "listen_addr": "127.0.0.1:7777",
  "server_ws_url": "ws://SERVER-IP:8080/ws",
  "plugin_name": "lethal-company-udp",
  "logging": {
    "format": "text"
  }
}
```

Security knobs:

- `http.auth_token` / `http.auth_tokens`: enable shared-token auth for WebSocket and control API.
- `auth_token`: client-side token sent during WebSocket connect.
- `auth_header`: header name, defaults to `X-ROAD-Token` when a token is set.
- `env:NAME`: token values can be read from environment variables.
- `configs/server-cloudflare-local.json`: local-only Cloudflare/Nginx preset for `127.0.0.1:8080` and `127.0.0.1:8081`.
- `allowed_hosts` / `allowed_origins`: optional Host and browser Origin allowlists.
- `max_connections`, `max_connections_per_ip`, `rate_limit_per_minute`: optional public deployment limits.
- `control.plugin_api_public=false`: keep plugin info/download endpoints private unless auth is enabled.
- When auth is enabled, `/dashboard` can still load the static shell and asks
  for the ROAD token in the browser; `/api/*` data remains protected.

Public Cloudflare helper:

```bash
road-proxy public-server
```

This starts an interactive wizard for TryCloudflare, existing tunnel-token mode,
or `cloudflared tunnel login` + DNS route mode. It generates a local-only ROAD
server config, a matching client config hint under `configs/.generated/`, and
prints the public `wss://.../ws` endpoint plus the generated ROAD token.
The wizard asks for local ROAD data/control ports. If the data port is changed,
the generated ROAD config, TryCloudflare `--url`, named tunnel ingress service,
and printed local origin are changed together.
When `cloudflared` is auto-downloaded, ROAD verifies SHA256 checksum data from
the latest Cloudflare `cloudflared` GitHub release when checksum data is
available.
Permanent domain mode uses `cloudflared tunnel login` and the generated
`cert.pem`; ROAD does not ask for a Cloudflare API token. The only manual
provider-side step is connecting the domain to Cloudflare nameservers.
In domain mode, ROAD writes a named tunnel ingress config that routes `/ws` to
the data port and routes `/dashboard` plus `/api/*` to the control port. Control
API data remains protected by the generated ROAD token.
Existing Cloudflare tunnel-token mode cannot edit Cloudflare's remote tunnel
configuration; that tunnel must already point to the local origin printed by
the wizard.

Useful stability knobs:

- `ws_idle_timeout`
- `ws_ping_interval`
- `max_ws_message_bytes`
- `udp_session_idle_timeout`
- `udp_metrics_log_interval`
- `buffer_size`
- `logging.format`: `text` or `json`

Set JSON logs when another tool will ingest ROAD output:

```json
{
  "logging": {
    "format": "json"
  }
}
```

Enable short UDP metadata recording when debugging a game session:

```json
{
  "udp_record": {
    "enabled": true,
    "path": "logs/udp-record-client.jsonl",
    "duration": "30s",
    "max_events": 5000
  }
}
```

The recorder writes JSONL packet metadata only: direction, timestamp, session,
source/destination labels, packet size, and optional RakNet sequence. It does
not write UDP payload bytes.

## Diagnostic Bundle

For support/debugging, collect a single zip with config JSON files, plugin
definitions, logs when a `logs/` folder exists, ROAD version metadata, and an
OS network snapshot (`netstat` on Windows, `ss` on Linux):

```powershell
./build/windows/road-proxy.exe diagnostic-bundle --out diagnostics
```

Useful flags:

- `--server`, `--client`: include specific config files.
- `--plugins`: include a specific plugin directory.
- `--logs`: include a specific log directory.
- `--skip-net`: skip the network snapshot.

## Control API

Available by default on `http://host:8081`:

- `GET /dashboard` serves the embedded Control Deck for clients, plugins,
  ports, traffic, errors, ping, active sessions, UDP diagnostics, security
  posture, and copyable endpoint hints.
- `GET /api/health`
- `GET /api/ping`
- `GET /api/stats` includes aggregate and per-plugin counters for sessions,
  bytes, errors, and `udp.rx` / `udp.tx` packet, byte, jitter, max-gap, loss,
  reorder, duplicate counters, plus active session snapshots.
- `GET /api/sessions` lists active sessions with plugin, transport, remote
  address, target address, tx/rx bytes, age, idle time, and UDP health counters.
- `GET /api/plugins`
- `GET /api/plugin/info/{name}`
- `GET /api/plugin/config/{name}`
- `GET /api/plugin/download/{name}`
- `GET /api/info`

## Testing

Run all tests:

```powershell
go test ./...
```

Measure ROAD server RTT without starting a game:

```powershell
./build/windows/road-proxy.exe ping --url ws://SERVER-IP:8080/ws --count 4
```

Open the local dashboard:

```text
http://SERVER-IP:8081/dashboard
```

Run UDP stress tests:

```powershell
go test ./internal/e2e -run "TestUDPWebSocket" -count=1 -v
go test ./internal/e2e -run TestUDPWebSocketSyncFiveClientsStress -count=1 -v
```

Run UDP impairment metric tests:

```powershell
go test ./internal/client -run TestUDPSessionMetricsWithImpairmentSimulation -count=1 -v
```

Run UDP over WebSocket HOL measurement:

```powershell
go test ./internal/e2e -run TestUDPWebSocketHOLMeasurement -count=1 -v
```

Run UDP truncation guard test:

```powershell
go test ./internal/e2e -run TestUDPWebSocketDropsOversizedDatagrams -count=1 -v
```

Run UDP reconnect continuity risk test:

```powershell
go test ./internal/e2e -run TestUDPReconnectMayChangeTargetVisibleSourcePort -count=1 -v
```

Run UDP peer broadcast slow peer measurement:

```powershell
go test ./internal/engine -run TestUDPPeerBroadcastSlowPeerBlocksCurrentBroadcast -count=1 -v
```

## Troubleshooting

If the game cannot connect:

- Confirm the game is pointed at the ROAD client listen address.
- Confirm `server_ws_url` points to the ROAD server data plane, not the control API.
- Confirm the server config enables the same plugin name used by the client.
- Confirm the plugin target address is reachable from the server machine.
- For UDP, watch client metrics and `studio-report.json` before changing peer broadcast.

If Linux says `Exec format error`:

```bash
file build/linux/amd64/road-proxy
uname -m
```

Then rebuild with the matching architecture.

## Repository Map

```text
cmd/server           ROAD server binary
cmd/client           ROAD client binary
cmd/road             interactive all-in-one menu binary
cmd/plugin-studio    Windows process scanner and plugin generator
internal/engine      TCP/UDP/WebSocket forwarding engine
internal/plugin      JSON plugin loader and runtime options
configs/             runtime config templates
plugins/             built-in plugin profiles
scripts/             build scripts only
deploy/              deployment helpers and service templates
docs/                acceptance notes, roadmap, diagnostics
```

## License

ROAD Proxy v3 is released under the GNU Affero General Public License v3.0
only (`AGPL-3.0-only`). See `LICENSE`.

You may use, study, modify, and share the code. If you distribute a modified
version or run a modified network service for users, you must provide the
corresponding source under the same license.

