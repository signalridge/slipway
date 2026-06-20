package cmd

import (
	"fmt"
	"strings"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type stageCommandSpec struct {
	Name           string
	ExpectedState  model.WorkflowState
	SupportsResume bool
}

func makeIntakeCmd() *cobra.Command {
	return makeStageCmd(stageCommandSpec{
		Name:          "intake",
		ExpectedState: model.StateS0Intake,
	})
}

func makePlanCmd() *cobra.Command {
	return makeStageCmd(stageCommandSpec{
		Name:          "plan",
		ExpectedState: model.StateS1Plan,
	})
}

func makeImplementCmd() *cobra.Command {
	return makeStageCmd(stageCommandSpec{
		Name:           "implement",
		ExpectedState:  model.StateS2Implement,
		SupportsResume: true,
	})
}

func makeStageCmd(spec stageCommandSpec) *cobra.Command {
	var (
		jsonOutput     bool
		resume         bool
		resumeResponse string
		diagnostics    bool
		changeSlug     string
	)

	cmd := &cobra.Command{
		Use:   spec.Name,
		Short: desc(spec.Name),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}
			if spec.SupportsResume {
				if err := validateRunFlags(resume, resumeResponse); err != nil {
					return err
				}
			}

			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			// Stage commands carry the project's effective auto setting (no per-stage
			// flag), so config-level auto-advance applies consistently with `run`.
			auto, err := resolveEffectiveAuto(root, nil, false, false)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, ref.Slug, spec.Name, func() error {
				if err := validateStageCommandEntry(root, ref, spec); err != nil {
					return err
				}
				effectiveResumeResponse := resumeResponse
				autoCheckpointAcknowledged := false
				if spec.SupportsResume {
					var err error
					effectiveResumeResponse, autoCheckpointAcknowledged, err = autoAckResumeResponse(root, ref, auto, resume, resumeResponse)
					if err != nil {
						return err
					}
					if err := validateResumeEntryForCommand(root, ref, resume, effectiveResumeResponse, spec.Name); err != nil {
						return err
					}
				}
				view, err := runStageLoop(root, ref, spec, effectiveResumeResponse, auto, autoCheckpointAcknowledged)
				if err != nil {
					return err
				}
				applyNextInvocationWorkspacePath(cmd, root, &view)
				if jsonOutput {
					if diagnostics {
						return encodeJSONResponse(cmd, view)
					}
					return encodeJSONResponse(cmd, buildNextHandoffView(view))
				}
				if diagnostics {
					return encodeJSONResponse(cmd, view)
				}
				return writeNextHuman(cmd.OutOrStdout(), view)
			})
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.Flags().BoolVar(&diagnostics, "diagnostics", false, "Include diagnostic governance/readiness details")
	if spec.SupportsResume {
		cmd.Flags().BoolVar(&resume, "resume", false, "Resume governed implementation from the latest incomplete wave when no active checkpoint exists")
		cmd.Flags().StringVar(&resumeResponse, "resume-response", "", "Response text for a paused implementation checkpoint")
	}
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	return cmd
}

func validateStageCommandEntry(root string, ref changeRef, spec stageCommandSpec) error {
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return err
	}
	if change.CurrentState == spec.ExpectedState {
		return nil
	}
	currentCommand := primaryCommandForState(change.CurrentState)
	return newGovernanceBlockedError(
		spec.Name+"_state_invalid",
		fmt.Sprintf("slipway %s can only run while current_state is %s; current_state=%s", spec.Name, spec.ExpectedState, change.CurrentState),
		fmt.Sprintf("Use `slipway %s` for the current state, or `slipway run` to drive the current stage automatically.", currentCommand),
		ref.Slug,
		map[string]any{
			"current_state":  change.CurrentState,
			"expected_state": spec.ExpectedState,
			"next_command":   "slipway " + currentCommand,
		},
	)
}

func runStageLoop(root string, ref changeRef, spec stageCommandSpec, resumeResponse string, auto bool, autoCheckpointAcknowledged bool) (nextView, error) {
	const maxIterations = maxAutoNextIterations

	var lastView nextView
	transitions := make([]progression.AdvanceSummary, 0, maxIterations)
	nextResumeResponse := resumeResponse
	nextAutoCheckpointAcknowledged := autoCheckpointAcknowledged
	for i := 0; i < maxIterations; i++ {
		view, err := buildNextViewForCommand(root, ref, nextViewOptions{
			ResumeResponse:             nextResumeResponse,
			AutoSkipEvidence:           true,
			Command:                    spec.Name,
			Auto:                       auto,
			AutoCheckpointAcknowledged: nextAutoCheckpointAcknowledged,
		})
		if err != nil {
			return nextView{}, err
		}
		view.Command = spec.Name
		view.DelegatedTo = spec.Name
		nextResumeResponse = ""
		nextAutoCheckpointAcknowledged = false
		lastView = view
		if view.Advanced != nil && (view.Advanced.Action == "advanced" || view.Advanced.Action == "done_ready") {
			transitions = append(transitions, *view.Advanced)
		}
		if view.CurrentState != spec.ExpectedState || shouldStopRunLoop(view) {
			break
		}
	}
	if len(transitions) > 0 {
		lastView.AutoTransitions = transitions
	}
	return lastView, nil
}

func primaryCommandForState(workflowState model.WorkflowState) string {
	switch workflowState {
	case model.StateS0Intake:
		return "intake"
	case model.StateS1Plan:
		return "plan"
	case model.StateS2Implement:
		return "implement"
	case model.StateS3Review:
		return "review"
	case model.StateDone:
		return "done"
	default:
		return "run"
	}
}

func setRunDelegation(view *nextView, delegatedTo string) {
	if view == nil {
		return
	}
	view.Command = "run"
	view.DelegatedTo = strings.TrimSpace(delegatedTo)
}
