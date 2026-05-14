package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"

	"road-proxy-v3/internal/udputil"
)

const (
	udpCheckMagic     = "ROADCHK1"
	udpCheckVersion   = 1
	udpCheckKindPing  = 1
	udpCheckKindAck   = 2
	udpCheckHeaderLen = 64
	udpCheckMaxPacket = 65507
)

type udpCheckPacket struct {
	Kind           byte
	PlayerID       uint32
	Sequence       uint64
	SentUnixNano   int64
	ServerUnixNano int64
	PayloadBytes   uint32
	TickRate       uint32
	State          uint64
}

type udpCheckSequenceStats struct {
	packets          uint64
	bytes            uint64
	uniquePackets    uint64
	duplicatePackets uint64
	reorderedPackets uint64
	lossPackets      uint64
	initialized      bool
	highestSeq       uint64
	seen             map[uint64]struct{}

	lastPacketAt time.Time
	lastGap      time.Duration
	maxGap       time.Duration
	jitterNanos  float64

	maxPayloadBytes uint64
	over1200Packets uint64
	over1400Packets uint64
	over1472Packets uint64
}

type udpCheckSequenceSnapshot struct {
	Packets          uint64
	Bytes            uint64
	UniquePackets    uint64
	DuplicatePackets uint64
	ReorderedPackets uint64
	LossPackets      uint64
	Jitter           time.Duration
	MaxGap           time.Duration
	MaxPayloadBytes  uint64
	Over1200Packets  uint64
	Over1400Packets  uint64
	Over1472Packets  uint64
}

type udpCheckRTTStats struct {
	samples []time.Duration
	total   time.Duration
	min     time.Duration
	max     time.Duration
	last    time.Duration
	jitter  float64
}

type udpCheckRTTSnapshot struct {
	Count  int
	Min    time.Duration
	Avg    time.Duration
	P95    time.Duration
	Max    time.Duration
	Jitter time.Duration
}

type udpCheckClientOptions struct {
	Target         string
	Players        int
	TickRate       int
	Duration       time.Duration
	PayloadBytes   int
	Grace          time.Duration
	ReportInterval time.Duration
	Out            io.Writer
}

type udpCheckClientPlayerResult struct {
	PlayerID uint32
	Sent     udpCheckSequenceSnapshot
	Ack      udpCheckSequenceSnapshot
	RTT      udpCheckRTTSnapshot
	Errors   uint64
}

type udpCheckClientResult struct {
	Target  string
	Players []udpCheckClientPlayerResult
}

type udpCheckServerOptions struct {
	Listen         string
	Duration       time.Duration
	BufferBytes    int
	ReportInterval time.Duration
	Out            io.Writer
	Ready          chan<- string
}

type udpCheckServerPeer struct {
	Key      string
	Address  string
	PlayerID uint32
	RX       udpCheckSequenceStats
	TX       udpCheckSequenceStats
	Errors   uint64
}

func runUDPCheckCommand(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: road-proxy udp-check <server|client> [flags]")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch args[0] {
	case "server":
		return runUDPCheckServerCommand(ctx, args[1:], out)
	case "client":
		return runUDPCheckClientCommand(ctx, args[1:], out)
	default:
		return fmt.Errorf("unknown udp-check mode %q; expected server or client", args[0])
	}
}

func runUDPCheckServerCommand(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("udp-check server", flag.ContinueOnError)
	fs.SetOutput(out)

	listen := fs.String("listen", "127.0.0.1:7777", "UDP listen address")
	durationRaw := fs.String("duration", "0", "test duration, 0 means until Ctrl+C")
	bufferBytes := fs.Int("buffer", udpCheckMaxPacket, "UDP read buffer bytes")
	reportRaw := fs.String("report-interval", "10s", "periodic report interval, 0 disables periodic reports")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected udp-check server args: %s", strings.Join(fs.Args(), " "))
	}
	duration, err := parseOptionalDuration(*durationRaw)
	if err != nil {
		return fmt.Errorf("invalid duration: %s", *durationRaw)
	}
	reportInterval, err := parseOptionalDuration(*reportRaw)
	if err != nil {
		return fmt.Errorf("invalid report-interval: %s", *reportRaw)
	}
	return runUDPCheckServer(ctx, udpCheckServerOptions{
		Listen:         *listen,
		Duration:       duration,
		BufferBytes:    *bufferBytes,
		ReportInterval: reportInterval,
		Out:            out,
	})
}

