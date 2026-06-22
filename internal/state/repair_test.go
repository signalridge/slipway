package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiagnoseBundleConsistencyAllPresent(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("consistent-bundle")
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", "consistent-bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	// change.yaml is written by SaveChange into the bundle directory.
	// Add tasks.md and assurance.md.
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("# Assurance\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestDiagnoseBundleConsistencyChangeYamlMissing(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("no-change-yaml")

	// Create slug registry dir and bundle dir, but don't write change.yaml via SaveChange.
	require.NoError(t, os.MkdirAll(ChangeDir(root, "no-change-yaml"), 0o755))
	bundleDir := filepath.Join(root, "artifacts", "changes", "no-change-yaml")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "change.yaml missing")
}

func TestDiagnoseBundleConsistencyTasksMissing(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("no-tasks")
	require.NoError(t, SaveChange(root, change))

	// Remove tasks.md — only change.yaml in bundle.
	bundleDir := filepath.Join(root, "artifacts", "changes", "no-tasks")

	result := DiagnoseBundleConsistency(root, change)
	_ = bundleDir
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "tasks.md missing")
}

// assurance.md is deferred to S3_REVIEW authoring (issue #141): before review its
// absence is the expected deferred state, so the bundle-consistency diagnostic
// must stay silent pre-S3 — neither error nor warning — rather than reporting a
// by-design deferral as a partial-write inconsistency.
func TestDiagnoseBundleConsistencyAssuranceDeferredPreReviewIsSilent(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("no-assurance-early")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", "no-assurance-early")
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings, "deferred assurance.md must not be flagged before S3_REVIEW")
}

// seedWavePlanRepairChange writes a tasks.md, materializes its wave-plan, and
// returns the change plus the materialized plan.
func seedWavePlanRepairChange(t *testing.T, slug, tasksMD string) (string, model.Change, model.WavePlan) {
	t.Helper()
	root := createRuntimeLayout(t)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(tasksMD), 0o644))
	plan, err := MaterializeWavePlan(root, change)
	require.NoError(t, err)
	return root, change, plan
}

func TestWavePlanRepairDriftRebuildsOnStructuralDriftWhenSummaryNotReady(t *testing.T) {
	t.Parallel()
	tasksA := "# Tasks\n\n- [ ] `t-01` original\n  - target_files: [\"a.go\"]\n  - task_kind: code\n"
	root, change, plan := seedWavePlanRepairChange(t, "wave-repair-structural", tasksA)

	// No drift before editing tasks.md.
	changed, preserveHistoricalEvidence, err := wavePlanRepairDrift(root, change, plan, nil)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.False(t, preserveHistoricalEvidence)

	// A structural edit (task_kind) with no ready summary must rebuild, not reuse
	// the readable-but-stale plan (#97 / REQ-006).
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"),
		[]byte("# Tasks\n\n- [ ] `t-01` original\n  - target_files: [\"a.go\"]\n  - task_kind: test\n"), 0o644))
	changed, preserveHistoricalEvidence, err = wavePlanRepairDrift(root, change, plan, nil)
	require.NoError(t, err)
	assert.True(t, changed, "structural drift with no ready summary must rebuild the wave-plan")
	assert.False(t, preserveHistoricalEvidence)
	changed, preserveHistoricalEvidence, err = wavePlanRepairDrift(root, change, plan, &model.ExecutionSummary{
		RunSummaryVersion: 1,
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:  "t-01",
			Verdict: model.TaskVerdictPass,
		}},
	})
	require.NoError(t, err)
	assert.True(t, changed, "structural drift must rebuild the wave-plan even when old execution evidence exists")
	assert.True(t, preserveHistoricalEvidence, "old execution evidence must be preserved when the task boundary drifted")
}

