package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deleteTestGit runs a git subcommand in dir and fails the test on error.
func deleteTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, string(out))
}

// withDeleteWorkspace creates a real git repo (so worktree provisioning works),
// initializes a Slipway workspace, chdirs into it (so cwd-based resolution
// behaves as in production), and invokes fn with the workspace root. Tests using
// it must not run in parallel because they chdir.
func withDeleteWorkspace(t *testing.T, fn func(root string)) {
	t.Helper()
	root := t.TempDir()
	deleteTestGit(t, root, "init")
	deleteTestGit(t, root, "config", "user.email", "test@example.com")
	deleteTestGit(t, root, "config", "user.name", "Test")
	deleteTestGit(t, root, "commit", "--allow-empty", "-m", "init")

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	defer func() { _ = os.Chdir(previousWD) }()

	_, _, err = runRootCommandIn(root, []string{"init", "--tools", "none"})
	require.NoError(t, err)

	fn(root)
}

// newGovernedChangeForDelete creates a governed change (which provisions a
// dedicated worktree on the real git repo) and returns its slug and worktree
// path.
func newGovernedChangeForDelete(t *testing.T, root, description string) (slug, worktreePath string) {
	t.Helper()
	stdout, stderr, err := runRootCommandIn(root, []string{"new", "--json", description})
	require.NoError(t, err)
	require.Empty(t, stderr)
	payload := decodeJSONMap(t, stdout)
	slug, _ = payload["slug"].(string)
	require.NotEmpty(t, slug)
	worktreePath, _ = payload["worktree_path"].(string)
	require.Equal(t, true, payload["worktree_created"], "expected a worktree to be provisioned")
	return slug, worktreePath
}

func bundleDirForDelete(t *testing.T, worktreePath, slug string) string {
	t.Helper()
	return filepath.Join(worktreePath, "artifacts", "changes", slug)
}

func deleteTargetsByKind(t *testing.T, raw any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	list, _ := raw.([]any)
	for _, item := range list {
		target, ok := item.(map[string]any)
		require.True(t, ok)
		kind, _ := target["kind"].(string)
		out[kind] = target
	}
	return out
}

// REQ-002: a bare delete (no --yes) prints a plan and deletes nothing.
func TestDeleteDryRunDefaultDeletesNothing(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "accidental change")
		bundleDir := bundleDirForDelete(t, worktreePath, slug)
		require.DirExists(t, bundleDir)

		stdout, stderr, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		payload := decodeJSONMap(t, stdout)
		assert.Equal(t, true, payload["dry_run"])
		assert.NotEqual(t, true, payload["executed"])
		targets := deleteTargetsByKind(t, payload["plan"])
		require.Contains(t, targets, "governed_bundle")
		assert.Equal(t, "delete", targets["governed_bundle"]["action"])

		// Nothing was deleted.
		assert.DirExists(t, bundleDir)
	})
}

// REQ-002, REQ-003: delete --yes removes the bundle + runtime binding (worktree
// preserved by default) and status stays healthy afterward.
func TestDeleteExecutesAndStatusStaysHealthy(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "accidental change")
		bundleDir := bundleDirForDelete(t, worktreePath, slug)
		runtimeDir := state.ChangeDir(root, slug)
		require.DirExists(t, runtimeDir)

		stdout, stderr, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--yes", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		payload := decodeJSONMap(t, stdout)
		assert.Equal(t, true, payload["executed"])
		removed := deleteTargetsByKind(t, payload["removed"])
		assert.Contains(t, removed, "governed_bundle")
		assert.Contains(t, removed, "runtime_binding")
		// Worktree preserved (not requested).
		assert.NotContains(t, removed, "worktree")

		assert.NoDirExists(t, bundleDir)
		assert.NoDirExists(t, runtimeDir)
		assert.DirExists(t, worktreePath, "worktree preserved without --worktree")

		// status must be healthy (exit 0), not a broken active-change binding.
		stdout, stderr, err = runRootCommandIn(root, []string{"status", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		statusPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "diagnostics", statusPayload["execution_mode"])
	})
}

// REQ-006: a partially-deleted bundle (change.yaml removed, other files remain)
// must NOT dead-end status; status exits 0 and routes to `slipway delete`.
func TestStatusRoutesToDeleteAfterPartialDelete(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "accidental change")
		bundleDir := bundleDirForDelete(t, worktreePath, slug)
		// Simulate a partial manual delete: remove the change.yaml authority but
		// leave other bundle files behind.
		require.NoError(t, os.Remove(filepath.Join(bundleDir, "change.yaml")))
		require.FileExists(t, filepath.Join(bundleDir, "intent.md"))

		stdout, _, err := runRootCommandIn(root, []string{"status", "--json"})
		require.NoError(t, err, "status must not dead-end on a partially-deleted change")
		payload := decodeJSONMap(t, stdout)

		blockers, _ := payload["blockers"].([]any)
		foundOrphan := false
		for _, b := range blockers {
			if bm, ok := b.(map[string]any); ok && bm["code"] == "orphaned_change_bundle" {
				foundOrphan = true
			}
		}
		assert.True(t, foundOrphan, "expected orphaned_change_bundle blocker")

		recovery, ok := payload["recovery"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "slipway delete --change "+slug, recovery["primary_command"])

		// The routed command heals the repo.
		_, _, err = runRootCommandIn(root, []string{"delete", "--change", slug, "--yes"})
		require.NoError(t, err)
		assert.NoDirExists(t, bundleDir)

		stdout, _, err = runRootCommandIn(root, []string{"status", "--json"})
		require.NoError(t, err)
		healed := decodeJSONMap(t, stdout)
		assert.Equal(t, "diagnostics", healed["execution_mode"])
	})
}

