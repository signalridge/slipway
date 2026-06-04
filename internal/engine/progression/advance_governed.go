package progression

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/signalridge/slipway/internal/engine/action"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

const planAuditLastCheckerFeedbackKey = "plan_audit.last_checker_feedback"

func AdvanceGoverned(root, slug string, opts ...AdvanceOptions) (summary AdvanceSummary, err error) {
	var options AdvanceOptions
	if len(opts) > 0 {
		options = opts[0]
	}
	change, err := state.LoadChange(root, slug)
	if err != nil {
		return AdvanceSummary{}, err
	}
	beforeChange := change
	defer func() {
		if err != nil || strings.TrimSpace(summary.Action) == "" || summary.Action == "query" {
			return
		}
		afterChange := change
		if reloaded, loadErr := state.LoadChange(root, slug); loadErr == nil {
			afterChange = reloaded
		}
		if eventErr := recordAdvanceLifecycleEvent(root, beforeChange, afterChange, summary, options.Command); eventErr != nil {
			summary.SideEffects = append(summary.SideEffects, SideEffect{
				Kind:   "lifecycle_event_write_failed",
				Detail: eventErr.Error(),
			})
		}
	}()

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

	if stale, err := stalePlanningRecoveryNeededForPlanAuditDigest(root, change); err != nil {
		return AdvanceSummary{}, err
	} else if stale {
		return beginStalePlanningRecovery(root, &change, fromState)
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
	if stalePlanningRecoveryIssueAvailable(change, executionSummaryCtx.Issues) {
		return beginStalePlanningRecovery(root, &change, fromState)
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

	if blockers, err := applyPendingWorktreePreflight(root, &change, fromState); err != nil {
		return AdvanceSummary{}, err
	} else if len(blockers) > 0 {
		return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(blockers)), nil
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

	// 4. Skill evidence evaluation
	nextSkillName, evidenceState := ResolveNextSkill(change)
	closeoutRequired := FinalCloseoutEvidenceRequired(presetPolicy)

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
		stampResult, err := stampPassingSkillDigests(root, change, passingSkills)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if len(stampResult.Blockers) > 0 {
			return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(stampResult.Blockers)), nil
		}
		preTransitionSideEffects = append(preTransitionSideEffects, skillEvidenceSideEffects(passingSkills)...)
		preTransitionSideEffects = append(preTransitionSideEffects, digestBackfilledSideEffects(stampResult.BackfilledSkills)...)
	}
	preTransitionSkillEvidence := skillEvidenceTraceFromPassing(root, change, passingSkills)

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
			summary.SideEffects = append(summary.SideEffects, preTransitionSideEffects...)
			summary.SkillEvidence = append(summary.SkillEvidence, preTransitionSkillEvidence...)
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
			summary := blockedAdvanceSummary(fromState, scopeResult.ReasonCodes)
			summary.SideEffects = append(summary.SideEffects, preTransitionSideEffects...)
			summary.SkillEvidence = append(summary.SkillEvidence, preTransitionSkillEvidence...)
			return saveChangeAndReturn(root, change, summary)
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
			summary.SideEffects = append(summary.SideEffects, preTransitionSideEffects...)
			summary.SkillEvidence = append(summary.SkillEvidence, preTransitionSkillEvidence...)
			return saveChangeAndReturn(root, change, summary)
		}
		preTransitionSideEffects = append(preTransitionSideEffects, sideEffects...)
	}

	if fromState == model.StateS4Verify {
		shipAuthority, err := EvaluateShipAuthority(root, change)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if shipAuthority.Result.Status == model.GateStatusBlocked {
			summary := blockedAdvanceSummary(fromState, shipAuthority.Result.ReasonCodes)
			summary.SideEffects = append(summary.SideEffects, preTransitionSideEffects...)
			summary.SkillEvidence = append(summary.SkillEvidence, preTransitionSkillEvidence...)
			return saveChangeAndReturn(root, change, summary)
		}
		stampResult, err := stampPassingSkillDigests(root, change, shipAuthorityPassingSkills(shipAuthority))
		if err != nil {
			return AdvanceSummary{}, err
		}
		if len(stampResult.Blockers) > 0 {
			summary := blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(stampResult.Blockers))
			summary.SideEffects = append(summary.SideEffects, preTransitionSideEffects...)
			summary.SkillEvidence = append(summary.SkillEvidence, preTransitionSkillEvidence...)
			return saveChangeAndReturn(root, change, summary)
		}
		preTransitionSideEffects = append(preTransitionSideEffects, digestBackfilledSideEffects(stampResult.BackfilledSkills)...)
	}

	// 7. State transition
	// S1_PLAN substep progression: advance within planning before leaving to S2_EXECUTE.
	if fromState == model.StateS1Plan {
		fromSub := string(change.PlanSubStep)
		nextSub := computeNextPlanSubStep(change.PlanSubStep)
		if nextSub != model.PlanSubStepNone {
			// Advance substep within S1_PLAN.
			change.AdvancePlanSubStep(nextSub)
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
			sideEffects = append(append([]SideEffect(nil), preTransitionSideEffects...), sideEffects...)
			return AdvanceSummary{
				Action:      "advanced",
				FromState:   fromState,
				ToState:     fromState,
				FromSubStep: fromSub,
				ToSubStep:   string(nextSub),
				Reason:      "plan_substep_progression",
				SideEffects: sideEffects,
				SkillEvidence: append(
					[]SkillEvidenceTrace(nil),
					preTransitionSkillEvidence...,
				),
				Message: fmt.Sprintf("Advanced to S1_PLAN/%s.", nextSub),
			}, nil
		}
		// Exiting S1_PLAN: audit clean path runs post-audit machine validation
		// inline before entering S2_EXECUTE. If validation fails, the change
		// persists at S1_PLAN/validate as a recovery-only substep.
		if change.PlanSubStep == model.PlanSubStepAudit {
			planResult := ValidatePlanningReadiness(root, change)
			if len(planResult.Blockers) > 0 {
				change.AdvancePlanSubStep(model.PlanSubStepValidate)
				if err := state.SaveChange(root, change); err != nil {
					return AdvanceSummary{}, err
				}
				summary := blockedAdvanceSummary(fromState, planResult.Blockers)
				summary.FromSubStep = fromSub
				summary.ToSubStep = string(model.PlanSubStepValidate)
				summary.RecoveryOnly = true
				summary.Reason = "plan_validation_failed"
				summary.SideEffects = append(summary.SideEffects, preTransitionSideEffects...)
				summary.SkillEvidence = append(summary.SkillEvidence, preTransitionSkillEvidence...)
				return summary, nil
			}
		}
		// validate substep already checked by the gate above; fall through to S2_EXECUTE.
	}

	// D1: Block S4_VERIFY→DONE in `next`; only `slipway done` finalizes.
	if toState == model.StateDone {
		summary := doneReadyAdvanceSummary(fromState, "All governance gates passed. Run `slipway done` to finalize.")
		summary.SideEffects = append(summary.SideEffects, preTransitionSideEffects...)
		summary.SkillEvidence = append(summary.SkillEvidence, preTransitionSkillEvidence...)
		return saveChangeAndReturn(root, change, summary)
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

	cleared := change.TransitionTo(toState)
	if change.ClearAutoPassHistory() {
		cleared = append(cleared, "last_auto_passed_states")
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
		SkillEvidence: append([]SkillEvidenceTrace(nil), preTransitionSkillEvidence...),
		ClearedFields: cleared,
		Message:       fmt.Sprintf("Advanced to %s.", toState),
	}, nil
}

