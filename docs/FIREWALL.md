# Firewall

Open only the ports needed for the selected deployment model.

## ROAD Defaults

```text
8080/tcp  WebSocket data plane
8081/tcp  Control API
```

Game ports are usually local to ROAD and do not need to be public unless the
game itself is also exposed directly.

## Windows Helper

Run PowerShell as Administrator:

```powershell
.\deploy\windows\open-firewall.ps1 -TcpPorts 8080,8081
```

Preview:

```powershell
.\deploy\windows\open-firewall.ps1 -TcpPorts 8080,8081 -WhatIfOnly
```

## Linux UFW Helper

```bash
./deploy/linux/firewall-ufw.sh --dry-run
./deploy/linux/firewall-ufw.sh
```

With Nginx/Cloudflare HTTP(S):

```bash
./deploy/linux/firewall-ufw.sh --with-http
```

Add a custom port:

```bash
./deploy/linux/firewall-ufw.sh --port 7777/udp
```

## Reverse Proxy Model

If Nginx or Cloudflare fronts ROAD:

- expose `80/tcp` and `443/tcp` publicly
- keep ROAD data/control ports bound to localhost when possible
- avoid exposing `8081/tcp` publicly unless the control API is intentionally reachable
- prefer `configs/server-cloudflare-local.json`
- set `ROAD_WS_TOKEN` plus matching client `auth_token` when the hostname is public
