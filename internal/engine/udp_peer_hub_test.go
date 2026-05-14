package engine

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestUDPPeerBroadcastSlowPeerBlocksCurrentBroadcast(t *testing.T) {
	hub := newUDPPeerHub()
	sender := &udpPeerSession{send: func([]byte) error { return nil }}
	slowDelay := 60 * time.Millisecond
	slow := &udpPeerSession{send: func([]byte) error {
		time.Sleep(slowDelay)
		return nil
	}}
	var fastSends atomic.Int32
	fast := &udpPeerSession{send: func([]byte) error {
		fastSends.Add(1)
		return nil
	}}

	unregisterSender := hub.register("game", sender)
	defer unregisterSender()
	unregisterSlow := hub.register("game", slow)
	defer unregisterSlow()
	unregisterFast := hub.register("game", fast)
	defer unregisterFast()

	start := time.Now()
	sent := hub.broadcast("game", sender, []byte("state"))
	elapsed := time.Since(start)

	if sent != 2 {
		t.Fatalf("sent peers = %d, want 2", sent)
	}
	if fastSends.Load() != 1 {
		t.Fatalf("fast peer sends = %d, want 1", fastSends.Load())
	}
	if elapsed < slowDelay {
		t.Fatalf("broadcast elapsed = %s, want at least slow peer delay %s", elapsed, slowDelay)
	}
}

func TestUDPPeerBroadcastSkipsSender(t *testing.T) {
	hub := newUDPPeerHub()
	var senderSends atomic.Int32
	sender := &udpPeerSession{send: func([]byte) error {
		senderSends.Add(1)
		return nil
	}}
	peer := &udpPeerSession{send: func([]byte) error { return nil }}

	unregisterSender := hub.register("game", sender)
	defer unregisterSender()
	unregisterPeer := hub.register("game", peer)
	defer unregisterPeer()

	if sent := hub.broadcast("game", sender, []byte("state")); sent != 1 {
		t.Fatalf("sent peers = %d, want 1", sent)
	}
	if senderSends.Load() != 0 {
		t.Fatalf("sender should not receive its own broadcast, got %d sends", senderSends.Load())
	}
}
