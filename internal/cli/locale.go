package cli

import (
	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/i18n"
)

func LoadTranslator(appConfigPath string) *i18n.Translator {
	lang := app.DefaultLanguage
	if settings, err := app.LoadAppSettings(appConfigPath); err == nil {
		lang = settings.Language
	}

	translator, err := i18n.Load(app.ResolveExistingPath("locales"), lang)
	if err != nil {
		return i18n.New(i18n.DefaultLanguage, nil, nil)
	}
	return translator
}
