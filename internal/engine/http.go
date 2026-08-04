package engine

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"road-proxy-v3/internal/plugin"
	"road-proxy-v3/internal/udputil"
)

func (e *Engine) setupDataServer() error {
	if !e.cfg.HTTP.Enabled {
		return nil
	}

	e.wsUpgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return e.checkWebSocketOrigin(r)
		},
		EnableCompression: e.cfg.HTTP.EnableCompression,
		ReadBufferSize:    e.cfg.TCP.BufferSize,
		WriteBufferSize:   e.cfg.TCP.BufferSize,
		HandshakeTimeout:  e.cfg.HTTPHandshakeTimeoutDuration(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(e.cfg.HTTP.WSEndpoint, e.handleWebSocket)
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		e.handlePing(w, r, "data")
	})
	if e.cfg.WSAuthEnabled() {
		mux.HandleFunc("/api/info", e.withDataAuth(e.handleDataInfo))
	} else {
		mux.HandleFunc("/api/info", e.handleDataInfo)
	}

	handler := e.hostAllowMiddleware(mux)

	e.dataServer = &http.Server{
		Addr:              e.cfg.HTTP.ListenAddr,
		Handler:           handler,
		ReadTimeout:       e.cfg.HTTPReadTimeoutDuration(),
		WriteTimeout:      e.cfg.HTTPWriteTimeoutDuration(),
		ReadHeaderTimeout: e.cfg.HTTPReadHeaderTimeoutDuration(),
		MaxHeaderBytes:    e.cfg.HTTP.MaxHeaderBytes,
	}

	return nil
}

func (e *Engine) setupControlServer() error {
	if !e.cfg.Control.Enabled {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", e.handleHealth)
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		e.handlePing(w, r, "control")
	})
	mux.HandleFunc("/api/stats", e.handleStats)
	mux.HandleFunc("/api/sessions", e.handleSessions)
	mux.HandleFunc("/api/plugins", e.handlePlugins)
	mux.HandleFunc("/api/plugin/info/", e.handlePluginInfo)
	mux.HandleFunc("/api/plugin/config/", e.handlePluginConfig)
	mux.HandleFunc("/api/plugin/download/", e.handlePluginDownload)
	mux.HandleFunc("/api/info", e.handleControlInfo)
	mux.HandleFunc("/dashboard", e.handleDashboard)
	mux.HandleFunc("/", e.handleRoot)

	handler := e.hostAllowMiddleware(mux)
	if e.cfg.WSAuthEnabled() {
		handler = e.controlAuthMiddleware(handler)
	}

	e.controlServer = &http.Server{
		Addr:              e.cfg.Control.ListenAddr,
		Handler:           handler,
		ReadTimeout:       e.cfg.ControlReadTimeoutDuration(),
		WriteTimeout:      e.cfg.ControlWriteTimeoutDuration(),
		ReadHeaderTimeout: e.cfg.ControlReadHeaderTimeoutDuration(),
		MaxHeaderBytes:    e.cfg.Control.MaxHeaderBytes,
	}

	return nil
}

func (e *Engine) serveHTTP(name string, server *http.Server, errCh chan<- error) {
	err := server.ListenAndServe()
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return
	}
	errCh <- fmt.Errorf("%s server failed: %w", name, err)
}

