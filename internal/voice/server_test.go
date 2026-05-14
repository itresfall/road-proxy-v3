package voice

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestServerHealth(t *testing.T) {
	server, err := NewServer(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestServerStatsEndpoint(t *testing.T) {
	server, err := NewServer(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var stats struct {
		Room        string `json:"room"`
		ActiveUsers int    `json:"active_users"`
		MaxClients  int    `json:"max_clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("stats JSON error = %v", err)
	}
	if stats.Room != defaultRoomName || stats.ActiveUsers != 0 || stats.MaxClients != defaultMaxClients {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestWebSocketJoinSendsUsers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PingInterval = "1h"
	server, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + cfg.WSEndpoint
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(ControlMessage{Type: "join", Name: "tester"}); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	_, firstPayload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(joined) error = %v", err)
	}
	var joined ControlMessage
	if err := json.Unmarshal(firstPayload, &joined); err != nil {
		t.Fatalf("joined JSON error = %v", err)
	}
	if joined.Type != "joined" || joined.Name != "tester" {
		t.Fatalf("joined = %+v, want type=joined name=tester", joined)
	}

	_, usersPayload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(users) error = %v", err)
	}
	var users ControlMessage
	if err := json.Unmarshal(usersPayload, &users); err != nil {
		t.Fatalf("users JSON error = %v", err)
	}
	if users.Type != "users" || len(users.Users) != 1 || users.Users[0].Name != "tester" {
		t.Fatalf("users = %+v, want tester user", users)
	}
}

func TestWebSocketStateUpdatesUsers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PingInterval = "1h"
	server, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + cfg.WSEndpoint
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(ControlMessage{Type: "join", Name: "tester"}); err != nil {
		t.Fatalf("WriteJSON(join) error = %v", err)
	}
	readControlMessage(t, conn, "joined")
	readControlMessage(t, conn, "users")

	muted := true
	if err := conn.WriteJSON(ControlMessage{Type: "state", Muted: &muted}); err != nil {
		t.Fatalf("WriteJSON(state) error = %v", err)
	}

	users := readControlMessage(t, conn, "users")
	if len(users.Users) != 1 || !users.Users[0].Muted {
		t.Fatalf("expected muted user update, got %+v", users)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	var stats struct {
		TotalJoins      uint64 `json:"total_joins"`
		ControlMessages uint64 `json:"control_messages"`
		StateChanges    uint64 `json:"state_changes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("stats JSON error = %v", err)
	}
	if stats.TotalJoins != 1 || stats.ControlMessages != 1 || stats.StateChanges != 1 {
		t.Fatalf("unexpected stats after state update: %+v", stats)
	}
}

func readControlMessage(t *testing.T, conn *websocket.Conn, wantType string) ControlMessage {
	t.Helper()

	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(%s) error = %v", wantType, err)
	}
	var msg ControlMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("%s JSON error = %v", wantType, err)
	}
	if msg.Type != wantType {
		t.Fatalf("message type = %q, want %q: %+v", msg.Type, wantType, msg)
	}
	return msg
}
