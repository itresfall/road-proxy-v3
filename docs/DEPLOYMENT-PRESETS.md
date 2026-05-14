# Deployment Presets

ROAD supports direct TCP/UDP port traffic over a WebSocket data plane. These
presets keep that scope explicit.

## LAN Preset

Use when all players can reach the host LAN IP.

Server:

```text
road-server on host machine
http.listen_addr = 0.0.0.0:8080
control.listen_addr = 127.0.0.1:8081 or 0.0.0.0:8081 only if trusted LAN
```

Client:

```text
server_ws_url = ws://HOST-LAN-IP:8080/ws
```

Firewall:

```text
allow 8080/tcp on host
avoid public control API exposure
```

## VPS Preset

Use when the server side runs on a Linux VPS near the game host or when the VPS
can reach the real game server target.

Server:

```text
/opt/road-proxy-v3/road-server -config configs/server.json
systemd: deploy/systemd/road-server.service
```

Deploy:

```powershell
./deploy/linux/deploy-linux.ps1 -Target user@VPS-IP -Arch amd64 -InstallSystemd -Restart
```

Firewall:

```text
allow 8080/tcp
allow 8081/tcp only if the control API is intentionally reachable
```

## Cloudflare / Nginx Preset

Use when TLS/WSS should terminate outside ROAD.

ROAD local side:

```text
config = configs/server-cloudflare-local.json
http.listen_addr = 127.0.0.1:8080
control.listen_addr = 127.0.0.1:8081
optional token env = ROAD_WS_TOKEN
```

Nginx:

```text
deploy/nginx/road-proxy.conf.example
```

Client:

```text
server_ws_url = wss://proxy.example.com/ws
auth_token = env:ROAD_WS_TOKEN
auth_header = X-ROAD-Token
```

Position:

- ROAD stays plain local WebSocket.
- TLS/WSS belongs to Nginx, Cloudflare Tunnel, or a similar edge layer.
- Official Steam/EOS lobby/relay traffic remains outside ROAD.
