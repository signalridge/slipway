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
		{TaskID: "b", DependsOn: nil, TargetFiles: []string{"x.go"}, TaskKind: model.TaskKindImplementation, Priority: 99},
		{TaskID: "a", DependsOn: nil, TargetFiles: []string{"x.go"}, TaskKind: model.TaskKindImplementation, Priority: 1},
		{TaskID: "c", DependsOn: []string{"a"}, TargetFiles: []string{"y.go"}, TaskKind: model.TaskKindImplementation},
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
		{TaskID: "a", TargetFiles: []string{"a.go"}, TaskKind: model.TaskKindImplementation},
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
		{TaskID: "a", TaskKind: model.TaskKindImplementation},
		{TaskID: "b", TargetFiles: []string{"b.go"}, TaskKind: model.TaskKindImplementation},
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
