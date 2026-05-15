package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"road-proxy-v3/internal/config"
)

const expectedServerServiceName = "road-proxy-v3"

func runClientPreflight(cfg *config.ClientConfig) error {
	base, err := apiBaseFromWSURL(cfg.ServerWSURL)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, base+"/api/info", nil)
	if err != nil {
		return err
	}
	for key, value := range preflightHeaders(cfg) {
		req.Header.Set(key, value)
	}

	resp, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("server preflight failed with HTTP %d: check client auth_token/%s", resp.StatusCode, config.DefaultAuthHeaderName)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server preflight failed with HTTP %d", resp.StatusCode)
	}

	var info struct {
		Service string `json:"service"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return fmt.Errorf("decode server preflight response: %w", err)
	}
	if strings.TrimSpace(info.Service) != expectedServerServiceName {
		return fmt.Errorf("server preflight returned unexpected service %q", strings.TrimSpace(info.Service))
	}
	return nil
}

func preflightHeaders(cfg *config.ClientConfig) map[string]string {
	headers := map[string]string{}
	for key, value := range cfg.Headers {
		headers[key] = value
	}
	authToken := config.ResolveSecret(cfg.AuthToken)
	if authToken == "" {
		return headers
	}
	authHeader := strings.TrimSpace(cfg.AuthHeader)
	if authHeader == "" {
		authHeader = config.DefaultAuthHeaderName
	}
	headers[authHeader] = config.AuthHeaderValue(authHeader, authToken)
	return headers
}

func apiBaseFromWSURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf("server_ws_url is invalid: %s", raw)
	}
	switch strings.ToLower(u.Scheme) {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	default:
		return "", fmt.Errorf("server_ws_url scheme must be ws or wss")
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}
