package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"road-proxy-v3/internal/config"
)

func TestHandlePingReturnsServerTime(t *testing.T) {
	e := New(config.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	rec := httptest.NewRecorder()
	e.handlePing(rec, req, "control")

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Service        string `json:"service"`
		Plane          string `json:"plane"`
		ServerTime     string `json:"server_time"`
		ServerUnixNano int64  `json:"server_unix_nano"`
		UptimeSeconds  int64  `json:"uptime_seconds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Service != "road-proxy-v3" {
		t.Fatalf("service = %q", body.Service)
	}
	if body.Plane != "control" {
		t.Fatalf("plane = %q", body.Plane)
	}
	if body.ServerTime == "" || body.ServerUnixNano <= 0 {
		t.Fatalf("missing server time fields: %#v", body)
	}
	if body.UptimeSeconds < 0 {
		t.Fatalf("uptime_seconds = %d", body.UptimeSeconds)
	}
}

func TestHandlePingRejectsNonGET(t *testing.T) {
	e := New(config.Default(), nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ping", nil)
	rec := httptest.NewRecorder()
	e.handlePing(rec, req, "control")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: got=%d", rec.Code)
	}
}
