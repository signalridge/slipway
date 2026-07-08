package cmd

import (
	"io"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/capability"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

type nextView struct {
	Command                     string                               `json:"command,omitempty"`
	DelegatedTo                 string                               `json:"delegated_to,omitempty"`
	Slug                        string                               `json:"slug"`
	QualityMode                 string                               `json:"quality_mode,omitempty"`
	WorkflowProfile             string                               `json:"workflow_profile,omitempty"`
	WorkflowPreset              string                               `json:"workflow_preset,omitempty"`
	SuggestedWorkflowPreset     string                               `json:"suggested_workflow_preset,omitempty"`
	EffectiveWorkflowPreset     string                               `json:"effective_workflow_preset,omitempty"`
	PresetConfirmationPending   bool                                 `json:"preset_confirmation_pending,omitempty"`
	PresetUpgradeReasons        []string                             `json:"preset_upgrade_reasons,omitempty"`
	GovernanceForecast          *governanceForecastView              `json:"governance_forecast,omitempty"`
	NeedsDiscovery              bool                                 `json:"needs_discovery,omitempty"`
	ComplexityLevel             string                               `json:"complexity_level,omitempty"`
	GuardrailDomain             string                               `json:"guardrail_domain,omitempty"`
	Phase                       model.UserPhase                      `json:"phase"`
	ExecutionMode               string                               `json:"execution_mode"`
	CurrentState                model.WorkflowState                  `json:"current_state"`
	IntakeSubStep               model.IntakeSubStep                  `json:"intake_substep,omitempty"`
	PlanSubStep                 model.PlanSubStep                    `json:"plan_substep,omitempty"`
	PlanningNote                string                               `json:"planning_note,omitempty"`
	LifecycleStatus             string                               `json:"lifecycle_status"`
	InvocationRoute             *invocationRouteView                 `json:"invocation_route,omitempty"`
	CurrentActionKind           string                               `json:"current_action_kind,omitempty"`
	CurrentActionCommand        string                               `json:"current_action_command,omitempty"`
	HostCapabilities            []hostCapabilityView                 `json:"host_capabilities,omitempty"`
	Advanced                    *progression.AdvanceSummary          `json:"advanced,omitempty"`
	AutoTransitions             []progression.AdvanceSummary         `json:"auto_transitions,omitempty"`
	NextSkill                   *nextSkillView                       `json:"next_skill"`
	ReviewBatch                 *reviewBatchView                     `json:"review_batch,omitempty"`
	InputContext                nextContext                          `json:"input_context"`
	Constraints                 *agentConstraints                    `json:"constraints,omitempty"`
	GovernanceSignals           *governanceSignalView                `json:"governance_signals,omitempty"`
	ActiveControls              []governanceControlView              `json:"active_controls,omitempty"`
	RequiredActions             []governanceActionView               `json:"required_actions,omitempty"`
	SkillEvidence               []skillEvidenceEntry                 `json:"skill_evidence,omitempty"`
	AutoPassEligible            []model.AutoPassedState              `json:"auto_pass_eligible,omitempty"`
	ArtifactAmendments          []artifact.AmendmentEvent            `json:"artifact_amendments,omitempty"`
	EvidenceFreshness           string                               `json:"evidence_freshness,omitempty"`
	ExecutionEvidenceFreshness  string                               `json:"execution_evidence_freshness,omitempty"`
	GovernanceEvidenceFreshness string                               `json:"governance_evidence_freshness,omitempty"`
	OverallReadinessFreshness   string                               `json:"overall_readiness_freshness,omitempty"`
	FreshnessDiagnostics        *state.ExecutionFreshnessDiagnostics `json:"freshness_diagnostics,omitempty"`
	Warnings                    []string                             `json:"warnings,omitempty"`
	Blockers                    []model.ReasonCode                   `json:"blockers"`
	Recovery                    *model.RecoverySummary               `json:"recovery,omitempty"`
	ConfirmationRequirement     confirmationRequirement              `json:"confirmation_requirement"`

	// planLocked records whether the lifecycle G_plan gate has approved the plan.
	// It is unexported (never serialized) and drives whether a parsed decision.md
	// selection is reported as a locked or pending skill_constraint (issue #140).
	planLocked bool
	// ownedAdvanceGateDeadEndBlockers holds the genuine dead-end reason codes of the
	// gate that OWNS advancement out of the current (state, plan substep), captured
	// only when that gate is blocked. It is unexported (never serialized) and is
	// applied to Blockers in the no-skill ready/run-to-advance posture so a
	// dead-end (e.g. plan_audit_origin_invalid) overrides "ready to advance" while
	// pacing blocks keep riding the normal handoff/run guidance (#382).
	ownedAdvanceGateDeadEndBlockers []model.ReasonCode
	// auto records whether auto-advance execution is in effect for this view. It is
	// unexported (never serialized) and only softens pure-pacing confirmation
	// boundaries in deriveConfirmationRequirement for non-guardrail changes; it
	// never weakens an evidence gate or a guardrail/sensitive boundary.
	auto bool
	// config is the loaded project configuration used to project host-facing
	// delegation directives without re-reading .slipway.yaml at every surface.
	config model.Config
}

type confirmationRequirement struct {
	Required                     bool   `json:"required"`
	Boundary                     string `json:"boundary"`
	FreshConfirmationRequired    bool   `json:"fresh_confirmation_required"`
	PriorAuthorizationSufficient bool   `json:"prior_authorization_sufficient"`
	Reason                       string `json:"reason"`
	NextAction                   string `json:"next_action,omitempty"`
	NextActionKind               string `json:"next_action_kind,omitempty"`
	NextCommand                  string `json:"next_command,omitempty"`
}

type hostCapabilityView struct {
	SkillName           string `json:"skill_name"`
	Capability          string `json:"capability"`
	Required            bool   `json:"required"`
	Availability        string `json:"availability"`
	FallbackSelected    bool   `json:"fallback_selected,omitempty"`
	FallbackMode        string `json:"fallback_mode,omitempty"`
	EvidenceRequirement string `json:"evidence_requirement,omitempty"`
	Remediation         string `json:"remediation,omitempty"`
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

type nextSkillView struct {
	Name                 string             `json:"name"`
	DisplayName          string             `json:"display_name,omitempty"`
	BlockingName         string             `json:"blocking_name,omitempty"`
	ResolutionReason     string             `json:"resolution_reason,omitempty"`
	Subagent             *subagentDirective `json:"subagent,omitempty"`
	SelectedReviewSkills []string           `json:"selected_review_skills,omitempty"`
	RequiredTokens       []string           `json:"required_tokens,omitempty"`
	VerificationDir      string             `json:"verification_dir"`
	State                string             `json:"state"`
	SkillConstraints     *skillConstraints  `json:"skill_constraints,omitempty"`
	ReviewContext        *reviewContextView `json:"review_context,omitempty"`
	TechniqueHints       []techniqueHint    `json:"technique_hints,omitempty"`
}

type reviewBatchView struct {
	Mode            string                 `json:"mode"`
	Subagent        *subagentDirective     `json:"subagent,omitempty"`
	Skills          []reviewBatchSkillView `json:"skills"`
	VerificationDir string                 `json:"verification_dir"`
	State           string                 `json:"state"`
}

type subagentDirective = model.ResolvedSubagentDirective

type reviewBatchSkillView struct {
	Name             string             `json:"name"`
	RequiredTokens   []string           `json:"required_tokens,omitempty"`
	ReviewContext    *reviewContextView `json:"review_context,omitempty"`
	TechniqueHints   []techniqueHint    `json:"technique_hints,omitempty"`
	SkillConstraints *skillConstraints  `json:"skill_constraints,omitempty"`
}

// skillConstraints carries per-skill metadata from the Go registry
// and state context, replacing level-conditional logic in skill templates.
type skillConstraints struct {
	LockedDecisions []string `json:"locked_decisions,omitempty"`
	// PendingDecisions carries the recommended-but-unconfirmed selected
	// approach/direction parsed from decision.md while the lifecycle has NOT yet
	// locked the plan (the G_plan gate is not approved). It is surfaced
	// separately from LockedDecisions so a host still sees the recommendation but
	// knows it is pending fresh confirmation and must not treat it as locked
	// (issue #140).
	PendingDecisions []string `json:"pending_decisions,omitempty"`
	GuardrailDomain  string   `json:"guardrail_domain,omitempty"`
	MitigationTarget string   `json:"mitigation_target,omitempty"`
	RunSummaryBound  bool     `json:"run_summary_bound,omitempty"`
	// RequiredHighRiskTokens lists the exact reference tokens the ship-verification
	// host must record (from a real SAST run) to satisfy the ship gate's high-risk
	// checks when the change has a guardrail domain. Populated only for the
	// ship-verification handoff so the next agent never has to guess the format.
	RequiredHighRiskTokens []string `json:"required_high_risk_tokens,omitempty"`
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
	ExecutionResume        *executionResumeContext    `json:"execution_resume,omitempty"`
	GateStatus             map[string]string          `json:"gate_status,omitempty"`
	ArtifactStatus         map[string]string          `json:"artifact_status,omitempty"`
	WavePlan               *wavePlanView              `json:"wave_plan,omitempty"`
}

// handoffContextView is the diagnostic-only bounded-reference projection surfaced
// as input_context.handoff_context on `slipway next --json --diagnostics`. It
// curates the durable paths a resuming or handing-off agent should read (change
// authority, config, lifecycle log, policy packs) plus parsed advisory policy
// content, under a bounded_references_only context policy. It is orthogonal to
// the retired context-pressure estimator: it never measures or estimates context
// usage (the former context_budget hint field was dropped with that removal).
type handoffContextView struct {
	WorkflowProfile   string              `json:"workflow_profile"`
	ContextPolicy     string              `json:"context_policy"`
	Trace             *handoffTraceView   `json:"trace,omitempty"`
	ReadRefs          []handoffReadRef    `json:"read_refs,omitempty"`
	PolicyPacks       []handoffPolicyPack `json:"policy_packs,omitempty"`
	Risk              *handoffRiskView    `json:"risk,omitempty"`
	ChangeAuthority   string              `json:"change_authority"`
	LifecycleEventLog string              `json:"lifecycle_event_log,omitempty"`
	ConfigPath        string              `json:"config_path,omitempty"`
	RequiredReads     []string            `json:"required_reads,omitempty"`
}

type handoffTraceView struct {
	CorrelationID string `json:"correlation_id"`
	EventLog      string `json:"event_log,omitempty"`
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

// wavePlanView is the diagnostic-only, non-persistable projection surfaced as
// input_context.wave_plan on `slipway next --json`. It is intentionally
// distinct from the persistable, engine-owned wave-plan.yaml cache schema
// (model.WavePlan): it carries view-only fields (WaveCount/wave_count and
// Advisories/advisories) that model.WavePlan does not define. These view-only
// fields must never be copied into the persisted cache — the cache parses with
// KnownFields(true) and will fail closed (wave_plan_unreadable) on them. Treat
// this struct as a read-side diagnostic surface only; do not round-trip it
// through wave-plan.yaml.
type wavePlanView struct {
	TotalTasks       int                `json:"total_tasks"`
	WaveCount        int                `json:"wave_count"`
	ExecutorSubagent *subagentDirective `json:"executor_subagent,omitempty"`
	Waves            []waveView         `json:"waves"`
	// Advisories are non-blocking wave-narrowing cues (REQ-006) computed in the
	// view layer only. They never block execution and are deliberately excluded
	// from wave-plan.yaml and every freshness hash; they exist solely on the
	// `slipway next` JSON so plan-audit can cite concrete evidence.
	Advisories []string `json:"advisories,omitempty"`
	ParseError string   `json:"parse_error,omitempty"`
}

type waveView struct {
	WaveIndex int  `json:"wave_index"`
	Parallel  bool `json:"parallel"`
	// Tasks in a parallel wave are dependency-free and file-disjoint, so the host
	// dispatches them concurrently by default unless parallelization is off.
	Tasks []waveTaskView `json:"tasks"`
}

type waveTaskView struct {
	TaskID      string   `json:"task_id"`
	Objective   string   `json:"objective,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
	TargetFiles []string `json:"target_files,omitempty"`
	TaskKind    string   `json:"task_kind"`
}

type executionResumeContext struct {
	// RunSummaryVersion is omitted when zero. Execution resume only exists once
	// an execution run has been recorded, so a real version is >=1; zero is the
	// "no summary yet" sentinel that `evidence task` rejects and must not be
	// surfaced as a recorded version (issue #211).
	RunSummaryVersion int      `json:"run_summary_version,omitempty"`
	CompletedTaskIDs  []string `json:"completed_task_ids,omitempty"`
	Freshness         string   `json:"freshness,omitempty"`
	ResumeWaveIndex   int      `json:"resume_wave_index,omitempty"`
}

func makeNextCmd() *cobra.Command {
	var jsonOutput bool
	var noAutoPass bool
	var diagnostics bool
	var changeSlug string

	cmd := &cobra.Command{
		Use:   "next",
		Short: desc("next"),
		Long:  desc("next"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromCommand(cmd)
			if err != nil {
				return err
			}
			readCtx := newStateReadContext(root)

			ref, err := resolveActiveChangeRefWithReadContext(readCtx, changeSlug)
			if err != nil {
				return err
			}

			// next reflects the project's effective auto setting in its rendered
			// confirmation requirement, but never advances state (preview-only), so
			// auto can soften the displayed requirement without any side effect.
			auto, err := resolveEffectiveAuto(root, nil, false, false)
			if err != nil {
				return err
			}

			return withChangeStateLock(root, ref.Slug, "next", func() error {
				lockedChange, err := readCtx.reloadChange(ref.Slug)
				if err != nil {
					return err
				}
				ref := changeRef{Slug: lockedChange.Slug}
				if jsonOutput && !diagnostics {
					view, err := buildNextViewForCommandWithReadContext(readCtx, ref, nextViewOptions{
						Preview:          true,
						AutoSkipEvidence: false,
						SkipAutoPass:     noAutoPass,
						Command:          "next",
						Auto:             auto,
					})
					if err != nil {
						return err
					}
					applyNextInvocationWorkspacePathWithReadContext(cmd, readCtx, &view)
					applyNextInvocationRouteWithReadContext(cmd, readCtx, lockedChange, strings.TrimSpace(changeSlug) != "", &view)
					return encodeJSONResponse(cmd, buildNextHandoffView(view))
				}

				// next is always query-only; state advancement is owned by `run`.
				view, err := buildNextViewForCommandWithReadContext(readCtx, ref, nextViewOptions{
					Preview:          true,
					AutoSkipEvidence: !jsonOutput,
					SkipAutoPass:     noAutoPass,
					Command:          "next",
					Auto:             auto,
				})
				if err != nil {
					return err
				}
				applyNextInvocationWorkspacePathWithReadContext(cmd, readCtx, &view)
				applyNextInvocationRouteWithReadContext(cmd, readCtx, lockedChange, strings.TrimSpace(changeSlug) != "", &view)

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
	cmd.Flags().BoolVar(&noAutoPass, "no-auto-pass", false, "Skip auto-pass and report eligibility instead")
	cmd.Flags().BoolVar(&diagnostics, "diagnostics", false, "Include diagnostic governance/readiness details")
	addChangeSelectorFlags(cmd, &changeSlug, "Explicit change slug")
	return cmd
}

// nextViewOptions carries the per-command knobs for buildNextViewForCommand.
// Grouping them in a named struct keeps the call sites self-documenting instead
// of relying on a long tail of positional booleans.
type nextViewOptions struct {
	// Preview makes the build read-only: no advancement side effects.
	Preview bool
	// AutoSkipEvidence advances but defers evidence prompts the caller surfaces.
	AutoSkipEvidence bool
	// SkipAutoPass advances state but suppresses auto-pass so the caller decides.
	SkipAutoPass bool
	// Command is the owning command name (e.g. "next", "run", a stage name).
	Command string
	// Auto carries execution.auto into advancement and confirmation softening.
	Auto bool
}

func buildNextViewForCommand(root string, ref changeRef, opts nextViewOptions) (nextView, error) {
	return buildNextViewForCommandWithReadContext(newStateReadContext(root), ref, opts)
}

func buildNextViewForCommandWithReadContext(readCtx *stateReadContext, ref changeRef, opts nextViewOptions) (nextView, error) {
	root := readCtx.root
	cfg, err := loadConfigAtRoot(root)
	if err != nil {
		return nextView{}, err
	}
	preview := opts.Preview
	autoSkipEvidence := opts.AutoSkipEvidence
	skipAutoPass := opts.SkipAutoPass
	command := opts.Command
	auto := opts.Auto
	// READ-ONLY PREVIEW INVARIANT: only the advancing path (run/stage) carries
	// auto into advancement so the engine preset auto-confirm can fire. A preview
	// query (`slipway next`) must never trigger an auto-confirm side effect, so
	// advancement-auto is forced false in preview. The preview still reflects the
	// softened confirmation requirement below via view.auto, which is read-only.
	advanced, err := advanceIfReadyAuto(root, ref, preview, skipAutoPass, command, auto && !preview)
	if err != nil {
		return nextView{}, err
	}
	command = strings.TrimSpace(command)

	view := newBaseNextView(root, cfg, command, ref, auto)
	if shouldExposeAdvancedSummaryToCaller(advanced) {
		view.Advanced = &advanced
	}

	// Preset confirmation gate: check BEFORE buildNextContextByMode to prevent
	// artifact reconciliation side effects (ReconcileFromFilesystem, SaveChange)
	// and artifact_status leakage when the preset is still pending.
	if pending, err := checkPresetPendingEarlyReturnWithReadContext(readCtx, ref, &view); err != nil {
		return nextView{}, err
	} else if pending {
		view.Recovery = model.BuildRecovery(view.Blockers)
		return view, nil
	}

	governedChange, execCtx, err := buildNextContextByModeWithReadContext(readCtx, &view, ref)
	if err != nil {
		return nextView{}, err
	}
	if governedChange != nil {
		// Surface the authoritative guardrail/sensitivity domain on the view so
		// deriveConfirmationRequirement can keep sensitive boundaries fail-closed
		// under auto. buildNextContextByMode does not set this field.
		view.GuardrailDomain = strings.TrimSpace(governedChange.GuardrailDomain)
	}
	view.Phase = model.PhaseFor(view.CurrentState)
	var nextSkillEvidence map[string]model.VerificationRecord
	var nextSkillArtifactProjection *progression.ArtifactProjection
	if governedChange != nil {
		nextSkillEvidence, nextSkillArtifactProjection, err = applyGovernedReadinessToNextView(root, readCtx, &view, ref, *governedChange, execCtx)
		if err != nil {
			return nextView{}, err
		}
	}

	// Attach the live tasks.md-derived wave projection in execution, and at review
	// only when S3 convergence needs to expose the authoritative task map. Settled
	// S3 handoffs stay lean and do not build diagnostic surfaces.
	if view.CurrentState == model.StateS2Implement && governedChange != nil {
		view.InputContext.WavePlan = buildWavePlan(root, view.InputContext.ArtifactBundle, view.config)
	} else if view.CurrentState == model.StateS3Review && governedChange != nil && s3WavePlanProjectionNeeded(root, *governedChange, view.Blockers) {
		view.InputContext.WavePlan = buildWavePlan(root, view.InputContext.ArtifactBundle, view.config)
	}

	if view.CurrentState == model.StateDone {
		view.NextSkill = nil
		view.Blockers = []model.ReasonCode{model.NewReasonCode("change_is_done", "")}
		return finalizeNextView(root, governedChange, view)
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
		return finalizeNextView(root, governedChange, view)
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

	return finalizeNextView(root, governedChange, view)
}

// newBaseNextView constructs the initial next-view scaffold shared by every
// build path: identity, the planning-phase default, "unknown" freshness fields,
// the initializing confirmation requirement, and the unexported auto/config
// carriers. Downstream steps overwrite these fields as governance context
// resolves.
func newBaseNextView(root string, cfg model.Config, command string, ref changeRef, auto bool) nextView {
	return nextView{
		Command:                     command,
		Slug:                        ref.Slug,
		Phase:                       model.PhasePlanning,
		EvidenceFreshness:           "unknown",
		ExecutionEvidenceFreshness:  "unknown",
		GovernanceEvidenceFreshness: "unknown",
		OverallReadinessFreshness:   "unknown",
		ConfirmationRequirement:     confirmationNoBoundary("initializing"),
		auto:                        auto,
		config:                      cfg,
		InputContext: nextContext{
			WorkspaceRoot: root,
		},
	}
}

// applyGovernedReadinessToNextView evaluates governance readiness for a governed
// change and folds the result into the view: preset fields, readiness
// diagnostics and blockers, the owning-advance-gate dead-end capture (#382),
// freshness diagnostics and readiness freshness, artifact amendments, the
// readiness-derived context and governance surface, and the plan-locked flag. It
// returns the passing-skill evidence and artifact projection the skill-view
// assembly consumes. It performs the same reads and view mutations, in the same
// order, as the inline block it was extracted from.
func applyGovernedReadinessToNextView(
	root string,
	readCtx *stateReadContext,
	view *nextView,
	ref changeRef,
	governedChange model.Change,
	execCtx *executionContext,
) (map[string]model.VerificationRecord, *progression.ArtifactProjection, error) {
	presetFields, err := buildWorkflowPresetView(root, governedChange)
	if err != nil {
		return nil, nil, err
	}
	view.WorkflowPreset = presetFields.WorkflowPreset
	view.SuggestedWorkflowPreset = presetFields.SuggestedWorkflowPreset
	view.EffectiveWorkflowPreset = presetFields.EffectiveWorkflowPreset
	view.PresetConfirmationPending = presetFields.PresetConfirmationPending
	view.PresetUpgradeReasons = presetFields.PresetUpgradeReasons
	view.GovernanceForecast = presetFields.GovernanceForecast

	verificationRecords, err := readCtx.verificationRecords(governedChange)
	if err != nil {
		return nil, nil, wrapGovernanceReadinessError("evaluate next skill evidence", ref.Slug, err)
	}
	readiness, err := progression.EvaluateGovernanceReadiness(
		root,
		governedChange,
		progression.GovernanceReadinessOptions{
			IncludeGateEvaluations: true,
			// Next needs projected artifact context for rendering, but keeps the
			// reconcile read-only by requesting only the in-memory projection.
			IncludeArtifactProjection: true,
			VerificationRecords:       verificationRecords,
		},
	)
	if err != nil {
		return nil, nil, wrapGovernanceReadinessError("evaluate next skill evidence", ref.Slug, err)
	}
	view.Warnings = append(view.Warnings, readiness.Diagnostics...)
	view.Blockers = appendReasonCodes(view.Blockers, readiness.Blockers)
	// Capture the genuine dead-end reason codes of the gate that OWNS
	// advancement out of the current (state, plan substep) when that gate is
	// blocked. They are applied later in applyReadyAdvanceDiagnostics, bounded
	// to the no-skill ready/run-to-advance posture, so a dead-end (e.g.
	// plan_audit_origin_invalid) stops `next` advertising the step ready to
	// advance while the gate that owns advancement is blocked (#382). PACING
	// blocks (required-skill handoffs and other host-handoff ride-along codes)
	// are filtered out here so they keep riding the normal handoff/run guidance;
	// surfacing every blocked visible gate over-surfaced that pacing work and
	// erased the no_skill_required / run_slipway_run_to_advance guidance.
	if owning := progression.OwningAdvanceGateID(view.CurrentState, governedChange.PlanSubStep); owning != "" {
		if eval, ok := readiness.GateEvaluations[gate.GateID(owning)]; ok && eval.Status == model.GateStatusBlocked {
			for _, rc := range eval.ReasonCodes {
				if !progression.HostHandoffBlockerCanRide(rc) {
					view.ownedAdvanceGateDeadEndBlockers = append(view.ownedAdvanceGateDeadEndBlockers, rc)
				}
			}
		}
	}
	view.FreshnessDiagnostics = attachFreshnessDiagnostics(readiness.FreshnessDiagnostics)
	var summary *model.ExecutionSummary
	if execCtx != nil {
		summary = execCtx.Summary
	}
	applyReadinessFreshnessToNext(root, view, governedChange, summary, readiness)
	if readiness.ArtifactProjection != nil && len(readiness.ArtifactProjection.Amendments) > 0 {
		view.ArtifactAmendments = append([]artifact.AmendmentEvent(nil), readiness.ArtifactProjection.Amendments...)
	}
	applyReadinessToNextContext(view, readiness)
	applyGovernanceSurfaceToNext(readiness, view)
	view.planLocked = planLockedFromGates(readiness)
	return readiness.PassingSkills, readiness.ArtifactProjection, nil
}

// finalizeNextView applies the terminal read-only diagnostics shared by every
// return path — ready-advance diagnostics, the host-capability contract, overall
// readiness freshness, the derived confirmation requirement and its mirrored
// current-action fields, and the recovery summary — then returns the completed
// view. It is a pure projection over the accumulated view state and carries no
// advancement side effects.
func finalizeNextView(root string, governedChange *model.Change, view nextView) (nextView, error) {
	applyReadyAdvanceDiagnostics(root, governedChange, &view)
	applyHostCapabilityContractToNext(&view)
	refreshOverallReadinessFreshnessForNext(&view)
	view.ConfirmationRequirement = deriveConfirmationRequirement(view)
	view.CurrentActionKind = view.ConfirmationRequirement.NextActionKind
	view.CurrentActionCommand = view.ConfirmationRequirement.NextCommand
	view.Recovery = model.BuildRecovery(view.Blockers)
	return view, nil
}

// advanceIfReadyAuto attempts state advancement unless in preview mode. When
// auto is true, the engine auto-confirms a pending preset upgrade-only; every
// evidence gate and guardrail control still blocks as in the non-auto path.
// Callers force auto false in preview so a query never mutates state.
func advanceIfReadyAuto(root string, ref changeRef, preview bool, skipAutoPass bool, command string, auto bool) (progression.AdvanceSummary, error) {
	if preview {
		return progression.AdvanceSummary{Action: "query"}, nil
	}
	command = strings.TrimSpace(command)
	if command == "" {
		command = "run"
	}

	opts := []progression.AdvanceOptions{{
		SkipAutoPass: skipAutoPass,
		Command:      command,
		Auto:         auto,
	}}
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

func applyReadyAdvanceDiagnostics(root string, change *model.Change, view *nextView) {
	if change == nil || view == nil || view.NextSkill != nil {
		return
	}
	if !hasReasonCode(view.Blockers, "no_skill_required") ||
		hasReasonCode(view.Blockers, "run_slipway_run_to_advance") ||
		hasNonPacingBlocker(*view) {
		return
	}
	canAdvance, blockers := noSkillStateAdvanceReadiness(root, *change)
	if !canAdvance || len(blockers) > 0 {
		return
	}
	// This routine run boundary must not co-occur with non-pacing blockers.
	// deriveConfirmationRequirement treats it as command_required before the
	// generic blocker check, so run/stage auto safety relies on emitters keeping
	// this advertisement purely routine.
	view.Blockers = model.NormalizeReasonCodes(append(
		view.Blockers,
		model.NewReasonCode("run_slipway_run_to_advance", string(view.CurrentState)),
	))
}

func projectDoneReadyForReadOnlyQuery(root string, change *model.Change, advanced *progression.AdvanceSummary) error {
	if change == nil || advanced == nil || advanced.Action != "query" || change.CurrentState != model.StateS3Review {
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
		FromState: model.StateS3Review,
		Reason:    "governance_gates_passed",
		Message:   "Governance gates passed; run `slipway done` to finalize.",
		Blockers:  []model.ReasonCode{model.NewReasonCode(reasonRunSlipwayDoneToFinalize, "")},
	}
	return nil
}

func advisoryDoneReadyWarnings(root string, ref changeRef, governedChange *model.Change, execCtx *executionContext, view nextView) ([]string, error) {
	if governedChange == nil || view.CurrentState != model.StateS3Review {
		return nil, nil
	}

	change := *governedChange
	var latestRunVersion int
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
	)
	if err != nil {
		return nil, wrapRequiredSkillsEvaluationError("evaluate ship-verification evidence", ref.Slug, err)
	}
	// ship-verification is the single always-required terminal S3 gate. A
	// done_ready projection already implies it passed; if its passing record is
	// present there is nothing to advise. Its absence is owned as a hard ship-gate
	// blocker by EvaluateShipAuthority, not surfaced as an advisory here.
	if _, ok := passingSkills[progression.SkillShipVerification]; ok {
		return nil, nil
	}
	return []string{
		"ship_gate_blocked:required_skill_missing:ship-verification",
	}, nil
}

// checkPresetPendingEarlyReturnWithReadContext loads the change minimally and,
// if preset confirmation is pending, populates the view with identity and
// preset fields only — no artifact reconciliation, no SaveChange, no
// artifact_status. Returns (true, nil) when the early return was taken.
func checkPresetPendingEarlyReturnWithReadContext(readCtx *stateReadContext, ref changeRef, view *nextView) (bool, error) {
	root := readCtx.root
	change, err := readCtx.loadChange(ref.Slug)
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
	// Auto softens explicitly allowlisted pure-pacing confirmation boundaries into
	// a standing-authorization continuation, but only when the change is NOT in a
	// guardrail/sensitive domain. Preset, unclassified skill, and evidence-gate
	// boundaries keep their hard_stop even under auto.
	autoSoftens := view.auto &&
		!isGuardrailSensitive(view.GuardrailDomain) &&
		!viewRequiresManualAutoBoundary(view)
	switch {
	case view.PresetConfirmationPending || hasReasonCode(view.Blockers, "preset_confirmation_required"):
		return confirmationHardStop("preset_confirmation_required")
	case hasReasonCode(view.Blockers, "run_slipway_done_to_finalize"):
		return confirmationCommandRequired("run_slipway_done_to_finalize")
	case hasReasonCode(view.Blockers, "run_slipway_run_to_advance"):
		return confirmationCommandRequired("run_slipway_run_to_advance")
	case hasReasonCode(view.Blockers, "s3_task_plan_drift_requires_inplace_convergence"):
		return confirmationCommandRequired("run_slipway_run_to_advance")
	case hasNonPacingBlocker(view):
		return confirmationGovernanceBlocked()
	case view.ReviewBatch != nil && len(view.ReviewBatch.Skills) > 0:
		if autoSoftens {
			return withSubagentDelegationPrerequisite(autoStandingAuthorization("review_batch"), view)
		}
		return withSubagentDelegationPrerequisite(confirmationHardStop("review_batch"), view)
	case view.NextSkill != nil:
		reason := "skill_handoff"
		if name := nextSkillHandoffName(view.NextSkill); name != "" {
			reason = "skill_handoff:" + name
		}
		if autoSoftens {
			return withSubagentDelegationPrerequisite(autoStandingAuthorization(reason), view)
		}
		return withSubagentDelegationPrerequisite(confirmationHardStop(reason), view)
	case hasReasonCode(view.Blockers, "no_skill_required") && !hasNonPacingBlocker(view):
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

func hasNonPacingBlocker(view nextView) bool {
	for _, reason := range view.Blockers {
		if confirmationBlockerCanRideHostHandoff(reason, view) {
			continue
		}
		switch strings.TrimSpace(reason.Code) {
		case "", "run_slipway_done_to_finalize", "run_slipway_run_to_advance", "no_skill_required":
			continue
		default:
			return true
		}
	}
	return false
}

func confirmationBlockerCanRideHostHandoff(reason model.ReasonCode, view nextView) bool {
	if progression.HostHandoffBlockerCanRide(reason) {
		return true
	}
	return viewHasReviewCompanionHandoff(view) &&
		progression.ReviewCompanionBlockerCanRide(reason)
}

func viewHasReviewCompanionHandoff(view nextView) bool {
	if view.ReviewBatch != nil && len(view.ReviewBatch.Skills) > 0 {
		return true
	}
	if view.NextSkill == nil {
		return false
	}
	return progression.ReviewCompanionSkillCanCarryBlockers(
		nextSkillHandoffName(view.NextSkill),
	)
}

func viewRequiresManualAutoBoundary(view nextView) bool {
	if view.ReviewBatch != nil {
		for _, skill := range view.ReviewBatch.Skills {
			if progression.SkillRequiresManualAutoBoundary(skill.Name) {
				return true
			}
		}
	}
	if view.NextSkill == nil {
		return false
	}
	return nextSkillRequiresManualAutoBoundary(view.NextSkill)
}

func nextSkillRequiresManualAutoBoundary(skill *nextSkillView) bool {
	if skill == nil {
		return false
	}
	return progression.SkillRequiresManualAutoBoundary(skill.BlockingName) ||
		progression.SkillRequiresManualAutoBoundary(skill.Name)
}

func nextSkillHandoffName(skill *nextSkillView) string {
	if skill == nil {
		return ""
	}
	if name := strings.TrimSpace(skill.BlockingName); name != "" {
		return name
	}
	return strings.TrimSpace(skill.Name)
}

func applyHostCapabilityContractToNext(view *nextView) {
	if view == nil {
		return
	}
	view.HostCapabilities = hostCapabilityViewsForSkills(selectedHostCapabilitySkillNamesForNext(*view))
	for _, req := range view.HostCapabilities {
		if code := hostCapabilityBlockerCode(req); code != "" {
			view.Blockers = appendReasonCodes(
				view.Blockers,
				[]model.ReasonCode{model.NewReasonCode(code, req.SkillName+":"+req.Capability)},
			)
		}
	}
}

func applyHostCapabilityContractToValidate(view *validateView, skills []string) {
	if view == nil {
		return
	}
	view.HostCapabilities = hostCapabilityViewsForSkills(skills)
	for _, req := range view.HostCapabilities {
		if code := hostCapabilityBlockerCode(req); code != "" {
			view.Blockers = appendReasonCodes(
				view.Blockers,
				[]model.ReasonCode{model.NewReasonCode(code, req.SkillName+":"+req.Capability)},
			)
		}
	}
	view.CanAdvance = len(view.Blockers) == 0
}

// hostCapabilityBlockerCode reports which blocker, if any, a required
// host-capability state emits. An available capability or an explicitly selected
// fallback emits nothing (the available path is unchanged). When the host has
// not affirmatively declared the capability available, the code distinguishes:
//   - "unavailable" (the host declared other capabilities but not this one) is a
//     first-class host_capability_unavailable blocker that fails closed.
//   - "unknown" (the host declared nothing) emits the continuable
//     subagent_dispatch_authorization_required blocker, which rides the handoff
//     so the next_action can name the delegation prerequisite plus the named
//     fallback without a silent dead-end or a silent bypass.
func hostCapabilityBlockerCode(req hostCapabilityView) string {
	if !req.Required || req.FallbackSelected {
		return ""
	}
	switch req.Availability {
	case "unavailable":
		return "host_capability_unavailable"
	case "unknown":
		return "subagent_dispatch_authorization_required"
	default: // "available"
		return ""
	}
}

func selectedHostCapabilitySkillNamesForNext(view nextView) []string {
	var names []string
	if view.ReviewBatch != nil {
		for _, skill := range view.ReviewBatch.Skills {
			names = append(names, skill.Name)
		}
	}
	if view.NextSkill != nil {
		names = append(names, nextSkillHandoffName(view.NextSkill))
	}
	return names
}

func selectedHostCapabilitySkillNamesForValidate(
	change model.Change,
	readiness progression.GovernanceReadiness,
	actionable *actionableNextSkillView,
) []string {
	if change.CurrentState == model.StateS3Review {
		if pending := pendingSelectedReviewSkills(change, readiness); len(pending) > 0 {
			return pending
		}
		// The selected review peers have converged, but the terminal
		// ship-verification gate may still owe fresh evidence. next/run already
		// resolve ship-verification as the S3 ship authority via
		// nextS3ShipAuthoritySkill once the peers pass; validate must surface the
		// same skill so its host-capability contract (the subagent-delegation
		// prerequisite or a fail-closed unavailable stop) is named for the terminal
		// gate too, rather than dead-ending after the peers pass.
		if shipSkill := nextS3ShipAuthoritySkill(readiness.PassingSkills, readiness.Blockers); shipSkill != "" {
			return []string{shipSkill}
		}
	}
	if actionable == nil {
		return nil
	}
	if name := strings.TrimSpace(actionable.BlockingName); name != "" {
		return []string{name}
	}
	return []string{strings.TrimSpace(actionable.Name)}
}

func hostCapabilityViewsForSkills(skillNames []string) []hostCapabilityView {
	if len(skillNames) == 0 {
		return nil
	}
	hostCapabilities := splitCapabilityEnv(os.Getenv("SLIPWAY_HOST_CAPABILITIES"))
	fallbacks := splitCapabilityEnv(os.Getenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS"))
	seen := map[string]bool{}
	var views []hostCapabilityView
	for _, skillName := range skillNames {
		skillName = strings.TrimSpace(skillName)
		if skillName == "" || seen[skillName] {
			continue
		}
		seen[skillName] = true
		req := capability.ResolveHostCapabilityRequirement(skillName, capability.Signals{
			HostCapabilities: hostCapabilities,
			Fallbacks:        fallbacks,
		})
		if req == nil {
			continue
		}
		views = append(views, hostCapabilityView{
			SkillName:           req.SkillID,
			Capability:          req.Capability,
			Required:            req.Required,
			Availability:        req.Availability,
			FallbackSelected:    req.FallbackSelected,
			FallbackMode:        req.FallbackMode,
			EvidenceRequirement: req.EvidenceRequirement,
			Remediation:         req.Remediation,
		})
	}
	return views
}

func splitCapabilityEnv(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\t', '\n', '\r':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func confirmationHardStop(reason string) confirmationRequirement {
	reason = strings.TrimSpace(reason)
	return confirmationRequirement{
		Required:                     true,
		Boundary:                     "hard_stop",
		FreshConfirmationRequired:    true,
		PriorAuthorizationSufficient: false,
		Reason:                       reason,
		NextAction:                   hardStopNextAction(reason),
		NextActionKind:               hardStopNextActionKind(reason),
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

func confirmationGovernanceBlocked() confirmationRequirement {
	return confirmationRequirement{
		Required:                     true,
		Boundary:                     "hard_stop",
		FreshConfirmationRequired:    true,
		PriorAuthorizationSufficient: false,
		Reason:                       "blocked_by_governance",
		NextAction:                   "resolve governance blockers before continuing",
		NextActionKind:               "blocker_resolution",
	}
}

// autoStandingAuthorization downgrades a pure-pacing host confirmation
// (review_batch or a non-sensitive skill_handoff) to a standing-authorization
// continuation under auto. It is NOT a hard stop and reports prior authorization
// as sufficient, but it preserves the same next action (run the review batch or
// the named skill, then record evidence). It never weakens an evidence gate: the
// recorded evidence is still required, only the manual confirmation pause is
// lifted. Guardrail/sensitive boundaries never reach this path.
func autoStandingAuthorization(reason string) confirmationRequirement {
	reason = strings.TrimSpace(reason)
	return confirmationRequirement{
		Required:                     false,
		Boundary:                     "evidence_continuation",
		FreshConfirmationRequired:    false,
		PriorAuthorizationSufficient: true,
		Reason:                       reason,
		NextAction:                   hardStopNextAction(reason),
		NextActionKind:               hardStopNextActionKind(reason),
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

// withSubagentDelegationPrerequisite enriches a review_batch or skill_handoff
// confirmation next_action when a subagent_dispatch_authorization_required
// blocker is riding it (the "unknown" host-capability state). It names the host
// subagent-delegation prerequisite and the explicit named-fallback path so the
// host is never left to silently infer stop / ask / violate, and never silently
// bypasses the gate. Hosts that declare subagent available carry no such blocker
// and are left unchanged.
func withSubagentDelegationPrerequisite(req confirmationRequirement, view nextView) confirmationRequirement {
	if !hasReasonCode(view.Blockers, "subagent_dispatch_authorization_required") {
		return req
	}
	req.NextAction = strings.TrimRight(strings.TrimSpace(req.NextAction), ".") +
		". Host subagent delegation is a prerequisite the host has not declared available: authorize subagent dispatch, " +
		"or explicitly select a named degraded fallback (e.g. same_context_degraded) and record fresh evidence for each listed skill " +
		"(see host_capabilities[] and recovery for the per-skill fallback names). For S3 review skills, record exactly one " +
		"context_origin:stage=review=<handle> reference and an additional fallback:<mode> reference when degraded"
	return req
}

func hardStopNextAction(reason string) string {
	if skillName, ok := strings.CutPrefix(reason, "skill_handoff:"); ok && strings.TrimSpace(skillName) != "" {
		skillName = strings.TrimSpace(skillName)
		if skillName == progression.SkillIntakeClarification {
			// #357: the intake approved-summary is a fresh hard gate by design.
			// A prior broad "continue" authorization does not substitute for
			// explicit approval of this intent summary, so the next_action must
			// state that policy rather than reading as an ordinary skill rerun.
			return "review and approve the intake Approved Summary, then run governance skill " +
				skillName + " and record evidence — the intake approved-summary is a FRESH HARD GATE BY DESIGN; " +
				"a prior broad \"continue\" authorization does not substitute for explicit approval of this intent summary"
		}
		return "run governance skill " + skillName + " and record evidence"
	}
	switch reason {
	case "preset_confirmation_required":
		return "confirm workflow preset before continuing"
	case "review_batch":
		return "run the parallel S3 review batch and record evidence for each listed skill"
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

// hardStopNextActionKind returns the machine-readable action kind for a
// hard-stop boundary.
func hardStopNextActionKind(reason string) string {
	switch {
	case reason == "preset_confirmation_required":
		return "preset_confirmation"
	case reason == "review_batch":
		return "review_batch"
	case strings.HasPrefix(reason, "skill_handoff"):
		return "skill_handoff"
	default:
		return "confirmation"
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
	writer := newFormatWriter(w)

	writer.Writef("Change: %s (%s)\n", view.Slug, workflowStateLabel(view.CurrentState, view.IntakeSubStep, view.PlanSubStep))
	writer.Writef("Phase: %s | Mode: %s | Status: %s\n", view.Phase, view.ExecutionMode, view.LifecycleStatus)
	if view.QualityMode != "" {
		writer.Writef("Quality: %s | Discovery Required: %t\n", view.QualityMode, view.NeedsDiscovery)
	}
	for _, line := range renderWorkflowPresetLines(workflowPresetView{
		WorkflowPreset:            view.WorkflowPreset,
		SuggestedWorkflowPreset:   view.SuggestedWorkflowPreset,
		EffectiveWorkflowPreset:   view.EffectiveWorkflowPreset,
		PresetConfirmationPending: view.PresetConfirmationPending,
		PresetUpgradeReasons:      view.PresetUpgradeReasons,
		GovernanceForecast:        view.GovernanceForecast,
	}) {
		writer.Writef("%s\n", line)
	}
	if view.PlanningNote != "" {
		writer.Writef("Planning Note: %s\n", view.PlanningNote)
	}

	if view.InputContext.Description != "" {
		writer.Writef("Description: %s\n", view.InputContext.Description)
	}
	if view.InputContext.Slug != "" {
		writer.Writef("Slug: %s\n", view.InputContext.Slug)
	}
	if view.Advanced != nil {
		if view.Advanced.Action == "advanced" || view.Advanced.Action == "done_ready" {
			writer.Writef("\nAdvanced: %s -> %s (%s)\n", view.Advanced.FromState, view.Advanced.ToState, view.Advanced.Message)
		}
		if len(view.Advanced.SideEffects) > 0 {
			heading := "Side Effects:"
			if view.Advanced.RecoveryOnly {
				heading = "Recovery Side Effects:"
			}
			writer.Writef("%s\n", heading)
			for _, effect := range view.Advanced.SideEffects {
				if effect.Detail != "" {
					writer.Writef("  - %s: %s\n", effect.Kind, effect.Detail)
				} else {
					writer.Writef("  - %s\n", effect.Kind)
				}
			}
		}
		if len(view.Advanced.AutoPassedStates) > 0 {
			writer.Writef("Auto-Passed:\n")
			for _, state := range view.Advanced.AutoPassedStates {
				writer.Writef("  - %s (%s)\n", state.State, state.Reason)
			}
		}
	}

	if view.GovernanceSignals != nil {
		writer.Writef("\nDetected Signals:\n")
		if len(view.GovernanceSignals.Domains) > 0 {
			writer.Writef("  Domains:       [%s]\n", strings.Join(view.GovernanceSignals.Domains, ", "))
		}
		writer.Writef("  Blast Radius:  %s\n", view.GovernanceSignals.BlastRadius)
	}
	if len(view.ActiveControls) > 0 {
		writer.Writef("\nActive Controls:\n")
		for _, ctrl := range view.ActiveControls {
			writer.Writef("  - %s (%s / %s)\n", ctrl.ControlID, ctrl.Mode, ctrl.Scope)
		}
	}
	if len(view.RequiredActions) > 0 {
		writer.Writef("\nRequired Actions:\n")
		for _, action := range view.RequiredActions {
			mark := " "
			if action.Satisfied {
				mark = "x"
			}
			writer.Writef("  [%s] %s: %s\n", mark, action.ControlID, action.Description)
		}
	}

	writer.Writef("\n")

	if view.NextSkill != nil {
		hydrateWriter := newFormatWriter(w)
		writer.Writef("Next Skill: %s\n", view.NextSkill.Name)
		writer.Writef("  Verification Dir: %s\n", view.NextSkill.VerificationDir)
		writer.Writef("  Evidence State: %s\n", view.NextSkill.State)
		if len(view.NextSkill.SelectedReviewSkills) > 0 {
			writer.Writef("  Selected Review Skills: %s\n", strings.Join(view.NextSkill.SelectedReviewSkills, ", "))
		}
		if len(view.NextSkill.RequiredTokens) > 0 {
			writer.Writef("  Required Tokens: %s\n", strings.Join(view.NextSkill.RequiredTokens, ", "))
		}
		if view.NextSkill.ReviewContext != nil {
			if len(view.NextSkill.ReviewContext.RequiredArtifactLayers) > 0 {
				writer.Writef("  Required Artifact Layers: %s\n", strings.Join(view.NextSkill.ReviewContext.RequiredArtifactLayers, ", "))
			}
			if len(view.NextSkill.ReviewContext.RequiredImplementationLayers) > 0 {
				writer.Writef("  Required Implementation Layers: %s\n", strings.Join(view.NextSkill.ReviewContext.RequiredImplementationLayers, ", "))
			}
		}
		if view.ReviewBatch != nil && len(view.ReviewBatch.Skills) > 0 {
			names := make([]string, 0, len(view.ReviewBatch.Skills))
			for _, batchSkill := range view.ReviewBatch.Skills {
				names = append(names, batchSkill.Name)
			}
			writer.Writef("  Review Batch: %s\n", strings.Join(names, ", "))
		}

		if len(view.NextSkill.TechniqueHints) > 0 {
			writer.Writef("\nTechnique Hints:\n")
			for _, hint := range view.NextSkill.TechniqueHints {
				writer.Writef("  - %s: %s\n", hint.Name, hint.Reason)
				writeHydrateLine(hydrateWriter, "    ", hint.HydrateReferences)
			}
		}

		if len(view.Warnings) > 0 {
			writer.Writef("\nWarnings:\n")
			for _, warning := range view.Warnings {
				writer.Writef("  - %s\n", warning)
			}
		}

		if len(view.Blockers) > 0 {
			writer.Writef("\nBlockers:\n")
			for _, b := range renderReasonCodeLines(view.Blockers) {
				writer.Writef("  - %s\n", b)
			}
		}
	} else {
		if len(view.Warnings) > 0 {
			for _, warning := range view.Warnings {
				writer.Writef("  %s\n", warning)
			}
		}
		if len(view.Blockers) > 0 {
			for _, b := range renderReasonCodeLines(view.Blockers) {
				writer.Writef("  %s\n", b)
			}
		}
	}

	return writer.Err()
}
