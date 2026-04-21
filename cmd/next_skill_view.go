package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/engine/capability"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/toolgen"
)

func resolveWorkspaceSkillPaths(root, skillName string) (promptPath, resolvedToolID string, err error) {
	wsRoot := root
	if len(toolgen.DetectExistingTools(wsRoot)) == 0 {
		wsRoot = invocationWorkspaceRoot(root)
	}
	cfg, err := toolgen.ResolveWorkspaceTool(wsRoot)
	if err != nil {
		return "", "", err
	}
	promptPath = toolgen.SkillPath(cfg, skillName)
	if strings.TrimSpace(cfg.ID) == "" {
		return "", "", fmt.Errorf("resolved tool id is empty for skill %q", skillName)
	}
	return promptPath, cfg.ID, nil
}

// assembleSkillView resolves the next skill, builds the skill view with technique hints,
// review context, constraints, and context budget, then applies guards.
// governedChange is the already-loaded executable change from buildNextContextByMode.
// precomputedPassingSkills lets callers reuse skill evidence that was already
// loaded earlier in the same invocation instead of reading verification files
// again.
func assembleSkillView(
	root string,
	view *nextView,
	ref changeRef,
	advanced progression.AdvanceSummary,
	governedChange *model.Change,
	execCtx *executionContext,
	precomputedPassingSkills map[string]model.VerificationRecord,
	autoSkipEvidence bool,
) error {
	// Build a synthetic Change for skill resolution when no governed change exists.
	resolveChange := model.Change{CurrentState: view.CurrentState}
	if governedChange != nil {
		resolveChange = *governedChange
	}
	nextSkillName, nextState := progression.ResolveNextSkill(resolveChange)

	var evidenceMap map[string]model.VerificationRecord
	if governedChange != nil && nextSkillName != "" {
		if precomputedPassingSkills != nil {
			evidenceMap = precomputedPassingSkills
		} else {
			var evalErr error
			presetPolicy, policyErr := governance.ResolvePresetPolicy(root, *governedChange)
			if policyErr != nil {
				return policyErr
			}
			latestRunVersion := 0
			if execCtx != nil {
				latestRunVersion = execCtx.LatestRunVersion
			} else {
				var err error
				latestRunVersion, err = state.LatestRelevantExecutionRunVersion(root, *governedChange)
				if err != nil {
					return err
				}
			}
			planningSubSteps := readOnlyRequiredSkillInputs(*governedChange)
			evidenceMap, _, evalErr = progression.EvaluateRequiredSkillsForChange(
				root,
				*governedChange,
				view.CurrentState,
				latestRunVersion,
				presetPolicy.CloseoutRefreshRequired,
				planningSubSteps...,
			)
			if evalErr != nil {
				return wrapRequiredSkillsEvaluationError("evaluate next skill evidence", ref.Slug, evalErr)
			}
		}
	}

	if evidenceMap != nil {
		requiredSkillEvidence, err := buildRequiredSkillEvidence(root, *governedChange, view.CurrentState, execCtx, precomputedPassingSkills)
		if err != nil {
			return wrapRequiredSkillsEvaluationError("evaluate required skill evidence", ref.Slug, err)
		}
		view.SkillEvidence = requiredSkillEvidence
	}

	if autoSkipEvidence {
		if nextSkillName == progression.SkillSpecComplianceReview && evidenceMap != nil {
			if _, hasSpecReview := evidenceMap[progression.SkillSpecComplianceReview]; hasSpecReview {
				nextSkillName = progression.SkillCodeQualityReview
			}
		}

		if nextSkillName == progression.SkillGoalVerification && evidenceMap != nil {
			if _, hasGoalVerification := evidenceMap[progression.SkillGoalVerification]; hasGoalVerification {
				nextSkillName = progression.SkillFinalCloseout
			}
		}
	}

	if nextSkillName == "" {
		view.NextSkill = nil
		blockers := append([]model.ReasonCode(nil), view.Blockers...)
		if advanced.Action == "blocked" {
			blockers = append(blockers, advanced.Blockers...)
		}
		if len(blockers) == 0 {
			blockers = append(blockers, model.NewReasonCode("no_skill_required", string(view.CurrentState)))
		}
		view.Blockers = model.NormalizeReasonCodes(blockers)
		return nil
	}

	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return wrapRequiredSkillsEvaluationError("load governance registry", ref.Slug, err)
	}

	verificationDir := state.DisplayPath(root, filepath.Dir(state.VerificationFilePath(root, ref.Slug, nextSkillName)))
	promptPath, resolvedToolID, err := resolveWorkspaceSkillPaths(root, nextSkillName)
	if err != nil {
		return err
	}

	ns := &nextSkillView{
		Name:            nextSkillName,
		PromptPath:      promptPath,
		VerificationDir: verificationDir,
		State:           nextState,
		ResolvedToolID:  resolvedToolID,
	}

	if governedChange != nil {
		if model.WorkflowState(nextState) == model.StateS1Plan && progression.HasEmptyCodebaseMap(root, view.InputContext.CodebaseMapDocs) {
			ns.TechniqueHints = append(ns.TechniqueHints, techniqueHint{
				Name:   "slipway codebase-map",
				Reason: "No durable codebase-map documents found. Run `slipway codebase-map` to establish brownfield context before planning.",
			})
		}
	}

	// Auto capability resolver: attach B1 catalog-skill hints on top of the
	// kernel's host selection. Never changes the next skill chosen by
	// ResolveNextSkill; only enriches TechniqueHints.
	ns.TechniqueHints = appendCatalogHints(ns.TechniqueHints, nextSkillName, governedChange, view)

	if nextSkillName == progression.SkillSpecComplianceReview || nextSkillName == progression.SkillCodeQualityReview {
		var guardrailDomain string
		if governedChange != nil {
			guardrailDomain = governedChange.GuardrailDomain
		}
		ns.ReviewContext = buildReviewContext(guardrailDomain)
	}

	if def, ok := skill.LookupDefinitionInRegistry(registry, nextSkillName); ok {
		ns.SkillConstraints = buildSkillConstraints(root, def, governedChange)
	}

	view.NextSkill = ns
	view.ContextBudget = estimateContextBudget(root, ns, view.InputContext)
	view.Constraints = deriveAgentConstraints(registry, nextSkillName)

	if advanced.Action == "blocked" {
		view.Blockers = appendReasonCodes(view.Blockers, advanced.Blockers)
	}
	applyContextBudgetGuard(view)

	return nil
}