func runUDPCheckClientCommand(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("udp-check client", flag.ContinueOnError)
	fs.SetOutput(out)

	target := fs.String("target", "127.0.0.1:25568", "UDP target address, usually ROAD client listen address")
	players := fs.Int("players", 1, "simulated player/socket count")
	tickRate := fs.Int("tickrate", 30, "packets per second per simulated player")
	durationRaw := fs.String("duration", "30s", "send duration")
	payloadBytes := fs.Int("payload", 256, "UDP datagram payload bytes")
	graceRaw := fs.String("grace", "2s", "time to wait for late ACKs after sending stops")
	reportRaw := fs.String("report-interval", "0", "periodic report interval, 0 disables periodic reports")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected udp-check client args: %s", strings.Join(fs.Args(), " "))
	}
	duration, err := time.ParseDuration(*durationRaw)
	if err != nil || duration <= 0 {
		return fmt.Errorf("invalid duration: %s", *durationRaw)
	}
	grace, err := parseOptionalDuration(*graceRaw)
	if err != nil {
		return fmt.Errorf("invalid grace: %s", *graceRaw)
	}
	reportInterval, err := parseOptionalDuration(*reportRaw)
	if err != nil {
		return fmt.Errorf("invalid report-interval: %s", *reportRaw)
	}
	result, err := runUDPCheckClient(ctx, udpCheckClientOptions{
		Target:         *target,
		Players:        *players,
		TickRate:       *tickRate,
		Duration:       duration,
		PayloadBytes:   *payloadBytes,
		Grace:          grace,
		ReportInterval: reportInterval,
		Out:            out,
	})
	if err != nil {
		return err
	}
	writeUDPCheckClientSummary(out, "final", result)
	return nil
}

func runUDPCheckServer(ctx context.Context, opt udpCheckServerOptions) error {
	if opt.Out == nil {
		opt.Out = io.Discard
	}
	if opt.BufferBytes < udpCheckHeaderLen || opt.BufferBytes > udpCheckMaxPacket {
		return fmt.Errorf("buffer must be between %d and %d bytes", udpCheckHeaderLen, udpCheckMaxPacket)
	}
	if strings.TrimSpace(opt.Listen) == "" {
		return fmt.Errorf("listen address cannot be empty")
	}

	if opt.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opt.Duration)
		defer cancel()
	}

	conn, err := net.ListenPacket("udp", opt.Listen)
	if err != nil {
		return fmt.Errorf("udp-check listen %s: %w", opt.Listen, err)
	}
	defer conn.Close()
	if opt.Ready != nil {
		opt.Ready <- conn.LocalAddr().String()
	}

	fmt.Fprintf(opt.Out, "udp-check server started: listen=%s\n", conn.LocalAddr().String())

	buffer := make([]byte, opt.BufferBytes)
	peers := map[string]*udpCheckServerPeer{}
	lastReport := time.Now()

	for {
		if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			return err
		}
		n, addr, readErr := conn.ReadFrom(buffer)
		now := time.Now()
		if readErr != nil {
			if ctx.Err() != nil {
				writeUDPCheckServerSummary(opt.Out, "final", peers)
				return nil
			}
			if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
				if shouldUDPCheckReport(now, &lastReport, opt.ReportInterval) {
					writeUDPCheckServerSummary(opt.Out, "periodic", peers)
				}
				continue
			}
			return readErr
		}
		if n == 0 {
			continue
		}

		packet, err := decodeUDPCheckPacket(buffer[:n])
		if err != nil || packet.Kind != udpCheckKindPing {
			continue
		}
		key := fmt.Sprintf("%s/p%d", addr.String(), packet.PlayerID)
		peer := peers[key]
		if peer == nil {
			peer = &udpCheckServerPeer{
				Key:      key,
				Address:  addr.String(),
				PlayerID: packet.PlayerID,
			}
			peers[key] = peer
		}
		peer.RX.Observe(now, packet.Sequence, n)

		packet.Kind = udpCheckKindAck
		packet.ServerUnixNano = now.UnixNano()
		ack := encodeUDPCheckPacket(packet, n)
		if _, err := conn.WriteTo(ack, addr); err != nil {
			peer.Errors++
			continue
		}
		peer.TX.Observe(time.Now(), packet.Sequence, len(ack))

		if shouldUDPCheckReport(now, &lastReport, opt.ReportInterval) {
			writeUDPCheckServerSummary(opt.Out, "periodic", peers)
		}
	}
}

