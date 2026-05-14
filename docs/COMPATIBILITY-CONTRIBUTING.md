# Compatibility Contribution Guide

ROAD compatibility data should stay useful, conservative, and evidence-based.
The goal is to build a ProtonDB-like knowledge layer for direct/LAN/local
IP:port games without pretending ROAD can carry every online transport.

## Two Different Things

`plugins/<name>/plugin.json` is runtime configuration. It tells ROAD where to
send TCP/UDP traffic.

`compat-profiles/<id>.json` is detection and documentation metadata. Plugin
Studio uses it to recognize a process and create safer plugin/config drafts.

A new game may need both. If a suitable plugin already exists, add only the
compat profile. If no plugin exists, add the plugin and the compat profile in
the same PR when possible.

## Good Profile Candidates

Good candidates expose one of these:

- Direct IP connect.
- LAN server browser plus known server port.
- Localhost server endpoint.
- Dedicated server binary with documented TCP/UDP ports.
- Mod or launch argument that forces client/server mode.

Poor candidates:

- Steam lobby only.
- EOS/relay only.
- Matchmaking only with no direct endpoint.
- Games that embed private IP/port in payloads unless a protocol adapter exists.
- Games that desync on direct LAN before ROAD is involved.

## Required PR Evidence

Include this evidence in the PR body:

```text
Game/mod:
Game version:
ROAD version or commit:
Host OS:
Client OS:
Transport tested: direct LAN / ROAD LAN / ROAD WSS-domain
Players tested:
Server target IP:port:
Client listen IP:port:
Launch arguments:
Result:
Known limitations:
Logs or screenshots:
```

Keep logs anonymous. Replace private domains, public IPs, usernames, and LAN
addresses unless they are necessary for debugging.

## Field Guidance

- `schema_version`: always `compat_profile.v1`.
- `id`: stable lowercase kebab-case identifier.
- `display_name`: human-readable game/mod name.
- `road_scope`: always `direct_lan_local_port_only`.
- `steam_lobby_supported`: normally `false`.
- `requires_game_launch_args`: `true` when flags such as server mode, LAN mode,
  or netmode are required.
- `match`: lowercase text tokens Plugin Studio can match against process names.
- `exe_names`: known executable names.
- `steam_app_ids`: optional identity hints only.
- `exe_hashes`: optional SHA256 identity hints; avoid adding hashes for rapidly
  changing modded executables unless useful.
- `plugin_name`: existing runtime plugin folder name.
- `network`: `tcp` or `udp`.
- `target_host`: usually `127.0.0.1` for local-hosted game servers.
- `target_port`: real game server port on the ROAD server side.
- `client_listen_port`: local port players connect to on the ROAD client side.
- `udp_peer_broadcast`: default `false`; require proof before enabling.
- `udp_reply_policy`: prefer `same_ip` for UDP unless a game needs a looser mode.
- `known_ports`: documented ports and their roles.
- `note_keys`: user-facing warning/explanation keys from locale files.
- `launch_advice_keys`: locale keys for host/client launch hints.

## Acceptance Levels

Use these labels in docs/issues when discussing validation quality:

| Level | Evidence |
| --- | --- |
| Experimental | Profile exists but has limited real-game validation. |
| Community validation | Works for at least one external user or small group, with logs. |
| Stable | Repeated tests across OS/network paths and at least 2-3 players when relevant. |
| Regression guarded | Has synthetic or game-specific test coverage beyond manual testing. |

Do not overstate compatibility. A profile can be useful even when the game has
known engine-side LAN bugs.

## Local Validation

Run these before opening a PR:

```powershell
go test ./internal/repoqa -run TestRepositoryCompatProfilesLoad -count=1
go test ./cmd/plugin-studio -count=1
go test ./...
go run ./cmd/road validate --all-configs
```

The repository QA test checks that each compatibility profile:

- Parses as JSON.
- Uses a matching filename and profile ID.
- Points to an existing plugin.
- Matches the plugin network type.
- Has valid ports and UDP policy.
- References locale keys that exist in English and Turkish.

## Security And Privacy

Never include tokens, private keys, personal usernames, private domains, or real
server credentials in profile PRs. Prefer placeholders such as `road.example.com`
and `127.0.0.1`.
