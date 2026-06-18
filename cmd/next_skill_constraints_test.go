package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/gate"
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

	sc := buildSkillConstraints(t.TempDir(), def, &change, true)
	require.NotNil(t, sc)
	assert.Contains(t, sc.RequiredHighRiskTokens, "high_risk_check:external_api_contracts.safety_baseline=pass")
}

func TestBuildSkillConstraintsNoHighRiskTokensWithoutGuardrailDomain(t *testing.T) {
	t.Parallel()
	change := model.NewChange("plain-change") // no guardrail domain
	def := skill.Definition{Name: progression.SkillGoalVerification}

	sc := buildSkillConstraints(t.TempDir(), def, &change, true)
	require.NotNil(t, sc)
	assert.Empty(t, sc.RequiredHighRiskTokens)
}

func TestParseDecisionItems(t *testing.T) {
	t.Parallel()
	t.Run("absent file returns nil", func(t *testing.T) {
		result := parseDecisionItems("/nonexistent/decision.md")
		assert.Nil(t, result)
	})

	t.Run("file without relevant sections returns nil", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "decision.md")
		require.NoError(t, os.WriteFile(p, []byte("## Objectives\n- obj1\n"), 0o644))
		result := parseDecisionItems(p)
		assert.Nil(t, result)
	})

	t.Run("empty sections returns nil", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "decision.md")
		require.NoError(t, os.WriteFile(p, []byte("## Alternatives Considered\n\n## Selected Approach\n\n## Risk\n"), 0o644))
		result := parseDecisionItems(p)
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
		result := parseDecisionItems(p)
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
		result := parseDecisionItems(p)
		require.Len(t, result, 2)
		assert.Contains(t, result[0], "Selected Direction:")
		assert.Contains(t, result[1], "Selected Approach:")
	})

	t.Run("unusable statuses return nil", func(t *testing.T) {
		tests := []struct {
			name    string
			heading string
			status  string
		}{
			{name: "superseded", heading: "## Status", status: "Superseded"},
			{name: "lowercase superseded heading", heading: "## status", status: "Superseded"},
			{name: "inactive", heading: "## Status", status: "Inactive"},
			{name: "unaccepted", heading: "## Status", status: "unaccepted"},
			{name: "drafted", heading: "## Status", status: "drafted"},
			{name: "mixed accepted superseded", heading: "## Status", status: "Accepted, superseded by DEC-001"},
			{
				name: "accepted status superseded by lifecycle",
				heading: `## Status
Accepted

## Lifecycle`,
				status: "Superseded by DEC-001",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tmp := t.TempDir()
				p := filepath.Join(tmp, "decision.md")
				content := tt.heading + `
` + tt.status + `

## Alternatives Considered
List approaches.

## Selected Approach
Use Go modules with a clean interface boundary for the new subsystem.

## Risk
- Low risk overall
`
				require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
				result := parseDecisionItems(p)
				assert.Nil(t, result)
			})
		}
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
		assert.Nil(t, sc.PendingDecisions, "scaffolded placeholder decision text is neither locked nor pending")
	})
}

// TestSkillConstraintsPendingDecisionsBeforePlanLock reproduces issue #140: a
// real, author-written Selected Approach in decision.md while the plan is NOT
// yet locked (G_plan not approved at S1_PLAN/audit, no plan-audit evidence)
// must be surfaced as a PENDING decision, never as a locked one — in both the
// full next view and the compact handoff payload.
func TestSkillConstraintsPendingDecisionsBeforePlanLock(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		slug := createGovernedRequest(t, root, "L2", "pending decisions test")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// S1_PLAN/audit without plan-audit evidence: the plan is not locked, so
		// G_plan is not approved.
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		// Write decision.md with a concrete (non-placeholder) selected approach.
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

		// The plan is not locked: the recommended approach is pending, not locked.
		assert.Empty(t, view.NextSkill.SkillConstraints.LockedDecisions,
			"a recommended approach must not appear as locked before the plan is locked (issue #140)")
		require.NotEmpty(t, view.NextSkill.SkillConstraints.PendingDecisions)
		assert.Contains(t, view.NextSkill.SkillConstraints.PendingDecisions[0], "Selected Approach:")

		var handoffOut bytes.Buffer
		handoffCmd := makeNextCmd()
		handoffCmd.SetArgs([]string{"--json"})
		handoffCmd.SetOut(&handoffOut)
		require.NoError(t, handoffCmd.Execute())

		var handoff nextHandoffView
		require.NoError(t, json.Unmarshal(handoffOut.Bytes(), &handoff))
		require.NotNil(t, handoff.NextSkill)
		require.NotNil(t, handoff.NextSkill.SkillConstraints)
		// The handoff clone preserves the pending field and never promotes it to locked.
		assert.Empty(t, handoff.NextSkill.SkillConstraints.LockedDecisions)
		require.NotEmpty(t, handoff.NextSkill.SkillConstraints.PendingDecisions)
		assert.Contains(t, handoff.NextSkill.SkillConstraints.PendingDecisions[0], "Selected Approach:")
	})
}

