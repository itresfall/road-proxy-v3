package voice

import (
	"errors"
	"sync"
)

var ErrRoomFull = errors.New("voice room is full")

type Room struct {
	name       string
	maxClients int

	mu      sync.RWMutex
	clients map[string]*Client
}

func NewRoom(name string, maxClients int) *Room {
	if maxClients <= 0 {
		maxClients = defaultMaxClients
	}
	return &Room{
		name:       name,
		maxClients: maxClients,
		clients:    make(map[string]*Client),
	}
}

func (r *Room) Add(c *Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.clients) >= r.maxClients {
		return ErrRoomFull
	}
	r.clients[c.id] = c
	return nil
}

func (r *Room) Remove(id string) {
	r.mu.Lock()
	delete(r.clients, id)
	r.mu.Unlock()
}

func (r *Room) Snapshot() []User {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]User, 0, len(r.clients))
	for _, c := range r.clients {
		users = append(users, c.User())
	}
	return users
}

func (r *Room) BroadcastUsers() {
	msg := ControlMessage{
		Type:  "users",
		Users: r.Snapshot(),
	}
	r.broadcastJSON(msg)
}

func (r *Room) BroadcastAudio(senderID string, payload []byte) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	delivered := 0
	for id, c := range r.clients {
		if id == senderID || c.IsDeafened() {
			continue
		}
		if c.Enqueue(webSocketBinaryMessage, payload) {
			delivered++
		}
	}
	return delivered
}

func (r *Room) broadcastJSON(msg ControlMessage) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, c := range r.clients {
		c.EnqueueJSON(msg)
	}
}
