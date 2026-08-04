# Sons Of The Forest UDP Acceptance

Sons Of The Forest is the first ROAD profile validated with a real multi-port
UDP dedicated server flow.

## Current Status

```text
status: working
tested_players: 3
last_verified: 2026-06-29
profile: plugins/son-of-the-forest-udp/plugin.json
result: long-play field test passed
```

## Scope

ROAD carries direct/LAN/local dedicated server UDP traffic for the observed
SOTF dedicated server ports:

```text
game:      127.0.0.1:8766
blob-sync: 127.0.0.1:9700
query:     127.0.0.1:27016
```

ROAD does not replace Steam lobby, Steam relay, or Steam authentication traffic.
Observed dynamic Steam-side ports and per-client source ports are not fixed
plugin targets.

## ROAD Profile

```text
plugin: son-of-the-forest-udp
network: udp
target mode: named multi-port targets
udp_peer_broadcast: false
udp_reply_policy: same_ip
```

Client listeners:

```text
game:      0.0.0.0:8766
blob-sync: 0.0.0.0:9700
query:     0.0.0.0:27016
```

If the game refuses `127.0.0.1`, connect to the ROAD client machine's LAN IPv4
with the same port.

## Transport Design

SOTF does not use one large shared WebSocket tunnel for all UDP ports. ROAD
creates separate WebSocket sessions per local UDP source/target flow:

```text
UDP listener 0.0.0.0:8766  -> wss://.../ws?plugin=son-of-the-forest-udp&target=game
UDP listener 0.0.0.0:9700  -> wss://.../ws?plugin=son-of-the-forest-udp&target=blob-sync
UDP listener 0.0.0.0:27016 -> wss://.../ws?plugin=son-of-the-forest-udp&target=query
```

The UDP payload is passed through unchanged. ROAD does not add a custom packet
header with port metadata. Target selection happens at WebSocket connection
setup time via `target=<id>`.

This preserves game packet format and reduces cross-port head-of-line blocking.

## Field Test Evidence

Reported gameplay:

- 3-player SOTF session.
- Approximately 10 hours of play reported by the host.
- One good-internet client played without visible ROAD issues.
- One poor-internet client that struggled with Steam online play was playable
  through ROAD and only disconnected a few times, then rejoined quickly.

Captured good-link sample:

```text
main session: anonymized client flow
duration in sample: 2m40s
tx/rx packets: 3329 / 3409
tx/rx jitter avg: 8.73ms / 3.19ms
tx/rx jitter p95: 10.95ms / 5.01ms
mtu warnings: 0
errors: 0
```

Captured poor-link stable sample before disconnect:

```text
main session: anonymized client flow
stable duration in sample: 2h56m30s
tx/rx packets: 212724 / 211980
tx/rx bytes: 13.43MiB / 113.51MiB
tx/rx jitter avg: 9.80ms / 8.33ms
tx/rx jitter p95: 15.75ms / 33.42ms
tx/rx jitter p99: 25.54ms / 69.71ms
mtu warnings: 0
```

The poor-link disconnect showed network/DNS failure symptoms:

```text
wsarecv / wsasend errors
temporary ROAD server hostname lookup failure
```

This points to client-side internet or DNS instability rather than a ROAD UDP
forwarding failure.

## Acceptance Criteria

The SOTF profile is considered working when:

- All three local UDP listeners start.
- The game can join through the ROAD client address.
- Main gameplay traffic shows sustained tx/rx packet growth.
- MTU counters stay below fragmentation thresholds during normal play.
- No repeated ROAD auth, target selection, or WebSocket handshake failures occur.
- Disconnects, if any, correlate with external network/DNS errors rather than
  plugin target selection or packet size failures.
