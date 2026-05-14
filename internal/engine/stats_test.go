package engine

import (
	"testing"
	"time"
)

func TestStatsSnapshotIncludesUDPMetrics(t *testing.T) {
	stats := NewStats()
	start := time.Unix(0, 0)

	stats.ObserveUDPRx(start, makeStatsRakNetDatagram(1))
	stats.ObserveUDPRx(start.Add(10*time.Millisecond), makeStatsRakNetDatagram(3))
	stats.ObserveUDPRx(start.Add(40*time.Millisecond), makeStatsRakNetDatagram(2))
	stats.ObserveUDPRx(start.Add(50*time.Millisecond), make([]byte, 1401))
	stats.ObserveUDPTx(start, []byte{0x01, 0x02})

	snapshot := stats.Snapshot()
	if snapshot.UDP.RX.Packets != 4 {
		t.Fatalf("udp.rx.packets = %d, want 4", snapshot.UDP.RX.Packets)
	}
	if snapshot.UDP.RX.LossPackets != 1 {
		t.Fatalf("udp.rx.loss_packets = %d, want 1", snapshot.UDP.RX.LossPackets)
	}
	if snapshot.UDP.RX.ReorderedPackets != 1 {
		t.Fatalf("udp.rx.reordered_packets = %d, want 1", snapshot.UDP.RX.ReorderedPackets)
	}
	if snapshot.UDP.RX.LossPercent <= 0 {
		t.Fatalf("expected positive udp.rx.loss_percent, got %.2f", snapshot.UDP.RX.LossPercent)
	}
	if snapshot.UDP.RX.JitterMS <= 0 {
		t.Fatalf("expected positive udp.rx.jitter_ms, got %.2f", snapshot.UDP.RX.JitterMS)
	}
	if snapshot.UDP.RX.MaxPayloadBytes != 1401 {
		t.Fatalf("udp.rx.max_payload_bytes = %d, want 1401", snapshot.UDP.RX.MaxPayloadBytes)
	}
	if snapshot.UDP.RX.Over1200Packets != 1 || snapshot.UDP.RX.Over1400Packets != 1 {
		t.Fatalf("unexpected udp.rx large packet counters: %#v", snapshot.UDP.RX)
	}
	if snapshot.UDP.TX.Packets != 1 {
		t.Fatalf("udp.tx.packets = %d, want 1", snapshot.UDP.TX.Packets)
	}
}

func TestStatsSnapshotIncludesPerPluginStats(t *testing.T) {
	stats := NewStats()
	start := time.Unix(0, 0)

	stats.RegisterPlugins([]string{"game", "idle"})
	stats.SessionStartPlugin("game")
	stats.AddRxPlugin("game", 5)
	stats.AddTxPlugin("game", 7)
	stats.ObserveUDPRxPlugin("game", start, makeStatsRakNetDatagram(1))
	stats.ObserveUDPRxPlugin("game", start.Add(10*time.Millisecond), makeStatsRakNetDatagram(3))
	stats.IncErrorPlugin("game")

	snapshot := stats.Snapshot()
	game, ok := snapshot.Plugins["game"]
	if !ok {
		t.Fatalf("game plugin stats missing: %#v", snapshot.Plugins)
	}
	if game.ActiveConnections != 1 || game.TotalConnections != 1 {
		t.Fatalf("unexpected game session stats: %#v", game)
	}
	if game.TotalBytesRx != 5 || game.TotalBytesTx != 7 {
		t.Fatalf("unexpected game byte stats: %#v", game)
	}
	if game.Errors != 1 {
		t.Fatalf("unexpected game errors: %#v", game)
	}
	if game.UDP.RX.LossPackets != 1 {
		t.Fatalf("game udp.rx.loss_packets = %d, want 1", game.UDP.RX.LossPackets)
	}
	if _, ok := snapshot.Plugins["idle"]; !ok {
		t.Fatalf("idle plugin should be present with zero stats: %#v", snapshot.Plugins)
	}

	stats.SessionEndPlugin("game")
	snapshot = stats.Snapshot()
	if got := snapshot.Plugins["game"].ActiveConnections; got != 0 {
		t.Fatalf("game active_connections = %d, want 0", got)
	}
}

func TestStatsSnapshotIncludesActiveSessions(t *testing.T) {
	stats := NewStats()
	start := time.Unix(0, 0)

	sessionID := stats.StartSession(SessionMeta{
		Plugin:     "game",
		Transport:  "websocket",
		Network:    "udp",
		RemoteAddr: "192.0.2.10:50000",
		TargetAddr: "127.0.0.1:7777",
	})
	stats.AddSessionRx(sessionID, 10)
	stats.AddSessionTx(sessionID, 20)
	stats.ObserveSessionUDPRx(sessionID, start, makeStatsRakNetDatagram(1))
	stats.ObserveSessionUDPRx(sessionID, start.Add(10*time.Millisecond), makeStatsRakNetDatagram(3))

	snapshot := stats.Snapshot()
	if len(snapshot.Sessions) != 1 {
		t.Fatalf("session count = %d, want 1: %#v", len(snapshot.Sessions), snapshot.Sessions)
	}
	session := snapshot.Sessions[0]
	if session.ID != sessionID {
		t.Fatalf("session id = %q, want %q", session.ID, sessionID)
	}
	if session.Plugin != "game" || session.RemoteAddr != "192.0.2.10:50000" || session.TargetAddr != "127.0.0.1:7777" {
		t.Fatalf("unexpected session identity: %#v", session)
	}
	if session.BytesRx != 10 || session.BytesTx != 20 {
		t.Fatalf("unexpected session bytes: %#v", session)
	}
	if session.UDP.RX.LossPackets != 1 {
		t.Fatalf("session udp.rx.loss_packets = %d, want 1", session.UDP.RX.LossPackets)
	}
	if snapshot.Plugins["game"].TotalBytesRx != 10 || snapshot.Plugins["game"].TotalBytesTx != 20 {
		t.Fatalf("plugin bytes not updated from session: %#v", snapshot.Plugins["game"])
	}

	stats.EndSession(sessionID)
	snapshot = stats.Snapshot()
	if len(snapshot.Sessions) != 0 {
		t.Fatalf("sessions should be empty after end: %#v", snapshot.Sessions)
	}
	if got := snapshot.ActiveConnections; got != 0 {
		t.Fatalf("active_connections = %d, want 0", got)
	}
}

func makeStatsRakNetDatagram(seq uint32) []byte {
	return []byte{
		0x84,
		byte(seq),
		byte(seq >> 8),
		byte(seq >> 16),
		0x00,
	}
}
