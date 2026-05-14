package main

import (
	"sort"
	"strings"
)

type packetFingerprintBuilder struct {
	flows map[flowFingerprintKey]*endpointFingerprintAccumulator
	ports map[portFingerprintKey]*portFingerprintAccumulator
}

func classifyPhasePortRoles(phases []capturePhaseSummary) []phasePortRole {
	acc := map[phasePortKey]*phasePortAccumulator{}
	for _, phase := range phases {
		addPhasePorts(acc, phase.Name, "local", phase.LocalPortHits)
		addPhasePorts(acc, phase.Name, "remote", phase.RemotePortHits)
	}

	roles := make([]phasePortRole, 0, len(acc))
	for key, item := range acc {
		phaseNames := sortedStringSet(item.Phases)
		roles = append(roles, phasePortRole{
			Network:   key.Network,
			Port:      key.Port,
			Role:      classifyPhaseRole(phaseNames, len(phases)),
			Phases:    phaseNames,
			Sources:   sortedStringSet(item.Sources),
			TotalHits: item.Hits,
		})
	}

	sort.Slice(roles, func(i, j int) bool {
		if roles[i].TotalHits != roles[j].TotalHits {
			return roles[i].TotalHits > roles[j].TotalHits
		}
		if roles[i].Network != roles[j].Network {
			return roles[i].Network < roles[j].Network
		}
		return roles[i].Port < roles[j].Port
	})
	if len(roles) > 32 {
		roles = roles[:32]
	}
	return roles
}

func addPhasePorts(acc map[phasePortKey]*phasePortAccumulator, phaseName, source string, portHits map[string]map[int]int) {
	for network, ports := range portHits {
		for port, hits := range ports {
			if port <= 0 || hits <= 0 {
				continue
			}
			key := phasePortKey{Network: network, Port: port}
			item := acc[key]
			if item == nil {
				item = &phasePortAccumulator{
					Phases:  map[string]struct{}{},
					Sources: map[string]struct{}{},
				}
				acc[key] = item
			}
			item.Phases[phaseName] = struct{}{}
			item.Sources[source] = struct{}{}
			item.Hits += hits
		}
	}
}

func classifyPhaseRole(phases []string, totalPhases int) string {
	if totalPhases > 0 && len(phases) == totalPhases {
		return "persistent"
	}
	if len(phases) == 1 {
		switch phases[0] {
		case "connect":
			return "connect_only"
		case "ingame":
			return "game_only"
		case "lobby":
			return "lobby_only"
		case "disconnect":
			return "disconnect_only"
		}
	}
	if stringListContains(phases, "connect") && stringListContains(phases, "ingame") && len(phases) == 2 {
		return "connect_and_game"
	}
	return "multi_phase"
}

func sortedStringSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for item := range set {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func stringListContains(list []string, needle string) bool {
	for _, item := range list {
		if item == needle {
			return true
		}
	}
	return false
}

func newPacketFingerprintBuilder() *packetFingerprintBuilder {
	return &packetFingerprintBuilder{
		flows: map[flowFingerprintKey]*endpointFingerprintAccumulator{},
		ports: map[portFingerprintKey]*portFingerprintAccumulator{},
	}
}

func (b *packetFingerprintBuilder) observe(tick int, ep endpoint) {
	if b == nil || tick <= 0 {
		return
	}
	network := strings.ToLower(strings.TrimSpace(ep.Proto))
	if network == "" {
		return
	}
	flowKey := flowFingerprintKey{
		Network:       network,
		LocalAddress:  normalizeEndpointAddress(ep.LocalAddr),
		LocalPort:     ep.LocalPort,
		RemoteAddress: normalizeEndpointAddress(ep.RemoteAddr),
		RemotePort:    ep.RemotePort,
	}
	flow := b.flows[flowKey]
	if flow == nil {
		flow = &endpointFingerprintAccumulator{
			Key:       flowKey,
			Direction: inferEndpointDirection(ep),
			States:    map[string]int{},
		}
		b.flows[flowKey] = flow
	}
	flow.Hits++
	updateFingerprintWindow(&flow.FirstTick, &flow.LastTick, &flow.ActiveTicks, &flow.BurstCount, &flow.currentStreak, &flow.MaxConsecutiveTicks, tick)
	state := normalizeEndpointState(ep.State)
	if state != "" {
		if flow.FirstState == "" {
			flow.FirstState = state
		}
		flow.LastState = state
		flow.States[state]++
		if state == "ESTABLISHED" && flow.FirstEstablishedTick == 0 {
			flow.FirstEstablishedTick = tick
		}
	}

	if ep.LocalPort > 0 {
		b.observePort(tick, network, "local", ep.LocalPort)
	}
	if ep.RemotePort > 0 {
		b.observePort(tick, network, "remote", ep.RemotePort)
	}
}

func (b *packetFingerprintBuilder) observePort(tick int, network, direction string, port int) {
	key := portFingerprintKey{Network: network, Direction: direction, Port: port}
	portFP := b.ports[key]
	if portFP == nil {
		portFP = &portFingerprintAccumulator{Key: key}
		b.ports[key] = portFP
	}
	portFP.Hits++
	updateFingerprintWindow(&portFP.FirstTick, &portFP.LastTick, &portFP.ActiveTicks, &portFP.BurstCount, &portFP.currentStreak, &portFP.MaxConsecutiveTicks, tick)
}

func updateFingerprintWindow(firstTick, lastTick, activeTicks, burstCount, currentStreak, maxConsecutiveTicks *int, tick int) {
	if *firstTick == 0 {
		*firstTick = tick
	}
	if *lastTick == tick {
		return
	}
	if *lastTick == 0 {
		*burstCount++
		*currentStreak = 1
	} else if *lastTick == tick-1 {
		*currentStreak++
	} else {
		*burstCount++
		*currentStreak = 1
	}
	if *currentStreak > *maxConsecutiveTicks {
		*maxConsecutiveTicks = *currentStreak
	}
	*lastTick = tick
	*activeTicks++
}

func (b *packetFingerprintBuilder) report(totalTicks int) *packetFingerprintReport {
	if b == nil {
		return nil
	}
	report := &packetFingerprintReport{
		Source:                "socket_snapshot",
		PacketSizeObserved:    false,
		PacketSizeSource:      "unavailable_without_packet_capture",
		PacketSizeNote:        "Current Plugin Studio scanners observe sockets, not packets. Packet sizes require a later pcap/WFP/AF_PACKET metadata capture path.",
		TotalTicks:            totalTicks,
		ObservedEndpointCount: len(b.flows),
		TopFlows:              buildFlowFingerprints(b.flows, totalTicks),
		PortFingerprints:      buildPortFingerprints(b.ports, totalTicks),
	}
	if len(report.TopFlows) == 0 && len(report.PortFingerprints) == 0 {
		return nil
	}
	return report
}

func buildFlowFingerprints(src map[flowFingerprintKey]*endpointFingerprintAccumulator, totalTicks int) []flowFingerprint {
	out := make([]flowFingerprint, 0, len(src))
	for _, item := range src {
		fp := flowFingerprint{
			Network:             item.Key.Network,
			Direction:           item.Direction,
			LocalAddress:        item.Key.LocalAddress,
			LocalPort:           item.Key.LocalPort,
			RemoteAddress:       item.Key.RemoteAddress,
			RemotePort:          item.Key.RemotePort,
			Hits:                item.Hits,
			ActiveTicks:         item.ActiveTicks,
			TickFrequency:       tickFrequency(item.ActiveTicks, totalTicks),
			FirstTick:           item.FirstTick,
			LastTick:            item.LastTick,
			MaxConsecutiveTicks: item.MaxConsecutiveTicks,
			BurstCount:          item.BurstCount,
			FirstState:          item.FirstState,
			LastState:           item.LastState,
			States:              item.States,
		}
		if item.FirstEstablishedTick > 0 && item.FirstTick > 0 {
			fp.HandshakeTicks = item.FirstEstablishedTick - item.FirstTick
			fp.HandshakeNote = "estimated_from_tcp_state_snapshots"
		}
		out = append(out, fp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hits != out[j].Hits {
			return out[i].Hits > out[j].Hits
		}
		if out[i].Network != out[j].Network {
			return out[i].Network < out[j].Network
		}
		if out[i].LocalPort != out[j].LocalPort {
			return out[i].LocalPort < out[j].LocalPort
		}
		return out[i].RemotePort < out[j].RemotePort
	})
	if len(out) > 16 {
		out = out[:16]
	}
	return out
}

func buildPortFingerprints(src map[portFingerprintKey]*portFingerprintAccumulator, totalTicks int) []portFingerprint {
	out := make([]portFingerprint, 0, len(src))
	for _, item := range src {
		out = append(out, portFingerprint{
			Network:             item.Key.Network,
			Direction:           item.Key.Direction,
			Port:                item.Key.Port,
			Hits:                item.Hits,
			ActiveTicks:         item.ActiveTicks,
			TickFrequency:       tickFrequency(item.ActiveTicks, totalTicks),
			FirstTick:           item.FirstTick,
			LastTick:            item.LastTick,
			MaxConsecutiveTicks: item.MaxConsecutiveTicks,
			BurstCount:          item.BurstCount,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hits != out[j].Hits {
			return out[i].Hits > out[j].Hits
		}
		if out[i].Network != out[j].Network {
			return out[i].Network < out[j].Network
		}
		if out[i].Direction != out[j].Direction {
			return out[i].Direction < out[j].Direction
		}
		return out[i].Port < out[j].Port
	})
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}

func aggregatePhasePacketFingerprints(phases []capturePhaseSummary) *packetFingerprintReport {
	totalTicks := 0
	observedEndpoints := 0
	ports := map[portFingerprintKey]*portFingerprintAccumulator{}
	flows := map[flowFingerprintKey]*endpointFingerprintAccumulator{}
	for _, phase := range phases {
		if phase.PacketFingerprint == nil {
			totalTicks += phase.Ticks
			continue
		}
		totalTicks += phase.PacketFingerprint.TotalTicks
		observedEndpoints += phase.PacketFingerprint.ObservedEndpointCount
		for _, fp := range phase.PacketFingerprint.PortFingerprints {
			key := portFingerprintKey{Network: fp.Network, Direction: fp.Direction, Port: fp.Port}
			cur := ports[key]
			if cur == nil {
				cur = &portFingerprintAccumulator{Key: key}
				ports[key] = cur
			}
			cur.Hits += fp.Hits
			cur.ActiveTicks += fp.ActiveTicks
			cur.MaxConsecutiveTicks = maxInt(cur.MaxConsecutiveTicks, fp.MaxConsecutiveTicks)
			cur.BurstCount += fp.BurstCount
			if cur.FirstTick == 0 || fp.FirstTick < cur.FirstTick {
				cur.FirstTick = fp.FirstTick
			}
			cur.LastTick = maxInt(cur.LastTick, fp.LastTick)
		}
		for _, fp := range phase.PacketFingerprint.TopFlows {
			key := flowFingerprintKey{
				Network:       fp.Network,
				LocalAddress:  fp.LocalAddress,
				LocalPort:     fp.LocalPort,
				RemoteAddress: fp.RemoteAddress,
				RemotePort:    fp.RemotePort,
			}
			cur := flows[key]
			if cur == nil {
				cur = &endpointFingerprintAccumulator{Key: key, Direction: fp.Direction, States: map[string]int{}}
				flows[key] = cur
			}
			cur.Hits += fp.Hits
			cur.ActiveTicks += fp.ActiveTicks
			cur.MaxConsecutiveTicks = maxInt(cur.MaxConsecutiveTicks, fp.MaxConsecutiveTicks)
			cur.BurstCount += fp.BurstCount
			if cur.FirstTick == 0 || fp.FirstTick < cur.FirstTick {
				cur.FirstTick = fp.FirstTick
			}
			cur.LastTick = maxInt(cur.LastTick, fp.LastTick)
			if cur.FirstState == "" {
				cur.FirstState = fp.FirstState
			}
			cur.LastState = fp.LastState
			for state, hits := range fp.States {
				cur.States[state] += hits
			}
		}
	}
	builder := &packetFingerprintBuilder{flows: flows, ports: ports}
	report := builder.report(totalTicks)
	if report != nil {
		report.ObservedEndpointCount = observedEndpoints
	}
	return report
}

func tickFrequency(activeTicks, totalTicks int) float64 {
	if totalTicks <= 0 || activeTicks <= 0 {
		return 0
	}
	return float64(activeTicks) / float64(totalTicks)
}

func inferEndpointDirection(ep endpoint) string {
	remoteAddr := normalizeEndpointAddress(ep.RemoteAddr)
	if ep.RemotePort <= 0 || remoteAddr == "" || remoteAddr == "*" || remoteAddr == "0.0.0.0" || remoteAddr == "::" {
		return "listener"
	}
	if isEphemeralPort(ep.LocalPort) && !isEphemeralPort(ep.RemotePort) {
		return "outbound"
	}
	if !isEphemeralPort(ep.LocalPort) && isEphemeralPort(ep.RemotePort) {
		return "inbound"
	}
	return "connected"
}

func normalizeEndpointAddress(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "*"
	}
	return raw
}

func normalizeEndpointState(raw string) string {
	state := strings.ToUpper(strings.TrimSpace(raw))
	switch state {
	case "", "*":
		return ""
	case "ESTAB":
		return "ESTABLISHED"
	default:
		return state
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
