package detect

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Ryoshkenn/zap/internal/config"
)

var appSearchDirs = []string{"/Applications"}

// Status describes whether a provider's command is available on PATH.
type Status struct {
	Provider      config.Provider
	Installed     bool
	Path          string // CLI binary path, or /Applications/<bundle>.app path
	AppBundlePath string // set when detected via /Applications on macOS
}

// Detect reports installation status for every provider in cfg.
func Detect(cfg *config.Config) []Status {
	out := make([]Status, 0, len(cfg.Providers))
	for _, p := range cfg.Providers {
		out = append(out, ProviderStatus(p))
	}
	return out
}

// ProviderStatus reports installation status for one provider.
func ProviderStatus(p config.Provider) Status {
	st := Status{Provider: p}
	if path, err := exec.LookPath(p.Command); err == nil {
		st.Installed = true
		st.Path = path
		return st
	}
	if p.AppBundle == "" {
		return st
	}
	for _, dir := range appSearchDirs {
		appPath := filepath.Join(dir, p.AppBundle+".app")
		if _, err := os.Stat(appPath); err == nil {
			st.Installed = true
			st.Path = appPath
			st.AppBundlePath = appPath
			return st
		}
	}
	return st
}

// IsInstalled reports whether a single command is on PATH.
func IsInstalled(command string) (string, bool) {
	path, err := exec.LookPath(command)
	if err != nil {
		return "", false
	}
	return path, true
}
