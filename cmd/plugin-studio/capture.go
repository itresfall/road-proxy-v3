package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func discoverCandidates() ([]processCandidate, error) {
	endpoints, err := collectEndpoints()
	if err != nil {
		return nil, err
	}

	infos, err := getProcessInfos()
	if err != nil {
		return nil, err
	}

	type agg struct {
		name        string
		exePath     string
		commandLine string
		tcp         int
		udp         int
		localTCP    map[int]struct{}
		localUDP    map[int]struct{}
	}
	aggByPID := map[int]*agg{}
	for _, ep := range endpoints {
		if ep.PID <= 0 {
			continue
		}
		cur, ok := aggByPID[ep.PID]
		if !ok {
			info := infos[ep.PID]
			cur = &agg{
				name:        info.Name,
				exePath:     info.ExePath,
				commandLine: info.CommandLine,
				localTCP:    map[int]struct{}{},
				localUDP:    map[int]struct{}{},
			}
			aggByPID[ep.PID] = cur
		}
		if cur.name == "" {
			cur.name = infos[ep.PID].Name
		}

		switch ep.Proto {
		case "tcp":
			cur.tcp++
			if ep.LocalPort > 0 {
				cur.localTCP[ep.LocalPort] = struct{}{}
			}
		case "udp":
			cur.udp++
			if ep.LocalPort > 0 {
				cur.localUDP[ep.LocalPort] = struct{}{}
			}
		}
	}

	candidates := make([]processCandidate, 0, len(aggByPID))
	for pid, cur := range aggByPID {
		name := strings.TrimSpace(cur.name)
		if name == "" {
			name = fmt.Sprintf("pid-%d", pid)
		}
		candidates = append(candidates, processCandidate{
			PID:         pid,
			Name:        name,
			ExePath:     cur.exePath,
			CommandLine: cur.commandLine,
			TCPCount:    cur.tcp,
			UDPCount:    cur.udp,
			LocalTCP:    mapKeysSorted(cur.localTCP),
			LocalUDP:    mapKeysSorted(cur.localUDP),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		ti := candidates[i].TCPCount + candidates[i].UDPCount
		tj := candidates[j].TCPCount + candidates[j].UDPCount
		if ti != tj {
			return ti > tj
		}
		return candidates[i].PID < candidates[j].PID
	})

	return candidates, nil
}

func captureProcess(pid int, processName string, duration, interval time.Duration) (*captureSummary, error) {
	if duration <= 0 {
		duration = 10 * time.Second
	}
	if interval <= 0 {
		interval = time.Second
	}

	summary := newCaptureSummary(pid, processName, int(duration/time.Second))
	fingerprint := newPacketFingerprintBuilder()

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		endpoints, err := collectEndpoints()
		if err != nil {
			return nil, err
		}

		summary.Ticks++
		tick := summary.Ticks
		for _, ep := range endpoints {
			if ep.PID != pid {
				continue
			}
			key := fmt.Sprintf("%s %s:%d -> %s:%d state=%s", ep.Proto, ep.LocalAddr, ep.LocalPort, ep.RemoteAddr, ep.RemotePort, ep.State)
			summary.EndpointHits[key]++

			if ep.LocalPort > 0 {
				summary.LocalPortHits[ep.Proto][ep.LocalPort]++
			}
			if ep.RemotePort > 0 {
				summary.RemotePortHits[ep.Proto][ep.RemotePort]++
			}
			fingerprint.observe(tick, ep)
		}

		time.Sleep(interval)
	}

	summary.TopLocalPorts["tcp"] = topPorts(summary.LocalPortHits["tcp"], 8)
	summary.TopLocalPorts["udp"] = topPorts(summary.LocalPortHits["udp"], 8)
	summary.TopRemotePorts["tcp"] = topPorts(summary.RemotePortHits["tcp"], 8)
	summary.TopRemotePorts["udp"] = topPorts(summary.RemotePortHits["udp"], 8)
	summary.RecommendedNet, summary.RecommendedPort = recommendNetworkAndPort(summary)
	summary.PacketFingerprint = fingerprint.report(summary.Ticks)
	summary.Topology = inferTopology(summary)
	return summary, nil
}

func newCaptureSummary(pid int, processName string, captureSeconds int) *captureSummary {
	return &captureSummary{
		ProcessPID:     pid,
		ProcessName:    processName,
		CaptureSeconds: captureSeconds,
		EndpointHits:   map[string]int{},
		LocalPortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {},
		},
		RemotePortHits: map[string]map[int]int{
			"tcp": {},
			"udp": {},
		},
		TopLocalPorts: map[string][]pair{
			"tcp": {},
			"udp": {},
		},
		TopRemotePorts: map[string][]pair{
			"tcp": {},
			"udp": {},
		},
	}
}

