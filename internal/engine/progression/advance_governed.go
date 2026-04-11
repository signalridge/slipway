package progression

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/engine/action"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

func AdvanceGoverned(root, slug string) (AdvanceSummary, error) {
	change, err := state.LoadChange(root, slug)
	if err != nil {
		return AdvanceSummary{}, err
	}

	fromState := change.CurrentState
	presetPolicy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return AdvanceSummary{}, err
	}

	// 0. Preset confirmation gate — must precede all advancement, including
	// S0_INTAKE, so pending presets cannot mutate change.yaml while surfacing a
	// blocked CLI view.
	if blockers := PresetConfirmationBlockers(change); len(blockers) > 0 {
		return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(blockers)), nil
	}

	// 1. S0_INTAKE: lightweight path — no bundle, no worktree, no wave sync.
	if fromState == model.StateS0Intake {
		return advanceIntake(root, &change, fromState)
	}

	// 2. Bundle precondition check.
	// Skip at S1_PLAN/research with NeedsDiscovery — research runs before
	// the full bundle is populated.
	skipBundleCheck := fromState == model.StateS1Plan &&
		change.PlanSubStep == model.PlanSubStepResearch &&
		change.NeedsDiscovery
	if ShouldCheckGovernedBundle(change) && !skipBundleCheck {
		bundleBlockers := GovernedBundleBlockers(root, change)
		if len(bundleBlockers) > 0 {
			return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(bundleBlockers)), nil
		}
	}
	assuranceBlockers := AssuranceContractBlockers(root, change)
	if len(assuranceBlockers) > 0 {
		return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(assuranceBlockers)), nil
	}

	// 3. Wave synchronization (only at S2_EXECUTE)
	if fromState == model.StateS2Execute {
		syncResult, err := SyncGovernedWaveExecution(root, change)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if len(syncResult.Blockers) > 0 {
			return blockedAdvanceSummary(fromState, syncResult.Blockers), nil
		}
	}

	executionSummaryCtx, err := state.LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		return AdvanceSummary{}, err
	}
	if len(executionSummaryCtx.Issues) > 0 {
		return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(executionSummaryCtx.Issues)), nil
	}

	// Ensure research artifact exists for discovery changes entering S1_PLAN/research.
	isResearchEntry := fromState == model.StateS1Plan && change.PlanSubStep == model.PlanSubStepResearch
	if isResearchEntry && change.NeedsDiscovery {
		if err := artifact.EnsureResearchArtifactForChange(root, change); err != nil {
			return AdvanceSummary{}, err
		}
	}

	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return AdvanceSummary{}, err
	}
	snap, snapErr := governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)
	if snapErr != nil {
		return AdvanceSummary{}, fmt.Errorf("recompute governance snapshot: %w", snapErr)
	}
	if blockers := governance.RequiredActionBlockers(change, governance.ResolveRuntimeRequiredActions(root, change, snap)); len(blockers) > 0 {
		return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(blockers)), nil
	}

	if summary, applied, err := attemptAutoPassSequence(root, change, fromState, fromState); err != nil {
		return AdvanceSummary{}, err
	} else if applied {
		return summary, nil
	}

	// 4. Skill evidence evaluation
	nextSkillName, evidenceState := ResolveNextSkill(change)
	closeoutRequired := presetPolicy.CloseoutRefreshRequired

	var passingSkills map[string]model.VerificationRecord
	// worktree-preflight is not a governance skill; its gate is checked in
	// step 5 (worktree validation). Skip governance skill evaluation when
	// the resolved next skill is worktree-preflight so the worktree gate
	// can surface the correct blocker.
	if nextSkillName != "" && nextSkillName != SkillWorktreePreflight {
		var skillBlockers []string
		passingSkills, skillBlockers, err = EvaluateRequiredSkillsForChange(
			root,
			change,
			model.WorkflowState(evidenceState),
			executionSummaryCtx.LatestRunVersion,
			closeoutRequired,
			change.PlanSubStep,
		)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if len(skillBlockers) > 0 {
			return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(skillBlockers)), nil
		}
	}

	// 5. Worktree validation (at S2_EXECUTE when NeedsDiscovery and worktree unbound)
	isWorktreeGateState := fromState == model.StateS2Execute
	if isWorktreeGateState && change.NeedsDiscovery && change.WorktreePath == "" {
		changeBeforeWorktreeBinding := change
		worktreePathBefore := change.WorktreePath
		worktreeBranchBefore := change.WorktreeBranch
		worktreeBlockers, err := GovernedWorktreeBlockers(root, &change, passingSkills)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if len(worktreeBlockers) > 0 {
			return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(worktreeBlockers)), nil
		}
		if change.WorktreePath != worktreePathBefore || change.WorktreeBranch != worktreeBranchBefore {
			if err := state.RelocateGovernedBundle(root, changeBeforeWorktreeBinding, change); err != nil {
				return AdvanceSummary{}, err
			}
			if err := state.SaveChange(root, change); err != nil {
				return AdvanceSummary{}, err
			}
		}
	}

	// Compute next state
	toState, err := ComputeNextGovernedState(change)
	if err != nil {
		return AdvanceSummary{}, err
	}

	if summary, applied, err := attemptAutoPassSequence(root, change, fromState, toState); err != nil {
		return AdvanceSummary{}, err
	} else if applied {
		return summary, nil
	}

	// 6. Gate evaluation (state-specific)

	// S1_PLAN.validate: post-audit recovery-only machine validation.
	isPlanValidateGate := fromState == model.StateS1Plan && change.PlanSubStep == model.PlanSubStepValidate
	if isPlanValidateGate {
		planResult := ValidatePlanningReadiness(root, change)
		if len(planResult.Blockers) > 0 {
			return blockedAdvanceSummary(fromState, planResult.Blockers), nil
		}
	}

	// G_scope gate: validates research.md structure and discovery evidence at
	// S1_PLAN/research before allowing progression to bundle. Without this,
	// discovery changes could skip research validation entirely.
	isResearchGate := fromState == model.StateS1Plan && change.PlanSubStep == model.PlanSubStepResearch && change.NeedsDiscovery
	if isResearchGate {
		scopeResult, err := EvaluateScopeGate(root, change, passingSkills)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if scopeResult.Status == model.GateStatusBlocked {
			return saveBlockedChange(root, change, fromState, scopeResult.ReasonCodes)
		}
	}

	isPlanAuditGate := fromState == model.StateS1Plan && change.PlanSubStep == model.PlanSubStepAudit
	if isPlanAuditGate {
		planResult := CheckGateWithIteration(root, &change, passingSkills, presetPolicy.MaxPlanAuditIterations)
		if planResult.Blocked {
			return saveBlockedChange(root, change, fromState, planResult.Blockers)
		}
	}

	if fromState == model.StateS4Verify {
		shipResult, err := EvaluateShipGate(root, change)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if shipResult.Status == model.GateStatusBlocked {
			return saveBlockedChange(root, change, fromState, shipResult.ReasonCodes)
		}
	}

	// 7. State transition
	// S1_PLAN substep progression: advance within planning before leaving to S2_EXECUTE.
	if fromState == model.StateS1Plan {
		nextSub := computeNextPlanSubStep(change.PlanSubStep)
		if nextSub != model.PlanSubStepNone {
			// Advance substep within S1_PLAN.
			change.PlanSubStep = nextSub
			if nextSub == model.PlanSubStepBundle {
				if err := ensureGovernedBundleScaffolded(root, &change); err != nil {
					return AdvanceSummary{}, err
				}
			}
			if err := state.SaveChange(root, change); err != nil {
				return AdvanceSummary{}, err
			}
			return AdvanceSummary{
				Action:    "advanced",
				FromState: fromState,
				ToState:   fromState,
				Message:   fmt.Sprintf("Advanced to S1_PLAN/%s.", nextSub),
			}, nil
		}
		// Exiting S1_PLAN: audit clean path runs post-audit machine validation
		// inline before entering S2_EXECUTE. If validation fails, the change
		// persists at S1_PLAN/validate as a recovery-only substep.
		if change.PlanSubStep == model.PlanSubStepAudit {
			planResult := ValidatePlanningReadiness(root, change)
			if len(planResult.Blockers) > 0 {
				change.PlanSubStep = model.PlanSubStepValidate
				if err := state.SaveChange(root, change); err != nil {
					return AdvanceSummary{}, err
				}
				return blockedAdvanceSummary(fromState, planResult.Blockers), nil
			}
		}
		// validate substep already checked by the gate above; fall through to S2_EXECUTE.
	}

	// D1: Block S4_VERIFY→DONE in `next`; only `slipway done` finalizes.
	if toState == model.StateDone {
		return saveChangeAndReturn(root, change, doneReadyAdvanceSummary(fromState, "All governance gates passed. Run `slipway done` to finalize."))
	}

	if toState == model.StateS2Execute {
		if _, err := state.MaterializeWavePlan(root, change); err != nil {
			return AdvanceSummary{}, err
		}
	}

	change.CurrentState = toState
	// Clear substeps when leaving their parent state.
	if toState != model.StateS1Plan {
		change.PlanSubStep = model.PlanSubStepNone
	}
	if toState != model.StateS0Intake {
		change.IntakeSubStep = model.IntakeSubStepNone
	}
	change.LastAutoPassedStates = nil
	if err := state.SaveChange(root, change); err != nil {
		return AdvanceSummary{}, err
	}

	return AdvanceSummary{
		Action:    "advanced",
		FromState: fromState,
		ToState:   toState,
		Message:   fmt.Sprintf("Advanced to %s.", toState),
	}, nil
}

