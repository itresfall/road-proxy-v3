package plugin

import "testing"

func TestSchemaValidatePasses(t *testing.T) {
	s := Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "minecraft",
		Version:       "3.0.0",
		Target: Target{
			Address: "127.0.0.1:25565",
		},
		Runtime: RuntimeConfig{
			Type: RuntimeTypeJSON,
			Mode: RuntimeModePassthrough,
		},
	}
	s.Normalize()

	if err := s.Validate(); err != nil {
		t.Fatalf("expected schema to validate, got error: %v", err)
	}
}

func TestSchemaValidateRejectsInvalidRuntimeType(t *testing.T) {
	s := Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "minecraft",
		Version:       "3.0.0",
		Target: Target{
			Address: "127.0.0.1:25565",
		},
		Runtime: RuntimeConfig{
			Type: "sharedlib",
			Mode: RuntimeModePassthrough,
		},
	}
	s.Normalize()
	s.Runtime.Type = "sharedlib"

	if err := s.Validate(); err == nil {
		t.Fatal("expected runtime.type validation error")
	}
}

func TestSchemaValidateRejectsXORWithoutKey(t *testing.T) {
	s := Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "minecraft",
		Version:       "3.0.0",
		Target: Target{
			Address: "127.0.0.1:25565",
		},
		Runtime: RuntimeConfig{
			Type: RuntimeTypeJSON,
			Mode: RuntimeModePipeline,
			ClientPipeline: []PipelineStep{
				{Op: "xor"},
			},
		},
	}
	s.Normalize()

	if err := s.Validate(); err == nil {
		t.Fatal("expected client pipeline validation error")
	}
}

func TestSchemaValidateRejectsInvalidTargetNetwork(t *testing.T) {
	s := Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "minecraft",
		Version:       "3.0.0",
		Target: Target{
			Network: "sctp",
			Address: "127.0.0.1:25565",
		},
		Runtime: RuntimeConfig{
			Type: RuntimeTypeJSON,
			Mode: RuntimeModePassthrough,
		},
	}
	s.Normalize()
	s.Target.Network = "sctp"

	if err := s.Validate(); err == nil {
		t.Fatal("expected target.network validation error")
	}
}

func TestSchemaValidateAcceptsNamedTargets(t *testing.T) {
	s := Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "son-of-the-forest-udp",
		Version:       "3.0.0",
		Target: Target{
			Network: "udp",
			Address: "127.0.0.1:8766",
		},
		Targets: []Target{
			{ID: "game", Network: "udp", Address: "127.0.0.1:8766"},
			{ID: "query", Network: "udp", Address: "127.0.0.1:27016"},
		},
		Runtime: RuntimeConfig{
			Type: RuntimeTypeJSON,
			Mode: RuntimeModePassthrough,
		},
	}
	s.Normalize()

	if err := s.Validate(); err != nil {
		t.Fatalf("expected named targets to validate, got error: %v", err)
	}
}

func TestSchemaValidateRejectsDuplicateNamedTargets(t *testing.T) {
	s := Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "game",
		Version:       "3.0.0",
		Target: Target{
			Network: "udp",
			Address: "127.0.0.1:7777",
		},
		Targets: []Target{
			{ID: "game", Network: "udp", Address: "127.0.0.1:7777"},
			{ID: "GAME", Network: "udp", Address: "127.0.0.1:7778"},
		},
		Runtime: RuntimeConfig{
			Type: RuntimeTypeJSON,
			Mode: RuntimeModePassthrough,
		},
	}
	s.Normalize()

	if err := s.Validate(); err == nil {
		t.Fatal("expected duplicate target id validation error")
	}
}

