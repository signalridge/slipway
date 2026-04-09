package cmd

import (
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/toolgen"
)

func resolveWorkspaceSkillPaths(root, skillName, agentHint string) (promptPath, agentDefinitionPath string) {
	cfg := toolgen.ResolveWorkspaceTool(root)
	promptPath = toolgen.SkillPath(cfg, skillName)
	if strings.TrimSpace(agentHint) != "" {
		agentDefinitionPath = toolgen.AgentPath(cfg, agentHint)
	}
	return promptPath, agentDefinitionPath
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

	verificationDir := state.DisplayPath(root, filepath.Dir(state.VerificationFilePath(root, ref.Slug, nextSkillName)))
	agentHint := skill.AgentHintForSkill(nextSkillName)
	promptPath, agentDefPath := resolveWorkspaceSkillPaths(root, nextSkillName, agentHint)

	ns := &nextSkillView{
		Name:                nextSkillName,
		PromptPath:          promptPath,
		VerificationDir:     verificationDir,
		State:               nextState,
		AgentHint:           agentHint,
		AgentDefinitionPath: agentDefPath,
	}

	if governedChange != nil {
		if model.WorkflowState(nextState) == model.StateS1Plan && progression.HasEmptyCodebaseMap(root, view.InputContext.CodebaseMapDocs) {
			ns.TechniqueHints = append(ns.TechniqueHints, techniqueHint{
				Name:   "slipway codebase-map",
				Reason: "No durable codebase-map documents found. Run `slipway codebase-map` to establish brownfield context before planning.",
			})
		}
	}

	if nextSkillName == progression.SkillSpecComplianceReview || nextSkillName == progression.SkillCodeQualityReview {
		var guardrailDomain string
		if governedChange != nil {
			guardrailDomain = governedChange.GuardrailDomain
		}
		ns.ReviewContext = buildReviewContext(guardrailDomain)
	}

	ns.SkillConstraints = buildSkillConstraints(root, nextSkillName, governedChange)

	view.NextSkill = ns
	view.ContextBudget = estimateContextBudget(root, ns, view.InputContext)
	view.Constraints = deriveAgentConstraints(nextSkillName)

	if advanced.Action == "blocked" {
		view.Blockers = appendReasonCodes(view.Blockers, advanced.Blockers)
	}
	applyContextBudgetGuard(view)

	return nil
}
