package governance

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectGovernanceHealthAllOK(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "healthy-change"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	snap := newTestSnapshot()
	require.NoError(t, SaveSnapshot(root, slug, snap))

	change := model.Change{
		Slug:            slug,
		CurrentState:    model.StateS1Plan,
		PlanSubStep:     model.PlanSubStepBundle,
		ArtifactSchema:  model.ArtifactSchemaExpanded,
		GuardrailDomain: "auth_authz",
	}

	report := CollectGovernanceHealth(root, change)
	assert.True(t, report.Healthy)
	assert.Equal(t, slug, report.Slug)

	for _, check := range report.Checks {
		assert.NotEqual(t, "FAIL", check.Status, "check %s should not fail", check.Name)
	}
}

func TestCollectGovernanceHealthNoSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "no-snapshot"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	change := model.Change{
		Slug:         slug,
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepBundle,
	}

	report := CollectGovernanceHealth(root, change)
	// Should be healthy (WARN for missing snapshot, but not FAIL).
	assert.True(t, report.Healthy)

	var names []string
	for _, check := range report.Checks {
		names = append(names, check.Name)
	}
	assert.Contains(t, names, "controls_config")
	assert.Contains(t, names, "signal_freshness")
	assert.Contains(t, names, "traceability_coherence")
	assert.Contains(t, names, "signal_control_coherence")
}

func TestCollectGovernanceHealthChecksAdvisoryPolicyPacks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "policy-pack-ok"
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".slipway", "policies"), 0o755))
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(`governance:
  policy_packs:
    - name: platform
      path: .slipway/policies/platform.yaml
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".slipway", "policies", "platform.yaml"), []byte(`version: 1
name: platform
advisory_rules:
  - require rollback notes for config changes
`), 0o644))

	report := CollectGovernanceHealth(root, model.Change{
		Slug:         slug,
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepBundle,
	})

	check := governanceHealthCheckByName(t, report, "policy_packs")
	assert.Equal(t, "OK", check.Status)
	assert.Contains(t, check.Message, "advisory policy pack")
}

func TestCollectGovernanceHealthWarnsOnPolicyPackBlockingFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".slipway", "policies"), 0o755))
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(`governance:
  policy_packs:
    - name: platform
      path: .slipway/policies/platform.yaml
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".slipway", "policies", "platform.yaml"), []byte(`version: 1
blocking_controls:
  - domain-review
`), 0o644))

	report := CollectGovernanceHealth(root, model.Change{
		Slug:         "policy-pack-warn",
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepBundle,
	})

	check := governanceHealthCheckByName(t, report, "policy_packs")
	assert.Equal(t, "WARN", check.Status)
	assert.Contains(t, check.Message, "unsupported blocking fields")
}

func governanceHealthCheckByName(t *testing.T, report GovernanceHealthReport, name string) GovernanceHealthCheck {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	require.Failf(t, "missing governance health check", "name=%s", name)
	return GovernanceHealthCheck{}
}

func TestCollectGovernanceHealthFailsOnUnknownControlOverrideIDs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "warning-controls"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))
	// .slipway.yaml is the sole authority; unknown control IDs should fail closed.
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(`governance:
  disabled_controls:
    - nonexistent-control
`), 0o644))

	change := model.Change{
		Slug:         slug,
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepBundle,
	}

	report := CollectGovernanceHealth(root, change)
	assert.False(t, report.Healthy)

	var controlsCheck GovernanceHealthCheck
	for _, check := range report.Checks {
		if check.Name == "controls_config" {
			controlsCheck = check
			break
		}
	}
	assert.Equal(t, "FAIL", controlsCheck.Status)
	assert.Contains(t, controlsCheck.Message, "parse error")
	assert.Contains(t, controlsCheck.Message, "unknown control_id")
}

func TestCollectGovernanceHealthStaleSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "stale-change"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	snap := newTestSnapshot()
	snap.ComputedAt = time.Now().UTC().Add(-2 * time.Hour)
	// Force write by making it different.
	snap.Summary.BlastRadius = model.SignalLevelHigh
	require.NoError(t, SaveSnapshot(root, slug, snap))

	change := model.Change{
		Slug:         slug,
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepBundle,
	}

	report := CollectGovernanceHealth(root, change)
	var freshnessCheck GovernanceHealthCheck
	for _, c := range report.Checks {
		if c.Name == "signal_freshness" {
			freshnessCheck = c
			break
		}
	}
	assert.Equal(t, "WARN", freshnessCheck.Status)
	assert.Contains(t, freshnessCheck.Message, "stale")
}

func TestCollectGovernanceHealthFailsOnInvalidBoundWorktree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "invalid-bound-worktree"
	initGitRepoForGovernanceHealthTests(t, root)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	snap := newTestSnapshot()
	require.NoError(t, SaveSnapshot(root, slug, snap))

	change := model.Change{
		Slug:           slug,
		CurrentState:   model.StateS2Execute,
		PlanSubStep:    model.PlanSubStepNone,
		WorktreePath:   root,
		WorktreeBranch: "main",
	}

	report := CollectGovernanceHealth(root, change)
	assert.False(t, report.Healthy)

	var worktreeCheck GovernanceHealthCheck
	for _, check := range report.Checks {
		if check.Name == "worktree_binding" {
			worktreeCheck = check
			break
		}
	}
	assert.Equal(t, "FAIL", worktreeCheck.Status)
	assert.Contains(t, worktreeCheck.Message, "dedicated_worktree_required")
}

func TestCollectGovernanceHealthSkipsSignalControlCoherenceWhenBoundWorktreeInvalid(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "invalid-bound-worktree-coherence"
	initGitRepoForGovernanceHealthTests(t, root)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	snap := newTestSnapshot()
	change := model.Change{
		Slug:           slug,
		CurrentState:   model.StateS2Execute,
		PlanSubStep:    model.PlanSubStepNone,
		WorktreePath:   root,
		WorktreeBranch: "main",
	}

	report := CollectGovernanceHealthWithSnapshot(root, change, snap)

	var coherenceCheck GovernanceHealthCheck
	for _, check := range report.Checks {
		if check.Name == "signal_control_coherence" {
			coherenceCheck = check
			break
		}
	}
	assert.Equal(t, "WARN", coherenceCheck.Status)
	assert.Contains(t, coherenceCheck.Message, "dedicated_worktree_required")
}

func TestCollectGovernanceHealthSnapshotReadFailureStillChecksWorktreeBinding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "invalid-bound-worktree-unreadable-snapshot"
	initGitRepoForGovernanceHealthTests(t, root)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))
	require.NoError(t, os.WriteFile(SnapshotPath(root, slug), []byte("version: ["), 0o644))

	change := model.Change{
		Slug:           slug,
		CurrentState:   model.StateS2Execute,
		PlanSubStep:    model.PlanSubStepNone,
		WorktreePath:   root,
		WorktreeBranch: "main",
	}

	report := CollectGovernanceHealth(root, change)
	assert.False(t, report.Healthy)

	var worktreeCheck GovernanceHealthCheck
	for _, check := range report.Checks {
		if check.Name == "worktree_binding" {
			worktreeCheck = check
			break
		}
	}
	assert.Equal(t, "FAIL", worktreeCheck.Status)
	assert.Contains(t, worktreeCheck.Message, "dedicated_worktree_required")
}

func TestRecomputeGovernanceSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "recompute-test"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	change := model.Change{
		Slug:            slug,
		CurrentState:    model.StateS1Plan,
		PlanSubStep:     model.PlanSubStepBundle,
		GuardrailDomain: "auth_authz",
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", slug)

	snap, err := RecomputeGovernanceSnapshot(root, change, bundleDir)
	require.NoError(t, err)

	assert.Equal(t, model.GovernanceSnapshotVersion, snap.Version)
	assert.Equal(t, model.SignalLevelLow, snap.Summary.BlastRadius)
	assert.Contains(t, snap.Summary.Domains, "auth_authz")
	// TriggerEvent is intentionally not part of GovernanceSnapshot.

	// Should have domain-related controls.
	var controlIDs []model.ControlID
	for _, c := range snap.ActiveControls {
		controlIDs = append(controlIDs, c.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlDomainReview)

	// Snapshot should be persisted.
	loaded, err := LoadSnapshot(root, slug)
	require.NoError(t, err)
	assert.Equal(t, snap.Version, loaded.Version)
}

func TestRecomputeGovernanceSnapshotUsesTasksChecklistTargetFilesPreExecution(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "planned-target-files"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	change := model.Change{
		Slug:           slug,
		CurrentState:   model.StateS1Plan,
		PlanSubStep:    model.PlanSubStepBundle,
		ArtifactSchema: model.ArtifactSchemaExpanded,
	}

	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	writePlanningTasksChecklist(t, bundleDir, `# Tasks

## Task List

- [ ] `+"`t-1`"+` planned blast radius
  - target_files: ["cmd/a.go", "cmd/b.go", "cmd/c.go", "cmd/d.go", "cmd/e.go", "cmd/f.go", "cmd/g.go", "cmd/h.go", "cmd/i.go", "cmd/j.go", "cmd/k.go"]
  - task_kind: code
  - covers: ["REQ-001"]
`)

	snap, err := RecomputeGovernanceSnapshot(root, change, bundleDir)
	require.NoError(t, err)

	assert.Equal(t, model.SignalLevelHigh, snap.Summary.BlastRadius)

	var controlIDs []model.ControlID
	for _, control := range snap.ActiveControls {
		controlIDs = append(controlIDs, control.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlIndependentReview)
	assert.Contains(t, controlIDs, model.ControlWorktreeIsolation)
}

func TestRecomputeGovernanceSnapshotPreservesExistingControlsWithoutCurrentCandidate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "monotonic-recompute-controls"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))
	require.NoError(t, SaveSnapshot(root, slug, model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{
			BlastRadius: model.SignalLevelHigh,
		},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			{
				ControlID:    model.ControlIndependentReview,
				Mode:         model.ControlModeBlocking,
				Scope:        model.ControlScopeReview,
				Active:       true,
				TriggeredBy:  []string{"blast_radius=high"},
				PolicySource: model.BuiltinPolicySource,
			},
		},
		ComputedAt: time.Now().UTC().Add(-time.Hour),
	}))

	change := model.Change{
		Slug:         slug,
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepBundle,
	}

	snap, err := RecomputeGovernanceSnapshot(root, change, filepath.Join(root, "artifacts", "changes", slug))
	require.NoError(t, err)
	assert.Equal(t, model.SignalLevelLow, snap.Summary.BlastRadius)

	var controlIDs []model.ControlID
	for _, control := range snap.ActiveControls {
		controlIDs = append(controlIDs, control.ControlID)
	}
	assert.Contains(t, controlIDs, model.ControlIndependentReview,
		"core recompute must preserve existing active controls unless an explicit deactivation path clears them")
}

func TestRecomputeGovernanceSnapshotRecoversFromUnreadableExistingSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "corrupt-existing-snapshot"

	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))
	require.NoError(t, os.WriteFile(
		SnapshotPath(root, slug),
		[]byte("version: ["),
		0o644,
	))

	change := model.Change{
		Slug:            slug,
		CurrentState:    model.StateS1Plan,
		PlanSubStep:     model.PlanSubStepBundle,
		GuardrailDomain: "auth_authz",
	}

	snap, err := RecomputeGovernanceSnapshot(root, change, filepath.Join(root, "artifacts", "changes", slug))
	require.NoError(t, err)
	assert.Equal(t, model.GovernanceSnapshotVersion, snap.Version)

	loaded, err := LoadSnapshot(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.GovernanceSnapshotVersion, loaded.Version)

	backups, err := filepath.Glob(filepath.Join(
		filepath.Dir(SnapshotPath(root, slug)),
		"governance_snapshot.broken.*.yaml",
	))
	require.NoError(t, err)
	require.Len(t, backups, 1, "expected unreadable snapshot to be backed up before regeneration")

	rawBackup, err := os.ReadFile(backups[0])
	require.NoError(t, err)
	assert.Equal(t, "version: [", string(rawBackup))
}

