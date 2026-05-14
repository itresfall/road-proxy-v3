package udputil

import (
	"fmt"
	"time"
)

const (
	RakNetDataPacketMin byte   = 0x80
	RakNetDataPacketMax byte   = 0x8d
	RakNetSeqModulo     uint32 = 1 << 24
)

// FlowMetrics tracks timing and RakNet-style sequence health for one UDP direction.
type FlowMetrics struct {
	packets uint64
	bytes   uint64

	lastPacketAt time.Time
	lastGap      time.Duration
	maxGap       time.Duration
	jitterNanos  float64

	seqPackets       uint64
	lossPackets      uint64
	reorderedPackets uint64
	duplicatePackets uint64
	seqInitialized   bool
	nextExpectedSeq  uint32

	maxPayloadBytes uint64
	over1200Packets uint64
	over1400Packets uint64
	over1472Packets uint64
}

type FlowSnapshot struct {
	Packets          uint64
	Bytes            uint64
	Jitter           time.Duration
	MaxGap           time.Duration
	SeqPackets       uint64
	LossPackets      uint64
	ReorderedPackets uint64
	DuplicatePackets uint64
	LossPercent      float64
	MaxPayloadBytes  uint64
	Over1200Packets  uint64
	Over1400Packets  uint64
	Over1472Packets  uint64
}

func (f *FlowMetrics) ObservePacket(ts time.Time, payload []byte) {
	f.packets++
	payloadBytes := uint64(len(payload))
	f.bytes += payloadBytes
	if payloadBytes > f.maxPayloadBytes {
		f.maxPayloadBytes = payloadBytes
	}
	if AboveConservativeMTU(len(payload)) {
		f.over1200Packets++
	}
	if AboveTunnelHOLRisk(len(payload)) {
		f.over1400Packets++
	}
	if AboveIPv4UDPFragmentRisk(len(payload)) {
		f.over1472Packets++
	}

	if !f.lastPacketAt.IsZero() {
		gap := ts.Sub(f.lastPacketAt)
		if gap < 0 {
			gap = 0
		}
		if f.lastGap > 0 {
			diff := gap - f.lastGap
			if diff < 0 {
				diff = -diff
			}
			diffNanos := float64(diff)
			if f.jitterNanos == 0 {
				f.jitterNanos = diffNanos
			} else {
				f.jitterNanos += (diffNanos - f.jitterNanos) / 16.0
			}
		}
		f.lastGap = gap
		if gap > f.maxGap {
			f.maxGap = gap
		}
	}
	f.lastPacketAt = ts

	seq, ok := ParseRakNetSequence(payload)
	if !ok {
		return
	}
	f.seqPackets++
	if !f.seqInitialized {
		f.seqInitialized = true
		f.nextExpectedSeq = (seq + 1) % RakNetSeqModulo
		return
	}

	forward := SeqForwardDistance(f.nextExpectedSeq, seq)
	switch {
	case forward == 0:
		f.nextExpectedSeq = (seq + 1) % RakNetSeqModulo
	case forward < RakNetSeqModulo/2:
		f.lossPackets += uint64(forward)
		f.nextExpectedSeq = (seq + 1) % RakNetSeqModulo
	default:
		prevExpected := (f.nextExpectedSeq + RakNetSeqModulo - 1) % RakNetSeqModulo
		if seq == prevExpected {
			f.duplicatePackets++
		} else {
			f.reorderedPackets++
		}
	}
}

func (f *FlowMetrics) Snapshot() FlowSnapshot {
	s := FlowSnapshot{
		Packets:          f.packets,
		Bytes:            f.bytes,
		Jitter:           time.Duration(f.jitterNanos),
		MaxGap:           f.maxGap,
		SeqPackets:       f.seqPackets,
		LossPackets:      f.lossPackets,
		ReorderedPackets: f.reorderedPackets,
		DuplicatePackets: f.duplicatePackets,
		MaxPayloadBytes:  f.maxPayloadBytes,
		Over1200Packets:  f.over1200Packets,
		Over1400Packets:  f.over1400Packets,
		Over1472Packets:  f.over1472Packets,
	}

	denom := f.seqPackets + f.lossPackets
	if denom > 0 {
		s.LossPercent = (float64(f.lossPackets) / float64(denom)) * 100
	}
	return s
}

func ParseRakNetSequence(payload []byte) (uint32, bool) {
	if len(payload) < 4 {
		return 0, false
	}
	packetID := payload[0]
	if packetID < RakNetDataPacketMin || packetID > RakNetDataPacketMax {
		return 0, false
	}
	seq := uint32(payload[1]) | (uint32(payload[2]) << 8) | (uint32(payload[3]) << 16)
	return seq, true
}

func SeqForwardDistance(expected uint32, received uint32) uint32 {
	if received >= expected {
		return received - expected
	}
	return (RakNetSeqModulo - expected) + received
}

func FormatLossSummary(s FlowSnapshot) string {
	if s.SeqPackets == 0 {
		return "n/a"
	}
	expected := s.SeqPackets + s.LossPackets
	return fmt.Sprintf(
		"%.2f%%(%d/%d,reorder=%d,dup=%d)",
		s.LossPercent,
		s.LossPackets,
		expected,
		s.ReorderedPackets,
		s.DuplicatePackets,
	)
}

func FormatSizeRiskSummary(s FlowSnapshot) string {
	return fmt.Sprintf(
		"max=%d,>1200=%d,>1400=%d,>1472=%d",
		s.MaxPayloadBytes,
		s.Over1200Packets,
		s.Over1400Packets,
		s.Over1472Packets,
	)
}
