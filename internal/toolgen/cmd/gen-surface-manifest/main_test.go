package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/toolgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithOptionsWritesChecksAndPrintsSurfaceManifest(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	var stdout bytes.Buffer

	require.NoError(t, runWithOptions(repoRoot, manifestCommandOptions{
		write:  true,
		stdout: &stdout,
	}))
	assert.Contains(t, stdout.String(), "wrote docs/SURFACE-MANIFEST.json")

	manifestPath := filepath.Join(repoRoot, toolgen.SurfaceManifestPath)
	committed, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	live, err := toolgen.EncodeSurfaceManifest(toolgen.BuildSurfaceManifest())
	require.NoError(t, err)
	assert.Equal(t, string(live), string(committed))

	stdout.Reset()
	require.NoError(t, runWithOptions(repoRoot, manifestCommandOptions{
		check:  true,
		stdout: &stdout,
	}))
	assert.Contains(t, stdout.String(), "docs/SURFACE-MANIFEST.json is up to date")

	stdout.Reset()
	require.NoError(t, runWithOptions(repoRoot, manifestCommandOptions{
		stdout: &stdout,
	}))
	assert.JSONEq(t, string(live), stdout.String())
}

func TestRunWithOptionsCheckFailsOnStaleSurfaceManifest(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifestPath := filepath.Join(repoRoot, toolgen.SurfaceManifestPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	require.NoError(t, os.WriteFile(manifestPath, []byte(`{"version":1,"rows":[]}`+"\n"), 0o644))

	err := runWithOptions(repoRoot, manifestCommandOptions{check: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docs/SURFACE-MANIFEST.json is stale")
	assert.Contains(t, err.Error(), "go run ./internal/toolgen/cmd/gen-surface-manifest --write")
	assert.Contains(t, err.Error(), "+ command/new")
}

func TestRunWithOptionsRejectsCheckAndWriteTogether(t *testing.T) {
	t.Parallel()

	err := runWithOptions(t.TempDir(), manifestCommandOptions{
		check: true,
		write: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "choose only one")
}
