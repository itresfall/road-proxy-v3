package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/cli"
	"road-proxy-v3/internal/client"
	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/logging"
	"road-proxy-v3/internal/version"
)

func main() {
	tr := cli.LoadTranslator(app.ResolveExistingPath("configs/app.json"))

	configPath := flag.String("config", "configs/client.json", tr.T("cmd.client.flag_config"))
	showVersion := flag.Bool("version", false, tr.T("cmd.flag_version"))
	skipPreflight := flag.Bool("skip-preflight", false, "skip startup server preflight check")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String("road-client"))
		return
	}

	layout, err := app.EnsureRuntimeLayout()
	if err != nil {
		log.Fatalf(tr.T("cmd.error.runtime_setup_failed"), err)
	}
	tr = cli.LoadTranslator(layout.AppConfigPath)

	resolvedConfigPath := app.ResolveExistingPath(*configPath)
	if app.IsDefaultRelativePath(*configPath, "configs/client.json") {
		resolvedConfigPath = layout.ClientConfigPath
	}

	cfg, err := config.LoadClient(resolvedConfigPath)
	if err != nil {
		log.Fatalf(tr.T("cmd.client.error.config_load_failed"), err)
	}
	if !*skipPreflight {
		if err := runClientPreflight(cfg); err != nil {
			log.Fatalf("client preflight failed: %v", err)
		}
	}

	logger := logging.New(cfg.Logging.Format, "road-client")
	tunnel := client.New(cfg, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := tunnel.Start(ctx); err != nil {
		logger.Fatalf(tr.T("cmd.client.error.stopped"), err)
	}
}