func TestRuntimePluginResolveTarget(t *testing.T) {
	s := Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "game",
		Version:       "3.0.0",
		Target: Target{
			Network: "udp",
			Address: "127.0.0.1:7777",
		},
		Targets: []Target{
			{ID: "query", Network: "udp", Address: "127.0.0.1:27016"},
		},
		Runtime: RuntimeConfig{
			Type: RuntimeTypeJSON,
			Mode: RuntimeModePassthrough,
		},
	}
	p := NewRuntimePlugin(&s)

	target, ok := p.ResolveTarget("query")
	if !ok {
		t.Fatal("expected query target to resolve")
	}
	if target.Address != "127.0.0.1:27016" {
		t.Fatalf("unexpected target address: %s", target.Address)
	}
	if _, ok := p.ResolveTarget("missing"); ok {
		t.Fatal("missing target should not resolve")
	}
}

func TestSchemaValidateRejectsInvalidUDPReplyPolicy(t *testing.T) {
	s := Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "game",
		Version:       "3.0.0",
		Target: Target{
			Network: "udp",
			Address: "127.0.0.1:7777",
		},
		Runtime: RuntimeConfig{
			Type:           RuntimeTypeJSON,
			Mode:           RuntimeModePassthrough,
			UDPReplyPolicy: "same_port",
		},
	}
	s.Normalize()

	if err := s.Validate(); err == nil {
		t.Fatal("expected runtime.udp_reply_policy validation error")
	}
}

func TestSchemaValidateRejectsMissingSchemaVersion(t *testing.T) {
	s := Schema{
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

	if err := s.Validate(); err == nil {
		t.Fatal("expected schema_version validation error")
	}
}

func TestSchemaValidateRejectsUnknownSchemaVersion(t *testing.T) {
	s := Schema{
		SchemaVersion: "v2",
		Name:          "minecraft",
		Version:       "3.0.0",
		Target: Target{
			Address: "127.0.0.1:25565",
		},
		Runtime: RuntimeConfig{
			Type: RuntimeTypeJSON,
			Mode: RuntimeModePassthrough,
		},
	}
	s.Normalize()

	if err := s.Validate(); err == nil {
		t.Fatal("expected unknown schema_version validation error")
	}
}

func TestSchemaValidateAcceptsCompatibilityMetadata(t *testing.T) {
	s := Schema{
		SchemaVersion: SchemaVersionV1,
		Name:          "game",
		Version:       "3.0.0",
		Target: Target{
			Network: "udp",
			Address: "127.0.0.1:7777",
		},
		Compatibility: CompatibilityMetadata{
			Status:        "working",
			TestedPlayers: 3,
			KnownPorts: []KnownPort{
				{Network: "udp", Port: 7777, Role: "target"},
			},
			LaunchArgs:   []string{"-netmode 1"},
			Notes:        []string{"validated profile"},
			LastVerified: "2026-05-11",
		},
		Runtime: RuntimeConfig{
			Type: RuntimeTypeJSON,
			Mode: RuntimeModePassthrough,
		},
	}

	if err := s.Validate(); err != nil {
		t.Fatalf("expected compatibility metadata to validate, got error: %v", err)
	}
}

func TestSchemaValidateRejectsInvalidCompatibilityMetadata(t *testing.T) {
	tests := []struct {
		name string
		meta CompatibilityMetadata
	}{
		{
			name: "bad status",
			meta: CompatibilityMetadata{Status: "maybe"},
		},
		{
			name: "bad players",
			meta: CompatibilityMetadata{TestedPlayers: -1},
		},
		{
			name: "bad port",
			meta: CompatibilityMetadata{KnownPorts: []KnownPort{{Network: "udp", Port: 0}}},
		},
		{
			name: "bad date",
			meta: CompatibilityMetadata{LastVerified: "11-05-2026"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Schema{
				SchemaVersion: SchemaVersionV1,
				Name:          "game",
				Version:       "3.0.0",
				Target: Target{
					Address: "127.0.0.1:7777",
				},
				Compatibility: tc.meta,
				Runtime: RuntimeConfig{
					Type: RuntimeTypeJSON,
					Mode: RuntimeModePassthrough,
				},
			}
			s.Normalize()

			if err := s.Validate(); err == nil {
				t.Fatal("expected compatibility metadata validation error")
			}
		})
	}
}
