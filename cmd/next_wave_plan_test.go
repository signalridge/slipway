package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWavePlanViewFromModelSurfacesParallel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	plan := model.WavePlan{
		Version:    model.WavePlanVersion,
		TotalTasks: 3,
		Waves: []model.WavePlanWave{
			{WaveIndex: 1, Parallel: true, Tasks: []model.WavePlanTask{{TaskID: "t-01"}, {TaskID: "t-02"}}},
			{WaveIndex: 2, Parallel: false, Tasks: []model.WavePlanTask{{TaskID: "t-03"}}},
		},
	}

	view := wavePlanViewFromModel(root, plan, true)
	require.NotNil(t, view)
	require.Len(t, view.Waves, 2)
	assert.True(t, view.Waves[0].Parallel, "multi-task wave is surfaced as parallel")
	assert.False(t, view.Waves[1].Parallel, "single-task wave is not parallel")
}

func TestAuthoritativeWavePlanViewReDerivesParallelFromCurrentConfig(t *testing.T) {
	t.Parallel()

	t.Run("stale persisted false becomes parallel by default", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		change := model.NewChange("stale-wave-plan-default")
		change.CurrentState = model.StateS2Execute
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, state.SaveWavePlan(root, change.Slug, model.WavePlan{
			Version: model.WavePlanVersion,
			GeneratedAt: time.Date(2026, 6, 9, 1, 0, 0, 0,
				time.UTC),
			TotalTasks: 2,
			Waves: []model.WavePlanWave{{
				WaveIndex: 1,
				Parallel:  false,
				Tasks: []model.WavePlanTask{
					{TaskID: "t-01"},
					{TaskID: "t-02"},
				},
			}},
		}))

		view := authoritativeWavePlanView(root, change)
		require.NotNil(t, view)
		require.Empty(t, view.ParseError)
		require.Len(t, view.Waves, 1)
		assert.True(t, view.Waves[0].Parallel)
	})

	t.Run("parallelization off suppresses stale persisted true", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		change := model.NewChange("stale-wave-plan-off")
		change.CurrentState = model.StateS2Execute
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, os.WriteFile(
			state.ConfigPath(root),
			[]byte("execution:\n  parallelization: off\n"),
			0o644,
		))
		require.NoError(t, state.SaveWavePlan(root, change.Slug, model.WavePlan{
			Version: model.WavePlanVersion,
			GeneratedAt: time.Date(2026, 6, 9, 1, 0, 0, 0,
				time.UTC),
			TotalTasks: 2,
			Waves: []model.WavePlanWave{{
				WaveIndex: 1,
				Parallel:  true,
				Tasks: []model.WavePlanTask{
					{TaskID: "t-01"},
					{TaskID: "t-02"},
				},
			}},
		}))

		view := authoritativeWavePlanView(root, change)
		require.NotNil(t, view)
		require.Empty(t, view.ParseError)
		require.Len(t, view.Waves, 1)
		assert.False(t, view.Waves[0].Parallel)
	})
}

// wavelessDependencyTasksFixture is the retired-`wave:` contract: no task
// declares a wave line; three dependency-free tasks with pairwise-disjoint
// target_files plus one task depending on all three. The engine must compute
// wave 1 = {t-01, t-02, t-03} (parallel) and wave 2 = {t-04} (not parallel).
const wavelessDependencyTasksFixture = `# Tasks

- [ ] ` + "`t-01`" + ` cover parser retirement of declared waves
  - target_files: ["internal/engine/wave/parse.go"]
  - task_kind: test
- [ ] ` + "`t-02`" + ` cover dependency-derived wave computation
  - target_files: ["internal/engine/wave/wave.go"]
  - task_kind: test
- [ ] ` + "`t-03`" + ` align the wave-plan view with computed waves
  - target_files: ["cmd/next_wave_plan.go"]
  - task_kind: code
- [ ] ` + "`t-04`" + ` integrate computed wave assignment end to end
  - depends_on: ["t-01", "t-02", "t-03"]
  - target_files: ["docs/wave-plan.md"]
  - task_kind: doc
`

