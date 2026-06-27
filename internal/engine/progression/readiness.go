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
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/scopecontract"
	"github.com/signalridge/slipway/internal/engine/sensitiveevidence"
	"github.com/signalridge/slipway/internal/engine/skill"
	freshnesspkg "github.com/signalridge/slipway/internal/freshness"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

type ArtifactReadiness struct {
	Ready       bool
	Blockers    []model.ReasonCode
	Diagnostics []string
}

type ArtifactProjectionNode struct {
	Name      string
	State     string
	DependsOn []string
	Ready     bool
	Required  bool
}

type ArtifactProjection struct {
	Nodes       []ArtifactProjectionNode
	Amendments  []artifact.AmendmentEvent
	Diagnostics []string
}

type GovernanceReadiness struct {
	ExecutionSummary     *model.ExecutionSummary
	EvidenceFreshness    freshnesspkg.EvidenceFreshness
	FreshnessDiagnostics state.ExecutionFreshnessDiagnostics
	SummaryIssues        []model.ReasonCode
	SignalSummary        *model.SignalSummary
	ActiveControls       []model.ControlActivation
	RequiredActions      []governance.RequiredAction
	// PassingSkills is intentionally scoped to the required skills for the
	// requested workflow state and active planning sub-step. It is not a dump of
	// every verification file found under verification/.
	PassingSkills       map[string]model.VerificationRecord
	verificationRecords map[string]model.VerificationRecord
	SkillBlockers       []model.ReasonCode
	GateEvaluations     map[gate.GateID]gate.GateEvaluation
	ArtifactReadiness   ArtifactReadiness
	ArtifactProjection  *ArtifactProjection
	ScopeContract       *scopecontract.Report
	SensitiveEvidence   *sensitiveevidence.Report
	ReviewSurface       *ReviewAuthority
	reviewAuthority     *ReviewAuthority
	ShipSurface         *ShipAuthority
	Blockers            []model.ReasonCode
	Diagnostics         []string
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
	// VerificationRecords lets callers that already loaded the authoritative
	// verification inventory reuse it for readiness. A nil map preserves the
	// default strict disk read.
	VerificationRecords map[string]model.VerificationRecord
}

