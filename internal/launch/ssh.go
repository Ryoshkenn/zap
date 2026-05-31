package launch

import "strings"

// DefaultRemoteShell is the login shell used on the remote host to ensure the
// provider command resolves on PATH. A login shell (-l) sources the user's
// profile, which is where nvm/npm-global tools like `claude` live.
const DefaultRemoteShell = "bash"

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
// into dir (if set) then exec's command + args inside a login shell so the
// remote PATH is fully populated. shell defaults to DefaultRemoteShell.
//
// Example: BuildRemoteCommand("~/proj", "claude", []string{"--yolo"}, "")
//
//	=> bash -lc 'cd ~/'\''proj'\'' && exec '\''claude'\'' '\''--yolo'\'''
func BuildRemoteCommand(dir, command string, args []string, shell string) string {
	if shell == "" {
		shell = DefaultRemoteShell
	}
	var inner strings.Builder
	if dir != "" {
		inner.WriteString("cd ")
		inner.WriteString(quoteRemoteDir(dir))
		inner.WriteString(" && ")
	}
	inner.WriteString("exec ")
	inner.WriteString(shellQuote(command))
	for _, a := range args {
		inner.WriteByte(' ')
		inner.WriteString(shellQuote(a))
	}
	return shell + " -lc " + shellQuote(inner.String())
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
