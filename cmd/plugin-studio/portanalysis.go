package main

import (
	"fmt"
	"sort"
	"strings"
)

func recommendNetworkAndPort(summary *captureSummary) (string, int) {
	if summary == nil {
		return "tcp", 0
	}
	if network, port, ok := advancedCaptureRecommendation(summary); ok {
		return network, port
	}

	tcpPort, tcpHits := topCandidatePort(summary.LocalPortHits["tcp"], false)
	udpPort, udpHits := topCandidatePort(summary.LocalPortHits["udp"], false)
	if udpPort > 0 && (tcpPort == 0 || udpHits >= tcpHits) {
		return "udp", udpPort
	}
	if tcpPort > 0 {
		return "tcp", tcpPort
	}

	tcpPort, tcpHits = topCandidatePort(summary.RemotePortHits["tcp"], false)
	udpPort, udpHits = topCandidatePort(summary.RemotePortHits["udp"], false)
	if udpPort > 0 && udpHits > tcpHits {
		return "udp", udpPort
	}
	if tcpPort > 0 {
		return "tcp", tcpPort
	}
	if udpPort > 0 {
		return "udp", udpPort
	}

	tcpPort, tcpHits = topCandidatePort(summary.LocalPortHits["tcp"], true)
	udpPort, udpHits = topCandidatePort(summary.LocalPortHits["udp"], true)
	if udpPort > 0 && udpHits > tcpHits {
		return "udp", udpPort
	}
	if tcpPort > 0 {
		return "tcp", tcpPort
	}
	if udpPort > 0 {
		return "udp", udpPort
	}

	tcpHits = sumPortHits(summary.LocalPortHits["tcp"])
	udpHits = sumPortHits(summary.LocalPortHits["udp"])
	network := "tcp"
	if udpHits > tcpHits {
		network = "udp"
	}
	port := topPort(summary.LocalPortHits[network])
	if port == 0 {
		port = topPort(summary.RemotePortHits[network])
	}
	return network, port
}

func advancedCaptureRecommendation(summary *captureSummary) (string, int, bool) {
	if summary == nil || summary.PacketFingerprint == nil || !summary.PacketFingerprint.PacketSizeObserved {
		return "", 0, false
	}
	ports := summary.PacketFingerprint.CapturedPorts
	if len(ports) == 0 {
		return "", 0, false
	}
	for _, scope := range []string{"local", "remote"} {
		if network, port := bestCapturedPort(ports, scope, false); port > 0 {
			return network, port, true
		}
	}
	for _, scope := range []string{"local", "remote"} {
		if network, port := bestCapturedPort(ports, scope, true); port > 0 {
			return network, port, true
		}
	}
	return "", 0, false
}

func bestCapturedPort(ports []capturedPortStat, scope string, allowEphemeral bool) (string, int) {
	bestNetwork := ""
	bestPort := 0
	bestPackets := 0
	for _, candidate := range ports {
		if candidate.Scope != scope || candidate.Port <= 0 || candidate.Packets <= 0 {
			continue
		}
		if isLikelyNoisePort(candidate.Port) || (!allowEphemeral && isEphemeralPort(candidate.Port)) {
			continue
		}
		if candidate.Packets > bestPackets ||
			(candidate.Packets == bestPackets && candidate.Network == "udp" && bestNetwork != "udp") ||
			(candidate.Packets == bestPackets && candidate.Network == bestNetwork && (bestPort == 0 || candidate.Port < bestPort)) {
			bestNetwork = candidate.Network
			bestPort = candidate.Port
			bestPackets = candidate.Packets
		}
	}
	return bestNetwork, bestPort
}

func topCandidatePort(src map[int]int, allowEphemeral bool) (int, int) {
	bestPort := 0
	bestHits := 0
	for port, hits := range src {
		if port <= 0 || hits <= 0 {
			continue
		}
		if isLikelyNoisePort(port) {
			continue
		}
		if !allowEphemeral && isEphemeralPort(port) {
			continue
		}
		if hits > bestHits || (hits == bestHits && (bestPort == 0 || port < bestPort)) {
			bestHits = hits
			bestPort = port
		}
	}
	return bestPort, bestHits
}

func isEphemeralPort(port int) bool {
	return port >= 49152 && port <= 65535
}

func isLikelyNoisePort(port int) bool {
	switch port {
	case 53, 80, 123, 137, 138, 139, 443, 1900, 5353:
		return true
	default:
		return false
	}
}

func topPorts(src map[int]int, limit int) []pair {
	if len(src) == 0 {
		return []pair{}
	}
	out := make([]pair, 0, len(src))
	for port, hits := range src {
		out = append(out, pair{Port: port, Hits: hits})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hits != out[j].Hits {
			return out[i].Hits > out[j].Hits
		}
		return out[i].Port < out[j].Port
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func topPort(src map[int]int) int {
	bestPort := 0
	bestHits := 0
	for port, hits := range src {
		if hits > bestHits {
			bestHits = hits
			bestPort = port
		}
	}
	return bestPort
}

func sumPortHits(src map[int]int) int {
	sum := 0
	for _, hits := range src {
		sum += hits
	}
	return sum
}

func mapKeysSorted(m map[int]struct{}) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

func firstPort(list []int) int {
	if len(list) == 0 {
		return 0
	}
	return list[0]
}

func printTopPorts(proto string, pairs []pair) {
	if len(pairs) == 0 {
		fmt.Printf(sm("studio.top_ports_none"), strings.ToUpper(proto))
		return
	}
	fmt.Printf(sm("studio.top_ports_candidates"), strings.ToUpper(proto))
	fmt.Println("")
	for _, p := range pairs {
		fmt.Printf("    %d (%d hits)\n", p.Port, p.Hits)
	}
}
