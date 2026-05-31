package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/config"
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

	mgr, hint := detectManager(exePath)

	// Gather the user-data dir (config and state share one dir).
	dataDir, _ := config.ConfigDir()

	// Show the plan.
	fmt.Println("zap uninstall plan:")
	if mgr != "" {
		fmt.Printf("  • binary: managed by %s — will NOT be deleted by zap\n", mgr)
		fmt.Printf("           run: %s\n", hint)
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

	if mgr != "" {
		fmt.Printf("\nNow run: %s\n", hint)
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
func detectManager(exePath string) (manager, hint string) {
	lower := strings.ToLower(exePath)
	switch {
	case strings.Contains(lower, "/cellar/") || strings.Contains(lower, "/homebrew/") || brewOwns(exePath):
		return "Homebrew", "brew uninstall zap"
	case strings.Contains(lower, "scoop"):
		return "Scoop", "scoop uninstall zap"
	}
	return "", ""
}

// brewOwns reports whether `brew --prefix zap` resolves, indicating Homebrew
// tracks this formula. Best-effort: any error means "not brew".
func brewOwns(exePath string) bool {
	brew, err := exec.LookPath("brew")
	if err != nil {
		return false
	}
	out, err := exec.Command(brew, "--prefix", "zap").Output()
	if err != nil {
		return false
	}
	prefix := strings.TrimSpace(string(out))
	return prefix != "" && strings.HasPrefix(exePath, prefix)
}
