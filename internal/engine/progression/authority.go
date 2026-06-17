package progression

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	ctxpack "github.com/signalridge/slipway/internal/engine/context"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	reviewengine "github.com/signalridge/slipway/internal/engine/review"
	engineskill "github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

type runtimeGovernanceInputs struct {
	Policy governance.PresetPolicy
	Paths  state.ResolvedChangePaths
}

type ReviewAuthority struct {
	Policy               governance.PresetPolicy
	ExecutionSummaryCtx  state.RelevantExecutionSummaryContext
	PassingSkills        map[string]model.VerificationRecord
	SelectedReviewSkills []string
	SkillBlockers        []model.ReasonCode
	LayerBlockers        []model.ReasonCode
	Blockers             []model.ReasonCode
}

type ShipAuthority struct {
	ReviewAuthority        ReviewAuthority
	Actions                []governance.RequiredAction
	Paths                  state.ResolvedChangePaths
	ManifestOK             bool
	ManifestBlockers       []model.ReasonCode
	ArtifactReady          bool
	VerifyPassingSkills    map[string]model.VerificationRecord
	VerifySkillBlockers    []model.ReasonCode
	VerificationReady      bool
	RequiredActionBlockers []model.ReasonCode
	HighRiskChecks         map[string]bool
	Result                 gate.GateEvaluation
}

func loadRuntimeGovernanceInputs(root string, change model.Change) (runtimeGovernanceInputs, error) {
	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return runtimeGovernanceInputs{}, err
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return runtimeGovernanceInputs{}, err
	}
	return runtimeGovernanceInputs{
		Policy: policy,
		Paths:  paths,
	}, nil
}

// FinalCloseoutEvidenceRequired returns whether final-closeout is required as
// S4 governance evidence for the resolved policy.
func FinalCloseoutEvidenceRequired(policy governance.PresetPolicy) bool {
	return policy.CloseoutRefreshRequired || policy.EffectivePreset != model.WorkflowPresetLight
}

func EvaluateReviewAuthority(root string, change model.Change) (ReviewAuthority, error) {
	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return ReviewAuthority{}, err
	}
	return evaluateReviewAuthorityWithPolicy(root, change, policy)
}

func evaluateReviewAuthorityWithPolicy(root string, change model.Change, policy governance.PresetPolicy) (ReviewAuthority, error) {
	executionSummaryCtx, err := state.LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		return ReviewAuthority{}, err
	}
	reviewSelection, err := reviewSkillSelectionForAuthority(root, change)
	if err != nil {
		return ReviewAuthority{}, err
	}
	selectedReviewSkills := engineskill.SelectedReviewSkills(reviewSelection)
	passingSkills, skillBlockers, err := EvaluateRequiredSkillsForChangeWithReviewSelection(
		root,
		change,
		model.StateS3Review,
		executionSummaryCtx.LatestRunVersion,
		policy.CloseoutRefreshRequired,
		reviewSelection,
	)
	if err != nil {
		return ReviewAuthority{}, err
	}
	blockers := model.ReasonCodesFromSpecs(skillBlockers)
	layerBlockers := []model.ReasonCode{}
	if artifactReviewEvidence, ok := passingSkills[SkillSpecComplianceReview]; ok {
		artifactCtx := resolveArtifactEvaluationContext(change, policy.EffectivePreset)
		projection, err := projectArtifactProjectionWithContext(root, change, artifactCtx)
		if err != nil {
			return ReviewAuthority{}, err
		}
		implementationReviewEvidence := passingSkills[SkillCodeQualityReview]
		layerBlockers = EvaluateReviewLayerBlockersFromNamedEvidence(change, artifactReviewEvidence, implementationReviewEvidence, &projection, false)
		blockers = append(blockers, layerBlockers...)
	}
	blockers = append(blockers, model.ReasonCodesFromSpecs(executionSummaryCtx.Issues)...)
	blockers = append(blockers, crossStageContextDistinctBlockers(
		root,
		change,
		passingSkills,
		crossStageContextReviewStagesForSelectedSkills(selectedReviewSkills),
		crossStageContextOwnedReviewStagesForSelectedSkills(selectedReviewSkills),
		policy.EffectivePreset != model.WorkflowPresetLight,
	)...)
	blockers = model.NormalizeReasonCodes(blockers)

	return ReviewAuthority{
		Policy:               policy,
		ExecutionSummaryCtx:  executionSummaryCtx,
		PassingSkills:        passingSkills,
		SelectedReviewSkills: selectedReviewSkills,
		SkillBlockers:        model.ReasonCodesFromSpecs(skillBlockers),
		LayerBlockers:        model.NormalizeReasonCodes(layerBlockers),
		Blockers:             blockers,
	}, nil
}

func reviewSkillSelectionForAuthority(root string, change model.Change) (engineskill.ReviewSkillSelection, error) {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return engineskill.ReviewSkillSelection{}, err
	}
	snap, err := previewGovernanceSnapshotForReadiness(root, change, paths.GovernedBundleDir)
	if err != nil {
		return engineskill.ReviewSkillSelection{}, err
	}
	return ReviewSkillSelectionFromControls(snap.ActiveControls), nil
}

