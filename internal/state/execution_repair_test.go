package state

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepairExecutionStateReportsLegacySidecarPathResolutionFailure(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := saveActiveChangeForTest(t, root, "legacy-sidecar-path-error")
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, ChangeRuntimeStateFileName), []byte(`artifacts:
  intent:
    id: intent
    state: draft
`), 0o644))

	original := resolveChangePathsForRepair
	resolveChangePathsForRepair = func(root string, change model.Change) (ResolvedChangePaths, error) {
		return ResolvedChangePaths{}, fmt.Errorf("boom")
	}
	t.Cleanup(func() {
		resolveChangePathsForRepair = original
	})

	result, err := RepairExecutionState(root, time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC), 0)
	require.NoError(t, err)

	assert.Contains(
		t,
		result.NonRepairableFindings,
		"legacy-sidecar-path-error: legacy sidecar cleanup path resolution failed: boom",
	)
	assert.NotContains(t, result.MigratedLegacySidecars, change.Slug)
}
