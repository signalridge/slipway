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
	Slug                      string                               `json:"slug"`
	QualityMode               string                               `json:"quality_mode,omitempty"`
	WorkflowProfile           string                               `json:"workflow_profile,omitempty"`
	WorkflowPreset            string                               `json:"workflow_preset,omitempty"`
	SuggestedWorkflowPreset   string                               `json:"suggested_workflow_preset,omitempty"`
	EffectiveWorkflowPreset   string                               `json:"effective_workflow_preset,omitempty"`
	PresetConfirmationPending bool                                 `json:"preset_confirmation_pending,omitempty"`
	PresetUpgradeReasons      []string                             `json:"preset_upgrade_reasons,omitempty"`
	GovernanceForecast        *governanceForecastView              `json:"governance_forecast,omitempty"`
	NeedsDiscovery            bool                                 `json:"needs_discovery,omitempty"`
	ComplexityLevel           string                               `json:"complexity_level,omitempty"`
	GuardrailDomain           string                               `json:"guardrail_domain,omitempty"`
	Phase                     model.UserPhase                      `json:"phase"`
	ExecutionMode             string                               `json:"execution_mode"`
	CurrentState              model.WorkflowState                  `json:"current_state"`
	IntakeSubStep             model.IntakeSubStep                  `json:"intake_substep,omitempty"`
	PlanSubStep               model.PlanSubStep                    `json:"plan_substep,omitempty"`
	PlanningNote              string                               `json:"planning_note,omitempty"`
	LifecycleStatus           string                               `json:"lifecycle_status"`
	Advanced                  *progression.AdvanceSummary          `json:"advanced,omitempty"`
	AutoTransitions           []progression.AdvanceSummary         `json:"auto_transitions,omitempty"`
	NextSkill                 *nextSkillView                       `json:"next_skill"`
	InputContext              nextContext                          `json:"input_context"`
	ContextBudget             *contextBudget                       `json:"context_budget,omitempty"`
	Constraints               *agentConstraints                    `json:"constraints,omitempty"`
	GovernanceSignals         *governanceSignalView                `json:"governance_signals,omitempty"`
	ActiveControls            []governanceControlView              `json:"active_controls,omitempty"`
	RequiredActions           []governanceActionView               `json:"required_actions,omitempty"`
	SkillEvidence             []skillEvidenceEntry                 `json:"skill_evidence,omitempty"`
	AutoPassEligible          []model.AutoPassedState              `json:"auto_pass_eligible,omitempty"`
	ArtifactAmendments        []artifact.AmendmentEvent            `json:"artifact_amendments,omitempty"`
	FreshnessDiagnostics      *state.ExecutionFreshnessDiagnostics `json:"freshness_diagnostics,omitempty"`
	Warnings                  []string                             `json:"warnings,omitempty"`
	Blockers                  []model.ReasonCode                   `json:"blockers"`
	Recovery                  *model.RecoverySummary               `json:"recovery,omitempty"`
	ConfirmationRequirement   confirmationRequirement              `json:"confirmation_requirement"`

	consumeActiveCheckpoint bool
}

type confirmationRequirement struct {
	Required                     bool   `json:"required"`
	Boundary                     string `json:"boundary"`
	FreshConfirmationRequired    bool   `json:"fresh_confirmation_required"`
	PriorAuthorizationSufficient bool   `json:"prior_authorization_sufficient"`
	Reason                       string `json:"reason"`
	ResumeResponseSupported      bool   `json:"resume_response_supported"`
	NextAction                   string `json:"next_action,omitempty"`
	NextActionKind               string `json:"next_action_kind,omitempty"`
	NextCommand                  string `json:"next_command,omitempty"`
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
	Name             string             `json:"name"`
	DisplayName      string             `json:"display_name,omitempty"`
	BlockingName     string             `json:"blocking_name,omitempty"`
	ResolutionReason string             `json:"resolution_reason,omitempty"`
	RequiredTokens   []string           `json:"required_tokens,omitempty"`
	VerificationDir  string             `json:"verification_dir"`
	State            string             `json:"state"`
	SkillConstraints *skillConstraints  `json:"skill_constraints,omitempty"`
	ReviewContext    *reviewContextView `json:"review_context,omitempty"`
	TechniqueHints   []techniqueHint    `json:"technique_hints,omitempty"`
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
	Kind              string   `json:"kind,omitempty"`
	Capability        string   `json:"capability,omitempty"`
	Language          string   `json:"language,omitempty"`
	Optional          bool     `json:"optional,omitempty"`
	Reason            string   `json:"reason"`
	HydrateReferences []string `json:"hydrate_references,omitempty"`
}

