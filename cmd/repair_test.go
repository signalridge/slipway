package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRepairFocusFlagHelpDoesNotAdvertiseSast(t *testing.T) {
	t.Parallel()
	// repair removed the false-promise `sast` focus (issue #88); the --focus flag
	// help must not keep advertising it (it was "Repair focus (e.g. sast)").
	flag := makeRepairCmd().Flags().Lookup("focus")
	require.NotNil(t, flag)
	assert.NotContains(t, strings.ToLower(flag.Usage), "sast",
		"repair --focus help must not advertise the removed sast focus, got %q", flag.Usage)
}

func TestRepairRejectsPlainGitRepoWithoutSlipwayMarkers(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644))

		err := makeRepairCmd().Execute()
		require.Error(t, err)
		assert.ErrorIs(t, err, fsutil.ErrProjectRootNotFound)

		_, statErr := os.Stat(state.ConfigPath(root))
		require.Error(t, statErr)
		assert.ErrorIs(t, statErr, os.ErrNotExist)
	})
}

func TestWriteRepairTextDoesNotReportNoopWhenUnrepairedDriftExists(t *testing.T) {
	var out bytes.Buffer

	err := writeRepairText(&out, repairSummary{
		UnrepairedDrift: []repairDriftFinding{{
			Reason:     state.StalePlanningEvidenceBlockerToken,
			Target:     "artifacts/changes/demo/tasks.md",
			NextAction: "regenerate wave plan from the current tasks.md",
		}},
	})

	require.NoError(t, err)
	text := out.String()
	assert.Contains(t, text, "Unrepaired drift:")
	assert.Contains(t, text, state.StalePlanningEvidenceBlockerToken)
	assert.NotContains(t, text, "No repairs were needed")
}

func TestApplyConfigRepairResultSurfacesFailureAsNonRepairableFinding(t *testing.T) {
	t.Parallel()

	// RED rationale: the pre-fix call site was
	//   `if backupPath, err := state.RepairCorruptConfig(...); err == nil { ... }`
	// which took the success branch and SWALLOWED a non-nil error — a config
	// repair FAILURE produced no finding and no ConfigBackupPath, making it
	// indistinguishable from "nothing to repair". `loadConfigAtRoot` re-reads the
	// same config immediately after and early-returns on any on-disk state that
	// makes RepairCorruptConfig fail, so the error branch is unreachable through
	// the full command; this exercises the extracted seam directly. After the fix
	// a non-nil error must surface as a non-repairable finding and must NOT record
	// a (success-only) backup path.
	failure := repairSummary{}
	applyConfigRepairResult(&failure, "unused-success-backup-path", errors.New("backup write failed"))

	require.Len(t, failure.NonRepairableFindings, 1)
	assert.Contains(t, failure.NonRepairableFindings[0], "config repair failed")
	assert.Contains(t, failure.NonRepairableFindings[0], "backup write failed")
	assert.Empty(t, failure.ConfigBackupPath, "a failed config repair must not report a success-path backup")

	// The success path is preserved: a nil error records the backup path and adds
	// no finding (behavior-preserving guarantee for the unchanged happy path).
	success := repairSummary{}
	backupPath := "/backups/slipway.yaml.broken.20260704T000000Z.yaml"
	applyConfigRepairResult(&success, backupPath, nil)
	assert.Equal(t, backupPath, success.ConfigBackupPath)
	assert.Empty(t, success.NonRepairableFindings)
}

