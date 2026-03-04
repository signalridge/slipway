package toolgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryHasFourTools(t *testing.T) {
	registry := Registry()
	require.Len(t, registry, 4)

	ids := []string{registry[0].ID, registry[1].ID, registry[2].ID, registry[3].ID}
	assert.Equal(t, []string{"claude", "codex", "cursor", "opencode"}, ids)
}

func TestResolveTools(t *testing.T) {
	all, err := ResolveTools("all")
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "cursor", "codex", "opencode"}, all)

	none, err := ResolveTools("none")
	require.NoError(t, err)
	assert.Nil(t, none)

	selected, err := ResolveTools("cursor,claude,cursor")
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "cursor"}, selected)

	_, err = ResolveTools("unknown")
	require.Error(t, err)
}

func TestGenerateDeterministicAndRefresh(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Generate(root, []string{"claude"}, false))

	skillPath := filepath.Join(root, ".claude", "skills", "spln-new", "SKILL.md")
	commandPath := filepath.Join(root, ".claude", "commands", "spln-new.md")
	firstSkill, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	firstCommand, err := os.ReadFile(commandPath)
	require.NoError(t, err)

	// Non-refresh generation should keep existing files unchanged.
	require.NoError(t, os.WriteFile(skillPath, []byte("custom"), 0o644))
	require.NoError(t, Generate(root, []string{"claude"}, false))
	secondSkill, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Equal(t, "custom", string(secondSkill))

	// Refresh should deterministically regenerate content.
	require.NoError(t, Generate(root, []string{"claude"}, true))
	refreshedSkill, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	refreshedCommand, err := os.ReadFile(commandPath)
	require.NoError(t, err)
	assert.Equal(t, string(firstSkill), string(refreshedSkill))
	assert.Equal(t, string(firstCommand), string(refreshedCommand))
}
