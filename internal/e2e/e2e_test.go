package e2e

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"road-proxy-v3/internal/client"
	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/engine"
	"road-proxy-v3/internal/plugin"
)

func TestTCPWebSocketTunnelRoundTrip(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tcpAddrs := mustUniqueTCPAddrs(t, 5)
	targetAddr := tcpAddrs[0]
	serverTCPAddr := tcpAddrs[1]
	serverWSAddr := tcpAddrs[2]
	serverControlAddr := tcpAddrs[3]
	clientListenAddr := tcpAddrs[4]

	targetErrCh := make(chan error, 1)
	go func() {
		targetErrCh <- runEchoTarget(ctx, targetAddr)
	}()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins")
	mustWritePluginSchema(t, filepath.Join(pluginRoot, "minecraft"), "minecraft", "tcp", targetAddr)

	serverCfg := config.Default()
	serverCfg.TCP.ListenAddr = serverTCPAddr
	serverCfg.HTTP.ListenAddr = serverWSAddr
	serverCfg.Control.ListenAddr = serverControlAddr
	serverCfg.Plugins.Dir = pluginRoot
	serverCfg.Plugins.Enabled = []string{"minecraft"}

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
	clientCfg.ListenAddr = clientListenAddr
	clientCfg.ServerWSURL = fmt.Sprintf("ws://%s/ws", serverWSAddr)
	clientCfg.PluginName = "minecraft"
	clientCfg.ConnectRetries = 5
	clientCfg.RetryInitialDelay = "100ms"
	clientCfg.RetryMaxDelay = "500ms"

	tunnel := client.New(clientCfg, log.New(io.Discard, "", 0))
	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- tunnel.Start(ctx)
	}()

	waitPort(t, clientListenAddr, 8*time.Second)

	const payload = "minecraft-e2e-ping"
	got := roundTrip(t, clientListenAddr, payload, 8*time.Second)
	if got != payload {
		t.Fatalf("round trip mismatch: got=%q want=%q", got, payload)
	}

	cancel()

	waitNoError(t, serverErrCh, 5*time.Second, "server")
	waitNoError(t, clientErrCh, 5*time.Second, "client")
	waitNoError(t, targetErrCh, 5*time.Second, "target")
}

func TestUDPWebSocketTunnelRoundTrip(t *testing.T) {
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
		targetErrCh <- runUDPEchoTarget(ctx, targetAddr)
	}()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins")
	mustWritePluginSchema(t, filepath.Join(pluginRoot, "valheim"), "valheim", "udp", targetAddr)

	serverCfg := config.Default()
	serverCfg.TCP.ListenAddr = serverTCPAddr
	serverCfg.HTTP.ListenAddr = serverWSAddr
	serverCfg.Control.ListenAddr = serverControlAddr
	serverCfg.Plugins.Dir = pluginRoot
	serverCfg.Plugins.Enabled = []string{"valheim"}

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
	clientCfg.PluginName = "valheim"
	clientCfg.ConnectRetries = 5
	clientCfg.RetryInitialDelay = "100ms"
	clientCfg.RetryMaxDelay = "500ms"

	tunnel := client.New(clientCfg, log.New(io.Discard, "", 0))
	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- tunnel.Start(ctx)
	}()

	const payload = "udp-ws-roundtrip"
	got := roundTripUDP(t, clientListenAddr, payload, 8*time.Second)
	if got != payload {
		t.Fatalf("udp round trip mismatch: got=%q want=%q", got, payload)
	}

	cancel()

	waitNoError(t, serverErrCh, 5*time.Second, "server")
	waitNoError(t, clientErrCh, 5*time.Second, "client")
	waitNoError(t, targetErrCh, 5*time.Second, "target")
}

