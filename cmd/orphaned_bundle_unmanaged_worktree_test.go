package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orphanRecoveryWorkspace creates a real git repo (so git worktree records are
// queryable by FindSlugWorktreeMatch), initializes the Slipway workspace, and
// chdirs into the root so cwd-based resolution matches production. Tests using it
// must not run in parallel because they chdir.
func orphanRecoveryWorkspace(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "commit", "--allow-empty", "-m", "init")

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(previousWD) })

	_, _, err = runRootCommandIn(root, []string{"init", "--tools", "none"})
	require.NoError(t, err)
	return root
}

// writeOrphanBundle materializes an orphan governed bundle for slug at
// artifacts/changes/<slug>: a directory that holds residue files but has lost its
// change.yaml authority. This is exactly what state.OrphanBundleSlugs detects.
func writeOrphanBundle(t *testing.T, root, slug string) {
	t.Helper()
	bundleDir := filepath.Join(state.ActiveBundlesDir(root), slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("orphaned residue\n"), 0o644))
	// No change.yaml is written: this is what makes the bundle an orphan.
	require.NoFileExists(t, filepath.Join(bundleDir, "change.yaml"))

	orphans, err := state.OrphanBundleSlugs(root)
	require.NoError(t, err)
	require.Contains(t, orphans, slug, "fixture must register %q as an orphan bundle", slug)
}

// addWorktreeOnBranch adds a real git worktree at path on a freshly-created
// branch, so FindSlugWorktreeMatch can read its (path, branch) record.
func addWorktreeOnBranch(t *testing.T, root, path, branch string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	runGit(t, root, "worktree", "add", "-b", branch, path)
}

// TestOrphanedChangeBundleErrorUnmanagedWorktreeRoute is the #285 core case: an
// orphan bundle whose slug also names a live worktree Slipway did NOT provision
// (default .worktrees/<slug> path but a hand-named branch, NOT feat/<slug>) must
// route to the non-destructive recovery, never to the destructive discard.
func TestOrphanedChangeBundleErrorUnmanagedWorktreeRoute(t *testing.T) {
	root := orphanRecoveryWorkspace(t)
	slug := "fix-283"
	externalBranch := "fix/issue-283-archived-worktree-resolution"

	writeOrphanBundle(t, root, slug)
	// Place an external worktree at the default convention path but on a branch
	// that is not feat/<slug>: SlipwayManaged must be false (path matches, branch
	// does not), so recovery must not recommend removing it.
	addWorktreeOnBranch(t, root, state.DefaultWorktreePath(root, slug), externalBranch)

	// Sanity: the match resolves as external (not managed) — the property the
	// #285 fix guarantees and this routing test depends on.
	match, ok, err := state.FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, match.SlipwayManaged, "external worktree on a hand-named branch must not be SlipwayManaged")

	cliErr := orphanedChangeBundleError(root, slug)
	require.NotNil(t, cliErr, "orphan bundle + unmanaged worktree must produce a recovery error")

	assert.Equal(t, "orphaned_bundle_unmanaged_worktree", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Equal(t, slug, cliErr.Slug)

	// Reason code carries the same code, scoped to the slug.
	require.NotEmpty(t, cliErr.Reasons)
	assert.Equal(t, "orphaned_bundle_unmanaged_worktree", string(cliErr.Reasons[0].Code))

	// Non-destructive remediation: it tells the operator to never pass --worktree
	// and must NOT suggest "add --worktree" (the destructive discard prose).
	assert.Contains(t, cliErr.Remediation, "never pass --worktree")
	assert.NotContains(t, cliErr.Remediation, "add --worktree")
	assert.NotContains(t, cliErr.Message, "add --worktree")

	// The unmanaged worktree path and branch are surfaced in details for triage.
	require.NotNil(t, cliErr.Details)
	assert.Equal(t, match.WorktreePath, cliErr.Details["unmanaged_worktree_path"])
	assert.Equal(t, externalBranch, cliErr.Details["unmanaged_worktree_branch"])
}

// TestOrphanedChangeBundleErrorManagedWorktreeRoute confirms a worktree Slipway
// DID provision (default path AND feat/<slug> branch) keeps the existing
// destructive-discard recovery: the discard path already owns it safely.
func TestOrphanedChangeBundleErrorManagedWorktreeRoute(t *testing.T) {
	root := orphanRecoveryWorkspace(t)
	slug := "managed-slug"

	writeOrphanBundle(t, root, slug)
	addWorktreeOnBranch(t, root, state.DefaultWorktreePath(root, slug), state.DefaultWorktreeBranch(slug))

	match, ok, err := state.FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, match.SlipwayManaged, "default path + feat/<slug> branch must be SlipwayManaged")

	cliErr := orphanedChangeBundleError(root, slug)
	require.NotNil(t, cliErr)

	assert.Equal(t, "orphaned_change_bundle", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	// The existing discard remediation MAY offer --worktree removal here because
	// Slipway owns this worktree.
	assert.Contains(t, cliErr.Remediation, "slipway delete --change "+slug)
	assert.Contains(t, cliErr.Remediation, "add --worktree")
}

