package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/config"
)

func startSettingsFlow(reader *bufio.Reader) error {
	layout, err := app.EnsureRuntimeLayout()
	if err != nil {
		return fmt.Errorf(msg("errors.runtime_setup_failed"), err)
	}

	for {
		showTitle()
		fmt.Println(msg("settings.title"))
		fmt.Println("========")
		fmt.Println(msg("settings.choose"))
		fmt.Println("  1) " + msg("settings.mode_language"))
		fmt.Println("  2) " + msg("settings.mode_client"))
		fmt.Println("  3) " + msg("settings.mode_server"))
		fmt.Println("  4) " + msg("settings.mode_paths"))
		fmt.Println("  5) " + msg("settings.mode_back"))

		choice, err := readChoice(reader, msg("settings.choice_1_5"), 1, 5, 5)
		if err != nil {
			return err
		}

		switch choice {
		case 1:
			if err := editLanguageSettings(reader, layout); err != nil {
				return err
			}
		case 2:
			if err := editClientSettings(reader, layout); err != nil {
				return err
			}
		case 3:
			if err := editServerSettings(reader, layout); err != nil {
				return err
			}
		case 4:
			showSettingsPaths(layout)
			if _, err := readLine(reader, msg("settings.press_enter")); err != nil {
				return err
			}
		case 5:
			return nil
		}
	}
}

func editLanguageSettings(reader *bufio.Reader, layout app.RuntimeLayout) error {
	settings, err := loadEditableAppSettings(layout.AppConfigPath)
	if err != nil {
		return err
	}

	showTitle()
	fmt.Println(msg("settings.language_title"))
	fmt.Printf(msg("settings.language_current"), settings.Language)
	fmt.Println("  1) " + msg("settings.language_tr"))
	fmt.Println("  2) " + msg("settings.language_en"))
	fmt.Println("  3) " + msg("settings.mode_back"))

	def := 2
	if settings.Language == "tr" {
		def = 1
	}
	choice, err := readChoice(reader, msg("settings.choice_1_3"), 1, 3, def)
	if err != nil {
		return err
	}
	if choice == 3 {
		return nil
	}

	if choice == 1 {
		settings.Language = "tr"
	} else {
		settings.Language = "en"
	}
	settings.Normalize()
	if err := writeJSONFile(layout.AppConfigPath, settings, "app settings"); err != nil {
		return err
	}
	setLanguage(settings.Language)
	return finishSettingsSave(reader)
}

