package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDoneArchivesGovernedExecution(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "refactor service modules")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		require.NoError(t, doneCmd.Execute())

		// Verify the change was archived.
		_, err = state.LoadChange(root, slug)
		require.Error(t, err)
	})
}

func TestDoneArchivesGovernedAsTerminalDoneState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "archive terminal governed state")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		require.NoError(t, doneCmd.Execute())

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusDone, archived.Status)
		assert.Equal(t, model.StateDone, archived.CurrentState)
	})
}

func TestDoneJSONReportsWorktreeArchivePathWhenRunFromWorktree(t *testing.T) {
	root := t.TempDir()
	initGitRepoForWorktreeTests(t, root)
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := "done-worktree-archive-path"
		branch := "feat/" + slug
		worktreeRoot := filepath.Join(t.TempDir(), slug)
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
		normalizedWT, err := state.NormalizePath(worktreeRoot)
		require.NoError(t, err)

		change := model.NewChange(slug)
		change.WorktreePath = normalizedWT
		change.WorktreeBranch = branch
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeShipReadyGovernedBundle(t, normalizedWT, change)
		writeAssuranceMD(t, normalizedWT, slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)
		writePassingFinalCloseoutEvidence(t, root, slug, 1)
		gitCommitAll(t, normalizedWT, "ship-ready bundle")
		refreshPassingSkillDigestsForTest(t, normalizedWT, slug)

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(normalizedWT))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		var out bytes.Buffer
		doneCmd := makeDoneCmd()
		doneCmd.SetOut(&out)
		doneCmd.SetArgs([]string{"--json"})
		require.NoError(t, doneCmd.Execute())

		var view doneView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		expectedArchive := filepath.Join(normalizedWT, "artifacts", "changes", "archived", slug)
		assert.Equal(t, state.DisplayPath(root, expectedArchive), view.ArchivePath)
		assert.True(t, view.ArchiveCommitRequired)
		require.FileExists(t, filepath.Join(expectedArchive, "change.yaml"))
		require.NoFileExists(t, filepath.Join(root, "artifacts", "changes", "archived", slug, "change.yaml"))
	})
}

func TestDoneJSONWarnsButArchivesWhenWorktreeChangesAreUncommitted(t *testing.T) {
	root := t.TempDir()
	initGitRepoForWorktreeTests(t, root)
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := "done-worktree-dirty-warning"
		branch := "feat/" + slug
		worktreeRoot := filepath.Join(t.TempDir(), slug)
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
		normalizedWT, err := state.NormalizePath(worktreeRoot)
		require.NoError(t, err)

		change := model.NewChange(slug)
		change.WorktreePath = normalizedWT
		change.WorktreeBranch = branch
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeShipReadyGovernedBundle(t, normalizedWT, change)
		writeAssuranceMD(t, normalizedWT, slug, validAssuranceContent())
		require.NoError(t, os.MkdirAll(filepath.Join(normalizedWT, "cmd"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(normalizedWT, "cmd", "done.go"), []byte("package cmd\n"), 0o644))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)
		writePassingFinalCloseoutEvidence(t, root, slug, 1)

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(normalizedWT))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		var out bytes.Buffer
		doneCmd := makeDoneCmd()
		doneCmd.SetOut(&out)
		doneCmd.SetArgs([]string{"--json"})
		require.NoError(t, doneCmd.Execute())

		// Dirty source no longer blocks: `done` archives and surfaces a
		// non-blocking advisory listing what to commit with the archived bundle.
		var view doneView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.True(t, view.Archived)
		assert.NotEmpty(t, view.WorktreeDirtyWarning)
		assert.Contains(t, view.WorktreeDirtyFiles, "cmd/done.go")
		assert.NotContains(t, view.WorktreeDirtyFiles, filepath.ToSlash(filepath.Join("artifacts", "changes", slug, "intent.md")))
		require.FileExists(t, filepath.Join(normalizedWT, "artifacts", "changes", "archived", slug, "change.yaml"))
	})
}

func TestDoneJSONAllowsUncommittedGovernedBundleArchive(t *testing.T) {
	root := t.TempDir()
	initGitRepoForWorktreeTests(t, root)
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := "done-worktree-bundle-dirty"
		branch := "feat/" + slug
		worktreeRoot := filepath.Join(t.TempDir(), slug)
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
		normalizedWT, err := state.NormalizePath(worktreeRoot)
		require.NoError(t, err)

		change := model.NewChange(slug)
		change.WorktreePath = normalizedWT
		change.WorktreeBranch = branch
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeShipReadyGovernedBundle(t, normalizedWT, change)
		writeAssuranceMD(t, normalizedWT, slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)
		writePassingFinalCloseoutEvidence(t, root, slug, 1)

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(normalizedWT))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		var out bytes.Buffer
		doneCmd := makeDoneCmd()
		doneCmd.SetOut(&out)
		doneCmd.SetArgs([]string{"--json"})
		require.NoError(t, doneCmd.Execute())

		var view doneView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.True(t, view.Archived)
		require.FileExists(t, filepath.Join(normalizedWT, "artifacts", "changes", "archived", slug, "change.yaml"))
		require.NoDirExists(t, filepath.Join(normalizedWT, "artifacts", "changes", slug))
	})
}

func TestDoneJSONWarnsDirtyNonActiveChangeArtifact(t *testing.T) {
	root := t.TempDir()
	initGitRepoForWorktreeTests(t, root)
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := "done-nonactive-artifact-dirty"
		branch := "feat/" + slug
		worktreeRoot := filepath.Join(t.TempDir(), slug)
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
		normalizedWT, err := state.NormalizePath(worktreeRoot)
		require.NoError(t, err)

		change := model.NewChange(slug)
		change.WorktreePath = normalizedWT
		change.WorktreeBranch = branch
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeShipReadyGovernedBundle(t, normalizedWT, change)
		writeAssuranceMD(t, normalizedWT, slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)
		writePassingFinalCloseoutEvidence(t, root, slug, 1)

		// A sibling/archived change bundle left uncommitted is reported in the
		// dirty advisory because only the active artifacts/changes/<slug>/ bundle
		// is exempt — but it no longer blocks `done`.
		siblingRel := filepath.ToSlash(filepath.Join("artifacts", "changes", "archived", "other-change", "change.yaml"))
		siblingPath := filepath.Join(normalizedWT, filepath.FromSlash(siblingRel))
		require.NoError(t, os.MkdirAll(filepath.Dir(siblingPath), 0o755))
		require.NoError(t, os.WriteFile(siblingPath, []byte("slug: other-change\n"), 0o644))

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(normalizedWT))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		var out bytes.Buffer
		doneCmd := makeDoneCmd()
		doneCmd.SetOut(&out)
		doneCmd.SetArgs([]string{"--json"})
		require.NoError(t, doneCmd.Execute())

		var view doneView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.True(t, view.Archived)
		assert.Contains(t, view.WorktreeDirtyFiles, siblingRel)
		assert.NotContains(t, view.WorktreeDirtyFiles, filepath.ToSlash(filepath.Join("artifacts", "changes", slug, "intent.md")))
	})
}