func (e *Engine) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !e.validateWSToken(r) {
		e.stats.IncError()
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	release, status, reason := e.admitWebSocket(r)
	if release == nil {
		e.stats.IncError()
		http.Error(w, websocketLimitLogMessage(status, reason), status)
		return
	}
	defer release()

	requestedPlugin := strings.TrimSpace(r.URL.Query().Get("plugin"))
	selectedPlugin, err := e.resolvePlugin(requestedPlugin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	requestedTarget := strings.TrimSpace(r.URL.Query().Get("target"))
	selectedTarget, ok := selectedPlugin.ResolveTarget(requestedTarget)
	if !ok {
		http.Error(w, fmt.Sprintf("unknown plugin target %q", requestedTarget), http.StatusBadRequest)
		return
	}
	targetNetwork := strings.TrimSpace(selectedTarget.Network)
	if targetNetwork == "" {
		targetNetwork = selectedPlugin.TargetNetwork()
	}
	targetAddress := strings.TrimSpace(selectedTarget.Address)
	if targetAddress == "" {
		http.Error(w, "plugin target address is empty", http.StatusBadGateway)
		return
	}

	wsConn, err := e.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		e.stats.IncError()
		e.logger.Printf("websocket upgrade failed: %v", err)
		return
	}
	defer wsConn.Close()
	wsConn.SetReadLimit(int64(e.cfg.HTTP.MaxWSMessageBytes))

	pluginName := selectedPlugin.Name()
	sessionID := e.stats.StartSession(SessionMeta{
		Plugin:     pluginName,
		Transport:  "websocket",
		Network:    targetNetwork,
		RemoteAddr: r.RemoteAddr,
		TargetAddr: targetAddress,
	})
	defer e.stats.EndSession(sessionID)

	if targetNetwork == "udp" {
		// Use packet socket so replies from alternate source ports are also accepted.
		targetAddr, resolveErr := net.ResolveUDPAddr("udp", targetAddress)
		if resolveErr != nil {
			e.stats.IncErrorPlugin(pluginName)
			http.Error(w, "target resolve failed", http.StatusBadGateway)
			return
		}

		packetConn, listenErr := net.ListenPacket("udp", "0.0.0.0:0")
		if listenErr != nil {
			e.stats.IncErrorPlugin(pluginName)
			http.Error(w, "target connect failed", http.StatusBadGateway)
			return
		}
		defer packetConn.Close()

		if err := e.proxyWebSocketUDP(wsConn, packetConn, targetAddr, selectedPlugin, sessionID); err != nil {
			e.stats.IncErrorPlugin(pluginName)
			e.logger.Printf("websocket udp proxy ended with error: %v", err)
		}
		return
	}

	serverConn, err := net.DialTimeout(
		targetNetwork,
		targetAddress,
		e.cfg.DialTimeoutDuration(),
	)
	if err != nil {
		e.stats.IncErrorPlugin(pluginName)
		http.Error(w, "target connect failed", http.StatusBadGateway)
		return
	}
	defer serverConn.Close()

	e.applyTCPOptions(serverConn)

	if err := e.proxyWebSocket(wsConn, serverConn, selectedPlugin, sessionID); err != nil {
		e.stats.IncErrorPlugin(pluginName)
		e.logger.Printf("websocket proxy ended with error: %v", err)
	}
}

