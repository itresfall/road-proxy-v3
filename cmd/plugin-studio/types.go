package main

import (
	"strconv"
)

type endpoint struct {
	Proto      string
	LocalAddr  string
	LocalPort  int
	RemoteAddr string
	RemotePort int
	State      string
	PID        int
}

type processCandidate struct {
	PID         int
	Name        string
	ExePath     string
	CommandLine string
	TCPCount    int
	UDPCount    int
	LocalTCP    []int
	LocalUDP    []int
	LastState   string
}

type processInfo struct {
	Name        string
	ExePath     string
	CommandLine string
}

type optionalBool struct {
	set   bool
	value bool
}

func (b *optionalBool) Set(raw string) error {
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return err
	}
	b.set = true
	b.value = value
	return nil
}

func (b optionalBool) String() string {
	if !b.set {
		return ""
	}
	return strconv.FormatBool(b.value)
}

func (b optionalBool) IsBoolFlag() bool {
	return true
}

type studioCLIOptions struct {
	Process          string
	PID              int
	Seconds          int
	MultiPhase       bool
	PhaseSeconds     int
	Network          string
	TargetHost       string
	TargetPort       int
	ClientListenPort int
	PluginName       string
	UDPPeerBroadcast optionalBool
	Force            bool
}

func (o studioCLIOptions) Enabled() bool {
	return o.Process != "" ||
		o.PID > 0 ||
		o.MultiPhase ||
		o.Network != "" ||
		o.TargetHost != "" ||
		o.TargetPort > 0 ||
		o.ClientListenPort > 0 ||
		o.PluginName != "" ||
		o.UDPPeerBroadcast.set ||
		o.Force
}

type capturePhase struct {
	Name           string
	LabelKey       string
	InstructionKey string
}

var defaultCapturePhases = []capturePhase{
	{Name: "lobby", LabelKey: "studio.phase.lobby", InstructionKey: "studio.phase.lobby_instruction"},
	{Name: "connect", LabelKey: "studio.phase.connect", InstructionKey: "studio.phase.connect_instruction"},
	{Name: "ingame", LabelKey: "studio.phase.ingame", InstructionKey: "studio.phase.ingame_instruction"},
	{Name: "disconnect", LabelKey: "studio.phase.disconnect", InstructionKey: "studio.phase.disconnect_instruction"},
}

type captureSummary struct {
	ProcessPID        int                      `json:"process_pid"`
	ProcessName       string                   `json:"process_name"`
	ProcessPath       string                   `json:"process_path,omitempty"`
	ProcessSHA256     string                   `json:"process_sha256,omitempty"`
	SteamAppID        int                      `json:"steam_app_id,omitempty"`
	SteamAppIDSource  string                   `json:"steam_app_id_source,omitempty"`
	CaptureSeconds    int                      `json:"capture_seconds"`
	Ticks             int                      `json:"ticks"`
	EndpointHits      map[string]int           `json:"endpoint_hits"`
	LocalPortHits     map[string]map[int]int   `json:"local_port_hits"`
	RemotePortHits    map[string]map[int]int   `json:"remote_port_hits"`
	TopLocalPorts     map[string][]pair        `json:"top_local_ports"`
	TopRemotePorts    map[string][]pair        `json:"top_remote_ports"`
	RecommendedNet    string                   `json:"recommended_network"`
	RecommendedPort   int                      `json:"recommended_port"`
	CompatProfile     string                   `json:"compat_profile,omitempty"`
	CompatConfidence  string                   `json:"compat_confidence,omitempty"`
	CompatReasons     []string                 `json:"compat_match_reasons,omitempty"`
	PortSelection     *portSelectionReport     `json:"port_selection,omitempty"`
	MultiPhase        *multiPhaseReport        `json:"multi_phase,omitempty"`
	PacketFingerprint *packetFingerprintReport `json:"packet_fingerprint,omitempty"`
	Topology          *topologyReport          `json:"topology,omitempty"`
	ClientListenPort  int                      `json:"client_listen_port,omitempty"`
	UDPPeerBroadcast  bool                     `json:"udp_peer_broadcast"`
	Notes             []string                 `json:"notes,omitempty"`
}

type multiPhaseReport struct {
	Enabled   bool                  `json:"enabled"`
	Phases    []capturePhaseSummary `json:"phases"`
	PortRoles []phasePortRole       `json:"port_roles"`
}

type capturePhaseSummary struct {
	Name              string                   `json:"name"`
	CaptureSeconds    int                      `json:"capture_seconds"`
	Ticks             int                      `json:"ticks"`
	EndpointHits      map[string]int           `json:"endpoint_hits"`
	LocalPortHits     map[string]map[int]int   `json:"local_port_hits"`
	RemotePortHits    map[string]map[int]int   `json:"remote_port_hits"`
	TopLocalPorts     map[string][]pair        `json:"top_local_ports"`
	TopRemotePorts    map[string][]pair        `json:"top_remote_ports"`
	RecommendedNet    string                   `json:"recommended_network"`
	RecommendedPort   int                      `json:"recommended_port"`
	PacketFingerprint *packetFingerprintReport `json:"packet_fingerprint,omitempty"`
}

