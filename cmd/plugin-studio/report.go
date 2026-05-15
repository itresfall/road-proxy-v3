package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"road-proxy-v3/internal/plugin"
	"road-proxy-v3/internal/version"
	"strconv"
	"strings"
	"time"
)

func suggestPluginName(processName, network string) string {
	base := strings.TrimSpace(processName)
	base = strings.TrimSuffix(base, ".exe")
	base = sanitizePluginName(base)
	if base == "" {
		base = "custom-game"
	}
	if network == "" {
		network = "tcp"
	}
	return sanitizePluginName(base + "-" + network)
}

func sanitizePluginName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteRune('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return ""
	}
	return out
}

func buildPluginDoc(pluginName, network, targetAddress, processName string, udpPeerBroadcast bool, notes []string, profile *compatProfile) map[string]any {
	serverTemplate, clientTemplate := generatedConfigMenuPaths(pluginName)
	supportsMultiplex := false
	if network == "udp" {
		supportsMultiplex = true
	}

	description := fmt.Sprintf("Auto-generated profile from Plugin Studio (%s)", processName)
	runtimeDoc := map[string]any{
		"type":               "json",
		"mode":               "passthrough",
		"enable_obfuscation": false,
		"udp_peer_broadcast": network == "udp" && udpPeerBroadcast,
		"client_pipeline":    []string{},
		"server_pipeline":    []string{},
	}
	if network == "udp" {
		runtimeDoc["udp_reply_policy"] = udpReplyPolicyForProfile(profile)
	}

	doc := map[string]any{
		"schema_version": "v1",
		"name":           pluginName,
		"version":        "3.0.0",
		"description":    description,
		"author":         "plugin-studio",
		"protocols": map[string]any{
			"supported": []string{network, "websocket"},
		},
		"target": map[string]any{
			"network": network,
			"address": targetAddress,
		},
		"menu": map[string]any{
			"server_config": serverTemplate,
			"client_config": clientTemplate,
		},
		"capabilities": map[string]any{
			"supports_reconnect": true,
			"supports_multiplex": supportsMultiplex,
		},
		"compatibility": buildCompatibilityDoc(network, targetAddress, notes, profile),
		"runtime":       runtimeDoc,
	}

	if len(notes) > 0 {
		doc["studio"] = map[string]any{
			"process_name": processName,
			"notes_source": studioNotesSource(profile),
		}
	}
	return doc
}

func generatedConfigNames(pluginName string) (serverName string, clientName string) {
	return fmt.Sprintf("server-%s.json", pluginName), fmt.Sprintf("client-%s.json", pluginName)
}

func generatedConfigMenuPaths(pluginName string) (serverPath string, clientPath string) {
	serverName, clientName := generatedConfigNames(pluginName)
	return fmt.Sprintf("configs/%s", serverName), fmt.Sprintf("configs/%s", clientName)
}

func udpReplyPolicyForProfile(profile *compatProfile) string {
	if profile != nil {
		switch profile.UDPReplyPolicy {
		case plugin.UDPReplyPolicyAny, plugin.UDPReplyPolicySameIP, plugin.UDPReplyPolicyStrict:
			return profile.UDPReplyPolicy
		}
	}
	return plugin.UDPReplyPolicyAny
}

func buildCompatibilityDoc(network, targetAddress string, notes []string, profile *compatProfile) map[string]any {
	doc := map[string]any{
		"status":         "experimental",
		"tested_players": 0,
		"last_verified":  time.Now().Format("2006-01-02"),
	}
	if profile != nil {
		doc["profile_id"] = profile.ID
		doc["notes_source"] = studioNotesSource(profile)
	}
	if host, portRaw, err := net.SplitHostPort(targetAddress); err == nil {
		if port, convErr := strconv.Atoi(portRaw); convErr == nil && port > 0 {
			doc["known_ports"] = []map[string]any{
				{
					"network": network,
					"port":    port,
					"role":    "target",
					"notes":   fmt.Sprintf("%s:%d", host, port),
				},
			}
		}
	}
	if profile == nil && len(notes) > 0 {
		doc["notes"] = notes
	}
	return doc
}

func studioNotesSource(profile *compatProfile) string {
	if profile == nil || profile.ID == "" {
		return "generated"
	}
	return "compat-profiles/" + profile.ID + ".json"
}

func fallbackProfileNotes(profile *compatProfile, network string) []string {
	notes := []string{}
	if profile == nil && network == "udp" {
		notes = append(notes, sm("studio.note.udp_peer_broadcast_default"))
	}
	return notes
}

func buildPortSelectionReport(summary *captureSummary, selectedNetwork string, selectedPort int, match compatMatch) *portSelectionReport {
	if summary == nil || selectedPort <= 0 || selectedNetwork == "" {
		return nil
	}

	reason := "capture_recommendation"
	if match.Profile != nil && match.Profile.TargetPort == selectedPort && match.Profile.Network == selectedNetwork {
		reason = "compat_profile_target"
	}

	report := &portSelectionReport{
		SelectedNetwork: selectedNetwork,
		SelectedPort:    selectedPort,
		Reason:          reason,
		Rejected:        rejectedPortCandidates(summary, selectedNetwork, selectedPort),
	}
	return report
}

