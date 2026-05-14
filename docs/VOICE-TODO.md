# ROAD Voice TODO

Voice is a separate runtime, not a ROAD plugin.

Current repo pieces:

- `cmd/voice-server`
- `internal/voice`
- `configs/voice-server.json`
- `deploy/systemd/voice-server.service`
- `docs/VOICE-CHAT-PLAN.md`

## Current MVP State

Done:

- WebSocket voice server binary exists.
- Join flow uses JSON control messages.
- User list broadcasts on join, leave, and state change.
- Audio frames are WebSocket binary messages.
- Current audio format is PCM-style raw relay at protocol level; the server does not transcode.
- Mute stops forwarding sender audio.
- Deafen stops delivering audio to that listener.
- `/api/health`, `/api/users`, and `/api/stats` exist.

Not done:

- Android app is outside this repo.
- Opus encoding is not integrated.
- Client-side jitter buffer is not implemented in this repo.
- Real two-phone latency/audio quality test is still required.

## Near-Term Tasks

1. Keep PCM relay until two-phone MVP proves the path.
2. Add Android client build/test notes outside ROAD repo.
3. Add Opus only after PCM behavior is stable.
4. Add client-side jitter buffer before judging real voice quality.
5. Use `/api/stats` and logs during tests.
6. Keep voice deployment separate from game proxy deployment.

## Related Docs

```text
docs/VOICE-CHAT-PLAN.md
docs/VOICE-OPUS-PLAN.md
docs/VOICE-JITTER-BUFFER.md
docs/VOICE-ANDROID-BUILD-TEST.md
docs/VOICE-TWO-PHONE-CHECKLIST.md
```
