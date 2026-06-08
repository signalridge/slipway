package artifact

import (
	"os"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileFromFilesystemMaterializesRequiredArtifactsFromDisk(t *testing.T) {
	root := t.TempDir()
	slug := "test-change"
	require.NoError(t, ScaffoldGovernedBundleForChange(root, model.NewChange(slug), ""))

	// requirements.md is authored directly by the host skill, not scaffolded by
	// the engine (issue #119). Write it to disk to simulate that authoring so the
	// test still proves reconcile materializes an on-disk artifact's hash.
	bundleDir, err := state.GovernedBundleDir(root, model.Change{Slug: slug})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		ResolveArtifactPath(bundleDir, "requirements.md"),
		[]byte("# Requirements\n### Requirement: Authored on disk\nREQ-001: The system MUST do the thing.\n"),
		0o644,
	))

	change := &model.Change{
		Slug:      slug,
		Artifacts: map[string]model.ArtifactState{},
	}

	_, reconcileErr := ReconcileFromFilesystem(root, change)
	require.NoError(t, reconcileErr)

	req, ok := change.Artifacts["requirements"]
	require.True(t, ok)
	assert.Equal(t, model.ArtifactLifecycleDraft, req.State)
	assert.Equal(t, ResolveArtifactPath(bundleDir, "requirements.md"), req.Path)
	assert.NotEmpty(t, req.ContentHash)

	_, hasResearch := change.Artifacts["research"]
	assert.False(t, hasResearch)
}
