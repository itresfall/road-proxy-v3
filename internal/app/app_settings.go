package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const DefaultLanguage = "en"

type AppSettings struct {
	Language string `json:"language"`
}

func DefaultAppSettings() *AppSettings {
	return &AppSettings{
		Language: DefaultLanguage,
	}
}

func (a *AppSettings) Normalize() {
	a.Language = NormalizeLanguage(a.Language)
}

func LoadAppSettings(path string) (*AppSettings, error) {
	cfg := DefaultAppSettings()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read app settings %q: %w", path, err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse app settings %q: %w", path, err)
	}
	cfg.Normalize()
	return cfg, nil
}

func NormalizeLanguage(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "tr", "tr-tr", "turkish":
		return "tr"
	case "", "en", "en-us", "en-gb", "eng", "english":
		return "en"
	default:
		return DefaultLanguage
	}
}