func runUDPCheckClient(ctx context.Context, opt udpCheckClientOptions) (udpCheckClientResult, error) {
	if opt.Out == nil {
		opt.Out = io.Discard
	}
	if strings.TrimSpace(opt.Target) == "" {
		return udpCheckClientResult{}, fmt.Errorf("target address cannot be empty")
	}
	if opt.Players < 1 {
		return udpCheckClientResult{}, fmt.Errorf("players must be >= 1")
	}
	if opt.TickRate < 1 {
		return udpCheckClientResult{}, fmt.Errorf("tickrate must be >= 1")
	}
	if opt.Duration <= 0 {
		return udpCheckClientResult{}, fmt.Errorf("duration must be > 0")
	}
	if opt.PayloadBytes < udpCheckHeaderLen || opt.PayloadBytes > udpCheckMaxPacket {
		return udpCheckClientResult{}, fmt.Errorf("payload must be between %d and %d bytes", udpCheckHeaderLen, udpCheckMaxPacket)
	}
	if opt.Grace < 0 {
		return udpCheckClientResult{}, fmt.Errorf("grace must be >= 0")
	}

	targetAddr, err := net.ResolveUDPAddr("udp", opt.Target)
	if err != nil {
		return udpCheckClientResult{}, err
	}

	results := make([]udpCheckClientPlayerResult, opt.Players)
	errCh := make(chan error, opt.Players)
	var wg sync.WaitGroup

	for player := 1; player <= opt.Players; player++ {
		player := player
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := runUDPCheckClientPlayer(ctx, uint32(player), targetAddr, opt)
			results[player-1] = result
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)
	if err := <-errCh; err != nil {
		return udpCheckClientResult{}, err
	}

	return udpCheckClientResult{
		Target:  opt.Target,
		Players: results,
	}, nil
}

