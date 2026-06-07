package progression

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/artifact"
	ctxpack "github.com/signalridge/slipway/internal/engine/context"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/scopecontract"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

type ArtifactReadiness struct {
	Ready       bool
	Required    []string
	Missing     []string
	Unreadable  []string
	Blockers    []model.ReasonCode
	Diagnostics []string
}

type ArtifactProjectionNode struct {
	Name      string
	State     string
	DependsOn []string
	Ready     bool
	Required  bool
	Source    string
}

type ArtifactProjection struct {
	Nodes       []ArtifactProjectionNode
	Amendments  []artifact.AmendmentEvent
	Diagnostics []string
}

type GovernanceReadiness struct {
	ExecutionSummary     *model.ExecutionSummary
	EvidenceFreshness    ctxpack.EvidenceFreshness
	FreshnessDiagnostics state.ExecutionFreshnessDiagnostics
	SummaryIssues        []model.ReasonCode
	SignalSummary        *model.SignalSummary
	ActiveControls       []model.ControlActivation
	RequiredActions      []governance.RequiredAction
	// PassingSkills is intentionally scoped to the required skills for the
	// requested workflow state and active planning sub-step. It is not a dump of
	// every verification file found under verification/.
	PassingSkills      map[string]model.VerificationRecord
	SkillBlockers      []model.ReasonCode
	GateEvaluations    map[gate.GateID]gate.GateEvaluation
	ArtifactReadiness  ArtifactReadiness
	ArtifactProjection *ArtifactProjection
	ScopeContract      *scopecontract.Report
	ReviewSurface      *ReviewAuthority
	reviewAuthority    *ReviewAuthority
	ShipSurface        *ShipAuthority
	Blockers           []model.ReasonCode
	Diagnostics        []string
}

type GovernanceReadinessOptions struct {
	// Callers should request the narrowest optional surfaces they need. Shared
	// blockers are always computed. Gate evaluations and artifact/review/ship
	// surfaces are opt-in because some commands only need blocker parity while
	// others also render gate, artifact, or review/ship-specific context.
	WorkflowStateOverride     *model.WorkflowState
	IncludeGateEvaluations    bool
	IncludeArtifactProjection bool
	IncludeReviewSurface      bool
	IncludeShipSurface        bool
}

const scopeContractRecoveryGuidanceDiagnostic = "scope_contract_recovery_guidance: fix tasks.md target_files or execution scope first; if a planning artifact structurally changes, stale_evidence recovery reopens the owning authority via `slipway run`; strict target_files-only drift rebuilds compatible generated evidence in S2."

type ArtifactReadinessReader interface {
	Evaluate(root string, change model.Change) (ArtifactReadiness, error)
}

type ArtifactProjectionReader interface {
	Project(root string, change model.Change) (ArtifactProjection, error)
}

type contextualArtifactReadinessReader struct {
	ctx artifactEvaluationContext
}
type contextualArtifactProjectionReader struct {
	ctx artifactEvaluationContext
}

type governanceReadinessReaders struct {
	artifactReadiness  ArtifactReadinessReader
	artifactProjection ArtifactProjectionReader
}

type artifactEvaluationContext struct {
	resolution     ChangeSchemaResolution
	requiredPreset model.WorkflowPreset
}

func EvaluateGovernanceReadiness(
	root string,
	change model.Change,
	opts GovernanceReadinessOptions,
) (GovernanceReadiness, error) {
	readiness, err := evaluateGovernanceReadinessBase(root, change, opts)
	if err != nil {
		return GovernanceReadiness{}, err
	}
	if opts.IncludeGateEvaluations || opts.IncludeShipSurface {
		gates, shipSurface, err := evaluateGateReadiness(root, change, readiness, opts)
		if err != nil {
			return GovernanceReadiness{}, err
		}
		effectiveState := change.CurrentState
		if opts.WorkflowStateOverride != nil && *opts.WorkflowStateOverride != "" {
			effectiveState = *opts.WorkflowStateOverride
		}
		if shipSurface != nil && effectiveState == model.StateS4Verify {
			// Verify-state read surfaces must expose the same ship blockers that
			// finalization uses; otherwise G_ship can be blocked while the shared
			// blocker surface still reports "ready".
			readiness.Blockers = model.NormalizeReasonCodes(append(readiness.Blockers, shipSurface.Result.ReasonCodes...))
		}
		if opts.IncludeGateEvaluations {
			readiness.GateEvaluations = gates
		}
		if opts.IncludeShipSurface {
			readiness.ShipSurface = shipSurface
		}
	}
	return readiness, nil
}

