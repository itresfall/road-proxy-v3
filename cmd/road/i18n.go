package main

import (
	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/i18n"
)

var currentTranslator = i18n.New(i18n.DefaultLanguage, nil, nil)

func setLanguage(lang string) {
	localeDir := app.ResolveExistingPath("locales")
	translator, err := i18n.Load(localeDir, lang)
	if err != nil {
		currentTranslator = i18n.New(i18n.DefaultLanguage, nil, nil)
		return
	}
	currentTranslator = translator
}

func activeLanguage() string {
	return currentTranslator.Language()
}

func msg(key string) string {
	return currentTranslator.T(key)
}
