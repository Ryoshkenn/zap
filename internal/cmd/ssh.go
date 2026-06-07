package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/launch"
	"github.com/Ryoshkenn/zap/internal/state"
	"github.com/Ryoshkenn/zap/internal/telemetry"
)

var launchSSH = launch.ExecSSH

// newSSHCmd builds `zap ssh <target> [provider] [remote-path]`.
//
// With only a target it opens an interactive remote shell. With a provider it
// launches that CLI on the remote host inside the given folder, exactly like a
// local `zap <provider>` but over ssh.
func newSSHCmd(cfg *config.Config) *cobra.Command {
	var yolo, safe, printOnly bool
	var shell string
	c := &cobra.Command{
		Use:   "ssh <target> [provider] [remote-path]",
		Short: "Launch a provider on a remote computer over SSH",
		Long: `Connect to a remote computer over SSH and launch a coding CLI there.

  zap ssh devbox                 # open a shell on devbox
  zap ssh devbox claude          # launch Claude in ~ on devbox
  zap ssh me@host claude ~/api   # launch Claude in ~/api on me@host
  zap ssh devbox codex --print   # show the ssh command instead of running it

The remote command runs inside a login shell so the provider resolves on the
remote PATH (override the shell with --shell if you don't use bash).`,
		Args: cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]

			s, _ := state.Load()
			if host := lookupHostShell(s, target); shell == "" && host != "" {
				shell = host
			}

			// Bare `zap ssh <target>` => interactive remote shell.
			if len(args) == 1 {
				if printOnly {
					fmt.Printf("ssh -t %s\n", shellArg(target))
					return nil
				}
				rememberHost(s, target, shell, "")
				telemetry.Track("zap_command", map[string]any{
					"command":      "ssh_shell",
					"has_provider": false,
				})
				telemetry.Shutdown()
				return launchSSH(target, "", "", nil, shell)
			}

			providerID := args[1]
			p := cfg.FindProvider(providerID)
			if p == nil {
				return fmt.Errorf("unknown provider %q (try `zap list`)", providerID)
			}

			remoteDir := "~"
			if len(args) == 3 {
				remoteDir = args[2]
			}

			flags := resolveFlags(*p, s, yolo, safe)

			if printOnly {
				sshArgs := launch.SSHArgs(target, remoteDir, p.Command, flags, shell)
				fmt.Print("ssh")
				for _, a := range sshArgs {
					fmt.Printf(" %s", shellArg(a))
				}
				fmt.Println()
				return nil
			}

			rememberRemoteLaunch(s, target, shell, remoteDir, p.ID, flags)
			telemetry.Track("zap_launch", map[string]any{
				"provider":    p.ID,
				"is_remote":   true,
				"is_yolo":     yolo,
				"has_model":   false,
				"launch_mode": "terminal",
				"trigger":     "direct",
			})
			telemetry.Shutdown()
			return launchSSH(target, remoteDir, p.Command, flags, shell)
		},
	}
	c.Flags().BoolVar(&yolo, "yolo", false, "enable the provider's dangerous flag")
	c.Flags().BoolVar(&safe, "safe", false, "disable any default dangerous flags")
	c.Flags().BoolVar(&printOnly, "print", false, "print the ssh command instead of running it")
	c.Flags().StringVar(&shell, "shell", "", "remote login shell used to resolve PATH (default bash)")
	return c
}

func lookupHostShell(s *state.State, target string) string {
	if s == nil {
		return ""
	}
	if h := s.FindSSHHost(target); h != nil {
		return h.Shell
	}
	return ""
}

func rememberHost(s *state.State, target, shell, dir string) {
	if s == nil {
		return
	}
	s.AddSSHHost(state.SSHHost{Target: target, Shell: shell})
	if dir != "" {
		s.TouchRemote(target, dir)
	}
	_ = s.Save()
}

func rememberRemoteLaunch(s *state.State, target, shell, dir, providerID string, flags []string) {
	if s == nil {
		return
	}
	s.AddSSHHost(state.SSHHost{Target: target, Shell: shell})
	s.TouchRemote(target, dir)
	s.SetLastLaunch(state.LastLaunch{
		ProviderID: providerID,
		Folder:     dir,
		Flags:      flags,
		SSHTarget:  target,
		SSHShell:   shell,
	})
	_ = s.Save()
}
