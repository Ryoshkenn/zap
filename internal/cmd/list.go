package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/detect"
	"github.com/Ryoshkenn/zap/internal/state"
)

func newListCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "List configured providers and their install status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] == "favorites" {
				return listFavorites()
			}
			return listProviders(cfg)
		},
	}
	return c
}

func listProviders(cfg *config.Config) error {
	statuses := detect.Detect(cfg)
	s, _ := state.Load()

	for _, st := range statuses {
		star := " "
		if s != nil && s.IsFavoriteProvider(st.Provider.ID) {
			star = "⭐"
		}
		mark := "✗"
		extra := st.Provider.InstallHint
		if st.Installed {
			mark = "✓"
			extra = st.Path
		}
		icon := st.Provider.Icon
		if icon == "" {
			icon = "  "
		}
		fmt.Fprintf(os.Stdout, "%s %s %s  %-12s %-22s %s\n",
			star, mark, icon, st.Provider.ID, st.Provider.Name, extra)
	}
	return nil
}

func listFavorites() error {
	s, err := state.Load()
	if err != nil {
		return err
	}
	if len(s.FavoriteFolders) == 0 && len(s.FavoriteProviders) == 0 {
		fmt.Println("no favorites yet — use `zap favorite` to star the current folder")
		return nil
	}
	if len(s.FavoriteProviders) > 0 {
		fmt.Println("⭐ providers:")
		for _, p := range s.FavoriteProviders {
			fmt.Printf("   %s\n", p)
		}
	}
	if len(s.FavoriteFolders) > 0 {
		fmt.Println("⭐ folders:")
		for _, f := range s.FavoriteFolders {
			fmt.Printf("   %s\n", f)
		}
	}
	return nil
}
