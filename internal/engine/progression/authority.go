package progression

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

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

// FinalCloseoutEvidenceRequired returns whether the closeout-conditional ship
// governance evidence is required for the resolved policy. The merged
// ship-verification gate is always required at S3 regardless of this value; the
// helper is retained for the registry's CloseoutConditional filtering and for
// caller-signature stability.
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
	shipPassing, shipSkillBlockers, err := loadFreshPassingRecordsForSkills(
		root,
		change,
		[]string{SkillShipVerification},
		readiness.ExecutionSummary,
		verifyPassingSkills,
	)
	if err != nil {
		return ShipAuthority{}, err
	}
	for skillName, record := range shipPassing {
		verifyPassingSkills[skillName] = record
	}
	// Assurance attestation is required on every standard/strict effective preset,
	// matching the Layer 2 floor (AssuranceContractBlockers and the done gate) —
	// NOT CloseoutRefreshRequired. The latter also trips for light +
	// quality_mode=full, where assurance.md is optional and the ship-verification
	// template instructs light to omit the reference; gating on it would block a
	// valid light/full ship verification and, conversely, miss a plain standard one
	// (CloseoutRefreshRequired is false there). EffectivePreset keeps both layers
	// consistent. The attestation now lives on the single ship-verification record.
	assuranceRequired := inputs.Policy.EffectivePreset != model.WorkflowPresetLight
	attestationBlockers := shipAssuranceAttestationBlockers(verifyPassingSkills, assuranceRequired)
	// Independence facets gated on the effective preset, matching the assurance
	// attestation floor — required on standard/strict, advisory (omitted) on light.
	independenceRequired := inputs.Policy.EffectivePreset != model.WorkflowPresetLight
	independencePresenceBlockers := shipReviewerIndependenceBlockers(verifyPassingSkills, independenceRequired)
	// The single retained ordering invariant: ship-verification must be timestamped
	// at or after every selected review peer (spec/code/independent/security). The
	// retired goal↔closeout chain-order and proof-reuse edges are gone. This is a
	// causal-validity invariant — did the terminal gate observe the FINAL review
	// evidence rather than precede it? — not a quality attestation, so it is
	// enforced on every preset with no light carveout, unlike the assurance and
	// reviewer-independence attestation facets above.
	orderingBlockers := shipReviewSetOrderingBlockers(
		verifyPassingSkills,
		reviewAuthority.PassingSkills,
		selectedReviewSkills,
	)
	verifySkillBlockers := append([]model.ReasonCode(nil), readiness.SkillBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, shipSkillBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, attestationBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, independencePresenceBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, orderingBlockers...)

	manifestOK, manifestBlockers := ValidateChangeYamlR0(
		filepath.Join(inputs.Paths.GovernedBundleDir, "change.yaml"),
		change.Slug,
	)
	artifactReady := readiness.ArtifactReadiness.Ready
	verificationReady := len(verifySkillBlockers) == 0 &&
		len(reviewAuthority.Blockers) == 0 &&
		ComputeVerificationReadiness(verifyPassingSkills, FinalCloseoutEvidenceRequired(inputs.Policy))
	requiredActions := cloneRequiredActions(readiness.RequiredActions)
	// G_ship's guardrail high-risk satisfaction is owned SOLELY by the
	// ship-verification record (REQ-005). Extracting over the whole passing-skill
	// map would let any selected review peer's record satisfy the SAST safety
	// baseline, contradicting the ship-verification template's "no other
	// satisfy-path" contract and diluting the gate's ownership. Scope to ship only.
	highRiskChecks := extractShipVerificationHighRiskChecks(verifyPassingSkills)

	unresolved := append([]model.ReasonCode{}, readiness.Blockers...)
	unresolved = append(unresolved, model.ReasonCodesFromSpecs(manifestBlockers)...)
	unresolved = append(unresolved, shipSkillBlockers...)
	// Surface the attestation blocker as an actionable G_ship reason — the same
	// channel the Layer 2 placeholder blocker travels (readiness.Blockers). Left
	// only in verifySkillBlockers it would flip verificationReady but EvaluateGShip
	// would emit only the generic ship_verification_evidence_missing, hiding the
	// specific, actionable ship_verification_assurance_attestation_missing code.
	unresolved = append(unresolved, attestationBlockers...)
	unresolved = append(unresolved, independencePresenceBlockers...)
	unresolved = append(unresolved, orderingBlockers...)
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

