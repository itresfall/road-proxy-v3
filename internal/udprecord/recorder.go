package udprecord

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"road-proxy-v3/internal/udputil"
)

type Options struct {
	Enabled   bool
	Path      string
	Role      string
	Duration  time.Duration
	MaxEvents int
}

type Event struct {
	Type           string  `json:"type"`
	Time           string  `json:"time"`
	UnixNano       int64   `json:"unix_nano"`
	Role           string  `json:"role"`
	Direction      string  `json:"direction"`
	Plugin         string  `json:"plugin,omitempty"`
	SessionID      string  `json:"session_id,omitempty"`
	Source         string  `json:"source,omitempty"`
	Destination    string  `json:"destination,omitempty"`
	SizeBytes      int     `json:"size_bytes"`
	RakNetSequence *uint32 `json:"raknet_sequence,omitempty"`
}

type metadataEvent struct {
	Type            string `json:"type"`
	Time            string `json:"time"`
	UnixNano        int64  `json:"unix_nano"`
	Role            string `json:"role"`
	DurationSeconds int64  `json:"duration_seconds"`
	MaxEvents       int    `json:"max_events"`
}

type Recorder struct {
	mu        sync.Mutex
	file      *os.File
	encoder   *json.Encoder
	logger    *log.Logger
	role      string
	startedAt time.Time
	stopAt    time.Time
	maxEvents int
	count     int
	closed    bool
	stopOnce  sync.Once
}

func New(opts Options, logger *log.Logger) (*Recorder, error) {
	if !opts.Enabled {
		return nil, nil
	}
	if opts.Path == "" {
		return nil, fmt.Errorf("udp recorder path is required")
	}
	if opts.MaxEvents < 0 {
		return nil, fmt.Errorf("udp recorder max events must be >= 0")
	}
	if opts.Duration < 0 {
		return nil, fmt.Errorf("udp recorder duration must be >= 0")
	}
	if logger == nil {
		logger = log.Default()
	}
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o755); err != nil {
		return nil, fmt.Errorf("create udp recorder directory: %w", err)
	}

	file, err := os.Create(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("create udp recorder file %q: %w", opts.Path, err)
	}

	now := time.Now()
	recorder := &Recorder{
		file:      file,
		encoder:   json.NewEncoder(file),
		logger:    logger,
		role:      opts.Role,
		startedAt: now,
		maxEvents: opts.MaxEvents,
	}
	if opts.Duration > 0 {
		recorder.stopAt = now.Add(opts.Duration)
	}

	if err := recorder.encoder.Encode(metadataEvent{
		Type:            "udp_record_start",
		Time:            now.UTC().Format(time.RFC3339Nano),
		UnixNano:        now.UnixNano(),
		Role:            recorder.role,
		DurationSeconds: int64(opts.Duration.Seconds()),
		MaxEvents:       opts.MaxEvents,
	}); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write udp recorder metadata: %w", err)
	}

	logger.Printf("udp recorder enabled: path=%s duration=%s max_events=%d", opts.Path, opts.Duration, opts.MaxEvents)
	return recorder, nil
}

func (r *Recorder) RecordPacket(direction, pluginName, sessionID, source, destination string, payload []byte) {
	if r == nil {
		return
	}
	now := time.Now()

	event := Event{
		Type:        "udp_packet",
		Time:        now.UTC().Format(time.RFC3339Nano),
		UnixNano:    now.UnixNano(),
		Role:        r.role,
		Direction:   direction,
		Plugin:      pluginName,
		SessionID:   sessionID,
		Source:      source,
		Destination: destination,
		SizeBytes:   len(payload),
	}
	if seq, ok := udputil.ParseRakNetSequence(payload); ok {
		event.RakNetSequence = &seq
	}
	r.record(event, now)
}

func (r *Recorder) Close() error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	return r.file.Close()
}

func (r *Recorder) record(event Event, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed || r.expiredLocked(now) {
		return
	}
	if err := r.encoder.Encode(event); err != nil {
		r.logger.Printf("udp recorder write failed: %v", err)
	}
	r.count++
}

func (r *Recorder) expiredLocked(now time.Time) bool {
	if !r.stopAt.IsZero() && now.After(r.stopAt) {
		r.logStoppedOnce("duration reached")
		return true
	}
	if r.maxEvents > 0 && r.count >= r.maxEvents {
		r.logStoppedOnce("max events reached")
		return true
	}
	return false
}

func (r *Recorder) logStoppedOnce(reason string) {
	r.stopOnce.Do(func() {
		r.logger.Printf("udp recorder stopped recording: %s", reason)
	})
}