func shipAuthorityPassingSkills(shipAuthority ShipAuthority) map[string]model.VerificationRecord {
	passing := map[string]model.VerificationRecord{}
	for skillName, record := range shipAuthority.ReviewAuthority.PassingSkills {
		passing[skillName] = record
	}
	for skillName, record := range shipAuthority.VerifyPassingSkills {
		passing[skillName] = record
	}
	return passing
}

func applyPendingWorktreePreflight(root string, change *model.Change, fromState model.WorkflowState) ([]string, error) {
	if change == nil {
		return nil, nil
	}
	if fromState != model.StateS2Execute || !change.NeedsDiscovery || strings.TrimSpace(change.WorktreePath) != "" {
		return nil, nil
	}

	before := *change
	derivation, err := DeriveWorktreeBlockers(root, *change, nil)
	if err != nil {
		return nil, err
	}
	if len(derivation.Blockers) > 0 {
		return derivation.Blockers, nil
	}
	if err := ApplyWorktreeMetadata(change, derivation); err != nil {
		return []string{"worktree_metadata_persist_failed:" + err.Error()}, nil
	}
	if change.WorktreePath == before.WorktreePath && change.WorktreeBranch == before.WorktreeBranch {
		return nil, nil
	}
	if err := state.RelocateGovernedBundle(root, before, *change); err != nil {
		return nil, err
	}
	if err := state.SaveChange(root, *change); err != nil {
		return nil, err
	}
	return nil, nil
}

