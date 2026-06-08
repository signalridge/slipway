package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type presetView struct {
	Slug           string `json:"slug"`
	WorkflowPreset string `json:"workflow_preset"`
}

func parseWorkflowPreset(raw string) (model.WorkflowPreset, error) {
	preset := model.WorkflowPreset(strings.TrimSpace(raw))
	if preset == "" {
		return "", nil
	}
	if !preset.IsValid() {
		return "", newInvalidUsageError(
			"invalid_preset",
			fmt.Sprintf("invalid --preset %q; expected light|standard|strict", raw),
			"Use one of: light, standard, strict.",
			nil,
		)
	}
	return preset, nil
}

func makePresetCmd() *cobra.Command {
	var changeSlug string
	cmd := &cobra.Command{
		Use:   "preset <light|standard|strict>",
		Short: desc("preset"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}
			preset, err := parseWorkflowPreset(args[0])
			if err != nil {
				return err
			}
			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}
			return withChangeStateLock(root, ref.Slug, "preset", func() error {
				change, err := state.LoadChange(root, ref.Slug)
				if err != nil {
					return err
				}
				// After leaving S1_PLAN, only allow upgrades (never downgrades).
				// This prevents relaxing governance mid-flight (e.g., switching
				// from strict to light at S3/S4 to bypass review/closeout).
				// Both S0_INTAKE and S1_PLAN allow free preset changes.
				if change.CurrentState != model.StateS0Intake && change.CurrentState != model.StateS1Plan {
					current := change.WorkflowPreset
					if !current.IsValid() {
						current = model.WorkflowPresetStandard
					}
					if preset.Rank() < current.Rank() {
						return newCLIError(
							categoryInvalidUsage,
							"preset_downgrade_rejected",
							fmt.Sprintf("cannot downgrade preset from %s to %s after leaving S1_PLAN", current, preset),
							"Preset can only be upgraded after planning. To restart with a lighter preset, pivot or cancel the change.",
							"",
							nil,
						)
					}
				}
				// Snapshot pre-command preset state so we can restore on
				// scaffold failure instead of blindly reverting to pending.
				origPreset := change.WorkflowPreset
				origSuggested := change.SuggestedWorkflowPreset

				needsScaffold := change.WorkflowPresetConfirmationPending() || !change.WorkflowPreset.IsValid() ||
					!progression.CheckGovernedBundleReady(root, change) ||
					preset != change.WorkflowPreset // Re-scaffold on preset upgrade to materialize artifacts (e.g. assurance.md)
				change.WorkflowPreset = preset
				change.SuggestedWorkflowPreset = ""
				if err := state.SaveChange(root, change); err != nil {
					return err
				}

				// During S0_INTAKE, preset confirmation must not materialize
				// downstream artifacts from empty templates. Keep only intent.md;
				// S1_PLAN/bundle scaffolds the full bundle from confirmed intent.
				if needsScaffold {
					var scaffoldErr error
					if change.CurrentState == model.StateS0Intake {
						scaffoldErr = artifact.ScaffoldIntentForChange(root, change)
					} else {
						resolution := progression.ResolveChangeSchemaDiagnostics(change)
						if len(resolution.Blockers) > 0 {
							err := fmt.Errorf("resolve artifact schema: %s", strings.Join(resolution.Blockers, ","))
							if restoreErr := restorePresetOnScaffoldFailure(root, &change, origPreset, origSuggested); restoreErr != nil {
								return errors.Join(err, restoreErr)
							}
							return err
						}
						policy, err := governance.ResolvePresetPolicy(root, change)
						if err != nil {
							if restoreErr := restorePresetOnScaffoldFailure(root, &change, origPreset, origSuggested); restoreErr != nil {
								return errors.Join(err, restoreErr)
							}
							return err
						}
						scaffoldErr = artifact.ScaffoldGovernedBundleForChange(root, change, policy.EffectivePreset, resolution.Schema)
					}
					if scaffoldErr != nil {
						if restoreErr := restorePresetOnScaffoldFailure(root, &change, origPreset, origSuggested); restoreErr != nil {
							return errors.Join(scaffoldErr, restoreErr)
						}
						return scaffoldErr
					}
				}
				if _, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
					Command:       "preset",
					EventType:     "preset.changed",
					Action:        "confirmed",
					Reason:        fmt.Sprintf("%s->%s", origPreset, change.WorkflowPreset),
					Result:        "ok",
					BeforeState:   change.CurrentState,
					AfterState:    change.CurrentState,
					BeforeSubStep: string(change.PlanSubStep),
					AfterSubStep:  string(change.PlanSubStep),
					SideEffects: []state.LifecycleSideEffect{
						{Kind: "workflow_preset_confirmed", Detail: string(change.WorkflowPreset)},
					},
				}); err != nil {
					return err
				}

				jsonFlag, _ := cmd.Flags().GetBool("json")
				if jsonFlag {
					return encodeJSONResponse(cmd, presetView{
						Slug:           change.Slug,
						WorkflowPreset: string(change.WorkflowPreset),
					})
				}
				writer := newFormatWriter(cmd.OutOrStdout())
				writer.Writef("preset confirmed: %s  change=%s\n", change.WorkflowPreset, change.Slug)
				writer.Writeln("next: slipway next")
				return writer.Err()
			})
		},
	}
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}

// restorePresetOnScaffoldFailure restores the pre-command preset state after
// scaffold failure. For first-time confirmations (origPreset empty), this
// reverts to pending-suggestion. For re-scaffolds of an already-confirmed
// preset, this preserves the original confirmed state so a transient failure
// doesn't erase valid governance decisions.
func restorePresetOnScaffoldFailure(root string, change *model.Change, origPreset, origSuggested model.WorkflowPreset) error {
	if change == nil {
		return nil
	}
	if origPreset.IsValid() {
		// Re-scaffold of already-confirmed preset: restore original state.
		change.WorkflowPreset = origPreset
		change.SuggestedWorkflowPreset = origSuggested
	} else {
		// First-time confirmation failed: revert to pending with the
		// attempted preset as suggestion so the user sees what they tried.
		change.SuggestedWorkflowPreset = change.WorkflowPreset
		change.WorkflowPreset = ""
	}
	if err := state.SaveChange(root, *change); err != nil {
		return fmt.Errorf("restore preset after scaffold failure: %w", err)
	}
	return nil
}
