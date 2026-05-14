package plugin

import (
	"fmt"
	"strings"
	"time"
)

const (
	RuntimeTypeBuiltin = "builtin"
	RuntimeTypeJSON    = "json"

	RuntimeModePassthrough = "passthrough"
	RuntimeModePipeline    = "pipeline"

	UDPReplyPolicyAny    = "any"
	UDPReplyPolicySameIP = "same_ip"
	UDPReplyPolicyStrict = "strict"

	SchemaVersionV1 = "v1"
)

type Schema struct {
	SchemaVersion string                `json:"schema_version"`
	Name          string                `json:"name"`
	Version       string                `json:"version"`
	Description   string                `json:"description"`
	Author        string                `json:"author"`
	Protocols     Protocols             `json:"protocols"`
	Target        Target                `json:"target"`
	Menu          Menu                  `json:"menu"`
	Capabilities  Capabilities          `json:"capabilities"`
	Compatibility CompatibilityMetadata `json:"compatibility"`
	Runtime       RuntimeConfig         `json:"runtime"`
}

type Protocols struct {
	Supported []string `json:"supported"`
}

type Target struct {
	Network string `json:"network"`
	Address string `json:"address"`
}

type Menu struct {
	ServerConfig string `json:"server_config"`
	ClientConfig string `json:"client_config"`
}

type CompatibilityMetadata struct {
	Status        string      `json:"status,omitempty"`
	TestedPlayers int         `json:"tested_players,omitempty"`
	KnownPorts    []KnownPort `json:"known_ports,omitempty"`
	LaunchArgs    []string    `json:"launch_args,omitempty"`
	Notes         []string    `json:"notes,omitempty"`
	LastVerified  string      `json:"last_verified,omitempty"`
}

type KnownPort struct {
	Network string `json:"network"`
	Port    int    `json:"port"`
	Role    string `json:"role,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

type RuntimeConfig struct {
	Type              string         `json:"type"`
	Mode              string         `json:"mode"`
	EnableObfuscation bool           `json:"enable_obfuscation"`
	UDPPeerBroadcast  bool           `json:"udp_peer_broadcast"`
	UDPReplyPolicy    string         `json:"udp_reply_policy"`
	ClientPipeline    []PipelineStep `json:"client_pipeline"`
	ServerPipeline    []PipelineStep `json:"server_pipeline"`
}

type Capabilities struct {
	SupportsReconnect bool `json:"supports_reconnect"`
	SupportsMultiplex bool `json:"supports_multiplex"`
}

type PipelineStep struct {
	Op  string `json:"op"`
	Key string `json:"key,omitempty"`
}

func (s *Schema) Normalize() {
	if s.Target.Network == "" {
		s.Target.Network = "tcp"
	}
	if s.Runtime.Type == "" {
		s.Runtime.Type = RuntimeTypeJSON
	}
	if s.Runtime.Mode == "" {
		s.Runtime.Mode = RuntimeModePassthrough
	}
	s.Runtime.UDPReplyPolicy = normalizeUDPReplyPolicy(s.Runtime.UDPReplyPolicy)
}

func (s *Schema) Validate() error {
	if s.SchemaVersion == "" {
		return fmt.Errorf("schema_version is required")
	}
	if s.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("schema_version must be %q", SchemaVersionV1)
	}
	if s.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if s.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if s.Target.Address == "" {
		return fmt.Errorf("target.address is required")
	}
	switch s.Target.Network {
	case "tcp", "udp":
	default:
		return fmt.Errorf("target.network must be %q or %q", "tcp", "udp")
	}

	switch s.Runtime.Type {
	case RuntimeTypeBuiltin, RuntimeTypeJSON:
	default:
		return fmt.Errorf("runtime.type must be %q or %q", RuntimeTypeBuiltin, RuntimeTypeJSON)
	}

	switch s.Runtime.Mode {
	case RuntimeModePassthrough, RuntimeModePipeline:
	default:
		return fmt.Errorf("runtime.mode must be %q or %q", RuntimeModePassthrough, RuntimeModePipeline)
	}
	switch s.Runtime.UDPReplyPolicy {
	case "", UDPReplyPolicyAny, UDPReplyPolicySameIP, UDPReplyPolicyStrict:
	default:
		return fmt.Errorf("runtime.udp_reply_policy must be %q, %q, or %q", UDPReplyPolicyAny, UDPReplyPolicySameIP, UDPReplyPolicyStrict)
	}

	if s.Runtime.Mode == RuntimeModePassthrough {
		if len(s.Runtime.ClientPipeline) > 0 || len(s.Runtime.ServerPipeline) > 0 {
			return fmt.Errorf("pipeline steps are not allowed when runtime.mode is passthrough")
		}
	}

	if err := validatePipeline(s.Runtime.ClientPipeline); err != nil {
		return fmt.Errorf("invalid client pipeline: %w", err)
	}
	if err := validatePipeline(s.Runtime.ServerPipeline); err != nil {
		return fmt.Errorf("invalid server pipeline: %w", err)
	}
	if err := validateCompatibility(s.Compatibility); err != nil {
		return fmt.Errorf("invalid compatibility metadata: %w", err)
	}

	return nil
}

func normalizeUDPReplyPolicy(policy string) string {
	policy = strings.ToLower(strings.TrimSpace(policy))
	switch policy {
	case "":
		return UDPReplyPolicyAny
	default:
		return policy
	}
}

func validateCompatibility(meta CompatibilityMetadata) error {
	switch meta.Status {
	case "", "unknown", "experimental", "partial", "working", "broken":
	default:
		return fmt.Errorf("status must be unknown, experimental, partial, working, or broken")
	}
	if meta.TestedPlayers < 0 {
		return fmt.Errorf("tested_players must be >= 0")
	}
	for i, knownPort := range meta.KnownPorts {
		if err := validateKnownPort(knownPort); err != nil {
			return fmt.Errorf("known_ports[%d]: %w", i, err)
		}
	}
	if meta.LastVerified != "" {
		if _, err := time.Parse("2006-01-02", meta.LastVerified); err != nil {
			return fmt.Errorf("last_verified must use YYYY-MM-DD")
		}
	}
	return nil
}

func validateKnownPort(knownPort KnownPort) error {
	switch knownPort.Network {
	case "tcp", "udp":
	default:
		return fmt.Errorf("network must be tcp or udp")
	}
	if knownPort.Port < 1 || knownPort.Port > 65535 {
		return fmt.Errorf("port must be in 1-65535")
	}
	return nil
}

func validatePipeline(steps []PipelineStep) error {
	for i := range steps {
		if err := validateStep(steps[i]); err != nil {
			return fmt.Errorf("step %d: %w", i, err)
		}
	}
	return nil
}

func validateStep(step PipelineStep) error {
	switch step.Op {
	case "noop":
		return nil
	case "xor":
		if step.Key == "" {
			return fmt.Errorf("xor step requires key")
		}
		return nil
	case "base64_encode":
		return nil
	case "base64_decode":
		return nil
	default:
		return fmt.Errorf("unsupported op %q", step.Op)
	}
}