type packetFingerprintReport struct {
	Source                string            `json:"source"`
	PacketSizeObserved    bool              `json:"packet_size_observed"`
	PacketSizeSource      string            `json:"packet_size_source"`
	PacketSizeNote        string            `json:"packet_size_note"`
	TotalTicks            int               `json:"total_ticks"`
	ObservedEndpointCount int               `json:"observed_endpoint_count"`
	TopFlows              []flowFingerprint `json:"top_flows"`
	PortFingerprints      []portFingerprint `json:"port_fingerprints"`
}

type topologyReport struct {
	Mode       string          `json:"mode"`
	Confidence string          `json:"confidence"`
	Score      int             `json:"score"`
	Reasons    []string        `json:"reasons"`
	Signals    topologySignals `json:"signals"`
}

type topologySignals struct {
	ListenerFlows           int      `json:"listener_flows"`
	OutboundFlows           int      `json:"outbound_flows"`
	InboundFlows            int      `json:"inbound_flows"`
	ConnectedFlows          int      `json:"connected_flows"`
	DistinctRemoteAddrs     int      `json:"distinct_remote_addrs"`
	DistinctRemotePorts     []int    `json:"distinct_remote_ports,omitempty"`
	StableLocalPorts        []int    `json:"stable_local_ports,omitempty"`
	StableRemotePorts       []int    `json:"stable_remote_ports,omitempty"`
	MultiPhasePortRoles     []string `json:"multi_phase_port_roles,omitempty"`
	NonEphemeralLocalPorts  int      `json:"non_ephemeral_local_ports"`
	NonEphemeralRemotePorts int      `json:"non_ephemeral_remote_ports"`
}

type flowFingerprint struct {
	Network             string         `json:"network"`
	Direction           string         `json:"direction"`
	LocalAddress        string         `json:"local_address"`
	LocalPort           int            `json:"local_port"`
	RemoteAddress       string         `json:"remote_address"`
	RemotePort          int            `json:"remote_port"`
	Hits                int            `json:"hits"`
	ActiveTicks         int            `json:"active_ticks"`
	TickFrequency       float64        `json:"tick_frequency"`
	FirstTick           int            `json:"first_tick"`
	LastTick            int            `json:"last_tick"`
	MaxConsecutiveTicks int            `json:"max_consecutive_ticks"`
	BurstCount          int            `json:"burst_count"`
	FirstState          string         `json:"first_state,omitempty"`
	LastState           string         `json:"last_state,omitempty"`
	States              map[string]int `json:"states,omitempty"`
	HandshakeTicks      int            `json:"handshake_ticks,omitempty"`
	HandshakeNote       string         `json:"handshake_note,omitempty"`
}

type portFingerprint struct {
	Network             string  `json:"network"`
	Direction           string  `json:"direction"`
	Port                int     `json:"port"`
	Hits                int     `json:"hits"`
	ActiveTicks         int     `json:"active_ticks"`
	TickFrequency       float64 `json:"tick_frequency"`
	FirstTick           int     `json:"first_tick"`
	LastTick            int     `json:"last_tick"`
	MaxConsecutiveTicks int     `json:"max_consecutive_ticks"`
	BurstCount          int     `json:"burst_count"`
}

type phasePortRole struct {
	Network   string   `json:"network"`
	Port      int      `json:"port"`
	Role      string   `json:"role"`
	Phases    []string `json:"phases"`
	Sources   []string `json:"sources"`
	TotalHits int      `json:"total_hits"`
}

type pair struct {
	Port int `json:"port"`
	Hits int `json:"hits"`
}

type portSelectionReport struct {
	SelectedNetwork string                `json:"selected_network"`
	SelectedPort    int                   `json:"selected_port"`
	Reason          string                `json:"reason"`
	Rejected        []portCandidateReport `json:"rejected,omitempty"`
}

type portCandidateReport struct {
	Network string `json:"network"`
	Port    int    `json:"port"`
	Hits    int    `json:"hits"`
	Source  string `json:"source"`
	Reason  string `json:"reason"`
}

type unknownGameReport struct {
	SchemaVersion      string                   `json:"schema_version"`
	ProcessName        string                   `json:"process_name"`
	ProcessPID         int                      `json:"process_pid"`
	ProcessPath        string                   `json:"process_path,omitempty"`
	ProcessSHA256      string                   `json:"process_sha256,omitempty"`
	SteamAppID         int                      `json:"steam_app_id,omitempty"`
	SteamAppIDSource   string                   `json:"steam_app_id_source,omitempty"`
	PluginName         string                   `json:"plugin_name"`
	CaptureDate        string                   `json:"capture_date"`
	ROADVersion        string                   `json:"road_version"`
	RecommendedNetwork string                   `json:"recommended_network"`
	RecommendedPort    int                      `json:"recommended_port"`
	TopLocalPorts      map[string][]pair        `json:"top_local_ports"`
	TopRemotePorts     map[string][]pair        `json:"top_remote_ports"`
	PortSelection      *portSelectionReport     `json:"port_selection,omitempty"`
	MultiPhase         *multiPhaseReport        `json:"multi_phase,omitempty"`
	PacketFingerprint  *packetFingerprintReport `json:"packet_fingerprint,omitempty"`
	Topology           *topologyReport          `json:"topology,omitempty"`
	Notes              []string                 `json:"notes,omitempty"`
}
