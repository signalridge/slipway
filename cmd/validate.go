package cmd

import (
	"errors"
	"io/fs"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/engine/scopecontract"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
	"github.com/spf13/cobra"
)

type validateView struct {
	Slug                      string                               `json:"slug"`
	QualityMode               string                               `json:"quality_mode,omitempty"`
	WorkflowPreset            string                               `json:"workflow_preset,omitempty"`
	SuggestedWorkflowPreset   string                               `json:"suggested_workflow_preset,omitempty"`
	EffectiveWorkflowPreset   string                               `json:"effective_workflow_preset,omitempty"`
	PresetConfirmationPending bool                                 `json:"preset_confirmation_pending,omitempty"`
	PresetUpgradeReasons      []string                             `json:"preset_upgrade_reasons,omitempty"`
	GovernanceForecast        *governanceForecastView              `json:"governance_forecast,omitempty"`
	NeedsDiscovery            bool                                 `json:"needs_discovery,omitempty"`
	Phase                     model.UserPhase                      `json:"phase,omitempty"`
	ExecutionMode             string                               `json:"execution_mode"`
	CurrentState              model.WorkflowState                  `json:"current_state"`
	IntakeSubStep             model.IntakeSubStep                  `json:"intake_substep,omitempty"`
	PlanSubStep               model.PlanSubStep                    `json:"plan_substep,omitempty"`
	PlanningNote              string                               `json:"planning_note,omitempty"`
	SkillsReady               map[string]string                    `json:"skills_ready"`
	Blockers                  []model.ReasonCode                   `json:"blockers"`
	Recovery                  *model.RecoverySummary               `json:"recovery,omitempty"`
	CanAdvance                bool                                 `json:"can_advance"`
	GateStatus                map[string]string                    `json:"gate_status,omitempty"`
	GateDetails               map[string]model.GateRecord          `json:"gate_details,omitempty"`
	EvidenceFreshness         string                               `json:"evidence_freshness"`
	FreshnessDiagnostics      *state.ExecutionFreshnessDiagnostics `json:"freshness_diagnostics,omitempty"`
	ActionableNextSkill       *actionableNextSkillView             `json:"actionable_next_skill,omitempty"`
	RequirementsContract      *requirementsContractView            `json:"requirements_contract,omitempty"`
	ScopeContract             *scopeContractView                   `json:"scope_contract,omitempty"`
	Diagnostics               []string                             `json:"diagnostics,omitempty"`
	Mode                      string                               `json:"mode,omitempty"`
	HydrateReferences         []string                             `json:"hydrate_references,omitempty"`
	ArtifactAmendments        []artifact.AmendmentEvent            `json:"artifact_amendments,omitempty"`
}

type actionableNextSkillView struct {
	Name             string   `json:"name"`
	DisplayName      string   `json:"display_name,omitempty"`
	BlockingName     string   `json:"blocking_name,omitempty"`
	ResolutionReason string   `json:"resolution_reason,omitempty"`
	RequiredTokens   []string `json:"required_tokens,omitempty"`
}

type requirementsContractView struct {
	Status  string `json:"status"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message,omitempty"`
}

type scopeContractView struct {
	Status                  string             `json:"status"`
	PlannedTargets          []string           `json:"planned_targets,omitempty"`
	ChangedFiles            []string           `json:"changed_files,omitempty"`
	OutOfScopeFiles         []string           `json:"out_of_scope_files,omitempty"`
	MissingContractTasks    []string           `json:"missing_contract_tasks,omitempty"`
	MissingChangedFileTasks []string           `json:"missing_changed_file_tasks,omitempty"`
	Blockers                []model.ReasonCode `json:"blockers,omitempty"`
}

func diagnosticValidateView(message string) validateView {
	return validateView{
		ExecutionMode:     "diagnostics",
		EvidenceFreshness: "unknown",
		Diagnostics:       []string{message},
	}
}

func shouldFallbackValidateDiagnostics(err error) bool {
	cliErr := asCLIError(err)
	if cliErr == nil || cliErr.Category != categoryPrecondition {
		return false
	}
	return cliErr.ErrorCode == "no_active_change" || cliErr.ErrorCode == "active_context_ambiguous"
}

func buildSkillsReady(passing map[string]model.VerificationRecord) map[string]string {
	skillsReady := make(map[string]string, len(passing))
	for name, rec := range passing {
		skillsReady[name] = rec.Verdict
	}
	return skillsReady
}

