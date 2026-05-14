package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultClientListenAddr     = "127.0.0.1:25568"
	defaultClientListenNetwork  = "tcp"
	defaultClientServerWSURL    = "ws://127.0.0.1:8080/ws"
	defaultClientPluginName     = "minecraft"
	defaultConnectRetries       = 3
	defaultRetryInitialDelay    = 300 * time.Millisecond
	defaultRetryMaxDelay        = 3 * time.Second
	defaultClientWSIdleTimeout  = 2 * time.Minute
	defaultClientWSPingEvery    = 30 * time.Second
	defaultClientUDPSessionIdle = 45 * time.Second
	defaultClientUDPMetricsLog  = 10 * time.Second
	defaultClientMaxWSBytes     = 1 << 20
	defaultClientHandshakeTime  = 5 * time.Second
	defaultClientReadWriteTime  = 30 * time.Second
)

type ClientConfig struct {
	ListenAddr        string            `json:"listen_addr"`
	ListenNetwork     string            `json:"listen_network"`
	ServerWSURL       string            `json:"server_ws_url"`
	PluginName        string            `json:"plugin_name"`
	AuthToken         string            `json:"auth_token"`
	AuthHeader        string            `json:"auth_header"`
	ConnectRetries    int               `json:"connect_retries"`
	RetryInitialDelay string            `json:"retry_initial_delay"`
	RetryMaxDelay     string            `json:"retry_max_delay"`
	WSIdleTimeout     string            `json:"ws_idle_timeout"`
	WSPingInterval    string            `json:"ws_ping_interval"`
	UDPSessionIdle    string            `json:"udp_session_idle_timeout"`
	UDPMetricsLog     string            `json:"udp_metrics_log_interval"`
	MaxWSMessageBytes int               `json:"max_ws_message_bytes"`
	BufferSize        int               `json:"buffer_size"`
	HandshakeTimeout  string            `json:"handshake_timeout"`
	ReadTimeout       string            `json:"read_timeout"`
	WriteTimeout      string            `json:"write_timeout"`
	EnableCompression bool              `json:"enable_compression"`
	Headers           map[string]string `json:"headers"`
	Logging           LoggingConfig     `json:"logging"`
	UDPRecord         UDPRecordConfig   `json:"udp_record"`
}

type ClientNormalizeOptions struct {
	ValidateServerWSURL bool
}

func DefaultClient() *ClientConfig {
	return &ClientConfig{
		ListenAddr:        defaultClientListenAddr,
		ListenNetwork:     defaultClientListenNetwork,
		ServerWSURL:       defaultClientServerWSURL,
		PluginName:        defaultClientPluginName,
		AuthToken:         "",
		AuthHeader:        "",
		ConnectRetries:    defaultConnectRetries,
		RetryInitialDelay: defaultRetryInitialDelay.String(),
		RetryMaxDelay:     defaultRetryMaxDelay.String(),
		WSIdleTimeout:     defaultClientWSIdleTimeout.String(),
		WSPingInterval:    defaultClientWSPingEvery.String(),
		UDPSessionIdle:    defaultClientUDPSessionIdle.String(),
		UDPMetricsLog:     defaultClientUDPMetricsLog.String(),
		MaxWSMessageBytes: defaultClientMaxWSBytes,
		BufferSize:        defaultBufferSize,
		HandshakeTimeout:  defaultClientHandshakeTime.String(),
		ReadTimeout:       defaultClientReadWriteTime.String(),
		WriteTimeout:      defaultClientReadWriteTime.String(),
		EnableCompression: false,
		Headers:           map[string]string{},
		Logging:           LoggingConfig{Format: "text"},
		UDPRecord: UDPRecordConfig{
			Enabled:   false,
			Path:      "logs/udp-record-client.jsonl",
			Duration:  defaultUDPRecordDuration.String(),
			MaxEvents: defaultUDPRecordMaxEvents,
		},
	}
}

func LoadClient(path string) (*ClientConfig, error) {
	return LoadClientWithOptions(path, ClientNormalizeOptions{ValidateServerWSURL: true})
}

func LoadClientWithOptions(path string, options ClientNormalizeOptions) (*ClientConfig, error) {
	cfg := DefaultClient()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read client config %q: %w", path, err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse client config %q: %w", path, err)
	}

	if err := cfg.NormalizeWithOptions(options); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *ClientConfig) Normalize() error {
	return c.NormalizeWithOptions(ClientNormalizeOptions{ValidateServerWSURL: true})
}

