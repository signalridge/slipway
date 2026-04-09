package state

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestArchiveChangeDoneFreezesArtifactsAndMigratesPaths(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "my-change"

	change := model.NewChange(slug)
	change.Artifacts = map[string]model.ArtifactState{
		"intent":       {ID: "intent", State: model.ArtifactLifecycleDraft},
		"requirements": {ID: "requirements", State: model.ArtifactLifecycleStale},
	}
	require.NoError(t, SaveChange(root, change))

	artifactDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(artifactDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(artifactDir, "change.yaml"), []byte("id: x"), 0o644))

	archived, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeStatusDone, archived.Status)
	assert.Equal(t, model.ArtifactLifecycleFrozen, archived.Artifacts["intent"].State)
	assert.Equal(t, model.ArtifactLifecycleFrozen, archived.Artifacts["requirements"].State)

	// Active change dir should be gone.
	_, err = os.Stat(BundleChangeFilePath(root, slug))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	// Artifact dir should be moved to archived.
	_, err = os.Stat(filepath.Join(root, "artifacts", "changes", slug))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	// Archive should exist in bundle archive path.
	_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug, "change.yaml"))
	require.NoError(t, err)
}

func TestArchiveChangeCancelledUsesDedicatedWorktreeBundleForL3(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "l3-change"
	worktreeRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(worktreeRoot, ".slipway.yaml"), []byte("defaults:\n  artifact_schema: expanded\n"), 0o644))

	change := model.NewChange(slug)
	change.NeedsDiscovery = true
	change.WorktreePath = worktreeRoot
	change.Status = model.ChangeStatusActive
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(worktreeRoot, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "change.yaml"), []byte("id: x"), 0o644))

	archived, err := ArchiveChange(root, change, model.ChangeStatusCancelled)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeStatusCancelled, archived.Status)

	_, err = os.Stat(filepath.Join(worktreeRoot, "artifacts", "changes", slug))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug, "change.yaml"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug))
	require.NoError(t, err)
}

func TestArchiveChangeCancelledAllowsUnboundL3BeforeWorktreeBinding(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "l3-unbound"

	change := model.NewChange(slug)
	change.NeedsDiscovery = true
	change.Status = model.ChangeStatusActive
	require.NoError(t, SaveChange(root, change))

	archived, err := ArchiveChange(root, change, model.ChangeStatusCancelled)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeStatusCancelled, archived.Status)

	_, err = os.Stat(BundleChangeFilePath(root, slug))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	// Archived change.yaml should be in bundle archive path.
	_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug, "change.yaml"))
	require.NoError(t, err)
}

func TestArchiveChangeDoneSucceedsWithoutRequirements(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "no-req-change"

	change := model.NewChange(slug)
	change.Artifacts = map[string]model.ArtifactState{
		"intent": {ID: "intent", State: model.ArtifactLifecycleDraft},
	}
	require.NoError(t, SaveChange(root, change))

	changeDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(changeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(changeDir, "change.yaml"), []byte("id: x"), 0o644))

	// No requirements.md — archive should still succeed.
	archived, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeStatusDone, archived.Status)
}

func TestArchiveChangeDoneRequiresActiveStatus(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("my-change")
	change.Status = model.ChangeStatusCancelled

	_, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.Error(t, err)
}

func TestArchiveChangeRejectsEmptySlugBeforeMutation(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	change := model.NewChange("")

	_, err := ArchiveChange(root, change, model.ChangeStatusCancelled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")

	_, statErr := os.Stat(filepath.Join(root, "artifacts", "changes", "change.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestArchiveChangeCancelAcceptsCancelled(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	change := model.NewChange("my-change")
	change.Status = model.ChangeStatusCancelled
	change.Artifacts = map[string]model.ArtifactState{
		"intent": {ID: "intent", State: model.ArtifactLifecycleDraft},
	}
	require.NoError(t, SaveChange(root, change))

	artifactDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(artifactDir, 0o755))

	archived, err := ArchiveChange(root, change, model.ChangeStatusCancelled)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeStatusCancelled, archived.Status)
	assert.Equal(t, model.ArtifactLifecycleFrozen, archived.Artifacts["intent"].State)
}

func TestArchiveChangeDoneMovesRuntimeToArchive(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "my-change"

	change := model.NewChange(slug)
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	archived, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeStatusDone, archived.Status)

	_, err = os.Stat(BundleChangeFilePath(root, slug))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug, "change.yaml"))
	require.NoError(t, err)
}

