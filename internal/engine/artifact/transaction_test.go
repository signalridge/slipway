package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaffoldGovernedBundleTransactionOpsDeferWritesUntilApplied(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("transactional-scaffold")

	ops, err := ScaffoldGovernedBundleTransactionOpsForChange(root, change, model.WorkflowPresetStandard)
	require.NoError(t, err)
	require.NotEmpty(t, ops)

	intentPath := filepath.Join(root, "artifacts", "changes", change.Slug, "intent.md")
	_, err = os.Stat(intentPath)
	assert.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, fsutil.ApplyFileTransaction(ops))

	_, err = os.Stat(intentPath)
	require.NoError(t, err)
}

func TestScaffoldGovernedBundleTransactionOpsRejectDanglingSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("transactional-scaffold-symlink")
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.Symlink(filepath.Join(bundleDir, "missing-parent", "intent.md"), filepath.Join(bundleDir, "intent.md")))

	_, err := ScaffoldGovernedBundleTransactionOpsForChange(root, change, model.WorkflowPresetStandard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "refuse scaffold artifact symlink")
}
