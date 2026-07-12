package cmd

import (
	"fmt"

	"github.com/signalridge/slipway/internal/adapter"
	"github.com/spf13/cobra"
)

type listOutput struct {
	ContractVersion int                  `json:"contract_version"`
	Hosts           []adapter.HostStatus `json:"hosts"`
}

func makeListOutput(statuses []adapter.HostStatus) listOutput {
	return listOutput{
		ContractVersion: machineContractVersion,
		Hosts:           append([]adapter.HostStatus{}, statuses...),
	}
}

func makeListCmd() *cobra.Command {
	var root string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List host adapters and their installation state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := resolveRoot(root)
			if err != nil {
				return err
			}
			statuses, err := adapter.List(resolved)
			if err != nil {
				return newRuntimeError("list_failed", err.Error(), inputlessCommandNext(resolved, "retry-list", "slipway", "list", "--root", resolved), nil)
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), makeListOutput(statuses))
			}
			for _, status := range statuses {
				if _, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"%-10s detected=%-5t installed=%-5t needs_refresh=%-5t capabilities=%d\n",
					status.ID,
					status.Detected,
					status.Installed,
					status.NeedsRefresh,
					len(status.Capabilities),
				); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "workspace root (default: current directory)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "emit JSON")
	return cmd
}
