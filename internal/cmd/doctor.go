package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/detect"
	"github.com/Ryoshkenn/zap/internal/sshconf"
	"github.com/Ryoshkenn/zap/internal/state"
)

// newDoctorCmd builds `zap doctor`: a health summary of zap's environment.
func newDoctorCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check zap's environment, config, and provider availability",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cfg)
		},
	}
}

func runDoctor(cfg *config.Config) error {
	fmt.Printf("zap %s  (%s/%s, %s)\n\n", Version, runtime.GOOS, runtime.GOARCH, runtime.Version())

	// Config.
	if cp, err := config.ConfigPath(); err == nil {
		mark := "✓ exists"
		if _, statErr := os.Stat(cp); os.IsNotExist(statErr) {
			mark = "– none (using built-in defaults)"
		}
		fmt.Printf("config:  %s  %s\n", cp, mark)
	}
	if _, err := config.Load(); err != nil {
		fmt.Printf("         %s parse error: %v\n", "✗", err)
	}

	// State.
	if sp, err := state.StatePath(); err == nil {
		mark := "✓ exists"
		if _, statErr := os.Stat(sp); os.IsNotExist(statErr) {
			mark = "– none yet"
		}
		fmt.Printf("state:   %s  %s\n", sp, mark)
	}

	// SSH.
	fmt.Println()
	if path, err := exec.LookPath("ssh"); err == nil {
		fmt.Printf("ssh:     ✓ %s\n", path)
	} else {
		fmt.Printf("ssh:     ✗ not found on PATH (remote launch unavailable)\n")
	}
	s, _ := state.Load()
	if s != nil && len(s.SSHHosts) > 0 {
		fmt.Printf("         %d saved host(s): ", len(s.SSHHosts))
		for i, h := range s.SSHHosts {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(h.Label())
		}
		fmt.Println()
	}
	if cfgHosts := sshconf.Hosts(); len(cfgHosts) > 0 {
		fmt.Printf("         %d host(s) in ~/.ssh/config\n", len(cfgHosts))
	}

	// Providers.
	fmt.Println("\nproviders:")
	statuses := detect.Detect(cfg)
	installed := 0
	for _, st := range statuses {
		mark := "✗"
		extra := st.Provider.InstallHint
		if st.Installed {
			mark = "✓"
			extra = st.Path
			installed++
		}
		icon := st.Provider.Icon
		if icon == "" {
			icon = "  "
		}
		fmt.Printf("  %s %s  %-12s %s\n", mark, icon, st.Provider.ID, extra)
	}
	fmt.Printf("\n%d of %d providers installed\n", installed, len(statuses))
	return nil
}