func evaluateGovernanceReadinessBase(
	root string,
	change model.Change,
	opts GovernanceReadinessOptions,
) (GovernanceReadiness, error) {
	return evaluateGovernanceReadinessBaseWithReaders(root, change, opts, governanceReadinessReaders{})
}

func evaluateGovernanceReadinessBaseWithReaders(
	root string,
	change model.Change,
	opts GovernanceReadinessOptions,
	readers governanceReadinessReaders,
) (GovernanceReadiness, error) {
	readiness := GovernanceReadiness{
		EvidenceFreshness: ctxpack.EvidenceFreshnessUnknown,
	}
	effectiveState := change.CurrentState
	if opts.WorkflowStateOverride != nil && *opts.WorkflowStateOverride != "" {
		effectiveState = *opts.WorkflowStateOverride
	}
	evaluationChange := change
	evaluationChange.CurrentState = effectiveState

	execCtx, err := state.LoadRelevantExecutionSummaryContext(root, evaluationChange)
	if err != nil {
		return GovernanceReadiness{}, err
	}
	readiness.ExecutionSummary = execCtx.Summary
	readiness.FreshnessDiagnostics = execCtx.Diagnostics
	readiness.SummaryIssues = model.ReasonCodesFromSpecs(execCtx.Issues)
	if state.ExecutionSummaryReady(execCtx.Summary) {
		readiness.EvidenceFreshness = ctxpack.EvidenceFreshness(strings.TrimSpace(execCtx.Diagnostics.Status))
		if readiness.EvidenceFreshness == "" {
			readiness.EvidenceFreshness = ctxpack.EvidenceFreshnessUnknown
		}
	}

	paths, err := state.ResolveChangePaths(root, evaluationChange)
	if err != nil {
		return GovernanceReadiness{}, err
	}
	snap, err := previewGovernanceSnapshotForReadiness(root, evaluationChange, paths.GovernedBundleDir)
	if err != nil {
		return GovernanceReadiness{}, err
	}
	readiness.SignalSummary = cloneSignalSummary(snap.Summary)
	readiness.ActiveControls = cloneControlActivations(snap.ActiveControls)
	readiness.RequiredActions = governance.ResolveRuntimeRequiredActions(root, evaluationChange, snap)
	readiness.Blockers = append(readiness.Blockers, model.ReasonCodesFromSpecs(governance.RequiredActionBlockers(evaluationChange, readiness.RequiredActions))...)

	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return GovernanceReadiness{}, err
	}
	artifactCtx := resolveArtifactEvaluationContext(root, evaluationChange, policy.EffectivePreset)
	artifactReadinessReader := readers.artifactReadiness
	if artifactReadinessReader == nil {
		artifactReadinessReader = contextualArtifactReadinessReader{ctx: artifactCtx}
	}
	artifactProjectionReader := readers.artifactProjection
	if artifactProjectionReader == nil {
		artifactProjectionReader = contextualArtifactProjectionReader{ctx: artifactCtx}
	}
	planningSubSteps := activePlanningSubStepsForState(evaluationChange, effectiveState)
	passingSkills, skillBlockers, err := EvaluateRequiredSkillsForChange(
		root,
		evaluationChange,
		effectiveState,
		execCtx.LatestRunVersion,
		FinalCloseoutEvidenceRequired(policy),
		planningSubSteps...,
	)
	if err != nil {
		return GovernanceReadiness{}, err
	}
	readiness.PassingSkills = cloneVerificationRecords(passingSkills)
	readiness.SkillBlockers = model.ReasonCodesFromSpecs(skillBlockers)
	var wavePreviewBlockers []model.ReasonCode
	readiness.SkillBlockers, wavePreviewBlockers, err = refineS2WaveExecutionSkillBlockers(
		root,
		evaluationChange,
		effectiveState,
		execCtx.Summary,
		readiness.SkillBlockers,
	)
	if err != nil {
		return GovernanceReadiness{}, err
	}
	readiness.Blockers = append(readiness.Blockers, readiness.SkillBlockers...)
	readiness.Blockers = append(readiness.Blockers, wavePreviewBlockers...)
	if effectiveState == model.StateS2Execute && evaluationChange.NeedsDiscovery && strings.TrimSpace(evaluationChange.WorktreePath) == "" {
		derivation, err := DeriveWorktreeBlockers(root, evaluationChange, passingSkills)
		if err != nil {
			return GovernanceReadiness{}, err
		}
		readiness.Blockers = append(readiness.Blockers, model.ReasonCodesFromSpecs(derivation.Blockers)...)
	} else {
		worktreeValidation, err := state.ValidateChangeWorktree(root, evaluationChange)
		if err != nil {
			return GovernanceReadiness{}, err
		}
		readiness.Blockers = append(readiness.Blockers, worktreeValidation.Blockers...)
	}

	artifactReadiness, err := artifactReadinessReader.Evaluate(root, evaluationChange)
	if err != nil {
		return GovernanceReadiness{}, err
	}
	readiness.ArtifactReadiness = artifactReadiness
	readiness.Diagnostics = append(readiness.Diagnostics, artifactReadiness.Diagnostics...)
	if ShouldCheckGovernedBundle(evaluationChange) {
		readiness.Blockers = append(readiness.Blockers, artifactReadiness.Blockers...)
		checklistResult := ValidateTasksChecklistDetailed(root, evaluationChange)
		readiness.Blockers = append(readiness.Blockers, model.ReasonCodesFromSpecs(checklistResult.Blockers)...)
		readiness.Diagnostics = append(readiness.Diagnostics, checklistResult.Warnings...)
	}

	readiness.Blockers = append(readiness.Blockers, model.ReasonCodesFromSpecs(AssuranceContractBlockers(root, evaluationChange))...)
	if effectiveState == model.StateS1Plan && evaluationChange.PlanSubStep == model.PlanSubStepValidate {
		planResult := ValidatePlanningReadiness(root, evaluationChange)
		readiness.Blockers = append(readiness.Blockers, planResult.Blockers...)
		readiness.Diagnostics = append(readiness.Diagnostics, planResult.Diagnostics...)
	}
	if state.ExecutionSummaryRelevantState(effectiveState) &&
		readiness.EvidenceFreshness == ctxpack.EvidenceFreshnessStale {
		readiness.Blockers = append(readiness.Blockers, model.NewReasonCode(state.StaleExecutionEvidenceBlockerToken, ""))
	}
	if state.ExecutionSummaryReady(execCtx.Summary) {
		scopeReport, err := scopecontract.EvaluateBundleWithChangedFiles(
			paths.GovernedBundleDir,
			execCtx.Summary,
			scopeContractWorkspaceChangedFiles(paths),
		)
		if err != nil {
			readiness.Blockers = append(readiness.Blockers, model.NewReasonCode(scopecontract.ReasonScopeContractEvaluationFailed, err.Error()))
			readiness.Diagnostics = append(readiness.Diagnostics, "scope_contract_evaluation_failed: "+err.Error())
		} else {
			cloned := scopeReport.Clone()
			readiness.ScopeContract = &cloned
			readiness.Blockers = append(readiness.Blockers, scopeReport.Blockers...)
			readiness.Diagnostics = append(readiness.Diagnostics, scopeReport.Diagnostics...)
			if scopeContractNeedsRecoveryGuidance(scopeReport) {
				readiness.Diagnostics = append(readiness.Diagnostics, scopeContractRecoveryGuidanceDiagnostic)
			}
		}
	}

	if opts.IncludeArtifactProjection {
		projection, err := artifactProjectionReader.Project(root, evaluationChange)
		if err != nil {
			return GovernanceReadiness{}, err
		}
		readiness.ArtifactProjection = &projection
		readiness.Diagnostics = append(readiness.Diagnostics, projection.Diagnostics...)
	}

	if opts.IncludeReviewSurface || effectiveState == model.StateS3Review || effectiveState == model.StateS4Verify {
		reviewSurface, err := EvaluateReviewAuthority(root, evaluationChange)
		if err != nil {
			return GovernanceReadiness{}, err
		}
		readiness.reviewAuthority = &reviewSurface
		if opts.IncludeReviewSurface {
			readiness.ReviewSurface = &reviewSurface
		}
		if effectiveState == model.StateS3Review || effectiveState == model.StateS4Verify {
			readiness.Blockers = append(readiness.Blockers, reviewSurface.Blockers...)
		}
	}

	readiness.Blockers = append(readiness.Blockers, readiness.SummaryIssues...)
	readiness.Blockers = model.NormalizeReasonCodes(readiness.Blockers)
	readiness.Diagnostics = stringutil.UniqueSorted(readiness.Diagnostics)
	return readiness, nil
}

