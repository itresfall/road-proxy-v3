package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/udprecord"
	"road-proxy-v3/internal/udputil"
)

type Tunnel struct {
	cfg         *config.ClientConfig
	logger      *log.Logger
	bufferPool  sync.Pool
	udpRecorder *udprecord.Recorder
}

type udpSession struct {
	key          string
	clientAddr   net.Addr
	wsConn       *websocket.Conn
	writeMu      sync.Mutex
	activityMu   sync.Mutex
	metricsMu    sync.Mutex
	lastActivity time.Time
	metrics      udpSessionMetrics
	closeOnce    sync.Once
	closed       chan struct{}
}

func (s *udpSession) touch(ts time.Time) {
	s.activityMu.Lock()
	s.lastActivity = ts
	s.activityMu.Unlock()
}

func (s *udpSession) lastSeen() time.Time {
	s.activityMu.Lock()
	defer s.activityMu.Unlock()
	return s.lastActivity
}

func (s *udpSession) close() {
	s.closeOnce.Do(func() {
		close(s.closed)
		_ = s.wsConn.Close()
	})
}

func New(cfg *config.ClientConfig, logger *log.Logger) *Tunnel {
	if logger == nil {
		logger = log.Default()
	}

	t := &Tunnel{
		cfg:    cfg,
		logger: logger,
	}
	t.bufferPool = sync.Pool{
		New: func() any {
			return make([]byte, cfg.BufferSize)
		},
	}
	return t
}

func (t *Tunnel) Start(ctx context.Context) error {
	switch t.cfg.ListenNetwork {
	case "tcp":
		return t.startTCP(ctx)
	case "udp":
		return t.startUDP(ctx)
	default:
		return fmt.Errorf("unsupported listen_network %q", t.cfg.ListenNetwork)
	}
}

func (t *Tunnel) startTCP(ctx context.Context) error {
	listener, err := net.Listen("tcp", t.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", t.cfg.ListenAddr, err)
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	targetWS, err := t.buildWSURL()
	if err != nil {
		return err
	}

	t.logger.Printf("client listener active: game_target=%s road_server=%s plugin=%s network=tcp", t.cfg.ListenAddr, targetWS, t.cfg.PluginName)

	for {
		localConn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			t.logger.Printf("accept error: %v", err)
			continue
		}

		go t.handleConnection(ctx, localConn, targetWS)
	}
}

