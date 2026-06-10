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

func partiallyDeleteBundleForDelete(t *testing.T, worktreePath, slug string) string {
	t.Helper()
	bundleDir := bundleDirForDelete(t, worktreePath, slug)
	require.NoError(t, os.Remove(filepath.Join(bundleDir, "change.yaml")))
	require.FileExists(t, filepath.Join(bundleDir, "intent.md"))
	return bundleDir
}

func fullyDeleteBundleForDelete(t *testing.T, worktreePath, slug string) string {
	t.Helper()
	bundleDir := bundleDirForDelete(t, worktreePath, slug)
	require.NoError(t, os.RemoveAll(bundleDir))
	return bundleDir
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

func assertOrphanStatusDeleteRecovery(t *testing.T, payload map[string]any, slug string) {
	t.Helper()
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
}

func assertOrphanErrorDeleteRecovery(t *testing.T, payload map[string]any, slug string) {
	t.Helper()
	assert.Equal(t, "orphaned_change_bundle", payload["error_code"])
	assert.Equal(t, slug, payload["slug"])
	details, ok := payload["details"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, details["orphaned_change_bundles"], slug)
	assert.Contains(t, payload["remediation"], "slipway delete --change "+slug)

	recovery, ok := payload["recovery"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "slipway delete --change "+slug, recovery["primary_command"])
}

func assertStaleRuntimeStatusDeleteRecovery(t *testing.T, payload map[string]any, slug string) {
	t.Helper()
	blockers, _ := payload["blockers"].([]any)
	foundStale := false
	for _, b := range blockers {
		if bm, ok := b.(map[string]any); ok && bm["code"] == "stale_runtime_binding" {
			foundStale = true
		}
	}
	assert.True(t, foundStale, "expected stale_runtime_binding blocker")

	recovery, ok := payload["recovery"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "slipway delete --change "+slug, recovery["primary_command"])
}

func assertStaleRuntimeErrorDeleteRecovery(t *testing.T, payload map[string]any, slug string) {
	t.Helper()
	assert.Equal(t, "stale_runtime_binding", payload["error_code"])
	assert.Equal(t, slug, payload["slug"])
	details, ok := payload["details"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, details["stale_runtime_bindings"], slug)
	assert.Contains(t, payload["remediation"], "slipway delete --change "+slug)

	recovery, ok := payload["recovery"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "slipway delete --change "+slug, recovery["primary_command"])
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

// Safety: --change is a slug, not a path. Reject traversal before locks or
// deletion paths are derived from it.
func TestDeleteRejectsTraversalSlug(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		victimDir := filepath.Join(root, "victim")
		require.NoError(t, os.MkdirAll(victimDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(victimDir, "keep.txt"), []byte("sentinel\n"), 0o644))

		_, stderr, err := runRootCommandIn(root, []string{"delete", "--change", "../../victim", "--yes", "--json"})
		require.Error(t, err)
		errPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "invalid_change_slug", errPayload["error_code"])
		assert.DirExists(t, victimDir, "invalid slug must not delete outside artifacts/changes")
		assert.NoFileExists(t, filepath.Join(state.GitStateDir(root), "victim.lock"), "invalid slug must be rejected before per-change lock creation")
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
		// Simulate a partial manual delete: remove the change.yaml authority but
		// leave other bundle files behind.
		bundleDir := partiallyDeleteBundleForDelete(t, worktreePath, slug)

		stdout, _, err := runRootCommandIn(root, []string{"status", "--json"})
		require.NoError(t, err, "status must not dead-end on a partially-deleted change")
		payload := decodeJSONMap(t, stdout)
		assertOrphanStatusDeleteRecovery(t, payload, slug)

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

func TestExplicitStatusRoutesToDeleteAfterPartialDelete(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "accidental change")
		partiallyDeleteBundleForDelete(t, worktreePath, slug)

		stdout, stderr, err := runRootCommandIn(root, []string{"status", "--json", "--change", slug})
		require.NoError(t, err, "explicit status must not dead-end on a partially-deleted change")
		assert.Empty(t, stderr)
		payload := decodeJSONMap(t, stdout)
		assert.Equal(t, "diagnostics", payload["execution_mode"])
		assertOrphanStatusDeleteRecovery(t, payload, slug)
	})
}