// planSubStepOrder defines the linear progression of S1_PLAN substeps.
// audit and validate are terminal — they exit S1_PLAN, not advance within it.
var planSubStepOrder = []model.PlanSubStep{
	model.PlanSubStepResearch,
	model.PlanSubStepBundle,
	model.PlanSubStepAudit,
}

// computeNextPlanSubStep returns the next planning substep, or PlanSubStepNone
// when the current substep is terminal (audit/validate exit S1_PLAN).
func computeNextPlanSubStep(current model.PlanSubStep) model.PlanSubStep {
	idx := slices.Index(planSubStepOrder, current)
	if idx < 0 || idx+1 >= len(planSubStepOrder) {
		return model.PlanSubStepNone
	}
	return planSubStepOrder[idx+1]
}

func beginStalePlanningRecovery(root string, change *model.Change, fromState model.WorkflowState) (AdvanceSummary, error) {
	if change == nil {
		return AdvanceSummary{}, fmt.Errorf("change is required for stale planning recovery")
	}
	paths, err := state.ResolveChangePaths(root, *change)
	if err != nil {
		return AdvanceSummary{}, err
	}

	verificationDir := filepath.Join(paths.GovernedBundleDir, "verification")
	planningEvidenceFiles := []string{
		filepath.Join(verificationDir, SkillPlanAudit+".yaml"),
		filepath.Join(verificationDir, state.WavePlanFileName),
		filepath.Join(verificationDir, state.ExecutionSummaryFileName),
	}
	downstreamVerificationFiles := []string{
		filepath.Join(verificationDir, SkillSpecComplianceReview+".yaml"),
		filepath.Join(verificationDir, SkillCodeQualityReview+".yaml"),
		filepath.Join(verificationDir, SkillGoalVerification+".yaml"),
		filepath.Join(verificationDir, SkillFinalCloseout+".yaml"),
	}
	sideEffects := make([]SideEffect, 0, len(planningEvidenceFiles)+len(downstreamVerificationFiles)+1)
	for _, path := range planningEvidenceFiles {
		removed, err := removeFileIfExists(path)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if removed {
			sideEffects = append(sideEffects, SideEffect{
				Kind:   "cleared_stale_planning_evidence",
				Detail: state.DisplayPath(root, path),
			})
		}
	}
	for _, path := range downstreamVerificationFiles {
		removed, err := removeFileIfExists(path)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if removed {
			sideEffects = append(sideEffects, SideEffect{
				Kind:   "cleared_stale_downstream_verification",
				Detail: state.DisplayPath(root, path),
			})
		}
	}
	sideEffects = append(sideEffects, SideEffect{
		Kind:   "preserved_runtime_execution_evidence",
		Detail: state.DisplayPath(root, state.ChangeDir(root, change.Slug)),
	})

	cleared := change.TransitionTo(model.StateS1Plan)
	if change.PlanSubStep != model.PlanSubStepAudit {
		change.AdvancePlanSubStep(model.PlanSubStepAudit)
		cleared = append(cleared, "plan_substep")
	}
	if change.ClearAutoPassHistory() {
		cleared = append(cleared, "last_auto_passed_states")
	}
	if change.ClearActiveCheckpoint() {
		cleared = append(cleared, "active_checkpoint")
	}
	if change.ResetPlanAuditIterations() {
		cleared = append(cleared, "plan_audit_iterations")
	}
	if change.ClearEvidenceRef(SkillPlanAudit) {
		cleared = append(cleared, "evidence_refs."+SkillPlanAudit)
	}
	if change.ClearEvidenceRef(planAuditLastCheckerFeedbackKey) {
		cleared = append(cleared, "evidence_refs."+planAuditLastCheckerFeedbackKey)
	}
	for _, skillName := range []string{
		SkillSpecComplianceReview,
		SkillCodeQualityReview,
		SkillGoalVerification,
		SkillFinalCloseout,
	} {
		if change.ClearEvidenceRef(skillName) {
			cleared = append(cleared, "evidence_refs."+skillName)
		}
	}

	if err := state.SaveChange(root, *change); err != nil {
		return AdvanceSummary{}, err
	}

	return AdvanceSummary{
		Action:        "advanced",
		FromState:     fromState,
		ToState:       model.StateS1Plan,
		ToSubStep:     string(model.PlanSubStepAudit),
		Reason:        "stale_planning_recovery_started",
		SideEffects:   sideEffects,
		RecoveryOnly:  true,
		ClearedFields: stringutil.UniqueSorted(cleared),
		Message:       "Reopened S1_PLAN/audit for stale planning evidence recovery.",
	}, nil
}

