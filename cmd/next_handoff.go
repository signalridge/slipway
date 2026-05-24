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
	Warnings         []string                `json:"warnings,omitempty"`
	Confirmation     bool                    `json:"confirmation_required"`
}

type nextSkillHandoff struct {
	Name             string            `json:"name"`
	VerificationDir  string            `json:"verification_dir"`
	State            string            `json:"state"`
	SkillConstraints *skillConstraints `json:"skill_constraints,omitempty"`
	TechniqueHints   []techniqueHint   `json:"technique_hints,omitempty"`
}

type contextBudgetHandoff struct {
	GuardAction      string  `json:"guard_action"`
	RemainingPercent float64 `json:"remaining_percent"`
}

type nextHandoffContext struct {
	WorkspaceRoot    string            `json:"workspace_root"`
	ArtifactBundle   string            `json:"artifact_bundle,omitempty"`
	ChangeAuthority  string            `json:"change_authority,omitempty"`
	CodebaseMapDir   string            `json:"codebase_map_dir,omitempty"`
	CodebaseMapDocs  map[string]string `json:"codebase_map_docs,omitempty"`
	ResumeCheckpoint *resumeCheckpoint `json:"resume_checkpoint,omitempty"`
}

func buildNextHandoffSourceView(root string, ref changeRef, resumeResponse string, preview bool, autoSkipEvidence bool, skipAutoPass bool, quickMode ...bool) (nextView, error) {
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
		if err := consumeNextCheckpoint(root, governedChange, &view); err != nil {
			return nextView{}, err
		}
		return view, nil
	}
	view.Phase = model.PhaseFor(view.CurrentState)

	var nextSkillEvidence map[string]model.VerificationRecord
	if governedChange != nil {
		readiness, err := progression.EvaluateGovernanceReadiness(
			root,
			*governedChange,
			progression.GovernanceReadinessOptions{
				IncludeGateEvaluations: true,
			},
		)
		if err != nil {
			return nextView{}, wrapGovernanceReadinessError("evaluate next skill evidence", ref.Slug, err)
		}
		view.Warnings = append(view.Warnings, readiness.Diagnostics...)
		view.Blockers = appendReasonCodes(view.Blockers, readiness.Blockers)
		nextSkillEvidence = readiness.PassingSkills
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

	if err := assembleSkillViewWithOptions(root, &view, ref, advanced, governedChange, execCtx, nextSkillEvidence, autoSkipEvidence, handoffSkillViewOptions); err != nil {
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
			VerificationDir:  view.NextSkill.VerificationDir,
			State:            view.NextSkill.State,
			SkillConstraints: cloneSkillConstraints(view.NextSkill.SkillConstraints),
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
			WorkspaceRoot:    view.InputContext.WorkspaceRoot,
			ArtifactBundle:   view.InputContext.ArtifactBundle,
			ChangeAuthority:  changeAuthority,
			CodebaseMapDir:   view.InputContext.CodebaseMapDir,
			CodebaseMapDocs:  view.InputContext.CodebaseMapDocs,
			ResumeCheckpoint: view.InputContext.ResumeCheckpoint,
		},
		AutoPassEligible: append([]model.AutoPassedState(nil), view.AutoPassEligible...),
		Blockers:         view.Blockers,
		Warnings:         view.Warnings,
		Confirmation:     view.Confirmation,
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
		GuardrailDomain:  in.GuardrailDomain,
		MitigationTarget: in.MitigationTarget,
		RunSummaryBound:  in.RunSummaryBound,
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