func TestDoneAllReadyWarnsDirtyBoundWorktree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitRepoForWorktreeTests(t, root)
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := "bulk-dirty-worktree"
		branch := "feat/" + slug
		worktreeRoot := filepath.Join(t.TempDir(), slug)
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
		normalizedWT, err := state.NormalizePath(worktreeRoot)
		require.NoError(t, err)

		change := model.NewChange(slug)
		change.WorktreePath = normalizedWT
		change.WorktreeBranch = branch
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeShipReadyGovernedBundle(t, normalizedWT, change)
		writeAssuranceMD(t, normalizedWT, slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)
		writePassingFinalCloseoutEvidence(t, root, slug, 1)
		gitCommitAll(t, normalizedWT, "ship-ready bundle")
		refreshPassingSkillDigestsForTest(t, normalizedWT, slug)

		require.NoError(t, os.MkdirAll(filepath.Join(normalizedWT, "cmd"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(normalizedWT, "cmd", "done.go"), []byte("package cmd\n"), 0o644))
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)
		writePassingFinalCloseoutEvidence(t, root, slug, 1)

		view := archiveAllDoneReady(root)
		assert.Empty(t, view.Failed)
		require.Len(t, view.Archived, 1)
		assert.Equal(t, slug, view.Archived[0].Slug)
		assert.Equal(t, string(model.ChangeStatusDone), view.Archived[0].Status)
		assert.NotEmpty(t, view.Archived[0].WorktreeDirtyWarning)
		assert.Contains(t, view.Archived[0].WorktreeDirtyFiles, "cmd/done.go")
		require.FileExists(t, filepath.Join(normalizedWT, "artifacts", "changes", "archived", slug, "change.yaml"))
	})
}

func TestDoneJSONOmitsArchiveCommitRequiredForRepoScopedChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := "done-repo-archive-no-worktree"
		change := model.NewChange(slug)
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeShipReadyGovernedBundle(t, root, change)
		writeAssuranceMD(t, root, slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)
		writePassingFinalCloseoutEvidence(t, root, slug, 1)

		var out bytes.Buffer
		doneCmd := makeDoneCmd()
		doneCmd.SetOut(&out)
		doneCmd.SetArgs([]string{"--json"})
		require.NoError(t, doneCmd.Execute())

		var view doneView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.False(t, view.ArchiveCommitRequired)
		assert.NotContains(t, out.String(), "archive_commit_required")
	})
}

func TestDoneReportsAndPersistsRemediationSources(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		source := model.NewChange("source-archived-workflow")
		source.Description = "source archived workflow"
		require.NoError(t, state.SaveChange(root, source))
		_, err := state.ArchiveChange(root, source, model.ChangeStatusDone)
		require.NoError(t, err)

		slug := createGovernedRequest(t, root, "L2", "fix archived workflow feedback")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		intentPath := filepath.Join(root, "artifacts", "changes", slug, "intent.md")
		require.NoError(t, os.WriteFile(intentPath, []byte(strings.Join([]string{
			"Remediates artifacts/changes/archived/source-archived-workflow/workflow-feedback.md",
			"Ignores placeholder artifacts/changes/archived/<source-slug> examples",
			"Ignores missing artifacts/changes/archived/missing-archived-workflow references",
			"",
		}, "\n")), 0o644))
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		var out bytes.Buffer
		doneCmd := commandForRoot(t, root, makeDoneCmd())
		doneCmd.SetOut(&out)
		doneCmd.SetArgs([]string{"--json"})
		require.NoError(t, doneCmd.Execute())

		var view doneView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "remediation", view.ArchiveKind)
		require.Len(t, view.RemediationSources, 1)
		assert.Equal(t, "source-archived-workflow", view.RemediationSources[0].Slug)

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		require.Len(t, archived.RemediationSources, 1)
		assert.Equal(t, "source-archived-workflow", archived.RemediationSources[0].Slug)
	})
}

func TestDoneRemediationSourceScanDoesNotFollowBundleSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		source := model.NewChange("external-archived-workflow")
		source.Description = "external archived workflow"
		require.NoError(t, state.SaveChange(root, source))
		_, err := state.ArchiveChange(root, source, model.ChangeStatusDone)
		require.NoError(t, err)

		slug := createGovernedRequest(t, root, "L2", "fix archived workflow feedback")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		externalDir := t.TempDir()
		external := filepath.Join(externalDir, "outside.md")
		require.NoError(t, os.WriteFile(external, []byte("Remediates artifacts/changes/archived/external-archived-workflow/workflow-feedback.md\n"), 0o644))
		linkPath := filepath.Join(root, "artifacts", "changes", slug, "intent.md")
		require.NoError(t, os.Remove(linkPath))
		if err := os.Symlink(external, linkPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}

		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		var out bytes.Buffer
		doneCmd := commandForRoot(t, root, makeDoneCmd())
		doneCmd.SetOut(&out)
		doneCmd.SetArgs([]string{"--json"})
		require.NoError(t, doneCmd.Execute())

		var view doneView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Empty(t, view.RemediationSources)
	})
}

func TestDoneGovernedEmptyAssuranceReturnsInvalid(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "refactor service modules")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		// Write an empty assurance.md (missing required headings).
		writeAssuranceMD(t, root, change.Slug, "")

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		err = doneCmd.Execute()
		require.Error(t, err)
		var cliErr *CLIError
		require.ErrorAs(t, err, &cliErr)
		assert.Equal(t, "assurance_invalid", cliErr.ErrorCode)
	})
}

func TestDoneGovernedValidAssuranceSucceeds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "refactor service modules")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		require.NoError(t, doneCmd.Execute())
	})
}

func TestDoneLightPresetAllowsMissingAssurance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := commandForRoot(t, root, makeNewCmd())
		create.SetContext(withIntentClassifierContext(create.Context(), simpleIntentClassifier()))
		create.SetArgs([]string{"--preset", "light", "rename helper comment"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowPresetLight, change.WorkflowPreset)

		markChangeReadyForDone(t, root, &change)

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		require.NoError(t, doneCmd.Execute())

		_, err = state.LoadChange(root, slug)
		require.Error(t, err)
	})
}

func TestDoneQuickFullRevalidatesShipGateBeforeArchive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "quick full closeout must be fresh")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.QualityMode = model.QualityModeFull
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)

		_, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		_, archiveErr := state.LoadArchivedChange(root, slug)
		require.Error(t, archiveErr)
	})
}

func TestDoneShipGateBlockedSurfacesStaleEvidenceRecovery(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "done surfaces stale recovery")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		// Stale the final-closeout authority by mutating one of its certified
		// inputs (assurance.md) after evidence was stamped. Structure stays valid
		// so the block comes from the ship gate, not artifact validation.
		assurancePath := filepath.Join(root, "artifacts", "changes", slug, "assurance.md")
		require.NoError(t, os.WriteFile(assurancePath, []byte(validAssuranceContent()+"\n\nMutated after closeout was stamped.\n"), 0o644))

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)

		// REQ-008 parity: done's recovery payload names the same reopen action
		// (`slipway run`) and the same token that next/validate/status surface.
		require.NotNil(t, cliErr.Recovery)
		assert.Equal(t, "slipway run", cliErr.Recovery.PrimaryCommand)
		foundStaleToken := false
		for _, r := range cliErr.Reasons {
			if r.Code == "stale_evidence_recovery_available" {
				foundStaleToken = true
			}
		}
		assert.True(t, foundStaleToken, "done recovery must surface stale_evidence_recovery_available")
	})
}

func TestDoneRequiresReviewEvidenceBeforeArchive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "review evidence must be fresh")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))
		writePassingGoalVerificationEvidence(t, root, slug, 1)
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)

		_, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		raw, readErr := os.ReadFile(filepath.Join(root, "artifacts", "changes", slug, "change.yaml"))
		require.NoError(t, readErr)
		assert.NotContains(t, string(raw), "gates:")
		_, archiveErr := state.LoadArchivedChange(root, slug)
		require.Error(t, archiveErr)
	})
}

func TestDoneRejectsReviewLayerBlockersBeforeArchive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "review layer blockers must stop done")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)
		writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
			References: []string{"layer:R0=pass"},
		})
		writeSkillVerification(t, root, slug, "code-quality-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC().Add(time.Second),
			RunVersion: 1,
			References: []string{"layer:QUALITY=pass"},
		})

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)
		assert.Contains(t, model.ReasonSpecs(cliErr.Reasons), "review_layer_missing:IR1")
		require.NotNil(t, cliErr.Recovery)
		assert.Contains(t, recoveryStepCodes(cliErr.Recovery), "review_layer_missing")
		assert.NotEmpty(t, cliErr.Recovery.PrimaryCommand)

		_, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		_, archiveErr := state.LoadArchivedChange(root, slug)
		require.Error(t, archiveErr)
	})
}

