package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSymlinkTargetWithinAcceptsTargetInsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	target := filepath.Join(root, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("target"), 0o644))

	resolved, info, err := ResolveSymlinkTargetWithin(root, filepath.Join(nested, "target.link"), "../target.txt")

	require.NoError(t, err)
	expected, err := filepath.EvalSymlinks(target)
	require.NoError(t, err)
	expected, err = filepath.Abs(expected)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(expected), resolved)
	assert.False(t, info.IsDir())
}

func TestResolveSymlinkTargetAcceptsExternalTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o644))

	resolved, info, err := ResolveSymlinkTarget(filepath.Join(root, "outside.link"), outside)

	require.NoError(t, err)
	expected, err := filepath.EvalSymlinks(outside)
	require.NoError(t, err)
	expected, err = filepath.Abs(expected)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(expected), resolved)
	assert.False(t, info.IsDir())
}

func TestResolveSymlinkTargetWithinRejectsTargetOutsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o644))

	_, _, err := ResolveSymlinkTargetWithin(root, filepath.Join(root, "outside.link"), outside)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSymlinkTargetOutsideRoot)
}

func TestResolveSymlinkTargetWithinReportsDanglingTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	_, _, err := ResolveSymlinkTargetWithin(root, filepath.Join(root, "dangling.link"), "missing.txt")

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrSymlinkTargetOutsideRoot))
}

func TestPathWithinIsSeparatorAware(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "root")
	require.NoError(t, os.MkdirAll(root, 0o755))
	assert.True(t, PathWithin(root, root))
	assert.True(t, PathWithin(root, filepath.Join(root, "child")))
	assert.False(t, PathWithin(root, root+"-sibling"))
}
