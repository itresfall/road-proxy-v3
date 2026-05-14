package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/config"
)

const (
	defaultPublicServerHTTPPort    = 8080
	defaultPublicServerControlPort = 8081
)

type publicServerLocalSettings struct {
	HTTPPort     int
	ControlPort  int
	HTTPAddr     string
	ControlAddr  string
	OriginURL    string
	ControlURL   string
	DashboardURL string
	PingURL      string
}

type publicServerRuntime struct {
	ConfigPath       string
	ClientConfigPath string
	Token            string
	Endpoint         string
	DashboardURL     string
	LocalOriginURL   string
	PluginName       string
}

func defaultPublicServerLocalSettings() publicServerLocalSettings {
	settings, err := newPublicServerLocalSettings(defaultPublicServerHTTPPort, defaultPublicServerControlPort)
	if err != nil {
		panic(err)
	}
	return settings
}

func newPublicServerLocalSettings(httpPort int, controlPort int) (publicServerLocalSettings, error) {
	if httpPort == controlPort {
		return publicServerLocalSettings{}, fmt.Errorf(msg("public.port_same"), httpPort)
	}
	httpAddr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	controlAddr := fmt.Sprintf("127.0.0.1:%d", controlPort)
	controlURL := "http://" + controlAddr
	return publicServerLocalSettings{
		HTTPPort:     httpPort,
		ControlPort:  controlPort,
		HTTPAddr:     httpAddr,
		ControlAddr:  controlAddr,
		OriginURL:    "http://" + httpAddr,
		ControlURL:   controlURL,
		DashboardURL: controlURL + "/dashboard",
		PingURL:      "http://" + httpAddr + "/api/ping",
	}, nil
}

func generateWizardToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func buildPublicServerConfig(
	layout app.RuntimeLayout,
	selected menuPlugin,
	local publicServerLocalSettings,
	publicHost string,
	endpoint string,
) (*config.Config, publicServerRuntime, error) {
	token, err := generateWizardToken()
	if err != nil {
		return nil, publicServerRuntime{}, fmt.Errorf("generate auth token: %w", err)
	}

	templatePath, err := buildServerConfigFromPlugin(layout, selected)
	if err != nil {
		return nil, publicServerRuntime{}, err
	}
	cfg, err := config.Load(templatePath)
	if err != nil {
		return nil, publicServerRuntime{}, err
	}

	cfg.TCP.ListenAddr = "127.0.0.1:0"
	cfg.HTTP.Enabled = true
	cfg.HTTP.ListenAddr = local.HTTPAddr
	cfg.HTTP.WSEndpoint = "/ws"
	cfg.HTTP.AuthToken = token
	cfg.HTTP.AuthTokens = []string{}
	cfg.HTTP.AuthHeader = config.DefaultAuthHeaderName
	cfg.HTTP.TrustProxyHeaders = true
	cfg.HTTP.MaxConnections = 16
	cfg.HTTP.MaxConnectionsPerIP = 8
	cfg.HTTP.RateLimitPerMinute = 60
	cfg.HTTP.AllowedHosts = publicAllowedHosts(publicHost)
	cfg.HTTP.AllowedOrigins = publicAllowedOrigins(publicHost, local)
	cfg.Control.Enabled = true
	cfg.Control.ListenAddr = local.ControlAddr
	cfg.Control.PluginAPIPublic = false
	cfg.Plugins.Dir = layout.PluginDir
	cfg.Plugins.Enabled = ensureSelectedPluginFirst(cfg.Plugins.Enabled, selected.Name)

	configPath, err := generatedConfigPath(layout, "server.public.menu.json")
	if err != nil {
		return nil, publicServerRuntime{}, err
	}
	if err := writeServerConfig(configPath, cfg); err != nil {
		return nil, publicServerRuntime{}, err
	}

	clientPath, err := writePublicClientConfig(layout, selected, endpoint, token)
	if err != nil {
		return nil, publicServerRuntime{}, err
	}

	return cfg, publicServerRuntime{
		ConfigPath:       configPath,
		ClientConfigPath: clientPath,
		Token:            token,
		Endpoint:         endpoint,
		DashboardURL:     local.DashboardURL,
		LocalOriginURL:   local.OriginURL,
		PluginName:       selected.Name,
	}, nil
}

func writePublicClientConfig(layout app.RuntimeLayout, selected menuPlugin, endpoint string, token string) (string, error) {
	clientCfg := config.DefaultClient()
	templateRel := strings.TrimSpace(selected.ClientTemplate)
	if templateRel != "" {
		if loaded, err := config.LoadClient(resolveRuntimePath(layout, templateRel)); err == nil {
			clientCfg = loaded
		}
	}
	clientCfg.ServerWSURL = endpoint
	clientCfg.PluginName = selected.Name
	clientCfg.ListenNetwork = selected.TargetNetwork
	clientCfg.AuthToken = token
	clientCfg.AuthHeader = config.DefaultAuthHeaderName

	outPath, err := generatedConfigPath(layout, "client.public.menu.json")
	if err != nil {
		return "", err
	}
	if err := writeClientConfig(outPath, clientCfg); err != nil {
		return "", err
	}
	return outPath, nil
}

func publicAllowedHosts(publicHost string) []string {
	host := strings.TrimSpace(publicHost)
	if host == "" {
		return []string{}
	}
	return []string{
		host,
		"127.0.0.1",
		"localhost",
		"::1",
	}
}

func publicAllowedOrigins(publicHost string, local publicServerLocalSettings) []string {
	origins := []string{
		local.ControlURL,
		fmt.Sprintf("http://localhost:%d", local.ControlPort),
	}
	host := strings.TrimSpace(publicHost)
	if host != "" {
		origins = append(origins, "https://"+host)
	}
	return origins
}

func ensureSelectedPluginFirst(enabled []string, selected string) []string {
	selected = strings.TrimSpace(selected)
	out := []string{}
	seen := map[string]struct{}{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	add(selected)
	for _, name := range enabled {
		add(name)
	}
	return out
}

func publicEndpointFromHTTPS(raw string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", err
	}
	if parsed.Host == "" {
		return "", "", fmt.Errorf("public URL has no host")
	}
	host := parsed.Hostname()
	if strings.TrimSpace(host) == "" {
		host = hostWithoutPortForWizard(parsed.Host)
	}
	return "wss://" + parsed.Host + "/ws", host, nil
}

func endpointFromHostname(hostname string) (string, string, error) {
	host := strings.TrimSpace(hostname)
	if host == "" {
		return "", "", fmt.Errorf("hostname is required")
	}
	if strings.Contains(host, "://") {
		parsed, err := url.Parse(host)
		if err != nil {
			return "", "", err
		}
		host = parsed.Host
	}
	host = strings.TrimSuffix(host, "/")
	if host == "" {
		return "", "", fmt.Errorf("hostname is required")
	}
	return "wss://" + host + "/ws", hostWithoutPortForWizard(host), nil
}

func hostWithoutPortForWizard(raw string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(strings.TrimSpace(raw), "[]")
}

func defaultNamedTunnelConfigPath(layout app.RuntimeLayout) string {
	return filepath.Join(layout.ConfigDir, ".generated", "cloudflared-road.yml")
}
