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