// TestRepairSurfacesUnrepairableConfigInsteadOfHardFailing covers the full
// `slipway repair --json` command path that the seam test above cannot reach: a
// config that stays unreadable after RepairCorruptConfig fails must surface as a
// non-repairable finding in the summary, not early-return config_parse_failure
// with an empty stdout that tells the operator to run the command they just ran
// (REQ-002).
func TestRepairSurfacesUnrepairableConfigInsteadOfHardFailing(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Corrupt the config so it no longer parses, then block the config backup
		// directory by planting a regular file at its path so RepairCorruptConfig
		// cannot back up + rewrite it. The config therefore stays unreadable for
		// the whole command.
		require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("broken: [unterminated\n"), 0o644))
		backupDir := state.ConfigBackupDir(root)
		require.NoError(t, os.MkdirAll(filepath.Dir(backupDir), 0o755))
		require.NoError(t, os.WriteFile(backupDir, []byte("blocker"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		// Must NOT hard-fail: the summary has to reach stdout so the operator sees
		// the finding rather than a bare config_parse_failure.
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		joined := strings.Join(summary.NonRepairableFindings, "\n")
		assert.Contains(t, joined, "config unreadable after repair",
			"expected the unrepairable-config finding in the repair summary")
		assert.Contains(t, joined, "manually",
			"config finding must direct the operator to manual correction, not a repair re-run")
	})
}

func TestRepairRestoresMissingBoundWorktreeScopeMetadata(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair restores missing bound worktree scope metadata")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		worktreeRoot := filepath.Join(t.TempDir(), slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		require.NoError(t, os.Remove(state.ConfigPath(worktreeRoot)))
		require.NoError(t, os.Remove(state.WorkspaceScopeMarkerPath(worktreeRoot)))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		_, err = os.Stat(state.ConfigPath(worktreeRoot))
		require.NoError(t, err)
		_, err = os.Stat(state.WorkspaceScopeMarkerPath(worktreeRoot))
		require.NoError(t, err)
		assert.Contains(t, summary.WorktreeScopeRepairs, slug)
	})
}

func TestRepairRejectsEmptyDirectoryWithoutSlipwayMarkers(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		err := makeRepairCmd().Execute()
		require.Error(t, err)
		assert.ErrorIs(t, err, fsutil.ErrProjectRootNotFound)

		_, statErr := os.Stat(state.ConfigPath(root))
		require.Error(t, statErr)
		assert.ErrorIs(t, statErr, os.ErrNotExist)
	})
}

func TestRepairRecoversMissingConfigWhenGitScopedRuntimeMarkerExists(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "slipway", "runtime"), 0o755))

		require.NoError(t, makeRepairCmd().Execute())

		cfg, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		assert.Equal(t, model.ArtifactSchemaExpanded, cfg.Defaults.ArtifactSchema)
	})
}

func TestRepairRecoversMissingConfigFromNestedDirectoryWhenRootMarkersExist(t *testing.T) {
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	nested := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.MkdirAll(state.GitRuntimeDir(root), 0o755))

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(nested))
	defer func() {
		_ = os.Chdir(previousWD)
	}()

	require.NoError(t, makeRepairCmd().Execute())

	cfg, err := model.LoadConfig(state.ConfigPath(root))
	require.NoError(t, err)
	assert.Equal(t, model.ArtifactSchemaExpanded, cfg.Defaults.ArtifactSchema)

	_, statErr := os.Stat(state.ConfigPath(nested))
	require.Error(t, statErr)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestCleanupUnheldLockAnchorsToleratesPerAnchorErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// An anchor whose parent path is a regular file makes the internal meta/anchor
	// stat fail with a non-ENOENT (ENOTDIR) error, exercising the per-anchor error
	// path. Best-effort hygiene must skip it rather than abort the whole repair.
	notADir := filepath.Join(root, "notadir")
	require.NoError(t, os.WriteFile(notADir, []byte(""), 0o644))
	failingAnchor := filepath.Join(notADir, "state.lock")

	// A normal unheld empty anchor that must still be cleaned despite the sibling
	// failure listed before it.
	cleanAnchor := filepath.Join(root, "locks", "state.lock")
	require.NoError(t, os.MkdirAll(filepath.Dir(cleanAnchor), 0o755))
	require.NoError(t, os.WriteFile(cleanAnchor, []byte(""), 0o644))

	cleaned := cleanupUnheldLockAnchors(root, []string{failingAnchor, cleanAnchor})

	assert.Equal(t, []string{state.DisplayPath(root, cleanAnchor)}, cleaned)
	assert.NoFileExists(t, cleanAnchor)
}

