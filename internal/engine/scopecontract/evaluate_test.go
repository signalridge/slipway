package scopecontract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluatePassesWhenChangedFilesAreWithinPlannedTargets(t *testing.T) {
	t.Parallel()

	report := Evaluate(wave.TaskPlan{Tasks: []wave.TaskNode{
		task("t-01", model.TaskKindCode, "cmd/validate.go", "internal/engine/scopecontract/**", "docs/"),
	}}, summary(taskRun("t-01", model.TaskKindCode, "cmd/validate.go", "internal/engine/scopecontract/evaluate.go", "docs/guide.md")))

	assert.Equal(t, StatusPass, report.Status)
	assert.Empty(t, report.Blockers)
	assert.Equal(t, []string{"cmd/validate.go", "docs/", "internal/engine/scopecontract/**"}, report.PlannedTargets)
	assert.Equal(t, []string{"cmd/validate.go", "docs/guide.md", "internal/engine/scopecontract/evaluate.go"}, report.ChangedFiles)
}

func TestEvaluateReportsScopeDriftDeterministically(t *testing.T) {
	t.Parallel()

	report := Evaluate(wave.TaskPlan{Tasks: []wave.TaskNode{
		task("t-01", model.TaskKindCode, "cmd/validate.go"),
	}}, summary(taskRun("t-01", model.TaskKindCode, "internal/secret.go", "cmd/review.go")))

	assert.Equal(t, StatusFail, report.Status)
	assert.Equal(t, []string{"cmd/review.go", "internal/secret.go"}, report.OutOfScopeFiles)
	assert.Contains(t, model.ReasonSpecs(report.Blockers), "scope_contract_drift:cmd/review.go,internal/secret.go")
}

func TestEvaluateTreatsBareNonGlobTargetAsExactFileOnly(t *testing.T) {
	t.Parallel()

	report := Evaluate(wave.TaskPlan{Tasks: []wave.TaskNode{
		task("t-01", model.TaskKindCode, "docs/api"),
	}}, summary(taskRun("t-01", model.TaskKindCode, "docs/api/v2.md")))

	assert.Equal(t, StatusFail, report.Status)
	assert.Equal(t, []string{"docs/api/v2.md"}, report.OutOfScopeFiles)
	assert.Contains(t, model.ReasonSpecs(report.Blockers), "scope_contract_drift:docs/api/v2.md")
}

func TestEvaluateAllowsExplicitDirectoryTargets(t *testing.T) {
	t.Parallel()

	report := Evaluate(wave.TaskPlan{Tasks: []wave.TaskNode{
		task("t-01", model.TaskKindCode, "docs/api/"),
	}}, summary(taskRun("t-01", model.TaskKindCode, "docs/api/v2.md")))

	assert.Equal(t, StatusPass, report.Status)
	assert.Empty(t, report.OutOfScopeFiles)
}

func TestEvaluateReportsMissingContractAndChangedFilesEvidence(t *testing.T) {
	t.Parallel()

	report := Evaluate(wave.TaskPlan{Tasks: []wave.TaskNode{
		task("t-01", model.TaskKindCode),
	}}, summary(model.ExecutionTaskSummary{
		TaskID:   "t-01",
		Verdict:  model.TaskVerdictPass,
		TaskKind: model.TaskKindCode,
	}))

	assert.Equal(t, StatusFail, report.Status)
	assert.Contains(t, model.ReasonSpecs(report.Blockers), "scope_contract_missing:t-01")
	assert.Contains(t, model.ReasonSpecs(report.Blockers), "scope_contract_changed_files_missing:t-01")
}

func TestEvaluateReportsPlannedMutableTaskMissingFromExecutionSummary(t *testing.T) {
	t.Parallel()

	report := Evaluate(wave.TaskPlan{Tasks: []wave.TaskNode{
		task("t-01", model.TaskKindCode, "cmd/validate.go"),
		task("t-02", model.TaskKindVerification, "artifacts/changes/demo/**"),
	}}, summary())

	assert.Equal(t, StatusFail, report.Status)
	assert.Equal(t, []string{"t-01"}, report.MissingChangedFileTasks)
	assert.Contains(t, model.ReasonSpecs(report.Blockers), "scope_contract_changed_files_missing:t-01")
}

func TestEvaluateDoesNotRequireChangedFilesForVerificationTasks(t *testing.T) {
	t.Parallel()

	report := Evaluate(wave.TaskPlan{Tasks: []wave.TaskNode{
		task("t-01", model.TaskKindVerification, "artifacts/changes/demo/**"),
	}}, summary(model.ExecutionTaskSummary{
		TaskID:   "t-01",
		Verdict:  model.TaskVerdictPass,
		TaskKind: model.TaskKindVerification,
	}))

	assert.Equal(t, StatusPass, report.Status)
	assert.Empty(t, report.MissingChangedFileTasks)
}

func TestEvaluateSkipsBeforeExecutionSummaryExists(t *testing.T) {
	t.Parallel()

	report := Evaluate(wave.TaskPlan{Tasks: []wave.TaskNode{
		task("t-01", model.TaskKindCode, "cmd/validate.go"),
	}}, nil)

	assert.Equal(t, StatusNotApplicable, report.Status)
	assert.Empty(t, report.Blockers)
}

func TestEvaluateBundleReadsTasksMarkdown(t *testing.T) {
	t.Parallel()

	bundleDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement validation
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/validate.go"]
  - task_kind: code
`), 0o644))

	report, err := EvaluateBundle(bundleDir, summary(taskRun("t-01", model.TaskKindCode, "cmd/validate.go")))
	require.NoError(t, err)
	assert.Equal(t, StatusPass, report.Status)
}

func task(taskID string, kind model.TaskKind, targetFiles ...string) wave.TaskNode {
	return wave.TaskNode{
		Node: wave.Node{
			TaskID:      taskID,
			TaskKind:    kind,
			TargetFiles: targetFiles,
		},
	}
}

func taskRun(taskID string, kind model.TaskKind, changedFiles ...string) model.ExecutionTaskSummary {
	return model.ExecutionTaskSummary{
		TaskID:       taskID,
		Verdict:      model.TaskVerdictPass,
		TaskKind:     kind,
		ChangedFiles: changedFiles,
	}
}

func summary(tasks ...model.ExecutionTaskSummary) *model.ExecutionSummary {
	out := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		OverallVerdict:    model.ExecutionVerdictPass,
		Tasks:             tasks,
	}
	out.Normalize()
	return out
}
