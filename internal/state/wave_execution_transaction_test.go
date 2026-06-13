package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaterializeWavePlanTransactionOpDefersWriteUntilApplied(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, model.SaveConfig(ConfigPath(root), model.DefaultConfig()))

	change := model.NewChange("transactional-wave-plan")
	require.NoError(t, SaveChange(root, change))

	bundleDir, err := GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement transaction
  - target_files: ["internal/fsutil/transaction.go"]
  - task_kind: code
`), 0o644))

	generatedAt := time.Date(2026, 6, 11, 1, 2, 3, 0, time.UTC)
	plan, op, err := MaterializeWavePlanTransactionOpAt(root, change, generatedAt)
	require.NoError(t, err)
	assert.Equal(t, 1, plan.TotalTasks)

	wavePlanPath := filepath.Join(bundleDir, "verification", WavePlanFileName)
	_, err = os.Stat(wavePlanPath)
	assert.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, fsutil.ApplyFileTransaction([]fsutil.FileTransactionOp{op}))

	loaded, err := LoadWavePlanForChange(root, change)
	require.NoError(t, err)
	assert.Equal(t, generatedAt, loaded.GeneratedAt)
}
