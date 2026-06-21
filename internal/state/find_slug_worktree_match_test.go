package state

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sanity-anchor the slug correspondence the external-branch cases rely on:
// SlugifyTitle is the same transform FindSlugWorktreeMatch applies to a branch.
func TestFindSlugWorktreeMatch_SlugifyBranchAssumptions(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "demo", model.SlugifyTitle("demo"))
	assert.Equal(t, "my-feature", model.SlugifyTitle("my-feature"))
	assert.Equal(t, "fix-issue-283-archived-worktree-resolution",
		model.SlugifyTitle("fix/issue-283-archived-worktree-resolution"),
		"a slash-bearing branch slugifies to the dashed slug the custom-path case relies on")
	assert.Equal(t, "change", model.SlugifyTitle(""),
		"empty branch slugifies to \"change\" — the collision a detached HEAD must avoid")
}

// normalize resolves a path the same way FindSlugWorktreeMatch does, so
// expected paths survive macOS's /var -> /private/var symlink resolution.
func normalize(t *testing.T, path string) string {
	t.Helper()
	out, err := NormalizePath(path)
	require.NoError(t, err)
	return out
}

// addDefaultConventionWorktree adds a git worktree at <root>/.worktrees/<slug>
// on a new branch, mirroring the path convention DefaultWorktreePath uses. The
// branch is supplied explicitly so callers can exercise both the genuine
// feat/<slug> convention and an external hand-named branch at the same path.
func addDefaultConventionWorktree(t *testing.T, root, slug, branch string) string {
	t.Helper()
	path := filepath.Join(root, ".worktrees", slug)
	runGit(t, root, "worktree", "add", "-b", branch, path)
	return path
}

// addCustomWorktree adds a git worktree at a custom (non-.worktrees) path on a
// new branch, so the match can only come from branch/slug correspondence, never
// from the default path convention.
func addCustomWorktree(t *testing.T, root, dirName, branch string) string {
	t.Helper()
	path := filepath.Join(root, dirName)
	runGit(t, root, "worktree", "add", "-b", branch, path)
	return path
}

// addDetachedWorktree adds a detached-HEAD git worktree at a custom path. A
// detached worktree carries no branch, so it must never be matched via
// SlugifyTitle's empty-string "change" fallback.
func addDetachedWorktree(t *testing.T, root, dirName string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git rev-parse HEAD failed: %s", string(out))
	sha := strings.TrimSpace(string(out))
	path := filepath.Join(root, dirName)
	runGit(t, root, "worktree", "add", "--detach", path, sha)
	return path
}

// TestFindSlugWorktreeMatch_ExternalDefaultPathHandNamedBranchNotManaged is the
// headline regression for issue #285: an external worktree a user placed at the default
// .worktrees/<slug> path but on a hand-named branch (NOT feat/<slug>) must be
// reported as a real, corresponding match yet SlipwayManaged==false, so
// orphan-bundle recovery never recommends destroying the user's live work.
//
// Pre-fix, managed was pathMatches || branchMatches, so the default-path-only
// match was wrongly classed SlipwayManaged==true.
func TestFindSlugWorktreeMatch_ExternalDefaultPathHandNamedBranchNotManaged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initGitRepoAt(t, root)

	const slug = "fix-283"
	const branch = "fix/issue-283-archived-worktree-resolution"
	addDefaultConventionWorktree(t, root, slug, branch)

	match, ok, err := FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err)
	require.True(t, ok, "an external worktree at the default path must still correspond to the slug")
	assert.False(t, match.SlipwayManaged,
		"#285: default path alone (hand-named branch) is NOT positive proof Slipway provisioned it")
	assert.Equal(t, branch, match.Branch)
	assert.True(t, strings.HasSuffix(filepath.ToSlash(match.WorktreePath), ".worktrees/fix-283"),
		"match must point at the external worktree path, got %q", match.WorktreePath)
}

// TestFindSlugWorktreeMatch_GenuineManaged covers a worktree carrying BOTH the
// default .worktrees/<slug> path AND the feat/<slug> branch: positive proof
// Slipway provisioned it, so SlipwayManaged==true.
func TestFindSlugWorktreeMatch_GenuineManaged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initGitRepoAt(t, root)

	const slug = "demo"
	addDefaultConventionWorktree(t, root, slug, DefaultWorktreeBranch(slug))

	match, ok, err := FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, match.SlipwayManaged,
		"default path + feat/<slug> branch is positive proof Slipway provisioned the worktree")
	assert.Equal(t, DefaultWorktreeBranch(slug), match.Branch)
	// FindSlugWorktreeMatch returns NormalizePath-normalized paths (symlinks
	// resolved), so compare against the normalized default path.
	assert.Equal(t, normalize(t, DefaultWorktreePath(root, slug)), match.WorktreePath)
}

// TestFindSlugWorktreeMatch_ManagedPreferredOverExternal verifies that when both
// a Slipway-managed worktree and a coincidentally-corresponding external
// worktree exist for the same slug, the managed one is returned.
func TestFindSlugWorktreeMatch_ManagedPreferredOverExternal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initGitRepoAt(t, root)

	const slug = "demo"
	// External worktree first: custom path on a branch whose SlugifyTitle == slug.
	addCustomWorktree(t, root, "external-demo", "demo")
	// Genuine managed worktree: default path + feat/<slug> branch.
	managedPath := addDefaultConventionWorktree(t, root, slug, DefaultWorktreeBranch(slug))

	match, ok, err := FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, match.SlipwayManaged, "the Slipway-managed worktree must win over an external match")
	assert.Equal(t, DefaultWorktreeBranch(slug), match.Branch)
	assert.Equal(t, normalize(t, managedPath), match.WorktreePath)
}

