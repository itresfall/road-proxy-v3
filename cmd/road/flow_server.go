package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/engine"
	"road-proxy-v3/internal/logging"
)

func startServerFlow(reader *bufio.Reader) error {
	showTitle()
	fmt.Println(msg("server.mode"))
	fmt.Println("===========")

	layout, err := app.EnsureRuntimeLayout()
	if err != nil {
		return fmt.Errorf(msg("errors.runtime_setup_failed"), err)
	}

	plugins, err := loadMenuPlugins(layout.PluginDir)
	if err != nil {
		return err
	}
	if len(plugins) == 0 {
		return fmt.Errorf(msg("errors.plugin_not_found"), layout.PluginDir)
	}

	def := defaultPluginName(layout, plugins)
	selected, err := promptPluginSelection(reader, plugins, def)
	if err != nil {
		return err
	}

	configPath, err := buildServerConfigFromPlugin(layout, selected)
	if err != nil {
		return err
	}
	_ = saveMenuState(layout, selected.Name)

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf(msg("errors.config_load_failed"), err)
	}
	if !filepath.IsAbs(cfg.Plugins.Dir) {
		cfg.Plugins.Dir = filepath.Join(layout.Root, cfg.Plugins.Dir)
	}
	if err := applyServerPortOverrides(reader, cfg, selected.TargetNetwork); err != nil {
		return err
	}

	runtimeConfigPath, err := generatedConfigPath(layout, "server.runtime.menu.json")
	if err != nil {
		return err
	}
	if err := writeServerConfig(runtimeConfigPath, cfg); err != nil {
		return err
	}
	configPath = runtimeConfigPath

	if err := ensureServerPortsAvailable(reader, cfg); err != nil {
		return err
	}

	showTitle()
	fmt.Println(msg("server.starting"))
	fmt.Printf(msg("common.runtime_line"), layout.Root)
	fmt.Printf("Config: %s\n\n", configPath)

	if cfg.HasOpenNoAuthListener() {
		fmt.Println(msg("server.warn_open_no_auth"))
	}

	proxy := engine.New(cfg, logging.New(cfg.Logging.Format, "road-proxy-server"))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return proxy.Start(ctx)
}

func ensureServerPortsAvailable(reader *bufio.Reader, cfg *config.Config) error {
	ports := getServerListenPorts(cfg)
	for _, port := range ports {
		if err := ensurePortFree(reader, "tcp", port); err != nil {
			return err
		}
	}
	return nil
}

func getServerListenPorts(cfg *config.Config) []int {
	seen := map[int]struct{}{}
	add := func(addr string) {
		port, err := parsePortFromAddr(addr)
		if err != nil || port <= 0 {
			return
		}
		seen[port] = struct{}{}
	}

	add(cfg.TCP.ListenAddr)
	if cfg.HTTP.Enabled {
		add(cfg.HTTP.ListenAddr)
	}
	if cfg.Control.Enabled {
		add(cfg.Control.ListenAddr)
	}

	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports
}

func readPortOverride(reader *bufio.Reader, prompt string, current int, allowZero bool) (int, error) {
	raw, err := readLine(reader, fmt.Sprintf(msg("port.prompt_format"), prompt, current))
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(raw) == "" {
		return current, nil
	}
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf(msg("port.invalid"), raw)
	}
	minPort := 1
	if allowZero {
		minPort = 0
	}
	if port < minPort || port > 65535 {
		if allowZero {
			return 0, fmt.Errorf(msg("port.range_zero"), port)
		}
		return 0, fmt.Errorf(msg("port.range_one"), port)
	}
	return port, nil
}

func applyServerPortOverrides(reader *bufio.Reader, cfg *config.Config, selectedTargetNetwork string) error {
	fmt.Println("")
	fmt.Println(msg("port.override_header"))

	tcpHost, tcpPort, err := splitListenAddr(cfg.TCP.ListenAddr)
	if err != nil {
		return fmt.Errorf(msg("port.parse_tcp"), err)
	}
	tcpPrompt := msg("port.tcp_listener")
	tcpAllowZero := false
	if strings.EqualFold(strings.TrimSpace(selectedTargetNetwork), "udp") {
		tcpPrompt = msg("port.engine_tcp_fallback")
		tcpAllowZero = true
	}
	newTCPPort, err := readPortOverride(reader, tcpPrompt, tcpPort, tcpAllowZero)
	if err != nil {
		return err
	}
	cfg.TCP.ListenAddr = fmt.Sprintf("%s:%d", tcpHost, newTCPPort)

	if cfg.HTTP.Enabled {
		httpHost, httpPort, err := splitListenAddr(cfg.HTTP.ListenAddr)
		if err != nil {
			return fmt.Errorf(msg("port.parse_http"), err)
		}
		newHTTPPort, err := readPortOverride(reader, msg("port.http_ws"), httpPort, false)
		if err != nil {
			return err
		}
		cfg.HTTP.ListenAddr = fmt.Sprintf("%s:%d", httpHost, newHTTPPort)
	}

	if cfg.Control.Enabled {
		controlHost, controlPort, err := splitListenAddr(cfg.Control.ListenAddr)
		if err != nil {
			return fmt.Errorf(msg("port.parse_control"), err)
		}
		newControlPort, err := readPortOverride(reader, msg("port.control_api"), controlPort, false)
		if err != nil {
			return err
		}
		cfg.Control.ListenAddr = fmt.Sprintf("%s:%d", controlHost, newControlPort)
	}
	return nil
}