// TestOrphanedChangeBundleErrorNoWorktreeRoute confirms an orphan bundle with no
// corresponding live worktree keeps the plain discard recovery.
func TestOrphanedChangeBundleErrorNoWorktreeRoute(t *testing.T) {
	root := orphanRecoveryWorkspace(t)
	slug := "lonely-slug"

	writeOrphanBundle(t, root, slug)

	// No worktree corresponds to the slug.
	_, ok, err := state.FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err)
	require.False(t, ok, "no worktree should correspond to the slug in this case")

	cliErr := orphanedChangeBundleError(root, slug)
	require.NotNil(t, cliErr)

	assert.Equal(t, "orphaned_change_bundle", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	require.NotNil(t, cliErr.Details)
	assert.Contains(t, cliErr.Details["orphaned_change_bundles"], slug)
	assert.Contains(t, cliErr.Remediation, "slipway delete --change "+slug)
}

// statusBlockerHasCode reports whether the status --json blockers list carries a
// reason with the given code. Blockers are encoded as model.ReasonCode objects,
// so each entry is a map with a "code" field.
func statusBlockerHasCode(t *testing.T, payload map[string]any, code string) bool {
	t.Helper()
	blockers, _ := payload["blockers"].([]any)
	for _, b := range blockers {
		if bm, ok := b.(map[string]any); ok && bm["code"] == code {
			return true
		}
	}
	return false
}

// assertNonDestructiveUnmanagedRecovery checks the status --json recovery object
// and full stdout for the #285 preserve-first contract: preserve_work class, no
// destructive `slipway delete` primary command, no "add --worktree" escalation,
// and the inspect/preserve prose present.
func assertNonDestructiveUnmanagedRecovery(t *testing.T, payload map[string]any, stdout string) {
	t.Helper()
	recovery, ok := payload["recovery"].(map[string]any)
	require.True(t, ok, "status must surface a recovery object")
	assert.Equal(t, "preserve_work", recovery["recovery_class"])

	// primary_command is omitempty, so it is absent (or empty) for preserve_work —
	// it must never route to the destructive discard command.
	primary, _ := recovery["primary_command"].(string)
	assert.Empty(t, primary, "preserve_work recovery must carry no primary_command")
	assert.NotContains(t, primary, "slipway delete")

	assert.NotContains(t, stdout, "add --worktree", "must never suggest the destructive --worktree escalation")
	assert.Contains(t, stdout, "never pass --worktree")
}

// TestStatusJSONUnmanagedWorktreeOrphanIsNonDestructive is the #285 status --json
// contract: an orphan bundle whose slug names a live worktree Slipway does NOT
// manage must surface a non-destructive preserve_work recovery, both unscoped and
// with an explicit --change selector. NOT parallel: the workspace helper chdirs.
func TestStatusJSONUnmanagedWorktreeOrphanIsNonDestructive(t *testing.T) {
	root := orphanRecoveryWorkspace(t)
	slug := "fix-283"
	externalBranch := "fix/issue-283-archived-worktree-resolution"

	writeOrphanBundle(t, root, slug)
	addWorktreeOnBranch(t, root, state.DefaultWorktreePath(root, slug), externalBranch)

	// Sanity: the match resolves as external (not managed).
	match, ok, err := state.FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, match.SlipwayManaged)

	// Unscoped status routes through the orphan diagnostics view.
	stdout, stderr, err := runRootCommandIn(root, []string{"status", "--json"})
	require.NoError(t, err, "status must not dead-end on an orphan bundle with a live unmanaged worktree")
	require.Empty(t, stderr)
	payload := decodeJSONMap(t, stdout)
	assert.Equal(t, "diagnostics", payload["execution_mode"])
	assert.True(t, statusBlockerHasCode(t, payload, "orphaned_bundle_unmanaged_worktree"),
		"blockers must include the orphaned_bundle_unmanaged_worktree reason")
	assertNonDestructiveUnmanagedRecovery(t, payload, stdout)

	// The same non-destructive recovery must hold with an explicit --change selector.
	stdout, stderr, err = runRootCommandIn(root, []string{"status", "--json", "--change", slug})
	require.NoError(t, err, "explicit status must not dead-end on the unmanaged-worktree orphan")
	require.Empty(t, stderr)
	payload = decodeJSONMap(t, stdout)
	assert.Equal(t, "diagnostics", payload["execution_mode"])
	assert.True(t, statusBlockerHasCode(t, payload, "orphaned_bundle_unmanaged_worktree"),
		"explicit blockers must include the orphaned_bundle_unmanaged_worktree reason")
	assertNonDestructiveUnmanagedRecovery(t, payload, stdout)
}
