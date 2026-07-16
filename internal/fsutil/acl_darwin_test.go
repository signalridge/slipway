//go:build darwin

package fsutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestrictToOwnerRemovesInheritedDarwinACL(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "inheriting")
	require.NoError(t, os.Mkdir(parent, 0o700))
	output, err := exec.Command(
		"/bin/chmod",
		"+a",
		"everyone allow read,file_inherit,directory_inherit",
		parent,
	).CombinedOutput()
	require.NoError(t, err, string(output))

	path := filepath.Join(parent, "private")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	require.Error(t, verifyDarwinExtendedACLAbsent(file), "fixture must inherit an ACL entry")

	require.NoError(t, RestrictToOwner(file))
	info, err := file.Stat()
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assert.NoError(t, verifyDarwinExtendedACLAbsent(file))
}