// shipAssuranceCompleteReference is the AI-driven ship attestation: the host's
// structured judgment, recorded in the ship-verification record, that every
// required assurance section is genuinely authored rather than scaffold.
const shipAssuranceCompleteReference = "closeout:assurance_complete=pass"

// shipAssuranceAttestationBlockers enforces the re-homed assurance attestation.
// When assurance is required for the change's effective preset (standard/strict),
// the passing ship-verification record must carry the assurance-complete
// attestation. The kernel does not re-read assurance prose here; it only requires
// the AI's attestation to be present, so ship-verification cannot reach the
// governed ship decision without the host explicitly vouching for the assurance
// content. A missing standard/strict ship-verification record is the same
// attestation failure as a passing-but-unattested one. Light preset (assurance
// optional) is unaffected.
func shipAssuranceAttestationBlockers(passingSkills map[string]model.VerificationRecord, assuranceRequired bool) []model.ReasonCode {
	if !assuranceRequired {
		return nil
	}
	record, ok := passingSkills[SkillShipVerification]
	if !ok {
		return []model.ReasonCode{shipAssuranceAttestationMissingBlocker()}
	}
	for _, ref := range record.References {
		if strings.TrimSpace(ref) == shipAssuranceCompleteReference {
			return nil
		}
	}
	return []model.ReasonCode{shipAssuranceAttestationMissingBlocker()}
}

func shipAssuranceAttestationMissingBlocker() model.ReasonCode {
	return model.NewReasonCode(
		"ship_verification_assurance_attestation_missing",
		"ship-verification must record "+shipAssuranceCompleteReference+" on standard/strict",
	)
}

// shipReviewerIndependenceReference is the engine-consumed presence facet of the
// independence contract: the ship-verification record's structured attestation
// that the ship judgment was produced by an independent reviewer context. The
// kernel only requires the token to be present; it does not re-derive
// independence here.
const shipReviewerIndependenceReference = "closeout:reviewer_independence=pass"

// shipReviewerIndependenceBlockers enforces the independence presence facet. When
// required (standard/strict effective preset), the passing ship-verification
// record must carry the reviewer-independence attestation; a missing record or a
// record lacking the token is a fail-closed blocker. Advisory (returns nil) on
// light.
func shipReviewerIndependenceBlockers(passingSkills map[string]model.VerificationRecord, required bool) []model.ReasonCode {
	if !required {
		return nil
	}
	record, ok := passingSkills[SkillShipVerification]
	if !ok {
		return []model.ReasonCode{shipReviewerIndependenceMissingBlocker()}
	}
	for _, ref := range record.References {
		if strings.TrimSpace(ref) == shipReviewerIndependenceReference {
			return nil
		}
	}
	return []model.ReasonCode{shipReviewerIndependenceMissingBlocker()}
}

func shipReviewerIndependenceMissingBlocker() model.ReasonCode {
	return model.NewReasonCode(
		"ship_verification_reviewer_independence_missing",
		"ship-verification must record "+shipReviewerIndependenceReference+" on standard/strict; rerun ship-verification",
	)
}

