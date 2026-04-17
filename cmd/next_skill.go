package cmd

import (
	"os"
	"path/filepath"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/review"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// buildSkillConstraints populates per-skill constraint fields from the loaded
// governance registry and artifact state, replacing level-conditional logic in
// skill templates.
// governedChange is the already-loaded unified Change (nil for intake-only mode).
func buildSkillConstraints(root string, def skill.Definition, governedChange *model.Change) *skillConstraints {
	sc := &skillConstraints{
		MitigationTarget: def.Mitigation,
		RunSummaryBound:  def.RunSummaryBound,
	}

	if governedChange != nil {
		sc.GuardrailDomain = governedChange.GuardrailDomain

		// Parse locked decisions from decision.md (canonical source for selected approach).
		if paths, err := state.ResolveChangePaths(root, *governedChange); err == nil {
			sc.LockedDecisions = parseLockedDecisions(filepath.Join(paths.GovernedBundleDir, "decision.md"))
		}
	}

	return sc
}

// parseLockedDecisions extracts locked decision items from decision.md.
// Returns nil if the file is absent or only contains scaffolded draft text.
func parseLockedDecisions(decisionPath string) []string {
	data, err := os.ReadFile(decisionPath)
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

func buildReviewContext(guardrailDomain string) *reviewContextView {
	artLayers := review.RequiredArtifactLayers(guardrailDomain, "")
	implLayers := review.RequiredImplementationLayers(guardrailDomain)
	optLayers := review.OptionalLayers()

	toStrings := func(layers []review.ReviewLayer) []string {
		out := make([]string, 0, len(layers))
		for _, l := range layers {
			out = append(out, string(l))
		}
		return out
	}

	return &reviewContextView{
		RequiredArtifactLayers:       toStrings(artLayers),
		RequiredImplementationLayers: toStrings(implLayers),
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