// declaredWaveTasksFixture carries the retired `wave:` metadata on exactly one
// task (t-02) so the retirement error must name that task unambiguously.
const declaredWaveTasksFixture = `# Tasks

- [ ] ` + "`t-01`" + ` waveless task without declared metadata
  - target_files: ["cmd/next.go"]
  - task_kind: code
- [ ] ` + "`t-02`" + ` task still carrying retired declared-wave metadata
  - wave: 2
  - depends_on: ["t-01"]
  - target_files: ["cmd/next_handoff.go"]
  - task_kind: code
`

// writeWavePlanTasksFixture writes tasks.md into the governed bundle location
// for slug and returns the project-relative artifact bundle path used by the
// derived wave-plan preview.
func writeWavePlanTasksFixture(t *testing.T, root, slug, content string) string {
	t.Helper()
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(content), 0o644))
	return filepath.Join("artifacts", "changes", slug)
}

func waveViewTaskIDs(w waveView) []string {
	ids := make([]string, 0, len(w.Tasks))
	for _, task := range w.Tasks {
		ids = append(ids, task.TaskID)
	}
	return ids
}

func wavePlanModelTaskIDs(w model.WavePlanWave) []string {
	ids := make([]string, 0, len(w.Tasks))
	for _, task := range w.Tasks {
		ids = append(ids, task.TaskID)
	}
	return ids
}

// assertWaveRetirementError pins the retirement contract with resilient
// substring checks: the error must name the offending task, the retired wave
// metadata, and the delete-the-line remediation (waves derive from depends_on).
func assertWaveRetirementError(t *testing.T, message string) {
	t.Helper()
	lower := strings.ToLower(message)
	assert.Contains(t, lower, "t-02", "retirement error must name the task carrying the wave: line")
	assert.Contains(t, lower, "wave", "retirement error must name the retired wave metadata")
	assert.Truef(t,
		strings.Contains(lower, "delete") || strings.Contains(lower, "remove") || strings.Contains(lower, "depends_on"),
		"retirement error %q must carry the delete-the-line remediation (delete/remove the wave: line; waves derive from depends_on)",
		message,
	)
}

func TestDerivedWavePlanPreviewComputesWavesFromDependencies(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	bundle := writeWavePlanTasksFixture(t, root, "waveless-preview", wavelessDependencyTasksFixture)

	view := derivedWavePlanView(root, bundle)
	require.NotNil(t, view)
	require.Emptyf(t, view.ParseError,
		"waveless tasks.md (no wave: lines) must yield a computed wave plan, got parse error: %s", view.ParseError)

	assert.Equal(t, 4, view.TotalTasks)
	assert.Equal(t, 2, view.WaveCount)
	require.Len(t, view.Waves, 2)

	assert.Equal(t, 1, view.Waves[0].WaveIndex)
	assert.ElementsMatch(t, []string{"t-01", "t-02", "t-03"}, waveViewTaskIDs(view.Waves[0]),
		"dependency-free tasks all land in computed wave 1")
	assert.True(t, view.Waves[0].Parallel, "multi-task computed wave is parallel by default")

	assert.Equal(t, 2, view.Waves[1].WaveIndex)
	assert.Equal(t, []string{"t-04"}, waveViewTaskIDs(view.Waves[1]),
		"task depending on all wave-1 tasks lands in computed wave 2")
	assert.False(t, view.Waves[1].Parallel, "single-task wave is not parallel")
}

