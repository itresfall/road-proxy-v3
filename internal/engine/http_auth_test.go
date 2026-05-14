package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"road-proxy-v3/internal/config"
)

func TestValidateWSTokenAllowsWhenAuthDisabled(t *testing.T) {
	cfg := config.Default()

	e := New(cfg, nil)

	req, err := http.NewRequest(http.MethodGet, "http://localhost/ws", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}

	if !e.validateWSToken(req) {
		t.Fatal("expected validateWSToken=true when auth is disabled")
	}
}

func TestValidateWSTokenRejectsWrongToken(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AuthToken = "secret"
	cfg.HTTP.AuthHeader = "X-Proxy-Token"

	e := New(cfg, nil)

	req, err := http.NewRequest(http.MethodGet, "http://localhost/ws", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("X-Proxy-Token", "wrong")

	if e.validateWSToken(req) {
		t.Fatal("expected wrong token to be rejected")
	}
}

func TestValidateWSTokenAcceptsConfiguredHeader(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AuthToken = "secret"
	cfg.HTTP.AuthHeader = "X-Proxy-Token"

	e := New(cfg, nil)

	req, err := http.NewRequest(http.MethodGet, "http://localhost/ws", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("X-Proxy-Token", "secret")

	if !e.validateWSToken(req) {
		t.Fatal("expected configured token to be accepted")
	}
}

func TestValidateWSTokenAcceptsAuthorizationBearer(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AuthToken = "secret"
	cfg.HTTP.AuthHeader = "Authorization"

	e := New(cfg, nil)

	req, err := http.NewRequest(http.MethodGet, "http://localhost/ws", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")

	if !e.validateWSToken(req) {
		t.Fatal("expected bearer token to be accepted")
	}
}

func TestControlAuthMiddlewareRejectsUnauthorized(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AuthToken = "secret"

	e := New(cfg, nil)
	handler := e.controlAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req, err := http.NewRequest(http.MethodGet, "http://localhost/api/stats", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", rec.Code)
	}
}

func TestControlAuthMiddlewareAcceptsAuthorized(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AuthToken = "secret"

	e := New(cfg, nil)
	handler := e.controlAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req, err := http.NewRequest(http.MethodGet, "http://localhost/api/stats", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set(config.DefaultAuthHeaderName, "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected authorized request, got %d", rec.Code)
	}
}

func TestControlAuthMiddlewareAllowsDashboardShell(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AuthToken = "secret"

	e := New(cfg, nil)
	handler := e.controlAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req, err := http.NewRequest(http.MethodGet, "http://localhost/dashboard", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected dashboard shell to pass through auth middleware, got %d", rec.Code)
	}
}
