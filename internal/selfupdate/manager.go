package selfupdate

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// UpgradeCommand returns the argv that upgrades zap for this install source.
// It returns nil for self-managed installs, which replace their own binary
// instead of shelling out.
func (s Source) UpgradeCommand() []string {
	switch s {
	case SourceHomebrew:
		return []string{"brew", "upgrade", "zap"}
	case SourceScoop:
		return []string{"scoop", "update", "zap"}
	}
	return nil
}

// UpgradeViaManager runs the package manager's upgrade command, streaming its
// output so the user sees brew's or scoop's own progress rather than a silent
// stall. zap never edits a managed install's files itself — the manager owns
// its receipts, symlinks, and version bookkeeping.
func UpgradeViaManager(ctx context.Context, src Source, stdout, stderr io.Writer) error {
	argv := src.UpgradeCommand()
	if len(argv) == 0 {
		return fmt.Errorf("no upgrade command for install source %q", src)
	}

	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("zap was installed with %s but %q is not on your PATH; "+
			"install it or upgrade zap manually", src, argv[0])
	}

	cmd := exec.CommandContext(ctx, bin, argv[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", joinArgv(argv), err)
	}
	return nil
}

func joinArgv(argv []string) string {
	out := ""
	for i, a := range argv {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