func (e *Engine) proxyWebSocket(
	wsConn *websocket.Conn,
	serverConn net.Conn,
	selectedPlugin *plugin.RuntimePlugin,
	sessionID string,
) error {
	errCh := make(chan error, 3)
	wsIdleTimeout := e.cfg.HTTPWSIdleTimeoutDuration()
	wsPingInterval := e.cfg.HTTPWSPingIntervalDuration()
	done := make(chan struct{})
	var wsWriteMu sync.Mutex

	writeWS := func(payload []byte) error {
		wsWriteMu.Lock()
		defer wsWriteMu.Unlock()
		if wsIdleTimeout > 0 {
			_ = wsConn.SetWriteDeadline(time.Now().Add(wsIdleTimeout))
		}
		return wsConn.WriteMessage(websocket.BinaryMessage, payload)
	}

	writeControl := func(messageType int, payload []byte, deadline time.Time) error {
		wsWriteMu.Lock()
		defer wsWriteMu.Unlock()
		return wsConn.WriteControl(messageType, payload, deadline)
	}

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
					if err := writeControl(
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
		for {
			if wsIdleTimeout > 0 {
				_ = wsConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
			}
			msgType, data, err := wsConn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
				continue
			}

			payload := data
			if !selectedPlugin.Passthrough() {
				payload, err = selectedPlugin.ProcessClientData(data)
				if err != nil {
					errCh <- err
					return
				}
			}

			if len(payload) == 0 {
				continue
			}

			if _, err := serverConn.Write(payload); err != nil {
				errCh <- err
				return
			}
			e.stats.AddSessionRx(sessionID, uint64(len(payload)))
		}
	}()

	go func() {
		buffer := e.bufferPool.Get().([]byte)
		defer e.bufferPool.Put(buffer)

		for {
			n, readErr := serverConn.Read(buffer)
			if n > 0 {
				payload := buffer[:n]
				if !selectedPlugin.Passthrough() {
					processed, processErr := selectedPlugin.ProcessServerData(payload)
					if processErr != nil {
						errCh <- processErr
						return
					}
					payload = processed
				}

				if len(payload) == 0 {
					continue
				}

				if err := writeWS(payload); err != nil {
					errCh <- err
					return
				}
				e.stats.AddSessionTx(sessionID, uint64(len(payload)))
			}

			if readErr != nil {
				errCh <- readErr
				return
			}
		}
	}()

	firstErr := <-errCh
	close(done)
	_ = wsConn.Close()
	_ = serverConn.Close()
	<-errCh
	return normalizeProxyError(firstErr)
}