func TestWavePlanRepairDriftRebuildsLegacyPlanMissingScopeHash(t *testing.T) {
	t.Parallel()
	tasksA := "# Tasks\n\n- [ ] `t-01` original\n  - target_files: [\"a.go\"]\n  - task_kind: code\n"
	root, change, plan := seedWavePlanRepairChange(t, "wave-repair-legacy-scope", tasksA)

	// Simulate a plan materialized before the scope-hash field existed: structure
	// hashes are present, scope hash is empty.
	plan.TasksPlanScopeHash = ""

	// A target_files-only edit keeps the structure identical but changes scope.
	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"),
		[]byte("# Tasks\n\n- [ ] `t-01` original\n  - target_files: [\"b.go\"]\n  - task_kind: code\n"), 0o644))

	changed, preserveHistoricalEvidence, err := wavePlanRepairDrift(root, change, plan, nil)
	require.NoError(t, err)
	assert.True(t, changed, "legacy plan with empty scope hash must rebuild on scope drift instead of carrying stale target_files")
	assert.False(t, preserveHistoricalEvidence)
}

func TestRepairExecutionStateUsesEffectiveParallelWhenRecoveringWaveRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		slug              string
		config            string
		persistedParallel bool
		want              model.WaveDispatchMode
	}{
		{
			name:              "default forced parallel overrides stale persisted false",
			slug:              "wave-repair-effective-parallel-default",
			persistedParallel: false,
			want:              model.WaveDispatchParallel,
		},
		{
			name:              "parallelization off suppresses stale persisted true",
			slug:              "wave-repair-effective-parallel-off",
			config:            "execution:\n  parallelization: off\n",
			persistedParallel: true,
			want:              "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root, change, plan := seedWavePlanRepairChange(t, tt.slug, twoIndependentTasksMD)
			if tt.config != "" {
				require.NoError(t, os.WriteFile(ConfigPath(root), []byte(tt.config), 0o644))
			}
			require.Len(t, plan.Waves, 1)
			plan.Waves[0].Parallel = tt.persistedParallel
			require.NoError(t, saveWavePlanForTest(root, change.Slug, plan))

			// A valid parallel dispatch token lets the recovered run record a
			// parallel mode when the wave is effectively parallel; the engine no
			// longer infers one from the Parallel flag alone (REQ-004). When
			// parallelization is off the wave is non-parallel, so the same token is
			// ignored and the recovered run records no dispatch mode.
			writeVerificationForTest(t, root, change.Slug, "wave-orchestration", model.VerificationRecord{
				Verdict:    model.VerificationVerdictPass,
				Blockers:   []model.ReasonCode{},
				Timestamp:  waveMaterializeTime,
				RunVersion: 1,
				References: []string{"dispatch_mode:wave=1:parallel_subagents"},
			})
			require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
				RunSummaryVersion: 1,
				CapturedAt:        waveMaterializeTime,
				Tasks: []model.ExecutionTaskSummary{
					{
						TaskID:       "t-01",
						Verdict:      model.TaskVerdictPass,
						TaskKind:     model.TaskKindCode,
						ChangedFiles: []string{"a.go"},
						TargetFiles:  []string{"a.go"},
						CapturedAt:   waveMaterializeTime,
					},
					{
						TaskID:       "t-02",
						Verdict:      model.TaskVerdictPass,
						TaskKind:     model.TaskKindCode,
						ChangedFiles: []string{"b.go"},
						TargetFiles:  []string{"b.go"},
						CapturedAt:   waveMaterializeTime,
					},
				},
			}))

			result, err := RepairExecutionState(root)
			require.NoError(t, err)
			assert.Contains(t, result.RecoveredWaveRuns, change.Slug)

			runs, err := LoadWaveRuns(root, change.Slug, 1)
			require.NoError(t, err)
			require.Len(t, runs, 1)
			assert.Equal(t, tt.want, runs[0].DispatchMode)
		})
	}
}

func TestDiagnoseBundleConsistencyAssuranceMissingErrorInReview(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("no-assurance-review")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", "no-assurance-review")
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "assurance.md missing")
}

