package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultTCPListenAddr        = "0.0.0.0:25567"
	defaultBufferSize           = 32768
	defaultDialTimeout          = 5 * time.Second
	defaultKeepAlive            = 30 * time.Second
	defaultPluginDir            = "plugins"
	defaultDataPlaneAddr        = "0.0.0.0:8080"
	defaultControlPlaneAddr     = "0.0.0.0:8081"
	defaultWSEndpoint           = "/ws"
	defaultWSIdleTimeout        = 2 * time.Minute
	defaultWSPingInterval       = 30 * time.Second
	defaultMaxWSMessageBytes    = 1 << 20
	defaultHTTPReadTimeout      = 10 * time.Second
	defaultHTTPWriteTimeout     = 10 * time.Second
	defaultHTTPHandshakeTimeout = 5 * time.Second
	defaultUDPRecordDuration    = 30 * time.Second
	defaultUDPRecordMaxEvents   = 5000
	minimumSafeBufferSize       = 1024
	DefaultAuthHeaderName       = "X-ROAD-Token"
)

type Config struct {
	TCP       TCPConfig       `json:"tcp"`
	HTTP      HTTPConfig      `json:"http"`
	Control   ControlConfig   `json:"control"`
	Plugins   PluginsConfig   `json:"plugins"`
	Logging   LoggingConfig   `json:"logging"`
	UDPRecord UDPRecordConfig `json:"udp_record"`
}

type TCPConfig struct {
	ListenAddr      string `json:"listen_addr"`
	BufferSize      int    `json:"buffer_size"`
	DialTimeout     string `json:"dial_timeout"`
	KeepAlivePeriod string `json:"keep_alive_period"`
}

type PluginsConfig struct {
	Dir     string   `json:"dir"`
	Enabled []string `json:"enabled"`
}

type LoggingConfig struct {
	Format string `json:"format"`
}

type UDPRecordConfig struct {
	Enabled   bool   `json:"enabled"`
	Path      string `json:"path"`
	Duration  string `json:"duration"`
	MaxEvents int    `json:"max_events"`
}

type HTTPConfig struct {
	Enabled             bool     `json:"enabled"`
	ListenAddr          string   `json:"listen_addr"`
	WSEndpoint          string   `json:"ws_endpoint"`
	AuthToken           string   `json:"auth_token"`
	AuthTokens          []string `json:"auth_tokens"`
	AuthHeader          string   `json:"auth_header"`
	AllowedOrigins      []string `json:"allowed_origins"`
	AllowedHosts        []string `json:"allowed_hosts"`
	TrustProxyHeaders   bool     `json:"trust_proxy_headers"`
	MaxConnections      int      `json:"max_connections"`
	MaxConnectionsPerIP int      `json:"max_connections_per_ip"`
	RateLimitPerMinute  int      `json:"rate_limit_per_minute"`
	WSIdleTimeout       string   `json:"ws_idle_timeout"`
	WSPingInterval      string   `json:"ws_ping_interval"`
	MaxWSMessageBytes   int      `json:"max_ws_message_bytes"`
	EnableCompression   bool     `json:"enable_compression"`
	ReadTimeout         string   `json:"read_timeout"`
	WriteTimeout        string   `json:"write_timeout"`
	HandshakeTimeout    string   `json:"handshake_timeout"`
	ReadHeaderTimeout   string   `json:"read_header_timeout"`
	MaxHeaderBytes      int      `json:"max_header_bytes"`
}

type ControlConfig struct {
	Enabled           bool   `json:"enabled"`
	ListenAddr        string `json:"listen_addr"`
	PluginAPIPublic   bool   `json:"plugin_api_public"`
	ReadTimeout       string `json:"read_timeout"`
	WriteTimeout      string `json:"write_timeout"`
	ReadHeaderTimeout string `json:"read_header_timeout"`
	MaxHeaderBytes    int    `json:"max_header_bytes"`
}

