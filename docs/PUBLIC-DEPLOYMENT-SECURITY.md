# Public Deployment Security

ROAD can be placed behind Cloudflare, Nginx, or Cloudflare Tunnel. The safest public model is:

```text
Internet
  -> Cloudflare / WAF / Access
  -> cloudflared or Nginx
  -> ROAD bound to 127.0.0.1:8080 and 127.0.0.1:8081
```

Use `configs/server-cloudflare-local.json` as the starting point for this model.

## ROAD Controls

Recommended server settings:

```json
{
  "http": {
    "listen_addr": "127.0.0.1:8080",
    "auth_token": "env:ROAD_WS_TOKEN",
    "auth_header": "X-ROAD-Token",
    "allowed_hosts": ["proxy.example.com"],
    "allowed_origins": ["https://proxy.example.com"],
    "trust_proxy_headers": true,
    "max_connections": 128,
    "max_connections_per_ip": 16,
    "rate_limit_per_minute": 60
  },
  "control": {
    "listen_addr": "127.0.0.1:8081",
    "plugin_api_public": false
  }
}
```

Recommended client settings:

```json
{
  "server_ws_url": "wss://proxy.example.com/ws",
  "auth_token": "env:ROAD_WS_TOKEN",
  "auth_header": "X-ROAD-Token"
}
```

Notes:

- If no token is configured, ROAD stays compatible with local/dev use and auth is disabled.
- `trust_proxy_headers` should be enabled only behind a trusted reverse proxy or Cloudflare Tunnel.
- `allowed_hosts` protects HTTP Host routing.
- `allowed_origins` mainly protects browser-origin WebSocket attempts; normal ROAD clients usually do not send an Origin header.
- `plugin_api_public=false` hides plugin info/download endpoints unless auth is enabled.

## Cloudflare Controls

Use at least one of these when the hostname is public:

- Cloudflare Access for private friend-group deployments.
- WAF rule that allows only expected countries, ASNs, or IP ranges.
- Rate limiting rule for `/ws` and `/api/*`.
- Bot/flood protection on the public hostname.

Cloudflare does not replace ROAD auth. It is an outer filter. ROAD token auth still prevents an arbitrary client from opening a game tunnel if the hostname is discovered.

## Risk Notes

- Directly exposing `8080` or `8081` to WAN bypasses Cloudflare protections.
- `udp_peer_broadcast=true` should stay off unless packet capture proves the game needs client-origin packets mirrored to peers.
- ROAD does not authenticate the actual game user. It authenticates access to the tunnel.
- Steam/EOS official lobby and relay traffic remain outside ROAD.
