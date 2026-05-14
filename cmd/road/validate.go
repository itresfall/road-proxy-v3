package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/config"
	"road-proxy-v3/internal/plugin"
)

func tryRunSubcommand(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "validate":
		return true, runValidateCommand(args[1:], os.Stdout)
	case "validate-plugin":
		return true, runValidatePluginCommand(args[1:], os.Stdout)
	case "generate-config":
		return true, runGenerateConfigCommand(args[1:], os.Stdout)
	case "ping":
		return true, runPingCommand(args[1:], os.Stdout)
	case "udp-check":
		return true, runUDPCheckCommand(args[1:], os.Stdout)
	case "diagnostic-bundle":
		return true, runDiagnosticBundleCommand(args[1:], os.Stdout)
	case "public-server":
		return true, runPublicServerCommand(args[1:], os.Stdout)
	case "help":
		printCommandHelp(os.Stdout)
		return true, nil
	default:
		if strings.HasPrefix(args[0], "-") {
			return false, nil
		}
		return true, fmt.Errorf(msg("validate.unknown_command"), args[0])
	}
}

func runValidateCommand(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(out)

	serverPath := fs.String("server", "configs/server.json", msg("validate.flag_server"))
	clientPath := fs.String("client", "configs/client.json", msg("validate.flag_client"))
	pluginDir := fs.String("plugins", "", msg("validate.flag_plugins"))
	allConfigs := fs.Bool("all-configs", false, msg("validate.flag_all_configs"))
	skipPlugins := fs.Bool("skip-plugins", false, msg("validate.flag_skip_plugins"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf(msg("validate.unexpected_args"), strings.Join(fs.Args(), " "))
	}

	if *allConfigs {
		return validateAllConfigs(*pluginDir, *skipPlugins, out)
	}

	resolvedServerPath := app.ResolveExistingPath(*serverPath)
	resolvedClientPath := app.ResolveExistingPath(*clientPath)

	if err := validateServerConfig(resolvedServerPath, *pluginDir, *skipPlugins); err != nil {
		return err
	}
	fmt.Fprintf(out, msg("validate.ok_server"), resolvedServerPath)

	if err := validateClientConfig(resolvedClientPath, *pluginDir, *skipPlugins); err != nil {
		return err
	}
	fmt.Fprintf(out, msg("validate.ok_client"), resolvedClientPath)
	fmt.Fprintln(out, msg("validate.ok_all"))
	return nil
}

func runValidatePluginCommand(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("validate-plugin", flag.ContinueOnError)
	fs.SetOutput(out)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf(msg("validate_plugin.usage"))
	}

	path := app.ResolveExistingPath(fs.Arg(0))
	schema, err := plugin.LoadSchemaFile(path)
	if err != nil {
		return err
	}
	if err := validatePluginFolderName(path, schema.Name); err != nil {
		return err
	}
	fmt.Fprintf(out, msg("validate.ok_plugin"), schema.Name, path)
	return nil
}

func validateAllConfigs(pluginDir string, skipPlugins bool, out io.Writer) error {
	configDir := app.ResolveExistingPath("configs")

	serverPaths, err := filepath.Glob(filepath.Join(configDir, "server*.json"))
	if err != nil {
		return err
	}
	clientPaths, err := filepath.Glob(filepath.Join(configDir, "client*.json"))
	if err != nil {
		return err
	}
	if len(serverPaths) == 0 && len(clientPaths) == 0 {
		return fmt.Errorf(msg("validate.no_configs_found"), configDir)
	}

	for _, path := range serverPaths {
		if err := validateServerConfig(path, pluginDir, skipPlugins); err != nil {
			return err
		}
		fmt.Fprintf(out, msg("validate.ok_server"), path)
	}
	for _, path := range clientPaths {
		if err := validateClientConfig(path, pluginDir, skipPlugins); err != nil {
			return err
		}
		fmt.Fprintf(out, msg("validate.ok_client"), path)
	}
	fmt.Fprintln(out, msg("validate.ok_all"))
	return nil
}

func validateServerConfig(path, pluginDir string, skipPlugins bool) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if skipPlugins {
		return nil
	}

	dir := resolvePluginDir(path, pluginDir, cfg.Plugins.Dir)
	if _, err := plugin.NewLoader(dir).LoadEnabled(cfg.Plugins.Enabled); err != nil {
		return err
	}
	return nil
}

func validateClientConfig(path, pluginDir string, skipPlugins bool) error {
	cfg, err := config.LoadClient(path)
	if err != nil {
		return err
	}
	if skipPlugins || cfg.PluginName == "" {
		return nil
	}

	dir := resolvePluginDir(path, pluginDir, "plugins")
	if _, err := plugin.NewLoader(dir).LoadEnabled([]string{cfg.PluginName}); err != nil {
		return err
	}
	return nil
}

func resolvePluginDir(configPath, explicitDir, configDir string) string {
	if strings.TrimSpace(explicitDir) != "" {
		return app.ResolveExistingPath(explicitDir)
	}
	if filepath.IsAbs(configDir) {
		return configDir
	}
	if resolved := app.ResolveExistingPath(configDir); resolved != configDir {
		return resolved
	}
	if configPath != "" {
		candidate := filepath.Join(filepath.Dir(configPath), configDir)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return configDir
}

func validatePluginFolderName(path, schemaName string) error {
	if filepath.Base(path) != "plugin.json" {
		return nil
	}
	parent := filepath.Base(filepath.Dir(path))
	if parent == "" || parent == "." || parent == string(filepath.Separator) {
		return nil
	}
	grandParent := filepath.Base(filepath.Dir(filepath.Dir(path)))
	if grandParent != "plugins" {
		return nil
	}
	if parent != schemaName {
		return fmt.Errorf("plugin folder %q does not match schema name %q", parent, schemaName)
	}
	return nil
}

func printCommandHelp(out io.Writer) {
	fmt.Fprintln(out, "ROAD Proxy v3")
	fmt.Fprintln(out)
	fmt.Fprintln(out, msg("validate.help"))
}
