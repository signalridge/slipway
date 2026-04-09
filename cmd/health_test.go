package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestHealthCommandReportsRepairableFindings(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("defaults: ["), 0o644))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotEmpty(t, view.Findings)
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		found := false
		for _, finding := range view.Findings {
			if finding.Category == "config" {
				found = true
				assert.True(t, finding.Repairable)
			}
		}
		assert.True(t, found)
	})
}

func TestHealthCommandRejectsUninitializedGitRepo(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		err := cmd.Execute()
		require.Error(t, err)
		assert.ErrorIs(t, err, fsutil.ErrProjectRootNotFound)
		assert.Contains(t, err.Error(), "run `slipway init`")
	})
}

func TestHealthCommandObservationsFlagIncludesSignalProvenance(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "health observations")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json", "--governance", "--observations", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))

		observations, ok := payload["observations"].([]any)
		require.True(t, ok, "expected observations in health output")
		require.NotEmpty(t, observations)

		obsMap, ok := observations[0].(map[string]any)
		require.True(t, ok)
		assert.NotEmpty(t, obsMap["id"])
		assert.NotEmpty(t, obsMap["signal"])
		assert.NotEmpty(t, obsMap["source"])
	})
}

func TestHealthCommandGovernanceReportsUnreadableSnapshotInsteadOfFailing(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "health unreadable snapshot")
		snapshotPath := governance.SnapshotPath(root, slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
		require.NoError(t, os.WriteFile(
			snapshotPath,
			[]byte("version: ["),
			0o644,
		))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json", "--governance", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)
		// Snapshot absence is now non-blocking (Plan B: delete-safe audit cache).
		// Health degrades to WARN, not FAIL, so overall may remain healthy.

		found := false
		for _, check := range view.Governance.Checks {
			if check.Name == "signal_freshness" {
				found = true
				assert.Equal(t, "WARN", check.Status)
				assert.Contains(t, check.Message, "governance_audit_data_unavailable")
			}
		}
		assert.True(t, found, "expected unreadable snapshot warning to surface in governance health")
	})
}

func TestHealthCommandGovernanceObservationsStillRenderWhenSnapshotUnreadable(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "health unreadable snapshot observations")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		snapshotPath := governance.SnapshotPath(root, slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
		require.NoError(t, os.WriteFile(snapshotPath, []byte("version: ["), 0o644))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json", "--governance", "--observations", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))

		observations, ok := payload["observations"].([]any)
		require.True(t, ok, "expected observations in health output")
		require.NotEmpty(t, observations)

		rawSnapshot, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		assert.Equal(t, "version: [", string(rawSnapshot), "health should diagnose unreadable snapshot without rewriting it")
	})
}

func TestHealthCommandGovernanceSkipsRecomputeWhenBoundWorktreeInvalid(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, "L2", "health invalid bound worktree")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		change.GuardrailDomain = "auth_authz"
		change.WorktreePath = root
		change.WorktreeBranch = currentGitBranch(t, root)
		require.NoError(t, state.SaveChange(root, change))
		writeAuthReviewGovernedBundle(t, root, slug)

		paths, err := state.ResolveChangePaths(root, change)
		require.NoError(t, err)
		_, err = governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)
		require.NoError(t, err)

		snapshotPath := governance.SnapshotPath(root, slug)
		before, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json", "--governance", "--observations", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)
		assert.Empty(t, view.Observations)

		foundCoherence := false
		foundWorktree := false
		for _, check := range view.Governance.Checks {
			switch check.Name {
			case "signal_control_coherence":
				foundCoherence = true
				assert.Equal(t, "WARN", check.Status)
				assert.Contains(t, check.Message, "dedicated_worktree_required")
			case "worktree_binding":
				foundWorktree = true
				assert.Equal(t, "FAIL", check.Status)
				assert.Contains(t, check.Message, "dedicated_worktree_required")
			}
		}
		assert.True(t, foundCoherence, "expected signal_control_coherence check")
		assert.True(t, foundWorktree, "expected worktree_binding check")

		after, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after), "health should not recompute snapshots when worktree binding is invalid")
	})
}

func TestHealthCommandGovernanceRecomputesCurrentArtifacts(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "health live recompute")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		require.NoError(t, state.SaveChange(root, change))

		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(`# Intent
INT-001: stabilize auth middleware
## Open Questions
(none)
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements
### Requirement: auth stability
REQ-001: Preserve auth middleware behavior. Traces to INT-001.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Patch the current middleware.
### Option B
Rewrite the middleware.

## Selected Approach
Choose Option A.

## Interfaces and Data Flow
Existing auth entry points remain stable.

## Rollout and Rollback
Deploy gradually and roll back to the prior middleware.

## Risk
Auth regressions remain localized.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
  covers: [REQ-001]
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Scope Summary
Auth middleware only.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
REQ-001: pending

## Residual Risks and Exceptions
Pending.

## Rollback Readiness
Rollback path documented.

## Archive Decision
Not ready.
`), 0o644))

		paths, err := state.ResolveChangePaths(root, change)
		require.NoError(t, err)
		_, err = governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