func Default() *Config {
	return &Config{
		TCP: TCPConfig{
			ListenAddr:      defaultTCPListenAddr,
			BufferSize:      defaultBufferSize,
			DialTimeout:     defaultDialTimeout.String(),
			KeepAlivePeriod: defaultKeepAlive.String(),
		},
		HTTP: HTTPConfig{
			Enabled:             true,
			ListenAddr:          defaultDataPlaneAddr,
			WSEndpoint:          defaultWSEndpoint,
			AuthToken:           "",
			AuthTokens:          []string{},
			AuthHeader:          "",
			AllowedOrigins:      []string{},
			AllowedHosts:        []string{},
			TrustProxyHeaders:   false,
			MaxConnections:      0,
			MaxConnectionsPerIP: 0,
			RateLimitPerMinute:  0,
			WSIdleTimeout:       defaultWSIdleTimeout.String(),
			WSPingInterval:      defaultWSPingInterval.String(),
			MaxWSMessageBytes:   defaultMaxWSMessageBytes,
			EnableCompression:   false,
			ReadTimeout:         defaultHTTPReadTimeout.String(),
			WriteTimeout:        defaultHTTPWriteTimeout.String(),
			HandshakeTimeout:    defaultHTTPHandshakeTimeout.String(),
			ReadHeaderTimeout:   defaultHTTPHandshakeTimeout.String(),
			MaxHeaderBytes:      1 << 20,
		},
		Control: ControlConfig{
			Enabled:           true,
			ListenAddr:        defaultControlPlaneAddr,
			PluginAPIPublic:   true,
			ReadTimeout:       defaultHTTPReadTimeout.String(),
			WriteTimeout:      defaultHTTPWriteTimeout.String(),
			ReadHeaderTimeout: defaultHTTPHandshakeTimeout.String(),
			MaxHeaderBytes:    1 << 20,
		},
		Plugins: PluginsConfig{
			Dir:     defaultPluginDir,
			Enabled: []string{"minecraft"},
		},
		Logging: LoggingConfig{
			Format: "text",
		},
		UDPRecord: UDPRecordConfig{
			Enabled:   false,
			Path:      "logs/udp-record-server.jsonl",
			Duration:  defaultUDPRecordDuration.String(),
			MaxEvents: defaultUDPRecordMaxEvents,
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	if err := cfg.Normalize(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Normalize() error {
	if c.TCP.ListenAddr == "" {
		c.TCP.ListenAddr = defaultTCPListenAddr
	}
	if c.TCP.BufferSize < minimumSafeBufferSize {
		c.TCP.BufferSize = defaultBufferSize
	}
	if c.HTTP.ListenAddr == "" {
		c.HTTP.ListenAddr = defaultDataPlaneAddr
	}
	if c.HTTP.WSEndpoint == "" {
		c.HTTP.WSEndpoint = defaultWSEndpoint
	}
	c.HTTP.AuthToken = ResolveSecret(c.HTTP.AuthToken)
	c.HTTP.AuthTokens = normalizeSecretList(c.HTTP.AuthToken, c.HTTP.AuthTokens)
	if len(c.HTTP.AuthTokens) > 0 {
		c.HTTP.AuthHeader = normalizeAuthHeaderName(c.HTTP.AuthHeader)
		if c.HTTP.AuthHeader == "" {
			c.HTTP.AuthHeader = DefaultAuthHeaderName
		}
		if !validHTTPHeaderName(c.HTTP.AuthHeader) {
			return fmt.Errorf("invalid http.auth_header: %q", c.HTTP.AuthHeader)
		}
	} else {
		c.HTTP.AuthHeader = strings.TrimSpace(c.HTTP.AuthHeader)
	}
	c.HTTP.AllowedOrigins = normalizeStringList(c.HTTP.AllowedOrigins)
	c.HTTP.AllowedHosts = normalizeStringList(c.HTTP.AllowedHosts)
	if c.HTTP.MaxConnections < 0 {
		return fmt.Errorf("invalid http.max_connections: must be >= 0")
	}
	if c.HTTP.MaxConnectionsPerIP < 0 {
		return fmt.Errorf("invalid http.max_connections_per_ip: must be >= 0")
	}
	if c.HTTP.RateLimitPerMinute < 0 {
		return fmt.Errorf("invalid http.rate_limit_per_minute: must be >= 0")
	}
	if c.HTTP.WSIdleTimeout == "" {
		c.HTTP.WSIdleTimeout = defaultWSIdleTimeout.String()
	}
	if c.HTTP.WSPingInterval == "" {
		c.HTTP.WSPingInterval = defaultWSPingInterval.String()
	}
	if c.HTTP.MaxWSMessageBytes <= 0 {
		c.HTTP.MaxWSMessageBytes = defaultMaxWSMessageBytes
	}
	if c.Control.ListenAddr == "" {
		c.Control.ListenAddr = defaultControlPlaneAddr
	}
	if c.Plugins.Dir == "" {
		c.Plugins.Dir = defaultPluginDir
	}
	if len(c.Plugins.Enabled) == 0 {
		return fmt.Errorf("plugins.enabled must include at least one plugin")
	}
	if err := c.Logging.Normalize(); err != nil {
		return err
	}
	if err := c.UDPRecord.Normalize("logs/udp-record-server.jsonl"); err != nil {
		return err
	}

	if _, err := parseDurationOrDefault(c.TCP.DialTimeout, defaultDialTimeout); err != nil {
		return fmt.Errorf("invalid tcp.dial_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.TCP.KeepAlivePeriod, defaultKeepAlive); err != nil {
		return fmt.Errorf("invalid tcp.keep_alive_period: %w", err)
	}
	if _, err := parseDurationOrDefault(c.HTTP.ReadTimeout, defaultHTTPReadTimeout); err != nil {
		return fmt.Errorf("invalid http.read_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.HTTP.WriteTimeout, defaultHTTPWriteTimeout); err != nil {
		return fmt.Errorf("invalid http.write_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.HTTP.HandshakeTimeout, defaultHTTPHandshakeTimeout); err != nil {
		return fmt.Errorf("invalid http.handshake_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.HTTP.WSIdleTimeout, defaultWSIdleTimeout); err != nil {
		return fmt.Errorf("invalid http.ws_idle_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.HTTP.WSPingInterval, defaultWSPingInterval); err != nil {
		return fmt.Errorf("invalid http.ws_ping_interval: %w", err)
	}
	if _, err := parseDurationOrDefault(c.HTTP.ReadHeaderTimeout, defaultHTTPHandshakeTimeout); err != nil {
		return fmt.Errorf("invalid http.read_header_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.Control.ReadTimeout, defaultHTTPReadTimeout); err != nil {
		return fmt.Errorf("invalid control.read_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.Control.WriteTimeout, defaultHTTPWriteTimeout); err != nil {
		return fmt.Errorf("invalid control.write_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.Control.ReadHeaderTimeout, defaultHTTPHandshakeTimeout); err != nil {
		return fmt.Errorf("invalid control.read_header_timeout: %w", err)
	}
	if c.HTTP.MaxHeaderBytes <= 0 {
		c.HTTP.MaxHeaderBytes = 1 << 20
	}
	if c.Control.MaxHeaderBytes <= 0 {
		c.Control.MaxHeaderBytes = 1 << 20
	}

	return nil
}

func (r *UDPRecordConfig) Normalize(defaultPath string) error {
	if r.Path == "" {
		r.Path = defaultPath
	}
	if r.Duration == "" {
		r.Duration = defaultUDPRecordDuration.String()
	}
	if r.MaxEvents == 0 {
		r.MaxEvents = defaultUDPRecordMaxEvents
	}
	if r.MaxEvents < 0 {
		return fmt.Errorf("invalid udp_record.max_events: must be >= 0")
	}
	duration, err := parseDurationOrDefault(r.Duration, defaultUDPRecordDuration)
	if err != nil {
		return fmt.Errorf("invalid udp_record.duration: %w", err)
	}
	if duration < 0 {
		return fmt.Errorf("invalid udp_record.duration: must be >= 0")
	}
	return nil
}

func (r UDPRecordConfig) DurationValue() time.Duration {
	d, _ := parseDurationOrDefault(r.Duration, defaultUDPRecordDuration)
	if d < 0 {
		return 0
	}
	return d
}

func (l *LoggingConfig) Normalize() error {
	switch strings.ToLower(strings.TrimSpace(l.Format)) {
	case "", "text":
		l.Format = "text"
	case "json":
		l.Format = "json"
	default:
		return fmt.Errorf("invalid logging.format: must be text or json")
	}
	return nil
}

func (c *Config) DialTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.TCP.DialTimeout, defaultDialTimeout)
	return d
}

func (c *Config) KeepAliveDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.TCP.KeepAlivePeriod, defaultKeepAlive)
	return d
}

func (c *Config) HTTPReadTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.HTTP.ReadTimeout, defaultHTTPReadTimeout)
	return d
}

func (c *Config) HTTPWriteTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.HTTP.WriteTimeout, defaultHTTPWriteTimeout)
	return d
}

func (c *Config) HTTPHandshakeTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.HTTP.HandshakeTimeout, defaultHTTPHandshakeTimeout)
	return d
}

func (c *Config) HTTPWSIdleTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.HTTP.WSIdleTimeout, defaultWSIdleTimeout)
	return d
}

func (c *Config) HTTPWSPingIntervalDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.HTTP.WSPingInterval, defaultWSPingInterval)
	return d
}

func (c *Config) HTTPReadHeaderTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.HTTP.ReadHeaderTimeout, defaultHTTPHandshakeTimeout)
	return d
}

func (c *Config) ControlReadTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.Control.ReadTimeout, defaultHTTPReadTimeout)
	return d
}

func (c *Config) ControlWriteTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.Control.WriteTimeout, defaultHTTPWriteTimeout)
	return d
}

func (c *Config) ControlReadHeaderTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.Control.ReadHeaderTimeout, defaultHTTPHandshakeTimeout)
	return d
}

func (c *Config) WSAuthEnabled() bool {
	return len(c.WSAuthTokens()) > 0
}

func (c *Config) WSAuthTokens() []string {
	if c == nil {
		return nil
	}
	return normalizeSecretList(c.HTTP.AuthToken, c.HTTP.AuthTokens)
}

func (c *Config) WSAuthHeader() string {
	if c == nil || len(c.WSAuthTokens()) == 0 {
		return ""
	}
	header := normalizeAuthHeaderName(c.HTTP.AuthHeader)
	if header == "" {
		return DefaultAuthHeaderName
	}
	return header
}

func parseDurationOrDefault(raw string, fallback time.Duration) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	return d, nil
}

func ResolveSecret(raw string) string {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(value), "env:") {
		name := strings.TrimSpace(value[len("env:"):])
		if name == "" {
			return ""
		}
		return strings.TrimSpace(os.Getenv(name))
	}
	return value
}

func AuthHeaderValue(header, token string) string {
	token = strings.TrimSpace(token)
	if strings.EqualFold(strings.TrimSpace(header), "Authorization") {
		return "Bearer " + token
	}
	return token
}

func normalizeSecretList(primary string, extra []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(raw string) {
		value := ResolveSecret(raw)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	add(primary)
	for _, token := range extra {
		add(token)
	}
	return out
}

func normalizeStringList(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeAuthHeaderName(raw string) string {
	header := strings.TrimSpace(raw)
	if header == "" {
		return ""
	}
	return httpCanonicalHeaderKey(header)
}

func httpCanonicalHeaderKey(header string) string {
	parts := strings.Split(header, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		part = strings.ToLower(part)
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "-")
}

func validHTTPHeaderName(header string) bool {
	if header == "" {
		return false
	}
	for _, r := range header {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune("!#$%&'*+-.^_`|~", r):
		default:
			return false
		}
	}
	return true
}
