package progression

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ctxpack "github.com/signalridge/slipway/internal/engine/context"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/governance"
	reviewengine "github.com/signalridge/slipway/internal/engine/review"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

type runtimeGovernanceInputs struct {
	Policy governance.PresetPolicy
	Paths  state.ResolvedChangePaths
}

type ReviewAuthority struct {
	Policy              governance.PresetPolicy
	ExecutionSummaryCtx state.RelevantExecutionSummaryContext
	PassingSkills       map[string]model.VerificationRecord
	SkillBlockers       []model.ReasonCode
	LayerBlockers       []model.ReasonCode
	Blockers            []model.ReasonCode
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
	passingSkills, skillBlockers, err := EvaluateRequiredSkillsForChange(
		root,
		change,
		model.StateS3Review,
		executionSummaryCtx.LatestRunVersion,
		policy.CloseoutRefreshRequired,
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
	blockers = model.NormalizeReasonCodes(blockers)

	return ReviewAuthority{
		Policy:              policy,
		ExecutionSummaryCtx: executionSummaryCtx,
		PassingSkills:       passingSkills,
		SkillBlockers:       model.ReasonCodesFromSpecs(skillBlockers),
		LayerBlockers:       model.NormalizeReasonCodes(layerBlockers),
		Blockers:            blockers,
	}, nil
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
	verifySkillBlockers := append([]model.ReasonCode(nil), readiness.SkillBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, attestationBlockers...)
	verifySkillBlockers = append(verifySkillBlockers, reuseBlockers...)

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
	if blocker := closeoutGoalVerificationReuseReviewBlocker(reviewPassingSkills, goalRecord.Timestamp.UTC()); blocker != nil {
		return []model.ReasonCode{*blocker}
	}
	if !goalRecord.Timestamp.IsZero() && closeoutRecord.Timestamp.UTC().Before(goalRecord.Timestamp.UTC()) {
		return []model.ReasonCode{closeoutGoalVerificationReuseInvalidBlocker(
			"final-closeout timestamp must not predate reused goal-verification",
		)}
	}
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

func closeoutGoalVerificationReuseReviewBlocker(
	reviewPassingSkills map[string]model.VerificationRecord,
	goalTimestamp time.Time,
) *model.ReasonCode {
	if goalTimestamp.IsZero() {
		blocker := closeoutGoalVerificationReuseInvalidBlocker("goal-verification timestamp is required for final-closeout reuse")
		return &blocker
	}
	for _, skillName := range []string{SkillSpecComplianceReview, SkillCodeQualityReview} {
		record, ok := reviewPassingSkills[skillName]
		if !ok || !record.IsPassing() || record.Timestamp.IsZero() {
			continue
		}
		if record.Timestamp.UTC().After(goalTimestamp) {
			blocker := closeoutGoalVerificationReuseInvalidBlocker(
				"goal-verification timestamp must be at or after latest review evidence: " + skillName,
			)
			return &blocker
		}
	}
	return nil
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
