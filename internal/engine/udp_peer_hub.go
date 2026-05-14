package engine

import "sync"

type udpPeerHub struct {
	mu       sync.RWMutex
	sessions map[string]map[*udpPeerSession]struct{}
}

type udpPeerSession struct {
	send func([]byte) error
}

func newUDPPeerHub() *udpPeerHub {
	return &udpPeerHub{
		sessions: map[string]map[*udpPeerSession]struct{}{},
	}
}

func (h *udpPeerHub) register(pluginName string, session *udpPeerSession) func() {
	h.mu.Lock()
	if h.sessions[pluginName] == nil {
		h.sessions[pluginName] = map[*udpPeerSession]struct{}{}
	}
	h.sessions[pluginName][session] = struct{}{}
	h.mu.Unlock()

	return func() {
		h.mu.Lock()
		delete(h.sessions[pluginName], session)
		if len(h.sessions[pluginName]) == 0 {
			delete(h.sessions, pluginName)
		}
		h.mu.Unlock()
	}
}

func (h *udpPeerHub) broadcast(pluginName string, from *udpPeerSession, payload []byte) int {
	h.mu.RLock()
	peers := make([]*udpPeerSession, 0, len(h.sessions[pluginName]))
	for session := range h.sessions[pluginName] {
		if session != from {
			peers = append(peers, session)
		}
	}
	h.mu.RUnlock()

	sent := 0
	for _, peer := range peers {
		if err := peer.send(append([]byte(nil), payload...)); err == nil {
			sent++
		}
	}
	return sent
}
