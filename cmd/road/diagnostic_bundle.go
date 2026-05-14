package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/version"
)

const diagnosticMaxFileBytes = 8 * 1024 * 1024

type diagnosticBundleOptions struct {
	OutDir     string `json:"out_dir"`
	ServerPath string `json:"server_path"`
	ClientPath string `json:"client_path"`
	PluginDir  string `json:"plugin_dir"`
	LogDir     string `json:"log_dir"`
	SkipNet    bool   `json:"skip_net"`
}

type diagnosticMetadata struct {
	CreatedAt  string                     `json:"created_at"`
	Version    string                     `json:"version"`
	GOOS       string                     `json:"goos"`
	GOARCH     string                     `json:"goarch"`
	WorkingDir string                     `json:"working_dir"`
	Options    diagnosticBundleOptions    `json:"options"`
	Notes      []diagnosticCollectionNote `json:"notes,omitempty"`
}

type diagnosticCollectionNote struct {
	Kind    string `json:"kind"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type diagnosticCollector struct {
	zipw  *zip.Writer
	added map[string]struct{}
	notes []diagnosticCollectionNote
}

func runDiagnosticBundleCommand(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("diagnostic-bundle", flag.ContinueOnError)
	fs.SetOutput(out)

	opts := diagnosticBundleOptions{}
	fs.StringVar(&opts.OutDir, "out", "diagnostics", msg("diagnostic.flag_out"))
	fs.StringVar(&opts.ServerPath, "server", "configs/server.json", msg("diagnostic.flag_server"))
	fs.StringVar(&opts.ClientPath, "client", "configs/client.json", msg("diagnostic.flag_client"))
	fs.StringVar(&opts.PluginDir, "plugins", "plugins", msg("diagnostic.flag_plugins"))
	fs.StringVar(&opts.LogDir, "logs", "logs", msg("diagnostic.flag_logs"))
	fs.BoolVar(&opts.SkipNet, "skip-net", false, msg("diagnostic.flag_skip_net"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf(msg("diagnostic.unexpected_args"), strings.Join(fs.Args(), " "))
	}

	zipPath, err := createDiagnosticBundle(opts)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, msg("diagnostic.created"), zipPath)
	return nil
}

func createDiagnosticBundle(opts diagnosticBundleOptions) (string, error) {
	outDir := app.ResolveExistingPath(opts.OutDir)
	if outDir == "" {
		outDir = "diagnostics"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create diagnostic output dir %q: %w", outDir, err)
	}

	zipPath := filepath.Join(outDir, "road-diagnostic-"+time.Now().Format("20060102-150405")+".zip")
	file, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create diagnostic bundle %q: %w", zipPath, err)
	}
	defer file.Close()

	zipw := zip.NewWriter(file)
	collector := &diagnosticCollector{
		zipw:  zipw,
		added: map[string]struct{}{},
	}

	if err := collector.addText("version.txt", version.String("road-proxy")+"\n"); err != nil {
		_ = zipw.Close()
		return "", err
	}

	collector.collectConfigFiles(opts)
	collector.collectPluginFiles(opts.PluginDir)
	collector.collectLogFiles(opts.LogDir)
	collector.collectNetworkSnapshot(opts.SkipNet)

	wd, _ := os.Getwd()
	metadata := diagnosticMetadata{
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		Version:    version.String("road-proxy"),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		WorkingDir: wd,
		Options:    opts,
		Notes:      collector.notes,
	}
	if err := collector.addJSON("metadata.json", metadata); err != nil {
		_ = zipw.Close()
		return "", err
	}

	if err := zipw.Close(); err != nil {
		return "", fmt.Errorf("finalize diagnostic bundle %q: %w", zipPath, err)
	}
	return zipPath, nil
}

func (c *diagnosticCollector) collectConfigFiles(opts diagnosticBundleOptions) {
	for _, item := range []struct {
		path string
		name string
	}{
		{path: opts.ServerPath, name: "server"},
		{path: opts.ClientPath, name: "client"},
	} {
		resolved := app.ResolveExistingPath(item.path)
		if !regularFileExists(resolved) {
			c.note("missing", item.path, "config file not found")
			continue
		}
		c.addFileLenient(resolved, filepath.Join("configs", filepath.Base(resolved)))
	}

	configDir := app.ResolveExistingPath("configs")
	if !directoryExists(configDir) {
		c.note("missing", "configs", "config directory not found")
		return
	}
	c.addDirectoryFiles(configDir, "configs", func(path string, info os.FileInfo) bool {
		return !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".json")
	})
}

func (c *diagnosticCollector) collectPluginFiles(pluginDir string) {
	resolved := app.ResolveExistingPath(pluginDir)
	if !directoryExists(resolved) {
		c.note("missing", pluginDir, "plugin directory not found")
		return
	}

	allowed := map[string]struct{}{
		".json": {},
		".md":   {},
		".txt":  {},
	}
	c.addDirectoryFiles(resolved, "plugins", func(path string, info os.FileInfo) bool {
		if info.IsDir() {
			return false
		}
		_, ok := allowed[strings.ToLower(filepath.Ext(path))]
		return ok
	})
}

func (c *diagnosticCollector) collectLogFiles(logDir string) {
	resolved := app.ResolveExistingPath(logDir)
	if !directoryExists(resolved) {
		c.note("missing", logDir, "log directory not found")
		return
	}

	allowed := map[string]struct{}{
		".log":   {},
		".txt":   {},
		".json":  {},
		".jsonl": {},
	}
	c.addDirectoryFiles(resolved, "logs", func(path string, info os.FileInfo) bool {
		if info.IsDir() {
			return false
		}
		_, ok := allowed[strings.ToLower(filepath.Ext(path))]
		return ok
	})
}

func (c *diagnosticCollector) collectNetworkSnapshot(skip bool) {
	if skip {
		c.note("skipped", "network", "network snapshot skipped by --skip-net")
		return
	}

	name, args := networkSnapshotCommand()
	if name == "" {
		c.note("unsupported", "network", "no network snapshot command for this platform")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		c.note("failed", name, "network snapshot command timed out")
		return
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		c.note("failed", name, message)
		return
	}

	dest := filepath.Join("network", name+".txt")
	c.addTextLenient(dest, string(output))
}

func networkSnapshotCommand() (string, []string) {
	switch runtime.GOOS {
	case "windows":
		return "netstat", []string{"-ano"}
	case "linux":
		return "ss", []string{"-tunap"}
	case "darwin":
		return "netstat", []string{"-anv"}
	default:
		return "", nil
	}
}

func (c *diagnosticCollector) addDirectoryFiles(root, destRoot string, include func(string, os.FileInfo) bool) {
	type candidate struct {
		src  string
		dest string
	}
	var files []candidate

	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			c.note("failed", path, err.Error())
			return nil
		}
		if info == nil {
			return nil
		}
		if info.IsDir() && (info.Name() == ".git" || info.Name() == "build" || info.Name() == "diagnostics") {
			if path != root {
				return filepath.SkipDir
			}
		}
		if !include(path, info) {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			c.note("failed", path, relErr.Error())
			return nil
		}
		files = append(files, candidate{
			src:  path,
			dest: filepath.Join(destRoot, rel),
		})
		return nil
	})
	if walkErr != nil {
		c.note("failed", root, walkErr.Error())
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].dest < files[j].dest
	})
	for _, file := range files {
		c.addFileLenient(file.src, file.dest)
	}
}

func (c *diagnosticCollector) addFileLenient(src, dest string) {
	if err := c.addFile(src, dest); err != nil {
		c.note("failed", src, err.Error())
	}
}

func (c *diagnosticCollector) addTextLenient(dest, body string) {
	if err := c.addText(dest, body); err != nil {
		c.note("failed", dest, err.Error())
	}
}

func (c *diagnosticCollector) addJSON(dest string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal diagnostic json %q: %w", dest, err)
	}
	return c.addText(dest, string(append(data, '\n')))
}

func (c *diagnosticCollector) addText(dest, body string) error {
	name, err := safeZipName(dest)
	if err != nil {
		return err
	}
	if _, ok := c.added[name]; ok {
		return nil
	}
	c.added[name] = struct{}{}

	writer, err := c.zipw.Create(name)
	if err != nil {
		return fmt.Errorf("create zip entry %q: %w", name, err)
	}
	if _, err := io.WriteString(writer, body); err != nil {
		return fmt.Errorf("write zip entry %q: %w", name, err)
	}
	return nil
}

func (c *diagnosticCollector) addFile(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("source is a directory")
	}
	if info.Size() > diagnosticMaxFileBytes {
		c.note("skipped", src, fmt.Sprintf("file exceeds %d bytes", diagnosticMaxFileBytes))
		return nil
	}

	name, err := safeZipName(dest)
	if err != nil {
		return err
	}
	if _, ok := c.added[name]; ok {
		return nil
	}
	c.added[name] = struct{}{}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	writer, err := c.zipw.Create(name)
	if err != nil {
		return fmt.Errorf("create zip entry %q: %w", name, err)
	}
	if _, err := io.Copy(writer, in); err != nil {
		return fmt.Errorf("copy %q into zip entry %q: %w", src, name, err)
	}
	return nil
}

func (c *diagnosticCollector) note(kind, path, message string) {
	c.notes = append(c.notes, diagnosticCollectionNote{
		Kind:    kind,
		Path:    path,
		Message: message,
	})
}

func safeZipName(raw string) (string, error) {
	name := filepath.ToSlash(filepath.Clean(strings.TrimSpace(raw)))
	name = strings.TrimPrefix(name, "./")
	if name == "" || name == "." {
		return "", fmt.Errorf("empty zip entry name")
	}
	if filepath.IsAbs(raw) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") || name == ".." {
		return "", fmt.Errorf("unsafe zip entry name %q", raw)
	}
	return name, nil
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
