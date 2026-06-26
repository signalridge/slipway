package cmd

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func snapshotNonGitTree(t *testing.T, root string) map[string]string {
	t.Helper()

	entries := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if rel == ".git" {
			return filepath.SkipDir
		}
		if strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
			return nil
		}
		if entry.IsDir() {
			entries["dir:"+filepath.ToSlash(rel)] = ""
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries["file:"+filepath.ToSlash(rel)] = string(raw)
		return nil
	}))
	return entries
}

func TestValidateNoActiveDiagnosticIsZeroWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	before := snapshotNonGitTree(t, root)

	cmd := commandForRoot(t, root, makeValidateCmd())
	cmd.SetArgs([]string{"--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())

	assert.Equal(t, before, snapshotNonGitTree(t, root))
	assert.Contains(t, out.String(), "no active change or ambiguous")
}

func TestValidateMissingExplicitSlugFailsClosed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	before := snapshotNonGitTree(t, root)

	cmd := commandForRoot(t, root, makeValidateCmd())
	cmd.SetArgs([]string{"--json", "--change", "definitely-not-a-change"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()

	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "change_not_found", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Equal(t, exitCodePrecondition, cliErr.ExitCode)
	assert.Equal(t, "definitely-not-a-change", cliErr.Slug)
	assert.Contains(t, cliErr.Remediation, "slipway status")
	assert.Equal(t, before, snapshotNonGitTree(t, root))
}

func TestValidateArchivedExplicitSlugIsZeroWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate archived zero write")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	_, err = state.ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	before := snapshotNonGitTree(t, root)

	cmd := commandForRoot(t, root, makeValidateCmd())
	cmd.SetArgs([]string{"--json", "--change", slug})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err = cmd.Execute()

	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "archived_change_not_validatable", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Equal(t, exitCodePrecondition, cliErr.ExitCode)
	assert.Equal(t, slug, cliErr.Slug)
	assert.Equal(t, before, snapshotNonGitTree(t, root))
}

func TestValidateOrphanActiveBundleIsZeroWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	orphanDir := filepath.Join(root, "artifacts", "changes", "orphan-active-bundle", "review")
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(orphanDir, "00-build-test.md"), []byte("stale review artifact\n"), 0o644))

	before := snapshotNonGitTree(t, root)

	cmd := commandForRoot(t, root, makeValidateCmd())
	cmd.SetArgs([]string{"--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	_ = cmd.Execute()

	assert.Equal(t, before, snapshotNonGitTree(t, root))
}

func TestValidateMalformedVerificationExplainsEngineOwnedRecovery(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "validate malformed verification recovery")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeMalformedVerificationFile(t, root, slug, "plan-audit")
		before := snapshotNonGitTree(t, root)

		cmd := commandForRoot(t, root, makeValidateCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		var out bytes.Buffer
		cmd.SetOut(&out)

		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "verification_load_failed", cliErr.ErrorCode)
		errorText := err.Error()
		assert.Contains(t, errorText, "VerificationRecord")
		assert.Contains(t, errorText, "verification/plan-audit.yaml")
		assert.Contains(t, errorText, "verification/plan-audit-notes.md")
		assert.Contains(t, errorText, "slipway evidence skill --skill plan-audit --notes-file verification/plan-audit-notes.md")
		assert.Equal(t, before, snapshotNonGitTree(t, root))
	})
}