func removeFileIfExists(path string) (bool, error) {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func stalePlanningRecoveryNeededForPlanAuditDigest(root string, change model.Change) (bool, error) {
	if !stalePlanningRecoveryState(change.CurrentState) {
		return false, nil
	}
	record, err := state.LoadVerification(root, change.Slug, SkillPlanAudit)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !record.IsPassing() {
		return false, nil
	}
	blockers, err := skillDigestFreshnessBlockers(root, change, SkillPlanAudit, record)
	if err != nil {
		return false, err
	}
	for _, blocker := range blockers {
		if strings.HasPrefix(blocker, "required_skill_stale:"+SkillPlanAudit+":") {
			return true, nil
		}
	}
	return false, nil
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
	projectCtx := change.ProjectContext
	if projectCtx.IsZero() {
		projectCtx = InferProjectContext(root)
	}
	docs, err := docSectionsFromIntent(root, *change)
	if err != nil {
		return fmt.Errorf("extracting doc sections from intent: %w", err)
	}
	if docs.Scope != "" || docs.Constraints != "" || docs.Acceptance != "" {
		return artifact.ScaffoldGovernedBundleForChangeWithContextAndDocs(root, *change, policy.EffectivePreset, projectCtx, docs, resolution.Schema)
	}
	return artifact.ScaffoldGovernedBundleForChangeWithContext(root, *change, policy.EffectivePreset, projectCtx, resolution.Schema)
}

func docSectionsFromIntent(root string, change model.Change) (artifact.DocSections, error) {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return artifact.DocSections{}, err
	}
	intentPath := filepath.Join(paths.GovernedBundleDir, "intent.md")
	data, err := os.ReadFile(intentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return artifact.DocSections{}, nil
		}
		return artifact.DocSections{}, err
	}
	return artifact.DocSections{
		Scope:       stringutil.LastMarkdownSectionContent(string(data), "## In Scope"),
		Constraints: stringutil.LastMarkdownSectionContent(string(data), "## Constraints"),
		Acceptance:  stringutil.LastMarkdownSectionContent(string(data), "## Acceptance Signals"),
	}, nil
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
	stalled := iteration > 1 && strings.TrimSpace(change.EvidenceRefs[planAuditLastCheckerFeedbackKey]) == feedback

	blockers := append([]model.ReasonCode(nil), result.ReasonCodes...)
	blockers = append(blockers, model.NewReasonCode("plan_audit_iteration", fmt.Sprintf("%d/%d", iteration, maxIterations)))
	blockers = append(blockers, model.NewReasonCode("plan_checker_feedback_required", "rerun_plan_audit_with_blocker_feedback"))

	if stalled {
		blockers = append(blockers, model.NewReasonCode("plan_audit_stalled", "checker feedback did not change from the previous failed audit"))
	}
	if stalled || iteration >= maxIterations {
		blockers = append(blockers, model.NewReasonCode("plan_audit_budget_exhausted", "consider `slipway pivot --kind rescope` or manual intervention"))
		blockers = append(blockers, model.NewReasonCode("plan_checker_loop_terminated", ""))
	}

	return PlanGateResult{
		Blocked:                 true,
		Blockers:                blockers,
		NextPlanAuditIterations: iteration,
		LastCheckerFeedback:     feedback,
		Stalled:                 stalled,
	}
}

