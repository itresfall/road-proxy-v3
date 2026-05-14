package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"road-proxy-v3/internal/config"
)

func TestLoadEnabledPluginsSupportsMultiple(t *testing.T) {
	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")

	mustWritePlugin(t, pluginsDir, "minecraft", "127.0.0.1:25565")
	mustWritePlugin(t, pluginsDir, "valve", "127.0.0.1:27015")

	cfg := config.Default()
	cfg.Plugins.Dir = pluginsDir
	cfg.Plugins.Enabled = []string{"minecraft", "valve"}

	e := New(cfg, nil)
	if err := e.loadEnabledPlugins(); err != nil {
		t.Fatalf("loadEnabledPlugins failed: %v", err)
	}

	if got := e.defaultPlugin.Name(); got != "minecraft" {
		t.Fatalf("unexpected default plugin: %s", got)
	}
	if len(e.enabledPlugin) != 2 {
		t.Fatalf("unexpected enabled plugin count: %d", len(e.enabledPlugin))
	}
}

func TestResolvePlugin(t *testing.T) {
	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")

	mustWritePlugin(t, pluginsDir, "minecraft", "127.0.0.1:25565")
	mustWritePlugin(t, pluginsDir, "valve", "127.0.0.1:27015")

	cfg := config.Default()
	cfg.Plugins.Dir = pluginsDir
	cfg.Plugins.Enabled = []string{"minecraft", "valve"}

	e := New(cfg, nil)
	if err := e.loadEnabledPlugins(); err != nil {
		t.Fatalf("loadEnabledPlugins failed: %v", err)
	}

	defaultPlugin, err := e.resolvePlugin("")
	if err != nil {
		t.Fatalf("resolvePlugin(default) failed: %v", err)
	}
	if defaultPlugin.Name() != "minecraft" {
		t.Fatalf("unexpected default resolved plugin: %s", defaultPlugin.Name())
	}

	valvePlugin, err := e.resolvePlugin("valve")
	if err != nil {
		t.Fatalf("resolvePlugin(valve) failed: %v", err)
	}
	if valvePlugin.Name() != "valve" {
		t.Fatalf("unexpected plugin resolved for valve: %s", valvePlugin.Name())
	}

	if _, err := e.resolvePlugin("unknown"); err == nil {
		t.Fatal("expected unknown plugin to fail")
	}
}

func mustWritePlugin(t *testing.T, root, name, target string) {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	raw := fmt.Sprintf(`{
  "schema_version": "v1",
  "name": %q,
  "version": "3.0.0",
  "description": "test plugin",
  "author": "test",
  "protocols": { "supported": ["tcp", "websocket"] },
  "target": { "network": "tcp", "address": %q },
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
}`, name, target)

	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write plugin schema: %v", err)
	}
}
