package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"road-proxy-v3/internal/udputil"
)

type Stats struct {
	startTime time.Time

	activeConnections atomic.Int64
	totalConnections  atomic.Int64
	totalBytesRx      atomic.Uint64
	totalBytesTx      atomic.Uint64
	errors            atomic.Int64

	udpMu sync.Mutex
	udpRX udputil.FlowMetrics
	udpTX udputil.FlowMetrics

	pluginsMu sync.Mutex
	plugins   map[string]*pluginStats

	nextSessionID atomic.Uint64
	sessionsMu    sync.Mutex
	sessions      map[string]*sessionStats
}

type pluginStats struct {
	activeConnections int64
	totalConnections  int64
	totalBytesRx      uint64
	totalBytesTx      uint64
	errors            int64
	udpRX             udputil.FlowMetrics
	udpTX             udputil.FlowMetrics
}

type SessionMeta struct {
	Plugin     string
	Transport  string
	Network    string
	RemoteAddr string
	TargetAddr string
}

type sessionStats struct {
	id         string
	plugin     string
	transport  string
	network    string
	remoteAddr string
	targetAddr string
	startedAt  time.Time
	lastSeenAt time.Time
	bytesRx    uint64
	bytesTx    uint64
	udpRX      udputil.FlowMetrics
	udpTX      udputil.FlowMetrics
}

type Snapshot struct {
	StartTime         string                         `json:"start_time"`
	UptimeSeconds     int64                          `json:"uptime_seconds"`
	ActiveConnections int64                          `json:"active_connections"`
	TotalConnections  int64                          `json:"total_connections"`
	TotalBytesRx      uint64                         `json:"total_bytes_rx"`
	TotalBytesTx      uint64                         `json:"total_bytes_tx"`
	Errors            int64                          `json:"errors"`
	UDP               UDPStatsSnapshot               `json:"udp"`
	Plugins           map[string]PluginStatsSnapshot `json:"plugins"`
	Sessions          []SessionSnapshot              `json:"sessions"`
}

type UDPStatsSnapshot struct {
	RX UDPFlowStatsSnapshot `json:"rx"`
	TX UDPFlowStatsSnapshot `json:"tx"`
}

type UDPFlowStatsSnapshot struct {
	Packets          uint64  `json:"packets"`
	Bytes            uint64  `json:"bytes"`
	JitterMS         float64 `json:"jitter_ms"`
	MaxGapMS         float64 `json:"max_gap_ms"`
	MaxPayloadBytes  uint64  `json:"max_payload_bytes"`
	Over1200Packets  uint64  `json:"packets_over_1200"`
	Over1400Packets  uint64  `json:"packets_over_1400"`
	Over1472Packets  uint64  `json:"packets_over_1472"`
	SeqPackets       uint64  `json:"seq_packets"`
	LossPackets      uint64  `json:"loss_packets"`
	LossPercent      float64 `json:"loss_percent"`
	ReorderedPackets uint64  `json:"reordered_packets"`
	DuplicatePackets uint64  `json:"duplicate_packets"`
}

type PluginStatsSnapshot struct {
	ActiveConnections int64            `json:"active_connections"`
	TotalConnections  int64            `json:"total_connections"`
	TotalBytesRx      uint64           `json:"total_bytes_rx"`
	TotalBytesTx      uint64           `json:"total_bytes_tx"`
	Errors            int64            `json:"errors"`
	UDP               UDPStatsSnapshot `json:"udp"`
}

type SessionSnapshot struct {
	ID          string           `json:"id"`
	Plugin      string           `json:"plugin"`
	Transport   string           `json:"transport"`
	Network     string           `json:"network"`
	RemoteAddr  string           `json:"remote_addr"`
	TargetAddr  string           `json:"target_addr"`
	StartedAt   string           `json:"started_at"`
	LastSeenAt  string           `json:"last_seen_at"`
	AgeSeconds  int64            `json:"age_seconds"`
	IdleSeconds int64            `json:"idle_seconds"`
	BytesRx     uint64           `json:"bytes_rx"`
	BytesTx     uint64           `json:"bytes_tx"`
	UDP         UDPStatsSnapshot `json:"udp"`
}

