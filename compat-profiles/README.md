# Compatibility Profiles

`compat-profiles/` is ROAD's community compatibility database. Plugin Studio uses
these JSON files to recognize known games, explain ROAD scope, and generate safer
plugin drafts.

A compatibility profile is not the runtime tunnel itself. Runtime forwarding
lives in `plugins/<name>/plugin.json`; compatibility detection lives here.

## When To Add A Profile

Add a profile when you have evidence that a game or mod can expose direct,
LAN, or local IP:port traffic that ROAD can carry.

Do not add profiles for games that only work through Steam, EOS, Epic, or other
closed lobby/relay transports unless they also have a direct/LAN/local endpoint.
ROAD is not a Steam/EOS relay replacement.

## Minimum Evidence

A useful profile PR should include:

- Game or mod name and version.
- ROAD version or commit.
- Operating system used for host and client.
- Network path tested: direct LAN, ROAD LAN, or ROAD over WSS/domain.
- Player count tested.
- Target protocol and port.
- Required launch arguments, if any.
- Result: works, partially works, or fails with notes.

If the evidence is weak, open an issue first instead of adding a permanent
profile.

## Profile Rules

- Use one file per game family: `compat-profiles/<id>.json`.
- Keep `id` equal to the filename without `.json`.
- Use lowercase kebab-case IDs, for example `example-game`.
- `plugin_name` must point to an existing folder under `plugins/`.
- `road_scope` must stay `direct_lan_local_port_only`.
- `steam_lobby_supported` should stay `false` unless direct evidence proves
  ROAD handles that exact transport. A Steam AppID match is only an identity
  signal.
- `udp_peer_broadcast` should stay `false` unless packet evidence proves the
  game requires client-to-client fanout.
- Add `note_keys` and `launch_advice_keys` to `locales/en.json` and
  `locales/tr.json`.

## Confidence Labels

Plugin Studio computes match confidence from signals such as process name, port,
protocol, Steam AppID, and executable hash:

| Confidence | Meaning |
| --- | --- |
| `bronze` | Weak hint, usually name or protocol only. |
| `silver` | Some matching signals, still needs manual checking. |
| `gold` | Strong match from process/port/protocol evidence. |
| `platinum` | Very strong identity signal, usually executable hash or multiple independent signals. |

Confidence is detection confidence, not a guarantee that the game netcode works
well through any proxy.

## Validate Before PR

```powershell
go test ./internal/repoqa -run TestRepositoryCompatProfilesLoad -count=1
go test ./cmd/plugin-studio -count=1
go run ./cmd/road validate --all-configs
```

See `docs/COMPATIBILITY-CONTRIBUTING.md` for the full contribution checklist.