func (e *Engine) proxyWebSocketUDP(
	wsConn *websocket.Conn,
	packetConn net.PacketConn,
	targetAddr net.Addr,
	selectedPlugin *plugin.RuntimePlugin,
	sessionID string,
) error {
	errCh := make(chan error, 3)
	wsIdleTimeout := e.cfg.HTTPWSIdleTimeoutDuration()
	wsPingInterval := e.cfg.HTTPWSPingIntervalDuration()
	done := make(chan struct{})
	var wsWriteMu sync.Mutex
	replyPolicy := selectedPlugin.UDPReplyPolicy()
	var wsOversizeLogOnce sync.Once
	var wsMTULogOnce sync.Once
	var targetOversizeLogOnce sync.Once
	var targetMTULogOnce sync.Once

	writeWS := func(payload []byte) error {
		wsWriteMu.Lock()
		defer wsWriteMu.Unlock()
		if wsIdleTimeout > 0 {
			_ = wsConn.SetWriteDeadline(time.Now().Add(wsIdleTimeout))
		}
		return wsConn.WriteMessage(websocket.BinaryMessage, payload)
	}

	writeControl := func(messageType int, payload []byte, deadline time.Time) error {
		wsWriteMu.Lock()
		defer wsWriteMu.Unlock()
		return wsConn.WriteControl(messageType, payload, deadline)
	}

	var peerSession *udpPeerSession
	if selectedPlugin.UDPPeerBroadcast() {
		peerSession = &udpPeerSession{
			send: writeWS,
		}
		unregister := e.udpPeers.register(selectedPlugin.Name(), peerSession)
		defer unregister()
	}

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
					if err := writeControl(
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
		for {
			if wsIdleTimeout > 0 {
				_ = wsConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
			}
			msgType, data, err := wsConn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
				continue
			}

			payload := data
			if !selectedPlugin.Passthrough() {
				payload, err = selectedPlugin.ProcessClientData(data)
				if err != nil {
					errCh <- err
					return
				}
			}

			if len(payload) == 0 {
				continue
			}
			if len(payload) > e.cfg.TCP.BufferSize {
				wsOversizeLogOnce.Do(func() {
					e.logger.Printf("udp websocket payload dropped before target write: plugin=%s size=%d max_payload=%d reason=truncation_guard", selectedPlugin.Name(), len(payload), e.cfg.TCP.BufferSize)
				})
				continue
			}
			if udputil.AboveConservativeMTU(len(payload)) {
				wsMTULogOnce.Do(func() {
					e.logger.Printf("udp datagram exceeds conservative WAN MTU: path=websocket_to_target plugin=%s size=%d threshold=%d", selectedPlugin.Name(), len(payload), udputil.ConservativeMTUPayloadBytes)
				})
			}

			if _, err := packetConn.WriteTo(payload, targetAddr); err != nil {
				errCh <- err
				return
			}
			now := time.Now()
			e.udpRecorder.RecordPacket("websocket_to_target", selectedPlugin.Name(), sessionID, "websocket", targetAddr.String(), payload)
			e.stats.AddSessionRx(sessionID, uint64(len(payload)))
			e.stats.ObserveSessionUDPRx(sessionID, now, payload)
			if peerSession != nil {
				if sent := e.udpPeers.broadcast(selectedPlugin.Name(), peerSession, payload); sent > 0 {
					e.stats.AddSessionTx(sessionID, uint64(len(payload)*sent))
					for i := 0; i < sent; i++ {
						e.udpRecorder.RecordPacket("peer_broadcast", selectedPlugin.Name(), sessionID, "websocket", "peer_websocket", payload)
						e.stats.ObserveSessionUDPTx(sessionID, now, payload)
					}
				}
			}
		}
	}()

	go func() {
		buffer := make([]byte, udputil.ReadBufferSize(e.cfg.TCP.BufferSize))

		for {
			n, sourceAddr, oversized, readErr := udputil.ReadDatagram(packetConn, buffer, e.cfg.TCP.BufferSize)
			if n > 0 {
				if oversized {
					targetOversizeLogOnce.Do(func() {
						e.logger.Printf("udp target reply dropped before websocket: plugin=%s source=%s read=%d max_payload=%d reason=truncation_guard", selectedPlugin.Name(), sourceAddr, n, e.cfg.TCP.BufferSize)
					})
					continue
				}
				if !udpReplyAllowed(replyPolicy, targetAddr, sourceAddr) {
					continue
				}
				if udputil.AboveConservativeMTU(n) {
					targetMTULogOnce.Do(func() {
						e.logger.Printf("udp datagram exceeds conservative WAN MTU: path=target_to_websocket plugin=%s size=%d threshold=%d", selectedPlugin.Name(), n, udputil.ConservativeMTUPayloadBytes)
					})
				}
				payload := buffer[:n]
				if !selectedPlugin.Passthrough() {
					processed, processErr := selectedPlugin.ProcessServerData(payload)
					if processErr != nil {
						errCh <- processErr
						return
					}
					payload = processed
				}

				if len(payload) == 0 {
					continue
				}

				if err := writeWS(payload); err != nil {
					errCh <- err
					return
				}
				now := time.Now()
				e.udpRecorder.RecordPacket("target_to_websocket", selectedPlugin.Name(), sessionID, addrString(sourceAddr), "websocket", payload)
				e.stats.AddSessionTx(sessionID, uint64(len(payload)))
				e.stats.ObserveSessionUDPTx(sessionID, now, payload)
			}

			if readErr != nil {
				errCh <- readErr
				return
			}
		}
	}()

	firstErr := <-errCh
	close(done)
	_ = wsConn.Close()
	_ = packetConn.Close()
	<-errCh
	return normalizeProxyError(firstErr)
}

func (e *Engine) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s := e.stats.Snapshot()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":             "ok",
		"uptime_seconds":     s.UptimeSeconds,
		"active_connections": s.ActiveConnections,
		"total_connections":  s.TotalConnections,
		"errors":             s.Errors,
		"default_plugin":     e.defaultPlugin.Info(),
		"enabled_plugins":    e.cfg.Plugins.Enabled,
	})
}

func (e *Engine) handlePing(w http.ResponseWriter, r *http.Request, plane string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := time.Now().UTC()
	s := e.stats.Snapshot()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service":          "road-proxy-v3",
		"plane":            plane,
		"server_time":      now.Format(time.RFC3339Nano),
		"server_unix_nano": now.UnixNano(),
		"uptime_seconds":   s.UptimeSeconds,
	})
}

