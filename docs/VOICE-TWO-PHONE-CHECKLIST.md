# Voice Two-Phone Checklist

Use this before calling the voice MVP usable.

## Setup

- Phone 1 and phone 2 have the same APK build.
- Both phones are on the same Wi-Fi or both can reach the public WSS endpoint.
- `voice-server` is running.
- `/api/health` returns `ok=true`.
- `/api/stats` is reachable.

## Functional Checks

- Phone 1 joins.
- Phone 2 joins.
- Both names appear in `/api/users`.
- Phone 1 audio reaches phone 2.
- Phone 2 audio reaches phone 1.
- Phone 1 mute stops phone 1 audio.
- Phone 2 deafen stops phone 2 receiving audio.
- Leave/rejoin updates the user list.

## Quality Checks

- 1 minute normal speech.
- 5 minute idle plus occasional speech.
- Screen off/on if Android allows background audio.
- Wi-Fi weak signal test if available.
- Mobile data test if public WSS endpoint is available.

## Metrics To Record

From `/api/stats`:

- `active_users`
- `total_joins`
- `audio_frames_rx`
- `audio_frames_tx`
- `audio_bytes_rx`
- `audio_bytes_tx`
- `audio_dropped_muted`
- `errors`

Result:

```text
date:
server:
apk:
network:
pass/fail:
notes:
```
