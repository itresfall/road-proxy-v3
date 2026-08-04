package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"time"
)

const (
	pcapngSectionHeaderBlock  = 0x0A0D0D0A
	pcapngInterfaceDescBlock  = 0x00000001
	pcapngEnhancedPacketBlock = 0x00000006
	pcapngLinkTypeEthernet    = 1
	pcapngLinkTypeRaw         = 101
)

type pcapngInterface struct {
	linkType       uint16
	timestampUnits float64
}

// capturedPacket is intentionally metadata-only. payloadPrefix is kept only
// long enough to classify well-known handshake markers, then discarded.
type capturedPacket struct {
	timestampNanos int64
	network        string
	sourceAddress  string
	sourcePort     int
	destAddress    string
	destPort       int
	payloadBytes   int
	originalBytes  int
	payloadPrefix  []byte
}

func parsePCAPNGFile(path string) ([]capturedPacket, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parsePCAPNG(data)
}

func parsePCAPNG(data []byte) ([]capturedPacket, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("pcapng file is too small")
	}

	var order binary.ByteOrder
	interfaces := []pcapngInterface{}
	packets := []capturedPacket{}
	for offset := 0; offset < len(data); {
		if len(data)-offset < 12 {
			return nil, fmt.Errorf("truncated pcapng block at byte %d", offset)
		}

		isSectionHeader := binary.LittleEndian.Uint32(data[offset:offset+4]) == pcapngSectionHeaderBlock
		if isSectionHeader {
			orderForSection, err := pcapngSectionByteOrder(data[offset:])
			if err != nil {
				return nil, err
			}
			order = orderForSection
			interfaces = []pcapngInterface{}
		}
		if order == nil {
			return nil, fmt.Errorf("pcapng stream does not start with a section header")
		}

		blockType := order.Uint32(data[offset : offset+4])
		blockLength := int(order.Uint32(data[offset+4 : offset+8]))
		if blockLength < 12 || blockLength > len(data)-offset {
			return nil, fmt.Errorf("invalid pcapng block length %d at byte %d", blockLength, offset)
		}
		if order.Uint32(data[offset+blockLength-4:offset+blockLength]) != uint32(blockLength) {
			return nil, fmt.Errorf("pcapng block length trailer mismatch at byte %d", offset)
		}

		switch blockType {
		case pcapngInterfaceDescBlock:
			iface, err := parsePCAPNGInterface(data[offset:offset+blockLength], order)
			if err != nil {
				return nil, err
			}
			interfaces = append(interfaces, iface)
		case pcapngEnhancedPacketBlock:
			packet, ok, err := parsePCAPNGEnhancedPacket(data[offset:offset+blockLength], order, interfaces)
			if err != nil {
				return nil, err
			}
			if ok {
				packets = append(packets, packet)
			}
		}
		offset += blockLength
	}
	return packets, nil
}

func pcapngSectionByteOrder(block []byte) (binary.ByteOrder, error) {
	if len(block) < 12 {
		return nil, fmt.Errorf("truncated pcapng section header")
	}
	const byteOrderMagic = 0x1A2B3C4D
	if binary.LittleEndian.Uint32(block[8:12]) == byteOrderMagic {
		return binary.LittleEndian, nil
	}
	if binary.BigEndian.Uint32(block[8:12]) == byteOrderMagic {
		return binary.BigEndian, nil
	}
	return nil, fmt.Errorf("unsupported pcapng byte order magic")
}

