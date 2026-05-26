package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type reviewOptions struct {
	changedOnly bool
	all         bool
	artifact    string
	focus       string
}

type reviewView struct {
	Slug               string                    `json:"slug"`
	ExecutionMode      string                    `json:"execution_mode"`
	QualityMode        string                    `json:"quality_mode,omitempty"`
	NeedsDiscovery     bool                      `json:"needs_discovery,omitempty"`
	CurrentState       string                    `json:"current_state"`
	Verdict            string                    `json:"verdict"`
	Mode               string                    `json:"mode,omitempty"`
	HydrateReferences  []string                  `json:"hydrate_references,omitempty"`
	ArtifactAmendments []artifact.AmendmentEvent `json:"artifact_amendments,omitempty"`
	Blockers           []model.ReasonCode        `json:"blockers,omitempty"`
	Waves              []reviewWaveView          `json:"waves,omitempty"`
	Gaps               *reviewGaps               `json:"gaps,omitempty"`
}

type reviewGaps struct {
	CodeToArtifact []string `json:"code_to_artifact,omitempty"`
	ArtifactToCode []string `json:"artifact_to_code,omitempty"`
}

type reviewWaveView struct {
	WaveIndex int      `json:"wave_index"`
	Verdict   string   `json:"verdict"`
	TaskRuns  []string `json:"task_runs,omitempty"`
}

func makeReviewCmd() *cobra.Command {
	opts := reviewOptions{}
	var changeSlug string
	var jsonOutput bool
	var hydrate bool
	var hydrateRefs []string
	var listFocuses bool
	var discoveryFormat string
	cmd := &cobra.Command{
		Use:   "review",
		Short: desc("review"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if listFocuses {
				return emitFocusDiscovery(cmd, "review", discoveryFormat)
			}
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
			if err := validateFocus("review", opts.focus); err != nil {
				return err
			}
			if len(hydrateRefs) > 0 && !hydrate {
				return newInvalidUsageError(
					"hydrate_ref_requires_hydrate",
					"`--hydrate-ref` requires `--hydrate`",
					"Add `--hydrate` to emit hydrate bodies, or remove `--hydrate-ref`.",
					map[string]any{"hydrate_refs": normalizeHydrateKeys(hydrateRefs)},
				)
			}
			if jsonOutput && hydrate {
				return newInvalidUsageError(
					"mutually_exclusive_flags",
					"`--hydrate` cannot be combined with `--json`",
					"Drop `--json` to emit hydrate bodies, or omit `--hydrate`.",
					nil,
				)
			}
			effectiveMode := resolveEffectiveFocus("review", opts.focus)
			hydrateKeys := normalizeHydrateKeys(resolveEffectiveFocusHydrate("review", opts.focus))
			if hydrate {
				var err error
				hydrateKeys, err = selectHydrateKeys(hydrateKeys, hydrateRefs)
				if err != nil {
					return err
				}
			}

			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}
			active, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, active.Slug, "review", func() error {
				view, err := buildReviewViewForSlug(root, active.Slug, reviewAll, effectiveMode, hydrateKeys)
				if err != nil {
					return err
				}

				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}
				if err := writeReviewText(cmd.OutOrStdout(), view); err != nil {
					return err
				}
				if hydrate {
					return emitHydrateBlocks(root, cmd.OutOrStdout(), view.HydrateReferences)
				}
				return nil
			})
		},
	}

	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	cmd.Flags().BoolVar(&opts.changedOnly, "changed-only", true, "Review only changed/stale units")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Run full review")
	cmd.Flags().StringVar(&opts.artifact, "artifact", "", "Artifact path (unsupported in MVP)")
	cmd.Flags().StringVar(&opts.focus, "focus", "", "Review focus (e.g. sast, calibration)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.Flags().BoolVar(&hydrate, "hydrate", false, "Append selected hydrate reference bodies (text output only)")
	cmd.Flags().StringArrayVar(&hydrateRefs, "hydrate-ref", nil, "Restrict `--hydrate` output to the selected `<skill-id>/<name>` reference (repeatable)")
	cmd.Flags().BoolVar(&listFocuses, "list-focuses", false, "List public --focus aliases for this command and exit")
	cmd.Flags().StringVar(&discoveryFormat, "format", "text", "Output format for --list-focuses: text|json")
	return cmd
}

