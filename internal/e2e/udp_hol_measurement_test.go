package e2e

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"road-proxy-v3/internal/client"
	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/engine"
)

const (
	holBulkPackets     = 16
	holBulkPayloadSize = 8 * 1024
)

func TestUDPWebSocketHOLMeasurement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	udpAddrs := mustUniqueUDPAddrs(t, 2)
	targetAddr := udpAddrs[0]
	clientListenAddr := udpAddrs[1]
	tcpAddrs := mustUniqueTCPAddrs(t, 3)
	serverTCPAddr := tcpAddrs[0]
	serverWSAddr := tcpAddrs[1]
	serverControlAddr := tcpAddrs[2]

	targetErrCh := make(chan error, 1)
	go func() {
		targetErrCh <- runUDPHOLTarget(ctx, targetAddr, holBulkPackets, holBulkPayloadSize)
	}()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins")
	mustWritePluginSchema(t, filepath.Join(pluginRoot, "udp-hol"), "udp-hol", "udp", targetAddr)

	serverCfg := config.Default()
	serverCfg.TCP.ListenAddr = serverTCPAddr
	serverCfg.HTTP.ListenAddr = serverWSAddr
	serverCfg.Control.ListenAddr = serverControlAddr
	serverCfg.Plugins.Dir = pluginRoot
	serverCfg.Plugins.Enabled = []string{"udp-hol"}

	server := engine.New(serverCfg, log.New(io.Discard, "", 0))
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.Start(ctx)
	}()

	select {
	case err := <-serverErrCh:
		t.Fatalf("server exited before ready: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	waitPort(t, serverWSAddr, 8*time.Second)

	clientCfg := config.DefaultClient()
	clientCfg.ListenNetwork = "udp"
	clientCfg.ListenAddr = clientListenAddr
	clientCfg.ServerWSURL = fmt.Sprintf("ws://%s/ws", serverWSAddr)
	clientCfg.PluginName = "udp-hol"
	clientCfg.ConnectRetries = 5
	clientCfg.RetryInitialDelay = "100ms"
	clientCfg.RetryMaxDelay = "500ms"
	clientCfg.UDPMetricsLog = "0s"

	tunnel := client.New(clientCfg, log.New(io.Discard, "", 0))
	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- tunnel.Start(ctx)
	}()

	conn, err := net.DialTimeout("udp", clientListenAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial udp client listen: %v", err)
	}
	defer conn.Close()
	if udpConn, ok := conn.(*net.UDPConn); ok {
		_ = udpConn.SetReadBuffer(4 << 20)
	}

	waitUDPPing(t, conn, 8*time.Second)
	baseline := measureUDPPingRTT(t, conn, 12)
	hol := measureUDPHOLRTT(t, conn, 6, holBulkPackets)

	baselineStats := summarizeDurations(baseline)
	holStats := summarizeDurations(hol)
	t.Logf(
		"udp websocket hol measurement baseline=%s hol_burst=%s bulk=%dpackets/%dbytes",
		baselineStats,
		holStats,
		holBulkPackets,
		holBulkPayloadSize,
	)

	if holStats.Count != len(hol) || baselineStats.Count != len(baseline) {
		t.Fatalf("invalid measurement result: baseline=%s hol=%s", baselineStats, holStats)
	}

	cancel()

	waitNoError(t, serverErrCh, 5*time.Second, "server")
	waitNoError(t, clientErrCh, 5*time.Second, "client")
	waitNoError(t, targetErrCh, 5*time.Second, "target")
}

func runUDPHOLTarget(ctx context.Context, addr string, bulkPackets, bulkPayloadSize int) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	buf := make([]byte, 2048)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, remoteAddr, err := conn.ReadFrom(buf)
		if n > 0 {
			msg := string(buf[:n])
			switch {
			case strings.HasPrefix(msg, "ping:"):
				_, _ = conn.WriteTo([]byte("pong:"+strings.TrimPrefix(msg, "ping:")), remoteAddr)
			case strings.HasPrefix(msg, "hol:"):
				id := strings.TrimPrefix(msg, "hol:")
				for i := 0; i < bulkPackets; i++ {
					payload := make([]byte, bulkPayloadSize)
					header := fmt.Sprintf("bulk:%s:%02d:", id, i)
					copy(payload, header)
					for j := len(header); j < len(payload); j++ {
						payload[j] = 'B'
					}
					_, _ = conn.WriteTo(payload, remoteAddr)
					// Pace the synthetic burst so loopback UDP buffers do not drop the tail.
					time.Sleep(250 * time.Microsecond)
				}
				_, _ = conn.WriteTo([]byte("hol-ack:"+id), remoteAddr)
			default:
				_, _ = conn.WriteTo(buf[:n], remoteAddr)
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
	}
}

