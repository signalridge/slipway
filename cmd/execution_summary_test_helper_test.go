package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/require"
)

func writeExecutionSummary(t *testing.T, root, slug string, summary model.ExecutionSummary) {
	t.Helper()
	require.NoError(t, state.SaveExecutionSummary(root, slug, summary))
}

func executionSummaryPathForTest(root, slug string) string {
	return state.ExecutionSummaryPathForRead(root, slug)
}

func verificationReadPathForTest(root, slug, skillName string) string {
	path := filepath.Join(state.VerificationDir(root, slug), skillName+".yaml")
	if normalizedPath, err := state.NormalizePath(path); err == nil {
		return normalizedPath
	}
	return path
}

func writePassingExecutionSummary(t *testing.T, root, slug string, runVersion int, taskIDs ...string) {
	t.Helper()
	now := time.Now().UTC()
	workspaceRoot := root
	var change *model.Change
	if loaded, err := state.LoadChange(root, slug); err == nil {
		change = &loaded
		if paths, err := state.ResolveChangePaths(root, loaded); err == nil && paths.WorkspaceRoot != "" {
			workspaceRoot = paths.WorkspaceRoot
		}
	}
	targetsByTaskID := plannedTargetsByTaskID(t, root, slug)
	tasks := make([]model.ExecutionTaskSummary, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		targetFiles := append([]string(nil), targetsByTaskID[taskID]...)
		changedFiles := append([]string(nil), targetFiles...)
		if len(changedFiles) == 0 {
			changedFiles = []string{"cmd/placeholder.go"}
		}
		for _, rel := range changedFiles {
			ensureExecutionSummaryInputFile(t, workspaceRoot, rel)
		}
		tasks = append(tasks, model.ExecutionTaskSummary{
			TaskID:       taskID,
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindCode,
			ChangedFiles: changedFiles,
			TargetFiles:  targetFiles,
			CapturedAt:   now,
		})
	}
	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: runVersion,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    append([]string(nil), taskIDs...),
		Tasks:             tasks,
	}
	if change != nil {
		if hash, err := state.CurrentTasksPlanState(root, *change); err == nil {
			summary.TasksPlanHash = hash
		}
	}
	writeExecutionSummary(t, root, slug, summary)
	refreshPassingSkillDigestsForTest(t, root, slug)
}

func ensureExecutionSummaryInputFile(t *testing.T, root, rel string) {
	t.Helper()
	if rel == "" || filepath.IsAbs(rel) {
		return
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(path); err == nil {
		return
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("// test fixture input\n"), 0o644))
}

func plannedTargetsByTaskID(t *testing.T, root, slug string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	change, err := state.LoadChange(root, slug)
	if err != nil {
		return out
	}
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return out
	}
	raw, err := os.ReadFile(filepath.Join(bundleDir, "tasks.md"))
	if err != nil {
		return out
	}
	plan, err := wave.ParseTaskPlan(string(raw))
	if err != nil {
		return out
	}
	for _, task := range plan.Tasks {
		out[task.TaskID] = append([]string(nil), task.TargetFiles...)
	}
	return out
}
