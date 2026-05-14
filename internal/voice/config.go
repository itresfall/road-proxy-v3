package voice

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const (
	defaultListenAddr        = "127.0.0.1:8090"
	defaultWSEndpoint        = "/ws"
	defaultRoomName          = "default"
	defaultMaxClients        = 8
	defaultMaxAudioFrameSize = 64 * 1024
	defaultReadTimeout       = 30 * time.Second
	defaultWriteTimeout      = 10 * time.Second
	defaultPingInterval      = 20 * time.Second
	defaultClientIdleTimeout = 60 * time.Second
)

type Config struct {
	ListenAddr        string `json:"listen_addr"`
	WSEndpoint        string `json:"ws_endpoint"`
	RoomName          string `json:"room_name"`
	MaxClients        int    `json:"max_clients"`
	MaxAudioFrameSize int    `json:"max_audio_frame_size"`
	ReadTimeout       string `json:"read_timeout"`
	WriteTimeout      string `json:"write_timeout"`
	PingInterval      string `json:"ping_interval"`
	ClientIdleTimeout string `json:"client_idle_timeout"`
}

func DefaultConfig() *Config {
	return &Config{
		ListenAddr:        defaultListenAddr,
		WSEndpoint:        defaultWSEndpoint,
		RoomName:          defaultRoomName,
		MaxClients:        defaultMaxClients,
		MaxAudioFrameSize: defaultMaxAudioFrameSize,
		ReadTimeout:       defaultReadTimeout.String(),
		WriteTimeout:      defaultWriteTimeout.String(),
		PingInterval:      defaultPingInterval.String(),
		ClientIdleTimeout: defaultClientIdleTimeout.String(),
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read voice config %q: %w", path, err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse voice config %q: %w", path, err)
	}

	if err := cfg.Normalize(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Normalize() error {
	if c.ListenAddr == "" {
		c.ListenAddr = defaultListenAddr
	}
	if c.WSEndpoint == "" {
		c.WSEndpoint = defaultWSEndpoint
	}
	if c.RoomName == "" {
		c.RoomName = defaultRoomName
	}
	if c.MaxClients <= 0 {
		c.MaxClients = defaultMaxClients
	}
	if c.MaxAudioFrameSize <= 0 {
		c.MaxAudioFrameSize = defaultMaxAudioFrameSize
	}
	if c.ReadTimeout == "" {
		c.ReadTimeout = defaultReadTimeout.String()
	}
	if c.WriteTimeout == "" {
		c.WriteTimeout = defaultWriteTimeout.String()
	}
	if c.PingInterval == "" {
		c.PingInterval = defaultPingInterval.String()
	}
	if c.ClientIdleTimeout == "" {
		c.ClientIdleTimeout = defaultClientIdleTimeout.String()
	}
	if _, err := c.ReadTimeoutDuration(); err != nil {
		return fmt.Errorf("invalid read_timeout: %w", err)
	}
	if _, err := c.WriteTimeoutDuration(); err != nil {
		return fmt.Errorf("invalid write_timeout: %w", err)
	}
	if _, err := c.PingIntervalDuration(); err != nil {
		return fmt.Errorf("invalid ping_interval: %w", err)
	}
	if _, err := c.ClientIdleTimeoutDuration(); err != nil {
		return fmt.Errorf("invalid client_idle_timeout: %w", err)
	}
	return nil
}

func (c *Config) ReadTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(c.ReadTimeout)
}

func (c *Config) WriteTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(c.WriteTimeout)
}

func (c *Config) PingIntervalDuration() (time.Duration, error) {
	return time.ParseDuration(c.PingInterval)
}

func (c *Config) ClientIdleTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(c.ClientIdleTimeout)
}