// REQ-004, REQ-008: --worktree removes the worktree but never the branch.
func TestDeleteWorktreeRemovesWorktreePreservingBranch(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "accidental change")
		require.DirExists(t, worktreePath)
		branch := "feat/" + slug

		stdout, _, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--worktree", "--yes", "--json"})
		require.NoError(t, err)
		payload := decodeJSONMap(t, stdout)
		removed := deleteTargetsByKind(t, payload["removed"])
		assert.Contains(t, removed, "worktree")

		assert.NoDirExists(t, worktreePath)
		// git no longer registers the worktree.
		out, err := exec.Command("git", "-C", root, "worktree", "list").CombinedOutput()
		require.NoError(t, err)
		assert.NotContains(t, string(out), worktreePath)
		// The implementation branch is preserved.
		branchOut, err := exec.Command("git", "-C", root, "branch", "--list", branch).CombinedOutput()
		require.NoError(t, err)
		assert.Contains(t, string(branchOut), branch, "implementation branch must not be deleted")
	})
}

// REQ-004: --worktree refuses a worktree with uncommitted tracked changes unless
// --force.
func TestDeleteWorktreeRefusesDirtyUnlessForce(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "accidental change")
		// Introduce an uncommitted tracked change in the worktree.
		tracked := filepath.Join(worktreePath, "tracked.txt")
		require.NoError(t, os.WriteFile(tracked, []byte("base\n"), 0o644))
		deleteTestGit(t, worktreePath, "add", "tracked.txt")
		deleteTestGit(t, worktreePath, "commit", "-m", "add tracked file")
		require.NoError(t, os.WriteFile(tracked, []byte("dirty\n"), 0o644))

		_, stderr, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--worktree", "--yes", "--json"})
		require.Error(t, err)
		errPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "delete_refused", errPayload["error_code"])
		assert.DirExists(t, worktreePath, "worktree must survive a refusal")

		// --force overrides the dirty refusal.
		_, _, err = runRootCommandIn(root, []string{"delete", "--change", slug, "--worktree", "--force", "--yes", "--json"})
		require.NoError(t, err)
		assert.NoDirExists(t, worktreePath)
	})
}

// REQ-008 / safety: a worktree-removal request from inside that worktree is
// refused so the operator does not delete the checkout they are standing in.
func TestDeleteRefusesCurrentWorktree(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "accidental change")

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(worktreePath))
		defer func() { _ = os.Chdir(previousWD) }()

		_, stderr, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--worktree", "--yes", "--json"})
		require.Error(t, err)
		errPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "delete_refused", errPayload["error_code"])
		assert.DirExists(t, worktreePath)
	})
}

// REQ-005: --archived purges an archived terminal record and is dry-run-gated.
func TestDeleteArchivedRecord(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "to be archived")
		// Archive it via the public cancel flow.
		_, _, err := runRootCommandIn(root, []string{"cancel", "--change", slug, "--json"})
		require.NoError(t, err)
		archivedDir := filepath.Join(worktreePath, "artifacts", "changes", "archived", slug)
		require.DirExists(t, archivedDir)

		// Dry-run keeps the record.
		stdout, _, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--archived", "--json"})
		require.NoError(t, err)
		payload := decodeJSONMap(t, stdout)
		assert.Equal(t, "archived", payload["mode"])
		assert.Equal(t, true, payload["dry_run"])
		assert.DirExists(t, archivedDir)

		// --yes purges it.
		_, _, err = runRootCommandIn(root, []string{"delete", "--change", slug, "--archived", "--yes", "--json"})
		require.NoError(t, err)
		assert.NoDirExists(t, archivedDir)
	})
}

// REQ-007: a change bound to another worktree yields an actionable
// `slipway delete --change <slug>` command rather than a dead-end error.
func TestResolutionErrorNamesDeleteCommand(t *testing.T) {
	t.Parallel()
	boundErr := &state.ChangeBoundElsewhereError{
		BoundChanges: []state.BoundChangeRef{
			{Slug: "abandoned-change", WorktreePath: "/tmp/wt/abandoned-change"},
		},
	}
	err := wrapResolutionError(boundErr)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "change_bound_to_other_worktree", cliErr.ErrorCode)
	assert.Contains(t, cliErr.Remediation, "slipway delete --change abandoned-change")
}
