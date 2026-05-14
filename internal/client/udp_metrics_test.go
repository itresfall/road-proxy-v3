package client

import (
	"testing"
	"time"
)

func TestParseRakNetSequence(t *testing.T) {
	seq, ok := parseRakNetSequence([]byte{0x84, 0x34, 0x12, 0x00})
	if !ok {
		t.Fatal("expected raknet sequence parse success")
	}
	if seq != 0x1234 {
		t.Fatalf("unexpected sequence: %d", seq)
	}
}

func TestParseRakNetSequenceRejectsUnsupportedPayload(t *testing.T) {
	if _, ok := parseRakNetSequence([]byte{0x01, 0x00, 0x00, 0x00}); ok {
		t.Fatal("expected non-raknet packet to be rejected")
	}
	if _, ok := parseRakNetSequence([]byte{0x84, 0x01, 0x02}); ok {
		t.Fatal("expected short packet to be rejected")
	}
}

func TestUDPSessionMetricsDetectsLossAndReorder(t *testing.T) {
	start := time.Unix(0, 0)
	m := newUDPSessionMetrics(start)

	m.observeTX(start, makeRakNetDatagram(10))
	m.observeTX(start.Add(10*time.Millisecond), makeRakNetDatagram(12)) // missing 11
	m.observeTX(start.Add(20*time.Millisecond), makeRakNetDatagram(11)) // reorder
	m.observeTX(start.Add(30*time.Millisecond), makeRakNetDatagram(12)) // duplicate

	s := m.snapshot(start.Add(40 * time.Millisecond))

	if s.TX.LossPackets != 1 {
		t.Fatalf("expected 1 lost packet, got %d", s.TX.LossPackets)
	}
	if s.TX.ReorderedPackets != 1 {
		t.Fatalf("expected 1 reordered packet, got %d", s.TX.ReorderedPackets)
	}
	if s.TX.DuplicatePackets != 1 {
		t.Fatalf("expected 1 duplicate packet, got %d", s.TX.DuplicatePackets)
	}
	if s.TX.LossPercent <= 0 {
		t.Fatalf("expected positive loss percent, got %.2f", s.TX.LossPercent)
	}
}

func TestUDPSessionMetricsJitterTracksGapVariance(t *testing.T) {
	start := time.Unix(0, 0)
	m := newUDPSessionMetrics(start)

	payload := []byte{0x01, 0x02, 0x03}
	m.observeRX(start, payload)
	m.observeRX(start.Add(10*time.Millisecond), payload)
	m.observeRX(start.Add(40*time.Millisecond), payload)

	s := m.snapshot(start.Add(50 * time.Millisecond))
	if s.RX.Jitter <= 0 {
		t.Fatalf("expected positive jitter, got %s", s.RX.Jitter)
	}
	if s.RX.MaxGap != 30*time.Millisecond {
		t.Fatalf("expected max gap 30ms, got %s", s.RX.MaxGap)
	}
}

func TestUDPSessionMetricsWithImpairmentSimulation(t *testing.T) {
	start := time.Unix(0, 0)
	m := newUDPSessionMetrics(start)

	stream := simulateRakNetImpairments(12, 10*time.Millisecond, udpImpairmentPlan{
		Drop: map[uint32]bool{
			4: true,
			5: true,
		},
		Duplicate: map[uint32]int{
			7: 1,
		},
		ReorderPairs: [][2]uint32{
			{9, 10},
		},
		ExtraDelayAfterSeq: map[uint32]time.Duration{
			8: 150 * time.Millisecond,
		},
	})

	for _, packet := range stream {
		m.observeRX(start.Add(packet.At), makeRakNetDatagram(packet.Seq))
	}

	s := m.snapshot(start.Add(500 * time.Millisecond))
	if s.RX.LossPackets != 3 {
		t.Fatalf("expected 3 lost packets, got %d", s.RX.LossPackets)
	}
	if s.RX.DuplicatePackets != 1 {
		t.Fatalf("expected 1 duplicate packet, got %d", s.RX.DuplicatePackets)
	}
	if s.RX.ReorderedPackets != 1 {
		t.Fatalf("expected 1 reordered packet, got %d", s.RX.ReorderedPackets)
	}
	if s.RX.Jitter <= 0 {
		t.Fatalf("expected positive jitter, got %s", s.RX.Jitter)
	}
	if s.RX.MaxGap < 150*time.Millisecond {
		t.Fatalf("expected burst max gap >=150ms, got %s", s.RX.MaxGap)
	}
	if s.RX.LossPercent <= 0 {
		t.Fatalf("expected positive loss percent, got %.2f", s.RX.LossPercent)
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

type udpImpairmentPlan struct {
	Drop               map[uint32]bool
	Duplicate          map[uint32]int
	ReorderPairs       [][2]uint32
	ExtraDelayAfterSeq map[uint32]time.Duration
}

type simulatedRakNetPacket struct {
	Seq uint32
	At  time.Duration
}

func simulateRakNetImpairments(count uint32, baseGap time.Duration, plan udpImpairmentPlan) []simulatedRakNetPacket {
	reorderAfter := map[uint32]uint32{}
	reorderSkip := map[uint32]bool{}
	for _, pair := range plan.ReorderPairs {
		if pair[0] == 0 || pair[1] == 0 {
			continue
		}
		reorderAfter[pair[0]] = pair[1]
		reorderSkip[pair[1]] = true
	}

	out := []simulatedRakNetPacket{}
	now := time.Duration(0)
	appendSeq := func(seq uint32) {
		if plan.Drop != nil && plan.Drop[seq] {
			now += baseGap
			return
		}
		out = append(out, simulatedRakNetPacket{Seq: seq, At: now})
		if plan.Duplicate != nil {
			for i := 0; i < plan.Duplicate[seq]; i++ {
				now += time.Millisecond
				out = append(out, simulatedRakNetPacket{Seq: seq, At: now})
			}
		}
		now += baseGap
		if plan.ExtraDelayAfterSeq != nil {
			now += plan.ExtraDelayAfterSeq[seq]
		}
	}

	for seq := uint32(1); seq <= count; seq++ {
		if reorderSkip[seq] {
			continue
		}
		if delayed, ok := reorderAfter[seq]; ok {
			appendSeq(delayed)
			appendSeq(seq)
			continue
		}
		appendSeq(seq)
	}
	return out
}
