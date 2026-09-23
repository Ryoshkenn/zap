//go:build windows

package launch

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Ryoshkenn/zap/internal/config"
)

// Open launches command as a detached GUI app, passing dir as the first argument.
// appBundlePath is ignored on Windows (macOS-only concept).
func Open(dir, command string, args []string, appBundlePath string) error {
	bin, baseArgs := config.SplitCommand(command)
	binPath, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("%s: not found on PATH", bin)
	}
	all := append(append([]string{dir}, baseArgs...), args...)
	cmd := exec.Command(binPath, all...)
	return cmd.Start()
}

// ExecSSH runs ssh as a child process, forwarding stdio and the exit code.
// Windows has no true exec, so zap stays as the parent while ssh owns the
// console. With an empty command it opens an interactive remote shell.
func ExecSSH(target, remoteDir, command string, args []string, shell string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh: not found on PATH")
	}
	cmd := exec.Command(sshPath, SSHArgs(target, remoteDir, command, args, shell)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}

// Exec runs command + args in dir, forwarding stdio. zap remains as parent.
// On Windows there's no true exec; we wait for completion and forward the exit code.
func Exec(dir, command string, args []string, env []string) error {
	bin, baseArgs := config.SplitCommand(command)
	binPath, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("%s: not found on PATH", bin)
	}
	fullArgs := append(append([]string(nil), baseArgs...), args...)
	cmd := exec.Command(binPath, fullArgs...)
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
