package cmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectRemediationSourcesSurfacesUnreadableBundleFile proves that an
// unreadable (permission-denied) regular file in the governed bundle causes the
// walk/read error to be surfaced instead of silently dropping remediation
// references.
//
// RED rationale: before the fix, detectRemediationSources discarded the
// filepath.WalkDir result and returned nil on per-file read errors, so this
// call returned no error and the reference in the unreadable file was silently
// omitted — require.Error would FAIL. After the fix, the per-file read error is
// returned from the walk callback and threaded out, so this call returns a
// non-nil error — require.Error PASSES.
func TestDetectRemediationSourcesSurfacesUnreadableBundleFile(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("chmod-based unreadable-file simulation is not portable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permission bits; cannot simulate an unreadable file")
	}

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "fix archived workflow feedback")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		paths, err := state.ResolveChangePaths(root, change)
		require.NoError(t, err)

		// A regular bundle file that carries a remediation reference the scan
		// would otherwise pick up, then made unreadable.
		unreadable := filepath.Join(paths.GovernedBundleDir, "remediation-source.md")
		require.NoError(t, os.WriteFile(
			unreadable,
			[]byte("Remediates artifacts/changes/archived/source-archived-workflow/workflow-feedback.md\n"),
			0o644,
		))
		require.NoError(t, os.Chmod(unreadable, 0o000))
		// Restore permissions so the temp-dir cleanup can remove the file.
		t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })

		refs, err := detectRemediationSources(root, change)
		require.Error(t, err, "unreadable bundle file must surface the read error, not be silently dropped")
		assert.ErrorIs(t, err, fs.ErrPermission)
		assert.Nil(t, refs)
	})
}

// TestDetectRemediationSourcesSkipsSymlinkWithoutError guards the behavior-
// preserving boundary: a symlinked bundle file is an intentional skip (matching
// ReadFileNoSymlink's refusal), not an unreadable-file failure, so the scan must
// still succeed with no error even though genuine read failures now surface.
func TestDetectRemediationSourcesSkipsSymlinkWithoutError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "fix archived workflow feedback")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		paths, err := state.ResolveChangePaths(root, change)
		require.NoError(t, err)

		external := filepath.Join(t.TempDir(), "outside.md")
		require.NoError(t, os.WriteFile(
			external,
			[]byte("Remediates artifacts/changes/archived/source-archived-workflow/workflow-feedback.md\n"),
			0o644,
		))
		link := filepath.Join(paths.GovernedBundleDir, "linked-source.md")
		if err := os.Symlink(external, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}

		_, err = detectRemediationSources(root, change)
		require.NoError(t, err)
	})
}
