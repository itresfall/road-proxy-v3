package repoqa

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/i18n"
	"road-proxy-v3/internal/plugin"
	"road-proxy-v3/internal/voice"
)

func TestRepositoryConfigsLoad(t *testing.T) {
	root := repositoryRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "configs", "*.json"))
	if err != nil {
		t.Fatalf("glob configs failed: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no config JSON files found")
	}

	for _, path := range paths {
		path := path
		base := filepath.Base(path)
		t.Run(base, func(t *testing.T) {
			switch {
			case base == "app.json":
				if _, err := app.LoadAppSettings(path); err != nil {
					t.Fatalf("load app settings failed: %v", err)
				}
			case base == "voice-server.json":
				if _, err := voice.LoadConfig(path); err != nil {
					t.Fatalf("load voice config failed: %v", err)
				}
			case base == "plugin.schema.v1.json":
				validatePluginSchemaDocument(t, path)
			case strings.HasPrefix(base, "client"):
				if _, err := config.LoadClient(path); err != nil {
					t.Fatalf("load client config failed: %v", err)
				}
			case strings.HasPrefix(base, "server"):
				if _, err := config.Load(path); err != nil {
					t.Fatalf("load server config failed: %v", err)
				}
			default:
				t.Fatalf("unclassified config file %q", base)
			}
		})
	}
}

func TestRepositoryPluginsLoad(t *testing.T) {
	root := repositoryRoot(t)
	loader := plugin.NewLoader(filepath.Join(root, "plugins"))

	names, err := loader.ListAvailable()
	if err != nil {
		t.Fatalf("list plugins failed: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no plugin directories found")
	}

	loaded, err := loader.LoadEnabled(names)
	if err != nil {
		t.Fatalf("load repository plugins failed: %v", err)
	}

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			runtimePlugin := loaded[name]
			if runtimePlugin == nil {
				t.Fatalf("plugin %q was not loaded", name)
			}

			info := runtimePlugin.Info()
			if info.Name != name {
				t.Fatalf("plugin info name = %q, want %q", info.Name, name)
			}
			switch info.TargetNetwork {
			case "tcp", "udp":
			default:
				t.Fatalf("invalid target network %q", info.TargetNetwork)
			}
			if strings.TrimSpace(info.TargetAddress) == "" {
				t.Fatal("target address is empty")
			}
		})
	}
}

func TestRepositoryLocalesLoad(t *testing.T) {
	root := repositoryRoot(t)
	localeDir := filepath.Join(root, "locales")

	en, err := i18n.Load(localeDir, "en")
	if err != nil {
		t.Fatalf("load English locale failed: %v", err)
	}
	if got := en.T("menu.choose_mode"); got == "menu.choose_mode" || strings.TrimSpace(got) == "" {
		t.Fatalf("English locale key missing: %q", got)
	}

	tr, err := i18n.Load(localeDir, "tr")
	if err != nil {
		t.Fatalf("load Turkish locale failed: %v", err)
	}
	if got := tr.T("menu.choose_mode"); got == "menu.choose_mode" || strings.TrimSpace(got) == "" {
		t.Fatalf("Turkish locale key missing: %q", got)
	}
	if got := tr.T("studio.title"); got == "studio.title" || strings.TrimSpace(got) == "" {
		t.Fatalf("Turkish Plugin Studio locale key missing: %q", got)
	}
	if got := en.T("studio.profile.gzdoom.note.netmode"); got == "studio.profile.gzdoom.note.netmode" || strings.TrimSpace(got) == "" {
		t.Fatalf("English Plugin Studio profile key missing: %q", got)
	}

	enCatalog := loadLocaleCatalog(t, filepath.Join(localeDir, "en.json"))
	trCatalog := loadLocaleCatalog(t, filepath.Join(localeDir, "tr.json"))
	for key := range enCatalog {
		if _, ok := trCatalog[key]; !ok {
			t.Fatalf("Turkish locale is missing key %q", key)
		}
	}
}

func validatePluginSchemaDocument(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read plugin schema document failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("plugin schema document is invalid JSON: %v", err)
	}

	for _, key := range []string{"$schema", "$id", "title", "type", "required", "properties"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("plugin schema document missing key %q", key)
		}
	}
}

func loadLocaleCatalog(t *testing.T, path string) map[string]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read locale failed: %v", err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var catalog map[string]string
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("parse locale failed: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatalf("empty locale catalog: %s", path)
	}
	return catalog
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