func NewStats() *Stats {
	return &Stats{
		startTime: time.Now(),
		plugins:   map[string]*pluginStats{},
		sessions:  map[string]*sessionStats{},
	}
}

func (s *Stats) SessionStart() {
	s.activeConnections.Add(1)
	s.totalConnections.Add(1)
}

func (s *Stats) SessionEnd() {
	s.activeConnections.Add(-1)
}

func (s *Stats) AddRx(bytes uint64) {
	s.totalBytesRx.Add(bytes)
}

func (s *Stats) AddTx(bytes uint64) {
	s.totalBytesTx.Add(bytes)
}

func (s *Stats) IncError() {
	s.errors.Add(1)
}

func (s *Stats) RegisterPlugin(pluginName string) {
	name := normalizePluginName(pluginName)
	if name == "" {
		return
	}

	s.pluginsMu.Lock()
	_ = s.pluginStatsLocked(name)
	s.pluginsMu.Unlock()
}

func (s *Stats) RegisterPlugins(pluginNames []string) {
	for _, name := range pluginNames {
		s.RegisterPlugin(name)
	}
}

func (s *Stats) SessionStartPlugin(pluginName string) {
	s.SessionStart()
	name := normalizePluginName(pluginName)
	if name == "" {
		return
	}

	s.pluginsMu.Lock()
	p := s.pluginStatsLocked(name)
	p.activeConnections++
	p.totalConnections++
	s.pluginsMu.Unlock()
}

func (s *Stats) SessionEndPlugin(pluginName string) {
	s.SessionEnd()
	name := normalizePluginName(pluginName)
	if name == "" {
		return
	}

	s.pluginsMu.Lock()
	p := s.pluginStatsLocked(name)
	if p.activeConnections > 0 {
		p.activeConnections--
	}
	s.pluginsMu.Unlock()
}

func (s *Stats) AddRxPlugin(pluginName string, bytes uint64) {
	s.AddRx(bytes)
	name := normalizePluginName(pluginName)
	if name == "" {
		return
	}

	s.pluginsMu.Lock()
	s.pluginStatsLocked(name).totalBytesRx += bytes
	s.pluginsMu.Unlock()
}

func (s *Stats) AddTxPlugin(pluginName string, bytes uint64) {
	s.AddTx(bytes)
	name := normalizePluginName(pluginName)
	if name == "" {
		return
	}

	s.pluginsMu.Lock()
	s.pluginStatsLocked(name).totalBytesTx += bytes
	s.pluginsMu.Unlock()
}

func (s *Stats) IncErrorPlugin(pluginName string) {
	s.IncError()
	name := normalizePluginName(pluginName)
	if name == "" {
		return
	}

	s.pluginsMu.Lock()
	s.pluginStatsLocked(name).errors++
	s.pluginsMu.Unlock()
}

func (s *Stats) StartSession(meta SessionMeta) string {
	pluginName := normalizePluginName(meta.Plugin)
	s.SessionStartPlugin(pluginName)

	now := time.Now()
	id := fmt.Sprintf("s-%d", s.nextSessionID.Add(1))

	s.sessionsMu.Lock()
	if s.sessions == nil {
		s.sessions = map[string]*sessionStats{}
	}
	s.sessions[id] = &sessionStats{
		id:         id,
		plugin:     pluginName,
		transport:  strings.TrimSpace(meta.Transport),
		network:    strings.TrimSpace(meta.Network),
		remoteAddr: strings.TrimSpace(meta.RemoteAddr),
		targetAddr: strings.TrimSpace(meta.TargetAddr),
		startedAt:  now,
		lastSeenAt: now,
	}
	s.sessionsMu.Unlock()

	return id
}

func (s *Stats) EndSession(id string) {
	pluginName := ""
	found := false

	s.sessionsMu.Lock()
	if session, ok := s.sessions[id]; ok {
		pluginName = session.plugin
		found = true
		delete(s.sessions, id)
	}
	s.sessionsMu.Unlock()

	if !found {
		return
	}
	if pluginName == "" {
		s.SessionEnd()
		return
	}
	s.SessionEndPlugin(pluginName)
}