// TestFindSlugWorktreeMatch_DetachedHeadDoesNotCollideWithChangeSlug guards the
// "change" slug edge: model.SlugifyTitle("") == "change", so before the fix a
// detached-HEAD worktree (empty branch) could be matched as the literal "change"
// slug. A detached worktree carries no branch and must never correspond.
func TestFindSlugWorktreeMatch_DetachedHeadDoesNotCollideWithChangeSlug(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initGitRepoAt(t, root)

	detachedPath := addDetachedWorktree(t, root, "detached-wt")

	match, ok, err := FindSlugWorktreeMatch(root, "change")
	require.NoError(t, err)
	assert.False(t, ok, "a detached-HEAD worktree must not match the literal \"change\" slug")
	assert.NotEqual(t, detachedPath, match.WorktreePath,
		"the detached worktree must never be returned for the \"change\" slug")
}

// TestFindSlugWorktreeMatch_ExternalSlugifiedBranchCustomPath covers an external
// worktree at a custom (non-default) path whose branch slugifies to the slug:
// it corresponds (ok==true) but is not managed (no default path proof). The
// branch carries a "/" so the raw branch != slug, and the worktree sits on a
// custom path != .worktrees/<slug> — so the ONLY thing that can make it
// correspond is the slugified-branch == slug rule (model.SlugifyTitle(branch)).
func TestFindSlugWorktreeMatch_ExternalSlugifiedBranchCustomPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initGitRepoAt(t, root)

	const slug = "fix-issue-283-archived-worktree-resolution"
	const branch = "fix/issue-283-archived-worktree-resolution"
	require.NotEqual(t, slug, branch, "branch must differ from slug so raw equality cannot match")
	require.Equal(t, slug, model.SlugifyTitle(branch), "only slugified-branch correspondence can match")

	customPath := addCustomWorktree(t, root, "elsewhere", branch)

	match, ok, err := FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err)
	require.True(t, ok, "a custom-path worktree whose branch slugifies to the slug must correspond")
	assert.False(t, match.SlipwayManaged, "a custom path is no proof Slipway provisioned the worktree")
	assert.Equal(t, branch, match.Branch)
	assert.Equal(t, normalize(t, customPath), match.WorktreePath)
}

// TestFindSlugWorktreeMatch_ExternalFeatBranchCustomPath covers branchMatches &&
// !pathMatches: a worktree on the EXACT feat/<slug> branch but at a CUSTOM path.
// SlugifyTitle("feat/<slug>") != slug, so ONLY the exact-branch rule makes it
// correspond; a custom path is no proof Slipway provisioned it -> not managed.
func TestFindSlugWorktreeMatch_ExternalFeatBranchCustomPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initGitRepoAt(t, root)

	const slug = "demo"
	branch := DefaultWorktreeBranch(slug) // feat/demo
	require.NotEqual(t, slug, model.SlugifyTitle(branch),
		"feat/<slug> must NOT slugify back to slug, so only branchMatches can match")

	customPath := addCustomWorktree(t, root, "feat-elsewhere", branch)

	match, ok, err := FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err)
	require.True(t, ok, "a custom-path worktree on the exact feat/<slug> branch must correspond")
	assert.False(t, match.SlipwayManaged, "a custom path is no proof Slipway provisioned the worktree")
	assert.Equal(t, branch, match.Branch)
	assert.Equal(t, normalize(t, customPath), match.WorktreePath)
}

// TestFindSlugWorktreeMatch_BranchSwitchInvalidatesCache is the regression for
// the worktree-list cache probe: an in-place branch switch (no worktree
// add/remove) rewrites .git/worktrees/<entry>/HEAD but leaves the entry set and
// the parent dir's mtime untouched. Before the probe fingerprinted each linked
// worktree's HEAD, a warm cache returned the STALE feat/<slug> branch, so the
// switched-away worktree still read SlipwayManaged==true. The match must instead
// drop SlipwayManaged and report the hand-named branch even with a warm cache.
func TestFindSlugWorktreeMatch_BranchSwitchInvalidatesCache(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initGitRepoAt(t, root)

	const slug = "cache-slug"
	path := addDefaultConventionWorktree(t, root, slug, DefaultWorktreeBranch(slug))

	// Warm the cache on the genuine managed worktree.
	m1, ok1, err1 := FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err1)
	require.True(t, ok1)
	require.True(t, m1.SlipwayManaged, "default path + feat/<slug> branch must be managed before the switch")

	// Switch the SAME worktree to a hand-named branch in place: no worktree entry
	// is added or removed, only HEAD is rewritten.
	runGit(t, path, "checkout", "-b", "hand/local-work")

	m2, ok2, err2 := FindSlugWorktreeMatch(root, slug)
	require.NoError(t, err2)
	require.True(t, ok2, "the worktree still sits at the default path, so it still corresponds")
	assert.False(t, m2.SlipwayManaged,
		"an in-place branch switch off feat/<slug> must drop SlipwayManaged even with a warm cache")
	assert.Equal(t, "hand/local-work", m2.Branch)
}

// TestFindSlugWorktreeMatch_NoMatch confirms a slug with no corresponding live
// worktree returns ok==false and a nil error.
func TestFindSlugWorktreeMatch_NoMatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initGitRepoAt(t, root)

	// Add an unrelated worktree so the listing is non-empty but non-matching.
	addCustomWorktree(t, root, "unrelated", "unrelated-branch")

	match, ok, err := FindSlugWorktreeMatch(root, "does-not-exist")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, SlugWorktreeMatch{}, match)
}
