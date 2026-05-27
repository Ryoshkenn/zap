//go:build !windows

package launch

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Exec changes directory to dir and replaces the current process with command + args.
// On Unix this uses syscall.Exec — zap disappears and the target CLI owns the TTY.
func Exec(dir, command string, args []string, env []string) error {
	binPath, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("%s: not found on PATH", command)
	}
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			return fmt.Errorf("chdir %s: %w", dir, err)
		}
	}
	argv := append([]string{binPath}, args...)
	if env == nil {
		env = os.Environ()
	}
	return syscall.Exec(binPath, argv, env)
}