func TestDoneRejectsExecutionSummaryLevelBlockersBeforeArchive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "summary blockers must stop done")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)

		const blocker = "session_isolation_warning:session_id=abc:shared_by=task-a,task-b"
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        time.Now().UTC(),
			OverallVerdict:    model.ExecutionVerdictFail,
			OpenBlockers:      []model.ReasonCode{model.ReasonCodeFromSpec(blocker)},
		})
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)
		assert.Contains(t, model.ReasonSpecs(cliErr.Reasons), blocker)
		assert.NotContains(t, model.ReasonSpecs(cliErr.Reasons), "stale_execution_evidence")

		_, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		_, archiveErr := state.LoadArchivedChange(root, slug)
		require.Error(t, archiveErr)
	})
}

func TestDoneRejectsChecklistBlockersBeforeArchive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "tasks checklist blockers must stop done")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		markChangeReadyForDone(t, root, &change)
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte("# Tasks\n"), 0o644))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "ship_gate_blocked", cliErr.ErrorCode)
		assert.Contains(t, model.ReasonSpecs(cliErr.Reasons), "tasks_checklist_empty")

		_, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		_, archiveErr := state.LoadArchivedChange(root, slug)
		require.Error(t, archiveErr)
	})
}

func TestDoneRejectsPlanAuditChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "plan audit change")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "not_done_ready", cliErr.ErrorCode)
	})
}

func TestDoneRejectsAllReadyWithExplicitRequest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		doneCmd.SetArgs([]string{"--all-ready", "--change", "some-slug"})
		err := doneCmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "flag_conflict", cliErr.ErrorCode)
		assert.Equal(t, categoryInvalidUsage, cliErr.Category)
		assert.Equal(t, exitCodeInvalidUsage, cliErr.ExitCode)
	})
}

func TestDoneBulkFallbackReasonCodesAreCanonical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item doneBulkItem
		code string
	}{
		{
			name: "list changes failed",
			item: newDoneBulkFailed("", "list_changes_failed", "permission denied"),
			code: "list_changes_failed",
		},
		{
			name: "load change failed",
			item: newDoneBulkFailed("bulk-load-failed", "load_change_failed", "parse error"),
			code: "load_change_failed",
		},
		{
			name: "change not active",
			item: newDoneBulkSkipped("bulk-cancelled", string(model.ChangeStatusCancelled), "change_not_active"),
			code: "change_not_active",
		},
		{
			name: "not done ready",
			item: newDoneBulkSkipped("bulk-not-ready", string(model.StateS2Execute), "not_done_ready"),
			code: "not_done_ready",
		},
		{
			name: "artifact reconcile failed",
			item: newDoneBulkFailed("bulk-reconcile-failed", "artifact_reconcile_failed", "permission denied"),
			code: "artifact_reconcile_failed",
		},
		{
			name: "artifact validation failed",
			item: newDoneBulkFailed("bulk-artifact-invalid", "artifact_validation_failed", "assurance.md missing"),
			code: "artifact_validation_failed",
		},
		{
			name: "lifecycle event write failed",
			item: newDoneBulkFailed("bulk-event-failed", "lifecycle_event_write_failed", "permission denied"),
			code: "lifecycle_event_write_failed",
		},
		{
			name: "archive failed",
			item: newDoneBulkFailed("bulk-archive-failed", "archive_failed", "rename failed"),
			code: "archive_failed",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Len(t, tt.item.ReasonCodes, 1)
			assert.Equal(t, tt.code, tt.item.ReasonCodes[0].Code)
			assert.NotEqual(t, "unknown_reason_code", tt.item.ReasonCodes[0].Code)
			step, ok := recoveryStepByCode(tt.item.ReasonCodes, tt.code)
			require.Truef(t, ok, "%s must remain recovery-routable", tt.code)
			assert.NotEmpty(t, step.Remediation)
		})
	}
}

func recoveryStepByCode(reasons []model.ReasonCode, code string) (model.RecoveryStep, bool) {
	recovery := model.BuildRecovery(reasons)
	if recovery == nil {
		return model.RecoveryStep{}, false
	}
	for _, step := range recovery.Steps {
		if step.Code == code {
			return step, true
		}
	}
	return model.RecoveryStep{}, false
}

func TestDoneRejectsMalformedConfigBeforeLockProtectedMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createActiveNonDiscoveryChange(t, root, "done malformed config guard")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("{invalid"), 0o644))

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		doneCmd.SetArgs([]string{"--change", slug})
		err = doneCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "config_parse_failure", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
		assert.Equal(t, exitCodeStateIntegrity, cliErr.ExitCode)
	})
}

func TestDoneAllReadyArchivesEligibleChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		readyL1 := model.NewChange("bulk-l1-ready")
		markChangeReadyForDone(t, root, &readyL1)
		writeAssuranceMD(t, root, readyL1.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, readyL1.Slug, 1, "t-01")

		readyL2 := model.NewChange("bulk-l2-ready")
		markChangeReadyForDone(t, root, &readyL2)
		writeAssuranceMD(t, root, readyL2.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, readyL2.Slug, 1, "t-01")

		notReady := model.NewChange("bulk-not-ready")
		notReady.CurrentState = model.StateS2Execute
		notReady.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, notReady))

		view := archiveAllDoneReady(root)
		require.Len(t, view.Archived, 2)
		assert.Equal(t, []doneBulkItem{
			newDoneBulkArchived("bulk-l1-ready"),
			newDoneBulkArchived("bulk-l2-ready"),
		}, view.Archived)
		require.Len(t, view.Skipped, 1)
		assert.Equal(t, newDoneBulkSkipped("bulk-not-ready", string(model.StateS2Execute), "not_done_ready"), view.Skipped[0])
		assert.Empty(t, view.Failed)

		_, err := state.LoadArchivedChange(root, readyL1.Slug)
		require.NoError(t, err)
		_, err = state.LoadArchivedChange(root, readyL2.Slug)
		require.NoError(t, err)
		_, err = state.LoadChange(root, notReady.Slug)
		require.NoError(t, err)
	})
}

func TestDoneAllReadyRespectsPerChangeLocks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		lockedChange := model.NewChange("bulk-locked")
		markChangeReadyForDone(t, root, &lockedChange)
		writeAssuranceMD(t, root, lockedChange.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, lockedChange.Slug, 1, "t-01")

		readyChange := model.NewChange("bulk-ready")
		markChangeReadyForDone(t, root, &readyChange)
		writeAssuranceMD(t, root, readyChange.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, readyChange.Slug, 1, "t-01")

		stopLockHolder := startStateLockHolder(t, state.ChangeStateLockPath(root, lockedChange.Slug))
		defer stopLockHolder()

		view := archiveAllDoneReady(root)
		require.Len(t, view.Archived, 1)
		assert.Equal(t, newDoneBulkArchived("bulk-ready"), view.Archived[0])
		require.Len(t, view.Failed, 1)
		assert.Equal(t, "bulk-locked", view.Failed[0].Slug)
		assert.Equal(t, "state_lock_timeout", view.Failed[0].Reason)
		assert.Contains(t, strings.ToLower(view.Failed[0].ErrorDetail), "state lock timeout")

		_, err := state.LoadArchivedChange(root, readyChange.Slug)
		require.NoError(t, err)
		_, err = state.LoadChange(root, lockedChange.Slug)
		require.NoError(t, err)
	})
}

func TestCancelArchivesDirectExecutionWithCancelledStatus(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createIntakeChangeFixture(t, root, "fix login timeout")

		cancelCmd := commandForRoot(t, root, makeCancelCmd())
		require.NoError(t, cancelCmd.Execute())

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusCancelled, archived.Status)
	})
}