func (t *Tunnel) startUDP(ctx context.Context) error {
	if err := t.setupUDPRecorder(); err != nil {
		return err
	}
	defer t.closeUDPRecorder()

	packetConn, err := net.ListenPacket("udp", t.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s (udp): %w", t.cfg.ListenAddr, err)
	}
	defer packetConn.Close()

	go func() {
		<-ctx.Done()
		_ = packetConn.Close()
	}()

	targetWS, err := t.buildWSURL()
	if err != nil {
		return err
	}

	t.logger.Printf("client listener active: game_target=%s road_server=%s plugin=%s network=udp", t.cfg.ListenAddr, targetWS, t.cfg.PluginName)

	sessions := map[string]*udpSession{}
	var sessionsMu sync.Mutex

	closeAllSessions := func() {
		sessionsMu.Lock()
		all := make([]*udpSession, 0, len(sessions))
		for key, session := range sessions {
			delete(sessions, key)
			all = append(all, session)
		}
		sessionsMu.Unlock()

		for _, session := range all {
			t.logUDPSessionMetrics(session, time.Now(), "close", true)
			session.close()
		}
	}
	defer closeAllSessions()

	removeSession := func(key string) {
		sessionsMu.Lock()
		session, ok := sessions[key]
		if ok {
			delete(sessions, key)
		}
		sessionsMu.Unlock()

		if ok {
			t.logUDPSessionMetrics(session, time.Now(), "close", true)
			session.close()
		}
	}

	getOrCreateSession := func(srcAddr net.Addr) (*udpSession, error) {
		key := srcAddr.String()

		sessionsMu.Lock()
		existing, ok := sessions[key]
		sessionsMu.Unlock()
		if ok {
			existing.touch(time.Now())
			return existing, nil
		}

		wsConn, _, dialErr := t.openWebSocket(ctx, targetWS)
		if dialErr != nil {
			return nil, fmt.Errorf("udp websocket dial failed: %w", dialErr)
		}

		session := &udpSession{
			key:        key,
			clientAddr: srcAddr,
			wsConn:     wsConn,
			metrics:    newUDPSessionMetrics(time.Now()),
			closed:     make(chan struct{}),
		}
		session.touch(time.Now())

		sessionsMu.Lock()
		if existing, ok := sessions[key]; ok {
			sessionsMu.Unlock()
			session.close()
			existing.touch(time.Now())
			return existing, nil
		}
		sessions[key] = session
		sessionsMu.Unlock()

		go func() {
			err := t.readFromUDPSession(packetConn, session)
			if normalized := normalizeProxyError(err); normalized != nil {
				t.logger.Printf("udp session %s closed with error: %v", session.key, normalized)
			}
			removeSession(session.key)
		}()

		return session, nil
	}

	udpIdle := t.cfg.UDPSessionIdleDuration()
	if udpIdle > 0 {
		sweepEvery := minDuration(10*time.Second, udpIdle)
		go func() {
			ticker := time.NewTicker(sweepEvery)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					now := time.Now()
					expired := make([]string, 0)

					sessionsMu.Lock()
					for key, session := range sessions {
						if now.Sub(session.lastSeen()) > udpIdle {
							expired = append(expired, key)
						}
					}
					sessionsMu.Unlock()

					for _, key := range expired {
						removeSession(key)
					}
				}
			}
		}()
	}

	metricsInterval := t.cfg.UDPMetricsLogIntervalDuration()
	if metricsInterval > 0 {
		go func() {
			ticker := time.NewTicker(metricsInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					sessionsMu.Lock()
					all := make([]*udpSession, 0, len(sessions))
					for _, session := range sessions {
						all = append(all, session)
					}
					sessionsMu.Unlock()

					now := time.Now()
					for _, session := range all {
						t.logUDPSessionMetrics(session, now, "periodic", false)
					}
				}
			}
		}()
	}

	buffer := make([]byte, udputil.ReadBufferSize(t.cfg.BufferSize))
	var localOversizeLogOnce sync.Once
	var localMTULogOnce sync.Once
	for {
		n, srcAddr, oversized, err := udputil.ReadDatagram(packetConn, buffer, t.cfg.BufferSize)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			t.logger.Printf("udp read error: %v", err)
			continue
		}
		if n == 0 {
			continue
		}
		if oversized {
			localOversizeLogOnce.Do(func() {
				t.logger.Printf("udp datagram dropped before websocket: source=%s read=%d max_payload=%d reason=truncation_guard", srcAddr, n, t.cfg.BufferSize)
			})
			continue
		}
		if udputil.AboveConservativeMTU(n) {
			localMTULogOnce.Do(func() {
				t.logger.Printf("udp datagram exceeds conservative WAN MTU: path=local_to_websocket size=%d threshold=%d", n, udputil.ConservativeMTUPayloadBytes)
			})
		}

		session, err := getOrCreateSession(srcAddr)
		if err != nil {
			t.logger.Printf("udp session create failed for %s: %v", srcAddr.String(), err)
			continue
		}

		payload := append([]byte(nil), buffer[:n]...)
		session.touch(time.Now())
		if err := t.writeUDPSession(session, payload); err != nil {
			if normalized := normalizeProxyError(err); normalized != nil {
				t.logger.Printf("udp session write failed for %s: %v", srcAddr.String(), normalized)
			}
			removeSession(session.key)
		}
	}
}

func (t *Tunnel) setupUDPRecorder() error {
	recorder, err := udprecord.New(udprecord.Options{
		Enabled:   t.cfg.UDPRecord.Enabled,
		Path:      t.cfg.UDPRecord.Path,
		Role:      "client",
		Duration:  t.cfg.UDPRecord.DurationValue(),
		MaxEvents: t.cfg.UDPRecord.MaxEvents,
	}, t.logger)
	if err != nil {
		return err
	}
	t.udpRecorder = recorder
	return nil
}