func buildValidateViewBase(
	root string,
	change model.Change,
	execMode string,
	executionSummary *model.ExecutionSummary,
	blockers []model.ReasonCode,
	skillsReady map[string]string,
	diagnostics []string,
) validateView {
	return validateView{
		Slug:          change.Slug,
		Phase:         model.PhaseFor(change.CurrentState),
		ExecutionMode: execMode,
		CurrentState:  change.CurrentState,
		IntakeSubStep: change.IntakeSubStep,
		PlanSubStep:   change.PlanSubStep,
		PlanningNote:  planningNote(change.CurrentState, change.PlanSubStep),
		SkillsReady:   skillsReady,
		Blockers:      model.NormalizeReasonCodes(blockers),
		Recovery:      buildValidateRecovery(blockers, nil),
		CanAdvance:    len(blockers) == 0,
		Diagnostics:   diagnostics,
		EvidenceFreshness: projectFreshnessForExecMode(
			root,
			change,
			executionSummary,
			blockers,
		),
	}
}

func buildValidateRecovery(blockers []model.ReasonCode, gateDetails map[string]model.GateRecord) *model.RecoverySummary {
	all := append([]model.ReasonCode(nil), blockers...)
	for _, gate := range gateDetails {
		all = append(all, gate.ReasonCodes...)
	}
	return model.BuildRecovery(all)
}

func makeValidateCmd() *cobra.Command {
	var changeSlug string
	var focus string
	var jsonOutput bool
	var listFocuses bool
	var discoveryFormat string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: desc("validate"),
		Long: desc("validate") + ".\n\n" +
			"Use this command to inspect current evidence and gate readiness without advancing state.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if listFocuses {
				return emitFocusDiscovery(cmd, "validate", discoveryFormat)
			}
			if err := validateFocus("validate", focus); err != nil {
				return err
			}
			effectiveMode := resolveEffectiveFocus("validate", focus)
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}

			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				if shouldFallbackValidateDiagnostics(err) {
					view := diagnosticValidateView("no active change or ambiguous; use `--change <slug>` or run `slipway repair`")
					view.Mode = effectiveMode
					view.HydrateReferences = normalizeHydrateKeys(resolveEffectiveFocusHydrate("validate", focus))
					return encodeJSONResponse(cmd, view)
				}
				return err
			}

			view, err := buildValidateViewForSlug(root, ref.Slug)
			if err != nil {
				return err
			}
			applyValidateInvocationWorkspacePath(cmd, root, &view)
			view.Mode = effectiveMode
			view.HydrateReferences = normalizeHydrateKeys(resolveEffectiveFocusHydrate("validate", focus))
			return encodeJSONResponse(cmd, view)
		},
	}
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit active change slug")
	cmd.Flags().StringVar(&focus, "focus", "", "Validate focus (e.g. sast, property, mutation)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output (validate currently emits JSON only)")
	cmd.Flags().BoolVar(&listFocuses, "list-focuses", false, "List public --focus aliases for this command and exit")
	cmd.Flags().StringVar(&discoveryFormat, "format", "text", "Output format for --list-focuses: text|json")
	return cmd
}

