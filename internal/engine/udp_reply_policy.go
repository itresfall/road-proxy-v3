package engine

import (
	"net"

	"road-proxy-v3/internal/plugin"
)

func udpReplyAllowed(policy string, targetAddr, sourceAddr net.Addr) bool {
	switch policy {
	case plugin.UDPReplyPolicyStrict:
		return sameUDPIPAndPort(targetAddr, sourceAddr)
	case plugin.UDPReplyPolicySameIP:
		return sameUDPIP(targetAddr, sourceAddr)
	case plugin.UDPReplyPolicyAny, "":
		return true
	default:
		return true
	}
}

func sameUDPIPAndPort(a, b net.Addr) bool {
	ua, okA := a.(*net.UDPAddr)
	ub, okB := b.(*net.UDPAddr)
	if !okA || !okB {
		return a != nil && b != nil && a.String() == b.String()
	}
	return ua.Port == ub.Port && ua.IP.Equal(ub.IP)
}

func sameUDPIP(a, b net.Addr) bool {
	ua, okA := a.(*net.UDPAddr)
	ub, okB := b.(*net.UDPAddr)
	if !okA || !okB {
		return sameUDPIPAndPort(a, b)
	}
	return ua.IP.Equal(ub.IP)
}