func captureProcessPhases(pid int, processName string, phaseSeconds int, interval time.Duration, beforePhase func(capturePhase) error) (*captureSummary, error) {
	if phaseSeconds < 5 {
		phaseSeconds = 5
	}

	phases := make([]capturePhaseSummary, 0, len(defaultCapturePhases))
	for _, phase := range defaultCapturePhases {
		if beforePhase != nil {
			if err := beforePhase(phase); err != nil {
				return nil, err
			}
		}
		summary, err := captureProcess(pid, processName, time.Duration(phaseSeconds)*time.Second, interval)
		if err != nil {
			return nil, err
		}
		phases = append(phases, buildCapturePhaseSummary(phase.Name, summary))
	}

	summary := aggregatePhaseSummaries(pid, processName, phases)
	summary.MultiPhase = &multiPhaseReport{
		Enabled:   true,
		Phases:    phases,
		PortRoles: classifyPhasePortRoles(phases),
	}
	return summary, nil
}

func buildCapturePhaseSummary(name string, summary *captureSummary) capturePhaseSummary {
	if summary == nil {
		return capturePhaseSummary{Name: name}
	}
	return capturePhaseSummary{
		Name:              name,
		CaptureSeconds:    summary.CaptureSeconds,
		Ticks:             summary.Ticks,
		EndpointHits:      summary.EndpointHits,
		LocalPortHits:     summary.LocalPortHits,
		RemotePortHits:    summary.RemotePortHits,
		TopLocalPorts:     summary.TopLocalPorts,
		TopRemotePorts:    summary.TopRemotePorts,
		RecommendedNet:    summary.RecommendedNet,
		RecommendedPort:   summary.RecommendedPort,
		PacketFingerprint: summary.PacketFingerprint,
	}
}

func aggregatePhaseSummaries(pid int, processName string, phases []capturePhaseSummary) *captureSummary {
	totalSeconds := 0
	summary := newCaptureSummary(pid, processName, 0)
	for _, phase := range phases {
		totalSeconds += phase.CaptureSeconds
		summary.Ticks += phase.Ticks
		mergeStringIntMap(summary.EndpointHits, phase.EndpointHits)
		mergePortHitMap(summary.LocalPortHits, phase.LocalPortHits)
		mergePortHitMap(summary.RemotePortHits, phase.RemotePortHits)
	}
	summary.CaptureSeconds = totalSeconds
	summary.TopLocalPorts["tcp"] = topPorts(summary.LocalPortHits["tcp"], 8)
	summary.TopLocalPorts["udp"] = topPorts(summary.LocalPortHits["udp"], 8)
	summary.TopRemotePorts["tcp"] = topPorts(summary.RemotePortHits["tcp"], 8)
	summary.TopRemotePorts["udp"] = topPorts(summary.RemotePortHits["udp"], 8)
	summary.RecommendedNet, summary.RecommendedPort = recommendNetworkAndPort(summary)
	summary.PacketFingerprint = aggregatePhasePacketFingerprints(phases)
	summary.Topology = inferTopology(summary)
	return summary
}

func mergeStringIntMap(dst, src map[string]int) {
	for key, hits := range src {
		dst[key] += hits
	}
}

func mergePortHitMap(dst, src map[string]map[int]int) {
	for network, ports := range src {
		if dst[network] == nil {
			dst[network] = map[int]int{}
		}
		for port, hits := range ports {
			dst[network][port] += hits
		}
	}
}

type phasePortKey struct {
	Network string
	Port    int
}

type phasePortAccumulator struct {
	Phases  map[string]struct{}
	Sources map[string]struct{}
	Hits    int
}

type flowFingerprintKey struct {
	Network       string
	LocalAddress  string
	LocalPort     int
	RemoteAddress string
	RemotePort    int
}

type endpointFingerprintAccumulator struct {
	Key                  flowFingerprintKey
	Direction            string
	Hits                 int
	ActiveTicks          int
	FirstTick            int
	LastTick             int
	MaxConsecutiveTicks  int
	BurstCount           int
	currentStreak        int
	FirstState           string
	LastState            string
	States               map[string]int
	FirstEstablishedTick int
}

type portFingerprintKey struct {
	Network   string
	Direction string
	Port      int
}

type portFingerprintAccumulator struct {
	Key                 portFingerprintKey
	Hits                int
	ActiveTicks         int
	FirstTick           int
	LastTick            int
	MaxConsecutiveTicks int
	BurstCount          int
	currentStreak       int
}
