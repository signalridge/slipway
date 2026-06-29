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
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

const planAuditLastCheckerFeedbackKey = "plan_audit.last_checker_feedback"

var applyGovernedFileTransaction = fsutil.ApplyFileTransaction
var appendLifecycleEvent = state.AppendLifecycleEvent

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
	// blocked CLI view. Under --auto, a pending preset is auto-confirmed
	// UPGRADE-ONLY to the suggested/effective preset and advancement continues;
	// this never downgrades and never bypasses a downstream evidence gate.
	if blockers := PresetConfirmationBlockers(change); len(blockers) > 0 {
		confirmed, err := autoConfirmPendingPreset(root, &change, options.Auto, presetPolicy, options.Command)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if !confirmed {
			return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(blockers)), nil
		}
		// The confirmed preset may change effective governance posture; re-resolve
		// so the rest of advancement runs under the now-confirmed preset.
		presetPolicy, err = governance.ResolvePresetPolicy(root, change)
		if err != nil {
			return AdvanceSummary{}, err
		}
	}

	if target, ok, err := staleEvidenceRepairTarget(root, change); err != nil {
		return AdvanceSummary{}, err
	} else if ok {
		if staleEvidenceRepairDeferredToReview(change, target) {
			// S2 owns implementation evidence. Upstream intake/planning drift is
			// reviewed at S3 as plan/code/evidence alignment; requiring an S0/S1
			// evidence replay here creates an impossible forward-only dead end.
		} else if fromState != model.StateS3Review {
			// Pre-S3 (S0_INTAKE / S1_PLAN / S2_IMPLEMENT) the owning stage still
			// holds this evidence (e.g. a stale intake-clarification or
			// wave-orchestration authority). Surface its blockers directly — whose
			// remediation is the state-valid `slipway run` / `slipway evidence
			// skill` — instead of the review-alignment path, which maps to the
			// S3-only `slipway fix` and would yield fix_state_invalid here. This
			// only changes the recommended command; the stale-evidence gate stays
			// fail-closed. Only S3, where review owns plan/code/evidence alignment,
			// keeps the forward-only review-alignment path below (issues #324, #376).
			return blockedAdvanceSummary(fromState, target.Blockers), nil
		} else {
			return forwardOnlyStaleEvidenceSummary(fromState, target), nil
		}
	}

	// 1. S0_INTAKE: lightweight path — no bundle, no worktree, no wave sync.
	if fromState == model.StateS0Intake {
		return advanceIntake(root, &change, fromState)
	}

	// 1b. Reconcile a bound worktree whose recorded branch drifted from its actual
	// git branch (no git mutation) so a branch mismatch resolves via `slipway run`
	// instead of a hollow `slipway repair` dead-end. Only a pure branch mismatch on
	// an otherwise-valid dedicated worktree is reconciled; every other authenticity
	// failure stays fail-closed in the bundle/worktree gates below.
	if fromState == model.StateS2Implement && strings.TrimSpace(change.WorktreePath) != "" {
		reconciled, err := state.ReconcileWorktreeBranchBinding(root, &change)
		if err != nil {
			return AdvanceSummary{}, err
		}
		if reconciled {
			if err := state.SaveChange(root, change); err != nil {
				return AdvanceSummary{}, err
			}
		}
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

	// 3. Wave synchronization at S2_IMPLEMENT and S3_REVIEW.
	//
	// At S2 this is the implement-stage sync. At S3 it is the in-place review
	// convergence path: when the author discovers more work at review and edits
	// tasks.md (for example adds a task), the wave plan re-materializes IN PLACE
	// from the current tasks.md at the SAME run version, folding the delta into
	// the review instead of forcing a backward rescope re-walk. Unchanged-task
	// evidence is preserved; a newly added or structurally changed task surfaces
	// as still-required work and blocks S3->S4 until its evidence is recorded
	// (fail-closed). The lifecycle state is never regressed to S1/S2.
	runWaveSync := fromState == model.StateS2Implement
	if fromState == model.StateS3Review {
		// Only re-sync at review when there is genuine convergence work to absorb:
		// either tasks.md drifted from the materialized wave plan (the author added
		// a task at review → re-materialize), or the per-task evidence ledger
		// advanced past the persisted execution summary (a folded-in task's
		// evidence was just recorded → rebuild the summary so its
		// incomplete_execution_task blocker clears). A settled review keeps the
		// existing read-only S3 behavior so done-ready, light-preset, and other
		// settled review flows are not perturbed by an unconditional re-sync.
		pending, err := reviewConvergencePending(root, change)
		if err != nil {
			return AdvanceSummary{}, err
		}
		runWaveSync = pending
	}
	if runWaveSync {
		syncResult, err := SyncGovernedWaveExecution(root, change)
		if err != nil {
			return AdvanceSummary{}, err
		}
		blockers := syncResult.Blockers
		if fromState == model.StateS3Review {
			// S3 absorbs the re-materialized plan delta in place: plan/evidence
			// drift on already-evidenced tasks is realigned through the diff-scoped
			// reviewers (who re-certify against the current plan), not replayed as a
			// backward gate — the same drift tokens blockingExecutionSummaryIssues
			// treats as review input. Genuinely missing or non-passing work
			// (incomplete tasks, failed verdicts, safety-net violations) still fails
			// closed here, so a task newly discovered at review cannot reach S4
			// without its own evidence.
			blockers = absorbedAtReviewWaveBlockers(blockers)
		}
		if len(blockers) > 0 {
			return blockedAdvanceSummary(fromState, blockers), nil
		}
	}

	executionSummaryCtx, err := state.LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		return AdvanceSummary{}, err
	}
	if target := staleEvidenceRepairFromReasonCodes(change, model.ReasonCodesFromSpecs(executionSummaryCtx.Issues)); target.SkillName != "" {
		if !staleEvidenceRepairDeferredToReview(change, target) {
			return forwardOnlyStaleEvidenceSummary(fromState, target), nil
		}
	}
	if issues := blockingExecutionSummaryIssues(change, executionSummaryCtx.Issues); len(issues) > 0 {
		return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(issues)), nil
	}

	// Scope Contract gate (owned by S2_IMPLEMENT): a satisfied execution summary
	// must also satisfy the Scope Contract. Out-of-scope drift (a changed file
	// outside the plan — typically an untracked scratch file or build artifact)
	// does not invalidate the recorded wave evidence, so it blocks visibly with
	// remediation while leaving wave-orchestration/execution-summary intact;
	// re-running after the file is removed or the plan is amended advances
	// normally (issue #136). Missing task changed-file evidence, by contrast, can
	// only be repaired by re-recording in the owning stage, so when the failure
	// surfaces downstream (S3_REVIEW) it is treated as review input instead of
	// requiring a return to S2.
	//
	// When the change is still IN S2_IMPLEMENT, every scope-contract blocker —
	// drift and missing-changed-file alike — blocks visibly without clearing
	// execution-summary.yaml / wave-orchestration.yaml. Clearing those files would
	// mask scope_contract_changed_files_missing behind run_summary_missing and
	// create a loop. Preserving the summary keeps the real blocker visible to
	// validate/status/next with actionable remediation (record the task's
	// --changed-file, or record it as a verification/investigation kind, which is
	// exempt). This fails closed: the block is kept, only the masking wipe is
	// removed.
	if target, err := scopeContractRepairTarget(root, change, executionSummaryCtx.Summary); err != nil {
		return AdvanceSummary{}, err
	} else if target.SkillName != "" {
		if fromState == model.StateS2Implement {
			// In S2 the execution evidence is still owned by the implementation
			// stage. Keep the summary intact and block visibly so the implement
			// command can record a scope amendment or task evidence repair without
			// mutating lifecycle state backward.
			return blockedAdvanceSummary(fromState, target.Blockers), nil
		}
		// S3 owns plan/code/evidence alignment through the selected review peers.
		// Do not force implement/wave-orchestration to replay after review has
		// started.
	}
	if target, err := sensitiveEvidenceRepairTarget(root, change, executionSummaryCtx.Summary); err != nil {
		return AdvanceSummary{}, err
	} else if target.SkillName != "" {
		if fromState == model.StateS2Implement {
			return blockedAdvanceSummary(fromState, target.Blockers), nil
		} else {
			return forwardOnlyStaleEvidenceSummary(fromState, target), nil
		}
	}

	preTransitionSideEffects := make([]SideEffect, 0, 2)

	// Ensure the bundle directory exists for discovery changes entering
	// S1_PLAN/research; research.md itself is authored by research-orchestration.
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
	// Advancement handles stale skill evidence above through the owning evidence
	// repair path, which preserves state-valid recovery. Do not re-route stale
	// S1 research evidence through generic required-action blockers here.
	if blockers := governance.RequiredActionBlockers(change, governance.ResolveRuntimeRequiredActions(root, change, snap, false)); len(blockers) > 0 {
		return blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(blockers)), nil
	}
	reviewSelection := ReviewSkillSelectionFromControls(snap.ActiveControls)

	// 4. Skill evidence evaluation
	//
	// This gate only needs to know whether a skill is required at the current
	// state and whether it is worktree-preflight; the required evidence set is
	// derived from the state and selected review inputs. S3_REVIEW's parallel
	// review set is evaluated there, so the conventional primary skill is
	// sufficient as the non-empty/worktree-preflight gate signal here.
	nextSkillName, evidenceState := PrimaryNextSkillWithReviewSelection(change, reviewSelection)

	var passingSkills map[string]model.VerificationRecord
	researchDiscoveryEvidence := gate.DiscoveryEvidenceState{}
	// worktree-preflight is not a governance skill; its gate is checked in
	// step 5 (worktree validation). Skip governance skill evaluation when
	// the resolved next skill is worktree-preflight so the worktree gate
	// can surface the correct blocker.
	if nextSkillName != "" && nextSkillName != SkillWorktreePreflight {
		var skillBlockers []string
		verificationRecords, err := state.ListVerificationsForChange(root, change)
		if err != nil {
			return AdvanceSummary{}, err
		}
		passingSkills, skillBlockers, err = evaluateRequiredSkillsForChangeWithReviewSelectionWithRecords(
			root,
			change,
			model.WorkflowState(evidenceState),
			executionSummaryCtx.LatestRunVersion,
			reviewSelection,
			verificationRecords,
			change.PlanSubStep,
		)
		if err != nil {
			return AdvanceSummary{}, err
		}
		researchDiscoveryEvidence = researchOrchestrationEvidenceStateFromSkillBlockers(
			verificationRecords,
			skillBlockers,
		)
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
	}
	preTransitionSkillEvidence := skillEvidenceTraceFromPassing(root, change, passingSkills)

	// 5. Worktree validation (at S2_IMPLEMENT when NeedsDiscovery and worktree unbound)
	isWorktreeGateState := fromState == model.StateS2Implement
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
		scopeResult, err := EvaluateScopeGate(root, change, passingSkills, researchDiscoveryEvidence)
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

	if fromState == model.StateS3Review {
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
	}

	// 7. State transition
	// S1_PLAN substep progression: advance within planning before leaving to S2_IMPLEMENT.
	if fromState == model.StateS1Plan {
		fromSub := string(change.PlanSubStep)
		nextSub := computeNextPlanSubStep(change.PlanSubStep)
		if nextSub != model.PlanSubStepNone {
			// Advance substep within S1_PLAN.
			change.AdvancePlanSubStep(nextSub)
			var sideEffects []SideEffect
			transactionOps := make([]fsutil.FileTransactionOp, 0, 2)
			if nextSub == model.PlanSubStepBundle {
				scaffoldOps, err := governedBundleScaffoldTransactionOps(root, &change)
				if err != nil {
					return AdvanceSummary{}, err
				}
				transactionOps = append(transactionOps, scaffoldOps...)
				sideEffects = append(sideEffects, SideEffect{
					Kind:   "scaffolded_bundle",
					Detail: "governed bundle artifacts created or verified",
				})
			}
			if err := applyGovernedChangeTransaction(root, change, transactionOps); err != nil {
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
		// inline before entering S2_IMPLEMENT. If validation fails, the change
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
		// validate substep already checked by the gate above; fall through to S2_IMPLEMENT.
	}

	// D1: Block S3_REVIEW→DONE in `next`; only `slipway done` finalizes.
	if toState == model.StateDone {
		summary := doneReadyAdvanceSummary(fromState, "All governance gates passed. Run `slipway done` to finalize.")
		summary.SideEffects = append(summary.SideEffects, preTransitionSideEffects...)
		summary.SkillEvidence = append(summary.SkillEvidence, preTransitionSkillEvidence...)
		return saveChangeAndReturn(root, change, summary)
	}

	sideEffects := append([]SideEffect(nil), preTransitionSideEffects...)
	transactionOps := make([]fsutil.FileTransactionOp, 0, 2)
	if toState == model.StateS2Implement {
		generatedAt := change.CreatedAt
		if planAudit, ok := passingSkills[SkillPlanAudit]; ok && !planAudit.Timestamp.IsZero() {
			generatedAt = planAudit.Timestamp
		}
		_, wavePlanOp, err := state.MaterializeWavePlanTransactionOpAt(root, change, generatedAt)
		if err != nil {
			return AdvanceSummary{}, err
		}
		transactionOps = append(transactionOps, wavePlanOp)
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
	if len(transactionOps) > 0 {
		if err := applyGovernedChangeTransaction(root, change, transactionOps); err != nil {
			return AdvanceSummary{}, err
		}
	} else {
		if err := state.SaveChange(root, change); err != nil {
			return AdvanceSummary{}, err
		}
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

// autoPresetConfirmTarget reports the preset a pending confirmation would be
// auto-confirmed to under --auto, and whether the auto-confirm is allowed.
//
// The target is the effective preset resolved by ResolvePresetPolicy, which
// folds in project min_preset and guardrail-domain forcing, so confirming to it
// can only hold or raise governance posture. The confirm is UPGRADE-ONLY: it is
// allowed only when the effective preset rank is >= the pending suggested preset
// rank. It MUST NEVER auto-downgrade, so a (defensive) downgrade target returns
// ok=false and the caller falls through to the manual preset hard-stop.
func autoPresetConfirmTarget(change model.Change, policy governance.PresetPolicy) (model.WorkflowPreset, bool) {
	if !change.WorkflowPresetConfirmationPending() {
		return "", false
	}
	target := policy.EffectivePreset
	if !target.IsValid() {
		return "", false
	}
	// Upgrade-only: never confirm to a preset weaker than the pending suggestion.
	if target.Rank() < change.SuggestedWorkflowPreset.Rank() {
		return "", false
	}
	return target, true
}

// autoConfirmPendingPreset auto-confirms a pending workflow-preset suggestion
// when --auto is effective, replicating the manual `slipway preset` confirm
// scaffold step (cmd/preset.go) so the engine path does not ship a
// confirm-without-scaffold divergence: at S0_INTAKE only intent.md is
// materialized; otherwise the full governed bundle is scaffolded under the
// confirmed effective preset. On success the change is persisted and a
// distinct `auto_preset_confirmed` lifecycle event is appended. It returns false
// (leaving the change untouched) when auto is off, nothing is pending, or the
// confirm would be a downgrade. Guardrail domains do not disable this path:
// upgrade-only confirmation can preserve or raise governance strictness, never
// relax it.
func autoConfirmPendingPreset(root string, change *model.Change, auto bool, policy governance.PresetPolicy, command string) (bool, error) {
	if change == nil || !auto {
		return false, nil
	}
	target, ok := autoPresetConfirmTarget(*change, policy)
	if !ok {
		return false, nil
	}

	beforePreset := change.SuggestedWorkflowPreset
	change.WorkflowPreset = target
	change.SuggestedWorkflowPreset = ""

	// Persist the confirmed preset to the authority file BEFORE scaffolding, then
	// scaffold, mirroring the manual `slipway preset` confirm ordering
	// (cmd/preset.go): authority-first plus restore-on-scaffold-failure. This
	// removes the window where artifacts are scaffolded on disk while change.yaml
	// still records the preset as pending.
	if err := state.SaveChange(root, *change); err != nil {
		return false, err
	}
	if scaffoldErr := autoConfirmScaffold(root, *change, target); scaffoldErr != nil {
		// Restore the pending suggestion so a transient scaffold failure never
		// strands a confirmed authority. Auto-confirm is always a first-time
		// confirmation, so the pending state is (invalid preset, suggested=before).
		change.WorkflowPreset = ""
		change.SuggestedWorkflowPreset = beforePreset
		if restoreErr := state.SaveChange(root, *change); restoreErr != nil {
			return false, errors.Join(scaffoldErr, restoreErr)
		}
		return false, scaffoldErr
	}

	// Attribute the event to the command that drove advancement (run/intake/plan/
	// implement), not a hardcoded "advance", so traces match normal advancement.
	command = strings.TrimSpace(command)
	if command == "" {
		command = "advance"
	}
	if _, err := appendLifecycleEvent(root, *change, state.LifecycleEvent{
		Command:       command,
		EventType:     "preset.changed",
		Action:        "confirmed",
		Reason:        "auto_preset_confirmed",
		Result:        "ok",
		BeforeState:   change.CurrentState,
		AfterState:    change.CurrentState,
		BeforeSubStep: string(change.PlanSubStep),
		AfterSubStep:  string(change.PlanSubStep),
		SideEffects: []state.LifecycleSideEffect{
			{Kind: "workflow_preset_confirmed", Detail: string(change.WorkflowPreset)},
			{Kind: "auto_preset_confirmed", Detail: fmt.Sprintf("%s->%s", beforePreset, change.WorkflowPreset)},
		},
	}); err != nil {
		change.WorkflowPreset = ""
		change.SuggestedWorkflowPreset = beforePreset
		if restoreErr := state.SaveChange(root, *change); restoreErr != nil {
			return false, errors.Join(err, restoreErr)
		}
		return false, err
	}

	return true, nil
}

// autoConfirmScaffold materializes the artifacts a confirmed preset requires,
// mirroring the manual `slipway preset` scaffold step: only intent.md at
// S0_INTAKE (so a pending confirm never materializes downstream artifacts from
// empty templates), otherwise the full governed bundle under the confirmed
// effective preset.
func autoConfirmScaffold(root string, change model.Change, target model.WorkflowPreset) error {
	if change.CurrentState == model.StateS0Intake {
		return artifact.ScaffoldIntentForChange(root, change)
	}
	resolution := ResolveChangeSchemaDiagnostics(change)
	if len(resolution.Blockers) > 0 {
		return fmt.Errorf("resolve artifact schema: %s", strings.Join(resolution.Blockers, ","))
	}
	return artifact.ScaffoldGovernedBundleForChange(root, change, target, resolution.Schema)
}

// tasksPlanDriftedFromWavePlan reports whether the current tasks.md structural
// projection differs from the materialized wave plan, i.e. the plan was edited
// since it was last materialized. It is the S3_REVIEW trigger for in-place
// convergence: only a real plan delta re-materializes the wave plan at review.
// A missing wave plan reports no drift (there is nothing materialized to
// converge against); the upstream bundle gate already guarantees a readable
// tasks.md by this point, so a read error is surfaced rather than masked.
func tasksPlanDriftedFromWavePlan(root string, change model.Change) (bool, error) {
	plan, err := state.LoadOptionalWavePlanForChange(root, change)
	if err != nil {
		return false, err
	}
	if plan == nil {
		return false, nil
	}
	current, err := state.CurrentTasksPlanStructuralState(root, change)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(current) != strings.TrimSpace(wavePlanStructuralHash(*plan)), nil
}

// reviewConvergencePending reports whether S3_REVIEW has in-place convergence
// work to absorb on this run. It is true when either tasks.md structurally
// drifted from the materialized wave plan (a task was added or restructured at
// review → re-materialize the plan), or the per-task evidence ledger advanced
// past the persisted execution summary (a folded-in task's evidence was recorded
// → rebuild the summary so its incomplete_execution_task blocker clears). A
// settled review — plan matches tasks.md and the summary reflects the ledger —
// reports no pending work, preserving the read-only S3 behavior that done-ready
// and light-preset flows rely on.
func reviewConvergencePending(root string, change model.Change) (bool, error) {
	drifted, err := tasksPlanDriftedFromWavePlan(root, change)
	if err != nil {
		return false, err
	}
	if drifted {
		return true, nil
	}
	return executionSummaryTrailsTaskEvidence(root, change)
}

// executionSummaryTrailsTaskEvidence reports whether the per-task evidence ledger
// records a task at the active run version that the persisted execution summary
// does not yet reflect — i.e. evidence for a folded-in task was recorded since
// the summary was last built. Re-syncing absorbs it and clears that task's
// incomplete_execution_task blocker. A missing summary, or a ledger already
// reflected in the summary, reports no trailing work so settled reviews stay
// read-only.
func executionSummaryTrailsTaskEvidence(root string, change model.Change) (bool, error) {
	summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return false, err
	}
	if summary == nil || summary.RunSummaryVersion < 1 {
		return false, nil
	}
	ledgerTasks, _, err := LoadExecutionTasksFromEvidence(root, change.Slug, summary.RunSummaryVersion)
	if err != nil {
		return false, err
	}
	runMap := summary.TaskRunMap()
	for _, task := range ledgerTasks {
		if _, ok := runMap[task.TaskID]; !ok {
			return true, nil
		}
	}
	return false, nil
}

func forwardOnlyStaleEvidenceSummary(fromState model.WorkflowState, target EvidenceRepairTarget) AdvanceSummary {
	blockers := append([]model.ReasonCode{}, target.Blockers...)
	if strings.TrimSpace(target.SkillName) != "" {
		blockers = append(blockers, model.NewReasonCode("review_alignment_required", target.SkillName))
	}
	summary := blockedAdvanceSummary(fromState, model.NormalizeReasonCodes(blockers))
	summary.Reason = "stale_evidence_requires_review_alignment"
	summary.Message = "Stale evidence must be realigned through review convergence; the lifecycle state was not changed."
	return summary
}

// reviewAbsorbedDriftToken reports whether a blocker code is plan/evidence drift
// that S3_REVIEW absorbs as review input rather than blocking on. At review the
// diff-scoped reviewers re-certify the bundle against the current plan, so a
// tasks.md edit (or the stale planning/execution evidence it produces) is
// realigned through review convergence instead of a backward replay. Missing or
// non-passing work is NOT in this set and continues to fail closed.
func reviewAbsorbedDriftToken(code string) bool {
	switch strings.TrimSpace(code) {
	case state.StalePlanningEvidenceBlockerToken,
		state.StaleExecutionEvidenceBlockerToken,
		"tasks_plan_changed_since_task_evidence":
		return true
	default:
		return false
	}
}

func blockingExecutionSummaryIssues(change model.Change, issues []string) []string {
	if change.CurrentState != model.StateS3Review {
		return issues
	}
	filtered := make([]string, 0, len(issues))
	for _, issue := range issues {
		reason := model.ReasonCodesFromSpecs([]string{issue})
		if len(reason) == 0 {
			continue
		}
		if reviewAbsorbedDriftToken(reason[0].Code) {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

// absorbedAtReviewWaveBlockers drops the in-place-absorbed plan/evidence drift
// tokens from a wave-sync result at S3_REVIEW, mirroring
// blockingExecutionSummaryIssues so the re-materialize path and the
// execution-summary path treat review drift identically. Hard blockers
// (incomplete tasks, non-pass verdicts, safety-net violations) are preserved so
// review convergence still fails closed on genuinely missing or broken work.
func absorbedAtReviewWaveBlockers(blockers []model.ReasonCode) []model.ReasonCode {
	filtered := make([]model.ReasonCode, 0, len(blockers))
	for _, blocker := range blockers {
		if reviewAbsorbedDriftToken(blocker.Code) {
			continue
		}
		filtered = append(filtered, blocker)
	}
	return filtered
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
	if fromState != model.StateS2Implement || !change.NeedsDiscovery || strings.TrimSpace(change.WorktreePath) != "" {
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

func governedBundleScaffoldTransactionOps(root string, change *model.Change) ([]fsutil.FileTransactionOp, error) {
	if change == nil {
		return nil, fmt.Errorf("change is required")
	}
	resolution := ResolveChangeSchemaDiagnostics(*change)
	if len(resolution.Blockers) > 0 {
		return nil, fmt.Errorf("resolve artifact schema: %s", strings.Join(resolution.Blockers, ","))
	}
	policy, err := governance.ResolvePresetPolicy(root, *change)
	if err != nil {
		return nil, err
	}
	return artifact.ScaffoldGovernedBundleTransactionOpsForChange(root, *change, policy.EffectivePreset, resolution.Schema)
}

func applyGovernedChangeTransaction(root string, change model.Change, ops []fsutil.FileTransactionOp) error {
	changeOps, err := state.SaveChangeTransactionOps(root, change)
	if err != nil {
		return err
	}
	transactionOps := make([]fsutil.FileTransactionOp, 0, len(ops)+len(changeOps))
	transactionOps = append(transactionOps, ops...)
	transactionOps = append(transactionOps, changeOps...)
	return applyGovernedFileTransaction(transactionOps)
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
	// Resolve the preset policy so the plan gate's plan-audit self-audit edge
	// enforces on standard/strict and stays advisory on light. A resolution
	// failure falls back to the zero policy (EffectivePreset != light), which
	// fails closed rather than silently relaxing the edge.
	presetPolicy, _ := governance.ResolvePresetPolicy(root, change)
	result := EvaluatePlanGate(root, change, passingSkills, presetPolicy)
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
		blockers = append(blockers, model.NewReasonCode("plan_audit_budget_exhausted", "manual plan revision required before continuing"))
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
	if _, err := appendLifecycleEvent(root, after, event); err != nil {
		return err
	}
	for _, derived := range derivedAdvanceLifecycleEvents(root, after, event, summary) {
		if _, err := appendLifecycleEvent(root, after, derived); err != nil {
			return err
		}
	}
	return nil
}

func advanceLifecycleEventType(summary AdvanceSummary) string {
	switch summary.Action {
	case "advanced":
		if isSubstepAdvance(summary) {
			return "state.substep_transitioned"
		}
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

func isSubstepAdvance(summary AdvanceSummary) bool {
	return summary.FromState != "" &&
		summary.FromState == summary.ToState &&
		summary.FromSubStep != "" &&
		summary.ToSubStep != "" &&
		summary.FromSubStep != summary.ToSubStep
}

func advanceGateID(before model.Change, summary AdvanceSummary) string {
	if summary.Action != "blocked" && summary.Action != "done_ready" {
		return ""
	}
	substep := before.PlanSubStep
	if summary.FromSubStep != "" {
		substep = model.PlanSubStep(summary.FromSubStep)
	}
	return OwningAdvanceGateID(before.CurrentState, substep)
}

// OwningAdvanceGateID reports the gate that owns advancement out of the given
// (state, plan substep), or "" when no gate owns advancement there. S0_INTAKE
// and S2_IMPLEMENT advance on run/evidence pacing rather than on a gate that
// can dead-end, so they own no advance gate. It is the pure (state, substep)
// projection of advanceGateID's ownership switch, exported so surfaces like
// `next` can ask which gate — if blocked by a genuine dead-end — must override a
// "ready to advance" posture without duplicating the ownership map.
func OwningAdvanceGateID(state model.WorkflowState, substep model.PlanSubStep) string {
	switch state {
	case model.StateS1Plan:
		switch substep {
		case model.PlanSubStepResearch:
			return string(gate.GateScope)
		case model.PlanSubStepAudit, model.PlanSubStepValidate:
			return string(gate.GatePlan)
		default:
			return string(gate.GatePlan)
		}
	case model.StateS3Review:
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

func EvaluatePlanGate(root string, change model.Change, passingSkills map[string]model.VerificationRecord, presetPolicy governance.PresetPolicy) gate.GateEvaluation {
	var planBlockers []model.ReasonCode
	if record, ok := passingSkills[SkillPlanAudit]; ok {
		// Local plan-audit self-audit edge: the plan-audit record must attest a
		// well-formed plan_origin and a well-formed audit_origin produced under
		// distinct fresh contexts. A missing handle, or an author context that
		// also audited its own bundle (plan_origin == audit_origin), fails closed
		// at error severity on standard/strict and is advisory on light. This is a
		// single local edge over one record's two handles; it adds no cross-stage
		// rung whose other endpoint is absent at S1.
		enforced := presetPolicy.EffectivePreset != model.WorkflowPresetLight
		planBlockers = append(planBlockers, planAuditOriginHandleBlockers(record, enforced)...)
	}
	bundleReady := CheckGovernedBundleReady(root, change)
	checklistBlockers := ValidateTasksChecklistDetailed(root, change).Blockers
	if len(checklistBlockers) > 0 {
		bundleReady = false
		planBlockers = append(planBlockers, model.ReasonCodesFromSpecs(checklistBlockers)...)
	}
	decisionBlockers := DecisionContractBlockers(root, change)
	if len(decisionBlockers) > 0 {
		bundleReady = false
		planBlockers = append(planBlockers, model.ReasonCodesFromSpecs(decisionBlockers)...)
	}
	return gate.EvaluateGPlan(bundleReady, planBlockers)
}

// planAuditOriginHandleBlockers enforces the local plan-audit self-audit edge on
// a present plan-audit record: it must carry a well-formed plan_origin and a
// well-formed audit_origin, and the two handles must differ. A missing handle,
// or plan_origin == audit_origin, returns a single plan_audit_origin_invalid
// blocker (error severity per the reason-code registry). When the edge is not
// enforced (light preset) it is advisory and returns nil. The check is local to
// one record's two handles and intentionally adds no cross-stage rung.
func planAuditOriginHandleBlockers(record model.VerificationRecord, enforced bool) []model.ReasonCode {
	if !enforced {
		return nil
	}
	planOrigin, planOK := model.PlanOriginHandleFromVerification(record)
	if !planOK {
		return []model.ReasonCode{planAuditOriginInvalidBlocker(
			SkillPlanAudit + " recorded no well-formed " + model.StageContextPlanOrigin + " handle",
		)}
	}
	auditOrigin, auditOK := model.AuditOriginHandleFromVerification(record)
	if !auditOK {
		return []model.ReasonCode{planAuditOriginInvalidBlocker(
			SkillPlanAudit + " recorded no well-formed " + model.StageContextAuditOrigin + " handle",
		)}
	}
	if planOrigin.Handle == auditOrigin.Handle {
		return []model.ReasonCode{planAuditOriginInvalidBlocker(
			SkillPlanAudit + " recorded the same " + model.StageContextPlanOrigin + " and " +
				model.StageContextAuditOrigin + " handle; the auditor must review under a distinct fresh context",
		)}
	}
	return nil
}

func planAuditOriginInvalidBlocker(detail string) model.ReasonCode {
	return model.NewReasonCode("plan_audit_origin_invalid", strings.TrimSpace(detail))
}

func EvaluateShipGate(root string, change model.Change) (gate.GateEvaluation, error) {
	shipAuthority, err := EvaluateShipAuthority(root, change)
	if err != nil {
		return gate.GateEvaluation{}, err
	}
	return shipAuthority.Result, nil
}

func EvaluateScopeGate(
	root string,
	change model.Change,
	passingSkills map[string]model.VerificationRecord,
	discoveryEvidence gate.DiscoveryEvidenceState,
) (gate.GateEvaluation, error) {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return gate.GateEvaluation{}, err
	}
	researchPath := filepath.Join(paths.GovernedBundleDir, "research.md")
	researchContent := ""
	var researchArtifactReasons []model.ReasonCode
	if data, err := os.ReadFile(researchPath); err == nil { // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
		researchContent = string(data)
	} else if os.IsNotExist(err) {
		researchArtifactReasons = append(researchArtifactReasons,
			model.NewReasonCode("missing_required_artifact", "research.md"))
	} else {
		researchArtifactReasons = append(researchArtifactReasons,
			model.NewReasonCode("required_artifact_unreadable", "research.md"))
	}

	_, discoveryOK := passingSkills[SkillResearchOrchestration]

	// Worktree validation is only meaningful when a worktree is already bound.
	// G_scope runs at S1_PLAN/research (before worktree creation at S2_IMPLEMENT),
	// so skip worktree validation when WorktreePath is empty.
	var worktreeReasons []model.ReasonCode
	if strings.TrimSpace(change.WorktreePath) != "" {
		var wtErr error
		worktreeReasons, wtErr = state.ValidateDedicatedWorktreeAuthenticityReasons(root, change.WorktreePath, change.WorktreeBranch)
		if wtErr != nil {
			return gate.GateEvaluation{}, wtErr
		}
	}
	return gate.EvaluateGScope(change, researchContent, discoveryOK, worktreeReasons, discoveryEvidence, researchArtifactReasons...), nil
}