func TestArchiveChangeDoneMovesEntireChangeDirIncludingEvidence(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "my-change"

	change := model.NewChange(slug)
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	// Create verification subdirectory in bundle (artifacts/changes/{slug}/verification/).
	verifyDir := VerificationDir(root, slug)
	require.NoError(t, os.MkdirAll(verifyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(verifyDir, "plan-audit.yaml"), []byte("verdict: pass\n"), 0o644))

	taskEvidenceDir := EvidenceTasksDir(root, slug, 1)
	require.NoError(t, os.MkdirAll(taskEvidenceDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(taskEvidenceDir, "t1.json"), []byte(`{"task":"t1"}`), 0o644))

	_, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	// Source change directory must be gone entirely.
	_, err = os.Stat(ChangeDir(root, slug))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	// Archive directory must contain change.yaml in bundle archive.
	_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug, "change.yaml"))
	require.NoError(t, err)

	// Verification files move with bundle to archived path.
	archivedVerification := filepath.Join(root, "artifacts", "changes", "archived", slug, "verification", "plan-audit.yaml")
	_, err = os.Stat(archivedVerification)
	require.NoError(t, err)

	// Archived changes no longer retain hidden runtime evidence trees.
	_, err = os.Stat(filepath.Join(ChangeDir(root, slug), "evidence", "tasks", "rv1", "t1.json"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestArchiveChangeFallsBackWhenBundleMoveCrossesFilesystems(t *testing.T) {
	root := createRuntimeLayout(t)
	slug := "cross-device-archive"
	worktreeRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(worktreeRoot, ".slipway.yaml"), []byte("defaults:\n  artifact_schema: expanded\n"), 0o644))

	originalRenameDir := renameDir
	renameDir = func(oldPath, newPath string) error {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: syscall.EXDEV}
	}
	t.Cleanup(func() {
		renameDir = originalRenameDir
	})

	change := model.NewChange(slug)
	change.NeedsDiscovery = true
	change.WorktreePath = worktreeRoot
	change.Status = model.ChangeStatusActive
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(worktreeRoot, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(filepath.Join(bundleDir, "verification"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "verification", "plan-audit.yaml"), []byte("verdict: pass\n"), 0o644))
	require.NoError(t, os.Symlink("plan-audit.yaml", filepath.Join(bundleDir, "verification", "latest.yaml")))

	_, err := ArchiveChange(root, change, model.ChangeStatusCancelled)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug, "verification", "plan-audit.yaml"))
	require.NoError(t, err)
	target, err := os.Readlink(filepath.Join(root, "artifacts", "changes", "archived", slug, "verification", "latest.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "plan-audit.yaml", target)
	_, err = os.Stat(bundleDir)
	assert.True(t, os.IsNotExist(err))
}

func TestArchiveChangeCrossFilesystemFailureDoesNotLeaveArchivedCopyVisible(t *testing.T) {
	root := createRuntimeLayout(t)
	slug := "cross-device-cleanup"
	worktreeRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(worktreeRoot, ".slipway.yaml"), []byte("defaults:\n  artifact_schema: expanded\n"), 0o644))

	originalRenameDir := renameDir
	renameDir = func(oldPath, newPath string) error {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: syscall.EXDEV}
	}

	change := model.NewChange(slug)
	change.NeedsDiscovery = true
	change.WorktreePath = worktreeRoot
	change.Status = model.ChangeStatusActive
	require.NoError(t, SaveChange(root, change))

	srcArtifacts, err := GovernedBundleDir(root, change)
	require.NoError(t, err)
	bundleDir := filepath.Join(worktreeRoot, "artifacts", "changes", slug)
	originalRemoveDirAll := removeDirAll
	removeDirAll = func(path string) error {
		if filepath.Clean(path) == filepath.Clean(srcArtifacts) {
			return errors.New("remove source failed")
		}
		return os.RemoveAll(path)
	}
	t.Cleanup(func() {
		renameDir = originalRenameDir
		removeDirAll = originalRemoveDirAll
	})
	require.NoError(t, os.MkdirAll(filepath.Join(bundleDir, "verification"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	_, err = ArchiveChange(root, change, model.ChangeStatusCancelled)
	require.Error(t, err)

	_, statErr := os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug))
	assert.ErrorIs(t, statErr, os.ErrNotExist, "failed cross-filesystem archive must not leave a visible archived copy behind")
	_, statErr = os.Stat(bundleDir)
	require.NoError(t, statErr, "source bundle should remain in place when archive-forward fails")
}

