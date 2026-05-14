package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/config"
)

type configProfileOverride struct {
	Name   string         `json:"name"`
	Server map[string]any `json:"server"`
	Client map[string]any `json:"client"`
}

func runGenerateConfigCommand(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("generate-config", flag.ContinueOnError)
	fs.SetOutput(out)

	profileName := fs.String("profile", "", msg("generate.flag_profile"))
	profileDir := fs.String("profiles", "configs/profiles", msg("generate.flag_profiles"))
	baseServerPath := fs.String("base-server", "configs/base/server.json", msg("generate.flag_base_server"))
	baseClientPath := fs.String("base-client", "configs/base/client.json", msg("generate.flag_base_client"))
	outputDir := fs.String("out", "configs/.generated", msg("generate.flag_out"))
	prefix := fs.String("prefix", "", msg("generate.flag_prefix"))
	clientInstances := fs.Int("client-instances", 1, msg("generate.flag_client_instances"))
	clientStartPort := fs.Int("client-start-port", 0, msg("generate.flag_client_start_port"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf(msg("generate.unexpected_args"), strings.Join(fs.Args(), " "))
	}

	name := strings.TrimSpace(*profileName)
	if name == "" {
		return fmt.Errorf(msg("generate.profile_required"))
	}
	if *prefix == "" {
		*prefix = name
	}
	if *clientInstances < 1 {
		return fmt.Errorf(msg("generate.client_instances_invalid"), *clientInstances)
	}
	if *clientStartPort < 0 || *clientStartPort > 65535 {
		return fmt.Errorf(msg("generate.client_start_port_invalid"), *clientStartPort)
	}

	profilePath := filepath.Join(app.ResolveExistingPath(*profileDir), name+".json")
	profile, err := loadConfigProfileOverride(profilePath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(profile.Name) != "" {
		name = strings.TrimSpace(profile.Name)
	}

	serverDoc, err := mergeConfigDocument(app.ResolveExistingPath(*baseServerPath), profile.Server)
	if err != nil {
		return err
	}
	clientDoc, err := mergeConfigDocument(app.ResolveExistingPath(*baseClientPath), profile.Client)
	if err != nil {
		return err
	}

	outDir := app.ResolveExistingPath(*outputDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	serverOut := filepath.Join(outDir, fmt.Sprintf("server-%s.json", sanitizeGeneratedConfigName(*prefix)))

	if err := writeJSONFile(serverOut, serverDoc, "server config"); err != nil {
		return err
	}
	if _, err := config.Load(serverOut); err != nil {
		return err
	}

	clientOutputs, err := writeGeneratedClientConfigs(outDir, sanitizeGeneratedConfigName(*prefix), clientDoc, *clientInstances, *clientStartPort)
	if err != nil {
		return err
	}
	for _, path := range clientOutputs {
		if _, err := config.LoadClient(path); err != nil {
			return err
		}
	}

	fmt.Fprintf(out, msg("generate.ok_server"), serverOut)
	for _, path := range clientOutputs {
		fmt.Fprintf(out, msg("generate.ok_client"), path)
	}
	fmt.Fprintf(out, msg("generate.ok_profile"), name)
	return nil
}

func writeGeneratedClientConfigs(
	outDir string,
	prefix string,
	clientDoc map[string]any,
	instances int,
	startPort int,
) ([]string, error) {
	outputs := make([]string, 0, instances)
	for i := 0; i < instances; i++ {
		doc := cloneJSONMap(clientDoc)
		if instances > 1 || startPort > 0 {
			if err := setClientInstanceListenAddr(doc, startPort, i); err != nil {
				return nil, err
			}
		}

		name := fmt.Sprintf("client-%s.json", prefix)
		if instances > 1 {
			name = fmt.Sprintf("client-%s-p%d.json", prefix, i+1)
		}
		path := filepath.Join(outDir, name)
		if err := writeJSONFile(path, doc, "client config"); err != nil {
			return nil, err
		}
		outputs = append(outputs, path)
	}
	return outputs, nil
}

func setClientInstanceListenAddr(doc map[string]any, startPort int, index int) error {
	raw, ok := doc["listen_addr"].(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return fmt.Errorf("client.listen_addr is required for multi-client generation")
	}
	host, portRaw, err := net.SplitHostPort(raw)
	if err != nil {
		return fmt.Errorf("client.listen_addr parse failed: %w", err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return fmt.Errorf("client.listen_addr port parse failed: %w", err)
	}
	if startPort > 0 {
		port = startPort
	}
	port += index
	if port < 1 || port > 65535 {
		return fmt.Errorf(msg("generate.client_port_overflow"), port)
	}
	doc["listen_addr"] = net.JoinHostPort(host, strconv.Itoa(port))
	return nil
}

func cloneJSONMap(src map[string]any) map[string]any {
	data, err := json.Marshal(src)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	if out == nil {
		return map[string]any{}
	}
	return out
}

func loadConfigProfileOverride(path string) (configProfileOverride, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return configProfileOverride{}, err
	}
	var profile configProfileOverride
	if err := json.Unmarshal(data, &profile); err != nil {
		return configProfileOverride{}, err
	}
	return profile, nil
}

func mergeConfigDocument(basePath string, override map[string]any) (map[string]any, error) {
	base, err := loadJSONMap(basePath)
	if err != nil {
		return nil, err
	}
	return mergeJSONMaps(base, override), nil
}

func loadJSONMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

func mergeJSONMaps(base, override map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(override))
	for key, value := range base {
		out[key] = value
	}
	for key, overrideValue := range override {
		if overrideMap, ok := overrideValue.(map[string]any); ok {
			if baseMap, ok := out[key].(map[string]any); ok {
				out[key] = mergeJSONMaps(baseMap, overrideMap)
				continue
			}
		}
		out[key] = overrideValue
	}
	return out
}

func sanitizeGeneratedConfigName(raw string) string {
	name := strings.TrimSpace(strings.ToLower(raw))
	if name == "" {
		return "profile"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range name {
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
		return "profile"
	}
	return out
}