func TestMaterializeWavePlanComputesWavesFromDependencies(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("waveless-materialization")
	change.CurrentState = model.StateS2Execute
	require.NoError(t, state.SaveChange(root, change))
	writeWavePlanTasksFixture(t, root, change.Slug, wavelessDependencyTasksFixture)

	plan, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err,
		"materialization must compute waves from depends_on; declared wave: lines are retired")

	assert.Equal(t, 4, plan.TotalTasks)
	require.Len(t, plan.Waves, 2)
	assert.Equal(t, 1, plan.Waves[0].WaveIndex)
	assert.ElementsMatch(t, []string{"t-01", "t-02", "t-03"}, wavePlanModelTaskIDs(plan.Waves[0]))
	assert.True(t, plan.Waves[0].Parallel, "multi-task computed wave is materialized parallel by default")
	assert.Equal(t, 2, plan.Waves[1].WaveIndex)
	assert.Equal(t, []string{"t-04"}, wavePlanModelTaskIDs(plan.Waves[1]))
	assert.False(t, plan.Waves[1].Parallel, "single-task wave is not parallel")

	view := authoritativeWavePlanView(root, change)
	require.NotNil(t, view)
	require.Empty(t, view.ParseError)
	assert.Equal(t, 2, view.WaveCount)
	require.Len(t, view.Waves, 2)
	assert.ElementsMatch(t, []string{"t-01", "t-02", "t-03"}, waveViewTaskIDs(view.Waves[0]))
	assert.True(t, view.Waves[0].Parallel)
	assert.Equal(t, []string{"t-04"}, waveViewTaskIDs(view.Waves[1]))
	assert.False(t, view.Waves[1].Parallel)
}

func TestWavePlanRejectsDeclaredWaveMetadata(t *testing.T) {
	t.Parallel()

	t.Run("derived preview surfaces retirement parse error", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		bundle := writeWavePlanTasksFixture(t, root, "declared-wave-preview", declaredWaveTasksFixture)

		view := derivedWavePlanView(root, bundle)
		require.NotNil(t, view)
		require.NotEmpty(t, view.ParseError, "a tasks.md carrying a wave: line must be rejected")
		assertWaveRetirementError(t, view.ParseError)
	})

	t.Run("materialization fails with retirement error", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		change := model.NewChange("declared-wave-materialization")
		change.CurrentState = model.StateS2Execute
		require.NoError(t, state.SaveChange(root, change))
		writeWavePlanTasksFixture(t, root, change.Slug, declaredWaveTasksFixture)

		_, err := state.MaterializeWavePlan(root, change)
		require.Error(t, err, "a tasks.md carrying a wave: line must fail materialization")
		assertWaveRetirementError(t, err.Error())
	})
}

// narrowingTasksFixture forces both REQ-006 advisories: t-01 has a dotted
// directory target (broad_target_files) and t-02→t-03 form a single linear depends_on
// chain off t-01 so every task lands on the dependency critical path
// (fully_serial_plan). Targets are otherwise file-disjoint, so serialization is
// dependency-driven, not conflict-driven.
const narrowingTasksFixture = `# Tasks

- [ ] ` + "`t-01`" + ` broad directory-scoped task
  - target_files: [".github/"]
  - task_kind: code
- [ ] ` + "`t-02`" + ` middle of the linear chain
  - depends_on: ["t-01"]
  - target_files: ["cmd/next.go"]
  - task_kind: code
- [ ] ` + "`t-03`" + ` tail of the linear chain
  - depends_on: ["t-02"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`

func TestDerivedWavePlanViewSurfacesNarrowingAdvisories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	bundle := writeWavePlanTasksFixture(t, root, "derived-advisories", narrowingTasksFixture)

	view := derivedWavePlanView(root, bundle)
	require.NotNil(t, view)
	require.Empty(t, view.ParseError)

	assert.Contains(t, view.Advisories, "broad_target_files:t-01",
		"derived path must surface the directory-target narrowing cue")
	assert.Contains(t, view.Advisories, "fully_serial_plan",
		"derived path must surface the linear-chain narrowing cue")
}

