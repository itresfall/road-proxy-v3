package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var tunnelUUIDPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

var tunnelAlreadyExistsErrors = []string{
	"already exists",
	"tunnel with name",
	"name is already used",
	"already registered",
}

var dnsAlreadyExistsErrors = []string{
	"record already exists",
	"already has a cname",
	"already exists",
	"an existing",
}

func startTokenTunnel(ctx context.Context, bin string, token string) (*cloudflaredProcess, error) {
	writer := &linePrefixWriter{out: os.Stdout, prefix: "[cloudflared] "}
	return startCloudflaredProcess(ctx, bin, []string{"tunnel", "run", "--token", token}, writer)
}

func runCloudflaredLoginIfNeeded(bin string) error {
	certPath := defaultCloudflaredCertPath()
	if certPath != "" {
		if info, err := os.Stat(certPath); err == nil && !info.IsDir() {
			fmt.Printf(msg("cloudflared.login_existing"), certPath)
			return nil
		}
	}
	fmt.Println(msg("cloudflared.login_start"))
	if err := runCloudflaredInteractive(bin, "tunnel", "login"); err != nil {
		return err
	}
	if certPath != "" {
		if _, err := os.Stat(certPath); err != nil {
			return fmt.Errorf("cloudflared login completed but cert.pem was not found: %s", certPath)
		}
	}
	return nil
}

type tunnelExistsError struct {
	Name   string
	UUID   string
	Output string
}

func (e *tunnelExistsError) Error() string {
	if e.UUID != "" {
		return fmt.Sprintf("tunnel %q already exists (UUID: %s)", e.Name, e.UUID)
	}
	return fmt.Sprintf("tunnel %q already exists", e.Name)
}

func createNamedTunnel(bin string, name string) (string, error) {
	output, err := runCloudflaredCapture(bin, "tunnel", "create", name)
	fmt.Print(output)
	if err != nil {
		if outputContainsAny(output, tunnelAlreadyExistsErrors) {
			return parseTunnelUUID(output), &tunnelExistsError{
				Name:   name,
				UUID:   parseTunnelUUID(output),
				Output: output,
			}
		}
		return "", fmt.Errorf("create tunnel failed: %w\n%s", err, strings.TrimSpace(output))
	}
	return parseTunnelUUID(output), nil
}

func createNamedTunnelOrReuse(reader *bufio.Reader, bin string, name string) (string, error) {
	uuid, err := createNamedTunnel(bin, name)
	if err == nil {
		return uuid, nil
	}

	var existsErr *tunnelExistsError
	if !errors.As(err, &existsErr) {
		return "", err
	}

	fmt.Printf(msg("cloudflared.tunnel_exists_warning"), existsErr.Error())
	reuse, promptErr := askYesNo(reader, msg("cloudflared.tunnel_reuse_prompt"), true)
	if promptErr != nil {
		return "", promptErr
	}
	if !reuse {
		return "", fmt.Errorf("existing tunnel was not reused")
	}
	if existsErr.UUID != "" {
		fmt.Printf(msg("cloudflared.tunnel_reusing_uuid"), existsErr.UUID)
		return existsErr.UUID, nil
	}

	listOutput, listErr := runCloudflaredCapture(bin, "tunnel", "list")
	if listErr == nil {
		if uuid := findTunnelUUIDByName(listOutput, name); uuid != "" {
			fmt.Printf(msg("cloudflared.tunnel_found_uuid"), uuid)
			return uuid, nil
		}
	}

	fmt.Println(msg("cloudflared.tunnel_uuid_missing"))
	return "", nil
}

func routeNamedTunnelDNS(bin string, tunnelName string, hostname string) error {
	return routeNamedTunnelDNSWithReader(bin, tunnelName, hostname, nil)
}

func routeNamedTunnelDNSWithReader(bin string, tunnelName string, hostname string, reader *bufio.Reader) error {
	output, err := runCloudflaredCapture(bin, "tunnel", "route", "dns", tunnelName, hostname)
	fmt.Print(output)
	if err == nil {
		return nil
	}

	if outputContainsAny(output, dnsAlreadyExistsErrors) {
		fmt.Printf(msg("cloudflared.dns_exists_warning"), hostname)
		if reader == nil {
			fmt.Println(msg("cloudflared.dns_reusing"))
			return nil
		}
		reuse, promptErr := askYesNo(reader, msg("cloudflared.dns_reuse_prompt"), true)
		if promptErr != nil {
			return promptErr
		}
		if reuse {
			fmt.Println(msg("cloudflared.dns_reusing"))
			return nil
		}
		return fmt.Errorf("existing DNS record was not reused")
	}

	return fmt.Errorf("add DNS route failed: %w\n%s", err, strings.TrimSpace(output))
}