func refineS2WaveExecutionSkillBlockers(
	root string,
	change model.Change,
	effectiveState model.WorkflowState,
	summary *model.ExecutionSummary,
	skillBlockers []model.ReasonCode,
) ([]model.ReasonCode, []model.ReasonCode, error) {
	if effectiveState != model.StateS2Execute ||
		state.ExecutionSummaryReady(summary) ||
		!hasWaveRunSummaryMissingSkillBlocker(skillBlockers) {
		return skillBlockers, nil, nil
	}

	preview, err := PreviewGovernedWaveExecution(root, change)
	if err != nil {
		return nil, nil, err
	}
	if len(preview.Blockers) == 0 {
		return skillBlockers, nil, nil
	}

	previewBlockers := model.NormalizeReasonCodes(preview.Blockers)
	if wavePreviewHasReplacementBlockers(previewBlockers) {
		return filterWaveRunSummaryMissingSkillBlockers(skillBlockers), previewBlockers, nil
	}
	if missingDetail, ok := missingTaskEvidencePreviewDetail(previewBlockers); ok {
		return enrichWaveRunSummaryMissingSkillBlockers(skillBlockers, missingDetail), nil, nil
	}
	return skillBlockers, nil, nil
}

func hasWaveRunSummaryMissingSkillBlocker(blockers []model.ReasonCode) bool {
	for _, blocker := range blockers {
		if isWaveRunSummaryMissingSkillBlocker(blocker) {
			return true
		}
	}
	return false
}

