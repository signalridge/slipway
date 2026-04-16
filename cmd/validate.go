package cmd

import (
	"github.com/signalridge/slipway/internal/engine/capability"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
	"github.com/spf13/cobra"
)

type validateView struct {
	Slug                      string                      `json:"slug"`
	QualityMode               string                      `json:"quality_mode,omitempty"`
	WorkflowPreset            string                      `json:"workflow_preset,omitempty"`
	SuggestedWorkflowPreset   string                      `json:"suggested_workflow_preset,omitempty"`
	EffectiveWorkflowPreset   string                      `json:"effective_workflow_preset,omitempty"`
	PresetConfirmationPending bool                        `json:"preset_confirmation_pending,omitempty"`
	PresetUpgradeReasons      []string                    `json:"preset_upgrade_reasons,omitempty"`
	GovernanceForecast        *governanceForecastView     `json:"governance_forecast,omitempty"`
	NeedsDiscovery            bool                        `json:"needs_discovery,omitempty"`
	Phase                     model.UserPhase             `json:"phase,omitempty"`
	ExecutionMode             string                      `json:"execution_mode"`
	CurrentState              model.WorkflowState         `json:"current_state"`
	IntakeSubStep             model.IntakeSubStep         `json:"intake_substep,omitempty"`
	PlanSubStep               model.PlanSubStep           `json:"plan_substep,omitempty"`
	PlanningNote              string                      `json:"planning_note,omitempty"`
	SkillsReady               map[string]string           `json:"skills_ready"`
	Blockers                  []model.ReasonCode          `json:"blockers"`
	CanAdvance                bool                        `json:"can_advance"`
	GateStatus                map[string]string           `json:"gate_status,omitempty"`
	GateDetails               map[string]model.GateRecord `json:"gate_details,omitempty"`
	EvidenceFreshness         string                      `json:"evidence_freshness"`
	Diagnostics               []string                    `json:"diagnostics,omitempty"`
	Mode                      string                      `json:"mode,omitempty"`
	HydrateReferences         []string                    `json:"hydrate_references,omitempty"`
	SuggestedCapabilities     []suggestedCapabilityView   `json:"suggested_capabilities,omitempty"`
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
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}

			ref, err := resolveActiveChangeRef(root, changeSlug)
			if err != nil {
				if shouldFallbackValidateDiagnostics(err) {
					view := diagnosticValidateView("no active change or ambiguous; use `--change <slug>` or run `slipway repair`")
					view.Mode = effectiveMode
					view.HydrateReferences = normalizeHydrateKeys(resolveEffectiveFocusHydrate("validate", focus))
					view.SuggestedCapabilities = buildSuggestedCapabilities(capability.Signals{
						Command: "validate",
						Focus:   focus,
					})
					return encodeJSONResponse(cmd, view)
				}
				return err
			}

			view, err := buildValidateViewForSlug(root, ref.Slug)
			if err != nil {
				return err
			}
			change, err := state.LoadChange(root, ref.Slug)
			if err != nil {
				return err
			}
			execSummary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
			if err != nil {
				return err
			}
			view.Mode = effectiveMode
			view.HydrateReferences = normalizeHydrateKeys(resolveEffectiveFocusHydrate("validate", focus))
			view.SuggestedCapabilities = buildSuggestedCapabilities(
				suggestedCapabilitySignalsForChange("validate", focus, change, execSummary, view.Blockers),
			)
			return encodeJSONResponse(cmd, view)
		},
	}
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
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

	// Validate only needs the shared blocker/gate snapshot. It intentionally
	// leaves projection/review/ship surfaces off to keep the read path narrow.
	readiness, err := progression.EvaluateGovernanceReadiness(root, change, progression.GovernanceReadinessOptions{
		IncludeGateEvaluations: true,
	})
	if err != nil {
		return validateView{}, wrapGovernanceReadinessError("validate readiness", change.Slug, err)
	}
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
	gateDetails := gateStatusFromEvaluations(readiness.GateEvaluations)
	gateStatus := map[string]string{}
	for name, gate := range gateDetails {
		gateStatus[name] = string(gate.Status)
	}
	view.GateStatus = gateStatus
	view.GateDetails = gateDetails
	return view, nil
}