func parsePCAPNGInterface(block []byte, order binary.ByteOrder) (pcapngInterface, error) {
	if len(block) < 20 {
		return pcapngInterface{}, fmt.Errorf("truncated pcapng interface block")
	}
	iface := pcapngInterface{
		linkType:       order.Uint16(block[8:10]),
		timestampUnits: 1_000_000, // pcapng default: microsecond resolution.
	}
	for offset := 16; offset+4 <= len(block)-4; {
		code := order.Uint16(block[offset : offset+2])
		length := int(order.Uint16(block[offset+2 : offset+4]))
		offset += 4
		if code == 0 {
			break
		}
		if length < 0 || offset+length > len(block)-4 {
			return pcapngInterface{}, fmt.Errorf("invalid pcapng interface option")
		}
		if code == 9 && length == 1 {
			resolution := block[offset]
			if resolution&0x80 == 0 {
				iface.timestampUnits = math.Pow10(int(resolution))
			} else {
				iface.timestampUnits = math.Pow(2, float64(resolution&0x7F))
			}
		}
		offset += paddedLength(length)
	}
	return iface, nil
}

func parsePCAPNGEnhancedPacket(block []byte, order binary.ByteOrder, interfaces []pcapngInterface) (capturedPacket, bool, error) {
	if len(block) < 32 {
		return capturedPacket{}, false, fmt.Errorf("truncated pcapng enhanced packet block")
	}
	interfaceID := int(order.Uint32(block[8:12]))
	if interfaceID < 0 || interfaceID >= len(interfaces) {
		return capturedPacket{}, false, fmt.Errorf("pcapng packet references unknown interface %d", interfaceID)
	}
	capturedLength := int(order.Uint32(block[20:24]))
	originalLength := int(order.Uint32(block[24:28]))
	packetEnd := 28 + capturedLength
	if capturedLength < 0 || packetEnd > len(block)-4 {
		return capturedPacket{}, false, fmt.Errorf("invalid pcapng captured packet length")
	}
	timestamp := uint64(order.Uint32(block[12:16]))<<32 | uint64(order.Uint32(block[16:20]))
	units := interfaces[interfaceID].timestampUnits
	if units <= 0 {
		units = 1_000_000
	}
	timestampNanos := int64(float64(timestamp) * (1_000_000_000 / units))
	return parseCapturedFrame(interfaces[interfaceID].linkType, block[28:packetEnd], originalLength, timestampNanos)
}

func paddedLength(length int) int {
	return (length + 3) &^ 3
}

func parseCapturedFrame(linkType uint16, frame []byte, originalLength int, timestampNanos int64) (capturedPacket, bool, error) {
	offset := 0
	etherType := uint16(0)
	switch linkType {
	case pcapngLinkTypeEthernet:
		if len(frame) < 14 {
			return capturedPacket{}, false, nil
		}
		etherType = binary.BigEndian.Uint16(frame[12:14])
		offset = 14
		for etherType == 0x8100 || etherType == 0x88A8 || etherType == 0x9100 {
			if len(frame) < offset+4 {
				return capturedPacket{}, false, nil
			}
			etherType = binary.BigEndian.Uint16(frame[offset+2 : offset+4])
			offset += 4
		}
	case pcapngLinkTypeRaw:
		if len(frame) == 0 {
			return capturedPacket{}, false, nil
		}
		if frame[0]>>4 == 4 {
			etherType = 0x0800
		} else if frame[0]>>4 == 6 {
			etherType = 0x86DD
		} else {
			return capturedPacket{}, false, nil
		}
	default:
		return capturedPacket{}, false, nil
	}

	switch etherType {
	case 0x0800:
		return parseIPv4CapturedPacket(frame, offset, originalLength, timestampNanos)
	case 0x86DD:
		return parseIPv6CapturedPacket(frame, offset, originalLength, timestampNanos)
	default:
		return capturedPacket{}, false, nil
	}
}

