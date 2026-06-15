package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIssue232ResolveExplicitChangeFallsBackToArchivedWhenActiveBundleAuthorityMissing
// reproduces #232: an archived DONE change whose active bundle directory survived
// (its change.yaml was moved to the archive) made `slipway validate`/`slipway next`
// dead-end on change_state_load_failed because resolveExplicitChange only attempted
// the archived fallback on os.ErrNotExist. LoadChange reports a missing-authority
// error for the surviving directory, so the fallback must also trigger on
// state.IsMissingBundleAuthority — mirroring status's predicate.
func TestIssue232ResolveExplicitChangeFallsBackToArchivedWhenActiveBundleAuthorityMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := "issue232-archived-missing-active-authority"
	change := model.NewChange(slug)
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	_, err := state.ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	// Re-create the active bundle directory with a stray file but no change.yaml,
	// so LoadChange reports a missing-authority error rather than os.ErrNotExist.
	activePath := state.BundleChangeFilePath(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(activePath), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(activePath), "notes.md"), []byte("orphaned active bundle\n"), 0o644))

	// Precondition: LoadChange surfaces the missing-authority signal (not NotExist).
	_, loadErr := state.LoadChange(root, slug)
	require.True(t, state.IsMissingBundleAuthority(loadErr))

	_, resolveErr := resolveExplicitChange(root, slug)
	cliErr := asCLIError(resolveErr)
	require.NotNil(t, cliErr)
	assert.Equal(t, "archived_change_not_validatable", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Equal(t, exitCodePrecondition, cliErr.ExitCode)
	assert.Equal(t, slug, cliErr.Slug)
	assert.Equal(t, string(model.ChangeStatusDone), cliErr.Details["status"])
	assert.Equal(t, true, cliErr.Details["archived"])
	assert.Contains(t, cliErr.Remediation, "archived")
	assert.NotEqual(t, "change_state_load_failed", cliErr.ErrorCode)
}

// TestIssue232ResolveExplicitChangeMissingAuthorityWithoutArchiveFailsClosed
// pins the fail-closed nuance from #232: broadening the archived-fallback trigger
// must NOT soften a genuine missing-authority error into no_active_change when
// there is no archived record. The surviving orphan directory is real
// active-bundle corruption and must fail closed to a recovery/integrity blocker.
func TestIssue232ResolveExplicitChangeMissingAuthorityWithoutArchiveFailsClosed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "issue232 orphaned active bundle")
	// Drop change.yaml but leave a stray file so the bundle directory survives;
	// LoadChange then reports a missing-authority error with no archived record.
	activePath := state.BundleChangeFilePath(root, slug)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(activePath), "notes.md"), []byte("orphaned\n"), 0o644))
	require.NoError(t, os.Remove(activePath))

	_, loadErr := state.LoadChange(root, slug)
	require.True(t, state.IsMissingBundleAuthority(loadErr))

	_, resolveErr := resolveExplicitChange(root, slug)
	cliErr := asCLIError(resolveErr)
	require.NotNil(t, cliErr)
	// Must never be softened to a "no change here" precondition.
	assert.NotEqual(t, "no_active_change", cliErr.ErrorCode)
	// Fail closed: the operator is routed to discard the orphaned bundle.
	assert.Equal(t, "orphaned_change_bundle", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Contains(t, cliErr.Remediation, "slipway delete")
}

// TestIssue232ResolveExplicitChangeMalformedActiveAuthorityFailsClosed confirms
// the os.ErrNotExist-only fall-through to change_state_load_failed still fires for
// genuinely corrupt active state (a malformed change.yaml is neither NotExist nor
// missing-authority), so the #232 broadening did not mask real corruption.
func TestIssue232ResolveExplicitChangeMalformedActiveAuthorityFailsClosed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "issue232 malformed active authority")
	require.NoError(t, os.WriteFile(state.BundleChangeFilePath(root, slug), []byte("slug: ["), 0o644))

	_, resolveErr := resolveExplicitChange(root, slug)
	cliErr := asCLIError(resolveErr)
	require.NotNil(t, cliErr)
	assert.Equal(t, "change_state_load_failed", cliErr.ErrorCode)
	assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	assert.Equal(t, slug, cliErr.Slug)
	assert.Contains(t, cliErr.Remediation, "slipway repair")
}
