package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/Ryoshkenn/zap/internal/config"
)

func newConfigCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Show or edit the user config file",
	}
	c.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the resolved config path",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.ConfigPath()
			if err != nil {
				return err
			}
			fmt.Println(p)
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Open config.yaml in $EDITOR (creating it if missing)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.EnsureConfigDir()
			if err != nil {
				return err
			}
			path, err := config.ConfigPath()
			if err != nil {
				return err
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				stub := "# zap config — see https://github.com/Ryoshkenn/zap\n" +
					"# providers:\n" +
					"#   claude:\n" +
					"#     default_flags: [\"--dangerously-skip-permissions\"]\n" +
					"# custom_providers:\n" +
					"#   - id: my-cli\n" +
					"#     name: My CLI\n" +
					"#     command: mycli\n"
				if err := os.WriteFile(path, []byte(stub), 0o644); err != nil {
					return err
				}
				fmt.Printf("created %s\n", path)
			}
			_ = dir
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = defaultEditor()
			}
			cmd2 := exec.Command(editor, path)
			cmd2.Stdin = os.Stdin
			cmd2.Stdout = os.Stdout
			cmd2.Stderr = os.Stderr
			return cmd2.Run()
		},
	})
	// `zap config` with no subcommand prints the path.
	c.RunE = func(cmd *cobra.Command, args []string) error {
		p, err := config.ConfigPath()
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	}
	return c
}
