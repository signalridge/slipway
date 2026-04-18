package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type nextView struct {
	Slug                      string                       `json:"slug"`
	QualityMode               string                       `json:"quality_mode,omitempty"`
	WorkflowPreset            string                       `json:"workflow_preset,omitempty"`
	SuggestedWorkflowPreset   string                       `json:"suggested_workflow_preset,omitempty"`
	EffectiveWorkflowPreset   string                       `json:"effective_workflow_preset,omitempty"`
	PresetConfirmationPending bool                         `json:"preset_confirmation_pending,omitempty"`
	PresetUpgradeReasons      []string                     `json:"preset_upgrade_reasons,omitempty"`
	GovernanceForecast        *governanceForecastView      `json:"governance_forecast,omitempty"`
	NeedsDiscovery            bool                         `json:"needs_discovery,omitempty"`
	ComplexityLevel           string                       `json:"complexity_level,omitempty"`
	GuardrailDomain           string                       `json:"guardrail_domain,omitempty"`
	Phase                     model.UserPhase              `json:"phase"`
	ExecutionMode             string                       `json:"execution_mode"`
	CurrentState              model.WorkflowState          `json:"current_state"`
	IntakeSubStep             model.IntakeSubStep          `json:"intake_substep,omitempty"`
	PlanSubStep               model.PlanSubStep            `json:"plan_substep,omitempty"`
	PlanningNote              string                       `json:"planning_note,omitempty"`
	LifecycleStatus           string                       `json:"lifecycle_status"`
	Advanced                  *progression.AdvanceSummary  `json:"advanced,omitempty"`
	AutoTransitions           []progression.AdvanceSummary `json:"auto_transitions,omitempty"`
	NextSkill                 *nextSkillView               `json:"next_skill"`
	InputContext              nextContext                  `json:"input_context"`
	ContextBudget             *contextBudget               `json:"context_budget,omitempty"`
	Constraints               *agentConstraints            `json:"constraints,omitempty"`
	GovernanceSignals         *governanceSignalView        `json:"governance_signals,omitempty"`
	ActiveControls            []governanceControlView      `json:"active_controls,omitempty"`
	RequiredActions           []governanceActionView       `json:"required_actions,omitempty"`
	SkillEvidence             []skillEvidenceEntry         `json:"skill_evidence,omitempty"`
	AutoPassEligible          []model.AutoPassedState      `json:"auto_pass_eligible,omitempty"`
	ArtifactAmendments        []artifact.AmendmentEvent    `json:"artifact_amendments,omitempty"`
	Warnings                  []string                     `json:"warnings,omitempty"`
	Blockers                  []model.ReasonCode           `json:"blockers"`
	Confirmation              bool                         `json:"confirmation_required"`

	consumeActiveCheckpoint bool
}

type skillEvidenceEntry struct {
	SkillName   string `json:"skill_name"`
	HasEvidence bool   `json:"has_evidence"`
	Status      string `json:"status,omitempty"`
	Verdict     string `json:"verdict,omitempty"`
}

type agentConstraints struct {
	AllowedOperations []string `json:"allowed_operations"`
	RequiredOutputs   []string `json:"required_outputs"`
	MaxRetries        int      `json:"max_retries"`
	HardGate          string   `json:"hard_gate,omitempty"`
}

type contextBudget struct {
	EstimatedTokens      int                     `json:"estimated_tokens"`
	AssumedContextWindow int                     `json:"assumed_context_window_tokens"`
	UtilizationPercent   float64                 `json:"utilization_percent"`
	RemainingPercent     float64                 `json:"remaining_percent"`
	Health               string                  `json:"health"`
	QualityCurve         string                  `json:"quality_curve"`
	GuardAction          string                  `json:"guard_action"`
	Thresholds           contextBudgetThresholds `json:"thresholds"`
	Breakdown            contextBudgetBreakdown  `json:"breakdown"`
}

type contextBudgetThresholds struct {
	WarnBelowRemainingPercent float64 `json:"warn_below_remaining_percent"`
	StopBelowRemainingPercent float64 `json:"stop_below_remaining_percent"`
}

type contextBudgetBreakdown struct {
	SkillPrompt int `json:"skill_prompt"`

	ArtifactContext int `json:"artifact_context"`
	StateContext    int `json:"state_context"`
}