func TestDerivedWavePlanViewPreservesExplicitNestedDirectoryAdvisory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	bundle := writeWavePlanTasksFixture(t, root, "derived-nested-directory-advisory", `# Tasks

- [ ] `+"`t-01`"+` creates a new package directory
  - target_files: ["internal/newpkg/"]
  - task_kind: code
`)

	view := derivedWavePlanView(root, bundle)
	require.NotNil(t, view)
	require.Empty(t, view.ParseError)

	assert.Contains(t, view.Advisories, "broad_target_files:t-01",
		"derived advisory analysis must preserve explicit nested directory targets whose trailing slash would otherwise be normalized away")
}

func TestWavePlanViewFromModelSurfacesNarrowingAdvisories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".github"), 0o755))
	plan := model.WavePlan{
		Version:    model.WavePlanVersion,
		TotalTasks: 3,
		Waves: []model.WavePlanWave{
			{WaveIndex: 1, Parallel: false, Tasks: []model.WavePlanTask{
				// Persisted wave plans carry normalized public paths, so an
				// explicit ".github/" directory target is read back as ".github".
				{TaskID: "t-01", TargetFiles: []string{".github"}, TaskKind: model.TaskKindCode},
			}},
			{WaveIndex: 2, Parallel: false, Tasks: []model.WavePlanTask{
				{TaskID: "t-02", DependsOn: []string{"t-01"}, TargetFiles: []string{"cmd/next.go"}, TaskKind: model.TaskKindCode},
			}},
			{WaveIndex: 3, Parallel: false, Tasks: []model.WavePlanTask{
				{TaskID: "t-03", DependsOn: []string{"t-02"}, TargetFiles: []string{"cmd/run.go"}, TaskKind: model.TaskKindCode},
			}},
		},
	}

	view := wavePlanViewFromModel(root, plan, true)
	require.NotNil(t, view)

	assert.Contains(t, view.Advisories, "broad_target_files:t-01",
		"from-model path must rebuild nodes and surface the directory-target cue")
	assert.Contains(t, view.Advisories, "fully_serial_plan",
		"from-model path must rebuild nodes and surface the linear-chain cue")
}

func TestWavePlanViewFromModelRestoresNestedDirectoryAdvisory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "engine"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "deploy"), []byte("#!/bin/sh\n"), 0o755))

	plan := model.WavePlan{
		Version:    model.WavePlanVersion,
		TotalTasks: 2,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  true,
			Tasks: []model.WavePlanTask{
				{
					TaskID:      "t-01",
					TargetFiles: []string{"internal/engine"},
					TaskKind:    model.TaskKindCode,
				},
				{
					TaskID:      "t-02",
					TargetFiles: []string{"scripts/deploy"},
					TaskKind:    model.TaskKindCode,
				},
			},
		}},
	}

	view := wavePlanViewFromModel(root, plan, true)
	require.NotNil(t, view)

	assert.Contains(t, view.Advisories, "broad_target_files:t-01",
		"from-model advisory analysis must recover nested directory targets whose trailing slash was removed by path normalization")
	assert.NotContains(t, view.Advisories, "broad_target_files:t-02",
		"an existing extensionless file must not be misreported as a broad directory target")
}

func TestAuthoritativeWavePlanViewPreservesExplicitDirectoryAdvisory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("explicit-directory-advisory")
	change.CurrentState = model.StateS2Execute
	require.NoError(t, state.SaveChange(root, change))
	writeWavePlanTasksFixture(t, root, change.Slug, `# Tasks

- [ ] `+"`t-01`"+` creates a new package directory
  - target_files: ["internal/newpkg/"]
  - task_kind: code
`)

	plan, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	require.Equal(t, []string{"internal/newpkg"}, plan.Waves[0].Tasks[0].TargetFiles,
		"materialized plans normalize away the explicit directory slash")

	view := authoritativeWavePlanView(root, change)
	require.NotNil(t, view)

	assert.Contains(t, view.Advisories, "broad_target_files:t-01",
		"authoritative view advisories must use tasks.md source targets, not only normalized wave-plan.yaml targets")
}

