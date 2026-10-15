package version

import (
	"os"
	"path/filepath"
	"strings"
)

// Version holds the application version, either injected at build time via
// -ldflags="-X 'gnnwatchtower/internal/version.Version=...'" or read from the VERSION file.
var Version string

// Get returns the single source of truth version for GNNwatchtower server & agent.
func Get() string {
	if Version != "" {
		return Version
	}

	candidates := []string{
		"VERSION",
		"VERSIYON",
		"../VERSION",
		"../../VERSION",
		"../../../VERSION",
		"/opt/gnnwatchtower/VERSION",
		"/opt/gnnwatchtower-agent/VERSION",
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "VERSION"),
			filepath.Join(dir, "VERSIYON"),
			filepath.Join(dir, "..", "VERSION"),
			filepath.Join(dir, "bin", "VERSION"),
		)
	}

	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			v := strings.TrimSpace(string(data))
			if v != "" {
				Version = v
				return Version
			}
		}
	}

	return "0.4.6"
}
