package client

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"road-proxy-v3/internal/config"
)

func TestBuildWSURLAddsPluginQuery(t *testing.T) {
	cfg := config.DefaultClient()
	cfg.ServerWSURL = "ws://localhost:8080/ws"
	cfg.PluginName = "minecraft"

	tunnel := New(cfg, nil)

	wsURL, err := tunnel.buildWSURL()
	if err != nil {
		t.Fatalf("buildWSURL failed: %v", err)
	}

	if wsURL != "ws://localhost:8080/ws?plugin=minecraft" {
		t.Fatalf("unexpected ws url: %s", wsURL)
	}
}

func TestBuildWSURLKeepsExistingPluginQuery(t *testing.T) {
	cfg := config.DefaultClient()
	cfg.ServerWSURL = "ws://localhost:8080/ws?plugin=valve"
	cfg.PluginName = "minecraft"

	tunnel := New(cfg, nil)

	wsURL, err := tunnel.buildWSURL()
	if err != nil {
		t.Fatalf("buildWSURL failed: %v", err)
	}

	if wsURL != "ws://localhost:8080/ws?plugin=valve" {
		t.Fatalf("unexpected ws url: %s", wsURL)
	}
}

func TestBuildWSURLRejectsInvalidScheme(t *testing.T) {
	cfg := config.DefaultClient()
	cfg.ServerWSURL = "http://localhost:8080/ws"

	tunnel := New(cfg, nil)

	if _, err := tunnel.buildWSURL(); err == nil {
		t.Fatal("expected invalid scheme error")
	}
}

func TestBuildDialHeadersAddsAuthToken(t *testing.T) {
	cfg := config.DefaultClient()
	cfg.AuthHeader = "X-Proxy-Token"
	cfg.AuthToken = "secret-token"

	tunnel := New(cfg, nil)
	header := tunnel.buildDialHeaders()

	if got := header.Get("X-Proxy-Token"); got != "secret-token" {
		t.Fatalf("unexpected auth token header: %q", got)
	}
}

func TestBuildDialHeadersUsesAuthorizationBearer(t *testing.T) {
	cfg := config.DefaultClient()
	cfg.AuthHeader = "Authorization"
	cfg.AuthToken = "secret-token"

	tunnel := New(cfg, nil)
	header := tunnel.buildDialHeaders()

	if got := header.Get("Authorization"); got != "Bearer secret-token" {
		t.Fatalf("unexpected authorization header: %q", got)
	}
}

func TestBuildDialHeadersPreservesCustomHeaders(t *testing.T) {
	cfg := config.DefaultClient()
	cfg.Headers = map[string]string{
		"X-Test": "ok",
	}

	tunnel := New(cfg, nil)
	header := tunnel.buildDialHeaders()

	if got := header.Get("X-Test"); got != "ok" {
		t.Fatalf("unexpected custom header value: %q", got)
	}
}

func TestFormatWebSocketDialErrorUnauthorized(t *testing.T) {
	err := formatWebSocketDialError(
		errors.New("websocket: bad handshake"),
		&http.Response{StatusCode: http.StatusUnauthorized},
	)
	if err == nil {
		t.Fatal("expected formatted error")
	}
	text := err.Error()
	if !strings.Contains(text, "authentication failed") || !strings.Contains(text, "X-ROAD-Token") {
		t.Fatalf("unexpected error text: %s", text)
	}
}

func TestShouldRetryWebSocketDialRejectsAuthFailures(t *testing.T) {
	if shouldRetryWebSocketDial(&http.Response{StatusCode: http.StatusUnauthorized}) {
		t.Fatal("401 should not be retried")
	}
	if shouldRetryWebSocketDial(&http.Response{StatusCode: http.StatusForbidden}) {
		t.Fatal("403 should not be retried")
	}
	if !shouldRetryWebSocketDial(&http.Response{StatusCode: http.StatusBadGateway}) {
		t.Fatal("transient gateway errors should be retried")
	}
}

func TestMinDuration(t *testing.T) {
	a := minDuration(100, 200)
	if a != 100 {
		t.Fatalf("expected 100, got %d", a)
	}
	b := minDuration(300, 200)
	if b != 200 {
		t.Fatalf("expected 200, got %d", b)
	}
}
