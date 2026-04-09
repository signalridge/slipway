package cmd

import (
	"strings"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type reviewOptions struct {
	changedOnly bool
	all         bool
	artifact    string
}

type reviewView struct {
	Slug           string             `json:"slug"`
	ExecutionMode  string             `json:"execution_mode"`
	QualityMode    string             `json:"quality_mode,omitempty"`
	NeedsDiscovery bool               `json:"needs_discovery,omitempty"`
	CurrentState   string             `json:"current_state"`
	Verdict        string             `json:"verdict"`
	Blockers       []model.ReasonCode `json:"blockers,omitempty"`
	Gaps           *reviewGaps        `json:"gaps,omitempty"`
}

type reviewGaps struct {
	CodeToArtifact []string `json:"code_to_artifact,omitempty"`
	ArtifactToCode []string `json:"artifact_to_code,omitempty"`
}

func makeReviewCmd() *cobra.Command {
	opts := reviewOptions{}
	var changeSlug string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Bidirectional artifact-code alignment review",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("all") && cmd.Flags().Changed("changed-only") && opts.all && opts.changedOnly {
				return newInvalidUsageError(
					"mutually_exclusive_flags",
					"`--changed-only` and `--all` are mutually exclusive",
					"Use either `--changed-only` or `--all`, not both.",
					nil,
				)
			}
			reviewAll := opts.all || !opts.changedOnly

			if opts.artifact != "" {
				return newInvalidUsageError(
					"unsupported_flag",
					"`--artifact` is not supported in MVP; use default changed-only review or --all",
					"Remove `--artifact` flag and use `--all` or default changed-only review.",
					nil,
				)
			}

			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			active, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, active.Slug, "review", func() error {
				change, err := loadActiveChange(
					root,
					active.Slug,
					"review requires active change; current status=%s",
					"Only active changes can be reviewed.",
				)
				if err != nil {
					return err
				}

				execCtx, err := loadExecutionContext(root, change)
				if err != nil {
					return err
				}
				if err := ensureReviewEntryState(change.CurrentState, execCtx.Summary); err != nil {
					return err
				}
				if change.CurrentState == model.StateS2Execute {
					change.CurrentState = model.StateS3Review
				}

				execMode := governedExecutionMode

				verdict, blockers := evaluateReviewVerdict(execCtx)
				reviewState := model.StateS3Review
				readiness, evalErr := progression.EvaluateGovernanceReadiness(
					root,
					change,
					progression.GovernanceReadinessOptions{
						WorkflowStateOverride: &reviewState,
						// Review renders both artifact and review-specific context, so
						// it opts into only those optional readiness surfaces.
						IncludeArtifactProjection: true,
						IncludeReviewSurface:      true,
					},
				)
				if evalErr != nil {
					return wrapGovernanceReadinessError("evaluate review prerequisites", change.Slug, evalErr)
				}
				artifactReviewEvidence := model.VerificationRecord{}
				if readiness.ReviewSurface != nil {
					artifactReviewEvidence = readiness.ReviewSurface.PassingSkills[progression.SkillSpecComplianceReview]
				}
				blockers = append(blockers, readiness.Blockers...)
				if reviewAll {
					blockers = append(blockers, progression.EvaluateReviewLayerBlockers(change, artifactReviewEvidence, readiness.ArtifactProjection, true)...)
				}
				blockers = model.NormalizeReasonCodes(blockers)
				if len(blockers) > 0 {
					verdict = "fail"
				}

				if verdict == "fail" && hasIntentDriftSignal(blockers, artifactReviewEvidence) {
					change.ReviewIntentDriftFailures++
				} else {
					change.ReviewIntentDriftFailures = 0
				}
				if change.ReviewIntentDriftFailures >= 2 {
					blockers = appendReasonCodes(blockers, []model.ReasonCode{model.NewReasonCode("pivot_required", "intent_drift")})
					verdict = "fail"
				}

				if verdict == "fail" {
					change.CurrentState = model.StateS2Execute
				}
				if err := state.SaveChange(root, change); err != nil {
					return err
				}
				profile := buildChangeProfileView(change)
				view := reviewView{
					Slug:           active.Slug,
					ExecutionMode:  execMode,
					QualityMode:    profile.QualityMode,
					NeedsDiscovery: profile.NeedsDiscovery,
					CurrentState:   string(change.CurrentState),
					Verdict:        verdict,
					Blockers:       blockers,
					Gaps:           classifyReviewGaps(blockers),
				}

				return encodeJSONResponse(cmd, view)
			})
		},
	}

	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().BoolVar(&opts.changedOnly, "changed-only", true, "Review only changed/stale units")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Run full review")
	cmd.Flags().StringVar(&opts.artifact, "artifact", "", "Artifact path (unsupported in MVP)")
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}

