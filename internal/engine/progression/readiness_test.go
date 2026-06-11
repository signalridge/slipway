package progression

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/engine/scopecontract"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScopeContractRecoveryGuidanceHasSurfaceParity pins #86 dead-end (4): the
// scope-contract recovery guidance diagnostic is emitted whenever the contract
// has blockers, at any lifecycle state (S2_EXECUTE as well as S3/S4) — no
// state-gated suppression.
func TestScopeContractRecoveryGuidanceHasSurfaceParity(t *testing.T) {
	t.Parallel()

	withBlockers := scopecontract.Report{Blockers: []model.ReasonCode{model.NewReasonCode("scope_contract_drift", "")}}
	assert.True(t, scopeContractNeedsRecoveryGuidance(withBlockers),
		"scope guidance must surface whenever there are blockers, including at S2_EXECUTE")

	clean := scopecontract.Report{}
	assert.False(t, scopeContractNeedsRecoveryGuidance(clean))
}

func TestEvaluateGovernanceReadinessBlocksSensitiveFilesWithoutOwningEvidence(t *testing.T) {
	t.Parallel()

	const migration = "db/migrations/001_create_users.sql"
	readiness := evaluateSensitiveMigrationReadiness(t, "go-test:./...", migration)

	assert.True(t, hasAdvanceReasonDetail(
		readiness.Blockers,
		"sensitive_evidence_missing",
		"schema_migration:"+migration,
	), "sensitive file changes must require owning evidence")
}

func TestEvaluateGovernanceReadinessPassesSensitiveFilesWithOwningEvidence(t *testing.T) {
	t.Parallel()

	readiness := evaluateSensitiveMigrationReadiness(t, "migration-applied:goose up", "db/migrations/001_create_users.sql")

	assert.False(t, hasAdvanceReasonCode(readiness.Blockers, "sensitive_evidence_missing"))
	require.NotNil(t, readiness.SensitiveEvidence)
	assert.Equal(t, "pass", string(readiness.SensitiveEvidence.Status))
}

func evaluateSensitiveMigrationReadiness(t *testing.T, evidenceRef string, migration string) GovernanceReadiness {
	t.Helper()

	root := t.TempDir()
	initGitWorkspaceForReadinessTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-sensitive-evidence")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetLight
	require.NoError(t, state.SaveChange(root, change))

	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [ ] `+"`t-01`"+` apply schema migration
  - wave: 1
  - target_files: ["`+migration+`"]
  - task_kind: code
`)

	capturedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:       "t-01",
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindCode,
			ChangedFiles: []string{migration},
			TargetFiles:  []string{migration},
			EvidenceRef:  evidenceRef,
			CapturedAt:   capturedAt,
		}},
	}))

	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)
	return readiness
}

func TestEvaluateArtifactReadinessWithContext_IgnoresDependenciesOutsideEligibleLevel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("custom-artifact-deps")
	change.ArtifactSchema = model.ArtifactSchemaCustom
	change.CustomArtifacts = []model.ArtifactDefinition{
		{Name: "research.md", RequiresDiscovery: true},
		{Name: "decision.md", DependsOn: []string{"research.md"}},
	}
	change.NeedsDiscovery = false

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte("# Decision\n"), 0o644))

	policy, err := governance.ResolvePresetPolicy(root, change)
	require.NoError(t, err)
	ctx := resolveArtifactEvaluationContext(change, policy.EffectivePreset)

	readiness, err := evaluateArtifactReadinessWithContext(root, change, ctx)
	require.NoError(t, err)
	assert.False(t, hasAdvanceReasonDetail(readiness.Blockers, "required_artifact_dependency_missing", "decision.md->research.md"))
	assert.True(t, readiness.Ready)
}

func TestEvaluateGovernanceReadinessRetainsTraceabilityActionBlockersWhenSnapshotIsUnreadable(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspaceForReadinessTests(t, root)
	require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

	change := model.NewChange("readiness-unreadable-snapshot")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	change.GuardrailDomain = "auth_authz"
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	require.NoError(t, state.SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(`# Intent
INT-001: protect auth flows
## Open Questions
- [ ] unresolved MFA question
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements
### Requirement: auth review
REQ-001: preserve MFA. Traces to INT-001.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Keep MFA.

## Selected Approach
Use A.

## Interfaces and Data Flow
Keep current MFA checks.

## Rollout and Rollback
Rollback available.

## Risk
Auth risk.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] audit auth flow
  covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Scope Summary
Auth review.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
REQ-001: pending review evidence

## Residual Risks and Exceptions
Pending.

## Archive Decision
Not ready.
`), 0o644))

	snapshotPath := governance.SnapshotPath(root, change.Slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
	require.NoError(t, os.WriteFile(snapshotPath, []byte("version: ["), 0o644))

	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)

	foundClarification := false
	for _, action := range readiness.RequiredActions {
		if action.ControlID == model.ControlClarification {
			foundClarification = true
			assert.False(t, action.Satisfied, "blocking open questions must remain unsatisfied when preview ignores a corrupt snapshot")
		}
	}
	assert.True(t, foundClarification, "expected clarification action from live traceability evaluation")
	assert.True(t, hasAdvanceReasonDetail(readiness.Blockers, "governance_action_required", "clarification: resolve or defer blocking open questions in intent before downstream artifacts"))
	assert.NotContains(t, readiness.Diagnostics, "governance_snapshot_unavailable: parse governance snapshot")
}

func initGitWorkspaceForReadinessTests(t *testing.T, root string) {
	t.Helper()

	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Slipway Test"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}
}