// TestSkillConstraintsLockedDecisionsAfterPlanLock verifies the confirmed path
// through real next JSON output: once G_plan is approved, the selected approach
// moves from pending_decisions to locked_decisions.
func TestSkillConstraintsLockedDecisionsAfterPlanLock(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		slug := createGovernedRequest(t, root, "L2", "locked decisions test")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeShipReadyGovernedBundle(t, root, change)
		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "decision.md", []byte(`# Decision
## Alternatives Considered
### Option A
Keep reporting parsed decision text as locked.

### Option B
Use the G_plan gate as the lock signal.

## Selected Approach
Use the G_plan gate state as the locked-vs-pending decision authority.

## Interfaces and Data Flow
Readiness GateEvaluations[G_plan] sets planLocked before skill constraints are assembled.

## Rollout and Rollback
Roll forward by preserving the recommendation under pending_decisions until G_plan is approved.

## Risk
Low risk; the public field split is additive and fail-closed before approval.
`)))
		writeSkillVerification(t, root, slug, progression.SkillPlanAudit, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			References: planAuditOriginReferences(),
		})
		refreshPassingSkillDigestsForTest(t, root, slug, progression.SkillPlanAudit)

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		require.NotNil(t, view.NextSkill.SkillConstraints)
		assert.Equal(t, "approved", view.InputContext.GateStatus[string(gate.GatePlan)])

		require.NotEmpty(t, view.NextSkill.SkillConstraints.LockedDecisions)
		assert.Contains(t, view.NextSkill.SkillConstraints.LockedDecisions[0], "Selected Approach:")
		assert.Empty(t, view.NextSkill.SkillConstraints.PendingDecisions)
	})
}

// TestBuildSkillConstraintsLockedVsPending pins the routing contract directly:
// the same parsed decision is reported as locked iff the plan is locked
// (G_plan approved), as pending otherwise, and placeholder/empty decision text
// is neither (issue #140).
func TestBuildSkillConstraintsLockedVsPending(t *testing.T) {
	t.Parallel()
	def := skill.Definition{Name: progression.SkillWaveOrchestration}

	writeDecision := func(t *testing.T, root string, change model.Change, body string) {
		t.Helper()
		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "decision.md"), []byte(body), 0o644))
	}

	realDecision := `## Alternatives Considered
List approaches.

## Selected Approach
Use event-driven architecture with Go channels.

## Risk
Low risk.
`

	t.Run("planLocked routes to locked_decisions", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		change := model.NewChange("locked-routing")
		writeDecision(t, root, change, realDecision)

		sc := buildSkillConstraints(root, def, &change, true)
		require.NotNil(t, sc)
		require.NotEmpty(t, sc.LockedDecisions)
		assert.Contains(t, sc.LockedDecisions[0], "Selected Approach:")
		assert.Empty(t, sc.PendingDecisions)
	})

	t.Run("not planLocked routes to pending_decisions", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		change := model.NewChange("pending-routing")
		writeDecision(t, root, change, realDecision)

		sc := buildSkillConstraints(root, def, &change, false)
		require.NotNil(t, sc)
		assert.Empty(t, sc.LockedDecisions)
		require.NotEmpty(t, sc.PendingDecisions)
		assert.Contains(t, sc.PendingDecisions[0], "Selected Approach:")
	})

	t.Run("placeholder decision is neither locked nor pending", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		change := model.NewChange("placeholder-routing")
		writeDecision(t, root, change, "## Selected Approach\nConfirm or replace this after research and user selection.\n")

		scLocked := buildSkillConstraints(root, def, &change, true)
		require.NotNil(t, scLocked)
		assert.Empty(t, scLocked.LockedDecisions)
		assert.Empty(t, scLocked.PendingDecisions)

		scPending := buildSkillConstraints(root, def, &change, false)
		require.NotNil(t, scPending)
		assert.Empty(t, scPending.LockedDecisions)
		assert.Empty(t, scPending.PendingDecisions)
	})

	t.Run("unusable status is neither locked nor pending", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			heading string
			status  string
		}{
			{name: "deprecated", heading: "## Status", status: "Deprecated"},
			{name: "lowercase superseded heading", heading: "## status", status: "Superseded"},
			{name: "inactive", heading: "## Status", status: "Inactive"},
			{name: "unaccepted", heading: "## Status", status: "unaccepted"},
			{name: "drafted", heading: "## Status", status: "drafted"},
			{name: "mixed accepted superseded", heading: "## Status", status: "Accepted, superseded by DEC-001"},
			{
				name: "accepted status superseded by lifecycle",
				heading: `## Status
Accepted

## Lifecycle`,
				status: "Superseded by DEC-001",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				change := model.NewChange("unusable-status-routing")
				writeDecision(t, root, change, tt.heading+`
`+tt.status+`

## Alternatives Considered
List approaches.

## Selected Approach
Use event-driven architecture with Go channels.

## Risk
Low risk.
`)

				scLocked := buildSkillConstraints(root, def, &change, true)
				require.NotNil(t, scLocked)
				assert.Empty(t, scLocked.LockedDecisions)
				assert.Empty(t, scLocked.PendingDecisions)

				scPending := buildSkillConstraints(root, def, &change, false)
				require.NotNil(t, scPending)
				assert.Empty(t, scPending.LockedDecisions)
				assert.Empty(t, scPending.PendingDecisions)
			})
		}
	})
}

// TestPlanLockedFromGates pins the lock signal: locked iff the G_plan gate is
// approved (issue #140).
func TestPlanLockedFromGates(t *testing.T) {
	t.Parallel()
	approved := progression.GovernanceReadiness{
		GateEvaluations: map[gate.GateID]gate.GateEvaluation{
			gate.GatePlan: {GateID: gate.GatePlan, Status: model.GateStatusApproved},
		},
	}
	blocked := progression.GovernanceReadiness{
		GateEvaluations: map[gate.GateID]gate.GateEvaluation{
			gate.GatePlan: {GateID: gate.GatePlan, Status: model.GateStatusBlocked},
		},
	}
	assert.True(t, planLockedFromGates(approved))
	assert.False(t, planLockedFromGates(blocked))
	assert.False(t, planLockedFromGates(progression.GovernanceReadiness{}), "missing G_plan evaluation is not locked")
}

func TestSkillConstraintsGuardrailDomainFromAdmission(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, []string{"claude"}, false))

		slug := createGovernedRequest(t, root, "L2", "guardrail domain test")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
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
