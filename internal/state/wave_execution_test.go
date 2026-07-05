package state

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadWaveRunsIgnoresPreviousRunVersionEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "wave-run-version-gating"
	dir := WaveEvidenceDir(root, slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	oldRun := []byte(`wave_index: 99
run_summary_version: 1
verdict: pass
`)
	currentRun := []byte(`wave_index: 1
run_summary_version: 2
verdict: pass
task_runs:
  - task_id: task-b
    run_summary_version: 2
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "wave-old.yaml"), oldRun, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "wave-01.yaml"), currentRun, 0o644))

	runs, err := LoadWaveRuns(root, slug, 2)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, 1, runs[0].WaveIndex)
	assert.Equal(t, 2, runs[0].RunSummaryVersion)
	assert.Equal(t, "task-b", runs[0].TaskRuns[0].TaskID)
}

// TestLoadWaveRunsStrictFirstMixedVersionClassification pins the strict-decode-first
// classification order: a clean non-matching-version file (and a foreign
// unknown-field file at a non-matching version) is skipped without surfacing a
// strict decode error, while a matching-version file with an unknown field still
// errors, and a run_summary_version < 1 file still hard-errors with the same
// "classify wave run ...: run_summary_version is required" wrapping.
func TestLoadWaveRunsStrictFirstMixedVersionClassification(t *testing.T) {
	t.Parallel()

	const runVersion = 2

	newDir := func(t *testing.T, slug string) (string, string) {
		t.Helper()
		root := t.TempDir()
		dir := WaveEvidenceDir(root, slug)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		return root, dir
	}
	writeRun := func(t *testing.T, dir, name, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	}

	t.Run("clean non-matching-version file is skipped not errored", func(t *testing.T) {
		t.Parallel()
		root, dir := newDir(t, "wave-strict-skip-clean")
		// Clean, known-fields-only file at a different run version: it strict-decodes
		// fine and MUST be skipped, never included alongside the matching run.
		writeRun(t, dir, "wave-old.yaml", "wave_index: 1\nrun_summary_version: 1\nverdict: pass\n")
		writeRun(t, dir, "wave-01.yaml", "wave_index: 1\nrun_summary_version: 2\nverdict: pass\n")

		runs, err := LoadWaveRuns(root, "wave-strict-skip-clean", runVersion)
		require.NoError(t, err)
		require.Len(t, runs, 1)
		assert.Equal(t, runVersion, runs[0].RunSummaryVersion)
	})

	t.Run("foreign non-matching-version file is skipped without surfacing strict error", func(t *testing.T) {
		t.Parallel()
		root, dir := newDir(t, "wave-strict-skip-foreign")
		// Unknown-field ("foreign") run at a non-matching version: the strict decode
		// fails, but the lenient version probe skips it just as the previous
		// probe-first classification did. The strict error must NOT surface.
		writeRun(t, dir, "wave-foreign.yaml", "wave_index: 1\nrun_summary_version: 1\nverdict: pass\nunknown_field: nope\n")

		runs, err := LoadWaveRuns(root, "wave-strict-skip-foreign", runVersion)
		require.NoError(t, err)
		require.Empty(t, runs)
	})

	t.Run("matching-version file with unknown field still errors", func(t *testing.T) {
		t.Parallel()
		root, dir := newDir(t, "wave-strict-unknown-field")
		writeRun(t, dir, "wave-01.yaml", "wave_index: 1\nrun_summary_version: 2\nverdict: pass\nunknown_field: nope\n")

		_, err := LoadWaveRuns(root, "wave-strict-unknown-field", runVersion)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse wave run")
		assert.Contains(t, err.Error(), "unknown_field", "a matching-version unknown field surfaces the strict decode error")
	})

	t.Run("run_summary_version below one hard-errors with classify wrapping", func(t *testing.T) {
		t.Parallel()
		root, dir := newDir(t, "wave-strict-version-zero")
		writeRun(t, dir, "wave-01.yaml", "wave_index: 1\nrun_summary_version: 0\nverdict: pass\n")

		_, err := LoadWaveRuns(root, "wave-strict-version-zero", runVersion)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "classify wave run")
		assert.Contains(t, err.Error(), "run_summary_version is required")
	})
}

func TestOrphanTaskEvidenceIgnoresPreviousRunVersionEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "orphan-task-version-gating"
	dir := EvidenceTasksDir(root, slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	writeEvidence := func(taskID string, runVersion int) {
		t.Helper()
		raw := []byte(`{"run_summary_version":` + strconv.Itoa(runVersion) + `}`)
		require.NoError(t, os.WriteFile(filepath.Join(dir, taskID+".json"), raw, 0o644))
	}
	writeEvidence("task-a", 1)
	writeEvidence("task-b", 2)
	writeEvidence("task-c", 2)

	orphaned, issues, err := orphanTaskEvidence(root, slug, 2, map[string]struct{}{"task-b": {}})
	require.NoError(t, err)
	require.Empty(t, issues)
	require.Len(t, orphaned, 1)
	assert.Equal(t, filepath.Join(dir, "task-c.json"), orphaned[0])
}

func TestOrphanTaskEvidenceReportsMalformedEvidenceAndContinues(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "orphan-task-malformed"
	dir := EvidenceTasksDir(root, slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "task-a.json"), []byte(`{"run_summary_version":2}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "task-b.json"), []byte(`{`), 0o644))

	orphaned, issues, err := orphanTaskEvidence(root, slug, 2, map[string]struct{}{})
	require.NoError(t, err)
	require.Len(t, orphaned, 1)
	assert.Equal(t, filepath.Join(dir, "task-a.json"), orphaned[0])
	require.Len(t, issues, 1)
	assert.Equal(t, filepath.Join(dir, "task-b.json"), issues[0].Path)
	assert.Contains(t, issues[0].Err.Error(), "parse task evidence")
}

