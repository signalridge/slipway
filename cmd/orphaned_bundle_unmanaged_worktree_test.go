package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyOrphanBundlesFailsClosedOnMatcherError proves the injectable
// classifier never lands a cross-check failure in the discardable Plain bucket:
// ownership it cannot verify must fail closed into Unknown (#285).
func TestClassifyOrphanBundlesFailsClosedOnMatcherError(t *testing.T) {
	failing := func(_, slug string) (state.SlugWorktreeMatch, bool, error) {
		return state.SlugWorktreeMatch{}, false, errors.New("git worktree list failed")
	}
	class := classifyOrphanBundlesWith("/does/not/matter", []string{"mystery-slug"}, failing)
	require.Empty(t, class.Plain, "a cross-check error must NOT land in the discardable Plain bucket")
	require.Empty(t, class.Unmanaged)
	require.Len(t, class.Unknown, 1)
	assert.Equal(t, "mystery-slug", class.Unknown[0].Slug)
}

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

func writeArchivedChangeWithActiveResidue(t *testing.T, root, slug string) {
	t.Helper()
	change := model.NewChange(slug)
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	require.NoError(t, state.SaveChange(root, change))
	_, err := state.ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	writeOrphanBundle(t, root, slug)
	require.FileExists(t, state.BundleArchivedChangeFilePath(root, slug))
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

	// A single-unmanaged case must NOT also fold in the plain discard reason or
	// details: there is no discardable residue here, only the preserved worktree.
	for _, r := range cliErr.Reasons {
		assert.NotEqual(t, "orphaned_change_bundle", string(r.Code),
			"a single unmanaged orphan must not carry the plain discard reason")
	}
	assert.Nil(t, cliErr.Details["orphaned_change_bundles"],
		"a single unmanaged orphan must not carry plain discard details")
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

func TestOrphanedChangeBundleErrorArchivedSameSlugTargetsActiveResidue(t *testing.T) {
	root := orphanRecoveryWorkspace(t)
	slug := "archived-with-active-residue"

	writeArchivedChangeWithActiveResidue(t, root, slug)

	cliErr := orphanedChangeBundleError(root, slug)
	require.NotNil(t, cliErr)
	assert.Equal(t, "orphaned_active_residue_archived_change", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Equal(t, slug, cliErr.Slug)
	assert.Contains(t, cliErr.Message, "active-state residue")
	assert.Contains(t, cliErr.Remediation, "active-state residue")
	assert.Contains(t, cliErr.Remediation, "archived record")
	assert.Contains(t, cliErr.Remediation, "source commits are not deletion targets")
	assert.Contains(t, cliErr.Remediation, "slipway delete --change "+slug)
	assert.NotContains(t, cliErr.Remediation, "--worktree")

	require.NotEmpty(t, cliErr.Reasons)
	assert.Equal(t, "orphaned_change_bundle", string(cliErr.Reasons[0].Code))
	assert.Contains(t, cliErr.Reasons[0].Message, "Active-state residue")
	require.NotNil(t, cliErr.Details)
	assert.Equal(t, filepath.ToSlash(filepath.Join("artifacts", "changes", "archived", slug, "change.yaml")), cliErr.Details["archive_path"])
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

	// The single-unmanaged status case carries no plain discard blocker, and no
	// recovery step may route to the destructive discard.
	assert.False(t, statusBlockerHasCode(t, payload, "orphaned_change_bundle"),
		"a single unmanaged orphan must not surface the plain discard blocker")
	steps, _ := recovery["steps"].([]any)
	for _, s := range steps {
		sm, ok := s.(map[string]any)
		if !ok {
			continue
		}
		assert.NotEqual(t, "discard_change", sm["recovery_class"],
			"no recovery step may be classed discard_change for a preserved worktree")
		cmd, _ := sm["command"].(string)
		assert.NotContains(t, cmd, "slipway delete",
			"no recovery step command may route to the destructive discard")
	}
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

func TestStatusJSONArchivedSameSlugOrphanTargetsActiveResidue(t *testing.T) {
	root := orphanRecoveryWorkspace(t)
	slug := "archived-with-active-residue"

	writeArchivedChangeWithActiveResidue(t, root, slug)

	stdout, stderr, err := runRootCommandIn(root, []string{"status", "--json", "--change", slug})
	require.NoError(t, err)
	require.Empty(t, stderr)
	payload := decodeJSONMap(t, stdout)
	assert.Equal(t, "diagnostics", payload["execution_mode"])
	assert.Nil(t, payload["archived"], "active residue recovery must not render the archived record as the target")
	assert.True(t, statusBlockerHasCode(t, payload, "orphaned_change_bundle"))

	recovery, ok := payload["recovery"].(map[string]any)
	require.True(t, ok, "status must surface a recovery object")
	assert.Equal(t, "discard_change", recovery["recovery_class"])
	assert.Equal(t, "slipway delete --change "+slug, recovery["primary_command"])
	assert.Contains(t, stdout, "active-state residue")
	assert.Contains(t, stdout, "archived record")
	assert.Contains(t, stdout, "source commits are not deletion targets")
	assert.NotContains(t, stdout, "--worktree")
}

// TestOrphanedChangeBundleErrorMixedOrphansClassifyEach is the #285 no-target
// path: an unmanaged-worktree orphan AND a plain discardable orphan together.
// The error must LEAD with the non-destructive unmanaged case, keep a reason for
// EACH orphan, and name the CONCRETE plain slug in the residue prose (never a
// literal "<slug>").
func TestOrphanedChangeBundleErrorMixedOrphansClassifyEach(t *testing.T) {
	root := orphanRecoveryWorkspace(t)
	const unmanaged = "fix-283"
	const plain = "lonely-slug"
	writeOrphanBundle(t, root, unmanaged)
	writeOrphanBundle(t, root, plain)
	addWorktreeOnBranch(t, root, state.DefaultWorktreePath(root, unmanaged), "fix/issue-283-archived-worktree-resolution")

	cliErr := orphanedChangeBundleError(root, "") // empty slug -> all orphans
	require.NotNil(t, cliErr)
	assert.Equal(t, "orphaned_bundle_unmanaged_worktree", cliErr.ErrorCode, "the non-destructive case must lead")

	codes := map[string]bool{}
	for _, r := range cliErr.Reasons {
		codes[string(r.Code)] = true
	}
	assert.True(t, codes["orphaned_bundle_unmanaged_worktree"], "unmanaged orphan keeps its reason")
	assert.True(t, codes["orphaned_change_bundle"], "plain orphan must not be dropped")

	require.NotNil(t, cliErr.Details)
	assert.Contains(t, cliErr.Details["unmanaged_worktree_orphans"], unmanaged)
	assert.Contains(t, cliErr.Details["orphaned_change_bundles"], plain)

	// F5: residue prose names the concrete plain slug, never a literal placeholder.
	assert.Contains(t, cliErr.Remediation, "slipway delete --change "+plain)
	assert.NotContains(t, cliErr.Remediation, "--change <slug>")
	assert.Contains(t, cliErr.Remediation, "never pass --worktree")
}

// TestOrphanedChangeBundleErrorTwoArchivedResidueOrphans covers the
// len(class.ArchivedResidue) > 1 branch: two archived same-slug active-residue
// orphans surfaced at once. The error must list BOTH slugs, keep a reason for
// each, still exclude --worktree, and name the archived record / source commits
// as non-targets. NOT parallel: the workspace helper chdirs.
func TestOrphanedChangeBundleErrorTwoArchivedResidueOrphans(t *testing.T) {
	root := orphanRecoveryWorkspace(t)
	const first = "archived-residue-a"
	const second = "archived-residue-b"
	writeArchivedChangeWithActiveResidue(t, root, first)
	writeArchivedChangeWithActiveResidue(t, root, second)

	cliErr := orphanedChangeBundleError(root, "") // empty slug -> all orphans
	require.NotNil(t, cliErr)
	assert.Equal(t, "orphaned_active_residue_archived_change", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)

	// Both archived-residue slugs are named in the message and remediation.
	assert.Contains(t, cliErr.Message, first)
	assert.Contains(t, cliErr.Message, second)
	assert.Contains(t, cliErr.Remediation, "slipway delete --change <slug>")
	// The first suggested concrete command names one of the residue slugs.
	assert.True(t,
		strings.Contains(cliErr.Remediation, "slipway delete --change "+first) ||
			strings.Contains(cliErr.Remediation, "slipway delete --change "+second),
		"remediation must offer a concrete first delete command for one residue slug")
	assert.Contains(t, cliErr.Remediation, "Archived records and source commits are not deletion targets")
	assert.NotContains(t, cliErr.Remediation, "--worktree")

	// A reason is kept for EACH archived-residue orphan; no orphan is dropped.
	// Assert the stable Code/Detail fields, not Message prose.
	residueDetails := []string{}
	for _, r := range cliErr.Reasons {
		assert.Equal(t, "orphaned_change_bundle", string(r.Code))
		residueDetails = append(residueDetails, r.Detail)
	}
	assert.ElementsMatch(t, []string{first, second}, residueDetails,
		"both archived-residue orphans must keep a reason scoped to their slug")

	require.NotNil(t, cliErr.Details)
	residueSlugs, ok := cliErr.Details["orphaned_active_residue_archived_changes"].([]string)
	require.True(t, ok, "details must list the archived-residue slugs")
	assert.ElementsMatch(t, []string{first, second}, residueSlugs)
	assert.Nil(t, cliErr.Details["orphaned_change_bundles"],
		"a pure archived-residue case must not carry plain discard details")
}

// TestOrphanedChangeBundleErrorMixedArchivedResidueAndPlainOrphans covers the
// `if len(class.Plain) > 0` sub-branch inside the archived-residue block: one
// archived same-slug active-residue orphan AND one plain orphan (no archived
// record) in the same scan. Both must be surfaced; the archived-residue path
// must say archived record / source commits are not targets and exclude
// --worktree, while the plain path uses the ordinary discard wording. NOT
// parallel: the workspace helper chdirs.
func TestOrphanedChangeBundleErrorMixedArchivedResidueAndPlainOrphans(t *testing.T) {
	root := orphanRecoveryWorkspace(t)
	const archived = "archived-residue-mixed"
	const plain = "plain-residue-mixed"
	writeArchivedChangeWithActiveResidue(t, root, archived)
	writeOrphanBundle(t, root, plain) // no archived record -> classified Plain

	// Sanity: the plain slug really has no archived record, so it is not folded
	// into the archived-residue classification.
	_, archErr := state.LoadArchivedChange(root, plain)
	require.Error(t, archErr, "plain orphan must have no archived record")

	cliErr := orphanedChangeBundleError(root, "") // empty slug -> all orphans
	require.NotNil(t, cliErr)
	// The archived-residue case leads (no unmanaged/unknown present).
	assert.Equal(t, "orphaned_active_residue_archived_change", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Equal(t, archived, cliErr.Slug)

	// Archived-residue prose: names the archived slug, the not-a-target framing,
	// and never escalates to --worktree.
	assert.Contains(t, cliErr.Remediation, "Active-state residue")
	assert.Contains(t, cliErr.Remediation, archived)
	assert.Contains(t, cliErr.Remediation, "source commits are not deletion targets")
	assert.NotContains(t, cliErr.Remediation, "--worktree")
	// Plain sub-branch: the plain slug is discardable with the ordinary wording,
	// distinguished as residue with no archived record.
	assert.Contains(t, cliErr.Remediation, "no archived record")
	assert.Contains(t, cliErr.Remediation, "slipway delete --change "+plain)

	// A reason is kept for EACH orphan: the archived-residue one and the plain one.
	// Assert the stable Code/Detail fields, not Message prose; the archived-vs-plain
	// split is verified through the structured Details map below.
	codes := map[string]int{}
	reasonDetails := []string{}
	for _, r := range cliErr.Reasons {
		codes[string(r.Code)]++
		reasonDetails = append(reasonDetails, r.Detail)
	}
	assert.Equal(t, 2, codes["orphaned_change_bundle"], "both orphans must keep an orphaned_change_bundle reason")
	assert.ElementsMatch(t, []string{archived, plain}, reasonDetails,
		"each orphan keeps a reason scoped to its own slug")

	require.NotNil(t, cliErr.Details)
	residueSlugs, ok := cliErr.Details["orphaned_active_residue_archived_changes"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{archived}, residueSlugs)
	plainSlugs, ok := cliErr.Details["orphaned_change_bundles"].([]string)
	require.True(t, ok, "the plain orphan must be surfaced in details")
	assert.Equal(t, []string{plain}, plainSlugs)
}