func (e *Engine) handleStats(w http.ResponseWriter, _ *http.Request) {
	s := e.stats.Snapshot()
	writeJSON(w, http.StatusOK, s)
}

func (e *Engine) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessions := e.stats.SessionsSnapshot()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active":   len(sessions),
		"sessions": sessions,
	})
}

func (e *Engine) handlePlugins(w http.ResponseWriter, _ *http.Request) {
	if !e.allowPluginAPI(w) {
		return
	}
	available, err := e.loader.ListAvailable()
	if err != nil {
		e.stats.IncError()
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":   e.cfg.Plugins.Enabled,
		"default":   e.defaultPlugin.Info(),
		"loaded":    e.loadedPluginInfos(),
		"available": available,
	})
}

func (e *Engine) handlePluginInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !e.allowPluginAPI(w) {
		return
	}

	pluginName, err := extractPluginName(r.URL.Path, "/api/plugin/info/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pluginJSON, err := e.readPluginJSON(pluginName, "plugin.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "plugin not found", http.StatusNotFound)
			return
		}
		e.stats.IncError()
		http.Error(w, "plugin info load failed", http.StatusInternalServerError)
		return
	}

	writeJSONRaw(w, http.StatusOK, pluginJSON)
}

func (e *Engine) handlePluginConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !e.allowPluginAPI(w) {
		return
	}

	pluginName, err := extractPluginName(r.URL.Path, "/api/plugin/config/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	configJSON, err := e.readPluginJSON(pluginName, "config.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "plugin config not found", http.StatusNotFound)
			return
		}
		e.stats.IncError()
		http.Error(w, "plugin config load failed", http.StatusInternalServerError)
		return
	}

	writeJSONRaw(w, http.StatusOK, configJSON)
}

func (e *Engine) handlePluginDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !e.allowPluginAPI(w) {
		return
	}

	pluginName, err := extractPluginName(r.URL.Path, "/api/plugin/download/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pluginJSON, err := e.readPluginJSON(pluginName, "plugin.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "plugin not found", http.StatusNotFound)
			return
		}
		e.stats.IncError()
		http.Error(w, "plugin download failed", http.StatusInternalServerError)
		return
	}

	bundle := map[string]interface{}{
		"name":       pluginName,
		"plugin":     json.RawMessage(pluginJSON),
		"has_config": false,
	}

	configJSON, err := e.readPluginJSON(pluginName, "config.json")
	if err == nil {
		bundle["config"] = json.RawMessage(configJSON)
		bundle["has_config"] = true
	} else if !errors.Is(err, os.ErrNotExist) {
		e.stats.IncError()
		http.Error(w, "plugin config load failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, bundle)
}

func (e *Engine) handleDataInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service": "road-proxy-v3",
		"plane":   "data",
		"ws_path": e.cfg.HTTP.WSEndpoint,
		"auth": map[string]interface{}{
			"enabled": e.cfg.WSAuthEnabled(),
			"header":  e.cfg.WSAuthHeader(),
		},
		"default_plugin":  e.defaultPlugin.Name(),
		"default_network": e.defaultPlugin.TargetNetwork(),
	})
}

