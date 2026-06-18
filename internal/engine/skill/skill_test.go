package skill

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/toolgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requiredSkillsForState(needsDiscovery bool, state model.WorkflowState, closeoutRequired bool) []string {
	return RequiredSkillsForStateWithRegistry(
		definitionsToSortedSlice(defaultGovernanceRegistry),
		needsDiscovery,
		state,
		closeoutRequired,
	)
}

func TestGovernanceRegistryCompleteness(t *testing.T) {
	t.Parallel()
	registry := definitionsToSortedSlice(defaultGovernanceRegistry)
	require.Len(t, registry, 10)
}

func TestRequiredSkillsByNeedsDiscoveryAndState(t *testing.T) {
	t.Parallel()
	// Non-discovery at S2_IMPLEMENT returns wave-orchestration
	assert.Equal(
		t,
		[]string{"wave-orchestration"},
		requiredSkillsForState(false, model.StateS2Implement, false),
	)
	// Non-discovery at S1_PLAN returns plan-audit only (discovery skills are discovery-only)
	assert.Equal(
		t,
		[]string{"plan-audit"},
		requiredSkillsForState(false, model.StateS1Plan, false),
	)
	// Discovery at S1_PLAN returns all planning skills
	assert.Equal(
		t,
		[]string{"plan-audit", "research-orchestration"},
		requiredSkillsForState(true, model.StateS1Plan, false),
	)
	// Non-discovery at S3_REVIEW returns the selected review gate skills.
	assert.Equal(
		t,
		[]string{"code-quality-review", "goal-verification", "independent-review", "spec-compliance-review"},
		requiredSkillsForState(false, model.StateS3Review, false),
	)
	// Final closeout is folded into S3_REVIEW when closeout evidence is required.
	assert.Equal(
		t,
		[]string{"code-quality-review", "final-closeout", "goal-verification", "independent-review", "spec-compliance-review"},
		requiredSkillsForState(true, model.StateS3Review, true),
	)
}

func TestRequiredSkillsForStateWithRegistry_S3SecuritySelection(t *testing.T) {
	t.Parallel()

	registry := definitionsToSortedSlice(defaultGovernanceRegistry)
	required := RequiredSkillsForStateWithRegistryWithReviewSelection(
		registry,
		false,
		model.StateS3Review,
		false,
		ReviewSkillSelection{SecurityReviewSelected: true},
	)

	assert.Equal(
		t,
		[]string{"code-quality-review", "goal-verification", "independent-review", "security-review", "spec-compliance-review"},
		required,
	)
}

func TestSelectedReviewSkillsForWorkflowProfileFiltersCodeQualityOnly(t *testing.T) {
	t.Parallel()

	selection := ReviewSkillSelection{SecurityReviewSelected: true}

	assert.Equal(
		t,
		[]string{"spec-compliance-review", "independent-review", "goal-verification", "security-review"},
		SelectedReviewSkillsForWorkflowProfile(selection, model.WorkflowProfileDocs),
	)
	assert.Equal(
		t,
		[]string{"spec-compliance-review", "code-quality-review", "independent-review", "goal-verification", "security-review"},
		SelectedReviewSkillsForWorkflowProfile(selection, model.WorkflowProfileCode),
	)
}

func TestFilterRequiredSkillsForWorkflowProfileWithReviewSelection_ProfilesKeepIndependentReview(t *testing.T) {
	t.Parallel()

	required := []string{
		"code-quality-review",
		"independent-review",
		"security-review",
		"spec-compliance-review",
	}

	for _, profile := range []model.WorkflowProfile{model.WorkflowProfileDocs, model.WorkflowProfileResearch} {
		got := FilterRequiredSkillsForWorkflowProfileWithReviewSelection(
			required,
			profile,
			ReviewSkillSelection{},
		)
		assert.Equal(t, []string{"independent-review", "spec-compliance-review"}, got)
	}
}

func TestIntakeClarificationRequiredAtS0Intake(t *testing.T) {
	t.Parallel()
	assert.Equal(
		t,
		[]string{"intake-clarification"},
		requiredSkillsForState(false, model.StateS0Intake, false),
	)
	assert.Equal(
		t,
		[]string{"intake-clarification"},
		requiredSkillsForState(true, model.StateS0Intake, false),
	)
}

func TestResearchOrchestrationRequiredForDiscoveryAtS1Plan(t *testing.T) {
	t.Parallel()
	required := requiredSkillsForState(true, model.StateS1Plan, false)
	assert.Equal(t, []string{"plan-audit", "research-orchestration"}, required)
	// Non-discovery only returns plan-audit (research/scope are discovery-only)
	assert.Equal(t, []string{"plan-audit"}, requiredSkillsForState(false, model.StateS1Plan, false))
}