func EvaluateShipAuthority(root string, change model.Change) (ShipAuthority, error) {
	verifyState := model.StateS4Verify
	readiness, err := evaluateGovernanceReadinessBase(
		root,
		change,
		GovernanceReadinessOptions{
			WorkflowStateOverride: &verifyState,
			IncludeReviewSurface:  true,
		},
	)
	if err != nil {
		return ShipAuthority{}, err
	}
	return buildShipAuthorityFromReadiness(root, change, readiness)
}

func buildShipAuthorityFromReadiness(root string, change model.Change, readiness GovernanceReadiness) (ShipAuthority, error) {
	inputs, err := loadRuntimeGovernanceInputs(root, change)
	if err != nil {
		return ShipAuthority{}, err
	}
	reviewAuthority, ok := readiness.cachedReviewAuthority()
	if !ok {
		reviewAuthority, err = evaluateReviewAuthorityWithPolicy(root, change, inputs.Policy)
		if err != nil {
			return ShipAuthority{}, err
		}
	}
	verifyPassingSkills := cloneVerificationRecords(readiness.PassingSkills)
	// Assurance attestation is required on every standard/strict effective preset,
	// matching the Layer 2 floor (AssuranceContractBlockers
	// and the done gate) — NOT CloseoutRefreshRequired. The latter also trips for
	// light + quality_mode=full, where assurance.md is optional and the closeout
	// template instructs light to omit the reference; gating on it would block a
	// valid light/full closeout and, conversely, miss a plain standard closeout
	// (CloseoutRefreshRequired is false there). EffectivePreset keeps both layers
	// consistent.
	assuranceRequired := inputs.Policy.EffectivePreset != model.WorkflowPresetLight
	attestationBlockers := closeoutAssuranceAttestationBlockers(verifyPassingSkills, assuranceRequired)
	reuseBlockers := closeoutGoalVerificationReuseBlockers(root, change, verifyPassingSkills, reviewAuthority.PassingSkills, readiness.ExecutionSummary)
	// P1 (issue #47 follow-on): independence facets gated on the effective preset,
	// matching the assurance attestation floor — required on standard/strict,
	// advisory (omitted) on light.
	independenceRequired := inputs.Policy.EffectivePreset != model.WorkflowPresetLight
	independencePresenceBlockers := closeoutReviewerIndependenceBlockers(verifyPassingSkills, independenceRequired)
	selectedReviewSkills := selectedReviewSkillsForAuthority(reviewAuthority)
	chainOrderBlockers := closeoutChainOrderBlockers(
		verifyPassingSkills,
		reviewAuthority.PassingSkills,
		selectedReviewSkills,
		independenceRequired,
	)
	// Ship owns the goal/closeout edges of the cross-stage distinct-context
	// lattice. It re-loads the base review participants (executor, audit_origin,
	// and selected reviewer skill names) and adds goal + closeout, but owns only
	// the goal/closeout stages, so review-owned edges do not re-fire here. Review
	// handles are read from the review authority's passing records, the same
	// surface the review gate used.
	shipContextBlockers := crossStageContextDistinctBlockers(
		root,
		change,
		mergeContextHandleRecords(reviewAuthority.PassingSkills, verifyPassingSkills),
		crossStageContextShipStagesForSelectedSkills(selectedReviewSkills),
		crossStageContextOwnedShipStages,
		independenceRequired,
	)
	verifySkillBlockers := append([]model.ReasonCode(nil), readiness.SkillBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, attestationBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, reuseBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, independencePresenceBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, chainOrderBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, shipContextBlockers...)

	manifestOK, manifestBlockers := ValidateChangeYamlR0(
		filepath.Join(inputs.Paths.GovernedBundleDir, "change.yaml"),
		change.Slug,
	)
	artifactReady := readiness.ArtifactReadiness.Ready
	verificationReady := len(verifySkillBlockers) == 0 &&
		len(reviewAuthority.Blockers) == 0 &&
		ComputeVerificationReadiness(verifyPassingSkills, FinalCloseoutEvidenceRequired(inputs.Policy))
	requiredActions := cloneRequiredActions(readiness.RequiredActions)
	requiredActionBlockers := model.ReasonCodesFromSpecs(governance.RequiredActionBlockers(change, requiredActions))
	highRiskChecks := ExtractHighRiskChecks(verifyPassingSkills)

	unresolved := append([]model.ReasonCode{}, readiness.Blockers...)
	unresolved = append(unresolved, model.ReasonCodesFromSpecs(manifestBlockers)...)
	// Surface the attestation blocker as an actionable G_ship reason — the same
	// channel the Layer 2 placeholder blocker travels (readiness.Blockers). Left
	// only in verifySkillBlockers it would flip verificationReady but EvaluateGShip
	// would emit only the generic verification_evidence_missing, hiding the
	// specific, actionable closeout_assurance_attestation_missing code.
	unresolved = append(unresolved, attestationBlockers...)
	unresolved = append(unresolved, reuseBlockers...)
	unresolved = append(unresolved, independencePresenceBlockers...)
	unresolved = append(unresolved, chainOrderBlockers...)
	unresolved = append(unresolved, shipContextBlockers...)
	unresolved = model.NormalizeReasonCodes(unresolved)

	return ShipAuthority{
		ReviewAuthority:        reviewAuthority,
		Actions:                requiredActions,
		Paths:                  inputs.Paths,
		ManifestOK:             manifestOK,
		ManifestBlockers:       model.ReasonCodesFromSpecs(manifestBlockers),
		ArtifactReady:          artifactReady,
		VerifyPassingSkills:    verifyPassingSkills,
		VerifySkillBlockers:    model.NormalizeReasonCodes(verifySkillBlockers),
		VerificationReady:      verificationReady,
		RequiredActionBlockers: model.NormalizeReasonCodes(requiredActionBlockers),
		HighRiskChecks:         highRiskChecks,
		Result: gate.EvaluateGShip(
			change,
			artifactReady,
			verificationReady,
			manifestOK,
			unresolved,
			highRiskChecks,
		),
	}, nil
}