func filterWaveRunSummaryMissingSkillBlockers(blockers []model.ReasonCode) []model.ReasonCode {
	filtered := make([]model.ReasonCode, 0, len(blockers))
	for _, blocker := range blockers {
		if isWaveRunSummaryMissingSkillBlocker(blocker) {
			continue
		}
		filtered = append(filtered, blocker)
	}
	return filtered
}

func enrichWaveRunSummaryMissingSkillBlockers(blockers []model.ReasonCode, detail string) []model.ReasonCode {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return blockers
	}
	enriched := make([]model.ReasonCode, 0, len(blockers))
	for _, blocker := range blockers {
		if isWaveRunSummaryMissingSkillBlocker(blocker) {
			enriched = append(enriched, model.NewReasonCode(
				blocker.Code,
				SkillWaveOrchestration+":run_summary_missing; "+detail,
			))
			continue
		}
		enriched = append(enriched, blocker)
	}
	return enriched
}

func isWaveRunSummaryMissingSkillBlocker(blocker model.ReasonCode) bool {
	return blocker.Code == "required_skill_not_ready" &&
		strings.HasPrefix(strings.TrimSpace(blocker.Detail), SkillWaveOrchestration+":run_summary_missing")
}

func missingTaskEvidencePreviewDetail(blockers []model.ReasonCode) (string, bool) {
	for _, blocker := range blockers {
		if blocker.Code != "missing_task_evidence_for_run_summary" {
			continue
		}
		detail := strings.TrimSpace(blocker.Detail)
		if detail == "" {
			continue
		}
		return detail, true
	}
	return "", false
}

func wavePreviewHasReplacementBlockers(blockers []model.ReasonCode) bool {
	for _, blocker := range blockers {
		if blocker.Code == "missing_task_evidence_for_run_summary" {
			continue
		}
		return true
	}
	return false
}

// scopeContractNeedsRecoveryGuidance reports whether the scope-contract recovery
// guidance diagnostic should be surfaced. It is emitted whenever the contract has
// blockers, at any lifecycle state (S2_EXECUTE as well as S3_REVIEW/S4_VERIFY) so
// the explanation has surface parity. The executable next action at S2 is already
// carried by per-blocker remediation and the scope-contract advance-reopen gate;
// this diagnostic is the narrative complement.
func scopeContractNeedsRecoveryGuidance(report scopecontract.Report) bool {
	return len(report.Blockers) > 0
}

func (r GovernanceReadiness) cachedReviewAuthority() (ReviewAuthority, bool) {
	if r.ReviewSurface != nil {
		return *r.ReviewSurface, true
	}
	if r.reviewAuthority != nil {
		return *r.reviewAuthority, true
	}
	return ReviewAuthority{}, false
}

func previewGovernanceSnapshotForReadiness(
	root string,
	change model.Change,
	bundleDir string,
) (model.GovernanceSnapshot, error) {
	return governance.PreviewGovernanceSnapshot(root, change, bundleDir)
}

