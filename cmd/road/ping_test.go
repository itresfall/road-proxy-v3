package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunPingCommandMeasuresEndpoint(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ping" {
			http.NotFound(w, r)
			return
		}
		hits.Add(1)
		_ = json.NewEncoder(w).Encode(pingResponse{
			Service:        "road-proxy-v3",
			Plane:          "data",
			ServerTime:     "2026-05-12T00:00:00Z",
			ServerUnixNano: 1778544000000000000,
			UptimeSeconds:  1,
		})
	}))
	defer server.Close()

	var out bytes.Buffer
	if err := runPingCommand([]string{
		"--url", server.URL,
		"--count", "2",
		"--interval", "0s",
		"--timeout", "2s",
	}, &out); err != nil {
		t.Fatalf("runPingCommand returned error: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("ping hits = %d, want 2", got)
	}
}

func TestRunPingCommandUsesClientConfigWhenURLEmpty(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ping" {
			http.NotFound(w, r)
			return
		}
		hits.Add(1)
		_ = json.NewEncoder(w).Encode(pingResponse{
			Service:        "road-proxy-v3",
			Plane:          "control",
			ServerTime:     "2026-05-12T00:00:00Z",
			ServerUnixNano: 1778544000000000000,
			UptimeSeconds:  1,
		})
	}))
	defer server.Close()

	root := t.TempDir()
	clientPath := filepath.Join(root, "client.json")
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	clientJSON := `{
  "listen_addr": "127.0.0.1:7777",
  "listen_network": "udp",
  "server_ws_url": "` + wsURL + `",
  "plugin_name": "game"
}`
	if err := os.WriteFile(clientPath, []byte(clientJSON), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	var out bytes.Buffer
	if err := runPingCommand([]string{
		"--client", clientPath,
		"--count", "1",
		"--timeout", "2s",
	}, &out); err != nil {
		t.Fatalf("runPingCommand returned error: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("ping hits = %d, want 1", got)
	}
}
