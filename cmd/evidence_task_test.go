package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvidenceTaskRecordsRuntimeEvidenceAndBuildsExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		capturedAt := time.Now().UTC()
		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:evidence-task",
			// Must stay within the fixture wave-plan's target_files
			// (cmd/lifecycle_commands_test.go) so the Scope Contract passes; an
			// out-of-scope changed_file now reopens to S2 (scope_contract gate).
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
			"--captured-at", capturedAt.Format(time.RFC3339Nano),
			"--session-id", "session-a",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceTaskView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		wavePlan, err := state.LoadWavePlanForChange(root, change)
		require.NoError(t, err)
		expectedFreshnessInputs := state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01", wavePlan.TasksPlanHash)
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, "t-01", view.TaskID)
		assert.True(t, view.Recorded)
		assert.True(t, view.FreshnessInputs.Equal(expectedFreshnessInputs))

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		raw, err := os.ReadFile(taskEvidencePath)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(raw, &payload))
		assert.NotContains(t, payload, "input_hash")
		assert.Contains(t, view.Path, ".git/slipway/runtime/changes/"+slug+"/evidence/tasks/t-01.json")

		task, parsedAt, sessionID, err := progression.ParseTaskEvidence(root, taskEvidencePath, 1)
		require.NoError(t, err)
		assert.Equal(t, "t-01", task.TaskID)
		assert.Equal(t, model.TaskVerdictPass, task.Verdict)
		assert.Equal(t, model.TaskKindVerification, task.TaskKind)
		assert.Equal(t, []string{"cmd/lifecycle_commands_test.go"}, task.ChangedFiles)
		assert.Equal(t, []string{"cmd/lifecycle_commands_test.go"}, task.TargetFiles)
		assert.True(t, capturedAt.Equal(parsedAt))
		assert.Equal(t, "session-a", sessionID)
		assert.True(t, task.FreshnessInputs.Equal(expectedFreshnessInputs))

		writeSkillVerification(t, root, slug, progression.SkillWaveOrchestration, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  capturedAt.Add(time.Second),
			RunVersion: 1,
			References: []string{"task:evidence:t-01"},
		})
		_, err = buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
		require.NoError(t, err)

		summary, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		require.Len(t, summary.Tasks, 1)
		assert.Equal(t, "t-01", summary.Tasks[0].TaskID)
		assert.Equal(t, model.TaskVerdictPass, summary.Tasks[0].Verdict)
		assert.True(t, summary.Tasks[0].FreshnessInputs.Equal(expectedFreshnessInputs))
	})
}

func TestEvidenceSkillRecordsCLIStampedVerificationAndDigest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 2, "t-01")
		writePassingWaveEvidence(t, root, slug, 2)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		notesPath := filepath.Join(root, "review-notes.md")
		require.NoError(t, os.WriteFile(notesPath, []byte("review notes from disk\n"), 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", "pass",
			"--reference", "layer:R0=pass",
			"--reference", "scope_contract:pass",
			"--notes-file", "review-notes.md",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.Skill)
		assert.Equal(t, 2, view.RunVersion)
		assert.True(t, view.Recorded)
		assert.True(t, view.Stamped)
		assert.Contains(t, view.Path, "verification/"+progression.SkillSpecComplianceReview+".yaml")

		rec, err := state.LoadVerification(root, slug, progression.SkillSpecComplianceReview)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Equal(t, 2, rec.RunVersion)
		assert.False(t, rec.Timestamp.IsZero())
		assert.Equal(t, "review notes from disk", rec.Notes)
		assert.Equal(t, []string{"layer:R0=pass", "scope_contract:pass"}, rec.References)

		digests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		require.NotNil(t, digests)
		assert.Contains(t, digests.Skills, progression.SkillSpecComplianceReview)
	})
}

func TestEvidenceSkillRejectsUnknownSkillWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--skill", "../escape",
			"--verdict", "pass",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_invalid", cliErr.ErrorCode)

		_, statErr := os.Stat(state.VerificationFilePath(root, slug, "../escape"))
		require.Error(t, statErr)
	})
}

func TestEvidenceSkillRejectsRunSummaryBoundSkillWithoutExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "skill evidence command")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", "pass",
		})
		err = cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_run_summary_missing", cliErr.ErrorCode)
	})
}