func TestWavePlanViewFromModelIgnoresNonEquivalentSourceTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	plan := model.WavePlan{
		Version:    model.WavePlanVersion,
		TotalTasks: 1,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Tasks: []model.WavePlanTask{{
				TaskID:      "t-01",
				TargetFiles: []string{"cmd/next.go"},
				TaskKind:    model.TaskKindCode,
			}},
		}},
	}
	sourceTargets := map[string][]string{
		"t-01": {"internal/newpkg/"},
	}

	view := wavePlanViewFromModel(root, plan, true, sourceTargets)
	require.NotNil(t, view)

	assert.NotContains(t, view.Advisories, "broad_target_files:t-01",
		"source targets that do not normalize to the materialized plan must not influence authoritative advisories")
}

func TestWavePlanViewFromModelRestoresExistingDirectoryAdvisoryWithSourceTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "engine"), 0o755))

	plan := model.WavePlan{
		Version:    model.WavePlanVersion,
		TotalTasks: 1,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  true,
			Tasks: []model.WavePlanTask{{
				TaskID:      "t-01",
				TargetFiles: []string{"internal/engine"},
				TaskKind:    model.TaskKindCode,
			}},
		}},
	}
	sourceTargets := map[string][]string{
		"t-01": {"internal/engine"},
	}

	view := wavePlanViewFromModel(root, plan, true, sourceTargets)
	require.NotNil(t, view)

	assert.Contains(t, view.Advisories, "broad_target_files:t-01",
		"equivalent source targets must preserve existing-directory recovery on the authoritative from-model path")
}

func TestCleanParallelPlanEmitsNoNarrowingAdvisories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	bundle := writeWavePlanTasksFixture(t, root, "clean-advisories", wavelessDependencyTasksFixture)

	view := derivedWavePlanView(root, bundle)
	require.NotNil(t, view)
	require.Empty(t, view.ParseError)
	// File-disjoint parallel roots with a single fan-in dependent: no broad
	// targets and the critical path (2) is below the node count (4).
	assert.Empty(t, view.Advisories,
		"a clean file-disjoint plan must surface no narrowing advisories")
}

func TestNarrowingAdvisoriesAreViewOnlyAndExcludedFromPersistedPlan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("advisories-view-only")
	change.CurrentState = model.StateS2Execute
	require.NoError(t, state.SaveChange(root, change))
	writeWavePlanTasksFixture(t, root, change.Slug, narrowingTasksFixture)

	plan, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	// The persisted plan model has no advisories field at all, so it cannot
	// enter wave-plan.yaml nor any freshness hash derived from the model.
	persistedJSON, err := json.Marshal(plan)
	require.NoError(t, err)
	assert.NotContains(t, string(persistedJSON), "advisories",
		"the persisted wave-plan model must not carry advisories")

	rawYAML, err := os.ReadFile(state.WavePlanPathForRead(root, change.Slug))
	require.NoError(t, err)
	assert.NotContains(t, string(rawYAML), "advisories",
		"wave-plan.yaml must not persist view-only narrowing advisories")
	// Freshness hashes are computed and persisted before the view layer runs;
	// none of them is empty and none is the advisory text.
	assert.NotEmpty(t, plan.TasksPlanHash)
	assert.NotContains(t, plan.TasksPlanHash, "fully_serial_plan")
	assert.NotContains(t, plan.TasksPlanScopeHash, "broad_target_files")

	// The same plan, surfaced through the view, DOES carry the advisories.
	view := wavePlanViewFromModel(root, plan, state.EffectiveForcedParallel(root))
	require.NotNil(t, view)
	assert.Contains(t, view.Advisories, "broad_target_files:t-01")
	assert.Contains(t, view.Advisories, "fully_serial_plan")
}
