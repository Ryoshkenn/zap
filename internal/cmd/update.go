package cmd

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/selfupdate"
)

// newUpdateCmd builds `zap update`: check for a newer release and, for
// self-managed installs, replace the running binary in place.
//
// The split mirrors `zap uninstall`: when a package manager owns the binary we
// print its upgrade command instead of touching the file, so Homebrew and Scoop
// bookkeeping stays correct.
func newUpdateCmd() *cobra.Command {
	var checkOnly, yes bool
	c := &cobra.Command{
		Use:   "update",
		Short: "Update zap to the latest release",
		Long: `Check GitHub for a newer zap release and install it.

If zap was installed with Homebrew or Scoop, this prints the right upgrade
command rather than overwriting a package-managed binary. For go-install and
manual installs, zap downloads the release archive for your platform, verifies
its SHA-256 against the release checksums, and swaps the binary in place.`,
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

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	res, err := selfupdate.Check(ctx, Version)
	if err != nil {
		return err
	}

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
		fmt.Fprintf(out, "zap was installed with %s, so it manages the binary.\nRun:\n\n  %s\n", src, src.UpgradeHint())
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