`), 0o644))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json", "--governance", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)

		found := false
		for _, check := range view.Governance.Checks {
			if check.Name == "traceability_coherence" {
				found = true
				assert.Equal(t, "FAIL", check.Status)
				assert.Contains(t, check.Message, "blocking traceability gaps")
			}
		}
		assert.True(t, found, "expected traceability_coherence check")
	})
}

func TestHealthCommandGovernancePreservesPersistedFreshnessSignal(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "health stale persisted snapshot")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(`# Intent
INT-001: stabilize auth middleware
## Open Questions
(none)
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements
### Requirement: auth stability
REQ-001: Preserve auth middleware behavior. Traces to INT-001.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Patch the current middleware.

## Selected Approach
Choose Option A.

## Interfaces and Data Flow
Existing auth entry points remain stable.

## Rollout and Rollback
Deploy gradually and roll back to the prior middleware.

## Risk
Auth regressions remain localized.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
  covers: [REQ-001]
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Scope Summary
Auth middleware only.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
REQ-001: pending

## Residual Risks and Exceptions
Pending.

## Rollback Readiness
Rollback path documented.

## Archive Decision
Not ready.
`), 0o644))

		paths, err := state.ResolveChangePaths(root, change)
		require.NoError(t, err)
		snap, err := governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)
		require.NoError(t, err)

		snap.ComputedAt = time.Now().UTC().Add(-2 * time.Hour)
		raw, err := yaml.Marshal(snap)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(governance.SnapshotPath(root, slug), raw, 0o644))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json", "--governance", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)

		found := false
		for _, check := range view.Governance.Checks {
			if check.Name == "signal_freshness" {
				found = true
				assert.Equal(t, "WARN", check.Status)
				assert.Contains(t, check.Message, "stale")
			}
		}
		assert.True(t, found, "expected signal_freshness check")
	})
}

func TestHealthCommandGovernanceUsesFreshnessFromRecomputedSnapshotWhenMaterialStateChanges(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "health refreshed snapshot")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		bundleDir := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(`# Intent
INT-001: stabilize auth middleware
## Open Questions
(none)
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements
### Requirement: auth stability
REQ-001: Preserve auth middleware behavior. Traces to INT-001.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Patch the current middleware.

## Selected Approach
Choose Option A.

## Interfaces and Data Flow
Existing auth entry points remain stable.

## Rollout and Rollback
Deploy gradually and roll back to the prior middleware.

## Risk
Auth regressions remain localized.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
  covers: [REQ-001]
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Scope Summary
Auth middleware only.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
REQ-001: pending

## Residual Risks and Exceptions
Pending.

## Rollback Readiness
Rollback path documented.

## Archive Decision
Not ready.
`), 0o644))

		paths, err := state.ResolveChangePaths(root, change)
		require.NoError(t, err)
		snap, err := governance.RecomputeGovernanceSnapshot(root, change, paths.GovernedBundleDir)
		require.NoError(t, err)

		snap.ComputedAt = time.Now().UTC().Add(-2 * time.Hour)
		raw, err := yaml.Marshal(snap)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(governance.SnapshotPath(root, slug), raw, 0o644))

		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [ ] update auth middleware
`), 0o644))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json", "--governance", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.Governance)

		found := false
		for _, check := range view.Governance.Checks {
			if check.Name == "signal_freshness" {
				found = true
				assert.Equal(t, "OK", check.Status)
				assert.NotContains(t, check.Message, "stale")
			}
		}
		assert.True(t, found, "expected signal_freshness check")
	})
}

func TestHealthCommandGovernanceBlocksOnStateLock(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		slug := createGovernedRequest(t, root, "L2", "health lock")
		lockPath := state.ChangeStateLockPath(root, slug)
		stopLockHolder := startStateLockHolder(t, lockPath)
		defer stopLockHolder()

		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--governance", "--change", slug})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "state lock timeout")
	})
}

func TestHealthCommandGovernanceSurfacesMultipleActiveChangeError(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		// Create two active changes at the state layer to trigger ErrMultipleActiveChanges.
		changeA := model.NewChange("health-multi-a")
		require.NoError(t, state.SaveChange(root, changeA))

		changeB := model.NewChange("health-multi-b")
		require.NoError(t, state.SaveChange(root, changeB))

		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--governance"})

		execErr := cmd.Execute()
		require.Error(t, execErr, "governance health should surface multiple active changes error")
		assert.Contains(t, execErr.Error(), "multiple active changes")
	})
}

func TestHealthCommandGovernanceWithNoActiveChangeDoesNotRenderRepoHealthFallback(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--governance"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		assert.NotContains(t, out.String(), "Repo Health:")
		assert.NotContains(t, out.String(), "Governance Health")
	})
}
