package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/version"
)

func main() {
	enableConsoleUTF8()
	initLanguageFromConfig()

	if handled, err := tryRunSubcommand(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintf(os.Stderr, msg("common.error_line"), err)
			os.Exit(1)
		}
		return
	}

	showVersion := flag.Bool("version", false, msg("cmd.flag_version"))
	flag.Parse()
	if *showVersion {
		fmt.Println(version.String("road-proxy"))
		return
	}

	layout, layoutErr := app.EnsureRuntimeLayout()
	if layoutErr == nil {
		settings, settingsErr := app.LoadAppSettings(layout.AppConfigPath)
		if settingsErr == nil {
			setLanguage(settings.Language)
		} else {
			setLanguage(app.DefaultLanguage)
		}
	} else {
		setLanguage(app.DefaultLanguage)
	}

	ran, err := tryRunMenuScript()
	if ran {
		if err != nil {
			log.Fatalf("menu script failed: %v", err)
		}
		return
	}

	runBuiltinMenu()
}

func initLanguageFromConfig() {
	settings, err := app.LoadAppSettings(app.ResolveExistingPath("configs/app.json"))
	if err != nil {
		setLanguage(app.DefaultLanguage)
		return
	}
	setLanguage(settings.Language)
}