func (t *Tunnel) closeUDPRecorder() {
	if err := t.udpRecorder.Close(); err != nil {
		t.logger.Printf("udp recorder close failed: %v", err)
	}
}

func (t *Tunnel) handleConnection(ctx context.Context, localConn net.Conn, wsURL string) {
	defer localConn.Close()
	t.applyTCPOptions(localConn)

	wsConn, _, err := t.openWebSocket(ctx, wsURL)
	if err != nil {
		t.logger.Printf("websocket dial failed: %v", err)
		return
	}
	defer wsConn.Close()

	if err := t.proxy(localConn, wsConn); err != nil {
		t.logger.Printf("connection closed with error: %v", err)
	}
}

func (t *Tunnel) openWebSocket(
	ctx context.Context,
	wsURL string,
) (*websocket.Conn, *http.Response, error) {
	header := t.buildDialHeaders()
	dialer := websocket.Dialer{
		HandshakeTimeout:  t.cfg.HandshakeTimeoutDuration(),
		ReadBufferSize:    t.cfg.BufferSize,
		WriteBufferSize:   t.cfg.BufferSize,
		EnableCompression: t.cfg.EnableCompression,
	}
	return t.dialWebSocketWithRetry(ctx, &dialer, wsURL, header)
}

func (t *Tunnel) readFromUDPSession(packetConn net.PacketConn, session *udpSession) error {
	pingErrCh := make(chan error, 1)
	wsIdleTimeout := t.cfg.WSIdleTimeoutDuration()
	wsPingInterval := t.cfg.WSPingIntervalDuration()
	var localOversizeLogOnce sync.Once
	var localMTULogOnce sync.Once

	session.wsConn.SetReadLimit(int64(t.cfg.MaxWSMessageBytes))
	if wsIdleTimeout > 0 {
		_ = session.wsConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
		session.wsConn.SetPongHandler(func(_ string) error {
			_ = session.wsConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
			return nil
		})
	}

	if wsPingInterval > 0 {
		go func() {
			ticker := time.NewTicker(wsPingInterval)
			defer ticker.Stop()

			for {
				select {
				case <-session.closed:
					return
				case <-ticker.C:
					session.writeMu.Lock()
					err := session.wsConn.WriteControl(
						websocket.PingMessage,
						nil,
						time.Now().Add(5*time.Second),
					)
					session.writeMu.Unlock()
					if err != nil {
						select {
						case pingErrCh <- err:
						default:
						}
						session.close()
						return
					}
				}
			}
		}()
	}

	for {
		if wsIdleTimeout > 0 {
			_ = session.wsConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
		}

		msgType, payload, err := session.wsConn.ReadMessage()
		if err != nil {
			select {
			case pingErr := <-pingErrCh:
				if pingErr != nil && normalizeProxyError(err) == nil {
					return pingErr
				}
			default:
			}
			return err
		}
		if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
			continue
		}
		if len(payload) == 0 {
			continue
		}
		if len(payload) > t.cfg.BufferSize {
			localOversizeLogOnce.Do(func() {
				t.logger.Printf("udp websocket payload dropped before local write: session=%s size=%d max_payload=%d reason=truncation_guard", session.key, len(payload), t.cfg.BufferSize)
			})
			continue
		}
		if udputil.AboveConservativeMTU(len(payload)) {
			localMTULogOnce.Do(func() {
				t.logger.Printf("udp datagram exceeds conservative WAN MTU: path=websocket_to_local session=%s size=%d threshold=%d", session.key, len(payload), udputil.ConservativeMTUPayloadBytes)
			})
		}

		if _, err := packetConn.WriteTo(payload, session.clientAddr); err != nil {
			return err
		}
		now := time.Now()
		t.udpRecorder.RecordPacket("websocket_to_local", t.cfg.PluginName, session.key, "websocket", addrString(session.clientAddr), payload)
		session.touch(now)
		session.metricsMu.Lock()
		session.metrics.observeRX(now, payload)
		session.metricsMu.Unlock()
	}
}