func (s *Stats) AddSessionRx(id string, bytes uint64) {
	pluginName := s.updateSessionBytes(id, bytes, 0, time.Now())
	if pluginName == "" {
		s.AddRx(bytes)
		return
	}
	s.AddRxPlugin(pluginName, bytes)
}

func (s *Stats) AddSessionTx(id string, bytes uint64) {
	pluginName := s.updateSessionBytes(id, 0, bytes, time.Now())
	if pluginName == "" {
		s.AddTx(bytes)
		return
	}
	s.AddTxPlugin(pluginName, bytes)
}

func (s *Stats) ObserveSessionUDPRx(id string, ts time.Time, payload []byte) {
	pluginName := ""

	s.sessionsMu.Lock()
	if session, ok := s.sessions[id]; ok {
		session.lastSeenAt = ts
		session.udpRX.ObservePacket(ts, payload)
		pluginName = session.plugin
	}
	s.sessionsMu.Unlock()

	if pluginName == "" {
		s.ObserveUDPRx(ts, payload)
		return
	}
	s.ObserveUDPRxPlugin(pluginName, ts, payload)
}

func (s *Stats) ObserveSessionUDPTx(id string, ts time.Time, payload []byte) {
	pluginName := ""

	s.sessionsMu.Lock()
	if session, ok := s.sessions[id]; ok {
		session.lastSeenAt = ts
		session.udpTX.ObservePacket(ts, payload)
		pluginName = session.plugin
	}
	s.sessionsMu.Unlock()

	if pluginName == "" {
		s.ObserveUDPTx(ts, payload)
		return
	}
	s.ObserveUDPTxPlugin(pluginName, ts, payload)
}

func (s *Stats) ObserveUDPRx(ts time.Time, payload []byte) {
	s.udpMu.Lock()
	s.udpRX.ObservePacket(ts, payload)
	s.udpMu.Unlock()
}

func (s *Stats) ObserveUDPTx(ts time.Time, payload []byte) {
	s.udpMu.Lock()
	s.udpTX.ObservePacket(ts, payload)
	s.udpMu.Unlock()
}

func (s *Stats) ObserveUDPRxPlugin(pluginName string, ts time.Time, payload []byte) {
	s.ObserveUDPRx(ts, payload)
	name := normalizePluginName(pluginName)
	if name == "" {
		return
	}

	s.pluginsMu.Lock()
	s.pluginStatsLocked(name).udpRX.ObservePacket(ts, payload)
	s.pluginsMu.Unlock()
}

func (s *Stats) ObserveUDPTxPlugin(pluginName string, ts time.Time, payload []byte) {
	s.ObserveUDPTx(ts, payload)
	name := normalizePluginName(pluginName)
	if name == "" {
		return
	}

	s.pluginsMu.Lock()
	s.pluginStatsLocked(name).udpTX.ObservePacket(ts, payload)
	s.pluginsMu.Unlock()
}

func (s *Stats) Snapshot() Snapshot {
	s.udpMu.Lock()
	udp := UDPStatsSnapshot{
		RX: udpFlowSnapshotToStats(s.udpRX.Snapshot()),
		TX: udpFlowSnapshotToStats(s.udpTX.Snapshot()),
	}
	s.udpMu.Unlock()
	plugins := s.pluginSnapshots()
	sessions := s.SessionsSnapshot()

	return Snapshot{
		StartTime:         s.startTime.UTC().Format(time.RFC3339),
		UptimeSeconds:     int64(time.Since(s.startTime).Seconds()),
		ActiveConnections: s.activeConnections.Load(),
		TotalConnections:  s.totalConnections.Load(),
		TotalBytesRx:      s.totalBytesRx.Load(),
		TotalBytesTx:      s.totalBytesTx.Load(),
		Errors:            s.errors.Load(),
		UDP:               udp,
		Plugins:           plugins,
		Sessions:          sessions,
	}
}

