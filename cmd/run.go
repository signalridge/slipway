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
		quickMode      bool
		changeSlug     string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: desc("run"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
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
				view, err := runGovernedLoop(root, ref, resumeResponse, quickMode)
				if err != nil {
					return err
				}
				change, err := state.LoadChange(root, ref.Slug)
				if err != nil {
					return err
				}
				if !change.InterruptedExecutionAt.IsZero() {
					change.InterruptedExecutionAt = time.Time{}
					if err := state.SaveChange(root, change); err != nil {
						return err
					}
				}
				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}
				return writeNextHuman(cmd.OutOrStdout(), view)
			})
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume governed execution from the latest incomplete wave when no active checkpoint exists")
	cmd.Flags().StringVar(&resumeResponse, "resume-response", "", "Response text for a paused checkpoint")
	cmd.Flags().BoolVar(&quickMode, "quick", false, "Disable advisory controls (clarification, research, independent_review, worktree_isolation)")
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
				"Resume the active checkpoint with `slipway run --resume-response \"<response>\"`.",
				nil,
			)
		}
		if strings.TrimSpace(resumeResponse) == "" {
			return validateResumeResponse(change.ActiveCheckpoint, "")
		}
		if err := validateActiveCheckpointAuthority(root, change, execCtx, "run"); err != nil {
			return err
		}
		return nil
	}

	_, resumeWaveIndex, err := loadResumableWaveExecution(root, change, execCtx, "run")
	if err != nil {
		return err
	}
	hasResumeContext := resumeWaveIndex > 0

	switch {
	case resume && !hasResumeContext:
		return newInvalidUsageError(
			"resume_unavailable",
			"--resume requested but no resumable governed execution state is available",
			"Run `slipway run` to start a fresh execution loop, or use `slipway status` to inspect the current wave state.",
			nil,
		)
	case !resume && hasResumeContext:
		return newInvalidUsageError(
			"resume_required",
			"resumable governed execution detected; use --resume to continue from the latest incomplete wave",
			"Resume the current incomplete wave with `slipway run --resume`.",
			nil,
		)
	}

	return nil
}

func runGovernedLoop(root string, ref changeRef, resumeResponse string, quickMode bool) (nextView, error) {
	const maxIterations = maxAutoNextIterations

	var lastView nextView
	transitions := make([]progression.AdvanceSummary, 0, maxIterations)
	nextResumeResponse := resumeResponse
	for i := 0; i < maxIterations; i++ {
		view, err := buildNextView(root, ref, nextResumeResponse, false, true, false, quickMode)
		if err != nil {
			return nextView{}, err
		}
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
