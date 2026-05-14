package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/plugin"
)

func TestHandleControlInfoShowsAuthEnabledWhenConfigured(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AuthToken = "secret-token"
	cfg.HTTP.AuthHeader = "X-Proxy-Token"
	e := New(cfg, nil)
	e.defaultPlugin = plugin.NewRuntimePlugin(&plugin.Schema{
		SchemaVersion: plugin.SchemaVersionV1,
		Name:          "minecraft",
		Version:       "3.0.0",
		Target: plugin.Target{
			Network: "tcp",
			Address: "127.0.0.1:25565",
		},
		Runtime: plugin.RuntimeConfig{
			Type: plugin.RuntimeTypeJSON,
			Mode: plugin.RuntimeModePassthrough,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/info", nil)
	rec := httptest.NewRecorder()
	e.handleControlInfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	auth, ok := body["auth"].(map[string]interface{})
	if !ok {
		t.Fatalf("auth object missing or invalid: %v", body["auth"])
	}

	if enabled, ok := auth["enabled"].(bool); !ok || !enabled {
		t.Fatalf("expected auth.enabled=true, got=%v", auth["enabled"])
	}
	if header, ok := auth["header"].(string); !ok || header != "X-Proxy-Token" {
		t.Fatalf("expected auth.header=X-Proxy-Token, got=%v", auth["header"])
	}
	if count, ok := auth["tokens_count"].(float64); !ok || count != 1 {
		t.Fatalf("expected auth.tokens_count=1, got=%v", auth["tokens_count"])
	}

	security, ok := body["security"].(map[string]interface{})
	if !ok {
		t.Fatalf("security object missing or invalid: %v", body["security"])
	}
	if enabled, ok := security["auth_enabled"].(bool); !ok || !enabled {
		t.Fatalf("expected security.auth_enabled=true, got=%v", security["auth_enabled"])
	}
	if header, ok := security["auth_header"].(string); !ok || header != "X-Proxy-Token" {
		t.Fatalf("expected security.auth_header=X-Proxy-Token, got=%v", security["auth_header"])
	}
	runtime, ok := body["runtime"].(map[string]interface{})
	if !ok {
		t.Fatalf("runtime object missing or invalid: %v", body["runtime"])
	}
	if bufferSize, ok := runtime["buffer_size"].(float64); !ok || bufferSize <= 0 {
		t.Fatalf("expected runtime.buffer_size > 0, got=%v", runtime["buffer_size"])
	}
}

func TestHandleControlInfoIncludesAuthDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AuthToken = ""
	cfg.HTTP.AuthTokens = nil
	e := New(cfg, nil)
	e.defaultPlugin = plugin.NewRuntimePlugin(&plugin.Schema{
		SchemaVersion: plugin.SchemaVersionV1,
		Name:          "minecraft",
		Version:       "3.0.0",
		Target: plugin.Target{
			Network: "tcp",
			Address: "127.0.0.1:25565",
		},
		Runtime: plugin.RuntimeConfig{
			Type: plugin.RuntimeTypeJSON,
			Mode: plugin.RuntimeModePassthrough,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/info", nil)
	rec := httptest.NewRecorder()
	e.handleControlInfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	auth, ok := body["auth"].(map[string]interface{})
	if !ok {
		t.Fatalf("auth object missing or invalid: %v", body["auth"])
	}

	if enabled, ok := auth["enabled"].(bool); !ok || enabled {
		t.Fatalf("expected auth.enabled=false, got=%v", auth["enabled"])
	}
}