func TestMaterializeWavePlanGeneratedAtUsesMaterializationTime(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := saveActiveChangeForTest(t, root, "wave-generated-at")
	bundleDir, err := GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(`# Tasks

- [ ] `+"`t-01`"+` prove generated_at boundary
  - target_files: ["internal/state/wave_execution.go"]
  - task_kind: test
  - acceptance: generated_at is materialization time
`), 0o644))

	tasksMTime := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	materializedAt := time.Date(2026, 6, 4, 3, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(tasksPath, tasksMTime, tasksMTime))

	plan, err := MaterializeWavePlanAt(root, change, materializedAt)
	require.NoError(t, err)
	assert.True(t, materializedAt.Equal(plan.GeneratedAt))
	assert.False(t, tasksMTime.Equal(plan.GeneratedAt))
}

func TestCurrentWavePlanRunSummaryVersionFailsClosedOnCorruptWavePlan(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := saveActiveChangeForTest(t, root, "corrupt-wave-plan-version")
	verifyDir, err := verificationDirForChange(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(verifyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(verifyDir, WavePlanFileName), []byte("version: [\n"), 0o644))
	require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		RunSummaryVersion: 7,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
	}))

	runVersion, err := currentWavePlanRunSummaryVersion(root, change)
	require.Error(t, err)
	assert.Equal(t, 0, runVersion)
	assert.Contains(t, err.Error(), "parse wave plan")
}

func TestLoadWavePlanFromPathUnsupportedFieldFailsClosedAsCacheUnreadable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, WavePlanFileName)
	// View-only fields (wave_count, advisories) live on the diagnostic
	// projection only; the persisted cache schema (model.WavePlan) rejects them
	// under KnownFields(true).
	require.NoError(t, os.WriteFile(path, []byte(
		"wave_count: 1\nadvisories: [\"narrow wave\"]\nwaves: []\n"), 0o644))

	_, err := loadWavePlanFromPath(path)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrWavePlanCacheUnreadable),
		"unsupported field must fail closed as a cache-unreadable condition, got: %v", err)
	assert.False(t, errors.Is(err, fs.ErrNotExist),
		"a corrupt cache is not a missing-file condition")
}

func TestLoadWavePlanFromPathMissingFileStaysNotExist(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), WavePlanFileName)
	_, err := loadWavePlanFromPath(path)
	require.Error(t, err)
	assert.True(t, errors.Is(err, fs.ErrNotExist),
		"missing cache must remain an fs.ErrNotExist condition, got: %v", err)
	assert.False(t, errors.Is(err, ErrWavePlanCacheUnreadable),
		"a missing cache must not be misreported as cache-unreadable")
}

const (
	twoIndependentTasksMD = "# Tasks\n\n" +
		"- [ ] `t-01` first\n  - target_files: [\"a.go\"]\n  - task_kind: code\n" +
		"- [ ] `t-02` second\n  - target_files: [\"b.go\"]\n  - task_kind: code\n"
	oneTaskMD = "# Tasks\n\n- [ ] `t-01` solo\n  - target_files: [\"a.go\"]\n  - task_kind: code\n"
)

