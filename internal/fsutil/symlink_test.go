package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathWithinIsSeparatorAware(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "root")
	require.NoError(t, os.MkdirAll(root, 0o755))
	assert.True(t, PathWithin(root, root))
	assert.True(t, PathWithin(root, filepath.Join(root, "child")))
	assert.False(t, PathWithin(root, root+"-sibling"))
}