func TestRepairDoesNotCleanFreshAtomicTempArtifacts(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		tmpDir := filepath.Join(root, ".git", "slipway", "locks", "changes")
		require.NoError(t, os.MkdirAll(tmpDir, 0o755))
		freshTemp := filepath.Join(tmpDir, ".tmp-demo.lock.meta-fresh")
		staleTemp := filepath.Join(tmpDir, ".tmp-demo.lock.meta-stale")
		require.NoError(t, os.WriteFile(freshTemp, []byte("fresh"), 0o644))
		require.NoError(t, os.WriteFile(staleTemp, []byte("stale"), 0o644))
		old := time.Now().Add(-5 * time.Minute)
		require.NoError(t, os.Chtimes(staleTemp, old, old))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		hasCleanedSuffix := func(suffix string) bool {
			for _, cleaned := range summary.CleanedAtomicTemps {
				if strings.HasSuffix(cleaned, suffix) {
					return true
				}
			}
			return false
		}
		staleSuffix := filepath.Join(".git", "slipway", "locks", "changes", ".tmp-demo.lock.meta-stale")
		freshSuffix := filepath.Join(".git", "slipway", "locks", "changes", ".tmp-demo.lock.meta-fresh")
		assert.True(t, hasCleanedSuffix(staleSuffix))
		assert.False(t, hasCleanedSuffix(freshSuffix))

		_, err := os.Stat(freshTemp)
		require.NoError(t, err)
		_, err = os.Stat(staleTemp)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestRepairRecoversMissingConfigAfterInitWorkspace(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		runGit(t, root, "init")
		runGit(t, root, "commit", "--allow-empty", "-m", "init")

		initTestWorkspace(t, root)
		require.NoError(t, os.Remove(state.ConfigPath(root)))

		require.NoError(t, makeRepairCmd().Execute())

		cfg, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		assert.Equal(t, model.ArtifactSchemaExpanded, cfg.Defaults.ArtifactSchema)
	})
}

func TestRepairRepairsInterruptedArchivedBundleResidue(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := "interrupted-archive"
		change := model.NewChange(slug)
		change.Status = model.ChangeStatusActive
		change.CurrentState = model.StateDone
		change.PlanSubStep = model.PlanSubStepNone
		change.Artifacts = map[string]model.ArtifactState{
			"intent": {ID: "intent", State: model.ArtifactLifecycleDraft},
		}
		require.NoError(t, state.SaveChange(root, change))

		activeBundleDir := filepath.Dir(state.BundleChangeFilePath(root, slug))
		archivedBundleDir := filepath.Join(state.ArchivedBundlesDir(root), slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(archivedBundleDir), 0o755))
		require.NoError(t, os.Rename(activeBundleDir, archivedBundleDir))

		require.NoError(t, os.MkdirAll(state.ChangeDir(root, slug), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Dir(state.TaskPIDFilePath(root, slug)), 0o755))
		require.NoError(t, os.WriteFile(state.TaskPIDFilePath(root, slug), []byte("[]"), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Dir(state.GovernanceSnapshotCachePath(root, slug)), 0o755))
		require.NoError(t, os.WriteFile(state.GovernanceSnapshotCachePath(root, slug), []byte("version: 1\n"), 0o644))

		require.NoError(t, makeRepairCmd().Execute())

		archived, err := state.LoadArchivedChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeStatusDone, archived.Status)
		assert.Equal(t, model.ArtifactLifecycleFrozen, archived.Artifacts["intent"].State)

		_, err = os.Stat(state.ChangeDir(root, slug))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))

		_, err = os.Stat(filepath.Dir(state.TaskPIDFilePath(root, slug)))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))

		_, err = os.Stat(filepath.Dir(state.GovernanceSnapshotCachePath(root, slug)))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestRepairReportsOrphanBundleDirectoryFinding(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", "orphan-dir"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "artifacts", "changes", "orphan-dir", "intent.md"), []byte("# orphan\n"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, "orphan-dir") && strings.Contains(finding, "change.yaml") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected orphan bundle finding in repair summary")
	})
}