func TestNextRoutesToDeleteAfterPartialDelete(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "accidental change")
		partiallyDeleteBundleForDelete(t, worktreePath, slug)

		_, stderr, err := runRootCommandIn(root, []string{"next", "--json"})
		require.Error(t, err, "next must route partially-deleted changes to delete recovery")
		payload := decodeJSONMap(t, stderr)
		assertOrphanErrorDeleteRecovery(t, payload, slug)
	})
}

func TestExplicitValidateRoutesToDeleteAfterPartialDelete(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "accidental change")
		partiallyDeleteBundleForDelete(t, worktreePath, slug)

		_, stderr, err := runRootCommandIn(root, []string{"validate", "--json", "--change", slug})
		require.Error(t, err, "explicit validate must route partially-deleted changes to delete recovery")
		payload := decodeJSONMap(t, stderr)
		assertOrphanErrorDeleteRecovery(t, payload, slug)
	})
}

// REQ-003: deleting the entire active bundle but leaving the runtime binding
// must still route through the public delete surface, not private .git/slipway
// inspection.
func TestStatusRoutesToDeleteAfterFullBundleDelete(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "stale runtime binding")
		bundleDir := fullyDeleteBundleForDelete(t, worktreePath, slug)
		runtimeDir := state.ChangeDir(root, slug)
		require.DirExists(t, runtimeDir)

		stdout, stderr, err := runRootCommandIn(root, []string{"status", "--json"})
		require.NoError(t, err, "status must route a stale runtime binding to delete recovery")
		assert.Empty(t, stderr)
		payload := decodeJSONMap(t, stdout)
		assert.Equal(t, "diagnostics", payload["execution_mode"])
		assertStaleRuntimeStatusDeleteRecovery(t, payload, slug)

		_, _, err = runRootCommandIn(root, []string{"delete", "--change", slug, "--yes"})
		require.NoError(t, err)
		assert.NoDirExists(t, bundleDir)
		assert.NoDirExists(t, runtimeDir)
	})
}

func TestExplicitStatusRoutesToDeleteAfterFullBundleDelete(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "stale runtime binding")
		fullyDeleteBundleForDelete(t, worktreePath, slug)

		stdout, stderr, err := runRootCommandIn(root, []string{"status", "--json", "--change", slug})
		require.NoError(t, err, "explicit status must route a stale runtime binding to delete recovery")
		assert.Empty(t, stderr)
		payload := decodeJSONMap(t, stdout)
		assert.Equal(t, "diagnostics", payload["execution_mode"])
		assertStaleRuntimeStatusDeleteRecovery(t, payload, slug)
	})
}

func TestNextRoutesToDeleteAfterFullBundleDelete(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "stale runtime binding")
		fullyDeleteBundleForDelete(t, worktreePath, slug)

		_, stderr, err := runRootCommandIn(root, []string{"next", "--json"})
		require.Error(t, err, "next must route a stale runtime binding to delete recovery")
		payload := decodeJSONMap(t, stderr)
		assertStaleRuntimeErrorDeleteRecovery(t, payload, slug)
	})
}

func TestValidateRoutesToDeleteAfterFullBundleDelete(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "stale runtime binding")
		fullyDeleteBundleForDelete(t, worktreePath, slug)

		_, stderr, err := runRootCommandIn(root, []string{"validate", "--json"})
		require.Error(t, err, "validate must route a stale runtime binding to delete recovery")
		payload := decodeJSONMap(t, stderr)
		assertStaleRuntimeErrorDeleteRecovery(t, payload, slug)
	})
}

