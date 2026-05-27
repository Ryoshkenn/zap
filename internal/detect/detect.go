package detect

import (
	"os/exec"

	"github.com/Ryoshkenn/zap/internal/config"
)

// Status describes whether a provider's command is available on PATH.
type Status struct {
	Provider  config.Provider
	Installed bool
	Path      string
}

// Detect reports installation status for every provider in cfg.
func Detect(cfg *config.Config) []Status {
	out := make([]Status, 0, len(cfg.Providers))
	for _, p := range cfg.Providers {
		st := Status{Provider: p}
		if path, err := exec.LookPath(p.Command); err == nil {
			st.Installed = true
			st.Path = path
		}
		out = append(out, st)
	}
	return out
}

// IsInstalled reports whether a single command is on PATH.
func IsInstalled(command string) (string, bool) {
	path, err := exec.LookPath(command)
	if err != nil {
		return "", false
	}
	return path, true
}
