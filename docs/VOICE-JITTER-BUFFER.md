# Voice Jitter Buffer

The jitter buffer belongs on the receiving client side. The server should keep
relaying frames and exposing metrics.

## Goal

Smooth short network jitter without adding excessive delay.

## Initial Parameters

```text
target_buffer_ms = 60
min_buffer_ms    = 40
max_buffer_ms    = 160
frame_ms         = 20
```

## Behavior

- Queue received audio frames by sequence or receive time.
- Start playback after `target_buffer_ms` is available.
- If frames arrive late, drop them instead of rewinding playback.
- If buffer underflows, play silence or comfort noise for one frame.
- If buffer grows beyond max, drop oldest queued frame.

## Metrics

Client should expose:

- current buffer depth in ms
- late frames
- dropped frames
- underflows
- average inter-arrival jitter

Server `/api/stats` can show relay-side frame counts but cannot prove playback
quality by itself.

## Acceptance Criteria

- Two phones can talk for 5 minutes without accumulating delay.
- Short Wi-Fi jitter does not create repeated audio gaps.
- Deafen/mute state changes still take effect quickly.
- Stats identify whether failures are capture, network, or playback side.