func TestPreviewGovernanceSnapshotRecoversFromUnreadableExistingSnapshotWithoutRewrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "corrupt-preview-snapshot"

	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))
	require.NoError(t, os.WriteFile(
		SnapshotPath(root, slug),
		[]byte("version: ["),
		0o644,
	))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	change := model.Change{
		Slug:            slug,
		CurrentState:    model.StateS1Plan,
		PlanSubStep:     model.PlanSubStepBundle,
		GuardrailDomain: "auth_authz",
	}

	snap, err := PreviewGovernanceSnapshot(root, change, filepath.Join(root, "artifacts", "changes", slug))
	require.NoError(t, err)
	assert.Equal(t, model.GovernanceSnapshotVersion, snap.Version)
	assert.Contains(t, snap.Summary.Domains, "auth_authz")

	raw, err := os.ReadFile(SnapshotPath(root, slug))
	require.NoError(t, err)
	assert.Equal(t, "version: [", string(raw), "preview path must not rewrite the unreadable snapshot")

	backups, err := filepath.Glob(filepath.Join(
		filepath.Dir(SnapshotPath(root, slug)),
		"governance_snapshot.broken.*.yaml",
	))
	require.NoError(t, err)
	assert.Empty(t, backups, "preview recovery must stay read-only")
}

func TestPreviewGovernanceSnapshotDoesNotPreserveStaleExistingControls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "stale-preview-controls"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))
	require.NoError(t, SaveSnapshot(root, slug, model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{BlastRadius: model.SignalLevelLow},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			{
				ControlID:    model.ControlDomainReview,
				Mode:         model.ControlModeBlocking,
				Scope:        model.ControlScopeReview,
				Active:       true,
				TriggeredBy:  []string{"domain_present"},
				PolicySource: model.BuiltinPolicySource,
			},
		},
		ComputedAt: time.Now().UTC(),
	}))

	change := model.Change{
		Slug:         slug,
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepBundle,
	}

	snap, err := PreviewGovernanceSnapshot(root, change, filepath.Join(root, "artifacts", "changes", slug))
	require.NoError(t, err)
	assert.Empty(t, snap.Summary.Domains)
	assert.Empty(t, snap.ActiveControls, "preview should reflect current derivation, not preserve stale controls from persisted snapshot")
}

func TestSignalControlCoherenceHonorsDisabledControls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "disabled-domain-review"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))
	// .slipway.yaml is the sole authority for governance overrides.
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(`governance:
  disabled_controls:
    - domain-review
`), 0o644))

	change := model.Change{
		Slug:            slug,
		CurrentState:    model.StateS1Plan,
		PlanSubStep:     model.PlanSubStepBundle,
		ArtifactSchema:  model.ArtifactSchemaExpanded,
		GuardrailDomain: "auth_authz",
	}

	_, err := RecomputeGovernanceSnapshot(root, change, filepath.Join(root, "artifacts", "changes", slug))
	require.NoError(t, err)

	snap, err := LoadSnapshot(root, slug)
	require.NoError(t, err)
	check := checkSignalControlCoherence(root, change, snap)
	assert.Equal(t, "OK", check.Status)
}

