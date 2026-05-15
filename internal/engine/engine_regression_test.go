package engine

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"road-proxy-v3/internal/config"
)

type readDataAndEOF struct {
	calls int
}

func (r *readDataAndEOF) Read(p []byte) (int, error) {
	r.calls++
	if r.calls > 1 {
		return 0, io.EOF
	}
	copy(p, "x")
	return 1, io.EOF
}

func TestCopyBufferedPreservesReadErrorWithPayload(t *testing.T) {
	e := New(config.Default(), nil)
	src := &readDataAndEOF{}
	var dst bytes.Buffer

	err := e.copyBuffered(&dst, src, func(payload []byte) ([]byte, error) {
		return payload, nil
	}, nil)

	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if got := dst.String(); got != "x" {
		t.Fatalf("unexpected copied payload: %q", got)
	}
	if src.calls != 1 {
		t.Fatalf("read error was not preserved; source read calls = %d", src.calls)
	}
}

func TestAdmitWebSocketPrunesExpiredRateWindows(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.RateLimitPerMinute = 100
	e := New(cfg, nil)

	now := time.Now()
	e.wsSecurity.rateByIP["198.51.100.10"] = &websocketRateWindow{start: now.Add(-websocketRateWindowTTL - time.Second)}
	e.wsSecurity.lastRatePrune = now.Add(-time.Minute - time.Second)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.RemoteAddr = "203.0.113.20:12345"
	release, status, reason := e.admitWebSocket(req)
	if release == nil {
		t.Fatalf("admitWebSocket rejected request: status=%d reason=%s", status, reason)
	}
	defer release()

	if _, ok := e.wsSecurity.rateByIP["198.51.100.10"]; ok {
		t.Fatal("expired rate window was not pruned")
	}
}