func rejectedPortCandidates(summary *captureSummary, selectedNetwork string, selectedPort int) []portCandidateReport {
	out := []portCandidateReport{}
	add := func(source, network string, pairs []pair) {
		for _, candidate := range pairs {
			if len(out) >= 12 {
				return
			}
			if candidate.Port <= 0 || (network == selectedNetwork && candidate.Port == selectedPort) {
				continue
			}
			out = append(out, portCandidateReport{
				Network: network,
				Port:    candidate.Port,
				Hits:    candidate.Hits,
				Source:  source,
				Reason:  rejectedPortReason(network, candidate.Port, selectedNetwork, selectedPort),
			})
		}
	}
	add("local", "tcp", summary.TopLocalPorts["tcp"])
	add("local", "udp", summary.TopLocalPorts["udp"])
	add("remote", "tcp", summary.TopRemotePorts["tcp"])
	add("remote", "udp", summary.TopRemotePorts["udp"])
	return out
}

func rejectedPortReason(network string, port int, selectedNetwork string, selectedPort int) string {
	if isLikelyNoisePort(port) {
		return "likely_noise_port"
	}
	if isEphemeralPort(port) {
		return "ephemeral_port"
	}
	if network != selectedNetwork {
		return "different_protocol"
	}
	if port != selectedPort {
		return "lower_priority_candidate"
	}
	return "not_selected"
}

func buildUnknownGameReport(selected processCandidate, pluginName string, summary *captureSummary, notes []string) unknownGameReport {
	report := unknownGameReport{
		SchemaVersion: "unknown_game_report.v1",
		ProcessName:   selected.Name,
		ProcessPID:    selected.PID,
		PluginName:    pluginName,
		CaptureDate:   time.Now().UTC().Format(time.RFC3339),
		ROADVersion:   version.String("plugin-studio"),
		Notes:         notes,
	}
	if summary != nil {
		report.ProcessPath = summary.ProcessPath
		report.ProcessSHA256 = summary.ProcessSHA256
		report.SteamAppID = summary.SteamAppID
		report.SteamAppIDSource = summary.SteamAppIDSource
		report.RecommendedNetwork = summary.RecommendedNet
		report.RecommendedPort = summary.RecommendedPort
		report.TopLocalPorts = summary.TopLocalPorts
		report.TopRemotePorts = summary.TopRemotePorts
		report.PortSelection = summary.PortSelection
		report.MultiPhase = summary.MultiPhase
		report.PacketFingerprint = summary.PacketFingerprint
		report.Topology = summary.Topology
	}
	return report
}

func writeUnknownGameReport(path string, report unknownGameReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func buildServerConfigDoc(pluginName string) map[string]any {
	return map[string]any{
		"tcp": map[string]any{
			"listen_addr":       "0.0.0.0:0",
			"buffer_size":       32768,
			"dial_timeout":      "5s",
			"keep_alive_period": "30s",
		},
		"http": map[string]any{
			"enabled":              true,
			"listen_addr":          "0.0.0.0:8080",
			"ws_endpoint":          "/ws",
			"ws_idle_timeout":      "10m0s",
			"ws_ping_interval":     "30s",
			"max_ws_message_bytes": 1048576,
			"enable_compression":   false,
			"read_timeout":         "10s",
			"write_timeout":        "10s",
			"handshake_timeout":    "5s",
			"read_header_timeout":  "5s",
			"max_header_bytes":     1048576,
		},
		"control": map[string]any{
			"enabled":             true,
			"listen_addr":         "0.0.0.0:8081",
			"read_timeout":        "10s",
			"write_timeout":       "10s",
			"read_header_timeout": "5s",
			"max_header_bytes":    1048576,
		},
		"plugins": map[string]any{
			"dir":     "plugins",
			"enabled": []string{pluginName},
		},
	}
}

func buildClientConfigDoc(pluginName, network, listenAddr string) map[string]any {
	return map[string]any{
		"listen_addr":              listenAddr,
		"listen_network":           network,
		"server_ws_url":            "ws://127.0.0.1:8080/ws",
		"plugin_name":              pluginName,
		"connect_retries":          6,
		"retry_initial_delay":      "100ms",
		"retry_max_delay":          "600ms",
		"ws_idle_timeout":          "10m0s",
		"ws_ping_interval":         "30s",
		"udp_session_idle_timeout": "10m0s",
		"udp_metrics_log_interval": "10s",
		"max_ws_message_bytes":     1048576,
		"buffer_size":              32768,
		"handshake_timeout":        "5s",
		"read_timeout":             "30s",
		"write_timeout":            "30s",
		"enable_compression":       false,
		"headers":                  map[string]string{},
	}
}

func writeJSONIfMissing(path string, doc any) (bool, error) {
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("target is a directory: %q", path)
		}
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("check %q: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create parent dir: %w", err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, fmt.Errorf("write %q: %w", path, err)
	}
	return true, nil
}

func printGeneratedConfig(label, path string, written bool) {
	state := sm("studio.config_kept")
	if written {
		state = sm("studio.config_created")
	}
	fmt.Printf("%s: %s (%s)\n", label, path, state)
}