func TestCancelArchivesGovernedExecutionWithCancelledStatus(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "refactor service modules")

		cancelCmd := commandForRoot(t, root, makeCancelCmd())
		require.NoError(t, cancelCmd.Execute())

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusCancelled, archived.Status)
	})
}

func TestCancelArchivesUnboundL3Change(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedChangeFixture(t, root, "investigate stale state cleanup", func(change *model.Change) {
			change.CurrentState = model.StateS0Intake
			change.IntakeSubStep = model.IntakeSubStepClarify
			change.NeedsDiscovery = true
		})

		cancelCmd := commandForRoot(t, root, makeCancelCmd())
		require.NoError(t, cancelCmd.Execute())

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusCancelled, archived.Status)
		assert.True(t, archived.NeedsDiscovery)
		assert.Empty(t, archived.WorktreePath)
	})
}

func TestCancelRejectsUnexpectedArgs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		createIntakeChangeFixture(t, root, "cancel should reject unexpected args")

		cancelCmd := commandForRoot(t, root, makeCancelCmd())
		cancelCmd.SetArgs([]string{"unexpected"})

		err := cancelCmd.Execute()
		require.Error(t, err)
		assertUnexpectedArgError(t, err)
	})
}

func TestCancelUsesHumanReadableOutputWithoutJSONFlag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createIntakeChangeFixture(t, root, "cancel text output")

		cancelCmd := commandForRoot(t, root, makeCancelCmd())
		var buf bytes.Buffer
		cancelCmd.SetOut(&buf)
		cancelCmd.SetErr(&buf)
		require.NoError(t, cancelCmd.Execute())

		assert.Contains(t, buf.String(), "Change cancelled: "+slug)
		assert.Contains(t, buf.String(), "Archived: true")
		assert.NotContains(t, buf.String(), `"archived":`)
	})
}

func TestCancelUsesJSONOutputWhenRequested(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createIntakeChangeFixture(t, root, "cancel json output")

		cancelCmd := commandForRoot(t, root, makeCancelCmd())
		cancelCmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cancelCmd.SetOut(&buf)
		cancelCmd.SetErr(&buf)
		require.NoError(t, cancelCmd.Execute())

		var view cancelView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.True(t, view.Archived)
		assert.Equal(t, string(model.ChangeStatusCancelled), view.Status)
	})
}

func TestPivotStateBoundaryRejected(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		createGovernedChangeFixture(t, root, "refactor service modules", func(change *model.Change) {
			change.CurrentState = model.StateS1Plan
			change.IntakeSubStep = ""
			change.PlanSubStep = model.PlanSubStepResearch
		})

		// S1_PLAN now allows reroute pivot; test that rescope IS rejected.
		pivotCmd := commandForRoot(t, root, makePivotCmd())
		pivotCmd.SetArgs([]string{"--rescope"})
		err := pivotCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "rescope_state_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
	})
}

func TestRunStalePlanningEvidenceReopensPlanAuditAndPreservesRuntimeEvidence(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		state model.WorkflowState
	}{
		{name: "review", state: model.StateS3Review},
		{name: "verify", state: model.StateS4Verify},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			slug, _ := prepareStalePlanningRecoveryFixture(t, root, tt.state)

			verificationDir := state.VerificationDir(root, slug)
			planAuditPath := filepath.Join(verificationDir, progression.SkillPlanAudit+".yaml")
			wavePlanPath := filepath.Join(verificationDir, state.WavePlanFileName)
			executionSummaryPath := state.ExecutionSummaryPathForRead(root, slug)
			waveOrchestrationPath := filepath.Join(verificationDir, progression.SkillWaveOrchestration+".yaml")
			specReviewPath := filepath.Join(verificationDir, progression.SkillSpecComplianceReview+".yaml")
			codeReviewPath := filepath.Join(verificationDir, progression.SkillCodeQualityReview+".yaml")
			goalVerificationPath := filepath.Join(verificationDir, progression.SkillGoalVerification+".yaml")
			finalCloseoutPath := filepath.Join(verificationDir, progression.SkillFinalCloseout+".yaml")
			waveRunPath := filepath.Join(state.WaveEvidenceDir(root, slug), "wave-01.yaml")
			taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")

			require.FileExists(t, planAuditPath)
			require.FileExists(t, wavePlanPath)
			require.FileExists(t, executionSummaryPath)
			require.FileExists(t, waveOrchestrationPath)
			require.FileExists(t, specReviewPath)
			require.FileExists(t, codeReviewPath)
			require.FileExists(t, goalVerificationPath)
			require.FileExists(t, finalCloseoutPath)
			require.FileExists(t, waveRunPath)
			require.FileExists(t, taskEvidencePath)

			// Stamp evidence digests for the target and downstream skills recovery clears.
			refreshPassingSkillDigestsForTest(t, root, slug,
				progression.SkillPlanAudit,
				progression.SkillSpecComplianceReview,
				progression.SkillCodeQualityReview,
				progression.SkillGoalVerification,
				progression.SkillFinalCloseout,
				progression.SkillWaveOrchestration,
			)
			preChange, err := state.LoadChange(root, slug)
			require.NoError(t, err)
			preDigests, err := state.LoadEvidenceDigestsForChange(root, preChange)
			require.NoError(t, err)
			for _, skillName := range []string{
				progression.SkillPlanAudit,
				progression.SkillSpecComplianceReview,
				progression.SkillCodeQualityReview,
				progression.SkillGoalVerification,
				progression.SkillFinalCloseout,
				progression.SkillWaveOrchestration,
			} {
				require.Contains(t, preDigests.Skills, skillName, "fixture should stamp a digest for %s", skillName)
			}

			runCmd := commandForRoot(t, root, makeRunCmd())
			runCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
			var buf bytes.Buffer
			runCmd.SetOut(&buf)
			require.NoError(t, runCmd.Execute())

			var view nextView
			require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
			require.NotNil(t, view.Advanced)
			assert.Equal(t, "advanced", view.Advanced.Action)
			assert.Equal(t, tt.state, view.Advanced.FromState)
			assert.Equal(t, model.StateS1Plan, view.Advanced.ToState)
			assert.Equal(t, string(model.PlanSubStepAudit), view.Advanced.ToSubStep)
			assert.True(t, view.Advanced.RecoveryOnly)
			assert.Equal(t, "stale_evidence_recovery_started", view.Advanced.Reason)
			require.NotNil(t, view.NextSkill)
			assert.Equal(t, progression.SkillPlanAudit, view.NextSkill.Name)

			recovered, err := state.LoadChange(root, slug)
			require.NoError(t, err)
			assert.Equal(t, model.StateS1Plan, recovered.CurrentState)
			assert.Equal(t, model.PlanSubStepAudit, recovered.PlanSubStep)

			require.NoFileExists(t, planAuditPath)
			require.NoFileExists(t, wavePlanPath)
			require.NoFileExists(t, executionSummaryPath)
			require.NoFileExists(t, waveOrchestrationPath)
			require.NoFileExists(t, specReviewPath)
			require.NoFileExists(t, codeReviewPath)
			require.NoFileExists(t, goalVerificationPath)
			require.NoFileExists(t, finalCloseoutPath)
			require.FileExists(t, waveRunPath)
			require.FileExists(t, taskEvidencePath)

			// Digest prune: a digest entry never outlives its verification record.
			postDigests, err := state.LoadEvidenceDigestsForChange(root, recovered)
			require.NoError(t, err)
			for _, skillName := range []string{
				progression.SkillPlanAudit,
				progression.SkillSpecComplianceReview,
				progression.SkillCodeQualityReview,
				progression.SkillGoalVerification,
				progression.SkillFinalCloseout,
				progression.SkillWaveOrchestration,
			} {
				assert.NotContains(t, postDigests.Skills, skillName, "stale evidence recovery must prune the %s digest with its record", skillName)
			}
		})
	}
}