func recordAdvanceLifecycleEvent(root string, before, after model.Change, summary AdvanceSummary, command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "advance"
	}
	correlationID := "corr-" + uuid.NewString()
	event := state.LifecycleEvent{
		CorrelationID: correlationID,
		Command:       command,
		ActorKind:     "cli",
		EventType:     advanceLifecycleEventType(summary),
		Action:        summary.Action,
		Reason:        summary.Reason,
		Result:        summary.Action,
		GateID:        advanceGateID(before, summary),
		SkillID:       firstBlockerDetail(summary.Blockers, "required_skill_missing"),
		ControlID:     firstGovernanceActionControl(summary.Blockers),
		BeforeState:   before.CurrentState,
		AfterState:    after.CurrentState,
		BeforeSubStep: advanceSummaryBeforeSubStep(before, summary),
		AfterSubStep:  advanceSummaryAfterSubStep(after, summary),
		Blockers:      append([]model.ReasonCode(nil), summary.Blockers...),
		EvidenceRefs:  lifecycleEvidenceRefs(after.EvidenceRefs),
		SideEffects:   lifecycleSideEffects(summary.SideEffects),
		ClearedFields: append([]string(nil), summary.ClearedFields...),
	}
	if _, err := state.AppendLifecycleEvent(root, after, event); err != nil {
		return err
	}
	for _, derived := range derivedAdvanceLifecycleEvents(root, after, event, summary) {
		if _, err := state.AppendLifecycleEvent(root, after, derived); err != nil {
			return err
		}
	}
	return nil
}

func advanceLifecycleEventType(summary AdvanceSummary) string {
	switch summary.Action {
	case "advanced":
		if len(summary.AutoPassedStates) > 0 {
			return "auto_pass.applied"
		}
		return "state.transitioned"
	case "blocked":
		return "gate.evaluated"
	case "done_ready":
		if len(summary.AutoPassedStates) > 0 {
			return "auto_pass.applied"
		}
		return "done.ready"
	default:
		return "advance." + summary.Action
	}
}

func advanceGateID(before model.Change, summary AdvanceSummary) string {
	if summary.Action != "blocked" && summary.Action != "done_ready" {
		return ""
	}
	switch before.CurrentState {
	case model.StateS1Plan:
		substep := before.PlanSubStep
		if summary.FromSubStep != "" {
			substep = model.PlanSubStep(summary.FromSubStep)
		}
		switch substep {
		case model.PlanSubStepResearch:
			return string(gate.GateScope)
		case model.PlanSubStepAudit, model.PlanSubStepValidate:
			return string(gate.GatePlan)
		default:
			return string(gate.GatePlan)
		}
	case model.StateS4Verify:
		return string(gate.GateShip)
	default:
		return ""
	}
}

