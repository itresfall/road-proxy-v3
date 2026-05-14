package udputil

import (
	"fmt"
	"net"
)

const (
	ConservativeMTUPayloadBytes = 1200
	TunnelHOLRiskPayloadBytes   = 1400
	IPv4UDPFragmentPayloadBytes = 1472
)

func ReadBufferSize(maxPayloadBytes int) int {
	if maxPayloadBytes < 1 {
		return 2
	}
	return maxPayloadBytes + 1
}

func ReadDatagram(conn net.PacketConn, buffer []byte, maxPayloadBytes int) (int, net.Addr, bool, error) {
	if maxPayloadBytes < 1 {
		return 0, nil, false, fmt.Errorf("max payload bytes must be positive")
	}
	if len(buffer) < ReadBufferSize(maxPayloadBytes) {
		return 0, nil, false, fmt.Errorf("udp read buffer too small: got=%d need=%d", len(buffer), ReadBufferSize(maxPayloadBytes))
	}

	n, addr, err := conn.ReadFrom(buffer[:ReadBufferSize(maxPayloadBytes)])
	return n, addr, n > maxPayloadBytes, err
}

func AboveConservativeMTU(payloadBytes int) bool {
	return payloadBytes > ConservativeMTUPayloadBytes
}

func AboveTunnelHOLRisk(payloadBytes int) bool {
	return payloadBytes > TunnelHOLRiskPayloadBytes
}

func AboveIPv4UDPFragmentRisk(payloadBytes int) bool {
	return payloadBytes > IPv4UDPFragmentPayloadBytes
}
