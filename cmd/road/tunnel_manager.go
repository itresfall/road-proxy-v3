package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type cloudflaredProcess struct {
	cmd     *exec.Cmd
	done    chan error
	cleanup func()
}

func ensureCloudflared(reader *bufio.Reader) (string, error) {
	if path, ok := findCloudflaredBinary(); ok {
		return path, nil
	}

	fmt.Println(msg("cloudflared.not_found"))
	download, err := askYesNo(reader, msg("cloudflared.download_prompt"), true)
	if err != nil {
		return "", err
	}
	if !download {
		return "", fmt.Errorf(msg("cloudflared.required"))
	}
	return downloadCloudflared(reader)
}

func findCloudflaredBinary() (string, bool) {
	if path, err := exec.LookPath("cloudflared"); err == nil {
		return path, true
	}
	if runtime.GOOS == "windows" {
		if path, err := exec.LookPath("cloudflared.exe"); err == nil {
			return path, true
		}
	}
	path := cloudflaredInstallPath()
	if path == "" {
		return "", false
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path, true
	}
	return "", false
}

func cloudflaredInstallPath() string {
	switch runtime.GOOS {
	case "windows":
		base := strings.TrimSpace(os.Getenv("APPDATA"))
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return ""
			}
			base = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(base, "road-proxy", "bin", "cloudflared.exe")
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(home, ".local", "bin", "cloudflared")
	default:
		return ""
	}
}

