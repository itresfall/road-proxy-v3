# Windows Service / NSSM

ROAD does not need a built-in Windows service wrapper. The practical path is
NSSM because it is simple, reversible, and keeps ROAD binaries unchanged.

## Layout

Example install folder:

```text
C:\road-proxy-v3
```

Expected files:

```text
road-server.exe
voice-server.exe
configs\
plugins\
locales\
docs\
compat-profiles\
deploy\
```

## Install ROAD Server Service

Install NSSM first, then run PowerShell as Administrator:

```powershell
.\deploy\windows\install-road-server-nssm.example.ps1 `
  -InstallDir C:\road-proxy-v3 `
  -NSSM C:\tools\nssm\nssm.exe `
  -ServiceName road-server `
  -Config configs\server.json
```

Start/stop:

```powershell
Start-Service road-server
Stop-Service road-server
Restart-Service road-server
```

Remove:

```powershell
nssm remove road-server confirm
```

## Install Voice Service

```powershell
.\deploy\windows\install-road-server-nssm.example.ps1 `
  -InstallDir C:\road-proxy-v3 `
  -NSSM C:\tools\nssm\nssm.exe `
  -ServiceName road-voice `
  -Config configs\voice-server.json `
  -Mode voice
```

## Firewall

Default ROAD ports:

```powershell
.\deploy\windows\open-firewall.ps1 -TcpPorts 8080,8081
```

Preview only:

```powershell
.\deploy\windows\open-firewall.ps1 -TcpPorts 8080,8081 -WhatIfOnly
```