func TestUDPWebSocketTunnelRoundTripThreeClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	udpAddrs := mustUniqueUDPAddrs(t, 4)
	targetAddr := udpAddrs[0]
	clientListenA := udpAddrs[1]
	clientListenB := udpAddrs[2]
	clientListenC := udpAddrs[3]
	tcpAddrs := mustUniqueTCPAddrs(t, 3)
	serverTCPAddr := tcpAddrs[0]
	serverWSAddr := tcpAddrs[1]
	serverControlAddr := tcpAddrs[2]

	targetErrCh := make(chan error, 1)
	go func() {
		targetErrCh <- runUDPEchoTarget(ctx, targetAddr)
	}()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins")
	mustWritePluginSchema(t, filepath.Join(pluginRoot, "fnaf"), "fnaf", "udp", targetAddr)

	serverCfg := config.Default()
	serverCfg.TCP.ListenAddr = serverTCPAddr
	serverCfg.HTTP.ListenAddr = serverWSAddr
	serverCfg.Control.ListenAddr = serverControlAddr
	serverCfg.Plugins.Dir = pluginRoot
	serverCfg.Plugins.Enabled = []string{"fnaf"}

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

	clientAddrs := []string{clientListenA, clientListenB, clientListenC}
	clientErrCh := make([]chan error, 0, len(clientAddrs))
	for _, addr := range clientAddrs {
		clientCfg := config.DefaultClient()
		clientCfg.ListenNetwork = "udp"
		clientCfg.ListenAddr = addr
		clientCfg.ServerWSURL = fmt.Sprintf("ws://%s/ws", serverWSAddr)
		clientCfg.PluginName = "fnaf"
		clientCfg.ConnectRetries = 5
		clientCfg.RetryInitialDelay = "100ms"
		clientCfg.RetryMaxDelay = "500ms"

		tunnel := client.New(clientCfg, log.New(io.Discard, "", 0))
		errCh := make(chan error, 1)
		clientErrCh = append(clientErrCh, errCh)
		go func(ch chan error) {
			ch <- tunnel.Start(ctx)
		}(errCh)
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(clientAddrs))
	for i, addr := range clientAddrs {
		i := i
		addr := addr
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 6; n++ {
				payload := fmt.Sprintf("fnaf-client-%d-msg-%d", i+1, n+1)
				got, err := roundTripUDPResult(addr, payload, 8*time.Second)
				if err != nil {
					errs <- fmt.Errorf("client %d roundtrip failed: %w", i+1, err)
					return
				}
				if got != payload {
					errs <- fmt.Errorf("client %d roundtrip mismatch: got=%q want=%q", i+1, got, payload)
					return
				}
			}
		}()
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