func cloudflaredDownloadURL() (string, error) {
	base := "https://github.com/cloudflare/cloudflared/releases/latest/download/"
	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return base + "cloudflared-windows-amd64.exe", nil
		case "386":
			return base + "cloudflared-windows-386.exe", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return base + "cloudflared-linux-amd64", nil
		case "arm64":
			return base + "cloudflared-linux-arm64", nil
		}
	}
	return "", fmt.Errorf("cloudflared auto-download unsupported on %s/%s", runtime.GOOS, runtime.GOARCH)
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Body    string        `json:"body"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var sha256Pattern = regexp.MustCompile(`(?i)\b[0-9a-f]{64}\b`)

func cloudflaredDownloadAssetName() (string, error) {
	rawURL, err := cloudflaredDownloadURL()
	if err != nil {
		return "", err
	}
	parts := strings.Split(rawURL, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[len(parts)-1]) == "" {
		return "", fmt.Errorf("cloudflared download asset name could not be resolved")
	}
	return parts[len(parts)-1], nil
}

func fetchCloudflaredChecksum(binaryName string) (string, error) {
	const apiURL = "https://api.github.com/repos/cloudflare/cloudflared/releases/latest"
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "road-proxy-v3")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api: HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("github api decode: %w", err)
	}

	if sum := parseChecksumText(release.Body, binaryName); sum != "" {
		return sum, nil
	}

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.TrimSpace(asset.BrowserDownloadURL) == "" {
			continue
		}
		if name != strings.ToLower(binaryName)+".sha256sum" &&
			name != strings.ToLower(binaryName)+".sha256" &&
			name != "sha256sums" &&
			name != "sha256sums.txt" {
			continue
		}
		content, err := downloadText(client, asset.BrowserDownloadURL)
		if err != nil {
			return "", err
		}
		if sum := parseChecksumText(content, binaryName); sum != "" {
			return sum, nil
		}
	}

	if strings.TrimSpace(release.TagName) != "" {
		return "", fmt.Errorf("checksum not found for %q in release %s", binaryName, release.TagName)
	}
	return "", fmt.Errorf("checksum not found for %q", binaryName)
}

func downloadText(client *http.Client, rawURL string) (string, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func parseChecksumText(content string, binaryName string) string {
	binaryName = strings.TrimSpace(binaryName)
	if binaryName == "" {
		return ""
	}

	trimmed := strings.TrimSpace(content)
	if sha256Pattern.MatchString(trimmed) && !strings.Contains(trimmed, "\n") {
		return strings.ToLower(sha256Pattern.FindString(trimmed))
	}

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(strings.ToLower(line), strings.ToLower(binaryName)) {
			continue
		}
		if sum := sha256Pattern.FindString(line); sum != "" {
			return strings.ToLower(sum)
		}
	}
	return ""
}

func verifyFileChecksum(path string, expectedHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	expected := strings.ToLower(strings.TrimSpace(expectedHex))
	if actual != expected {
		return fmt.Errorf("sha256 mismatch: expected=%s actual=%s", expected, actual)
	}
	return nil
}

func downloadCloudflared(reader *bufio.Reader) (string, error) {
	url, err := cloudflaredDownloadURL()
	if err != nil {
		return "", err
	}
	dest := cloudflaredInstallPath()
	if dest == "" {
		return "", fmt.Errorf("cloudflared install path could not be resolved")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}

	binaryName, err := cloudflaredDownloadAssetName()
	if err != nil {
		return "", err
	}
	fmt.Print(msg("cloudflared.checksum_fetching"))
	expectedChecksum, checksumErr := fetchCloudflaredChecksum(binaryName)
	if checksumErr != nil {
		fmt.Printf(msg("cloudflared.checksum_fetch_failed"), checksumErr)
		cont, promptErr := askYesNo(reader, msg("cloudflared.checksum_continue_prompt"), false)
		if promptErr != nil {
			return "", promptErr
		}
		if !cont {
			return "", fmt.Errorf(msg("cloudflared.checksum_required"))
		}
	} else {
		fmt.Printf(msg("cloudflared.checksum_fetch_ok"), expectedChecksum[:16])
	}

	fmt.Printf(msg("cloudflared.downloading"), url)
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	tmp := dest + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(tmp, 0o755)
	}
	if checksumErr == nil && expectedChecksum != "" {
		fmt.Print(msg("cloudflared.sha256_verifying"))
		if err := verifyFileChecksum(tmp, expectedChecksum); err != nil {
			_ = os.Remove(tmp)
			return "", fmt.Errorf("download rejected: %w", err)
		}
		fmt.Println(msg("cloudflared.ok"))
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}

	if out, err := exec.Command(dest, "version").CombinedOutput(); err != nil {
		return "", fmt.Errorf("downloaded cloudflared did not run: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	fmt.Printf(msg("cloudflared.ready"), dest)
	return dest, nil
}

func startCloudflaredProcess(ctx context.Context, bin string, args []string, writer io.Writer) (*cloudflaredProcess, error) {
	return startCloudflaredProcessWithCleanup(ctx, bin, args, writer, nil)
}

func startCloudflaredProcessWithCleanup(ctx context.Context, bin string, args []string, writer io.Writer, cleanup func()) (*cloudflaredProcess, error) {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = writer
	cmd.Stderr = writer
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	proc := &cloudflaredProcess{
		cmd:     cmd,
		done:    make(chan error, 1),
		cleanup: cleanup,
	}
	go func() {
		err := cmd.Wait()
		if proc.cleanup != nil {
			proc.cleanup()
		}
		proc.done <- err
	}()
	go func() {
		<-ctx.Done()
		_ = proc.Stop(5 * time.Second)
	}()
	return proc, nil
}

func (p *cloudflaredProcess) Stop(timeout time.Duration) error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
		return nil
	}
	select {
	case <-p.done:
		return nil
	default:
	}

	_ = p.cmd.Process.Signal(os.Interrupt)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-p.done:
		return err
	case <-timer.C:
		_ = p.cmd.Process.Kill()
		<-p.done
		return nil
	}
}

func runCloudflaredInteractive(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCloudflaredCapture(bin string, args ...string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func waitForProcessError(done <-chan error) error {
	err := <-done
	if err == nil {
		return fmt.Errorf("cloudflared exited")
	}
	return err
}

type linePrefixWriter struct {
	mu     sync.Mutex
	out    io.Writer
	prefix string
}

func (w *linePrefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.out == nil {
		return len(p), nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		fmt.Fprintf(w.out, "%s%s\n", w.prefix, scanner.Text())
	}
	return len(p), nil
}
