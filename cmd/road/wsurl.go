package main

import (
	"encoding/json"
	"errors"
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

const expectedServerServiceName = "road-proxy-v3"

type roadServerInfo struct {
	Service        string `json:"service"`
	DefaultPlugin  string `json:"default_plugin"`
	DefaultNetwork string `json:"default_network"`
}

func getServerDefaultPluginProfile(cfg *config.ClientConfig) (serverPluginProfile, error) {
	base, err := apiBaseFromWSURL(cfg.ServerWSURL)
	if err != nil {
		return serverPluginProfile{}, err
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

	info, err := fetchRoadServerInfo(httpClient, base, headers)
	if err != nil {
		return serverPluginProfile{}, err
	}
	if strings.TrimSpace(info.DefaultPlugin) != "" {
		return serverPluginProfile{
			Name:          info.DefaultPlugin,
			TargetNetwork: info.DefaultNetwork,
		}, nil
	}

	var p pluginsResp
	if err := fetchJSON(httpClient, base+"/api/plugins", headers, &p); err == nil {
		if strings.TrimSpace(p.Default.Name) != "" {
			return serverPluginProfile{
				Name:          p.Default.Name,
				TargetNetwork: p.Default.TargetNetwork,
			}, nil
		}
	} else if isAuthHTTPStatus(err) {
		return serverPluginProfile{}, err
	}

	var h healthResp
	if err := fetchJSON(httpClient, base+"/api/health", headers, &h); err == nil {
		if strings.TrimSpace(h.DefaultPlugin.Name) != "" {
			return serverPluginProfile{
				Name:          h.DefaultPlugin.Name,
				TargetNetwork: h.DefaultPlugin.TargetNetwork,
			}, nil
		}
	} else if isAuthHTTPStatus(err) {
		return serverPluginProfile{}, err
	}

	return serverPluginProfile{}, fmt.Errorf(msg("ws.default_plugin_not_found"))
}

func shouldRequireClientProfile(cfg *config.ClientConfig, endpointChanged bool) bool {
	if endpointChanged {
		return true
	}
	if strings.TrimSpace(cfg.AuthToken) != "" {
		return true
	}
	u, err := url.Parse(strings.TrimSpace(cfg.ServerWSURL))
	if err != nil || strings.TrimSpace(u.Host) == "" {
		return false
	}
	return !isLocalHostOrLAN(u.Hostname())
}

func preflightClientCheck(cfg *config.ClientConfig, required bool) error {
	if !required {
		return nil
	}
	base, err := apiBaseFromWSURL(cfg.ServerWSURL)
	if err != nil {
		return err
	}

	httpClient := &http.Client{Timeout: 6 * time.Second}
	headers := clientProfileFetchHeaders(cfg)
	if _, err := fetchRoadServerInfo(httpClient, base, headers); err != nil {
		if isAuthHTTPStatus(err) {
			return fmt.Errorf(msg("client.preflight_auth_failed"), err)
		}
		return fmt.Errorf(msg("client.preflight_failed"), err)
	}
	return nil
}

func fetchRoadServerInfo(httpClient *http.Client, base string, headers http.Header) (roadServerInfo, error) {
	var info roadServerInfo
	if err := fetchJSON(httpClient, base+"/api/info", headers, &info); err != nil {
		return roadServerInfo{}, err
	}
	if err := validateServerServiceName(info.Service); err != nil {
		return roadServerInfo{}, err
	}
	return info, nil
}

func validateServerServiceName(service string) error {
	if strings.TrimSpace(service) != expectedServerServiceName {
		return fmt.Errorf(msg("ws.unexpected_service"), strings.TrimSpace(service), expectedServerServiceName)
	}
	return nil
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
		return httpStatusError{endpoint: endpoint, statusCode: resp.StatusCode}
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

type httpStatusError struct {
	endpoint   string
	statusCode int
}

func (e httpStatusError) Error() string {
	if e.statusCode == http.StatusUnauthorized || e.statusCode == http.StatusForbidden {
		return fmt.Sprintf("http %d from %s (ROAD auth token missing or invalid)", e.statusCode, e.endpoint)
	}
	return fmt.Sprintf("http %d from %s", e.statusCode, e.endpoint)
}

func isAuthHTTPStatus(err error) bool {
	var statusErr httpStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	return statusErr.statusCode == http.StatusUnauthorized || statusErr.statusCode == http.StatusForbidden
}
