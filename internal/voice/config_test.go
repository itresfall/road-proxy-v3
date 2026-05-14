package voice

import "testing"

func TestConfigNormalizeDefaults(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if cfg.ListenAddr != defaultListenAddr {
		t.Fatalf("ListenAddr = %q, want %q", cfg.ListenAddr, defaultListenAddr)
	}
	if cfg.WSEndpoint != defaultWSEndpoint {
		t.Fatalf("WSEndpoint = %q, want %q", cfg.WSEndpoint, defaultWSEndpoint)
	}
}

func TestConfigRejectsInvalidDuration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PingInterval = "soon"
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected invalid duration error")
	}
}