func TestEvidenceSkillRecordsWaveOrchestrationBeforeExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		taskCmd := commandForRoot(t, root, makeEvidenceCmd())
		taskCmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:wave-bootstrap-task",
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
		})
		require.NoError(t, taskCmd.Execute())

		summary, err := state.LoadOptionalExecutionSummary(root, slug)
		require.NoError(t, err)
		require.Nil(t, summary)

		notesPath := filepath.Join(root, "wave-notes.md")
		require.NoError(t, os.WriteFile(notesPath, []byte("wave evidence from task ledger\n"), 0o644))
		skillCmd := commandForRoot(t, root, makeEvidenceCmd())
		skillCmd.SetArgs([]string{
			"skill",
			"--json",
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes-file", "wave-notes.md",
		})
		var out bytes.Buffer
		skillCmd.SetOut(&out)
		require.NoError(t, skillCmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, progression.SkillWaveOrchestration, view.Skill)
		assert.Equal(t, 1, view.RunVersion)
		assert.True(t, view.Recorded)
		assert.True(t, view.Stamped)

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Equal(t, 1, rec.RunVersion)
		assert.Equal(t, "wave evidence from task ledger", rec.Notes)

		digests, err := state.LoadOptionalEvidenceDigestsForChange(root, model.NewChange(slug))
		require.NoError(t, err)
		require.NotNil(t, digests)
		assert.Contains(t, digests.Skills, progression.SkillWaveOrchestration)
	})
}

func TestEvidenceSkillRejectsWrongWorkflowStateWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS2Execute
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", "pass",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_wrong_state", cliErr.ErrorCode)

		_, statErr := os.Stat(state.VerificationFilePath(root, slug, progression.SkillSpecComplianceReview))
		require.Error(t, statErr)
	})
}

func TestEvidenceTaskRejectsUnsafeTaskID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "../escape",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:unsafe",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_id_invalid", cliErr.ErrorCode)
	})
}

func TestNormalizeEvidencePathsUsesPublicSlashPaths(t *testing.T) {
	t.Parallel()

	got, err := normalizeEvidencePaths([]string{`cmd\run.go`, "cmd/run.go"})
	require.NoError(t, err)
	assert.Equal(t, []string{"cmd/run.go"}, got)
}

func TestNormalizeEvidencePathsRejectsWindowsAbsolutePath(t *testing.T) {
	t.Parallel()

	_, err := normalizeEvidencePaths([]string{`C:\tmp\file.go`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace-relative")
}

func TestEvidenceTaskRejectsInvalidVerdictWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "maybe",
			"--evidence-ref", "test:invalid-verdict",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_verdict_invalid", cliErr.ErrorCode)

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		_, statErr := os.Stat(taskEvidencePath)
		require.Error(t, statErr)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestEvidenceTaskRejectsFutureCapturedAtWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:future-captured-at",
			"--captured-at", time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano),
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_captured_at_invalid", cliErr.ErrorCode)

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		_, statErr := os.Stat(taskEvidencePath)
		require.Error(t, statErr)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestEvidenceTaskRejectsRunVersionMismatchWhenWaveEvidenceExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)
		writePassingWaveEvidence(t, root, slug, 2)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:wrong-run-version",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_run_summary_version_mismatch", cliErr.ErrorCode)
		assert.Equal(t, 2, cliErr.Details["expected"])
		assert.Equal(t, 1, cliErr.Details["got"])

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		_, statErr := os.Stat(taskEvidencePath)
		require.Error(t, statErr)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestEvidenceTaskRejectsNonWorkspaceRelativePathsWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		flag      string
		value     string
		errorCode string
	}{
		{
			name:      "changed file parent traversal",
			flag:      "--changed-file",
			value:     "../escape.go",
			errorCode: "evidence_task_changed_file_invalid",
		},
		{
			name:      "target file absolute path",
			flag:      "--target-file",
			value:     "/tmp/escape.go",
			errorCode: "evidence_task_target_file_invalid",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			withCommandWorkspace(t, root, func() {
				initTestWorkspace(t, root)
				slug, _ := createEvidenceTaskFixture(t, root)

				cmd := commandForRoot(t, root, makeEvidenceCmd())
				cmd.SetArgs([]string{
					"task",
					"--task-id", "t-01",
					"--run-summary-version", "1",
					"--task-kind", "verification",
					"--verdict", "pass",
					"--evidence-ref", "test:invalid-path",
					tt.flag, tt.value,
				})
				err := cmd.Execute()
				cliErr := asCLIError(err)
				require.NotNil(t, cliErr)
				assert.Equal(t, tt.errorCode, cliErr.ErrorCode)

				taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
				_, statErr := os.Stat(taskEvidencePath)
				require.Error(t, statErr)
				assert.True(t, os.IsNotExist(statErr))
			})
		})
	}
}

func createEvidenceTaskFixture(t *testing.T, root string) (string, model.Change) {
	t.Helper()

	slug := createGovernedRequest(t, root, "L2", "evidence task command")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	return slug, change
}