func parseIPv4CapturedPacket(frame []byte, offset, originalLength int, timestampNanos int64) (capturedPacket, bool, error) {
	if len(frame) < offset+20 || frame[offset]>>4 != 4 {
		return capturedPacket{}, false, nil
	}
	headerLength := int(frame[offset]&0x0F) * 4
	if headerLength < 20 || len(frame) < offset+headerLength {
		return capturedPacket{}, false, nil
	}
	fragment := binary.BigEndian.Uint16(frame[offset+6:offset+8]) & 0x1FFF
	if fragment != 0 {
		return capturedPacket{}, false, nil
	}
	totalLength := int(binary.BigEndian.Uint16(frame[offset+2 : offset+4]))
	if totalLength < headerLength {
		return capturedPacket{}, false, nil
	}
	packetEnd := offset + totalLength
	if packetEnd > len(frame) {
		packetEnd = len(frame)
	}
	return parseTransportCapturedPacket(
		frame,
		offset+headerLength,
		packetEnd,
		frame[offset+9],
		net.IP(frame[offset+12:offset+16]).String(),
		net.IP(frame[offset+16:offset+20]).String(),
		totalLength-headerLength,
		originalLength,
		timestampNanos,
	)
}

func parseIPv6CapturedPacket(frame []byte, offset, originalLength int, timestampNanos int64) (capturedPacket, bool, error) {
	if len(frame) < offset+40 || frame[offset]>>4 != 6 {
		return capturedPacket{}, false, nil
	}
	payloadLength := int(binary.BigEndian.Uint16(frame[offset+4 : offset+6]))
	packetEnd := offset + 40 + payloadLength
	if packetEnd > len(frame) {
		packetEnd = len(frame)
	}
	nextHeader := frame[offset+6]
	transportOffset := offset + 40
	for {
		switch nextHeader {
		case 0, 43, 60:
			if transportOffset+2 > packetEnd {
				return capturedPacket{}, false, nil
			}
			nextHeader, transportOffset = frame[transportOffset], transportOffset+(int(frame[transportOffset+1])+1)*8
		case 44:
			if transportOffset+8 > packetEnd {
				return capturedPacket{}, false, nil
			}
			nextHeader, transportOffset = frame[transportOffset], transportOffset+8
		case 51:
			if transportOffset+2 > packetEnd {
				return capturedPacket{}, false, nil
			}
			nextHeader, transportOffset = frame[transportOffset], transportOffset+(int(frame[transportOffset+1])+2)*4
		default:
			transportBytes := 40 + payloadLength - (transportOffset - offset)
			return parseTransportCapturedPacket(
				frame,
				transportOffset,
				packetEnd,
				nextHeader,
				net.IP(frame[offset+8:offset+24]).String(),
				net.IP(frame[offset+24:offset+40]).String(),
				transportBytes,
				originalLength,
				timestampNanos,
			)
		}
		if transportOffset > packetEnd {
			return capturedPacket{}, false, nil
		}
	}
}

func parseTransportCapturedPacket(frame []byte, offset, packetEnd int, protocol byte, sourceAddress, destAddress string, transportBytes, originalLength int, timestampNanos int64) (capturedPacket, bool, error) {
	if offset+8 > packetEnd || offset+8 > len(frame) {
		return capturedPacket{}, false, nil
	}
	packet := capturedPacket{
		timestampNanos: timestampNanos,
		sourceAddress:  sourceAddress,
		destAddress:    destAddress,
		sourcePort:     int(binary.BigEndian.Uint16(frame[offset : offset+2])),
		destPort:       int(binary.BigEndian.Uint16(frame[offset+2 : offset+4])),
		originalBytes:  originalLength,
	}

	payloadOffset := 0
	switch protocol {
	case 17: // UDP
		packet.network = "udp"
		udpLength := int(binary.BigEndian.Uint16(frame[offset+4 : offset+6]))
		if udpLength < 8 {
			return capturedPacket{}, false, nil
		}
		packet.payloadBytes = udpLength - 8
		payloadOffset = offset + 8
	case 6: // TCP
		if offset+20 > packetEnd || offset+20 > len(frame) {
			return capturedPacket{}, false, nil
		}
		packet.network = "tcp"
		tcpHeaderLength := int(frame[offset+12]>>4) * 4
		if tcpHeaderLength < 20 {
			return capturedPacket{}, false, nil
		}
		packet.payloadBytes = transportBytes - tcpHeaderLength
		if packet.payloadBytes < 0 {
			packet.payloadBytes = 0
		}
		payloadOffset = offset + tcpHeaderLength
	default:
		return capturedPacket{}, false, nil
	}

	if packet.payloadBytes > 0 && payloadOffset < len(frame) && payloadOffset < packetEnd {
		available := packetEnd - payloadOffset
		if available > len(frame)-payloadOffset {
			available = len(frame) - payloadOffset
		}
		if available > 32 {
			available = 32
		}
		if available > 0 {
			packet.payloadPrefix = append([]byte(nil), frame[payloadOffset:payloadOffset+available]...)
		}
	}
	return packet, true, nil
}

