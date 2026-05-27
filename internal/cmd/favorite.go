package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/state"
)

func newFavoriteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "favorite [path|provider]",
		Short: "Star the current folder, a path, or a provider",
		Long: `With no args, stars the current directory.
With an existing path, stars that folder.
With a known provider ID (claude, codex, gemini, opencode, ...), stars that provider.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return toggleFavorite(args, true)
		},
	}
}

func newUnfavoriteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unfavorite [path|provider]",
		Short: "Remove a folder or provider from favorites",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return toggleFavorite(args, false)
		},
	}
}

func toggleFavorite(args []string, add bool) error {
	s, err := state.Load()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		applyFolder(s, dir, add)
		return s.Save()
	}

	target := args[0]

	// Provider ID?
	cfg, err := config.Load()
	if err == nil && cfg.FindProvider(target) != nil {
		applyProvider(s, target, add)
		return s.Save()
	}

	// Otherwise treat as folder path.
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return fmt.Errorf("%q is neither a known provider ID nor an existing directory", target)
	}
	applyFolder(s, abs, add)
	return s.Save()
}

func applyFolder(s *state.State, path string, add bool) {
	if add {
		if s.AddFavoriteFolder(path) {
			fmt.Printf("⭐ starred folder: %s\n", path)
		} else {
			fmt.Printf("already starred: %s\n", path)
		}
	} else {
		if s.RemoveFavoriteFolder(path) {
			fmt.Printf("removed: %s\n", path)
		} else {
			fmt.Printf("not in favorites: %s\n", path)
		}
	}
}

func applyProvider(s *state.State, id string, add bool) {
	if add {
		if s.AddFavoriteProvider(id) {
			fmt.Printf("⭐ starred provider: %s\n", id)
		} else {
			fmt.Printf("already starred: %s\n", id)
		}
	} else {
		if s.RemoveFavoriteProvider(id) {
			fmt.Printf("removed: %s\n", id)
		} else {
			fmt.Printf("not in favorites: %s\n", id)
		}
	}
}
