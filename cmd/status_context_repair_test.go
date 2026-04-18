package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusDiagnosticsWhenNoActiveRequest(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json"})
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		assert.Equal(t, "diagnostics", payload["execution_mode"])
		assert.Equal(t, "unknown", payload["evidence_freshness"])
	})
}

func TestStatusCommandDefaultsToBoundWorktreeChangeWhenMultipleActiveChanges(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		boundSlug := createGovernedRequest(t, root, "L3", "bound worktree status default route")
		boundChange, err := state.LoadChange(root, boundSlug)
		require.NoError(t, err)
		boundChange.CurrentState = model.StateS2Execute
		boundChange.PlanSubStep = model.PlanSubStepNone
		boundChange.NeedsDiscovery = true
		require.NoError(t, state.SaveChange(root, boundChange))

		worktreeRoot := filepath.Join(t.TempDir(), boundSlug)
		branch := "feat/" + boundSlug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		relocated := boundChange
		require.NoError(t, state.PersistScopeWorktreeMetadata(&relocated, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, boundChange, relocated))
		require.NoError(t, state.SaveChange(root, relocated))

		otherChange := model.NewChange("second-active-change")
		require.NoError(t, state.SaveChange(root, otherChange))
		otherSlug := otherChange.Slug
		require.NotEqual(t, boundSlug, otherSlug)

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(worktreeRoot))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json"})
		require.NoError(t, cmd.Execute())

		var view statusView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, boundSlug, view.Slug)
		assert.Equal(t, "governed", view.ExecutionMode)
		assert.Equal(t, model.StateS2Execute, view.CurrentState)
		assert.NotEqual(t, otherSlug, view.Slug)
	})
}

func TestResolveStatusRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		changes           []model.Change
		wantSlug          string
		wantMultiRequest  bool
		wantDiagnosticMsg string
	}{
		{
			name:              "no active changes returns diagnostics",
			wantDiagnosticMsg: "no active change; start one with `slipway new`",
		},
		{
			name: "single change returns detail route",
			changes: []model.Change{
				{Slug: "change-1", Status: model.ChangeStatusActive},
			},
			wantSlug: "change-1",
		},
		{
			name: "multiple changes route to multi change summary",
			changes: []model.Change{
				{Slug: "change-1", Status: model.ChangeStatusActive},
				{Slug: "change-2", Status: model.ChangeStatusActive},
			},
			wantMultiRequest: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := resolveStatusRoute(tt.changes)
			if tt.wantSlug != "" {
				require.NotNil(t, route.change)
				assert.Equal(t, tt.wantSlug, route.change.Slug)
				assert.False(t, route.multiChange)
				assert.Nil(t, route.diagnostics)
				return
			}

			if tt.wantMultiRequest {
				assert.True(t, route.multiChange)
				assert.Nil(t, route.change)
				assert.Nil(t, route.diagnostics)
				return
			}

			require.NotNil(t, route.diagnostics)
			assert.Equal(t, []string{tt.wantDiagnosticMsg}, route.diagnostics.Diagnostics)
			assert.Equal(t, "diagnostics", route.diagnostics.ExecutionMode)
			assert.Equal(t, "unknown", route.diagnostics.EvidenceFreshness)
		})
	}
}

func TestShouldFallbackValidateDiagnostics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "no active change falls back",
			err:  newPreconditionError("no_active_change", "none", "create one", "", nil),
			want: true,
		},
		{
			name: "ambiguous active context falls back",
			err:  newPreconditionError("active_context_ambiguous", "ambiguous", "use new", "", nil),
			want: true,
		},
		{
			name: "inactive change does not fall back",
			err:  newPreconditionError("not_active", "inactive", "pick active", "req-1", nil),
			want: false,
		},
		{
			name: "state integrity does not fall back",
			err:  newStateIntegrityError("broken_state", "broken", "repair", "req-1", nil),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldFallbackValidateDiagnostics(tt.err))
		})
	}
}

func TestBuildStatusViewFromChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createActiveNonDiscoveryChange(t, root, "status change builder")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Equal(t, "governed", view.ExecutionMode)
	assert.Equal(t, slug, view.Slug)
	assert.Equal(t, string(change.Status), view.LifecycleStatus)
	assert.Equal(t, change.CurrentState, view.CurrentState)
	assert.Equal(t, filepath.Join("artifacts", "changes", slug, "change.yaml"), view.SourceStateFile)
}

func TestBuildGovernedStatusView(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status governed builder")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Equal(t, "governed", view.ExecutionMode)
	assert.Equal(t, slug, view.Slug)
	assert.Equal(t, string(change.Status), view.LifecycleStatus)
	assert.Equal(t, change.CurrentState, view.CurrentState)
	assert.Equal(t, filepath.Join("artifacts", "changes", slug, "change.yaml"), view.SourceStateFile)
	require.Contains(t, view.GateStatus, "G_plan")
	assert.NotEmpty(t, view.ArtifactDAG)
}

