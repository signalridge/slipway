package cmd

import (
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/stretchr/testify/require"
)

func TestInputlessCommandNextPreservesAbsoluteRoot(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "root with spaces")
	next := inputlessCommandNext(root, "list-runs", "slipway", "status", "--root", root)
	require.NoError(t, next.Validate())
	require.Equal(t, autopilot.NextOperationResume, next.Operation)
	require.Regexp(t, `^sha256:[0-9a-f]{64}$`, next.WorkspaceIdentity)
	require.NotEqual(t, root, next.WorkspaceIdentity)
	require.Equal(t, []string{"slipway", "status", "--root", root}, next.Variants[0].BaseArgv)
}
