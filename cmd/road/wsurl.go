package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"road-proxy-v3/internal/config"
)

func normalizeClientWSURL(input string) (string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", fmt.Errorf(msg("ws.connection_empty"))
	}

	candidate := raw
	if !strings.Contains(raw, "://") {
		probe, err := url.Parse("ws://" + raw)
		if err != nil || strings.TrimSpace(probe.Host) == "" {
			return "", fmt.Errorf(msg("ws.invalid_connection"), input)
		}
		scheme := "wss"
		if isLocalHostOrLAN(probe.Hostname()) {
			scheme = "ws"
		}
		candidate = scheme + "://" + raw
	}

	u, err := url.Parse(candidate)
	if err != nil || strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf(msg("ws.invalid_connection"), input)
	}

	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf(msg("ws.unsupported_protocol"), u.Scheme)
	}

	if u.Path == "" || u.Path == "/" {
		u.Path = "/ws"
	}

	return u.String(), nil
}

func isLocalHostOrLAN(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	if h == "" {
		return false
	}
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

type serverPluginProfile struct {
	Name          string
	TargetNetwork string
}

func getServerDefaultPluginProfile(cfg *config.ClientConfig) (serverPluginProfile, error) {
	base, err := apiBaseFromWSURL(cfg.ServerWSURL)
	if err != nil {
		return serverPluginProfile{}, err
	}

	type infoResp struct {
		DefaultPlugin  string `json:"default_plugin"`
		DefaultNetwork string `json:"default_network"`
	}
	type pluginsResp struct {
		Default struct {
			Name          string `json:"name"`
			TargetNetwork string `json:"target_network"`
		} `json:"default"`
	}
	type healthResp struct {
		DefaultPlugin struct {
			Name          string `json:"name"`
			TargetNetwork string `json:"target_network"`
		} `json:"default_plugin"`
	}

	httpClient := &http.Client{Timeout: 6 * time.Second}
	headers := clientProfileFetchHeaders(cfg)

	var info infoResp
	if err := fetchJSON(httpClient, base+"/api/info", headers, &info); err == nil {
		if strings.TrimSpace(info.DefaultPlugin) != "" {
			return serverPluginProfile{
				Name:          info.DefaultPlugin,
				TargetNetwork: info.DefaultNetwork,
			}, nil
		}
	}

	var p pluginsResp
	if err := fetchJSON(httpClient, base+"/api/plugins", headers, &p); err == nil {
		if strings.TrimSpace(p.Default.Name) != "" {
			return serverPluginProfile{
				Name:          p.Default.Name,
				TargetNetwork: p.Default.TargetNetwork,
			}, nil
		}
	}

	var h healthResp
	if err := fetchJSON(httpClient, base+"/api/health", headers, &h); err == nil {
		if strings.TrimSpace(h.DefaultPlugin.Name) != "" {
			return serverPluginProfile{
				Name:          h.DefaultPlugin.Name,
				TargetNetwork: h.DefaultPlugin.TargetNetwork,
			}, nil
		}
	}

	return serverPluginProfile{}, fmt.Errorf(msg("ws.default_plugin_not_found"))
}

func apiBaseFromWSURL(wsURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(wsURL))
	if err != nil || strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf(msg("ws.server_url_invalid"), wsURL)
	}

	switch strings.ToLower(u.Scheme) {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	case "http", "https":
	default:
		return "", fmt.Errorf(msg("ws.server_url_scheme_unsupported"), u.Scheme)
	}

	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func clientProfileFetchHeaders(cfg *config.ClientConfig) http.Header {
	headers := make(http.Header)
	for key, value := range cfg.Headers {
		headers.Set(key, value)
	}

	authToken := config.ResolveSecret(cfg.AuthToken)
	if authToken != "" {
		authHeader := strings.TrimSpace(cfg.AuthHeader)
		if authHeader == "" {
			authHeader = config.DefaultAuthHeaderName
		}
		headers.Set(authHeader, config.AuthHeaderValue(authHeader, authToken))
	}
	return headers
}

func fetchJSON(httpClient *http.Client, endpoint string, headers http.Header, out any) error {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