func TestUDPWebSocketPeerBroadcastThreeClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	udpAddrs := mustUniqueUDPAddrs(t, 4)
	targetAddr := udpAddrs[0]
	clientListenAddrs := []string{udpAddrs[1], udpAddrs[2], udpAddrs[3]}
	tcpAddrs := mustUniqueTCPAddrs(t, 3)
	serverTCPAddr := tcpAddrs[0]
	serverWSAddr := tcpAddrs[1]
	serverControlAddr := tcpAddrs[2]

	targetErrCh := make(chan error, 1)
	go func() {
		targetErrCh <- runUDPDrainTarget(ctx, targetAddr)
	}()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins")
	mustWritePluginSchemaWithUDPPeerBroadcast(t, filepath.Join(pluginRoot, "gzdoom-peer"), "gzdoom-peer", targetAddr, true)

	serverCfg := config.Default()
	serverCfg.TCP.ListenAddr = serverTCPAddr
	serverCfg.HTTP.ListenAddr = serverWSAddr
	serverCfg.Control.ListenAddr = serverControlAddr
	serverCfg.Plugins.Dir = pluginRoot
	serverCfg.Plugins.Enabled = []string{"gzdoom-peer"}

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

	clientErrCh := make([]chan error, 0, len(clientListenAddrs))
	for _, addr := range clientListenAddrs {
		clientCfg := config.DefaultClient()
		clientCfg.ListenNetwork = "udp"
		clientCfg.ListenAddr = addr
		clientCfg.ServerWSURL = fmt.Sprintf("ws://%s/ws", serverWSAddr)
		clientCfg.PluginName = "gzdoom-peer"
		clientCfg.ConnectRetries = 5
		clientCfg.RetryInitialDelay = "100ms"
		clientCfg.RetryMaxDelay = "500ms"
		clientCfg.UDPMetricsLog = "0s"

		tunnel := client.New(clientCfg, log.New(io.Discard, "", 0))
		errCh := make(chan error, 1)
		clientErrCh = append(clientErrCh, errCh)
		go func(ch chan error) {
			ch <- tunnel.Start(ctx)
		}(errCh)
	}

	time.Sleep(300 * time.Millisecond)

	var wg sync.WaitGroup
	errs := make(chan error, len(clientListenAddrs))
	for i, addr := range clientListenAddrs {
		clientID := byte(i + 1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runUDPPeerBroadcastBot(ctx, addr, clientID, len(clientListenAddrs)-1, 5*time.Second); err != nil {
				errs <- fmt.Errorf("client %d peer broadcast failed: %w", clientID, err)
			}
		}()
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

func TestUDPWebSocketAcceptsAlternateSourcePortReply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	udpAddrs := mustUniqueUDPAddrs(t, 3)
	targetPrimaryAddr := udpAddrs[0]
	targetAlternateAddr := udpAddrs[1]
	clientListenAddr := udpAddrs[2]
	tcpAddrs := mustUniqueTCPAddrs(t, 3)
	serverTCPAddr := tcpAddrs[0]
	serverWSAddr := tcpAddrs[1]
	serverControlAddr := tcpAddrs[2]

	targetErrCh := make(chan error, 1)
	go func() {
		targetErrCh <- runUDPAlternateReplyTarget(ctx, targetPrimaryAddr, targetAlternateAddr)
	}()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins")
	mustWritePluginSchemaWithUDPReplyPolicy(t, filepath.Join(pluginRoot, "fnaf"), "fnaf", targetPrimaryAddr, plugin.UDPReplyPolicySameIP)

	serverCfg := config.Default()
	serverCfg.TCP.ListenAddr = serverTCPAddr
	serverCfg.HTTP.ListenAddr = serverWSAddr
	serverCfg.Control.ListenAddr = serverControlAddr
	serverCfg.Plugins.Dir = pluginRoot
	serverCfg.Plugins.Enabled = []string{"fnaf"}

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
	clientCfg.PluginName = "fnaf"
	clientCfg.ConnectRetries = 5
	clientCfg.RetryInitialDelay = "100ms"
	clientCfg.RetryMaxDelay = "500ms"

	tunnel := client.New(clientCfg, log.New(io.Discard, "", 0))
	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- tunnel.Start(ctx)
	}()

	const payload = "need-alternate-port-reply"
	got := roundTripUDP(t, clientListenAddr, payload, 8*time.Second)
	want := "alt:" + payload
	if got != want {
		t.Fatalf("udp alt-source roundtrip mismatch: got=%q want=%q", got, want)
	}

	cancel()

	waitNoError(t, serverErrCh, 5*time.Second, "server")
	waitNoError(t, clientErrCh, 5*time.Second, "client")
	waitNoError(t, targetErrCh, 5*time.Second, "target")
}

