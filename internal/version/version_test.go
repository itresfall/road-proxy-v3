package version

import (
	"strings"
	"testing"
)

func TestStringIncludesIdentityAndBuildMetadata(t *testing.T) {
	out := String("road-test")

	for _, want := range []string{
		"road-test",
		Version,
		"commit=" + Commit,
		"build=" + BuildDate,
		"go=",
		"os=",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("version string %q does not contain %q", out, want)
		}
	}
}
