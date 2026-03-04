package action

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLevelPaths(t *testing.T) {
	assert.Equal(t, []model.WorkflowState{
		model.StateS0Intake,
		model.StateS1Analyze,
		model.StateS6RunWaves,
		model.StateS7Review,
		model.StateS8Verify,
		model.StateDone,
	}, LevelPath(model.LevelL1))

	assert.Equal(t, []model.WorkflowState{
		model.StateS0Intake,
		model.StateS1Analyze,
		model.StateS4SpecBundle,
		model.StateS5PlanAudit,
		model.StateS6RunWaves,
		model.StateS7Review,
		model.StateS8Verify,
		model.StateDone,
	}, LevelPath(model.LevelL2))
}

func TestEvaluateFixedLevelSafety(t *testing.T) {
	conflicts := EvaluateFixedLevelSafety(model.LevelL1, "auth_authz")
	assert.Contains(t, conflicts, "fixed_level_guardrail_conflict")

	conflicts = EvaluateFixedLevelSafety(model.LevelL3, "auth_authz")
	assert.Empty(t, conflicts)
}

func TestRunL1DoAutoChecksPass(t *testing.T) {
	requestID := mustRequestID(t)
	admission := model.NewAdmissionState(requestID)
	admission.CurrentState = model.StateS6RunWaves
	admission.LatestFrozenRunSummaryVersion = 1
	admission.TaskRuns = map[string]model.TaskRun{
		"t1__rv1": {
			TaskID:            "t1",
			RunSummaryVersion: 1,
			Verdict:           model.TaskVerdictPass,
			EvidenceRef:       ".spln/evidence/tasks/r/rv1/t1.json",
		},
	}

	result := RunL1DoAutoChecks(admission)
	assert.True(t, result.DoneReady)
	assert.Empty(t, result.Blockers)
	assert.Equal(t, model.StateS8Verify, result.NextState)
}

func TestRunL1DoAutoChecksFail(t *testing.T) {
	requestID := mustRequestID(t)
	admission := model.NewAdmissionState(requestID)
	admission.CurrentState = model.StateS6RunWaves
	admission.LatestFrozenRunSummaryVersion = 1
	admission.TaskRuns = map[string]model.TaskRun{
		"t1__rv1": {
			TaskID:            "t1",
			RunSummaryVersion: 1,
			Verdict:           model.TaskVerdictFail,
		},
	}

	result := RunL1DoAutoChecks(admission)
	assert.False(t, result.DoneReady)
	assert.NotEmpty(t, result.Blockers)
	assert.Equal(t, model.StateS6RunWaves, result.NextState)
}

func TestCanFinalizeDone(t *testing.T) {
	assert.True(t, CanFinalizeDone(model.StateS8Verify))
	assert.False(t, CanFinalizeDone(model.StateS6RunWaves))
}

func TestApplyAnalyzeOverrideAdmissionPersistsConflictsAndKeepsLevel(t *testing.T) {
	requestID := mustRequestID(t)
	admission := model.NewAdmissionState(requestID)
	admission.Level = model.LevelL1
	admission.LevelSource = model.LevelSourceUserSelected
	admission.CurrentState = model.StateS6RunWaves
	admission.RouteSnapshot = model.RouteSnapshot{
		Scores: model.Scores{},
	}
	admission.TaskRuns = map[string]model.TaskRun{
		"task-a__rv1": {
			TaskID:            "task-a",
			RunSummaryVersion: 1,
			Verdict:           model.TaskVerdictPass,
		},
	}
	admission.ActionHistory = []model.ActionEvent{
		{Action: "do", State: model.StateS6RunWaves, Timestamp: time.Now().UTC()},
	}

	err := ApplyAnalyzeOverrideAdmission(&admission, []string{
		"fixed_level_guardrail_conflict",
		"",
		"fixed_level_guardrail_conflict",
		"requires_pivot",
	})
	require.NoError(t, err)

	assert.Equal(t, model.StateS1Analyze, admission.CurrentState)
	assert.Equal(t, model.LevelL1, admission.Level)
	assert.Equal(t, model.LevelSourceUserSelected, admission.LevelSource)
	assert.Equal(t, []string{"fixed_level_guardrail_conflict", "requires_pivot"}, admission.RouteSnapshot.BlockingConflicts)
	assert.Len(t, admission.TaskRuns, 1)
	assert.Len(t, admission.ActionHistory, 1)
}

