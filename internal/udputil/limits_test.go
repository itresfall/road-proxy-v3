package udputil

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestReadDatagramDetectsOversizedPacket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	sender, err := net.Dial("udp", conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	defer sender.Close()

	const maxPayload = 4
	if _, err := sender.Write([]byte("12345")); err != nil {
		t.Fatalf("write udp: %v", err)
	}

	buffer := make([]byte, ReadBufferSize(maxPayload))
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	n, _, oversized, err := ReadDatagram(conn, buffer, maxPayload)
	if err != nil {
		t.Fatalf("read datagram: %v", err)
	}
	if !oversized {
		t.Fatal("expected oversized datagram to be detected")
	}
	if n != maxPayload+1 {
		t.Fatalf("read size = %d, want %d", n, maxPayload+1)
	}
}

func TestReadDatagramAcceptsPacketAtLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	sender, err := net.Dial("udp", conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	defer sender.Close()

	const maxPayload = 4
	if _, err := sender.Write([]byte("1234")); err != nil {
		t.Fatalf("write udp: %v", err)
	}

	buffer := make([]byte, ReadBufferSize(maxPayload))
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	n, _, oversized, err := ReadDatagram(conn, buffer, maxPayload)
	if err != nil {
		t.Fatalf("read datagram: %v", err)
	}
	if oversized {
		t.Fatal("packet at limit should not be oversized")
	}
	if n != maxPayload {
		t.Fatalf("read size = %d, want %d", n, maxPayload)
	}
}

func TestMTURiskThresholds(t *testing.T) {
	if AboveConservativeMTU(ConservativeMTUPayloadBytes) {
		t.Fatal("conservative threshold should be exclusive")
	}
	if !AboveConservativeMTU(ConservativeMTUPayloadBytes + 1) {
		t.Fatal("expected conservative MTU warning above threshold")
	}
	if AboveTunnelHOLRisk(TunnelHOLRiskPayloadBytes) {
		t.Fatal("HOL threshold should be exclusive")
	}
	if !AboveTunnelHOLRisk(TunnelHOLRiskPayloadBytes + 1) {
		t.Fatal("expected HOL warning above threshold")
	}
	if AboveIPv4UDPFragmentRisk(IPv4UDPFragmentPayloadBytes) {
		t.Fatal("IPv4 fragment threshold should be exclusive")
	}
	if !AboveIPv4UDPFragmentRisk(IPv4UDPFragmentPayloadBytes + 1) {
		t.Fatal("expected IPv4 fragment warning above threshold")
	}
}
