package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"road-proxy-v3/internal/config"
)

func TestHandleDashboardReturnsHTML(t *testing.T) {
	e := New(config.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	e.handleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("content-type = %q, want text/html", got)
	}
	body := rec.Body.String()
	for _, want := range []string{"ROAD Control Deck", "/api/stats", "/api/sessions", "/api/ping", "UDP Diagnostics", "Auth header"} {
		if !strings.Contains(body, want) {
			t.Fatalf("dashboard body missing %q", want)
		}
	}
}

func TestHandleDashboardRejectsNonGET(t *testing.T) {
	e := New(config.Default(), nil)

	req := httptest.NewRequest(http.MethodPost, "/dashboard", nil)
	rec := httptest.NewRecorder()
	e.handleDashboard(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: got=%d", rec.Code)
	}
}

func TestHandleRootIncludesDashboardRoute(t *testing.T) {
	e := New(config.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.handleRoot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Routes []string `json:"routes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, route := range body.Routes {
		if route == "/dashboard" {
			return
		}
	}
	t.Fatalf("dashboard route missing: %#v", body.Routes)
}