func buildReviewViewForSlug(root, slug string, reviewAll bool, effectiveMode string, hydrateKeys []string) (reviewView, error) {
	change, err := loadActiveChange(
		root,
		slug,
		"review requires active change; current status=%s",
		"Only active changes can be reviewed.",
	)
	if err != nil {
		return reviewView{}, err
	}

	execCtx, err := loadExecutionContext(root, change)
	if err != nil {
		return reviewView{}, err
	}
	if err := ensureReviewEntryState(change.CurrentState, execCtx.Summary); err != nil {
		return reviewView{}, err
	}
	if change.CurrentState == model.StateS2Execute {
		change.CurrentState = model.StateS3Review
	}

	reviewState := model.StateS3Review
	readiness, err := progression.EvaluateGovernanceReadiness(
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
	if err != nil {
		return reviewView{}, wrapGovernanceReadinessError("evaluate review prerequisites", change.Slug, err)
	}
	artifactReviewEvidence := model.VerificationRecord{}
	if readiness.ReviewSurface != nil {
		artifactReviewEvidence = readiness.ReviewSurface.PassingSkills[progression.SkillSpecComplianceReview]
	}

	var waveViews []reviewWaveView
	var waveCtx *waveExecutionContext
	if execCtx.Ready {
		waveCtx, err = loadAuthoritativeWaveExecution(root, change, execCtx.LatestRunVersion, "review")
		if err != nil {
			return reviewView{}, err
		}
	}

	verdict, blockers, waveStatus := evaluateReviewVerdict(execCtx, waveCtx)
	waveViews = waveStatus
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
		return reviewView{}, err
	}

	profile := buildChangeProfileView(change)
	view := reviewView{
		Slug:              slug,
		ExecutionMode:     governedExecutionMode,
		QualityMode:       profile.QualityMode,
		NeedsDiscovery:    profile.NeedsDiscovery,
		CurrentState:      string(change.CurrentState),
		Verdict:           verdict,
		Mode:              effectiveMode,
		HydrateReferences: hydrateKeys,
		Blockers:          blockers,
		Waves:             waveViews,
		Gaps:              classifyReviewGaps(blockers),
	}
	if readiness.ArtifactProjection != nil && len(readiness.ArtifactProjection.Amendments) > 0 {
		view.ArtifactAmendments = append([]artifact.AmendmentEvent(nil), readiness.ArtifactProjection.Amendments...)
	}
	return view, nil
}

func writeReviewText(w io.Writer, view reviewView) error {
	writer := newFormatWriter(w)
	writer.Writef("Change: %s\n", view.Slug)
	writer.Writef("State: %s\n", view.CurrentState)
	writer.Writef("Verdict: %s\n", view.Verdict)
	if strings.TrimSpace(view.Mode) != "" {
		writer.Writef("Mode: %s\n", view.Mode)
	}
	writeHydrateLine(writer, "", view.HydrateReferences)

	if len(view.Waves) > 0 {
		writer.Writef("Waves:\n")
		for _, wave := range view.Waves {
			line := fmt.Sprintf("  - wave %d: %s", wave.WaveIndex, wave.Verdict)
			if len(wave.TaskRuns) > 0 {
				line += " [" + strings.Join(wave.TaskRuns, ", ") + "]"
			}
			writer.Writef("%s\n", line)
		}
	}

	if len(view.Blockers) > 0 {
		writer.Writef("Blockers:\n")
		for _, blocker := range model.ReasonSpecs(view.Blockers) {
			writer.Writef("  - %s\n", blocker)
		}
	}

	if view.Gaps != nil {
		if len(view.Gaps.CodeToArtifact) > 0 {
			writer.Writef("Code->Artifact Gaps:\n")
			for _, gap := range view.Gaps.CodeToArtifact {
				writer.Writef("  - %s\n", gap)
			}
		}
		if len(view.Gaps.ArtifactToCode) > 0 {
			writer.Writef("Artifact->Code Gaps:\n")
			for _, gap := range view.Gaps.ArtifactToCode {
				writer.Writef("  - %s\n", gap)
			}
		}
	}

	return writer.Err()
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

func evaluateReviewVerdict(execCtx executionContext, waveCtx *waveExecutionContext) (string, []model.ReasonCode, []reviewWaveView) {
	if !execCtx.Ready {
		return "fail", []model.ReasonCode{model.NewReasonCode("missing_run_summary", "")}, nil
	}
	summary := execCtx.Summary

	blockers := append([]model.ReasonCode(nil), execCtx.SummaryBlockers...)
	waveViews := []reviewWaveView{}
	if waveCtx != nil {
		runByWave := make(map[int]model.WaveRun, len(waveCtx.Runs))
		for _, run := range waveCtx.Runs {
			runByWave[run.WaveIndex] = run
		}
		for _, plannedWave := range waveCtx.Plan.Waves {
			run, ok := runByWave[plannedWave.WaveIndex]
			if !ok {
				blockers = append(blockers, model.NewReasonCode("wave_run_missing", plannedWaveLabel(plannedWave.WaveIndex)))
				waveViews = append(waveViews, reviewWaveView{
					WaveIndex: plannedWave.WaveIndex,
					Verdict:   string(model.WaveVerdictPending),
				})
				continue
			}
			taskRuns := make([]string, 0, len(run.TaskRuns))
			for _, ref := range run.TaskRuns {
				taskRuns = append(taskRuns, ref.TaskID)
			}
			waveViews = append(waveViews, reviewWaveView{
				WaveIndex: plannedWave.WaveIndex,
				Verdict:   string(run.Verdict),
				TaskRuns:  taskRuns,
			})
			if run.Verdict != model.WaveVerdictPass {
				blockers = append(blockers, model.NewReasonCode("non_pass_wave", plannedWaveDetail(run.WaveIndex, run.Verdict)))
			}
		}
	}
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
		return "fail", model.NormalizeReasonCodes(blockers), waveViews
	}
	return "pass", nil, waveViews
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
			lower == "task_blockers" ||
			lower == "non_pass_wave" ||
			lower == "wave_run_missing":
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

func plannedWaveLabel(index int) string {
	return fmt.Sprintf("wave-%02d", index)
}

func plannedWaveDetail(index int, verdict model.WaveVerdict) string {
	return plannedWaveLabel(index) + ":" + string(verdict)
}
