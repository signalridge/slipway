package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/toolgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequiredHighRiskTokenHints(t *testing.T) {
	t.Parallel()
	assert.Equal(t,
		[]string{"high_risk_check:external_api_contracts.safety_baseline=pass"},
		requiredHighRiskTokenHints(model.GuardrailDomainExternalAPIContracts),
	)
	assert.Nil(t, requiredHighRiskTokenHints(""))
}

func TestBuildSkillConstraintsSurfacesHighRiskTokensForGoalVerification(t *testing.T) {
	t.Parallel()
	change := model.NewChange("guardrail-change")
	change.GuardrailDomain = model.GuardrailDomainExternalAPIContracts
	def := skill.Definition{Name: progression.SkillGoalVerification}

	sc := buildSkillConstraints(t.TempDir(), def, &change)
	require.NotNil(t, sc)
	assert.Contains(t, sc.RequiredHighRiskTokens, "high_risk_check:external_api_contracts.safety_baseline=pass")
}

func TestBuildSkillConstraintsNoHighRiskTokensWithoutGuardrailDomain(t *testing.T) {
	t.Parallel()
	change := model.NewChange("plain-change") // no guardrail domain
	def := skill.Definition{Name: progression.SkillGoalVerification}

	sc := buildSkillConstraints(t.TempDir(), def, &change)
	require.NotNil(t, sc)
	assert.Empty(t, sc.RequiredHighRiskTokens)
}

func TestParseLockedDecisions(t *testing.T) {
	t.Parallel()
	t.Run("absent file returns nil", func(t *testing.T) {
		result := parseLockedDecisions("/nonexistent/decision.md")
		assert.Nil(t, result)
	})

	t.Run("file without relevant sections returns nil", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "decision.md")
		require.NoError(t, os.WriteFile(p, []byte("## Objectives\n- obj1\n"), 0o644))
		result := parseLockedDecisions(p)
		assert.Nil(t, result)
	})

	t.Run("empty sections returns nil", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "decision.md")
		require.NoError(t, os.WriteFile(p, []byte("## Alternatives Considered\n\n## Selected Approach\n\n## Risk\n"), 0o644))
		result := parseLockedDecisions(p)
		assert.Nil(t, result)
	})

	t.Run("populated decision returns selected approach", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "decision.md")
		content := `## Alternatives Considered
List approaches.

## Selected Approach
Use Go modules with a clean interface boundary for the new subsystem.

## Risk
- Low risk overall
`
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		result := parseLockedDecisions(p)
		require.NotNil(t, result)
		assert.Contains(t, result[0], "Selected Approach:")
	})

	t.Run("explicit selected direction is preserved", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "decision.md")
		content := `## Alternatives Considered
### Approach A
Keep the existing execution path.

### Approach B
Refactor before implementing the change.

### Selected Direction
Approach B because the current seams are too coupled for a safe direct change.

## Selected Approach
Use the refactor-first path and keep the external contract stable.

## Risk
- Medium risk overall
`
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		result := parseLockedDecisions(p)
		require.Len(t, result, 2)
		assert.Contains(t, result[0], "Selected Direction:")
		assert.Contains(t, result[1], "Selected Approach:")
	})
}

func TestSkillConstraintsPopulatedInNextOutput(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		slug := createGovernedRequest(t, root, "L2", "constraints test")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Set state to S1_PLAN/audit which resolves to plan-audit skill.
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		require.NotNil(t, view.NextSkill.SkillConstraints)

		sc := view.NextSkill.SkillConstraints
		assert.Equal(t, "stale or incomplete plan bundle", sc.MitigationTarget)
		assert.False(t, sc.RunSummaryBound)
		assert.Nil(t, sc.LockedDecisions, "fresh scaffolded seeded decision text must not be treated as a locked human-reviewed decision")
	})
}

func TestSkillConstraintsLockedDecisionsFromDecision(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		slug := createGovernedRequest(t, root, "L2", "locked decisions test")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		// Write decision.md with selected approach (canonical source for locked decisions).
		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "decision.md"), []byte(`## Alternatives Considered
List approaches.

## Selected Approach
Use event-driven architecture with Go channels.

## Interfaces and Data Flow
Standard interfaces.

## Rollout and Rollback
Standard rollout.

## Risk
Low risk.
`), 0o644))

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		require.NotNil(t, view.NextSkill.SkillConstraints)

		require.NotNil(t, view.NextSkill.SkillConstraints.LockedDecisions)
		assert.Contains(t, view.NextSkill.SkillConstraints.LockedDecisions[0], "Selected Approach:")

		var handoffOut bytes.Buffer
		handoffCmd := makeNextCmd()
		handoffCmd.SetArgs([]string{"--json"})
		handoffCmd.SetOut(&handoffOut)
		require.NoError(t, handoffCmd.Execute())

		var handoff nextHandoffView
		require.NoError(t, json.Unmarshal(handoffOut.Bytes(), &handoff))
		require.NotNil(t, handoff.NextSkill)
		require.NotNil(t, handoff.NextSkill.SkillConstraints)
		require.NotEmpty(t, handoff.NextSkill.SkillConstraints.LockedDecisions)
		assert.Contains(t, handoff.NextSkill.SkillConstraints.LockedDecisions[0], "Selected Approach:")
	})
}

func TestSkillConstraintsGuardrailDomainFromAdmission(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		slug := createGovernedRequest(t, root, "L2", "guardrail domain test")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		require.NotNil(t, view.NextSkill.SkillConstraints)

		assert.Equal(t, "auth_authz", view.NextSkill.SkillConstraints.GuardrailDomain)
	})
}

func TestDeriveAgentConstraintsDoesNotGateFinalCloseout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, toolgen.Generate(root, []string{"claude"}, true))
	registry, err := skill.LoadGovernanceRegistry(root)
	require.NoError(t, err)
	c := deriveAgentConstraints(registry, "final-closeout")
	require.NotNil(t, c)
	assert.Empty(t, c.HardGate)
	assert.Contains(t, c.AllowedOperations, "write_evidence")
}
