package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/engine"
	"road-proxy-v3/internal/logging"
)

func runPublicServerCommand(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("public-server", flag.ContinueOnError)
	fs.SetOutput(out)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected public-server args: %s", strings.Join(fs.Args(), " "))
	}
	return startPublicServerWizard(bufio.NewReader(os.Stdin))
}

func startPublicServerWizard(reader *bufio.Reader) error {
	showTitle()
	fmt.Println(msg("public.title"))
	fmt.Println("====================")
	fmt.Println(msg("public.subtitle"))
	fmt.Println()

	layout, err := app.EnsureRuntimeLayout()
	if err != nil {
		return fmt.Errorf(msg("errors.runtime_setup_failed"), err)
	}
	lock, err := acquirePublicServerLock(layout)
	if err != nil {
		return err
	}
	defer lock.Release()

	bin, err := ensureCloudflared(reader)
	if err != nil {
		return err
	}

	plugins, err := loadMenuPlugins(layout.PluginDir)
	if err != nil {
		return err
	}
	if len(plugins) == 0 {
		return fmt.Errorf(msg("errors.plugin_not_found"), layout.PluginDir)
	}
	selected, err := promptPluginSelection(reader, plugins, defaultPluginName(layout, plugins))
	if err != nil {
		return err
	}
	_ = saveMenuState(layout, selected.Name)

	fmt.Println()
	fmt.Println(msg("public.cloudflare_mode"))
	fmt.Println("  1) " + msg("public.mode_trycloudflare"))
	fmt.Println("  2) " + msg("public.mode_domain"))
	fmt.Println("  3) " + msg("public.mode_token"))
	fmt.Println("  4) " + msg("public.mode_cancel"))
	choice, err := readChoice(reader, msg("public.choice_1_4_default_1"), 1, 4, 1)
	if err != nil {
		return err
	}
	if choice == 4 {
		return nil
	}

	fmt.Println()
	fmt.Println(msg("public.auth_policy_notice"))

	local, err := promptPublicServerLocalSettings(reader)
	if err != nil {
		return err
	}

	switch choice {
	case 1:
		return runTryCloudflarePublicServer(reader, layout, selected, bin, local)
	case 2:
		return runNamedCloudflarePublicServer(reader, layout, selected, bin, local)
	case 3:
		return runTokenCloudflarePublicServer(reader, layout, selected, bin, local)
	default:
		return nil
	}
}

func runTryCloudflarePublicServer(reader *bufio.Reader, layout app.RuntimeLayout, selected menuPlugin, bin string, local publicServerLocalSettings) error {
	if err := ensurePublicServerPorts(reader, local); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Println(msg("public.trycloudflare_starting"))
	tunnel, urlCh, err := startTryCloudflareTunnel(ctx, bin, local.OriginURL)
	if err != nil {
		return err
	}
	defer func() {
		_ = tunnel.Stop(5 * time.Second)
	}()

	publicURL, err := waitForTunnelURL(ctx, urlCh, tunnel, 30*time.Second)
	if err != nil {
		return err
	}
	endpoint, publicHost, err := publicEndpointFromHTTPS(publicURL)
	if err != nil {
		return err
	}

	cfg, runtimeInfo, err := buildPublicServerConfig(layout, selected, local, publicHost, endpoint)
	if err != nil {
		return err
	}
	return runPublicEngineUntilStopped(ctx, cfg, runtimeInfo, tunnel.done)
}

func runNamedCloudflarePublicServer(reader *bufio.Reader, layout app.RuntimeLayout, selected menuPlugin, bin string, local publicServerLocalSettings) error {
	if err := ensurePublicServerPorts(reader, local); err != nil {
		return err
	}
	if err := runCloudflaredLoginIfNeeded(bin); err != nil {
		return err
	}

	hostname, err := readRequiredLine(reader, msg("public.hostname_prompt"))
	if err != nil {
		return err
	}
	endpoint, publicHost, err := endpointFromHostname(hostname)
	if err != nil {
		return err
	}

	defaultName := "road-proxy-" + time.Now().Format("20060102-150405")
	tunnelName, err := readLine(reader, fmt.Sprintf(msg("public.tunnel_name_prompt"), defaultName))
	if err != nil {
		return err
	}
	if strings.TrimSpace(tunnelName) == "" {
		tunnelName = defaultName
	}

	fmt.Println(msg("public.tunnel_creating"))
	tunnelUUID, err := createNamedTunnelOrReuse(reader, bin, tunnelName)
	if err != nil {
		return err
	}
	if tunnelUUID != "" {
		fmt.Printf(msg("public.tunnel_uuid"), tunnelUUID)
	}

	fmt.Println(msg("public.dns_route_adding"))
	if err := routeNamedTunnelDNSWithReader(bin, tunnelName, publicHost, reader); err != nil {
		return err
	}

	cloudflaredConfig := defaultNamedTunnelConfigPath(layout)
	if err := writeNamedTunnelConfig(cloudflaredConfig, tunnelName, tunnelUUID, publicHost, local); err != nil {
		return err
	}
	fmt.Printf(msg("public.cloudflared_config"), cloudflaredConfig)

	cfg, runtimeInfo, err := buildPublicServerConfig(layout, selected, local, publicHost, endpoint)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	tunnel, err := startNamedTunnel(ctx, bin, cloudflaredConfig, tunnelName)
	if err != nil {
		return err
	}
	defer func() {
		_ = tunnel.Stop(5 * time.Second)
	}()
	return runPublicEngineUntilStopped(ctx, cfg, runtimeInfo, tunnel.done)
}

