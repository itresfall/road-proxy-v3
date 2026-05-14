package e2e

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"road-proxy-v3/internal/client"
	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/engine"
)

const (
	stressUpdateMagic   uint32 = 0x55504431 // "UPD1"
	stressSnapshotMagic uint32 = 0x53594E31 // "SYN1"

	stressUpdateHeaderSize  = 4 + 2 + 4 + 2 + 2 + 2
	stressUpdatePayloadSize = 640
	stressUpdatePacketSize  = stressUpdateHeaderSize + stressUpdatePayloadSize

	stressSnapshotHeaderSize = 4 + 4 + 2 + 2
	stressSnapshotEntrySize  = 2 + 4 + 4 + 2 + 2 + 2
	stressSnapshotTailSize   = 700
)

type stressState struct {
	clientID uint16
	addr     net.Addr
	seq      uint32
	x        int16
	y        int16
	z        int16
	lastSeen time.Time
}

func TestUDPWebSocketSyncFiveClientsStress(t *testing.T) {
	stressDuration := stressDurationFromEnv(8 * time.Second)
	minFullRatio := stressMinFullRatioFromEnv(0.85)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	udpAddrs := mustUniqueUDPAddrs(t, 6)
	targetAddr := udpAddrs[0]
	clientListenAddrs := []string{
		udpAddrs[1],
		udpAddrs[2],
		udpAddrs[3],
		udpAddrs[4],
		udpAddrs[5],
	}
	tcpAddrs := mustUniqueTCPAddrs(t, 3)
	serverTCPAddr := tcpAddrs[0]
	serverWSAddr := tcpAddrs[1]
	serverControlAddr := tcpAddrs[2]

	targetErrCh := make(chan error, 1)
	go func() {
		targetErrCh <- runUDPStressSyncTarget(ctx, targetAddr)
	}()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins")
	mustWritePluginSchema(t, filepath.Join(pluginRoot, "udp-sync-stress"), "udp-sync-stress", "udp", targetAddr)

	serverCfg := config.Default()
	serverCfg.TCP.ListenAddr = serverTCPAddr
	serverCfg.HTTP.ListenAddr = serverWSAddr
	serverCfg.Control.ListenAddr = serverControlAddr
	serverCfg.Plugins.Dir = pluginRoot
	serverCfg.Plugins.Enabled = []string{"udp-sync-stress"}

	server := engine.New(serverCfg, log.New(io.Discard, "", 0))
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.Start(ctx)
	}()

	select {
	case err := <-serverErrCh:
		t.Fatalf("server exited before ready: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	waitPort(t, serverWSAddr, 8*time.Second)

	clientErrCh := make([]chan error, 0, len(clientListenAddrs))
	for _, addr := range clientListenAddrs {
		clientCfg := config.DefaultClient()
		clientCfg.ListenNetwork = "udp"
		clientCfg.ListenAddr = addr
		clientCfg.ServerWSURL = fmt.Sprintf("ws://%s/ws", serverWSAddr)
		clientCfg.PluginName = "udp-sync-stress"
		clientCfg.ConnectRetries = 6
		clientCfg.RetryInitialDelay = "100ms"
		clientCfg.RetryMaxDelay = "600ms"
		clientCfg.UDPMetricsLog = "0s"

		tunnel := client.New(clientCfg, log.New(io.Discard, "", 0))
		errCh := make(chan error, 1)
		clientErrCh = append(clientErrCh, errCh)
		go func(ch chan error) {
			ch <- tunnel.Start(ctx)
		}(errCh)
	}

	// Let all local UDP listeners and WS sessions settle.
	time.Sleep(300 * time.Millisecond)

	var wg sync.WaitGroup
	errs := make(chan error, len(clientListenAddrs))

	for i, localAddr := range clientListenAddrs {
		wg.Add(1)
		go func(botID int, proxyLocalAddr string) {
			defer wg.Done()
			clientID := uint16(botID + 1)
			if err := runUDPStressBot(
				ctx,
				proxyLocalAddr,
				clientID,
				len(clientListenAddrs),
				stressDuration,
				minFullRatio,
			); err != nil {
				errs <- fmt.Errorf("bot %d failed: %w", botID+1, err)
				return
			}
		}(i, localAddr)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	cancel()

	waitNoError(t, serverErrCh, 5*time.Second, "server")
	for i, ch := range clientErrCh {
		waitNoError(t, ch, 5*time.Second, fmt.Sprintf("client-%d", i+1))
	}
	waitNoError(t, targetErrCh, 5*time.Second, "target")
}

func runUDPStressSyncTarget(ctx context.Context, addr string) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	states := map[uint16]stressState{}
	var mu sync.RWMutex

	errCh := make(chan error, 2)

	go func() {
		buf := make([]byte, 2048)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, remoteAddr, readErr := conn.ReadFrom(buf)
			if n > 0 {
				clientID, seq, x, y, z, ok := parseStressUpdatePacket(buf[:n])
				if ok {
					mu.Lock()
					states[clientID] = stressState{
						clientID: clientID,
						addr:     remoteAddr,
						seq:      seq,
						x:        x,
						y:        y,
						z:        z,
						lastSeen: time.Now(),
					}
					mu.Unlock()
				}
			}
			if readErr != nil {
				if ctx.Err() != nil {
					errCh <- nil
					return
				}
				if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
					continue
				}
				errCh <- readErr
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(20 * time.Millisecond) // 50Hz broadcast
		defer ticker.Stop()

		var tick uint32
		for {
			select {
			case <-ctx.Done():
				errCh <- nil
				return
			case <-ticker.C:
				now := time.Now()
				tick++

				mu.RLock()
				active := make([]stressState, 0, len(states))
				for _, st := range states {
					if now.Sub(st.lastSeen) <= 3*time.Second {
						active = append(active, st)
					}
				}
				mu.RUnlock()

				if len(active) == 0 {
					continue
				}
				sort.Slice(active, func(i, j int) bool {
					return active[i].clientID < active[j].clientID
				})

				packet := buildStressSnapshotPacket(tick, now, active)
				for _, st := range active {
					if _, err := conn.WriteTo(packet, st.addr); err != nil {
						if ctx.Err() != nil {
							errCh <- nil
							return
						}
						errCh <- err
						return
					}
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func runUDPStressBot(
	ctx context.Context,
	proxyLocalAddr string,
	clientID uint16,
	expectedPlayers int,
	testWindow time.Duration,
	minFullRatio float64,
) error {
	if testWindow < 2*time.Second {
		testWindow = 2 * time.Second
	}
	if minFullRatio <= 0 || minFullRatio > 1 {
		minFullRatio = 0.85
	}

	conn, err := net.DialTimeout("udp", proxyLocalAddr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("dial proxy local udp %s: %w", proxyLocalAddr, err)
	}
	defer conn.Close()

	updatesDone := make(chan struct{})
	defer close(updatesDone)

	go func() {
		ticker := time.NewTicker(15 * time.Millisecond) // ~66 pps update stream
		defer ticker.Stop()

		var seq uint32
		for {
			select {
			case <-ctx.Done():
				return
			case <-updatesDone:
				return
			case <-ticker.C:
				seq++
				packet := buildStressUpdatePacket(clientID, seq)
				_ = conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
				_, _ = conn.Write(packet)
			}
		}
	}()

	startAt := time.Now()
	deadline := startAt.Add(testWindow)
	warmup := testWindow / 5
	if warmup < 1500*time.Millisecond {
		warmup = 1500 * time.Millisecond
	}
	if warmup > 4*time.Second {
		warmup = 4 * time.Second
	}

	observed := map[uint16]struct{}{}
	var totalAfterWarmup int
	var fullAfterWarmup int
	var maxSnapshotGap time.Duration
	var lastSnapshotAt time.Time
	buf := make([]byte, 4096)

	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, readErr := conn.Read(buf)
		if readErr != nil {
			if isRetryableUDPReadError(readErr) {
				time.Sleep(30 * time.Millisecond)
				continue
			}
			return fmt.Errorf("read snapshot: %w", readErr)
		}

		ids, ok := parseStressSnapshotPacketClientIDs(buf[:n])
		if !ok {
			continue
		}

		now := time.Now()
		if !lastSnapshotAt.IsZero() {
			gap := now.Sub(lastSnapshotAt)
			if gap > maxSnapshotGap {
				maxSnapshotGap = gap
			}
		}
		lastSnapshotAt = now

		for _, id := range ids {
			observed[id] = struct{}{}
		}

		if now.Sub(startAt) >= warmup {
			totalAfterWarmup++
			if countUniqueClientIDs(ids) >= expectedPlayers {
				fullAfterWarmup++
			}
		}
	}

	if len(observed) < expectedPlayers {
		return fmt.Errorf("timeout waiting for %d players, observed=%d", expectedPlayers, len(observed))
	}
	if totalAfterWarmup == 0 {
		return fmt.Errorf("no snapshots captured after warmup")
	}

	fullRatio := float64(fullAfterWarmup) / float64(totalAfterWarmup)
	if fullRatio < minFullRatio {
		return fmt.Errorf(
			"sync quality too low: full_ratio=%.2f min=%.2f (full=%d total=%d)",
			fullRatio,
			minFullRatio,
			fullAfterWarmup,
			totalAfterWarmup,
		)
	}

	if maxSnapshotGap > 1500*time.Millisecond {
		return fmt.Errorf("snapshot gaps too large: max_gap=%s", maxSnapshotGap)
	}

	return nil
}

func buildStressUpdatePacket(clientID uint16, seq uint32) []byte {
	packet := make([]byte, stressUpdatePacketSize)
	binary.BigEndian.PutUint32(packet[0:4], stressUpdateMagic)
	binary.BigEndian.PutUint16(packet[4:6], clientID)
	binary.BigEndian.PutUint32(packet[6:10], seq)

	// Deterministic movement pattern + medium payload to stress serialization and WS framing.
	x := int16((int(seq)*17 + int(clientID)*11) % 30000)
	y := int16((int(seq)*23 + int(clientID)*7) % 30000)
	z := int16((int(seq)*13 + int(clientID)*5) % 30000)
	binary.BigEndian.PutUint16(packet[10:12], uint16(x))
	binary.BigEndian.PutUint16(packet[12:14], uint16(y))
	binary.BigEndian.PutUint16(packet[14:16], uint16(z))

	for i := stressUpdateHeaderSize; i < len(packet); i++ {
		packet[i] = byte((int(clientID) + int(seq) + i) % 251)
	}
	return packet
}

func parseStressUpdatePacket(packet []byte) (clientID uint16, seq uint32, x, y, z int16, ok bool) {
	if len(packet) < stressUpdateHeaderSize {
		return 0, 0, 0, 0, 0, false
	}
	if binary.BigEndian.Uint32(packet[0:4]) != stressUpdateMagic {
		return 0, 0, 0, 0, 0, false
	}
	clientID = binary.BigEndian.Uint16(packet[4:6])
	seq = binary.BigEndian.Uint32(packet[6:10])
	x = int16(binary.BigEndian.Uint16(packet[10:12]))
	y = int16(binary.BigEndian.Uint16(packet[12:14]))
	z = int16(binary.BigEndian.Uint16(packet[14:16]))
	if clientID == 0 {
		return 0, 0, 0, 0, 0, false
	}
	return clientID, seq, x, y, z, true
}

func buildStressSnapshotPacket(tick uint32, now time.Time, active []stressState) []byte {
	count := len(active)
	packetSize := stressSnapshotHeaderSize + (count * stressSnapshotEntrySize) + stressSnapshotTailSize
	packet := make([]byte, packetSize)

	binary.BigEndian.PutUint32(packet[0:4], stressSnapshotMagic)
	binary.BigEndian.PutUint32(packet[4:8], tick)
	binary.BigEndian.PutUint16(packet[8:10], uint16(count))
	// [10:12] reserved

	offset := stressSnapshotHeaderSize
	for _, st := range active {
		ageMs := now.Sub(st.lastSeen).Milliseconds()
		if ageMs < 0 {
			ageMs = 0
		}
		if ageMs > int64(^uint32(0)) {
			ageMs = int64(^uint32(0))
		}

		binary.BigEndian.PutUint16(packet[offset:offset+2], st.clientID)
		offset += 2
		binary.BigEndian.PutUint32(packet[offset:offset+4], st.seq)
		offset += 4
		binary.BigEndian.PutUint32(packet[offset:offset+4], uint32(ageMs))
		offset += 4
		binary.BigEndian.PutUint16(packet[offset:offset+2], uint16(st.x))
		offset += 2
		binary.BigEndian.PutUint16(packet[offset:offset+2], uint16(st.y))
		offset += 2
		binary.BigEndian.PutUint16(packet[offset:offset+2], uint16(st.z))
		offset += 2
	}

	for i := offset; i < len(packet); i++ {
		packet[i] = byte((int(tick) + i) % 251)
	}

	return packet
}

func parseStressSnapshotPacketClientIDs(packet []byte) ([]uint16, bool) {
	if len(packet) < stressSnapshotHeaderSize {
		return nil, false
	}
	if binary.BigEndian.Uint32(packet[0:4]) != stressSnapshotMagic {
		return nil, false
	}
	count := int(binary.BigEndian.Uint16(packet[8:10]))
	required := stressSnapshotHeaderSize + (count * stressSnapshotEntrySize)
	if count < 0 || len(packet) < required {
		return nil, false
	}

	ids := make([]uint16, 0, count)
	offset := stressSnapshotHeaderSize
	for i := 0; i < count; i++ {
		id := binary.BigEndian.Uint16(packet[offset : offset+2])
		offset += stressSnapshotEntrySize
		if id != 0 {
			ids = append(ids, id)
		}
	}
	return ids, true
}

func countUniqueClientIDs(ids []uint16) int {
	if len(ids) == 0 {
		return 0
	}
	seen := make(map[uint16]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		seen[id] = struct{}{}
	}
	return len(seen)
}

func stressDurationFromEnv(defaultValue time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv("ROAD_SYNC_STRESS_DURATION"))
	if raw == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultValue
	}
	return d
}

func stressMinFullRatioFromEnv(defaultValue float64) float64 {
	raw := strings.TrimSpace(os.Getenv("ROAD_SYNC_STRESS_MIN_FULL_RATIO"))
	if raw == "" {
		return defaultValue
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 || v > 1 {
		return defaultValue
	}
	return v
}

func TestStressPluginSchemaExists(t *testing.T) {
	path := filepath.Join("..", "..", "plugins", "udp-sync-stress", "plugin.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stress plugin schema missing: %v", err)
	}
}