func waitUDPPing(t *testing.T, conn net.Conn, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		id := fmt.Sprintf("warmup-%d", time.Now().UnixNano())
		if err := writePacket(conn, "ping:"+id, 500*time.Millisecond); err != nil {
			t.Fatalf("write udp warmup ping: %v", err)
		}
		if _, err := readUntilPacket(conn, "pong:"+id, 500*time.Millisecond); err == nil {
			return
		}
		time.Sleep(80 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for udp ping through proxy")
}

func measureUDPPingRTT(t *testing.T, conn net.Conn, count int) []time.Duration {
	t.Helper()

	samples := make([]time.Duration, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("baseline-%02d-%d", i, time.Now().UnixNano())
		start := time.Now()
		if err := writePacket(conn, "ping:"+id, 1*time.Second); err != nil {
			t.Fatalf("write udp baseline ping: %v", err)
		}
		if _, err := readUntilPacket(conn, "pong:"+id, 2*time.Second); err != nil {
			t.Fatalf("read udp baseline pong: %v", err)
		}
		samples = append(samples, time.Since(start))
		time.Sleep(10 * time.Millisecond)
	}
	return samples
}

func measureUDPHOLRTT(t *testing.T, conn net.Conn, count int, expectedBulk int) []time.Duration {
	t.Helper()

	samples := make([]time.Duration, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("hol-%02d-%d", i, time.Now().UnixNano())
		start := time.Now()
		if err := writePacket(conn, "hol:"+id, 1*time.Second); err != nil {
			t.Fatalf("write udp hol request: %v", err)
		}

		seenBulk := 0
		for {
			payload, err := readPacket(conn, 4*time.Second)
			if err != nil {
				t.Fatalf("read udp hol packet after %d/%d bulk packets: %v", seenBulk, expectedBulk, err)
			}
			msg := string(payload)
			if strings.HasPrefix(msg, "bulk:"+id+":") {
				seenBulk++
				continue
			}
			if msg == "hol-ack:"+id {
				break
			}
		}

		if seenBulk != expectedBulk {
			t.Fatalf("expected %d hol bulk packets, got %d", expectedBulk, seenBulk)
		}
		samples = append(samples, time.Since(start))
		time.Sleep(10 * time.Millisecond)
	}
	return samples
}

func writePacket(conn net.Conn, payload string, timeout time.Duration) error {
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	_, err := conn.Write([]byte(payload))
	return err
}

func readUntilPacket(conn net.Conn, expected string, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		payload, err := readPacket(conn, time.Until(deadline))
		if err != nil {
			return nil, err
		}
		if string(payload) == expected {
			return payload, nil
		}
	}
	return nil, fmt.Errorf("timeout waiting for %q", expected)
}

func readPacket(conn net.Conn, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	buf := make([]byte, holBulkPayloadSize+1024)

	for {
		if remaining := time.Until(deadline); remaining <= 0 {
			return nil, fmt.Errorf("timeout after %s", timeout)
		} else {
			_ = conn.SetReadDeadline(time.Now().Add(remaining))
		}
		n, err := conn.Read(buf)
		if err == nil {
			return append([]byte(nil), buf[:n]...), nil
		}
		if isRetryableUDPReadError(err) {
			time.Sleep(30 * time.Millisecond)
			continue
		}
		return nil, err
	}
}

type durationSummary struct {
	Count int
	Min   time.Duration
	P50   time.Duration
	P95   time.Duration
	Max   time.Duration
}

func summarizeDurations(samples []time.Duration) durationSummary {
	if len(samples) == 0 {
		return durationSummary{}
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	return durationSummary{
		Count: len(sorted),
		Min:   sorted[0],
		P50:   sorted[percentileIndex(len(sorted), 0.50)],
		P95:   sorted[percentileIndex(len(sorted), 0.95)],
		Max:   sorted[len(sorted)-1],
	}
}

func percentileIndex(count int, percentile float64) int {
	if count <= 1 {
		return 0
	}
	idx := int(float64(count-1)*percentile + 0.5)
	if idx < 0 {
		return 0
	}
	if idx >= count {
		return count - 1
	}
	return idx
}

func (s durationSummary) String() string {
	return fmt.Sprintf(
		"count=%d min=%s p50=%s p95=%s max=%s",
		s.Count,
		formatMeasurementDuration(s.Min),
		formatMeasurementDuration(s.P50),
		formatMeasurementDuration(s.P95),
		formatMeasurementDuration(s.Max),
	)
}

func formatMeasurementDuration(d time.Duration) time.Duration {
	if d > 0 && d < time.Microsecond {
		return time.Microsecond
	}
	return d.Truncate(time.Microsecond)
}