func TestBuildGovernedStatusViewRecomputesGovernanceReadinessWithoutPersistingSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status governance recompute")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = "auth_authz"
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte(`# Intent
INT-001: protect auth flows
## Open Questions
(none)
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "requirements.md"), []byte(`# Requirements
### Requirement: auth review
REQ-001: Auth changes must keep MFA intact. Traces to INT-001.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Keep existing MFA policy and update login checks.
### Option B
Refactor login middleware with the same MFA contract.

## Selected Approach
Adopt Option A because it changes less code.

## Interfaces and Data Flow
Auth entrypoints keep the existing MFA contract.

## Rollout and Rollback
Roll forward with a guarded rollout and roll back by restoring the prior auth handler.

## Risk
Regression risk is concentrated in auth flows.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks
- [ ] audit auth flow
  covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "assurance.md"), []byte(`# Assurance
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

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.GovernanceSignals)
	assert.Contains(t, view.GovernanceSignals.Domains, "auth_authz")
	require.NotEmpty(t, view.ActiveControls)
	require.NotEmpty(t, view.RequiredActions)

	foundDomainReview := false
	for _, action := range view.RequiredActions {
		if action.ControlID == "domain-review" {
			foundDomainReview = true
			assert.False(t, action.Satisfied)
		}
	}
	assert.True(t, foundDomainReview, "expected domain-review action to be surfaced")

	_, err = os.Stat(governance.SnapshotPath(root, slug))
	assert.True(t, os.IsNotExist(err), "status should not persist governance snapshots on read paths")
}

func TestBuildGovernedStatusViewChecksInvalidBoundWorktreeBeforeBundle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initGitRepoForWorktreeTests(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status invalid bound worktree")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	change.WorktreePath = root
	change.WorktreeBranch = currentGitBranch(t, root)
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	requireBlockerContains(t, view.Blockers, state.WorktreeReasonDedicatedRequired)
}

func TestBuildMultiChangeSummaryView(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	governedSlug := createGovernedRequest(t, root, "L2", "status governed summary")
	createIntakeChangeFixture(t, root, "status admission summary")

	changes, err := state.ListChanges(root)
	require.NoError(t, err)

	view := buildMultiChangeSummaryView(changes)
	require.Len(t, view.ActiveChanges, len(changes))

	found := false
	for _, entry := range view.ActiveChanges {
		if entry.Slug == governedSlug {
			assert.Equal(t, "governed", entry.ExecMode)
			found = true
		}
	}
	assert.True(t, found, "expected governed entry in multi-change summary")
}

func TestBuildGovernedStatusViewPlanAuditKeepsNextAction(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "status plan finalized")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, view.NextReadyActions, "next")
}

func TestStatusDirectExecutionView(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := makeNewCmd()
		create.SetArgs([]string{"fix login timeout"})
		require.NoError(t, create.Execute())

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json"})
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		assert.Equal(t, "governed", payload["execution_mode"])
		assert.Equal(t, "S0_INTAKE", payload["current_state"])
	})
}

func assertReadOnlyArtifactReconcileDoesNotPersist(
	t *testing.T,
	description string,
	expectedHash string,
	run func(out *bytes.Buffer) error,
) {
	t.Helper()

	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", description)

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.Artifacts["intent"] = model.ArtifactState{
			ID:          "intent",
			Path:        "intent.md",
			State:       model.ArtifactLifecycleApproved,
			ContentHash: expectedHash,
			UpdatedAt:   time.Now().UTC(),
		}
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, os.Remove(filepath.Join(root, "artifacts", "changes", change.Slug, "intent.md")))

		var out bytes.Buffer
		require.NoError(t, run(&out))

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		intent, ok := after.Artifacts["intent"]
		require.True(t, ok)
		assert.Equal(t, model.ArtifactLifecycleApproved, intent.State)
		assert.Equal(t, expectedHash, intent.ContentHash)
	})
}

func TestStatusDoesNotPersistArtifactReconcile(t *testing.T) {
	assertReadOnlyArtifactReconcileDoesNotPersist(
		t,
		"status read-only reconcile",
		"fixed-hash-before-status",
		func(out *bytes.Buffer) error {
			cmd := makeStatusCmd()
			cmd.SetOut(out)
			return cmd.Execute()
		},
	)
}

func TestStatusRejectsInvalidFormat(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cmd := makeStatusCmd()
		cmd.SetArgs([]string{"--format", "xml"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid --format "xml"; expected text|yaml|json`)
	})
}

func TestValidateDoesNotPersistArtifactReconcile(t *testing.T) {
	assertReadOnlyArtifactReconcileDoesNotPersist(
		t,
		"validate read-only reconcile",
		"fixed-hash-before-validate",
		func(out *bytes.Buffer) error {
			cmd := makeValidateCmd()
			cmd.SetOut(out)
			return cmd.Execute()
		},
	)
}

func TestRepairRecoversCorruptConfig(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		configPath := state.ConfigPath(root)
		require.NoError(t, os.WriteFile(configPath, []byte("execution: ["), 0o644))

		var out bytes.Buffer
		repair := makeRepairCmd()
		repair.SetArgs([]string{"--json"})
		repair.SetOut(&out)
		require.NoError(t, repair.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))

		backupPath, ok := payload["config_backup_path"].(string)
		require.True(t, ok)
		assert.NotEmpty(t, backupPath)
		_, err := os.Stat(backupPath)
		require.NoError(t, err)

		_, err = model.LoadConfig(configPath)
		require.NoError(t, err)
	})
}

func TestRepairRecreatesMissingConfig(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		require.NoError(t, os.MkdirAll(filepath.Join(state.GitStateDir(root), "runtime"), 0o755))

		configPath := state.ConfigPath(root)
		require.NoError(t, os.Remove(configPath))

		var out bytes.Buffer
		repair := makeRepairCmd()
		repair.SetArgs([]string{"--json"})
		repair.SetOut(&out)
		require.NoError(t, repair.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		_, hasBackup := payload["config_backup_path"]
		assert.False(t, hasBackup, "missing config should be recreated without backup")

		_, err := model.LoadConfig(configPath)
		require.NoError(t, err)
	})
}

func TestResolveExplicitRequestInactiveRemediationDoesNotPointToUnsupportedStatusRequest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "inactive explicit request remediation")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.Status = model.ChangeStatusDone
	require.NoError(t, state.SaveChange(root, change))

	_, err = resolveExplicitChange(root, slug)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "not_active", cliErr.ErrorCode)
	assert.NotContains(t, cliErr.Remediation, "status --change")
}
