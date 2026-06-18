package toolgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyOwnership(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pristinePath := filepath.Join(root, ".claude", "skills", "slipway-new", "SKILL.md")
	modifiedPath := filepath.Join(root, ".claude", "skills", "slipway-run", "SKILL.md")
	unknownPath := filepath.Join(root, ".claude", "skills", "user-owned", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(pristinePath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(modifiedPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(unknownPath), 0o755))
	require.NoError(t, os.WriteFile(pristinePath, []byte("generated pristine"), 0o644))
	require.NoError(t, os.WriteFile(modifiedPath, []byte("generated before edit"), 0o644))
	require.NoError(t, os.WriteFile(unknownPath, []byte("user content"), 0o644))

	manifest := ownershipManifest{
		Version: ownershipManifestVersion,
		ToolID:  "claude",
		Files: []ownershipManifestFile{
			{
				Path:   ".claude/skills/slipway-new/SKILL.md",
				SHA256: hashBytes([]byte("generated pristine")),
			},
			{
				Path:   ".claude/skills/slipway-run/SKILL.md",
				SHA256: hashBytes([]byte("generated before edit")),
			},
		},
	}
	require.NoError(t, os.WriteFile(modifiedPath, []byte("user edit"), 0o644))

	got, err := classifyOwnership(root, manifest, ".claude/skills/slipway-new/SKILL.md")
	require.NoError(t, err)
	assert.Equal(t, ownershipPristineManagedFile, got)

	got, err = classifyOwnership(root, manifest, ".claude/skills/slipway-run/SKILL.md")
	require.NoError(t, err)
	assert.Equal(t, ownershipManagedModifiedFile, got)

	got, err = classifyOwnership(root, manifest, ".claude/skills/user-owned/SKILL.md")
	require.NoError(t, err)
	assert.Equal(t, ownershipUnknownUserFile, got)
}
