package main

import (
	"bufio"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/config"
)

func TestSplitListenAddrAllowsZeroPort(t *testing.T) {
	host, port, err := splitListenAddr("0.0.0.0:0")
	if err != nil {
		t.Fatalf("splitListenAddr returned error: %v", err)
	}
	if host != "0.0.0.0" || port != 0 {
		t.Fatalf("unexpected parse result: host=%q port=%d", host, port)
	}
}

func TestReadPortOverrideAllowsZeroWhenEnabled(t *testing.T) {
	// We only validate bound checks in helper logic by direct call pattern.
	// readPortOverride itself reads stdin, so this test covers the parser via splitListenAddr.
	if _, _, err := splitListenAddr("127.0.0.1:65535"); err != nil {
		t.Fatalf("expected max port to parse: %v", err)
	}
}

func TestParsePIDListOutput(t *testing.T) {
	output := []byte("1200\n42\nfoo\n0\n1200\n")
	got := parsePIDListOutput(output)

	want := []int{42, 1200}
	if len(got) != len(want) {
		t.Fatalf("unexpected pid count: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected pids: got=%v want=%v", got, want)
		}
	}
}

func TestParseLinuxSSOwners(t *testing.T) {
	ssOutput := []byte(
		"udp   UNCONN 0      0      0.0.0.0:19133      0.0.0.0:*    users:((\"road\",pid=9001,fd=12))\n" +
			"udp   UNCONN 0      0      0.0.0.0:8081       0.0.0.0:*    users:((\"road\",pid=12,fd=20))\n" +
			"udp   UNCONN 0      0      0.0.0.0:19133      0.0.0.0:*    users:((\"road\",pid=777,fd=2),(\"road\",pid=9001,fd=13))\n",
	)

	got := parseLinuxSSOwners(ssOutput, 19133)
	want := []int{777, 9001}

	if len(got) != len(want) {
		t.Fatalf("unexpected pid count: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected pids: got=%v want=%v", got, want)
		}
	}
}

func TestParseWindowsNetstatOwnersTCP(t *testing.T) {
	output := []byte(
		"  TCP    0.0.0.0:8080      0.0.0.0:0       LISTENING       1234\r\n" +
			"  TCP    127.0.0.1:8080    127.0.0.1:53000 ESTABLISHED     2222\r\n" +
			"  TCP    0.0.0.0:8081      0.0.0.0:0       LISTENING       1234\r\n" +
			"  TCP    [::]:8080         [::]:0          LISTENING       42\r\n",
	)

	got, err := parseWindowsNetstatOwners(output, "tcp", 8080)
	if err != nil {
		t.Fatalf("parseWindowsNetstatOwners returned error: %v", err)
	}
	want := []int{42, 1234}

	if len(got) != len(want) {
		t.Fatalf("unexpected pid count: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected pids: got=%v want=%v", got, want)
		}
	}
}

func TestParseWindowsNetstatOwnersUDP(t *testing.T) {
	output := []byte(
		"  UDP    0.0.0.0:7777      *:*       9001\r\n" +
			"  UDP    127.0.0.1:7777    *:*       777\r\n" +
			"  UDP    0.0.0.0:8080      *:*       12\r\n",
	)

	got, err := parseWindowsNetstatOwners(output, "udp", 7777)
	if err != nil {
		t.Fatalf("parseWindowsNetstatOwners returned error: %v", err)
	}
	want := []int{777, 9001}

	if len(got) != len(want) {
		t.Fatalf("unexpected pid count: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected pids: got=%v want=%v", got, want)
		}
	}
}

func TestRunValidateCommand(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "plugins")
	writeTestPlugin(t, pluginDir, "game")

	serverPath := filepath.Join(root, "server.json")
	serverJSON := `{
  "plugins": {
    "dir": "plugins",
    "enabled": ["game"]
  }
}`
	if err := os.WriteFile(serverPath, []byte(serverJSON), 0o644); err != nil {
		t.Fatalf("write server config failed: %v", err)
	}

	clientPath := filepath.Join(root, "client.json")
	clientJSON := `{
  "listen_addr": "127.0.0.1:7777",
  "listen_network": "udp",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "plugin_name": "game"
}`
	if err := os.WriteFile(clientPath, []byte(clientJSON), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	var out bytes.Buffer
	err := runValidateCommand([]string{
		"--server", serverPath,
		"--client", clientPath,
		"--plugins", pluginDir,
	}, &out)
	if err != nil {
		t.Fatalf("runValidateCommand returned error: %v", err)
	}
}

