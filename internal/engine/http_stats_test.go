package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"road-proxy-v3/internal/config"
)

func TestHandleStatsIncludesUDPJitterAndLoss(t *testing.T) {
	e := New(config.Default(), nil)
	start := time.Unix(0, 0)

	e.stats.ObserveUDPRx(start, makeStatsRakNetDatagram(1))
	e.stats.ObserveUDPRx(start.Add(10*time.Millisecond), makeStatsRakNetDatagram(3))
	e.stats.ObserveUDPRx(start.Add(40*time.Millisecond), makeStatsRakNetDatagram(2))

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	e.handleStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}

	var body Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.UDP.RX.LossPackets != 1 {
		t.Fatalf("udp.rx.loss_packets = %d, want 1", body.UDP.RX.LossPackets)
	}
	if body.UDP.RX.JitterMS <= 0 {
		t.Fatalf("expected positive udp.rx.jitter_ms, got %.2f", body.UDP.RX.JitterMS)
	}
}

func TestHandleStatsIncludesPerPluginStats(t *testing.T) {
	e := New(config.Default(), nil)
	e.stats.SessionStartPlugin("game")
	e.stats.AddRxPlugin("game", 12)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	e.handleStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}

	var body Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	game, ok := body.Plugins["game"]
	if !ok {
		t.Fatalf("game plugin stats missing: %#v", body.Plugins)
	}
	if game.ActiveConnections != 1 || game.TotalBytesRx != 12 {
		t.Fatalf("unexpected game stats: %#v", game)
	}
}

func TestHandleSessionsReturnsActiveSessionList(t *testing.T) {
	e := New(config.Default(), nil)
	sessionID := e.stats.StartSession(SessionMeta{
		Plugin:     "game",
		Transport:  "websocket",
		Network:    "udp",
		RemoteAddr: "192.0.2.10:50000",
		TargetAddr: "127.0.0.1:7777",
	})
	e.stats.AddSessionTx(sessionID, 42)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	rec := httptest.NewRecorder()
	e.handleSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Active   int               `json:"active"`
		Sessions []SessionSnapshot `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Active != 1 || len(body.Sessions) != 1 {
		t.Fatalf("unexpected sessions body: %#v", body)
	}
	if body.Sessions[0].ID != sessionID || body.Sessions[0].BytesTx != 42 {
		t.Fatalf("unexpected session: %#v", body.Sessions[0])
	}
}

func TestHandleSessionsRejectsNonGET(t *testing.T) {
	e := New(config.Default(), nil)

	req := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	rec := httptest.NewRecorder()
	e.handleSessions(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: got=%d", rec.Code)
	}
}
