package state

import (
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
  - wave: 1
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

const (
	twoIndependentTasksMD = "# Tasks\n\n" +
		"- [ ] `t-01` first\n  - wave: 1\n  - target_files: [\"a.go\"]\n  - task_kind: code\n" +
		"- [ ] `t-02` second\n  - wave: 1\n  - target_files: [\"b.go\"]\n  - task_kind: code\n"
	oneTaskMD = "# Tasks\n\n- [ ] `t-01` solo\n  - wave: 1\n  - target_files: [\"a.go\"]\n  - task_kind: code\n"
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

func TestBuildWaveRunsDerivesParallelDispatchMode(t *testing.T) {
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

	runs, err := BuildWaveRuns(plan, 1, nil)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, model.WaveDispatchParallel, runs[0].DispatchMode, "parallel wave records its dispatch mode")
	assert.Equal(t, model.WaveDispatchMode(""), runs[1].DispatchMode, "sequential wave records no dispatch mode")
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
		nil,
		map[int]model.WaveDispatchMode{1: model.WaveDispatchDegradedSequential},
	)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, model.WaveDispatchDegradedSequential, runs[0].DispatchMode)
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
