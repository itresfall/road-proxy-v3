package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "0.1.0-dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func String(appName string) string {
	return fmt.Sprintf("%s %s (commit=%s, build=%s, go=%s, os=%s/%s)",
		appName,
		Version,
		Commit,
		BuildDate,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
}
