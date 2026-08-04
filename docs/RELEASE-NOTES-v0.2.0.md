# ROAD Proxy v3 v0.2.0

## Highlights

- Added field-tested Sons Of The Forest dedicated-server support with named
  multi-port UDP targets for `8766`, `9700`, and `27016`.
- Each SOTF UDP listener uses an independent WebSocket session selected by
  `target=<id>` during connection setup. Game payloads remain unchanged.
- Added optional Windows Plugin Studio packet metadata capture using built-in
  `pktmon`. It records aggregate packet size, timing, packet-rate, port, and
  conservative RakNet/SLikeNet signals, then deletes temporary raw captures.
- Plugin Studio falls back to normal socket scanning when advanced capture is
  unavailable. Use `--advanced-capture required` to make capture mandatory.
- Removed incomplete draft profiles from release inputs and hardened release
  cleanup so stale release-note artifacts do not remain in `build/release`.

## Downloads

- Windows amd64: normal Windows PCs.
- Windows arm64: experimental.
- Linux amd64: normal Linux PCs and servers.
- Linux arm64: ARM64 Linux devices and servers.

Each archive has a matching `.sha256` file. Verify it before using a public
download.

## Important Scope

ROAD carries direct/LAN/local IP:port TCP or UDP flows over WebSocket/WSS. It
is not a VPN, LAN broadcast emulator, or a replacement for Steam/EOS lobby and
relay transports. Public deployments should use token authentication.