func startNamedTunnel(ctx context.Context, bin string, configPath string, tunnelName string) (*cloudflaredProcess, error) {
	writer := &linePrefixWriter{out: os.Stdout, prefix: "[cloudflared] "}
	return startCloudflaredProcess(ctx, bin, []string{"tunnel", "--config", configPath, "run", tunnelName}, writer)
}

func parseTunnelUUID(output string) string {
	return tunnelUUIDPattern.FindString(output)
}

func findTunnelUUIDByName(listOutput string, name string) string {
	nameLower := strings.ToLower(strings.TrimSpace(name))
	for _, line := range strings.Split(listOutput, "\n") {
		if strings.Contains(strings.ToLower(line), nameLower) {
			if uuid := parseTunnelUUID(line); uuid != "" {
				return uuid
			}
		}
	}
	return ""
}

func defaultCloudflaredCertPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cloudflared", "cert.pem")
}

func defaultCloudflaredCredentialPath(uuid string) string {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cloudflared", uuid+".json")
}

func writeNamedTunnelConfig(path, tunnelName, tunnelUUID, hostname string, local publicServerLocalSettings) error {
	tunnelRef := strings.TrimSpace(tunnelUUID)
	if tunnelRef == "" {
		tunnelRef = strings.TrimSpace(tunnelName)
	}
	if tunnelRef == "" {
		return fmt.Errorf("tunnel name is required")
	}
	if strings.TrimSpace(hostname) == "" {
		return fmt.Errorf("hostname is required")
	}
	if strings.TrimSpace(local.OriginURL) == "" {
		return fmt.Errorf("origin URL is required")
	}
	if strings.TrimSpace(local.ControlURL) == "" {
		return fmt.Errorf("control URL is required")
	}

	var b strings.Builder
	b.WriteString("tunnel: ")
	b.WriteString(yamlQuote(tunnelRef))
	b.WriteString("\n")
	if cred := defaultCloudflaredCredentialPath(tunnelUUID); cred != "" {
		b.WriteString("credentials-file: ")
		b.WriteString(yamlQuote(filepath.ToSlash(cred)))
		b.WriteString("\n")
	}
	b.WriteString("ingress:\n")
	b.WriteString("  - hostname: ")
	b.WriteString(yamlQuote(hostname))
	b.WriteString("\n")
	b.WriteString("    path: ")
	b.WriteString(yamlQuote("/ws"))
	b.WriteString("\n")
	b.WriteString("    service: ")
	b.WriteString(yamlQuote(local.OriginURL))
	b.WriteString("\n")
	b.WriteString("  - hostname: ")
	b.WriteString(yamlQuote(hostname))
	b.WriteString("\n")
	b.WriteString("    path: ")
	b.WriteString(yamlQuote("/dashboard"))
	b.WriteString("\n")
	b.WriteString("    service: ")
	b.WriteString(yamlQuote(local.ControlURL))
	b.WriteString("\n")
	b.WriteString("  - hostname: ")
	b.WriteString(yamlQuote(hostname))
	b.WriteString("\n")
	b.WriteString("    path: ")
	b.WriteString(yamlQuote("/api/.*"))
	b.WriteString("\n")
	b.WriteString("    service: ")
	b.WriteString(yamlQuote(local.ControlURL))
	b.WriteString("\n")
	b.WriteString("  - hostname: ")
	b.WriteString(yamlQuote(hostname))
	b.WriteString("\n")
	b.WriteString("    service: ")
	b.WriteString(yamlQuote(local.OriginURL))
	b.WriteString("\n")
	b.WriteString("  - service: http_status:404\n")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func yamlQuote(raw string) string {
	escaped := strings.ReplaceAll(raw, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func outputContainsAny(output string, patterns []string) bool {
	lower := strings.ToLower(output)
	for _, pattern := range patterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}
