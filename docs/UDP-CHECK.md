# ROAD UDP Check

`udp-check` is a synthetic UDP game-flow tester built into `road-proxy`.
It is used to test ROAD without depending on a real game's netcode.

The test packet contains:

- player id
- sequence number
- client send timestamp
- server receive timestamp in ACKs
- tickrate metadata
- deterministic filler payload

The result reports:

- missing ACKs
- duplicate ACKs
- reordered ACKs
- RTT min/avg/p95/max
- RTT jitter
- max receive gap
- large datagram counters

## Direct Baseline

Run the server on the target machine:

```powershell
.\road-proxy.exe udp-check server --listen 0.0.0.0:7777
```

Run the client against that machine directly:

```powershell
.\road-proxy.exe udp-check client --target SERVER_IP:7777 --players 3 --tickrate 30 --duration 2m --payload 256
```

This checks the network path without ROAD.

## ROAD Path

Run the UDP check server where the ROAD server can reach it:

```powershell
.\road-proxy.exe udp-check server --listen 127.0.0.1:7777
```

Use a UDP plugin whose target is:

```text
127.0.0.1:7777
```

Run the normal ROAD server and client, then run the UDP check client against the
ROAD client listen address:

```powershell
.\road-proxy.exe udp-check client --target 127.0.0.1:25568 --players 3 --tickrate 30 --duration 10m --payload 256
```

Each simulated player uses a separate local UDP socket. ROAD should therefore
create separate UDP sessions/WebSocket flows, similar to multiple local game
clients.

## Payload Size Sweep

Use different payload sizes to expose tunnel/HOL risk:

```powershell
.\road-proxy.exe udp-check client --target 127.0.0.1:25568 --players 3 --tickrate 30 --duration 2m --payload 256
.\road-proxy.exe udp-check client --target 127.0.0.1:25568 --players 3 --tickrate 30 --duration 2m --payload 1200
.\road-proxy.exe udp-check client --target 127.0.0.1:25568 --players 3 --tickrate 30 --duration 2m --payload 1401
.\road-proxy.exe udp-check client --target 127.0.0.1:25568 --players 3 --tickrate 30 --duration 2m --payload 1473
```

Threshold meaning:

- `>1200`: conservative WAN MTU risk.
- `>1400`: tunnel/WebSocket head-of-line risk becomes more likely.
- `>1472`: IPv4 UDP fragmentation risk for normal 1500 MTU paths.

## Interpreting Output

Example client summary:

```text
player=1 sent=18000 ack=18000 missing=0 loss=0.00% dup=0 reorder=0 rtt_min=3ms rtt_avg=8ms rtt_p95=18ms rtt_max=55ms rtt_jitter=2ms max_gap=90ms size=max=256,>1200=0,>1400=0,>1472=0 errors=0
```

Important signals:

- `missing > 0`: packet/ACK loss or tunnel/session drop.
- `reorder > 0`: ACK order changed.
- `dup > 0`: duplicate ACK observed.
- high `rtt_p95` or `rtt_max`: visible latency spike.
- high `max_gap`: traffic stopped for that long.
- `>1400` or `>1472`: large datagrams may amplify WebSocket/TCP stalls.

## What This Does Not Prove

`udp-check` proves ROAD's packet transport behavior for synthetic packets. It
does not prove that a real game's item ownership, prediction, rollback, or LAN
mode is correct.

Use it together with:

- direct LAN baseline
- ROAD LAN baseline
- ROAD WSS/domain test
- game-specific acceptance test

