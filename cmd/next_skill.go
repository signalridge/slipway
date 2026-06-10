package cmd

import (
	"os"
	"path/filepath"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/engine/review"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// buildSkillConstraints populates per-skill constraint fields from the loaded
// governance registry and artifact state, replacing level-conditional logic in
// skill templates.
// governedChange is the already-loaded unified Change (nil for intake-only mode).
func buildSkillConstraints(root string, def skill.Definition, governedChange *model.Change, planLocked bool) *skillConstraints {
	sc := &skillConstraints{
		MitigationTarget: def.Mitigation,
		RunSummaryBound:  def.RunSummaryBound,
	}

	if governedChange != nil {
		sc.GuardrailDomain = governedChange.GuardrailDomain

		// Parse the selected approach/direction from decision.md and route it by
		// lifecycle lock state: a decision is "locked" only once the G_plan gate
		// has approved the plan. While the plan is not yet locked, a recommended
		// Selected Approach is still pending fresh user confirmation and must be
		// surfaced as pending, never as a locked decision (issue #140).
		if paths, err := state.ResolveChangePaths(root, *governedChange); err == nil {
			decisions := parseDecisionItems(filepath.Join(paths.GovernedBundleDir, "decision.md"))
			if planLocked {
				sc.LockedDecisions = decisions
			} else {
				sc.PendingDecisions = decisions
			}
		}

		// Surface the exact high-risk reference tokens goal-verification must
		// record so a guardrail-domain change is never a dead-end (issue #88).
		if def.Name == progression.SkillGoalVerification && governedChange.GuardrailDomain != "" {
			sc.RequiredHighRiskTokens = requiredHighRiskTokenHints(governedChange.GuardrailDomain)
		}
	}

	return sc
}

// requiredHighRiskTokenHints returns the recordable goal-verification reference
// tokens (one per required high-risk check) for a guardrail domain, e.g.
// "high_risk_check:external_api_contracts.safety_baseline=pass".
func requiredHighRiskTokenHints(domain string) []string {
	checks := gate.RequiredHighRiskChecks(domain)
	if len(checks) == 0 {
		return nil
	}
	out := make([]string, 0, len(checks))
	for _, checkID := range checks {
		out = append(out, "high_risk_check:"+checkID+"=pass")
	}
	return out
}

// planLockedFromGates reports whether the lifecycle has locked the plan — the
// G_plan gate is approved. Decisions parsed from decision.md are only "locked"
// once this is true; before plan approval a recommended Selected Approach is
// still pending fresh confirmation and is surfaced as pending (issue #140).
func planLockedFromGates(readiness progression.GovernanceReadiness) bool {
	eval, ok := readiness.GateEvaluations[gate.GatePlan]
	return ok && eval.Status == model.GateStatusApproved
}

// parseDecisionItems extracts the selected approach/direction items from
// decision.md. Returns nil if the file is absent or only contains scaffolded
// draft text. Whether these items are reported as locked or pending is decided
// by the caller from the lifecycle G_plan gate state (issue #140).
func parseDecisionItems(decisionPath string) []string {
	data, err := os.ReadFile(decisionPath) // #nosec G304 -- path is resolved from CLI/project authority before this read.
	if err != nil {
		return nil
	}
	content := string(data)

	// decision.md uses "Selected Approach" as its canonical section
	decisions := artifact.ParseDecisionLockedDecisions(content)
	if len(decisions) == 0 {
		return nil
	}
	return decisions
}

func buildReviewContext(change *model.Change, projection *progression.ArtifactProjection, reviewAll bool, skillName string) *reviewContextView {
	optLayers := review.OptionalLayers()

	toStrings := func(layers []review.ReviewLayer) []string {
		out := make([]string, 0, len(layers))
		for _, l := range layers {
			out = append(out, string(l))
		}
		return out
	}

	var artLayers []string
	var implLayers []string
	if change != nil {
		requiredLayers := progression.RequiredReviewLayerNamesForSkill(*change, projection, reviewAll, skillName)
		switch skillName {
		case progression.SkillSpecComplianceReview:
			artLayers = requiredLayers
		case progression.SkillCodeQualityReview:
			implLayers = requiredLayers
		}
	}

	return &reviewContextView{
		RequiredArtifactLayers:       artLayers,
		RequiredImplementationLayers: implLayers,
		OptionalLayers:               toStrings(optLayers),
	}
}

// deriveAgentConstraints returns the allowed operations, required outputs,
// and hard-gate flag for a given skill, sourced from the skill registry.
func deriveAgentConstraints(registry []skill.Definition, skillName string) *agentConstraints {
	c := &agentConstraints{
		MaxRetries: defaultMaxRetriesPerSkill,
	}
	if def, ok := skill.LookupDefinitionInRegistry(registry, skillName); ok {
		c.AllowedOperations = def.AllowedOperations
		c.RequiredOutputs = def.RequiredOutputs
		c.HardGate = def.HardGate
	}
	if len(c.AllowedOperations) == 0 {
		c.AllowedOperations = []string{"read_codebase", "write_evidence"}
	}
	if len(c.RequiredOutputs) == 0 {
		c.RequiredOutputs = []string{"evidence_record"}
	}
	return c
}
