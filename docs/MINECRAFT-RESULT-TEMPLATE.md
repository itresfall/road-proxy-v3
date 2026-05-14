# Minecraft Test Result Template

Use this template after each acceptance run.

## Metadata

- Date:
- Operator:
- Branch/Commit:
- Environment (LAN/WAN/Cloudflare route):
- Minecraft Version:

## Config Snapshot

- Server config path:
- Client config path:
- Active plugin:
- Plugin target address:
- WS endpoint:
- Auth enabled (yes/no):

## Preflight / Config Check

- `go test ./...` result:
- Server/client config check result:
- Warnings:

## Session Metrics

- Session length (minutes):
- Join time (seconds):
- Median ping (ms):
- P95 ping (ms):
- Disconnect count:
- Forced reconnect count:

## API Snapshots

- Start `errors`:
- Mid `errors`:
- End `errors`:
- End `active_connections`:

## Outcome

- Acceptance verdict: PASS / FAIL
- Main failure reason (if FAIL):
- Notes for next iteration:
