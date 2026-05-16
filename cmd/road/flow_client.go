package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os/signal"
	"strings"
	"syscall"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/client"
	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/logging"
)

func startClientFlow(reader *bufio.Reader) error {
	showTitle()
	fmt.Println(msg("client.mode"))
	fmt.Println("===========")

	layout, err := app.EnsureRuntimeLayout()
	if err != nil {
		return fmt.Errorf(msg("errors.runtime_setup_failed"), err)
	}
	configPath := layout.ClientConfigPath

	cfg, err := loadClientConfigForMenu(reader, configPath)
	if err != nil {
		return fmt.Errorf(msg("errors.client_config_load_failed"), err)
	}
	endpointChanged, err := applyClientConnectionInput(reader, cfg)
	if err != nil {
		return err
	}
	requireProfile := shouldRequireClientProfile(cfg, endpointChanged)
	if err := applyClientProfileFromServerWithAuthRetry(reader, cfg, requireProfile); err != nil {
		return err
	}

	configPath, err = generatedConfigPath(layout, "client.menu.json")
	if err != nil {
		return err
	}
	if err := writeClientConfig(configPath, cfg); err != nil {
		return err
	}

	showTitle()
	fmt.Println(msg("client.starting"))
	fmt.Printf(msg("common.runtime_line"), layout.Root)
	fmt.Printf("Config: %s\n\n", configPath)

	if err := ensureClientPortAvailable(reader, cfg); err != nil {
		return err
	}

	if err := preflightClientCheck(cfg, requireProfile); err != nil {
		return err
	}

	printClientReadyInstructions(cfg)

	tunnel := client.New(cfg, logging.New(cfg.Logging.Format, "road-proxy-client"))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return tunnel.Start(ctx)
}

func loadClientConfigForMenu(reader *bufio.Reader, configPath string) (*config.ClientConfig, error) {
	cfg, err := config.LoadClient(configPath)
	if err == nil {
		return cfg, nil
	}

	lenientCfg, lenientErr := config.LoadClientWithOptions(configPath, config.ClientNormalizeOptions{ValidateServerWSURL: false})
	if lenientErr != nil {
		return nil, err
	}

	fmt.Printf(msg("client.server_ws_url_invalid"), err)
	if repairErr := repairClientServerWSURL(reader, lenientCfg); repairErr != nil {
		return nil, repairErr
	}
	return lenientCfg, nil
}

func repairClientServerWSURL(reader *bufio.Reader, cfg *config.ClientConfig) error {
	current := strings.TrimSpace(cfg.ServerWSURL)
	if current == "" {
		current = "127.0.0.1:8080"
	}

	for {
		raw, err := readLine(reader, fmt.Sprintf(msg("client.server_ws_url_repair_prompt"), current))
		if err != nil {
			return err
		}
		if strings.TrimSpace(raw) == "" {
			raw = current
		}

		normalized, err := normalizeClientWSURL(raw)
		if err != nil {
			fmt.Printf(msg("client.server_ws_url_repair_failed"), err)
			continue
		}
		if err := config.ValidateServerWSURL(normalized); err != nil {
			fmt.Printf(msg("client.server_ws_url_repair_failed"), err)
			continue
		}

		cfg.ServerWSURL = normalized
		fmt.Printf(msg("client.endpoint_selected"), normalized)
		return nil
	}
}

func applyClientConnectionInput(reader *bufio.Reader, cfg *config.ClientConfig) (bool, error) {
	current := strings.TrimSpace(cfg.ServerWSURL)
	if current == "" {
		current = "ws://127.0.0.1:8080/ws"
	}

	raw, err := readLine(reader, fmt.Sprintf(msg("client.connection_prompt"), current))
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(raw) == "" {
		return false, nil
	}

	normalized, err := normalizeClientWSURL(raw)
	if err != nil {
		return false, err
	}
	cfg.ServerWSURL = normalized
	fmt.Printf(msg("client.endpoint_selected"), normalized)
	return true, nil
}

func promptClientAuthToken(reader *bufio.Reader, cfg *config.ClientConfig) error {
	raw, err := readLine(reader, msg("client.auth_token_prompt"))
	if err != nil {
		return err
	}
	token := strings.TrimSpace(raw)
	if token == "" {
		if strings.TrimSpace(cfg.AuthToken) == "" {
			fmt.Print(msg("client.auth_token_missing_hint"))
		}
		return nil
	}

	cfg.AuthToken = token
	if strings.TrimSpace(cfg.AuthHeader) == "" {
		cfg.AuthHeader = config.DefaultAuthHeaderName
	}
	fmt.Printf(msg("client.auth_token_set"), cfg.AuthHeader)
	return nil
}

func applyClientProfileFromServerWithAuthRetry(reader *bufio.Reader, cfg *config.ClientConfig, requireProfile bool) error {
	err := applyClientProfileFromServer(cfg, requireProfile)
	if err == nil {
		return nil
	}
	if !isAuthHTTPStatus(err) {
		return err
	}

	fmt.Print(msg("client.auth_required"))
	if promptErr := promptClientAuthToken(reader, cfg); promptErr != nil {
		return promptErr
	}

	err = applyClientProfileFromServer(cfg, requireProfile)
	if err == nil {
		return nil
	}
	if isAuthHTTPStatus(err) {
		return fmt.Errorf(msg("client.preflight_auth_failed"), err)
	}
	return err
}

func applyClientProfileFromServer(cfg *config.ClientConfig, requireProfile bool) error {
	profile, err := getServerDefaultPluginProfile(cfg)
	if err != nil {
		if requireProfile && isAuthHTTPStatus(err) {
			return err
		}
		if requireProfile {
			return fmt.Errorf(msg("client.profile_fetch_error"), cfg.ServerWSURL, err)
		}
		fmt.Printf(msg("client.profile_fetch_warning"), err)
		return nil
	}

	if strings.TrimSpace(profile.Name) != "" {
		cfg.PluginName = profile.Name
		fmt.Printf(msg("client.plugin_selected"), profile.Name)
	}

	netw := strings.ToLower(strings.TrimSpace(profile.TargetNetwork))
	if netw == "tcp" || netw == "udp" {
		cfg.ListenNetwork = netw
	}
	return nil
}

func printClientReadyInstructions(cfg *config.ClientConfig) {
	target, host, port := splitDisplayTarget(cfg.ListenAddr)

	fmt.Println(msg("client.ready"))
	fmt.Printf(msg("client.game_target_line"), target)
	if host != "" && port != "" {
		fmt.Printf(msg("client.game_host_port_line"), host, port)
	}
	fmt.Printf(msg("client.remote_line"), cfg.ServerWSURL)
	if strings.TrimSpace(cfg.PluginName) != "" {
		fmt.Printf(msg("client.active_plugin_line"), cfg.PluginName)
	}
	fmt.Println(msg("client.stop_hint"))
	fmt.Println()
	fmt.Println(msg("client.logs_follow"))
}

func splitDisplayTarget(addr string) (target string, host string, port string) {
	target = strings.TrimSpace(addr)
	if target == "" {
		return "", "", ""
	}

	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return target, "", ""
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), host, port
}