type reviewContextView struct {
	RequiredArtifactLayers       []string `json:"required_artifact_layers,omitempty"`
	RequiredImplementationLayers []string `json:"required_implementation_layers,omitempty"`
	OptionalLayers               []string `json:"optional_layers,omitempty"`
}

type nextContext struct {
	Description            string                     `json:"description,omitempty"`
	WorkspaceRoot          string                     `json:"workspace_root"`
	Slug                   string                     `json:"slug,omitempty"`
	ArtifactBundle         string                     `json:"artifact_bundle,omitempty"`
	CodebaseMapDir         string                     `json:"codebase_map_dir,omitempty"`
	CodebaseMapDocs        map[string]string          `json:"codebase_map_docs,omitempty"`
	CodebaseMapStatus      string                     `json:"codebase_map_status,omitempty"`
	CodebaseMapDocStates   map[string]string          `json:"codebase_map_doc_states,omitempty"`
	ProjectContext         *model.ProjectContext      `json:"project_context,omitempty"`
	HandoffContext         *handoffContextView        `json:"handoff_context,omitempty"`
	ContextDependencies    *model.ContextDependencies `json:"context_dependencies,omitempty"`
	SelectedPriorContext   []selectedPriorContextView `json:"selected_prior_context,omitempty"`
	UnresolvedDependencies []unresolvedDependencyView `json:"unresolved_dependencies,omitempty"`
	ResumeCheckpoint       *resumeCheckpoint          `json:"resume_checkpoint,omitempty"`
	GateStatus             map[string]string          `json:"gate_status,omitempty"`
	ArtifactStatus         map[string]string          `json:"artifact_status,omitempty"`
	WavePlan               *wavePlanView              `json:"wave_plan,omitempty"`
}

type handoffContextView struct {
	WorkflowProfile   string                 `json:"workflow_profile"`
	ContextPolicy     string                 `json:"context_policy"`
	Trace             *handoffTraceView      `json:"trace,omitempty"`
	ContextBudget     *handoffBudgetHintView `json:"context_budget,omitempty"`
	ReadRefs          []handoffReadRef       `json:"read_refs,omitempty"`
	PolicyPacks       []handoffPolicyPack    `json:"policy_packs,omitempty"`
	Risk              *handoffRiskView       `json:"risk,omitempty"`
	ChangeAuthority   string                 `json:"change_authority"`
	LifecycleEventLog string                 `json:"lifecycle_event_log,omitempty"`
	ConfigPath        string                 `json:"config_path,omitempty"`
	RequiredReads     []string               `json:"required_reads,omitempty"`
}

type handoffTraceView struct {
	CorrelationID string `json:"correlation_id"`
	EventLog      string `json:"event_log,omitempty"`
}

type handoffBudgetHintView struct {
	Mode           string `json:"mode"`
	MaxInlineBytes int    `json:"max_inline_bytes"`
}

type handoffReadRef struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type handoffPolicyPack struct {
	Name                 string   `json:"name"`
	Path                 string   `json:"path"`
	Mode                 string   `json:"mode"`
	SchemaVersion        string   `json:"schema_version,omitempty"`
	AdvisoryRules        []string `json:"advisory_rules,omitempty"`
	ArtifactRequirements []string `json:"artifact_requirements,omitempty"`
	RecommendedReviewers []string `json:"recommended_reviewers,omitempty"`
	Terminology          []string `json:"terminology,omitempty"`
}

