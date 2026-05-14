package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExistingPathReturnsExistingFromCWD(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "configs", "client.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(target, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	got := ResolveExistingPath("configs/client.json")
	if filepath.ToSlash(got) != "configs/client.json" {
		t.Fatalf("unexpected resolved path: got=%q", got)
	}
}

func TestIsDefaultRelativePath(t *testing.T) {
	tests := []struct {
		raw         string
		defaultPath string
		want        bool
	}{
		{raw: "", defaultPath: "configs/server.json", want: true},
		{raw: "configs/server.json", defaultPath: "configs/server.json", want: true},
		{raw: `configs\server.json`, defaultPath: "configs/server.json", want: true},
		{raw: "configs/custom.json", defaultPath: "configs/server.json", want: false},
	}

	for _, tc := range tests {
		got := IsDefaultRelativePath(tc.raw, tc.defaultPath)
		if got != tc.want {
			t.Fatalf("IsDefaultRelativePath(%q, %q) = %v, want %v", tc.raw, tc.defaultPath, got, tc.want)
		}
	}
}

func TestResolveExistingPathKeepsMissingRelativePath(t *testing.T) {
	got := ResolveExistingPath("configs/not-found.json")
	if filepath.ToSlash(got) != "configs/not-found.json" {
		t.Fatalf("unexpected resolved path: got=%q", got)
	}
}