func TestUDPWebSocketRejectsAlternateSourcePortReplyWithStrictPolicy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	udpAddrs := mustUniqueUDPAddrs(t, 3)
	targetPrimaryAddr := udpAddrs[0]
	targetAlternateAddr := udpAddrs[1]
	clientListenAddr := udpAddrs[2]
	tcpAddrs := mustUniqueTCPAddrs(t, 3)
	serverTCPAddr := tcpAddrs[0]
	serverWSAddr := tcpAddrs[1]
	serverControlAddr := tcpAddrs[2]

	targetErrCh := make(chan error, 1)
	go func() {
		targetErrCh <- runUDPAlternateReplyTarget(ctx, targetPrimaryAddr, targetAlternateAddr)
	}()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins")
	mustWritePluginSchemaWithUDPReplyPolicy(t, filepath.Join(pluginRoot, "fnaf-strict"), "fnaf-strict", targetPrimaryAddr, plugin.UDPReplyPolicyStrict)

	serverCfg := config.Default()
	serverCfg.TCP.ListenAddr = serverTCPAddr
	serverCfg.HTTP.ListenAddr = serverWSAddr
	serverCfg.Control.ListenAddr = serverControlAddr
	serverCfg.Plugins.Dir = pluginRoot
	serverCfg.Plugins.Enabled = []string{"fnaf-strict"}

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
	clientCfg.PluginName = "fnaf-strict"
	clientCfg.ConnectRetries = 5
	clientCfg.RetryInitialDelay = "100ms"
	clientCfg.RetryMaxDelay = "500ms"
	clientCfg.UDPMetricsLog = "0s"

	tunnel := client.New(clientCfg, log.New(io.Discard, "", 0))
	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- tunnel.Start(ctx)
	}()

	if got, err := roundTripUDPResult(clientListenAddr, "strict-should-drop-alt-source", 1200*time.Millisecond); err == nil {
		t.Fatalf("strict udp reply policy should reject alternate source port reply, got %q", got)
	}

	cancel()

	waitNoError(t, serverErrCh, 5*time.Second, "server")
	waitNoError(t, clientErrCh, 5*time.Second, "client")
	waitNoError(t, targetErrCh, 5*time.Second, "target")
}

func TestUDPWebSocketDropsOversizedDatagrams(t *testing.T) {
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
		targetErrCh <- runUDPTruncationTarget(ctx, targetAddr, 1025)
	}()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins")
	mustWritePluginSchema(t, filepath.Join(pluginRoot, "udp-truncation"), "udp-truncation", "udp", targetAddr)

	serverCfg := config.Default()
	serverCfg.TCP.ListenAddr = serverTCPAddr
	serverCfg.TCP.BufferSize = 1024
	serverCfg.HTTP.ListenAddr = serverWSAddr
	serverCfg.Control.ListenAddr = serverControlAddr
	serverCfg.Plugins.Dir = pluginRoot
	serverCfg.Plugins.Enabled = []string{"udp-truncation"}

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
	clientCfg.PluginName = "udp-truncation"
	clientCfg.BufferSize = 1024
	clientCfg.ConnectRetries = 5
	clientCfg.RetryInitialDelay = "100ms"
	clientCfg.RetryMaxDelay = "500ms"
	clientCfg.UDPMetricsLog = "0s"

	tunnel := client.New(clientCfg, log.New(io.Discard, "", 0))
	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- tunnel.Start(ctx)
	}()

	if got := roundTripUDP(t, clientListenAddr, "small-ok", 8*time.Second); got != "small-ok" {
		t.Fatalf("small roundtrip mismatch: got=%q", got)
	}
	if got, err := roundTripUDPResult(clientListenAddr, strings.Repeat("L", 1025), 900*time.Millisecond); err == nil {
		t.Fatalf("oversized local datagram should be dropped, got %q", got)
	}
	if got, err := roundTripUDPResult(clientListenAddr, "large-reply", 900*time.Millisecond); err == nil {
		t.Fatalf("oversized target reply should be dropped, got %q", got)
	}
	if got := roundTripUDP(t, clientListenAddr, "small-after-drop", 8*time.Second); got != "small-after-drop" {
		t.Fatalf("small roundtrip after oversized drops mismatch: got=%q", got)
	}

	cancel()

	waitNoError(t, serverErrCh, 5*time.Second, "server")
	waitNoError(t, clientErrCh, 5*time.Second, "client")
	waitNoError(t, targetErrCh, 5*time.Second, "target")
}

