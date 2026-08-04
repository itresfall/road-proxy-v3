# ROAD Config and Plugin System

Date: 2026-05-11

This document records the Phase 5 config/plugin decisions.

## Validation Commands

Validate one server/client pair:

```powershell
road-proxy validate --server configs/server.json --client configs/client.json
```

Validate all current server/client configs:

```powershell
road-proxy validate --all-configs
```

Validate a plugin file:

```powershell
road-proxy validate-plugin plugins/<name>/plugin.json
```

`validate` loads server config, client config, referenced plugin JSON files, and checks `server_ws_url` syntax. It does not open sockets or contact remote hosts.

Generate plain server/client configs from base files and a profile override:

```powershell
road-proxy generate-config --profile gzdoom
road-proxy generate-config --profile lethal-company --out configs/.generated
road-proxy generate-config --profile gzdoom --client-instances 2 --client-start-port 5031
```

## Plugin Compatibility Metadata

Plugin schema v1 now supports optional `compatibility` metadata:

```json
{
  "compatibility": {
    "status": "working",
    "tested_players": 3,
    "known_ports": [
      {
        "network": "udp",
        "port": 5029,
        "role": "target",
        "notes": "GZDoom UDP game traffic"
      }
    ],
    "launch_args": [
      "-host 3 -netmode 1",
      "-join 127.0.0.1 -netmode 1"
    ],
    "notes": [
      "Keep udp_peer_broadcast disabled."
    ],
    "last_verified": "2026-05-11"
  }
}
```

Allowed status values:

- `unknown`
- `experimental`
- `partial`
- `working`
- `broken`

`last_verified` uses `YYYY-MM-DD`.

## UDP Runtime Policy

UDP plugins can set two runtime-specific fields:

```json
{
  "runtime": {
    "udp_peer_broadcast": false,
    "udp_reply_policy": "same_ip"
  }
}
```

`udp_peer_broadcast` controls whether client-origin UDP packets are also sent to other active ROAD clients for the same plugin. It should stay `false` unless packet captures prove the game needs peer-origin packets.

`udp_reply_policy` controls which source addresses are accepted as replies from the game/server side:

- `strict`: accept only the exact configured target IP:port.
- `same_ip`: accept the configured target IP on any source port.
- `any`: accept replies from any source; this is the legacy default when the field is omitted.

Recommended default for known direct/LAN UDP game profiles is `same_ip`. Use `strict` only when the game always replies from the exact target port. Use `any` only for compatibility experiments or unknown games.

## Multi-Port UDP Targets

Most plugins use one legacy `target`. A game server with multiple fixed UDP
listeners can also declare `targets[]`; the legacy target remains available as
a fallback for older ROAD clients:

```json
{
  "target": { "id": "game", "host": "127.0.0.1", "port": 8766 },
  "targets": [
    { "id": "game", "host": "127.0.0.1", "port": 8766 },
    { "id": "blob-sync", "host": "127.0.0.1", "port": 9700 }
  ]
}
```

The matching client config uses `udp_listeners[]`. Each listener creates its
own WebSocket session and selects the destination during setup with
`target=<id>`. ROAD does not rewrite the game UDP payload or multiplex all
ports into a custom packet format. This is the required shape for the Sons Of
The Forest dedicated-server profile.

## Plugin Studio Advanced Capture

Socket snapshots remain the default cross-platform discovery mechanism.
On Windows, Plugin Studio can additionally use the built-in `pktmon` tool to
collect bounded packet metadata during a capture. The resulting report can
include real UDP/TCP payload size distribution, packet timing, packets per
second, port activity, and conservative RakNet/SLikeNet offline-magic signals.

```powershell
plugin-studio.exe --process "Game.exe" --advanced-capture auto
plugin-studio.exe --process "Game.exe" --advanced-capture off
plugin-studio.exe --process "Game.exe" --advanced-capture required
```

`auto` is the default. It gracefully falls back to socket snapshots when the
process is not elevated or `pktmon` is unavailable. `required` stops instead
of falling back. Raw PktMon files are kept in a temporary directory only and
deleted after their aggregate metadata has been parsed. Linux packet capture
support remains a later roadmap item.

## UDP Datagram Limits

ROAD now treats configured `buffer_size` as the maximum UDP datagram payload it will forward for that side of the tunnel.

Behavior:

