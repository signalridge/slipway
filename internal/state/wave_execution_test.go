package state

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

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
