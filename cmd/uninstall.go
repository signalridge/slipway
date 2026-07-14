package cmd

import (
	"github.com/signalridge/slipway/internal/adapter"
	"github.com/spf13/cobra"
)

func makeUninstallCmd() *cobra.Command {
	var root string
	var tools []string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove pristine Slipway-managed host capabilities",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := resolveRoot(root)
			if err != nil {
				return err
			}
			report, err := adapter.Uninstall(adapter.UninstallOptions{Root: resolved, Tools: tools})
			if err != nil {
				if selectionErr := unknownHostSelectionCLIError(err, resolved); selectionErr != nil {
					return selectionErr
				}
				return adapterMutationError("uninstall_failed", err, resolved, report)
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), makeChangeReportOutput(report))
			}
			return writeChangeReport(cmd, "Uninstalled", report)
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "workspace root (default: current directory)")
	cmd.Flags().StringSliceVar(&tools, "tool", nil, "host adapter to uninstall; repeat or use all")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "emit JSON")
	return cmd
}