func TestExplicitValidateRoutesToDeleteAfterFullBundleDelete(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "stale runtime binding")
		fullyDeleteBundleForDelete(t, worktreePath, slug)

		_, stderr, err := runRootCommandIn(root, []string{"validate", "--json", "--change", slug})
		require.Error(t, err, "explicit validate must route a stale runtime binding to delete recovery")
		payload := decodeJSONMap(t, stderr)
		assertStaleRuntimeErrorDeleteRecovery(t, payload, slug)
	})
}

func TestStatusReportsStaleRuntimeBindingWithAnotherActiveChange(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		staleSlug, staleWorktreePath := newGovernedChangeForDelete(t, root, "stale runtime binding")
		fullyDeleteBundleForDelete(t, staleWorktreePath, staleSlug)
		activeSlug, _ := newGovernedChangeForDelete(t, root, "still active")

		stdout, stderr, err := runRootCommandIn(root, []string{"status", "--json"})
		require.NoError(t, err, "status must surface stale bindings even when another active change exists")
		assert.Empty(t, stderr)
		payload := decodeJSONMap(t, stdout)
		assert.Equal(t, "diagnostics", payload["execution_mode"])
		assert.NotEqual(t, activeSlug, payload["slug"])
		assertStaleRuntimeStatusDeleteRecovery(t, payload, staleSlug)
	})
}

func TestDeleteUnregistersAlreadyMissingBoundWorktreeAndRemovesRuntime(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "removed worktree binding")
		runtimeDir := state.ChangeDir(root, slug)
		require.DirExists(t, runtimeDir)

		require.NoError(t, os.RemoveAll(worktreePath))
		require.NoDirExists(t, worktreePath)

		stdout, stderr, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--worktree", "--yes", "--json"})
		require.NoError(t, err, "missing bound worktree should not block runtime cleanup")
		assert.Empty(t, stderr)
		payload := decodeJSONMap(t, stdout)
		removed := deleteTargetsByKind(t, payload["removed"])
		assert.Contains(t, removed, "runtime_binding")
		require.Contains(t, removed, "worktree")
		assert.Equal(t, "delete", removed["worktree"]["action"])
		assert.Contains(t, removed["worktree"]["reason"], "metadata will be removed")
		skipped := deleteTargetsByKind(t, payload["skipped"])
		assert.NotContains(t, skipped, "worktree")
		assert.NoDirExists(t, runtimeDir)

		worktrees, err := exec.Command("git", "-C", root, "worktree", "list", "--porcelain").CombinedOutput()
		require.NoError(t, err)
		assert.NotContains(t, string(worktrees), worktreePath, "missing worktree metadata must be unregistered")

		stdout, stderr, err = runRootCommandIn(root, []string{"status", "--json"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		statusPayload := decodeJSONMap(t, stdout)
		assert.Equal(t, "diagnostics", statusPayload["execution_mode"])
	})
}

func TestDeleteNothingToDeleteTextDoesNotSayDeleted(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		stdout, stderr, err := runRootCommandIn(root, []string{"delete", "--change", "already-deleted", "--yes"})
		require.NoError(t, err)
		assert.Empty(t, stderr)
		assert.Contains(t, stdout, "Nothing to delete for already-deleted")
		assert.NotContains(t, stdout, "Deleted already-deleted")
		assert.Contains(t, stdout, "Skipped:")
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

// REQ-004 / safety: --worktree must not silently drop arbitrary untracked work
// unless the operator explicitly passes --force.
func TestDeleteWorktreeRefusesUnsafeUntrackedUnlessForce(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "accidental change")
		untracked := filepath.Join(worktreePath, "untracked-draft.txt")
		require.NoError(t, os.WriteFile(untracked, []byte("draft\n"), 0o644))

		_, stderr, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--worktree", "--yes", "--json"})
		require.Error(t, err)
		errPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "delete_refused", errPayload["error_code"])
		assert.Contains(t, stderr, "untracked-draft.txt")
		assert.DirExists(t, worktreePath, "worktree must survive an untracked-file refusal")

		// --force overrides the untracked-file refusal.
		_, _, err = runRootCommandIn(root, []string{"delete", "--change", slug, "--worktree", "--force", "--yes", "--json"})
		require.NoError(t, err)
		assert.NoDirExists(t, worktreePath)
	})
}

