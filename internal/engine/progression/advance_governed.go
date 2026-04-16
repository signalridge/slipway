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

const planAuditLastCheckerFeedbackKey = "plan_audit.last_checker_feedback"

func AdvanceGoverned(root, slug string, opts ...AdvanceOptions) (AdvanceSummary, error) {
	var options AdvanceOptions
	if len(opts) > 0 {
		options = opts[0]
	}
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

	preTransitionSideEffects := make([]SideEffect, 0, 2)

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

	if !options.SkipAutoPass {
		if summary, applied, err := attemptAutoPassSequence(root, change, fromState, fromState); err != nil {
			return AdvanceSummary{}, err
		} else if applied {
			return summary, nil
		}
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
		derivation, err := DeriveWorktreeBlockers(root, change, passingSkills)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if len(derivation.Blockers) > 0 {
			return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(derivation.Blockers)), nil
		}
		if err := ApplyWorktreeMetadata(&change, derivation); err != nil {
			return blockedAdvanceSummary(fromState, []model.ReasonCode{
				model.NewReasonCode("worktree_metadata_persist_failed", err.Error()),
			}), nil
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

	if !options.SkipAutoPass {
		if summary, applied, err := attemptAutoPassSequence(root, change, fromState, toState); err != nil {
			return AdvanceSummary{}, err
		} else if applied {
			return summary, nil
		}
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
		planResult := CheckGateWithIteration(root, change, passingSkills, presetPolicy.MaxPlanAuditIterations)
		sideEffects, err := ApplyPlanGateResult(&change, planResult)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if planResult.Blocked {
			summary := blockedAdvanceSummary(fromState, planResult.Blockers)
			summary.FromSubStep = string(change.PlanSubStep)
			summary.ToSubStep = string(change.PlanSubStep)
			summary.Reason = "plan_audit_feedback_recorded"
			summary.SideEffects = sideEffects
			return saveChangeAndReturn(root, change, summary)
		}
		preTransitionSideEffects = append(preTransitionSideEffects, sideEffects...)
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
		fromSub := string(change.PlanSubStep)
		nextSub := computeNextPlanSubStep(change.PlanSubStep)
		if nextSub != model.PlanSubStepNone {
			// Advance substep within S1_PLAN.
			change.PlanSubStep = nextSub
			var sideEffects []SideEffect
			if nextSub == model.PlanSubStepBundle {
				if err := ensureGovernedBundleScaffolded(root, &change); err != nil {
					return AdvanceSummary{}, err
				}
				sideEffects = append(sideEffects, SideEffect{
					Kind:   "scaffolded_bundle",
					Detail: "governed bundle artifacts created or verified",
				})
			}
			if err := state.SaveChange(root, change); err != nil {
				return AdvanceSummary{}, err
			}
			return AdvanceSummary{
				Action:      "advanced",
				FromState:   fromState,
				ToState:     fromState,
				FromSubStep: fromSub,
				ToSubStep:   string(nextSub),
				Reason:      "plan_substep_progression",
				SideEffects: sideEffects,
				Message:     fmt.Sprintf("Advanced to S1_PLAN/%s.", nextSub),
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
				summary := blockedAdvanceSummary(fromState, planResult.Blockers)
				summary.FromSubStep = fromSub
				summary.ToSubStep = string(model.PlanSubStepValidate)
				summary.RecoveryOnly = true
				summary.Reason = "plan_validation_failed"
				return summary, nil
			}
		}
		// validate substep already checked by the gate above; fall through to S2_EXECUTE.
	}

	// D1: Block S4_VERIFY→DONE in `next`; only `slipway done` finalizes.
	if toState == model.StateDone {
		return saveChangeAndReturn(root, change, doneReadyAdvanceSummary(fromState, "All governance gates passed. Run `slipway done` to finalize."))
	}

	sideEffects := append([]SideEffect(nil), preTransitionSideEffects...)
	if toState == model.StateS2Execute {
		if _, err := state.MaterializeWavePlan(root, change); err != nil {
			return AdvanceSummary{}, err
		}
		sideEffects = append(sideEffects, SideEffect{
			Kind:   "materialized_wave_plan",
			Detail: "wave plan materialized from tasks.md",
		})
	}

	fromSub := ""
	if fromState == model.StateS1Plan {
		fromSub = string(change.PlanSubStep)
	}

	var cleared []string
	change.CurrentState = toState
	// Clear substeps when leaving their parent state.
	if toState != model.StateS1Plan && change.PlanSubStep != model.PlanSubStepNone {
		cleared = append(cleared, "plan_substep")
		change.PlanSubStep = model.PlanSubStepNone
	}
	if toState != model.StateS0Intake && change.IntakeSubStep != model.IntakeSubStepNone {
		cleared = append(cleared, "intake_substep")
		change.IntakeSubStep = model.IntakeSubStepNone
	}
	if change.LastAutoPassedStates != nil {
		cleared = append(cleared, "last_auto_passed_states")
		change.LastAutoPassedStates = nil
	}
	if err := state.SaveChange(root, change); err != nil {
		return AdvanceSummary{}, err
	}

	return AdvanceSummary{
		Action:        "advanced",
		FromState:     fromState,
		ToState:       toState,
		FromSubStep:   fromSub,
		Reason:        "state_progression",
		SideEffects:   sideEffects,
		ClearedFields: cleared,
		Message:       fmt.Sprintf("Advanced to %s.", toState),
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

// CheckGateWithIteration evaluates the G_plan gate with iteration tracking
// without mutating the input change. Callers must explicitly apply the returned
// mutation contract if they want to persist it.
func CheckGateWithIteration(root string, change model.Change, passingSkills map[string]model.VerificationRecord, maxIterations int) PlanGateResult {
	result := EvaluatePlanGate(root, change, passingSkills)
	if result.Status != model.GateStatusBlocked {
		return PlanGateResult{
			NextPlanAuditIterations:  0,
			ClearLastCheckerFeedback: true,
		}
	}

	iteration := change.PlanAuditIterations + 1
	feedback := strings.Join(model.ReasonMessages(result.ReasonCodes), "; ")
	if strings.TrimSpace(feedback) == "" {
		feedback = strings.Join(model.ReasonSpecs(result.ReasonCodes), "; ")
	}

	blockers := append([]model.ReasonCode(nil), result.ReasonCodes...)
	blockers = append(blockers, model.NewReasonCode("plan_audit_iteration", fmt.Sprintf("%d/%d", iteration, maxIterations)))
	blockers = append(blockers, model.NewReasonCode("plan_checker_feedback_required", "rerun_plan_audit_with_blocker_feedback"))

	if iteration >= maxIterations {
		blockers = append(blockers, model.NewReasonCode("plan_audit_budget_exhausted", "consider `slipway pivot --kind rescope` or manual intervention"))
		blockers = append(blockers, model.NewReasonCode("plan_checker_loop_terminated", ""))
	}

	return PlanGateResult{
		Blocked:                 true,
		Blockers:                blockers,
		NextPlanAuditIterations: iteration,
		LastCheckerFeedback:     feedback,
	}
}

// ApplyPlanGateResult persists the explicit mutation contract returned by
// CheckGateWithIteration and reports any runtime-owned side effects.
func ApplyPlanGateResult(change *model.Change, result PlanGateResult) ([]SideEffect, error) {
	if change == nil {
		return nil, fmt.Errorf("change is required")
	}
	change.Normalize()

	sideEffects := make([]SideEffect, 0, 2)
	if change.PlanAuditIterations != result.NextPlanAuditIterations {
		change.PlanAuditIterations = result.NextPlanAuditIterations
		sideEffects = append(sideEffects, SideEffect{
			Kind:   "updated_plan_audit_iterations",
			Detail: fmt.Sprintf("%d", result.NextPlanAuditIterations),
		})
	}

	if change.EvidenceRefs == nil {
		change.EvidenceRefs = map[string]string{}
	}
	if result.ClearLastCheckerFeedback {
		if _, ok := change.EvidenceRefs[planAuditLastCheckerFeedbackKey]; ok {
			delete(change.EvidenceRefs, planAuditLastCheckerFeedbackKey)
			sideEffects = append(sideEffects, SideEffect{
				Kind:   "cleared_plan_checker_feedback",
				Detail: planAuditLastCheckerFeedbackKey,
			})
		}
		return sideEffects, nil
	}

	feedback := strings.TrimSpace(result.LastCheckerFeedback)
	if feedback == "" {
		return sideEffects, nil
	}
	if existing := strings.TrimSpace(change.EvidenceRefs[planAuditLastCheckerFeedbackKey]); existing != feedback {
		change.EvidenceRefs[planAuditLastCheckerFeedbackKey] = feedback
		sideEffects = append(sideEffects, SideEffect{
			Kind:   "recorded_plan_checker_feedback",
			Detail: planAuditLastCheckerFeedbackKey,
		})
	}
	return sideEffects, nil
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