func TestCollectGovernanceHealthFailsSignalControlCoherenceWhenTasksChecklistImpliesMissingControls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "coherence-missing-controls"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	change := model.Change{
		Slug:           slug,
		CurrentState:   model.StateS1Plan,
		PlanSubStep:    model.PlanSubStepBundle,
		ArtifactSchema: model.ArtifactSchemaExpanded,
	}

	writePlanningTasksChecklist(t, filepath.Join(root, "artifacts", "changes", slug), `# Tasks

## Task List

- [ ] `+"`t-1`"+` planned blast radius
  - target_files: ["cmd/a.go", "cmd/b.go", "cmd/c.go", "cmd/d.go", "cmd/e.go", "cmd/f.go", "cmd/g.go", "cmd/h.go", "cmd/i.go", "cmd/j.go", "cmd/k.go"]
  - task_kind: code
  - covers: ["REQ-001"]
`)

	snap := model.GovernanceSnapshot{
		Version:      model.GovernanceSnapshotVersion,
		Summary:      model.SignalSummary{BlastRadius: model.SignalLevelLow},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
		ComputedAt:   time.Now().UTC(),
	}

	report := CollectGovernanceHealthWithSnapshot(root, change, snap)

	var coherence GovernanceHealthCheck
	for _, check := range report.Checks {
		if check.Name == "signal_control_coherence" {
			coherence = check
			break
		}
	}
	assert.Equal(t, "FAIL", coherence.Status)
	assert.Contains(t, coherence.Message, "independent-review")
	assert.Contains(t, coherence.Message, "worktree-isolation")
}

func TestCollectGovernanceHealthFailsSignalControlCoherenceOnUnexpectedControls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "coherence-unexpected-controls"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	change := model.Change{
		Slug:           slug,
		CurrentState:   model.StateS1Plan,
		PlanSubStep:    model.PlanSubStepBundle,
		ArtifactSchema: model.ArtifactSchemaExpanded,
	}

	research := model.ControlActivation{
		ControlID:    model.ControlResearch,
		Mode:         model.ControlModeBlocking,
		Scope:        model.ControlScopeDiscovery,
		Active:       true,
		TriggeredBy:  []string{"needs_discovery=true"},
		PolicySource: model.BuiltinPolicySource,
	}
	snap := model.GovernanceSnapshot{
		Version:        model.GovernanceSnapshotVersion,
		Summary:        model.SignalSummary{BlastRadius: model.SignalLevelLow},
		Traceability:   model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
		ActiveControls: []model.ControlActivation{research},
		ComputedAt:     time.Now().UTC(),
	}

	report := CollectGovernanceHealthWithSnapshot(root, change, snap)

	var coherence GovernanceHealthCheck
	for _, check := range report.Checks {
		if check.Name == "signal_control_coherence" {
			coherence = check
			break
		}
	}
	assert.Equal(t, "FAIL", coherence.Status)
	assert.Contains(t, coherence.Message, "unexpected active controls")
	assert.Contains(t, coherence.Message, "research")
}

func TestCollectGovernanceHealthDetectsStaleControlMode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "stale-mode"

	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))

	change := model.Change{
		Slug:            slug,
		CurrentState:    model.StateS1Plan,
		PlanSubStep:     model.PlanSubStepBundle,
		ArtifactSchema:  model.ArtifactSchemaExpanded,
		GuardrailDomain: "auth_authz",
	}

	// Snapshot with worktree-isolation in blocking mode (stale — current default is advisory).
	now := time.Now().UTC()
	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{
			Domains:     []string{"auth_authz"},
			BlastRadius: model.SignalLevelLow,
		},
		Traceability: model.TraceabilitySummary{Status: model.TraceabilityStatusOK},
		ActiveControls: []model.ControlActivation{
			{
				ControlID:    model.ControlDomainReview,
				Mode:         model.ControlModeBlocking,
				Scope:        model.ControlScopeReview,
				Active:       true,
				TriggeredBy:  []string{"auth_authz"},
				PolicySource: model.BuiltinPolicySource,
			},
			{
				ControlID:    model.ControlIndependentReview,
				Mode:         model.ControlModeBlocking,
				Scope:        model.ControlScopeReview,
				Active:       true,
				TriggeredBy:  []string{"domain_present"},
				PolicySource: model.BuiltinPolicySource,
			},
			{
				ControlID:    model.ControlWorktreeIsolation,
				Mode:         model.ControlModeBlocking, // stale: should be advisory
				Scope:        model.ControlScopeExecution,
				Active:       true,
				TriggeredBy:  []string{"domain_present"},
				PolicySource: model.BuiltinPolicySource,
			},
		},
		ComputedAt: now,
	}

	report := CollectGovernanceHealthWithSnapshot(root, change, snap)

	var coherence GovernanceHealthCheck
	for _, check := range report.Checks {
		if check.Name == "signal_control_coherence" {
			coherence = check
			break
		}
	}
	assert.Equal(t, "FAIL", coherence.Status)
	assert.Contains(t, coherence.Message, "stale control mode")
	assert.Contains(t, coherence.Message, "worktree-isolation")
}

