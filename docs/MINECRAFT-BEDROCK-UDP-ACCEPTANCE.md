# Minecraft Bedrock UDP Acceptance Checklist

This checklist defines the minimum gate for saying "Minecraft Bedrock over UDP->WebSocket is playable" in `proje-v3`.

## 1. Preflight

Run repository tests first:

```powershell
go test ./...
```

Check the active configs manually:

- Server config path: `configs/server-udp.example.json`
- Client config path: `configs/client-udp.example.json`
- Server plugin target should point at the real Bedrock UDP server.
- Client `listen_network` should be `udp`.
- Client `plugin_name` should match an enabled server plugin.

## 2. Start Order

1. Start Bedrock server on plugin target address (default `192.168.1.101:19132`).
2. Start proxy server:
   ```bash
   go run ./cmd/server -config configs/server-udp.example.json
   ```
3. Start proxy client:
   ```bash
   go run ./cmd/client -config configs/client-udp.example.json
   ```
4. In Bedrock client, add server `127.0.0.1` port `19133`.

## 3. In-Game Acceptance Session

Run one uninterrupted session of at least 15 minutes:

1. Join world from clean game start.
2. Sprint/fly through loaded and new chunks.
3. Break/place blocks and open inventories.
4. Change dimension once (Nether/End) if available.
5. Disconnect and reconnect once.

## 4. Pass Criteria

All items below must pass:

1. Join time <= 12 seconds.
2. No disconnect loops and no forced reconnect during normal play.
3. Stable gameplay with no packet loss spikes visible to player.
4. `GET /api/health` stays `status=ok`.
5. `GET /api/stats` `errors` does not grow continuously during session.
6. `active_connections` returns to `0` after full exit.

## 5. Evidence To Save

1. Config paths and edited values.
2. `GET /api/stats` snapshot at start, middle, and end.
3. `udp metrics` log excerpts (periodic and close) from client output.
4. Notes for any lag spikes, rubber-banding, or disconnects with timestamps.

Use `docs/MINECRAFT-BEDROCK-UDP-RESULT-TEMPLATE.md` for consistent reporting.

## 6. Fast Triage If Failed

1. No Bedrock server in list: verify local server `127.0.0.1:19133` and client UDP mode.
2. Join timeout: verify plugin `target.address` points to replying NIC/IP on Bedrock host.
3. Random disconnects: verify `ws_ping_interval`/`ws_idle_timeout` and LAN packet loss.
