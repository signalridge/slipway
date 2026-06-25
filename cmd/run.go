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
		jsonOutput  bool
		resume      bool
		diagnostics bool
		changeSlug  string
		auto        bool
		noAuto      bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: desc("run"),
		Long: desc("run") + `

Repo-level configuration keys (including execution.auto, which --auto / --no-auto
override for a single run) are inspected and changed with ` + "`slipway config`" + `
(list/get/set).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
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
				if err := validateRunEntry(root, ref, resume); err != nil {
					return err
				}
				view, err := runGovernedLoop(root, ref, effectiveAuto)
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
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume governed execution from the latest incomplete wave")
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

// isGuardrailSensitive reports whether a change's guardrail domain marks it as a
// sensitive domain that must keep failing closed to manual review under auto.
//
// An empty domain reports non-sensitive on the authority of upstream intake
// classification: a sensitive change carries a non-empty GuardrailDomain set by
// the governed intake/plan stages, so a blank domain here means the classifier
// found no sensitive domain rather than that classification was skipped.
func isGuardrailSensitive(guardrailDomain string) bool {
	return strings.TrimSpace(guardrailDomain) != ""
}

func validateRunEntry(root string, ref changeRef, resume bool) error {
	return validateResumeEntryForCommand(root, ref, resume, "run")
}

func validateResumeEntryForCommand(
	root string,
	ref changeRef,
	resume bool,
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

func runGovernedLoop(
	root string,
	ref changeRef,
	auto bool,
) (nextView, error) {
	buildNext := func() (nextView, error) {
		view, err := buildNextViewForCommand(root, ref, nextViewOptions{
			AutoSkipEvidence: true,
			Command:          "run",
			Auto:             auto,
		})
		return view, err
	}
	return runGovernedLoopWithBuilder(root, ref, buildNext)
}

func runGovernedLoopWithBuilder(
	root string,
	ref changeRef,
	buildNext func() (nextView, error),
) (nextView, error) {
	const maxIterations = maxAutoNextIterations

	var lastView nextView
	transitions := make([]progression.AdvanceSummary, 0, maxIterations)
	delegatedTo := "run"
	if change, err := state.LoadChange(root, ref.Slug); err == nil {
		delegatedTo = primaryCommandForState(change.CurrentState)
	}
	for i := 0; i < maxIterations; i++ {
		view, err := buildNext()
		if err != nil {
			return nextView{}, err
		}
		setRunDelegation(&view, delegatedTo)
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
