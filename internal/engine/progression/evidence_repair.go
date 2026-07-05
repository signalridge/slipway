package progression

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/signalridge/slipway/internal/engine/action"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/scopecontract"
	"github.com/signalridge/slipway/internal/engine/sensitiveevidence"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

type EvidenceRepairTarget struct {
	SkillName   string
	State       model.WorkflowState
	PlanSubStep model.PlanSubStep
	Blockers    []model.ReasonCode
}

func (t EvidenceRepairTarget) Label() string {
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

func (p staleEvidencePosition) workflowState() model.WorkflowState {
	path := action.WorkflowPath(true)
	if p.stateIndex >= 0 && p.stateIndex < len(path) {
		return path[p.stateIndex]
	}
	return ""
}

func (p staleEvidencePosition) planSubStep() model.PlanSubStep {
	if p.workflowState() != model.StateS1Plan {
		return model.PlanSubStepNone
	}
	if p.subRank >= 0 && p.subRank < len(planSubStepOrder) {
		return planSubStepOrder[p.subRank]
	}
	if p.subRank == len(planSubStepOrder) {
		return model.PlanSubStepValidate
	}
	return model.PlanSubStepNone
}

func StaleEvidenceRepairAvailable(
	root string,
	change model.Change,
	blockers []model.ReasonCode,
) (EvidenceRepairTarget, bool, error) {
	if target, ok, err := staleEvidenceRepairTarget(root, change); err != nil || ok {
		if ok && staleEvidenceRepairDeferredToReview(change, target) {
			return EvidenceRepairTarget{}, false, nil
		}
		return target, ok, err
	}
	target := staleEvidenceRepairFromReasonCodes(change, blockers)
	if target.SkillName != "" && staleEvidenceRepairDeferredToReview(change, target) {
		return EvidenceRepairTarget{}, false, nil
	}
	return target, target.SkillName != "", nil
}

func staleEvidenceRepairTarget(root string, change model.Change) (EvidenceRepairTarget, bool, error) {
	authorities, err := staleEvidenceAuthorities(root, change, true)
	if err != nil {
		return EvidenceRepairTarget{}, false, err
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
			return EvidenceRepairTarget{}, false, err
		}
		if !record.IsPassing() {
			continue
		}
		if authority.SkillName == SkillIntakeClarification {
			superseded, err := freshPlanAuditSupersedesIntakeDrift(root, change)
			if err != nil {
				return EvidenceRepairTarget{}, false, err
			}
			if superseded {
				continue
			}
		}
		blockers, err := skillDigestFreshnessBlockers(root, change, authority.SkillName)
		if err != nil {
			return EvidenceRepairTarget{}, false, err
		}
		staleBlockers := staleSkillReasonCodes(authority.SkillName, blockers)
		if len(staleBlockers) == 0 {
			continue
		}
		return EvidenceRepairTarget{
			SkillName:   authority.SkillName,
			State:       authority.State,
			PlanSubStep: authority.PlanSubStep,
			Blockers:    staleBlockers,
		}, true, nil
	}
	return EvidenceRepairTarget{}, false, nil
}

func freshPlanAuditSupersedesIntakeDrift(root string, change model.Change) (bool, error) {
	if compareStaleEvidencePosition(
		currentStaleEvidencePosition(change),
		staleEvidencePositionFor(model.StateS1Plan, model.PlanSubStepAudit),
	) < 0 {
		return false, nil
	}
	record, err := state.LoadVerification(root, change.Slug, SkillPlanAudit)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !record.IsPassing() {
		return false, nil
	}
	digests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
	if err != nil {
		return false, err
	}
	if digests == nil {
		return false, nil
	}
	digests.Normalize()
	stored, ok := digests.Skills[SkillPlanAudit]
	if !ok {
		return false, nil
	}
	current, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
	if err != nil {
		if digestStampUnavailable(err) {
			return false, nil
		}
		return false, err
	}
	fresh, _ := model.EvidenceFreshness(stored, current.Inputs)
	return fresh, nil
}