func evaluateGateReadiness(
	root string,
	change model.Change,
	currentReadiness GovernanceReadiness,
	opts GovernanceReadinessOptions,
) (map[gate.GateID]gate.GateEvaluation, *ShipAuthority, error) {
	var result map[gate.GateID]gate.GateEvaluation
	var err error
	effectiveState := change.CurrentState
	if opts.WorkflowStateOverride != nil && *opts.WorkflowStateOverride != "" {
		effectiveState = *opts.WorkflowStateOverride
	}
	if opts.IncludeGateEvaluations {
		planSkills, planSkillBlockers, err := gatePlanningSkillRecords(root, change, model.PlanSubStepAudit)
		if err != nil {
			return nil, nil, err
		}
		planEval := EvaluatePlanGate(root, change, planSkills)
		planEval.ReasonCodes = model.NormalizeReasonCodes(append(planEval.ReasonCodes, model.ReasonCodesFromSpecs(planSkillBlockers)...))
		if len(planEval.ReasonCodes) > 0 {
			planEval.Status = model.GateStatusBlocked
		}
		result = map[gate.GateID]gate.GateEvaluation{
			gate.GatePlan: planEval,
		}
		if change.NeedsDiscovery && effectiveState != model.StateS0Intake {
			scopeSkills, scopeSkillBlockers, err := gatePlanningSkillRecords(root, change, model.PlanSubStepResearch)
			if err != nil {
				return nil, nil, err
			}
			scopeEval, err := EvaluateScopeGate(root, change, scopeSkills)
			if err != nil {
				return nil, nil, err
			}
			scopeEval.ReasonCodes = model.NormalizeReasonCodes(append(scopeEval.ReasonCodes, model.ReasonCodesFromSpecs(scopeSkillBlockers)...))
			if len(scopeEval.ReasonCodes) > 0 {
				scopeEval.Status = model.GateStatusBlocked
			}
			result[gate.GateScope] = scopeEval
		}
	}
	shipReadiness := currentReadiness
	needsVerifyRefresh := effectiveState != model.StateS4Verify
	if !needsVerifyRefresh {
		_, needsVerifyRefresh = currentReadiness.cachedReviewAuthority()
		needsVerifyRefresh = !needsVerifyRefresh
	}
	if needsVerifyRefresh {
		verifyState := model.StateS4Verify
		shipReadiness, err = evaluateGovernanceReadinessBase(
			root,
			change,
			GovernanceReadinessOptions{
				WorkflowStateOverride: &verifyState,
				IncludeReviewSurface:  true,
			},
		)
		if err != nil {
			return nil, nil, err
		}
	}
	shipSurface, err := buildShipAuthorityFromReadiness(root, change, shipReadiness)
	if err != nil {
		return nil, nil, err
	}
	if opts.IncludeGateEvaluations {
		result[gate.GateShip] = shipSurface.Result
	}
	return result, &shipSurface, nil
}

func (r contextualArtifactReadinessReader) Evaluate(root string, change model.Change) (ArtifactReadiness, error) {
	return evaluateArtifactReadinessWithContext(root, change, r.ctx)
}

func evaluateArtifactReadinessWithContext(root string, change model.Change, ctx artifactEvaluationContext) (ArtifactReadiness, error) {
	result := ArtifactReadiness{
		Diagnostics: append([]string(nil), ctx.resolution.Warnings...),
		Blockers:    model.ReasonCodesFromSpecs(ctx.resolution.Blockers),
	}

	required := artifact.RequiredArtifactsForChange(
		ctx.resolution.Schema,
		change.NeedsDiscovery,
		change.WorkflowPreset,
		ctx.requiredPreset,
	)
	result.Required = append(result.Required, required...)

	base, err := state.GovernedBundleDir(root, change)
	if err != nil {
		result.Blockers = append(result.Blockers, model.NewReasonCode("governed_bundle_path_invalid", err.Error()))
		result.Blockers = model.NormalizeReasonCodes(result.Blockers)
		return result, nil
	}

	specByName := map[string]artifact.ArtifactSpec{}
	for _, spec := range ctx.resolution.Schema {
		specByName[spec.Name] = spec
	}
	eligibleByLevel := map[string]struct{}{}
	for _, name := range required {
		eligibleByLevel[name] = struct{}{}
	}
	requiredSet := map[string]struct{}{}
	for _, name := range required {
		requiredSet[name] = struct{}{}
	}
	for _, name := range required {
		spec, ok := specByName[name]
		if !ok {
			result.Blockers = append(result.Blockers, model.NewReasonCode("required_artifact_schema_missing", name))
			continue
		}
		for _, dep := range spec.DependsOn {
			if _, inLevel := eligibleByLevel[dep]; !inLevel {
				continue
			}
			if _, ok := requiredSet[dep]; ok {
				continue
			}
			result.Blockers = append(result.Blockers, model.NewReasonCode("required_artifact_dependency_missing", fmt.Sprintf("%s->%s", name, dep)))
		}
	}

	for _, name := range required {
		path := artifact.ResolveArtifactPath(base, change.Slug, name)
		if _, err := os.Stat(path); err != nil {
			switch {
			case errors.Is(err, fs.ErrNotExist):
				result.Missing = append(result.Missing, name)
				result.Blockers = append(result.Blockers, model.NewReasonCode("missing_required_artifact", name))
			default:
				result.Unreadable = append(result.Unreadable, name)
				result.Blockers = append(result.Blockers, model.NewReasonCode("required_artifact_unreadable", name))
			}
		}
	}

	sortStrings(&result.Required)
	sortStrings(&result.Missing)
	sortStrings(&result.Unreadable)
	result.Blockers = model.NormalizeReasonCodes(result.Blockers)
	result.Diagnostics = stringutil.UniqueSorted(result.Diagnostics)
	result.Ready = len(result.Blockers) == 0
	return result, nil
}