func (t *Tunnel) writeUDPSession(session *udpSession, payload []byte) error {
	wsIdleTimeout := t.cfg.WSIdleTimeoutDuration()

	session.writeMu.Lock()
	defer session.writeMu.Unlock()

	if wsIdleTimeout > 0 {
		_ = session.wsConn.SetWriteDeadline(time.Now().Add(wsIdleTimeout))
	}
	if err := session.wsConn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		return err
	}

	now := time.Now()
	t.udpRecorder.RecordPacket("local_to_websocket", t.cfg.PluginName, session.key, addrString(session.clientAddr), "websocket", payload)
	session.touch(now)
	session.metricsMu.Lock()
	session.metrics.observeTX(now, payload)
	session.metricsMu.Unlock()
	return nil
}

func (t *Tunnel) logUDPSessionMetrics(
	session *udpSession,
	now time.Time,
	reason string,
	force bool,
) {
	interval := t.cfg.UDPMetricsLogIntervalDuration()
	if interval <= 0 {
		return
	}

	session.metricsMu.Lock()
	if !force && !session.metrics.shouldReport(now, interval) {
		session.metricsMu.Unlock()
		return
	}
	snapshot := session.metrics.snapshot(now)
	session.metrics.lastReportAt = now
	session.metricsMu.Unlock()

	t.logger.Printf(
		"udp metrics [%s] session=%s age=%s tx=%dpkts/%dB rx=%dpkts/%dB tx_jitter=%.2fms rx_jitter=%.2fms tx_max_gap=%s rx_max_gap=%s tx_loss=%s rx_loss=%s tx_size=%s rx_size=%s",
		reason,
		session.key,
		snapshot.Age.Truncate(time.Millisecond),
		snapshot.TX.Packets,
		snapshot.TX.Bytes,
		snapshot.RX.Packets,
		snapshot.RX.Bytes,
		float64(snapshot.TX.Jitter)/float64(time.Millisecond),
		float64(snapshot.RX.Jitter)/float64(time.Millisecond),
		snapshot.TX.MaxGap.Truncate(time.Millisecond),
		snapshot.RX.MaxGap.Truncate(time.Millisecond),
		formatUDPLossSummary(snapshot.TX),
		formatUDPLossSummary(snapshot.RX),
		formatUDPSizeSummary(snapshot.TX),
		formatUDPSizeSummary(snapshot.RX),
	)
}

func (t *Tunnel) dialWebSocketWithRetry(
	ctx context.Context,
	dialer *websocket.Dialer,
	wsURL string,
	header http.Header,
) (*websocket.Conn, *http.Response, error) {
	attempts := t.cfg.ConnectRetries + 1
	delay := t.cfg.RetryInitialDelayDuration()
	maxDelay := t.cfg.RetryMaxDelayDuration()

	if delay <= 0 {
		delay = 100 * time.Millisecond
	}
	if maxDelay < delay {
		maxDelay = delay
	}

	var lastErr error
	var lastResp *http.Response

	for attempt := 1; attempt <= attempts; attempt++ {
		wsConn, resp, err := dialer.DialContext(ctx, wsURL, header)
		if err == nil {
			return wsConn, resp, nil
		}

		lastErr = formatWebSocketDialError(err, resp)
		lastResp = resp

		if ctx.Err() != nil {
			return nil, lastResp, ctx.Err()
		}
		if !shouldRetryWebSocketDial(resp) {
			break
		}
		if attempt == attempts {
			break
		}

		t.logger.Printf(
			"websocket dial attempt %d/%d failed, retrying in %s: %v",
			attempt,
			attempts,
			delay,
			lastErr,
		)

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, lastResp, ctx.Err()
		case <-timer.C:
		}

		delay = minDuration(delay*2, maxDelay)
	}

	return nil, lastResp, lastErr
}

