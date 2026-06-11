package progression

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/action"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/scopecontract"
	"github.com/signalridge/slipway/internal/engine/sensitiveevidence"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

type StaleEvidenceTarget struct {
	SkillName   string
	State       model.WorkflowState
	PlanSubStep model.PlanSubStep
	Blockers    []model.ReasonCode
}

func (t StaleEvidenceTarget) Label() string {
	return staleEvidenceAuthorityLabel(t.State, t.PlanSubStep)
}

type staleEvidenceAuthority struct {
	SkillName   string
	State       model.WorkflowState
	PlanSubStep model.PlanSubStep
	position    staleEvidencePosition
}

type staleEvidencePosition struct {
	stateIndex int
	subRank    int
}

func StaleEvidenceRecoveryAvailable(
	root string,
	change model.Change,
	blockers []model.ReasonCode,
) (StaleEvidenceTarget, bool, error) {
	if target, ok, err := staleReopenTarget(root, change); err != nil || ok {
		return target, ok, err
	}
	target := staleReopenFromReasonCodes(change, blockers)
	return target, target.SkillName != "", nil
}

func staleReopenTarget(root string, change model.Change) (StaleEvidenceTarget, bool, error) {
	authorities, err := staleEvidenceAuthorities(root, change, true)
	if err != nil {
		return StaleEvidenceTarget{}, false, err
	}
	current := currentStaleEvidencePosition(change)
	for _, authority := range authorities {
		if compareStaleEvidencePosition(authority.position, current) > 0 {
			continue
		}
		record, err := state.LoadVerification(root, change.Slug, authority.SkillName)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
				continue
			}
			return StaleEvidenceTarget{}, false, err
		}
		if !record.IsPassing() {
			continue
		}
		blockers, err := skillDigestFreshnessBlockers(root, change, authority.SkillName)
		if err != nil {
			return StaleEvidenceTarget{}, false, err
		}
		staleBlockers := staleSkillReasonCodes(authority.SkillName, blockers)
		if len(staleBlockers) == 0 {
			continue
		}
		return StaleEvidenceTarget{
			SkillName:   authority.SkillName,
			State:       authority.State,
			PlanSubStep: authority.PlanSubStep,
			Blockers:    staleBlockers,
		}, true, nil
	}
	return StaleEvidenceTarget{}, false, nil
}

func staleReopenFromReasonCodes(change model.Change, blockers []model.ReasonCode) StaleEvidenceTarget {
	if compareStaleEvidencePosition(
		currentStaleEvidencePosition(change),
		staleEvidencePositionFor(model.StateS1Plan, model.PlanSubStepAudit),
	) < 0 {
		return StaleEvidenceTarget{}
	}
	for _, blocker := range model.NormalizeReasonCodes(blockers) {
		if strings.TrimSpace(blocker.Code) != state.StalePlanningEvidenceBlockerToken {
			continue
		}
		return StaleEvidenceTarget{
			SkillName:   SkillPlanAudit,
			State:       model.StateS1Plan,
			PlanSubStep: model.PlanSubStepAudit,
			Blockers:    []model.ReasonCode{blocker},
		}
	}
	return StaleEvidenceTarget{}
}