func (r contextualArtifactProjectionReader) Project(root string, change model.Change) (ArtifactProjection, error) {
	return projectArtifactProjectionWithContext(root, change, r.ctx)
}

func projectArtifactProjectionWithContext(root string, change model.Change, ctx artifactEvaluationContext) (ArtifactProjection, error) {
	required := artifact.RequiredArtifactsForChange(
		ctx.resolution.Schema,
		change.NeedsDiscovery,
		change.WorkflowPreset,
		ctx.requiredPreset,
	)
	requiredSet := map[string]struct{}{}
	for _, name := range required {
		requiredSet[name] = struct{}{}
	}

	projectedChange := cloneChangeForProjection(change)
	reconcileResult, err := artifact.ReconcileFromFilesystem(root, &projectedChange, ctx.requiredPreset)
	if err != nil {
		return ArtifactProjection{}, err
	}
	artifactStates := projectedChange.Artifacts
	specByName := map[string]artifact.ArtifactSpec{}
	for _, spec := range ctx.resolution.Schema {
		specByName[spec.Name] = spec
	}

	stateOf := func(name string) string {
		if as, ok := projectionArtifactState(name, artifactStates); ok {
			return string(as.State)
		}
		return "pending"
	}
	doneStates := map[string]bool{
		string(model.ArtifactLifecycleApproved): true,
		string(model.ArtifactLifecycleFrozen):   true,
	}

	nodeNames := make([]string, 0, len(required)+len(artifactStates))
	included := map[string]struct{}{}
	for _, name := range required {
		nodeNames = append(nodeNames, name)
		included[name] = struct{}{}
	}
	for _, as := range artifactStates {
		name := projectionArtifactNodeName(as)
		if name == "" {
			continue
		}
		if _, ok := included[name]; ok {
			continue
		}
		nodeNames = append(nodeNames, name)
		included[name] = struct{}{}
	}

	nodes := make([]ArtifactProjectionNode, 0, len(nodeNames))
	appendNode := func(name string, required bool, source string) {
		spec, hasSpec := specByName[name]
		deps := make([]string, 0)
		ready := true
		if hasSpec {
			deps = make([]string, 0, len(spec.DependsOn))
			for _, dep := range spec.DependsOn {
				if _, ok := included[dep]; !ok {
					continue
				}
				deps = append(deps, dep)
				if !doneStates[stateOf(dep)] {
					ready = false
				}
			}
		}
		nodes = append(nodes, ArtifactProjectionNode{
			Name:      name,
			State:     stateOf(name),
			DependsOn: deps,
			Ready:     ready,
			Required:  required,
			Source:    source,
		})
	}

	for _, spec := range ctx.resolution.Schema {
		if _, ok := requiredSet[spec.Name]; !ok {
			continue
		}
		appendNode(spec.Name, true, "filesystem_projection")
	}
	for _, name := range nodeNames {
		if _, ok := requiredSet[name]; ok {
			continue
		}
		appendNode(name, false, "change_state")
	}
	slices.SortFunc(nodes, func(a, b ArtifactProjectionNode) int {
		return strings.Compare(a.Name, b.Name)
	})
	return ArtifactProjection{
		Nodes:       nodes,
		Amendments:  append([]artifact.AmendmentEvent(nil), reconcileResult.Amendments...),
		Diagnostics: append([]string(nil), ctx.resolution.Warnings...),
	}, nil
}

func projectionArtifactState(name string, artifactStates map[string]model.ArtifactState) (model.ArtifactState, bool) {
	if as, ok := artifactStates[name]; ok {
		return as, true
	}
	artifactID := strings.TrimSuffix(name, filepath.Ext(name))
	as, ok := artifactStates[artifactID]
	return as, ok
}

func projectionArtifactNodeName(as model.ArtifactState) string {
	if base := strings.TrimSpace(filepath.Base(as.Path)); base != "" && base != "." {
		return base
	}
	id := strings.TrimSpace(as.ID)
	if id == "" {
		return ""
	}
	if strings.Contains(id, ".") {
		return id
	}
	if id == "change" {
		return "change.yaml"
	}
	return id + ".md"
}

