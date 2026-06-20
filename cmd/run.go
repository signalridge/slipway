package cmd

import (
	"strings"
	"time"

	ctxpack "github.com/signalridge/slipway/internal/engine/context"
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
		auto           bool
		noAuto         bool
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

			effectiveAuto, err := resolveEffectiveAuto(root, cmd, auto, noAuto)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, ref.Slug, "run", func() error {
				// Auto-acknowledge a non-sensitive human_verify checkpoint at the
				// entry path so entry validation passes and the loop continues past
				// it. The injected response flows through both entry validation and
				// the loop, so the checkpoint is consumed exactly as an operator
				// --resume-response would. Decision/human_action/guardrail checkpoints
				// are deliberately left untouched and still fail closed.
				effectiveResumeResponse, autoCheckpointAcknowledged, err := autoAckResumeResponse(root, ref, effectiveAuto, resume, resumeResponse)
				if err != nil {
					return err
				}
				if err := validateRunEntry(root, ref, resume, effectiveResumeResponse); err != nil {
					return err
				}
				view, err := runGovernedLoopWithCheckpointAck(root, ref, effectiveResumeResponse, effectiveAuto, autoCheckpointAcknowledged)
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
	cmd.Flags().BoolVar(&auto, "auto", false, "Force auto-advance execution for this run, overriding execution.auto config")
	cmd.Flags().BoolVar(&noAuto, "no-auto", false, "Force manual advancement for this run, overriding execution.auto config")
	cmd.MarkFlagsMutuallyExclusive("auto", "no-auto")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	return cmd
}

// resolveEffectiveAuto resolves the tri-state auto override. `--auto` forces auto
// for the run; `--no-auto` forces manual. Otherwise the project config value
// (execution.auto) is the effective setting. The flags are mutually exclusive
// (enforced by cobra) so at most one is ever Changed. A negative `--no-auto=false`
// is NOT an affirmative auto override: it falls through to config rather than
// flipping auto on, so the negative safety flag can never enable auto by itself.
func resolveEffectiveAuto(root string, cmd *cobra.Command, auto, noAuto bool) (bool, error) {
	if cmd != nil {
		if cmd.Flags().Changed("auto") {
			return auto, nil
		}
		if cmd.Flags().Changed("no-auto") && noAuto {
			return false, nil
		}
	}
	cfg, err := loadConfigAtRoot(root)
	if err != nil {
		return false, err
	}
	return cfg.Execution.AutoEnabled(), nil
}

// autoAckResumeResponse returns the resume response to use for entry validation
// and the run loop. When auto is effective, no operator response was supplied,
// and an active checkpoint is a non-sensitive human_verify, it injects a
// standing auto acknowledgment so the entry validator permits continuation and
// the loop consumes the checkpoint. Decision and human_action checkpoints, and
// any checkpoint in a guardrail/sensitive domain, are never auto-acknowledged —
// they keep failing closed exactly as in the non-auto path.
func autoAckResumeResponse(root string, ref changeRef, auto, resume bool, resumeResponse string) (string, bool, error) {
	if !auto || resume || strings.TrimSpace(resumeResponse) != "" {
		return resumeResponse, false, nil
	}
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return "", false, err
	}
	eligible, err := autoAckEligibleCheckpoint(root, change)
	if err != nil {
		return "", false, err
	}
	if !eligible {
		return resumeResponse, false, nil
	}
	return autoAcknowledgedResponse, true, nil
}

// autoAckEligibleCheckpoint reports whether the change has an active checkpoint
// that may be auto-acknowledged under auto: a FRESH human_verify checkpoint in a
// non-guardrail change. Decision and human_action checkpoints, and any checkpoint
// in a guardrail/sensitive domain, are excluded so they remain manual. A stale or
// unknown-freshness checkpoint is also excluded: auto must never silently consume
// outdated checkpoint state, so it keeps failing closed to the manual hard stop
// exactly as in the non-auto path.
func autoAckEligibleCheckpoint(root string, change model.Change) (bool, error) {
	if change.ActiveCheckpoint == nil {
		return false, nil
	}
	if isGuardrailSensitive(change.GuardrailDomain) {
		return false, nil
	}
	if model.CheckpointKind(change.ActiveCheckpoint.CheckpointType) != model.CheckpointHumanVerify {
		return false, nil
	}
	execCtx, err := loadExecutionContext(root, change)
	if err != nil {
		return false, err
	}
	freshness := projectFreshnessForExecMode(root, change, execCtx.Summary, execCtx.SummaryBlockers)
	return freshness == string(ctxpack.EvidenceFreshnessFresh), nil
}

// autoAcknowledgedResponse is the standing response injected for an
// auto-acknowledged non-sensitive human_verify checkpoint.
const autoAcknowledgedResponse = "auto-acknowledged"

// isGuardrailSensitive reports whether a change's guardrail domain marks it as a
// sensitive domain that must keep failing closed to manual review under auto.
func isGuardrailSensitive(guardrailDomain string) bool {
	return strings.TrimSpace(guardrailDomain) != ""
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

func runGovernedLoop(root string, ref changeRef, resumeResponse string, auto bool) (nextView, error) {
	buildNext := func(nextResumeResponse string) (nextView, error) {
		return buildNextViewForCommand(root, ref, nextResumeResponse, false, true, false, "run", auto, false)
	}
	return runGovernedLoopWithBuilder(root, ref, resumeResponse, buildNext)
}

func runGovernedLoopWithCheckpointAck(
	root string,
	ref changeRef,
	resumeResponse string,
	auto bool,
	autoCheckpointAcknowledged bool,
) (nextView, error) {
	nextAutoCheckpointAcknowledged := autoCheckpointAcknowledged
	buildNext := func(nextResumeResponse string) (nextView, error) {
		view, err := buildNextViewForCommand(root, ref, nextResumeResponse, false, true, false, "run", auto, nextAutoCheckpointAcknowledged)
		nextAutoCheckpointAcknowledged = false
		return view, err
	}
	return runGovernedLoopWithBuilder(root, ref, resumeResponse, buildNext)
}

func runGovernedLoopWithBuilder(
	root string,
	ref changeRef,
	resumeResponse string,
	buildNext func(string) (nextView, error),
) (nextView, error) {
	const maxIterations = maxAutoNextIterations

	var lastView nextView
	transitions := make([]progression.AdvanceSummary, 0, maxIterations)
	nextResumeResponse := resumeResponse
	delegatedTo := "run"
	if change, err := state.LoadChange(root, ref.Slug); err == nil {
		delegatedTo = primaryCommandForState(change.CurrentState)
	}
	for i := 0; i < maxIterations; i++ {
		view, err := buildNext(nextResumeResponse)
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
