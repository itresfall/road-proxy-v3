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
	"road-proxy-v3/internal/version"
	"road-proxy-v3/internal/voice"
)

func main() {
	tr := cli.LoadTranslator(app.ResolveExistingPath("configs/app.json"))

	configPath := flag.String("config", "configs/voice-server.json", tr.T("cmd.voice.flag_config"))
	showVersion := flag.Bool("version", false, tr.T("cmd.flag_version"))
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String("voice-server"))
		return
	}

	resolvedConfigPath := app.ResolveExistingPath(*configPath)
	cfg, err := voice.LoadConfig(resolvedConfigPath)
	if err != nil {
		log.Fatalf(tr.T("cmd.voice.error.config_load_failed"), err)
	}

	server, err := voice.NewServer(cfg, log.Default())
	if err != nil {
		log.Fatalf(tr.T("cmd.voice.error.setup_failed"), err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		log.Fatalf(tr.T("cmd.voice.error.stopped"), err)
	}
}