type nextSkillView struct {
	Name                string             `json:"name"`
	PromptPath          string             `json:"prompt_path"`
	VerificationDir     string             `json:"verification_dir"`
	State               string             `json:"state"`
	AgentHint           string             `json:"agent_hint,omitempty"`
	AgentDefinitionPath string             `json:"agent_definition_path,omitempty"`
	SkillConstraints    *skillConstraints  `json:"skill_constraints,omitempty"`
	ReviewContext       *reviewContextView `json:"review_context,omitempty"`
	TechniqueHints      []techniqueHint    `json:"technique_hints,omitempty"`
}

// skillConstraints carries per-skill metadata from the Go registry
// and state context, replacing level-conditional logic in skill templates.
type skillConstraints struct {
	LockedDecisions  []string `json:"locked_decisions,omitempty"`
	GuardrailDomain  string   `json:"guardrail_domain,omitempty"`
	MitigationTarget string   `json:"mitigation_target,omitempty"`
	RunSummaryBound  bool     `json:"run_summary_bound,omitempty"`
}

type techniqueHint struct {
	Name              string   `json:"name"`
	Reason            string   `json:"reason"`
	HydrateReferences []string `json:"hydrate_references,omitempty"`
}

type reviewContextView struct {
	RequiredArtifactLayers       []string `json:"required_artifact_layers"`
	RequiredImplementationLayers []string `json:"required_implementation_layers"`
	OptionalLayers               []string `json:"optional_layers"`
}

type nextContext struct {
	Description            string                     `json:"description,omitempty"`
	WorkspaceRoot          string                     `json:"workspace_root"`
	Slug                   string                     `json:"slug,omitempty"`
	ArtifactBundle         string                     `json:"artifact_bundle,omitempty"`
	CodebaseMapDir         string                     `json:"codebase_map_dir,omitempty"`
	CodebaseMapDocs        map[string]string          `json:"codebase_map_docs,omitempty"`
	ContextDependencies    *model.ContextDependencies `json:"context_dependencies,omitempty"`
	SelectedPriorContext   []selectedPriorContextView `json:"selected_prior_context,omitempty"`
	UnresolvedDependencies []unresolvedDependencyView `json:"unresolved_dependencies,omitempty"`
	ResumeCheckpoint       *resumeCheckpoint          `json:"resume_checkpoint,omitempty"`
	GateStatus             map[string]string          `json:"gate_status,omitempty"`
	ArtifactStatus         map[string]string          `json:"artifact_status,omitempty"`
	WavePlan               *wavePlanView              `json:"wave_plan,omitempty"`
}

type wavePlanView struct {
	TotalTasks int        `json:"total_tasks"`
	WaveCount  int        `json:"wave_count"`
	Waves      []waveView `json:"waves"`
	ParseError string     `json:"parse_error,omitempty"`
}

type waveView struct {
	WaveIndex int            `json:"wave_index"`
	Tasks     []waveTaskView `json:"tasks"`
}

