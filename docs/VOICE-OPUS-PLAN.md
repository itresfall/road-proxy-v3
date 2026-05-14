# Voice Opus Plan

PCM stays the MVP path. Opus is the next step only after the basic Android to
voice-server path is proven with real devices.

## Why Not First

- PCM is easier to debug.
- The Go server only relays frames; codec work belongs primarily in clients.
- Android Opus integration adds build/runtime complexity.
- Broken capture/playback should not be confused with server relay bugs.

## Target Design

Client responsibilities:

- capture microphone PCM
- encode PCM to Opus
- send Opus frames as WebSocket binary messages
- receive Opus frames
- decode Opus to PCM
- play PCM through audio output

Server responsibilities:

- relay binary frames
- enforce max frame size
- keep mute/deafen state
- expose stats
- avoid transcoding in the MVP server

## Frame Metadata

If raw binary frames are not enough, add a tiny header before payload:

```text
magic      2 bytes  "RV"
version    1 byte
codec      1 byte   0=pcm, 1=opus
sequence   4 bytes
timestamp  8 bytes  monotonic client time or sample clock
payload    n bytes
```

Do not add this header until Android needs mixed codec/version negotiation.

## Acceptance Criteria

- PCM two-phone test passes first.
- Opus test shows lower bandwidth than PCM.
- Added latency stays acceptable.
- Packet loss does not produce long playback stalls.
- The server remains codec-agnostic unless a strong reason appears.
