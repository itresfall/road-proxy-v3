package main

import (
	"os"
	"path/filepath"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/i18n"
)

var studioTranslator = i18n.New(i18n.DefaultLanguage, nil, nil)

func init() {
	loadStudioLanguage(app.DefaultLanguage)
}

func initStudioLanguage() {
	lang := app.DefaultLanguage
	if settings, err := app.LoadAppSettings(resolveStudioExistingPath("configs/app.json")); err == nil {
		lang = settings.Language
	}
	loadStudioLanguage(lang)
}

func loadStudioLanguage(lang string) {
	translator, err := i18n.Load(resolveStudioExistingPath("locales"), lang)
	if err != nil {
		studioTranslator = i18n.New(i18n.DefaultLanguage, nil, nil)
		return
	}
	studioTranslator = translator
}

func reloadStudioLanguage(layout app.RuntimeLayout) {
	settings, err := app.LoadAppSettings(layout.AppConfigPath)
	if err != nil {
		loadStudioLanguage(app.DefaultLanguage)
		return
	}
	loadStudioLanguage(settings.Language)
}

func sm(key string) string {
	return studioTranslator.T(key)
}

func resolveStudioExistingPath(raw string) string {
	resolved := app.ResolveExistingPath(raw)
	if resolved != raw || filepath.IsAbs(raw) {
		return resolved
	}

	cwd, err := os.Getwd()
	if err != nil {
		return resolved
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(cwd, raw)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return resolved
}
