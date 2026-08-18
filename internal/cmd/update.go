package cmd

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/selfupdate"
	"github.com/Ryoshkenn/zap/internal/state"
)

// newUpdateCmd builds `zap update`: check for a newer release and install it
// the way this install expects to be upgraded.
//
// Homebrew and Scoop installs are upgraded by running the package manager, not
// by overwriting the binary — the manager owns its receipts and version
// bookkeeping, and a hand-swapped file would leave it out of sync. go-install
// and manual installs get an in-place, checksum-verified binary swap.
func newUpdateCmd() *cobra.Command {
	var checkOnly, yes bool
	c := &cobra.Command{
		Use:   "update",
		Short: "Update zap to the latest release",
		Long: `Check GitHub for a newer zap release and install it.

zap upgrades itself the same way it was installed:

  Homebrew        runs "brew upgrade zap"
  Scoop           runs "scoop update zap"
  go install      downloads the release archive and swaps the binary
  manual install  downloads the release archive and swaps the binary

Downloaded archives are verified against the release's SHA-256 checksums
before anything is installed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, checkOnly, yes)
		},
	}
	c.Flags().BoolVar(&checkOnly, "check", false, "only report whether an update is available")
	c.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return c
}

func runUpdate(cmd *cobra.Command, checkOnly, yes bool) error {
	out := cmd.OutOrStdout()

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
	defer cancel()

	res, err := selfupdate.Check(ctx, Version)
	if err != nil {
		return err
	}

	// An explicit `zap update` is also a fresh check, so the background
	// checker does not go re-ask the API minutes later.
	cacheUpdateCheck(res)

	if !res.Available {
		if !res.Current.Valid() {
			fmt.Fprintf(out, "zap %s is a local build; latest release is %s\n", Version, res.Release.TagName)
			return nil
		}
		fmt.Fprintf(out, "zap %s is already the latest release.\n", Version)
		return nil
	}

	fmt.Fprintf(out, "Update available: %s → %s\n", Version, res.Release.TagName)
	fmt.Fprintf(out, "Release notes: %s\n\n", res.Release.HTMLURL)
	if checkOnly {
		return nil
	}

	exePath, err := selfupdate.ExePath()
	if err != nil {
		return fmt.Errorf("locate zap binary: %w", err)
	}
	src := selfupdate.Detect(exePath)

	if !src.SelfManaged() {
		fmt.Fprintf(out, "Installed with %s — running: %s\n\n", src, src.UpgradeHint())
		if !yes {
			ok, err := confirm(cmd, "Proceed?")
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, "Cancelled.")
				return nil
			}
		}
		if err := selfupdate.UpgradeViaManager(ctx, src, out, cmd.ErrOrStderr()); err != nil {
			return err
		}
		fmt.Fprintf(out, "\n%s finished. Run `zap --version` to confirm.\n", src)
		return nil
	}

	fmt.Fprintf(out, "This will replace %s\n", exePath)
	if !yes {
		ok, err := confirm(cmd, "Install the update?")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, "Cancelled.")
			return nil
		}
	}

	fmt.Fprintln(out, "Downloading and verifying…")
	if _, err := selfupdate.Apply(ctx, res.Release, exePath); err != nil {
		return err
	}
	fmt.Fprintf(out, "Updated to %s.\n", res.Release.TagName)
	return nil
}

// cacheUpdateCheck stores a successful check so the TUI's background checker
// can stay quiet for the next 24 hours. Failures to persist are ignored: a
// missed cache write costs one extra API call, which is not worth an error.
func cacheUpdateCheck(res *selfupdate.Result) {
	st, err := state.Load()
	if err != nil || st == nil {
		return
	}
	st.RecordUpdateCheck(time.Now(), res.Release.TagName, res.Release.HTMLURL)
	_ = st.Save()
}

// confirm asks a y/N question on the command's input stream.
func confirm(cmd *cobra.Command, question string) (bool, error) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N] ", question)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	a := strings.ToLower(strings.TrimSpace(line))
	return a == "y" || a == "yes", nil
}
