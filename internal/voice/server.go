package voice

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Server struct {
	cfg    *Config
	logger *log.Logger
	room   *Room

	httpServer        *http.Server
	readTimeout       time.Duration
	writeTimeout      time.Duration
	pingInterval      time.Duration
	clientIdleTimeout time.Duration

	upgrader websocket.Upgrader
	stats    voiceStats
}

type voiceStats struct {
	startedAt         time.Time
	totalJoins        atomic.Uint64
	controlMessages   atomic.Uint64
	stateChanges      atomic.Uint64
	audioFramesRx     atomic.Uint64
	audioFramesTx     atomic.Uint64
	audioBytesRx      atomic.Uint64
	audioBytesTx      atomic.Uint64
	audioDroppedMuted atomic.Uint64
	errors            atomic.Uint64
}

type statsSnapshot struct {
	StartTime         string `json:"start_time"`
	UptimeSeconds     int64  `json:"uptime_seconds"`
	Room              string `json:"room"`
	ActiveUsers       int    `json:"active_users"`
	MaxClients        int    `json:"max_clients"`
	TotalJoins        uint64 `json:"total_joins"`
	ControlMessages   uint64 `json:"control_messages"`
	StateChanges      uint64 `json:"state_changes"`
	AudioFramesRx     uint64 `json:"audio_frames_rx"`
	AudioFramesTx     uint64 `json:"audio_frames_tx"`
	AudioBytesRx      uint64 `json:"audio_bytes_rx"`
	AudioBytesTx      uint64 `json:"audio_bytes_tx"`
	AudioDroppedMuted uint64 `json:"audio_dropped_muted"`
	Errors            uint64 `json:"errors"`
}

func NewServer(cfg *Config, logger *log.Logger) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = log.Default()
	}

	readTimeout, _ := cfg.ReadTimeoutDuration()
	writeTimeout, _ := cfg.WriteTimeoutDuration()
	pingInterval, _ := cfg.PingIntervalDuration()
	clientIdleTimeout, _ := cfg.ClientIdleTimeoutDuration()

	s := &Server{
		cfg:               cfg,
		logger:            logger,
		room:              NewRoom(cfg.RoomName, cfg.MaxClients),
		readTimeout:       readTimeout,
		writeTimeout:      writeTimeout,
		pingInterval:      pingInterval,
		clientIdleTimeout: clientIdleTimeout,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			ReadBufferSize:  cfg.MaxAudioFrameSize,
			WriteBufferSize: cfg.MaxAudioFrameSize,
		},
	}
	s.stats.startedAt = time.Now()

	mux := http.NewServeMux()
	mux.HandleFunc(cfg.WSEndpoint, s.handleWebSocket)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/users", s.handleUsers)
	mux.HandleFunc("/", s.handleRoot)

	s.httpServer = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: readTimeout,
	}

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Printf("voice server started: listen=%s ws=%s room=%s", s.cfg.ListenAddr, s.cfg.WSEndpoint, s.cfg.RoomName)
		err := s.httpServer.ListenAndServe()
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.stats.errors.Add(1)
		s.logger.Printf("voice websocket upgrade failed: %v", err)
		return
	}

	if err := conn.SetReadDeadline(time.Now().Add(s.clientIdleTimeout)); err != nil {
		s.stats.errors.Add(1)
		s.logger.Printf("voice read deadline failed: %v", err)
		_ = conn.Close()
		return
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(s.clientIdleTimeout))
	})
	conn.SetReadLimit(int64(s.cfg.MaxAudioFrameSize))

	client, err := s.readJoin(conn)
	if err != nil {
		s.stats.errors.Add(1)
		writeControlError(conn, s.writeTimeout, err.Error())
		_ = conn.Close()
		return
	}
	if err := s.room.Add(client); err != nil {
		s.stats.errors.Add(1)
		writeControlError(conn, s.writeTimeout, err.Error())
		_ = conn.Close()
		return
	}

	s.stats.totalJoins.Add(1)
	s.logger.Printf("voice client joined: id=%s name=%q", client.id, client.name)
	client.EnqueueJSON(ControlMessage{Type: "joined", ID: client.id, Name: client.name})
	s.room.BroadcastUsers()

	done := make(chan struct{})
	go s.writeLoop(conn, client, done)
	s.readLoop(conn, client)

	s.room.Remove(client.id)
	client.CloseSend()
	<-done
	_ = conn.Close()
	s.room.BroadcastUsers()
	s.logger.Printf("voice client left: id=%s name=%q", client.id, client.name)
}