var waveMaterializeTime = time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)

func writeBundleTasksForTest(t *testing.T, root string, change model.Change, tasksMD string) {
	t.Helper()
	bundleDir, err := GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(tasksMD), 0o644))
}

func TestMaterializeWavePlanMarksMultiTaskWaveParallel(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := saveActiveChangeForTest(t, root, "wave-parallel-multi")
	writeBundleTasksForTest(t, root, change, twoIndependentTasksMD)

	plan, err := MaterializeWavePlanAt(root, change, waveMaterializeTime)
	require.NoError(t, err)
	require.Len(t, plan.Waves, 1)
	require.Len(t, plan.Waves[0].Tasks, 2)
	assert.True(t, plan.Waves[0].Parallel, "a multi-task wave is forced parallel by default")
}

func TestMaterializeWavePlanSingleTaskWaveNotParallel(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := saveActiveChangeForTest(t, root, "wave-parallel-single")
	writeBundleTasksForTest(t, root, change, oneTaskMD)

	plan, err := MaterializeWavePlanAt(root, change, waveMaterializeTime)
	require.NoError(t, err)
	require.Len(t, plan.Waves, 1)
	assert.False(t, plan.Waves[0].Parallel, "a single-task wave is never parallel")
}

func TestMaterializeWavePlanParallelizationOff(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := saveActiveChangeForTest(t, root, "wave-parallel-off")
	require.NoError(t, os.WriteFile(ConfigPath(root), []byte("execution:\n  parallelization: off\n"), 0o644))
	writeBundleTasksForTest(t, root, change, twoIndependentTasksMD)

	plan, err := MaterializeWavePlanAt(root, change, waveMaterializeTime)
	require.NoError(t, err)
	require.Len(t, plan.Waves, 1)
	assert.False(t, plan.Waves[0].Parallel, "parallelization: off suppresses the forced-parallel signal")
}

func TestMaterializeWavePlanNormalizesBackslashTargetFiles(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := saveActiveChangeForTest(t, root, "wave-normalize-backslash-target")
	writeBundleTasksForTest(t, root, change, "# Tasks\n\n"+
		"- [ ] `t-01` first\n  - target_files: [\"cmd\\\\run.go\"]\n  - task_kind: code\n")

	plan, err := MaterializeWavePlanAt(root, change, waveMaterializeTime)
	require.NoError(t, err)
	require.Len(t, plan.Waves, 1)
	require.Len(t, plan.Waves[0].Tasks, 1)
	assert.Equal(t, []string{"cmd/run.go"}, plan.Waves[0].Tasks[0].TargetFiles)
}

func TestLoadWavePlanForChangePreservesMaterializedParallel(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := saveActiveChangeForTest(t, root, "wave-parallel-load-persisted")
	writeBundleTasksForTest(t, root, change, twoIndependentTasksMD)

	materialized, err := MaterializeWavePlanAt(root, change, waveMaterializeTime)
	require.NoError(t, err)
	require.True(t, materialized.Waves[0].Parallel)

	require.NoError(t, os.WriteFile(ConfigPath(root), []byte("execution:\n  parallelization: off\n"), 0o644))
	loaded, err := LoadWavePlanForChange(root, change)
	require.NoError(t, err)
	require.Len(t, loaded.Waves, 1)
	assert.True(t, loaded.Waves[0].Parallel, "loaded wave plans preserve the materialized dispatch evidence")
}

func TestApplyEffectiveParallelDoesNotMutateInputPlan(t *testing.T) {
	t.Parallel()

	plan := model.WavePlan{
		Version:     model.WavePlanVersion,
		GeneratedAt: waveMaterializeTime,
		TotalTasks:  2,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  false,
			Tasks: []model.WavePlanTask{
				{TaskID: "t-02", TargetFiles: []string{"b.go", "a.go"}},
				{TaskID: "t-01", DependsOn: []string{"x", "a"}},
			},
		}},
	}

	effective := ApplyEffectiveParallel(plan, true)

	require.Len(t, effective.Waves, 1)
	assert.True(t, effective.Waves[0].Parallel)
	assert.False(t, plan.Waves[0].Parallel, "effective conversion must not mutate the caller's persisted plan value")
	assert.Equal(t, []string{"b.go", "a.go"}, plan.Waves[0].Tasks[0].TargetFiles, "normalization must not sort through caller-owned slices")
	assert.Equal(t, []string{"x", "a"}, plan.Waves[0].Tasks[1].DependsOn, "normalization must not sort through caller-owned slices")
}

