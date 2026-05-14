# Cloudflare Wizard Limits

This document defines what ROAD Public Server Wizard can automate without asking
for a Cloudflare API token, and what remains outside the product scope.

## Project Decision

ROAD will not ask normal users for a Cloudflare API token in the Public Server
Wizard. The supported permanent-domain flow is based on:

```text
cloudflared tunnel login
-> ~/.cloudflared/cert.pem
-> cloudflared tunnel create
-> cloudflared tunnel route dns
-> cloudflared tunnel run
```

That certificate is enough for the Cloudflare-side tunnel and DNS route work
after the domain is already active in the user's Cloudflare account.

ROAD will also not automate registrar-side nameserver changes. Every registrar
has a different API and account model, so this is documented as a short manual
step instead.

## TryCloudflare Mode

Command path:

```text
road-proxy public-server -> 1) TryCloudflare quick temporary URL
```

Properties:

- No Cloudflare account is required.
- No domain is required.
- No Cloudflare token is required.
- ROAD runs `cloudflared tunnel --url http://127.0.0.1:8080`.
- The generated URL is temporary and changes between runs.
- The URL stops working when the wizard is stopped.

ROAD uses an isolated temporary `cloudflared` config file in this mode. This
keeps an existing user-level `~/.cloudflared/config.yml` from changing the quick
tunnel behavior.

ROAD also uses `configs/.generated/public-server.lock` to prevent starting two
Public Server Wizard instances at the same time. This avoids two local ROAD
servers or two `cloudflared` processes racing for the same public endpoint.

The wizard asks for local ROAD data/control ports. The default data origin is:

```text
http://127.0.0.1:8080
```

If the data port is changed, ROAD updates these values from the same setting:

- ROAD `http.listen_addr`.
- TryCloudflare `cloudflared tunnel --url`.
- TryCloudflare isolated `config.yml`.
- Named tunnel ingress `service` entries.
- Generated server/client hint configs.
- Printed local dashboard URL.

Named domain mode writes multi-service ingress rules:

```text
/ws         -> ROAD data origin, for example http://127.0.0.1:8080
/dashboard  -> ROAD control origin, for example http://127.0.0.1:8081
/api/*      -> ROAD control origin, for example http://127.0.0.1:8081
fallback    -> ROAD data origin
```

The dashboard shell can load through the domain, but control API data remains
protected by the generated ROAD token. Use Cloudflare Access/WAF/IP allowlists
for stricter public deployments.

Existing Cloudflare tunnel-token mode cannot rewrite Cloudflare's remote tunnel
configuration. The Cloudflare-side tunnel must already target the local origin
printed by ROAD.

## Domain Mode Without API Token

Command path:

```text
road-proxy public-server -> 2) Domain + cloudflared login + DNS route
```

This mode uses `cloudflared tunnel login`. The browser login flow writes
`~/.cloudflared/cert.pem`, then ROAD can call these `cloudflared` commands:

```text
cloudflared tunnel create <name>
cloudflared tunnel route dns <name> <hostname>
cloudflared tunnel run <name>
```

The selected domain must already be on the Cloudflare account used during login.
ROAD does not need a Cloudflare API token for this flow because `cloudflared`
uses the login certificate.

If the tunnel name or DNS record already exists, ROAD asks whether to reuse the
existing object instead of failing immediately.

## Existing Token Mode

Command path:

```text
road-proxy public-server -> 3) Run with existing Cloudflare tunnel token
```

This mode runs:

```text
cloudflared tunnel run --token <TOKEN>
```

It does not create or edit DNS records. The tunnel and hostname should already
exist in Cloudflare.

## Connecting A Domain To Cloudflare

This is the only required manual domain-provider step before ROAD's permanent
domain mode can work.

1. Add the domain to Cloudflare.
2. Cloudflare shows two nameservers.
3. Open the domain provider panel where the domain was purchased.
4. Replace the current nameservers with the two Cloudflare nameservers.
5. Wait for DNS propagation.
6. In Cloudflare, refresh/check until the domain is active.
7. Run `road-proxy public-server` and choose domain mode.

After this setup, ROAD can create the tunnel and DNS route through `cloudflared`
without asking for a Cloudflare API token.

## Auto Download Security

When ROAD downloads `cloudflared`, it fetches checksum data from the latest
Cloudflare `cloudflared` GitHub release and verifies the downloaded binary with
SHA256 before installing it.

If checksum data cannot be fetched, ROAD asks before continuing. This keeps the
easy path available while making the trust decision explicit.

## Outside The Product Scope

These features are not planned for the normal wizard path:

- Listing all zones/domains in the account.
- Choosing a domain from a Cloudflare API result.
- Changing nameservers at the domain registrar.
- Automating registrar-specific domain setup.
- Creating Cloudflare Access policies.
- Creating WAF rules.
- Managing IP allowlists.
- Editing or deleting existing tunnels beyond basic reuse.

An API-token path may be reconsidered only if there is a concrete need that the
`cloudflared tunnel login` certificate flow cannot cover.
