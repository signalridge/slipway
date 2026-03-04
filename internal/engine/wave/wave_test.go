package wave

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeL1Brief(t *testing.T) {
	requestID := mustRequestID(t)
	nodes := NormalizeL1Brief(requestID, []string{"a", "b"})
	require.Len(t, nodes, 2)
	assert.Equal(t, "l1-"+requestID[:8]+"-01", nodes[0].TaskID)
	assert.Equal(t, "l1-"+requestID[:8]+"-02", nodes[1].TaskID)
	assert.Equal(t, []string{nodes[0].TaskID}, nodes[1].DependsOn)
}

func TestPlanWavesDeterministicAndConflictSplit(t *testing.T) {
	nodes := []Node{
		{TaskID: "b", DependsOn: nil, TargetFiles: []string{"x.go"}, TaskKind: model.TaskKindCode, Priority: 99},
		{TaskID: "a", DependsOn: nil, TargetFiles: []string{"x.go"}, TaskKind: model.TaskKindCode, Priority: 1},
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

func TestPlanWavesEmptyTargetIsolated(t *testing.T) {
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

func TestDetectPostWaveFileOverlap(t *testing.T) {
	results := []TaskResult{
		{TaskID: "a", ChangedFiles: []string{"x.go"}},
		{TaskID: "b", ChangedFiles: []string{"x.go"}},
	}
	conflicts := DetectPostWaveFileOverlap(results)
	assert.NotEmpty(t, conflicts)
}

func TestPersistFrozenRunSummaryAndPointer(t *testing.T) {
	root := t.TempDir()
	requestID := mustRequestID(t)

	require.NoError(t, os.MkdirAll(filepath.Join(root, ".spln", "runtime", "admissions"), 0o755))
	admission := model.NewAdmissionState(requestID)
	admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
	require.NoError(t, state.SaveAdmission(root, admission))

	summary := RunSummary{
		RequestID:         requestID,
		RunSummaryVersion: 1,
		CompletedTasks:    []string{"t1"},
		NonPassTasks:      []string{},
		CarriedDebt:       []string{},
		EvidenceSet:       []string{"e1"},
		OpenBlockers:      []string{},
		FrozenAt:          time.Now().UTC(),
	}
	path, err := PersistFrozenRunSummary(root, summary)
	require.NoError(t, err)

	_, err = os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, UpdateFrozenRunSummaryPointer(root, requestID, 1))
	loaded, err := state.LoadAdmission(root, requestID)
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.LatestFrozenRunSummaryVersion)
}

func TestExecutePlanRetryThenPass(t *testing.T) {
	plan := []Wave{
		{
			Nodes: []Node{
				{TaskID: "task-a", TaskKind: model.TaskKindCode},
			},
		},
	}

	attempts := map[string]int{}
	executor := func(node Node, _ int) model.TaskRun {
		attempts[node.TaskID]++
		verdict := model.TaskVerdictFail
		if attempts[node.TaskID] >= 2 {
			verdict = model.TaskVerdictPass
		}
		return model.TaskRun{
			TaskID:       node.TaskID,
			TaskKind:     node.TaskKind,
			Verdict:      verdict,
			ChangedFiles: []string{node.TaskID + ".go"},
			Blockers:     blockersForVerdict(verdict),
		}
	}

	decider := func(checkpoint ControlCheckpoint) ControlDecision {
		return ControlDecisionRetry
	}

	result, err := ExecutePlan(plan, 1, ExecutionOptions{
		Parallelization:   false,
		MaxRetriesPerTask: 2,
	}, executor, decider)
	require.NoError(t, err)
	require.False(t, result.Aborted)
	require.False(t, result.PivotRequired)
	require.Nil(t, result.Checkpoint)

	run := result.TaskRuns["task-a__rv1"]
	assert.Equal(t, model.TaskVerdictPass, run.Verdict)
}

func TestExecutePlanRetryBudgetExhaustedReturnsCheckpoint(t *testing.T) {
	plan := []Wave{
		{
			Nodes: []Node{
				{TaskID: "task-a", TaskKind: model.TaskKindCode},
			},
		},
	}

	executor := func(node Node, _ int) model.TaskRun {
		return model.TaskRun{
			TaskID:   node.TaskID,
			TaskKind: node.TaskKind,
			Verdict:  model.TaskVerdictFail,
			Blockers: []string{"failed"},
		}
	}
	decider := func(_ ControlCheckpoint) ControlDecision {
		return ControlDecisionRetry
	}

	result, err := ExecutePlan(plan, 1, ExecutionOptions{
		Parallelization:   false,
		MaxRetriesPerTask: 1,
	}, executor, decider)
	require.NoError(t, err)
	require.NotNil(t, result.Checkpoint)
	assert.Contains(t, result.Checkpoint.NonPassTaskIDs, "task-a")
}

func TestExecutePlanOtherTaskRequiresManualCheckpoint(t *testing.T) {
	plan := []Wave{
		{
			Nodes: []Node{
				{TaskID: "task-other", TaskKind: model.TaskKindOther},
			},
		},
	}
	executor := func(node Node, _ int) model.TaskRun {
		return model.TaskRun{
			TaskID:   node.TaskID,
			TaskKind: node.TaskKind,
			Verdict:  model.TaskVerdictPass,
		}
	}

	result, err := ExecutePlan(plan, 1, ExecutionOptions{}, executor, nil)
	require.NoError(t, err)
	require.NotNil(t, result.Checkpoint)
	assert.True(t, result.Aborted)

	run := result.TaskRuns["task-other__rv1"]
	assert.Equal(t, model.TaskVerdictIncomplete, run.Verdict)
	assert.Contains(t, run.Blockers, "manual_checkpoint_required")
}

func TestExecutePlanPivotDecision(t *testing.T) {
	plan := []Wave{
		{
			Nodes: []Node{
				{TaskID: "task-a", TaskKind: model.TaskKindCode},
			},
		},
	}
	executor := func(node Node, _ int) model.TaskRun {
		return model.TaskRun{
			TaskID:   node.TaskID,
			TaskKind: node.TaskKind,
			Verdict:  model.TaskVerdictBlocked,
			Blockers: []string{"blocked"},
		}
	}
	decider := func(_ ControlCheckpoint) ControlDecision {
		return ControlDecisionPivot
	}

	result, err := ExecutePlan(plan, 1, ExecutionOptions{}, executor, decider)
	require.NoError(t, err)
	assert.True(t, result.PivotRequired)
	require.NotNil(t, result.Checkpoint)
	assert.Equal(t, 0, result.Checkpoint.WaveIndex)
}

func blockersForVerdict(verdict model.TaskVerdict) []string {
	if verdict == model.TaskVerdictPass {
		return nil
	}
	return []string{"retryable"}
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

func mustRequestID(t *testing.T) string {
	t.Helper()
	id, err := model.NewRequestID()
	require.NoError(t, err)
	return id
}