// assuranceCompleteReference is the AI-driven closeout attestation: the host's
// structured judgment, recorded in the final-closeout verification record, that
// every required assurance section is genuinely authored rather than scaffold.
const assuranceCompleteReference = "closeout:assurance_complete=pass"

const closeoutGoalVerificationReuseReference = "closeout:goal_verification_reuse=pass"
const closeoutGoalVerificationReuseRunVersionPrefix = "closeout:goal_verification_reuse_run_version="
const s4VerificationRecoveryRemediation = "rerun goal-verification, then rerun final-closeout"

func S4VerificationRecoveryRemediation() string {
	return s4VerificationRecoveryRemediation
}

// closeoutAssuranceAttestationBlockers enforces Layer 1 of issue #47. When
// assurance is required for the change's effective preset (standard/strict),
// the passing final-closeout record must carry the assurance-complete
// attestation. The kernel does not re-read assurance prose here; it only
// requires the AI's attestation to be present, so a closeout cannot reach ship
// without the host explicitly vouching for the assurance content. Missing
// standard/strict final-closeout evidence is the same attestation failure as a
// passing-but-unattested record. Light preset (assurance optional) is
// unaffected.
func closeoutAssuranceAttestationBlockers(passingSkills map[string]model.VerificationRecord, assuranceRequired bool) []model.ReasonCode {
	if !assuranceRequired {
		return nil
	}
	record, ok := passingSkills[SkillFinalCloseout]
	if !ok {
		return []model.ReasonCode{closeoutAssuranceAttestationMissingBlocker()}
	}
	for _, ref := range record.References {
		if strings.TrimSpace(ref) == assuranceCompleteReference {
			return nil
		}
	}
	return []model.ReasonCode{closeoutAssuranceAttestationMissingBlocker()}
}

func closeoutAssuranceAttestationMissingBlocker() model.ReasonCode {
	return model.NewReasonCode(
		"closeout_assurance_attestation_missing",
		"final-closeout must record "+assuranceCompleteReference+" on standard/strict",
	)
}

func closeoutGoalVerificationReuseBlockers(
	root string,
	change model.Change,
	passingSkills map[string]model.VerificationRecord,
	reviewPassingSkills map[string]model.VerificationRecord,
	summary *model.ExecutionSummary,
) []model.ReasonCode {
	closeoutRecord, ok := passingSkills[SkillFinalCloseout]
	if !ok || !closeoutRecord.IsPassing() {
		return nil
	}
	if !referencesContain(closeoutRecord.References, closeoutGoalVerificationReuseReference) {
		return nil
	}
	reuseRunVersion, ok, err := closeoutGoalVerificationReuseRunVersion(closeoutRecord.References)
	if err != nil {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker(err.Error())}
	}
	if !ok {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker(
			"final-closeout must record " + closeoutGoalVerificationReuseRunVersionPrefix + "<run_version>",
		)}
	}
	if reuseRunVersion < 1 {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker("reuse run_version must be >= 1")}
	}

	goalRecord, ok := passingSkills[SkillGoalVerification]
	if !ok || !goalRecord.IsPassing() {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker(
			"goal-verification must be passing before final-closeout can reuse it",
		)}
	}
	if goalRecord.RunVersion != reuseRunVersion {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker(
			fmtReuseRunMismatch("goal-verification", reuseRunVersion, goalRecord.RunVersion),
		)}
	}
	if closeoutRecord.RunVersion != reuseRunVersion {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker(
			fmtReuseRunMismatch("final-closeout", reuseRunVersion, closeoutRecord.RunVersion),
		)}
	}
	if !state.ExecutionSummaryReady(summary) {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker(
			"execution-summary.yaml must be ready before final-closeout can reuse goal-verification",
		)}
	}
	if summary.RunSummaryVersion != reuseRunVersion {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker(
			fmtReuseRunMismatch("execution-summary", reuseRunVersion, summary.RunSummaryVersion),
		)}
	}
	latestExecutionEvidenceAt := summary.LatestRelevantUpdateAt().UTC()
	if !latestExecutionEvidenceAt.IsZero() && goalRecord.Timestamp.UTC().Before(latestExecutionEvidenceAt) {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker(
			"goal-verification timestamp must be at or after latest execution evidence",
		)}
	}
	// The cross-stage ordering halves (review <= goal, closeout >= goal) have moved
	// out of this opt-in reuse gate into the always-on closeoutChainOrderBlockers
	// invariant, which carries its own distinct reason code.
	freshness := state.ExecutionSummaryFreshness(root, change, summary)
	if freshness != ctxpack.EvidenceFreshnessFresh {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker(
			"execution-summary freshness must be fresh, got " + string(freshness),
		)}
	}
	if blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillGoalVerification, summary); err != nil {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker("goal-verification digest cannot be evaluated: " + err.Error())}
	} else if len(blockers) > 0 {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker("goal-verification inputs changed: " + strings.Join(blockers, ","))}
	}
	if blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillFinalCloseout, summary); err != nil {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker("final-closeout digest cannot be evaluated: " + err.Error())}
	} else if len(blockers) > 0 {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker("final-closeout inputs changed: " + strings.Join(blockers, ","))}
	}
	return nil
}

