package cmd

import (
	"path/filepath"
	"testing"
	"time"

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
	tasks := make([]model.ExecutionTaskSummary, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		tasks = append(tasks, model.ExecutionTaskSummary{
			TaskID:       taskID,
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindCode,
			ChangedFiles: []string{"cmd/placeholder.go"},
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
	writeExecutionSummary(t, root, slug, summary)
}