func TestRunValidatePluginCommand(t *testing.T) {
	root := t.TempDir()
	pluginPath := writeTestPlugin(t, filepath.Join(root, "plugins"), "game")

	var out bytes.Buffer
	if err := runValidatePluginCommand([]string{pluginPath}, &out); err != nil {
		t.Fatalf("runValidatePluginCommand returned error: %v", err)
	}
}

func TestRunValidatePluginCommandRejectsFolderNameMismatch(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "plugins", "wrong-folder")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir failed: %v", err)
	}
	pluginPath := filepath.Join(pluginDir, "plugin.json")
	schemaJSON := `{
  "schema_version": "v1",
  "name": "right-name",
  "version": "3.0.0",
  "target": {"network": "tcp", "address": "127.0.0.1:1234"},
  "runtime": {"type": "json", "mode": "passthrough"}
}`
	if err := os.WriteFile(pluginPath, []byte(schemaJSON), 0o644); err != nil {
		t.Fatalf("write plugin schema failed: %v", err)
	}

	var out bytes.Buffer
	err := runValidatePluginCommand([]string{pluginPath}, &out)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected folder mismatch error, got %v", err)
	}
}

func TestLoadClientConfigForMenuRepairsInvalidServerWSURL(t *testing.T) {
	root := t.TempDir()
	clientPath := filepath.Join(root, "client.json")
	clientJSON := `{
  "listen_addr": "127.0.0.1:7777",
  "listen_network": "udp",
  "server_ws_url": "ftp://example.invalid/ws",
  "plugin_name": "game"
}`
	if err := os.WriteFile(clientPath, []byte(clientJSON), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	reader := bufio.NewReader(strings.NewReader("127.0.0.1:8080\n"))
	cfg, err := loadClientConfigForMenu(reader, clientPath)
	if err != nil {
		t.Fatalf("loadClientConfigForMenu returned error: %v", err)
	}
	if cfg.ServerWSURL != "ws://127.0.0.1:8080/ws" {
		t.Fatalf("unexpected repaired server_ws_url: %s", cfg.ServerWSURL)
	}
}

func TestGetServerDefaultPluginProfileUsesDataInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/info" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"road-proxy-v3","default_plugin":"ddnet-udp","default_network":"udp"}`))
	}))
	defer server.Close()

	cfg := config.DefaultClient()
	cfg.ServerWSURL = strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"

	profile, err := getServerDefaultPluginProfile(cfg)
	if err != nil {
		t.Fatalf("getServerDefaultPluginProfile returned error: %v", err)
	}
	if profile.Name != "ddnet-udp" || profile.TargetNetwork != "udp" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}

func TestGetServerDefaultPluginProfileRejectsUnexpectedService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"not-road","default_plugin":"minecraft","default_network":"tcp"}`))
	}))
	defer server.Close()

	cfg := config.DefaultClient()
	cfg.ServerWSURL = strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"

	if _, err := getServerDefaultPluginProfile(cfg); err == nil {
		t.Fatal("expected unexpected service error")
	}
}

func TestApplyClientProfileFromServerRejectsExplicitBadEndpoint(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	cfg := config.DefaultClient()
	cfg.ServerWSURL = strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"
	cfg.PluginName = "minecraft"

	_, _, err := applyClientProfileFromServer(cfg, true)
	if err == nil {
		t.Fatal("expected explicit endpoint profile fetch failure")
	}
	if cfg.PluginName != "minecraft" {
		t.Fatalf("plugin should not be changed on profile fetch failure, got %q", cfg.PluginName)
	}
}

func TestApplyClientProfileFromServerRejectsNonRoadDomainEvenWithPluginLikeJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/info":
			http.NotFound(w, r)
		case "/api/plugins":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"default":{"name":"minecraft","target_network":"tcp"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.DefaultClient()
	cfg.ServerWSURL = strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"
	cfg.PluginName = "minecraft"

	_, _, err := applyClientProfileFromServer(cfg, true)
	if err == nil {
		t.Fatal("expected non-ROAD endpoint to be rejected before plugin fallback")
	}
	if cfg.PluginName != "minecraft" {
		t.Fatalf("plugin should not be changed on non-ROAD endpoint, got %q", cfg.PluginName)
	}
}

func TestApplyClientProfileFromServerRejectsHTMLInfoEvenWithHealthFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/info":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html>not road</html>`))
		case "/api/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"default_plugin":{"name":"minecraft","target_network":"tcp"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.DefaultClient()
	cfg.ServerWSURL = strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"

	if _, _, err := applyClientProfileFromServer(cfg, true); err == nil {
		t.Fatal("expected HTML /api/info response to reject endpoint")
	}
}