type waveTaskView struct {
	TaskID      string   `json:"task_id"`
	Objective   string   `json:"objective,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
	TargetFiles []string `json:"target_files,omitempty"`
	TaskKind    string   `json:"task_kind"`
}

type resumeCheckpoint struct {
	RunSummaryVersion   int      `json:"run_summary_version"`
	CompletedTaskIDs    []string `json:"completed_task_ids,omitempty"`
	Freshness           string   `json:"freshness,omitempty"`
	ResumeWaveIndex     int      `json:"resume_wave_index,omitempty"`
	PausedTaskID        string   `json:"paused_task_id,omitempty"`
	PausedWaveIndex     int      `json:"paused_wave_index,omitempty"`
	CheckpointType      string   `json:"checkpoint_type,omitempty"`
	UserResponsePayload string   `json:"user_response_payload,omitempty"`
}

func makeNextCmd() *cobra.Command {
	var jsonOutput bool
	var preview bool
	var contextGuard bool
	var noAutoPass bool
	var quickMode bool
	var hookLite bool
	var changeSlug string

	cmd := &cobra.Command{
		Use:   "next",
		Short: desc("next"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}

			// Validate flag conflicts before acquiring any locks.
			if err := validateNextFlags(preview, contextGuard, hookLite); err != nil {
				return err
			}

			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, ref.Slug, "next", func() error {
				if hookLite {
					view, err := buildLightweightNextView(root, ref)
					if err != nil {
						return err
					}
					return encodeJSONResponse(cmd, view)
				}

				view, err := buildNextView(root, ref, "", preview, !jsonOutput, noAutoPass, quickMode)
				if err != nil {
					return err
				}

				if contextGuard {
					return writeContextGuardHookMessages(cmd.OutOrStdout(), view)
				}

				if jsonOutput {
					return encodeJSONResponse(cmd, view)
				}
				return writeNextHuman(cmd.OutOrStdout(), view)
			})
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.Flags().BoolVar(&preview, "preview", false, "Show next skill context without state advancement")
	cmd.Flags().BoolVar(&contextGuard, "context-guard", false, "Output context budget guard messages in hook format (requires --preview)")
	cmd.Flags().BoolVar(&noAutoPass, "no-auto-pass", false, "Skip auto-pass and report eligibility instead")
	cmd.Flags().BoolVar(&quickMode, "quick", false, "Disable advisory controls (clarification, research, independent_review, worktree_isolation)")
	cmd.Flags().BoolVar(&hookLite, "hook-lite", false, "")
	_ = cmd.Flags().MarkHidden("hook-lite")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	return cmd
}

func validateNextFlags(preview bool, contextGuard bool, hookLite bool) error {
	if contextGuard && !preview {
		return newCLIError(
			categoryInvalidUsage,
			"flag_conflict",
			"--context-guard requires --preview",
			"Use --context-guard with --preview.",
			"",
			nil,
		)
	}
	if hookLite && !preview {
		return newCLIError(
			categoryInvalidUsage,
			"flag_conflict",
			"--hook-lite requires --preview",
			"Use --hook-lite with --preview.",
			"",
			nil,
		)
	}

	return nil
}

func buildNextView(root string, ref changeRef, resumeResponse string, preview bool, autoSkipEvidence bool, skipAutoPass bool, quickMode ...bool) (nextView, error) {
	quick := len(quickMode) > 0 && quickMode[0]
	advanced, err := advanceIfReady(root, ref, preview, skipAutoPass, quick)
	if err != nil {
		return nextView{}, err
	}

	view := nextView{
		Slug:         ref.Slug,
		Phase:        model.PhasePlanning,
		Confirmation: true,
		InputContext: nextContext{
			WorkspaceRoot: root,
		},
	}
	if shouldExposeAdvancedSummaryToCaller(advanced) {
		view.Advanced = &advanced
	}

	// Preset confirmation gate: check BEFORE buildNextContextByMode to prevent
	// artifact reconciliation side effects (ReconcileFromFilesystem, SaveChange)
	// and artifact_status leakage when the preset is still pending.
	if pending, err := checkPresetPendingEarlyReturn(root, ref, &view); err != nil {
		return nextView{}, err
	} else if pending {
		return view, nil
	}

	governedChange, execCtx, err := buildNextContextByMode(root, &view, ref, resumeResponse, preview)
	if err != nil {
		return nextView{}, err
	}
	finalize := func() (nextView, error) {
		if err := consumeNextCheckpoint(root, governedChange, &view); err != nil {
			return nextView{}, err
		}
		return view, nil
	}
	view.Phase = model.PhaseFor(view.CurrentState)
	var nextSkillEvidence map[string]model.VerificationRecord
	if governedChange != nil {
		presetFields, err := buildWorkflowPresetView(root, *governedChange)
		if err != nil {
			return nextView{}, err
		}
		view.WorkflowPreset = presetFields.WorkflowPreset
		view.SuggestedWorkflowPreset = presetFields.SuggestedWorkflowPreset
		view.EffectiveWorkflowPreset = presetFields.EffectiveWorkflowPreset
		view.PresetConfirmationPending = presetFields.PresetConfirmationPending
		view.PresetUpgradeReasons = presetFields.PresetUpgradeReasons
		view.GovernanceForecast = presetFields.GovernanceForecast

		readiness, err := progression.EvaluateGovernanceReadiness(
			root,
			*governedChange,
			progression.GovernanceReadinessOptions{
				IncludeGateEvaluations: true,
				// Next needs projected artifact context for rendering, but keeps the
				// reconcile read-only by requesting only the in-memory projection.
				IncludeArtifactProjection: true,
			},
		)
		if err != nil {
			return nextView{}, wrapGovernanceReadinessError("evaluate next skill evidence", ref.Slug, err)
		}
		view.Warnings = append(view.Warnings, readiness.Diagnostics...)
		view.Blockers = appendReasonCodes(view.Blockers, readiness.Blockers)
		if readiness.ArtifactProjection != nil && len(readiness.ArtifactProjection.Amendments) > 0 {
			view.ArtifactAmendments = append([]artifact.AmendmentEvent(nil), readiness.ArtifactProjection.Amendments...)
		}
		nextSkillEvidence = readiness.PassingSkills
		applyReadinessToNextContext(&view, readiness)
		applyGovernanceSurfaceToNext(readiness, &view)
	}

	// Attach wave plan when at S2_EXECUTE for governed changes.
	if view.CurrentState == model.StateS2Execute && governedChange != nil {
		view.InputContext.WavePlan = buildWavePlan(root, governedChange, view.InputContext.ArtifactBundle)
	}

	if view.CurrentState == model.StateDone {
		view.NextSkill = nil
		view.Blockers = []model.ReasonCode{model.NewReasonCode("change_is_done", "")}
		return finalize()
	}
	if advanced.Action == "done_ready" {
		view.NextSkill = nil
		view.Blockers = appendReasonCodes(view.Blockers, advanced.Blockers)
		warnings, err := advisoryDoneReadyWarnings(root, ref, governedChange, execCtx, view)
		if err != nil {
			return nextView{}, err
		}
		view.Warnings = append(view.Warnings, warnings...)
		return finalize()
	}

	if skipAutoPass && governedChange != nil {
		eligible, eligErr := progression.AutoPassEligibility(root, *governedChange)
		if eligErr == nil && len(eligible) > 0 {
			view.AutoPassEligible = eligible
		}
	}

	if err := assembleSkillView(root, &view, ref, advanced, governedChange, execCtx, nextSkillEvidence, autoSkipEvidence); err != nil {
		return nextView{}, err
	}

	return finalize()
}

func consumeNextCheckpoint(root string, change *model.Change, view *nextView) error {
	if view == nil || !view.consumeActiveCheckpoint {
		return nil
	}
	if change == nil || change.ActiveCheckpoint == nil {
		view.consumeActiveCheckpoint = false
		return nil
	}

	change.ActiveCheckpoint = nil
	if err := state.SaveChange(root, *change); err != nil {
		return err
	}
	view.consumeActiveCheckpoint = false
	return nil
}

// advanceIfReady attempts state advancement unless in preview mode.
// When skipAutoPass is true, advancement proceeds but auto-pass is
// suppressed so the caller can decide whether to accept auto-pass.
func advanceIfReady(root string, ref changeRef, preview bool, skipAutoPass bool, quickMode bool) (progression.AdvanceSummary, error) {
	if preview {
		return progression.AdvanceSummary{Action: "preview"}, nil
	}

	var opts []progression.AdvanceOptions
	if skipAutoPass || quickMode {
		opts = append(opts, progression.AdvanceOptions{
			SkipAutoPass: skipAutoPass,
			QuickMode:    quickMode,
		})
	}
	advanced, err := tryAdvance(root, ref, opts...)
	if err != nil {
		return progression.AdvanceSummary{}, err
	}
	return advanced, nil
}

func shouldExposeAdvancedSummaryToCaller(summary progression.AdvanceSummary) bool {
	switch summary.Action {
	case "advanced", "done_ready":
		return true
	case "blocked":
		return true
	default:
		return false
	}
}

func advisoryDoneReadyWarnings(root string, ref changeRef, governedChange *model.Change, execCtx *executionContext, view nextView) ([]string, error) {
	if governedChange == nil || view.CurrentState != model.StateS4Verify {
		return nil, nil
	}

	change := *governedChange
	latestRunVersion := 0
	if execCtx != nil {
		latestRunVersion = execCtx.LatestRunVersion
	} else {
		var err error
		latestRunVersion, err = state.LatestRelevantExecutionRunVersion(root, change)
		if err != nil {
			return nil, err
		}
	}
	passingSkills, _, err := progression.EvaluateRequiredSkillsForChange(
		root,
		change,
		change.CurrentState,
		latestRunVersion,
		true,
	)
	if err != nil {
		return nil, wrapRequiredSkillsEvaluationError("evaluate advisory closeout evidence", ref.Slug, err)
	}
	if _, ok := passingSkills[progression.SkillFinalCloseout]; ok {
		return nil, nil
	}
	presetPolicy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return nil, err
	}
	if presetPolicy.CloseoutRefreshRequired {
		return []string{
			"ship_gate_blocked:required_skill_missing:final-closeout",
		}, nil
	}
	return []string{
		"optional_closeout_available: final-closeout evidence is missing or stale; run final-closeout before `slipway done` only if refreshed closeout evidence is desired",
	}, nil
}

// checkPresetPendingEarlyReturn loads the change minimally and, if preset
// confirmation is pending, populates the view with identity and preset fields
// only — no artifact reconciliation, no SaveChange, no artifact_status.
// Returns (true, nil) when the early return was taken.
func checkPresetPendingEarlyReturn(root string, ref changeRef, view *nextView) (bool, error) {
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return false, err
	}
	if !change.WorkflowPresetConfirmationPending() {
		return false, nil
	}

	presetFields, err := buildWorkflowPresetView(root, change)
	if err != nil {
		return false, err
	}

	profile := buildChangeProfileView(change)
	view.CurrentState = change.CurrentState
	view.IntakeSubStep = change.IntakeSubStep
	view.PlanSubStep = change.PlanSubStep
	view.PlanningNote = planningNote(change.CurrentState, change.PlanSubStep)
	view.LifecycleStatus = string(change.Status)
	view.ExecutionMode = governedExecutionMode
	view.Phase = model.PhaseFor(change.CurrentState)
	view.QualityMode = profile.QualityMode
	view.NeedsDiscovery = profile.NeedsDiscovery
	view.ComplexityLevel = profile.ComplexityLevel
	view.GuardrailDomain = profile.GuardrailDomain
	view.InputContext.Description = change.Description
	view.InputContext.Slug = change.Slug
	view.WorkflowPreset = presetFields.WorkflowPreset
	view.SuggestedWorkflowPreset = presetFields.SuggestedWorkflowPreset
	view.EffectiveWorkflowPreset = presetFields.EffectiveWorkflowPreset
	view.PresetConfirmationPending = presetFields.PresetConfirmationPending
	view.PresetUpgradeReasons = presetFields.PresetUpgradeReasons
	view.GovernanceForecast = presetFields.GovernanceForecast
	view.Blockers = []model.ReasonCode{model.NewReasonCode("preset_confirmation_required", "")}
	view.NextSkill = nil
	return true, nil
}

// buildLightweightNextView preserves the ordinary preview semantics for hook
// callers, then strips heavyweight output sections that session-start does not
// need.
func buildLightweightNextView(root string, ref changeRef) (nextView, error) {
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return nextView{}, err
	}

	view, err := buildNextView(root, ref, "", true, false, false)
	if err != nil {
		return nextView{}, err
	}

	view.InputContext = nextContext{
		WorkspaceRoot: root,
		Description:   change.Description,
		Slug:          change.Slug,
	}
	view.ContextBudget = nil
	view.Constraints = nil
	view.GovernanceSignals = nil
	view.ActiveControls = nil
	view.RequiredActions = nil
	view.SkillEvidence = nil
	view.AutoPassEligible = nil
	view.ArtifactAmendments = nil
	view.AutoTransitions = nil
	if view.NextSkill != nil {
		view.NextSkill.SkillConstraints = nil
		view.NextSkill.ReviewContext = nil
		view.NextSkill.TechniqueHints = nil
	}
	return view, nil
}

func writeNextHuman(w io.Writer, view nextView) error {
	var writeErr error
	writeLine := func(format string, args ...any) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintf(w, format, args...)
	}

	writeLine("Change: %s (%s)\n", view.Slug, workflowStateLabel(view.CurrentState, view.IntakeSubStep, view.PlanSubStep))
	writeLine("Phase: %s | Mode: %s | Status: %s\n", view.Phase, view.ExecutionMode, view.LifecycleStatus)
	if view.QualityMode != "" {
		writeLine("Quality: %s | Discovery Required: %t\n", view.QualityMode, view.NeedsDiscovery)
	}
	for _, line := range renderWorkflowPresetLines(workflowPresetView{
		WorkflowPreset:            view.WorkflowPreset,
		SuggestedWorkflowPreset:   view.SuggestedWorkflowPreset,
		EffectiveWorkflowPreset:   view.EffectiveWorkflowPreset,
		PresetConfirmationPending: view.PresetConfirmationPending,
		PresetUpgradeReasons:      view.PresetUpgradeReasons,
		GovernanceForecast:        view.GovernanceForecast,
	}) {
		writeLine("%s\n", line)
	}
	if view.PlanningNote != "" {
		writeLine("Planning Note: %s\n", view.PlanningNote)
	}

	if view.InputContext.Description != "" {
		writeLine("Description: %s\n", view.InputContext.Description)
	}
	if view.InputContext.Slug != "" {
		writeLine("Slug: %s\n", view.InputContext.Slug)
	}
	if view.Advanced != nil {
		if view.Advanced.Action == "advanced" || view.Advanced.Action == "done_ready" {
			writeLine("\nAdvanced: %s -> %s (%s)\n", view.Advanced.FromState, view.Advanced.ToState, view.Advanced.Message)
		}
		if len(view.Advanced.AutoPassedStates) > 0 {
			writeLine("Auto-Passed:\n")
			for _, state := range view.Advanced.AutoPassedStates {
				writeLine("  - %s (%s)\n", state.State, state.Reason)
			}
		}
	}

	if view.GovernanceSignals != nil {
		writeLine("\nDetected Signals:\n")
		if len(view.GovernanceSignals.Domains) > 0 {
			writeLine("  Domains:       [%s]\n", strings.Join(view.GovernanceSignals.Domains, ", "))
		}
		writeLine("  Blast Radius:  %s\n", view.GovernanceSignals.BlastRadius)
	}
	if len(view.ActiveControls) > 0 {
		writeLine("\nActive Controls:\n")
		for _, ctrl := range view.ActiveControls {
			writeLine("  - %s (%s / %s)\n", ctrl.ControlID, ctrl.Mode, ctrl.Scope)
		}
	}
	if len(view.RequiredActions) > 0 {
		writeLine("\nRequired Actions:\n")
		for _, action := range view.RequiredActions {
			mark := " "
			if action.Satisfied {
				mark = "x"
			}
			writeLine("  [%s] %s: %s\n", mark, action.ControlID, action.Description)
		}
	}

	writeLine("\n")

	if view.NextSkill != nil {
		hydrateWriter := newFormatWriter(w)
		writeLine("Next Skill: %s\n", view.NextSkill.Name)
		writeLine("  Prompt: %s\n", view.NextSkill.PromptPath)
		writeLine("  Verification Dir: %s\n", view.NextSkill.VerificationDir)
		writeLine("  Evidence State: %s\n", view.NextSkill.State)

		if len(view.NextSkill.TechniqueHints) > 0 {
			writeLine("\nTechnique Hints:\n")
			for _, hint := range view.NextSkill.TechniqueHints {
				writeLine("  - %s: %s\n", hint.Name, hint.Reason)
				writeHydrateLine(hydrateWriter, "    ", hint.HydrateReferences)
			}
		}

		if len(view.Warnings) > 0 {
			writeLine("\nWarnings:\n")
			for _, warning := range view.Warnings {
				writeLine("  - %s\n", warning)
			}
		}

		if len(view.Blockers) > 0 {
			writeLine("\nBlockers:\n")
			for _, b := range renderReasonCodeLines(view.Blockers) {
				writeLine("  - %s\n", b)
			}
		}
	} else {
		if len(view.Warnings) > 0 {
			for _, warning := range view.Warnings {
				writeLine("  %s\n", warning)
			}
		}
		if len(view.Blockers) > 0 {
			for _, b := range renderReasonCodeLines(view.Blockers) {
				writeLine("  %s\n", b)
			}
		}
	}

	return writeErr
}