func TestDeleteWorktreeRefusesUntrackedCodebaseMapUnlessForce(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "codebase map cleanup")
		codebaseMap := filepath.Join(worktreePath, "artifacts", "codebase", "ARCHITECTURE.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(codebaseMap), 0o755))
		require.NoError(t, os.WriteFile(codebaseMap, []byte("valuable local context\n"), 0o644))

		_, stderr, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--worktree", "--yes", "--json"})
		require.Error(t, err)
		errPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "delete_refused", errPayload["error_code"])
		assert.Contains(t, stderr, "artifacts/codebase/ARCHITECTURE.md")
		assert.DirExists(t, worktreePath, "worktree must survive an untracked codebase-map refusal")

		_, _, err = runRootCommandIn(root, []string{"delete", "--change", slug, "--worktree", "--force", "--yes", "--json"})
		require.NoError(t, err)
		assert.NoDirExists(t, worktreePath)
	})
}

func TestDeleteWorktreeRefusesIgnoredUntrackedUnlessForce(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		gitignorePath := filepath.Join(root, ".gitignore")
		raw, err := os.ReadFile(gitignorePath)
		if err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		raw = append(raw, []byte("\nsecret.env\n")...)
		require.NoError(t, os.WriteFile(gitignorePath, raw, 0o644))
		deleteTestGit(t, root, "add", ".gitignore", ".slipway.yaml")
		deleteTestGit(t, root, "commit", "-m", "track slipway config and ignored local files")

		slug, worktreePath := newGovernedChangeForDelete(t, root, "ignored untracked cleanup")
		ignored := filepath.Join(worktreePath, "secret.env")
		require.NoError(t, os.WriteFile(ignored, []byte("local secret\n"), 0o644))

		_, stderr, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--worktree", "--yes", "--json"})
		require.Error(t, err)
		errPayload := decodeJSONMap(t, stderr)
		assert.Equal(t, "delete_refused", errPayload["error_code"])
		assert.Contains(t, stderr, "secret.env")
		assert.DirExists(t, worktreePath, "worktree must survive an ignored-file refusal")

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

func TestDeleteArchivedReportsIgnoredWorktreeFlags(t *testing.T) {
	withDeleteWorkspace(t, func(root string) {
		slug, worktreePath := newGovernedChangeForDelete(t, root, "to be archived")
		_, _, err := runRootCommandIn(root, []string{"cancel", "--change", slug, "--json"})
		require.NoError(t, err)
		archivedDir := filepath.Join(worktreePath, "artifacts", "changes", "archived", slug)
		require.DirExists(t, archivedDir)

		stdout, _, err := runRootCommandIn(root, []string{"delete", "--change", slug, "--archived", "--worktree", "--force", "--json"})
		require.NoError(t, err)
		payload := decodeJSONMap(t, stdout)
		targets := deleteTargetsByKind(t, payload["plan"])
		require.Contains(t, targets, "worktree")
		assert.Equal(t, "skip", targets["worktree"]["action"])
		assert.Contains(t, targets["worktree"]["reason"], "--archived only purges archived records")
		assert.Contains(t, targets["worktree"]["reason"], "--worktree/--force")
		assert.DirExists(t, archivedDir)
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

func TestDeleteHelpDoesNotRenderYesAsTakingValue(t *testing.T) {
	t.Parallel()

	stdout, _, err := runRootCommand([]string{"delete", "--help"})
	require.NoError(t, err)
	assert.Contains(t, stdout, "--yes")
	assert.NotContains(t, stdout, "--yes delete")
}
