package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureRuntimeLayoutAtCreatesCoreFiles(t *testing.T) {
	root := t.TempDir()

	layout, err := EnsureRuntimeLayoutAt(root)
	if err != nil {
		t.Fatalf("EnsureRuntimeLayoutAt failed: %v", err)
	}

	if layout.Root != root {
		t.Fatalf("unexpected layout root: got=%q want=%q", layout.Root, root)
	}

	required := []string{
		filepath.Join(root, "configs", "server.json"),
		filepath.Join(root, "configs", "client.json"),
		filepath.Join(root, "configs", "app.json"),
		filepath.Join(root, "plugins", "minecraft", "plugin.json"),
		filepath.Join(root, "plugins", "minecraft-bedrock-udp", "plugin.json"),
	}

	for _, path := range required {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("expected file missing %q: %v", path, statErr)
		}
	}
}

func TestEnsureRuntimeLayoutAtDoesNotOverwriteExisting(t *testing.T) {
	root := t.TempDir()
	clientPath := filepath.Join(root, "configs", "client.json")

	if err := os.MkdirAll(filepath.Dir(clientPath), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	const sentinel = `{"custom":"keep"}`
	if err := os.WriteFile(clientPath, []byte(sentinel), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if _, err := EnsureRuntimeLayoutAt(root); err != nil {
		t.Fatalf("EnsureRuntimeLayoutAt failed: %v", err)
	}

	data, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if strings.TrimSpace(string(data)) != sentinel {
		t.Fatalf("client config was overwritten: got=%q want=%q", strings.TrimSpace(string(data)), sentinel)
	}
}
