package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var tunnelURLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`),
	regexp.MustCompile(`https://[a-z0-9.-]+\.cloudflareaccess\.com`),
}

type tunnelURLCapture struct {
	mu    sync.Mutex
	out   io.Writer
	found chan string
	once  sync.Once
}

func newTunnelURLCapture(out io.Writer) *tunnelURLCapture {
	return &tunnelURLCapture{
		out:   out,
		found: make(chan string, 1),
	}
}

func (w *tunnelURLCapture) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.out != nil {
		_, _ = w.out.Write(p)
	}
	text := string(p)
	for _, pattern := range tunnelURLPatterns {
		if match := pattern.FindString(text); match != "" {
			w.once.Do(func() {
				w.found <- match
			})
			break
		}
	}
	return len(p), nil
}

func startTryCloudflareTunnel(ctx context.Context, bin string, originURL string) (*cloudflaredProcess, <-chan string, error) {
	tmpDir, err := os.MkdirTemp("", "road-cloudflared-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create isolated cloudflared config dir: %w", err)
	}
	isolatedConfig, err := writeTryCloudflareConfig(tmpDir, originURL)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, nil, err
	}

	capture := newTunnelURLCapture(nil)
	args := []string{
		"tunnel",
		"--config", isolatedConfig,
		"--no-autoupdate",
		"--url", originURL,
	}
	proc, err := startCloudflaredProcessWithCleanup(ctx, bin, args, capture, func() {
		_ = os.RemoveAll(tmpDir)
	})
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, nil, err
	}
	return proc, capture.found, nil
}

func writeTryCloudflareConfig(dir string, originURL string) (string, error) {
	path := filepath.Join(dir, "config.yml")
	data := fmt.Sprintf("# ROAD isolated TryCloudflare config\nurl: %q\n", originURL)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		return "", fmt.Errorf("write isolated cloudflared config: %w", err)
	}
	return path, nil
}

func waitForTunnelURL(ctx context.Context, urlCh <-chan string, proc *cloudflaredProcess, timeout time.Duration) (string, error) {
	fmt.Print(msg("cloudflared.tunnel_url_wait"))
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case url := <-urlCh:
			fmt.Println()
			fmt.Printf(msg("cloudflared.tunnel_url_ready"), url)
			return url, nil
		case err := <-proc.done:
			fmt.Println()
			if err == nil {
				return "", fmt.Errorf("cloudflared exited before URL was emitted")
			}
			return "", fmt.Errorf("cloudflared stopped before URL was emitted: %w", err)
		case <-ticker.C:
			fmt.Print(".")
		case <-timer.C:
			fmt.Println()
			return "", fmt.Errorf("cloudflared URL timeout (%s)", timeout)
		case <-ctx.Done():
			fmt.Println()
			return "", ctx.Err()
		}
	}
}