func TestLoadGovernanceRegistryFromGeneratedSkills(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

	registry, err := LoadGovernanceRegistry(root)
	require.NoError(t, err)
	require.Len(t, registry, 10)

	defByName := map[string]Definition{}
	for _, def := range registry {
		defByName[def.Name] = def
	}

	planAudit := defByName["plan-audit"]
	assert.Equal(t, model.StateS1Plan, planAudit.State)
	assert.Equal(t, model.PlanSubStepAudit, planAudit.PlanSubStep)
	assert.Equal(t, "stale or incomplete plan bundle", planAudit.Mitigation)
	assert.False(t, planAudit.RunSummaryBound)
	assert.False(t, planAudit.DiscoveryOnly)

	research := defByName["research-orchestration"]
	assert.Equal(t, model.StateS1Plan, research.State)
	assert.Equal(t, model.PlanSubStepResearch, research.PlanSubStep)
	assert.True(t, research.DiscoveryOnly)

	_, ok := defByName["workflow"]
	assert.False(t, ok, "standalone workflow export must not enter governance registry")
}

func TestLoadGovernanceRegistryWithoutGeneratedSkillsUsesDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	registry, err := LoadGovernanceRegistry(root)
	require.NoError(t, err)
	require.Len(t, registry, 10)

	def, ok := LookupDefinitionInRegistry(registry, "wave-orchestration")
	require.True(t, ok)
	assert.Equal(t, model.StateS2Implement, def.State)
}

func TestLoadGovernanceRegistrySkipsUnknownSkillID(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "slipway-custom")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`
---
skill_id: custom
name: slipway-custom
description: "unknown skill"
---
body
`), 0o644))

	// Unknown skills are silently skipped (not errors).
	registry, err := LoadGovernanceRegistry(root)
	require.NoError(t, err)
	require.Len(t, registry, 10)
}

func TestLoadGovernanceRegistryRejectsMissingFrontmatterForKnownGeneratedSkill(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "slipway-wave-orchestration")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("body\n"), 0o644))

	_, err := LoadGovernanceRegistry(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing frontmatter")
	assert.Contains(t, err.Error(), "wave-orchestration")
}

func TestLoadGovernanceRegistryMinimalFrontmatter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

	registry, err := LoadGovernanceRegistry(root)
	require.NoError(t, err)
	require.Len(t, registry, 10)

	// Values come from Go registry defaults, not frontmatter overrides.
	defByName := map[string]Definition{}
	for _, def := range registry {
		defByName[def.Name] = def
	}
	defaults := defaultGovernanceRegistryMap()
	for name, def := range defByName {
		expected := defaults[name]
		assert.Equal(t, expected.State, def.State, "state mismatch for %s", name)
		assert.Equal(t, expected.Mitigation, def.Mitigation, "mitigation mismatch for %s", name)
		assert.Equal(t, expected.RunSummaryBound, def.RunSummaryBound, "run_summary_bound mismatch for %s", name)
		assert.Equal(t, expected.DiscoveryOnly, def.DiscoveryOnly, "discovery_only mismatch for %s", name)
	}
}

func TestLoadGovernanceRegistryIgnoresGeneratedRoutingMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "slipway-wave-orchestration")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`
---
skill_id: wave-orchestration
name: slipway-wave-orchestration
description: "tampered"
state: S9_DONE
hard_gate: fake
---
body
`), 0o644))

	registry, err := LoadGovernanceRegistry(root)
	require.NoError(t, err)

	def, ok := LookupDefinitionInRegistry(registry, "wave-orchestration")
	require.True(t, ok)
	assert.Equal(t, model.StateS2Implement, def.State)
	assert.Equal(t, "uncontrolled parallel execution drift", def.Mitigation)
}

func TestLoadGovernanceRegistryRejectsMissingSkillID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "slipway-wave-orchestration")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`
---
name: slipway-wave-orchestration
description: "tampered"
---
body
`), 0o644))

	_, err := LoadGovernanceRegistry(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing skill_id")
}

func TestLoadGovernanceRegistryRejectsMissingSkillIDWithLegacyBareName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "slipway-wave-orchestration")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`
---
name: wave-orchestration
description: "tampered"
---
body
`), 0o644))

	_, err := LoadGovernanceRegistry(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing skill_id")
	assert.Contains(t, err.Error(), "wave-orchestration")
}

func TestLoadGovernanceRegistryRejectsPublicNameSkillIDDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "slipway-wave-orchestration")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`
---
skill_id: wave-orchestration
name: slipway-plan-audit
description: "tampered"
---
body
`), 0o644))

	_, err := LoadGovernanceRegistry(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slipway-wave-orchestration")
}

func TestLoadGovernanceRegistryDoesNotReadRemovedAgentsConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

	require.NoError(t, os.WriteFile(filepath.Join(root, ".slipway.yaml"), []byte(`
agents:
  mappings:
    wave-orchestration: slipway-reviewer
`), 0o644))

	registry, err := LoadGovernanceRegistry(root)
	require.NoError(t, err)

	def, ok := LookupDefinitionInRegistry(registry, "wave-orchestration")
	require.True(t, ok)
	assert.Equal(t, "uncontrolled parallel execution drift", def.Mitigation)
}
