# Docker Evaluation

Docker is useful for repeatable Linux deployment, but it should not become the
default ROAD path yet.

## Decision

Do not add Docker as the primary deployment model for the first stable release.

Reasons:

- ROAD is usually tied to host networking and local game ports.
- Docker bridge networking can confuse direct/LAN/local IP:port assumptions.
- UDP debugging is simpler with native binaries and systemd.
- Current Windows/Linux binary packages are easier for the target users.

## When Docker Makes Sense

- VPS deployment where only ROAD server runs in the container.
- Reverse proxy stack already uses Docker Compose.
- Host networking is acceptable on Linux.
- Config/plugins are mounted as volumes.

## Future Docker Shape

Possible Dockerfile behavior:

```text
FROM scratch or distroless
COPY road-server /road-server
COPY configs /configs
COPY plugins /plugins
ENTRYPOINT ["/road-server", "-config", "/configs/server.json"]
```

Possible Compose behavior:

```text
network_mode: host
volumes:
  - ./configs:/configs:ro
  - ./plugins:/plugins:ro
```

## Acceptance Criteria Before Adding

- Linux host-network example tested.
- UDP profile tested through Docker with at least one known working game/mod.
- Docs clearly warn that Docker bridge mode is not the recommended default.
- Compose example does not imply Steam/EOS relay support.