func buildValidateViewForSlug(root, slug string) (validateView, error) {
	change, err := state.LoadChange(root, slug)
	if err != nil {
		return validateView{}, err
	}

	execMode := governedExecutionMode
	execCtx, err := loadExecutionContext(root, change)
	if err != nil {
		return validateView{}, err
	}

	presetFields, err := buildWorkflowPresetView(root, change)
	if err != nil {
		return validateView{}, err
	}

	// Preset confirmation is a universal early S1 blocker. When pending,
	// return a minimal view — skip artifact reconciliation, skill evaluation,
	// worktree validation, governance surface, and all downstream checks.
	if change.WorkflowPresetConfirmationPending() {
		view := buildValidateViewBase(root, change, execMode, execCtx.Summary, []model.ReasonCode{model.NewReasonCode("preset_confirmation_required", "")}, nil, nil)
		profile := buildChangeProfileView(change)
		view.QualityMode = profile.QualityMode
		view.NeedsDiscovery = profile.NeedsDiscovery
		view.WorkflowPreset = presetFields.WorkflowPreset
		view.SuggestedWorkflowPreset = presetFields.SuggestedWorkflowPreset
		view.EffectiveWorkflowPreset = presetFields.EffectiveWorkflowPreset
		view.PresetConfirmationPending = presetFields.PresetConfirmationPending
		view.PresetUpgradeReasons = presetFields.PresetUpgradeReasons
		view.GovernanceForecast = presetFields.GovernanceForecast
		return view, nil
	}

	var requirementsContract *requirementsContractView
	if bundleDir, err := state.GovernedBundleDir(root, change); err == nil {
		contract, err := artifact.EvaluateRequirementsContract(bundleDir, change.Slug)
		if err == nil {
			requirementsContract = &requirementsContractView{
				Status:  string(contract.Status),
				Source:  contract.Source,
				Message: contract.Message,
			}
		}
	}

	// Validate remains read-only, but includes artifact projection so JSON callers
	// can distinguish stable artifacts from projected auto-amendments.
	readiness, err := progression.EvaluateGovernanceReadiness(root, change, progression.GovernanceReadinessOptions{
		IncludeGateEvaluations:    true,
		IncludeArtifactProjection: true,
	})
	if err != nil && errors.Is(err, fs.ErrPermission) {
		readiness, err = progression.EvaluateGovernanceReadiness(root, change, progression.GovernanceReadinessOptions{
			IncludeGateEvaluations: true,
		})
	}
	if err != nil {
		return validateView{}, wrapGovernanceReadinessError("validate readiness", change.Slug, err)
	}
	readiness.GateEvaluations = defaultVisibleGateEvaluations(change, readiness.GateEvaluations)
	blockers := append([]model.ReasonCode(nil), readiness.Blockers...)
	diagnostics := append([]string{}, readiness.Diagnostics...)

	view := buildValidateViewBase(
		root,
		change,
		execMode,
		execCtx.Summary,
		blockers,
		buildSkillsReady(readiness.PassingSkills),
		stringutil.UniqueSorted(diagnostics),
	)
	profile := buildChangeProfileView(change)
	view.QualityMode = profile.QualityMode
	view.WorkflowPreset = presetFields.WorkflowPreset
	view.SuggestedWorkflowPreset = presetFields.SuggestedWorkflowPreset
	view.EffectiveWorkflowPreset = presetFields.EffectiveWorkflowPreset
	view.PresetConfirmationPending = presetFields.PresetConfirmationPending
	view.PresetUpgradeReasons = presetFields.PresetUpgradeReasons
	view.GovernanceForecast = presetFields.GovernanceForecast
	view.NeedsDiscovery = profile.NeedsDiscovery
	view.RequirementsContract = requirementsContract
	view.ScopeContract = buildScopeContractView(readiness.ScopeContract)
	view.FreshnessDiagnostics = attachFreshnessDiagnostics(readiness.FreshnessDiagnostics)
	view.ActionableNextSkill = buildActionableNextSkillView(change, readiness)
	gateDetails := gateStatusFromEvaluations(readiness.GateEvaluations)
	gateStatus := map[string]string{}
	for name, gate := range gateDetails {
		gateStatus[name] = string(gate.Status)
	}
	view.GateStatus = gateStatus
	view.GateDetails = gateDetails
	if staleTarget, ok, err := progression.StaleEvidenceRecoveryAvailable(root, change, appendValidateRecoveryInputs(view.Blockers, gateDetails)); err != nil {
		return validateView{}, err
	} else if ok {
		view.Blockers = appendReasonCodes(
			view.Blockers,
			[]model.ReasonCode{
				model.NewReasonCode("stale_evidence_recovery_available", staleTarget.Label()),
				model.NewReasonCode("run_slipway_run_to_advance", string(change.CurrentState)),
			},
		)
	}
	view.Recovery = buildValidateRecovery(view.Blockers, gateDetails)
	if readiness.ArtifactProjection != nil && len(readiness.ArtifactProjection.Amendments) > 0 {
		view.ArtifactAmendments = append([]artifact.AmendmentEvent(nil), readiness.ArtifactProjection.Amendments...)
	}
	return view, nil
}

func appendValidateRecoveryInputs(blockers []model.ReasonCode, gateDetails map[string]model.GateRecord) []model.ReasonCode {
	all := append([]model.ReasonCode(nil), blockers...)
	for _, gate := range gateDetails {
		all = append(all, gate.ReasonCodes...)
	}
	return model.NormalizeReasonCodes(all)
}

func buildScopeContractView(report *scopecontract.Report) *scopeContractView {
	if report == nil {
		return nil
	}
	return &scopeContractView{
		Status:                  string(report.Status),
		PlannedTargets:          append([]string(nil), report.PlannedTargets...),
		ChangedFiles:            append([]string(nil), report.ChangedFiles...),
		OutOfScopeFiles:         append([]string(nil), report.OutOfScopeFiles...),
		MissingContractTasks:    append([]string(nil), report.MissingContractTasks...),
		MissingChangedFileTasks: append([]string(nil), report.MissingChangedFileTasks...),
		Blockers:                append([]model.ReasonCode(nil), report.Blockers...),
	}
}

func buildActionableNextSkillView(change model.Change, readiness progression.GovernanceReadiness) *actionableNextSkillView {
	displaySkill, _ := progression.ResolveNextSkill(change)
	if displaySkill == "" {
		return nil
	}
	actionableSkill := displaySkill
	reason := ""
	if resolved, resolvedReason := resolveActionableBlockingSkill(displaySkill, readiness.PassingSkills, readiness.Blockers); resolved != "" {
		actionableSkill = resolved
		reason = resolvedReason
	} else if skillHasPassingEvidence(readiness.PassingSkills, displaySkill) {
		return nil
	}
	view := &actionableNextSkillView{
		Name:           actionableSkill,
		RequiredTokens: progression.RequiredReviewLayerTokensForSkill(change, readiness.ArtifactProjection, false, actionableSkill),
	}
	if displaySkill != actionableSkill {
		view.DisplayName = displaySkill
		view.BlockingName = actionableSkill
		view.ResolutionReason = reason
	}
	return view
}
