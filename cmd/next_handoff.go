package cmd

import (
	"path/filepath"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

type nextHandoffView struct {
	Slug             string                  `json:"slug"`
	Phase            model.UserPhase         `json:"phase"`
	ExecutionMode    string                  `json:"execution_mode,omitempty"`
	CurrentState     model.WorkflowState     `json:"current_state"`
	LifecycleStatus  string                  `json:"lifecycle_status,omitempty"`
	NextSkill        *nextSkillHandoff       `json:"next_skill"`
	ContextBudget    *contextBudgetHandoff   `json:"context_budget,omitempty"`
	InputContext     nextHandoffContext      `json:"input_context"`
	AutoPassEligible []model.AutoPassedState `json:"auto_pass_eligible,omitempty"`
	Blockers         []model.ReasonCode      `json:"blockers"`
	Recovery         *model.RecoverySummary  `json:"recovery,omitempty"`
	Warnings         []string                `json:"warnings,omitempty"`
	Confirmation     confirmationRequirement `json:"confirmation_requirement"`
}

type nextSkillHandoff struct {
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

type contextBudgetHandoff struct {
	GuardAction      string  `json:"guard_action"`
	RemainingPercent float64 `json:"remaining_percent"`
}

type nextHandoffContext struct {
	WorkspaceRoot        string            `json:"workspace_root"`
	ArtifactBundle       string            `json:"artifact_bundle,omitempty"`
	ChangeAuthority      string            `json:"change_authority,omitempty"`
	CodebaseMapDir       string            `json:"codebase_map_dir,omitempty"`
	CodebaseMapDocs      map[string]string `json:"codebase_map_docs,omitempty"`
	CodebaseMapStatus    string            `json:"codebase_map_status,omitempty"`
	CodebaseMapDocStates map[string]string `json:"codebase_map_doc_states,omitempty"`
	ResumeCheckpoint     *resumeCheckpoint `json:"resume_checkpoint,omitempty"`
}

func buildNextHandoffSourceView(root string, ref changeRef, resumeResponse string, preview bool, autoSkipEvidence bool, skipAutoPass bool) (nextView, error) {
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

	if pending, err := checkPresetPendingEarlyReturn(root, ref, &view); err != nil {
		return nextView{}, err
	} else if pending {
		return view, nil
	}

	governedChange, execCtx, err := buildNextHandoffContextByMode(root, &view, ref, resumeResponse, preview)
	if err != nil {
		return nextView{}, err
	}
	finalize := func() (nextView, error) {
		view.ConfirmationRequirement = deriveConfirmationRequirement(view)
		if err := consumeNextCheckpoint(root, governedChange, &view); err != nil {
			return nextView{}, err
		}
		return view, nil
	}
	view.Phase = model.PhaseFor(view.CurrentState)

	var nextSkillEvidence map[string]model.VerificationRecord
	var nextSkillArtifactProjection *progression.ArtifactProjection
	if governedChange != nil {
		readiness, err := progression.EvaluateGovernanceReadiness(
			root,
			*governedChange,
			progression.GovernanceReadinessOptions{
				IncludeGateEvaluations:    true,
				IncludeArtifactProjection: true,
			},
		)
		if err != nil {
			return nextView{}, wrapGovernanceReadinessError("evaluate next skill evidence", ref.Slug, err)
		}
		view.Warnings = append(view.Warnings, readiness.Diagnostics...)
		view.Blockers = appendReasonCodes(view.Blockers, readiness.Blockers)
		// The compact handoff source view deliberately omits freshness
		// diagnostics: nextHandoffView has no such field, and the diagnostic
		// surfaces are built only by the --diagnostics next view. Keeping this
		// path free of them preserves the narrow handoff contract.
		nextSkillEvidence = readiness.PassingSkills
		nextSkillArtifactProjection = readiness.ArtifactProjection
		// A parsed decision.md selection is only a locked skill_constraint once the
		// G_plan gate is approved; otherwise it is carried as pending so the
		// handoff payload stays consistent with the full next view (issue #140).
		view.planLocked = planLockedFromGates(readiness)
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

	if err := assembleSkillViewWithOptions(root, &view, ref, advanced, governedChange, execCtx, nextSkillEvidence, nextSkillArtifactProjection, autoSkipEvidence, handoffSkillViewOptions); err != nil {
		return nextView{}, err
	}
	return finalize()
}

func buildNextHandoffContextByMode(root string, view *nextView, ref changeRef, resumeResponse string, preview bool) (*model.Change, *executionContext, error) {
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return nil, nil, err
	}

	view.CurrentState = change.CurrentState
	view.PlanSubStep = change.PlanSubStep
	view.PlanningNote = planningNote(change.CurrentState, change.PlanSubStep)
	view.LifecycleStatus = string(change.Status)
	view.ExecutionMode = governedExecutionMode
	profile := buildChangeProfileView(change)
	view.QualityMode = profile.QualityMode
	view.WorkflowProfile = profile.WorkflowProfile
	view.NeedsDiscovery = profile.NeedsDiscovery
	view.InputContext.Description = change.Description
	view.InputContext.Slug = change.Slug

	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return nil, nil, err
	}
	view.InputContext.WorkspaceRoot = paths.WorkspaceRoot
	view.InputContext.ArtifactBundle = state.DisplayPath(root, paths.GovernedBundleDir)
	view.InputContext.CodebaseMapDir = state.DisplayPath(paths.WorkspaceRoot, paths.CodebaseMapDir)
	view.InputContext.CodebaseMapDocs = artifact.CodebaseMapDisplayDocs(paths.WorkspaceRoot, paths.CodebaseMapDir)
	view.InputContext.CodebaseMapStatus, view.InputContext.CodebaseMapDocStates = codebaseMapStatusForContext(paths.WorkspaceRoot, view.InputContext.CodebaseMapDocs)
	view.InputContext.HandoffContext = &handoffContextView{
		ChangeAuthority: state.DisplayPath(root, filepath.Join(paths.GovernedBundleDir, "change.yaml")),
	}

	execCtx, err := loadExecutionContext(root, change)
	if err != nil {
		return nil, nil, err
	}
	if err := buildResumeCheckpoint(root, &change, execCtx, view, resumeResponse, preview); err != nil {
		return nil, nil, err
	}
	return &change, &execCtx, nil
}

func buildNextHandoffView(view nextView) nextHandoffView {
	var nextSkill *nextSkillHandoff
	if view.NextSkill != nil {
		nextSkill = &nextSkillHandoff{
			Name:             view.NextSkill.Name,
			DisplayName:      view.NextSkill.DisplayName,
			BlockingName:     view.NextSkill.BlockingName,
			ResolutionReason: view.NextSkill.ResolutionReason,
			RequiredTokens:   append([]string(nil), view.NextSkill.RequiredTokens...),
			VerificationDir:  view.NextSkill.VerificationDir,
			State:            view.NextSkill.State,
			SkillConstraints: cloneSkillConstraints(view.NextSkill.SkillConstraints),
			ReviewContext:    cloneReviewContext(view.NextSkill.ReviewContext),
			TechniqueHints:   cloneTechniqueHints(view.NextSkill.TechniqueHints),
		}
	}
	budget := buildContextBudgetHandoff(view.ContextBudget)
	changeAuthority := ""
	if budget != nil && budget.GuardAction == "stop" && view.InputContext.HandoffContext != nil {
		changeAuthority = view.InputContext.HandoffContext.ChangeAuthority
	}
	return nextHandoffView{
		Slug:            view.Slug,
		Phase:           view.Phase,
		ExecutionMode:   view.ExecutionMode,
		CurrentState:    view.CurrentState,
		LifecycleStatus: view.LifecycleStatus,
		NextSkill:       nextSkill,
		ContextBudget:   budget,
		InputContext: nextHandoffContext{
			WorkspaceRoot:        view.InputContext.WorkspaceRoot,
			ArtifactBundle:       view.InputContext.ArtifactBundle,
			ChangeAuthority:      changeAuthority,
			CodebaseMapDir:       view.InputContext.CodebaseMapDir,
			CodebaseMapDocs:      view.InputContext.CodebaseMapDocs,
			CodebaseMapStatus:    view.InputContext.CodebaseMapStatus,
			CodebaseMapDocStates: view.InputContext.CodebaseMapDocStates,
			ResumeCheckpoint:     view.InputContext.ResumeCheckpoint,
		},
		AutoPassEligible: append([]model.AutoPassedState(nil), view.AutoPassEligible...),
		Blockers:         view.Blockers,
		Recovery:         model.BuildRecovery(view.Blockers),
		Warnings:         view.Warnings,
		Confirmation:     view.ConfirmationRequirement,
	}
}

func buildContextBudgetHandoff(budget *contextBudget) *contextBudgetHandoff {
	if budget == nil {
		return nil
	}
	switch budget.GuardAction {
	case "warn", "stop":
		return &contextBudgetHandoff{
			GuardAction:      budget.GuardAction,
			RemainingPercent: budget.RemainingPercent,
		}
	default:
		return nil
	}
}

func cloneSkillConstraints(in *skillConstraints) *skillConstraints {
	if in == nil {
		return nil
	}
	return &skillConstraints{
		LockedDecisions:  append([]string(nil), in.LockedDecisions...),
		PendingDecisions: append([]string(nil), in.PendingDecisions...),
		GuardrailDomain:  in.GuardrailDomain,
		MitigationTarget: in.MitigationTarget,
		RunSummaryBound:  in.RunSummaryBound,
	}
}

func cloneReviewContext(in *reviewContextView) *reviewContextView {
	if in == nil {
		return nil
	}
	return &reviewContextView{
		RequiredArtifactLayers:       append([]string(nil), in.RequiredArtifactLayers...),
		RequiredImplementationLayers: append([]string(nil), in.RequiredImplementationLayers...),
		OptionalLayers:               append([]string(nil), in.OptionalLayers...),
	}
}

func cloneTechniqueHints(in []techniqueHint) []techniqueHint {
	if len(in) == 0 {
		return nil
	}
	out := make([]techniqueHint, 0, len(in))
	for _, hint := range in {
		hint.HydrateReferences = append([]string(nil), hint.HydrateReferences...)
		out = append(out, hint)
	}
	return out
}
