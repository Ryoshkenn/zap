package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/telemetry"
	"github.com/Ryoshkenn/zap/internal/ui"
)

var Version = "dev"

// NewRootCmd builds the cobra root with all subcommands wired in.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "zap",
		Short: "Terminal launcher for AI coding CLIs",
		Long: `zap picks a folder and a provider (Claude, Codex, Gemini, opencode...) and
hands off control to that CLI in the chosen folder.

Run "zap" with no args for the interactive picker, or use subcommands for fast launches:
  zap claude                  # launch Claude in current folder
  zap claude /path/to/repo    # launch Claude in that folder
  zap claude --yolo           # launch Claude with --dangerously-skip-permissions
  zap favorite                # star the current folder
  zap favorite claude         # star a provider
  zap list                    # show providers and install status`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ui.Run()
		},
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zap: failed to load config: %v\n", err)
		os.Exit(1)
	}

	root.AddCommand(newFavoriteCmd())
	root.AddCommand(newUnfavoriteCmd())
	root.AddCommand(newListCmd(cfg))
	root.AddCommand(newConfigCmd())
	root.AddCommand(newSSHCmd(cfg))
	root.AddCommand(newLastCmd(cfg))
	root.AddCommand(newDoctorCmd(cfg))
	root.AddCommand(newUninstallCmd())
	root.AddCommand(newUpdateCmd())

	// Dynamic per-provider subcommands so `zap claude /path --yolo` works.
	for _, p := range cfg.Providers {
		root.AddCommand(newProviderCmd(p))
	}

	return root
}

// Execute runs the root command and exits with an appropriate status.
func Execute(version string) {
	Version = version
	telemetry.Init(version)
	telemetry.Track("zap_opened", nil)

	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		telemetry.Shutdown()
		fmt.Fprintf(os.Stderr, "zap: %v\n", err)
		os.Exit(1)
	}
	// Flush for commands that return normally (non-exec paths).
	telemetry.Shutdown()
}
