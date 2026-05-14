# Lethal Company Direct UDP Acceptance

This profile is for direct/LAN/local UDP traffic only. It is not a Steam
lobby/relay replacement.

## Current Status

```text
status: experimental
tested_players: 2
last_verified: 2026-05-11
profile: plugins/lethal-company-udp/plugin.json
3_player_result: community-validation
```

Two-player direct/local UDP joining worked. Three-player validation is a
community test target, not a ROAD release blocker.

Important: Lethal Company can show object/physics desync even on direct LAN
without ROAD. Compare against direct LAN before treating a desync as a proxy
bug.

## Scope

ROAD can carry this flow when the game/mod talks to:

```text
127.0.0.1:7777
```

If the game path uses official Steam lobby/relay transport, ROAD will not see or
carry that traffic.

## ROAD Profile

```text
plugin: lethal-company-udp
network: udp
target: 127.0.0.1:7777
client listen: 127.0.0.1:7777
udp_peer_broadcast: false
udp_reply_policy: same_ip
```

## Two-Player Pass Criteria

- Client can join through ROAD.
- Gameplay starts without immediate disconnect.
- ROAD `/api/stats` shows UDP rx/tx activity.
- No continuously increasing ROAD errors.

## Three-Player Community Criteria

Run with three separate players/machines if possible:

- Player 1 hosts or provides the direct/local UDP endpoint.
- Player 2 joins through ROAD.
- Player 3 joins through ROAD.
- All players can see each other.
- No player-specific desync or drop after gameplay starts.
- Record `/api/stats` before, during, and after the run.

## Evidence To Save

- Server/client config file names.
- Whether the game was LAN/direct/local or Steam lobby.
- `/api/stats` snapshot.
- Client/server console logs if available.
- If enabled, `logs/udp-record-client.jsonl` and `logs/udp-record-server.jsonl`.