func loadMenuPlugins(pluginRoot string) ([]menuPlugin, error) {
	entries, err := os.ReadDir(pluginRoot)
	if err != nil {
		return nil, fmt.Errorf(msg("plugins.read_failed"), err)
	}

	plugins := make([]menuPlugin, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(pluginRoot, entry.Name(), "plugin.json")
		data, err := os.ReadFile(pluginPath)
		if err != nil {
			continue
		}

		var raw struct {
			Name   string `json:"name"`
			Target struct {
				Network string `json:"network"`
			} `json:"target"`
			Menu struct {
				ServerConfig string `json:"server_config"`
				ClientConfig string `json:"client_config"`
			} `json:"menu"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		name := strings.TrimSpace(raw.Name)
		if name == "" {
			name = entry.Name()
		}

		network := strings.ToLower(strings.TrimSpace(raw.Target.Network))
		if network == "" {
			network = "tcp"
		}

		plugins = append(plugins, menuPlugin{
			Name:           name,
			TargetNetwork:  network,
			ServerTemplate: strings.TrimSpace(raw.Menu.ServerConfig),
			ClientTemplate: strings.TrimSpace(raw.Menu.ClientConfig),
		})
	}

	sort.Slice(plugins, func(i, j int) bool {
		return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name)
	})
	return plugins, nil
}

func promptPluginSelection(reader *bufio.Reader, plugins []menuPlugin, defaultPlugin string) (menuPlugin, error) {
	fmt.Println(msg("plugins.selection"))
	defIdx := 1
	for i := range plugins {
		mark := " "
		if plugins[i].Name == defaultPlugin {
			mark = "*"
			defIdx = i + 1
		}
		net := plugins[i].TargetNetwork
		if net == "" {
			net = "tcp"
		}
		fmt.Printf("  %d) %s%s [%s]\n", i+1, mark, plugins[i].Name, net)
	}

	choice, err := readChoice(
		reader,
		fmt.Sprintf(msg("plugins.choice"), len(plugins), defIdx),
		1,
		len(plugins),
		defIdx,
	)
	if err != nil {
		return menuPlugin{}, err
	}
	return plugins[choice-1], nil
}

func buildServerConfigFromPlugin(layout app.RuntimeLayout, selected menuPlugin) (string, error) {
	templateRel := strings.TrimSpace(selected.ServerTemplate)
	fallbackRel := "configs/server.json"
	if strings.EqualFold(selected.TargetNetwork, "udp") {
		fallbackRel = "configs/server-udp.example.json"
	}
	if templateRel == "" {
		templateRel = fallbackRel
	}

	templatePath := resolveRuntimePath(layout, templateRel)
	if _, err := os.Stat(templatePath); err != nil {
		templatePath = resolveRuntimePath(layout, fallbackRel)
	}

	cfg, err := config.Load(templatePath)
	if err != nil {
		return "", fmt.Errorf(msg("config.template_load_failed"), templatePath, err)
	}

	cfg.Plugins.Dir = "plugins"
	enabled := make([]string, 0, len(cfg.Plugins.Enabled)+1)
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
		enabled = append(enabled, name)
	}
	add(selected.Name)
	for _, name := range cfg.Plugins.Enabled {
		add(name)
	}
	cfg.Plugins.Enabled = enabled
	if strings.EqualFold(selected.TargetNetwork, "udp") {
		cfg.TCP.ListenAddr = "0.0.0.0:0"
	}

	outPath, err := generatedConfigPath(layout, "server.menu.json")
	if err != nil {
		return "", err
	}
	if err := writeServerConfig(outPath, cfg); err != nil {
		return "", err
	}
	return outPath, nil
}

func defaultPluginName(layout app.RuntimeLayout, plugins []menuPlugin) string {
	if len(plugins) == 0 {
		return ""
	}
	state, err := loadMenuState(layout)
	if err == nil {
		wanted := strings.TrimSpace(state.SelectedPlugin)
		if wanted != "" {
			for i := range plugins {
				if plugins[i].Name == wanted {
					return wanted
				}
			}
		}
	}
	return plugins[0].Name
}

func loadMenuState(layout app.RuntimeLayout) (menuState, error) {
	path := filepath.Join(layout.Root, ".road-menu-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return menuState{}, err
	}
	var state menuState
	if err := json.Unmarshal(data, &state); err != nil {
		return menuState{}, err
	}
	return state, nil
}

func saveMenuState(layout app.RuntimeLayout, pluginName string) error {
	state := menuState{SelectedPlugin: pluginName}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(layout.Root, ".road-menu-state.json"), data, 0o644)
}