const scopeContractRecoveryGuidanceDiagnostic = "scope_contract_recovery_guidance: out-of-scope drift preserves recorded wave evidence. Remove a build-artifact/scratch file or rely on an ignore/local exclude; to keep legitimate same-intent work, record a scope amendment by amending the owning task target_files in tasks.md. In S2, refresh the affected implementation evidence; in S3 review, keep the state in review/fix and let the selected reviewers verify the current plan/code/evidence. If the objective changed, open a new governed change."

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
		if shipSurface != nil && effectiveState == model.StateS3Review {
			// S3 read surfaces must expose the same ship blockers that finalization
			// uses; otherwise G_ship can be blocked while the shared blocker
			// surface still reports "ready".
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
		EvidenceFreshness: freshnesspkg.EvidenceFreshnessUnknown,
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
	readiness.FreshnessDiagnostics = state.ProjectExecutionFreshnessDiagnosticsForState(effectiveState, execCtx.Diagnostics)
	readiness.SummaryIssues = model.ReasonCodesFromSpecs(execCtx.Issues)
	if state.ExecutionSummaryReady(execCtx.Summary) {
		readiness.EvidenceFreshness = state.ProjectExecutionFreshnessForState(effectiveState, execCtx.Diagnostics)
	}
	if state.ExecutionFreshnessIsS3TaskPlanAmendment(effectiveState, execCtx.Diagnostics) {
		readiness.Diagnostics = append(readiness.Diagnostics, state.S3TaskPlanAmendmentDiagnostic)
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
	reviewSelection := ReviewSkillSelectionFromControls(snap.ActiveControls)

	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return GovernanceReadiness{}, err
	}
	artifactCtx := resolveArtifactEvaluationContext(evaluationChange, policy.EffectivePreset)
	artifactReadinessReader := readers.artifactReadiness
	if artifactReadinessReader == nil {
		artifactReadinessReader = contextualArtifactReadinessReader{ctx: artifactCtx}
	}
	artifactProjectionReader := readers.artifactProjection
	if artifactProjectionReader == nil {
		artifactProjectionReader = contextualArtifactProjectionReader{ctx: artifactCtx}
	}
	verificationRecords, err := governanceReadinessVerificationRecords(root, evaluationChange, opts)
	if err != nil {
		return GovernanceReadiness{}, err
	}
	readiness.verificationRecords = cloneVerificationRecords(verificationRecords)
	planningSubSteps := activePlanningSubStepsForState(evaluationChange, effectiveState)
	passingSkills, skillBlockers, err := evaluateRequiredSkillsForChangeWithReviewSelectionWithRecords(
		root,
		evaluationChange,
		effectiveState,
		execCtx.LatestRunVersion,
		FinalCloseoutEvidenceRequired(policy),
		reviewSelection,
		verificationRecords,
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
	if effectiveState == model.StateS2Implement && evaluationChange.NeedsDiscovery && strings.TrimSpace(evaluationChange.WorktreePath) == "" {
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
		readiness.Blockers = append(readiness.Blockers, model.ReasonCodesFromSpecs(DecisionContractBlockers(root, evaluationChange))...)
	}

	readiness.Blockers = append(readiness.Blockers, model.ReasonCodesFromSpecs(AssuranceContractBlockers(root, evaluationChange))...)
	if effectiveState == model.StateS1Plan && evaluationChange.PlanSubStep == model.PlanSubStepValidate {
		planResult := ValidatePlanningReadiness(root, evaluationChange)
		readiness.Blockers = append(readiness.Blockers, planResult.Blockers...)
		readiness.Diagnostics = append(readiness.Diagnostics, planResult.Diagnostics...)
	}
	if state.ExecutionSummaryRelevantState(effectiveState) &&
		readiness.EvidenceFreshness == freshnesspkg.EvidenceFreshnessStale {
		readiness.Blockers = append(readiness.Blockers, model.NewReasonCode(state.StaleExecutionEvidenceBlockerToken, ""))
	}
	if state.ExecutionSummaryReady(execCtx.Summary) {
		// workspaceChangedFiles yields both the scope-contract changed set and the
		// codebase-map context files it exempts from that set, from a single git
		// scan, so the disclosed exemption can never drift from the set actually
		// filtered out.
		changedFiles, exemptContextFiles := workspaceChangedFiles(paths, workspaceChangedFilesOptions{})
		scopeReport, err := scopecontract.EvaluateBundleWithChangedFiles(
			paths.GovernedBundleDir,
			execCtx.Summary,
			changedFiles,
		)
		if err != nil {
			readiness.Blockers = append(readiness.Blockers, model.NewReasonCode(scopecontract.ReasonScopeContractEvaluationFailed, err.Error()))
			readiness.Diagnostics = append(readiness.Diagnostics, "scope_contract_evaluation_failed: "+err.Error())
		} else {
			cloned := scopeReport.Clone()
			cloned.ExemptContextFiles = exemptContextFiles
			readiness.ScopeContract = &cloned
			readiness.Blockers = append(readiness.Blockers, scopeReport.Blockers...)
			if scopeContractNeedsRecoveryGuidance(scopeReport) {
				readiness.Diagnostics = append(readiness.Diagnostics, scopeContractRecoveryGuidanceDiagnostic)
			}
		}

		sensitiveReport := sensitiveevidence.Evaluate(execCtx.Summary, changedFiles)
		if sensitiveReport.Status != sensitiveevidence.StatusNotApplicable {
			cloned := sensitiveReport
			readiness.SensitiveEvidence = &cloned
		}
		readiness.Blockers = append(readiness.Blockers, sensitiveReport.Blockers...)
	}

	if opts.IncludeArtifactProjection {
		projection, err := artifactProjectionReader.Project(root, evaluationChange)
		if err != nil {
			return GovernanceReadiness{}, err
		}
		readiness.ArtifactProjection = &projection
		readiness.Diagnostics = append(readiness.Diagnostics, projection.Diagnostics...)
	}

	if opts.IncludeReviewSurface || effectiveState == model.StateS3Review {
		reviewSurface, err := evaluateReviewAuthorityWithPolicyAndRecords(
			root,
			evaluationChange,
			policy,
			verificationRecords,
		)
		if err != nil {
			return GovernanceReadiness{}, err
		}
		readiness.reviewAuthority = &reviewSurface
		if opts.IncludeReviewSurface {
			readiness.ReviewSurface = &reviewSurface
		}
		if effectiveState == model.StateS3Review {
			readiness.Blockers = append(readiness.Blockers, reviewSurface.Blockers...)
		}
	}

	readiness.Blockers = append(readiness.Blockers, readiness.SummaryIssues...)
	if effectiveState == model.StateS3Review {
		readiness.Blockers = filterS3ReviewAlignmentInputBlockers(readiness.Blockers)
	}
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
	if effectiveState != model.StateS2Implement ||
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
// blockers, at any lifecycle state (S2_IMPLEMENT as well as S3_REVIEW) so
// the explanation has surface parity. The executable next action at S2 is already
// carried by per-blocker remediation and the scope-contract repair gate;
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

func governanceReadinessVerificationRecords(
	root string,
	change model.Change,
	opts GovernanceReadinessOptions,
) (map[string]model.VerificationRecord, error) {
	if opts.VerificationRecords != nil {
		return cloneVerificationRecords(opts.VerificationRecords), nil
	}
	return state.ListVerificationsForChange(root, change)
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
		result = map[gate.GateID]gate.GateEvaluation{}
		verificationRecords := opts.VerificationRecords
		if verificationRecords == nil {
			verificationRecords = currentReadiness.verificationRecords
		}
		if planningGatesClosed(effectiveState) {
			result[gate.GatePlan] = approvedGateEvaluation(gate.GatePlan)
			if change.NeedsDiscovery {
				result[gate.GateScope] = approvedGateEvaluation(gate.GateScope)
			}
		} else {
			planSkills, planSkillBlockers, err := gatePlanningSkillRecordsWithRecords(
				root,
				change,
				model.PlanSubStepAudit,
				verificationRecords,
			)
			if err != nil {
				return nil, nil, err
			}
			// Resolve the preset policy so the plan gate's plan-audit self-audit edge
			// enforces on standard/strict and stays advisory on light. A resolution
			// failure falls back to the zero policy (EffectivePreset != light), which
			// fails closed rather than silently relaxing the edge.
			planPresetPolicy, _ := governance.ResolvePresetPolicy(root, change)
			planEval := EvaluatePlanGate(root, change, planSkills, planPresetPolicy)
			planEval.ReasonCodes = model.NormalizeReasonCodes(append(planEval.ReasonCodes, model.ReasonCodesFromSpecs(planSkillBlockers)...))
			if len(planEval.ReasonCodes) > 0 {
				planEval.Status = model.GateStatusBlocked
			}
			result[gate.GatePlan] = planEval
			if change.NeedsDiscovery && effectiveState != model.StateS0Intake {
				scopeSkills, scopeSkillBlockers, err := gatePlanningSkillRecordsWithRecords(
					root,
					change,
					model.PlanSubStepResearch,
					verificationRecords,
				)
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
	}
	shipReadiness := currentReadiness
	needsShipRefresh := effectiveState != model.StateS3Review
	if !needsShipRefresh {
		_, needsShipRefresh = currentReadiness.cachedReviewAuthority()
		needsShipRefresh = !needsShipRefresh
	}
	if needsShipRefresh {
		shipState := model.StateS3Review
		shipReadiness, err = evaluateGovernanceReadinessBase(
			root,
			change,
			GovernanceReadinessOptions{
				WorkflowStateOverride: &shipState,
				IncludeReviewSurface:  true,
				VerificationRecords:   currentReadiness.verificationRecords,
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

func planningGatesClosed(state model.WorkflowState) bool {
	switch state {
	case model.StateS2Implement, model.StateS3Review, model.StateDone:
		return true
	default:
		return false
	}
}

func approvedGateEvaluation(gateID gate.GateID) gate.GateEvaluation {
	return gate.GateEvaluation{
		GateID: gateID,
		Status: model.GateStatusApproved,
	}
}

func filterS3ReviewAlignmentInputBlockers(blockers []model.ReasonCode) []model.ReasonCode {
	if len(blockers) == 0 {
		return nil
	}
	out := make([]model.ReasonCode, 0, len(blockers))
	for _, blocker := range blockers {
		if s3ReviewAlignmentInputBlocker(blocker) {
			continue
		}
		out = append(out, blocker)
	}
	return out
}

func s3ReviewAlignmentInputBlocker(blocker model.ReasonCode) bool {
	code := strings.TrimSpace(blocker.Code)
	switch code {
	case state.StalePlanningEvidenceBlockerToken,
		state.StaleExecutionEvidenceBlockerToken,
		"tasks_plan_changed_since_task_evidence",
		scopecontract.ReasonScopeContractDrift,
		scopecontract.ReasonScopeContractChangedFilesMissing:
		return true
	case "required_skill_stale":
		target, _, _ := strings.Cut(strings.TrimSpace(blocker.Detail), ":")
		switch strings.TrimSpace(target) {
		case SkillIntakeClarification,
			SkillResearchOrchestration,
			SkillPlanAudit,
			SkillWaveOrchestration:
			return true
		default:
			return false
		}
	default:
		return false
	}
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
		// assurance.md existence is owned by AssuranceContractBlockers at
		// S3_REVIEW+ (issue #141); the generic pre-S3 readiness gate must not
		// report it missing and strand the change before review.
		if existenceOwnedByDedicatedGate(name) {
			continue
		}
		path := artifact.ResolveArtifactPath(base, name)
		if _, err := os.Stat(path); err != nil {
			switch {
			case errors.Is(err, fs.ErrNotExist):
				result.Blockers = append(result.Blockers, model.NewReasonCode("missing_required_artifact", name))
			default:
				result.Blockers = append(result.Blockers, model.NewReasonCode("required_artifact_unreadable", name))
			}
		}
	}

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
	appendNode := func(name string, required bool) {
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
		})
	}

	for _, spec := range ctx.resolution.Schema {
		if _, ok := requiredSet[spec.Name]; !ok {
			continue
		}
		appendNode(spec.Name, true)
	}
	for _, name := range nodeNames {
		if _, ok := requiredSet[name]; ok {
			continue
		}
		appendNode(name, false)
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

func resolveArtifactEvaluationContext(change model.Change, requiredPreset model.WorkflowPreset) artifactEvaluationContext {
	if !requiredPreset.IsValid() {
		requiredPreset = change.WorkflowPreset
	}
	return artifactEvaluationContext{
		resolution:     ResolveChangeSchemaDiagnostics(change),
		requiredPreset: requiredPreset,
	}
}

func scopeContractWorkspaceChangedFiles(paths state.ResolvedChangePaths) []string {
	changed, _ := workspaceChangedFiles(paths, workspaceChangedFilesOptions{})
	return changed
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
	changed, _ := workspaceChangedFiles(paths, workspaceChangedFilesOptions{
		includeGovernedBundle:  false,
		includeLocalState:      true,
		exemptActiveBundleOnly: true,
	})
	return changed
}

// workspaceChangedFiles returns the Git-visible changed files for scope-contract
// and dirty-advisory accounting. The second result, exemptContext, is the set of
// dirty codebase-map context artifacts (artifacts/codebase/**) that the
// scope-contract filter intentionally drops from the changed set. It is collected
// in the same pass that drops them — and only on the scope-contract path
// (opts.includeLocalState == false) — so the disclosed exemption is exactly the
// set filtered out, never a separately re-derived (and potentially divergent) one.
// Both results are unique-sorted; either may be nil.
func workspaceChangedFiles(paths state.ResolvedChangePaths, opts workspaceChangedFilesOptions) (changed, exemptContext []string) {
	workspaceRoot := strings.TrimSpace(paths.WorkspaceRoot)
	if workspaceRoot == "" {
		return nil, nil
	}
	files := gitNameOnly(workspaceRoot, "diff", "--name-only", "HEAD", "--")
	for _, file := range gitNameOnly(workspaceRoot, "ls-files", "--others", "--exclude-standard") {
		if opts.includeLocalState || scopeContractUntrackedChangedFile(workspaceRoot, file) {
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return nil, nil
	}

	bundleRel := ""
	if rel, err := filepath.Rel(workspaceRoot, paths.GovernedBundleDir); err == nil {
		bundleRel = filepath.ToSlash(rel)
	}
	filtered := make([]string, 0, len(files))
	var exempt []string
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
				exempt = append(exempt, file)
				continue
			}
		}
		filtered = append(filtered, file)
	}
	if len(filtered) > 0 {
		changed = stringutil.UniqueSorted(filtered)
	}
	if len(exempt) > 0 {
		exemptContext = stringutil.UniqueSorted(exempt)
	}
	return changed, exemptContext
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
	raw, err := os.ReadFile(filepath.Join(workspaceRoot, ".gitignore")) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
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
	raw, err := os.ReadFile(filepath.Join(workspaceRoot, fsutil.ProjectConfigFileName)) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
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
	raw, err := os.ReadFile(filepath.Join(workspaceRoot, ".gitignore")) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
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
	cmd := exec.Command("git", "-C", workspaceRoot, "show", "HEAD:"+filepath.ToSlash(file)) // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func gitNameOnly(workspaceRoot string, args ...string) []string {
	files, err := gitNameOnlyResult(workspaceRoot, args...)
	if err != nil {
		return nil
	}
	return files
}

func gitNameOnlyResult(workspaceRoot string, args ...string) ([]string, error) {
	cmd := exec.Command("git", append([]string{"-C", workspaceRoot}, args...)...) // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func gatePlanningSkillRecords(
	root string,
	change model.Change,
	planSubStep model.PlanSubStep,
) (map[string]model.VerificationRecord, []string, error) {
	return gatePlanningSkillRecordsWithRecords(root, change, planSubStep, nil)
}

func gatePlanningSkillRecordsWithRecords(
	root string,
	change model.Change,
	planSubStep model.PlanSubStep,
	verificationRecords map[string]model.VerificationRecord,
) (map[string]model.VerificationRecord, []string, error) {
	var subSteps []model.PlanSubStep
	if planSubStep != model.PlanSubStepNone {
		subSteps = []model.PlanSubStep{planSubStep}
	}
	passingSkills, skillBlockers, err := evaluateRequiredSkillsForChangeWithReviewSelectionWithRecords(
		root,
		change,
		model.StateS1Plan,
		0,
		false,
		skill.ReviewSkillSelection{},
		verificationRecords,
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
