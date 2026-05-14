# Linux Deployment

ROAD keeps deployment files under `deploy/` so the `scripts/` directory can stay
limited to build scripts.

## Build

From Windows PowerShell:

```powershell
./scripts/build-linux.ps1 -Arch amd64
```

From Linux:

```bash
./scripts/build-linux.sh amd64
```

## Copy To A Linux Host

From Windows PowerShell with OpenSSH available:

```powershell
./deploy/linux/deploy-linux.ps1 -Target user@SERVER-IP -Arch amd64
```

Preview the SSH/SCP actions without changing the remote host:

```powershell
./deploy/linux/deploy-linux.ps1 -Target user@SERVER-IP -Arch amd64 -InstallSystemd -Restart -WhatIfOnly
```

Default remote install path:

```text
/opt/road-proxy-v3
```

The deploy command copies:

- ROAD Linux binaries
- `configs/`
- `plugins/`
- `locales/`
- `docs/`
- `compat-profiles/`
- `deploy/`

## Install systemd Services

Install and enable the ROAD server service:

```powershell
./deploy/linux/deploy-linux.ps1 -Target user@SERVER-IP -Arch amd64 -InstallSystemd
```

Install server plus voice service:

```powershell
./deploy/linux/deploy-linux.ps1 -Target user@SERVER-IP -Arch amd64 -InstallSystemd -IncludeVoice
```

Install and restart immediately:

```powershell
./deploy/linux/deploy-linux.ps1 -Target user@SERVER-IP -Arch amd64 -InstallSystemd -Restart
```

Templates:

```text
deploy/systemd/road-server.service
deploy/systemd/voice-server.service
```

The templates assume:

- install directory: `/opt/road-proxy-v3`
- service user/group: `road`
- server config: `configs/server.json`
- voice config: `configs/voice-server.json`

## Remote Commands

Check status:

```bash
sudo systemctl status road-server.service
sudo journalctl -u road-server.service -f
```

Restart after config changes:

```bash
sudo systemctl restart road-server.service
```

## Firewall

Open only the ports you actually expose. Default ROAD ports:

```text
8080/tcp  WebSocket data plane
8081/tcp  Control API
```

Example with UFW:

```bash
sudo ufw allow 8080/tcp
sudo ufw allow 8081/tcp
```

If Nginx/Cloudflare fronts ROAD, expose `80/tcp` and `443/tcp` publicly and keep
ROAD bound to localhost where possible.

## TLS/WSS Position

ROAD itself should stay a local/plain WebSocket service for now. TLS/WSS should
be terminated by a reverse proxy such as Nginx or Cloudflare Tunnel.

The included Nginx example lives at:

```text
deploy/nginx/road-proxy.conf.example
```
