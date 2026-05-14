# UDP Recorder

ROAD can write short UDP packet metadata captures as JSON Lines. This is a
debugging tool for timing, direction, packet size, session identity, and
RakNet-style sequence inspection.

It does not save UDP payload bytes.

## Server Config

```json
{
  "udp_record": {
    "enabled": true,
    "path": "logs/udp-record-server.jsonl",
    "duration": "30s",
    "max_events": 5000
  }
}
```

Server directions:

- `websocket_to_target`: packet arrived from ROAD client and was written to the game/server target.
- `target_to_websocket`: packet arrived from the game/server target and was written back to ROAD client.
- `peer_broadcast`: UDP peer broadcast copied a client-origin packet to another ROAD peer session.

## Client Config

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

Client directions:

- `local_to_websocket`: local game client packet was written into the ROAD WebSocket tunnel.
- `websocket_to_local`: ROAD WebSocket packet was written back to the local game client address.

## JSONL Shape

The first line is capture metadata:

```json
{"type":"udp_record_start","time":"2026-05-12T10:00:00Z","role":"client","duration_seconds":30,"max_events":5000}
```

Packet lines contain metadata only:

```json
{"type":"udp_packet","time":"2026-05-12T10:00:01Z","role":"client","direction":"local_to_websocket","plugin":"gzdoom-udp","session_id":"127.0.0.1:5031","source":"127.0.0.1:5031","destination":"websocket","size_bytes":64}
```

If a packet looks like RakNet data, `raknet_sequence` is included.

## Rules

- Keep captures short. Default is `30s` and `5000` packet events.
- Use this for metadata analysis, not as a packet replay source.
- Since payload is not stored, exact game traffic cannot be reconstructed from this file.
- If a future analyzer needs payload, it must be a separate explicit opt-in mode.