func sortStrings(values *[]string) {
	if len(*values) == 0 {
		*values = nil
		return
	}
	slices.Sort(*values)
}

func resolveArtifactEvaluationContext(root string, change model.Change, requiredPreset model.WorkflowPreset) artifactEvaluationContext {
	if !requiredPreset.IsValid() {
		requiredPreset = change.WorkflowPreset
	}
	return artifactEvaluationContext{
		resolution:     ResolveChangeSchemaDiagnostics(change),
		requiredPreset: requiredPreset,
	}
}

func scopeContractWorkspaceChangedFiles(paths state.ResolvedChangePaths) []string {
	return workspaceChangedFiles(paths, workspaceChangedFilesOptions{})
}

type workspaceChangedFilesOptions struct {
	includeGovernedBundle bool
	includeLocalState     bool
	// exemptActiveBundleOnly narrows the governed-bundle dirty exemption to the
	// active change's bundle. When false, every artifacts/changes/ path is
	// exempted (scope-contract behavior). `done` archive sets this true so a
	// dirty sibling or archived bundle is still reported in the non-blocking
	// dirty advisory — only the active artifacts/changes/<slug>/ bundle is
	// suppressed because `done` rewrites it into the archived record (REQ-004).
	exemptActiveBundleOnly bool
}

// WorkspaceChangedFilesForDoneArchive returns Git-visible changed files that
// `done` reports in its non-blocking dirty-worktree advisory (REQ-004). Archive
// still completes; only the active governed bundle is exempted, because `done`
// rewrites it into the archived bundle. Any other dirty governance artifact is
// surfaced so the operator can commit it alongside the archived record.
func WorkspaceChangedFilesForDoneArchive(paths state.ResolvedChangePaths) []string {
	return workspaceChangedFiles(paths, workspaceChangedFilesOptions{
		includeGovernedBundle:  false,
		includeLocalState:      true,
		exemptActiveBundleOnly: true,
	})
}

func workspaceChangedFiles(paths state.ResolvedChangePaths, opts workspaceChangedFilesOptions) []string {
	workspaceRoot := strings.TrimSpace(paths.WorkspaceRoot)
	if workspaceRoot == "" {
		return nil
	}
	files := gitNameOnly(workspaceRoot, "diff", "--name-only", "HEAD", "--")
	for _, file := range gitNameOnly(workspaceRoot, "ls-files", "--others", "--exclude-standard") {
		if opts.includeLocalState || scopeContractUntrackedChangedFile(workspaceRoot, file) {
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return nil
	}

	bundleRel := ""
	if rel, err := filepath.Rel(workspaceRoot, paths.GovernedBundleDir); err == nil {
		bundleRel = filepath.ToSlash(rel)
	}
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		file = filepath.ToSlash(strings.TrimSpace(file))
		file = strings.TrimPrefix(file, "./")
		if file == "" {
			continue
		}
		if opts.includeLocalState && generatedOnlyLocalStateChangedFile(workspaceRoot, file) {
			continue
		}
		if !opts.includeGovernedBundle {
			if !opts.exemptActiveBundleOnly && (file == "artifacts" || strings.HasPrefix(file, "artifacts/changes/")) {
				continue
			}
			if bundleRel != "" && bundleRel != "." && (file == bundleRel || strings.HasPrefix(file, bundleRel+"/")) {
				continue
			}
			if !opts.includeLocalState && scopeContractContextArtifactChangedFile(file) {
				continue
			}
		}
		filtered = append(filtered, file)
	}
	if len(filtered) == 0 {
		return nil
	}
	return stringutil.UniqueSorted(filtered)
}

func scopeContractContextArtifactChangedFile(file string) bool {
	file = filepath.ToSlash(strings.TrimSpace(file))
	file = strings.TrimPrefix(file, "./")
	return file == "artifacts/codebase" || strings.HasPrefix(file, "artifacts/codebase/")
}

func scopeContractUntrackedChangedFile(workspaceRoot, file string) bool {
	file = filepath.ToSlash(strings.TrimSpace(file))
	file = strings.TrimPrefix(file, "./")
	if file == "" {
		return false
	}
	if file == fsutil.ProjectConfigFileName {
		return false
	}
	if file == ".gitignore" && scopeContractGeneratedOnlyGitIgnore(workspaceRoot) {
		return false
	}
	if file == "artifacts" || strings.HasPrefix(file, "artifacts/changes/") {
		return false
	}
	return true
}

