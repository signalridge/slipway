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
		GovernanceRegistry(),
		needsDiscovery,
		state,
		closeoutRequired,
		"",
	)
}

func requiredSkillsForStateWithGuardrail(
	needsDiscovery bool,
	state model.WorkflowState,
	closeoutRequired bool,
	guardrailDomain string,
) []string {
	return RequiredSkillsForStateWithRegistry(
		GovernanceRegistry(),
		needsDiscovery,
		state,
		closeoutRequired,
		guardrailDomain,
	)
}

func TestGovernanceRegistryCompleteness(t *testing.T) {
	t.Parallel()
	registry := GovernanceRegistry()
	require.Len(t, registry, 9)
}

func TestRequiredSkillsByNeedsDiscoveryAndState(t *testing.T) {
	t.Parallel()
	// Non-discovery at S2_EXECUTE returns wave-orchestration
	assert.Equal(
		t,
		[]string{"wave-orchestration"},
		requiredSkillsForState(false, model.StateS2Execute, false),
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
	// Non-discovery at S3_REVIEW returns review skills
	assert.Equal(
		t,
		[]string{"code-quality-review", "spec-compliance-review"},
		requiredSkillsForState(false, model.StateS3Review, false),
	)
	// Discovery at S4_VERIFY with closeout returns both
	assert.Equal(
		t,
		[]string{"final-closeout", "goal-verification"},
		requiredSkillsForState(true, model.StateS4Verify, true),
	)
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

func TestTDDGovernanceRequiredOnlyForGuardrailDomain(t *testing.T) {
	t.Parallel()
	assert.Equal(
		t,
		[]string{"wave-orchestration"},
		requiredSkillsForStateWithGuardrail(false, model.StateS2Execute, false, ""),
	)
	assert.Equal(
		t,
		[]string{"tdd-governance", "wave-orchestration"},
		requiredSkillsForStateWithGuardrail(false, model.StateS2Execute, false, "auth_authz"),
	)
}

func TestLoadGovernanceRegistryFromGeneratedSkills(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

	registry, err := LoadGovernanceRegistry(root)
	require.NoError(t, err)
	require.Len(t, registry, 9)

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
}

func TestLoadGovernanceRegistrySkipsUnknownSkillName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "slipway", "custom")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`
---
name: custom-skill
description: "unknown skill"
---
body
`), 0o644))

	// Unknown skills are silently skipped (not errors).
	registry, err := LoadGovernanceRegistry(root)
	require.NoError(t, err)
	require.Len(t, registry, 9)
}

func TestLoadGovernanceRegistryMinimalFrontmatter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

	registry, err := LoadGovernanceRegistry(root)
	require.NoError(t, err)
	require.Len(t, registry, 9)

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

func TestLoadGovernanceRegistryAppliesConfiguredAgentMappings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

	cfg := model.DefaultConfig()
	cfg.Agents.Mappings = map[string]string{}
	cfg.Agents.Mappings["wave-orchestration"] = "slipway-reviewer"
	require.NoError(t, model.SaveConfig(filepath.Join(root, ".slipway.yaml"), cfg))

	registry, err := LoadGovernanceRegistry(root)
	require.NoError(t, err)

	def, ok := LookupDefinitionInRegistry(registry, "wave-orchestration")
	require.True(t, ok)
	assert.Equal(t, "slipway-reviewer", def.AgentHint)
}

func TestLoadGovernanceRegistryRejectsUnknownConfiguredAgent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

	cfg := model.DefaultConfig()
	cfg.Agents.Mappings = map[string]string{}
	cfg.Agents.Mappings["wave-orchestration"] = "slipway-missing"
	require.NoError(t, model.SaveConfig(filepath.Join(root, ".slipway.yaml"), cfg))

	_, err := LoadGovernanceRegistry(root)
	require.Error(t, err)

	var regErr *GovernanceRegistryError
	require.ErrorAs(t, err, &regErr)
	assert.Contains(t, err.Error(), `unknown agent "slipway-missing"`)
}

func TestLoadGovernanceRegistryRejectsManualOnlyConfiguredAgent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))

	cfg := model.DefaultConfig()
	cfg.Agents.Mappings = map[string]string{}
	cfg.Agents.Mappings["wave-orchestration"] = "slipway-executor"
	require.NoError(t, model.SaveConfig(filepath.Join(root, ".slipway.yaml"), cfg))

	_, err := LoadGovernanceRegistry(root)
	require.Error(t, err)

	var regErr *GovernanceRegistryError
	require.ErrorAs(t, err, &regErr)
	assert.Contains(t, err.Error(), `manual-only`)
}
