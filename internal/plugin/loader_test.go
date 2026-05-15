package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderLoadEnabled(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "minecraft")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	schemaJSON := `{
  "schema_version": "v1",
  "name": "minecraft",
  "version": "3.0.0",
  "target": {"network": "tcp", "address": "127.0.0.1:25565"},
  "runtime": {"type": "json", "mode": "passthrough"}
}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(schemaJSON), 0o644); err != nil {
		t.Fatalf("write plugin schema failed: %v", err)
	}

	loader := NewLoader(root)
	plugins, err := loader.LoadEnabled([]string{"minecraft"})
	if err != nil {
		t.Fatalf("load enabled failed: %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	p, ok := plugins["minecraft"]
	if !ok {
		t.Fatalf("minecraft plugin missing from map")
	}
	if p.TargetAddress() != "127.0.0.1:25565" {
		t.Fatalf("unexpected target address: %s", p.TargetAddress())
	}
}

func TestLoaderLoadEnabledTrimsNames(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "minecraft")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	schemaJSON := `{
  "schema_version": "v1",
  "name": "minecraft",
  "version": "3.0.0",
  "target": {"network": "tcp", "address": "127.0.0.1:25565"},
  "runtime": {"type": "json", "mode": "passthrough"}
}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(schemaJSON), 0o644); err != nil {
		t.Fatalf("write plugin schema failed: %v", err)
	}

	plugins, err := NewLoader(root).LoadEnabled([]string{" minecraft "})
	if err != nil {
		t.Fatalf("load enabled failed: %v", err)
	}
	if _, ok := plugins["minecraft"]; !ok {
		t.Fatalf("trimmed minecraft plugin missing from map: %#v", plugins)
	}
}

func TestLoaderLoadEnabledRejectsEmptyName(t *testing.T) {
	loader := NewLoader(t.TempDir())
	if _, err := loader.LoadEnabled([]string{" "}); err == nil {
		t.Fatal("expected empty plugin name error")
	}
}

func TestLoaderLoadEnabledFailsForMissingPlugin(t *testing.T) {
	loader := NewLoader(t.TempDir())
	if _, err := loader.LoadEnabled([]string{"does-not-exist"}); err == nil {
		t.Fatal("expected missing plugin error")
	}
}