// closeoutReviewerIndependenceReference is the engine-consumed presence facet of
// the P1 independence contract: the final-closeout record's structured
// attestation that the closeout judgment was produced by an independent reviewer
// context. Pattern-A presence (analogue of assuranceCompleteReference): the
// kernel only requires the token to be present, it does not re-derive
// independence here.
const closeoutReviewerIndependenceReference = "closeout:reviewer_independence=pass"

// closeoutReviewerIndependenceBlockers enforces the P1 presence facet (REQ-001).
// When required (standard/strict effective preset), the passing final-closeout
// record must carry the reviewer-independence attestation; a missing record or a
// record lacking the token is a fail-closed blocker. Advisory (returns nil) on
// light.
func closeoutReviewerIndependenceBlockers(passingSkills map[string]model.VerificationRecord, required bool) []model.ReasonCode {
	if !required {
		return nil
	}
	record, ok := passingSkills[SkillFinalCloseout]
	if !ok {
		return []model.ReasonCode{closeoutReviewerIndependenceMissingBlocker()}
	}
	for _, ref := range record.References {
		if strings.TrimSpace(ref) == closeoutReviewerIndependenceReference {
			return nil
		}
	}
	return []model.ReasonCode{closeoutReviewerIndependenceMissingBlocker()}
}

func closeoutReviewerIndependenceMissingBlocker() model.ReasonCode {
	return model.NewReasonCode(
		"closeout_reviewer_independence_missing",
		"final-closeout must record "+closeoutReviewerIndependenceReference+" on standard/strict; rerun final-closeout",
	)
}

// closeoutChainOrderBlockers enforces the always-on P1 cross-stage ordering
// invariant closeout >= goal-verification >= max(selected review evidence)
// across the independence-critical verdicts (REQ-001). It is independent of the
// opt-in closeout:goal_verification_reuse=pass token and carries its own distinct
// reason code closeout_chain_order_invalid (never folded into
// closeout_goal_verification_reuse_invalid). Each pair is compared only when BOTH
// records are present, passing, and carry a non-zero timestamp; a genuinely
// absent record is owned by the required-skill-missing blocker, so this gate does
// not double-fire on absence. Advisory (returns nil) on light.
func closeoutChainOrderBlockers(
	passingSkills map[string]model.VerificationRecord,
	reviewPassingSkills map[string]model.VerificationRecord,
	selectedReviewSkills []string,
	required bool,
) []model.ReasonCode {
	if !required {
		return nil
	}
	goalRecord, ok := passingSkills[SkillGoalVerification]
	if !ok || !goalRecord.IsPassing() || goalRecord.Timestamp.IsZero() {
		// Goal absence/staleness is owned by the required-skill-missing and reuse
		// gates; ordering has nothing to compare against.
		return nil
	}
	goalAt := goalRecord.Timestamp.UTC()

	// review <= goal: each present, passing, non-zero review verdict must not be
	// stamped after goal-verification.
	for _, skillName := range normalizeReviewSkillNames(selectedReviewSkills) {
		reviewRecord, ok := reviewPassingSkills[skillName]
		if !ok || !reviewRecord.IsPassing() || reviewRecord.Timestamp.IsZero() {
			continue
		}
		if reviewRecord.Timestamp.UTC().After(goalAt) {
			return []model.ReasonCode{closeoutChainOrderInvalidBlocker(
				"goal-verification must be at or after review evidence: " + skillName,
			)}
		}
	}

	// closeout >= goal: when final-closeout is present, passing, and timestamped it
	// must not predate goal-verification.
	closeoutRecord, ok := passingSkills[SkillFinalCloseout]
	if ok && closeoutRecord.IsPassing() && !closeoutRecord.Timestamp.IsZero() {
		if closeoutRecord.Timestamp.UTC().Before(goalAt) {
			return []model.ReasonCode{closeoutChainOrderInvalidBlocker(
				"final-closeout must not predate goal-verification",
			)}
		}
	}
	return nil
}

func closeoutChainOrderInvalidBlocker(detail string) model.ReasonCode {
	return model.NewReasonCode("closeout_chain_order_invalid", appendS4VerificationRecovery(detail))
}

