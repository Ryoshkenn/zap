package launch

import (
	"fmt"
	"strings"

	"github.com/Ryoshkenn/zap/internal/config"
)

// remoteLoginShell is used when no shell is configured for a host: the remote
// user's own login shell, as reported by sshd. It is expanded on the remote side.
const remoteLoginShell = `"$SHELL"`

// ValidateSSHTarget rejects destinations ssh would misparse: an empty string,
// embedded whitespace, or a leading '-' (which ssh reads as an option).
func ValidateSSHTarget(target string) error {
	switch {
	case target == "":
		return fmt.Errorf("ssh target is empty")
	case strings.HasPrefix(target, "-"):
		return fmt.Errorf("ssh target %q must not start with '-'", target)
	case strings.ContainsAny(target, " \t\r\n"):
		return fmt.Errorf("ssh target %q must not contain whitespace", target)
	}
	return nil
}

// shellQuote single-quotes s for POSIX shells, escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// quoteRemoteDir quotes a remote directory while preserving a leading ~ so the
// remote shell still expands it to the user's home.
func quoteRemoteDir(dir string) string {
	switch {
	case dir == "~":
		return "~"
	case strings.HasPrefix(dir, "~/"):
		return "~/" + shellQuote(dir[2:])
	default:
		return shellQuote(dir)
	}
}

// BuildRemoteCommand produces the single command string handed to ssh. It cd's
// into dir (if set) then exec's command + args inside a login, interactive
// shell. Both matter: version managers (nvm, fnm, bun…) and most installers
// add themselves to PATH in ~/.bashrc or ~/.zshrc, which a non-interactive
// `bash -lc` never reads, so `claude` would be "command not found". ssh -t
// provides the tty an interactive shell expects.
//
// shell picks the remote shell; empty means the remote user's own login shell.
// command may include baked-in base args ("opencode run"); they are split out
// so each word is quoted separately.
//
// Example: BuildRemoteCommand("~/proj", "claude", []string{"--yolo"}, "")
//
//	=> exec "$SHELL" -lic 'cd ~/'\''proj'\'' && exec '\''claude'\'' '\''--yolo'\'''
func BuildRemoteCommand(dir, command string, args []string, shell string) string {
	if shell == "" {
		shell = remoteLoginShell
	}
	var inner strings.Builder
	if dir != "" {
		inner.WriteString("cd ")
		inner.WriteString(quoteRemoteDir(dir))
		inner.WriteString(" && ")
	}
	inner.WriteString("exec ")
	bin, baseArgs := config.SplitCommand(command)
	inner.WriteString(shellQuote(bin))
	for _, a := range baseArgs {
		inner.WriteByte(' ')
		inner.WriteString(shellQuote(a))
	}
	for _, a := range args {
		inner.WriteByte(' ')
		inner.WriteString(shellQuote(a))
	}
	return "exec " + shell + " -lic " + shellQuote(inner.String())
}

// SSHArgs builds the argv (excluding the ssh binary itself) for launching
// command in remoteDir on target. With an empty command it returns an
// interactive login shell on the remote (`ssh -t target`).
func SSHArgs(target, remoteDir, command string, args []string, shell string) []string {
	if command == "" {
		return []string{"-t", target}
	}
	return []string{"-t", target, BuildRemoteCommand(remoteDir, command, args, shell)}
}