func TestRunStalePlanningRecoveryHumanOutputListsSideEffects(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug, _ := prepareStalePlanningRecoveryFixture(t, root, model.StateS3Review)

	runCmd := commandForRoot(t, root, makeRunCmd())
	runCmd.SetArgs([]string{"--change", slug})
	var buf bytes.Buffer
	runCmd.SetOut(&buf)
	require.NoError(t, runCmd.Execute())

	stdout := buf.String()
	assert.Contains(t, stdout, "Advanced: S3_REVIEW -> S1_PLAN")
	assert.Contains(t, stdout, "Recovery Side Effects:")
	assert.Contains(t, stdout, "cleared_stale_verification")
	assert.Contains(t, stdout, "cleared_stale_generated_evidence")
	assert.Contains(t, stdout, "preserved_runtime_execution_evidence")
	assert.Contains(t, stdout, "artifacts/changes/"+slug+"/verification/plan-audit.yaml")
	assert.Contains(t, stdout, "artifacts/changes/"+slug+"/verification/wave-orchestration.yaml")
	assert.Contains(t, stdout, "artifacts/changes/"+slug+"/verification/spec-compliance-review.yaml")
}

func TestRunStalePlanningRecoveryRequiresFreshReviewAfterExecutionRefresh(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug, _ := prepareStalePlanningRecoveryBaseFixture(t, root, model.StateS4Verify)

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	requirementsPath := artifact.ResolveArtifactPath(bundlePath, "requirements.md")
	rawRequirements, err := os.ReadFile(requirementsPath)
	require.NoError(t, err)
	require.NoError(t, writeBundleArtifactFile(
		bundlePath,
		slug,
		"requirements.md",
		append(rawRequirements, []byte("\nAdditional context changed after review.\n")...),
	))
	staleAt := time.Now().UTC().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(requirementsPath, staleAt, staleAt))

	// Simulate the operator refreshing task evidence after recovery while keeping
	// the same run_version. Old review/verify evidence must not become reusable
	// just because the rebuilt execution summary has the same run_version.
	writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
		"changed_files": []string{"cmd/done.go"},
		"captured_at":   staleAt.Add(time.Second).Format(time.RFC3339Nano),
	})

	recoveryCmd := commandForRoot(t, root, makeRunCmd())
	recoveryCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var recoveryBuf bytes.Buffer
	recoveryCmd.SetOut(&recoveryBuf)
	require.NoError(t, recoveryCmd.Execute())

	var recoveryView nextView
	require.NoError(t, json.Unmarshal(recoveryBuf.Bytes(), &recoveryView))
	require.NotNil(t, recoveryView.Advanced)
	assert.Equal(t, "stale_evidence_recovery_started", recoveryView.Advanced.Reason)
	require.NotNil(t, recoveryView.NextSkill)
	assert.Equal(t, progression.SkillPlanAudit, recoveryView.NextSkill.Name)

	writeSkillVerification(t, root, slug, progression.SkillPlanAudit, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  staleAt.Add(2 * time.Second),
		RunVersion: 0,
		References: append([]string{"plan-audit:refreshed"}, planAuditOriginReferences()...),
	})

	advanceCmd := commandForRoot(t, root, makeRunCmd())
	advanceCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var advanceBuf bytes.Buffer
	advanceCmd.SetOut(&advanceBuf)
	require.NoError(t, advanceCmd.Execute())

	var advanceView nextView
	require.NoError(t, json.Unmarshal(advanceBuf.Bytes(), &advanceView))
	assert.Equal(t, model.StateS2Execute, advanceView.CurrentState)
	require.NotNil(t, advanceView.NextSkill)
	assert.Equal(t, progression.SkillWaveOrchestration, advanceView.NextSkill.Name)

	writeSkillVerification(t, root, slug, progression.SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  staleAt.Add(3 * time.Second),
		RunVersion: 1,
		References: []string{"wave-orchestration:refreshed"},
	})

	syncCmd := commandForRoot(t, root, makeRunCmd())
	syncCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var syncBuf bytes.Buffer
	syncCmd.SetOut(&syncBuf)
	require.NoError(t, syncCmd.Execute())

	var syncView nextView
	require.NoError(t, json.Unmarshal(syncBuf.Bytes(), &syncView))
	assert.Equal(t, model.StateS3Review, syncView.CurrentState)
	require.NotNil(t, syncView.Advanced)
	assert.Equal(t, "advanced", syncView.Advanced.Action)
	assert.Equal(t, model.StateS2Execute, syncView.Advanced.FromState)
	assert.Equal(t, model.StateS3Review, syncView.Advanced.ToState)
	require.NotNil(t, syncView.NextSkill)
	assert.Equal(t, progression.SkillSpecComplianceReview, syncView.NextSkill.Name)
	reasons := model.ReasonSpecs(syncView.Blockers)
	assert.Contains(t, reasons, "required_skill_missing:"+progression.SkillSpecComplianceReview)
	assert.NotContains(t, reasons, "run_slipway_run_to_advance:"+string(model.StateS3Review))
	assert.Equal(t, "skill_handoff:"+progression.SkillSpecComplianceReview, syncView.ConfirmationRequirement.Reason)
	assert.False(t, syncView.ConfirmationRequirement.ResumeResponseSupported)
	assert.Equal(t, "run governance skill "+progression.SkillSpecComplianceReview+" and record evidence; --resume-response is only for active checkpoints", syncView.ConfirmationRequirement.NextAction)
}

func TestRunStalePlanningRecoveryRefreshesEvidenceInOrder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug, _ := prepareStalePlanningRecoveryFixture(t, root, model.StateS3Review)

	runCmd := commandForRoot(t, root, makeRunCmd())
	runCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var recoveryBuf bytes.Buffer
	runCmd.SetOut(&recoveryBuf)
	require.NoError(t, runCmd.Execute())

	recovered, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS1Plan, recovered.CurrentState)
	assert.Equal(t, model.PlanSubStepAudit, recovered.PlanSubStep)
	require.NoFileExists(t, filepath.Join(state.VerificationDir(root, slug), progression.SkillPlanAudit+".yaml"))
	require.NoFileExists(t, filepath.Join(state.VerificationDir(root, slug), state.WavePlanFileName))
	require.NoFileExists(t, state.ExecutionSummaryPathForRead(root, slug))

	blockedCmd := commandForRoot(t, root, makeRunCmd())
	blockedCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var blockedBuf bytes.Buffer
	blockedCmd.SetOut(&blockedBuf)
	require.NoError(t, blockedCmd.Execute())

	var blockedView nextView
	require.NoError(t, json.Unmarshal(blockedBuf.Bytes(), &blockedView))
	require.NotNil(t, blockedView.Advanced)
	assert.Equal(t, "blocked", blockedView.Advanced.Action)
	assert.Contains(t, model.ReasonSpecs(blockedView.Blockers), "required_skill_missing:plan-audit")
	require.NoFileExists(t, filepath.Join(state.VerificationDir(root, slug), state.WavePlanFileName))
	require.NoFileExists(t, state.ExecutionSummaryPathForRead(root, slug))

	writeSkillVerification(t, root, slug, progression.SkillPlanAudit, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Add(3 * time.Second),
		RunVersion: 0,
		References: append([]string{"plan-audit:refreshed"}, planAuditOriginReferences()...),
	})

	advanceCmd := commandForRoot(t, root, makeRunCmd())
	advanceCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var advanceBuf bytes.Buffer
	advanceCmd.SetOut(&advanceBuf)
	require.NoError(t, advanceCmd.Execute())

	var advanceView nextView
	require.NoError(t, json.Unmarshal(advanceBuf.Bytes(), &advanceView))
	require.NotNil(t, advanceView.Advanced)
	assert.Equal(t, "advanced", advanceView.Advanced.Action)
	assert.Equal(t, model.StateS2Execute, advanceView.CurrentState)
	require.NotNil(t, advanceView.NextSkill)
	assert.Equal(t, progression.SkillWaveOrchestration, advanceView.NextSkill.Name)
	require.FileExists(t, filepath.Join(state.VerificationDir(root, slug), state.WavePlanFileName))
	require.NoFileExists(t, state.ExecutionSummaryPathForRead(root, slug))

	syncCmd := commandForRoot(t, root, makeRunCmd())
	syncCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var syncBuf bytes.Buffer
	syncCmd.SetOut(&syncBuf)
	require.NoError(t, syncCmd.Execute())

	var syncView nextView
	require.NoError(t, json.Unmarshal(syncBuf.Bytes(), &syncView))
	require.NotNil(t, syncView.Advanced)
	assert.Equal(t, "blocked", syncView.Advanced.Action)
	assert.Contains(t, model.ReasonSpecs(syncView.Blockers), "required_skill_missing:"+progression.SkillWaveOrchestration)

	_, err = state.LoadExecutionSummary(root, slug)
	require.Error(t, err)

	writeSkillVerification(t, root, slug, progression.SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Add(4 * time.Second),
		RunVersion: 1,
		References: []string{"wave-orchestration:refreshed"},
	})

	resyncCmd := commandForRoot(t, root, makeRunCmd())
	resyncCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var resyncBuf bytes.Buffer
	resyncCmd.SetOut(&resyncBuf)
	require.NoError(t, resyncCmd.Execute())

	var resyncView nextView
	require.NoError(t, json.Unmarshal(resyncBuf.Bytes(), &resyncView))
	require.NotNil(t, resyncView.Advanced)
	assert.Equal(t, "blocked", resyncView.Advanced.Action)
	assert.Contains(t, model.ReasonSpecs(resyncView.Blockers), "tasks_plan_changed_since_task_evidence:t-01")

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.Contains(t, model.ReasonSpecs(summary.OpenBlockers), "tasks_plan_changed_since_task_evidence:t-01")
}

