package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClientNormalizesDefaults(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	cfg, err := LoadClient(path)
	if err != nil {
		t.Fatalf("load client config failed: %v", err)
	}

	if cfg.PluginName == "" {
		t.Fatal("plugin_name should be defaulted")
	}
	if cfg.ListenNetwork != "tcp" {
		t.Fatalf("listen_network should be defaulted to tcp, got %q", cfg.ListenNetwork)
	}
	if cfg.AuthHeader != "" {
		t.Fatalf("auth_header should be empty, got %q", cfg.AuthHeader)
	}
	if cfg.ConnectRetries < 0 {
		t.Fatalf("connect_retries should be normalized, got %d", cfg.ConnectRetries)
	}
	if cfg.RetryInitialDelayDuration() <= 0 {
		t.Fatal("retry_initial_delay should be parsed to positive duration")
	}
	if cfg.RetryMaxDelayDuration() <= 0 {
		t.Fatal("retry_max_delay should be parsed to positive duration")
	}
	if cfg.WSIdleTimeoutDuration() <= 0 {
		t.Fatal("ws_idle_timeout should be parsed to positive duration")
	}
	if cfg.WSPingIntervalDuration() <= 0 {
		t.Fatal("ws_ping_interval should be parsed to positive duration")
	}
	if cfg.UDPSessionIdleDuration() <= 0 {
		t.Fatal("udp_session_idle_timeout should be parsed to positive duration")
	}
	if cfg.UDPMetricsLogIntervalDuration() <= 0 {
		t.Fatal("udp_metrics_log_interval should be parsed to positive duration")
	}
	if cfg.MaxWSMessageBytes <= 0 {
		t.Fatalf("max_ws_message_bytes should be positive, got %d", cfg.MaxWSMessageBytes)
	}
	if cfg.BufferSize < 1024 {
		t.Fatalf("buffer size should be normalized, got %d", cfg.BufferSize)
	}
	if cfg.Logging.Format != "text" {
		t.Fatalf("logging.format should default to text, got %q", cfg.Logging.Format)
	}
	if cfg.UDPRecord.Path == "" || cfg.UDPRecord.DurationValue() <= 0 || cfg.UDPRecord.MaxEvents <= 0 {
		t.Fatalf("udp_record defaults were not normalized: %#v", cfg.UDPRecord)
	}
}

func TestLoadClientAcceptsJSONLoggingFormat(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "logging": {"format": "json"}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	cfg, err := LoadClient(path)
	if err != nil {
		t.Fatalf("load client config failed: %v", err)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("logging.format = %q, want json", cfg.Logging.Format)
	}
}

func TestLoadClientRejectsInvalidLoggingFormat(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "logging": {"format": "xml"}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected client config load error for invalid logging.format")
	}
}

func TestLoadClientRejectsInvalidUDPRecordMaxEvents(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "udp_record": {"enabled": true, "max_events": -1}
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected client config load error for invalid udp_record.max_events")
	}
}

func TestLoadClientRejectsInvalidDuration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "handshake_timeout": "invalid"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected client config load error")
	}
}

func TestLoadClientRejectsInvalidRetryDuration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "retry_initial_delay": "invalid"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected client config load error for invalid retry_initial_delay")
	}
}

func TestLoadClientRejectsInvalidWSIdleTimeout(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "ws_idle_timeout": "invalid"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected client config load error for invalid ws_idle_timeout")
	}
}

func TestLoadClientRejectsInvalidWSPingInterval(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "ws_ping_interval": "invalid"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected client config load error for invalid ws_ping_interval")
	}
}

func TestLoadClientRejectsInvalidUDPMetricsLogInterval(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "udp_metrics_log_interval": "invalid"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected client config load error for invalid udp_metrics_log_interval")
	}
}

func TestLoadClientRejectsInvalidListenNetwork(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "listen_network": "sctp",
  "server_ws_url": "ws://127.0.0.1:8080/ws"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected client config load error for invalid listen_network")
	}
}

func TestLoadClientAcceptsUDPListeners(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_network": "udp",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "udp_listeners": [
    {"listen_addr": "127.0.0.1:8766", "target": "game"},
    {"listen_addr": "127.0.0.1:27016", "target": "query"}
  ]
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	cfg, err := LoadClient(path)
	if err != nil {
		t.Fatalf("load client config failed: %v", err)
	}
	if len(cfg.UDPListeners) != 2 {
		t.Fatalf("expected 2 udp listeners, got %d", len(cfg.UDPListeners))
	}
	if cfg.UDPListeners[1].Target != "query" {
		t.Fatalf("unexpected second target: %q", cfg.UDPListeners[1].Target)
	}
}

func TestLoadClientRejectsUDPListenersForTCP(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_network": "tcp",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "udp_listeners": [
    {"listen_addr": "127.0.0.1:8766", "target": "game"}
  ]
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected udp_listeners with tcp listen_network to fail")
	}
}

func TestLoadClientRejectsDuplicateUDPListeners(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_network": "udp",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "udp_listeners": [
    {"listen_addr": "127.0.0.1:8766", "target": "game"},
    {"listen_addr": "127.0.0.1:8766", "target": "query"}
  ]
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected duplicate udp listeners to fail")
	}
}

func TestLoadClientRejectsInvalidServerWSURL(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "http://127.0.0.1:8080/ws"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected client config load error for invalid server_ws_url")
	}
}

func TestLoadClientWithOptionsCanSkipServerWSURLValidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ftp://127.0.0.1:8080/ws"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	cfg, err := LoadClientWithOptions(path, ClientNormalizeOptions{ValidateServerWSURL: false})
	if err != nil {
		t.Fatalf("expected lenient client config load to pass, got %v", err)
	}
	if cfg.ServerWSURL != "ftp://127.0.0.1:8080/ws" {
		t.Fatalf("unexpected server_ws_url: %s", cfg.ServerWSURL)
	}
}

func TestLoadClientEnablesAuthTokenConfig(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.json")
	t.Setenv("ROAD_PROXY_TEST_CLIENT_AUTH_TOKEN", "client-secret")

	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "auth_token": "env:ROAD_PROXY_TEST_CLIENT_AUTH_TOKEN",
  "auth_header": "X-Proxy-Token"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	cfg, err := LoadClient(path)
	if err != nil {
		t.Fatalf("load client config failed: %v", err)
	}
	if cfg.AuthToken != "env:ROAD_PROXY_TEST_CLIENT_AUTH_TOKEN" || cfg.AuthHeader != "X-Proxy-Token" {
		t.Fatalf("unexpected auth config: auth_token=%q auth_header=%q", cfg.AuthToken, cfg.AuthHeader)
	}
	if got := ResolveSecret(cfg.AuthToken); got != "client-secret" {
		t.Fatalf("auth_token env did not resolve at runtime: %q", got)
	}
}

func TestLoadClientRejectsMissingEnvAuthToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.json")
	raw := `{
  "listen_addr": "127.0.0.1:25568",
  "server_ws_url": "ws://127.0.0.1:8080/ws",
  "auth_token": "env:ROAD_PROXY_TEST_MISSING_CLIENT_AUTH_TOKEN"
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write client config failed: %v", err)
	}

	if _, err := LoadClient(path); err == nil {
		t.Fatal("expected missing env auth token to fail runtime load")
	}
	if _, err := LoadClientWithOptions(path, ClientNormalizeOptions{ValidateServerWSURL: true, AllowMissingEnvSecrets: true}); err != nil {
		t.Fatalf("validation load should allow missing env auth token: %v", err)
	}
}
