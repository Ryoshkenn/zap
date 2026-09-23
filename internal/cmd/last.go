package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/launch"
	"github.com/Ryoshkenn/zap/internal/state"
	"github.com/Ryoshkenn/zap/internal/telemetry"
)

// newLastCmd builds `zap last` (alias `resume`): replay the previous launch.
func newLastCmd(cfg *config.Config) *cobra.Command {
	var printOnly bool
	c := &cobra.Command{
		Use:     "last",
		Aliases: []string{"resume"},
		Short:   "Re-launch the most recent folder + provider",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, _ := state.Load()
			if s == nil || s.LastLaunch == nil {
				return fmt.Errorf("no previous launch recorded yet — run zap once first")
			}
			l := s.LastLaunch

			p := cfg.FindProvider(l.ProviderID)
			if p == nil {
				return fmt.Errorf("provider %q from last launch is no longer configured", l.ProviderID)
			}

			where := abbrevHome(l.Folder)
			if l.SSHTarget != "" {
				where = l.SSHTarget + ":" + l.Folder
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "↻ %s in %s\n", p.Name, where)

			// Model-selector launches store ["run", <model>] rather than flags.
			flags := l.Flags
			if !p.ModelSelector {
				flags = p.NormalizeFlags(flags)
			}

			// Remote replay.
			if l.SSHTarget != "" {
				if printOnly {
					printSSH(launch.SSHArgs(l.SSHTarget, l.Folder, p.Command, flags, l.SSHShell))
					return nil
				}
				telemetry.Track("zap_launch", map[string]any{
					"provider":    l.ProviderID,
					"is_remote":   true,
					"is_yolo":     false,
					"has_model":   l.Model != "",
					"launch_mode": "terminal",
					"trigger":     "last",
				})
				telemetry.Shutdown()
				return launchSSH(l.SSHTarget, l.Folder, p.Command, flags, l.SSHShell)
			}

			// Local replay.
			st := resolveProviderStatus(*p)
			if !st.Installed {
				return fmt.Errorf("%s is no longer installed", p.Name)
			}
			if printOnly {
				return printLocal(l.Folder, st.Provider.Command, flags)
			}
			recordLaunch(s, l.Folder, p.ID, flags, l.Model, st)
			telemetry.Track("zap_launch", map[string]any{
				"provider":    l.ProviderID,
				"is_remote":   false,
				"is_yolo":     false,
				"has_model":   l.Model != "",
				"launch_mode": resolveLaunchMode(st, s),
				"trigger":     "last",
			})
			telemetry.Shutdown()
			return launchProvider(l.Folder, st, flags, s)
		},
	}
	c.Flags().BoolVar(&printOnly, "print", false, "print the command instead of running it")
	return c
}
