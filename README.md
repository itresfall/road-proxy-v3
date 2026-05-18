# ROAD Proxy v3

ROAD Proxy v3 is a WebSocket-backed TCP/UDP tunnel for LAN/direct multiplayer games and local services.

It is meant for cases where a game or service can connect to a direct `IP:port`, but players are not on the same network. ROAD runs a small local client near the player, a server near the real game host, and carries the traffic over WebSocket or WSS.

## Start Here

For most users, use the interactive menu binary first.

On Windows:

1. Download `road-proxy-v3_0.1.0_windows_amd64.zip` from the latest release.
2. Extract the zip.
3. Run `road-proxy.exe`.
4. Use the menu:

```text
1) Engine/Server
2) Client
3) Public Server Wizard
4) Settings
5) Exit
```

The all-in-one `road-proxy.exe` is the recommended entry point. It includes server mode, client mode, public Cloudflare setup, language/settings, config generation, diagnostics, and validation helpers. The separate `road-server.exe` and `road-client.exe` binaries are for scripted or advanced use.

## Public Server Wizard

If you want a friend to connect over the internet without manual port forwarding:

1. Open `road-proxy.exe`.
2. Choose `Public Server Wizard`.
3. Pick a plugin, for example Minecraft, DDNet, GZDoom, Sven Co-op, or Lethal Company direct UDP.
4. Choose TryCloudflare for a quick temporary URL, or domain mode for a named Cloudflare tunnel.
5. Share the printed `wss://.../ws` ROAD address with the client player.
6. If the client asks for it, share the printed Auth token too.

Auth-free ROAD servers do not trigger a token prompt. Public Wizard servers always generate a token because Cloudflare endpoints are internet reachable.

## Client Player Flow

On the joining PC:

1. Open `road-proxy.exe`.
2. Choose `Client`.
3. Paste the ROAD address, for example `wss://example.trycloudflare.com/ws`.
4. If asked, paste the Auth token.
5. The menu prints the local game address, usually something like `127.0.0.1:25568`.
6. Put that local address into the game or mod.

ROAD prints beginner-friendly connection hints first. Technical logs appear below them.

## Linux Note

Linux builds are command-line binaries. Many desktop environments will not run them correctly by double-click.

Simple way:

1. Extract `road-proxy-v3_0.1.0_linux_amd64.zip` or the matching ARM64 zip.
2. Open a terminal.
3. Drag the built `road-proxy` file into the terminal so the full path appears.
4. Press Enter.

Equivalent terminal commands:

```bash
chmod +x ./road-proxy
./road-proxy
```

Use `linux_amd64` for normal Intel/AMD Linux PCs. Use `linux_arm64` for ARM64 systems.

## Downloads

| File | Use |
| --- | --- |
| `road-proxy-v3_0.1.0_windows_amd64.zip` | Normal Windows PCs. Start with `road-proxy.exe`. |
| `road-proxy-v3_0.1.0_windows_arm64.zip` | Windows on ARM64. Experimental until more hardware testing exists. |
| `road-proxy-v3_0.1.0_linux_amd64.zip` | Normal Linux PCs/servers. Run from terminal. |
| `road-proxy-v3_0.1.0_linux_arm64.zip` | ARM64 Linux servers/devices. Run from terminal. |
| `*.sha256` | Checksum files for verifying downloads. |

## What It Does

- Tunnels TCP or UDP traffic over a WebSocket data plane.
- Uses JSON plugin profiles instead of native `.dll` or `.so` plugins.
- Supports WSS through Cloudflare Tunnel.
- Includes UDP metrics, MTU warnings, diagnostics, and synthetic UDP test tools.
- Ships with an interactive menu and a browser dashboard/control API.
- Includes Plugin Studio for Windows/Linux process scanning and draft profile generation.

## What It Is Not

- Not a full VPN or virtual LAN adapter.
- Not a LAN broadcast/multicast emulator.
- Not a Steam, EOS, or official lobby/relay replacement.
- Not a protocol-aware payload rewriter unless a future adapter adds that explicitly.

