package progression

import (
	"path/filepath"
	"strings"

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
		artifactCtx := resolveArtifactEvaluationContext(root, change, policy.EffectivePreset)
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
	verifySkillBlockers := append([]model.ReasonCode(nil), readiness.SkillBlockers...)

	manifestOK, manifestBlockers := ValidateChangeYamlR0(
		filepath.Join(inputs.Paths.GovernedBundleDir, "change.yaml"),
		change.Slug,
	)
	artifactReady := readiness.ArtifactReadiness.Ready
	verificationReady := len(verifySkillBlockers) == 0 &&
		len(reviewAuthority.Blockers) == 0 &&
		ComputeVerificationReadiness(verifyPassingSkills, inputs.Policy.CloseoutRefreshRequired)
	requiredActions := cloneRequiredActions(readiness.RequiredActions)
	requiredActionBlockers := model.ReasonCodesFromSpecs(governance.RequiredActionBlockers(change, requiredActions))
	highRiskChecks := ExtractHighRiskChecks(verifyPassingSkills)

	unresolved := append([]model.ReasonCode{}, readiness.Blockers...)
	unresolved = append(unresolved, model.ReasonCodesFromSpecs(manifestBlockers)...)
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

func EvaluateReviewLayerBlockers(
	change model.Change,
	artifactReviewEvidence model.VerificationRecord,
	projection *ArtifactProjection,
	reviewAll bool,
) []model.ReasonCode {
	return EvaluateReviewLayerBlockersFromNamedEvidence(change, artifactReviewEvidence, model.VerificationRecord{}, projection, reviewAll)
}

func EvaluateReviewLayerBlockersFromEvidence(
	change model.Change,
	reviewEvidence []model.VerificationRecord,
	projection *ArtifactProjection,
	reviewAll bool,
) []model.ReasonCode {
	if len(reviewEvidence) == 0 || reviewEvidence[0].Verdict == "" {
		return model.ReasonCodesFromSpecs([]string{"required_skill_missing:spec-compliance-review"})
	}
	artifactReviewEvidence := reviewEvidence[0]
	implementationReviewEvidence := model.VerificationRecord{}
	if len(reviewEvidence) > 1 {
		implementationReviewEvidence = reviewEvidence[1]
	}
	return EvaluateReviewLayerBlockersFromNamedEvidence(change, artifactReviewEvidence, implementationReviewEvidence, projection, reviewAll)
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
	case "change", "status", "change.yaml":
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
