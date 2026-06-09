package wave

import (
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanWavesBuildsDeclaredWavePlan(t *testing.T) {
	t.Parallel()
	nodes := []Node{
		{TaskID: "b", WaveIndex: 1, DependsOn: nil, TargetFiles: []string{"y.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "a", WaveIndex: 1, DependsOn: nil, TargetFiles: []string{"x.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "c", WaveIndex: 2, DependsOn: []string{"a"}, TargetFiles: []string{"z.go"}, TaskKind: model.TaskKindCode},
	}
	waves, err := PlanWaves(nodes)
	require.NoError(t, err)
	require.Len(t, waves, 2)
	assert.Equal(t, []Node{nodes[1], nodes[0]}, waves[0].Nodes)
	assert.Equal(t, []Node{nodes[2]}, waves[1].Nodes)
}

func TestPlanWavesRejectsStaticConflictsInsideDeclaredWave(t *testing.T) {
	t.Parallel()
	_, err := PlanWaves([]Node{
		{TaskID: "a", WaveIndex: 1, TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "b", WaveIndex: 1, TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "static target conflict")
}

func TestPlanWavesRejectsStaticConflictsWithPathAliases(t *testing.T) {
	t.Parallel()

	_, err := PlanWaves([]Node{
		{TaskID: "a", WaveIndex: 1, TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "b", WaveIndex: 1, TargetFiles: []string{"./a.go"}, TaskKind: model.TaskKindCode},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "static target conflict")
	assert.Contains(t, err.Error(), "a.go")
}

func TestPlanWavesAllowsTaskIDThatLooksLikeLegacyRunSuffix(t *testing.T) {
	t.Parallel()

	plan, err := PlanWaves([]Node{
		{TaskID: "task-a__legacy", WaveIndex: 1, TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
	})
	require.NoError(t, err)
	require.Len(t, plan, 1)
}

func TestPlanWavesRejectsMissingWaveDeclarations(t *testing.T) {
	t.Parallel()
	_, err := PlanWaves([]Node{
		{TaskID: "a", TaskKind: model.TaskKindCode},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required wave declaration")
}

func TestPlanWavesRejectsSameWaveDependencies(t *testing.T) {
	t.Parallel()
	_, err := PlanWaves([]Node{
		{TaskID: "a", WaveIndex: 1, TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "b", WaveIndex: 1, DependsOn: []string{"a"}, TargetFiles: []string{"b.go"}, TaskKind: model.TaskKindCode},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "same or later wave")
}

func TestPlanWavesRejectsWaveGaps(t *testing.T) {
	t.Parallel()
	_, err := PlanWaves([]Node{
		{TaskID: "a", WaveIndex: 1, TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "b", WaveIndex: 3, TargetFiles: []string{"b.go"}, TaskKind: model.TaskKindCode},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wave 2 missing")
}

func TestTaskPlanSemanticHashIgnoresCheckboxState(t *testing.T) {
	t.Parallel()

	unchecked := `# Tasks

- [ ] ` + "`task-a`" + ` Implement A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`
	checked := `# Tasks

- [x] ` + "`task-a`" + ` Implement A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`

	left, err := TaskPlanSemanticHash(unchecked)
	require.NoError(t, err)
	right, err := TaskPlanSemanticHash(checked)
	require.NoError(t, err)
	assert.Equal(t, left, right)
}

func TestTaskPlanSemanticHashAllowsEmptyPlan(t *testing.T) {
	t.Parallel()

	hash, err := TaskPlanSemanticHash("")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestTaskPlanSemanticHashRejectsMalformedPlan(t *testing.T) {
	t.Parallel()

	_, err := TaskPlanSemanticHash(`# Tasks

- [ ] ` + "`task-a`" + ` Implement A
  - unexpected_key: value
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown metadata key")
}

func TestExecutionResultJSONUsesTaskResultsKey(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(ExecutionResult{
		TaskResults: map[string]model.TaskRun{
			"task-a": {
				TaskID:            "task-a",
				RunSummaryVersion: 1,
				Verdict:           model.TaskVerdictPass,
			},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"task_results"`)
	assert.NotContains(t, string(raw), `"task_runs"`)
}
