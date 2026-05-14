package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDiagnosticBundleCommandCreatesZip(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "configs")
	pluginDir := filepath.Join(root, "plugins")
	logDir := filepath.Join(root, "logs")
	outDir := filepath.Join(root, "diagnostics")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir configs failed: %v", err)
	}
	serverPath := filepath.Join(configDir, "server.json")
	clientPath := filepath.Join(configDir, "client.json")
	if err := os.WriteFile(serverPath, []byte(`{"plugins":{"enabled":["game"]}}`), 0o644); err != nil {
		t.Fatalf("write server config failed: %v", err)
	}
	if err := os.WriteFile(clientPath, []byte(`{"plugin_name":"game"}`), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	writeTestPlugin(t, pluginDir, "game")

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir logs failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "road.log"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write log failed: %v", err)
	}

	var out bytes.Buffer
	err := runDiagnosticBundleCommand([]string{
		"--out", outDir,
		"--server", serverPath,
		"--client", clientPath,
		"--plugins", pluginDir,
		"--logs", logDir,
		"--skip-net",
	}, &out)
	if err != nil {
		t.Fatalf("runDiagnosticBundleCommand returned error: %v", err)
	}
	if !strings.Contains(out.String(), outDir) {
		t.Fatalf("expected output to include bundle dir, got %q", out.String())
	}

	matches, err := filepath.Glob(filepath.Join(outDir, "road-diagnostic-*.zip"))
	if err != nil {
		t.Fatalf("glob diagnostic bundle failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one diagnostic bundle, got %v", matches)
	}

	names := zipEntryNames(t, matches[0])
	for _, want := range []string{
		"version.txt",
		"metadata.json",
		"configs/server.json",
		"configs/client.json",
		"plugins/game/plugin.json",
		"logs/road.log",
	} {
		if !names[want] {
			t.Fatalf("diagnostic bundle missing %s; entries=%v", want, names)
		}
	}
}

func TestSafeZipNameRejectsTraversal(t *testing.T) {
	badNames := []string{
		"",
		"..",
		"../x.txt",
		"safe/../../x.txt",
		filepath.Join(string(filepath.Separator), "tmp", "x.txt"),
	}
	for _, name := range badNames {
		if got, err := safeZipName(name); err == nil {
			t.Fatalf("safeZipName(%q) = %q, expected error", name, got)
		}
	}

	if got, err := safeZipName(`logs\road.log`); err != nil || got != "logs/road.log" {
		t.Fatalf("safeZipName returned %q, %v", got, err)
	}
}

func zipEntryNames(t *testing.T, path string) map[string]bool {
	t.Helper()

	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer reader.Close()

	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	return names
}
