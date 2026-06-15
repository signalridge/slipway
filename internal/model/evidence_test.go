package model

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

// TestComputeFileContentHashIsCRLFInvariant verifies that text content differing
// only by CRLF vs LF line endings produces the same digest.
func TestComputeFileContentHashIsCRLFInvariant(t *testing.T) {
	t.Parallel()

	lfPath := writeTempFile(t, "lf.txt", []byte("alpha\nbeta\ngamma\n"))
	crlfPath := writeTempFile(t, "crlf.txt", []byte("alpha\r\nbeta\r\ngamma\r\n"))

	lfDigest, err := ComputeFileContentHash(lfPath)
	require.NoError(t, err)
	crlfDigest, err := ComputeFileContentHash(crlfPath)
	require.NoError(t, err)

	require.Equal(t, lfDigest, crlfDigest)
}

// TestComputeFileContentHashLeavesLFUnchanged verifies that for LF-only text
// content the digest equals the raw sha256 of the bytes (no behavior change).
func TestComputeFileContentHashLeavesLFUnchanged(t *testing.T) {
	t.Parallel()

	raw := []byte("alpha\nbeta\ngamma\n")
	path := writeTempFile(t, "lf.txt", raw)

	digest, err := ComputeFileContentHash(path)
	require.NoError(t, err)

	sum := sha256.Sum256(raw)
	require.Equal(t, hex.EncodeToString(sum[:]), digest)
}

// TestComputeFileContentHashBinaryIsByteExact verifies that binary content (with
// a NUL byte) is hashed byte-exact, so content differing only by CRLF vs LF
// produces different digests.
func TestComputeFileContentHashBinaryIsByteExact(t *testing.T) {
	t.Parallel()

	crlfPath := writeTempFile(t, "crlf.bin", []byte("alpha\x00\r\nbeta"))
	lfPath := writeTempFile(t, "lf.bin", []byte("alpha\x00\nbeta"))

	crlfDigest, err := ComputeFileContentHash(crlfPath)
	require.NoError(t, err)
	lfDigest, err := ComputeFileContentHash(lfPath)
	require.NoError(t, err)

	require.NotEqual(t, lfDigest, crlfDigest)
}

func TestGitAttributesKeepArtifactBinariesByteExact(t *testing.T) {
	t.Parallel()

	root := gitRepoRoot(t)
	attrs := gitCheckAttr(
		t,
		root,
		"artifacts/example.bin",
		"artifacts/example.mp4",
		"artifacts/example.dat",
		"artifacts/changes/example/intent.md",
	)

	require.Equal(t, "unset", attrs["artifacts/example.bin"]["text"])
	require.Equal(t, "unset", attrs["artifacts/example.mp4"]["text"])
	require.NotEqual(t, "set", attrs["artifacts/example.dat"]["text"],
		"unknown artifact extensions must use text=auto, not forced text normalization")
	require.Equal(t, "lf", attrs["artifacts/example.dat"]["eol"])

	require.Equal(t, "auto", attrs["artifacts/changes/example/intent.md"]["text"])
	require.Equal(t, "lf", attrs["artifacts/changes/example/intent.md"]["eol"])
}

func gitRepoRoot(t *testing.T) string {
	t.Helper()

	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output() // #nosec G204 -- test executes a fixed git command.
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

func gitCheckAttr(t *testing.T, root string, paths ...string) map[string]map[string]string {
	t.Helper()

	args := append([]string{"-C", root, "check-attr", "text", "eol", "--"}, paths...)
	out, err := exec.Command("git", args...).CombinedOutput() // #nosec G204 -- test executes a fixed git command with fixed attribute names and test-controlled paths.
	require.NoError(t, err, string(out))

	result := make(map[string]map[string]string, len(paths))
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, ": ", 3)
		require.Len(t, parts, 3, "unexpected git check-attr output line %q", line)
		if result[parts[0]] == nil {
			result[parts[0]] = map[string]string{}
		}
		result[parts[0]][parts[1]] = parts[2]
	}
	return result
}
