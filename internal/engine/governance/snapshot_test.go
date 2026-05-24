package governance

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSnapshot() model.GovernanceSnapshot {
	now := time.Now().UTC()
	return model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{
			Domains:     []string{"auth_authz"},
			BlastRadius: model.SignalLevelMedium,
		},
		Observations: []model.SignalObservation{
			{
				ID:     "obs-1",
				Signal: model.SignalDomain,
				Level:  model.SignalLevelHigh,
				Source: "change.guardrail_domain",
				Reason: "guardrail domain auth_authz declared",
			},
		},
		Traceability: model.TraceabilitySummary{
			Status:  model.TraceabilityStatusOK,
			Message: "all REQ IDs traced",
		},
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
				Mode:         model.ControlModeAdvisory,
				Scope:        model.ControlScopeReview,
				Active:       true,
				TriggeredBy:  []string{"domain_present"},
				PolicySource: model.BuiltinPolicySource,
			},
			{
				ControlID:    model.ControlWorktreeIsolation,
				Mode:         model.ControlModeAdvisory,
				Scope:        model.ControlScopeExecution,
				Active:       true,
				TriggeredBy:  []string{"domain_present"},
				PolicySource: model.BuiltinPolicySource,
			},
		},
		ComputedAt: now,
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-change"

	// Create the change directory structure.
	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))

	snap := newTestSnapshot()

	// Save.
	require.NoError(t, SaveSnapshot(root, slug, snap))

	// Load.
	loaded, err := LoadSnapshot(root, slug)
	require.NoError(t, err)

	assert.Equal(t, snap.Version, loaded.Version)
	assert.Equal(t, snap.Summary.BlastRadius, loaded.Summary.BlastRadius)
	assert.Equal(t, snap.Summary.Domains, loaded.Summary.Domains)
	assert.Len(t, loaded.Observations, 1)
	assert.Equal(t, "obs-1", loaded.Observations[0].ID)
	assert.Equal(t, snap.Traceability.Status, loaded.Traceability.Status)
	assert.Len(t, loaded.ActiveControls, 3)
	assert.Equal(t, model.ControlDomainReview, loaded.ActiveControls[0].ControlID)
	assert.Equal(t, model.ControlIndependentReview, loaded.ActiveControls[1].ControlID)
	assert.Equal(t, model.ControlWorktreeIsolation, loaded.ActiveControls[2].ControlID)
	assert.True(t, loaded.ActiveControls[0].Active)
}

func TestSnapshotLoadMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	snap, err := LoadSnapshot(root, "nonexistent")
	require.NoError(t, err)
	assert.Equal(t, 0, snap.Version)
}

func TestSnapshotSkipUnchangedWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "skip-test"

	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))

	snap := newTestSnapshot()
	require.NoError(t, SaveSnapshot(root, slug, snap))

	// Get file stat.
	path := SnapshotPath(root, slug)
	info1, err := os.Stat(path)
	require.NoError(t, err)

	// Save the same snapshot again — should be skipped.
	require.NoError(t, SaveSnapshot(root, slug, snap))

	info2, err := os.Stat(path)
	require.NoError(t, err)

	// File should not have been rewritten.
	assert.Equal(t, info1.ModTime(), info2.ModTime())
}

func TestSnapshotRewriteOnMaterialChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "rewrite-test"

	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))

	snap := newTestSnapshot()
	require.NoError(t, SaveSnapshot(root, slug, snap))

	// Change the blast radius.
	snap.Summary.BlastRadius = model.SignalLevelHigh
	snap.ComputedAt = time.Now().UTC()
	require.NoError(t, SaveSnapshot(root, slug, snap))

	loaded, err := LoadSnapshot(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.SignalLevelHigh, loaded.Summary.BlastRadius)
}

func TestSnapshotRewriteOnTraceabilitySummaryChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "traceability-rewrite"

	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))

	snap := newTestSnapshot()
	require.NoError(t, SaveSnapshot(root, slug, snap))

	path := SnapshotPath(root, slug)
	info1, err := os.Stat(path)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	snap.Traceability.Message = "updated gap details"
	snap.ComputedAt = time.Now().UTC()
	require.NoError(t, SaveSnapshot(root, slug, snap))

	info2, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info2.ModTime().After(info1.ModTime()) || info2.ModTime().Equal(info1.ModTime()) == false)

	loaded, err := LoadSnapshot(root, slug)
	require.NoError(t, err)
	assert.Equal(t, "updated gap details", loaded.Traceability.Message)
}

func TestSnapshotRewriteOnFreshnessAndObservationChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "freshness-rewrite"

	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))

	snap := newTestSnapshot()
	require.NoError(t, SaveSnapshot(root, slug, snap))

	path := SnapshotPath(root, slug)
	info1, err := os.Stat(path)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	updatedAt := time.Now().UTC()
	snap.ComputedAt = updatedAt
	snap.Observations[0].Reason = "updated provenance"
	require.NoError(t, SaveSnapshot(root, slug, snap))

	info2, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info2.ModTime().After(info1.ModTime()) || !info2.ModTime().Equal(info1.ModTime()))

	loaded, err := LoadSnapshot(root, slug)
	require.NoError(t, err)
	assert.Equal(t, "updated provenance", loaded.Observations[0].Reason)
	assert.Equal(t, updatedAt, loaded.ComputedAt)
}

func TestSnapshotSkipRewriteOnVolatileFieldChangeOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "volatile-skip"

	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))

	snap := newTestSnapshot()
	require.NoError(t, SaveSnapshot(root, slug, snap))

	path := SnapshotPath(root, slug)
	info1, err := os.Stat(path)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	updated := snap
	updated.ComputedAt = time.Now().UTC()
	require.NoError(t, SaveSnapshot(root, slug, updated))

	info2, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, info1.ModTime(), info2.ModTime(), "ComputedAt-only change should not rewrite the snapshot")

	loaded, err := LoadSnapshot(root, slug)
	require.NoError(t, err)
	assert.Equal(t, snap.ComputedAt, loaded.ComputedAt)
}

func TestSnapshotValidation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "invalid-test"

	snap := model.GovernanceSnapshot{
		Version: 0, // invalid
	}
	err := SaveSnapshot(root, slug, snap)
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("version must equal %d", model.GovernanceSnapshotVersion))
}

func TestSnapshotLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "unknown-fields"

	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))
	require.NoError(t, os.WriteFile(SnapshotPath(root, slug), []byte(`
version: 2
summary:
  blast_radius: low
traceability:
  status: ok
computed_at: 2026-03-22T00:00:00Z
entry_surface: quick
`), 0o644))

	_, err := LoadSnapshot(root, slug)
	require.Error(t, err, "unknown fields must be rejected by KnownFields(true)")
}

func TestSnapshotRemove(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "remove-test"

	require.NoError(t, os.MkdirAll(filepath.Dir(SnapshotPath(root, slug)), 0o755))

	snap := newTestSnapshot()
	require.NoError(t, SaveSnapshot(root, slug, snap))
	_, err := os.Stat(SnapshotPath(root, slug))
	require.NoError(t, err)

	require.NoError(t, os.Remove(SnapshotPath(root, slug)))
	_, err = os.Stat(SnapshotPath(root, slug))
	assert.ErrorIs(t, err, os.ErrNotExist)

}

func TestSnapshotPath(t *testing.T) {
	t.Parallel()
	path := SnapshotPath("/project", "my-change")
	expected := filepath.Join(state.GitStateDir("/project"), "cache", "changes", "my-change", "governance_snapshot.yaml")
	assert.Equal(t, expected, path)
}
