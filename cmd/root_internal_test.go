package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandPrintsCanonicalScopeRoot(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, ".slipway.yaml"), []byte("defaults:\n  artifact_schema: expanded\n"), 0o644))

		scopeRoot := filepath.Join(root, "services", "billing")
		require.NoError(t, os.MkdirAll(scopeRoot, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(scopeRoot, ".slipway.yaml"), []byte("defaults:\n  artifact_schema: expanded\n"), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "slipway", "scopes", "services", "billing"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "slipway", "scope-root"), []byte("scope\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "slipway", "scopes", "services", "billing", "scope-root"), []byte("scope\n"), 0o644))

		nested := filepath.Join(scopeRoot, "pkg", "feature")
		require.NoError(t, os.MkdirAll(nested, 0o755))

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(nested))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		var out bytes.Buffer
		var errOut bytes.Buffer
		cmd := makeRootPathCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{})

		require.NoError(t, cmd.Execute())

		expected, err := filepath.EvalSymlinks(scopeRoot)
		require.NoError(t, err)
		assert.Equal(t, expected+"\n", out.String())
		assert.Empty(t, errOut.String())
	})
}

func TestRepairRootFromWDExplainsMissingRepairMarkers(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		ensureTestGitRepo(t, root)

		_, err := repairRootFromWD()
		require.Error(t, err)
		assert.ErrorIs(t, err, fsutil.ErrProjectRootNotFound)
		assert.Contains(t, err.Error(), "no slipway repair markers")
	})
}
