package main

import (
	"strings"
	"testing"
)

func TestParseAddressPort(t *testing.T) {
	tests := []struct {
		in       string
		wantHost string
		wantPort int
	}{
		{in: "0.0.0.0:25565", wantHost: "0.0.0.0", wantPort: 25565},
		{in: "[::]:8080", wantHost: "::", wantPort: 8080},
		{in: "*:*", wantHost: "*", wantPort: 0},
		{in: "127.0.0.1:0", wantHost: "127.0.0.1", wantPort: 0},
	}

	for _, tc := range tests {
		host, port, err := parseAddressPort(tc.in)
		if err != nil {
			t.Fatalf("parseAddressPort(%q) returned err: %v", tc.in, err)
		}
		if host != tc.wantHost || port != tc.wantPort {
			t.Fatalf("parseAddressPort(%q) got (%q,%d), want (%q,%d)", tc.in, host, port, tc.wantHost, tc.wantPort)
		}
	}
}

func TestParseNetstatOutputTCP(t *testing.T) {
	raw := `
  TCP    0.0.0.0:25565         0.0.0.0:0              LISTENING       8078
  TCP    [::]:25565            [::]:0                 LISTENING       8078
`
	got, err := parseNetstatOutput("tcp", raw)
	if err != nil {
		t.Fatalf("parseNetstatOutput returned err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(got))
	}
	if got[0].Proto != "tcp" || got[0].LocalPort != 25565 || got[0].PID != 8078 {
		t.Fatalf("unexpected first endpoint: %+v", got[0])
	}
}

func TestParseWindowsPowerShellEndpointCSV(t *testing.T) {
	tcpRaw := `"LocalAddress","LocalPort","RemoteAddress","RemotePort","State","OwningProcess"
"0.0.0.0","25565","0.0.0.0","0","Listen","8078"
"192.168.1.5","53312","192.168.1.7","7777","Established","9001"
`
	tcp, err := parseWindowsPowerShellEndpointCSV("tcp", tcpRaw)
	if err != nil {
		t.Fatalf("parseWindowsPowerShellEndpointCSV tcp returned err: %v", err)
	}
	if len(tcp) != 2 {
		t.Fatalf("expected 2 tcp endpoints, got %d: %#v", len(tcp), tcp)
	}
	if tcp[0].Proto != "tcp" || tcp[0].LocalPort != 25565 || tcp[0].State != "LISTEN" || tcp[0].PID != 8078 {
		t.Fatalf("unexpected tcp endpoint: %#v", tcp[0])
	}
	if tcp[1].RemotePort != 7777 || tcp[1].State != "ESTABLISHED" {
		t.Fatalf("unexpected tcp flow endpoint: %#v", tcp[1])
	}

	udpRaw := `"LocalAddress","LocalPort","OwningProcess"
"127.0.0.1","5029","1234"
`
	udp, err := parseWindowsPowerShellEndpointCSV("udp", udpRaw)
	if err != nil {
		t.Fatalf("parseWindowsPowerShellEndpointCSV udp returned err: %v", err)
	}
	if len(udp) != 1 {
		t.Fatalf("expected 1 udp endpoint, got %d: %#v", len(udp), udp)
	}
	if udp[0].Proto != "udp" || udp[0].LocalPort != 5029 || udp[0].RemoteAddr != "*" || udp[0].PID != 1234 {
		t.Fatalf("unexpected udp endpoint: %#v", udp[0])
	}
}

func TestParseSSOutput(t *testing.T) {
	raw := `
udp   UNCONN 0 0 127.0.0.1:5029 0.0.0.0:* users:(("gzdoom",pid=1234,fd=13))
tcp   LISTEN 0 128 0.0.0.0:25565 0.0.0.0:* users:(("java",pid=2345,fd=42))
tcp   ESTAB 0 0 192.168.1.5:53312 192.168.1.7:7777 users:(("game",pid=3456,fd=11))
`
	got, err := parseSSOutput(raw)
	if err != nil {
		t.Fatalf("parseSSOutput returned err: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 endpoints, got %d: %#v", len(got), got)
	}
	if got[0].Proto != "udp" || got[0].LocalPort != 5029 || got[0].PID != 1234 {
		t.Fatalf("unexpected UDP endpoint: %#v", got[0])
	}
	if got[2].RemotePort != 7777 || got[2].State != "ESTAB" {
		t.Fatalf("unexpected established endpoint: %#v", got[2])
	}
}

func TestParseLsofOutput(t *testing.T) {
	raw := `COMMAND  PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
gzdoom  1234 user   13u  IPv4 123456      0t0  UDP 127.0.0.1:5029
java    2345 user   42u  IPv4 123457      0t0  TCP *:25565 (LISTEN)
game    3456 user   11u  IPv4 123458      0t0  TCP 192.168.1.5:53312->192.168.1.7:7777 (ESTABLISHED)
`
	got, err := parseLsofOutput(raw)
	if err != nil {
		t.Fatalf("parseLsofOutput returned err: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 endpoints, got %d: %#v", len(got), got)
	}
	if got[1].Proto != "tcp" || got[1].LocalPort != 25565 || got[1].State != "LISTEN" {
		t.Fatalf("unexpected listen endpoint: %#v", got[1])
	}
	if got[2].RemotePort != 7777 || got[2].PID != 3456 {
		t.Fatalf("unexpected flow endpoint: %#v", got[2])
	}
}

func TestParseProcNetOutput(t *testing.T) {
	raw := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:13A5 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 111111 1 0000000000000000 100 0 0 10 0
`
	got := parseProcNetOutput("udp", raw, map[string]int{"111111": 1234}, false)
	if len(got) != 1 {
		t.Fatalf("expected 1 endpoint, got %d: %#v", len(got), got)
	}
	if got[0].LocalAddr != "127.0.0.1" || got[0].LocalPort != 5029 || got[0].PID != 1234 {
		t.Fatalf("unexpected proc endpoint: %#v", got[0])
	}
	if got[0].State != "LISTEN" {
		t.Fatalf("unexpected proc state: %#v", got[0])
	}
}

func TestLinuxSocketHelpers(t *testing.T) {
	if got := parseSocketInode("socket:[12345]"); got != "12345" {
		t.Fatalf("parseSocketInode = %q", got)
	}
	if got := formatProcIPv4("0100007F"); got != "127.0.0.1" {
		t.Fatalf("formatProcIPv4 = %q", got)
	}
}

func TestRecommendNetworkAndPort(t *testing.T) {
	s := &captureSummary{
		LocalPortHits: map[string]map[int]int{
			"tcp": {25565: 2},
			"udp": {19132: 8},
		},
		RemotePortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {},
		},
	}
	net, port := recommendNetworkAndPort(s)
	if net != "udp" || port != 19132 {
		t.Fatalf("unexpected recommendation: %s %d", net, port)
	}
}

func TestRecommendPrefersNonEphemeralUDPOverEphemeralTCPNoise(t *testing.T) {
	s := &captureSummary{
		LocalPortHits: map[string]map[int]int{
			"tcp": {58158: 30},
			"udp": {5029: 3},
		},
		RemotePortHits: map[string]map[int]int{
			"tcp": {443: 30},
			"udp": {},
		},
	}

	net, port := recommendNetworkAndPort(s)
	if net != "udp" || port != 5029 {
		t.Fatalf("unexpected recommendation: %s %d", net, port)
	}
}

func TestRecommendFallsBackToEphemeralWhenOnlyCandidate(t *testing.T) {
	s := &captureSummary{
		LocalPortHits: map[string]map[int]int{
			"tcp": {58158: 5},
			"udp": {},
		},
		RemotePortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {},
		},
	}

	net, port := recommendNetworkAndPort(s)
	if net != "tcp" || port != 58158 {
		t.Fatalf("unexpected recommendation: %s %d", net, port)
	}
}

func TestRecommendPrefersRemoteGamePortOverLocalEphemeral(t *testing.T) {
	s := &captureSummary{
		LocalPortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {62000: 20},
		},
		RemotePortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {7777: 20},
		},
	}

	net, port := recommendNetworkAndPort(s)
	if net != "udp" || port != 7777 {
		t.Fatalf("unexpected recommendation: %s %d", net, port)
	}
}

func TestBuildPluginDocUDPDefaultsPeerBroadcastOff(t *testing.T) {
	doc := buildPluginDoc("lethal-company-udp", "udp", "127.0.0.1:7777", "Lethal Company.exe", false, nil, nil)

	runtimeDoc, ok := doc["runtime"].(map[string]any)
	if !ok {
		t.Fatalf("runtime doc missing or wrong type: %#v", doc["runtime"])
	}
	if got := runtimeDoc["udp_peer_broadcast"]; got != false {
		t.Fatalf("udp_peer_broadcast = %#v, want false", got)
	}
	if got := runtimeDoc["udp_reply_policy"]; got != "any" {
		t.Fatalf("udp_reply_policy = %#v, want any", got)
	}

	caps, ok := doc["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("capabilities doc missing or wrong type: %#v", doc["capabilities"])
	}
	if got := caps["supports_multiplex"]; got != true {
		t.Fatalf("supports_multiplex = %#v, want true", got)
	}
}

func TestBuildPluginDocUsesProfileUDPReplyPolicy(t *testing.T) {
	profile := &compatProfile{ID: "lethal-company", UDPReplyPolicy: "same_ip"}
	doc := buildPluginDoc("lethal-company-udp", "udp", "127.0.0.1:7777", "Lethal Company.exe", false, nil, profile)

	runtimeDoc, ok := doc["runtime"].(map[string]any)
	if !ok {
		t.Fatalf("runtime doc missing or wrong type: %#v", doc["runtime"])
	}
	if got := runtimeDoc["udp_reply_policy"]; got != "same_ip" {
		t.Fatalf("udp_reply_policy = %#v, want same_ip", got)
	}
}

func TestBuildPluginDocReferencesKnownProfileNotesSource(t *testing.T) {
	profile := &compatProfile{ID: "gzdoom"}
	doc := buildPluginDoc("gzdoom-udp", "udp", "127.0.0.1:5029", "gzdoom.exe", false, []string{"note"}, profile)

	compatibility, ok := doc["compatibility"].(map[string]any)
	if !ok {
		t.Fatalf("compatibility doc missing or wrong type: %#v", doc["compatibility"])
	}
	if got := compatibility["profile_id"]; got != "gzdoom" {
		t.Fatalf("profile_id = %#v, want gzdoom", got)
	}
	if _, ok := compatibility["notes"]; ok {
		t.Fatalf("known profile notes should not be copied into plugin compatibility doc: %#v", compatibility)
	}
	if got := compatibility["notes_source"]; got != "compat-profiles/gzdoom.json" {
		t.Fatalf("notes_source = %#v", got)
	}
}

func TestStudioCLIOptionsEnabled(t *testing.T) {
	if (studioCLIOptions{}).Enabled() {
		t.Fatal("empty options should not enable non-interactive mode")
	}
	if !(studioCLIOptions{Process: "gzdoom"}).Enabled() {
		t.Fatal("process option should enable non-interactive mode")
	}
	if !(studioCLIOptions{MultiPhase: true}).Enabled() {
		t.Fatal("multi-phase option should enable non-interactive mode")
	}
	var enabled optionalBool
	if err := enabled.Set("true"); err != nil {
		t.Fatalf("optional bool parse failed: %v", err)
	}
	if !(studioCLIOptions{UDPPeerBroadcast: enabled}).Enabled() {
		t.Fatal("udp peer broadcast override should enable non-interactive mode")
	}
}

func TestSelectCLIProcessCandidate(t *testing.T) {
	candidates := []processCandidate{
		{PID: 100, Name: "System.exe"},
		{PID: 200, Name: "GZDoom.exe"},
	}
	got, err := selectCLIProcessCandidate(candidates, studioCLIOptions{Process: "gzdoom"})
	if err != nil {
		t.Fatalf("select by process returned error: %v", err)
	}
	if got.PID != 200 {
		t.Fatalf("selected PID = %d, want 200", got.PID)
	}

	got, err = selectCLIProcessCandidate(candidates, studioCLIOptions{PID: 100})
	if err != nil {
		t.Fatalf("select by PID returned error: %v", err)
	}
	if got.Name != "System.exe" {
		t.Fatalf("selected process = %s", got.Name)
	}
}

func TestMatchCompatProfile(t *testing.T) {
	s := &captureSummary{
		LocalPortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {7777: 4},
		},
		RemotePortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {},
		},
	}

	profile := matchCompatProfile("Lethal Company.exe", s)
	if profile == nil || profile.ID != "lethal-company" {
		t.Fatalf("unexpected profile: %#v", profile)
	}

	match := matchCompatProfileWithReasons("Lethal Company.exe", s)
	if match.Confidence != "gold" {
		t.Fatalf("unexpected confidence: %#v", match)
	}
	if !containsText(match.Reasons, "exe_name_match:lethal") {
		t.Fatalf("expected exe match reason, got %#v", match.Reasons)
	}
	if !containsText(match.Reasons, "port_match:udp/7777") {
		t.Fatalf("expected port match reason, got %#v", match.Reasons)
	}
}

func TestMatchCompatProfileUsesSteamAppIDAsSignal(t *testing.T) {
	s := &captureSummary{
		SteamAppID:     1966720,
		RecommendedNet: "udp",
		LocalPortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {7777: 1},
		},
		RemotePortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {},
		},
	}

	match := matchCompatProfileWithReasons("Lethal Company.exe", s)
	if match.Profile == nil || match.Profile.ID != "lethal-company" {
		t.Fatalf("unexpected profile: %#v", match)
	}
	if match.Confidence != "platinum" {
		t.Fatalf("unexpected confidence: %#v", match)
	}
	if !containsText(match.Reasons, "steam_app_id_match:1966720") {
		t.Fatalf("expected Steam AppID reason, got %#v", match.Reasons)
	}
}

func TestEvaluateCompatProfileUsesExeSHA256AsSignal(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	profile := &compatProfile{
		ID:          "hash-game",
		DisplayName: "Hash Game",
		Network:     "udp",
		TargetPort:  7777,
		ExeHashes: map[string]string{
			"sha256": strings.ToUpper(hash),
		},
	}
	s := &captureSummary{ProcessSHA256: hash}

	match := evaluateCompatProfile(profile, "unknown.exe", s)
	if match.Profile == nil {
		t.Fatalf("expected hash-only match")
	}
	if !containsText(match.Reasons, "exe_hash_match:sha256") {
		t.Fatalf("expected hash match reason, got %#v", match.Reasons)
	}
}

func TestLoadCompatProfilesFromRepository(t *testing.T) {
	profiles, err := loadCompatProfiles()
	if err != nil {
		t.Fatalf("loadCompatProfiles returned error: %v", err)
	}
	if len(profiles) < 4 {
		t.Fatalf("expected repository compat profiles, got %d", len(profiles))
	}
	ids := map[string]bool{}
	for _, profile := range profiles {
		ids[profile.ID] = true
		if profile.ROADScope != "direct_lan_local_port_only" {
			t.Fatalf("profile %s has invalid ROAD scope %q", profile.ID, profile.ROADScope)
		}
		if profile.SteamLobbySupported {
			t.Fatalf("profile %s must not claim Steam lobby support", profile.ID)
		}
	}
	for _, id := range []string{"gzdoom", "lethal-company", "minecraft-bedrock", "minecraft-java"} {
		if !ids[id] {
			t.Fatalf("missing compat profile %q", id)
		}
	}
}

func TestProfileNotesForGZDoomAndLethal(t *testing.T) {
	gzdoomProfile := matchCompatProfile("gzdoom.exe", &captureSummary{
		LocalPortHits: map[string]map[int]int{"tcp": {}, "udp": {5029: 1}},
		RemotePortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {},
		},
	})
	gzdoom := profileNotes(gzdoomProfile)
	if len(gzdoom) == 0 {
		t.Fatal("expected GZDoom notes")
	}
	if !containsText(gzdoom, "-netmode 1") {
		t.Fatalf("expected GZDoom netmode note, got %#v", gzdoom)
	}

	lethalProfile := matchCompatProfile("Lethal Company.exe", &captureSummary{
		LocalPortHits: map[string]map[int]int{"tcp": {}, "udp": {7777: 1}},
		RemotePortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {},
		},
	})
	lethal := profileNotes(lethalProfile)
	if len(lethal) == 0 {
		t.Fatal("expected Lethal Company notes")
	}
	if !containsText(lethal, "127.0.0.1:7777") {
		t.Fatalf("expected Lethal port note, got %#v", lethal)
	}
}

func TestFallbackProfileNotesOnlyForUnknownUDP(t *testing.T) {
	if got := fallbackProfileNotes(&compatProfile{ID: "known"}, "udp"); len(got) != 0 {
		t.Fatalf("known profile should not get fallback notes, got %#v", got)
	}
	if got := fallbackProfileNotes(nil, "udp"); !containsText(got, "UDP") {
		t.Fatalf("unknown UDP should get generic fallback note, got %#v", got)
	}
}

func TestBuildPortSelectionReport(t *testing.T) {
	s := &captureSummary{
		TopLocalPorts: map[string][]pair{
			"tcp": {{Port: 443, Hits: 10}},
			"udp": {{Port: 7777, Hits: 8}, {Port: 62000, Hits: 6}},
		},
		TopRemotePorts: map[string][]pair{
			"tcp": {},
			"udp": {{Port: 7777, Hits: 8}},
		},
	}
	report := buildPortSelectionReport(s, "udp", 7777, compatMatch{})
	if report == nil {
		t.Fatal("expected port selection report")
	}
	if report.SelectedNetwork != "udp" || report.SelectedPort != 7777 {
		t.Fatalf("unexpected selection: %#v", report)
	}
	if !containsRejectedPort(report.Rejected, 443, "likely_noise_port") {
		t.Fatalf("expected rejected noise port, got %#v", report.Rejected)
	}
	if !containsRejectedPort(report.Rejected, 62000, "ephemeral_port") {
		t.Fatalf("expected rejected ephemeral port, got %#v", report.Rejected)
	}
}

func TestAggregatePhaseSummariesAndClassifyRoles(t *testing.T) {
	phases := []capturePhaseSummary{
		{
			Name:           "lobby",
			CaptureSeconds: 5,
			Ticks:          1,
			LocalPortHits:  map[string]map[int]int{"tcp": {}, "udp": {7777: 1}},
			RemotePortHits: map[string]map[int]int{"tcp": {}, "udp": {}},
		},
		{
			Name:           "connect",
			CaptureSeconds: 5,
			Ticks:          1,
			LocalPortHits:  map[string]map[int]int{"tcp": {}, "udp": {7777: 1, 60000: 1}},
			RemotePortHits: map[string]map[int]int{"tcp": {}, "udp": {}},
		},
		{
			Name:           "ingame",
			CaptureSeconds: 5,
			Ticks:          1,
			LocalPortHits:  map[string]map[int]int{"tcp": {}, "udp": {7777: 1, 5029: 1}},
			RemotePortHits: map[string]map[int]int{"tcp": {}, "udp": {}},
		},
		{
			Name:           "disconnect",
			CaptureSeconds: 5,
			Ticks:          1,
			LocalPortHits:  map[string]map[int]int{"tcp": {}, "udp": {7777: 1}},
			RemotePortHits: map[string]map[int]int{"tcp": {}, "udp": {}},
		},
	}

	summary := aggregatePhaseSummaries(42, "game.exe", phases)
	if summary.CaptureSeconds != 20 || summary.Ticks != 4 {
		t.Fatalf("unexpected aggregate summary: %#v", summary)
	}
	if summary.LocalPortHits["udp"][7777] != 4 {
		t.Fatalf("expected aggregate hits for 7777, got %#v", summary.LocalPortHits["udp"])
	}

	roles := classifyPhasePortRoles(phases)
	if !containsPhasePortRole(roles, "udp", 7777, "persistent") {
		t.Fatalf("expected persistent 7777 role, got %#v", roles)
	}
	if !containsPhasePortRole(roles, "udp", 60000, "connect_only") {
		t.Fatalf("expected connect_only 60000 role, got %#v", roles)
	}
	if !containsPhasePortRole(roles, "udp", 5029, "game_only") {
		t.Fatalf("expected game_only 5029 role, got %#v", roles)
	}
}

func TestPacketFingerprintBuilder(t *testing.T) {
	builder := newPacketFingerprintBuilder()
	builder.observe(1, endpoint{
		Proto:      "tcp",
		LocalAddr:  "127.0.0.1",
		LocalPort:  62000,
		RemoteAddr: "127.0.0.1",
		RemotePort: 7777,
		State:      "SYN_SENT",
		PID:        42,
	})
	builder.observe(2, endpoint{
		Proto:      "tcp",
		LocalAddr:  "127.0.0.1",
		LocalPort:  62000,
		RemoteAddr: "127.0.0.1",
		RemotePort: 7777,
		State:      "ESTABLISHED",
		PID:        42,
	})
	builder.observe(4, endpoint{
		Proto:      "tcp",
		LocalAddr:  "127.0.0.1",
		LocalPort:  62000,
		RemoteAddr: "127.0.0.1",
		RemotePort: 7777,
		State:      "ESTABLISHED",
		PID:        42,
	})
	builder.observe(2, endpoint{
		Proto:      "udp",
		LocalAddr:  "127.0.0.1",
		LocalPort:  5029,
		RemoteAddr: "*",
		RemotePort: 0,
		PID:        42,
	})

	report := builder.report(4)
	if report == nil {
		t.Fatal("expected fingerprint report")
	}
	if report.PacketSizeObserved {
		t.Fatal("socket snapshot fingerprint must not claim packet size observation")
	}
	if len(report.TopFlows) != 2 {
		t.Fatalf("expected 2 flows, got %#v", report.TopFlows)
	}
	tcpFlow := findFlowFingerprint(report.TopFlows, "tcp", 62000, 7777)
	if tcpFlow == nil {
		t.Fatalf("missing tcp flow: %#v", report.TopFlows)
	}
	if tcpFlow.Direction != "outbound" {
		t.Fatalf("direction = %q, want outbound", tcpFlow.Direction)
	}
	if tcpFlow.ActiveTicks != 3 || tcpFlow.MaxConsecutiveTicks != 2 || tcpFlow.BurstCount != 2 {
		t.Fatalf("unexpected burst stats: %#v", tcpFlow)
	}
	if tcpFlow.HandshakeTicks != 1 || tcpFlow.HandshakeNote == "" {
		t.Fatalf("unexpected handshake stats: %#v", tcpFlow)
	}
	if tcpFlow.TickFrequency != 0.75 {
		t.Fatalf("tick frequency = %v, want 0.75", tcpFlow.TickFrequency)
	}
	if !containsPortFingerprint(report.PortFingerprints, "tcp", "remote", 7777) {
		t.Fatalf("expected remote port fingerprint for 7777, got %#v", report.PortFingerprints)
	}
}

func TestInferTopologyServerHost(t *testing.T) {
	builder := newPacketFingerprintBuilder()
	builder.observe(1, endpoint{Proto: "udp", LocalAddr: "0.0.0.0", LocalPort: 7777, RemoteAddr: "*", RemotePort: 0, PID: 42})
	builder.observe(2, endpoint{Proto: "udp", LocalAddr: "0.0.0.0", LocalPort: 7777, RemoteAddr: "*", RemotePort: 0, PID: 42})
	report := inferTopology(&captureSummary{PacketFingerprint: builder.report(2)})

	if report.Mode != "server_or_host" {
		t.Fatalf("mode = %q, want server_or_host: %#v", report.Mode, report)
	}
	if !containsText(report.Reasons, "listener_flow_seen") {
		t.Fatalf("expected listener reason, got %#v", report.Reasons)
	}
	if report.Confidence == "unknown" {
		t.Fatalf("expected non-unknown confidence: %#v", report)
	}
}

func TestInferTopologyClientToServer(t *testing.T) {
	builder := newPacketFingerprintBuilder()
	builder.observe(1, endpoint{Proto: "tcp", LocalAddr: "127.0.0.1", LocalPort: 62000, RemoteAddr: "10.0.0.20", RemotePort: 7777, State: "ESTABLISHED", PID: 42})
	builder.observe(2, endpoint{Proto: "tcp", LocalAddr: "127.0.0.1", LocalPort: 62000, RemoteAddr: "10.0.0.20", RemotePort: 7777, State: "ESTABLISHED", PID: 42})
	report := inferTopology(&captureSummary{PacketFingerprint: builder.report(2)})

	if report.Mode != "client_to_server" {
		t.Fatalf("mode = %q, want client_to_server: %#v", report.Mode, report)
	}
	if !containsText(report.Reasons, "outbound_flow_seen") {
		t.Fatalf("expected outbound reason, got %#v", report.Reasons)
	}
}

func TestInferTopologyPeerToPeerCandidate(t *testing.T) {
	builder := newPacketFingerprintBuilder()
	builder.observe(1, endpoint{Proto: "udp", LocalAddr: "127.0.0.1", LocalPort: 7777, RemoteAddr: "10.0.0.20", RemotePort: 7778, PID: 42})
	builder.observe(2, endpoint{Proto: "udp", LocalAddr: "127.0.0.1", LocalPort: 7777, RemoteAddr: "10.0.0.20", RemotePort: 7778, PID: 42})
	builder.observe(1, endpoint{Proto: "udp", LocalAddr: "127.0.0.1", LocalPort: 7777, RemoteAddr: "10.0.0.30", RemotePort: 7779, PID: 42})
	builder.observe(2, endpoint{Proto: "udp", LocalAddr: "127.0.0.1", LocalPort: 7777, RemoteAddr: "10.0.0.30", RemotePort: 7779, PID: 42})
	report := inferTopology(&captureSummary{PacketFingerprint: builder.report(2)})

	if report.Mode != "peer_to_peer_candidate" {
		t.Fatalf("mode = %q, want peer_to_peer_candidate: %#v", report.Mode, report)
	}
	if report.Signals.DistinctRemoteAddrs != 2 {
		t.Fatalf("expected two remote addresses: %#v", report.Signals)
	}
	if !containsText(report.Reasons, "multiple_remote_addresses") {
		t.Fatalf("expected p2p reason, got %#v", report.Reasons)
	}
}

func TestBuildUnknownGameReport(t *testing.T) {
	s := &captureSummary{
		ProcessPath:      `C:\Games\Unknown\unknown.exe`,
		ProcessSHA256:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		SteamAppID:       12345,
		SteamAppIDSource: "steam_appid_txt",
		RecommendedNet:   "udp",
		RecommendedPort:  7777,
		TopLocalPorts: map[string][]pair{
			"tcp": {},
			"udp": {{Port: 7777, Hits: 5}},
		},
		TopRemotePorts: map[string][]pair{
			"tcp": {},
			"udp": {},
		},
	}
	s.PortSelection = buildPortSelectionReport(s, "udp", 7777, compatMatch{})
	s.MultiPhase = &multiPhaseReport{Enabled: true}
	fingerprint := newPacketFingerprintBuilder()
	fingerprint.observe(1, endpoint{Proto: "udp", LocalAddr: "127.0.0.1", LocalPort: 7777, RemoteAddr: "*", PID: 42})
	s.PacketFingerprint = fingerprint.report(1)
	s.Topology = inferTopology(s)

	report := buildUnknownGameReport(processCandidate{PID: 42, Name: "unknown.exe"}, "unknown-udp", s, []string{"note"})
	if report.SchemaVersion != "unknown_game_report.v1" {
		t.Fatalf("unexpected schema: %#v", report)
	}
	if report.ProcessName != "unknown.exe" || report.PluginName != "unknown-udp" {
		t.Fatalf("unexpected report identity: %#v", report)
	}
	if report.ProcessSHA256 != s.ProcessSHA256 || report.SteamAppID != 12345 || report.SteamAppIDSource != "steam_appid_txt" {
		t.Fatalf("missing process signals in report: %#v", report)
	}
	if report.PortSelection == nil {
		t.Fatal("expected port selection in unknown report")
	}
	if report.MultiPhase == nil || !report.MultiPhase.Enabled {
		t.Fatalf("expected multi-phase report, got %#v", report.MultiPhase)
	}
	if report.PacketFingerprint == nil || report.PacketFingerprint.ObservedEndpointCount != 1 {
		t.Fatalf("expected packet fingerprint report, got %#v", report.PacketFingerprint)
	}
	if report.Topology == nil || report.Topology.Mode == "" {
		t.Fatalf("expected topology report, got %#v", report.Topology)
	}
}

func TestParseSteamAppIDText(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{in: `steam://rungameid/1966720`, want: 1966720},
		{in: `SteamAppId=480`, want: 480},
		{in: `steam_appid: "12345"`, want: 12345},
		{in: `steam.exe -applaunch 1966720`, want: 1966720},
		{in: `no app id here`, want: 0},
	}
	for _, tc := range tests {
		if got := parseSteamAppIDText(tc.in); got != tc.want {
			t.Fatalf("parseSteamAppIDText(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func containsRejectedPort(list []portCandidateReport, port int, reason string) bool {
	for _, item := range list {
		if item.Port == port && item.Reason == reason {
			return true
		}
	}
	return false
}

func containsPhasePortRole(list []phasePortRole, network string, port int, role string) bool {
	for _, item := range list {
		if item.Network == network && item.Port == port && item.Role == role {
			return true
		}
	}
	return false
}

func findFlowFingerprint(list []flowFingerprint, network string, localPort, remotePort int) *flowFingerprint {
	for i := range list {
		if list[i].Network == network && list[i].LocalPort == localPort && list[i].RemotePort == remotePort {
			return &list[i]
		}
	}
	return nil
}

func containsPortFingerprint(list []portFingerprint, network, direction string, port int) bool {
	for _, item := range list {
		if item.Network == network && item.Direction == direction && item.Port == port {
			return true
		}
	}
	return false
}

func containsText(list []string, needle string) bool {
	for _, item := range list {
		if strings.Contains(item, needle) {
			return true
		}
	}
	return false
}
