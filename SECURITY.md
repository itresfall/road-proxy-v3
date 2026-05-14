# Security Policy

ROAD Proxy v3 is a network-facing tool. Treat public deployments as exposed
infrastructure, not as a private LAN toy.

## Supported Versions

Only tagged releases from this repository are considered supported. Development
builds such as `0.1.0-dev` are best-effort snapshots.

| Version | Supported |
| --- | --- |
| Latest tagged release | Yes |
| Older tagged releases | Best effort |
| Untagged development builds | No |

## Reporting A Vulnerability

If you find a security issue, do not publish a working exploit first.

Preferred report contents:

- ROAD version and commit, from `road-proxy --version`.
- Operating system and architecture.
- Deployment mode: LAN, VPS, Cloudflare, TryCloudflare, or other reverse proxy.
- Relevant config snippets with tokens removed.
- Reproduction steps.
- Expected impact.

If GitHub private vulnerability reporting is enabled for the repository, use
that path. Otherwise open a minimal public issue that says a security report is
available, without exploit details, and ask for a private contact path.

## Public Deployment Baseline

For internet-facing deployments:

- Use `configs/server-cloudflare-local.json` or equivalent local-only binding
  behind Cloudflare, Nginx, or another reverse proxy.
- Enable ROAD shared-token auth for WebSocket and control API access.
- Keep `control.plugin_api_public=false` unless you intentionally expose plugin
  metadata.
- Configure Host and Origin allowlists where practical.
- Configure connection and rate limits.
- Prefer Cloudflare Access, WAF rules, or IP allowlists for untrusted networks.
- Do not publish ROAD tokens in screenshots, logs, issues, or support bundles.

See `docs/PUBLIC-DEPLOYMENT-SECURITY.md` for detailed deployment guidance.

## Out Of Scope

ROAD is not a VPN, malware sandbox, anti-cheat bypass, Steam/EOS relay
replacement, or anonymity tool. Reports that depend on those unsupported use
cases may be closed as out of scope.