func derivedAdvanceLifecycleEvents(root string, change model.Change, base state.LifecycleEvent, summary AdvanceSummary) []state.LifecycleEvent {
	var derived []state.LifecycleEvent
	for _, blocker := range base.Blockers {
		switch blocker.Code {
		case "governance_action_required":
			controlID := governanceActionControl(blocker)
			if controlID == "" {
				continue
			}
			derived = append(derived, lifecycleDerivedEvent(base, "control.triggered", controlID, ""))
		case "required_skill_missing":
			skillID := strings.TrimSpace(blocker.Detail)
			if skillID == "" {
				continue
			}
			derived = append(derived, lifecycleDerivedEvent(base, "skill.presented", "", skillID))
		}
	}
	for _, evidence := range summary.SkillEvidence {
		skillID := strings.TrimSpace(evidence.SkillName)
		if skillID == "" {
			continue
		}
		event := lifecycleDerivedEvent(base, "skill.evidence_recorded", "", skillID)
		event.Result = "recorded"
		event.Reason = "verification_evidence_consumed"
		event.EvidenceRefs = map[string]string{
			skillID: skillEvidenceRef(root, change, evidence),
		}
		event.Diagnostics = skillEvidenceDiagnostics(evidence)
		derived = append(derived, event)
	}
	for _, sideEffect := range summary.SideEffects {
		if sideEffect.Kind != digestBackfilledFromLegacyVerdictEvent {
			continue
		}
		skillID := strings.TrimSpace(sideEffect.Detail)
		if skillID == "" {
			continue
		}
		event := lifecycleDerivedEvent(base, digestBackfilledFromLegacyVerdictEvent, "", skillID)
		event.Result = "recorded"
		event.Reason = "legacy_verdict_digest_backfilled"
		event.EvidenceRefs = map[string]string{
			"evidence-digests": state.DisplayPath(root, state.EvidenceDigestsPathForRead(root, change.Slug)),
		}
		event.Diagnostics = []string{"skill=" + skillID}
		derived = append(derived, event)
	}
	return derived
}

func lifecycleDerivedEvent(base state.LifecycleEvent, eventType, controlID, skillID string) state.LifecycleEvent {
	base.EventID = ""
	base.EventType = eventType
	base.ControlID = controlID
	base.SkillID = skillID
	base.Blockers = nil
	return base
}

func advanceSummaryBeforeSubStep(before model.Change, summary AdvanceSummary) string {
	if strings.TrimSpace(summary.FromSubStep) != "" {
		return summary.FromSubStep
	}
	switch before.CurrentState {
	case model.StateS0Intake:
		return string(before.IntakeSubStep)
	case model.StateS1Plan:
		return string(before.PlanSubStep)
	default:
		return ""
	}
}

func advanceSummaryAfterSubStep(after model.Change, summary AdvanceSummary) string {
	if strings.TrimSpace(summary.ToSubStep) != "" {
		return summary.ToSubStep
	}
	switch after.CurrentState {
	case model.StateS0Intake:
		return string(after.IntakeSubStep)
	case model.StateS1Plan:
		return string(after.PlanSubStep)
	default:
		return ""
	}
}

func lifecycleSideEffects(sideEffects []SideEffect) []state.LifecycleSideEffect {
	if len(sideEffects) == 0 {
		return nil
	}
	out := make([]state.LifecycleSideEffect, 0, len(sideEffects))
	for _, sideEffect := range sideEffects {
		out = append(out, state.LifecycleSideEffect{
			Kind:   sideEffect.Kind,
			Detail: sideEffect.Detail,
		})
	}
	return out
}

