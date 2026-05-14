package engine

import (
	"net"
	"testing"

	"road-proxy-v3/internal/plugin"
)

func TestUDPReplyAllowedStrictRequiresSameIPAndPort(t *testing.T) {
	target := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7777}

	if !udpReplyAllowed(plugin.UDPReplyPolicyStrict, target, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7777}) {
		t.Fatal("strict should accept exact target source")
	}
	if udpReplyAllowed(plugin.UDPReplyPolicyStrict, target, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7778}) {
		t.Fatal("strict should reject alternate source port")
	}
	if udpReplyAllowed(plugin.UDPReplyPolicyStrict, target, &net.UDPAddr{IP: net.ParseIP("127.0.0.2"), Port: 7777}) {
		t.Fatal("strict should reject alternate source IP")
	}
}

func TestUDPReplyAllowedSameIPAcceptsAlternatePort(t *testing.T) {
	target := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7777}

	if !udpReplyAllowed(plugin.UDPReplyPolicySameIP, target, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7778}) {
		t.Fatal("same_ip should accept alternate source port on same IP")
	}
	if udpReplyAllowed(plugin.UDPReplyPolicySameIP, target, &net.UDPAddr{IP: net.ParseIP("127.0.0.2"), Port: 7778}) {
		t.Fatal("same_ip should reject alternate source IP")
	}
}

func TestUDPReplyAllowedAnyAcceptsAlternateIPAndPort(t *testing.T) {
	target := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7777}
	source := &net.UDPAddr{IP: net.ParseIP("127.0.0.2"), Port: 7778}

	if !udpReplyAllowed(plugin.UDPReplyPolicyAny, target, source) {
		t.Fatal("any should accept alternate source IP and port")
	}
}
