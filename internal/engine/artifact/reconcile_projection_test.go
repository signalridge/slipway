package artifact

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileFromFilesystemMaterializesRequiredArtifactsFromDisk(t *testing.T) {
	root := t.TempDir()
	slug := "test-change"
	require.NoError(t, ScaffoldGovernedBundleForChangeWithContext(root, model.NewChange(slug), "", model.ProjectContext{}))

	change := &model.Change{
		Slug:      slug,
		Artifacts: map[string]model.ArtifactState{},
	}

	_, reconcileErr := ReconcileFromFilesystem(root, change)
	require.NoError(t, reconcileErr)

	req, ok := change.Artifacts["requirements"]
	require.True(t, ok)
	assert.Equal(t, model.ArtifactLifecycleDraft, req.State)
	bundleDir, err := state.GovernedBundleDir(root, *change)
	require.NoError(t, err)
	assert.Equal(t, ResolveArtifactPath(bundleDir, "requirements.md"), req.Path)
	assert.NotEmpty(t, req.ContentHash)

	_, hasResearch := change.Artifacts["research"]
	assert.False(t, hasResearch)
}
