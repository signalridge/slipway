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

func TestDoneJSONWarnsWhenWorktreeSourceChangesAreUncommitted(t *testing.T) {
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
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)

		require.NoError(t, os.MkdirAll(filepath.Join(normalizedWT, "cmd"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(normalizedWT, "cmd", "done.go"), []byte("package cmd\n"), 0o644))

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
		assert.Contains(t, view.WorktreeDirtyWarning, "uncommitted source changes")
		assert.Contains(t, view.WorktreeDirtyFiles, "cmd/done.go")
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
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))
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
		assert.Contains(t, strings.ToLower(cliErr.Message), "review_layer_missing:ir1")

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
		assert.Contains(t, cliErr.Message, "session_isolation_warning")
		assert.NotContains(t, cliErr.Message, "stale_execution_evidence")

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
		assert.Contains(t, cliErr.Message, "tasks_checklist_empty")

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
		assert.Equal(t, "pivot_state_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
	})
}

func TestPivotRescopeRejectedOutsideS6(t *testing.T) {
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

		pivotCmd := commandForRoot(t, root, makePivotCmd())
		pivotCmd.SetArgs([]string{"--rescope"})
		err = pivotCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "rescope_state_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryGovernanceBlocked, cliErr.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, cliErr.ExitCode)
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
		assert.Equal(t, "pivot_state_invalid", cliErr.ErrorCode)
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
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
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
  - wave: 1
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
REQ-001: The fixture must provide a valid governed bundle.
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
  - wave: 1
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

	writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: runSummaryVersion,
		References: []string{fmt.Sprintf("run_summary_version=%d", runSummaryVersion)},
	})
}

func writePassingReviewEvidencePack(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()
	writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: runSummaryVersion,
		References: []string{"layer:R0=pass", "layer:R3=pass"},
	})
	writeSkillVerification(t, root, slug, "code-quality-review", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Add(time.Second),
		RunVersion: runSummaryVersion,
		References: []string{"layer:IR1=pass", "layer:IR3=pass", "layer:QUALITY=pass"},
	})
}

func writePassingGoalVerificationEvidence(t *testing.T, root, slug string, runSummaryVersion int) {
	t.Helper()
	writeSkillVerification(t, root, slug, "goal-verification", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: runSummaryVersion,
		References: []string{"verification:pass"},
	})
}