func TestUDPReconnectMayChangeTargetVisibleSourcePort(t *testing.T) {
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
		targetErrCh <- runUDPSourceObserverTarget(ctx, targetAddr)
	}()

	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins")
	mustWritePluginSchema(t, filepath.Join(pluginRoot, "udp-reconnect"), "udp-reconnect", "udp", targetAddr)

	serverCfg := config.Default()
	serverCfg.TCP.ListenAddr = serverTCPAddr
	serverCfg.HTTP.ListenAddr = serverWSAddr
	serverCfg.Control.ListenAddr = serverControlAddr
	serverCfg.Plugins.Dir = pluginRoot
	serverCfg.Plugins.Enabled = []string{"udp-reconnect"}

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
	clientCfg.PluginName = "udp-reconnect"
	clientCfg.ConnectRetries = 5
	clientCfg.RetryInitialDelay = "100ms"
	clientCfg.RetryMaxDelay = "500ms"
	clientCfg.UDPSessionIdle = "100ms"
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

	observed := map[string]struct{}{}
	for i := 0; i < 6; i++ {
		got, err := roundTripUDPConnResult(conn, fmt.Sprintf("source-check-%d", i), 8*time.Second)
		if err != nil {
			t.Fatalf("source observer roundtrip %d failed: %v", i, err)
		}
		source, ok := strings.CutPrefix(got, "source:")
		if !ok || source == "" {
			t.Fatalf("unexpected source observer response: %q", got)
		}
		observed[source] = struct{}{}
		if len(observed) > 1 {
			break
		}
		time.Sleep(350 * time.Millisecond)
	}

	if len(observed) < 2 {
		t.Fatalf("expected reconnect to expose at least two target-visible UDP source addresses, got %#v", observed)
	}

	cancel()

	waitNoError(t, serverErrCh, 5*time.Second, "server")
	waitNoError(t, clientErrCh, 5*time.Second, "client")
	waitNoError(t, targetErrCh, 5*time.Second, "target")
}

func runUDPPeerBroadcastBot(ctx context.Context, proxyLocalAddr string, clientID byte, expectedPeers int, testWindow time.Duration) error {
	conn, err := net.DialTimeout("udp", proxyLocalAddr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("dial proxy local udp %s: %w", proxyLocalAddr, err)
	}
	defer conn.Close()

	stopWrites := make(chan struct{})
	defer close(stopWrites)

	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()

		seq := byte(0)
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopWrites:
				return
			case <-ticker.C:
				seq++
				_ = conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
				_, _ = conn.Write([]byte{'P', clientID, seq})
			}
		}
	}()

	observed := map[byte]struct{}{}
	deadline := time.Now().Add(testWindow)
	buf := make([]byte, 64)

	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, readErr := conn.Read(buf)
		if readErr != nil {
			if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
				continue
			}
			return fmt.Errorf("read peer packet: %w", readErr)
		}
		if n < 3 || buf[0] != 'P' {
			continue
		}
		peerID := buf[1]
		if peerID != 0 && peerID != clientID {
			observed[peerID] = struct{}{}
		}
	}

	if len(observed) < expectedPeers {
		return fmt.Errorf("observed %d/%d peers", len(observed), expectedPeers)
	}
	return nil
}

func runEchoTarget(ctx context.Context, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 4096)
			for {
				_ = c.SetReadDeadline(time.Now().Add(30 * time.Second))
				n, err := c.Read(buf)
				if n > 0 {
					_ = c.SetWriteDeadline(time.Now().Add(30 * time.Second))
					_, _ = c.Write(buf[:n])
				}
				if err != nil {
					return
				}
			}
		}(conn)
	}
}

