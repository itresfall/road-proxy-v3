package main

import (
	"context"
	"io"
	"testing"
	"time"
)

func TestUDPCheckPacketRoundTrip(t *testing.T) {
	packet := udpCheckPacket{
		Kind:           udpCheckKindPing,
		PlayerID:       2,
		Sequence:       42,
		SentUnixNano:   123,
		ServerUnixNano: 456,
		PayloadBytes:   256,
		TickRate:       30,
		State:          99,
	}

	encoded := encodeUDPCheckPacket(packet, 256)
	if len(encoded) != 256 {
		t.Fatalf("encoded len = %d, want 256", len(encoded))
	}
	decoded, err := decodeUDPCheckPacket(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded.Kind != packet.Kind ||
		decoded.PlayerID != packet.PlayerID ||
		decoded.Sequence != packet.Sequence ||
		decoded.SentUnixNano != packet.SentUnixNano ||
		decoded.ServerUnixNano != packet.ServerUnixNano ||
		decoded.PayloadBytes != 256 ||
		decoded.TickRate != packet.TickRate ||
		decoded.State != packet.State {
		t.Fatalf("decoded packet mismatch: %#v", decoded)
	}
}

func TestUDPCheckLoopback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan string, 1)
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- runUDPCheckServer(ctx, udpCheckServerOptions{
			Listen:         "127.0.0.1:0",
			BufferBytes:    udpCheckMaxPacket,
			ReportInterval: 0,
			Out:            io.Discard,
			Ready:          ready,
		})
	}()

	addr := <-ready
	result, err := runUDPCheckClient(context.Background(), udpCheckClientOptions{
		Target:       addr,
		Players:      2,
		TickRate:     20,
		Duration:     200 * time.Millisecond,
		PayloadBytes: 128,
		Grace:        500 * time.Millisecond,
		Out:          io.Discard,
	})
	if err != nil {
		t.Fatalf("client failed: %v", err)
	}
	cancel()
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatalf("server failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}

	if len(result.Players) != 2 {
		t.Fatalf("players = %d, want 2", len(result.Players))
	}
	for _, player := range result.Players {
		if player.Sent.UniquePackets == 0 {
			t.Fatalf("player %d sent no packets", player.PlayerID)
		}
		if player.Ack.UniquePackets == 0 {
			t.Fatalf("player %d received no acks", player.PlayerID)
		}
		if player.RTT.Count == 0 {
			t.Fatalf("player %d has no RTT samples", player.PlayerID)
		}
	}
}