func TestApplyAnalyzeOverrideChangePersistsConflictsAndKeepsLevel(t *testing.T) {
	change := model.NewChangeState(mustRequestID(t), "slug")
	change.Level = model.LevelL2
	change.LevelSource = model.LevelSourceAuto
	change.CurrentState = model.StateS7Review
	change.RouteSnapshot = model.RouteSnapshot{
		Scores: model.Scores{},
	}
	change.TaskRuns = map[string]model.TaskRun{
		"task-b__rv2": {
			TaskID:            "task-b",
			RunSummaryVersion: 2,
			Verdict:           model.TaskVerdictPass,
		},
	}

	err := ApplyAnalyzeOverrideChange(&change, []string{"needs_reroute"})
	require.NoError(t, err)

	assert.Equal(t, model.StateS1Analyze, change.CurrentState)
	assert.Equal(t, model.LevelL2, change.Level)
	assert.Equal(t, model.LevelSourceAuto, change.LevelSource)
	assert.Equal(t, []string{"needs_reroute"}, change.RouteSnapshot.BlockingConflicts)
	assert.Len(t, change.TaskRuns, 1)
}

func TestResolveLoopTransitionMatrix(t *testing.T) {
	cases := []struct {
		name      string
		input     LoopTransitionInput
		wantState model.WorkflowState
	}{
		{
			name: "S5 plan audit fail to S4",
			input: LoopTransitionInput{
				Level:        model.LevelL2,
				CurrentState: model.StateS5PlanAudit,
				Trigger:      LoopTriggerPlanAuditFailed,
			},
			wantState: model.StateS4SpecBundle,
		},
		{
			name: "S6 retry to S6",
			input: LoopTransitionInput{
				Level:        model.LevelL2,
				CurrentState: model.StateS6RunWaves,
				Trigger:      LoopTriggerRetry,
			},
			wantState: model.StateS6RunWaves,
		},
		{
			name: "S6 reroute to S1",
			input: LoopTransitionInput{
				Level:        model.LevelL2,
				CurrentState: model.StateS6RunWaves,
				Trigger:      LoopTriggerPivotReroute,
			},
			wantState: model.StateS1Analyze,
		},
		{
			name: "S7 review fail to S6",
			input: LoopTransitionInput{
				Level:        model.LevelL2,
				CurrentState: model.StateS7Review,
				Trigger:      LoopTriggerReviewFailed,
			},
			wantState: model.StateS6RunWaves,
		},
		{
			name: "S7 reroute to S1",
			input: LoopTransitionInput{
				Level:        model.LevelL2,
				CurrentState: model.StateS7Review,
				Trigger:      LoopTriggerPivotReroute,
			},
			wantState: model.StateS1Analyze,
		},
		{
			name: "S8 review override to S7",
			input: LoopTransitionInput{
				Level:        model.LevelL2,
				CurrentState: model.StateS8Verify,
				Trigger:      LoopTriggerReviewOverride,
			},
			wantState: model.StateS7Review,
		},
		{
			name: "S8 verify fail to S6",
			input: LoopTransitionInput{
				Level:        model.LevelL2,
				CurrentState: model.StateS8Verify,
				Trigger:      LoopTriggerVerifyFailed,
			},
			wantState: model.StateS6RunWaves,
		},
		{
			name: "S8 reroute to S1",
			input: LoopTransitionInput{
				Level:        model.LevelL2,
				CurrentState: model.StateS8Verify,
				Trigger:      LoopTriggerPivotReroute,
			},
			wantState: model.StateS1Analyze,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveLoopTransition(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.wantState, got)
		})
	}
}