func TestArchiveChangeRollsBackWhenPersistingArchivedAuthorityFails(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	slug := "archive-write-rollback"
	change := model.NewChange(slug)
	change.Status = model.ChangeStatusActive
	change.Artifacts = map[string]model.ArtifactState{
		"intent": {ID: "intent", State: model.ArtifactLifecycleDraft},
	}
	require.NoError(t, SaveChange(root, change))

	require.NoError(t, os.MkdirAll(ChangeDir(root, slug), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ChangeDir(root, slug), "scratch.txt"), []byte("runtime"), 0o644))

	activeBundleDir := filepath.Join(root, "artifacts", "changes", slug)
	archivedBundleDir := filepath.Join(root, "artifacts", "changes", "archived", slug)
	require.NoError(t, os.Chmod(activeBundleDir, 0o500))
	t.Cleanup(func() {
		for _, dir := range []string{activeBundleDir, archivedBundleDir} {
			if _, err := os.Stat(dir); err == nil {
				_ = os.Chmod(dir, 0o755)
			}
		}
	})

	_, err := ArchiveChange(root, change, model.ChangeStatusCancelled)
	require.Error(t, err)

	loaded, loadErr := LoadChange(root, slug)
	require.NoError(t, loadErr)
	assert.Equal(t, model.ChangeStatusActive, loaded.Status)
	assert.Equal(t, model.ArtifactLifecycleDraft, loaded.Artifacts["intent"].State)

	_, statErr := os.Stat(filepath.Join(activeBundleDir, "change.yaml"))
	require.NoError(t, statErr)
	_, statErr = os.Stat(archivedBundleDir)
	require.ErrorIs(t, statErr, os.ErrNotExist)
	_, statErr = os.Stat(filepath.Join(ChangeDir(root, slug), "scratch.txt"))
	require.NoError(t, statErr)
}

func TestStageAndMoveDirAcrossFilesystemsPreservesSourceOnPromoteFailure(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "source")
	dst := filepath.Join(root, "target")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "tasks.md"), []byte("# Tasks\n"), 0o644))

	originalPromoteDir := promoteDir
	promoteDir = func(oldPath, newPath string) error {
		return errors.New("promote failed")
	}
	t.Cleanup(func() {
		promoteDir = originalPromoteDir
	})

	err := stageAndMoveDirAcrossFilesystems(src, dst)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "promote staged cross-filesystem move")

	_, statErr := os.Stat(filepath.Join(src, "tasks.md"))
	require.NoError(t, statErr, "source bundle must remain when staged promotion fails")

	_, statErr = os.Stat(dst)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "failed staged promotion must not leave a visible target copy behind")

	stagedDirs, globErr := filepath.Glob(filepath.Join(filepath.Dir(dst), filepath.Base(dst)+".staging-*"))
	require.NoError(t, globErr)
	assert.Empty(t, stagedDirs, "failed staged promotion must clean up temporary staging directories")
}