func editClientSettings(reader *bufio.Reader, layout app.RuntimeLayout) error {
	cfg, err := loadEditableClientConfig(layout.ClientConfigPath)
	if err != nil {
		return fmt.Errorf(msg("errors.client_config_load_failed"), err)
	}

	showTitle()
	fmt.Println(msg("settings.client_title"))
	fmt.Printf(msg("settings.config_file_line"), layout.ClientConfigPath)

	serverURL, err := readSettingString(reader, msg("settings.client_server_ws_url"), cfg.ServerWSURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(serverURL) != strings.TrimSpace(cfg.ServerWSURL) {
		normalized, normErr := normalizeClientWSURL(serverURL)
		if normErr != nil {
			return normErr
		}
		cfg.ServerWSURL = normalized
	}

	cfg.ListenAddr, err = readSettingString(reader, msg("settings.client_listen_addr"), cfg.ListenAddr)
	if err != nil {
		return err
	}
	cfg.ListenNetwork, err = readNetworkSetting(reader, cfg.ListenNetwork)
	if err != nil {
		return err
	}
	cfg.PluginName, cfg.ListenNetwork, err = readClientPluginSetting(reader, layout, cfg.PluginName, cfg.ListenNetwork)
	if err != nil {
		return err
	}

	if err := editClientAuthSettings(reader, cfg); err != nil {
		return err
	}
	cfg.EnableCompression, err = readSettingBool(reader, msg("settings.ws_compression"), cfg.EnableCompression)
	if err != nil {
		return err
	}
	cfg.UDPMetricsLog, err = readSettingString(reader, msg("settings.client_udp_metrics"), cfg.UDPMetricsLog)
	if err != nil {
		return err
	}
	cfg.Logging.Format, err = readLoggingFormatSetting(reader, cfg.Logging.Format)
	if err != nil {
		return err
	}
	cfg.UDPRecord.Enabled, err = readSettingBool(reader, msg("settings.udp_record"), cfg.UDPRecord.Enabled)
	if err != nil {
		return err
	}

	if err := validateEditableClientConfig(cfg); err != nil {
		return err
	}
	if err := writeClientConfig(layout.ClientConfigPath, cfg); err != nil {
		return err
	}
	return finishSettingsSave(reader)
}

func editServerSettings(reader *bufio.Reader, layout app.RuntimeLayout) error {
	cfg, err := loadEditableServerConfig(layout.ServerConfigPath)
	if err != nil {
		return fmt.Errorf(msg("errors.config_load_failed"), err)
	}

	showTitle()
	fmt.Println(msg("settings.server_title"))
	fmt.Printf(msg("settings.config_file_line"), layout.ServerConfigPath)

	cfg.HTTP.Enabled, err = readSettingBool(reader, msg("settings.server_http_enabled"), cfg.HTTP.Enabled)
	if err != nil {
		return err
	}
	cfg.HTTP.ListenAddr, err = readSettingString(reader, msg("settings.server_http_addr"), cfg.HTTP.ListenAddr)
	if err != nil {
		return err
	}
	cfg.Control.Enabled, err = readSettingBool(reader, msg("settings.server_control_enabled"), cfg.Control.Enabled)
	if err != nil {
		return err
	}
	cfg.Control.ListenAddr, err = readSettingString(reader, msg("settings.server_control_addr"), cfg.Control.ListenAddr)
	if err != nil {
		return err
	}
	cfg.Control.PluginAPIPublic, err = readSettingBool(reader, msg("settings.server_plugin_api_public"), cfg.Control.PluginAPIPublic)
	if err != nil {
		return err
	}

	if err := editServerAuthSettings(reader, cfg); err != nil {
		return err
	}
	cfg.HTTP.EnableCompression, err = readSettingBool(reader, msg("settings.ws_compression"), cfg.HTTP.EnableCompression)
	if err != nil {
		return err
	}
	cfg.HTTP.MaxConnections, err = readSettingNonNegativeInt(reader, msg("settings.server_max_connections"), cfg.HTTP.MaxConnections)
	if err != nil {
		return err
	}
	cfg.HTTP.MaxConnectionsPerIP, err = readSettingNonNegativeInt(reader, msg("settings.server_max_connections_per_ip"), cfg.HTTP.MaxConnectionsPerIP)
	if err != nil {
		return err
	}
	cfg.HTTP.RateLimitPerMinute, err = readSettingNonNegativeInt(reader, msg("settings.server_rate_limit"), cfg.HTTP.RateLimitPerMinute)
	if err != nil {
		return err
	}
	cfg.Logging.Format, err = readLoggingFormatSetting(reader, cfg.Logging.Format)
	if err != nil {
		return err
	}
	cfg.UDPRecord.Enabled, err = readSettingBool(reader, msg("settings.udp_record"), cfg.UDPRecord.Enabled)
	if err != nil {
		return err
	}

	if err := validateEditableServerConfig(cfg); err != nil {
		return err
	}
	if err := writeServerConfig(layout.ServerConfigPath, cfg); err != nil {
		return err
	}
	return finishSettingsSave(reader)
}

func showSettingsPaths(layout app.RuntimeLayout) {
	showTitle()
	fmt.Println(msg("settings.paths_title"))
	fmt.Printf(msg("settings.path_app"), layout.AppConfigPath)
	fmt.Printf(msg("settings.path_client"), layout.ClientConfigPath)
	fmt.Printf(msg("settings.path_server"), layout.ServerConfigPath)
	fmt.Printf(msg("settings.path_plugins"), layout.PluginDir)
}

func loadEditableAppSettings(path string) (*app.AppSettings, error) {
	settings, err := app.LoadAppSettings(path)
	if err == nil {
		return settings, nil
	}
	settings = app.DefaultAppSettings()
	return settings, nil
}

func loadEditableClientConfig(path string) (*config.ClientConfig, error) {
	cfg := config.DefaultClient()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := validateEditableClientConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadEditableServerConfig(path string) (*config.Config, error) {
	cfg := config.Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := validateEditableServerConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validateEditableClientConfig(cfg *config.ClientConfig) error {
	clone := *cfg
	return clone.NormalizeWithOptions(config.ClientNormalizeOptions{ValidateServerWSURL: true, AllowMissingEnvSecrets: true})
}

func validateEditableServerConfig(cfg *config.Config) error {
	clone := *cfg
	return clone.NormalizeWithOptions(config.NormalizeOptions{AllowMissingEnvSecrets: true})
}

func readSettingString(reader *bufio.Reader, label, current string) (string, error) {
	raw, err := readLine(reader, fmt.Sprintf(msg("settings.prompt_string"), label, current))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(raw) == "" {
		return current, nil
	}
	return strings.TrimSpace(raw), nil
}

func readSettingBool(reader *bufio.Reader, label string, current bool) (bool, error) {
	return askYesNo(reader, fmt.Sprintf(msg("settings.prompt_bool"), label, enabledLabel(current)), current)
}

func readSettingNonNegativeInt(reader *bufio.Reader, label string, current int) (int, error) {
	for {
		raw, err := readLine(reader, fmt.Sprintf(msg("settings.prompt_int"), label, current))
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(raw) == "" {
			return current, nil
		}
		value, convErr := strconv.Atoi(strings.TrimSpace(raw))
		if convErr == nil && value >= 0 {
			return value, nil
		}
		fmt.Println(msg("settings.invalid_non_negative"))
	}
}

func readNetworkSetting(reader *bufio.Reader, current string) (string, error) {
	current = strings.ToLower(strings.TrimSpace(current))
	if current != "udp" {
		current = "tcp"
	}
	def := 1
	if current == "udp" {
		def = 2
	}
	fmt.Printf(msg("settings.current_network"), current)
	fmt.Println("  1) tcp")
	fmt.Println("  2) udp")
	choice, err := readChoice(reader, msg("settings.choice_1_2_keep"), 1, 2, def)
	if err != nil {
		return "", err
	}
	if choice == 2 {
		return "udp", nil
	}
	return "tcp", nil
}

func readLoggingFormatSetting(reader *bufio.Reader, current string) (string, error) {
	current = strings.ToLower(strings.TrimSpace(current))
	if current != "json" {
		current = "text"
	}
	def := 1
	if current == "json" {
		def = 2
	}
	fmt.Printf(msg("settings.current_logging"), current)
	fmt.Println("  1) text")
	fmt.Println("  2) json")
	choice, err := readChoice(reader, msg("settings.choice_1_2_keep"), 1, 2, def)
	if err != nil {
		return "", err
	}
	if choice == 2 {
		return "json", nil
	}
	return "text", nil
}

func readClientPluginSetting(reader *bufio.Reader, layout app.RuntimeLayout, currentName, currentNetwork string) (string, string, error) {
	change, err := askYesNo(reader, fmt.Sprintf(msg("settings.client_plugin_change"), currentName), false)
	if err != nil {
		return "", "", err
	}
	if !change {
		return currentName, currentNetwork, nil
	}

	plugins, err := loadMenuPlugins(layout.PluginDir)
	if err != nil || len(plugins) == 0 {
		name, readErr := readSettingString(reader, msg("settings.client_plugin_name"), currentName)
		return name, currentNetwork, readErr
	}
	selected, err := promptPluginSelection(reader, plugins, currentName)
	if err != nil {
		return "", "", err
	}
	network := strings.ToLower(strings.TrimSpace(selected.TargetNetwork))
	if network != "tcp" && network != "udp" {
		network = currentNetwork
	}
	return selected.Name, network, nil
}

func editClientAuthSettings(reader *bufio.Reader, cfg *config.ClientConfig) error {
	enabled, err := readSettingBool(reader, msg("settings.client_auth"), strings.TrimSpace(cfg.AuthToken) != "")
	if err != nil {
		return err
	}
	if !enabled {
		cfg.AuthToken = ""
		cfg.AuthHeader = ""
		return nil
	}

	token, err := readSettingString(reader, msg("settings.auth_token"), cfg.AuthToken)
	if err != nil {
		return err
	}
	cfg.AuthToken = strings.TrimSpace(token)
	header, err := readSettingString(reader, msg("settings.auth_header"), defaultIfEmpty(cfg.AuthHeader, config.DefaultAuthHeaderName))
	if err != nil {
		return err
	}
	cfg.AuthHeader = strings.TrimSpace(header)
	return nil
}

func editServerAuthSettings(reader *bufio.Reader, cfg *config.Config) error {
	enabled, err := readSettingBool(reader, msg("settings.server_auth"), cfg.WSAuthEnabled())
	if err != nil {
		return err
	}
	if !enabled {
		cfg.HTTP.AuthToken = ""
		cfg.HTTP.AuthTokens = []string{}
		cfg.HTTP.AuthHeader = ""
		return nil
	}

	extraTokens := append([]string(nil), cfg.HTTP.AuthTokens...)
	rawToken := strings.TrimSpace(cfg.HTTP.AuthToken)
	if rawToken == "" && len(extraTokens) > 0 {
		rawToken = strings.TrimSpace(extraTokens[0])
	}
	if rawToken == "" {
		rawToken, err = generateWizardToken()
		if err != nil {
			return fmt.Errorf("generate auth token: %w", err)
		}
	}
	token, err := readSettingString(reader, msg("settings.auth_token"), rawToken)
	if err != nil {
		return err
	}
	cfg.HTTP.AuthToken = strings.TrimSpace(token)
	cfg.HTTP.AuthTokens = preserveExtraAuthTokens(extraTokens, cfg.HTTP.AuthToken)
	header, err := readSettingString(reader, msg("settings.auth_header"), defaultIfEmpty(cfg.HTTP.AuthHeader, config.DefaultAuthHeaderName))
	if err != nil {
		return err
	}
	cfg.HTTP.AuthHeader = strings.TrimSpace(header)
	return nil
}

func preserveExtraAuthTokens(existing []string, primary string) []string {
	primary = strings.TrimSpace(primary)
	seen := map[string]struct{}{}
	if primary != "" {
		seen[primary] = struct{}{}
	}
	out := make([]string, 0, len(existing))
	for _, token := range existing {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func finishSettingsSave(reader *bufio.Reader) error {
	fmt.Println(msg("settings.saved"))
	restart, err := askYesNo(reader, msg("settings.restart_prompt"), false)
	if err != nil {
		return err
	}
	if !restart {
		fmt.Println(msg("settings.reload_hint"))
		return nil
	}
	fmt.Println(msg("settings.restarting"))
	return restartSelf()
}

func restartSelf() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}

func enabledLabel(value bool) string {
	if value {
		return msg("settings.enabled")
	}
	return msg("settings.disabled")
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstString(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