func (s *Server) readJoin(conn *websocket.Conn) (*Client, error) {
	messageType, payload, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("join read failed: %w", err)
	}
	if messageType != websocket.TextMessage {
		return nil, fmt.Errorf("first message must be join JSON")
	}

	var msg ControlMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("join parse failed: %w", err)
	}
	if msg.Type != "join" {
		return nil, fmt.Errorf("first message must be type=join")
	}
	name := strings.TrimSpace(msg.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(name) > 32 {
		return nil, fmt.Errorf("name is too long")
	}

	id, err := randomID()
	if err != nil {
		return nil, err
	}
	return NewClient(id, name, 64), nil
}

func (s *Server) readLoop(conn *websocket.Conn, client *Client) {
	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := conn.SetReadDeadline(time.Now().Add(s.clientIdleTimeout)); err != nil {
			return
		}

		switch messageType {
		case websocket.TextMessage:
			s.handleControl(client, payload)
		case websocket.BinaryMessage:
			s.stats.audioFramesRx.Add(1)
			s.stats.audioBytesRx.Add(uint64(len(payload)))
			if client.IsMuted() {
				s.stats.audioDroppedMuted.Add(1)
				continue
			}
			delivered := s.room.BroadcastAudio(client.id, payload)
			if delivered > 0 {
				s.stats.audioFramesTx.Add(uint64(delivered))
				s.stats.audioBytesTx.Add(uint64(len(payload) * delivered))
			}
		}
	}
}

func (s *Server) handleControl(client *Client, payload []byte) {
	s.stats.controlMessages.Add(1)
	var msg ControlMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		s.stats.errors.Add(1)
		client.EnqueueJSON(ControlMessage{Type: "error", Error: "invalid control message"})
		return
	}

	switch msg.Type {
	case "state":
		client.SetState(msg.Muted, msg.Deafened)
		s.stats.stateChanges.Add(1)
		s.room.BroadcastUsers()
	case "ping":
		client.EnqueueJSON(ControlMessage{Type: "pong", T: msg.T})
	default:
		s.stats.errors.Add(1)
		client.EnqueueJSON(ControlMessage{Type: "error", Error: "unsupported control message"})
	}
}

func (s *Server) writeLoop(conn *websocket.Conn, client *Client, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(s.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-client.send:
			if !ok {
				_ = conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(s.writeTimeout))
				return
			}
			if err := conn.SetWriteDeadline(time.Now().Add(s.writeTimeout)); err != nil {
				return
			}
			if err := conn.WriteMessage(msg.messageType, msg.payload); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(s.writeTimeout)); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats := s.snapshotStats()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"service":        "road-voice",
		"room":           s.cfg.RoomName,
		"users":          stats.ActiveUsers,
		"uptime_seconds": stats.UptimeSeconds,
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.snapshotStats())
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"users": s.room.Snapshot(),
	})
}

func (s *Server) snapshotStats() statsSnapshot {
	startedAt := s.stats.startedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	return statsSnapshot{
		StartTime:         startedAt.UTC().Format(time.RFC3339),
		UptimeSeconds:     int64(time.Since(startedAt).Seconds()),
		Room:              s.cfg.RoomName,
		ActiveUsers:       len(s.room.Snapshot()),
		MaxClients:        s.cfg.MaxClients,
		TotalJoins:        s.stats.totalJoins.Load(),
		ControlMessages:   s.stats.controlMessages.Load(),
		StateChanges:      s.stats.stateChanges.Load(),
		AudioFramesRx:     s.stats.audioFramesRx.Load(),
		AudioFramesTx:     s.stats.audioFramesTx.Load(),
		AudioBytesRx:      s.stats.audioBytesRx.Load(),
		AudioBytesTx:      s.stats.audioBytesTx.Load(),
		AudioDroppedMuted: s.stats.audioDroppedMuted.Load(),
		Errors:            s.stats.errors.Load(),
	}
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "road-voice",
		"ws":      s.cfg.WSEndpoint,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeControlError(conn *websocket.Conn, writeTimeout time.Duration, message string) {
	payload, err := json.Marshal(ControlMessage{Type: "error", Error: message})
	if err != nil {
		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	_ = conn.WriteMessage(websocket.TextMessage, payload)
}

func randomID() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("random id failed: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
