//go:build windows

package launch

import (
	"fmt"
	"os"
	"os/exec"
)

// Exec runs command + args in dir, forwarding stdio. zap remains as parent.
// On Windows there's no true exec; we wait for completion and forward the exit code.
func Exec(dir, command string, args []string, env []string) error {
	binPath, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("%s: not found on PATH", command)
	}
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if env != nil {
		cmd.Env = env
	}
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}