func filterCapturedPacketsForSummary(packets []capturedPacket, summary *captureSummary) []capturedPacket {
	if summary == nil || len(packets) == 0 {
		return nil
	}
	ports := map[string]map[int]struct{}{}
	for _, source := range []map[string]map[int]int{summary.LocalPortHits, summary.RemotePortHits} {
		for network, hits := range source {
			set := ports[network]
			if set == nil {
				set = map[int]struct{}{}
				ports[network] = set
			}
			for port := range hits {
				if port > 0 {
					set[port] = struct{}{}
				}
			}
		}
	}

	matched := make([]capturedPacket, 0, len(packets))
	for _, packet := range packets {
		if _, ok := ports[packet.network][packet.sourcePort]; ok {
			matched = append(matched, packet)
			continue
		}
		if _, ok := ports[packet.network][packet.destPort]; ok {
			matched = append(matched, packet)
		}
	}
	return matched
}

func applyCapturedPacketData(summary *captureSummary, report *advancedCaptureReport, packets []capturedPacket) {
	if summary == nil {
		return
	}
	summary.AdvancedCapture = report
	if report == nil || report.Status != "captured" || len(packets) == 0 {
		return
	}
	if summary.PacketFingerprint == nil {
		summary.PacketFingerprint = &packetFingerprintReport{}
	}
	fingerprint := summary.PacketFingerprint
	fingerprint.Source = "socket_snapshot+pktmon_pcapng"
	fingerprint.PacketSizeObserved = true
	fingerprint.PacketSizeSource = "pktmon_pcapng"
	fingerprint.PacketSizeNote = "Packet size is the UDP payload or TCP data length. Pktmon raw frames were temporary and deleted after parsing."
	fingerprint.PacketSize = summarizeCapturedPacketSizes(packets)
	fingerprint.PacketTiming = summarizeCapturedPacketTiming(packets)
	fingerprint.CapturedPorts = summarizeCapturedPorts(packets, summary.LocalPortHits)
	fingerprint.LibrarySignals = detectLibrarySignals(packets)
	summary.RecommendedNet, summary.RecommendedPort = recommendNetworkAndPort(summary)
	summary.Topology = inferTopology(summary)
}

func summarizeCapturedPacketSizes(packets []capturedPacket) *packetSizeStats {
	if len(packets) == 0 {
		return nil
	}
	stats := &packetSizeStats{Unit: "transport_payload_bytes", samples: make([]int, 0, len(packets))}
	total := 0
	for _, packet := range packets {
		size := packet.payloadBytes
		if size < 0 {
			continue
		}
		stats.samples = append(stats.samples, size)
		total += size
		if stats.Packets == 0 || size < stats.MinBytes {
			stats.MinBytes = size
		}
		if size > stats.MaxBytes {
			stats.MaxBytes = size
		}
		if size > 1200 {
			stats.Over1200Bytes++
		}
		if size > 1400 {
			stats.Over1400Bytes++
		}
		if size > 1472 {
			stats.Over1472Bytes++
		}
		stats.Packets++
	}
	if stats.Packets == 0 {
		return nil
	}
	stats.AverageBytes = float64(total) / float64(stats.Packets)
	stats.P95Bytes = percentileInt(stats.samples, 0.95)
	return stats
}

