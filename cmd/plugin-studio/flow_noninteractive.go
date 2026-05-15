package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"road-proxy-v3/internal/app"
)

func runNonInteractiveStudio(layout app.RuntimeLayout, opts studioCLIOptions) error {
	if opts.PID <= 0 && opts.Process == "" {
		return fmt.Errorf("non-interactive mode requires --pid or --process")
	}

	candidates, err := discoverCandidates()
	if err != nil {
		return err
	}
	selected, err := selectCLIProcessCandidate(candidates, opts)
	if err != nil {
		return err
	}

	seconds := opts.Seconds
	if seconds < 5 {
		seconds = 5
	}
	phaseSeconds := opts.PhaseSeconds
	if phaseSeconds < 5 {
		phaseSeconds = 5
	}
	var summary *captureSummary
	if opts.MultiPhase {
		summary, err = captureProcessPhases(selected.PID, selected.Name, phaseSeconds, time.Second, nil)
	} else {
		summary, err = captureProcess(selected.PID, selected.Name, time.Duration(seconds)*time.Second, time.Second)
	}
	if err != nil {
		return fmt.Errorf("capture failed: %w", err)
	}
	enrichCaptureWithProcessSignals(summary, selected)

	network := summary.RecommendedNet
	if network == "" {
		network = "tcp"
	}
	port := summary.RecommendedPort
	if port == 0 {
		if selectedPort := firstPort(selected.LocalTCP); selectedPort > 0 {
			port = selectedPort
			network = "tcp"
		} else if selectedPort := firstPort(selected.LocalUDP); selectedPort > 0 {
			port = selectedPort
			network = "udp"
		}
	}
	host := "127.0.0.1"

	profileMatch := matchCompatProfileWithReasons(selected.Name, summary)
	profile := profileMatch.Profile
	if profile != nil {
		if profile.Network != "" {
			network = profile.Network
		}
		if profile.TargetHost != "" {
			host = profile.TargetHost
		}
		if profile.TargetPort > 0 {
			port = profile.TargetPort
		}
	}

	if opts.Network != "" {
		network = strings.ToLower(strings.TrimSpace(opts.Network))
	}
	if network != "tcp" && network != "udp" {
		return fmt.Errorf("network must be tcp or udp")
	}
	if opts.TargetHost != "" {
		host = opts.TargetHost
	}
	if opts.TargetPort > 0 {
		port = opts.TargetPort
	}
	if port <= 0 || port > 65535 {
		return fmt.Errorf("target port must be in 1-65535")
	}

	clientListenPort := port
	if profile != nil && profile.ClientListenPort > 0 {
		clientListenPort = profile.ClientListenPort
	}
	if opts.ClientListenPort > 0 {
		clientListenPort = opts.ClientListenPort
	}
	if clientListenPort <= 0 || clientListenPort > 65535 {
		return fmt.Errorf("client listen port must be in 1-65535")
	}

	pluginName := suggestPluginName(selected.Name, network)
	if profile != nil && profile.PluginName != "" {
		pluginName = profile.PluginName
	}
	if opts.PluginName != "" {
		pluginName = sanitizePluginName(opts.PluginName)
	}
	if pluginName == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	notes := mergeNotes(profileNotes(profile), fallbackProfileNotes(profile, network))
	udpPeerBroadcast := false
	if profile != nil {
		udpPeerBroadcast = profile.UDPPeerBroadcast
	}
	if opts.UDPPeerBroadcast.set {
		udpPeerBroadcast = opts.UDPPeerBroadcast.value
	}

	targetAddress := fmt.Sprintf("%s:%d", host, port)
	pluginDoc := buildPluginDoc(pluginName, network, targetAddress, selected.Name, udpPeerBroadcast, notes, profile)
	pluginPath := filepath.Join(layout.PluginDir, pluginName, "plugin.json")
	if _, err := os.Stat(pluginPath); err == nil && !opts.Force {
		return fmt.Errorf("plugin already exists: %s (use --force to overwrite)", pluginPath)
	}
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		return fmt.Errorf("create plugin dir: %w", err)
	}
	pluginJSON, err := json.MarshalIndent(pluginDoc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugin json: %w", err)
	}
	pluginJSON = append(pluginJSON, '\n')
	if err := os.WriteFile(pluginPath, pluginJSON, 0o644); err != nil {
		return fmt.Errorf("write plugin: %w", err)
	}

	summary.RecommendedNet = network
	summary.RecommendedPort = port
	if profile != nil {
		summary.CompatProfile = profile.ID
	}
	summary.CompatConfidence = profileMatch.Confidence
	summary.CompatReasons = profileMatch.Reasons
	summary.PortSelection = buildPortSelectionReport(summary, network, port, profileMatch)
	summary.ClientListenPort = clientListenPort
	summary.UDPPeerBroadcast = network == "udp" && udpPeerBroadcast
	summary.Notes = notes

	reportPath := filepath.Join(layout.PluginDir, pluginName, "studio-report.json")
	reportJSON, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal studio report: %w", err)
	}
	reportJSON = append(reportJSON, '\n')
	if err := os.WriteFile(reportPath, reportJSON, 0o644); err != nil {
		return fmt.Errorf("write studio report: %w", err)
	}
	if profile == nil {
		unknownPath := filepath.Join(layout.PluginDir, pluginName, "unknown-game-report.json")
		if err := writeUnknownGameReport(unknownPath, buildUnknownGameReport(selected, pluginName, summary, notes)); err != nil {
			return fmt.Errorf("write unknown game report: %w", err)
		}
	}

	clientListenAddr := fmt.Sprintf("127.0.0.1:%d", clientListenPort)
	serverConfigName, clientConfigName := generatedConfigNames(pluginName)
	serverConfigPath := filepath.Join(layout.ConfigDir, serverConfigName)
	clientConfigPath := filepath.Join(layout.ConfigDir, clientConfigName)
	serverConfigWritten, err := writeJSONIfMissing(serverConfigPath, buildServerConfigDoc(pluginName))
	if err != nil {
		return fmt.Errorf("write server config: %w", err)
	}
	clientConfigWritten, err := writeJSONIfMissing(clientConfigPath, buildClientConfigDoc(pluginName, network, clientListenAddr))
	if err != nil {
		return fmt.Errorf("write client config: %w", err)
	}

	fmt.Println(sm("studio.created"))
	fmt.Printf(sm("studio.output_plugin"), pluginPath)
	fmt.Printf(sm("studio.output_report"), reportPath)
	printGeneratedConfig(sm("studio.output_server_config"), serverConfigPath, serverConfigWritten)
	printGeneratedConfig(sm("studio.output_client_config"), clientConfigPath, clientConfigWritten)
	return nil
}

func selectCLIProcessCandidate(candidates []processCandidate, opts studioCLIOptions) (processCandidate, error) {
	if opts.PID > 0 {
		for _, candidate := range candidates {
			if candidate.PID == opts.PID {
				return candidate, nil
			}
		}
		return processCandidate{}, fmt.Errorf("no process with network activity found for PID %d", opts.PID)
	}

	needle := normalizeProfileText(opts.Process)
	for _, candidate := range candidates {
		name := normalizeProfileText(candidate.Name)
		if name == needle || strings.Contains(name, needle) {
			return candidate, nil
		}
	}
	return processCandidate{}, fmt.Errorf("no process with network activity matched %q", opts.Process)
}
