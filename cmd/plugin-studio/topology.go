package main

import (
	"fmt"
	"sort"
	"strings"
)

func inferTopology(summary *captureSummary) *topologyReport {
	if summary == nil || summary.PacketFingerprint == nil {
		return &topologyReport{
			Mode:       "unknown",
			Confidence: "unknown",
			Reasons:    []string{"no_packet_fingerprint"},
		}
	}

	signals := topologySignals{}
	remoteAddrs := map[string]struct{}{}
	remotePorts := map[int]struct{}{}
	stableLocalPorts := map[int]struct{}{}
	stableRemotePorts := map[int]struct{}{}
	nonEphemeralLocalPorts := map[int]struct{}{}
	nonEphemeralRemotePorts := map[int]struct{}{}
	reasons := []string{}

	for _, flow := range summary.PacketFingerprint.TopFlows {
		switch flow.Direction {
		case "listener":
			signals.ListenerFlows++
		case "outbound":
			signals.OutboundFlows++
		case "inbound":
			signals.InboundFlows++
		default:
			signals.ConnectedFlows++
		}
		if isMeaningfulRemoteAddress(flow.RemoteAddress) {
			remoteAddrs[flow.RemoteAddress] = struct{}{}
		}
		if flow.RemotePort > 0 {
			remotePorts[flow.RemotePort] = struct{}{}
		}
	}

	for _, port := range summary.PacketFingerprint.PortFingerprints {
		if port.Direction == "local" && !isEphemeralPort(port.Port) {
			nonEphemeralLocalPorts[port.Port] = struct{}{}
			if port.TickFrequency >= 0.5 {
				stableLocalPorts[port.Port] = struct{}{}
			}
		}
		if port.Direction == "remote" && !isEphemeralPort(port.Port) {
			nonEphemeralRemotePorts[port.Port] = struct{}{}
			if port.TickFrequency >= 0.5 {
				stableRemotePorts[port.Port] = struct{}{}
			}
		}
	}

	signals.DistinctRemoteAddrs = len(remoteAddrs)
	signals.DistinctRemotePorts = sortedIntSet(remotePorts)
	signals.StableLocalPorts = sortedIntSet(stableLocalPorts)
	signals.StableRemotePorts = sortedIntSet(stableRemotePorts)
	signals.NonEphemeralLocalPorts = len(nonEphemeralLocalPorts)
	signals.NonEphemeralRemotePorts = len(nonEphemeralRemotePorts)
	if summary.MultiPhase != nil {
		signals.MultiPhasePortRoles = topologyPhaseRoles(summary.MultiPhase.PortRoles)
	}

	serverScore := 0
	clientScore := 0
	p2pScore := 0
	if signals.ListenerFlows > 0 {
		serverScore += 30
		reasons = append(reasons, "listener_flow_seen")
	}
	if signals.InboundFlows > 0 {
		serverScore += 20
		reasons = append(reasons, "inbound_flow_seen")
	}
	if len(signals.StableLocalPorts) > 0 {
		serverScore += 20
		reasons = append(reasons, "stable_local_game_port")
	}
	if signals.OutboundFlows > 0 {
		clientScore += 30
		reasons = append(reasons, "outbound_flow_seen")
	}
	if len(signals.StableRemotePorts) > 0 {
		clientScore += 20
		reasons = append(reasons, "stable_remote_game_port")
	}
	if signals.ListenerFlows == 0 && signals.OutboundFlows > 0 {
		clientScore += 10
		reasons = append(reasons, "no_listener_with_outbound_flow")
	}
	if signals.DistinctRemoteAddrs >= 2 {
		p2pScore += 30
		reasons = append(reasons, "multiple_remote_addresses")
	}
	if len(signals.DistinctRemotePorts) >= 2 {
		p2pScore += 15
		reasons = append(reasons, "multiple_remote_ports")
	}
	if signals.InboundFlows > 0 && signals.OutboundFlows > 0 && signals.ListenerFlows == 0 {
		p2pScore += 25
		reasons = append(reasons, "bidirectional_non_listener_flows")
	}
	if len(signals.StableLocalPorts) > 0 && len(signals.StableRemotePorts) > 0 && signals.DistinctRemoteAddrs >= 2 {
		p2pScore += 20
		reasons = append(reasons, "stable_local_and_remote_ports_across_peers")
	}

	mode := "unknown"
	score := maxInt(maxInt(serverScore, clientScore), p2pScore)
	switch {
	case p2pScore >= 35 && p2pScore >= serverScore+10 && p2pScore >= clientScore+10:
		mode = "peer_to_peer_candidate"
	case serverScore >= 35 && serverScore >= clientScore+15:
		mode = "server_or_host"
	case clientScore >= 35 && clientScore >= serverScore+15:
		mode = "client_to_server"
	case score > 0:
		mode = "mixed_or_unclear"
		reasons = append(reasons, fmt.Sprintf("scores_server_%d_client_%d_p2p_%d", serverScore, clientScore, p2pScore))
	default:
		reasons = append(reasons, "insufficient_socket_topology_signals")
	}

	return &topologyReport{
		Mode:       mode,
		Confidence: topologyConfidence(score),
		Score:      score,
		Reasons:    mergeReasonStrings(reasons),
		Signals:    signals,
	}
}

func topologyConfidence(score int) string {
	switch {
	case score >= 70:
		return "high"
	case score >= 45:
		return "medium"
	case score >= 20:
		return "low"
	default:
		return "unknown"
	}
}

func topologyPhaseRoles(roles []phasePortRole) []string {
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		out = append(out, fmt.Sprintf("%s:%s/%d", role.Role, role.Network, role.Port))
	}
	sort.Strings(out)
	if len(out) > 16 {
		out = out[:16]
	}
	return out
}

func sortedIntSet(set map[int]struct{}) []int {
	out := make([]int, 0, len(set))
	for item := range set {
		out = append(out, item)
	}
	sort.Ints(out)
	return out
}

func isMeaningfulRemoteAddress(addr string) bool {
	addr = normalizeEndpointAddress(addr)
	return addr != "" && addr != "*" && addr != "0.0.0.0" && addr != "::"
}

func mergeReasonStrings(reasons []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			continue
		}
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		out = append(out, reason)
	}
	return out
}