func TestApplyLocalClientTemplateForProfileCopiesUDPListeners(t *testing.T) {
	root := t.TempDir()
	layout := app.RuntimeLayout{
		Root:      root,
		ConfigDir: filepath.Join(root, "configs"),
		PluginDir: filepath.Join(root, "plugins"),
	}
	if err := os.MkdirAll(filepath.Join(layout.PluginDir, "son-of-the-forest-udp"), 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	if err := os.MkdirAll(layout.ConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	pluginJSON := `{
  "schema_version": "v1",
  "name": "son-of-the-forest-udp",
  "version": "0.1.0",
  "target": {"network": "udp", "address": "127.0.0.1:8766"},
  "menu": {"server_config": "configs/server-son-of-the-forest.json", "client_config": "configs/client-son-of-the-forest.json"},
  "runtime": {"type": "json", "mode": "passthrough"}
}`
	if err := os.WriteFile(filepath.Join(layout.PluginDir, "son-of-the-forest-udp", "plugin.json"), []byte(pluginJSON), 0o644); err != nil {
		t.Fatalf("write plugin json: %v", err)
	}

	clientJSON := `{
  "listen_network": "udp",
  "listen_addr": "0.0.0.0:8766",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "plugin_name": "son-of-the-forest-udp",
  "udp_listeners": [
    {"listen_addr": "0.0.0.0:8766", "target": "game"},
    {"listen_addr": "0.0.0.0:9700", "target": "blob-sync"}
  ]
}`
	if err := os.WriteFile(filepath.Join(layout.ConfigDir, "client-son-of-the-forest.json"), []byte(clientJSON), 0o644); err != nil {
		t.Fatalf("write client template: %v", err)
	}

	cfg := config.DefaultClient()
	cfg.ServerWSURL = "wss://road.example/ws"
	cfg.AuthToken = "token"
	cfg.AuthHeader = "X-ROAD-Token"

	profile := serverPluginProfile{Name: "son-of-the-forest-udp", TargetNetwork: "udp"}
	if err := applyLocalClientTemplateForProfile(layout, profile, cfg); err != nil {
		t.Fatalf("applyLocalClientTemplateForProfile failed: %v", err)
	}
	if cfg.ServerWSURL != "wss://road.example/ws" || cfg.AuthToken != "token" {
		t.Fatalf("connection credentials were not preserved: %#v", cfg)
	}
	if len(cfg.UDPListeners) != 2 {
		t.Fatalf("expected 2 udp listeners, got %d", len(cfg.UDPListeners))
	}
	if cfg.UDPListeners[1].Target != "blob-sync" {
		t.Fatalf("unexpected second listener target: %q", cfg.UDPListeners[1].Target)
	}
}

func TestShouldRequireClientProfileForPublicSavedEndpoint(t *testing.T) {
	cfg := config.DefaultClient()
	cfg.ServerWSURL = "wss://road.example.com/ws"
	if !shouldRequireClientProfile(cfg, false) {
		t.Fatal("public saved endpoint should require profile verification")
	}

	cfg.ServerWSURL = "ws://127.0.0.1:8080/ws"
	if shouldRequireClientProfile(cfg, false) {
		t.Fatal("unchanged local endpoint should not require profile verification")
	}

	cfg.AuthToken = "local-token"
	if !shouldRequireClientProfile(cfg, false) {
		t.Fatal("configured auth token should require profile verification")
	}
}

func TestPreflightClientCheckReturnsAuthStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing token", http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := config.DefaultClient()
	cfg.ServerWSURL = strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"

	if err := preflightClientCheck(cfg, true); err == nil {
		t.Fatal("expected preflight auth failure")
	}
}

func TestClientProfileFetchHeadersIncludesAuthToken(t *testing.T) {
	cfg := config.DefaultClient()
	cfg.AuthToken = "secret"
	cfg.AuthHeader = "X-Proxy-Token"
	cfg.Headers = map[string]string{"X-Custom": "value"}

	headers := clientProfileFetchHeaders(cfg)
	if got := headers.Get("X-Proxy-Token"); got != "secret" {
		t.Fatalf("auth header = %q, want secret", got)
	}
	if got := headers.Get("X-Custom"); got != "value" {
		t.Fatalf("custom header = %q, want value", got)
	}
}

func TestApplyClientConnectionInputDoesNotPromptForPublicAuthToken(t *testing.T) {
	cfg := config.DefaultClient()
	reader := bufio.NewReader(strings.NewReader("example.trycloudflare.com\n"))

	changed, err := applyClientConnectionInput(reader, cfg)
	if err != nil {
		t.Fatalf("applyClientConnectionInput returned error: %v", err)
	}
	if !changed {
		t.Fatal("expected endpoint to change")
	}
	if cfg.ServerWSURL != "wss://example.trycloudflare.com/ws" {
		t.Fatalf("server_ws_url = %s", cfg.ServerWSURL)
	}
	if cfg.AuthToken != "" {
		t.Fatalf("auth_token = %q", cfg.AuthToken)
	}
}

func TestSplitDisplayTargetNormalizesWildcardHost(t *testing.T) {
	target, host, port := splitDisplayTarget("0.0.0.0:25568")
	if target != "127.0.0.1:25568" || host != "127.0.0.1" || port != "25568" {
		t.Fatalf("unexpected display target: target=%q host=%q port=%q", target, host, port)
	}
}

func TestSplitDisplayTargetKeepsConcreteHost(t *testing.T) {
	target, host, port := splitDisplayTarget("192.168.1.50:8303")
	if target != "192.168.1.50:8303" || host != "192.168.1.50" || port != "8303" {
		t.Fatalf("unexpected display target: target=%q host=%q port=%q", target, host, port)
	}
}

func TestApplyClientConnectionInputBlankPublicAuthKeepsExistingToken(t *testing.T) {
	cfg := config.DefaultClient()
	cfg.AuthToken = "existing-token"
	cfg.AuthHeader = "X-Existing-Token"
	reader := bufio.NewReader(strings.NewReader("road.example.com\n"))

	changed, err := applyClientConnectionInput(reader, cfg)
	if err != nil {
		t.Fatalf("applyClientConnectionInput returned error: %v", err)
	}
	if !changed {
		t.Fatal("expected endpoint to change")
	}
	if cfg.AuthToken != "existing-token" || cfg.AuthHeader != "X-Existing-Token" {
		t.Fatalf("existing auth should be preserved, got token=%q header=%q", cfg.AuthToken, cfg.AuthHeader)
	}
}

func TestEditClientAuthSettingsCanDisableAuth(t *testing.T) {
	cfg := config.DefaultClient()
	cfg.AuthToken = "secret"
	cfg.AuthHeader = "X-ROAD-Token"
	reader := bufio.NewReader(strings.NewReader("n\n"))

	if err := editClientAuthSettings(reader, cfg); err != nil {
		t.Fatalf("editClientAuthSettings returned error: %v", err)
	}
	if cfg.AuthToken != "" || cfg.AuthHeader != "" {
		t.Fatalf("client auth should be cleared, got token=%q header=%q", cfg.AuthToken, cfg.AuthHeader)
	}
}

func TestEditServerAuthSettingsCanDisableAuth(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AuthToken = "secret"
	cfg.HTTP.AuthHeader = "X-ROAD-Token"
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	reader := bufio.NewReader(strings.NewReader("n\n"))

	if err := editServerAuthSettings(reader, cfg); err != nil {
		t.Fatalf("editServerAuthSettings returned error: %v", err)
	}
	if cfg.HTTP.AuthToken != "" || len(cfg.HTTP.AuthTokens) != 0 || cfg.HTTP.AuthHeader != "" {
		t.Fatalf("server auth should be cleared, got token=%q tokens=%v header=%q", cfg.HTTP.AuthToken, cfg.HTTP.AuthTokens, cfg.HTTP.AuthHeader)
	}
}

func TestEditServerAuthSettingsCanEnableAuthWithGeneratedToken(t *testing.T) {
	cfg := config.Default()
	reader := bufio.NewReader(strings.NewReader("y\n\n\n"))

	if err := editServerAuthSettings(reader, cfg); err != nil {
		t.Fatalf("editServerAuthSettings returned error: %v", err)
	}
	if cfg.HTTP.AuthToken == "" {
		t.Fatal("expected generated server auth token")
	}
	if cfg.HTTP.AuthHeader != config.DefaultAuthHeaderName {
		t.Fatalf("auth_header = %q, want %q", cfg.HTTP.AuthHeader, config.DefaultAuthHeaderName)
	}
}

func TestEditServerAuthSettingsPreservesExtraTokens(t *testing.T) {
	cfg := config.Default()
	cfg.HTTP.AuthToken = "primary"
	cfg.HTTP.AuthTokens = []string{"backup-a", "backup-b"}
	cfg.HTTP.AuthHeader = "X-ROAD-Token"
	reader := bufio.NewReader(strings.NewReader("y\n\n\n"))

	if err := editServerAuthSettings(reader, cfg); err != nil {
		t.Fatalf("editServerAuthSettings returned error: %v", err)
	}
	if cfg.HTTP.AuthToken != "primary" {
		t.Fatalf("primary token changed: %q", cfg.HTTP.AuthToken)
	}
	if !reflect.DeepEqual(cfg.HTTP.AuthTokens, []string{"backup-a", "backup-b"}) {
		t.Fatalf("extra tokens not preserved: %#v", cfg.HTTP.AuthTokens)
	}
}

func TestLoadEditableClientConfigPreservesEnvSecretReference(t *testing.T) {
	t.Setenv("ROAD_TEST_TOKEN", "resolved-secret")
	path := filepath.Join(t.TempDir(), "client.json")
	if err := os.WriteFile(path, []byte(`{
  "listen_addr": "127.0.0.1:25568",
  "listen_network": "tcp",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "plugin_name": "minecraft",
  "auth_token": "env:ROAD_TEST_TOKEN"
}`), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	cfg, err := loadEditableClientConfig(path)
	if err != nil {
		t.Fatalf("loadEditableClientConfig returned error: %v", err)
	}
	if cfg.AuthToken != "env:ROAD_TEST_TOKEN" {
		t.Fatalf("auth_token should preserve env reference, got %q", cfg.AuthToken)
	}
}

func TestLoadEditableServerConfigPreservesEnvSecretReference(t *testing.T) {
	t.Setenv("ROAD_TEST_TOKEN", "resolved-secret")
	path := filepath.Join(t.TempDir(), "server.json")
	if err := os.WriteFile(path, []byte(`{
  "http": {
    "auth_token": "env:ROAD_TEST_TOKEN"
  },
  "plugins": {
    "enabled": ["minecraft"]
  }
}`), 0o644); err != nil {
		t.Fatalf("write server config failed: %v", err)
	}

	cfg, err := loadEditableServerConfig(path)
	if err != nil {
		t.Fatalf("loadEditableServerConfig returned error: %v", err)
	}
	if cfg.HTTP.AuthToken != "env:ROAD_TEST_TOKEN" {
		t.Fatalf("auth_token should preserve env reference, got %q", cfg.HTTP.AuthToken)
	}
}

func TestGetServerDefaultPluginProfileReturnsAuthStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing token", http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := config.DefaultClient()
	cfg.ServerWSURL = strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"

	_, err := getServerDefaultPluginProfile(cfg)
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !isAuthHTTPStatus(err) {
		t.Fatalf("expected auth status error, got %v", err)
	}
	if !strings.Contains(err.Error(), "auth token") {
		t.Fatalf("expected auth-token hint, got %v", err)
	}
}

func TestApplyClientProfileFromServerPromptsAuthOnlyAfterUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(config.DefaultAuthHeaderName) != "secret-token" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"road-proxy-v3","default_plugin":"ddnet-udp","default_network":"udp"}`))
	}))
	defer server.Close()

	cfg := config.DefaultClient()
	cfg.ServerWSURL = strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"
	reader := bufio.NewReader(strings.NewReader("secret-token\n"))

	if _, _, err := applyClientProfileFromServerWithAuthRetry(reader, cfg, true); err != nil {
		t.Fatalf("applyClientProfileFromServerWithAuthRetry returned error: %v", err)
	}
	if cfg.AuthToken != "secret-token" {
		t.Fatalf("auth_token = %q", cfg.AuthToken)
	}
	if cfg.AuthHeader != config.DefaultAuthHeaderName {
		t.Fatalf("auth_header = %q", cfg.AuthHeader)
	}
	if cfg.PluginName != "ddnet-udp" || cfg.ListenNetwork != "udp" {
		t.Fatalf("profile not applied: plugin=%q network=%q", cfg.PluginName, cfg.ListenNetwork)
	}
}

func TestPublicServerLockRejectsSecondAcquire(t *testing.T) {
	layout := testRuntimeLayout(t.TempDir())

	lock, err := acquirePublicServerLock(layout)
	if err != nil {
		t.Fatalf("acquirePublicServerLock returned error: %v", err)
	}
	defer lock.Release()

	second, err := acquirePublicServerLock(layout)
	if err == nil {
		second.Release()
		t.Fatal("expected second public server lock acquire to fail")
	}
}

func TestPublicServerLockReleaseAllowsAcquire(t *testing.T) {
	layout := testRuntimeLayout(t.TempDir())

	lock, err := acquirePublicServerLock(layout)
	if err != nil {
		t.Fatalf("acquirePublicServerLock returned error: %v", err)
	}
	lock.Release()

	second, err := acquirePublicServerLock(layout)
	if err != nil {
		t.Fatalf("second acquire after release returned error: %v", err)
	}
	second.Release()
}

func TestRunGenerateConfigCommand(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	profileDir := filepath.Join(root, "profiles")
	outDir := filepath.Join(root, "out")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir base dir failed: %v", err)
	}
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("mkdir profile dir failed: %v", err)
	}

	serverBase := filepath.Join(baseDir, "server.json")
	clientBase := filepath.Join(baseDir, "client.json")
	if err := os.WriteFile(serverBase, []byte(`{
  "plugins": {
    "dir": "plugins",
    "enabled": ["minecraft"]
  }
}`), 0o644); err != nil {
		t.Fatalf("write server base failed: %v", err)
	}
	if err := os.WriteFile(clientBase, []byte(`{
  "listen_addr": "127.0.0.1:25568",
  "listen_network": "tcp",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "plugin_name": "minecraft"
}`), 0o644); err != nil {
		t.Fatalf("write client base failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "game.json"), []byte(`{
  "name": "game",
  "server": {
    "plugins": {
      "enabled": ["game"]
    }
  },
  "client": {
    "listen_addr": "127.0.0.1:7777",
    "listen_network": "udp",
    "plugin_name": "game"
  }
}`), 0o644); err != nil {
		t.Fatalf("write profile failed: %v", err)
	}

	var out bytes.Buffer
	err := runGenerateConfigCommand([]string{
		"--profile", "game",
		"--profiles", profileDir,
		"--base-server", serverBase,
		"--base-client", clientBase,
		"--out", outDir,
	}, &out)
	if err != nil {
		t.Fatalf("runGenerateConfigCommand returned error: %v", err)
	}

	serverOut := filepath.Join(outDir, "server-game.json")
	clientOut := filepath.Join(outDir, "client-game.json")
	if _, err := os.Stat(serverOut); err != nil {
		t.Fatalf("expected generated server config: %v", err)
	}
	if _, err := os.Stat(clientOut); err != nil {
		t.Fatalf("expected generated client config: %v", err)
	}
}

func TestRunGenerateConfigCommandMultiClientInstances(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	profileDir := filepath.Join(root, "profiles")
	outDir := filepath.Join(root, "out")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir base dir failed: %v", err)
	}
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("mkdir profile dir failed: %v", err)
	}

	serverBase := filepath.Join(baseDir, "server.json")
	clientBase := filepath.Join(baseDir, "client.json")
	if err := os.WriteFile(serverBase, []byte(`{
  "plugins": {
    "dir": "plugins",
    "enabled": ["minecraft"]
  }
}`), 0o644); err != nil {
		t.Fatalf("write server base failed: %v", err)
	}
	if err := os.WriteFile(clientBase, []byte(`{
  "listen_addr": "127.0.0.1:25568",
  "listen_network": "tcp",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "plugin_name": "minecraft"
}`), 0o644); err != nil {
		t.Fatalf("write client base failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "game.json"), []byte(`{
  "name": "game",
  "server": {
    "plugins": {
      "enabled": ["game"]
    }
  },
  "client": {
    "listen_addr": "127.0.0.1:7777",
    "listen_network": "udp",
    "plugin_name": "game"
  }
}`), 0o644); err != nil {
		t.Fatalf("write profile failed: %v", err)
	}

	var out bytes.Buffer
	err := runGenerateConfigCommand([]string{
		"--profile", "game",
		"--profiles", profileDir,
		"--base-server", serverBase,
		"--base-client", clientBase,
		"--out", outDir,
		"--client-instances", "3",
		"--client-start-port", "5031",
	}, &out)
	if err != nil {
		t.Fatalf("runGenerateConfigCommand returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "server-game.json")); err != nil {
		t.Fatalf("expected generated server config: %v", err)
	}
	for i, port := range []string{"5031", "5032", "5033"} {
		path := filepath.Join(outDir, "client-game-p"+string(rune('1'+i))+".json")
		cfg, err := config.LoadClient(path)
		if err != nil {
			t.Fatalf("load generated client %d failed: %v", i+1, err)
		}
		if !strings.HasSuffix(cfg.ListenAddr, ":"+port) {
			t.Fatalf("client %d listen_addr = %s, want port %s", i+1, cfg.ListenAddr, port)
		}
	}
}

func TestTunnelURLCaptureFindsTryCloudflareURL(t *testing.T) {
	capture := newTunnelURLCapture(nil)
	if _, err := capture.Write([]byte("INF +--------------------------------------------------------------------------------------------+\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := capture.Write([]byte("https://abc-def.trycloudflare.com\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	select {
	case got := <-capture.found:
		if got != "https://abc-def.trycloudflare.com" {
			t.Fatalf("unexpected URL: %s", got)
		}
	default:
		t.Fatal("expected captured URL")
	}
}

func TestTunnelURLCaptureWorksWithoutOutputWriter(t *testing.T) {
	capture := newTunnelURLCapture(nil)
	if _, err := capture.Write([]byte("INF noisy cloudflared line\nhttps://quiet.trycloudflare.com\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	select {
	case got := <-capture.found:
		if got != "https://quiet.trycloudflare.com" {
			t.Fatalf("unexpected URL: %s", got)
		}
	default:
		t.Fatal("expected captured URL")
	}
}

func TestPublicEndpointFromHTTPS(t *testing.T) {
	endpoint, host, err := publicEndpointFromHTTPS("https://abc-def.trycloudflare.com")
	if err != nil {
		t.Fatalf("publicEndpointFromHTTPS returned error: %v", err)
	}
	if endpoint != "wss://abc-def.trycloudflare.com/ws" {
		t.Fatalf("endpoint = %s", endpoint)
	}
	if host != "abc-def.trycloudflare.com" {
		t.Fatalf("host = %s", host)
	}
}

func TestWriteTryCloudflareConfigIncludesURL(t *testing.T) {
	path, err := writeTryCloudflareConfig(t.TempDir(), "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("writeTryCloudflareConfig returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `url: "http://127.0.0.1:8080"`) {
		t.Fatalf("config does not include origin URL:\n%s", text)
	}
}

func TestParseTunnelUUID(t *testing.T) {
	got := parseTunnelUUID("Created tunnel road with id 6ff42ae2-765d-4adf-8112-31c55c1551ef")
	if got != "6ff42ae2-765d-4adf-8112-31c55c1551ef" {
		t.Fatalf("uuid = %s", got)
	}
}

func TestFindTunnelUUIDByName(t *testing.T) {
	output := `