func TestRepairRemovesEmptyOrphanBundleDirectory(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		emptyResidue := filepath.Join(root, "artifacts", "changes", "empty-residue")
		require.NoError(t, os.MkdirAll(filepath.Join(emptyResidue, "verification"), 0o755))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.Contains(t, summary.RemovedEmptyOrphanBundles, "empty-residue")
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "empty_orphan_bundle", Target: "empty-residue"})
		assert.Empty(t, summary.NonRepairableFindings)
		require.NoDirExists(t, emptyResidue)
	})
}

func TestRepairReportsUnreadableChangeAuthorityFinding(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		change := model.NewChange("corrupt-change")
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, os.WriteFile(state.BundleChangeFilePath(root, change.Slug), []byte("slug: corrupt-change\ncurrent_state: [\n"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, "corrupt-change") && strings.Contains(finding, "unreadable") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected unreadable change authority finding in repair summary")
	})
}

func TestRepairReportsHiddenUnreadableChangeAuthorityFinding(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair reports hidden unreadable authority")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		worktreeRoot := filepath.Join(t.TempDir(), slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		require.NoError(t, os.Remove(state.ConfigPath(worktreeRoot)))
		require.NoError(t, os.Remove(state.WorkspaceScopeMarkerPath(worktreeRoot)))
		require.NoError(t, os.WriteFile(state.BundleChangeFilePath(worktreeRoot, slug), []byte("slug: hidden\ncurrent_state: [\n"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, slug) && strings.Contains(finding, "change authority unreadable") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected hidden unreadable change authority finding in repair summary")
	})
}

func TestRepairReportsUnreadableExecutionSummaryFinding(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair reports unreadable execution summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		summaryPath := executionSummaryPathForTest(root, slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(summaryPath), 0o755))
		require.NoError(t, os.WriteFile(summaryPath, []byte("version: ["), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, slug) && strings.Contains(finding, "execution summary unreadable") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected execution summary finding in repair summary")
	})
}

func TestRepairRebuildsUnreadableExecutionSummaryWithoutResidualDrift(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair should converge unreadable execution summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` rebuild unreadable execution summary
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		tasksPlanHash, err := state.CurrentTasksPlanStructuralState(root, change)
		require.NoError(t, err)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"changed_files":    []string{"cmd/repair.go"},
			"target_files":     []string{"cmd/repair.go"},
			"freshness_inputs": state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01", tasksPlanHash),
		})
		writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
		})

		summaryPath := executionSummaryPathForTest(root, slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(summaryPath), 0o755))
		require.NoError(t, os.WriteFile(summaryPath, []byte("version: ["), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.Contains(t, summary.MaterializedWavePlans, slug)
		assert.Contains(t, summary.RebuiltExecutionSummaries, slug)
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "materialized_wave_plan", Target: slug})
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "rebuilt_execution_summary", Target: slug})

		for _, finding := range summary.NonRepairableFindings {
			assert.False(t,
				strings.Contains(finding, slug) && strings.Contains(finding, "execution summary unreadable"),
				"rebuilt summary must not leave unreadable finding: %s", finding,
			)
		}
		for _, drift := range summary.UnrepairedDrift {
			assert.False(t,
				strings.Contains(drift.Target, slug) && strings.Contains(drift.Reason, "execution summary unreadable"),
				"rebuilt summary must not leave unreadable drift: %+v", drift,
			)
		}

		rebuilt, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		assert.True(t, state.ExecutionSummaryReady(&rebuilt))
	})
}

func TestBuildUnrepairedDriftFindingsKeepsActionableTargets(t *testing.T) {
	t.Parallel()

	drift := buildUnrepairedDriftFindings([]string{
		"bundle directory exists without change.yaml: orphan-dir",
		"multiple active changes are active: demo-a, demo-b",
		"demo: execution summary unreadable: bad yaml",
	})

	require.Len(t, drift, 3)
	assert.Contains(t, drift, repairDriftFinding{
		Target:     filepath.ToSlash(filepath.Join("artifacts", "changes", "orphan-dir")),
		Reason:     "bundle directory exists without change.yaml",
		NextAction: "repair or replace the authoritative change.yaml before continuing",
	})
	assert.Contains(t, drift, repairDriftFinding{
		Target:     "demo-a, demo-b",
		Reason:     "multiple active changes are active",
		NextAction: "run `slipway status` to inspect, then resolve one with `slipway cancel --change <slug>` or `slipway done --change <slug>`",
	})
	assert.Contains(t, drift, repairDriftFinding{
		Target:     "demo",
		Reason:     "execution summary unreadable: bad yaml",
		NextAction: "regenerate execution-summary.yaml from current wave-backed task evidence",
	})
	// Dead-end strings (#86) must not survive.
	for _, f := range drift {
		assert.NotEqual(t, "inspect the named artifact and rerun the owning Slipway command after correction", f.NextAction)
	}
}

func TestRepairMaterializesWavePlanAndRecoversWaveRuns(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair should recover wave execution artifacts")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` recover wave execution state
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.Contains(t, summary.MaterializedWavePlans, slug)
		assert.Contains(t, summary.RecoveredWaveRuns, slug)
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "materialized_wave_plan", Target: slug})
		assert.Contains(t, summary.AppliedRepairs, repairAppliedFinding{Kind: "recovered_wave_run", Target: slug})

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		_, err = state.LoadWavePlanForChange(root, change)
		require.NoError(t, err)
		runs, err := state.LoadWaveRuns(root, slug, 1)
		require.NoError(t, err)
		require.Len(t, runs, 1)
	})
}