// crossStageContextOwnedReviewStagesForSelectedSkills is the set of lattice
// stages the review authority owns: the executor handle set, the plan auditor,
// and each selected review skill by skill name. CrossStageContextCollisions
// keeps an edge only when at least one endpoint is owned, so the review gate
// fires the selected reviewer/executor/audit-origin edges without re-owning the
// plan author/auditor self-audit edge (owned by the plan gate) or the
// goal/closeout edges (owned by the ship gate).
func crossStageContextOwnedReviewStagesForSelectedSkills(selectedReviewSkills []string) map[string]struct{} {
	stages := map[string]struct{}{
		model.StageContextExecutor:    {},
		model.StageContextAuditOrigin: {},
	}
	for _, skillName := range normalizeReviewSkillNames(selectedReviewSkills) {
		stages[skillName] = struct{}{}
	}
	return stages
}

// crossStageContextOwnedShipStages is the set of lattice stages the ship
// authority newly owns: goal-verification and final-closeout. The same base
// participant set is re-loaded plus these two stages; owning exactly goal +
// closeout adds only the edges those two stages introduce without double-firing
// review-owned edges, which carry no owned endpoint here.
var crossStageContextOwnedShipStages = map[string]struct{}{
	model.StageContextGoal:     {},
	model.StageContextCloseout: {},
}

// crossStageContextStageHandleSkill maps a single-handle lattice stage to the
// skill whose passing verification record carries its context-origin handle.
// executor is excluded (it is a handle set sourced from the wave record);
// audit_origin is excluded (it rides the plan-audit record's audit_origin token,
// not a context_origin:stage= token).
var crossStageContextStageHandleSkill = map[string]string{
	model.StageContextGoal:     SkillGoalVerification,
	model.StageContextCloseout: SkillFinalCloseout,
}

// crossStageContextParticipants builds the lattice participants for the requested
// single-handle stages plus the executor handle set and the plan auditor handle.
// Only present-and-passing records contribute a participant; an absent or
// non-passing record contributes nothing and is silent (its absence is owned by
// the required-skill-missing blocker, not this gate). A present-passing record
// that yields no well-formed context-origin handle fails closed with
// context_origin_handle_invalid. The returned invalid blockers, when non-empty,
// short-circuit collision evaluation in the caller.
func crossStageContextParticipants(
	root string,
	change model.Change,
	passingSkills map[string]model.VerificationRecord,
	stages map[string]struct{},
) (map[string]model.ContextParticipant, []model.ReasonCode) {
	participants := map[string]model.ContextParticipant{}
	var invalid []model.ReasonCode

	// executor <- S2 wave-orchestration record, flattened to a handle set. The
	// wave record is the executor stage's evidence; a present-passing wave record
	// with no executor handles yields an empty set, which collides with nothing
	// and is treated as silent rather than invalid (per-task executor handles are
	// owned by the executor_agent_missing gate, not this lattice).
	if _, want := stages[model.StageContextExecutor]; want {
		if waveRecord, ok, err := LatestPassingWaveEvidence(root, change.Slug); err == nil && ok {
			set := model.ExecutorParticipantHandleSetFromVerification(waveRecord)
			if len(set) > 0 {
				participants[model.StageContextExecutor] = model.ContextParticipant{HandleSet: set}
			}
		}
	}

	// audit_origin <- S1 plan-audit record's audit_origin handle. A present
	// plan-audit record with no well-formed audit_origin handle fails closed.
	if _, want := stages[model.StageContextAuditOrigin]; want {
		if record, ok := loadPresentPassingVerification(root, change.Slug, SkillPlanAudit); ok {
			handle, ok := model.AuditOriginHandleFromVerification(record)
			if !ok {
				invalid = append(invalid, contextOriginHandleInvalidBlocker(
					SkillPlanAudit+" ("+model.StageContextAuditOrigin+") recorded no well-formed context-origin handle",
				))
			} else {
				participants[model.StageContextAuditOrigin] = model.ContextParticipant{Handle: handle.Handle}
			}
		}
	}

	// Selected review skills <- each selected reviewer's passing record, parsed
	// for context_origin:stage=review=<handle>. The participant key is the skill
	// name, not the shared "review" stage label, so peer reviewers can collide
	// with each other deterministically.
	for _, skillName := range reviewParticipantSkillNames(stages) {
		record, ok := passingSkills[skillName]
		if !ok || !record.IsPassing() {
			continue
		}
		handle, ok := model.ReviewContextOriginHandleFromVerification(record)
		if !ok {
			invalid = append(invalid, contextOriginHandleInvalidBlocker(
				skillName+" ("+model.StageContextReview+") recorded no context-origin handle for selected reviewer",
			))
			continue
		}
		participants[skillName] = model.ContextParticipant{Handle: handle.Handle}
	}

	// Single-handle verification/closeout stages <- each owning skill's passing
	// record, parsed for its self-describing context_origin:stage= handle.
	for _, stage := range []string{
		model.StageContextGoal,
		model.StageContextCloseout,
	} {
		if _, want := stages[stage]; !want {
			continue
		}
		skillName := crossStageContextStageHandleSkill[stage]
		record, ok := passingSkills[skillName]
		if !ok || !record.IsPassing() {
			continue
		}
		handles, ok := model.ContextOriginHandlesFromVerification(record)
		if !ok {
			invalid = append(invalid, contextOriginHandleInvalidBlocker(
				skillName+" ("+stage+") recorded no well-formed context-origin handle",
			))
			continue
		}
		handle, ok := handles[stage]
		if !ok || strings.TrimSpace(handle.Handle) == "" {
			invalid = append(invalid, contextOriginHandleInvalidBlocker(
				skillName+" ("+stage+") recorded no context-origin handle for stage "+stage,
			))
			continue
		}
		participants[stage] = model.ContextParticipant{Handle: handle.Handle}
	}

	return participants, model.NormalizeReasonCodes(invalid)
}

