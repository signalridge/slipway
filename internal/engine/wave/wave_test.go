package wave

import (
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanWavesDeterministicAndConflictSplit(t *testing.T) {
	t.Parallel()
	nodes := []Node{
		{TaskID: "b", DependsOn: nil, TargetFiles: []string{"x.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "a", DependsOn: nil, TargetFiles: []string{"x.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "c", DependsOn: []string{"a"}, TargetFiles: []string{"y.go"}, TaskKind: model.TaskKindCode},
	}
	waves, err := PlanWaves(nodes)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(waves), 2)
	// a and b overlap target_files, so they must not be in same wave.
	for _, wave := range waves {
		var ids []string
		for _, n := range wave.Nodes {
			ids = append(ids, n.TaskID)
		}
		assert.False(t, containsAll(ids, "a", "b"))
	}
}

func TestPlanWavesOtherIsolation(t *testing.T) {
	t.Parallel()
	nodes := []Node{
		{TaskID: "a", TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindCode},
		{TaskID: "z", TaskKind: model.TaskKindOther},
	}
	waves, err := PlanWaves(nodes)
	require.NoError(t, err)

	for _, wave := range waves {
		if len(wave.Nodes) == 1 && wave.Nodes[0].TaskKind == model.TaskKindOther {
			return
		}
	}
	t.Fatalf("expected isolated wave for task_kind=other")
}

func TestPlanWavesRejectsReservedTaskIDDelimiter(t *testing.T) {
	t.Parallel()

	_, err := PlanWaves([]Node{
		{TaskID: "task-a__rvshadow", TaskKind: model.TaskKindCode},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `task_id must not contain delimiter "__rv"`)
}

func TestPlanWavesEmptyTargetIsolated(t *testing.T) {
	t.Parallel()
	nodes := []Node{
		{TaskID: "a", TaskKind: model.TaskKindCode},
		{TaskID: "b", TargetFiles: []string{"b.go"}, TaskKind: model.TaskKindCode},
	}
	waves, err := PlanWaves(nodes)
	require.NoError(t, err)
	for _, wave := range waves {
		if len(wave.Nodes) > 1 {
			for _, n := range wave.Nodes {
				assert.NotEmpty(t, n.TargetFiles)
			}
		}
	}
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
			"task-a__rv1": {
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

func containsAll(haystack []string, values ...string) bool {
	found := map[string]bool{}
	for _, item := range haystack {
		found[item] = true
	}
	for _, value := range values {
		if !found[value] {
			return false
		}
	}
	return true
}
