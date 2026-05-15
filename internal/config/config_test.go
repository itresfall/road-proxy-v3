package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadNormalizesDefaults(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.json")

	raw := `{
  "tcp": {"listen_addr": "127.0.0.1:25567"},
  "plugins": {"dir": "plugins", "enabled": ["minecraft"]}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}

	if cfg.HTTP.ListenAddr == "" {
		t.Fatal("http.listen_addr should be defaulted")
	}
	if cfg.Control.ListenAddr == "" {
		t.Fatal("control.listen_addr should be defaulted")
	}
	if cfg.HTTP.WSEndpoint == "" {
		t.Fatal("http.ws_endpoint should be defaulted")
	}
	if cfg.HTTP.AuthHeader != "" {
		t.Fatalf("http.auth_header should be empty, got %q", cfg.HTTP.AuthHeader)
	}
	if cfg.HTTP.WSIdleTimeout == "" {
		t.Fatal("http.ws_idle_timeout should be defaulted")
	}
	if cfg.HTTP.WSPingInterval == "" {
		t.Fatal("http.ws_ping_interval should be defaulted")
	}
	if cfg.HTTP.MaxWSMessageBytes <= 0 {
		t.Fatalf("http.max_ws_message_bytes should be positive, got %d", cfg.HTTP.MaxWSMessageBytes)
	}
	if cfg.TCP.BufferSize < 1024 {
		t.Fatalf("buffer size should be normalized, got %d", cfg.TCP.BufferSize)
	}
	if cfg.Logging.Format != "text" {
		t.Fatalf("logging.format should default to text, got %q", cfg.Logging.Format)
	}
	if cfg.UDPRecord.Path == "" || cfg.UDPRecord.DurationValue() <= 0 || cfg.UDPRecord.MaxEvents <= 0 {
		t.Fatalf("udp_record defaults were not normalized: %#v", cfg.UDPRecord)
	}
}

func TestLoadAcceptsJSONLoggingFormat(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.json")

	raw := `{
  "tcp": {"listen_addr": "127.0.0.1:25567"},
  "plugins": {"dir": "plugins", "enabled": ["minecraft"]},
  "logging": {"format": "json"}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("logging.format = %q, want json", cfg.Logging.Format)
	}
}

func TestLoadRejectsInvalidLoggingFormat(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.json")

	raw := `{
  "tcp": {"listen_addr": "127.0.0.1:25567"},
  "plugins": {"dir": "plugins", "enabled": ["minecraft"]},
  "logging": {"format": "xml"}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected config load error for invalid logging.format")
	}
}

func TestLoadRejectsInvalidUDPRecordDuration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.json")

	raw := `{
  "tcp": {"listen_addr": "127.0.0.1:25567"},
  "plugins": {"dir": "plugins", "enabled": ["minecraft"]},
  "udp_record": {"enabled": true, "duration": "bad-duration"}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected config load error for invalid udp_record.duration")
	}
}

func TestLoadRejectsInvalidHTTPDuration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.json")

	raw := `{
  "tcp": {"listen_addr": "127.0.0.1:25567"},
  "http": {"read_timeout": "not-a-duration"},
  "plugins": {"dir": "plugins", "enabled": ["minecraft"]}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected config load error for invalid http.read_timeout")
	}
}

func TestLoadRejectsInvalidWSIdleTimeout(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.json")

	raw := `{
  "tcp": {"listen_addr": "127.0.0.1:25567"},
  "http": {"ws_idle_timeout": "bad-duration"},
  "plugins": {"dir": "plugins", "enabled": ["minecraft"]}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected config load error for invalid http.ws_idle_timeout")
	}
}

func TestLoadRejectsInvalidWSPingInterval(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.json")

	raw := `{
  "tcp": {"listen_addr": "127.0.0.1:25567"},
  "http": {"ws_ping_interval": "bad-duration"},
  "plugins": {"dir": "plugins", "enabled": ["minecraft"]}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected config load error for invalid http.ws_ping_interval")
	}
}

func TestLoadRejectsInvalidSecurityLimits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.json")

	raw := `{
  "http": {"max_connections": -1},
  "plugins": {"dir": "plugins", "enabled": ["minecraft"]}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected config load error for invalid max_connections")
	}
}

func TestLoadEnablesAuthTokenConfig(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.json")
	t.Setenv("ROAD_PROXY_TEST_AUTH_TOKEN", "env-secret")

	raw := `{
  "tcp": {"listen_addr": "127.0.0.1:25567"},
  "http": {
    "auth_token": "env:ROAD_PROXY_TEST_AUTH_TOKEN",
    "auth_tokens": ["a", "b"],
    "auth_header": "X-Proxy-Token"
  },
  "plugins": {"dir": "plugins", "enabled": ["minecraft"]}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if !cfg.WSAuthEnabled() {
		t.Fatal("expected ws auth to be enabled")
	}
	if cfg.HTTP.AuthToken != "env:ROAD_PROXY_TEST_AUTH_TOKEN" {
		t.Fatalf("auth_token should preserve env reference, got %q", cfg.HTTP.AuthToken)
	}
	if cfg.HTTP.AuthHeader != "X-Proxy-Token" {
		t.Fatalf("unexpected auth header: %q", cfg.HTTP.AuthHeader)
	}
	tokens := cfg.WSAuthTokens()
	if len(tokens) != 3 || tokens[0] != "env-secret" || tokens[1] != "a" || tokens[2] != "b" {
		t.Fatalf("unexpected auth tokens: %#v", tokens)
	}
}

func TestLoadRejectsMissingEnvAuthToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.json")
	raw := `{
  "http": {"auth_token": "env:ROAD_PROXY_TEST_MISSING_AUTH_TOKEN"},
  "plugins": {"dir": "plugins", "enabled": ["minecraft"]}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected missing env auth token to fail runtime load")
	}
	if _, err := LoadWithOptions(path, NormalizeOptions{AllowMissingEnvSecrets: true}); err != nil {
		t.Fatalf("validation load should allow missing env auth token: %v", err)
	}
}

func TestHasOpenNoAuthListener(t *testing.T) {
	cfg := Default()
	if !cfg.HasOpenNoAuthListener() {
		t.Fatal("default wildcard listeners without auth should be reported")
	}

	cfg.HTTP.AuthToken = "secret"
	if !cfg.WSAuthEnabled() {
		t.Fatal("expected auth to be enabled")
	}
	if cfg.HasOpenNoAuthListener() {
		t.Fatal("auth-protected wildcard listeners should not be reported")
	}
}

func TestNormalizePluginNames(t *testing.T) {
	cfg := Default()
	cfg.Plugins.Enabled = []string{" minecraft ", "gzdoom-udp"}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if got := strings.Join(cfg.Plugins.Enabled, ","); got != "minecraft,gzdoom-udp" {
		t.Fatalf("unexpected normalized plugins: %#v", cfg.Plugins.Enabled)
	}
}

func TestNormalizeRejectsDuplicatePluginNames(t *testing.T) {
	cfg := Default()
	cfg.Plugins.Enabled = []string{"minecraft", " minecraft "}
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected duplicate plugin name error")
	}
}
