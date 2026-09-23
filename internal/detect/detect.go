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
//
// For providers with an AppBundle, an installed .app wins over whatever the
// command resolves to on PATH: shims like Cursor's ~/.local/bin/cursor (which
// only fronts `cursor agent`) sit on PATH without the IDE, and launching them
// with a folder fails. `open -a` on the bundle always opens the real app.
func ProviderStatus(p config.Provider) Status {
	st := Status{Provider: p}
	bin, _ := config.SplitCommand(p.Command)
	cliPath, cliErr := exec.LookPath(bin)
	if appPath := findAppBundle(p.AppBundle); appPath != "" {
		st.Installed = true
		st.Path = appPath
		st.AppBundlePath = appPath
		return st
	}
	if cliErr == nil {
		st.Installed = true
		st.Path = cliPath
	}
	return st
}

// findAppBundle returns the path of <bundle>.app in the app search dirs, or "".
func findAppBundle(bundle string) string {
	if bundle == "" {
		return ""
	}
	dirs := appSearchDirs
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(append([]string(nil), dirs...), filepath.Join(home, "Applications"))
	}
	for _, dir := range dirs {
		appPath := filepath.Join(dir, bundle+".app")
		if _, err := os.Stat(appPath); err == nil {
			return appPath
		}
	}
	return ""
}

// IsInstalled reports whether a single command is on PATH.
func IsInstalled(command string) (string, bool) {
	bin, _ := config.SplitCommand(command)
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", false
	}
	return path, true
}