func runTokenCloudflarePublicServer(reader *bufio.Reader, layout app.RuntimeLayout, selected menuPlugin, bin string, local publicServerLocalSettings) error {
	if err := ensurePublicServerPorts(reader, local); err != nil {
		return err
	}
	fmt.Printf(msg("public.token_origin_hint"), local.OriginURL)
	token, err := readRequiredLine(reader, msg("public.token_prompt"))
	if err != nil {
		return err
	}
	hostname, err := readRequiredLine(reader, msg("public.public_hostname_prompt"))
	if err != nil {
		return err
	}

	endpoint, publicHost, err := endpointFromHostname(hostname)
	if err != nil {
		return err
	}

	cfg, runtimeInfo, err := buildPublicServerConfig(layout, selected, local, publicHost, endpoint)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	tunnel, err := startTokenTunnel(ctx, bin, token)
	if err != nil {
		return err
	}
	defer func() {
		_ = tunnel.Stop(5 * time.Second)
	}()
	return runPublicEngineUntilStopped(ctx, cfg, runtimeInfo, tunnel.done)
}

func runPublicEngineUntilStopped(ctx context.Context, cfg *config.Config, runtimeInfo publicServerRuntime, tunnelDone <-chan error) error {
	proxy := engine.New(cfg, logging.New(cfg.Logging.Format, "road-proxy-public"))
	engineDone := make(chan error, 1)
	go func() {
		engineDone <- proxy.Start(ctx)
	}()

	if err := waitForLocalDataPlane(ctx, engineDone, runtimeInfo.LocalOriginURL+"/api/ping"); err != nil {
		return err
	}
	printPublicServerReady(runtimeInfo)

	select {
	case <-ctx.Done():
		select {
		case err := <-engineDone:
			return err
		case <-time.After(6 * time.Second):
			return nil
		}
	case err := <-engineDone:
		return err
	case err := <-tunnelDone:
		if err == nil {
			return fmt.Errorf("cloudflared stopped")
		}
		return fmt.Errorf("cloudflared stopped: %w", err)
	}
}

func promptPublicServerLocalSettings(reader *bufio.Reader) (publicServerLocalSettings, error) {
	defaults := defaultPublicServerLocalSettings()
	for {
		fmt.Println()
		fmt.Println(msg("public.local_ports_header"))
		httpPort, err := readPortOverride(reader, msg("public.data_port"), defaults.HTTPPort, false)
		if err != nil {
			return publicServerLocalSettings{}, err
		}
		controlPort, err := readPortOverride(reader, msg("public.control_port"), defaults.ControlPort, false)
		if err != nil {
			return publicServerLocalSettings{}, err
		}
		local, err := newPublicServerLocalSettings(httpPort, controlPort)
		if err == nil {
			fmt.Printf(msg("public.local_origin_line"), local.OriginURL)
			return local, nil
		}
		fmt.Println(err)
	}
}

func ensurePublicServerPorts(reader *bufio.Reader, local publicServerLocalSettings) error {
	if err := ensurePortFree(reader, "tcp", local.HTTPPort); err != nil {
		return err
	}
	if err := ensurePortFree(reader, "tcp", local.ControlPort); err != nil {
		return err
	}
	return nil
}

func readRequiredLine(reader *bufio.Reader, prompt string) (string, error) {
	for {
		value, err := readLine(reader, prompt)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
		fmt.Println(msg("public.required_field"))
	}
}

func printPublicServerReady(info publicServerRuntime) {
	fmt.Println()
	fmt.Println(msg("public.ready"))
	fmt.Println(msg("public.share_header"))
	fmt.Printf(msg("public.client_endpoint_line"), info.Endpoint)
	fmt.Printf(msg("public.client_token_line"), info.Token)
	fmt.Printf(msg("public.client_plugin_line"), info.PluginName)
	fmt.Println()
	fmt.Println(msg("public.host_header"))
	fmt.Printf(msg("public.dashboard_line"), info.DashboardURL)
	fmt.Println()
	fmt.Println(msg("public.advanced_header"))
	fmt.Printf(msg("public.auth_header_line"), "X-ROAD-Token")
	fmt.Printf(msg("public.local_origin_line"), info.LocalOriginURL)
	fmt.Printf(msg("public.server_config_line"), info.ConfigPath)
	fmt.Printf(msg("public.client_config_line"), info.ClientConfigPath)
	fmt.Println()
	fmt.Println(msg("public.stop_hint"))
}

func waitForLocalDataPlane(ctx context.Context, errCh <-chan error, pingURL string) error {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return nil
		case <-tick.C:
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
			resp, err := client.Do(req)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 500 {
					return nil
				}
			}
		}
	}
}

func publicConfigBaseName(path string) string {
	return filepath.Base(path)
}