func udpFlowSnapshotToStats(snapshot udputil.FlowSnapshot) UDPFlowStatsSnapshot {
	return UDPFlowStatsSnapshot{
		Packets:          snapshot.Packets,
		Bytes:            snapshot.Bytes,
		JitterMS:         durationMillis(snapshot.Jitter),
		MaxGapMS:         durationMillis(snapshot.MaxGap),
		MaxPayloadBytes:  snapshot.MaxPayloadBytes,
		Over1200Packets:  snapshot.Over1200Packets,
		Over1400Packets:  snapshot.Over1400Packets,
		Over1472Packets:  snapshot.Over1472Packets,
		SeqPackets:       snapshot.SeqPackets,
		LossPackets:      snapshot.LossPackets,
		LossPercent:      snapshot.LossPercent,
		ReorderedPackets: snapshot.ReorderedPackets,
		DuplicatePackets: snapshot.DuplicatePackets,
	}
}

func durationMillis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func (s *Stats) pluginStatsLocked(pluginName string) *pluginStats {
	if s.plugins == nil {
		s.plugins = map[string]*pluginStats{}
	}
	p, ok := s.plugins[pluginName]
	if ok {
		return p
	}
	p = &pluginStats{}
	s.plugins[pluginName] = p
	return p
}

func (s *Stats) pluginSnapshots() map[string]PluginStatsSnapshot {
	s.pluginsMu.Lock()
	defer s.pluginsMu.Unlock()

	out := make(map[string]PluginStatsSnapshot, len(s.plugins))
	for name, p := range s.plugins {
		out[name] = PluginStatsSnapshot{
			ActiveConnections: p.activeConnections,
			TotalConnections:  p.totalConnections,
			TotalBytesRx:      p.totalBytesRx,
			TotalBytesTx:      p.totalBytesTx,
			Errors:            p.errors,
			UDP: UDPStatsSnapshot{
				RX: udpFlowSnapshotToStats(p.udpRX.Snapshot()),
				TX: udpFlowSnapshotToStats(p.udpTX.Snapshot()),
			},
		}
	}
	return out
}

func (s *Stats) SessionsSnapshot() []SessionSnapshot {
	now := time.Now()

	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	out := make([]SessionSnapshot, 0, len(s.sessions))
	for _, session := range s.sessions {
		out = append(out, session.snapshot(now))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt == out[j].StartedAt {
			return out[i].ID < out[j].ID
		}
		return out[i].StartedAt < out[j].StartedAt
	})
	return out
}

func (s *Stats) updateSessionBytes(id string, bytesRx, bytesTx uint64, now time.Time) string {
	pluginName := ""

	s.sessionsMu.Lock()
	if session, ok := s.sessions[id]; ok {
		session.bytesRx += bytesRx
		session.bytesTx += bytesTx
		session.lastSeenAt = now
		pluginName = session.plugin
	}
	s.sessionsMu.Unlock()

	return pluginName
}

func (s *sessionStats) snapshot(now time.Time) SessionSnapshot {
	return SessionSnapshot{
		ID:          s.id,
		Plugin:      s.plugin,
		Transport:   s.transport,
		Network:     s.network,
		RemoteAddr:  s.remoteAddr,
		TargetAddr:  s.targetAddr,
		StartedAt:   s.startedAt.UTC().Format(time.RFC3339),
		LastSeenAt:  s.lastSeenAt.UTC().Format(time.RFC3339),
		AgeSeconds:  nonNegativeSeconds(now.Sub(s.startedAt)),
		IdleSeconds: nonNegativeSeconds(now.Sub(s.lastSeenAt)),
		BytesRx:     s.bytesRx,
		BytesTx:     s.bytesTx,
		UDP: UDPStatsSnapshot{
			RX: udpFlowSnapshotToStats(s.udpRX.Snapshot()),
			TX: udpFlowSnapshotToStats(s.udpTX.Snapshot()),
		},
	}
}

func nonNegativeSeconds(d time.Duration) int64 {
	if d < 0 {
		return 0
	}
	return int64(d.Seconds())
}

func normalizePluginName(pluginName string) string {
	return strings.TrimSpace(pluginName)
}
