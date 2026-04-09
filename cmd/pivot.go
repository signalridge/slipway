package cmd

import (
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/spf13/cobra"
)

type pivotView struct {
	Slug          string `json:"slug"`
	Kind          string `json:"kind"`
	ExecutionMode string `json:"execution_mode"`
	CurrentState  string `json:"current_state"`
}

func makePivotCmd() *cobra.Command {
	var changeSlug string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "pivot",
		Short: "Reroute or rescope an active change",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Resolve pivot kind from --reroute/--rescope flags; default reroute.
			kind := string(gate.PivotKindReroute)
			if f := cmd.Flags().Lookup("rescope"); f != nil && f.Changed {
				kind = string(gate.PivotKindRescope)
			}

			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			active, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, active.Slug, "pivot", func() error {
				_, err := loadActiveChange(
					root,
					active.Slug,
					"cannot pivot non-active change status %q",
					"Only active changes can be pivoted.",
				)
				if err != nil {
					return err
				}

				var view pivotView
				// All slipway changes are governed.
				view, err = executeGovernedPivot(root, active.Slug, kind)
				if err != nil {
					return err
				}

				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}
				writer := newFormatWriter(cmd.OutOrStdout())
				writer.Writef("Change: %s\n", view.Slug)
				writer.Writef("Kind: %s | Mode: %s\n", view.Kind, view.ExecutionMode)
				writer.Writef("State: %s\n", view.CurrentState)
				return writer.Err()
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().Bool("reroute", false, "Reroute: re-evaluate routing/discovery path (default when no flag given)")
	cmd.Flags().Bool("rescope", false, "Rescope: adjust scope within the current governed change")
	return cmd
}