func ensureReviewEntryState(current model.WorkflowState, summary *model.ExecutionSummary) error {
	summaryReady := state.ExecutionSummaryReady(summary)
	switch current {
	case model.StateS2Execute, model.StateS3Review, model.StateS4Verify:
		if !summaryReady {
			return newGovernanceBlockedError(
				"missing_run_summary",
				"review requires execution summary evidence; run wave-orchestration first",
				"Complete wave execution to produce execution-summary.yaml before review.",
				"",
				nil,
			)
		}
		return nil
	default:
		return newGovernanceBlockedError(
			"review_state_invalid",
			"review is allowed only in S2_EXECUTE/S3_REVIEW/S4_VERIFY",
			"Advance the change to S2_EXECUTE or later before reviewing.",
			"",
			nil,
		)
	}
}

func evaluateReviewVerdict(execCtx executionContext) (string, []model.ReasonCode) {
	if !execCtx.Ready {
		return "fail", []model.ReasonCode{model.NewReasonCode("missing_run_summary", "")}
	}
	summary := execCtx.Summary

	blockers := append([]model.ReasonCode(nil), execCtx.SummaryBlockers...)
	for _, task := range summary.Tasks {
		if task.Verdict != model.TaskVerdictPass {
			blockers = append(blockers, model.NewReasonCode("non_pass_task", task.TaskID))
		}
		if len(task.Blockers) > 0 {
			key, err := model.BuildTaskRunKey(task.TaskID, summary.RunSummaryVersion)
			if err != nil {
				blockers = append(blockers, model.NewReasonCode("task_blockers_invalid_key", task.TaskID))
				continue
			}
			blockers = append(blockers, model.NewReasonCode("task_blockers", key))
		}
	}
	if summary.OverallVerdict == model.ExecutionVerdictFail && len(blockers) == 0 {
		blockers = append(blockers, model.NewReasonCode("execution_verdict_fail", ""))
	}
	if len(blockers) > 0 {
		return "fail", model.NormalizeReasonCodes(blockers)
	}
	return "pass", nil
}

func hasIntentDriftSignal(blockers []model.ReasonCode, artifactReviewEvidence model.VerificationRecord) bool {
	for _, blocker := range blockers {
		normalizedCode := strings.ToLower(strings.TrimSpace(blocker.Code))
		normalizedDetail := strings.ToLower(strings.TrimSpace(blocker.Detail))
		if (normalizedCode == "pivot_required" && normalizedDetail == "intent_drift") ||
			normalizedCode == "intent_drift" {
			return true
		}
	}
	for _, ref := range artifactReviewEvidence.References {
		raw := strings.TrimSpace(strings.ToLower(ref))
		if raw == "intent_drift:true" || strings.HasPrefix(raw, "intent_drift:yes") {
			return true
		}
	}
	return false
}

// classifyReviewGaps splits blockers into bidirectional gap categories.
func classifyReviewGaps(blockers []model.ReasonCode) *reviewGaps {
	if len(blockers) == 0 {
		return nil
	}
	var codeToArt, artToCode []model.ReasonCode
	for _, b := range blockers {
		lower := strings.ToLower(b.Code)
		switch {
		case lower == "review_layer_missing" ||
			lower == "review_layer_failed" ||
			lower == "required_skill_missing":
			artToCode = append(artToCode, b)
		case lower == "non_pass_task" ||
			lower == "task_blockers":
			codeToArt = append(codeToArt, b)
		default:
			artToCode = append(artToCode, b)
		}
	}
	if len(codeToArt) == 0 && len(artToCode) == 0 {
		return nil
	}
	return &reviewGaps{
		CodeToArtifact: model.ReasonSpecs(codeToArt),
		ArtifactToCode: model.ReasonSpecs(artToCode),
	}
}
