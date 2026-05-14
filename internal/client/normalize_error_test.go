package client

import (
	"testing"

	"github.com/gorilla/websocket"
)

func TestNormalizeProxyErrorIgnoresAbnormalClosureUnexpectedEOF(t *testing.T) {
	err := normalizeProxyError(&websocket.CloseError{
		Code: websocket.CloseAbnormalClosure,
		Text: "unexpected EOF",
	})
	if err != nil {
		t.Fatalf("expected nil for abnormal closure unexpected EOF, got: %v", err)
	}
}

func TestNormalizeProxyErrorKeepsAbnormalClosureOtherErrors(t *testing.T) {
	input := &websocket.CloseError{
		Code: websocket.CloseAbnormalClosure,
		Text: "tls handshake failed",
	}
	if err := normalizeProxyError(input); err == nil {
		t.Fatal("expected abnormal closure with non-EOF text to be preserved")
	}
}