// computeNextPlanSubStep returns the next planning substep, or PlanSubStepNone
// when the current substep is the final one before exiting S1_PLAN.
func computeNextPlanSubStep(current model.PlanSubStep) model.PlanSubStep {
	switch current {
	case model.PlanSubStepResearch:
		return model.PlanSubStepBundle
	case model.PlanSubStepBundle:
		return model.PlanSubStepAudit
	default:
		// audit, validate: ready to exit S1_PLAN.
		return model.PlanSubStepNone
	}
}

func ensureGovernedBundleScaffolded(root string, change *model.Change) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	resolution := ResolveChangeSchemaDiagnostics(*change)
	if len(resolution.Blockers) > 0 {
		return fmt.Errorf("resolve artifact schema: %s", strings.Join(resolution.Blockers, ","))
	}
	policy, err := governance.ResolvePresetPolicy(root, *change)
	if err != nil {
		return err
	}
	return artifact.ScaffoldGovernedBundleForChangeWithPreset(root, *change, policy.EffectivePreset, resolution.Schema)
}

// ComputeNextGovernedState determines the next workflow state for a governed change.
func ComputeNextGovernedState(change model.Change) (model.WorkflowState, error) {
	path := action.WorkflowPath(change.NeedsDiscovery)

	for i, s := range path {
		if s == change.CurrentState && i+1 < len(path) {
			return path[i+1], nil
		}
	}

	return "", fmt.Errorf("%w: no next state from %s", ErrNoNextState, change.CurrentState)
}

