# DDNet UDP Acceptance

DDNet is the preferred free UDP game baseline for ROAD validation.

## Current Status

```text
status: experimental
tested_players: 0
last_verified: 2026-05-13
profile: plugins/ddnet-udp/plugin.json
3_player_result: pending
```

## Scope

ROAD carries DDNet direct UDP traffic when the DDNet server is reachable at:

```text
127.0.0.1:8303
```

The recommended ROAD client listen address is:

```text
127.0.0.1:18303
```

Use direct connect. Do not use Steam lobby/server-browser behavior as ROAD
evidence.

## ROAD Profile

```text
plugin: ddnet-udp
network: udp
target: 127.0.0.1:8303
client listen: 127.0.0.1:18303
udp_peer_broadcast: false
udp_reply_policy: same_ip
```

## Baseline Before Game Test

Start a synthetic UDP target where the ROAD server can reach it:

```powershell
.\road-proxy.exe udp-check server --listen 127.0.0.1:8303
```

Run ROAD server/client with the DDNet profile, then test through the ROAD client:

```powershell
.\road-proxy.exe udp-check client --target 127.0.0.1:18303 --players 3 --tickrate 30 --duration 5m --payload 256
```

Expected baseline:

```text
missing=0
dup=0
reorder=0
no multi-second max_gap during stable network
```

## Three-Player Game Criteria

Run with three separate players/machines if possible:

- Player 1 hosts DDNet server on the ROAD server side.
- Player 2 joins through ROAD.
- Player 3 joins through ROAD.
- All players can move, hook, collide, and see each other consistently.
- No repeated timeout/disconnect.
- ROAD client logs should not show sustained multi-second gaps.
- `/api/stats` should show active UDP sessions and packet counters.

## Evidence To Save

- Server/client config file names.
- DDNet server port and map.
- ROAD `udp-check` summary.
- ROAD client/server console logs.
- `/api/stats` snapshot during the run.
- If enabled, `logs/udp-record-client.jsonl` and `logs/udp-record-server.jsonl`.

