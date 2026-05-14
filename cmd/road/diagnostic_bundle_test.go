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
	if err := os.WriteFile(serverPath, []byte(`{"http":{"auth_token":"server-secret","auth_tokens":["backup-secret"]},"plugins":{"enabled":["game"]}}`), 0o644); err != nil {
		t.Fatalf("write server config failed: %v", err)
	}
	if err := os.WriteFile(clientPath, []byte(`{"plugin_name":"game","server_ws_url":"wss://example.test/ws?token=query-secret","auth_token":"client-secret"}`), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	writeTestPlugin(t, pluginDir, "game")

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir logs failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "road.log"), []byte("hello\nX-ROAD-Token: log-secret\nAuthorization: Bearer bearer-secret\n{\"auth_token\":\"jsonl-secret\"}\n"), 0o644); err != nil {
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

	serverJSON := zipEntryText(t, matches[0], "configs/server.json")
	clientJSON := zipEntryText(t, matches[0], "configs/client.json")
	logText := zipEntryText(t, matches[0], "logs/road.log")
	allText := serverJSON + clientJSON + logText
	for _, leaked := range []string{"server-secret", "backup-secret", "client-secret", "query-secret", "log-secret", "bearer-secret", "jsonl-secret"} {
		if strings.Contains(allText, leaked) {
			t.Fatalf("diagnostic bundle leaked %q in:\n%s", leaked, allText)
		}
	}
	if !strings.Contains(allText, "[REDACTED]") {
		t.Fatalf("expected diagnostic redaction marker, got:\n%s", allText)
	}
}

func TestSafeZipNameRejectsTraversal(t *testing.T) {
	badNames := []string{
		"",
		"..",
		"../x.txt",
		"safe/../../x.txt",
		`C:\tmp\x.txt`,
		`\tmp\x.txt`,
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

func TestRedactDiagnosticPathMasksUserHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		t.Skip("user home not available")
	}

	input := filepath.Join(home, "RoadProxy", "configs", "server.json")
	got := redactDiagnosticPath(input)
	if strings.Contains(strings.ToLower(got), strings.ToLower(home)) {
		t.Fatalf("redacted path still contains home dir: %q", got)
	}
	if !strings.HasPrefix(got, "~") {
		t.Fatalf("redacted path should start with ~, got %q", got)
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

func zipEntryText(t *testing.T, path string, entryName string) string {
	t.Helper()

	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.Name != entryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s failed: %v", entryName, err)
		}
		defer rc.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			t.Fatalf("read zip entry %s failed: %v", entryName, err)
		}
		return buf.String()
	}
	t.Fatalf("zip entry not found: %s", entryName)
	return ""
}
