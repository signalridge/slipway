package cmd

import (
	"os"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/toolgen"
	"github.com/spf13/cobra"
)

type initOptions struct {
	tools   string
	refresh bool
}

func makeInitCmd() *cobra.Command {
	opts := initOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: desc("init"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := os.Getwd()
			if err != nil {
				return err
			}

			toolsSpecified := cmd.Flags().Changed("tools")
			selectedTools, err := toolgen.ResolveTools(opts.tools)
			if err != nil {
				return err
			}

			if err := bootstrap.InitWorkspace(root, selectedTools, opts.refresh, toolsSpecified); err != nil {
				return err
			}

			writer := newFormatWriter(cmd.OutOrStdout())
			writer.Writeln("initialized slipway workspace")
			return writer.Err()
		},
	}

	cmd.Flags().StringVar(&opts.tools, "tools", "", "Tool adapters to generate: all|none|comma list (e.g. claude,cursor)")
	cmd.Flags().BoolVar(&opts.refresh, "refresh", false, "Regenerate tool artifacts deterministically")

	return cmd
}
