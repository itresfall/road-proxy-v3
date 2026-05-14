package plugin

import (
	"encoding/base64"
	"fmt"
)

type RuntimePlugin struct {
	schema *Schema
}

type Info struct {
	SchemaVersion      string       `json:"schema_version"`
	Name               string       `json:"name"`
	Version            string       `json:"version"`
	Description        string       `json:"description"`
	Author             string       `json:"author"`
	SupportedProtocols []string     `json:"supported_protocols"`
	TargetNetwork      string       `json:"target_network"`
	TargetAddress      string       `json:"target_address"`
	Capabilities       Capabilities `json:"capabilities"`
	RuntimeType        string       `json:"runtime_type"`
	RuntimeMode        string       `json:"runtime_mode"`
	Passthrough        bool         `json:"passthrough"`
	UDPPeerBroadcast   bool         `json:"udp_peer_broadcast"`
	UDPReplyPolicy     string       `json:"udp_reply_policy"`
}

func NewRuntimePlugin(schema *Schema) *RuntimePlugin {
	copySchema := *schema
	copySchema.Normalize()
	return &RuntimePlugin{schema: &copySchema}
}

func (p *RuntimePlugin) Name() string {
	return p.schema.Name
}

func (p *RuntimePlugin) Info() Info {
	return Info{
		SchemaVersion:      p.schema.SchemaVersion,
		Name:               p.schema.Name,
		Version:            p.schema.Version,
		Description:        p.schema.Description,
		Author:             p.schema.Author,
		SupportedProtocols: append([]string(nil), p.schema.Protocols.Supported...),
		TargetNetwork:      p.schema.Target.Network,
		TargetAddress:      p.schema.Target.Address,
		Capabilities:       p.schema.Capabilities,
		RuntimeType:        p.schema.Runtime.Type,
		RuntimeMode:        p.schema.Runtime.Mode,
		Passthrough:        p.Passthrough(),
		UDPPeerBroadcast:   p.UDPPeerBroadcast(),
		UDPReplyPolicy:     p.UDPReplyPolicy(),
	}
}

func (p *RuntimePlugin) TargetNetwork() string {
	return p.schema.Target.Network
}

func (p *RuntimePlugin) TargetAddress() string {
	return p.schema.Target.Address
}

func (p *RuntimePlugin) Passthrough() bool {
	return p.schema.Runtime.Mode == RuntimeModePassthrough &&
		!p.schema.Runtime.EnableObfuscation &&
		len(p.schema.Runtime.ClientPipeline) == 0 &&
		len(p.schema.Runtime.ServerPipeline) == 0
}

func (p *RuntimePlugin) UDPPeerBroadcast() bool {
	return p.schema.Target.Network == "udp" && p.schema.Runtime.UDPPeerBroadcast
}

func (p *RuntimePlugin) UDPReplyPolicy() string {
	if p.schema.Target.Network != "udp" {
		return UDPReplyPolicyAny
	}
	return normalizeUDPReplyPolicy(p.schema.Runtime.UDPReplyPolicy)
}

func (p *RuntimePlugin) ProcessClientData(in []byte) ([]byte, error) {
	return p.process(in, p.schema.Runtime.ClientPipeline)
}

func (p *RuntimePlugin) ProcessServerData(in []byte) ([]byte, error) {
	return p.process(in, p.schema.Runtime.ServerPipeline)
}

func (p *RuntimePlugin) process(in []byte, pipeline []PipelineStep) ([]byte, error) {
	out := append([]byte(nil), in...)
	steps := pipeline

	if p.schema.Runtime.EnableObfuscation && len(steps) == 0 {
		steps = []PipelineStep{
			{Op: "xor", Key: p.schema.Name},
		}
	}

	for _, step := range steps {
		var err error
		out, err = runStep(out, step)
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func runStep(in []byte, step PipelineStep) ([]byte, error) {
	switch step.Op {
	case "", "noop":
		return in, nil
	case "xor":
		key := []byte(step.Key)
		if len(key) == 0 {
			return nil, fmt.Errorf("xor key is empty")
		}
		out := make([]byte, len(in))
		for i := range in {
			out[i] = in[i] ^ key[i%len(key)]
		}
		return out, nil
	case "base64_encode":
		out := make([]byte, base64.StdEncoding.EncodedLen(len(in)))
		base64.StdEncoding.Encode(out, in)
		return out, nil
	case "base64_decode":
		out := make([]byte, base64.StdEncoding.DecodedLen(len(in)))
		n, err := base64.StdEncoding.Decode(out, in)
		if err != nil {
			return nil, fmt.Errorf("base64 decode failed: %w", err)
		}
		return out[:n], nil
	default:
		return nil, fmt.Errorf("unsupported pipeline op %q", step.Op)
	}
}
