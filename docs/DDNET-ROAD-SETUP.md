# DDNet ROAD Setup

DDNet / DDraceNetwork is the first recommended free UDP game for ROAD
acceptance tests.

Steam AppID:

```text
412220
```

Default DDNet UDP server port:

```text
8303
```

ROAD profile:

```text
plugins/ddnet-udp/plugin.json
```

Generated configs:

```text
configs/server-ddnet.json
configs/client-ddnet.json
```

## Install

Install from Steam:

```powershell
Start-Process "steam://install/412220"
```

If Steam is already installed but DDNet is not, this opens the DDNet install
flow.

## Start ROAD

Server side:

```powershell
.\build\windows\road-server.exe -config configs\server-ddnet.json
```

Client side:

```powershell
.\build\windows\road-client.exe -config configs\client-ddnet.json
```

If using the interactive menu, select the `ddnet-udp` plugin.

## Start DDNet Server

Launch DDNet's dedicated server on the machine where ROAD server can reach it.
The target must be:

```text
127.0.0.1:8303
```

If DDNet is installed through Steam, the dedicated server executable is usually
inside the DDNet install folder.

## Join Through ROAD

On each client machine, connect DDNet to:

```text
127.0.0.1:18303
```

Use direct connect/manual address entry. Do not use server browser discovery as
the ROAD validation signal.

## Pre-Game ROAD Baseline

Before the game test, run:

```powershell
.\build\windows\road-proxy.exe udp-check server --listen 127.0.0.1:8303
.\build\windows\road-proxy.exe udp-check client --target 127.0.0.1:18303 --players 3 --tickrate 30 --duration 5m --payload 256
```

The clean target is:

```text
missing=0
dup=0
reorder=0
no sustained multi-second max_gap
```

## Optional Laptop Bot Macro

Use the visible macro recorder when there is no second human player. It records
your own DDNet keyboard/mouse input and replays it on the Linux laptop as a real
DDNet client.

Install the Python dependency:

```bash
python -m pip install -r tools/ddnet-macro/requirements.txt
```

Record on the main PC:

```bash
python tools/ddnet-macro/ddnet_macro.py record --out ddnet-tutorial.jsonl --stop-key f9 --countdown 5
```

Focus DDNet before the countdown ends, play the route, then press `F9`.

Replay on the Linux laptop:

```bash
python tools/ddnet-macro/ddnet_macro.py play --in ddnet-tutorial.jsonl --countdown 5
```

Focus the Linux DDNet window before the countdown ends. The laptop must be a
real DDNet client connected to the ROAD local address.

Linux note: replay works best under X11 with the default backend. Fedora
Wayland may block synthetic input; use the `ydotool` backend there:

```bash
sudo dnf install ydotool
python tools/ddnet-macro/ddnet_macro.py play --in ddnet-tutorial.jsonl --backend ydotool --countdown 5
```

The macro records and replays keyboard down/up, mouse movement, left/right/middle
click down/up, and mouse wheel scroll.