func summarizeCapturedPacketTiming(packets []capturedPacket) *packetTimingStats {
	if len(packets) == 0 {
		return nil
	}
	timestamps := make([]int64, 0, len(packets))
	for _, packet := range packets {
		if packet.timestampNanos > 0 {
			timestamps = append(timestamps, packet.timestampNanos)
		}
	}
	if len(timestamps) == 0 {
		return &packetTimingStats{Packets: len(packets)}
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	stats := &packetTimingStats{
		Packets:            len(packets),
		firstTimestampNano: timestamps[0],
		lastTimestampNano:  timestamps[len(timestamps)-1],
	}
	if len(timestamps) < 2 {
		return stats
	}
	stats.DurationMillis = float64(stats.lastTimestampNano-stats.firstTimestampNano) / 1_000_000
	if stats.DurationMillis > 0 {
		stats.PacketsPerSecond = float64(len(timestamps)) / (stats.DurationMillis / 1000)
	}
	for i := 1; i < len(timestamps); i++ {
		gap := float64(timestamps[i]-timestamps[i-1]) / 1_000_000
		if i == 1 || gap < stats.MinGapMillis {
			stats.MinGapMillis = gap
		}
		if gap > stats.MaxGapMillis {
			stats.MaxGapMillis = gap
		}
		stats.gapSamplesMillis = append(stats.gapSamplesMillis, gap)
		stats.AverageGapMillis += gap
	}
	stats.AverageGapMillis /= float64(len(stats.gapSamplesMillis))
	stats.P95GapMillis = percentileFloat(stats.gapSamplesMillis, 0.95)
	return stats
}

func summarizeCapturedPorts(packets []capturedPacket, localHits map[string]map[int]int) []capturedPortStat {
	type accumulator struct {
		capturedPortStat
		timestamps []int64
	}
	items := map[string]*accumulator{}
	add := func(network, scope string, port int, packet capturedPacket) {
		if port <= 0 {
			return
		}
		key := network + ":" + scope + ":" + fmt.Sprint(port)
		item := items[key]
		if item == nil {
			item = &accumulator{capturedPortStat: capturedPortStat{Network: network, Scope: scope, Port: port}}
			items[key] = item
		}
		item.Packets++
		item.PayloadBytes += int64(packet.payloadBytes)
		if packet.timestampNanos > 0 {
			item.timestamps = append(item.timestamps, packet.timestampNanos)
		}
	}
	for _, packet := range packets {
		localPorts := localHits[packet.network]
		_, sourceIsLocal := localPorts[packet.sourcePort]
		_, destIsLocal := localPorts[packet.destPort]
		if sourceIsLocal {
			add(packet.network, "local", packet.sourcePort, packet)
			add(packet.network, "remote", packet.destPort, packet)
		}
		if destIsLocal {
			add(packet.network, "local", packet.destPort, packet)
			add(packet.network, "remote", packet.sourcePort, packet)
		}
	}
	out := make([]capturedPortStat, 0, len(items))
	for _, item := range items {
		if len(item.timestamps) >= 2 {
			sort.Slice(item.timestamps, func(i, j int) bool { return item.timestamps[i] < item.timestamps[j] })
			duration := float64(item.timestamps[len(item.timestamps)-1]-item.timestamps[0]) / 1_000_000_000
			if duration > 0 {
				item.PacketsPerSecond = float64(item.Packets) / duration
			}
			item.BurstCount = 1
			for i := 1; i < len(item.timestamps); i++ {
				if item.timestamps[i]-item.timestamps[i-1] > int64(time.Second) {
					item.BurstCount++
				}
			}
		} else if item.Packets > 0 {
			item.BurstCount = 1
		}
		out = append(out, item.capturedPortStat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Packets != out[j].Packets {
			return out[i].Packets > out[j].Packets
		}
		if out[i].Network != out[j].Network {
			return out[i].Network < out[j].Network
		}
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].Port < out[j].Port
	})
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}

