package udprecord

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecorderWritesMetadataOnlyEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "udp.jsonl")
	recorder, err := New(Options{
		Enabled:   true,
		Path:      path,
		Role:      "client",
		Duration:  time.Minute,
		MaxEvents: 2,
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	recorder.RecordPacket("local_to_websocket", "game", "s-1", "127.0.0.1:1", "server", []byte{0x80, 1, 0, 0, 9})
	recorder.RecordPacket("websocket_to_local", "game", "s-1", "server", "127.0.0.1:1", []byte{1, 2, 3})
	recorder.RecordPacket("ignored", "game", "s-1", "a", "b", []byte{1})
	if err := recorder.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	events := readJSONLines(t, path)
	if len(events) != 3 {
		t.Fatalf("expected metadata plus two packet events, got %d lines: %#v", len(events), events)
	}
	if events[0]["type"] != "udp_record_start" {
		t.Fatalf("unexpected metadata event: %#v", events[0])
	}
	firstPacket := events[1]
	if firstPacket["type"] != "udp_packet" {
		t.Fatalf("unexpected packet event: %#v", firstPacket)
	}
	if firstPacket["size_bytes"].(float64) != 5 {
		t.Fatalf("unexpected packet size: %#v", firstPacket["size_bytes"])
	}
	if firstPacket["raknet_sequence"].(float64) != 1 {
		t.Fatalf("unexpected raknet sequence: %#v", firstPacket["raknet_sequence"])
	}
	if _, ok := firstPacket["payload"]; ok {
		t.Fatal("recorder must not write UDP payload bytes")
	}
}

func TestRecorderDisabledReturnsNil(t *testing.T) {
	recorder, err := New(Options{Enabled: false}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if recorder != nil {
		t.Fatalf("expected nil disabled recorder, got %#v", recorder)
	}
}

func readJSONLines(t *testing.T, path string) []map[string]any {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open jsonl failed: %v", err)
	}
	defer file.Close()

	var events []map[string]any
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("decode json line failed: %v", err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan jsonl failed: %v", err)
	}
	return events
}
