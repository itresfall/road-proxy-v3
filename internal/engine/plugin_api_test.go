package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"road-proxy-v3/internal/config"
)

func TestHandlePluginInfoReturnsPluginJSON(t *testing.T) {
	root := t.TempDir()
	mustWritePluginFiles(t, root, "minecraft", false)

	cfg := config.Default()
	cfg.Plugins.Dir = root
	e := New(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/plugin/info/minecraft", nil)
	rec := httptest.NewRecorder()
	e.handlePluginInfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got := body["name"]; got != "minecraft" {
		t.Fatalf("unexpected plugin name in response: %v", got)
	}
}

func TestHandlePluginConfigReturnsNotFoundWhenMissing(t *testing.T) {
	root := t.TempDir()
	mustWritePluginFiles(t, root, "minecraft", false)

	cfg := config.Default()
	cfg.Plugins.Dir = root
	e := New(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/plugin/config/minecraft", nil)
	rec := httptest.NewRecorder()
	e.handlePluginConfig(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlePluginDownloadReturnsBundle(t *testing.T) {
	root := t.TempDir()
	mustWritePluginFiles(t, root, "minecraft", true)

	cfg := config.Default()
	cfg.Plugins.Dir = root
	e := New(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/plugin/download/minecraft", nil)
	rec := httptest.NewRecorder()
	e.handlePluginDownload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Name      string                 `json:"name"`
		HasConfig bool                   `json:"has_config"`
		Plugin    map[string]interface{} `json:"plugin"`
		Config    map[string]interface{} `json:"config"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Name != "minecraft" {
		t.Fatalf("unexpected bundle name: %s", body.Name)
	}
	if !body.HasConfig {
		t.Fatal("expected has_config=true")
	}
	if body.Plugin["name"] != "minecraft" {
		t.Fatalf("unexpected plugin.name: %v", body.Plugin["name"])
	}
	if body.Config["profile"] != "test" {
		t.Fatalf("unexpected config.profile: %v", body.Config["profile"])
	}
}

func TestHandlePluginInfoRejectsInvalidPluginName(t *testing.T) {
	cfg := config.Default()
	e := New(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/plugin/info/../secrets", nil)
	rec := httptest.NewRecorder()
	e.handlePluginInfo(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}
}

func mustWritePluginFiles(t *testing.T, root, name string, withConfig bool) {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	pluginJSON := `{
  "schema_version": "v1",
  "name": "` + name + `",
  "version": "3.0.0",
  "description": "test plugin",
  "author": "test",
  "protocols": { "supported": ["tcp", "websocket"] },
  "target": { "network": "tcp", "address": "127.0.0.1:25565" },
  "capabilities": {
    "supports_reconnect": true,
    "supports_multiplex": false
  },
  "runtime": {
    "type": "json",
    "mode": "passthrough",
    "enable_obfuscation": false,
    "client_pipeline": [],
    "server_pipeline": []
  }
}`

	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(pluginJSON), 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}

	if withConfig {
		configJSON := `{"profile":"test","notes":"plugin config"}`
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(configJSON), 0o644); err != nil {
			t.Fatalf("write config.json: %v", err)
		}
	}
}