func (e *Engine) withDataAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !e.validateWSToken(r) {
			e.stats.IncError()
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (e *Engine) handleControlInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service":        "road-proxy-v3",
		"plane":          "control",
		"listen":         e.cfg.Control.ListenAddr,
		"control_listen": e.cfg.Control.ListenAddr,
		"data_listen":    e.cfg.HTTP.ListenAddr,
		"ws_path":        e.cfg.HTTP.WSEndpoint,
		"auth": map[string]interface{}{
			"enabled":      e.cfg.WSAuthEnabled(),
			"header":       e.cfg.WSAuthHeader(),
			"tokens_count": len(e.cfg.WSAuthTokens()),
		},
		"security": map[string]interface{}{
			"auth_enabled":           e.cfg.WSAuthEnabled(),
			"auth_header":            e.cfg.WSAuthHeader(),
			"tokens_count":           len(e.cfg.WSAuthTokens()),
			"allowed_hosts":          e.cfg.HTTP.AllowedHosts,
			"allowed_origins":        e.cfg.HTTP.AllowedOrigins,
			"trust_proxy_headers":    e.cfg.HTTP.TrustProxyHeaders,
			"max_connections":        e.cfg.HTTP.MaxConnections,
			"max_connections_per_ip": e.cfg.HTTP.MaxConnectionsPerIP,
			"rate_limit_per_minute":  e.cfg.HTTP.RateLimitPerMinute,
			"plugin_api_public":      e.cfg.Control.PluginAPIPublic,
		},
		"runtime": map[string]interface{}{
			"buffer_size":          e.cfg.TCP.BufferSize,
			"ws_idle_timeout":      e.cfg.HTTP.WSIdleTimeout,
			"ws_ping_interval":     e.cfg.HTTP.WSPingInterval,
			"max_ws_message_bytes": e.cfg.HTTP.MaxWSMessageBytes,
			"logging_format":       e.cfg.Logging.Format,
			"udp_record_enabled":   e.cfg.UDPRecord.Enabled,
			"udp_record_path":      e.cfg.UDPRecord.Path,
		},
		"enabled_plugins": e.cfg.Plugins.Enabled,
		"default_plugin":  e.defaultPlugin.Name(),
		"default_target":  e.defaultPlugin.TargetAddress(),
		"default_network": e.defaultPlugin.TargetNetwork(),
	})
}

func (e *Engine) handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service": "road-proxy-v3",
		"routes": []string{
			"/api/health",
			"/api/ping",
			"/api/stats",
			"/api/sessions",
			"/api/plugins",
			"/api/plugin/info/{name}",
			"/api/plugin/config/{name}",
			"/api/plugin/download/{name}",
			"/api/info",
			"/dashboard",
		},
	})
}

func writeJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONRaw(w http.ResponseWriter, code int, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(payload)
}

func (e *Engine) validateWSToken(r *http.Request) bool {
	tokens := e.cfg.WSAuthTokens()
	if len(tokens) == 0 {
		return true
	}

	headerName := e.cfg.WSAuthHeader()
	provided := strings.TrimSpace(r.Header.Get(headerName))
	if provided == "" && !strings.EqualFold(headerName, "Authorization") {
		provided = strings.TrimSpace(r.Header.Get("Authorization"))
	}
	if provided == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(provided), "bearer ") {
		provided = strings.TrimSpace(provided[len("bearer "):])
	}

	for _, token := range tokens {
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

func (e *Engine) controlAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDashboardShellRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		if !e.validateWSToken(r) {
			e.stats.IncError()
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isDashboardShellRequest(r *http.Request) bool {
	return r.Method == http.MethodGet && r.URL.Path == "/dashboard"
}

func (e *Engine) loadedPluginInfos() []plugin.Info {
	names := make([]string, 0, len(e.enabledPlugin))
	for name := range e.enabledPlugin {
		names = append(names, name)
	}
	sort.Strings(names)

	infos := make([]plugin.Info, 0, len(names))
	for _, name := range names {
		infos = append(infos, e.enabledPlugin[name].Info())
	}

	return infos
}

func extractPluginName(path, prefix string) (string, error) {
	if !strings.HasPrefix(path, prefix) {
		return "", fmt.Errorf("invalid plugin endpoint")
	}

	name := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if name == "" {
		return "", fmt.Errorf("plugin name is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid plugin name")
	}

	return name, nil
}

func (e *Engine) readPluginJSON(pluginName, fileName string) ([]byte, error) {
	path := filepath.Join(e.cfg.Plugins.Dir, pluginName, fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid json in %s: %w", path, err)
	}
	return data, nil
}