// shipReviewSetOrderingBlockers enforces the single retained S3 ordering
// invariant: ship-verification must be timestamped at or after every selected
// review peer (spec/code/independent/security). ship-verification is the terminal
// gate, so it must observe the final review evidence rather than precede it. Each
// peer is compared only when BOTH the ship record and that peer's record are
// present, passing, and carry a non-zero timestamp; a genuinely absent record is
// owned by the required-skill-missing blocker. Enforced on every preset: a ship
// verdict that structurally predates a selected peer never observed that peer's
// final evidence, which is fail-open regardless of blast radius, so unlike the
// assurance/independence attestation facets there is no light advisory carveout.
func shipReviewSetOrderingBlockers(
	passingSkills map[string]model.VerificationRecord,
	reviewPassingSkills map[string]model.VerificationRecord,
	selectedReviewSkills []string,
) []model.ReasonCode {
	shipRecord, ok := passingSkills[SkillShipVerification]
	if !ok || !shipRecord.IsPassing() || shipRecord.Timestamp.IsZero() {
		return nil
	}
	shipAt := shipRecord.Timestamp.UTC()
	reviewRecords := mergeContextHandleRecords(reviewPassingSkills, passingSkills)
	for _, skillName := range normalizeReviewSkillNames(selectedReviewSkills) {
		reviewRecord, ok := reviewRecords[skillName]
		if !ok || !reviewRecord.IsPassing() || reviewRecord.Timestamp.IsZero() {
			continue
		}
		if reviewRecord.Timestamp.UTC().After(shipAt) {
			return []model.ReasonCode{shipReviewSetOrderingInvalidBlocker(
				"ship-verification must be at or after selected reviewer evidence: " + skillName,
			)}
		}
	}
	return nil
}

func shipReviewSetOrderingInvalidBlocker(detail string) model.ReasonCode {
	return model.NewReasonCode("ship_verification_ordering_invalid", appendShipRecovery(detail))
}

// extractShipVerificationHighRiskChecks returns the guardrail high-risk check
// results that satisfy G_ship, read ONLY from the ship-verification record. The
// terminal gate owns the guardrail SAST safety baseline (REQ-005): a review
// peer's record carrying a high_risk_check reference must NOT satisfy the gate,
// so a sensitive-domain change cannot reach the ship decision unless
// ship-verification itself recorded the passing baseline. A missing ship record
// yields no checks, leaving G_ship blocked with high_risk_check_missing.
func extractShipVerificationHighRiskChecks(passingSkills map[string]model.VerificationRecord) map[string]bool {
	record, ok := passingSkills[SkillShipVerification]
	if !ok {
		return map[string]bool{}
	}
	return ExtractHighRiskChecks(map[string]model.VerificationRecord{SkillShipVerification: record})
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
// single-handle stages plus the executor handle set and each selected review
// peer handle. Only present-and-passing records contribute a participant; an absent or
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

	// fix <- every selected reviewer's recorded context_origin:stage=fix handles,
	// unioned into one set. The fix stage is multi-valued — a reviewer's record
	// accumulates one handle per fresh-context repair subagent / batch — so the
	// model exposes it as a never-fail-closed set rather than the single-valued
	// handles map. Reading it through FixContextOriginHandleSetFromVerification
	// (rather than the single-valued map, which no longer stores fix) collects
	// every fix handle across all selected reviewers without poisoning a reviewer's
	// own review-handle parse.
	if _, want := stages[model.StageContextFix]; want {
		fixHandles := map[string]struct{}{}
		for _, skillName := range reviewParticipantSkillNames(stages) {
			record, ok := passingSkills[skillName]
			if !ok || !record.IsPassing() {
				continue
			}
			for handle := range model.FixContextOriginHandleSetFromVerification(record) {
				fixHandles[handle] = struct{}{}
			}
		}
		if len(fixHandles) > 0 {
			participants[model.StageContextFix] = model.ContextParticipant{HandleSet: fixHandles}
		}
	}

	return participants, model.NormalizeReasonCodes(invalid)
}

// mergeContextHandleRecords overlays the terminal ship-verification passing
// record onto the review-stage passing records so the ship lattice can resolve
// every single-handle stage from one map. The review records are the
// selected-reviewer source the review gate already used; the ship-verification
// record wins on key collision since it is the ship gate's own surface.
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

// shipRecoveryRemediation is the recovery suffix for the ship ordering invariant:
// the only fix is to re-run the selected reviewer set and then re-run
// ship-verification so its timestamp observes the final review evidence.
const shipRecoveryRemediation = "rerun the selected reviewer set, then rerun ship-verification"

func appendShipRecovery(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return shipRecoveryRemediation
	}
	if strings.Contains(detail, "ship-verification") {
		return detail
	}
	return detail + "; " + shipRecoveryRemediation
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
