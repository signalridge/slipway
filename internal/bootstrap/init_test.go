package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitWorkspaceCreatesRuntimeMinimalLayout(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, InitWorkspace(root, nil, false))

	required := []string{
		".spln/config.yaml",
		".spln/runtime/admissions",
		".spln/runtime/changes",
		".spln/archive/admissions",
		".spln/archive/changes",
		".spln/archive/config",
		".spln/evidence/skills",
		".spln/evidence/tasks",
		".spln/evidence/runs",
		"aircraft/changes",
		"aircraft/changes/archived",
	}
	for _, rel := range required {
		_, err := os.Stat(filepath.Join(root, rel))
		require.NoErrorf(t, err, "missing %s", rel)
	}

	_, err := os.Stat(filepath.Join(root, ".spln", "README.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestInitWorkspaceToolsAllAndNone(t *testing.T) {
	rootAll := t.TempDir()
	require.NoError(t, InitWorkspace(rootAll, []string{"claude", "cursor", "codex", "opencode"}, false))
	_, err := os.Stat(filepath.Join(rootAll, ".claude", "skills", "spln-new", "SKILL.md"))
	require.NoError(t, err)

	rootNone := t.TempDir()
	require.NoError(t, InitWorkspace(rootNone, nil, false))
	_, err = os.Stat(filepath.Join(rootNone, ".claude"))
	assert.True(t, os.IsNotExist(err))
}