func TestResolveLoopTransitionRescopeSemantics(t *testing.T) {
	next, err := ResolveLoopTransition(LoopTransitionInput{
		Level:             model.LevelL2,
		CurrentState:      model.StateS6RunWaves,
		Trigger:           LoopTriggerPivotRescope,
		AnalyzeLevel:      model.LevelL2,
		PivotGateApproved: true,
	})
	require.NoError(t, err)
	assert.Equal(t, model.StateS4SpecBundle, next)

	next, err = ResolveLoopTransition(LoopTransitionInput{
		Level:             model.LevelL3,
		CurrentState:      model.StateS6RunWaves,
		Trigger:           LoopTriggerPivotRescope,
		AnalyzeLevel:      model.LevelL3,
		PivotGateApproved: true,
	})
	require.NoError(t, err)
	assert.Equal(t, model.StateS3ScopeConfirmation, next)

	next, err = ResolveLoopTransition(LoopTransitionInput{
		Level:             model.LevelL2,
		CurrentState:      model.StateS6RunWaves,
		Trigger:           LoopTriggerPivotRescope,
		AnalyzeLevel:      model.LevelL3,
		PivotGateApproved: true,
	})
	require.NoError(t, err)
	assert.Equal(t, model.StateS1Analyze, next)
}

func TestResolveLoopTransitionRescopeRejectedOutsideS6(t *testing.T) {
	_, err := ResolveLoopTransition(LoopTransitionInput{
		Level:             model.LevelL3,
		CurrentState:      model.StateS7Review,
		Trigger:           LoopTriggerPivotRescope,
		AnalyzeLevel:      model.LevelL3,
		PivotGateApproved: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rescope requires current_state=S6_RUN_WAVES")
}

func TestResolveLoopTransitionRescopeRequiresApprovedPivot(t *testing.T) {
	_, err := ResolveLoopTransition(LoopTransitionInput{
		Level:             model.LevelL2,
		CurrentState:      model.StateS6RunWaves,
		Trigger:           LoopTriggerPivotRescope,
		AnalyzeLevel:      model.LevelL2,
		PivotGateApproved: false,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rescope requires approved pivot")
}

func TestRunS2Discover(t *testing.T) {
	change := model.NewChangeState(mustRequestID(t), "slug")
	change.CurrentState = model.StateS2Discover

	err := RunS2Discover(&change, "")
	require.Error(t, err)
	assert.Equal(t, model.StateS2Discover, change.CurrentState)

	require.NoError(t, RunS2Discover(&change, "findings"))
	assert.Equal(t, model.StateS3ScopeConfirmation, change.CurrentState)
}

func TestRunS3ScopeConfirmation(t *testing.T) {
	repoRoot, worktreePath := setupRepoWithWorktree(t)

	change := model.NewChangeState(mustRequestID(t), "slug")
	change.CurrentState = model.StateS3ScopeConfirmation
	require.NoError(t, RunS3ScopeConfirmation(&change, repoRoot, worktreePath, "feature"))

	assert.Equal(t, model.StateS4SpecBundle, change.CurrentState)
	assert.Equal(t, worktreePath, change.WorktreePath)
	assert.Equal(t, "feature", change.WorktreeBranch)
}

func TestRunS4SpecBundle(t *testing.T) {
	root := t.TempDir()
	change := model.NewChangeState(mustRequestID(t), "slug")
	change.Level = model.LevelL2
	change.CurrentState = model.StateS4SpecBundle

	err := RunS4SpecBundle(root, &change)
	require.Error(t, err)
	assert.Equal(t, model.StateS4SpecBundle, change.CurrentState)

	base := filepath.Join(root, "aircraft", "changes", "slug")
	require.NoError(t, os.MkdirAll(base, 0o755))
	for _, file := range []string{"change.yaml", "proposal.md", "spec.md", "design.md", "tasks.md", "assurance.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(base, file), []byte("x"), 0o644))
	}
	require.NoError(t, RunS4SpecBundle(root, &change))
	assert.Equal(t, model.StateS5PlanAudit, change.CurrentState)
}

func setupRepoWithWorktree(t *testing.T) (repoRoot string, worktreePath string) {
	t.Helper()
	repoRoot = t.TempDir()
	worktreePath = filepath.Join(t.TempDir(), "feature-wt")

	runGit(t, repoRoot, "init", "--initial-branch=main")
	runGit(t, repoRoot, "config", "user.email", "test@example.com")
	runGit(t, repoRoot, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello"), 0o644))
	runGit(t, repoRoot, "add", ".")
	runGit(t, repoRoot, "commit", "-m", "init")
	runGit(t, repoRoot, "branch", "feature")
	runGit(t, repoRoot, "worktree", "add", worktreePath, "feature")

	return repoRoot, worktreePath
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}

func mustRequestID(t *testing.T) string {
	t.Helper()
	id, err := model.NewRequestID()
	require.NoError(t, err)
	return id
}