func TestPivotRescopeReachableFromS3Review(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "refactor service modules")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		// Rescope must be reachable from S3_REVIEW: scope-drift recovery was
		// stranded when stale review evidence reopened a change to S3 while rescope
		// still required S2. Rescope resets the change to S0_INTAKE for re-planning.
		pivotCmd := commandForRoot(t, root, makePivotCmd())
		pivotCmd.SetArgs([]string{"--rescope"})
		require.NoError(t, pivotCmd.Execute())

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS0Intake, reloaded.CurrentState)
	})
}

func TestPivotRescopeRejectedAtIntakeState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		createIntakeChangeFixture(t, root, "narrow intake request")

		pivotCmd := commandForRoot(t, root, makePivotCmd())
		pivotCmd.SetArgs([]string{"--rescope"})
		err := pivotCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "rescope_state_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
	})
}

func TestPivotDefaultsToRerouteWithoutReason(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createActiveNonDiscoveryChange(t, root, "fix login timeout")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makePivotCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		assert.Equal(t, "reroute", payload["kind"])
	})
}

func TestRequestScopedCommandsRejectAmbiguousActiveContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		writeActiveChange(t, root, "ambig-a")
		writeActiveChange(t, root, "ambig-b")

		commands := []func() *cobra.Command{
			makeDoneCmd,
			makeCancelCmd,
			makePivotCmd,
			makeNextCmd,
		}
		for _, factory := range commands {
			cmd := commandForRoot(t, root, factory())
			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ambiguous")
		}
	})
}

func TestCancelPreemptsInFlightTasks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process signaling is Unix-only")
	}
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createIntakeChangeFixture(t, root, "fix login timeout")

		// Ignore SIGINT so cancel must escalate to SIGKILL after grace period.
		proc := exec.Command("sh", "-c", "trap '' INT; sleep 30")
		require.NoError(t, proc.Start())
		t.Cleanup(func() {
			if proc.ProcessState == nil {
				_ = proc.Process.Kill()
				_, _ = proc.Process.Wait()
			}
		})

		b, err := json.Marshal([]int{proc.Process.Pid})
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(state.TaskPIDFilePath(root, slug)), 0o755))
		require.NoError(t, os.WriteFile(state.TaskPIDFilePath(root, slug), b, 0o644))

		cfg := model.DefaultConfig()
		cfg.Execution.CancelGracePeriodSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		cancelCmd := commandForRoot(t, root, makeCancelCmd())
		require.NoError(t, cancelCmd.Execute())

		waited := make(chan struct{})
		go func() {
			_, _ = proc.Process.Wait()
			close(waited)
		}()
		select {
		case <-waited:
		case <-time.After(4 * time.Second):
			t.Fatalf("process %d was not terminated by cancel preemption", proc.Process.Pid)
		}

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		// Archive scrubs absolute-path evidence refs (runtime-local paths become
		// dangling after per-change runtime state is removed). The preemption
		// evidence was written to a git-local absolute path, so it must not
		// survive into the archived change.
		for key := range archived.EvidenceRefs {
			assert.False(t, strings.HasPrefix(key, "cancel_preemption_"),
				"archived change must not retain runtime-local cancel preemption evidence ref")
		}
	})
}

func TestMutatingCommandsBlockOnStateLock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createIntakeChangeFixture(t, root, "fix login timeout")

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		// One representative per-change command is sufficient here because the
		// helper path is already covered by dedicated command-level tests.
		t.Run("per_change_lock", func(t *testing.T) {
			stopLockHolder := startStateLockHolder(t, state.ChangeStateLockPath(root, slug))
			defer stopLockHolder()

			err := commandForRoot(t, root, makePivotCmd()).Execute()
			require.Error(t, err, "pivot")
			assert.Contains(t, strings.ToLower(err.Error()), "state lock timeout", "pivot")
		})

		t.Run("repair_lock", func(t *testing.T) {
			repairLockPath := state.RepairLockPath(root)
			stopLockHolder := startStateLockHolder(t, repairLockPath)
			defer stopLockHolder()

			err := commandForRoot(t, root, makeRepairCmd()).Execute()
			require.Error(t, err, "repair")
			assert.Contains(t, strings.ToLower(err.Error()), "state lock timeout", "repair")
		})
	})
}

func assertUnexpectedArgError(t *testing.T, err error) {
	t.Helper()

	msg := strings.ToLower(err.Error())
	assert.True(
		t,
		strings.Contains(msg, "accepts 0 arg") || strings.Contains(msg, "unknown command"),
		"expected unexpected-arg rejection, got %q",
		err.Error(),
	)
}

func TestRequestCommandBlocksOnChangeCreateLock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		lockPath := state.ChangeCreateLockPath(root)
		stopLockHolder := startStateLockHolder(t, lockPath)
		defer stopLockHolder()

		cmd := commandForRoot(t, root, makeNewCmd())
		cmd.SetArgs([]string{"change lock follow-up"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "state lock timeout")
	})
}

func TestGovernedPivotRerouteUpdatesGuardrailDomain(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createActiveNonDiscoveryChange(t, root, "fix login timeout")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		// Advance to S2_EXECUTE and modify for pivot reroute test.
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.Description = "update auth middleware policy"
		require.NoError(t, state.SaveChange(root, change))

		pivot := commandForRoot(t, root, makePivotCmd())
		require.NoError(t, pivot.Execute())

		updated, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		assert.Equal(t, model.ChangeStatusActive, updated.Status)
		assert.Equal(t, model.StateS1Plan, updated.CurrentState)
		// Pivot reroute preserves original guardrail domain and forces discovery.
		assert.Empty(t, updated.GuardrailDomain)
		assert.True(t, updated.NeedsDiscovery)
	})
}

func TestChangeYamlStableAfterSave(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "refactor service modules")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, slug, change.Slug)
		assert.NotEmpty(t, change.BaseRef)

		// Save and verify change.yaml is stable after re-save.
		originalBaseRef := change.BaseRef
		change.NeedsDiscovery = true
		require.NoError(t, state.SaveChange(root, change))

		updatedChange, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, originalBaseRef, updatedChange.BaseRef, "base_ref must be preserved across saves")
		assert.Equal(t, slug, updatedChange.Slug)
		assert.True(t, updatedChange.NeedsDiscovery)
	})
}

