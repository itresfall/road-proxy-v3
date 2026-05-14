package main

import (
	"flag"
	"fmt"
	"runtime"
	"strings"

	"road-proxy-v3/internal/app"
	"road-proxy-v3/internal/version"
)

func main() {
	initStudioLanguage()

	showVersion := flag.Bool("version", false, "print version and exit")
	processFlag := flag.String("process", "", "process name substring for non-interactive capture")
	pidFlag := flag.Int("pid", 0, "process PID for non-interactive capture")
	secondsFlag := flag.Int("seconds", 20, "capture duration seconds for non-interactive mode")
	multiPhaseFlag := flag.Bool("multi-phase", false, "capture lobby/connect/ingame/disconnect phases")
	phaseSecondsFlag := flag.Int("phase-seconds", 8, "capture duration seconds per multi-phase step")
	networkFlag := flag.String("network", "", "override network: tcp or udp")
	targetHostFlag := flag.String("target-host", "", "override target host")
	targetPortFlag := flag.Int("target-port", 0, "override target port")
	clientListenPortFlag := flag.Int("client-listen-port", 0, "override client listen port")
	pluginNameFlag := flag.String("plugin-name", "", "override generated plugin name")
	forceFlag := flag.Bool("force", false, "overwrite existing generated plugin")
	var udpPeerBroadcastFlag optionalBool
	flag.Var(&udpPeerBroadcastFlag, "udp-peer-broadcast", "override UDP peer broadcast")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.String("plugin-studio"))
		return
	}

	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		fmt.Printf(sm("studio.platform_unsupported"), runtime.GOOS)
		return
	}

	layout, err := app.EnsureRuntimeLayout()
	if err != nil {
		fmt.Printf(sm("studio.error_runtime_setup"), err)
		return
	}
	reloadStudioLanguage(layout)

	cliOptions := studioCLIOptions{
		Process:          strings.TrimSpace(*processFlag),
		PID:              *pidFlag,
		Seconds:          *secondsFlag,
		MultiPhase:       *multiPhaseFlag,
		PhaseSeconds:     *phaseSecondsFlag,
		Network:          strings.TrimSpace(*networkFlag),
		TargetHost:       strings.TrimSpace(*targetHostFlag),
		TargetPort:       *targetPortFlag,
		ClientListenPort: *clientListenPortFlag,
		PluginName:       strings.TrimSpace(*pluginNameFlag),
		UDPPeerBroadcast: udpPeerBroadcastFlag,
		Force:            *forceFlag,
	}
	if cliOptions.Enabled() {
		if err := runNonInteractiveStudio(layout, cliOptions); err != nil {
			fmt.Printf(sm("studio.error"), err)
		}
		return
	}

	runInteractiveStudio(layout)
}