func TestStageAndMoveDirAcrossFilesystemsReportsRollbackFailure(t *testing.T) {
	root := t.TempDir()
	srcParent := filepath.Join(root, "src-parent")
	dstParent := filepath.Join(root, "dst-parent")
	src := filepath.Join(srcParent, "source")
	dst := filepath.Join(dstParent, "target")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.MkdirAll(dstParent, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "tasks.md"), []byte("# Tasks\n"), 0o644))

	originalPromoteDir := promoteDir
	promoteDir = func(oldPath, newPath string) error {
		return os.Rename(oldPath, newPath)
	}
	originalRemoveDirAll := removeDirAll
	removeDirAll = func(path string) error {
		clean := filepath.Clean(path)
		switch clean {
		case filepath.Clean(src):
			return errors.New("remove source failed")
		case filepath.Clean(dst):
			return errors.New("rollback failed")
		default:
			return os.RemoveAll(path)
		}
	}
	t.Cleanup(func() {
		promoteDir = originalPromoteDir
		removeDirAll = originalRemoveDirAll
	})

	err := stageAndMoveDirAcrossFilesystems(src, dst)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove source after promoting cross-filesystem move")
	assert.Contains(t, err.Error(), "rollback failed")

	_, statErr := os.Stat(src)
	require.NoError(t, statErr, "source directory should remain when rollback also fails")
	_, statErr = os.Stat(dst)
	require.NoError(t, statErr, "rollback failure should leave the promoted target directory visible for manual cleanup")
}

func TestCopyDirRecursivePreservesSymlinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	src := filepath.Join(root, "source")
	dst := filepath.Join(root, "target")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "tasks.md"), []byte("# Tasks\n"), 0o644))
	require.NoError(t, os.Symlink("tasks.md", filepath.Join(src, "tasks.link")))

	require.NoError(t, copyDirRecursive(src, dst))

	linkTarget, err := os.Readlink(filepath.Join(dst, "tasks.link"))
	require.NoError(t, err)
	assert.Equal(t, "tasks.md", linkTarget)

	content, err := os.ReadFile(filepath.Join(dst, "tasks.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Tasks\n", string(content))
}

func TestScrubArchivedExecutionSummaryRuntimeEvidenceRefsDirect(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	slug := "scrub-archived-summary"
	verifyDir := filepath.Join(root, "artifacts", "changes", "archived", slug, "verification")
	require.NoError(t, os.MkdirAll(verifyDir, 0o755))
	change := model.NewChange(slug)
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	changeRaw, err := yaml.Marshal(change)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(BundleArchivedChangeFilePath(root, slug), changeRaw, 0o644))

	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:      "task-a",
				Verdict:     model.TaskVerdictPass,
				TaskKind:    model.TaskKindCode,
				EvidenceRef: filepath.ToSlash(filepath.Join(".git", "slipway", "runtime", "changes", slug, "evidence", "tasks", "rv1", "task-a.json")),
			},
			{
				TaskID:      "task-b",
				Verdict:     model.TaskVerdictPass,
				TaskKind:    model.TaskKindDoc,
				EvidenceRef: filepath.ToSlash(filepath.Join("artifacts", "changes", "archived", slug, "verification", "task-b.md")),
			},
		},
	}
	summary.Normalize()
	summary.SyncDerivedFields()
	raw, err := yaml.Marshal(summary)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(verifyDir, ExecutionSummaryFileName), raw, 0o644))

	require.NoError(t, scrubArchivedExecutionSummaryRuntimeEvidenceRefs(root, slug))

	scrubbed, err := LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	require.Empty(t, scrubbed.Tasks[0].EvidenceRef)
	require.Equal(t, filepath.ToSlash(filepath.Join("artifacts", "changes", "archived", slug, "verification", "task-b.md")), scrubbed.Tasks[1].EvidenceRef)
}

