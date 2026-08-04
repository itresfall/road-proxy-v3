package main

import (
	"encoding/binary"
	"testing"
)

func TestParsePCAPNGAndApplyCapturedPacketData(t *testing.T) {
	rakNetMagic := []byte{0x05, 0x00, 0xFF, 0xFF, 0x00, 0xFE, 0xFE, 0xFE, 0xFE, 0xFD, 0xFD, 0xFD, 0xFD, 0x12, 0x34, 0x56, 0x78}
	largePayload := append(append([]byte(nil), rakNetMagic...), make([]byte, 1201-len(rakNetMagic))...)
	data := buildTestPCAPNG([]testPCAPPacket{
		{timestamp: 1_000_000, frame: buildTestUDPFrame(8766, 50000, largePayload)},
		{timestamp: 1_100_000, frame: buildTestUDPFrame(8766, 50000, make([]byte, 200))},
	})

	packets, err := parsePCAPNG(data)
	if err != nil {
		t.Fatalf("parsePCAPNG: %v", err)
	}
	if len(packets) != 2 {
		t.Fatalf("packet count = %d, want 2", len(packets))
	}
	if packets[0].network != "udp" || packets[0].sourcePort != 8766 || packets[0].payloadBytes != 1201 {
		t.Fatalf("unexpected first packet: %#v", packets[0])
	}

	summary := newCaptureSummary(42, "game.exe", 10)
	summary.LocalPortHits["udp"][8766] = 3
	summary.PacketFingerprint = &packetFingerprintReport{Source: "socket_snapshot"}
	matched := filterCapturedPacketsForSummary(packets, summary)
	if len(matched) != 2 {
		t.Fatalf("matched packets = %d, want 2", len(matched))
	}
	applyCapturedPacketData(summary, &advancedCaptureReport{Backend: "pktmon", Status: "captured"}, matched)

	if !summary.PacketFingerprint.PacketSizeObserved {
		t.Fatal("expected real packet size observation")
	}
	stats := summary.PacketFingerprint.PacketSize
	if stats == nil || stats.Packets != 2 || stats.MaxBytes != 1201 || stats.P95Bytes != 1201 || stats.Over1200Bytes != 1 {
		t.Fatalf("unexpected packet size stats: %#v", stats)
	}
	if summary.RecommendedNet != "udp" || summary.RecommendedPort != 8766 {
		t.Fatalf("advanced capture recommendation = %s/%d, want udp/8766", summary.RecommendedNet, summary.RecommendedPort)
	}
	if len(summary.PacketFingerprint.LibrarySignals) != 1 || summary.PacketFingerprint.LibrarySignals[0].Library != "raknet_or_slikenet" {
		t.Fatalf("expected RakNet signal, got %#v", summary.PacketFingerprint.LibrarySignals)
	}
}

func TestNormalizeAdvancedCaptureMode(t *testing.T) {
	for _, raw := range []string{"", "auto", "off", "required", " AUTO "} {
		if _, err := normalizeAdvancedCaptureMode(raw); err != nil {
			t.Fatalf("normalizeAdvancedCaptureMode(%q): %v", raw, err)
		}
	}
	if _, err := normalizeAdvancedCaptureMode("always"); err == nil {
		t.Fatal("expected unsupported advanced capture mode to fail")
	}
}

type testPCAPPacket struct {
	timestamp uint64
	frame     []byte
}

func buildTestPCAPNG(packets []testPCAPPacket) []byte {
	data := make([]byte, 0)
	sectionBody := make([]byte, 16)
	binary.LittleEndian.PutUint32(sectionBody[0:4], 0x1A2B3C4D)
	binary.LittleEndian.PutUint16(sectionBody[4:6], 1)
	binary.LittleEndian.PutUint16(sectionBody[6:8], 0)
	for i := 8; i < 16; i++ {
		sectionBody[i] = 0xFF
	}
	data = append(data, buildTestPCAPNGBlock(pcapngSectionHeaderBlock, sectionBody)...)

	interfaceBody := make([]byte, 8)
	binary.LittleEndian.PutUint16(interfaceBody[0:2], pcapngLinkTypeEthernet)
	binary.LittleEndian.PutUint32(interfaceBody[4:8], 65535)
	data = append(data, buildTestPCAPNGBlock(pcapngInterfaceDescBlock, interfaceBody)...)

	for _, packet := range packets {
		payload := append([]byte(nil), packet.frame...)
		body := make([]byte, 20+paddedLength(len(payload)))
		binary.LittleEndian.PutUint32(body[0:4], 0)
		binary.LittleEndian.PutUint32(body[4:8], uint32(packet.timestamp>>32))
		binary.LittleEndian.PutUint32(body[8:12], uint32(packet.timestamp))
		binary.LittleEndian.PutUint32(body[12:16], uint32(len(payload)))
		binary.LittleEndian.PutUint32(body[16:20], uint32(len(payload)))
		copy(body[20:], payload)
		data = append(data, buildTestPCAPNGBlock(pcapngEnhancedPacketBlock, body)...)
	}
	return data
}

func buildTestPCAPNGBlock(blockType uint32, body []byte) []byte {
	length := 12 + len(body)
	block := make([]byte, length)
	binary.LittleEndian.PutUint32(block[0:4], blockType)
	binary.LittleEndian.PutUint32(block[4:8], uint32(length))
	copy(block[8:], body)
	binary.LittleEndian.PutUint32(block[length-4:], uint32(length))
	return block
}

func buildTestUDPFrame(sourcePort, destPort int, payload []byte) []byte {
	ipLength := 20 + 8 + len(payload)
	frame := make([]byte, 14+ipLength)
	copy(frame[0:6], []byte{0x10, 0x11, 0x12, 0x13, 0x14, 0x15})
	copy(frame[6:12], []byte{0x20, 0x21, 0x22, 0x23, 0x24, 0x25})
	binary.BigEndian.PutUint16(frame[12:14], 0x0800)
	ip := frame[14:]
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], uint16(ipLength))
	ip[8] = 64
	ip[9] = 17
	copy(ip[12:16], []byte{192, 168, 1, 10})
	copy(ip[16:20], []byte{192, 168, 1, 20})
	udp := ip[20:]
	binary.BigEndian.PutUint16(udp[0:2], uint16(sourcePort))
	binary.BigEndian.PutUint16(udp[2:4], uint16(destPort))
	binary.BigEndian.PutUint16(udp[4:6], uint16(8+len(payload)))
	copy(udp[8:], payload)
	return frame
}
