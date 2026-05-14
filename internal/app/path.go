package app

import (
	"os"
	"path/filepath"
	"strings"
)

func IsDefaultRelativePath(raw, defaultPath string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return true
	}
	return filepath.Clean(trimmed) == filepath.Clean(defaultPath)
}

// ResolveExistingPath tries to resolve a relative path by checking:
// 1) current working directory
// 2) executable directory
// 3) parent directories of executable directory (up to 4 levels)
//
// If no candidate exists, it returns the original path.
func ResolveExistingPath(raw string) string {
	path := strings.TrimSpace(raw)
	if path == "" || filepath.IsAbs(path) {
		return path
	}

	candidates := make([]string, 0, 8)
	candidates = append(candidates, path)

	exePath, err := os.Executable()
	if err == nil {
		base := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(base, path))

		parent := base
		for i := 0; i < 4; i++ {
			next := filepath.Dir(parent)
			if next == parent {
				break
			}
			parent = next
			candidates = append(candidates, filepath.Join(parent, path))
		}
	}

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
	}

	return path
}
