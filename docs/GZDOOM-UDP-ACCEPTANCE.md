# GZDoom UDP Acceptance

This document records the working rule for GZDoom/FNaF Doom Reborn style UDP
sessions through ROAD.

## Status

```text
status: working
tested_players: 3
last_verified: 2026-05-11
profile: plugins/gzdoom-udp/plugin.json
```

## Required Game Rule

Use GZDoom packet-server mode:

```text
-netmode 1
```

Do not try to fix 3-player desync with ROAD peer broadcast. The correct fix is
to prevent GZDoom from switching into the peer-to-peer behavior that breaks a
plain port proxy.

## Host Pattern

```powershell
gzdoom.exe ... -host 3 -netmode 1
```

## Client Pattern

```powershell
gzdoom.exe ... -join 127.0.0.1 -netmode 1
```

## ROAD Profile

```text
plugin: gzdoom-udp
network: udp
target: 127.0.0.1:5029
client listen: 127.0.0.1:5029 or generated per-client ports
udp_peer_broadcast: false
udp_reply_policy: same_ip
```

## Pass Criteria

- 3 players join.
- Host sees both clients.
- Each client sees host and the other client.
- No repeated "out of sync" disconnect after gameplay starts.
- `udp_peer_broadcast` stays disabled.
- ROAD server `/api/stats` does not show continuously increasing errors.

## Notes

- One player per machine is the cleanest setup.
- Same-machine multi-client tests should use separate generated client listen
  ports.
- If desync appears again, first confirm every launch command includes
  `-netmode 1`.