func TestRequestCreationCreatesCanonicalBundleState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		create := commandForRoot(t, root, makeNewCmd())
		create.SetArgs([]string{"verify change dir structure"})
		require.NoError(t, create.Execute())

		slug := singleChangeSlug(t, state.ActiveBundlesDir(root))
		_, err := os.Stat(state.BundleChangeFilePath(root, slug))
		require.NoError(t, err)

		_, err = os.Stat(state.ChangeDir(root, slug))
		assert.True(t, os.IsNotExist(err), "new should not eagerly create git-local runtime dirs")

		// Verify change is loadable via state package.
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, slug, change.Slug)
	})
}

func TestArchiveMovesChangeDirAndArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "archive migration test")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Advance to archivable state.
		markChangeReadyForDone(t, root, &change)
		writeAssuranceMD(t, root, slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		runtimeEvidence := filepath.Join(state.ChangeDir(root, slug), "evidence", "runs", "latest.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(runtimeEvidence), 0o755))
		require.NoError(t, os.WriteFile(runtimeEvidence, []byte("{}"), 0o644))

		_, err = os.Stat(filepath.Join(root, "artifacts", "changes", slug))
		require.NoError(t, err, "artifact dir must exist before archive")

		doneCmd := commandForRoot(t, root, makeDoneCmd())
		require.NoError(t, doneCmd.Execute())

		// Post-conditions: artifact dir moved to archived location.
		_, err = os.Stat(filepath.Join(root, "artifacts", "changes", slug))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err), "active artifact dir must not exist after archive")

		_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug))
		require.NoError(t, err, "archived bundle dir must exist after archive")

		_, err = os.Stat(state.ChangeDir(root, slug))
		assert.True(t, os.IsNotExist(err), "archive should delete git-local runtime dirs")
	})
}

// writeActiveChange creates a minimal active change to use in multi-change tests.
func writeActiveChange(t *testing.T, root, slug string) {
	t.Helper()
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.QualityMode = model.QualityModeStandard
	change.ComplexityLevel = "simple"
	require.NoError(t, state.SaveChange(root, change))
}

func createGovernedChangeFixture(t *testing.T, root, description string, mutate func(*model.Change)) string {
	t.Helper()

	slug, err := generateUniqueChangeSlug(description, func(candidate string) (bool, error) {
		return state.ChangeSlugExists(root, candidate)
	})
	require.NoError(t, err)

	change := model.NewChange(slug)
	change.Description = description
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.QualityMode = model.QualityModeStandard
	change.ComplexityLevel = "simple"
	change.ArtifactSchema = model.ArtifactSchemaExpanded

	if mutate != nil {
		mutate(&change)
	}

	require.NoError(t, state.SaveChange(root, change))
	writeMinimalGovernedBundle(t, root, change)
	return slug
}

// createGovernedRequest creates and routes a governed (L2/L3) request.
// Returns the slug. The change exists in artifacts/changes/<slug>/change.yaml after routing.
// The change is advanced to S1_PLAN to simulate having passed intake.
func createGovernedRequest(t *testing.T, root, level, description string) string {
	t.Helper()
	return createGovernedChangeFixture(t, root, description, func(change *model.Change) {
		// Advance past S0 intake to S1_PLAN (simulating intake completion).
		change.CurrentState = model.StateS1Plan
		change.IntakeSubStep = ""
		change.PlanSubStep = model.PlanSubStepResearch
		change.NeedsDiscovery = level == "L3"
	})
}

// createIntakeChangeFixture creates a change at S0_INTAKE (what `new` produces by default).
func createIntakeChangeFixture(t *testing.T, root, description string) string {
	t.Helper()
	return createGovernedChangeFixture(t, root, description, func(change *model.Change) {
		change.CurrentState = model.StateS0Intake
		change.IntakeSubStep = model.IntakeSubStepClarify
		change.NeedsDiscovery = true
		change.ComplexityLevel = "complex"
	})
}

// createActiveNonDiscoveryChange creates a non-discovery governed change and advances it to S5_RUN_WAVES.
// Returns the slug.
func createActiveNonDiscoveryChange(t *testing.T, root, description string) string {
	t.Helper()
	return createGovernedChangeFixture(t, root, description, func(change *model.Change) {
		change.CurrentState = model.StateS2Execute
		change.IntakeSubStep = ""
		change.PlanSubStep = model.PlanSubStepNone
		change.NeedsDiscovery = false
	})
}

func simpleIntentClassifier() *recordingIntentClassifier {
	return &recordingIntentClassifier{
		classification: progression.IntentClassification{
			GuardrailDomain: "",
			NeedsDiscovery:  false,
			Complexity:      "simple",
		},
	}
}

func validAssuranceContent() string {
	return `## Scope Summary
Summary of scope

## Verification Verdict
All verified

## Evidence Index
Evidence listed

## Requirement Coverage
All requirements covered

## Residual Risks and Exceptions
None identified

## Rollback Readiness
Rollback remains available and documented.

## Archive Decision
Approved for archive`
}

func writeAssuranceMD(t *testing.T, root, slug, content string) {
	t.Helper()
	dir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assurance.md"), []byte(content), 0o644))
	refreshPassingSkillDigestsForTest(t, root, slug)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}

func gitCommitAll(t *testing.T, dir, message string) {
	t.Helper()
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", message)
}

// Cross-stage context-origin handles for the governed independence lattice.
// Every stage that participates in the lattice (plan author, plan auditor, the
// selected reviews, goal verification, final closeout) carries a DISTINCT handle so
// the chain-wide independence gates (plan_audit_origin_invalid,
// context_origin_handle_invalid, cross_stage_context_not_distinct) are
// satisfied. The synthetic wave evidence declares degraded_sequential dispatch
// and so contributes NO executor handles; none of these values may collide.
const (
	testPlanOriginHandle      = "plan-author-h"
	testAuditOriginHandle     = "plan-auditor-h"
	testSpecContextHandle     = "spec-compliance-context"
	testCodeContextHandle     = "code-quality-context"
	testGoalContextHandle     = "goal-verification-context"
	testCloseoutContextHandle = "final-closeout-context"
)

// planAuditOriginReferences returns the distinct plan_origin/audit_origin token
// pair the S1 plan gate now requires of any plan-audit record that advances past
// G_plan. The audit_origin handle is also a downstream review-authority lattice
// participant, so it stays distinct from every context_origin:stage= handle.
func planAuditOriginReferences() []string {
	return []string{
		model.PlanOriginReferencePrefix + testPlanOriginHandle,
		model.AuditOriginReferencePrefix + testAuditOriginHandle,
	}
}

func writeSkillVerification(t *testing.T, root, slug, skillName string, rec model.VerificationRecord) {
	t.Helper()

	rec.Normalize()
	require.NoError(t, rec.Validate())

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	paths, err := state.ResolveChangePaths(root, change)
	require.NoError(t, err)

	dir := filepath.Join(paths.GovernedBundleDir, "verification")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	raw, err := yaml.Marshal(rec)
	require.NoError(t, err)
	require.NoError(t, fsutil.WriteFileAtomic(filepath.Join(dir, skillName+".yaml"), raw, 0o644))
}

