package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"road-proxy-v3/internal/app"
)

func runInteractiveStudio(layout app.RuntimeLayout) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println(sm("studio.title"))
	fmt.Println("=====================")
	fmt.Printf(sm("common.runtime_line"), layout.Root)
	fmt.Println("")

	candidates, err := discoverCandidates()
	if err != nil {
		fmt.Printf(sm("studio.error_scan_failed"), err)
		return
	}
	if len(candidates) == 0 {
		fmt.Println(sm("studio.no_process"))
		return
	}

	fmt.Println(sm("studio.active_processes"))
	const maxDisplayProcesses = 120
	limit := len(candidates)
	if limit > maxDisplayProcesses {
		limit = maxDisplayProcesses
	}
	for i := 0; i < limit; i++ {
		c := candidates[i]
		fmt.Printf("  %d) PID=%d %s | tcp=%d udp=%d\n", i+1, c.PID, c.Name, c.TCPCount, c.UDPCount)
	}
	fmt.Println("")

	choice, err := readChoice(reader, fmt.Sprintf(sm("studio.process_choice"), limit), 1, limit, 1)
	if err != nil {
		fmt.Printf(sm("studio.error"), err)
		return
	}
	selected := candidates[choice-1]

	defaultSeconds := 20
	seconds, err := readIntWithDefault(reader, fmt.Sprintf(sm("studio.capture_seconds"), defaultSeconds), defaultSeconds)
	if err != nil {
		fmt.Printf(sm("studio.error"), err)
		return
	}
	if seconds < 5 {
		seconds = 5
	}

	multiPhase, err := askYesNo(reader, sm("studio.multi_phase_prompt"), false)
	if err != nil {
		fmt.Printf(sm("studio.error"), err)
		return
	}

	var summary *captureSummary
	if multiPhase {
		defaultPhaseSeconds := 8
		phaseSeconds, err := readIntWithDefault(reader, fmt.Sprintf(sm("studio.phase_seconds"), defaultPhaseSeconds), defaultPhaseSeconds)
		if err != nil {
			fmt.Printf(sm("studio.error"), err)
			return
		}
		if phaseSeconds < 5 {
			phaseSeconds = 5
		}
		fmt.Println("")
		summary, err = captureProcessPhases(selected.PID, selected.Name, phaseSeconds, time.Second, func(phase capturePhase) error {
			_, readErr := readText(reader, fmt.Sprintf(sm("studio.phase_ready_prompt"), sm(phase.LabelKey), sm(phase.InstructionKey)))
			return readErr
		})
	} else {
		fmt.Println("")
		fmt.Printf(sm("studio.capture_started"), selected.PID, selected.Name, seconds)
		fmt.Println(sm("studio.capture_instruction"))
		fmt.Println("")

		summary, err = captureProcess(selected.PID, selected.Name, time.Duration(seconds)*time.Second, time.Second)
	}
	if err != nil {
		fmt.Printf(sm("studio.error_capture_failed"), err)
		return
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

	fmt.Println(sm("studio.pre_analysis"))
	printTopPorts("tcp", summary.TopLocalPorts["tcp"])
	printTopPorts("udp", summary.TopLocalPorts["udp"])
	fmt.Printf(sm("studio.recommendation"), network, port)
	if profile != nil {
		fmt.Printf(sm("studio.compat_profile"), profile.DisplayName, profile.ID)
		fmt.Printf(sm("studio.compat_confidence"), profileMatch.Confidence)
	}
	fmt.Println("")

	netInput, err := readText(reader, fmt.Sprintf(sm("studio.network_prompt"), network))
	if err != nil {
		fmt.Printf(sm("studio.error"), err)
		return
	}
	netInput = strings.ToLower(strings.TrimSpace(netInput))
	if netInput == "tcp" || netInput == "udp" {
		network = netInput
	}

	hostInput, err := readText(reader, fmt.Sprintf(sm("studio.host_prompt"), host))
	if err != nil {
		fmt.Printf(sm("studio.error"), err)
		return
	}
	hostInput = strings.TrimSpace(hostInput)
	if hostInput != "" {
		host = hostInput
	}

	portInput, err := readIntWithDefault(reader, fmt.Sprintf(sm("studio.target_port_prompt"), port), port)
	if err != nil {
		fmt.Printf(sm("studio.error"), err)
		return
	}
	if portInput > 0 {
		port = portInput
	}
	if port <= 0 {
		fmt.Println(sm("studio.valid_port_required"))
		return
	}

	clientListenPortDefault := port
	if profile != nil && profile.ClientListenPort > 0 {
		clientListenPortDefault = profile.ClientListenPort
	}
	clientListenPort, err := readIntWithDefault(reader, fmt.Sprintf(sm("studio.client_listen_port_prompt"), clientListenPortDefault), clientListenPortDefault)
	if err != nil {
		fmt.Printf(sm("studio.error"), err)
		return
	}
	if clientListenPort <= 0 {
		clientListenPort = port
	}

	defaultPluginName := suggestPluginName(selected.Name, network)
	if profile != nil && profile.PluginName != "" {
		defaultPluginName = profile.PluginName
	}
	pluginName, err := readText(reader, fmt.Sprintf(sm("studio.plugin_name_prompt"), defaultPluginName))
	if err != nil {
		fmt.Printf(sm("studio.error"), err)
		return
	}
	pluginName = sanitizePluginName(pluginName)
	if pluginName == "" {
		pluginName = defaultPluginName
	}
	if pluginName == "" {
		fmt.Println(sm("studio.plugin_name_required"))
		return
	}

	notes := mergeNotes(profileNotes(profile), fallbackProfileNotes(profile, network))
	if len(notes) > 0 {
		fmt.Println("")
		fmt.Println(sm("studio.profile_notes"))
		for _, note := range notes {
			fmt.Printf("  - %s\n", note)
		}
	}

	udpPeerBroadcast := false
	if profile != nil {
		udpPeerBroadcast = profile.UDPPeerBroadcast
	}
	if network == "udp" {
		fmt.Println("")
		fmt.Println(sm("studio.udp_peer_warning"))
		fmt.Println(sm("studio.gzdoom_netmode_advice"))
		udpPeerBroadcast, err = askYesNo(reader, sm("studio.udp_peer_prompt"), udpPeerBroadcast)
		if err != nil {
			fmt.Printf(sm("studio.error"), err)
			return
		}
	}

	pluginPath := filepath.Join(layout.PluginDir, pluginName, "plugin.json")
	if _, err := os.Stat(pluginPath); err == nil {
		overwrite, askErr := askYesNo(reader, fmt.Sprintf(sm("studio.overwrite_prompt"), pluginPath), false)
		if askErr != nil {
			fmt.Printf(sm("studio.error"), askErr)
			return
		}
		if !overwrite {
			fmt.Println(sm("studio.cancelled"))
			return
		}
	}

	targetAddress := fmt.Sprintf("%s:%d", host, port)
	pluginDoc := buildPluginDoc(pluginName, network, targetAddress, selected.Name, udpPeerBroadcast, notes, profile)
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		fmt.Printf(sm("studio.error_plugin_dir"), err)
		return
	}
	pluginJSON, err := json.MarshalIndent(pluginDoc, "", "  ")
	if err != nil {
		fmt.Printf(sm("studio.error_plugin_json"), err)
		return
	}
	pluginJSON = append(pluginJSON, '\n')
	if err := os.WriteFile(pluginPath, pluginJSON, 0o644); err != nil {
		fmt.Printf(sm("studio.error_plugin_write"), err)
		return
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
	summary.UDPPeerBroadcast = udpPeerBroadcast
	summary.Notes = notes
	reportPath := filepath.Join(layout.PluginDir, pluginName, "studio-report.json")
	reportJSON, _ := json.MarshalIndent(summary, "", "  ")
	if len(reportJSON) > 0 {
		reportJSON = append(reportJSON, '\n')
		_ = os.WriteFile(reportPath, reportJSON, 0o644)
	}
	if profile == nil {
		unknownPath := filepath.Join(layout.PluginDir, pluginName, "unknown-game-report.json")
		_ = writeUnknownGameReport(unknownPath, buildUnknownGameReport(selected, pluginName, summary, notes))
	}

	clientListenAddr := fmt.Sprintf("127.0.0.1:%d", clientListenPort)
	serverConfigPath := filepath.Join(layout.ConfigDir, fmt.Sprintf("server-%s.json", pluginName))
	clientConfigPath := filepath.Join(layout.ConfigDir, fmt.Sprintf("client-%s.json", pluginName))
	serverConfigWritten, serverConfigErr := writeJSONIfMissing(serverConfigPath, buildServerConfigDoc(pluginName))
	if serverConfigErr != nil {
		fmt.Printf(sm("studio.error_server_config"), serverConfigErr)
		return
	}
	clientConfigWritten, clientConfigErr := writeJSONIfMissing(clientConfigPath, buildClientConfigDoc(pluginName, network, clientListenAddr))
	if clientConfigErr != nil {
		fmt.Printf(sm("studio.error_client_config"), clientConfigErr)
		return
	}

	fmt.Println("")
	fmt.Println(sm("studio.created"))
	fmt.Printf(sm("studio.output_plugin"), pluginPath)
	fmt.Printf(sm("studio.output_report"), reportPath)
	printGeneratedConfig(sm("studio.output_server_config"), serverConfigPath, serverConfigWritten)
	printGeneratedConfig(sm("studio.output_client_config"), clientConfigPath, clientConfigWritten)
	fmt.Println("")
	fmt.Println(sm("studio.next_step"))
}