- If a UDP datagram is larger than `buffer_size`, ROAD reads with a one-byte guard and drops the datagram instead of forwarding a silently truncated payload.
- The drop is logged once per affected path as `reason=truncation_guard`.
- UDP datagrams above 1200 bytes are still forwarded if they fit `buffer_size`, but ROAD logs a one-time conservative WAN/MTU warning.
- The 1200-byte threshold is a warning threshold, not a hard limit.

Why:

- Go `PacketConn.ReadFrom` cannot otherwise tell ROAD the original datagram length after a too-small read buffer.
- Forwarding a truncated game packet is worse than dropping it because it can corrupt protocol state.
- 1200 bytes is a conservative cross-internet payload target and lines up with QUIC-style deployment assumptions, but some LAN/local games can still send larger payloads successfully.

## Base Config + Profile Override

Current state:

- Each profile can still have explicit `configs/server-*.json` and `configs/client-*.json`.
- This is simple, but it duplicates common HTTP/control/WebSocket settings.

Implemented model:

1. Keep canonical base files:

```text
configs/base/server.json
configs/base/client.json
```

2. Keep game/profile overrides as small files:

```text
configs/profiles/gzdoom.json
configs/profiles/lethal-company.json
configs/profiles/minecraft-java.json
configs/profiles/minecraft-bedrock.json
```

3. Merge order:

```text
base config -> profile override -> local/user override
```

4. Merge rules:

- Objects merge recursively.
- Scalars replace.
- Arrays replace, not append. This avoids accidental duplicate enabled plugins or headers.
- Empty string means explicit empty value only for fields where that is valid.
- Unknown keys should fail validation once strict mode exists.

5. Generated output is still plain server/client JSON:

```text
configs/.generated/server-gzdoom.json
configs/.generated/client-gzdoom.json
configs/.generated/client-gzdoom-p1.json
configs/.generated/client-gzdoom-p2.json
```

Reason: runtime binaries stay simple and do not need to understand profile merge logic on the hot path.

For same-machine multi-client testing, `--client-instances N` writes multiple
client config files with incrementing listen ports. See
`docs/MULTI-CLIENT-INSTANCES.md`.

## WebSocket Auth Fields

`auth_token`, `auth_tokens`, and `auth_header` protect the WebSocket data plane. When at least one token is configured, the same token check is also applied to control API data endpoints. The `/dashboard` HTML shell can load so a browser user can enter the token, but its `/api/*` data requests remain protected.

Current behavior:

- Server config accepts `http.auth_token` and `http.auth_tokens`.
- Client config accepts `auth_token`.
- `env:NAME` values are kept in config files and resolved at runtime; missing env values fail runtime startup when auth depends on them.
- If `auth_header` is omitted while a token is configured, ROAD uses `X-ROAD-Token`.
- If `auth_header` is `Authorization`, clients send `Bearer <token>` and the server accepts both bearer and raw token values.
- If no token is configured, auth stays disabled for local/dev compatibility.
- Public deployments can also set `http.allowed_hosts`, `http.allowed_origins`, `http.max_connections`, `http.max_connections_per_ip`, and `http.rate_limit_per_minute`.
- `control.plugin_api_public=false` keeps plugin info/download endpoints private unless auth is enabled.

Example:

```json
{
  "http": {
    "auth_token": "env:ROAD_WS_TOKEN",
    "auth_header": "X-ROAD-Token"
  }
}
```

Client:

```json
{
  "auth_token": "env:ROAD_WS_TOKEN",
  "auth_header": "X-ROAD-Token"
}
```

## Client Timeout Fields

Client config has:

- `read_timeout`
- `write_timeout`
- `ws_idle_timeout`
- `ws_ping_interval`

Current runtime behavior:

- `read_timeout` and `write_timeout` are parsed and validated.
- WebSocket read/write deadlines are currently driven by `ws_idle_timeout`.
- `ws_ping_interval` keeps the tunnel alive and refreshes the read deadline through pong handling.

Decision:

- Keep `read_timeout` and `write_timeout` validated for config stability.
- Do not tune them expecting runtime behavior until a later transport cleanup explicitly wires them into read/write deadlines.
- Prefer tuning `ws_idle_timeout` and `ws_ping_interval` today.

## `server_ws_url` Validation And Repair

Client config now validates `server_ws_url` during load:

- It must not be empty.
- It must parse as a URL.
- Scheme must be `ws` or `wss`.
- Host must be present.

The interactive `road-proxy` client flow can repair an invalid `server_ws_url` by reading the config in a temporary lenient mode, asking for a new ROAD server address, normalizing it, and then writing a generated config.
