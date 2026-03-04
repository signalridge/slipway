package cmd

import (
	"fmt"
	"os"

	"github.com/signalridge/speclane/internal/bootstrap"
	"github.com/signalridge/speclane/internal/toolgen"
	"github.com/spf13/cobra"
)

type initOptions struct {
	tools   string
	refresh bool
}

func newInitCmd() *cobra.Command {
	opts := initOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize runtime layout and optional tool artifacts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := os.Getwd()
			if err != nil {
				return err
			}

			selectedTools, err := toolgen.ResolveTools(opts.tools)
			if err != nil {
				return err
			}

			if err := bootstrap.InitWorkspace(root, selectedTools, opts.refresh); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "initialized speclane workspace")
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.tools, "tools", "all", "Tool generation target set: all|none|comma list")
	cmd.Flags().BoolVar(&opts.refresh, "refresh", false, "Regenerate tool artifacts deterministically")

	return cmd
}