func detectLibrarySignals(packets []capturedPacket) []librarySignal {
	rakNet := 0
	for _, packet := range packets {
		if packet.network == "udp" && hasRakNetOfflineMagic(packet.payloadPrefix) {
			rakNet++
		}
	}
	if rakNet == 0 {
		return nil
	}
	return []librarySignal{{
		Library:    "raknet_or_slikenet",
		Confidence: "high",
		Evidence:   "raknet_offline_magic_after_packet_id",
		Packets:    rakNet,
	}}
}

func aggregatePhaseAdvancedCaptures(phases []capturePhaseSummary) *advancedCaptureReport {
	var result *advancedCaptureReport
	for _, phase := range phases {
		report := phase.AdvancedCapture
		if report == nil {
			continue
		}
		if result == nil {
			copyReport := *report
			result = &copyReport
			continue
		}
		result.CapturedPackets += report.CapturedPackets
		result.MatchedPackets += report.MatchedPackets
		if result.Backend == "" {
			result.Backend = report.Backend
		}
		if report.Status == "captured" {
			result.Status = "captured"
			result.Note = "one or more capture phases used pktmon metadata; raw capture files were deleted"
		} else if result.Status != "captured" && report.Status == "captured_no_matching_packets" {
			result.Status = "captured_no_matching_packets"
		}
	}
	return result
}

func mergePhaseCapturedFingerprintData(report *packetFingerprintReport, phases []capturePhaseSummary) {
	if report == nil {
		return
	}
	sizes := []*packetSizeStats{}
	timings := []*packetTimingStats{}
	ports := []capturedPortStat{}
	signals := []librarySignal{}
	for _, phase := range phases {
		if phase.PacketFingerprint == nil || !phase.PacketFingerprint.PacketSizeObserved {
			continue
		}
		if phase.PacketFingerprint.PacketSize != nil {
			sizes = append(sizes, phase.PacketFingerprint.PacketSize)
		}
		if phase.PacketFingerprint.PacketTiming != nil {
			timings = append(timings, phase.PacketFingerprint.PacketTiming)
		}
		ports = append(ports, phase.PacketFingerprint.CapturedPorts...)
		signals = append(signals, phase.PacketFingerprint.LibrarySignals...)
	}
	if len(sizes) == 0 {
		return
	}
	report.Source = "socket_snapshot+pktmon_pcapng"
	report.PacketSizeObserved = true
	report.PacketSizeSource = "pktmon_pcapng"
	report.PacketSizeNote = "Packet size is the UDP payload or TCP data length. Pktmon raw frames were temporary and deleted after parsing."
	report.PacketSize = mergePacketSizeStats(sizes)
	report.PacketTiming = mergePacketTimingStats(timings)
	report.CapturedPorts = mergeCapturedPortStats(ports)
	report.LibrarySignals = mergeLibrarySignals(signals)
}

func mergePacketSizeStats(items []*packetSizeStats) *packetSizeStats {
	values := []int{}
	for _, item := range items {
		if item == nil {
			continue
		}
		values = append(values, item.samples...)
	}
	packets := make([]capturedPacket, 0, len(values))
	for _, value := range values {
		packets = append(packets, capturedPacket{payloadBytes: value})
	}
	return summarizeCapturedPacketSizes(packets)
}

