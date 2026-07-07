package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionSummaryEqualIgnoresMonotonicClockData(t *testing.T) {
	t.Parallel()

	capturedAt := time.Now().UTC()
	taskCapturedAt := capturedAt.Add(-time.Minute)

	left := ExecutionSummary{
		Version:           ExecutionSummaryVersion,
		RunSummaryVersion: 2,
		CapturedAt:        capturedAt,
		OverallVerdict:    ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []ExecutionTaskSummary{
			{
				TaskID:            "task-a",
				Verdict:           TaskVerdictPass,
				TaskKind:          TaskKindCode,
				ChangedFiles:      []string{"cmd/status.go"},
				TargetFiles:       []string{"cmd/status.go"},
				EvidenceRef:       "artifacts/changes/demo/verification/task-a.yaml",
				EvidenceInputHash: "hash-a",
				Blockers:          []ReasonCode{},
				CapturedAt:        taskCapturedAt,
			},
		},
	}
	right := ExecutionSummary{
		Version:           ExecutionSummaryVersion,
		RunSummaryVersion: 2,
		CapturedAt:        capturedAt.Round(0),
		OverallVerdict:    ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []ExecutionTaskSummary{
			{
				TaskID:            "task-a",
				Verdict:           TaskVerdictPass,
				TaskKind:          TaskKindCode,
				ChangedFiles:      []string{"cmd/status.go"},
				TargetFiles:       []string{"cmd/status.go"},
				EvidenceRef:       "artifacts/changes/demo/verification/task-a.yaml",
				EvidenceInputHash: "hash-a",
				Blockers:          []ReasonCode{},
				CapturedAt:        taskCapturedAt.Round(0),
			},
		},
	}

	assert.True(t, left.Equal(right))
	assert.True(t, right.Equal(left))
}

func TestExecutionSummaryValidateRejectsDuplicateTaskIDs(t *testing.T) {
	t.Parallel()

	err := ExecutionSummary{
		Version:           ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    ExecutionVerdictFail,
		NonPassTasks:      []string{"task-a"},
		Tasks: []ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: TaskVerdictFail, TaskKind: TaskKindCode, CapturedAt: time.Now().UTC()},
			{TaskID: "task-a", Verdict: TaskVerdictFail, TaskKind: TaskKindDoc, CapturedAt: time.Now().UTC()},
		},
	}.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate task_id")
}

func TestExecutionSummaryValidateRejectsCompletedTaskSetMismatch(t *testing.T) {
	t.Parallel()

	err := ExecutionSummary{
		Version:           ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    ExecutionVerdictPass,
		CompletedTasks:    []string{},
		Tasks: []ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: TaskVerdictPass, TaskKind: TaskKindCode, CapturedAt: time.Now().UTC()},
		},
	}.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "completed_tasks")
}

func TestExecutionTaskSummaryEqualDistinguishesNoOpJustification(t *testing.T) {
	t.Parallel()

	base := ExecutionTaskSummary{
		TaskID:   "task-a",
		Verdict:  TaskVerdictPass,
		TaskKind: TaskKindCode,
	}
	justified := base
	justified.NoOpJustification = "no safe behavior-preserving change exists"

	assert.False(t, base.Equal(justified))
	assert.True(t, justified.Equal(justified))
}

func TestExecutionTaskSummaryNormalizeTrimsNoOpJustification(t *testing.T) {
	t.Parallel()

	task := ExecutionTaskSummary{
		TaskID:            "task-a",
		Verdict:           TaskVerdictPass,
		TaskKind:          TaskKindCode,
		NoOpJustification: "  spaced justification  ",
	}
	task.Normalize()

	assert.Equal(t, "spaced justification", task.NoOpJustification)
}

func TestExecutionSummaryValidateRejectsNonPassTaskSetMismatch(t *testing.T) {
	t.Parallel()

	err := ExecutionSummary{
		Version:           ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    ExecutionVerdictFail,
		NonPassTasks:      []string{},
		Tasks: []ExecutionTaskSummary{
			{TaskID: "task-a", Verdict: TaskVerdictFail, TaskKind: TaskKindCode, CapturedAt: time.Now().UTC()},
		},
	}.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "non_pass_tasks")
}
