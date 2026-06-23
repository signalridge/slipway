package cmd

import (
	"fmt"

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
			root, err := workspaceRootFromCommandOrWD(cmd)
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

			// Make the per-tool invocation surface explicit at setup time. When
			// --tools was omitted with --refresh, the generated set is the
			// auto-detected sentinelized adapters.
			generatedTools := selectedTools
			if len(generatedTools) == 0 && opts.refresh && !toolsSpecified {
				generatedTools = toolgen.DetectExistingTools(root)
			}
			for _, toolID := range generatedTools {
				cfg, ok := toolgen.LookupTool(toolID)
				if !ok {
					continue
				}
				writer.Writeln(fmt.Sprintf("  %s: %s", toolID, cfg.InvocationSummary()))
				if toolID == "codex" {
					writer.Writeln("    codex hooks: generated in .codex/config.toml but inert until Codex trusts this repo and each hook; Slipway never edits global Codex trust")
				}
			}
			return writer.Err()
		},
	}

	cmd.Flags().StringVar(&opts.tools, "tools", "", "Tool adapters to generate: all|none|comma list (e.g. claude,cursor)")
	cmd.Flags().BoolVar(&opts.refresh, "refresh", false, "Regenerate tool artifacts deterministically")

	return cmd
}
