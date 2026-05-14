package client

import (
	"time"

	"road-proxy-v3/internal/udputil"
)

type udpFlowMetrics = udputil.FlowMetrics

type udpSessionMetrics struct {
	startedAt    time.Time
	lastReportAt time.Time
	tx           udpFlowMetrics
	rx           udpFlowMetrics
}

type udpFlowMetricsSnapshot = udputil.FlowSnapshot

type udpSessionMetricsSnapshot struct {
	Age time.Duration
	TX  udpFlowMetricsSnapshot
	RX  udpFlowMetricsSnapshot
}

func newUDPSessionMetrics(now time.Time) udpSessionMetrics {
	return udpSessionMetrics{
		startedAt:    now,
		lastReportAt: now,
	}
}

func (m *udpSessionMetrics) observeTX(ts time.Time, payload []byte) {
	m.tx.ObservePacket(ts, payload)
}

func (m *udpSessionMetrics) observeRX(ts time.Time, payload []byte) {
	m.rx.ObservePacket(ts, payload)
}

func (m *udpSessionMetrics) shouldReport(now time.Time, interval time.Duration) bool {
	if interval <= 0 {
		return false
	}
	return now.Sub(m.lastReportAt) >= interval
}

func (m *udpSessionMetrics) snapshot(now time.Time) udpSessionMetricsSnapshot {
	return udpSessionMetricsSnapshot{
		Age: now.Sub(m.startedAt),
		TX:  m.tx.Snapshot(),
		RX:  m.rx.Snapshot(),
	}
}

func parseRakNetSequence(payload []byte) (uint32, bool) {
	return udputil.ParseRakNetSequence(payload)
}

func seqForwardDistance(expected uint32, received uint32) uint32 {
	return udputil.SeqForwardDistance(expected, received)
}

func formatUDPLossSummary(s udpFlowMetricsSnapshot) string {
	return udputil.FormatLossSummary(s)
}

func formatUDPSizeSummary(s udpFlowMetricsSnapshot) string {
	return udputil.FormatSizeRiskSummary(s)
}
