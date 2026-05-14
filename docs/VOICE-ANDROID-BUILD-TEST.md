# Voice Android Build / Test

The Android client is intentionally outside this ROAD repo.

Expected location:

```text
C:\Projects\road-voice-android
```

## Build

```powershell
cd C:\Projects\road-voice-android
./scripts/build-debug.ps1
```

Expected APK:

```text
app\build\outputs\apk\debug\app-debug.apk
```

## Server

From ROAD repo:

```powershell
go run ./cmd/voice-server -config configs/voice-server.json
```

Or from build output:

```powershell
./build/windows/voice-server.exe -config configs/voice-server.json
```

## Android Test Inputs

Use a reachable endpoint:

```text
ws://LAN-IP:8090/ws
wss://voice.example.com/ws
```

Each phone should use a unique display name.

## Basic Test

1. Start `voice-server`.
2. Open `/api/health`.
3. Connect phone 1.
4. Connect phone 2.
5. Confirm `/api/users` shows both users.
6. Talk from phone 1 and confirm phone 2 hears it.
7. Toggle mute on phone 1 and confirm audio stops.
8. Toggle deafen on phone 2 and confirm audio stops only for phone 2.
9. Check `/api/stats` for frame counters.