func formatWebSocketDialError(err error, resp *http.Response) error {
	if err == nil {
		return nil
	}
	if resp == nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf(
			"ROAD authentication failed (HTTP %d): check client auth_token/%s: %w",
			resp.StatusCode,
			config.DefaultAuthHeaderName,
			err,
		)
	default:
		return fmt.Errorf("%w (HTTP %d)", err, resp.StatusCode)
	}
}

func shouldRetryWebSocketDial(resp *http.Response) bool {
	if resp == nil {
		return true
	}
	return resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden
}

func (t *Tunnel) buildDialHeaders() http.Header {
	header := make(http.Header)
	for key, value := range t.cfg.Headers {
		header.Set(key, value)
	}
	authToken := config.ResolveSecret(t.cfg.AuthToken)
	if authToken != "" {
		authHeader := strings.TrimSpace(t.cfg.AuthHeader)
		if authHeader == "" {
			authHeader = config.DefaultAuthHeaderName
		}
		header.Set(authHeader, config.AuthHeaderValue(authHeader, authToken))
	}
	return header
}

func (t *Tunnel) proxy(localConn net.Conn, wsConn *websocket.Conn) error {
	errCh := make(chan error, 3)
	wsIdleTimeout := t.cfg.WSIdleTimeoutDuration()
	wsPingInterval := t.cfg.WSPingIntervalDuration()
	done := make(chan struct{})
	wsConn.SetReadLimit(int64(t.cfg.MaxWSMessageBytes))

	if wsIdleTimeout > 0 {
		_ = wsConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
		wsConn.SetPongHandler(func(_ string) error {
			_ = wsConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
			return nil
		})
	}

	if wsPingInterval > 0 {
		go func() {
			ticker := time.NewTicker(wsPingInterval)
			defer ticker.Stop()

			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					if err := wsConn.WriteControl(
						websocket.PingMessage,
						nil,
						time.Now().Add(5*time.Second),
					); err != nil {
						select {
						case errCh <- err:
						default:
						}
						return
					}
				}
			}
		}()
	}

	go func() {
		buffer := t.bufferPool.Get().([]byte)
		defer t.bufferPool.Put(buffer)

		for {
			n, err := localConn.Read(buffer)
			if n > 0 {
				if wsIdleTimeout > 0 {
					_ = wsConn.SetWriteDeadline(time.Now().Add(wsIdleTimeout))
				}
				if err := wsConn.WriteMessage(websocket.BinaryMessage, buffer[:n]); err != nil {
					errCh <- err
					return
				}
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		for {
			if wsIdleTimeout > 0 {
				_ = wsConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
			}
			msgType, payload, err := wsConn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
				continue
			}
			if len(payload) == 0 {
				continue
			}
			if _, err := localConn.Write(payload); err != nil {
				errCh <- err
				return
			}
		}
	}()

	firstErr := <-errCh
	close(done)
	_ = localConn.Close()
	_ = wsConn.Close()
	<-errCh

	return normalizeProxyError(firstErr)
}

func (t *Tunnel) buildWSURL() (string, error) {
	raw := strings.TrimSpace(t.cfg.ServerWSURL)
	if raw == "" {
		return "", fmt.Errorf("server_ws_url cannot be empty")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse server_ws_url: %w", err)
	}

	if u.Scheme != "ws" && u.Scheme != "wss" {
		return "", fmt.Errorf("server_ws_url scheme must be ws or wss")
	}
	if u.Host == "" {
		return "", fmt.Errorf("server_ws_url host is required")
	}

	query := u.Query()
	if t.cfg.PluginName != "" && query.Get("plugin") == "" {
		query.Set("plugin", t.cfg.PluginName)
	}
	u.RawQuery = query.Encode()

	return u.String(), nil
}

func (t *Tunnel) applyTCPOptions(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetNoDelay(true)
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
}

func normalizeProxyError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	if websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
	) {
		return nil
	}
	if closeErr, ok := err.(*websocket.CloseError); ok {
		if closeErr.Code == websocket.CloseAbnormalClosure &&
			strings.Contains(strings.ToLower(closeErr.Text), "unexpected eof") {
			return nil
		}
	}
	return err
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func addrString(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}
