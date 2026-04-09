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

// buildSkillConstraints populates per-skill constraint fields from the Go registry
// and artifact state, replacing level-conditional logic in skill templates.
// governedChange is the already-loaded unified Change (nil for intake-only mode).
func buildSkillConstraints(root, skillName string, governedChange *model.Change) *skillConstraints {
	def, ok := skill.LookupDefinition(skillName)
	if !ok {
		return nil
	}

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
// and hard-gate flag for a given skill in the current runtime context.
func deriveAgentConstraints(skillName string) *agentConstraints {
	c := &agentConstraints{
		MaxRetries: defaultMaxRetriesPerSkill,
	}
	switch skillName {
	case progression.SkillResearchOrchestration:
		c.AllowedOperations = []string{"read_codebase", "read_artifacts", "write_evidence"}
		c.RequiredOutputs = []string{"evidence_record"}
	case progression.SkillIntakeClarification:
		c.AllowedOperations = []string{"read_codebase", "read_artifacts", "write_evidence"}
		c.RequiredOutputs = []string{"evidence_record"}
	case progression.SkillPlanAudit:
		c.AllowedOperations = []string{"read_artifacts", "write_evidence"}
		c.RequiredOutputs = []string{"evidence_record"}
		c.HardGate = string(gate.GatePlan)
	case progression.SkillWaveOrchestration:
		c.AllowedOperations = []string{"read_codebase", "read_artifacts", "write_code", "run_tests", "write_evidence", "git_commit"}
		c.RequiredOutputs = []string{"evidence_record", "task_results", "changed_files"}
	case progression.SkillSpecComplianceReview, progression.SkillCodeQualityReview:
		c.AllowedOperations = []string{"read_codebase", "read_artifacts", "write_evidence"}
		c.RequiredOutputs = []string{"evidence_record", "review_findings"}
	case progression.SkillTDDGovernance:
		c.AllowedOperations = []string{"read_codebase", "read_artifacts", "write_evidence"}
		c.RequiredOutputs = []string{"evidence_record"}
	case progression.SkillGoalVerification:
		c.AllowedOperations = []string{"read_codebase", "read_artifacts", "run_tests", "write_evidence"}
		c.RequiredOutputs = []string{"evidence_record"}
		c.HardGate = string(gate.GateShip)
	case progression.SkillFinalCloseout:
		c.AllowedOperations = []string{"read_codebase", "read_artifacts", "run_tests", "write_evidence"}
		c.RequiredOutputs = []string{"evidence_record"}
	default:
		c.AllowedOperations = []string{"read_codebase", "write_evidence"}
		c.RequiredOutputs = []string{"evidence_record"}
	}
	return c
}
