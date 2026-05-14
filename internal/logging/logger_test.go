package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONLoggerWritesStructuredLine(t *testing.T) {
	var out bytes.Buffer
	logger := NewWithWriter(&out, FormatJSON, "test-component")

	logger.Printf("hello %s", "road")

	var record map[string]string
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &record); err != nil {
		t.Fatalf("decode log JSON: %v; raw=%q", err, out.String())
	}
	if record["component"] != "test-component" {
		t.Fatalf("component = %q", record["component"])
	}
	if record["level"] != "info" {
		t.Fatalf("level = %q", record["level"])
	}
	if record["msg"] != "hello road" {
		t.Fatalf("msg = %q", record["msg"])
	}
	if record["ts"] == "" {
		t.Fatal("ts is empty")
	}
}

func TestTextLoggerKeepsPlainOutput(t *testing.T) {
	var out bytes.Buffer
	logger := NewWithWriter(&out, FormatText, "test-component")

	logger.Print("plain")

	if !strings.Contains(out.String(), "plain") {
		t.Fatalf("text log missing message: %q", out.String())
	}
	if strings.Contains(out.String(), `"msg"`) {
		t.Fatalf("text log should not be JSON: %q", out.String())
	}
}
