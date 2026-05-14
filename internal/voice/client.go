package voice

import (
	"encoding/json"
	"sync"
)

const (
	webSocketTextMessage   = 1
	webSocketBinaryMessage = 2
)

type outboundMessage struct {
	messageType int
	payload     []byte
}

type Client struct {
	id   string
	name string
	send chan outboundMessage

	mu       sync.RWMutex
	muted    bool
	deafened bool
}

func NewClient(id, name string, sendBuffer int) *Client {
	if sendBuffer <= 0 {
		sendBuffer = 64
	}
	return &Client{
		id:   id,
		name: name,
		send: make(chan outboundMessage, sendBuffer),
	}
}

func (c *Client) User() User {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return User{
		ID:       c.id,
		Name:     c.name,
		Muted:    c.muted,
		Deafened: c.deafened,
	}
}

func (c *Client) SetState(muted, deafened *bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if muted != nil {
		c.muted = *muted
	}
	if deafened != nil {
		c.deafened = *deafened
	}
}

func (c *Client) IsMuted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.muted
}

func (c *Client) IsDeafened() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.deafened
}

func (c *Client) EnqueueJSON(msg ControlMessage) bool {
	payload, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	return c.Enqueue(webSocketTextMessage, payload)
}

func (c *Client) Enqueue(messageType int, payload []byte) bool {
	select {
	case c.send <- outboundMessage{messageType: messageType, payload: payload}:
		return true
	default:
		// Realtime voice should drop late frames instead of blocking the room.
		return false
	}
}

func (c *Client) CloseSend() {
	close(c.send)
}
