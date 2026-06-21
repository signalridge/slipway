package progression

import (
	"fmt"
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
	PassingSkills        map[string]model.VerificationRecord
	SelectedReviewSkills []string
	SkillBlockers        []model.ReasonCode
	Blockers             []model.ReasonCode
}

type ShipAuthority struct {
	ReviewAuthority     ReviewAuthority
	Actions             []governance.RequiredAction
	VerifyPassingSkills map[string]model.VerificationRecord
	VerifySkillBlockers []model.ReasonCode
	Result              gate.GateEvaluation
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
// closeout governance evidence for the resolved policy.
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
	selectedReviewSkills := selectedReviewSkillsForSelection(change, reviewSelection)
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
	extraPassing, extraSkillBlockers, err := loadFreshPassingRecordsForSkills(
		root,
		change,
		selectedReviewSkills,
		executionSummaryCtx.Summary,
		passingSkills,
	)
	if err != nil {
		return ReviewAuthority{}, err
	}
	for skillName, record := range extraPassing {
		passingSkills[skillName] = record
	}
	skillBlockers = append(skillBlockers, model.ReasonSpecs(extraSkillBlockers)...)
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
		PassingSkills:        passingSkills,
		SelectedReviewSkills: selectedReviewSkills,
		SkillBlockers:        model.ReasonCodesFromSpecs(skillBlockers),
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

func loadFreshPassingRecordsForSkills(
	root string,
	change model.Change,
	skillNames []string,
	summary *model.ExecutionSummary,
	existing map[string]model.VerificationRecord,
) (map[string]model.VerificationRecord, []model.ReasonCode, error) {
	return loadFreshPassingRecordsForSkillsWithRequirement(root, change, skillNames, summary, existing, true)
}

func loadOptionalFreshPassingRecordsForSkills(
	root string,
	change model.Change,
	skillNames []string,
	summary *model.ExecutionSummary,
	existing map[string]model.VerificationRecord,
) (map[string]model.VerificationRecord, []model.ReasonCode, error) {
	return loadFreshPassingRecordsForSkillsWithRequirement(root, change, skillNames, summary, existing, false)
}

func loadFreshPassingRecordsForSkillsWithRequirement(
	root string,
	change model.Change,
	skillNames []string,
	summary *model.ExecutionSummary,
	existing map[string]model.VerificationRecord,
	required bool,
) (map[string]model.VerificationRecord, []model.ReasonCode, error) {
	out := map[string]model.VerificationRecord{}
	verifications, err := state.ListVerificationsForChange(root, change)
	if err != nil {
		return nil, nil, err
	}
	for _, skillName := range stringutil.UniqueSorted(skillNames) {
		skillName = strings.TrimSpace(skillName)
		if skillName == "" {
			continue
		}
		if record, ok := existing[skillName]; ok && record.IsPassing() {
			out[skillName] = record
			continue
		}
		record, ok := verifications[skillName]
		if !ok {
			if required {
				return out, []model.ReasonCode{model.NewReasonCode("required_skill_missing", skillName)}, nil
			}
			continue
		}
		if !record.IsPassing() {
			return out, []model.ReasonCode{model.NewReasonCode(requiredSkillReadinessBlockerCode(record), skillName)}, nil
		}
		digestBlockers, err := skillDigestFreshnessBlockersWithSummary(root, change, skillName, summary)
		if err != nil {
			return nil, nil, err
		}
		if len(digestBlockers) > 0 {
			return out, model.ReasonCodesFromSpecs(digestBlockers), nil
		}
		out[skillName] = record
	}
	return out, nil, nil
}

func requiredSkillReadinessBlockerCode(record model.VerificationRecord) string {
	switch {
	case record.Verdict == model.VerificationVerdictFail:
		return "required_skill_not_passed"
	case len(record.Blockers) > 0:
		return "required_skill_blockers_present"
	default:
		return "required_skill_not_ready"
	}
}

func EvaluateShipAuthority(root string, change model.Change) (ShipAuthority, error) {
	shipState := model.StateS3Review
	readiness, err := evaluateGovernanceReadinessBase(
		root,
		change,
		GovernanceReadinessOptions{
			WorkflowStateOverride: &shipState,
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
	selectedReviewSkills := selectedReviewSkillsForAuthority(reviewAuthority)
	goalPassing, goalSkillBlockers, err := loadFreshPassingRecordsForSkills(
		root,
		change,
		[]string{SkillGoalVerification},
		readiness.ExecutionSummary,
		verifyPassingSkills,
	)
	if err != nil {
		return ShipAuthority{}, err
	}
	for skillName, record := range goalPassing {
		verifyPassingSkills[skillName] = record
	}
	closeoutPassing, closeoutSkillBlockers, err := loadOptionalFreshPassingRecordsForSkills(
		root,
		change,
		[]string{SkillFinalCloseout},
		readiness.ExecutionSummary,
		verifyPassingSkills,
	)
	if err != nil {
		return ShipAuthority{}, err
	}
	for skillName, record := range closeoutPassing {
		verifyPassingSkills[skillName] = record
	}
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
	// P1 (issue #47 follow-on): independence facets gated on the effective preset,
	// matching the assurance attestation floor — required on standard/strict,
	// advisory (omitted) on light.
	independenceRequired := inputs.Policy.EffectivePreset != model.WorkflowPresetLight
	independencePresenceBlockers := closeoutReviewerIndependenceBlockers(verifyPassingSkills, independenceRequired)
	chainOrderBlockers := closeoutChainOrderBlockers(
		verifyPassingSkills,
		reviewAuthority.PassingSkills,
		selectedReviewSkills,
		independenceRequired,
	)
	reuseBlockers := closeoutGoalVerificationReuseBlockers(
		root,
		change,
		verifyPassingSkills,
		reviewAuthority.PassingSkills,
		readiness.ExecutionSummary,
	)
	reviewSetFinalizationBlockers := s3ReviewSetFinalizationBlockers(
		root,
		change,
		verifyPassingSkills,
		reviewAuthority.PassingSkills,
		selectedReviewSkills,
		readiness.ExecutionSummary,
		independenceRequired,
	)
	verifySkillBlockers := append([]model.ReasonCode(nil), readiness.SkillBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, goalSkillBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, closeoutSkillBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, attestationBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, independencePresenceBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, chainOrderBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, reuseBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, reviewSetFinalizationBlockers...)

	manifestOK, manifestBlockers := ValidateChangeYamlR0(
		filepath.Join(inputs.Paths.GovernedBundleDir, "change.yaml"),
		change.Slug,
	)
	artifactReady := readiness.ArtifactReadiness.Ready
	verificationReady := len(verifySkillBlockers) == 0 &&
		len(reviewAuthority.Blockers) == 0 &&
		ComputeVerificationReadiness(verifyPassingSkills, FinalCloseoutEvidenceRequired(inputs.Policy))
	requiredActions := cloneRequiredActions(readiness.RequiredActions)
	highRiskChecks := ExtractHighRiskChecks(verifyPassingSkills)

	unresolved := append([]model.ReasonCode{}, readiness.Blockers...)
	unresolved = append(unresolved, model.ReasonCodesFromSpecs(manifestBlockers)...)
	unresolved = append(unresolved, goalSkillBlockers...)
	unresolved = append(unresolved, closeoutSkillBlockers...)
	// Surface the attestation blocker as an actionable G_ship reason — the same
	// channel the Layer 2 placeholder blocker travels (readiness.Blockers). Left
	// only in verifySkillBlockers it would flip verificationReady but EvaluateGShip
	// would emit only the generic verification_evidence_missing, hiding the
	// specific, actionable closeout_assurance_attestation_missing code.
	unresolved = append(unresolved, attestationBlockers...)
	unresolved = append(unresolved, independencePresenceBlockers...)
	unresolved = append(unresolved, chainOrderBlockers...)
	unresolved = append(unresolved, reuseBlockers...)
	unresolved = append(unresolved, reviewSetFinalizationBlockers...)
	unresolved = model.NormalizeReasonCodes(unresolved)

	return ShipAuthority{
		ReviewAuthority:     reviewAuthority,
		Actions:             requiredActions,
		VerifyPassingSkills: verifyPassingSkills,
		VerifySkillBlockers: model.NormalizeReasonCodes(verifySkillBlockers),
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
const closeoutRecoveryRemediation = "rerun the selected reviewer set, then rerun final-closeout"

type proofReuseEdge struct {
	sourceSkill                         string
	sourceLabel                         string
	consumerSkill                       string
	consumerLabel                       string
	reuseRunVersion                     int
	requireExecutionSummaryFreshness    bool
	requireSourceAfterExecutionEvidence bool
	digestChecks                        []proofReuseDigestCheck
	blocker                             func(string) model.ReasonCode
}

type proofReuseDigestCheck struct {
	skillName string
	label     string
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

	blockers := proofReuseEdgeBlockers(root, change, passingSkills, summary, proofReuseEdge{
		sourceSkill:                         SkillGoalVerification,
		sourceLabel:                         "goal-verification",
		consumerSkill:                       SkillFinalCloseout,
		consumerLabel:                       "final-closeout",
		reuseRunVersion:                     reuseRunVersion,
		requireExecutionSummaryFreshness:    true,
		requireSourceAfterExecutionEvidence: true,
		digestChecks: []proofReuseDigestCheck{
			{skillName: SkillGoalVerification, label: "goal-verification"},
			{skillName: SkillFinalCloseout, label: "final-closeout"},
		},
		blocker: closeoutGoalVerificationReuseInvalidBlocker,
	})
	if len(blockers) > 0 {
		return blockers
	}
	return nil
}

func proofReuseEdgeBlockers(
	root string,
	change model.Change,
	passingSkills map[string]model.VerificationRecord,
	summary *model.ExecutionSummary,
	edge proofReuseEdge,
) []model.ReasonCode {
	blocker := edge.blocker
	if blocker == nil {
		blocker = func(detail string) model.ReasonCode {
			return model.NewReasonCode("proof_reuse_invalid", strings.TrimSpace(detail))
		}
	}
	if edge.reuseRunVersion < 1 {
		return []model.ReasonCode{blocker("reuse run_version must be >= 1")}
	}

	sourceRecord, ok := passingSkills[edge.sourceSkill]
	if !ok || !sourceRecord.IsPassing() {
		return []model.ReasonCode{blocker(
			proofReuseLabel(edge.sourceLabel, edge.sourceSkill) + " must be passing before " +
				proofReuseLabel(edge.consumerLabel, edge.consumerSkill) + " can reuse it",
		)}
	}
	consumerRecord, ok := passingSkills[edge.consumerSkill]
	if !ok || !consumerRecord.IsPassing() {
		return []model.ReasonCode{blocker(
			proofReuseLabel(edge.consumerLabel, edge.consumerSkill) + " must be passing before it can reuse " +
				proofReuseLabel(edge.sourceLabel, edge.sourceSkill),
		)}
	}
	if sourceRecord.RunVersion != edge.reuseRunVersion {
		return []model.ReasonCode{blocker(
			fmtReuseRunMismatch(proofReuseLabel(edge.sourceLabel, edge.sourceSkill), edge.reuseRunVersion, sourceRecord.RunVersion),
		)}
	}
	if consumerRecord.RunVersion != edge.reuseRunVersion {
		return []model.ReasonCode{blocker(
			fmtReuseRunMismatch(proofReuseLabel(edge.consumerLabel, edge.consumerSkill), edge.reuseRunVersion, consumerRecord.RunVersion),
		)}
	}
	if !state.ExecutionSummaryReady(summary) {
		return []model.ReasonCode{blocker(
			"execution-summary.yaml must be ready before " +
				proofReuseLabel(edge.consumerLabel, edge.consumerSkill) + " can reuse " +
				proofReuseLabel(edge.sourceLabel, edge.sourceSkill),
		)}
	}
	if summary.RunSummaryVersion != edge.reuseRunVersion {
		return []model.ReasonCode{blocker(
			fmtReuseRunMismatch("execution-summary", edge.reuseRunVersion, summary.RunSummaryVersion),
		)}
	}
	if edge.requireSourceAfterExecutionEvidence {
		latestExecutionEvidenceAt := summary.LatestRelevantUpdateAt().UTC()
		if !latestExecutionEvidenceAt.IsZero() && sourceRecord.Timestamp.UTC().Before(latestExecutionEvidenceAt) {
			return []model.ReasonCode{blocker(
				proofReuseLabel(edge.sourceLabel, edge.sourceSkill) +
					" timestamp must be at or after latest execution evidence",
			)}
		}
	}
	if edge.requireExecutionSummaryFreshness {
		diagnostics := state.ExecutionSummaryFreshnessDiagnostics(root, change, summary)
		freshness := state.ProjectExecutionFreshnessForState(model.StateS3Review, diagnostics)
		if freshness != ctxpack.EvidenceFreshnessFresh {
			return []model.ReasonCode{blocker(
				"execution-summary freshness must be fresh, got " + string(freshness),
			)}
		}
	}
	for _, check := range edge.digestChecks {
		label := proofReuseLabel(check.label, check.skillName)
		blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, check.skillName, summary)
		if err != nil {
			return []model.ReasonCode{blocker(label + " digest cannot be evaluated: " + err.Error())}
		}
		if len(blockers) > 0 {
			return []model.ReasonCode{blocker(label + " inputs changed: " + strings.Join(blockers, ","))}
		}
	}
	return nil
}

func proofReuseLabel(label, fallback string) string {
	if trimmed := strings.TrimSpace(label); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fallback)
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

func s3ReviewSetFinalizationBlockers(
	root string,
	change model.Change,
	passingSkills map[string]model.VerificationRecord,
	reviewPassingSkills map[string]model.VerificationRecord,
	selectedReviewSkills []string,
	summary *model.ExecutionSummary,
	required bool,
) []model.ReasonCode {
	closeoutRecord, closeoutOK := passingSkills[SkillFinalCloseout]
	if !closeoutOK || !closeoutRecord.IsPassing() {
		if required {
			return []model.ReasonCode{model.NewReasonCode("required_skill_missing", SkillFinalCloseout)}
		}
		return nil
	}
	cycleRunVersion, err := reviewerDigestCycleRunVersion(root, change, summary)
	if err != nil {
		return []model.ReasonCode{model.NewReasonCode("required_skill_not_ready", SkillFinalCloseout+":suite_result_invalid("+err.Error()+")")}
	}
	if cycleRunVersion < 1 {
		return []model.ReasonCode{model.NewReasonCode("required_skill_not_ready", SkillFinalCloseout+":suite_result_missing")}
	}

	var blockers []model.ReasonCode
	if closeoutRecord.RunVersion != cycleRunVersion {
		blockers = append(blockers, runVersionMismatchBlocker(SkillFinalCloseout, cycleRunVersion, closeoutRecord.RunVersion))
	}

	reviewRecords := mergeContextHandleRecords(reviewPassingSkills, passingSkills)
	for _, skillName := range normalizeReviewSkillNames(selectedReviewSkills) {
		record, ok := reviewRecords[skillName]
		if !ok {
			blockers = append(blockers, model.NewReasonCode("required_skill_missing", skillName))
			continue
		}
		if !record.IsPassing() {
			blockers = append(blockers, model.NewReasonCode(requiredSkillReadinessBlockerCode(record), skillName))
			continue
		}
		if record.RunVersion != cycleRunVersion {
			blockers = append(blockers, runVersionMismatchBlocker(skillName, cycleRunVersion, record.RunVersion))
		}
	}

	reviewDigestBlockers, err := selectedReviewerDigestFreshnessBlockers(root, change, selectedReviewSkills, cycleRunVersion, summary)
	if err != nil {
		return []model.ReasonCode{model.NewReasonCode("required_skill_not_ready", SkillFinalCloseout+":review_digest_unavailable("+err.Error()+")")}
	}
	blockers = append(blockers, reviewDigestBlockers...)
	if digestBlockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillFinalCloseout, summary); err != nil {
		return []model.ReasonCode{model.NewReasonCode("required_skill_not_ready", SkillFinalCloseout+":digest_unavailable("+err.Error()+")")}
	} else {
		blockers = append(blockers, model.ReasonCodesFromSpecs(digestBlockers)...)
	}
	return model.NormalizeReasonCodes(blockers)
}

func selectedReviewerDigestFreshnessBlockers(
	root string,
	change model.Change,
	selectedReviewSkills []string,
	cycleRunVersion int,
	summary *model.ExecutionSummary,
) ([]model.ReasonCode, error) {
	var changedInputs []string
	for _, skillName := range normalizeReviewSkillNames(selectedReviewSkills) {
		changed, err := reviewerDigestFreshAtCycle(root, change, skillName, cycleRunVersion, summary)
		if err != nil {
			return nil, err
		}
		changedInputs = append(changedInputs, changed...)
	}
	changedInputs = stringutil.UniqueSorted(changedInputs)
	if len(changedInputs) == 0 {
		return nil, nil
	}
	var blockers []model.ReasonCode
	// The current reviewer input model has shared suite inputs plus an
	// undifferentiated workspace diff. When one selected reviewer digest changes,
	// ownership cannot prove a narrower reviewer, so stale the full selected set.
	for _, skillName := range normalizeReviewSkillNames(selectedReviewSkills) {
		blockers = append(blockers, model.ReasonCodesFromSpecs(staleSkillDigestBlockers(skillName, changedInputs))...)
	}
	return model.NormalizeReasonCodes(blockers), nil
}

func runVersionMismatchBlocker(skillName string, expected, got int) model.ReasonCode {
	return model.NewReasonCode(
		"required_skill_not_ready",
		fmt.Sprintf("%s:run_version_mismatch(got=%d,want=%d)", skillName, got, expected),
	)
}

// closeoutChainOrderBlockers enforces the folded S3 ordering invariant:
// final-closeout must be strictly last relative to the unordered selected S3
// peer set. Goal-verification is one S3 peer, so there is no structural review
// <= goal-verification edge here. Each pair is compared only when BOTH records
// are present, passing, and carry a non-zero timestamp; a genuinely absent
// record is owned by the required-skill-missing blocker. Advisory (returns nil)
// on light.
func closeoutChainOrderBlockers(
	passingSkills map[string]model.VerificationRecord,
	reviewPassingSkills map[string]model.VerificationRecord,
	selectedReviewSkills []string,
	required bool,
) []model.ReasonCode {
	closeoutRecord, ok := passingSkills[SkillFinalCloseout]
	if !ok || !closeoutRecord.IsPassing() || closeoutRecord.Timestamp.IsZero() {
		return nil
	}
	if !required {
		return nil
	}
	closeoutAt := closeoutRecord.Timestamp.UTC()
	reviewRecords := mergeContextHandleRecords(reviewPassingSkills, passingSkills)
	for _, skillName := range normalizeReviewSkillNames(selectedReviewSkills) {
		reviewRecord, ok := reviewRecords[skillName]
		if !ok || !reviewRecord.IsPassing() || reviewRecord.Timestamp.IsZero() {
			continue
		}
		reviewAt := reviewRecord.Timestamp.UTC()
		if reviewAt.After(closeoutAt) {
			return []model.ReasonCode{closeoutChainOrderInvalidBlocker(
				"final-closeout must be at or after selected reviewer evidence: " + skillName,
			)}
		}
	}
	return nil
}

func closeoutChainOrderInvalidBlocker(detail string) model.ReasonCode {
	return model.NewReasonCode("closeout_chain_order_invalid", appendCloseoutRecovery(detail))
}

// crossStageContextOwnedReviewStagesForSelectedSkills is the set of lattice
// stages the review authority owns: the executor handle set and each selected
// review skill by skill name. S1 plan-audit only authorizes entry into S2; once
// execution has reached S3, current plan/code/evidence alignment is owned by the
// selected review peers rather than the retired audit_origin record.
func crossStageContextOwnedReviewStagesForSelectedSkills(selectedReviewSkills []string) map[string]struct{} {
	stages := map[string]struct{}{
		model.StageContextExecutor: {},
		model.StageContextFix:      {},
	}
	for _, skillName := range normalizeReviewSkillNames(selectedReviewSkills) {
		stages[skillName] = struct{}{}
	}
	return stages
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
			invalid = append(invalid, selectedReviewContextOriginInvalidBlocker(skillName))
			continue
		}
		participants[skillName] = model.ContextParticipant{Handle: handle.Handle}
	}

	if _, want := stages[model.StageContextFix]; want {
		fixHandles := map[string]struct{}{}
		for _, skillName := range reviewParticipantSkillNames(stages) {
			record, ok := passingSkills[skillName]
			if !ok || !record.IsPassing() {
				continue
			}
			handles, ok := model.ContextOriginHandlesFromVerification(record)
			if !ok {
				continue
			}
			handle, ok := handles[model.StageContextFix]
			if !ok || strings.TrimSpace(handle.Handle) == "" {
				continue
			}
			fixHandles[handle.Handle] = struct{}{}
		}
		if len(fixHandles) > 0 {
			participants[model.StageContextFix] = model.ContextParticipant{HandleSet: fixHandles}
		}
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

// crossStageContextReviewStagesForSelectedSkills is the participant set for S3:
// executor and the selected reviewer skill names. The ship gate no longer adds
// goal/closeout stages to this lattice.
func crossStageContextReviewStagesForSelectedSkills(selectedReviewSkills []string) map[string]struct{} {
	return crossStageContextOwnedReviewStagesForSelectedSkills(selectedReviewSkills)
}

func selectedReviewSkillsForAuthority(authority ReviewAuthority) []string {
	if len(authority.SelectedReviewSkills) > 0 {
		return normalizeReviewSkillNames(authority.SelectedReviewSkills)
	}
	return normalizeReviewSkillNames(engineskill.SelectedReviewSkills(engineskill.ReviewSkillSelection{}))
}

func selectedReviewSkillsForSelection(change model.Change, selection engineskill.ReviewSkillSelection) []string {
	return normalizeReviewSkillNames(
		engineskill.SelectedReviewSkillsForWorkflowProfile(selection, change.EffectiveWorkflowProfile()),
	)
}

func normalizeReviewSkillNames(skillNames []string) []string {
	if len(skillNames) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(skillNames))
	for _, skillName := range skillNames {
		trimmed := strings.TrimSpace(skillName)
		if trimmed == "" || !isS3ReviewSetSkill(trimmed) {
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

func isS3ReviewSetSkill(skillName string) bool {
	return engineskill.IsReviewSkill(skillName)
}

func reviewParticipantSkillNames(stages map[string]struct{}) []string {
	if len(stages) == 0 {
		return nil
	}
	names := make([]string, 0, len(stages))
	for stage := range stages {
		if isS3ReviewSetSkill(stage) {
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

// SelectedReviewContextOriginInvalid reports whether the current review
// authority has already accepted a selected review skill as passing but the same
// selected skill is blocked only by its missing or malformed review
// context_origin handle. Callers use this to permit a narrow replacement of the
// invalid record without reopening arbitrary current passing evidence.
func SelectedReviewContextOriginInvalid(root string, change model.Change, skillName string) (bool, error) {
	authority, err := EvaluateReviewAuthority(root, change)
	if err != nil {
		return false, err
	}
	return selectedReviewContextOriginInvalid(authority, skillName), nil
}

func selectedReviewContextOriginInvalid(authority ReviewAuthority, skillName string) bool {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" || !slices.Contains(normalizeReviewSkillNames(authority.SelectedReviewSkills), skillName) {
		return false
	}
	record, ok := authority.PassingSkills[skillName]
	if !ok || !record.IsPassing() {
		return false
	}
	if _, ok := model.ReviewContextOriginHandleFromVerification(record); ok {
		return false
	}
	return selectedReviewContextOriginInvalidBlockerForSkill(authority.Blockers, skillName)
}

func selectedReviewContextOriginInvalidBlockerForSkill(blockers []model.ReasonCode, skillName string) bool {
	prefix := selectedReviewContextOriginInvalidDetail(skillName)
	for _, blocker := range blockers {
		if strings.TrimSpace(blocker.Code) != "context_origin_handle_invalid" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(blocker.Detail), prefix) {
			return true
		}
	}
	return false
}

func selectedReviewContextOriginInvalidBlocker(skillName string) model.ReasonCode {
	return contextOriginHandleInvalidBlocker(selectedReviewContextOriginInvalidDetail(skillName))
}

func selectedReviewContextOriginInvalidDetail(skillName string) string {
	return strings.TrimSpace(skillName) + " (" + model.StageContextReview + ") recorded no context-origin handle for selected reviewer"
}

func crossStageContextNotDistinctBlocker(detail string) model.ReasonCode {
	return model.NewReasonCode(
		"cross_stage_context_not_distinct",
		strings.TrimSpace(detail),
	)
}

func proofReuseContentPaths(change model.Change, summary *model.ExecutionSummary) []string {
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
			if proofReuseSkipsContentPath(change, trimmed) {
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

func proofReuseSkipsContentPath(change model.Change, rel string) bool {
	trimmed := strings.Trim(strings.TrimSpace(filepath.ToSlash(rel)), "/")
	bundleDir := "artifacts/changes/" + strings.TrimSpace(change.Slug)
	eventsDir := bundleDir + "/events"
	verificationDir := bundleDir + "/verification"
	return trimmed == eventsDir ||
		strings.HasPrefix(trimmed, eventsDir+"/") ||
		trimmed == verificationDir ||
		strings.HasPrefix(trimmed, verificationDir+"/")
}

func proofReuseWorkspacePaths(workspaceRoot, rel string) ([]string, bool, error) {
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
	return model.NewReasonCode("closeout_goal_verification_reuse_invalid", appendCloseoutRecovery(detail))
}

func fmtReuseRunMismatch(subject string, expected, got int) string {
	return subject + " run_version mismatch: expected=" + strconv.Itoa(expected) + " got=" + strconv.Itoa(got)
}

func appendCloseoutRecovery(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return closeoutRecoveryRemediation
	}
	if strings.Contains(detail, "goal-verification") && strings.Contains(detail, "final-closeout") {
		return detail
	}
	return detail + "; " + closeoutRecoveryRemediation
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
