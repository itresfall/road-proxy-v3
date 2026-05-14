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

func TestRepositoryCompatProfilesLoad(t *testing.T) {
	root := repositoryRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "compat-profiles", "*.json"))
	if err != nil {
		t.Fatalf("glob compat profiles failed: %v", err)
	}

	profilePaths := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.HasPrefix(filepath.Base(path), "_") {
			continue
		}
		profilePaths = append(profilePaths, path)
	}
	if len(profilePaths) == 0 {
		t.Fatal("no compatibility profile JSON files found")
	}

	loader := plugin.NewLoader(filepath.Join(root, "plugins"))
	pluginNames, err := loader.ListAvailable()
	if err != nil {
		t.Fatalf("list plugins failed: %v", err)
	}
	plugins, err := loader.LoadEnabled(pluginNames)
	if err != nil {
		t.Fatalf("load plugins failed: %v", err)
	}

	localeDir := filepath.Join(root, "locales")
	enCatalog := loadLocaleCatalog(t, filepath.Join(localeDir, "en.json"))
	trCatalog := loadLocaleCatalog(t, filepath.Join(localeDir, "tr.json"))
	seenIDs := map[string]string{}

	for _, path := range profilePaths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read compat profile failed: %v", err)
			}

			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("parse compat profile object failed: %v", err)
			}
			validateRequiredCompatProfileKeys(t, raw)

			var profile repositoryCompatProfile
			if err := json.Unmarshal(data, &profile); err != nil {
				t.Fatalf("parse compat profile failed: %v", err)
			}
			validateRepositoryCompatProfile(t, path, profile, plugins, enCatalog, trCatalog, seenIDs)
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

type repositoryCompatProfile struct {
	SchemaVersion          string                      `json:"schema_version"`
	ID                     string                      `json:"id"`
	DisplayName            string                      `json:"display_name"`
	ROADScope              string                      `json:"road_scope"`
	SteamLobbySupported    bool                        `json:"steam_lobby_supported"`
	RequiresGameLaunchArgs bool                        `json:"requires_game_launch_args"`
	Match                  []string                    `json:"match"`
	ExeNames               []string                    `json:"exe_names"`
	PluginName             string                      `json:"plugin_name"`
	Network                string                      `json:"network"`
	TargetHost             string                      `json:"target_host"`
	TargetPort             int                         `json:"target_port"`
	ClientListenPort       int                         `json:"client_listen_port"`
	UDPPeerBroadcast       bool                        `json:"udp_peer_broadcast"`
	UDPReplyPolicy         string                      `json:"udp_reply_policy"`
	KnownPorts             []repositoryCompatKnownPort `json:"known_ports"`
	NoteKeys               []string                    `json:"note_keys"`
	LaunchAdviceKeys       []string                    `json:"launch_advice_keys"`
}

type repositoryCompatKnownPort struct {
	Network string `json:"network"`
	Port    int    `json:"port"`
	Role    string `json:"role"`
	Notes   string `json:"notes"`
}

func validateRequiredCompatProfileKeys(t *testing.T, raw map[string]json.RawMessage) {
	t.Helper()

	for _, key := range []string{
		"schema_version",
		"id",
		"display_name",
		"road_scope",
		"steam_lobby_supported",
		"requires_game_launch_args",
		"plugin_name",
		"network",
		"target_host",
		"target_port",
		"client_listen_port",
		"udp_peer_broadcast",
	} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("compat profile missing required key %q", key)
		}
	}
	if _, hasMatch := raw["match"]; !hasMatch {
		if _, hasExeNames := raw["exe_names"]; !hasExeNames {
			t.Fatal("compat profile must include match or exe_names")
		}
	}
}

func validateRepositoryCompatProfile(
	t *testing.T,
	path string,
	profile repositoryCompatProfile,
	plugins map[string]*plugin.RuntimePlugin,
	enCatalog map[string]string,
	trCatalog map[string]string,
	seenIDs map[string]string,
) {
	t.Helper()

	if profile.SchemaVersion != "compat_profile.v1" {
		t.Fatalf("schema_version = %q, want compat_profile.v1", profile.SchemaVersion)
	}
	if strings.TrimSpace(profile.ID) == "" {
		t.Fatal("id is required")
	}
	wantFile := profile.ID + ".json"
	if filepath.Base(path) != wantFile {
		t.Fatalf("profile filename = %q, want %q", filepath.Base(path), wantFile)
	}
	if previousPath, ok := seenIDs[profile.ID]; ok {
		t.Fatalf("duplicate compat profile id %q also used by %s", profile.ID, previousPath)
	}
	seenIDs[profile.ID] = path

	if strings.TrimSpace(profile.DisplayName) == "" {
		t.Fatal("display_name is required")
	}
	if profile.ROADScope != "direct_lan_local_port_only" {
		t.Fatalf("road_scope = %q, want direct_lan_local_port_only", profile.ROADScope)
	}
	if len(profile.Match)+len(profile.ExeNames) == 0 {
		t.Fatal("match or exe_names is required")
	}

	runtimePlugin := plugins[profile.PluginName]
	if runtimePlugin == nil {
		t.Fatalf("plugin_name %q does not exist in plugins/", profile.PluginName)
	}
	info := runtimePlugin.Info()
	if info.TargetNetwork != profile.Network {
		t.Fatalf("network = %q, plugin %q target network = %q", profile.Network, profile.PluginName, info.TargetNetwork)
	}
	switch profile.Network {
	case "tcp", "udp":
	default:
		t.Fatalf("network must be tcp or udp, got %q", profile.Network)
	}
	if strings.TrimSpace(profile.TargetHost) == "" {
		t.Fatal("target_host is required")
	}
	if profile.TargetPort <= 0 || profile.TargetPort > 65535 {
		t.Fatalf("target_port out of range: %d", profile.TargetPort)
	}
	if profile.ClientListenPort < 0 || profile.ClientListenPort > 65535 {
		t.Fatalf("client_listen_port out of range: %d", profile.ClientListenPort)
	}
	switch profile.UDPReplyPolicy {
	case "", "any", "same_ip", "strict":
	default:
		t.Fatalf("udp_reply_policy must be empty, any, same_ip, or strict, got %q", profile.UDPReplyPolicy)
	}
	if profile.Network == "tcp" && profile.UDPPeerBroadcast {
		t.Fatal("udp_peer_broadcast must be false for tcp profiles")
	}
	for i, knownPort := range profile.KnownPorts {
		if knownPort.Network != "tcp" && knownPort.Network != "udp" {
			t.Fatalf("known_ports[%d].network must be tcp or udp, got %q", i, knownPort.Network)
		}
		if knownPort.Port <= 0 || knownPort.Port > 65535 {
			t.Fatalf("known_ports[%d].port out of range: %d", i, knownPort.Port)
		}
	}
	for _, key := range append(append([]string{}, profile.NoteKeys...), profile.LaunchAdviceKeys...) {
		if strings.TrimSpace(key) == "" {
			t.Fatal("empty locale key in note_keys or launch_advice_keys")
		}
		if _, ok := enCatalog[key]; !ok {
			t.Fatalf("English locale missing compat profile key %q", key)
		}
		if _, ok := trCatalog[key]; !ok {
			t.Fatalf("Turkish locale missing compat profile key %q", key)
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