// mergeContextHandleRecords overlays the verify-stage passing records (goal,
// closeout) onto the review-stage passing records so the ship lattice can
// resolve every single-handle stage from one map. The review records are the
// selected-reviewer source the review gate already used; the verify records win
// on key collision since they are the ship gate's own surface.
func mergeContextHandleRecords(
	reviewPassingSkills map[string]model.VerificationRecord,
	verifyPassingSkills map[string]model.VerificationRecord,
) map[string]model.VerificationRecord {
	merged := make(map[string]model.VerificationRecord, len(reviewPassingSkills)+len(verifyPassingSkills))
	for skillName, record := range reviewPassingSkills {
		merged[skillName] = record
	}
	for skillName, record := range verifyPassingSkills {
		merged[skillName] = record
	}
	return merged
}

func loadPresentPassingVerification(root, slug, skillName string) (model.VerificationRecord, bool) {
	record, err := state.LoadVerification(root, slug, skillName)
	if err != nil {
		// An absent record (or any load failure) contributes no participant and is
		// silent here; record presence/freshness is owned by the
		// required-skill-missing and digest gates, not the distinct-context lattice.
		return model.VerificationRecord{}, false
	}
	if !record.IsPassing() {
		return model.VerificationRecord{}, false
	}
	return record, true
}

// crossStageContextDistinctBlockers enforces the generalized P2 distinct-context
// lattice (REQ-002). It builds the lattice participants for the requested stages,
// fails closed with context_origin_handle_invalid for any present-passing record
// that lacks a well-formed handle, and otherwise emits one
// cross_stage_context_not_distinct blocker per colliding stage pair that has an
// owned endpoint. Advisory (returns nil) on light.
func crossStageContextDistinctBlockers(
	root string,
	change model.Change,
	passingSkills map[string]model.VerificationRecord,
	stages map[string]struct{},
	ownedStages map[string]struct{},
	required bool,
) []model.ReasonCode {
	if !required {
		return nil
	}
	participants, invalid := crossStageContextParticipants(root, change, passingSkills, stages)
	if len(invalid) > 0 {
		// A malformed handle makes the distinctness comparison meaningless; fail
		// closed on the invalid-handle code rather than emit a misleading collision.
		return invalid
	}
	collisions := model.CrossStageContextCollisions(participants, ownedStages)
	if len(collisions) == 0 {
		return nil
	}
	blockers := make([]model.ReasonCode, 0, len(collisions))
	for _, pair := range collisions {
		blockers = append(blockers, crossStageContextNotDistinctBlocker(pair[0]+"|"+pair[1]))
	}
	return model.NormalizeReasonCodes(blockers)
}

// crossStageContextReviewStagesForSelectedSkills is the participant set both
// review and ship load: executor, audit_origin, and the selected reviewer skill
// names. It mirrors the review-owned stage set exactly; the ship gate adds goal
// + closeout to this base before evaluating its own edges.
func crossStageContextReviewStagesForSelectedSkills(selectedReviewSkills []string) map[string]struct{} {
	return crossStageContextOwnedReviewStagesForSelectedSkills(selectedReviewSkills)
}

func crossStageContextShipStagesForSelectedSkills(selectedReviewSkills []string) map[string]struct{} {
	stages := crossStageContextReviewStagesForSelectedSkills(selectedReviewSkills)
	stages[model.StageContextGoal] = struct{}{}
	stages[model.StageContextCloseout] = struct{}{}
	return stages
}

func selectedReviewSkillsForAuthority(authority ReviewAuthority) []string {
	if len(authority.SelectedReviewSkills) > 0 {
		return normalizeReviewSkillNames(authority.SelectedReviewSkills)
	}
	return engineskill.SelectedReviewSkills(engineskill.ReviewSkillSelection{})
}

