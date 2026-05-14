package logging

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	FormatText = "text"
	FormatJSON = "json"
)

func New(format, component string) *log.Logger {
	return NewWithWriter(os.Stderr, format, component)
}

func NewWithWriter(out io.Writer, format, component string) *log.Logger {
	if normalizeFormat(format) == FormatJSON {
		return log.New(&jsonWriter{out: out, component: strings.TrimSpace(component)}, "", 0)
	}
	return log.New(out, "", log.LstdFlags)
}

func normalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatJSON:
		return FormatJSON
	default:
		return FormatText
	}
}

type jsonWriter struct {
	out       io.Writer
	component string
	mu        sync.Mutex
}

func (w *jsonWriter) Write(p []byte) (int, error) {
	message := strings.TrimRight(string(p), "\r\n")
	lines := strings.Split(message, "\n")

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		record := map[string]string{
			"ts":    time.Now().UTC().Format(time.RFC3339Nano),
			"level": "info",
			"msg":   line,
		}
		if w.component != "" {
			record["component"] = w.component
		}
		data, err := json.Marshal(record)
		if err != nil {
			return 0, err
		}
		if _, err := w.out.Write(append(data, '\n')); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}
