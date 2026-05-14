# Cloudflare Free Integration (WSS)

This project does not proxy raw game TCP/UDP through Cloudflare directly. The supported model is:

1. Game traffic enters local ROAD client as TCP/UDP.
2. ROAD client sends data over WebSocket (`ws`/`wss`).
3. Cloudflare proxies HTTPS/WSS.
4. Origin reverse proxy forwards `/ws` to ROAD server.

## Target Architecture

```text
Game -> road client (local tcp/udp listener)
     -> wss://proxy.example.com/ws
     -> Cloudflare edge
     -> Nginx or cloudflared
     -> road server (127.0.0.1:8080 /ws)
     -> game server target
```

## Origin Host Setup

Use Linux + Nginx or Cloudflare Tunnel. Keep ROAD data/control ports local-only on the origin when Cloudflare is in front.

Example Nginx setup:

```bash
sudo apt update
sudo apt install -y nginx
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

Apply the example Nginx config and edit hostname/certificate paths:

```bash
sudo cp deploy/nginx/road-proxy.conf.example /etc/nginx/sites-available/road-proxy.conf
sudo ln -s /etc/nginx/sites-available/road-proxy.conf /etc/nginx/sites-enabled/road-proxy.conf
sudo nginx -t
sudo systemctl reload nginx
```

Important:

- Keep ROAD `http.listen_addr` and `control.listen_addr` local-only on the origin if Nginx is proxying them.
- Do not expose `8080` and `8081` directly to WAN unless you intentionally want that.
- Prefer `configs/server-cloudflare-local.json` for Cloudflare Tunnel or local reverse proxy deployments.
- Set `ROAD_WS_TOKEN` and use matching client `auth_token` when the hostname is reachable by untrusted users.
- See `docs/PUBLIC-DEPLOYMENT-SECURITY.md` for Cloudflare Access, WAF, allowlist, and rate-limit guidance.

## Config Approach

Use the local-only preset for Cloudflare Tunnel or reverse proxy deployments. For custom ports/plugins, copy the normal configs and edit only the endpoint/listen values you need.

Security-oriented local-only preset:

```text
Server template: configs/server-cloudflare-local.json
Listen: 127.0.0.1:8080 / 127.0.0.1:8081
Optional token env: ROAD_WS_TOKEN
Security doc: docs/PUBLIC-DEPLOYMENT-SECURITY.md
```

TCP example:

```text
Server template: configs/server.json
Client template: configs/client.json
Client server_ws_url: wss://proxy.example.com/ws
```

UDP example:

```text
Server template: configs/server-udp.example.json
Client template: configs/client-udp.example.json
Client server_ws_url: wss://proxy.example.com/ws
```

Run from source:

```powershell
go run ./cmd/server -config configs/server.json
go run ./cmd/client -config configs/client.json
```

Run from Windows build output:

```powershell
./build/windows/road-server.exe -config configs/server.json
./build/windows/road-client.exe -config configs/client.json
```

## Verification

From any machine:

```bash
curl -sS https://proxy.example.com/api/health
curl -sS https://proxy.example.com/api/info
```

Expected:

1. `/api/health` returns `status=ok`.
2. Client connects with `wss://.../ws`.
3. No continuous reconnect loop logs.

## Frequent Issues

1. `525 SSL handshake failed`: origin cert/key mismatch or wrong path in Nginx.
2. `521 Web server is down`: Nginx or tunnel is not reachable.
3. Game joins but random drops: check `ws_idle_timeout`, `ws_ping_interval`, and UDP metrics logs.

## Optional: No Port Forwarding

If opening `443` is not possible, use `cloudflared tunnel` and map the hostname to `http://localhost:8080` or to local Nginx.

## Public Server Wizard

ROAD also includes an interactive Cloudflare helper:

```bash
road-proxy public-server
```

Or from the built-in menu:

```text
3) Public Server Wizard
```

The wizard can:

- Find `cloudflared` on `PATH`.
- Optionally download `cloudflared` into a per-user directory when it is missing.
- Start a TryCloudflare quick tunnel and capture the generated `trycloudflare.com` URL.
- Run an existing Cloudflare tunnel token.
- Run `cloudflared tunnel login`, create a named tunnel, add DNS with `cloudflared tunnel route dns`, and start the tunnel with a ROAD-generated ingress config.
- Generate a secure ROAD server config with local-only bind, token auth, public deployment limits, and private plugin API.
- Generate `configs/.generated/client.public.menu.json` for clients.

Output includes:

```text
Endpoint: wss://example.trycloudflare.com/ws
Auth header: X-ROAD-Token
Auth token: <generated-token>
Local dashboard: http://127.0.0.1:8081/dashboard
```

Give remote clients both `Endpoint` and `Auth token`. In interactive client
mode they enter the endpoint first, then paste the token when ROAD asks for it.
If the client logs HTTP `401` or `403`, Cloudflare routing reached ROAD but ROAD
rejected the missing or wrong token.

Notes:

- TryCloudflare URLs are temporary.
- Domain mode requires the domain to be on the Cloudflare account selected during `cloudflared tunnel login`.
- Permanent domain mode uses the `cert.pem` created by `cloudflared tunnel login`; ROAD does not ask for a Cloudflare API token.
- Registrar-side nameserver changes are manual and documented in `docs/CLOUDFLARE-WIZARD-LIMITS.md`.
- `cloudflared` auto-download verifies SHA256 checksum data from the latest Cloudflare `cloudflared` GitHub release when checksum data is available.
- See `docs/CLOUDFLARE-WIZARD-LIMITS.md` for what can be automated without a Cloudflare API token.