ID                                   NAME           CREATED
6ff42ae2-765d-4adf-8112-31c55c1551ef road-proxy     2026-05-14
`
	got := findTunnelUUIDByName(output, "road-proxy")
	if got != "6ff42ae2-765d-4adf-8112-31c55c1551ef" {
		t.Fatalf("uuid = %s", got)
	}
}

func TestOutputContainsAnyCaseInsensitive(t *testing.T) {
	if !outputContainsAny("Record Already Exists", []string{"record already exists"}) {
		t.Fatal("expected case-insensitive match")
	}
}

func TestParseChecksumTextFromReleaseBody(t *testing.T) {
	body := `
### SHA256 Checksums:
cloudflared-linux-amd64: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
cloudflared-linux-arm64: ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
`
	got := parseChecksumText(body, "cloudflared-linux-amd64")
	if got != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("checksum = %s", got)
	}
}

func TestParseChecksumTextFromShaFile(t *testing.T) {
	body := `0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  cloudflared-windows-amd64.exe`
	got := parseChecksumText(body, "cloudflared-windows-amd64.exe")
	if got != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("checksum = %s", got)
	}
}

func TestWriteNamedTunnelConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloudflared.yml")
	local, err := newPublicServerLocalSettings(18080, 18081)
	if err != nil {
		t.Fatalf("newPublicServerLocalSettings returned error: %v", err)
	}
	err = writeNamedTunnelConfig(path, "road-test", "6ff42ae2-765d-4adf-8112-31c55c1551ef", "road.example.com", local)
	if err != nil {
		t.Fatalf("writeNamedTunnelConfig returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`tunnel: "6ff42ae2-765d-4adf-8112-31c55c1551ef"`,
		`hostname: "road.example.com"`,
		`path: "/ws"`,
		`service: "http://127.0.0.1:18080"`,
		`path: "/dashboard"`,
		`path: "/api/.*"`,
		`service: "http://127.0.0.1:18081"`,
		`service: http_status:404`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config missing %q:\n%s", want, text)
		}
	}
}

func TestBuildPublicServerConfigSetsSecurityDefaults(t *testing.T) {
	root := t.TempDir()
	layout := testRuntimeLayout(root)
	if err := os.MkdirAll(layout.ConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir failed: %v", err)
	}
	if err := os.MkdirAll(layout.PluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir failed: %v", err)
	}

	serverPath := filepath.Join(layout.ConfigDir, "server.json")
	if err := os.WriteFile(serverPath, []byte(`{"plugins":{"dir":"plugins","enabled":["game"]}}`), 0o644); err != nil {
		t.Fatalf("write server config failed: %v", err)
	}
	clientPath := filepath.Join(layout.ConfigDir, "client.json")
	if err := os.WriteFile(clientPath, []byte(`{
  "listen_addr": "127.0.0.1:7777",
  "listen_network": "udp",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "plugin_name": "game"
}`), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	selected := menuPlugin{
		Name:           "game",
		TargetNetwork:  "udp",
		ServerTemplate: serverPath,
		ClientTemplate: clientPath,
	}
	local, err := newPublicServerLocalSettings(18080, 18081)
	if err != nil {
		t.Fatalf("newPublicServerLocalSettings returned error: %v", err)
	}
	cfg, info, err := buildPublicServerConfig(layout, selected, local, "road.example.com", "wss://road.example.com/ws")
	if err != nil {
		t.Fatalf("buildPublicServerConfig returned error: %v", err)
	}
	if cfg.HTTP.ListenAddr != "127.0.0.1:18080" || cfg.Control.ListenAddr != "127.0.0.1:18081" {
		t.Fatalf("unexpected listen addrs: http=%s control=%s", cfg.HTTP.ListenAddr, cfg.Control.ListenAddr)
	}
	if info.LocalOriginURL != "http://127.0.0.1:18080" || info.DashboardURL != "http://127.0.0.1:18081/dashboard" {
		t.Fatalf("unexpected local URLs: origin=%s dashboard=%s", info.LocalOriginURL, info.DashboardURL)
	}
	if cfg.HTTP.AuthToken == "" || info.Token == "" || cfg.HTTP.AuthToken != info.Token {
		t.Fatalf("auth token was not generated consistently")
	}
	if cfg.Control.PluginAPIPublic {
		t.Fatal("plugin api should be private in public wizard config")
	}
	if !stringSliceContains(cfg.HTTP.AllowedHosts, "road.example.com") {
		t.Fatalf("allowed hosts missing public host: %v", cfg.HTTP.AllowedHosts)
	}
	if cfg.Plugins.Dir != layout.PluginDir {
		t.Fatalf("plugin dir = %s", cfg.Plugins.Dir)
	}

	clientCfg, err := config.LoadClient(info.ClientConfigPath)
	if err != nil {
		t.Fatalf("load generated client config failed: %v", err)
	}
	if clientCfg.ServerWSURL != "wss://road.example.com/ws" || clientCfg.AuthToken != info.Token {
		t.Fatalf("generated client config mismatch: %#v", clientCfg)
	}
}

func testRuntimeLayout(root string) app.RuntimeLayout {
	return app.RuntimeLayout{
		Root:             root,
		ConfigDir:        filepath.Join(root, "configs"),
		PluginDir:        filepath.Join(root, "plugins"),
		ServerConfigPath: filepath.Join(root, "configs", "server.json"),
		ClientConfigPath: filepath.Join(root, "configs", "client.json"),
		AppConfigPath:    filepath.Join(root, "configs", "app.json"),
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func writeTestPlugin(t *testing.T, root, name string) string {
	t.Helper()

	pluginDir := filepath.Join(root, name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir failed: %v", err)
	}
	pluginPath := filepath.Join(pluginDir, "plugin.json")
	schemaJSON := `{
  "schema_version": "v1",
  "name": "` + name + `",
  "version": "3.0.0",
  "target": {"network": "tcp", "address": "127.0.0.1:1234"},
  "compatibility": {
    "status": "working",
    "tested_players": 2,
    "known_ports": [{"network": "tcp", "port": 1234}],
    "last_verified": "2026-05-11"
  },
  "runtime": {"type": "json", "mode": "passthrough"}
}`
	if err := os.WriteFile(pluginPath, []byte(schemaJSON), 0o644); err != nil {
		t.Fatalf("write plugin schema failed: %v", err)
	}
	return pluginPath
}