func normalizeReviewSkillNames(skillNames []string) []string {
	if len(skillNames) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(skillNames))
	for _, skillName := range skillNames {
		trimmed := strings.TrimSpace(skillName)
		if trimmed == "" || !engineskill.IsReviewSkill(trimmed) {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	slices.Sort(normalized)
	return normalized
}

func reviewParticipantSkillNames(stages map[string]struct{}) []string {
	if len(stages) == 0 {
		return nil
	}
	names := make([]string, 0, len(stages))
	for stage := range stages {
		if engineskill.IsReviewSkill(stage) {
			names = append(names, stage)
		}
	}
	slices.Sort(names)
	return names
}

func contextOriginHandleInvalidBlocker(detail string) model.ReasonCode {
	return model.NewReasonCode(
		"context_origin_handle_invalid",
		strings.TrimSpace(detail)+"; re-run the owning stage in a fresh native subagent so it records a valid context-origin handle",
	)
}

func crossStageContextNotDistinctBlocker(detail string) model.ReasonCode {
	return model.NewReasonCode(
		"cross_stage_context_not_distinct",
		strings.TrimSpace(detail),
	)
}

func closeoutGoalVerificationReuseContentPaths(change model.Change, summary *model.ExecutionSummary) []string {
	if summary == nil {
		return nil
	}
	paths := []string{}
	appendPaths := func(values []string) {
		for _, path := range values {
			trimmed := strings.TrimSpace(filepath.ToSlash(path))
			if trimmed == "" {
				continue
			}
			if closeoutGoalVerificationReuseSkipsContentPath(change, trimmed) {
				continue
			}
			paths = append(paths, trimmed)
		}
	}
	for _, task := range summary.Tasks {
		appendPaths(task.ChangedFiles)
		appendPaths(task.TargetFiles)
	}
	return stringutil.UniqueSorted(paths)
}

func closeoutGoalVerificationReuseSkipsContentPath(change model.Change, rel string) bool {
	trimmed := strings.Trim(strings.TrimSpace(filepath.ToSlash(rel)), "/")
	verificationDir := "artifacts/changes/" + strings.TrimSpace(change.Slug) + "/verification"
	return trimmed == verificationDir || strings.HasPrefix(trimmed, verificationDir+"/")
}

func closeoutGoalVerificationReuseWorkspacePaths(workspaceRoot, rel string) ([]string, bool, error) {
	trimmed := strings.TrimSpace(filepath.ToSlash(rel))
	if trimmed == "" {
		return nil, false, nil
	}
	if strings.HasPrefix(trimmed, "/") {
		return nil, false, nil
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return nil, false, nil
		}
	}
	if strings.ContainsAny(trimmed, "*?[") {
		matches, err := filepath.Glob(filepath.Join(workspaceRoot, filepath.FromSlash(trimmed)))
		if err != nil {
			return nil, true, err
		}
		if len(matches) == 0 {
			return nil, true, os.ErrNotExist
		}
		return stringutil.UniqueSorted(matches), true, nil
	}
	return []string{filepath.Join(workspaceRoot, filepath.FromSlash(trimmed))}, true, nil
}

func closeoutGoalVerificationReuseRunVersion(references []string) (int, bool, error) {
	for _, ref := range references {
		trimmed := strings.TrimSpace(ref)
		if !strings.HasPrefix(trimmed, closeoutGoalVerificationReuseRunVersionPrefix) {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(trimmed, closeoutGoalVerificationReuseRunVersionPrefix))
		if raw == "" {
			return 0, true, strconv.ErrSyntax
		}
		runVersion, err := strconv.Atoi(raw)
		if err != nil {
			return 0, true, err
		}
		return runVersion, true, nil
	}
	return 0, false, nil
}

func referencesContain(references []string, needle string) bool {
	for _, ref := range references {
		if strings.TrimSpace(ref) == needle {
			return true
		}
	}
	return false
}

func closeoutGoalVerificationReuseInvalidBlocker(detail string) model.ReasonCode {
	return model.NewReasonCode("closeout_goal_verification_reuse_invalid", appendS4VerificationRecovery(detail))
}

func fmtReuseRunMismatch(subject string, expected, got int) string {
	return subject + " run_version mismatch: expected=" + strconv.Itoa(expected) + " got=" + strconv.Itoa(got)
}

func appendS4VerificationRecovery(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return s4VerificationRecoveryRemediation
	}
	if strings.Contains(detail, "goal-verification") && strings.Contains(detail, "final-closeout") {
		return detail
	}
	return detail + "; " + s4VerificationRecoveryRemediation
}

func EvaluateReviewLayerBlockersFromNamedEvidence(
	change model.Change,
	artifactReviewEvidence model.VerificationRecord,
	implementationReviewEvidence model.VerificationRecord,
	projection *ArtifactProjection,
	reviewAll bool,
) []model.ReasonCode {
	if artifactReviewEvidence.Verdict == "" {
		return model.ReasonCodesFromSpecs([]string{"required_skill_missing:spec-compliance-review"})
	}
	return evaluateReviewLayerBlockersByEvidence(change, artifactReviewEvidence, implementationReviewEvidence, projection, reviewAll)
}

func RequiredReviewLayerTokensForSkill(
	change model.Change,
	projection *ArtifactProjection,
	reviewAll bool,
	skillName string,
) []string {
	requiredLayerNames := RequiredReviewLayerNamesForSkill(change, projection, reviewAll, skillName)
	if len(requiredLayerNames) == 0 {
		return nil
	}
	tokens := make([]string, 0, len(requiredLayerNames))
	for _, layerName := range requiredLayerNames {
		tokens = append(tokens, "layer:"+layerName+"=pass")
	}
	return stringutil.UniqueSorted(tokens)
}

func RequiredReviewLayerNamesForSkill(
	change model.Change,
	projection *ArtifactProjection,
	reviewAll bool,
	skillName string,
) []string {
	requiredLayers := requiredReviewLayersForSkill(change, projection, reviewAll, skillName)
	if len(requiredLayers) == 0 {
		return nil
	}
	layerNames := make([]string, 0, len(requiredLayers))
	for layer := range requiredLayers {
		layerNames = append(layerNames, string(layer))
	}
	return stringutil.UniqueSorted(layerNames)
}