func (c *ClientConfig) NormalizeWithOptions(options ClientNormalizeOptions) error {
	if c.ListenAddr == "" {
		c.ListenAddr = defaultClientListenAddr
	}
	if c.ListenNetwork == "" {
		c.ListenNetwork = defaultClientListenNetwork
	}
	c.ListenNetwork = strings.ToLower(strings.TrimSpace(c.ListenNetwork))
	if c.ServerWSURL == "" {
		c.ServerWSURL = defaultClientServerWSURL
	}
	if options.ValidateServerWSURL {
		if err := ValidateServerWSURL(c.ServerWSURL); err != nil {
			return fmt.Errorf("invalid client.server_ws_url: %w", err)
		}
	}
	if c.PluginName == "" {
		c.PluginName = defaultClientPluginName
	}
	c.AuthToken = ResolveSecret(c.AuthToken)
	if c.AuthToken != "" {
		c.AuthHeader = normalizeAuthHeaderName(c.AuthHeader)
		if c.AuthHeader == "" {
			c.AuthHeader = DefaultAuthHeaderName
		}
		if !validHTTPHeaderName(c.AuthHeader) {
			return fmt.Errorf("invalid client.auth_header: %q", c.AuthHeader)
		}
	} else {
		c.AuthHeader = strings.TrimSpace(c.AuthHeader)
	}
	if c.ConnectRetries < 0 {
		c.ConnectRetries = 0
	}
	if c.RetryInitialDelay == "" {
		c.RetryInitialDelay = defaultRetryInitialDelay.String()
	}
	if c.RetryMaxDelay == "" {
		c.RetryMaxDelay = defaultRetryMaxDelay.String()
	}
	if c.WSIdleTimeout == "" {
		c.WSIdleTimeout = defaultClientWSIdleTimeout.String()
	}
	if c.WSPingInterval == "" {
		c.WSPingInterval = defaultClientWSPingEvery.String()
	}
	if c.UDPSessionIdle == "" {
		c.UDPSessionIdle = defaultClientUDPSessionIdle.String()
	}
	if c.UDPMetricsLog == "" {
		c.UDPMetricsLog = defaultClientUDPMetricsLog.String()
	}
	if c.MaxWSMessageBytes <= 0 {
		c.MaxWSMessageBytes = defaultClientMaxWSBytes
	}
	if c.BufferSize < minimumSafeBufferSize {
		c.BufferSize = defaultBufferSize
	}
	if c.Headers == nil {
		c.Headers = map[string]string{}
	}
	if err := c.Logging.Normalize(); err != nil {
		return err
	}
	if err := c.UDPRecord.Normalize("logs/udp-record-client.jsonl"); err != nil {
		return err
	}

	switch c.ListenNetwork {
	case "tcp", "udp":
	default:
		return fmt.Errorf("invalid client.listen_network: must be tcp or udp")
	}

	if _, err := parseDurationOrDefault(c.HandshakeTimeout, defaultClientHandshakeTime); err != nil {
		return fmt.Errorf("invalid client.handshake_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.RetryInitialDelay, defaultRetryInitialDelay); err != nil {
		return fmt.Errorf("invalid client.retry_initial_delay: %w", err)
	}
	if _, err := parseDurationOrDefault(c.RetryMaxDelay, defaultRetryMaxDelay); err != nil {
		return fmt.Errorf("invalid client.retry_max_delay: %w", err)
	}
	if _, err := parseDurationOrDefault(c.WSIdleTimeout, defaultClientWSIdleTimeout); err != nil {
		return fmt.Errorf("invalid client.ws_idle_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.WSPingInterval, defaultClientWSPingEvery); err != nil {
		return fmt.Errorf("invalid client.ws_ping_interval: %w", err)
	}
	if _, err := parseDurationOrDefault(c.UDPSessionIdle, defaultClientUDPSessionIdle); err != nil {
		return fmt.Errorf("invalid client.udp_session_idle_timeout: %w", err)
	}
	udpMetricsLogDuration, err := parseDurationOrDefault(c.UDPMetricsLog, defaultClientUDPMetricsLog)
	if err != nil {
		return fmt.Errorf("invalid client.udp_metrics_log_interval: %w", err)
	}
	if udpMetricsLogDuration < 0 {
		return fmt.Errorf("invalid client.udp_metrics_log_interval: must be >= 0")
	}
	if _, err := parseDurationOrDefault(c.ReadTimeout, defaultClientReadWriteTime); err != nil {
		return fmt.Errorf("invalid client.read_timeout: %w", err)
	}
	if _, err := parseDurationOrDefault(c.WriteTimeout, defaultClientReadWriteTime); err != nil {
		return fmt.Errorf("invalid client.write_timeout: %w", err)
	}

	return nil
}

func (c *ClientConfig) HandshakeTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.HandshakeTimeout, defaultClientHandshakeTime)
	return d
}

func (c *ClientConfig) RetryInitialDelayDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.RetryInitialDelay, defaultRetryInitialDelay)
	return d
}

func (c *ClientConfig) RetryMaxDelayDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.RetryMaxDelay, defaultRetryMaxDelay)
	return d
}

func (c *ClientConfig) WSIdleTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.WSIdleTimeout, defaultClientWSIdleTimeout)
	return d
}

func (c *ClientConfig) WSPingIntervalDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.WSPingInterval, defaultClientWSPingEvery)
	return d
}

func (c *ClientConfig) UDPSessionIdleDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.UDPSessionIdle, defaultClientUDPSessionIdle)
	return d
}

func (c *ClientConfig) UDPMetricsLogIntervalDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.UDPMetricsLog, defaultClientUDPMetricsLog)
	if d < 0 {
		return 0
	}
	return d
}

func (c *ClientConfig) ReadTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.ReadTimeout, defaultClientReadWriteTime)
	return d
}

func (c *ClientConfig) WriteTimeoutDuration() time.Duration {
	d, _ := parseDurationOrDefault(c.WriteTimeout, defaultClientReadWriteTime)
	return d
}

func ValidateServerWSURL(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fmt.Errorf("server_ws_url cannot be empty")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("parse server_ws_url: %w", err)
	}
	switch parsed.Scheme {
	case "ws", "wss":
	default:
		return fmt.Errorf("server_ws_url scheme must be ws or wss")
	}
	if parsed.Host == "" {
		return fmt.Errorf("server_ws_url host is required")
	}
	return nil
}