func TestScrubArchivedExecutionSummaryMalformedSummaryRemovesFile(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	slug := "scrub-malformed-archived-summary"
	verifyDir := filepath.Join(root, "artifacts", "changes", "archived", slug, "verification")
	require.NoError(t, os.MkdirAll(verifyDir, 0o755))

	summaryPath := filepath.Join(verifyDir, ExecutionSummaryFileName)
	require.NoError(t, os.WriteFile(summaryPath, []byte("tasks: [\n"), 0o644))

	require.NoError(t, scrubArchivedExecutionSummaryRuntimeEvidenceRefs(root, slug))

	_, err := os.Stat(summaryPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestArchiveChangeScrubsRuntimeEvidenceRefsFromChange(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "cancel-evidence-scrub"

	change := model.NewChange(slug)
	change.Status = model.ChangeStatusCancelled
	// Simulate cancel writing a preemption evidence ref as absolute path.
	evidencePath := filepath.Join(ChangeDir(root, slug), "evidence", "tasks", "cancel", "12345.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(evidencePath), 0o755))
	require.NoError(t, os.WriteFile(evidencePath, []byte(`{"outcome":"cancelled"}`), 0o644))
	change.EvidenceRefs["cancel_preemption_12345"] = evidencePath
	// Inline text ref should survive.
	change.EvidenceRefs["plan_audit.last_checker_feedback"] = "intent drift detected"
	require.NoError(t, SaveChange(root, change))

	_, err := ArchiveChange(root, change, model.ChangeStatusCancelled)
	require.NoError(t, err)

	raw, err := os.ReadFile(BundleArchivedChangeFilePath(root, slug))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "evidence_refs:")

	archived, err := LoadArchivedChange(root, slug)
	require.NoError(t, err)

	// Absolute-path evidence ref must be scrubbed.
	assert.NotContains(t, archived.EvidenceRefs, "cancel_preemption_12345")
	// Inline text evidence ref must survive.
	assert.Equal(t, "intent drift detected", archived.EvidenceRefs["plan_audit.last_checker_feedback"])
}

func TestArchiveChangeRemovesGitScopedSnapshotAndTaskPID(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	slug := "git-scoped-sidecars"

	change := model.NewChange(slug)
	require.NoError(t, SaveChange(root, change))

	snapshotPath := GovernanceSnapshotCachePath(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
	require.NoError(t, os.WriteFile(snapshotPath, []byte("version: 2\n"), 0o644))

	pidPath := TaskPIDFilePath(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(pidPath), 0o755))
	require.NoError(t, os.WriteFile(pidPath, []byte("[123]"), 0o644))

	_, err := ArchiveChange(root, change, model.ChangeStatusCancelled)
	require.NoError(t, err)

	_, err = os.Stat(snapshotPath)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(pidPath)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestArchiveChangeNotDiscoverableAfterArchive(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "my-change"

	change := model.NewChange(slug)
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	// Before archive, the change should be discoverable.
	changes, err := ListChanges(root)
	require.NoError(t, err)
	found := false
	for _, c := range changes {
		if c.Slug == slug {
			found = true
		}
	}
	assert.True(t, found, "change should be discoverable before archive")

	// Transition to done status and archive.
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	_, err = ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	// After archive, the change must not be discoverable.
	changes, err = ListChanges(root)
	require.NoError(t, err)
	for _, c := range changes {
		assert.NotEqual(t, slug, c.Slug, "archived change must not appear in active changes")
	}
}

func TestContextDependenciesPersistsThroughArchive(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "my-change"

	change := model.NewChange(slug)
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = "security_credentials"
	change.ContextDependencies = model.ContextDependencies{
		Requires: []model.ContextRequirement{
			{Slug: "baseline-auth", Provides: []string{"auth-contract"}},
		},
	}
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", slug), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "artifacts", "changes", slug, "change.yaml"), []byte("x"), 0o644))

	_, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	raw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", "archived", slug, "change.yaml"))
	require.NoError(t, err)
	var archived model.Change
	require.NoError(t, yaml.Unmarshal(raw, &archived))
	assert.Equal(t, change.ContextDependencies, archived.ContextDependencies)
}

func TestDecodeAndValidateChangeRejectsPersistedGateReceipts(t *testing.T) {
	t.Parallel()

	raw := []byte(`
slug: gate-receipt
status: active
current_state: S1_PLAN
plan_substep: bundle
gates:
  G_plan:
    gate_id: G_plan
    status: approved
`)

	_, err := decodeAndValidateChange(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gates")
}

func TestLoadArchivedChangeMissingBundleReturnsNotExist(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "archive-missing-bundle"

	_, err := LoadArchivedChange(root, slug)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoadArchivedChangeReadsBundleArchiveFirst(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	slug := "archived-bundle-first"

	change := model.NewChange(slug)
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	_, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	loaded, err := LoadArchivedChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.ChangeStatusDone, loaded.Status)
	assert.Equal(t, model.StateDone, loaded.CurrentState)
}
