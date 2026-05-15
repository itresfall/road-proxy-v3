package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/plugin"
	"road-proxy-v3/internal/udprecord"
)

type Engine struct {
	cfg    *config.Config
	logger *log.Logger
	loader *plugin.Loader

	bufferPool sync.Pool

	stats         *Stats
	enabledPlugin map[string]*plugin.RuntimePlugin
	defaultPlugin *plugin.RuntimePlugin
	udpPeers      *udpPeerHub
	udpRecorder   *udprecord.Recorder
	wsSecurity    websocketSecurityState

	tcpListener   net.Listener
	dataServer    *http.Server
	controlServer *http.Server
	wsUpgrader    websocket.Upgrader
}

func New(cfg *config.Config, logger *log.Logger) *Engine {
	if logger == nil {
		logger = log.Default()
	}

	e := &Engine{
		cfg:      cfg,
		logger:   logger,
		loader:   plugin.NewLoader(cfg.Plugins.Dir),
		stats:    NewStats(),
		udpPeers: newUDPPeerHub(),
		wsSecurity: websocketSecurityState{
			activeByIP: map[string]int{},
			rateByIP:   map[string]*websocketRateWindow{},
		},
	}

	e.bufferPool = sync.Pool{
		New: func() any {
			return make([]byte, cfg.TCP.BufferSize)
		},
	}

	return e
}

func (e *Engine) Start(ctx context.Context) error {
	if err := e.loadEnabledPlugins(); err != nil {
		return err
	}
	if err := e.setupUDPRecorder(); err != nil {
		return err
	}
	defer e.closeUDPRecorder()

	if err := e.setupTCPListener(); err != nil {
		return err
	}
	if err := e.setupDataServer(); err != nil {
		e.closeStartupResources()
		return err
	}
	if err := e.setupControlServer(); err != nil {
		e.closeStartupResources()
		return err
	}

	errCh := make(chan error, 3)
	go e.runTCPAcceptLoop(ctx, errCh)
	if e.dataServer != nil {
		go e.serveHTTP("data-plane", e.dataServer, errCh)
	}
	if e.controlServer != nil {
		go e.serveHTTP("control-plane", e.controlServer, errCh)
	}

	e.logger.Printf(
		"v3 engine started: tcp=%s data=%s ws=%s control=%s default_plugin=%s target=%s enabled=%v",
		e.cfg.TCP.ListenAddr,
		e.cfg.HTTP.ListenAddr,
		e.cfg.HTTP.WSEndpoint,
		e.cfg.Control.ListenAddr,
		e.defaultPlugin.Name(),
		e.defaultPlugin.TargetAddress(),
		e.cfg.Plugins.Enabled,
	)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return e.shutdown(shutdownCtx)
	case err := <-errCh:
		e.stats.IncError()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = e.shutdown(shutdownCtx)
		return err
	}
}

func (e *Engine) setupUDPRecorder() error {
	recorder, err := udprecord.New(udprecord.Options{
		Enabled:   e.cfg.UDPRecord.Enabled,
		Path:      e.cfg.UDPRecord.Path,
		Role:      "server",
		Duration:  e.cfg.UDPRecord.DurationValue(),
		MaxEvents: e.cfg.UDPRecord.MaxEvents,
	}, e.logger)
	if err != nil {
		return err
	}
	e.udpRecorder = recorder
	return nil
}

func (e *Engine) closeUDPRecorder() {
	if err := e.udpRecorder.Close(); err != nil {
		e.logger.Printf("udp recorder close failed: %v", err)
	}
}

func (e *Engine) closeStartupResources() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.shutdown(shutdownCtx); err != nil {
		e.logger.Printf("startup cleanup failed: %v", err)
	}
}

func (e *Engine) loadEnabledPlugins() error {
	loadedPlugins, err := e.loader.LoadEnabled(e.cfg.Plugins.Enabled)
	if err != nil {
		return fmt.Errorf("load enabled plugins: %w", err)
	}

	if len(loadedPlugins) == 0 {
		return fmt.Errorf("at least 1 enabled plugin is required")
	}

	e.enabledPlugin = loadedPlugins

	defaultName := strings.TrimSpace(e.cfg.Plugins.Enabled[0])
	defaultPlugin, ok := loadedPlugins[defaultName]
	if !ok {
		return fmt.Errorf("default plugin %q is not loaded", defaultName)
	}
	e.defaultPlugin = defaultPlugin
	enabledNames := make([]string, 0, len(loadedPlugins))
	for _, rawName := range e.cfg.Plugins.Enabled {
		name := strings.TrimSpace(rawName)
		if name != "" {
			enabledNames = append(enabledNames, name)
		}
	}
	e.stats.RegisterPlugins(enabledNames)
	for _, name := range enabledNames {
		if p := loadedPlugins[name]; p != nil && p.UDPPeerBroadcast() {
			e.logger.Printf("warning: plugin %s has udp_peer_broadcast enabled; use only for proven peer/lockstep UDP games", name)
		}
	}

	return nil
}

func (e *Engine) setupTCPListener() error {
	listener, err := net.Listen("tcp", e.cfg.TCP.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", e.cfg.TCP.ListenAddr, err)
	}
	e.tcpListener = listener
	return nil
}

