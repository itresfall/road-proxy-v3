# Minecraft Bedrock UDP Test Result Template

Use this template after each Bedrock UDP acceptance run.

## Metadata

- Date:
- Operator:
- Branch/Commit:
- Environment (LAN/WAN):
- Bedrock Version:

## Config Snapshot

- Server config path:
- Client config path:
- Active plugin:
- Plugin target address:
- Client listen address:
- WS endpoint:
- Auth enabled (yes/no):

## Preflight / Config Check

- `go test ./...` result:
- Server/client UDP config check result:
- Warnings:

## Session Metrics

- Session length (minutes):
- Join time (seconds):
- Disconnect count:
- Forced reconnect count:
- Rubber-band incidents:
- Client `udp metrics` tx/rx packet totals:
- Client `udp metrics` tx/rx jitter (ms):
- Client `udp metrics` tx/rx loss estimate:

## API Snapshots

- Start `errors`:
- Mid `errors`:
- End `errors`:
- End `active_connections`:

## Outcome

- Acceptance verdict: PASS / FAIL
- Main failure reason (if FAIL):
- Notes for next iteration:
