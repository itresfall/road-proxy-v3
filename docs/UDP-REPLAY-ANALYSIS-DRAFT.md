# UDP Replay / Analysis Draft

This is a design note for a future UDP analysis tool. The current UDP recorder
writes metadata only, so it can support timing analysis but not byte-perfect
traffic replay.

## Current Input

`docs/UDP-RECORDER.md` defines JSONL packet metadata:

- timestamp
- role: `client` or `server`
- direction
- plugin
- session id
- source and destination labels
- packet size
- optional RakNet sequence

Payload bytes are intentionally absent.

## Useful Analyses

- Direction timeline: show client->server and server->client packet timing.
- Packet rate: packets per second per direction and per session.
- Burst detection: identify clustered packets and long idle gaps.
- Size distribution: min, p50, p95, max packet sizes.
- RakNet sequence gaps: loss, reorder, duplicate candidates when sequence is present.
- Session split: compare each local client/session independently.
- Peer broadcast check: detect whether peer broadcast is active and how often it duplicates traffic.

## CLI Shape

Possible future command:

```powershell
road-proxy udp-analyze --input logs/udp-record-client.jsonl
road-proxy udp-analyze --input logs/udp-record-server.jsonl --format markdown
```

Possible output:

```text
UDP analysis
role=client plugin=gzdoom-udp duration=30s events=1280
local_to_websocket: pps=21.3 size_p95=96B jitter=4.2ms max_gap=180ms
websocket_to_local: pps=20.8 size_p95=112B jitter=5.1ms max_gap=210ms
raknet: loss_candidates=2 reorder_candidates=1 duplicate_candidates=0
```

## Replay Boundary

Metadata-only replay can simulate timing and size, but it cannot reproduce real
game behavior because packet payloads are missing.

Possible future modes:

- `udp-analyze`: safe metadata-only analysis. Good default.
- `udp-simulate`: generate dummy UDP packets using recorded timing and sizes.
- `udp-replay`: only if a separate explicit payload capture mode exists.

`udp-replay` must not be presented as available while the recorder stores only
metadata.

## Acceptance Criteria

- Analyzer reads client and server JSONL files.
- Analyzer rejects files with payload-only assumptions clearly.
- Analyzer handles missing `raknet_sequence`.
- Analyzer produces text and JSON output.
- Analyzer can compare two captures from the same test run.
- Tests cover malformed lines, empty files, max event captures, and mixed client/server roles.