func staleEvidenceRepairFromReasonCodes(change model.Change, blockers []model.ReasonCode) EvidenceRepairTarget {
	if compareStaleEvidencePosition(
		currentStaleEvidencePosition(change),
		staleEvidencePositionFor(model.StateS1Plan, model.PlanSubStepAudit),
	) < 0 {
		return EvidenceRepairTarget{}
	}
	for _, blocker := range model.NormalizeReasonCodes(blockers) {
		if !state.StalePlanningEvidenceBlocker(blocker) {
			continue
		}
		return EvidenceRepairTarget{
			SkillName:   SkillPlanAudit,
			State:       model.StateS1Plan,
			PlanSubStep: model.PlanSubStepAudit,
			Blockers:    []model.ReasonCode{blocker},
		}
	}
	return EvidenceRepairTarget{}
}

func staleEvidenceRepairDeferredToReview(change model.Change, target EvidenceRepairTarget) bool {
	currentState := change.CurrentState
	if currentState != model.StateS2Implement && currentState != model.StateS3Review {
		return false
	}
	return compareStaleEvidencePosition(
		staleEvidencePositionFor(target.State, target.PlanSubStep),
		staleEvidencePositionFor(currentState, model.PlanSubStepNone),
	) < 0
}

func staleEvidenceAuthorities(root string, change model.Change, requiredOnly bool) ([]staleEvidenceAuthority, error) {
	registry, err := skill.LoadGovernanceRegistry(root)
	if err != nil {
		return nil, err
	}
	reviewSelection := skill.ReviewSkillSelection{}
	if requiredOnly {
		var err error
		reviewSelection, err = reviewSkillSelectionForRepair(root, change)
		if err != nil {
			return nil, err
		}
	}
	authorities := make([]staleEvidenceAuthority, 0, len(registry))
	for _, def := range registry {
		if requiredOnly && !staleEvidenceDefinitionApplies(change, def, reviewSelection) {
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

func reviewSkillSelectionForRepair(root string, change model.Change) (skill.ReviewSkillSelection, error) {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return skill.ReviewSkillSelection{}, err
	}
	snap, err := governance.PreviewGovernanceSnapshot(root, change, paths.GovernedBundleDir)
	if err != nil {
		return skill.ReviewSkillSelection{}, err
	}
	return ReviewSkillSelectionFromControls(snap.ActiveControls), nil
}

func staleEvidenceDefinitionApplies(
	change model.Change,
	def skill.Definition,
	reviewSelection skill.ReviewSkillSelection,
) bool {
	if def.DiscoveryOnly && !change.NeedsDiscovery {
		return false
	}
	if def.State == model.StateS3Review && skill.IsReviewSkill(def.Name) && !skill.ReviewSkillSelected(def.Name, reviewSelection) {
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
// planSubStepOrder, so the stale repair ordering tracks the canonical planning
// progression in lockstep — there is no parallel hand-maintained rank table
// (REQ-003). `validate` is the terminal post-audit substep that exits S1_PLAN; it
// is intentionally absent from planSubStepOrder (the forward progression) and is
// ranked explicitly immediately after it. PlanSubStepNone — the value carried by
// every S0/S2/S3 authority, whose order is already fixed by its workflow state
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
	// Stale-evidence ordering needs a single authority skill per state; select
	// the conventional primary (spec-compliance-review at S3_REVIEW).
	skillName, _ := PrimaryNextSkill(change)
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
		if staleEvidenceRepairDetail(reason.Detail) {
			out = append(out, reason)
		}
	}
	return model.NormalizeReasonCodes(out)
}

func staleEvidenceRepairDetail(detail string) bool {
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

func staleEvidenceAuthorityLabel(workflowState model.WorkflowState, subStep model.PlanSubStep) string {
	if workflowState == model.StateS1Plan && subStep != model.PlanSubStepNone {
		return string(workflowState) + "/" + string(subStep)
	}
	return string(workflowState)
}

// sharedWorkspaceChangedFilesScan returns a memoized workspace changed-file scan
// shared by the scope-contract and sensitive-evidence repair evaluators so a
// single AdvanceGoverned pass forks git at most once instead of once per
// evaluator. The scan runs lazily on first call — which happens inside an
// evaluator closure, after executionSummaryRepairTarget's ExecutionSummaryReady
// and S2-position guards — so a not-ready summary, or a scope-contract evaluator
// that early-returns before the sensitive-evidence evaluator runs, still never
// forks git. Both evaluators resolve identical paths (same root+change), so
// memoizing on the paths supplied by the first call is equivalent to the
// per-evaluator scan it replaces; only the git fork is shared, and it returns the
// same changed slice scopeContractWorkspaceChangedFiles yields today.
func sharedWorkspaceChangedFilesScan() func(state.ResolvedChangePaths) []string {
	var once sync.Once
	var changed []string
	return func(paths state.ResolvedChangePaths) []string {
		once.Do(func() {
			changed = scopeContractWorkspaceChangedFiles(paths)
		})
		return changed
	}
}

// scopeContractRepairTarget returns an S2_IMPLEMENT repair target when a
// satisfied execution summary nonetheless fails the Scope Contract. The Scope
// Contract is owned by S2_IMPLEMENT because it scores wave-execution evidence,
// but it can only be evaluated once the execution summary exists. Downstream
// callers use this target as a blocker/review-alignment hint; this helper does not
// mutate lifecycle state. It returns the zero target when the summary is not
// ready, the change has not yet reached S2_IMPLEMENT, evaluation errors are
// surfaced elsewhere, or the contract passes. changedFiles is the shared memoized
// workspace scan (see sharedWorkspaceChangedFilesScan).
func scopeContractRepairTarget(root string, change model.Change, summary *model.ExecutionSummary, changedFiles func(state.ResolvedChangePaths) []string) (EvidenceRepairTarget, error) {
	return executionSummaryRepairTarget(root, change, summary, func(paths state.ResolvedChangePaths) []model.ReasonCode {
		report, err := scopecontract.EvaluateBundleWithChangedFiles(
			paths.GovernedBundleDir,
			summary,
			changedFiles(paths),
		)
		if err != nil {
			return nil
		}
		return report.Blockers
	})
}

func sensitiveEvidenceRepairTarget(root string, change model.Change, summary *model.ExecutionSummary, changedFiles func(state.ResolvedChangePaths) []string) (EvidenceRepairTarget, error) {
	return executionSummaryRepairTarget(root, change, summary, func(paths state.ResolvedChangePaths) []model.ReasonCode {
		return sensitiveevidence.Evaluate(summary, changedFiles(paths)).Blockers
	})
}

// executionSummaryRepairTarget factors the shared S2_IMPLEMENT repair-target
// sequence used by scopeContractRepairTarget and sensitiveEvidenceRepairTarget:
// require a ready execution summary, require the change to have reached
// S2_IMPLEMENT, resolve the change paths, then build a wave-orchestration repair
// target from the blockers the supplied evaluator reports. evaluate is the only
// behavioral difference between the two callers; it receives the resolved paths
// (closing over the summary) and returns the blockers to repair, empty when there
// is nothing to repair. A non-nil error is returned only when path resolution
// fails; a non-zero target only when evaluate yields at least one blocker.
func executionSummaryRepairTarget(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	evaluate func(paths state.ResolvedChangePaths) []model.ReasonCode,
) (EvidenceRepairTarget, error) {
	if !state.ExecutionSummaryReady(summary) {
		return EvidenceRepairTarget{}, nil
	}
	if compareStaleEvidencePosition(
		currentStaleEvidencePosition(change),
		staleEvidencePositionFor(model.StateS2Implement, model.PlanSubStepNone),
	) < 0 {
		return EvidenceRepairTarget{}, nil
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return EvidenceRepairTarget{}, err
	}
	blockers := evaluate(paths)
	if len(blockers) == 0 {
		return EvidenceRepairTarget{}, nil
	}
	return EvidenceRepairTarget{
		SkillName:   SkillWaveOrchestration,
		State:       model.StateS2Implement,
		PlanSubStep: model.PlanSubStepNone,
		Blockers:    blockers,
	}, nil
}
