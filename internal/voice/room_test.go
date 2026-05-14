package voice

import "testing"

func TestRoomRejectsWhenFull(t *testing.T) {
	room := NewRoom("test", 1)
	if err := room.Add(NewClient("1", "one", 1)); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := room.Add(NewClient("2", "two", 1)); err != ErrRoomFull {
		t.Fatalf("Add() error = %v, want ErrRoomFull", err)
	}
}

func TestRoomBroadcastSkipsSenderAndDeafened(t *testing.T) {
	room := NewRoom("test", 3)
	sender := NewClient("1", "sender", 1)
	listener := NewClient("2", "listener", 1)
	deafened := NewClient("3", "deafened", 1)
	v := true
	deafened.SetState(nil, &v)

	for _, c := range []*Client{sender, listener, deafened} {
		if err := room.Add(c); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	delivered := room.BroadcastAudio(sender.id, []byte("audio"))
	if delivered != 1 {
		t.Fatalf("delivered = %d, want 1", delivered)
	}

	select {
	case <-listener.send:
	default:
		t.Fatal("listener did not receive audio")
	}
	select {
	case <-sender.send:
		t.Fatal("sender received own audio")
	default:
	}
	select {
	case <-deafened.send:
		t.Fatal("deafened client received audio")
	default:
	}
}

func TestClientSetStatePartial(t *testing.T) {
	client := NewClient("1", "tester", 1)
	enabled := true
	disabled := false

	client.SetState(&enabled, nil)
	if !client.IsMuted() || client.IsDeafened() {
		t.Fatalf("unexpected state after mute: %+v", client.User())
	}

	client.SetState(nil, &enabled)
	if !client.IsMuted() || !client.IsDeafened() {
		t.Fatalf("unexpected state after deafen: %+v", client.User())
	}

	client.SetState(&disabled, nil)
	if client.IsMuted() || !client.IsDeafened() {
		t.Fatalf("unexpected state after unmute: %+v", client.User())
	}
}
