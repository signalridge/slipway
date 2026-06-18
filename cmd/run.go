package cmd

import (
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

func makeRunCmd() *cobra.Command {
	var (
		jsonOutput     bool
		resume         bool
		resumeResponse string
		diagnostics    bool
		changeSlug     string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: desc("run"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}
			if err := validateRunFlags(resume, resumeResponse); err != nil {
				return err
			}

			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, ref.Slug, "run", func() error {
				if err := validateRunEntry(root, ref, resume, resumeResponse); err != nil {
					return err
				}
				view, err := runGovernedLoop(root, ref, resumeResponse)
				if err != nil {
					return err
				}
				applyNextInvocationWorkspacePath(cmd, root, &view)
				change, err := state.LoadChange(root, ref.Slug)
				if err != nil {
					return err
				}
				if !change.InterruptedExecutionAt.IsZero() {
					beforeChange := change
					change.InterruptedExecutionAt = time.Time{}
					if err := state.SaveChange(root, change); err != nil {
						return err
					}
					if err := appendCLILifecycleEvent(root, change, state.LifecycleEvent{
						Command:       "run",
						EventType:     "resume.succeeded",
						Action:        "resumed",
						Reason:        "interrupted_execution_cleared",
						Result:        "success",
						BeforeState:   beforeChange.CurrentState,
						AfterState:    change.CurrentState,
						ClearedFields: []string{"interrupted_execution_at"},
					}); err != nil {
						return err
					}
				}
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
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume governed execution from the latest incomplete wave when no active checkpoint exists")
	cmd.Flags().StringVar(&resumeResponse, "resume-response", "", "Response text for a paused checkpoint")
	cmd.Flags().BoolVar(&diagnostics, "diagnostics", false, "Include diagnostic governance/readiness details")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	return cmd
}

func validateRunFlags(resume bool, resumeResponse string) error {
	if resume && strings.TrimSpace(resumeResponse) != "" {
		return newCLIError(
			categoryInvalidUsage,
			"flag_conflict",
			"--resume cannot be used with --resume-response",
			"Use --resume for non-checkpoint resumes, or --resume-response for active checkpoints.",
			"",
			nil,
		)
	}
	return nil
}

func validateRunEntry(root string, ref changeRef, resume bool, resumeResponse string) error {
	return validateResumeEntryForCommand(root, ref, resume, resumeResponse, "run")
}

func validateResumeEntryForCommand(
	root string,
	ref changeRef,
	resume bool,
	resumeResponse string,
	commandName string,
) error {
	commandName = strings.TrimSpace(commandName)
	if commandName == "" {
		commandName = "run"
	}
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return err
	}
	execCtx, err := loadExecutionContext(root, change)
	if err != nil {
		return err
	}

	if change.ActiveCheckpoint != nil {
		if resume {
			return newInvalidUsageError(
				"resume_response_required",
				"active checkpoint exists; use --resume-response instead of --resume",
				"Resume the active checkpoint with `slipway "+commandName+" --resume-response \"<response>\"`.",
				nil,
			)
		}
		if strings.TrimSpace(resumeResponse) == "" {
			return validateResumeResponse(change.ActiveCheckpoint, "")
		}
		if err := validateActiveCheckpointAuthority(root, change, execCtx, commandName); err != nil {
			return err
		}
		return nil
	}

	resumeWaveIndex, err := loadResumableWaveExecution(root, change, execCtx, commandName)
	if err != nil {
		if !resumableWavePlanHasStructuralDrift(root, change) {
			return err
		}
		resumeWaveIndex = 0
	}
	hasResumeContext := resumeWaveIndex > 0
	if hasResumeContext && resumableWavePlanHasStructuralDrift(root, change) {
		hasResumeContext = false
	}

	switch {
	case resume && !hasResumeContext:
		return newInvalidUsageError(
			"resume_unavailable",
			"--resume requested but no resumable governed execution state is available; current_state="+string(change.CurrentState),
			"Resume only applies to interrupted S2_IMPLEMENT wave execution. For the current state, use `slipway "+commandName+"`, `slipway validate --json`, or record the required review evidence.",
			map[string]any{
				"current_state":    change.CurrentState,
				"resumable_states": []model.WorkflowState{model.StateS2Implement},
				"next_action":      "use normal " + commandName + "/validate/review evidence flow for non-resumable states",
			},
		)
	case !resume && hasResumeContext:
		return newInvalidUsageError(
			"resume_required",
			"resumable governed execution detected; use --resume to continue from the latest incomplete wave",
			"Resume the current incomplete wave with `slipway "+commandName+" --resume`.",
			nil,
		)
	}

	return nil
}

func resumableWavePlanHasStructuralDrift(root string, change model.Change) bool {
	plan, err := state.LoadWavePlanForChange(root, change)
	if err != nil {
		return false
	}
	currentHash, err := state.CurrentTasksPlanStructuralState(root, change)
	if err != nil {
		return false
	}
	plan.Normalize()
	planHash := strings.TrimSpace(plan.EffectiveStructuralHash)
	if planHash == "" {
		planHash = strings.TrimSpace(plan.TasksPlanStructuralHash)
	}
	if planHash == "" {
		planHash = strings.TrimSpace(plan.TasksPlanHash)
	}
	return planHash != "" && currentHash != "" && planHash != currentHash
}

func runGovernedLoop(root string, ref changeRef, resumeResponse string) (nextView, error) {
	const maxIterations = maxAutoNextIterations

	var lastView nextView
	transitions := make([]progression.AdvanceSummary, 0, maxIterations)
	nextResumeResponse := resumeResponse
	delegatedTo := "run"
	if change, err := state.LoadChange(root, ref.Slug); err == nil {
		delegatedTo = primaryCommandForState(change.CurrentState)
	}
	for i := 0; i < maxIterations; i++ {
		view, err := buildNextViewForCommand(root, ref, nextResumeResponse, false, true, false, "run")
		if err != nil {
			return nextView{}, err
		}
		setRunDelegation(&view, delegatedTo)
		nextResumeResponse = ""
		lastView = view
		if view.Advanced != nil && (view.Advanced.Action == "advanced" || view.Advanced.Action == "done_ready") {
			transitions = append(transitions, *view.Advanced)
		}
		if shouldStopRunLoop(view) {
			break
		}
	}
	if len(transitions) > 0 {
		lastView.AutoTransitions = transitions
	}
	return lastView, nil
}

func shouldStopRunLoop(view nextView) bool {
	switch {
	case view.CurrentState == model.StateDone:
		return true
	case len(view.Blockers) > 0:
		return true
	case hasPendingRunCheckpoint(view.InputContext.ResumeCheckpoint):
		return true
	case view.Advanced != nil && view.Advanced.Action == "done_ready":
		return true
	case view.NextSkill != nil:
		return true
	case view.Advanced == nil || view.Advanced.Action == "noop":
		return true
	default:
		return false
	}
}

func hasPendingRunCheckpoint(checkpoint *resumeCheckpoint) bool {
	if checkpoint == nil {
		return false
	}
	if strings.TrimSpace(checkpoint.PausedTaskID) == "" {
		return false
	}
	return strings.TrimSpace(checkpoint.UserResponsePayload) == ""
}
