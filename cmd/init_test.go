package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCommandToolsNone(t *testing.T) {
	root := t.TempDir()
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--tools", "none"})
	require.NoError(t, cmd.Execute())

	_, err = os.Stat(filepath.Join(root, ".spln", "config.yaml"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, ".claude"))
	assert.True(t, os.IsNotExist(err))
}

func TestInitCommandToolsAll(t *testing.T) {
	root := t.TempDir()
	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--tools", "all", "--refresh"})
	require.NoError(t, cmd.Execute())

	_, err = os.Stat(filepath.Join(root, ".claude", "skills", "spln-new", "SKILL.md"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, ".cursor", "commands", "spln-new.md"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, ".codex", "skills", "spln-goal-verification", "SKILL.md"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, ".opencode", "commands", "spln-review.md"))
	require.NoError(t, err)
}