// CheckGateWithIteration evaluates the G_plan gate with iteration tracking.
func CheckGateWithIteration(root string, change *model.Change, passingSkills map[string]model.VerificationRecord, maxIterations int) PlanGateResult {
	result := EvaluatePlanGate(root, *change, passingSkills)
	if result.Status != model.GateStatusBlocked {
		change.PlanAuditIterations = 0
		if change.EvidenceRefs != nil {
			delete(change.EvidenceRefs, "plan_audit.last_checker_feedback")
		}
		return PlanGateResult{}
	}

	change.PlanAuditIterations++
	iteration := change.PlanAuditIterations
	if change.EvidenceRefs == nil {
		change.EvidenceRefs = map[string]string{}
	}
	change.EvidenceRefs["plan_audit.last_checker_feedback"] = strings.Join(model.ReasonMessages(result.ReasonCodes), "; ")

	blockers := append([]model.ReasonCode(nil), result.ReasonCodes...)
	blockers = append(blockers, model.NewReasonCode("plan_audit_iteration", fmt.Sprintf("%d/%d", iteration, maxIterations)))
	blockers = append(blockers, model.NewReasonCode("plan_checker_feedback_required", "rerun_plan_audit_with_blocker_feedback"))

	if iteration >= maxIterations {
		blockers = append(blockers, model.NewReasonCode("plan_audit_budget_exhausted", "consider `slipway pivot --kind rescope` or manual intervention"))
		blockers = append(blockers, model.NewReasonCode("plan_checker_loop_terminated", ""))
	}

	return PlanGateResult{Blocked: true, Blockers: blockers}
}

func EvaluatePlanGate(root string, change model.Change, passingSkills map[string]model.VerificationRecord) gate.GateEvaluation {
	planAuditPass := false
	var planBlockers []model.ReasonCode
	if record, ok := passingSkills[SkillPlanAudit]; ok {
		planAuditPass = record.IsPassing()
	} else {
		planBlockers = append(planBlockers, model.NewReasonCode("plan_audit_evidence_missing", ""))
	}
	bundleReady := CheckGovernedBundleReady(root, change)
	checklistBlockers := ValidateTasksChecklistDetailed(root, change).Blockers
	if len(checklistBlockers) > 0 {
		bundleReady = false
		planBlockers = append(planBlockers, model.ReasonCodesFromSpecs(checklistBlockers)...)
	}
	return gate.EvaluateGPlan(bundleReady, planAuditPass, planBlockers)
}

func EvaluateShipGate(root string, change model.Change) (gate.GateEvaluation, error) {
	shipAuthority, err := EvaluateShipAuthority(root, change)
	if err != nil {
		return gate.GateEvaluation{}, err
	}
	return shipAuthority.Result, nil
}

func EvaluateScopeGate(root string, change model.Change, passingSkills map[string]model.VerificationRecord) (gate.GateEvaluation, error) {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return gate.GateEvaluation{}, err
	}
	researchPath := filepath.Join(paths.GovernedBundleDir, "research.md")
	researchContent := ""
	if data, err := os.ReadFile(researchPath); err == nil {
		researchContent = string(data)
	}

	_, discoveryOK := passingSkills[SkillResearchOrchestration]

	// Worktree validation is only meaningful when a worktree is already bound.
	// G_scope runs at S1_PLAN/research (before worktree creation at S2_EXECUTE),
	// so skip worktree validation when WorktreePath is empty.
	var worktreeReasons []model.ReasonCode
	if strings.TrimSpace(change.WorktreePath) != "" {
		var wtErr error
		worktreeReasons, wtErr = state.ValidateDedicatedWorktreeAuthenticityReasons(root, change.WorktreePath, change.WorktreeBranch)
		if wtErr != nil {
			return gate.GateEvaluation{}, wtErr
		}
	}
	return gate.EvaluateGScope(change, researchContent, discoveryOK, worktreeReasons), nil
}
