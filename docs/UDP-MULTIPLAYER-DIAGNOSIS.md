# UDP Multiplayer Diagnosis

## Problem Pattern

Observed with a GZDoom/FNAF Doom Reborn style session:

- Host player can see both remote players.
- Remote player 2 and remote player 3 can see the host.
- Remote player 2 and remote player 3 do not reliably see each other.
- The game reports the other remote player as out of sync.

This does not match a simple UDP packet-loss problem. It matches a topology problem: the original UDP proxy path only guaranteed client-to-host and host-to-client traffic.

## Existing UDP Coverage

The old UDP tests covered authoritative server behavior:

- Multiple clients send packets to one target server.
- The target server broadcasts snapshots back to every active client.
- This model works and is covered by `TestUDPWebSocketSyncFiveClientsStress`.

That test is useful for Minecraft Bedrock-like server behavior, but it does not prove peer/lockstep games.

## Added Mode

`runtime.udp_peer_broadcast` adds an optional peer relay mode.

When enabled for a UDP plugin:

1. A client-origin UDP packet is still sent to the configured target address.
2. The same packet is also relayed to other active WebSocket sessions using the same plugin.
3. The sender does not receive its own packet back.

This is intended for peer/lockstep UDP games where clients need to receive each other's input/state packets and the game has no usable packet-server/client-server mode.

## Current Profile

The final GZDoom fix was not peer broadcast. GZDoom's default network mode switches 3-player sessions into peer-to-peer, which is not a stable fit for a plain port proxy. The stable setup is:

- Add `-netmode 1` to the GZDoom host and all clients.
- Keep `runtime.udp_peer_broadcast` disabled.

The `plugins/gzdoom-udp/plugin.json` profile uses:

```json
"udp_peer_broadcast": false
```

Keep this disabled for normal authoritative UDP games unless testing proves they need it.

## Verification

Relevant tests:

```powershell
go test ./internal/e2e -run 'TestUDPWebSocket(PeerBroadcastThreeClients|SyncFiveClientsStress|TunnelRoundTripThreeClients|AcceptsAlternateSourcePortReply)' -count=1 -v
go test ./internal/engine -run TestUDPPeerBroadcastSlowPeerBlocksCurrentBroadcast -count=1 -v
```

Meaning:

- `TunnelRoundTripThreeClients`: basic multi-client UDP roundtrip.
- `SyncFiveClientsStress`: authoritative server snapshot model.
- `AcceptsAlternateSourcePortReply`: replies from alternate UDP source ports.
- `PeerBroadcastThreeClients`: peer-origin packets are visible to other clients.
- `SlowPeerBlocksCurrentBroadcast`: current peer broadcast is synchronous; a slow peer can delay the broadcast call.

## Remaining Caveat

This mode does not emulate a full LAN broadcast domain and does not rewrite game-specific peer addresses inside payloads. If a game embeds IP/port values in its protocol and requires direct peer sockets, a deeper protocol-aware adapter may still be needed.

Peer broadcast is also currently synchronous. See:

```text
docs/UDP-PEER-BROADCAST-SLOW-PEER.md
```