func reopenToStaleStage(
	root string,
	change *model.Change,
	target StaleEvidenceTarget,
	fromState model.WorkflowState,
) (AdvanceSummary, error) {
	if change == nil {
		return AdvanceSummary{}, fmt.Errorf("change is required for stale evidence recovery")
	}
	paths, err := state.ResolveChangePaths(root, *change)
	if err != nil {
		return AdvanceSummary{}, err
	}
	authorities, err := staleEvidenceAuthorities(root, *change, false)
	if err != nil {
		return AdvanceSummary{}, err
	}
	targetPosition := staleEvidencePositionFor(target.State, target.PlanSubStep)
	verificationDir := filepath.Join(paths.GovernedBundleDir, "verification")
	sideEffects := []SideEffect{}
	clearedSkills := []string{}
	for _, authority := range authorities {
		if compareStaleEvidencePosition(authority.position, targetPosition) < 0 {
			continue
		}
		path := filepath.Join(verificationDir, authority.SkillName+".yaml")
		removed, err := clearRecoveryEvidence(root, *change, recoveryEvidenceFile{
			path:  path,
			skill: authority.SkillName,
		})
		if err != nil {
			return AdvanceSummary{}, err
		}
		if removed {
			clearedSkills = append(clearedSkills, authority.SkillName)
			sideEffects = append(sideEffects, SideEffect{
				Kind:   "cleared_stale_verification",
				Detail: state.DisplayPath(root, path),
			})
		}
	}
	if shouldClearWavePlan(targetPosition) {
		path := filepath.Join(verificationDir, state.WavePlanFileName)
		removed, err := clearRecoveryEvidence(root, *change, recoveryEvidenceFile{path: path})
		if err != nil {
			return AdvanceSummary{}, err
		}
		if removed {
			sideEffects = append(sideEffects, SideEffect{
				Kind:   "cleared_stale_generated_evidence",
				Detail: state.DisplayPath(root, path),
			})
		}
	}
	if shouldClearExecutionSummary(targetPosition) {
		path := filepath.Join(verificationDir, state.ExecutionSummaryFileName)
		removed, err := clearRecoveryEvidence(root, *change, recoveryEvidenceFile{path: path})
		if err != nil {
			return AdvanceSummary{}, err
		}
		if removed {
			sideEffects = append(sideEffects, SideEffect{
				Kind:   "cleared_stale_generated_evidence",
				Detail: state.DisplayPath(root, path),
			})
		}
	}
	sideEffects = append(sideEffects, SideEffect{
		Kind:   "preserved_runtime_execution_evidence",
		Detail: state.DisplayPath(root, state.ChangeDir(root, change.Slug)),
	})

	cleared := change.TransitionTo(target.State)
	toSubStep := string(target.PlanSubStep)
	switch target.State {
	case model.StateS1Plan:
		if target.PlanSubStep != model.PlanSubStepNone && change.PlanSubStep != target.PlanSubStep {
			change.AdvancePlanSubStep(target.PlanSubStep)
			cleared = append(cleared, "plan_substep")
		}
	case model.StateS0Intake:
		// Re-walk the intake phase from its entry substep (clarify), where
		// intake-clarification runs. TransitionTo only seeds the entry substep
		// from IntakeSubStepNone, so a reopen from a machine-only substep
		// (research/confirm) would otherwise strand the change on a substep with
		// no routable skill — the #90 dead-end. Resetting to the entry substep
		// restores the normal intake-clarification handoff.
		if change.IntakeSubStep != model.IntakeEntrySubStep() {
			change.AdvanceIntakeSubStep(model.IntakeEntrySubStep())
			cleared = append(cleared, "intake_substep")
		}
		toSubStep = string(change.IntakeSubStep)
	}
	if change.ClearAutoPassHistory() {
		cleared = append(cleared, "last_auto_passed_states")
	}
	if change.ClearActiveCheckpoint() {
		cleared = append(cleared, "active_checkpoint")
	}
	if target.State == model.StateS0Intake {
		if change.ResetEvidenceRefs() {
			cleared = append(cleared, "evidence_refs")
		}
	} else {
		for _, skillName := range stringutil.UniqueSorted(clearedSkills) {
			if change.ClearEvidenceRef(skillName) {
				cleared = append(cleared, "evidence_refs."+skillName)
			}
		}
	}
	if compareStaleEvidencePosition(
		targetPosition,
		staleEvidencePositionFor(model.StateS1Plan, model.PlanSubStepAudit),
	) <= 0 {
		if change.ResetPlanAuditIterations() {
			cleared = append(cleared, "plan_audit_iterations")
		}
		if change.ClearEvidenceRef(planAuditLastCheckerFeedbackKey) {
			cleared = append(cleared, "evidence_refs."+planAuditLastCheckerFeedbackKey)
		}
	}

	if err := state.SaveChange(root, *change); err != nil {
		return AdvanceSummary{}, err
	}
	return AdvanceSummary{
		Action:        "advanced",
		FromState:     fromState,
		ToState:       target.State,
		ToSubStep:     toSubStep,
		Reason:        "stale_evidence_recovery_started",
		RecoveryOnly:  true,
		SideEffects:   sideEffects,
		ClearedFields: stringutil.UniqueSorted(cleared),
		Message:       "Reopened " + target.Label() + " for stale evidence recovery.",
		Blockers:      target.Blockers,
	}, nil
}

func staleEvidenceAuthorities(root string, change model.Change, requiredOnly bool) ([]staleEvidenceAuthority, error) {
	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return nil, err
	}
	closeoutRequired := true
	if requiredOnly {
		policy, err := governance.ResolvePresetPolicy(root, change)
		if err != nil {
			return nil, err
		}
		closeoutRequired = FinalCloseoutEvidenceRequired(policy)
	}
	authorities := make([]staleEvidenceAuthority, 0, len(registry))
	for _, def := range registry {
		if requiredOnly && !staleEvidenceDefinitionApplies(change, def, closeoutRequired) {
			continue
		}
		position, ok := staleEvidencePositionForDefinition(def)
		if !ok {
			continue
		}
		authorities = append(authorities, staleEvidenceAuthority{
			SkillName:   strings.TrimSpace(def.Name),
			State:       def.State,
			PlanSubStep: def.PlanSubStep,
			position:    position,
		})
	}
	slices.SortFunc(authorities, compareStaleEvidenceAuthority)
	return authorities, nil
}

