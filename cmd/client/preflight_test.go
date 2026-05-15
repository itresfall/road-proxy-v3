package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"road-proxy-v3/internal/config"
)

func TestRunClientPreflightRejectsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := config.DefaultClient()
	cfg.ServerWSURL = strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"
	cfg.AuthToken = "secret"
	cfg.AuthHeader = config.DefaultAuthHeaderName

	if err := runClientPreflight(cfg); err == nil {
		t.Fatal("expected unauthorized preflight error")
	}
}

func TestRunClientPreflightAcceptsRoadInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(config.DefaultAuthHeaderName) != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"road-proxy-v3"}`))
	}))
	defer server.Close()

	cfg := config.DefaultClient()
	cfg.ServerWSURL = strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"
	cfg.AuthToken = "secret"
	cfg.AuthHeader = config.DefaultAuthHeaderName

	if err := runClientPreflight(cfg); err != nil {
		t.Fatalf("preflight failed: %v", err)
	}
}
