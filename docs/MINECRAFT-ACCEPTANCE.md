# Minecraft Acceptance Checklist

This checklist defines the minimum gate for saying "Minecraft over TCP->WebSocket is playable" in `proje-v3`.

## 1. Preflight

Run repository tests first:

```powershell
go test ./...
```

Check the active configs manually:

- Server config path: `configs/server.json`
- Client config path: `configs/client.json`
- Server plugin target should point at the real Minecraft server.
- Client `server_ws_url` should point at the ROAD server data plane.
- Client `plugin_name` should match an enabled server plugin.

## 2. Start Order

1. Start Minecraft server on plugin target address (default `127.0.0.1:25565`).
2. Start proxy server:
   ```bash
   go run ./cmd/server -config configs/server.json
   ```
3. Start proxy client:
   ```bash
   go run ./cmd/client -config configs/client.json
   ```
4. In Minecraft client, connect to `127.0.0.1:25568` (or your `client.listen_addr`).

## 3. In-Game Acceptance Session

Run one uninterrupted session of at least 10 minutes:

1. Join world from clean client start.
2. Move quickly through loaded and newly generated chunks.
3. Break/place blocks and open inventory/chests.
4. Use one teleport or nether portal transition.
5. Disconnect and reconnect once.

## 4. Pass Criteria

All items below must pass:

1. Join time <= 10 seconds.
2. No disconnect loops and no forced reconnect during normal play.
3. Median ping <= 65 ms during stable gameplay.
4. P95 ping <= 90 ms.
5. `GET /api/health` stays `status=ok`.
6. `GET /api/stats` `errors` does not increase continuously.
7. `active_connections` returns to `0` after full exit.

## 5. Evidence To Save

1. Config paths and edited values.
2. `GET /api/stats` snapshot at start, middle, and end.
3. Short notes for any spikes or disconnects with time.

Use `docs/MINECRAFT-RESULT-TEMPLATE.md` for consistent reporting.

## 6. Fast Triage If Failed

1. `target connect failed`: verify plugin target server is running.
2. Join timeout with no errors: verify WS path and plugin name match configs.
