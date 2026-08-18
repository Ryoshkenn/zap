package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/selfupdate"
)

// newUninstallCmd builds `zap uninstall`: remove zap from this computer.
//
// If zap was installed by a package manager (Homebrew, Scoop) we never delete
// the binary ourselves — that would corrupt the manager's bookkeeping — and
// instead print the correct uninstall command. For go-install / manual
// installs we remove the binary directly. --purge also deletes config + state.
func newUninstallCmd() *cobra.Command {
	var purge, yes bool
	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove zap from this computer",
		Long: `Remove zap from this computer.

By default this removes the zap binary (for go-install / manual installs) and
leaves your config and state in place. Use --purge to also delete them.

If zap was installed via Homebrew or Scoop, this prints the correct package
manager command instead of deleting the binary.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(purge, yes, cmd.InOrStdin())
		},
	}
	c.Flags().BoolVar(&purge, "purge", false, "also remove zap's config and state files")
	c.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return c
}

func runUninstall(purge, yes bool, in interface{ Read([]byte) (int, error) }) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate zap binary: %w", err)
	}
	if resolved, rerr := filepath.EvalSymlinks(exePath); rerr == nil {
		exePath = resolved
	}

	src := selfupdate.Detect(exePath)
	managed := !src.SelfManaged()

	// Gather the user-data dir (config and state share one dir).
	dataDir, _ := config.ConfigDir()

	// Show the plan.
	fmt.Println("zap uninstall plan:")
	if managed {
		fmt.Printf("  • binary: managed by %s — will NOT be deleted by zap\n", src)
		fmt.Printf("           run: %s\n", uninstallHint(src))
	} else {
		fmt.Printf("  • binary: %s  (will be removed)\n", exePath)
	}
	if purge {
		if dataDir != "" {
			fmt.Printf("  • config + state: %s  (will be removed)\n", dataDir)
		}
	} else {
		fmt.Println("  • config + state: kept (use --purge to remove)")
	}

	if !yes {
		fmt.Print("\nProceed? [y/N] ")
		reader := bufio.NewReader(in)
		line, _ := reader.ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			fmt.Println("aborted.")
			return nil
		}
	}

	// Remove user data first (cheap, reversible-ish; the binary removal may
	// replace the running process on Windows).
	if purge && dataDir != "" {
		if err := os.RemoveAll(dataDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove %s: %v\n", dataDir, err)
		} else {
			fmt.Printf("removed %s\n", dataDir)
		}
	}

	if managed {
		fmt.Printf("\nNow run: %s\n", uninstallHint(src))
		return nil
	}

	if err := removeSelf(exePath); err != nil {
		return fmt.Errorf("remove %s: %w", exePath, err)
	}
	fmt.Printf("removed %s\nzap is uninstalled. Thanks for using it!\n", exePath)
	return nil
}

// detectManager guesses how zap was installed from its binary path. It returns
// a manager name and the command the user should run, or ("", "") when zap
// appears to be a standalone (go-install or manually placed) binary.

// uninstallHint is the package-manager command that removes zap. It is the
// uninstall counterpart to Source.UpgradeHint.
func uninstallHint(src selfupdate.Source) string {
	switch src {
	case selfupdate.SourceHomebrew:
		return "brew uninstall zap"
	case selfupdate.SourceScoop:
		return "scoop uninstall zap"
	}
	return ""
}