type handoffRiskView struct {
	GuardrailDomain string   `json:"guardrail_domain,omitempty"`
	Controls        []string `json:"controls,omitempty"`
	WorkflowProfile string   `json:"workflow_profile,omitempty"`
	Hints           []string `json:"hints,omitempty"`
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
	var contextGuard bool
	var noAutoPass bool
	var diagnostics bool
	var changeSlug string

	cmd := &cobra.Command{
		Use:   "next",
		Short: desc("next"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}

			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, ref.Slug, "next", func() error {
				if jsonOutput && !diagnostics && !contextGuard {
					view, err := buildNextHandoffSourceView(root, ref, "", true, false, noAutoPass)
					if err != nil {
						return err
					}
					applyNextInvocationWorkspacePath(cmd, root, &view)
					return encodeJSONResponse(cmd, buildNextHandoffView(view))
				}

				// next is always query-only; state advancement is owned by `run`.
				view, err := buildNextView(root, ref, "", true, !jsonOutput, noAutoPass)
				if err != nil {
					return err
				}
				applyNextInvocationWorkspacePath(cmd, root, &view)

				if contextGuard {
					return writeContextGuardHookMessages(cmd.OutOrStdout(), view)
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
	cmd.Flags().BoolVar(&contextGuard, "context-guard", false, "Output context budget guard messages in hook format")
	cmd.Flags().BoolVar(&noAutoPass, "no-auto-pass", false, "Skip auto-pass and report eligibility instead")
	cmd.Flags().BoolVar(&diagnostics, "diagnostics", false, "Include diagnostic governance/readiness details")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	return cmd
}

func buildNextView(root string, ref changeRef, resumeResponse string, preview bool, autoSkipEvidence bool, skipAutoPass bool) (nextView, error) {
	advanced, err := advanceIfReady(root, ref, preview, skipAutoPass)
	if err != nil {
		return nextView{}, err
	}

	view := nextView{
		Slug:                    ref.Slug,
		Phase:                   model.PhasePlanning,
		ConfirmationRequirement: confirmationNoBoundary("initializing"),
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
		view.Recovery = model.BuildRecovery(view.Blockers)
		return view, nil
	}

	governedChange, execCtx, err := buildNextContextByMode(root, &view, ref, resumeResponse, preview)
	if err != nil {
		return nextView{}, err
	}
	finalize := func() (nextView, error) {
		view.ConfirmationRequirement = deriveConfirmationRequirement(view)
		if err := consumeNextCheckpoint(root, governedChange, &view); err != nil {
			return nextView{}, err
		}
		view.Recovery = model.BuildRecovery(view.Blockers)
		return view, nil
	}
	view.Phase = model.PhaseFor(view.CurrentState)
	var nextSkillEvidence map[string]model.VerificationRecord
	var nextSkillArtifactProjection *progression.ArtifactProjection
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
		view.FreshnessDiagnostics = attachFreshnessDiagnostics(readiness.FreshnessDiagnostics)
		if readiness.ArtifactProjection != nil && len(readiness.ArtifactProjection.Amendments) > 0 {
			view.ArtifactAmendments = append([]artifact.AmendmentEvent(nil), readiness.ArtifactProjection.Amendments...)
		}
		nextSkillEvidence = readiness.PassingSkills
		nextSkillArtifactProjection = readiness.ArtifactProjection
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
	if err := projectDoneReadyForReadOnlyQuery(root, governedChange, &advanced); err != nil {
		return nextView{}, err
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

	if err := assembleSkillView(root, &view, ref, advanced, governedChange, execCtx, nextSkillEvidence, nextSkillArtifactProjection, autoSkipEvidence); err != nil {
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

	beforeChange := *change
	checkpoint := *change.ActiveCheckpoint
	change.ActiveCheckpoint = nil
	if err := state.SaveChange(root, *change); err != nil {
		return err
	}
	if err := appendCLILifecycleEvent(root, *change, state.LifecycleEvent{
		Command:       "run",
		EventType:     "checkpoint.resolved",
		Action:        "resolved",
		Reason:        checkpoint.CheckpointType,
		Result:        "response_recorded",
		BeforeState:   beforeChange.CurrentState,
		AfterState:    change.CurrentState,
		Diagnostics:   []string{"task_id=" + checkpoint.PausedTaskID},
		SideEffects:   []state.LifecycleSideEffect{{Kind: "active_checkpoint_cleared"}},
		ClearedFields: []string{"active_checkpoint"},
	}); err != nil {
		return err
	}
	view.consumeActiveCheckpoint = false
	return nil
}

// advanceIfReady attempts state advancement unless in preview mode.
// When skipAutoPass is true, advancement proceeds but auto-pass is
// suppressed so the caller can decide whether to accept auto-pass.
func advanceIfReady(root string, ref changeRef, preview bool, skipAutoPass bool) (progression.AdvanceSummary, error) {
	if preview {
		return progression.AdvanceSummary{Action: "query"}, nil
	}

	var opts []progression.AdvanceOptions
	if skipAutoPass {
		opts = append(opts, progression.AdvanceOptions{
			SkipAutoPass: skipAutoPass,
			Command:      "run",
		})
	} else {
		opts = append(opts, progression.AdvanceOptions{
			Command: "run",
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
	case "query", "advanced", "done_ready":
		return true
	case "blocked":
		return true
	default:
		return false
	}
}

func projectDoneReadyForReadOnlyQuery(root string, change *model.Change, advanced *progression.AdvanceSummary) error {
	if change == nil || advanced == nil || advanced.Action != "query" || change.CurrentState != model.StateS4Verify {
		return nil
	}
	shipAuthority, err := progression.EvaluateShipAuthority(root, *change)
	if err != nil {
		return wrapGovernanceReadinessError("evaluate done-ready projection", change.Slug, err)
	}
	if shipAuthority.Result.Status != model.GateStatusApproved {
		return nil
	}
	*advanced = progression.AdvanceSummary{
		Action:    "done_ready",
		FromState: model.StateS4Verify,
		Reason:    "governance_gates_passed",
		Message:   "Governance gates passed; run `slipway done` to finalize.",
		Blockers:  []model.ReasonCode{model.NewReasonCode("run_slipway_done_to_finalize", "")},
	}
	return nil
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
	view.ConfirmationRequirement = confirmationHardStop("preset_confirmation_required")
	return true, nil
}

func deriveConfirmationRequirement(view nextView) confirmationRequirement {
	switch {
	case view.PresetConfirmationPending || hasReasonCode(view.Blockers, "preset_confirmation_required"):
		return confirmationHardStop("preset_confirmation_required")
	case hasPendingRunCheckpoint(view.InputContext.ResumeCheckpoint):
		return confirmationHardStop("resume_checkpoint")
	case view.NextSkill != nil:
		reason := "skill_handoff"
		if strings.TrimSpace(view.NextSkill.BlockingName) != "" {
			reason = "skill_handoff:" + strings.TrimSpace(view.NextSkill.BlockingName)
		} else if strings.TrimSpace(view.NextSkill.Name) != "" {
			reason = "skill_handoff:" + strings.TrimSpace(view.NextSkill.Name)
		}
		return confirmationHardStop(reason)
	case hasReasonCode(view.Blockers, "run_slipway_done_to_finalize"):
		return confirmationCommandRequired("run_slipway_done_to_finalize")
	case hasReasonCode(view.Blockers, "run_slipway_run_to_advance"):
		return confirmationCommandRequired("run_slipway_run_to_advance")
	case len(view.Blockers) > 0:
		return confirmationCommandRequired("blocked_by_governance")
	case len(view.AutoPassEligible) > 0:
		return confirmationEvidenceContinuation("auto_pass_available")
	default:
		return confirmationNoBoundary("no_confirmation_boundary")
	}
}

func hasReasonCode(reasons []model.ReasonCode, code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func confirmationHardStop(reason string) confirmationRequirement {
	reason = strings.TrimSpace(reason)
	resumeResponseSupported := reason == "resume_checkpoint"
	kind, command := hardStopNextActionKind(reason, resumeResponseSupported)
	return confirmationRequirement{
		Required:                     true,
		Boundary:                     "hard_stop",
		FreshConfirmationRequired:    true,
		PriorAuthorizationSufficient: false,
		Reason:                       reason,
		ResumeResponseSupported:      resumeResponseSupported,
		NextAction:                   hardStopNextAction(reason, resumeResponseSupported),
		NextActionKind:               kind,
		NextCommand:                  command,
	}
}

func confirmationCommandRequired(reason string) confirmationRequirement {
	reason = strings.TrimSpace(reason)
	kind, command := commandBoundaryNextActionKind(reason)
	return confirmationRequirement{
		Required:                     false,
		Boundary:                     "command_required",
		FreshConfirmationRequired:    false,
		PriorAuthorizationSufficient: true,
		Reason:                       reason,
		NextAction:                   commandBoundaryNextAction(reason),
		NextActionKind:               kind,
		NextCommand:                  command,
	}
}

func confirmationEvidenceContinuation(reason string) confirmationRequirement {
	return confirmationRequirement{
		Required:                     false,
		Boundary:                     "evidence_continuation",
		FreshConfirmationRequired:    false,
		PriorAuthorizationSufficient: true,
		Reason:                       strings.TrimSpace(reason),
		NextAction:                   "continue with eligible evidence",
		NextActionKind:               "none",
	}
}

func confirmationNoBoundary(reason string) confirmationRequirement {
	return confirmationRequirement{
		Required:                     false,
		Boundary:                     "none",
		FreshConfirmationRequired:    false,
		PriorAuthorizationSufficient: true,
		Reason:                       strings.TrimSpace(reason),
		NextAction:                   "no action required",
		NextActionKind:               "none",
	}
}

func hardStopNextAction(reason string, resumeResponseSupported bool) string {
	if resumeResponseSupported {
		return "resume pending checkpoint with slipway run --resume-response"
	}
	if skillName, ok := strings.CutPrefix(reason, "skill_handoff:"); ok && strings.TrimSpace(skillName) != "" {
		return "complete governance skill handoff: " + strings.TrimSpace(skillName)
	}
	switch reason {
	case "preset_confirmation_required":
		return "confirm workflow preset before continuing"
	default:
		return "complete required confirmation before continuing"
	}
}

func commandBoundaryNextAction(reason string) string {
	switch reason {
	case "run_slipway_done_to_finalize":
		return "run slipway done to finalize"
	case "run_slipway_run_to_advance":
		return "run slipway run to advance"
	case "blocked_by_governance":
		return "resolve governance blockers before continuing"
	default:
		return "run the command indicated by blockers"
	}
}

// hardStopNextActionKind returns the machine-readable action kind and, when one
// applies, the exact command to run for a hard-stop boundary. Callers branch on
// next_action_kind rather than parsing the human-readable next_action prose.
// next_command is populated only when it is runnable as-is; checkpoint resume
// requires an operator-supplied response argument, so it leaves next_command
// empty and is signaled by resume_response_supported instead.
func hardStopNextActionKind(reason string, resumeResponseSupported bool) (kind, command string) {
	switch {
	case resumeResponseSupported:
		return "checkpoint_resume", ""
	case reason == "preset_confirmation_required":
		return "preset_confirmation", ""
	case strings.HasPrefix(reason, "skill_handoff"):
		return "skill_handoff", ""
	default:
		return "confirmation", ""
	}
}

// commandBoundaryNextActionKind mirrors commandBoundaryNextAction as a
// machine-readable kind plus the exact command for command-boundary stops.
func commandBoundaryNextActionKind(reason string) (kind, command string) {
	switch reason {
	case "run_slipway_done_to_finalize":
		return "command", "slipway done"
	case "run_slipway_run_to_advance":
		return "command", "slipway run"
	case "blocked_by_governance":
		return "blocker_resolution", ""
	default:
		return "command", ""
	}
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
		if len(view.Advanced.SideEffects) > 0 {
			heading := "Side Effects:"
			if view.Advanced.RecoveryOnly {
				heading = "Recovery Side Effects:"
			}
			writeLine("%s\n", heading)
			for _, effect := range view.Advanced.SideEffects {
				if effect.Detail != "" {
					writeLine("  - %s: %s\n", effect.Kind, effect.Detail)
				} else {
					writeLine("  - %s\n", effect.Kind)
				}
			}
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
		writeLine("  Verification Dir: %s\n", view.NextSkill.VerificationDir)
		writeLine("  Evidence State: %s\n", view.NextSkill.State)
		if len(view.NextSkill.RequiredTokens) > 0 {
			writeLine("  Required Tokens: %s\n", strings.Join(view.NextSkill.RequiredTokens, ", "))
		}
		if view.NextSkill.ReviewContext != nil {
			if len(view.NextSkill.ReviewContext.RequiredArtifactLayers) > 0 {
				writeLine("  Required Artifact Layers: %s\n", strings.Join(view.NextSkill.ReviewContext.RequiredArtifactLayers, ", "))
			}
			if len(view.NextSkill.ReviewContext.RequiredImplementationLayers) > 0 {
				writeLine("  Required Implementation Layers: %s\n", strings.Join(view.NextSkill.ReviewContext.RequiredImplementationLayers, ", "))
			}
		}

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