func TestRepairRebuildsUnreadableWavePlanAndWaveRuns(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair should rebuild unreadable wave artifacts")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` rebuild unreadable wave artifacts
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)

		require.NoError(t, os.WriteFile(state.WavePlanPathForRead(root, slug), []byte("version: [\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(state.WaveEvidenceDir(root, slug), "wave-01.yaml"), []byte("wave_index: [\n"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.Contains(t, summary.MaterializedWavePlans, slug)
		assert.Contains(t, summary.RecoveredWaveRuns, slug)

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		plan, err := state.LoadWavePlanForChange(root, change)
		require.NoError(t, err)
		assert.Equal(t, model.WavePlanVersion, plan.Version)

		runs, err := state.LoadWaveRuns(root, slug, 1)
		require.NoError(t, err)
		require.Len(t, runs, 1)
		assert.Equal(t, model.WaveVerdictPass, runs[0].Verdict)
	})
}

func TestRepairReportsMalformedTaskEvidenceWithoutFailing(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair reports malformed task evidence")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` report malformed task evidence
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		require.NoError(t, os.MkdirAll(state.EvidenceTasksDir(root, slug), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(state.EvidenceTasksDir(root, slug), "broken.json"), []byte("{"), 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, slug) &&
				strings.Contains(finding, "task evidence unreadable") &&
				strings.Contains(finding, "broken.json") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected repair to report malformed task evidence without failing")
	})
}

func TestRepairRebuildsWavePlanButPreservesHistoricalExecutionEvidenceWhenTasksDrifted(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair should not rewrite drifted historical execution")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` historical executed task
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_id":  "t-01",
			"verdict":  "pass",
			"evidence": "historical",
		})

		tasksPath := filepath.Join(bundlePath, "tasks.md")
		updatedAt := time.Now().UTC().Add(2 * time.Second)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-02`"+` replacement task after drift
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		require.NoError(t, os.Chtimes(tasksPath, updatedAt, updatedAt))

		evidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		_, err = os.Stat(evidencePath)
		require.NoError(t, err)

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		assert.Contains(t, summary.MaterializedWavePlans, slug)
		assert.NotContains(t, summary.RecoveredWaveRuns, slug)
		assert.NotContains(t, summary.PrunedTaskEvidence, filepath.ToSlash(filepath.Join(slug, "t-01.json")))

		foundBlocked := false
		for _, finding := range summary.NonRepairableFindings {
			if strings.Contains(finding, slug) && strings.Contains(finding, "wave plan repair blocked") {
				foundBlocked = true
				break
			}
		}
		assert.False(t, foundBlocked, "wave-plan drift must rebuild the derived plan instead of becoming non-repairable")
		for _, drift := range summary.UnrepairedDrift {
			if drift.Target == slug && strings.Contains(drift.Reason, "wave plan repair blocked") {
				t.Fatalf("wave-plan drift must not remain a repair blocker: %+v", drift)
			}
		}

		_, err = os.Stat(evidencePath)
		require.NoError(t, err, "historical task evidence must be preserved")

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		wavePlan, err := state.LoadWavePlanForChange(root, change)
		require.NoError(t, err, "repair must materialize the current derived wave-plan")
		plannedTasks := state.PlannedTaskIDSet(wavePlan)
		assert.Contains(t, plannedTasks, "t-02")
		assert.NotContains(t, plannedTasks, "t-01")
	})
}

