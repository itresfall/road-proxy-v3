# roadctl Plan

`roadctl` should be a thin operational command layer, not a second runtime.

## Goal

Group common start/stop/status/log/deploy checks behind one predictable CLI
without hiding the existing binaries.

## Proposed Commands

```text
roadctl status
roadctl logs --service road-server
roadctl start --service road-server
roadctl stop --service road-server
roadctl restart --service road-server
roadctl validate
roadctl diagnose --out diagnostics
roadctl ports
```

## Platform Behavior

Windows:

- detect NSSM services first
- fallback to process lookup by binary name
- use PowerShell firewall APIs for `ports`

Linux:

- detect systemd services first
- fallback to process lookup by binary name
- use `ss` for port inspection

## Do Not Duplicate

`roadctl diagnose` should call or share logic with:

```text
road-proxy diagnostic-bundle
```

`roadctl validate` should call or share logic with:

```text
road-proxy validate --all-configs
```

## Acceptance Criteria

- Works on Windows and Linux.
- Does not require macOS support.
- Has a `--dry-run` mode for mutating commands.
- Has text output first; JSON output can be added later.
- Keeps direct binary usage documented and supported.