func requiredReviewLayersForSkill(
	change model.Change,
	projection *ArtifactProjection,
	reviewAll bool,
	skillName string,
) map[reviewengine.ReviewLayer]struct{} {
	requiredLayers := map[reviewengine.ReviewLayer]struct{}{}
	switch skillName {
	case SkillSpecComplianceReview:
		for _, artifactID := range artifactScopeForReview(projection, reviewAll) {
			artifactName := reviewArtifactNameForLayers(artifactID)
			for _, layer := range reviewengine.RequiredArtifactLayers(change.GuardrailDomain, artifactName) {
				requiredLayers[layer] = struct{}{}
			}
		}
	case SkillCodeQualityReview:
		for _, layer := range reviewengine.RequiredImplementationLayers(change.GuardrailDomain) {
			requiredLayers[layer] = struct{}{}
		}
	}
	return requiredLayers
}

func evaluateReviewLayerBlockersByEvidence(
	change model.Change,
	artifactReviewEvidence model.VerificationRecord,
	implementationReviewEvidence model.VerificationRecord,
	projection *ArtifactProjection,
	reviewAll bool,
) []model.ReasonCode {
	requiredArtifactLayers := requiredReviewLayersForSkill(change, projection, reviewAll, SkillSpecComplianceReview)
	requiredImplementationLayers := requiredReviewLayersForSkill(change, projection, reviewAll, SkillCodeQualityReview)
	if implementationReviewEvidence.Verdict == "" {
		requiredImplementationLayers = map[reviewengine.ReviewLayer]struct{}{}
	}

	blockers := make([]string, 0, len(requiredArtifactLayers)+len(requiredImplementationLayers))
	blockers = append(blockers, reviewLayerBlockerSpecs(requiredArtifactLayers, parseReviewLayerOutcomes(artifactReviewEvidence.References))...)
	blockers = append(blockers, reviewLayerBlockerSpecs(requiredImplementationLayers, parseReviewLayerOutcomes(implementationReviewEvidence.References))...)
	return model.ReasonCodesFromSpecs(stringutil.UniqueSorted(blockers))
}

func reviewLayerBlockerSpecs(
	requiredLayers map[reviewengine.ReviewLayer]struct{},
	outcomes map[reviewengine.ReviewLayer]bool,
) []string {
	blockers := make([]string, 0, len(requiredLayers))
	for layer := range requiredLayers {
		passed, ok := outcomes[layer]
		if !ok {
			blockers = append(blockers, "review_layer_missing:"+string(layer))
			continue
		}
		if !passed {
			blockers = append(blockers, "review_layer_failed:"+string(layer))
		}
	}
	return blockers
}

// artifactScopeForReview maps the in-memory projection to the artifact IDs whose
// review-layer evidence must be present. When projection is unavailable, the
// default changed-only review falls back to `change.yaml` only; full review does
// not invent artifact scope and therefore enforces only the domain-wide
// implementation layers.
func artifactScopeForReview(projection *ArtifactProjection, reviewAll bool) []string {
	if projection == nil {
		if reviewAll {
			return nil
		}
		return []string{"change"}
	}

	keys := make([]string, 0, len(projection.Nodes))
	for _, node := range projection.Nodes {
		key := reviewArtifactIDFromProjectionNode(node.Name)
		if key == "" {
			continue
		}
		if reviewAll {
			keys = append(keys, key)
			continue
		}
		if node.State == string(model.ArtifactLifecycleDraft) || node.State == string(model.ArtifactLifecycleStale) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 && !reviewAll {
		keys = append(keys, "change")
	}
	return stringutil.UniqueSorted(keys)
}

func reviewArtifactIDFromProjectionNode(name string) string {
	base := strings.TrimSpace(filepath.Base(name))
	switch strings.ToLower(base) {
	case "", ".":
		return ""
	case "change.yaml":
		return "change"
	default:
		return strings.TrimSuffix(base, ".md")
	}
}

func reviewArtifactNameForLayers(artifactID string) string {
	base := strings.TrimSpace(filepath.Base(artifactID))
	switch strings.ToLower(base) {
	case "", ".":
		return ""
	case "change", "change.yaml":
		return "change.yaml"
	}
	if filepath.Ext(base) != "" {
		return base
	}
	return base + ".md"
}

func parseReviewLayerOutcomes(references []string) map[reviewengine.ReviewLayer]bool {
	out := map[reviewengine.ReviewLayer]bool{}
	for _, ref := range references {
		raw := strings.TrimSpace(strings.ToLower(ref))
		if !strings.HasPrefix(raw, "layer:") {
			continue
		}
		raw = strings.TrimPrefix(raw, "layer:")
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			continue
		}
		layer := reviewengine.ReviewLayer(strings.ToUpper(strings.TrimSpace(parts[0])))
		switch strings.TrimSpace(parts[1]) {
		case "pass", "passed", "ok", "true":
			out[layer] = true
		case "fail", "failed", "false":
			out[layer] = false
		}
	}
	return out
}

func cloneRequiredActions(src []governance.RequiredAction) []governance.RequiredAction {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]governance.RequiredAction, 0, len(src))
	cloned = append(cloned, src...)
	return cloned
}
