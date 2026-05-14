package udputil

import (
	"testing"
	"time"
)

func TestParseRakNetSequence(t *testing.T) {
	seq, ok := ParseRakNetSequence([]byte{0x84, 0x34, 0x12, 0x00})
	if !ok {
		t.Fatal("expected raknet sequence parse success")
	}
	if seq != 0x1234 {
		t.Fatalf("unexpected sequence: %d", seq)
	}
}

func TestFlowMetricsDetectsLossReorderDuplicateAndJitter(t *testing.T) {
	start := time.Unix(0, 0)
	var m FlowMetrics

	m.ObservePacket(start, makeRakNetDatagram(10))
	m.ObservePacket(start.Add(10*time.Millisecond), makeRakNetDatagram(12))
	m.ObservePacket(start.Add(40*time.Millisecond), makeRakNetDatagram(11))
	m.ObservePacket(start.Add(45*time.Millisecond), makeRakNetDatagram(12))

	s := m.Snapshot()
	if s.Packets != 4 {
		t.Fatalf("packets = %d, want 4", s.Packets)
	}
	if s.LossPackets != 1 {
		t.Fatalf("loss_packets = %d, want 1", s.LossPackets)
	}
	if s.ReorderedPackets != 1 {
		t.Fatalf("reordered_packets = %d, want 1", s.ReorderedPackets)
	}
	if s.DuplicatePackets != 1 {
		t.Fatalf("duplicate_packets = %d, want 1", s.DuplicatePackets)
	}
	if s.Jitter <= 0 {
		t.Fatalf("expected positive jitter, got %s", s.Jitter)
	}
	if s.MaxGap != 30*time.Millisecond {
		t.Fatalf("max_gap = %s, want 30ms", s.MaxGap)
	}
}

func TestFlowMetricsTracksPayloadSizeRisk(t *testing.T) {
	var m FlowMetrics
	start := time.Unix(0, 0)

	m.ObservePacket(start, make([]byte, 100))
	m.ObservePacket(start.Add(time.Millisecond), make([]byte, ConservativeMTUPayloadBytes+1))
	m.ObservePacket(start.Add(2*time.Millisecond), make([]byte, TunnelHOLRiskPayloadBytes+1))
	m.ObservePacket(start.Add(3*time.Millisecond), make([]byte, IPv4UDPFragmentPayloadBytes+1))

	s := m.Snapshot()
	if s.MaxPayloadBytes != IPv4UDPFragmentPayloadBytes+1 {
		t.Fatalf("max payload = %d, want %d", s.MaxPayloadBytes, IPv4UDPFragmentPayloadBytes+1)
	}
	if s.Over1200Packets != 3 {
		t.Fatalf("over 1200 packets = %d, want 3", s.Over1200Packets)
	}
	if s.Over1400Packets != 2 {
		t.Fatalf("over 1400 packets = %d, want 2", s.Over1400Packets)
	}
	if s.Over1472Packets != 1 {
		t.Fatalf("over 1472 packets = %d, want 1", s.Over1472Packets)
	}
}

func makeRakNetDatagram(seq uint32) []byte {
	return []byte{
		0x84,
		byte(seq),
		byte(seq >> 8),
		byte(seq >> 16),
		0x00,
	}
}
