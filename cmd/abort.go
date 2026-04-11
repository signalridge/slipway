package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type abortView struct {
	Slug            string `json:"slug"`
	ExecutionMode   string `json:"execution_mode"`
	Status          string `json:"status"`
	CurrentState    string `json:"current_state"`
	InterruptPIDs   []int  `json:"interrupt_pids,omitempty"`
	ForceKilledPIDs []int  `json:"force_killed_pids,omitempty"`
}

func makeAbortCmd() *cobra.Command {
	var changeSlug string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "abort",
		Short: desc("abort"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			active, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, active.Slug, "abort", func() error {
				change, err := loadActiveChange(
					root,
					active.Slug,
					"cannot abort non-active change status %q",
					"Only active changes can be aborted.",
				)
				if err != nil {
					return err
				}
				if change.CurrentState != model.StateS2Execute {
					return newInvalidUsageError(
						"abort_state_invalid",
						fmt.Sprintf("abort requires S2_EXECUTE state, current: %s", change.CurrentState),
						"Use `slipway cancel` to terminate and archive the change outside execution, or return to S2_EXECUTE before aborting.",
						nil,
					)
				}

				cfg, err := loadConfigAtRoot(root)
				if err != nil {
					return err
				}
				interrupted, forceKilled, err := preemptInFlightTasks(
					root,
					active.Slug,
					time.Duration(cfg.Execution.CancelGracePeriodSeconds)*time.Second,
				)
				if err != nil {
					return err
				}
				preemptionEvidenceRef, err := writePreemptionEvidence(root, active.Slug, "abort", interrupted, forceKilled)
				if err != nil {
					return err
				}

				change.ActiveCheckpoint = nil
				change.InterruptedExecutionAt = time.Now().UTC()
				if strings.TrimSpace(preemptionEvidenceRef) != "" {
					key := fmt.Sprintf("abort_preemption_%d", time.Now().UTC().UnixNano())
					change.EvidenceRefs[key] = preemptionEvidenceRef
				}
				if err := state.SaveChange(root, change); err != nil {
					return err
				}
				nextAction := "run"
				if execCtx, err := loadExecutionContext(root, change); err != nil {
					nextAction = "repair"
				} else {
					nextAction, _, _ = projectStatusExecutionAction(root, change, execCtx)
				}

				view := abortView{
					Slug:            active.Slug,
					ExecutionMode:   governedExecutionMode,
					Status:          string(change.Status),
					CurrentState:    string(change.CurrentState),
					InterruptPIDs:   interrupted,
					ForceKilledPIDs: forceKilled,
				}
				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}

				writer := newFormatWriter(cmd.OutOrStdout())
				writer.Writef("Execution aborted for %s\n", active.Slug)
				writer.Writef("State: %s\n", change.CurrentState)
				if len(interrupted) > 0 {
					writer.Writef("Interrupted PIDs: %v\n", interrupted)
				}
				if len(forceKilled) > 0 {
					writer.Writef("Force-killed PIDs: %v\n", forceKilled)
				}
				switch nextAction {
				case "repair":
					writer.Writef("Run `slipway repair`, then inspect `slipway status` before retrying execution.\n")
				default:
					writer.Writef("Use `slipway %s` to continue later, or `slipway status` to inspect blockers.\n", nextAction)
				}
				return writer.Err()
			})
		},
	}
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	return cmd
}