func runUDPCheckClientPlayer(
	ctx context.Context,
	playerID uint32,
	targetAddr *net.UDPAddr,
	opt udpCheckClientOptions,
) (udpCheckClientPlayerResult, error) {
	conn, err := net.DialUDP("udp", nil, targetAddr)
	if err != nil {
		return udpCheckClientPlayerResult{}, err
	}
	defer conn.Close()

	type state struct {
		mu      sync.Mutex
		sent    map[uint64]time.Time
		sentSeq udpCheckSequenceStats
		ackSeq  udpCheckSequenceStats
		rtt     udpCheckRTTStats
		errors  uint64
	}
	st := &state{
		sent: map[uint64]time.Time{},
	}

	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buffer := make([]byte, udpCheckMaxPacket)
		for {
			if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
				return
			}
			n, err := conn.Read(buffer)
			now := time.Now()
			if err != nil {
				if readCtx.Err() != nil {
					return
				}
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				st.mu.Lock()
				st.errors++
				st.mu.Unlock()
				continue
			}
			packet, err := decodeUDPCheckPacket(buffer[:n])
			if err != nil || packet.Kind != udpCheckKindAck || packet.PlayerID != playerID {
				continue
			}

			st.mu.Lock()
			sentAt, ok := st.sent[packet.Sequence]
			if ok {
				st.rtt.Observe(now.Sub(sentAt))
			}
			st.ackSeq.Observe(now, packet.Sequence, n)
			st.mu.Unlock()
		}
	}()

	interval := time.Second / time.Duration(opt.TickRate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	deadline := time.NewTimer(opt.Duration)
	defer deadline.Stop()

	send := func(seq uint64) error {
		now := time.Now()
		packet := udpCheckPacket{
			Kind:         udpCheckKindPing,
			PlayerID:     playerID,
			Sequence:     seq,
			SentUnixNano: now.UnixNano(),
			PayloadBytes: uint32(opt.PayloadBytes),
			TickRate:     uint32(opt.TickRate),
			State:        (uint64(playerID) << 48) ^ seq,
		}
		payload := encodeUDPCheckPacket(packet, opt.PayloadBytes)
		if _, err := conn.Write(payload); err != nil {
			return err
		}
		st.mu.Lock()
		st.sent[seq] = now
		st.sentSeq.Observe(now, seq, len(payload))
		st.mu.Unlock()
		return nil
	}

	var seq uint64 = 1
	if err := send(seq); err != nil {
		return udpCheckClientPlayerResult{}, err
	}
	seq++

sendLoop:
	for {
		select {
		case <-ctx.Done():
			break sendLoop
		case <-deadline.C:
			break sendLoop
		case <-ticker.C:
			if err := send(seq); err != nil {
				st.mu.Lock()
				st.errors++
				st.mu.Unlock()
			}
			seq++
		}
	}

	if opt.Grace > 0 {
		timer := time.NewTimer(opt.Grace)
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
		timer.Stop()
	}
	cancelRead()
	_ = conn.Close()
	<-readDone

	st.mu.Lock()
	defer st.mu.Unlock()
	return udpCheckClientPlayerResult{
		PlayerID: playerID,
		Sent:     st.sentSeq.Snapshot(),
		Ack:      st.ackSeq.Snapshot(),
		RTT:      st.rtt.Snapshot(),
		Errors:   st.errors,
	}, nil
}

func encodeUDPCheckPacket(packet udpCheckPacket, payloadBytes int) []byte {
	if payloadBytes < udpCheckHeaderLen {
		payloadBytes = udpCheckHeaderLen
	}
	buf := make([]byte, payloadBytes)
	copy(buf[0:8], udpCheckMagic)
	buf[8] = udpCheckVersion
	buf[9] = packet.Kind
	binary.BigEndian.PutUint16(buf[10:12], 0)
	binary.BigEndian.PutUint32(buf[12:16], packet.PlayerID)
	binary.BigEndian.PutUint64(buf[16:24], packet.Sequence)
	binary.BigEndian.PutUint64(buf[24:32], uint64(packet.SentUnixNano))
	binary.BigEndian.PutUint64(buf[32:40], uint64(packet.ServerUnixNano))
	binary.BigEndian.PutUint32(buf[40:44], uint32(payloadBytes))
	binary.BigEndian.PutUint32(buf[44:48], packet.TickRate)
	binary.BigEndian.PutUint64(buf[48:56], packet.State)
	for i := udpCheckHeaderLen; i < len(buf); i++ {
		buf[i] = byte((uint64(i) + packet.Sequence*31 + uint64(packet.PlayerID)*17) & 0xff)
	}
	return buf
}

func decodeUDPCheckPacket(data []byte) (udpCheckPacket, error) {
	if len(data) < udpCheckHeaderLen {
		return udpCheckPacket{}, fmt.Errorf("udp-check packet too short: %d", len(data))
	}
	if string(data[0:8]) != udpCheckMagic {
		return udpCheckPacket{}, fmt.Errorf("udp-check magic mismatch")
	}
	if data[8] != udpCheckVersion {
		return udpCheckPacket{}, fmt.Errorf("udp-check version mismatch: %d", data[8])
	}
	return udpCheckPacket{
		Kind:           data[9],
		PlayerID:       binary.BigEndian.Uint32(data[12:16]),
		Sequence:       binary.BigEndian.Uint64(data[16:24]),
		SentUnixNano:   int64(binary.BigEndian.Uint64(data[24:32])),
		ServerUnixNano: int64(binary.BigEndian.Uint64(data[32:40])),
		PayloadBytes:   binary.BigEndian.Uint32(data[40:44]),
		TickRate:       binary.BigEndian.Uint32(data[44:48]),
		State:          binary.BigEndian.Uint64(data[48:56]),
	}, nil
}