func staleEvidenceDefinitionApplies(change model.Change, def skill.Definition, closeoutRequired bool) bool {
	if def.DiscoveryOnly && !change.NeedsDiscovery {
		return false
	}
	if def.CloseoutConditional && !closeoutRequired {
		return false
	}
	if def.Name == SkillCodeQualityReview && !change.EffectiveWorkflowProfile().RequiresCodeQualityReview() {
		return false
	}
	return true
}

func staleEvidencePositionForDefinition(def skill.Definition) (staleEvidencePosition, bool) {
	if strings.TrimSpace(def.Name) == "" {
		return staleEvidencePosition{}, false
	}
	return staleEvidencePositionFor(def.State, def.PlanSubStep), true
}

func currentStaleEvidencePosition(change model.Change) staleEvidencePosition {
	return staleEvidencePositionFor(change.CurrentState, change.PlanSubStep)
}

func staleEvidencePositionFor(workflowState model.WorkflowState, subStep model.PlanSubStep) staleEvidencePosition {
	return staleEvidencePosition{
		stateIndex: workflowStateIndex(workflowState),
		subRank:    stalePlanSubStepRank(subStep),
	}
}

func workflowStateIndex(workflowState model.WorkflowState) int {
	path := action.WorkflowPath(true)
	idx := slices.Index(path, workflowState)
	if idx >= 0 {
		return idx
	}
	return len(path)
}

// stalePlanSubStepRank ranks an S1_PLAN substep for earliest-affected ordering.
// The forward substeps (research, bundle, audit) derive their rank directly from
// planSubStepOrder, so the stale-recovery ordering tracks the canonical planning
// progression in lockstep — there is no parallel hand-maintained rank table
// (REQ-003). `validate` is the terminal post-audit substep that exits S1_PLAN; it
// is intentionally absent from planSubStepOrder (the forward progression) and is
// ranked explicitly immediately after it. PlanSubStepNone — the value carried by
// every S0/S2/S3/S4 authority, whose order is already fixed by its workflow state
// — ranks first because it never tie-breaks against a real S1 substep.
func stalePlanSubStepRank(subStep model.PlanSubStep) int {
	switch subStep {
	case model.PlanSubStepNone:
		return 0
	case model.PlanSubStepValidate:
		return len(planSubStepOrder)
	default:
		if idx := slices.Index(planSubStepOrder, subStep); idx >= 0 {
			return idx
		}
		return len(planSubStepOrder)
	}
}

func compareStaleEvidenceAuthority(a, b staleEvidenceAuthority) int {
	if cmp := compareStaleEvidencePosition(a.position, b.position); cmp != 0 {
		return cmp
	}
	primaryA := primaryAuthoritySkill(a)
	primaryB := primaryAuthoritySkill(b)
	if a.SkillName == primaryA && b.SkillName != primaryB {
		return -1
	}
	if a.SkillName != primaryA && b.SkillName == primaryB {
		return 1
	}
	return strings.Compare(a.SkillName, b.SkillName)
}

func primaryAuthoritySkill(authority staleEvidenceAuthority) string {
	change := model.NewChange("_")
	change.CurrentState = authority.State
	change.PlanSubStep = authority.PlanSubStep
	change.WorktreePath = "."
	skillName, _ := ResolveNextSkill(change)
	return skillName
}

func compareStaleEvidencePosition(a, b staleEvidencePosition) int {
	if a.stateIndex < b.stateIndex {
		return -1
	}
	if a.stateIndex > b.stateIndex {
		return 1
	}
	if a.subRank < b.subRank {
		return -1
	}
	if a.subRank > b.subRank {
		return 1
	}
	return 0
}

func staleSkillReasonCodes(skillName string, blockers []string) []model.ReasonCode {
	prefix := "required_skill_stale:" + strings.TrimSpace(skillName) + ":"
	out := []model.ReasonCode{}
	for _, blocker := range blockers {
		blocker = strings.TrimSpace(blocker)
		if !strings.HasPrefix(blocker, prefix) {
			continue
		}
		reason := model.ReasonCodeFromSpec(blocker)
		if staleEvidenceRecoveryDetail(reason.Detail) {
			out = append(out, reason)
		}
	}
	return model.NormalizeReasonCodes(out)
}

