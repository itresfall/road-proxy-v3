package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: "en"},
		{in: "tr", want: "tr"},
		{in: "eng", want: "en"},
		{in: "en-US", want: "en"},
		{in: "unknown", want: "en"},
	}

	for _, tc := range tests {
		got := NormalizeLanguage(tc.in)
		if got != tc.want {
			t.Fatalf("NormalizeLanguage(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLoadAppSettingsWithBOM(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app.json")

	// UTF-8 BOM + JSON body
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"language":"eng"}`)...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write app settings failed: %v", err)
	}

	cfg, err := LoadAppSettings(path)
	if err != nil {
		t.Fatalf("LoadAppSettings failed: %v", err)
	}
	if cfg.Language != "en" {
		t.Fatalf("unexpected language: got=%q want=%q", cfg.Language, "en")
	}
}
