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

func TestValidateNoOpJustificationEnforcesEnvelope(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		task            ExecutionTaskSummary
		hasChangedFiles bool
		wantErr         error
	}{
		{
			name: "empty justification always valid",
			task: ExecutionTaskSummary{Verdict: TaskVerdictFail, TaskKind: TaskKindCode},
		},
		{
			name:            "empty justification valid even with changed files",
			task:            ExecutionTaskSummary{Verdict: TaskVerdictPass, TaskKind: TaskKindCode},
			hasChangedFiles: true,
		},
		{
			name: "pass code zero files is the legitimate shape",
			task: ExecutionTaskSummary{Verdict: TaskVerdictPass, TaskKind: TaskKindCode, NoOpJustification: "no safe behavior-preserving change exists"},
		},
		{
			name:            "justification with changed files is a contradiction",
			task:            ExecutionTaskSummary{Verdict: TaskVerdictPass, TaskKind: TaskKindCode, NoOpJustification: "j"},
			hasChangedFiles: true,
			wantErr:         ErrNoOpJustificationWithChangedFiles,
		},
		{
			name:    "justification on a non-pass code task is out of envelope",
			task:    ExecutionTaskSummary{Verdict: TaskVerdictFail, TaskKind: TaskKindCode, NoOpJustification: "j"},
			wantErr: ErrNoOpJustificationInvalidTask,
		},
		{
			name:    "justification on a pass verification task is out of envelope",
			task:    ExecutionTaskSummary{Verdict: TaskVerdictPass, TaskKind: TaskKindVerification, NoOpJustification: "j"},
			wantErr: ErrNoOpJustificationInvalidTask,
		},
		{
			name:    "justification on a pass investigation task is out of envelope",
			task:    ExecutionTaskSummary{Verdict: TaskVerdictPass, TaskKind: TaskKindInvestigation, NoOpJustification: "j"},
			wantErr: ErrNoOpJustificationInvalidTask,
		},
		{
			name:    "justification on a pass doc task is out of envelope",
			task:    ExecutionTaskSummary{Verdict: TaskVerdictPass, TaskKind: TaskKindDoc, NoOpJustification: "j"},
			wantErr: ErrNoOpJustificationInvalidTask,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.task.ValidateNoOpJustification(tc.hasChangedFiles)
			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestExecutionTaskSummaryValidateEnforcesNoOpJustificationEnvelope(t *testing.T) {
	t.Parallel()

	// Validate() must route through the envelope authority so the field cannot
	// ride out-of-envelope on any task that reaches Validate — including a summary
	// loaded from or saved to disk.
	contradiction := ExecutionTaskSummary{
		TaskID:            "task-a",
		Verdict:           TaskVerdictPass,
		TaskKind:          TaskKindCode,
		ChangedFiles:      []string{"internal/foo.go"},
		NoOpJustification: "no safe behavior-preserving change exists",
	}
	assert.ErrorIs(t, contradiction.Validate(), ErrNoOpJustificationWithChangedFiles)

	outOfEnvelope := ExecutionTaskSummary{
		TaskID:            "task-a",
		Verdict:           TaskVerdictPass,
		TaskKind:          TaskKindVerification,
		NoOpJustification: "no safe behavior-preserving change exists",
	}
	assert.ErrorIs(t, outOfEnvelope.Validate(), ErrNoOpJustificationInvalidTask)

	legitimate := ExecutionTaskSummary{
		TaskID:            "task-a",
		Verdict:           TaskVerdictPass,
		TaskKind:          TaskKindCode,
		NoOpJustification: "no safe behavior-preserving change exists",
	}
	assert.NoError(t, legitimate.Validate())
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
