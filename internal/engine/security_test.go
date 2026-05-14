package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"road-proxy-v3/internal/config"
)

func TestHostAllowedWithAllowlist(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AllowedHosts = []string{"proxy.example.com"}
	e := New(cfg, nil)

	allowed := httptest.NewRequest(http.MethodGet, "http://proxy.example.com/ws", nil)
	if !e.hostAllowed(allowed) {
		t.Fatal("expected allowed host")
	}

	blocked := httptest.NewRequest(http.MethodGet, "http://evil.example.com/ws", nil)
	if e.hostAllowed(blocked) {
		t.Fatal("expected disallowed host to be blocked")
	}
}

func TestCheckWebSocketOriginWithAllowlist(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AllowedHosts = []string{"proxy.example.com"}
	cfg.HTTP.AllowedOrigins = []string{"https://proxy.example.com"}
	e := New(cfg, nil)

	allowed := httptest.NewRequest(http.MethodGet, "http://proxy.example.com/ws", nil)
	allowed.Header.Set("Origin", "https://proxy.example.com")
	if !e.checkWebSocketOrigin(allowed) {
		t.Fatal("expected allowed origin")
	}

	blocked := httptest.NewRequest(http.MethodGet, "http://proxy.example.com/ws", nil)
	blocked.Header.Set("Origin", "https://evil.example.com")
	if e.checkWebSocketOrigin(blocked) {
		t.Fatal("expected disallowed origin to be blocked")
	}
}

func TestAdmitWebSocketRateLimit(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.RateLimitPerMinute = 1
	e := New(cfg, nil)
	req := httptest.NewRequest(http.MethodGet, "http://localhost/ws", nil)
	req.RemoteAddr = "192.0.2.10:50000"

	release, status, reason := e.admitWebSocket(req)
	if release == nil {
		t.Fatalf("first admit failed: status=%d reason=%s", status, reason)
	}
	release()

	release, status, _ = e.admitWebSocket(req)
	if release != nil || status != http.StatusTooManyRequests {
		t.Fatalf("expected rate limit rejection, release=%v status=%d", release != nil, status)
	}
}

func TestAdmitWebSocketConnectionLimit(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.MaxConnections = 1
	e := New(cfg, nil)

	req1 := httptest.NewRequest(http.MethodGet, "http://localhost/ws", nil)
	req1.RemoteAddr = "192.0.2.10:50000"
	release, status, reason := e.admitWebSocket(req1)
	if release == nil {
		t.Fatalf("first admit failed: status=%d reason=%s", status, reason)
	}
	defer release()

	req2 := httptest.NewRequest(http.MethodGet, "http://localhost/ws", nil)
	req2.RemoteAddr = "192.0.2.11:50000"
	release2, status, _ := e.admitWebSocket(req2)
	if release2 != nil || status != http.StatusServiceUnavailable {
		t.Fatalf("expected global connection limit rejection, release=%v status=%d", release2 != nil, status)
	}
}

func TestAdmitWebSocketPerIPConnectionLimit(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.MaxConnectionsPerIP = 1
	e := New(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "http://localhost/ws", nil)
	req.RemoteAddr = "192.0.2.10:50000"
	release, status, reason := e.admitWebSocket(req)
	if release == nil {
		t.Fatalf("first admit failed: status=%d reason=%s", status, reason)
	}
	defer release()

	release2, status, _ := e.admitWebSocket(req)
	if release2 != nil || status != http.StatusTooManyRequests {
		t.Fatalf("expected per-ip connection limit rejection, release=%v status=%d", release2 != nil, status)
	}
}

func TestPluginAPIPrivateRequiresAuthEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.Control.PluginAPIPublic = false
	e := New(cfg, nil)

	rec := httptest.NewRecorder()
	if e.allowPluginAPI(rec) {
		t.Fatal("expected private plugin API to reject when auth is disabled")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", rec.Code)
	}
}