func buildRequiredSkillEvidence(
	root string,
	change model.Change,
	workflowState model.WorkflowState,
	execCtx *executionContext,
	precomputedPassingSkills map[string]model.VerificationRecord,
) ([]skillEvidenceEntry, error) {
	presetPolicy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return nil, err
	}
	latestRunVersion := 0
	if execCtx != nil {
		latestRunVersion = execCtx.LatestRunVersion
	} else {
		latestRunVersion, err = state.LatestRelevantExecutionRunVersion(root, change)
		if err != nil {
			return nil, err
		}
	}
	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return nil, err
	}
	var planSubSteps []model.PlanSubStep
	if workflowState == model.StateS1Plan && change.PlanSubStep != model.PlanSubStepNone {
		planSubSteps = []model.PlanSubStep{change.PlanSubStep}
	}
	required := skill.RequiredSkillsForStateWithRegistry(
		registry,
		change.NeedsDiscovery,
		workflowState,
		presetPolicy.CloseoutRefreshRequired,
		planSubSteps...,
	)
	evidence := make([]skillEvidenceEntry, 0, len(required))
	if precomputedPassingSkills != nil {
		for _, skillName := range required {
			entry := skillEvidenceEntry{
				SkillName: skillName,
				Status:    "missing",
			}
			if rec, ok := precomputedPassingSkills[skillName]; ok {
				entry.HasEvidence = true
				entry.Verdict = rec.Verdict
				entry.Status = "passing"
			}
			evidence = append(evidence, entry)
		}
		return evidence, nil
	}
	verifications, err := state.ListVerificationsForChange(root, change)
	if err != nil {
		return nil, err
	}
	for _, skillName := range required {
		entry := skillEvidenceEntry{
			SkillName: skillName,
			Status:    "missing",
		}
		rec, ok := verifications[skillName]
		if !ok {
			evidence = append(evidence, entry)
			continue
		}
		entry.HasEvidence = true
		entry.Verdict = rec.Verdict
		entry.Status = "not_ready"
		if def, ok := skill.LookupDefinitionInRegistry(registry, skillName); ok {
			switch {
			case def.RunSummaryBound && latestRunVersion < 1:
				entry.Status = "stale"
			case def.RunSummaryBound && latestRunVersion > 0 && rec.RunVersion != latestRunVersion:
				entry.Status = "stale"
			case rec.IsPassing():
				entry.Status = "passing"
			case rec.Verdict == model.VerificationVerdictFail:
				entry.Status = "failed"
			case len(rec.Blockers) > 0:
				entry.Status = "blocked"
			}
		} else if rec.IsPassing() {
			entry.Status = "passing"
		}
		evidence = append(evidence, entry)
	}
	return evidence, nil
}

// appendCatalogHints runs the capability resolver against the current host
// and returns technique-hint entries derived from its support attachments.
// The resolver is read-only with respect to kernel progression.
func appendCatalogHints(
	existing []techniqueHint,
	hostSkill string,
	governedChange *model.Change,
	view *nextView,
) []techniqueHint {
	sig := capability.Signals{Host: hostSkill}
	if governedChange != nil {
		sig.Paths = append(sig.Paths, governedChange.WorktreePath)
	}
	if view != nil {
		for _, rc := range view.Blockers {
			sig.Blockers = append(sig.Blockers, rc.Code)
		}
	}

	reg := capability.DefaultRegistry()
	resolution := capability.Resolve(reg, sig)
	for _, support := range resolution.Supports {
		existing = append(existing, techniqueHint{
			Name:              fmt.Sprintf("skill:%s", support.SkillID),
			Reason:            fmt.Sprintf("[%s] %s", support.Kind, support.Reason),
			HydrateReferences: normalizeHydrateKeys(capability.HydrateReferenceKeysForSkill(reg, support.SkillID)),
		})
	}
	return existing
}