## Built-In Runtime Plugins

| Plugin | Network | Typical use |
| --- | --- | --- |
| `minecraft` | TCP | Minecraft Java/direct TCP services. |
| `minecraft-bedrock-udp` | UDP | Minecraft Bedrock/RakNet UDP. |
| `ddnet-udp` | UDP | Recommended free UDP baseline test game. |
| `gzdoom-udp` | UDP | GZDoom direct UDP. Use `-netmode 1` for 3+ players. |
| `sven-coop-udp` | UDP | Sven Co-op/GoldSrc direct server traffic. |
| `lethal-company-udp` | UDP | Experimental direct/LAN/local UDP profile, not Steam relay traffic. |

A plugin target is the real service on the server side. A client listen address is what the local player connects to.

## Documentation

Full documentation lives in [`docs/`](./docs/). Key starting points:

- [Full user guide](./docs/USER-GUIDE.md) - the old detailed README, kept for deeper reference.
- [Compatibility profiles](./compat-profiles/README.md) - add or document a game profile.
- [Compatibility contribution rules](./docs/COMPATIBILITY-CONTRIBUTING.md) - evidence expected for game profiles.
- [Public deployment security](./docs/PUBLIC-DEPLOYMENT-SECURITY.md) - public server expectations and auth guidance.
- [Cloudflare integration](./docs/CLOUDFLARE-INTEGRATION.md) - TryCloudflare, named tunnels, and limits.
- [Linux deployment](./docs/LINUX-DEPLOYMENT.md) - Linux server deployment notes.
- [UDP diagnostics](./docs/UDP-MULTIPLAYER-DIAGNOSIS.md) - how to reason about UDP game issues.
- [UDP check tool](./docs/UDP-CHECK.md) - synthetic UDP test flow without depending on a real game.
- [Plugin Studio](./docs/CONFIG-PLUGIN-SYSTEM.md) - config and plugin profile model.
- [Release checklist](./docs/RELEASE-CHECKLIST.md) - release preflight steps.

## Build From Source

Requirements:

- Go 1.23 or newer.
- Windows or Linux for maintained builds.
- macOS is not an official target; tested external PRs are welcome.

Windows build:

```powershell
./scripts/build-windows.ps1
```

Linux build from PowerShell:

```powershell
./scripts/build-linux.ps1 -Arch amd64
./scripts/build-linux.ps1 -Arch arm64
```

Cross-platform release packages:

```powershell
$env:ROAD_VERSION = "0.1.0"
./scripts/build-cross.ps1 -Package -IncludeWindowsArm64 -IncludeLinuxArm64
```

Run tests and validation:

```powershell
go test ./...
go vet ./...
go run ./cmd/road validate --all-configs
```

## Repository Map

```text
cmd/road             Interactive all-in-one menu binary
cmd/server           Standalone ROAD server binary
cmd/client           Standalone ROAD client binary
cmd/plugin-studio    Windows/Linux process scanner and plugin generator
internal/engine      TCP/UDP/WebSocket forwarding engine
configs/             Runtime config templates
plugins/             Built-in runtime plugin profiles
compat-profiles/     Community game compatibility profiles
scripts/             Build helpers and release plugin manifest
deploy/              Deployment helpers and service templates
docs/                User guide, diagnostics, security, roadmap, acceptance notes
```

## Security And Contributions

Security reporting and public deployment expectations are documented in [`SECURITY.md`](./SECURITY.md) and [`docs/PUBLIC-DEPLOYMENT-SECURITY.md`](./docs/PUBLIC-DEPLOYMENT-SECURITY.md).

Contribution rules, supported scope, macOS policy, and PR expectations are in [`CONTRIBUTING.md`](./CONTRIBUTING.md).

## License

ROAD Proxy v3 is released under the GNU Affero General Public License v3.0 only (`AGPL-3.0-only`). See [`LICENSE`](./LICENSE).

You may use, study, modify, and share the code. If you distribute a modified version or run a modified network service for users, you must provide the corresponding source under the same license.
