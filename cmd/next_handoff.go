package cmd

import (
	"github.com/signalridge/slipway/internal/model"
)

type nextHandoffView struct {
	Command                     string                  `json:"command,omitempty"`
	DelegatedTo                 string                  `json:"delegated_to,omitempty"`
	Slug                        string                  `json:"slug"`
	Phase                       model.UserPhase         `json:"phase"`
	ExecutionMode               string                  `json:"execution_mode,omitempty"`
	CurrentState                model.WorkflowState     `json:"current_state"`
	LifecycleStatus             string                  `json:"lifecycle_status,omitempty"`
	InvocationRoute             *invocationRouteView    `json:"invocation_route,omitempty"`
	HostCapabilities            []hostCapabilityView    `json:"host_capabilities,omitempty"`
	NextSkill                   *nextSkillHandoff       `json:"next_skill"`
	ReviewBatch                 *reviewBatchView        `json:"review_batch,omitempty"`
	InputContext                nextHandoffContext      `json:"input_context"`
	AutoPassEligible            []model.AutoPassedState `json:"auto_pass_eligible,omitempty"`
	EvidenceFreshness           string                  `json:"evidence_freshness,omitempty"`
	ExecutionEvidenceFreshness  string                  `json:"execution_evidence_freshness,omitempty"`
	GovernanceEvidenceFreshness string                  `json:"governance_evidence_freshness,omitempty"`
	OverallReadinessFreshness   string                  `json:"overall_readiness_freshness,omitempty"`
	Blockers                    []model.ReasonCode      `json:"blockers"`
	Recovery                    *model.RecoverySummary  `json:"recovery,omitempty"`
	Warnings                    []string                `json:"warnings,omitempty"`
	Confirmation                confirmationRequirement `json:"confirmation_requirement"`
}

type nextSkillHandoff struct {
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

type nextHandoffContext struct {
	WorkspaceRoot        string                  `json:"workspace_root"`
	ArtifactBundle       string                  `json:"artifact_bundle,omitempty"`
	CodebaseMapDir       string                  `json:"codebase_map_dir,omitempty"`
	CodebaseMapDocs      map[string]string       `json:"codebase_map_docs,omitempty"`
	CodebaseMapStatus    string                  `json:"codebase_map_status,omitempty"`
	CodebaseMapDocStates map[string]string       `json:"codebase_map_doc_states,omitempty"`
	WavePlan             *wavePlanView           `json:"wave_plan,omitempty"`
	ExecutionResume      *executionResumeContext `json:"execution_resume,omitempty"`
}

func buildNextHandoffView(view nextView) nextHandoffView {
	var nextSkill *nextSkillHandoff
	if view.NextSkill != nil {
		nextSkill = &nextSkillHandoff{
			Name:                 view.NextSkill.Name,
			DisplayName:          view.NextSkill.DisplayName,
			BlockingName:         view.NextSkill.BlockingName,
			ResolutionReason:     view.NextSkill.ResolutionReason,
			Subagent:             cloneSubagentDirective(view.NextSkill.Subagent),
			SelectedReviewSkills: append([]string(nil), view.NextSkill.SelectedReviewSkills...),
			RequiredTokens:       append([]string(nil), view.NextSkill.RequiredTokens...),
			VerificationDir:      view.NextSkill.VerificationDir,
			State:                view.NextSkill.State,
			SkillConstraints:     cloneSkillConstraints(view.NextSkill.SkillConstraints),
			ReviewContext:        cloneReviewContext(view.NextSkill.ReviewContext),
			TechniqueHints:       cloneTechniqueHints(view.NextSkill.TechniqueHints),
		}
	}
	return nextHandoffView{
		Command:         view.Command,
		DelegatedTo:     view.DelegatedTo,
		Slug:            view.Slug,
		Phase:           view.Phase,
		ExecutionMode:   view.ExecutionMode,
		CurrentState:    view.CurrentState,
		LifecycleStatus: view.LifecycleStatus,
		InvocationRoute: view.InvocationRoute,
		HostCapabilities: append(
			[]hostCapabilityView(nil),
			view.HostCapabilities...,
		),
		NextSkill:   nextSkill,
		ReviewBatch: cloneReviewBatch(view.ReviewBatch),
		InputContext: nextHandoffContext{
			WorkspaceRoot:        view.InputContext.WorkspaceRoot,
			ArtifactBundle:       view.InputContext.ArtifactBundle,
			CodebaseMapDir:       view.InputContext.CodebaseMapDir,
			CodebaseMapDocs:      view.InputContext.CodebaseMapDocs,
			CodebaseMapStatus:    view.InputContext.CodebaseMapStatus,
			CodebaseMapDocStates: view.InputContext.CodebaseMapDocStates,
			WavePlan:             view.InputContext.WavePlan,
			ExecutionResume:      view.InputContext.ExecutionResume,
		},
		AutoPassEligible:            append([]model.AutoPassedState(nil), view.AutoPassEligible...),
		EvidenceFreshness:           view.EvidenceFreshness,
		ExecutionEvidenceFreshness:  view.ExecutionEvidenceFreshness,
		GovernanceEvidenceFreshness: view.GovernanceEvidenceFreshness,
		OverallReadinessFreshness:   view.OverallReadinessFreshness,
		Blockers:                    view.Blockers,
		Recovery:                    model.BuildRecovery(view.Blockers),
		Warnings:                    view.Warnings,
		Confirmation:                view.ConfirmationRequirement,
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

func cloneReviewBatch(in *reviewBatchView) *reviewBatchView {
	if in == nil {
		return nil
	}
	out := &reviewBatchView{
		Mode:            in.Mode,
		Subagent:        cloneSubagentDirective(in.Subagent),
		VerificationDir: in.VerificationDir,
		State:           in.State,
	}
	if len(in.Skills) > 0 {
		out.Skills = make([]reviewBatchSkillView, 0, len(in.Skills))
		for _, batchSkill := range in.Skills {
			out.Skills = append(out.Skills, reviewBatchSkillView{
				Name:             batchSkill.Name,
				RequiredTokens:   append([]string(nil), batchSkill.RequiredTokens...),
				ReviewContext:    cloneReviewContext(batchSkill.ReviewContext),
				TechniqueHints:   cloneTechniqueHints(batchSkill.TechniqueHints),
				SkillConstraints: cloneSkillConstraints(batchSkill.SkillConstraints),
			})
		}
	}
	return out
}