func mergePacketTimingStats(items []*packetTimingStats) *packetTimingStats {
	if len(items) == 0 {
		return nil
	}
	stats := &packetTimingStats{}
	for _, item := range items {
		if item == nil {
			continue
		}
		stats.Packets += item.Packets
		stats.gapSamplesMillis = append(stats.gapSamplesMillis, item.gapSamplesMillis...)
		if stats.firstTimestampNano == 0 || (item.firstTimestampNano > 0 && item.firstTimestampNano < stats.firstTimestampNano) {
			stats.firstTimestampNano = item.firstTimestampNano
		}
		if item.lastTimestampNano > stats.lastTimestampNano {
			stats.lastTimestampNano = item.lastTimestampNano
		}
	}
	if stats.Packets == 0 {
		return nil
	}
	if stats.lastTimestampNano > stats.firstTimestampNano {
		stats.DurationMillis = float64(stats.lastTimestampNano-stats.firstTimestampNano) / 1_000_000
		stats.PacketsPerSecond = float64(stats.Packets) / (stats.DurationMillis / 1000)
	}
	if len(stats.gapSamplesMillis) == 0 {
		return stats
	}
	for i, gap := range stats.gapSamplesMillis {
		if i == 0 || gap < stats.MinGapMillis {
			stats.MinGapMillis = gap
		}
		if gap > stats.MaxGapMillis {
			stats.MaxGapMillis = gap
		}
		stats.AverageGapMillis += gap
	}
	stats.AverageGapMillis /= float64(len(stats.gapSamplesMillis))
	stats.P95GapMillis = percentileFloat(stats.gapSamplesMillis, 0.95)
	return stats
}

func mergeCapturedPortStats(items []capturedPortStat) []capturedPortStat {
	type accumulator struct {
		capturedPortStat
		rateWeight int
	}
	byKey := map[string]*accumulator{}
	for _, item := range items {
		key := item.Network + ":" + item.Scope + ":" + fmt.Sprint(item.Port)
		cur := byKey[key]
		if cur == nil {
			cur = &accumulator{capturedPortStat: capturedPortStat{Network: item.Network, Scope: item.Scope, Port: item.Port}}
			byKey[key] = cur
		}
		cur.Packets += item.Packets
		cur.PayloadBytes += item.PayloadBytes
		cur.BurstCount += item.BurstCount
		cur.PacketsPerSecond += item.PacketsPerSecond * float64(item.Packets)
		cur.rateWeight += item.Packets
	}
	out := make([]capturedPortStat, 0, len(byKey))
	for _, item := range byKey {
		if item.rateWeight > 0 {
			item.PacketsPerSecond /= float64(item.rateWeight)
		}
		out = append(out, item.capturedPortStat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Packets != out[j].Packets {
			return out[i].Packets > out[j].Packets
		}
		if out[i].Network != out[j].Network {
			return out[i].Network < out[j].Network
		}
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].Port < out[j].Port
	})
	return out
}

func mergeLibrarySignals(items []librarySignal) []librarySignal {
	byKey := map[string]*librarySignal{}
	for _, item := range items {
		key := item.Library + ":" + item.Evidence
		cur := byKey[key]
		if cur == nil {
			copyItem := item
			byKey[key] = &copyItem
			continue
		}
		cur.Packets += item.Packets
	}
	out := make([]librarySignal, 0, len(byKey))
	for _, item := range byKey {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Packets != out[j].Packets {
			return out[i].Packets > out[j].Packets
		}
		return out[i].Library < out[j].Library
	})
	return out
}

func hasRakNetOfflineMagic(payload []byte) bool {
	magic := []byte{0x00, 0xFF, 0xFF, 0x00, 0xFE, 0xFE, 0xFE, 0xFE, 0xFD, 0xFD, 0xFD, 0xFD, 0x12, 0x34, 0x56, 0x78}
	if len(payload) < len(magic)+1 {
		return false
	}
	for i, value := range magic {
		if payload[i+1] != value {
			return false
		}
	}
	return true
}

func percentileInt(values []int, percentile float64) int {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]int(nil), values...)
	sort.Ints(copyValues)
	index := int(math.Ceil(float64(len(copyValues))*percentile)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(copyValues) {
		index = len(copyValues) - 1
	}
	return copyValues[index]
}

func percentileFloat(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	index := int(math.Ceil(float64(len(copyValues))*percentile)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(copyValues) {
		index = len(copyValues) - 1
	}
	return copyValues[index]
}