func TestCheckControlsConfigWarnsOnInvalidThresholds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write config with valid YAML structure but semantically invalid threshold.
	// Note: ParseConfigYAML now rejects invalid thresholds at parse time,
	// so we test checkControlsConfig's handling of the parse error.
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(`governance:
  thresholds:
    worktree_blast_radius: severe
`), 0o644))

	check := checkControlsConfig(root, model.NewChange("invalid-thresholds"))
	assert.Equal(t, "FAIL", check.Status, "invalid threshold should cause parse failure")
	assert.Contains(t, check.Message, "parse error")
}

func TestCheckControlsConfigFailsOnInvalidMode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(`governance:
  controls:
    worktree-isolation: severe
`), 0o644))

	check := checkControlsConfig(root, model.NewChange("invalid-mode"))
	assert.Equal(t, "FAIL", check.Status)
	assert.Contains(t, check.Message, "parse error")
	assert.Contains(t, check.Message, "invalid mode")
	assert.Contains(t, check.Message, "worktree-isolation")
}

func TestCollectGovernanceHealthFailsWhenExecutionSummaryHasSummaryLevelBlockers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("health-summary-blockers")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	for _, file := range []string{"intent.md", "requirements.md", "decision.md", "tasks.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, file), []byte("# "+file+"\n"), 0o644))
	}

	const blocker = "session_isolation_warning:session_id=abc:shared_by=task-a,task-b"
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 2,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		OpenBlockers:      []model.ReasonCode{model.NewReasonCode("session_isolation_warning", "session_id=abc:shared_by=task-a,task-b")},
	}))

	snap, err := PreviewGovernanceSnapshot(root, change, bundleDir)
	require.NoError(t, err)

	report := CollectGovernanceHealthWithSnapshot(root, change, snap)
	assert.False(t, report.Healthy)

	found := false
	for _, check := range report.Checks {
		if check.Name != "signal_control_coherence" {
			continue
		}
		found = true
		assert.Equal(t, "FAIL", check.Status)
		assert.Contains(t, check.Message, blocker)
	}
	assert.True(t, found, "expected signal_control_coherence check")
}

func initGitRepoForGovernanceHealthTests(t *testing.T, root string) {
	t.Helper()
	runGitForGovernanceHealthTests(t, root, "init", "-b", "main")
	runGitForGovernanceHealthTests(t, root, "config", "user.email", "test@example.com")
	runGitForGovernanceHealthTests(t, root, "config", "user.name", "Slipway Test")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("health"), 0o644))
	runGitForGovernanceHealthTests(t, root, "add", ".")
	runGitForGovernanceHealthTests(t, root, "commit", "-m", "init")
}

func runGitForGovernanceHealthTests(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}

func TestCheckControlsConfigFailsOnCorruptConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write syntactically invalid YAML.
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("governance: [broken"), 0o644))

	check := checkControlsConfig(root, model.NewChange("corrupt-config"))
	assert.Equal(t, "FAIL", check.Status, "corrupt .slipway.yaml must produce FAIL, not OK")
	assert.Contains(t, check.Message, "parse error")
}

func TestCheckControlsConfigOKWhenMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// No .slipway.yaml at all.
	check := checkControlsConfig(root, model.NewChange("missing-config"))
	assert.Equal(t, "OK", check.Status, "missing .slipway.yaml is fine — no custom governance")
}

func TestCheckControlsConfigUsesCanonicalConfigForBoundWorkspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	worktreeRoot := t.TempDir()
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("governance: [broken"), 0o644))
	require.NoError(t, os.WriteFile(state.ConfigPath(worktreeRoot), []byte(`governance:
  controls:
    worktree-isolation: advisory
`), 0o644))

	change := model.NewChange("bound-controls-config")
	change.WorktreePath = worktreeRoot

	check := checkControlsConfig(root, change)
	assert.Equal(t, "FAIL", check.Status)
	assert.Contains(t, check.Message, ".slipway.yaml parse error")
}

func writePlanningTasksChecklist(t *testing.T, bundleDir, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(content), 0o644))
}

// TestTraceabilityCoherenceHealthIsStageAware proves that health.go threads the
// change's lifecycle state into traceability evaluation, so an incomplete
// per-requirement assurance coverage is a non-blocking WARN during S2 execution
// (issue #92) but still fails closed at S3 review. Uses the standard preset so
// the light-preset audit-gap downgrade does not mask the distinction.
func TestTraceabilityCoherenceHealthIsStageAware(t *testing.T) {
	t.Parallel()

	writeBundle := func(t *testing.T, includeAssurance bool) (root, slug, bundleDir string) {
		t.Helper()
		root = t.TempDir()
		slug = "stage-aware-health"
		bundleDir = filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.MkdirAll(bundleDir, 0o755))
		writeFile(t, filepath.Join(bundleDir, "intent.md"), `INT-001: Intent`)
		writeFile(t, resolveTestArtifact(bundleDir, slug), `# Requirements
### Requirement: Something
REQ-001: Something. INT-001
### Requirement: Something Else
REQ-002: Something else. INT-001
`)
		writeFile(t, filepath.Join(bundleDir, "tasks.md"), `# Tasks
- [ ] `+"`t-01`"+` first task
  covers: [REQ-001, REQ-002]
`)
		if includeAssurance {
			writeFile(t, filepath.Join(bundleDir, "assurance.md"), `# Assurance
## Requirement Coverage
REQ-001: verified via tests
`)
		}
		return root, slug, bundleDir
	}

	traceCheck := func(t *testing.T, report GovernanceHealthReport) GovernanceHealthCheck {
		t.Helper()
		for _, c := range report.Checks {
			if c.Name == "traceability_coherence" {
				return c
			}
		}
		t.Fatal("traceability_coherence check not found")
		return GovernanceHealthCheck{}
	}

	const assuranceIssue = "requirement missing assurance coverage verdict"
	const missingAssuranceIssue = "assurance.md missing at review/verify phase"

	t.Run("S2_EXECUTE reports WARN, not a blocking incident", func(t *testing.T) {
		t.Parallel()
		root, slug, bundleDir := writeBundle(t, true)
		change := model.Change{
			Slug:           slug,
			CurrentState:   model.StateS2Execute,
			WorkflowPreset: model.WorkflowPresetStandard,
			ArtifactSchema: model.ArtifactSchemaExpanded,
		}
		snap, err := PreviewGovernanceSnapshot(root, change, bundleDir)
		require.NoError(t, err)
		report := CollectGovernanceHealthWithSnapshot(root, change, snap)

		check := traceCheck(t, report)
		assert.Equal(t, "WARN", check.Status)
		// The gap is still reported, but as advisory — it must not be the thing
		// that drives governance unhealthy. Other bare-tempdir checks (e.g.
		// worktree_binding) may FAIL for reasons unrelated to #92, so assert the
		// assurance gap specifically does not produce a traceability FAIL rather
		// than asserting the whole report is healthy.
		assert.True(t, hasGapIssue(check.TraceabilityGaps, assuranceIssue), "assurance gap should still be reported")
		assert.False(t, hasBlockingGapIssue(check.TraceabilityGaps, assuranceIssue), "assurance gap must be non-blocking before review")
		for _, c := range report.Checks {
			if c.Status == "FAIL" {
				assert.NotEqual(t, "traceability_coherence", c.Name,
					"incomplete assurance coverage must not drive traceability FAIL at S2")
			}
		}
	})

	t.Run("S3_REVIEW fails closed", func(t *testing.T) {
		t.Parallel()
		root, slug, bundleDir := writeBundle(t, true)
		change := model.Change{
			Slug:           slug,
			CurrentState:   model.StateS3Review,
			WorkflowPreset: model.WorkflowPresetStandard,
			ArtifactSchema: model.ArtifactSchemaExpanded,
		}
		snap, err := PreviewGovernanceSnapshot(root, change, bundleDir)
		require.NoError(t, err)
		report := CollectGovernanceHealthWithSnapshot(root, change, snap)

		check := traceCheck(t, report)
		assert.Equal(t, "FAIL", check.Status)
		assert.True(t, hasBlockingGapIssue(check.TraceabilityGaps, assuranceIssue), "assurance gap must fail closed at review")
	})

	t.Run("S3_REVIEW missing assurance fails closed", func(t *testing.T) {
		t.Parallel()
		root, slug, bundleDir := writeBundle(t, false)
		change := model.Change{
			Slug:           slug,
			CurrentState:   model.StateS3Review,
			WorkflowPreset: model.WorkflowPresetStandard,
			ArtifactSchema: model.ArtifactSchemaExpanded,
		}
		snap, err := PreviewGovernanceSnapshot(root, change, bundleDir)
		require.NoError(t, err)
		report := CollectGovernanceHealthWithSnapshot(root, change, snap)

		check := traceCheck(t, report)
		assert.Equal(t, "FAIL", check.Status)
		assert.True(t, hasBlockingGapIssue(check.TraceabilityGaps, missingAssuranceIssue),
			"missing assurance.md must fail closed at review")
	})

	t.Run("DONE missing assurance fails closed", func(t *testing.T) {
		t.Parallel()
		root, slug, bundleDir := writeBundle(t, false)
		change := model.Change{
			Slug:           slug,
			CurrentState:   model.StateDone,
			WorkflowPreset: model.WorkflowPresetStandard,
			ArtifactSchema: model.ArtifactSchemaExpanded,
		}
		snap, err := PreviewGovernanceSnapshot(root, change, bundleDir)
		require.NoError(t, err)
		report := CollectGovernanceHealthWithSnapshot(root, change, snap)

		check := traceCheck(t, report)
		assert.Equal(t, "FAIL", check.Status)
		assert.True(t, hasBlockingGapIssue(check.TraceabilityGaps, missingAssuranceIssue),
			"missing assurance.md must fail closed at done")
	})

	t.Run("S3_REVIEW light preset keeps missing assurance optional", func(t *testing.T) {
		t.Parallel()
		root, slug, bundleDir := writeBundle(t, false)
		change := model.Change{
			Slug:           slug,
			CurrentState:   model.StateS3Review,
			WorkflowPreset: model.WorkflowPresetLight,
			ArtifactSchema: model.ArtifactSchemaExpanded,
		}
		snap, err := PreviewGovernanceSnapshot(root, change, bundleDir)
		require.NoError(t, err)
		report := CollectGovernanceHealthWithSnapshot(root, change, snap)

		check := traceCheck(t, report)
		assert.Equal(t, "OK", check.Status)
		assert.False(t, hasGapIssue(check.TraceabilityGaps, missingAssuranceIssue),
			"light preset keeps assurance.md optional")
	})
}