func writeShipReadyGovernedBundle(t *testing.T, root string, change model.Change) {
	t.Helper()

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "intent.md", []byte(`# Intent
INT-001: finalize governed closeout
## Open Questions
(none)
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "requirements.md", []byte(`# Requirements
### Requirement: ShipReadyCloseout
REQ-001: The change MUST satisfy ship-ready governance evidence before finalization.

#### Scenario: Finalization requires fresh evidence
GIVEN a change carrying ship-ready governance evidence
WHEN finalization runs the ship gate
THEN the change is allowed to finalize.
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "decision.md", []byte(`# Decision
## Alternatives Considered
### Option A
Keep finalization gate-local.
### Option B
Reuse shared readiness across finalization.

## Selected Approach
Use shared readiness so finalization and read surfaces stay aligned.

## Interfaces and Data Flow
G_ship consumes the shared readiness surface.

## Rollout and Rollback
Roll forward by enforcing the same blockers across commands.

## Risk
Low risk; failures should surface as explicit readiness blockers.
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` verify ship readiness parity
  - depends_on: []
  - target_files: ["cmd/done.go"]
  - task_kind: verification
  - covers: [REQ-001]
`)))
}

func writeMinimalGovernedBundle(t *testing.T, root string, change model.Change) {
	t.Helper()

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "intent.md", []byte(`# Intent
INT-001: test fixture intent

## Open Questions
(none)
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "requirements.md", []byte(`# Requirements
### Requirement: FixtureContract
REQ-001: The fixture MUST provide a valid governed bundle.

#### Scenario: Fixture bundle validates
GIVEN a minimal governed bundle fixture
WHEN governed validation runs
THEN the bundle passes structural and substance checks.
`)))
	if change.NeedsDiscovery {
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "research.md", []byte(`## Research Findings

### Architecture
- Fixture architecture is intentionally minimal.

### Patterns
- Fixture patterns use direct file writes.

### Risks
- Low risk fixture.

### Test Strategy
- Command tests assert the target command surface.

## Alternatives Considered
### Option A
Use minimal fixture files.

### Option B
Run full scaffold for every command test.

Selected: Option A for test runtime.

## Unknowns
- Remaining: None.

## Assumptions
- Fixture files only need structural validity.

## Canonical References
- cmd/lifecycle_commands_test.go
`)))
	}
	// This shared fixture intentionally uses concrete prose, not the comment-only
	// instructions scaffold. The decision gate currently rejects missing/empty
	// sections and pure scaffold comments; if it later rejects weak draft prose,
	// this fixture should be upgraded with it.
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "decision.md", []byte(`# Decision
## Alternatives Considered
### Option A
Use minimal fixture files.

### Option B
Run full scaffold for every command test.

## Selected Approach
Pending investigation. Fixture draft text must not be treated as a locked
human-reviewed decision.

## Interfaces and Data Flow
No production interfaces change.

## Rollout and Rollback
Fixture-only setup.

## Risk
Low risk fixture.
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` exercise command fixture
  - depends_on: []
  - target_files: ["cmd/lifecycle_commands_test.go"]
  - task_kind: verification
  - covers: [REQ-001]
`)))
	if change.WorkflowPreset != model.WorkflowPresetLight {
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
	}
}

func markChangeReadyForDone(t *testing.T, root string, change *model.Change) {
	t.Helper()
	require.NotNil(t, change)
	change.CurrentState = model.StateS4Verify
	change.IntakeSubStep = ""
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, *change))
	writeShipReadyGovernedBundle(t, root, *change)
	writePassingExecutionSummary(t, root, change.Slug, 1, "t-01")
	writePassingWaveEvidence(t, root, change.Slug, 1)
	writePassingReviewEvidencePack(t, root, change.Slug, 1)
	writePassingGoalVerificationEvidence(t, root, change.Slug, 1)
	if change.WorkflowPreset != model.WorkflowPresetLight {
		writePassingFinalCloseoutEvidence(t, root, change.Slug, 1)
	}
}

func writePassingWaveEvidence(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	require.NoError(t, err)
	if state.ExecutionSummaryReady(summary) {
		materializeWaveExecutionForSummary(t, root, slug)
	}

	references := []string{fmt.Sprintf("run_summary_version=%d", runSummaryVersion)}
	// Declare degraded_sequential dispatch for every planned wave so synthetic
	// passing evidence clears the fail-closed dispatch-evidence gate. degraded
	// (not parallel_subagents) is the honest minimal claim for single-threaded test
	// fixtures and, unlike parallel_subagents, requires no per-task executor handles.
	if plan, planErr := state.LoadWavePlanForChange(root, change); planErr == nil {
		for _, planWave := range plan.Waves {
			references = append(references,
				fmt.Sprintf("dispatch_mode:wave=%d:degraded_sequential", planWave.WaveIndex),
				// A bare degraded_sequential claim is no longer self-sufficient; it must
				// be paired with a tool-unavailable justification for the same wave.
				fmt.Sprintf("degraded_dispatch_justification:wave=%d:tool_unavailable=synthetic test fixture cannot dispatch subagents", planWave.WaveIndex),
			)
		}
	}

	writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: runSummaryVersion,
		References: references,
	})
	refreshPassingSkillDigestsForTest(t, root, slug, progression.SkillWaveOrchestration)
}

func writePassingReviewEvidencePack(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()
	// Stamp the mandatory review trio at one instant so the always-on chain-order
	// invariant (closeout >= goal >= max(selected reviews)) holds: the later
	// goal/closeout helpers observe a non-earlier time.Now(). The reviews are
	// unordered peers, each carrying a distinct selected-review context_origin
	// handle so the cross-stage independence gate is satisfied.
	reviewStampedAt := time.Now().UTC()
	writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  reviewStampedAt,
		RunVersion: runSummaryVersion,
		References: []string{
			"layer:R0=pass",
			"layer:R3=pass",
			model.ContextOriginReferencePrefix + model.StageContextReview + "=" + testSpecContextHandle,
		},
	})
	writeSkillVerification(t, root, slug, "code-quality-review", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  reviewStampedAt,
		RunVersion: runSummaryVersion,
		References: []string{
			"layer:IR1=pass",
			"layer:IR3=pass",
			"layer:QUALITY=pass",
			model.ContextOriginReferencePrefix + model.StageContextReview + "=" + testCodeContextHandle,
		},
	})
	writeSkillVerification(t, root, slug, progression.SkillIndependentReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  reviewStampedAt,
		RunVersion: runSummaryVersion,
		References: []string{
			"independent-review:pass",
			model.ContextOriginReferencePrefix + model.StageContextReview + "=independent-review-context",
		},
	})
	refreshPassingSkillDigestsForTest(
		t,
		root,
		slug,
		progression.SkillSpecComplianceReview,
		progression.SkillCodeQualityReview,
		progression.SkillIndependentReview,
	)
}

func writePassingGoalVerificationEvidence(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()
	writeSkillVerification(t, root, slug, "goal-verification", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: runSummaryVersion,
		References: []string{"verification:pass", model.ContextOriginReferencePrefix + "goal=" + testGoalContextHandle},
	})
	refreshPassingSkillDigestsForTest(t, root, slug, progression.SkillGoalVerification)
}

func writePassingFinalCloseoutEvidence(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()
	writeSkillVerification(t, root, slug, "final-closeout", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: runSummaryVersion,
		References: []string{"closeout:assurance_complete=pass", "closeout:reviewer_independence=pass", model.ContextOriginReferencePrefix + "closeout=" + testCloseoutContextHandle},
	})
	refreshPassingSkillDigestsForTest(t, root, slug, progression.SkillFinalCloseout)
}

func refreshPassingSkillDigestsForTest(t *testing.T, root, slug string, skillNames ...string) {
	t.Helper()

	change, err := state.LoadChange(root, slug)
	if err != nil {
		return
	}
	summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	require.NoError(t, err)

	if len(skillNames) == 0 {
		skillNames = []string{
			progression.SkillWaveOrchestration,
			progression.SkillSpecComplianceReview,
			progression.SkillCodeQualityReview,
			progression.SkillSecurityReview,
			progression.SkillIndependentReview,
			progression.SkillGoalVerification,
			progression.SkillFinalCloseout,
		}
	}
	for _, skillName := range skillNames {
		rec, err := state.LoadVerification(root, slug, skillName)
		if err != nil || !rec.IsPassing() {
			continue
		}
		if err := progression.StampEvidenceDigestForSkill(root, change, skillName, rec, summary); err != nil {
			continue
		}
	}
}
