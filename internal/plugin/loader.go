package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Loader struct {
	pluginDir string
}

func NewLoader(pluginDir string) *Loader {
	return &Loader{pluginDir: pluginDir}
}

func (l *Loader) LoadEnabled(enabled []string) (map[string]*RuntimePlugin, error) {
	if len(enabled) == 0 {
		return nil, fmt.Errorf("enabled plugins cannot be empty")
	}

	result := make(map[string]*RuntimePlugin, len(enabled))
	for _, rawPluginName := range enabled {
		pluginName := strings.TrimSpace(rawPluginName)
		if pluginName == "" {
			return nil, fmt.Errorf("enabled plugin name cannot be empty")
		}
		if _, exists := result[pluginName]; exists {
			return nil, fmt.Errorf("enabled plugin %q is duplicated", pluginName)
		}
		schema, err := l.loadSchema(pluginName)
		if err != nil {
			return nil, err
		}
		result[pluginName] = NewRuntimePlugin(schema)
	}

	return result, nil
}

func (l *Loader) ListAvailable() ([]string, error) {
	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("read plugin dir %q: %w", l.pluginDir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Empty scratch directories are not plugins. Only advertise folders that
		// actually contain a schema, while still surfacing malformed schemas later
		// through LoadEnabled.
		schemaPath := filepath.Join(l.pluginDir, entry.Name(), "plugin.json")
		if _, err := os.Stat(schemaPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat plugin schema %q: %w", schemaPath, err)
		}
		names = append(names, entry.Name())
	}

	sort.Strings(names)
	return names, nil
}

func (l *Loader) loadSchema(pluginName string) (*Schema, error) {
	path := filepath.Join(l.pluginDir, pluginName, "plugin.json")
	schema, err := LoadSchemaFile(path)
	if err != nil {
		return nil, err
	}

	if schema.Name != pluginName {
		return nil, fmt.Errorf("plugin schema name %q does not match folder %q", schema.Name, pluginName)
	}

	return schema, nil
}

func LoadSchemaFile(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plugin schema %q: %w", path, err)
	}

	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("parse plugin schema %q: %w", path, err)
	}

	schema.Normalize()
	if err := schema.Validate(); err != nil {
		return nil, fmt.Errorf("validate plugin schema %q: %w", path, err)
	}

	return &schema, nil
}
