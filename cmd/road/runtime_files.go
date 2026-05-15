package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/config"
)

func resolveRuntimePath(layout app.RuntimeLayout, pathValue string) string {
	if filepath.IsAbs(pathValue) {
		return filepath.Clean(pathValue)
	}
	return filepath.Join(layout.Root, filepath.FromSlash(pathValue))
}

func generatedConfigPath(layout app.RuntimeLayout, fileName string) (string, error) {
	dir := filepath.Join(layout.ConfigDir, ".generated")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf(msg("runtime.generated_config_dir_failed"), err)
	}
	return filepath.Join(dir, fileName), nil
}

func writeClientConfig(path string, cfg *config.ClientConfig) error {
	return writeJSONFile(path, cfg, "client config")
}

func writeServerConfig(path string, cfg *config.Config) error {
	return writeJSONFile(path, cfg, "server config")
}

func writeJSONFile(path string, value any, label string) error {
	parent := filepath.Dir(path)
	if parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf(msg("runtime.parent_dir_failed"), label, err)
		}
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf(msg("runtime.marshal_failed"), label, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf(msg("runtime.save_failed"), label, err)
	}
	return nil
}