func TestDiagnoseBundleConsistencyLightPresetDoesNotRequireAssuranceInReview(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("light-no-assurance-review")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", "light-no-assurance-review")
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestDiagnoseBundleConsistencyUsesCanonicalConfigForBoundWorkspace(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	worktreeRoot := t.TempDir()
	require.NoError(t, os.WriteFile(ConfigPath(root), []byte("governance:\n  min_preset: strict\n"), 0o644))
	require.NoError(t, os.WriteFile(ConfigPath(worktreeRoot), []byte("governance:\n  min_preset: light\n"), 0o644))

	change := model.NewChange("worktree-light-no-assurance-review")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(worktreeRoot, "artifacts", "changes", change.Slug)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	assert.Contains(t, result.Errors, "assurance.md missing in governed bundle — required for review/verify/done phase")
	assert.Empty(t, result.Warnings)
}

func TestDiagnoseBundleConsistencyNoBundleNoop(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("no-bundle")

	result := DiagnoseBundleConsistency(root, change)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestRepairMissingConfigCreatesDefault(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	// Remove config so repair triggers.
	os.Remove(ConfigPath(root))

	_, err := RepairCorruptConfig(root, time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	cfg, err := model.LoadConfig(ConfigPath(root))
	require.NoError(t, err)
	assert.Equal(t, model.ArtifactSchemaExpanded, cfg.Defaults.ArtifactSchema)
}

func TestRepairArchivedTerminalStatusSanitizesSiblingWorktreeArchiveInPlace(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	slug := "repair-worktree-archive"
	bundleDir := filepath.Join(worktreeRoot, "artifacts", "changes", slug)
	change := model.NewChange(slug)
	change.WorktreePath = worktreeRoot
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	change.Artifacts = map[string]model.ArtifactState{
		"intent": {
			ID:    "intent",
			Path:  filepath.Join(bundleDir, "intent.md"),
			State: model.ArtifactLifecycleFrozen,
		},
	}
	require.NoError(t, SaveChange(root, change))

	archivedBundleDir := filepath.Join(worktreeRoot, "artifacts", "changes", "archived", slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(archivedBundleDir), 0o755))
	require.NoError(t, os.Rename(bundleDir, archivedBundleDir))
	archivedChangePath := filepath.Join(archivedBundleDir, "change.yaml")
	raw, err := os.ReadFile(archivedChangePath)
	require.NoError(t, err)
	raw = append(raw, []byte("worktree_path: "+worktreeRoot+"\n")...)
	require.NoError(t, os.WriteFile(archivedChangePath, raw, 0o644))
	require.NoError(t, os.MkdirAll(ChangeDir(root, slug), 0o755))

	repaired, err := RepairArchivedTerminalStatus(root, slug)
	require.NoError(t, err)
	assert.True(t, repaired)

	_, err = os.Stat(filepath.Join(archivedBundleDir, "change.yaml"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(root, "artifacts", "changes", "archived", slug))
	assert.True(t, os.IsNotExist(err))

	raw, err = os.ReadFile(filepath.Join(archivedBundleDir, "change.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "worktree_path:")
	assert.NotContains(t, string(raw), worktreeRoot)
	assert.Contains(t, string(raw), "path: intent.md")

	loaded, err := LoadArchivedChange(root, slug)
	require.NoError(t, err)
	assert.Empty(t, loaded.WorktreePath)
	assert.Equal(t, "intent.md", loaded.Artifacts["intent"].Path)

	_, err = os.Stat(ChangeDir(root, slug))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestDiagnoseBundleConsistencyDetectsCorruptChangeYaml(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("corrupt-yaml-diag")
	require.NoError(t, SaveChange(root, change))

	paths, err := ResolveChangePaths(root, change)
	require.NoError(t, err)

	// Write corrupt YAML into the bundle's change.yaml.
	require.NoError(t, os.WriteFile(
		filepath.Join(paths.GovernedBundleDir, "change.yaml"),
		[]byte("slug: [invalid yaml"), 0o644))

	result := DiagnoseBundleConsistency(root, change)
	require.NotEmpty(t, result.Errors, "should detect corrupt change.yaml")
	assert.Contains(t, result.Errors[0], "YAML parse error")
}