func generatedOnlyLocalStateChangedFile(workspaceRoot, file string) bool {
	switch file {
	case ".gitignore":
		return generatedOnlyGitIgnoreChange(workspaceRoot)
	case fsutil.ProjectConfigFileName:
		return generatedOnlyProjectConfigChange(workspaceRoot)
	default:
		return false
	}
}

func generatedOnlyGitIgnoreChange(workspaceRoot string) bool {
	raw, err := os.ReadFile(filepath.Join(workspaceRoot, ".gitignore"))
	if err != nil {
		return false
	}
	content := normalizeLineEndings(string(raw))
	headContent, ok := gitHeadFileContent(workspaceRoot, ".gitignore")
	if !ok {
		return strings.TrimSpace(content) == strings.TrimSpace(state.LocalStateGitIgnoreBlock())
	}
	expected, err := state.NormalizeLocalStateGitIgnoreContent(headContent)
	if err != nil {
		return false
	}
	return strings.TrimSpace(content) == strings.TrimSpace(normalizeLineEndings(expected))
}

func generatedOnlyProjectConfigChange(workspaceRoot string) bool {
	if _, ok := gitHeadFileContent(workspaceRoot, fsutil.ProjectConfigFileName); ok {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(workspaceRoot, fsutil.ProjectConfigFileName))
	if err != nil {
		return false
	}
	cfg, err := model.ParseConfigYAML(raw)
	if err != nil {
		return false
	}
	current, err := cfg.ToYAML()
	if err != nil {
		return false
	}
	defaultConfig, err := model.DefaultConfig().ToYAML()
	if err != nil {
		return false
	}
	return bytes.Equal(bytes.TrimSpace(current), bytes.TrimSpace(defaultConfig))
}

func scopeContractGeneratedOnlyGitIgnore(workspaceRoot string) bool {
	raw, err := os.ReadFile(filepath.Join(workspaceRoot, ".gitignore"))
	if err != nil {
		return false
	}
	content := normalizeLineEndings(string(raw))
	return strings.TrimSpace(content) == strings.TrimSpace(state.LocalStateGitIgnoreBlock())
}

func normalizeLineEndings(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

func gitHeadFileContent(workspaceRoot, file string) (string, bool) {
	cmd := exec.Command("git", "-C", workspaceRoot, "show", "HEAD:"+filepath.ToSlash(file))
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func gitNameOnly(workspaceRoot string, args ...string) []string {
	cmd := exec.Command("git", append([]string{"-C", workspaceRoot}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(string(out), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func gatePlanningSkillRecords(
	root string,
	change model.Change,
	planSubStep model.PlanSubStep,
) (map[string]model.VerificationRecord, []string, error) {
	var subSteps []model.PlanSubStep
	if planSubStep != model.PlanSubStepNone {
		subSteps = []model.PlanSubStep{planSubStep}
	}
	passingSkills, skillBlockers, err := EvaluateRequiredSkillsForChange(
		root,
		change,
		model.StateS1Plan,
		0,
		false,
		subSteps...,
	)
	if err != nil {
		return nil, nil, err
	}
	return passingSkills, skillBlockers, nil
}

func cloneChangeForProjection(change model.Change) model.Change {
	cloned := change
	cloned.Artifacts = cloneArtifactStates(change.Artifacts)
	cloned.EvidenceRefs = cloneStringMap(change.EvidenceRefs)
	cloned.LastAutoPassedStates = append([]model.AutoPassedState(nil), change.LastAutoPassedStates...)
	return cloned
}

func cloneArtifactStates(src map[string]model.ArtifactState) map[string]model.ArtifactState {
	if len(src) == 0 {
		return map[string]model.ArtifactState{}
	}
	cloned := make(map[string]model.ArtifactState, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func cloneVerificationRecords(src map[string]model.VerificationRecord) map[string]model.VerificationRecord {
	if len(src) == 0 {
		return map[string]model.VerificationRecord{}
	}
	cloned := make(map[string]model.VerificationRecord, len(src))
	for key, value := range src {
		record := value
		record.Blockers = append([]model.ReasonCode(nil), value.Blockers...)
		record.References = append([]string(nil), value.References...)
		cloned[key] = record
	}
	return cloned
}

func cloneSignalSummary(src model.SignalSummary) *model.SignalSummary {
	cloned := src
	cloned.Domains = append([]string(nil), src.Domains...)
	return &cloned
}

func cloneControlActivations(src []model.ControlActivation) []model.ControlActivation {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]model.ControlActivation, 0, len(src))
	for _, ctrl := range src {
		activation := ctrl
		activation.TriggeredBy = append([]string(nil), ctrl.TriggeredBy...)
		cloned = append(cloned, activation)
	}
	return cloned
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}
