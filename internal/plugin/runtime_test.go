package plugin

import (
	"bytes"
	"testing"
)

func TestRuntimePluginXORRoundTrip(t *testing.T) {
	s := &Schema{
		Name:    "minecraft",
		Version: "3.0.0",
		Target: Target{
			Address: "127.0.0.1:25565",
		},
		Runtime: RuntimeConfig{
			Type: RuntimeTypeJSON,
			Mode: RuntimeModePipeline,
			ClientPipeline: []PipelineStep{
				{Op: "xor", Key: "k"},
			},
			ServerPipeline: []PipelineStep{
				{Op: "xor", Key: "k"},
			},
		},
	}
	s.Normalize()

	r := NewRuntimePlugin(s)
	plain := []byte("hello-world")

	encoded, err := r.ProcessClientData(plain)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := r.ProcessServerData(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bytes.Equal(plain, decoded) {
		t.Fatalf("round trip mismatch: got %q want %q", string(decoded), string(plain))
	}
}

func TestRuntimePluginPassthrough(t *testing.T) {
	s := &Schema{
		Name:    "minecraft",
		Version: "3.0.0",
		Target: Target{
			Address: "127.0.0.1:25565",
		},
		Runtime: RuntimeConfig{
			Type: RuntimeTypeJSON,
			Mode: RuntimeModePassthrough,
		},
	}
	s.Normalize()

	r := NewRuntimePlugin(s)
	if !r.Passthrough() {
		t.Fatal("expected passthrough mode")
	}

	info := r.Info()
	if info.Name != "minecraft" {
		t.Fatalf("unexpected plugin info name: %s", info.Name)
	}
	if !info.Passthrough {
		t.Fatal("info should report passthrough=true")
	}
}

func TestRuntimePluginUDPPeerBroadcast(t *testing.T) {
	p := NewRuntimePlugin(&Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "gzdoom-peer",
		Version:       "1.0.0",
		Target: Target{
			Network: "udp",
			Address: "127.0.0.1:5029",
		},
		Runtime: RuntimeConfig{
			Type:             RuntimeTypeJSON,
			Mode:             RuntimeModePassthrough,
			UDPPeerBroadcast: true,
		},
	})

	if !p.UDPPeerBroadcast() {
		t.Fatal("expected udp peer broadcast to be enabled for udp plugin")
	}

	info := p.Info()
	if !info.UDPPeerBroadcast {
		t.Fatal("expected plugin info to expose udp_peer_broadcast=true")
	}
}

func TestRuntimePluginUDPReplyPolicy(t *testing.T) {
	p := NewRuntimePlugin(&Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "lethal",
		Version:       "1.0.0",
		Target: Target{
			Network: "udp",
			Address: "127.0.0.1:7777",
		},
		Runtime: RuntimeConfig{
			Type:           RuntimeTypeJSON,
			Mode:           RuntimeModePassthrough,
			UDPReplyPolicy: UDPReplyPolicySameIP,
		},
	})

	if got := p.UDPReplyPolicy(); got != UDPReplyPolicySameIP {
		t.Fatalf("udp reply policy = %q, want %q", got, UDPReplyPolicySameIP)
	}
	if got := p.Info().UDPReplyPolicy; got != UDPReplyPolicySameIP {
		t.Fatalf("info udp reply policy = %q, want %q", got, UDPReplyPolicySameIP)
	}
}
