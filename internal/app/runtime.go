package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"road-proxy-v3/internal/config"
)

// RuntimeLayout keeps runtime paths under the executable location.
type RuntimeLayout struct {
	Root             string
	ConfigDir        string
	PluginDir        string
	ServerConfigPath string
	ClientConfigPath string
	AppConfigPath    string
}

type seedPlugin struct {
	Name                 string
	Description          string
	Network              string
	Address              string
	ServerConfigTemplate string
	ClientConfigTemplate string
	SupportsMultiplex    bool
}

// RuntimeRoot returns the executable directory if available.
func RuntimeRoot() string {
	exePath, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exePath)
		if strings.TrimSpace(dir) != "" {
			return dir
		}
	}

	wd, err := os.Getwd()
	if err == nil && strings.TrimSpace(wd) != "" {
		return wd
	}

	return "."
}

// EnsureRuntimeLayout creates runtime folders/files under the executable directory.
func EnsureRuntimeLayout() (RuntimeLayout, error) {
	return EnsureRuntimeLayoutAt(RuntimeRoot())
}

// EnsureRuntimeLayoutAt creates runtime folders/files under the given root.
func EnsureRuntimeLayoutAt(root string) (RuntimeLayout, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}

	absRoot, err := filepath.Abs(root)
	if err == nil {
		root = absRoot
	}

	layout := RuntimeLayout{
		Root:             root,
		ConfigDir:        filepath.Join(root, "configs"),
		PluginDir:        filepath.Join(root, "plugins"),
		ServerConfigPath: filepath.Join(root, "configs", "server.json"),
		ClientConfigPath: filepath.Join(root, "configs", "client.json"),
		AppConfigPath:    filepath.Join(root, "configs", "app.json"),
	}

	if err := os.MkdirAll(layout.ConfigDir, 0o755); err != nil {
		return RuntimeLayout{}, fmt.Errorf("create config dir %q: %w", layout.ConfigDir, err)
	}
	if err := os.MkdirAll(layout.PluginDir, 0o755); err != nil {
		return RuntimeLayout{}, fmt.Errorf("create plugin dir %q: %w", layout.PluginDir, err)
	}

	if err := seedConfig(layout.ConfigDir, "server.json", defaultServerConfigJSON()); err != nil {
		return RuntimeLayout{}, err
	}
	if err := seedConfig(layout.ConfigDir, "client.json", defaultClientConfigJSON()); err != nil {
		return RuntimeLayout{}, err
	}
	if err := seedConfig(layout.ConfigDir, "server-udp.example.json", defaultUDPServerConfigJSON()); err != nil {
		return RuntimeLayout{}, err
	}
	if err := seedConfig(layout.ConfigDir, "client-udp.example.json", defaultUDPClientConfigJSON()); err != nil {
		return RuntimeLayout{}, err
	}
	if err := seedConfig(layout.ConfigDir, "app.json", defaultAppConfigJSON()); err != nil {
		return RuntimeLayout{}, err
	}

	plugins := []seedPlugin{
		{
			Name:                 "minecraft",
			Description:          "Minecraft TCP profile for v3 engine",
			Network:              "tcp",
			Address:              "127.0.0.1:25565",
			ServerConfigTemplate: "configs/server.json",
			ClientConfigTemplate: "configs/client.json",
			SupportsMultiplex:    false,
		},
		{
			Name:                 "minecraft-bedrock-udp",
			Description:          "Minecraft Bedrock UDP profile for v3 engine",
			Network:              "udp",
			Address:              "127.0.0.1:19132",
			ServerConfigTemplate: "configs/server-udp.example.json",
			ClientConfigTemplate: "configs/client-udp.example.json",
			SupportsMultiplex:    true,
		},
	}

	for _, p := range plugins {
		content, buildErr := buildPluginSeedJSON(p)
		if buildErr != nil {
			return RuntimeLayout{}, buildErr
		}
		pluginJSONPath := filepath.Join(layout.PluginDir, p.Name, "plugin.json")
		if err := seedFile(pluginJSONPath, filepath.Join("plugins", p.Name, "plugin.json"), content); err != nil {
			return RuntimeLayout{}, err
		}
	}

	return layout, nil
}

func seedConfig(configDir, name string, fallback []byte) error {
	dest := filepath.Join(configDir, name)
	return seedFile(dest, filepath.Join("configs", name), fallback)
}

func seedFile(destPath, sourceRelative string, fallback []byte) error {
	if info, err := os.Stat(destPath); err == nil {
		if info.IsDir() {
			return fmt.Errorf("seed target is a directory: %q", destPath)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check seed target %q: %w", destPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create seed parent dir for %q: %w", destPath, err)
	}

	resolvedSource := ResolveExistingPath(sourceRelative)
	if resolvedSource != "" {
		if srcInfo, err := os.Stat(resolvedSource); err == nil && !srcInfo.IsDir() {
			data, readErr := os.ReadFile(resolvedSource)
			if readErr == nil {
				if writeErr := os.WriteFile(destPath, data, 0o644); writeErr == nil {
					return nil
				}
			}
		}
	}

	if len(fallback) == 0 {
		return fmt.Errorf("no fallback seed content for %q", destPath)
	}
	if err := os.WriteFile(destPath, fallback, 0o644); err != nil {
		return fmt.Errorf("write seed file %q: %w", destPath, err)
	}
	return nil
}

func defaultServerConfigJSON() []byte {
	cfg := config.Default()
	return mustJSON(cfg)
}

func defaultClientConfigJSON() []byte {
	cfg := config.DefaultClient()
	return mustJSON(cfg)
}

func defaultUDPServerConfigJSON() []byte {
	cfg := config.Default()
	cfg.TCP.ListenAddr = "0.0.0.0:0"
	cfg.Plugins.Enabled = []string{"minecraft-bedrock-udp"}
	return mustJSON(cfg)
}

func defaultUDPClientConfigJSON() []byte {
	cfg := config.DefaultClient()
	cfg.ListenAddr = "127.0.0.1:19133"
	cfg.ListenNetwork = "udp"
	cfg.PluginName = "minecraft-bedrock-udp"
	return mustJSON(cfg)
}

func defaultAppConfigJSON() []byte {
	cfg := DefaultAppSettings()
	return mustJSON(cfg)
}

func buildPluginSeedJSON(seed seedPlugin) ([]byte, error) {
	doc := map[string]any{
		"schema_version": "v1",
		"name":           seed.Name,
		"version":        "3.0.0",
		"description":    seed.Description,
		"author":         "road",
		"protocols": map[string]any{
			"supported": []string{seed.Network, "websocket"},
		},
		"target": map[string]any{
			"network": seed.Network,
			"address": seed.Address,
		},
		"menu": map[string]any{
			"server_config": seed.ServerConfigTemplate,
			"client_config": seed.ClientConfigTemplate,
		},
		"capabilities": map[string]any{
			"supports_reconnect": true,
			"supports_multiplex": seed.SupportsMultiplex,
		},
		"runtime": map[string]any{
			"type":               "json",
			"mode":               "passthrough",
			"enable_obfuscation": false,
			"udp_peer_broadcast": false,
			"client_pipeline":    []string{},
			"server_pipeline":    []string{},
		},
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal plugin seed %q: %w", seed.Name, err)
	}
	return append(data, '\n'), nil
}

func mustJSON(v any) []byte {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(data, '\n')
}