func TestRepairRebuildsReadyButStaleExecutionSummaryDrift(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair reports stale ready execution summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` rebuild first stale task
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
- [ ] `+"`t-02`"+` rebuild second stale task
  - depends_on: []
  - target_files: ["docs/commands.md"]
  - task_kind: code
`)))
		wavePlan, err := state.MaterializeWavePlan(root, change)
		require.NoError(t, err)
		runtimeCapturedAt := time.Now().UTC()
		summaryCapturedAt := runtimeCapturedAt.Add(time.Hour)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_id":     "t-01",
			"captured_at": runtimeCapturedAt.Format(time.RFC3339Nano),
		})
		writeTaskEvidenceFile(t, root, slug, 1, "t-02", map[string]any{
			"task_id":     "t-02",
			"captured_at": runtimeCapturedAt.Format(time.RFC3339Nano),
		})
		writePassingWaveEvidence(t, root, slug, 1)
		staleSummary := model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        summaryCapturedAt,
			OverallVerdict:    model.ExecutionVerdictPass,
			TasksPlanHash:     wavePlan.TasksPlanHash,
			CompletedTasks:    []string{"t-01", "t-02"},
			Tasks: []model.ExecutionTaskSummary{{
				TaskID:          "t-01",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"cmd/repair.go"},
				CapturedAt:      summaryCapturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01", "previous-task-plan-hash"),
			}, {
				TaskID:          "t-02",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"docs/commands.md"},
				CapturedAt:      summaryCapturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-02", "previous-task-plan-hash"),
			}},
		}
		staleSummary.Normalize()
		rawSummary, err := yaml.Marshal(staleSummary)
		require.NoError(t, err)
		summaryPath := executionSummaryPathForTest(root, slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(summaryPath), 0o755))
		require.NoError(t, fsutil.WriteFileAtomic(summaryPath, rawSummary, 0o644))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		assert.Contains(t, summary.RebuiltExecutionSummaries, slug)
		for _, drift := range summary.UnrepairedDrift {
			assert.False(t, strings.Contains(drift.Target, slug), "rebuilt stale summary must not remain unrepaired: %+v", drift)
		}
		require.Contains(t, summary.PathAuthority, slug)
		taskEvidencePath := summary.PathAuthority[slug].TaskEvidencePath
		assert.True(t, filepath.IsAbs(taskEvidencePath))
		assert.True(t, strings.HasSuffix(taskEvidencePath, "/.git/slipway/runtime/changes/"+slug+"/evidence/tasks"), taskEvidencePath)

		rebuilt, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		require.Len(t, rebuilt.Tasks, 2)
		for _, task := range rebuilt.Tasks {
			assert.True(t, runtimeCapturedAt.Equal(task.CapturedAt), "task %s captured_at was not rebuilt from runtime evidence", task.TaskID)
		}
	})
}