func TestMaterializeWavePlanParallelDoesNotChangeHashes(t *testing.T) {
	t.Parallel()

	rootForced := createRuntimeLayout(t)
	cForced := saveActiveChangeForTest(t, rootForced, "wave-parallel-hash-forced")
	writeBundleTasksForTest(t, rootForced, cForced, twoIndependentTasksMD)
	forced, err := MaterializeWavePlanAt(rootForced, cForced, waveMaterializeTime)
	require.NoError(t, err)

	rootOff := createRuntimeLayout(t)
	cOff := saveActiveChangeForTest(t, rootOff, "wave-parallel-hash-off")
	require.NoError(t, os.WriteFile(ConfigPath(rootOff), []byte("execution:\n  parallelization: off\n"), 0o644))
	writeBundleTasksForTest(t, rootOff, cOff, twoIndependentTasksMD)
	off, err := MaterializeWavePlanAt(rootOff, cOff, waveMaterializeTime)
	require.NoError(t, err)

	require.NotEqual(t, forced.Waves[0].Parallel, off.Waves[0].Parallel, "the parallel signal differs")
	assert.Equal(t, forced.TasksPlanHash, off.TasksPlanHash, "but the freshness hash is unaffected")
	assert.Equal(t, forced.TasksPlanStructuralHash, off.TasksPlanStructuralHash)
	assert.Equal(t, forced.TasksPlanScopeHash, off.TasksPlanScopeHash)
	assert.Equal(t, forced.TasksPlanSemanticHash, off.TasksPlanSemanticHash)
}

func TestBuildWaveRunsLeavesDispatchModeEmptyWithoutEvidence(t *testing.T) {
	t.Parallel()

	plan := model.WavePlan{
		Version:     model.WavePlanVersion,
		GeneratedAt: waveMaterializeTime,
		TotalTasks:  3,
		Waves: []model.WavePlanWave{
			{WaveIndex: 1, Parallel: true, Tasks: []model.WavePlanTask{{TaskID: "t-01"}, {TaskID: "t-02"}}},
			{WaveIndex: 2, Parallel: false, Tasks: []model.WavePlanTask{{TaskID: "t-03"}}},
		},
	}

	tasks := []model.ExecutionTaskSummary{
		{TaskID: "t-01", Verdict: model.TaskVerdictPass, CapturedAt: waveMaterializeTime},
		{TaskID: "t-02", Verdict: model.TaskVerdictPass, CapturedAt: waveMaterializeTime},
		{TaskID: "t-03", Verdict: model.TaskVerdictPass, CapturedAt: waveMaterializeTime},
	}

	// No dispatch evidence: the engine must NOT infer a parallel dispatch mode for
	// the started parallel wave (REQ-004). It records no dispatch mode; the
	// fail-closed blocker is surfaced by DispatchEvidenceBlockers at the sync layer.
	runs, err := BuildWaveRuns(plan, 1, tasks, nil)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, model.WaveDispatchMode(""), runs[0].DispatchMode, "started parallel wave without dispatch evidence records no inferred mode")
	assert.Equal(t, model.WaveDispatchMode(""), runs[1].DispatchMode, "sequential wave records no dispatch mode")
}

func TestBuildWaveRunsDoesNotRecordDispatchForPendingWave(t *testing.T) {
	t.Parallel()

	plan := model.WavePlan{
		Version:     model.WavePlanVersion,
		GeneratedAt: waveMaterializeTime,
		TotalTasks:  4,
		Waves: []model.WavePlanWave{
			{WaveIndex: 1, Parallel: true, Tasks: []model.WavePlanTask{{TaskID: "t-01"}, {TaskID: "t-02"}}},
			{WaveIndex: 2, Parallel: true, Tasks: []model.WavePlanTask{{TaskID: "t-03"}, {TaskID: "t-04"}}},
		},
	}
	tasks := []model.ExecutionTaskSummary{
		{TaskID: "t-01", Verdict: model.TaskVerdictPass, CapturedAt: waveMaterializeTime},
		{TaskID: "t-02", Verdict: model.TaskVerdictPass, CapturedAt: waveMaterializeTime},
	}

	// Wave 1 declares a valid parallel token and is started; wave 2 declares a
	// token too but is still pending, so it must record no dispatch mode.
	runs, err := BuildWaveRuns(
		plan,
		1,
		tasks,
		map[int]model.WaveDispatchMode{
			1: model.WaveDispatchParallel,
			2: model.WaveDispatchDegradedSequential,
		},
	)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, model.WaveVerdictPass, runs[0].Verdict)
	assert.Equal(t, model.WaveDispatchParallel, runs[0].DispatchMode)
	assert.Equal(t, model.WaveVerdictPending, runs[1].Verdict)
	assert.Empty(t, runs[1].DispatchMode, "pending waves have not been dispatched yet")
}