func lifecycleEvidenceRefs(refs map[string]string) map[string]string {
	if len(refs) == 0 {
		return nil
	}
	out := make(map[string]string, len(refs))
	for key, value := range refs {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func skillEvidenceTraceFromPassing(root string, change model.Change, passingSkills map[string]model.VerificationRecord) []SkillEvidenceTrace {
	if len(passingSkills) == 0 {
		return nil
	}
	names := make([]string, 0, len(passingSkills))
	for skillName := range passingSkills {
		if strings.TrimSpace(skillName) != "" {
			names = append(names, skillName)
		}
	}
	slices.Sort(names)
	traces := make([]SkillEvidenceTrace, 0, len(names))
	for _, skillName := range names {
		record := passingSkills[skillName]
		trace := SkillEvidenceTrace{
			SkillName:   skillName,
			RunVersion:  record.RunVersion,
			Timestamp:   record.Timestamp,
			References:  append([]string(nil), record.References...),
			EvidenceRef: state.DisplayPath(root, state.VerificationFilePath(root, change.Slug, skillName)),
		}
		traces = append(traces, trace)
	}
	return traces
}

func skillEvidenceSideEffects(passingSkills map[string]model.VerificationRecord) []SideEffect {
	if len(passingSkills) == 0 {
		return nil
	}
	names := make([]string, 0, len(passingSkills))
	for skillName := range passingSkills {
		if strings.TrimSpace(skillName) != "" {
			names = append(names, skillName)
		}
	}
	slices.Sort(names)
	sideEffects := make([]SideEffect, 0, len(names))
	for _, skillName := range names {
		sideEffects = append(sideEffects, SideEffect{
			Kind:   "skill_evidence_recorded",
			Detail: skillName,
		})
	}
	return sideEffects
}

func digestBackfilledSideEffects(skillNames []string) []SideEffect {
	if len(skillNames) == 0 {
		return nil
	}
	skillNames = stringutil.UniqueSorted(skillNames)
	sideEffects := make([]SideEffect, 0, len(skillNames))
	for _, skillName := range skillNames {
		skillName = strings.TrimSpace(skillName)
		if skillName == "" {
			continue
		}
		sideEffects = append(sideEffects, SideEffect{
			Kind:   digestBackfilledFromLegacyVerdictEvent,
			Detail: skillName,
		})
	}
	return sideEffects
}

func skillEvidenceRef(root string, change model.Change, evidence SkillEvidenceTrace) string {
	if strings.TrimSpace(evidence.EvidenceRef) != "" {
		return evidence.EvidenceRef
	}
	return state.DisplayPath(root, state.VerificationFilePath(root, change.Slug, evidence.SkillName))
}

func skillEvidenceDiagnostics(evidence SkillEvidenceTrace) []string {
	diagnostics := []string{fmt.Sprintf("run_version=%d", evidence.RunVersion)}
	if !evidence.Timestamp.IsZero() {
		diagnostics = append(diagnostics, "timestamp="+evidence.Timestamp.UTC().Format(time.RFC3339))
	}
	for _, ref := range evidence.References {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			diagnostics = append(diagnostics, "reference="+ref)
		}
	}
	return diagnostics
}

func firstBlockerDetail(blockers []model.ReasonCode, code string) string {
	for _, blocker := range blockers {
		if blocker.Code == code {
			return strings.TrimSpace(blocker.Detail)
		}
	}
	return ""
}

func firstGovernanceActionControl(blockers []model.ReasonCode) string {
	for _, blocker := range blockers {
		if blocker.Code != "governance_action_required" {
			continue
		}
		if controlID := governanceActionControl(blocker); controlID != "" {
			return controlID
		}
	}
	return ""
}

func governanceActionControl(blocker model.ReasonCode) string {
	detail := strings.TrimSpace(blocker.Detail)
	if detail == "" {
		return ""
	}
	controlID, _, _ := strings.Cut(detail, ":")
	return strings.TrimSpace(controlID)
}

// ApplyPlanGateResult persists the explicit mutation contract returned by
// CheckGateWithIteration and reports any runtime-owned side effects.
func ApplyPlanGateResult(change *model.Change, result PlanGateResult) ([]SideEffect, error) {
	if change == nil {
		return nil, fmt.Errorf("change is required")
	}
	change.Normalize()

	sideEffects := make([]SideEffect, 0, 2)
	if change.RecordPlanAuditIterations(result.NextPlanAuditIterations) {
		sideEffects = append(sideEffects, SideEffect{
			Kind:   "updated_plan_audit_iterations",
			Detail: fmt.Sprintf("%d", result.NextPlanAuditIterations),
		})
	}

	if result.ClearLastCheckerFeedback {
		if change.ClearEvidenceRef(planAuditLastCheckerFeedbackKey) {
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
	if change.RecordEvidenceRef(planAuditLastCheckerFeedbackKey, feedback) {
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
