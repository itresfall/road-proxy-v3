package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestTranslatorFallbackAndFormatting(t *testing.T) {
	tr := New("tr", map[string]string{
		"hello":   "Hello %s",
		"only_en": "Only English",
	}, map[string]string{
		"hello": "Merhaba %s",
	})

	if got := fmt.Sprintf(tr.T("hello"), "ROAD"); got != "Merhaba ROAD" {
		t.Fatalf("translated text = %q", got)
	}
	if got := tr.T("only_en"); got != "Only English" {
		t.Fatalf("fallback text = %q", got)
	}
	if got := tr.T("missing_key"); got != "missing_key" {
		t.Fatalf("missing text = %q", got)
	}

	missing := tr.MissingKeys()
	if len(missing) != 2 || missing[0] != "missing_key" || missing[1] != "only_en" {
		t.Fatalf("missing keys = %v", missing)
	}
}

func TestLoadUsesEnglishFallback(t *testing.T) {
	root := t.TempDir()
	writeLocale(t, root, "en.json", `{"hello":"Hello","only_en":"Only English"}`)
	writeLocale(t, root, "tr.json", `{"hello":"Merhaba"}`)

	tr, err := Load(root, "tr")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got := tr.T("hello"); got != "Merhaba" {
		t.Fatalf("translated text = %q", got)
	}
	if got := tr.T("only_en"); got != "Only English" {
		t.Fatalf("fallback text = %q", got)
	}
}

func writeLocale(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write locale %s failed: %v", name, err)
	}
}