func TestBuildWaveRunsRecordsDegradedDispatchMode(t *testing.T) {
	t.Parallel()

	plan := model.WavePlan{
		Version:     model.WavePlanVersion,
		GeneratedAt: waveMaterializeTime,
		TotalTasks:  2,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  true,
			Tasks: []model.WavePlanTask{
				{TaskID: "t-01"},
				{TaskID: "t-02"},
			},
		}},
	}

	runs, err := BuildWaveRuns(
		plan,
		1,
		[]model.ExecutionTaskSummary{
			{TaskID: "t-01", Verdict: model.TaskVerdictPass, CapturedAt: waveMaterializeTime},
			{TaskID: "t-02", Verdict: model.TaskVerdictPass, CapturedAt: waveMaterializeTime},
		},
		map[int]model.WaveDispatchMode{1: model.WaveDispatchDegradedSequential},
	)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, model.WaveDispatchDegradedSequential, runs[0].DispatchMode)
}

// TestBuildWaveRunsDropsStaleDispatchModes proves that a dispatch token that does
// not apply to the wave — one keyed to another wave, or an invalid token — is
// dropped. Dropping now means recording no dispatch mode (fail closed) rather than
// falling back to an inferred parallel mode (REQ-004).
func TestBuildWaveRunsDropsStaleDispatchModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		wave         model.WavePlanWave
		dispatchByID map[int]model.WaveDispatchMode
		wantMode     model.WaveDispatchMode
	}{
		{
			name: "unknown wave records no mode",
			wave: model.WavePlanWave{
				WaveIndex: 1,
				Parallel:  true,
				Tasks:     []model.WavePlanTask{{TaskID: "t-01"}, {TaskID: "t-02"}},
			},
			dispatchByID: map[int]model.WaveDispatchMode{2: model.WaveDispatchDegradedSequential},
			wantMode:     "",
		},
		{
			name: "non parallel wave ignores stale dispatch",
			wave: model.WavePlanWave{
				WaveIndex: 1,
				Parallel:  false,
				Tasks:     []model.WavePlanTask{{TaskID: "t-01"}},
			},
			dispatchByID: map[int]model.WaveDispatchMode{1: model.WaveDispatchDegradedSequential},
		},
		{
			name: "invalid dispatch records no mode",
			wave: model.WavePlanWave{
				WaveIndex: 1,
				Parallel:  true,
				Tasks:     []model.WavePlanTask{{TaskID: "t-01"}, {TaskID: "t-02"}},
			},
			dispatchByID: map[int]model.WaveDispatchMode{1: model.WaveDispatchMode("sequential")},
			wantMode:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan := model.WavePlan{
				Version:     model.WavePlanVersion,
				GeneratedAt: waveMaterializeTime,
				TotalTasks:  len(tt.wave.Tasks),
				Waves:       []model.WavePlanWave{tt.wave},
			}

			tasks := make([]model.ExecutionTaskSummary, 0, len(tt.wave.Tasks))
			for _, task := range tt.wave.Tasks {
				tasks = append(tasks, model.ExecutionTaskSummary{
					TaskID:     task.TaskID,
					Verdict:    model.TaskVerdictPass,
					CapturedAt: waveMaterializeTime,
				})
			}

			runs, err := BuildWaveRuns(plan, 1, tasks, tt.dispatchByID)
			require.NoError(t, err)
			require.Len(t, runs, 1)
			assert.Equal(t, tt.wantMode, runs[0].DispatchMode)
		})
	}
}

func TestWaveRunsEquivalentDetectsDispatchModeChange(t *testing.T) {
	t.Parallel()

	base := []model.WaveRun{{
		WaveIndex:         1,
		RunSummaryVersion: 1,
		Verdict:           model.WaveVerdictPass,
		DispatchMode:      model.WaveDispatchParallel,
	}}
	withoutDispatch := []model.WaveRun{{
		WaveIndex:         1,
		RunSummaryVersion: 1,
		Verdict:           model.WaveVerdictPass,
	}}

	assert.True(t, waveRunsEquivalent(base, base))
	assert.False(t, waveRunsEquivalent(base, withoutDispatch))
}