func (s *udpCheckSequenceStats) Observe(ts time.Time, seq uint64, payloadBytes int) {
	if s.seen == nil {
		s.seen = map[uint64]struct{}{}
	}
	s.packets++
	s.bytes += uint64(payloadBytes)
	if uint64(payloadBytes) > s.maxPayloadBytes {
		s.maxPayloadBytes = uint64(payloadBytes)
	}
	if udputil.AboveConservativeMTU(payloadBytes) {
		s.over1200Packets++
	}
	if udputil.AboveTunnelHOLRisk(payloadBytes) {
		s.over1400Packets++
	}
	if udputil.AboveIPv4UDPFragmentRisk(payloadBytes) {
		s.over1472Packets++
	}
	if !s.lastPacketAt.IsZero() {
		gap := ts.Sub(s.lastPacketAt)
		if gap < 0 {
			gap = 0
		}
		if s.lastGap > 0 {
			diff := gap - s.lastGap
			if diff < 0 {
				diff = -diff
			}
			if s.jitterNanos == 0 {
				s.jitterNanos = float64(diff)
			} else {
				s.jitterNanos += (float64(diff) - s.jitterNanos) / 16.0
			}
		}
		s.lastGap = gap
		if gap > s.maxGap {
			s.maxGap = gap
		}
	}
	s.lastPacketAt = ts

	if _, ok := s.seen[seq]; ok {
		s.duplicatePackets++
		return
	}
	s.seen[seq] = struct{}{}
	s.uniquePackets++

	if !s.initialized {
		s.initialized = true
		s.highestSeq = seq
		return
	}
	if seq > s.highestSeq {
		if seq > s.highestSeq+1 {
			s.lossPackets += seq - s.highestSeq - 1
		}
		s.highestSeq = seq
		return
	}
	s.reorderedPackets++
}

func (s *udpCheckSequenceStats) Snapshot() udpCheckSequenceSnapshot {
	return udpCheckSequenceSnapshot{
		Packets:          s.packets,
		Bytes:            s.bytes,
		UniquePackets:    s.uniquePackets,
		DuplicatePackets: s.duplicatePackets,
		ReorderedPackets: s.reorderedPackets,
		LossPackets:      s.lossPackets,
		Jitter:           time.Duration(s.jitterNanos),
		MaxGap:           s.maxGap,
		MaxPayloadBytes:  s.maxPayloadBytes,
		Over1200Packets:  s.over1200Packets,
		Over1400Packets:  s.over1400Packets,
		Over1472Packets:  s.over1472Packets,
	}
}

func (r *udpCheckRTTStats) Observe(sample time.Duration) {
	if sample < 0 {
		return
	}
	if len(r.samples) == 0 || sample < r.min {
		r.min = sample
	}
	if sample > r.max {
		r.max = sample
	}
	if len(r.samples) > 0 {
		diff := sample - r.last
		if diff < 0 {
			diff = -diff
		}
		if r.jitter == 0 {
			r.jitter = float64(diff)
		} else {
			r.jitter += (float64(diff) - r.jitter) / 16.0
		}
	}
	r.last = sample
	r.samples = append(r.samples, sample)
	r.total += sample
}