func TestRepairDoesNotRewriteReadyButStaleExecutionSummaryWhenTaskEvidenceInvalid(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair leaves invalid task evidence alone")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` keep valid stale task
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
- [ ] `+"`t-02`"+` leave invalid task evidence unrepaired
  - depends_on: []
  - target_files: ["docs/commands.md"]
  - task_kind: code
`)))
		writePassingWaveEvidence(t, root, slug, 1)

		runtimeCapturedAt := time.Now().UTC()
		summaryCapturedAt := runtimeCapturedAt.Add(time.Hour)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_id":     "t-01",
			"captured_at": runtimeCapturedAt.Format(time.RFC3339Nano),
		})
		writeTaskEvidenceFile(t, root, slug, 1, "t-02", map[string]any{
			"task_id":     "t-02",
			"task_kind":   "not-a-task-kind",
			"captured_at": runtimeCapturedAt.Format(time.RFC3339Nano),
		})
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        summaryCapturedAt,
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"t-01", "t-02"},
			Tasks: []model.ExecutionTaskSummary{{
				TaskID:          "t-01",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"cmd/repair.go"},
				CapturedAt:      summaryCapturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01"),
			}, {
				TaskID:          "t-02",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"docs/commands.md"},
				CapturedAt:      summaryCapturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-02"),
			}},
		})
		summaryPath := executionSummaryPathForTest(root, slug)
		before, err := os.ReadFile(summaryPath)
		require.NoError(t, err)

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.NotContains(t, summary.RebuiltExecutionSummaries, slug)

		foundInvalidEvidence := false
		for _, drift := range summary.UnrepairedDrift {
			if drift.Target == slug && strings.Contains(drift.Reason, "task_evidence_invalid:t-02.json") {
				foundInvalidEvidence = true
				assert.Contains(t, drift.NextAction, "execution-summary.yaml")
			}
		}
		assert.True(t, foundInvalidEvidence, "expected invalid task evidence to remain unrepaired")

		after, err := os.ReadFile(summaryPath)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after), "repair must not rewrite execution-summary.yaml when task evidence is invalid")
	})
}

func TestRepairReportsMissingRuntimeTaskEvidenceWithCommandHint(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair reports missing task evidence source")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writePassingWaveEvidence(t, root, slug, 1)

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.NotContains(t, summary.RebuiltExecutionSummaries, slug)

		found := false
		for _, drift := range summary.UnrepairedDrift {
			if drift.Target != slug || !strings.Contains(drift.Reason, "missing_task_evidence_for_run_summary") {
				continue
			}
			found = true
			assert.Contains(t, drift.Reason, "record_command=slipway evidence task --task-id <task_id> --verdict <verdict> --evidence-ref <ref> [--changed-file <path> ...] --json")
			assert.Contains(t, drift.Reason, "host_fields=task_id,verdict,evidence_ref,changed_files,no_op_justification,blockers,session_id")
			assert.NotContains(t, drift.Reason, "required_fields=task_id,run_summary_version,task_kind")
		}
		assert.True(t, found, "expected repair to report missing runtime task evidence for %s", slug)
	})
}

func TestRepairDoesNotRebuildWhenPlanningEvidenceIsStale(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair leaves stale planning evidence alone")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "requirements.md", []byte(`# Requirements
### Requirement: Original
REQ-001: Original requirement.
`)))
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` preserve planning drift
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`)))
		wavePlan, err := state.MaterializeWavePlan(root, change)
		require.NoError(t, err)
		writePassingWaveEvidence(t, root, slug, 1)

		capturedAt := time.Now().UTC()
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"captured_at": capturedAt.Format(time.RFC3339Nano),
		})
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        capturedAt,
			OverallVerdict:    model.ExecutionVerdictPass,
			TasksPlanHash:     wavePlan.TasksPlanHash,
			CompletedTasks:    []string{"t-01"},
			Tasks: []model.ExecutionTaskSummary{{
				TaskID:       "t-01",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"cmd/repair.go"},
				CapturedAt:   capturedAt,
			}},
		})

		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` preserve changed planning drift
  - depends_on: []
  - target_files: ["cmd/repair.go", "cmd/run.go"]
  - task_kind: code
`)))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		assert.NotContains(t, summary.RebuiltExecutionSummaries, slug)

		for _, drift := range summary.UnrepairedDrift {
			assert.False(t,
				strings.Contains(drift.Target, "tasks.md") &&
					strings.Contains(drift.Reason, state.StalePlanningEvidenceBlockerToken),
				"S3 task-plan amendments belong to review/fix, not local repair",
			)
		}
	})
}

func TestRepairRoutesStaleGovernanceDigestToSlipwayRun(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair routes stale digest to run")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		verdictAt := time.Now().UTC().Add(-time.Hour)
		rec := model.VerificationRecord{
			Verdict:   model.VerificationVerdictPass,
			Blockers:  []model.ReasonCode{},
			Timestamp: verdictAt,
		}
		writeSkillVerification(t, root, slug, progression.SkillPlanAudit, rec)
		require.NoError(t, progression.StampEvidenceDigestForSkill(root, change, progression.SkillPlanAudit, rec, nil))

		// Drift a plan-audit input after the verdict so its digest goes stale.
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		requirementsPath := filepath.Join(bundlePath, "requirements.md")
		require.NoError(t, os.WriteFile(requirementsPath, []byte("# Requirements\nREQ-001 changed after verdict\n"), 0o644))
		afterVerdict := verdictAt.Add(time.Hour)
		require.NoError(t, os.Chtimes(requirementsPath, afterVerdict, afterVerdict))

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))

		found := false
		for _, drift := range summary.UnrepairedDrift {
			if drift.Target == progression.SkillPlanAudit && strings.Contains(drift.Reason, "evidence digest") {
				found = true
				assert.Contains(t, drift.NextAction, "slipway run")
				assert.Contains(t, drift.NextAction, "plan-audit")
			}
		}
		assert.True(t, found, "expected repair to route the stale plan-audit digest through slipway run")
	})
}

func TestRepairDriftNextActionDigestGuidanceUsesRun(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		reason string
		target string
	}{
		{name: "required_skill_stale with skill target", reason: "required_skill_stale: plan-audit:requirements.md", target: "plan-audit"},
		{name: "evidence digest reason with skill target", reason: `slug: evidence digest for governance skill "research-orchestration" is stale`, target: "research-orchestration"},
		{name: "digest reason without a known skill", reason: "required_skill_stale drift", target: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := repairDriftNextAction(tc.reason, tc.target)
			assert.Contains(t, got, "slipway run")
			assert.NotContains(t, got, "restamp")
		})
	}
}

func TestRepairDriftNextActionGenericAndDualActive(t *testing.T) {
	t.Parallel()

	// #86: a generic drift finding routes to `slipway run`, not the
	// "inspect the named artifact and rerun" dead-end.
	generic := repairDriftNextAction("some unclassified drift", "artifacts/changes/demo")
	assert.Contains(t, generic, "slipway run")
	assert.NotContains(t, generic, "inspect the named artifact")

	// #86: the dual-active finding names executable resolution commands.
	dual := repairDriftNextAction("multiple active changes are active", "demo-a, demo-b")
	assert.Contains(t, dual, "slipway status")
	assert.Contains(t, dual, "slipway cancel --change")
	assert.Contains(t, dual, "slipway done --change")
}

func TestRepairPathAuthorityUsesLinkedWorktreeInvocationWorkspace(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "repair path authority linked worktree")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		worktreeRoot := filepath.Join(root, ".worktrees", slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		runtimeCapturedAt := time.Now().UTC()
		summaryCapturedAt := runtimeCapturedAt.Add(time.Hour)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"captured_at": runtimeCapturedAt.Format(time.RFC3339Nano),
		})
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        summaryCapturedAt,
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"t-01"},
			Tasks: []model.ExecutionTaskSummary{{
				TaskID:          "t-01",
				Verdict:         model.TaskVerdictPass,
				TaskKind:        model.TaskKindCode,
				ChangedFiles:    []string{"cmd/repair.go"},
				CapturedAt:      summaryCapturedAt,
				FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(bound, 1, "t-01"),
			}},
		})

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(worktreeRoot))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		var out bytes.Buffer
		cmd := makeRepairCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(out.Bytes(), &summary))
		require.Contains(t, summary.PathAuthority, slug)
		require.NotNil(t, summary.PathAuthority[slug])
		assert.Equal(t, state.DisplayPath(root, worktreeRoot), summary.PathAuthority[slug].InvocationWorkspacePath)
		assert.Equal(t, state.DisplayPath(root, worktreeRoot), summary.PathAuthority[slug].BoundWorkspacePath)
	})
}