func staleEvidenceRecoveryDetail(detail string) bool {
	detail = strings.TrimSpace(detail)
	switch {
	case detail == "":
		return false
	case detail == "input_digest_unavailable" || strings.HasSuffix(detail, ":input_digest_unavailable"):
		return false
	case detail == "input_digest_missing" || strings.HasSuffix(detail, ":input_digest_missing"):
		return false
	default:
		return true
	}
}

func shouldClearWavePlan(target staleEvidencePosition) bool {
	return compareStaleEvidencePosition(target, staleEvidencePositionFor(model.StateS1Plan, model.PlanSubStepAudit)) <= 0
}

func shouldClearExecutionSummary(target staleEvidencePosition) bool {
	return compareStaleEvidencePosition(target, staleEvidencePositionFor(model.StateS2Execute, model.PlanSubStepNone)) <= 0
}

func staleEvidenceAuthorityLabel(workflowState model.WorkflowState, subStep model.PlanSubStep) string {
	if workflowState == model.StateS1Plan && subStep != model.PlanSubStepNone {
		return string(workflowState) + "/" + string(subStep)
	}
	return string(workflowState)
}

// scopeContractReopenTarget returns an S2_EXECUTE reopen target when a satisfied
// execution summary nonetheless fails the Scope Contract. The Scope Contract is
// owned by S2_EXECUTE (it scores the wave-execution evidence), but it can only
// be evaluated once the run summary exists — which is produced at the moment the
// change advances out of S2. Without this gate the failure first surfaces in
// S3_REVIEW, where task evidence can no longer be recorded, stranding the change
// with a recovery hint that points at a command that cannot fix it. Reopening to
// S2_EXECUTE makes the failure fail closed to a rerun of wave-orchestration in
// its owning stage. Returns the zero target when the summary is not ready, the
// change has not yet reached S2_EXECUTE, evaluation errors (surfaced via
// readiness), or the contract passes.
func scopeContractReopenTarget(root string, change model.Change, summary *model.ExecutionSummary) (StaleEvidenceTarget, error) {
	if !state.ExecutionSummaryReady(summary) {
		return StaleEvidenceTarget{}, nil
	}
	if compareStaleEvidencePosition(
		currentStaleEvidencePosition(change),
		staleEvidencePositionFor(model.StateS2Execute, model.PlanSubStepNone),
	) < 0 {
		return StaleEvidenceTarget{}, nil
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return StaleEvidenceTarget{}, err
	}
	report, err := scopecontract.EvaluateBundleWithChangedFiles(
		paths.GovernedBundleDir,
		summary,
		scopeContractWorkspaceChangedFiles(paths),
	)
	if err != nil || len(report.Blockers) == 0 {
		return StaleEvidenceTarget{}, nil
	}
	return StaleEvidenceTarget{
		SkillName:   SkillWaveOrchestration,
		State:       model.StateS2Execute,
		PlanSubStep: model.PlanSubStepNone,
		Blockers:    report.Blockers,
	}, nil
}

func sensitiveEvidenceReopenTarget(root string, change model.Change, summary *model.ExecutionSummary) (StaleEvidenceTarget, error) {
	if !state.ExecutionSummaryReady(summary) {
		return StaleEvidenceTarget{}, nil
	}
	if compareStaleEvidencePosition(
		currentStaleEvidencePosition(change),
		staleEvidencePositionFor(model.StateS2Execute, model.PlanSubStepNone),
	) < 0 {
		return StaleEvidenceTarget{}, nil
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return StaleEvidenceTarget{}, err
	}
	report := sensitiveevidence.Evaluate(summary, scopeContractWorkspaceChangedFiles(paths))
	if len(report.Blockers) == 0 {
		return StaleEvidenceTarget{}, nil
	}
	return StaleEvidenceTarget{
		SkillName:   SkillWaveOrchestration,
		State:       model.StateS2Execute,
		PlanSubStep: model.PlanSubStepNone,
		Blockers:    report.Blockers,
	}, nil
}

// scopeContractDriftOnly reports whether every scope-contract blocker is
// out-of-scope drift (a changed file outside the plan) rather than missing task
// changed-file evidence. Drift is non-destructive: the recorded wave evidence is
// still valid — the worktree merely holds a file outside the plan (e.g. an
// untracked scratch file or build artifact). Such a case must block visibly with
// remediation, not silently clear wave-orchestration/execution-summary evidence
// and reopen in place (issue #136). Missing task changed-file evidence
// (scope_contract_changed_files_missing / scope_contract_missing), by contrast,
// can only be repaired by re-recording in S2_EXECUTE, so it still reopens.
func scopeContractDriftOnly(blockers []model.ReasonCode) bool {
	if len(blockers) == 0 {
		return false
	}
	for _, blocker := range blockers {
		if blocker.Code != scopecontract.ReasonScopeContractDrift {
			return false
		}
	}
	return true
}