func (r *udpCheckRTTStats) Snapshot() udpCheckRTTSnapshot {
	if len(r.samples) == 0 {
		return udpCheckRTTSnapshot{}
	}
	sorted := append([]time.Duration(nil), r.samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	p95Index := int(float64(len(sorted)-1) * 0.95)
	return udpCheckRTTSnapshot{
		Count:  len(r.samples),
		Min:    r.min,
		Avg:    r.total / time.Duration(len(r.samples)),
		P95:    sorted[p95Index],
		Max:    r.max,
		Jitter: time.Duration(r.jitter),
	}
}

func writeUDPCheckClientSummary(out io.Writer, reason string, result udpCheckClientResult) {
	fmt.Fprintf(out, "udp-check client summary [%s] target=%s players=%d\n", reason, result.Target, len(result.Players))
	var totalSent, totalAck, totalBytes, totalMissing, totalErrors uint64
	for _, player := range result.Players {
		missing := player.Sent.UniquePackets - minUint64(player.Sent.UniquePackets, player.Ack.UniquePackets)
		totalSent += player.Sent.UniquePackets
		totalAck += player.Ack.UniquePackets
		totalBytes += player.Sent.Bytes
		totalMissing += missing
		totalErrors += player.Errors
		lossPercent := percent(missing, player.Sent.UniquePackets)
		fmt.Fprintf(
			out,
			"player=%d sent=%d ack=%d missing=%d loss=%.2f%% dup=%d reorder=%d rtt_min=%s rtt_avg=%s rtt_p95=%s rtt_max=%s rtt_jitter=%s max_gap=%s size=max=%d,>1200=%d,>1400=%d,>1472=%d errors=%d\n",
			player.PlayerID,
			player.Sent.UniquePackets,
			player.Ack.UniquePackets,
			missing,
			lossPercent,
			player.Ack.DuplicatePackets,
			player.Ack.ReorderedPackets,
			formatCheckDuration(player.RTT.Min),
			formatCheckDuration(player.RTT.Avg),
			formatCheckDuration(player.RTT.P95),
			formatCheckDuration(player.RTT.Max),
			formatCheckDuration(player.RTT.Jitter),
			formatCheckDuration(player.Ack.MaxGap),
			player.Sent.MaxPayloadBytes,
			player.Sent.Over1200Packets,
			player.Sent.Over1400Packets,
			player.Sent.Over1472Packets,
			player.Errors,
		)
	}
	fmt.Fprintf(
		out,
		"total sent=%d ack=%d missing=%d loss=%.2f%% bytes=%d errors=%d\n",
		totalSent,
		totalAck,
		totalMissing,
		percent(totalMissing, totalSent),
		totalBytes,
		totalErrors,
	)
}

func writeUDPCheckServerSummary(out io.Writer, reason string, peers map[string]*udpCheckServerPeer) {
	fmt.Fprintf(out, "udp-check server summary [%s] peers=%d\n", reason, len(peers))
	keys := make([]string, 0, len(peers))
	for key := range peers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		peer := peers[key]
		rx := peer.RX.Snapshot()
		tx := peer.TX.Snapshot()
		fmt.Fprintf(
			out,
			"peer=%s player=%d rx=%dpkts/%dB tx=%dpkts/%dB rx_gap=%s rx_jitter=%s rx_loss_seq=%d rx_dup=%d rx_reorder=%d size=max=%d,>1200=%d,>1400=%d,>1472=%d errors=%d\n",
			peer.Address,
			peer.PlayerID,
			rx.UniquePackets,
			rx.Bytes,
			tx.UniquePackets,
			tx.Bytes,
			formatCheckDuration(rx.MaxGap),
			formatCheckDuration(rx.Jitter),
			rx.LossPackets,
			rx.DuplicatePackets,
			rx.ReorderedPackets,
			rx.MaxPayloadBytes,
			rx.Over1200Packets,
			rx.Over1400Packets,
			rx.Over1472Packets,
			peer.Errors,
		)
	}
}

func parseOptionalDuration(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return 0, nil
	}
	return time.ParseDuration(raw)
}

func shouldUDPCheckReport(now time.Time, lastReport *time.Time, interval time.Duration) bool {
	if interval <= 0 {
		return false
	}
	if now.Sub(*lastReport) < interval {
		return false
	}
	*lastReport = now
	return true
}

func formatCheckDuration(d time.Duration) string {
	return d.Truncate(time.Microsecond).String()
}

func percent(part, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return (float64(part) / float64(total)) * 100
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
