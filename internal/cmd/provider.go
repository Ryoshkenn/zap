package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/detect"
	"github.com/Ryoshkenn/zap/internal/launch"
	"github.com/Ryoshkenn/zap/internal/state"
)

var (
	launchExec = launch.Exec
	launchOpen = launch.Open
)

// newProviderCmd builds `zap <provider> [path] [--yolo|--safe]`.
func newProviderCmd(p config.Provider) *cobra.Command {
	var yolo, safe bool
	c := &cobra.Command{
		Use:   p.ID + " [path]",
		Short: fmt.Sprintf("Launch %s", p.Name),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := resolveProviderStatus(p)
			if !st.Installed {
				if p.InstallHint != "" {
					return fmt.Errorf("%s is not installed.\nInstall: %s", p.Name, p.InstallHint)
				}
				return fmt.Errorf("%s is not installed (command %q not on PATH)", p.Name, p.Command)
			}

			dir, err := resolveDir(args)
			if err != nil {
				return err
			}

			s, _ := state.Load()

			// Model-selector providers (e.g. Ollama) require a chosen model.
			// Mirror the interactive flow: launch with ["run", <model>] and
			// skip DefaultFlags (Ollama has none, and the interactive path
			// clears them too). Fail clearly if no default has been saved.
			if p.ModelSelector {
				var model string
				if s != nil {
					model, _ = s.PreferredModelFor(p.ID)
				}
				if model == "" {
					return fmt.Errorf(
						"%s requires a model to launch.\n"+
							"No default model is saved yet.\n"+
							"Run `zap` interactively, select %s, and choose a default model first.",
						p.Name, p.Name,
					)
				}
				if s != nil {
					s.TouchRecent(dir)
					_ = s.Save()
				}
				return launchProvider(dir, st, []string{"run", model}, s)
			}

			flags := resolveFlags(p, s, yolo, safe)

			if s != nil {
				s.TouchRecent(dir)
				_ = s.Save()
			}

			return launchProvider(dir, st, flags, s)
		},
	}
	if hasYoloFlag(p) {
		c.Flags().BoolVar(&yolo, "yolo", false, "enable the dangerous flag for this provider")
		c.Flags().BoolVar(&safe, "safe", false, "disable any default dangerous flags")
	}
	return c
}

func resolveProviderStatus(p config.Provider) detect.Status {
	return detect.ProviderStatus(p)
}

func launchProvider(dir string, st detect.Status, flags []string, s *state.State) error {
	mode := st.Provider.LaunchMode
	if mode == "" {
		mode = "terminal"
	}
	if s != nil {
		if saved, ok := s.LaunchModeFor(st.Provider.ID); ok {
			mode = saved
		}
	}
	if mode == "app" {
		return launchOpen(dir, st.Provider.Command, flags, st.AppBundlePath)
	}
	return launchExec(dir, st.Provider.Command, flags, os.Environ())
}

func resolveDir(args []string) (string, error) {
	if len(args) == 0 {
		return os.Getwd()
	}
	p := args[0]
	if !filepath.IsAbs(p) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		p = filepath.Join(cwd, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("path %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", abs)
	}
	return abs, nil
}

// resolveFlags layers config defaults, saved preferences, and CLI overrides.
// Priority (highest wins): --safe / --yolo > state.PreferredFlags > p.DefaultFlags.
func resolveFlags(p config.Provider, s *state.State, yolo, safe bool) []string {
	var out []string
	if s != nil {
		if saved, ok := s.PreferredFlagsFor(p.ID); ok {
			out = append(out, saved...)
		}
	}
	if out == nil {
		out = append([]string(nil), p.DefaultFlags...)
	}

	yoloFlag := findYoloFlag(p)
	if safe && yoloFlag != "" {
		out = removeAll(out, yoloFlag)
	} else if yolo && yoloFlag != "" && !contains(out, yoloFlag) {
		out = append(out, yoloFlag)
	}
	return out
}

func hasYoloFlag(p config.Provider) bool {
	return findYoloFlag(p) != ""
}

func findYoloFlag(p config.Provider) string {
	for _, f := range p.Flags {
		if f.ID == "yolo" {
			return f.Flag
		}
	}
	return ""
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func removeAll(ss []string, target string) []string {
	out := ss[:0]
	for _, s := range ss {
		if s != target {
			out = append(out, s)
		}
	}
	return out
}
