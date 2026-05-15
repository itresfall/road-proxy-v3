package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"path/filepath"
	"syscall"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/cli"
	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/engine"
	"road-proxy-v3/internal/logging"
	"road-proxy-v3/internal/version"
)

func main() {
	tr := cli.LoadTranslator(app.ResolveExistingPath("configs/app.json"))

	configPath := flag.String("config", "configs/server.json", tr.T("cmd.server.flag_config"))
	showVersion := flag.Bool("version", false, tr.T("cmd.flag_version"))
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String("road-server"))
		return
	}

	layout, err := app.EnsureRuntimeLayout()
	if err != nil {
		log.Fatalf(tr.T("cmd.error.runtime_setup_failed"), err)
	}
	tr = cli.LoadTranslator(layout.AppConfigPath)

	resolvedConfigPath := app.ResolveExistingPath(*configPath)
	if app.IsDefaultRelativePath(*configPath, "configs/server.json") {
		resolvedConfigPath = layout.ServerConfigPath
	}

	cfg, err := config.Load(resolvedConfigPath)
	if err != nil {
		log.Fatalf(tr.T("cmd.server.error.config_load_failed"), err)
	}
	if !filepath.IsAbs(cfg.Plugins.Dir) {
		cfg.Plugins.Dir = filepath.Join(layout.Root, cfg.Plugins.Dir)
	}

	logger := logging.New(cfg.Logging.Format, "road-server")
	if cfg.HasOpenNoAuthListener() {
		logger.Print(tr.T("server.warn_open_no_auth"))
	}
	proxy := engine.New(cfg, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := proxy.Start(ctx); err != nil {
		logger.Fatalf(tr.T("cmd.server.error.engine_stopped"), err)
	}
}