func (e *Engine) runTCPAcceptLoop(ctx context.Context, errCh chan<- error) {
	for {
		conn, err := e.tcpListener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			errCh <- fmt.Errorf("tcp accept error: %w", err)
			return
		}
		go e.handleTCPConnection(conn)
	}
}

func (e *Engine) handleTCPConnection(clientConn net.Conn) {
	selectedPlugin := e.defaultPlugin
	pluginName := selectedPlugin.Name()

	sessionID := e.stats.StartSession(SessionMeta{
		Plugin:     pluginName,
		Transport:  "tcp",
		Network:    selectedPlugin.TargetNetwork(),
		RemoteAddr: addrString(clientConn.RemoteAddr()),
		TargetAddr: selectedPlugin.TargetAddress(),
	})
	defer e.stats.EndSession(sessionID)
	defer clientConn.Close()

	e.applyTCPOptions(clientConn)

	serverConn, err := net.DialTimeout(
		selectedPlugin.TargetNetwork(),
		selectedPlugin.TargetAddress(),
		e.cfg.DialTimeoutDuration(),
	)
	if err != nil {
		e.stats.IncErrorPlugin(pluginName)
		e.logger.Printf("tcp dial target failed: %v", err)
		return
	}
	defer serverConn.Close()

	e.applyTCPOptions(serverConn)

	if selectedPlugin.Passthrough() {
		if err := e.proxyPassthrough(clientConn, serverConn, sessionID); err != nil {
			e.stats.IncErrorPlugin(pluginName)
			e.logger.Printf("tcp passthrough ended with error: %v", err)
		}
		return
	}

	if err := e.proxyWithPipeline(clientConn, serverConn, selectedPlugin, sessionID); err != nil {
		e.stats.IncErrorPlugin(pluginName)
		e.logger.Printf("tcp pipeline proxy ended with error: %v", err)
	}
}

func (e *Engine) proxyPassthrough(clientConn, serverConn net.Conn, sessionID string) error {
	errCh := make(chan error, 2)

	go func() {
		errCh <- e.copyBuffered(serverConn, clientConn, nil, func(n int) {
			e.stats.AddSessionRx(sessionID, uint64(n))
		})
	}()
	go func() {
		errCh <- e.copyBuffered(clientConn, serverConn, nil, func(n int) {
			e.stats.AddSessionTx(sessionID, uint64(n))
		})
	}()

	firstErr := <-errCh
	_ = clientConn.Close()
	_ = serverConn.Close()
	<-errCh

	return normalizeProxyError(firstErr)
}

func (e *Engine) proxyWithPipeline(clientConn, serverConn net.Conn, selectedPlugin *plugin.RuntimePlugin, sessionID string) error {
	errCh := make(chan error, 2)

	go func() {
		errCh <- e.copyBuffered(serverConn, clientConn, selectedPlugin.ProcessClientData, func(n int) {
			e.stats.AddSessionRx(sessionID, uint64(n))
		})
	}()
	go func() {
		errCh <- e.copyBuffered(clientConn, serverConn, selectedPlugin.ProcessServerData, func(n int) {
			e.stats.AddSessionTx(sessionID, uint64(n))
		})
	}()

	firstErr := <-errCh
	_ = clientConn.Close()
	_ = serverConn.Close()
	<-errCh

	return normalizeProxyError(firstErr)
}

func (e *Engine) copyBuffered(
	dst io.Writer,
	src io.Reader,
	processor func([]byte) ([]byte, error),
	onBytesWritten func(int),
) error {
	buffer := e.bufferPool.Get().([]byte)
	defer e.bufferPool.Put(buffer)

	for {
		n, readErr := src.Read(buffer)
		if n > 0 {
			payload := buffer[:n]
			if processor != nil {
				processed, processErr := processor(payload)
				if processErr != nil {
					return processErr
				}
				payload = processed
			}

			if len(payload) > 0 {
				if _, err := dst.Write(payload); err != nil {
					return err
				}
				if onBytesWritten != nil {
					onBytesWritten(len(payload))
				}
			}
		}

		if readErr != nil {
			return readErr
		}
	}
}

func (e *Engine) shutdown(ctx context.Context) error {
	var firstErr error

	if e.tcpListener != nil {
		if err := e.tcpListener.Close(); err != nil && !errors.Is(err, net.ErrClosed) && firstErr == nil {
			firstErr = err
		}
	}
	if e.dataServer != nil {
		if err := e.dataServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) && firstErr == nil {
			firstErr = err
		}
	}
	if e.controlServer != nil {
		if err := e.controlServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) && firstErr == nil {
			firstErr = err
		}
	}

	if firstErr == nil {
		e.logger.Printf("v3 engine stopped")
	}

	return firstErr
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

func (e *Engine) applyTCPOptions(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}

	_ = tcpConn.SetNoDelay(true)
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(e.cfg.KeepAliveDuration())
}

func addrString(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

func (e *Engine) resolvePlugin(name string) (*plugin.RuntimePlugin, error) {
	requested := strings.TrimSpace(name)
	if requested == "" {
		return e.defaultPlugin, nil
	}

	p, ok := e.enabledPlugin[requested]
	if !ok {
		return nil, fmt.Errorf("requested plugin %q is not enabled", requested)
	}

	return p, nil
}