func runUDPDrainTarget(ctx context.Context, addr string) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	buf := make([]byte, 4096)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, _, err := conn.ReadFrom(buf)
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

func runUDPEchoTarget(ctx context.Context, addr string) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	buf := make([]byte, 4096)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, remoteAddr, err := conn.ReadFrom(buf)
		if n > 0 {
			_, _ = conn.WriteTo(buf[:n], remoteAddr)
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

func runUDPTruncationTarget(ctx context.Context, addr string, largeReplySize int) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	buf := make([]byte, 4096)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, remoteAddr, err := conn.ReadFrom(buf)
		if n > 0 {
			msg := string(buf[:n])
			if msg == "large-reply" {
				_, _ = conn.WriteTo([]byte(strings.Repeat("R", largeReplySize)), remoteAddr)
			} else {
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

func runUDPSourceObserverTarget(ctx context.Context, addr string) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	buf := make([]byte, 4096)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, remoteAddr, err := conn.ReadFrom(buf)
		if n > 0 {
			_, _ = conn.WriteTo([]byte("source:"+remoteAddr.String()), remoteAddr)
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

func runUDPAlternateReplyTarget(ctx context.Context, primaryAddr, alternateAddr string) error {
	primaryConn, err := net.ListenPacket("udp", primaryAddr)
	if err != nil {
		return err
	}
	defer primaryConn.Close()

	alternateConn, err := net.ListenPacket("udp", alternateAddr)
	if err != nil {
		return err
	}
	defer alternateConn.Close()

	go func() {
		<-ctx.Done()
		_ = primaryConn.Close()
		_ = alternateConn.Close()
	}()

	buf := make([]byte, 4096)
	for {
		_ = primaryConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, remoteAddr, err := primaryConn.ReadFrom(buf)
		if n > 0 {
			resp := append([]byte("alt:"), buf[:n]...)
			_, _ = alternateConn.WriteTo(resp, remoteAddr)
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

func mustWritePluginSchema(t *testing.T, pluginDir string, pluginName, targetNetwork, targetAddr string) {
	t.Helper()

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	schema := fmt.Sprintf(`{
  "schema_version": "v1",
  "name": %q,
  "version": "3.0.0",
  "description": "e2e schema",
  "author": "test",
  "protocols": { "supported": ["tcp", "websocket"] },
  "target": { "network": %q, "address": %q },
  "capabilities": {
    "supports_reconnect": true,
    "supports_multiplex": false
  },
  "runtime": {
    "type": "json",
    "mode": "passthrough",
    "enable_obfuscation": false,
    "client_pipeline": [],
    "server_pipeline": []
  }
}`, pluginName, targetNetwork, targetAddr)

	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(schema), 0o644); err != nil {
		t.Fatalf("write plugin schema: %v", err)
	}
}

func mustWritePluginSchemaWithUDPPeerBroadcast(t *testing.T, pluginDir string, pluginName, targetAddr string, peerBroadcast bool) {
	t.Helper()

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	schema := fmt.Sprintf(`{
  "schema_version": "v1",
  "name": %q,
  "version": "3.0.0",
  "description": "e2e udp peer broadcast schema",
  "author": "test",
  "protocols": { "supported": ["udp", "websocket"] },
  "target": { "network": "udp", "address": %q },
  "capabilities": {
    "supports_reconnect": true,
    "supports_multiplex": true
  },
  "runtime": {
    "type": "json",
    "mode": "passthrough",
    "enable_obfuscation": false,
    "udp_peer_broadcast": %t,
    "client_pipeline": [],
    "server_pipeline": []
  }
}`, pluginName, targetAddr, peerBroadcast)

	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(schema), 0o644); err != nil {
		t.Fatalf("write plugin schema: %v", err)
	}
}

func mustWritePluginSchemaWithUDPReplyPolicy(t *testing.T, pluginDir string, pluginName, targetAddr, replyPolicy string) {
	t.Helper()

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	schema := fmt.Sprintf(`{
  "schema_version": "v1",
  "name": %q,
  "version": "3.0.0",
  "description": "e2e udp reply policy schema",
  "author": "test",
  "protocols": { "supported": ["udp", "websocket"] },
  "target": { "network": "udp", "address": %q },
  "capabilities": {
    "supports_reconnect": true,
    "supports_multiplex": true
  },
  "runtime": {
    "type": "json",
    "mode": "passthrough",
    "enable_obfuscation": false,
    "udp_reply_policy": %q,
    "client_pipeline": [],
    "server_pipeline": []
  }
}`, pluginName, targetAddr, replyPolicy)

	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(schema), 0o644); err != nil {
		t.Fatalf("write plugin schema: %v", err)
	}
}

func mustFreeAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("alloc free port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func mustFreeUDPAddr(t *testing.T) string {
	t.Helper()

	l, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("alloc free udp port: %v", err)
	}
	addr := l.LocalAddr().String()
	_ = l.Close()
	return addr
}

func mustUniqueTCPAddrs(t *testing.T, n int) []string {
	t.Helper()

	listeners := make([]net.Listener, 0, n)
	addrs := make([]string, 0, n)

	for i := 0; i < n; i++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			for _, existing := range listeners {
				_ = existing.Close()
			}
			t.Fatalf("alloc tcp port %d/%d: %v", i+1, n, err)
		}
		listeners = append(listeners, l)
		addrs = append(addrs, l.Addr().String())
	}

	for _, l := range listeners {
		_ = l.Close()
	}

	return addrs
}

func mustUniqueUDPAddrs(t *testing.T, n int) []string {
	t.Helper()

	conns := make([]net.PacketConn, 0, n)
	addrs := make([]string, 0, n)

	for i := 0; i < n; i++ {
		c, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			for _, existing := range conns {
				_ = existing.Close()
			}
			t.Fatalf("alloc udp port %d/%d: %v", i+1, n, err)
		}
		conns = append(conns, c)
		addrs = append(addrs, c.LocalAddr().String())
	}

	for _, c := range conns {
		_ = c.Close()
	}

	return addrs
}

func waitPort(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(80 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for port %s", addr)
}

func roundTrip(t *testing.T, addr, payload string, timeout time.Duration) string {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		t.Fatalf("dial client listen: %v", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}

	return string(buf[:n])
}

func roundTripUDP(t *testing.T, addr, payload string, timeout time.Duration) string {
	t.Helper()

	got, err := roundTripUDPResult(addr, payload, timeout)
	if err != nil {
		t.Fatalf("read udp payload: %v", err)
	}
	return got
}

func roundTripUDPResult(addr, payload string, timeout time.Duration) (string, error) {
	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return "", fmt.Errorf("dial udp client listen: %w", err)
	}
	defer conn.Close()

	return roundTripUDPConnResult(conn, payload, timeout)
}

func roundTripUDPConnResult(conn net.Conn, payload string, timeout time.Duration) (string, error) {
	buf := make([]byte, 4096)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		_ = conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
		if _, err := conn.Write([]byte(payload)); err != nil {
			return "", fmt.Errorf("write udp payload: %w", err)
		}

		_ = conn.SetReadDeadline(time.Now().Add(350 * time.Millisecond))
		n, err := conn.Read(buf)
		if err == nil {
			return string(buf[:n]), nil
		}
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			continue
		}
		return "", fmt.Errorf("read udp payload: %w", err)
	}

	return "", fmt.Errorf("read udp payload: timeout after %s", timeout)
}

func waitNoError(t *testing.T, ch <-chan error, timeout time.Duration, name string) {
	t.Helper()

	select {
	case err := <-ch:
		if err != nil {
			t.Fatalf("%s exited with error: %v", name, err)
		}
	case <-time.After(timeout):
		t.Fatalf("timeout waiting %s shutdown", name)
	}
}
