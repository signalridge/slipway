package progression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ctxpack "github.com/signalridge/slipway/internal/engine/context"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanAuditInputDigestExcludesAssuranceAndIncludesStructuralTasks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("plan-audit-digest")
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	first, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
	require.NoError(t, err)
	require.NotContains(t, first.Inputs, "assurance.md", "plan-audit never audits assurance.md; it must not be a plan-audit digest input")
	require.Contains(t, first.Inputs, "decision.md")
	require.Contains(t, first.Inputs, "tasks.md")

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("# Intent\r\nship digest freshness\r\n"), 0o644))
	crlfIntent, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
	require.NoError(t, err)
	assert.Equal(t, first.Inputs["intent.md"], crlfIntent.Inputs["intent.md"], "line-ending-only prose rewrites must not stale planning evidence")

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(checkedDigestTasks()), 0o644))
	checked, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
	require.NoError(t, err)
	assert.Equal(t, first.Inputs["tasks.md"], checked.Inputs["tasks.md"], "checkbox-only edits must not stale tasks.md")

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(scopeOnlyDigestTasks()), 0o644))
	scopeOnly, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
	require.NoError(t, err)
	assert.Equal(t, first.Inputs["tasks.md"], scopeOnly.Inputs["tasks.md"], "target_files-only edits are scope changes, not structural plan-audit changes")

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(realChangedDigestTasks()), 0o644))
	realChange, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
	require.NoError(t, err)
	assert.NotEqual(t, first.Inputs["tasks.md"], realChange.Inputs["tasks.md"], "objective changes must stale the structural tasks.md input")

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("# Assurance\nchanged\n"), 0o644))
	changedAssurance, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
	require.NoError(t, err)
	assert.NotContains(t, changedAssurance.Inputs, "assurance.md", "editing assurance.md must not introduce a plan-audit digest input")
	assert.Equal(t, realChange.Inputs, changedAssurance.Inputs, "assurance.md changes must not affect the plan-audit digest")
}

func TestPlanAuditInputDigestIgnoresScaffoldOnlyProseEdits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		artifact string
		content  string
	}{
		{
			name:     "intent scaffold-only section",
			artifact: "intent.md",
			content:  "# Intent\nship digest freshness\n\n## Deferred Ideas\n<!-- Identified but postponed ideas -->\n",
		},
		{
			name:     "requirements scaffold-only section",
			artifact: "requirements.md",
			content:  "# Requirements\nREQ-001 digest freshness\n\n## Requirements\n<!-- Author each requirement here. -->\n",
		},
		{
			name:     "research scaffold-only section",
			artifact: "research.md",
			content:  "# Research\ncurrent facts\n\n## Assumptions\n<!-- Assumptions with the evidence that supports them. -->\n",
		},
		{
			name:     "decision scaffold-only section",
			artifact: "decision.md",
			content:  "# Decision\nuse digest inputs\n\n## Risk\n<!-- Concrete risks found by inspecting the affected code and contracts. -->\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			change := model.NewChange("plan-audit-prose-scaffold-" + strings.ReplaceAll(tt.name, " ", "-"))
			require.NoError(t, state.SaveChange(root, change))
			bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

			before, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
			require.NoError(t, err)

			require.NoError(t, os.WriteFile(filepath.Join(bundleDir, tt.artifact), []byte(tt.content), 0o644))
			after, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
			require.NoError(t, err)

			assert.Equal(t, before.Inputs[tt.artifact], after.Inputs[tt.artifact],
				"scaffold-only edits must not stale %s", tt.artifact)
		})
	}
}

func TestProseFileInputHashTreatsKnownDefaultsAsNonMaterial(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	intentPath := filepath.Join(root, "intent.md")

	require.NoError(t, os.WriteFile(intentPath, []byte(`# Intent

## Summary
Describe the change objective.
`), 0o644))
	defaultHash, err := computeProseFileInputHash(intentPath)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(intentPath, []byte(`# Intent

## Summary
<!-- Summary is still un-authored scaffold. -->
`), 0o644))
	commentOnlyHash, err := computeProseFileInputHash(intentPath)
	require.NoError(t, err)
	assert.Equal(t, defaultHash, commentOnlyHash,
		"the engine-owned intent summary default must hash like an empty scaffold section")

	require.NoError(t, os.WriteFile(intentPath, []byte(`# Intent

## Summary
Ship prose digest materiality.
`), 0o644))
	authoredHash, err := computeProseFileInputHash(intentPath)
	require.NoError(t, err)
	assert.NotEqual(t, defaultHash, authoredHash,
		"unknown non-empty authored prose must remain material")
}

func TestGovernedSkillInputDigestsCoverStageAuthorities(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())
	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})

	intake, err := certifiedSkillInputDigest(root, change, SkillIntakeClarification, nil)
	require.NoError(t, err)
	assert.Len(t, intake.Inputs, 1)
	assert.Contains(t, intake.Inputs, "intent.md")

	research, err := certifiedSkillInputDigest(root, change, SkillResearchOrchestration, nil)
	require.NoError(t, err)
	assert.Contains(t, research.Inputs, "intent.md")
	assert.Contains(t, research.Inputs, "research.md")

	planAudit, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
	require.NoError(t, err)
	for _, rel := range []string{
		"decision.md",
		"intent.md",
		"requirements.md",
		"research.md",
		"tasks.md",
	} {
		assert.Contains(t, planAudit.Inputs, rel)
	}
	assert.NotContains(t, planAudit.Inputs, "assurance.md", "S1 plan-audit does not own the S4 assurance contract")

	finalCloseout, err := certifiedSkillInputDigest(root, change, SkillFinalCloseout, summary)
	require.NoError(t, err)
	assert.Contains(t, finalCloseout.Inputs, "assurance.md")
	assert.Contains(t, finalCloseout.Inputs, "changed_target_files")
	assert.Contains(t, finalCloseout.Inputs, "tracked.go")
}

func TestLateAssuranceEditDoesNotStalePlanAuditAtS1(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("late-assurance-edit")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: verdictAt,
	}
	// Planning inputs settled before the verdict; the plan-audit digest is stamped.
	beforeVerdict := verdictAt.Add(-time.Minute)
	for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md", "assurance.md", "tasks.md"} {
		require.NoError(t, os.Chtimes(filepath.Join(bundleDir, rel), beforeVerdict, beforeVerdict))
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, rec)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillPlanAudit, rec, nil))

	// A late closeout-style edit to assurance.md, well after the plan-audit verdict.
	afterVerdict := verdictAt.Add(time.Hour)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("# Assurance\nrewritten during closeout\n"), 0o644))
	require.NoError(t, os.Chtimes(filepath.Join(bundleDir, "assurance.md"), afterVerdict, afterVerdict))

	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	require.Empty(t, blockers, "a late assurance.md edit must not stale plan-audit")
	assert.Contains(t, passing, SkillPlanAudit)

	// The stamped plan-audit digest must not carry an assurance.md input at all.
	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.NotContains(t, digests.Skills[SkillPlanAudit].Inputs, "assurance.md")
}

func TestEvaluateRequiredSkillsUsesContentDigestNotMTime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("required-skill-digest")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC),
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, rec)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillPlanAudit, rec, nil))

	olderThanEvidence := rec.Timestamp.Add(-time.Hour)
	require.NoError(t, os.Chtimes(filepath.Join(bundleDir, "requirements.md"), olderThanEvidence, olderThanEvidence))
	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	require.Empty(t, blockers)
	assert.Contains(t, passing, SkillPlanAudit)

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte("# Requirements\nREQ-001 real change\n"), 0o644))
	require.NoError(t, os.Chtimes(filepath.Join(bundleDir, "requirements.md"), olderThanEvidence, olderThanEvidence))
	passing, blockers, err = EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	assert.Empty(t, passing)
	assert.Contains(t, blockers, "required_skill_stale:plan-audit:requirements.md")
}

func TestEvaluateRequiredSkillsAcceptsRefreshedVerdictAfterDigestDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("required-skill-digest-refreshed")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	originalAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	original := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: originalAt,
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, original)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillPlanAudit, original, nil))

	refreshedAt := originalAt.Add(2 * time.Hour)
	assurancePath := filepath.Join(bundleDir, "assurance.md")
	require.NoError(t, os.WriteFile(assurancePath, []byte("# Assurance\nrefreshed after review\n"), 0o644))
	beforeRefreshed := refreshedAt.Add(-time.Minute)
	for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md", "assurance.md", "tasks.md"} {
		require.NoError(t, os.Chtimes(filepath.Join(bundleDir, rel), beforeRefreshed, beforeRefreshed))
	}

	refreshed := original
	refreshed.Timestamp = refreshedAt
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, refreshed)
	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	require.Empty(t, blockers)
	require.Contains(t, passing, SkillPlanAudit)

	result, err := stampPassingSkillDigests(root, change, passing)
	require.NoError(t, err)
	require.Empty(t, result.Blockers)
	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	stored := digests.Skills[SkillPlanAudit]
	assert.Equal(t, refreshedAt, stored.VerdictTimestamp)
}

func TestEvaluateRequiredSkillsIgnoresUnchangedTaskMTimeInRefreshedVerdictWindow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("refreshed-verdict-task-mtime")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	originalAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	original := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: originalAt,
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, original)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillPlanAudit, original, nil))

	refreshedAt := originalAt.Add(2 * time.Hour)
	assurancePath := filepath.Join(bundleDir, "assurance.md")
	require.NoError(t, os.WriteFile(assurancePath, []byte("# Assurance\nrefreshed after review\n"), 0o644))
	beforeRefreshed := refreshedAt.Add(-time.Minute)
	require.NoError(t, os.Chtimes(assurancePath, beforeRefreshed, beforeRefreshed))

	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(checkedDigestTasks()), 0o644))
	afterRefreshed := refreshedAt.Add(time.Minute)
	require.NoError(t, os.Chtimes(tasksPath, afterRefreshed, afterRefreshed))

	refreshed := original
	refreshed.Timestamp = refreshedAt
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, refreshed)
	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	require.Empty(t, blockers)
	assert.Contains(t, passing, SkillPlanAudit)
}

func TestMissingDigestEntryStampsCurrentPlanningInputsWithoutTimestampRefusal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("missing-digest-stamps-current")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: verdictAt,
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, rec)

	afterVerdict := verdictAt.Add(time.Minute)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte("# Decision\ncurrent plan text\n"), 0o644))
	for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md", "assurance.md", "tasks.md"} {
		require.NoError(t, os.Chtimes(filepath.Join(bundleDir, rel), afterVerdict, afterVerdict))
	}

	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	require.Empty(t, blockers)
	require.Contains(t, passing, SkillPlanAudit)

	result, err := stampPassingSkillDigests(root, change, passing)
	require.NoError(t, err)
	require.Empty(t, result.Blockers)
	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, digests.Skills[SkillPlanAudit].Inputs, "decision.md")
}

func TestMissingWaveDigestEntryStampsCurrentRuntimeTaskEvidenceWithoutTimestampRefusal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("missing-wave-digest-stamps-current")
	change.CurrentState = model.StateS4Verify
	require.NoError(t, state.SaveChange(root, change))
	writeTasksAndMaterializeWavePlan(t, root, change, uncheckedDigestTasks())
	plan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)

	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	capturedAt := verdictAt.Add(-time.Minute)
	writeWaveDigestTaskEvidence(t, root, change, plan.TasksPlanHash, "test:first", capturedAt)
	tasks, issues, err := LoadExecutionTasksFromEvidence(root, change.Slug, 1)
	require.NoError(t, err)
	require.Empty(t, issues)
	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		TasksPlanHash:     plan.TasksPlanHash,
		Tasks:             tasks,
	}
	summary.SyncDerivedFields()
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  verdictAt,
		RunVersion: 1,
	}
	beforeVerdict := verdictAt.Add(-time.Minute)
	require.NoError(t, os.Chtimes(state.WavePlanPathForRead(root, change.Slug), beforeVerdict, beforeVerdict))
	taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, change.Slug), "t-01.json")
	require.NoError(t, os.Chtimes(taskEvidencePath, beforeVerdict, beforeVerdict))

	blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillWaveOrchestration, summary)
	require.NoError(t, err)
	assert.Empty(t, blockers)

	afterVerdict := verdictAt.Add(time.Minute)
	require.NoError(t, os.Chtimes(taskEvidencePath, afterVerdict, afterVerdict))
	blockers, err = skillDigestFreshnessBlockersWithSummary(root, change, SkillWaveOrchestration, summary)
	require.NoError(t, err)
	assert.Empty(t, blockers)

	result, err := stampPassingSkillDigests(root, change, map[string]model.VerificationRecord{
		SkillWaveOrchestration: record,
	})
	require.NoError(t, err)
	require.Empty(t, result.Blockers)
	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, digests.Skills[SkillWaveOrchestration].Inputs, "runtime_task_evidence")
}

func TestStampPassingSkillDigestsDoesNotBlockCurrentStageOnFutureAcceptedEvidence(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	change.CurrentState = model.StateS2Execute
	require.NoError(t, state.SaveChange(root, change))
	writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())
	writeTasksAndMaterializeWavePlan(t, root, change, uncheckedDigestTasks())
	plan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)

	capturedAt := time.Date(2026, 6, 4, 3, 0, 0, 0, time.UTC)
	writeWaveDigestTaskEvidence(t, root, change, plan.TasksPlanHash, "test:wave", capturedAt)
	tasks, issues, err := LoadExecutionTasksFromEvidence(root, change.Slug, 1)
	require.NoError(t, err)
	require.Empty(t, issues)
	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		TasksPlanHash:     plan.TasksPlanHash,
		Tasks:             tasks,
	}
	summary.SyncDerivedFields()
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	waveRecord := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  capturedAt.Add(time.Minute),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, change.Slug, SkillWaveOrchestration, waveRecord)

	reviewRecord := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  capturedAt.Add(2 * time.Minute),
		RunVersion: 1,
		References: []string{"layer:R0=pass"},
	}
	writeVerificationForTest(t, root, change.Slug, SkillSpecComplianceReview, reviewRecord)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillSpecComplianceReview, reviewRecord, summary))
	_, err = state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "skill.evidence_recorded",
		SkillID:   SkillSpecComplianceReview,
		Result:    "recorded",
		Reason:    "verification_evidence_consumed",
	})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(root, "tracked.go"), []byte("package main\n\nconst changedForFutureReview = true\n"), 0o644))
	result, err := stampPassingSkillDigests(root, change, map[string]model.VerificationRecord{
		SkillWaveOrchestration: waveRecord,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Blockers, "future-stage review evidence must not block S2 wave digest stamping")
}

func TestStampPassingSkillDigestsStampsPreviouslyConsumedEvidenceWithoutLegacyEvent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("previously-consumed-digest-stamp")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: verdictAt,
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, rec)
	_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "skill.evidence_recorded",
		SkillID:   SkillPlanAudit,
		Result:    "recorded",
		Reason:    "verification_evidence_consumed",
	})
	require.NoError(t, err)

	result, err := stampPassingSkillDigests(root, change, map[string]model.VerificationRecord{
		SkillPlanAudit: rec,
	})
	require.NoError(t, err)
	require.Empty(t, result.Blockers)

	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, digests.Skills, SkillPlanAudit)

	firstStampRoot := t.TempDir()
	firstStampChange := model.NewChange("first-digest-stamp-event")
	firstStampChange.CurrentState = model.StateS1Plan
	firstStampChange.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(firstStampRoot, firstStampChange))
	writeDigestPlanningBundle(t, firstStampRoot, firstStampChange, uncheckedDigestTasks())
	writeVerificationForTest(t, firstStampRoot, firstStampChange.Slug, SkillPlanAudit, rec)
	firstStamp, err := stampPassingSkillDigests(firstStampRoot, firstStampChange, map[string]model.VerificationRecord{
		SkillPlanAudit: rec,
	})
	require.NoError(t, err)
	require.Empty(t, firstStamp.Blockers)
}

func TestStampPassingSkillDigestsStampsPreviouslyAcceptedResearchWithoutStoredDigest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("previously-accepted-research-missing-digest")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS4Verify
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	researchRecord := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: verdictAt,
	}
	writeVerificationForTest(t, root, change.Slug, SkillResearchOrchestration, researchRecord)
	_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "skill.evidence_recorded",
		SkillID:   SkillResearchOrchestration,
		Result:    "recorded",
		Reason:    "verification_evidence_consumed",
	})
	require.NoError(t, err)

	intentPath := filepath.Join(bundleDir, "intent.md")
	require.NoError(t, os.WriteFile(intentPath, []byte("# Intent\nchanged after research verdict\n"), 0o644))
	afterVerdict := verdictAt.Add(time.Minute)
	require.NoError(t, os.Chtimes(intentPath, afterVerdict, afterVerdict))

	result, err := stampPassingSkillDigests(root, change, map[string]model.VerificationRecord{})
	require.NoError(t, err)
	require.Empty(t, result.Blockers)

	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, digests.Skills, SkillResearchOrchestration)
}

func TestStampPassingSkillDigestsUsesStoredDigestForPreviouslyAcceptedPlanAuditCheckboxWriteback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("plan-audit-checkbox-writeback")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS2Execute
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	planVerdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	planRecord := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: planVerdictAt,
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, planRecord)
	beforeVerdict := planVerdictAt.Add(-time.Minute)
	for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md", "assurance.md", "tasks.md"} {
		require.NoError(t, os.Chtimes(filepath.Join(bundleDir, rel), beforeVerdict, beforeVerdict))
	}
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillPlanAudit, planRecord, nil))
	_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "skill.evidence_recorded",
		SkillID:   SkillPlanAudit,
		Result:    "recorded",
		Reason:    "verification_evidence_consumed",
	})
	require.NoError(t, err)

	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(checkedDigestTasks()), 0o644))
	afterVerdict := planVerdictAt.Add(time.Hour)
	require.NoError(t, os.Chtimes(tasksPath, afterVerdict, afterVerdict))

	researchRecord := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: afterVerdict.Add(time.Minute),
	}
	writeVerificationForTest(t, root, change.Slug, SkillResearchOrchestration, researchRecord)
	result, err := stampPassingSkillDigests(root, change, map[string]model.VerificationRecord{
		SkillResearchOrchestration: researchRecord,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Blockers)

	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, digests.Skills, SkillPlanAudit)
	assert.Contains(t, digests.Skills, SkillResearchOrchestration)
}

func TestStampPassingSkillDigestsBlocksDirectPassingSkillWhenDigestInputUnavailable(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("direct-digest-input-unavailable")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())
	require.NoError(t, os.Remove(filepath.Join(bundleDir, "tasks.md")))

	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC),
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, rec)

	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	assert.Empty(t, passing)
	assert.Contains(t, blockers, "required_skill_stale:plan-audit:input_digest_unavailable")

	result, err := stampPassingSkillDigests(root, change, map[string]model.VerificationRecord{
		SkillPlanAudit: rec,
	})
	require.NoError(t, err)
	assert.Contains(t, result.Blockers, "required_skill_stale:plan-audit:input_digest_unavailable")

	digests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	if digests != nil {
		assert.NotContains(t, digests.Skills, SkillPlanAudit)
	}
}

func TestStampPassingSkillDigestsSkipsPreviouslyAcceptedWaveWhenRecoveryClearedInputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("recovery-cleared-wave-inputs")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	planRecord := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	}
	waveRecord := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Add(-time.Minute),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, planRecord)
	writeVerificationForTest(t, root, change.Slug, SkillWaveOrchestration, waveRecord)
	_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "skill.evidence_recorded",
		SkillID:   SkillWaveOrchestration,
		Result:    "recorded",
		Reason:    "verification_evidence_consumed",
	})
	require.NoError(t, err)

	result, err := stampPassingSkillDigests(root, change, map[string]model.VerificationRecord{
		SkillPlanAudit: planRecord,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Blockers)

	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, digests.Skills, SkillPlanAudit)
	assert.NotContains(t, digests.Skills, SkillWaveOrchestration)
}

func TestStampPassingSkillDigestsDoesNotLetHistoricalIntakeBlockPlanAcceptance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("historical-intake-does-not-block-plan")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	planRecord := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	}
	intakeRecord := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC().Add(-time.Hour),
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, planRecord)
	writeVerificationForTest(t, root, change.Slug, SkillIntakeClarification, intakeRecord)
	_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "skill.evidence_recorded",
		SkillID:   SkillIntakeClarification,
		Result:    "recorded",
		Reason:    "verification_evidence_consumed",
	})
	require.NoError(t, err)

	result, err := stampPassingSkillDigests(root, change, map[string]model.VerificationRecord{
		SkillPlanAudit: planRecord,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Blockers)

	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, digests.Skills, SkillPlanAudit)
	assert.NotContains(t, digests.Skills, SkillIntakeClarification)
}

func TestFeatureActiveMissingDigestEntryBackfillsWhenInputsUnchanged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("feature-active-missing-digest-entry")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: verdictAt,
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, rec)
	// Inputs settle into the current digest on the next mutating acceptance.
	beforeVerdict := verdictAt.Add(-time.Minute)
	for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md", "tasks.md"} {
		require.NoError(t, os.Chtimes(filepath.Join(bundleDir, rel), beforeVerdict, beforeVerdict))
	}
	require.NoError(t, state.SaveEvidenceDigests(root, change.Slug, model.EvidenceDigests{
		Version: model.EvidenceDigestsVersion,
		Skills:  map[string]model.SkillDigest{},
	}))

	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	require.Empty(t, blockers)
	assert.Contains(t, passing, SkillPlanAudit, "first host verdict can be accepted and stamped by the next mutating run")

	_, err = state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "skill.evidence_recorded",
		SkillID:   SkillPlanAudit,
		Result:    "recorded",
		Reason:    "verification_evidence_consumed",
	})
	require.NoError(t, err)

	// New contract: a recorded orphan stays passing and is stamped by the next
	// mutating acceptance; no timestamp or input_digest_missing gate applies.
	passing, blockers, err = EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	require.Empty(t, blockers)
	assert.Contains(t, passing, SkillPlanAudit)

	// Mutating advancement stamps the missing digest entry.
	result, err := stampPassingSkillDigests(root, change, map[string]model.VerificationRecord{SkillPlanAudit: rec})
	require.NoError(t, err)
	require.Empty(t, result.Blockers)
	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, digests.Skills, SkillPlanAudit)
}

func TestStampPassingSkillDigestsBackfillsFeatureActiveMissingResearchDigestWhenUnchanged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("feature-active-research-digest")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	researchRecord := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC),
	}
	beforeVerdict := researchRecord.Timestamp.Add(-time.Minute)
	for _, rel := range []string{"intent.md", "research.md"} {
		require.NoError(t, os.Chtimes(filepath.Join(bundleDir, rel), beforeVerdict, beforeVerdict))
	}
	writeVerificationForTest(t, root, change.Slug, SkillResearchOrchestration, researchRecord)
	_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventType: "skill.evidence_recorded",
		SkillID:   SkillResearchOrchestration,
		Result:    "recorded",
		Reason:    "verification_evidence_consumed",
	})
	require.NoError(t, err)
	require.NoError(t, state.SaveEvidenceDigests(root, change.Slug, model.EvidenceDigests{
		Version: model.EvidenceDigestsVersion,
		Skills: map[string]model.SkillDigest{
			SkillPlanAudit: {Inputs: map[string]string{"intent.md": "existing"}},
		},
	}))

	passing, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepResearch)
	require.NoError(t, err)
	require.Empty(t, blockers)
	assert.Contains(t, passing, SkillResearchOrchestration)

	result, err := stampPassingSkillDigests(root, change, map[string]model.VerificationRecord{})
	require.NoError(t, err)
	require.Empty(t, result.Blockers)
	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, digests.Skills, SkillResearchOrchestration, "missing digest entry is stamped from the current accepted inputs")

	passing, blockers, err = EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepResearch)
	require.NoError(t, err)
	require.Empty(t, blockers)
	assert.Contains(t, passing, SkillResearchOrchestration)
}

func TestResearchOrchestrationInputDigestIncludesResearchArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("research-digest-artifact")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepResearch
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	record := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC),
	}
	writeVerificationForTest(t, root, change.Slug, SkillResearchOrchestration, record)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillResearchOrchestration, record, nil))

	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	require.Contains(t, digests.Skills[SkillResearchOrchestration].Inputs, "intent.md")
	require.Contains(t, digests.Skills[SkillResearchOrchestration].Inputs, "research.md")

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "research.md"), []byte("# Research\nchanged after research verdict\n"), 0o644))
	blockers, err := skillDigestFreshnessBlockers(root, change, SkillResearchOrchestration)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:research-orchestration:research.md")
}

func TestStoredGoalDigestStalesWhenInputContentChanges(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	change.CurrentState = model.StateS4Verify
	require.NoError(t, state.SaveChange(root, change))
	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  verdictAt,
		RunVersion: 1,
	}
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillGoalVerification, record, summary))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tracked.go"), []byte("package main\n\nconst goalDigestChanged = true\n"), 0o644))

	blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillGoalVerification, summary)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:goal-verification:tracked.go")
}

func TestGoalAndCloseoutDigestIgnoresEvidenceRefOnlyChangeYAMLMutation(t *testing.T) {
	t.Parallel()

	for _, skillName := range []string{SkillGoalVerification, SkillFinalCloseout} {
		t.Run(skillName, func(t *testing.T) {
			t.Parallel()

			root, change := createReviewInputDigestFixture(t)
			change.CurrentState = model.StateS4Verify
			require.NoError(t, state.SaveChange(root, change))
			changeYAMLRel := filepath.ToSlash(filepath.Join("artifacts", "changes", change.Slug, "change.yaml"))
			summary := digestPolicyExecutionSummary(change, []string{changeYAMLRel})
			require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))
			record := model.VerificationRecord{
				Verdict:    model.VerificationVerdictPass,
				Blockers:   []model.ReasonCode{},
				Timestamp:  time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC),
				RunVersion: 1,
			}
			require.NoError(t, StampEvidenceDigestForSkill(root, change, skillName, record, summary))

			change.RecordEvidenceRef(skillName, filepath.ToSlash(filepath.Join("artifacts", "changes", change.Slug, "verification", skillName+".yaml")))
			require.NoError(t, state.SaveChange(root, change))

			blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, skillName, summary)
			require.NoError(t, err)
			assert.NotContains(t, blockers, "required_skill_stale:"+skillName+":"+changeYAMLRel)
			assert.Empty(t, blockers)

			change.Description = "meaningful change.yaml mutation after evidence"
			require.NoError(t, state.SaveChange(root, change))

			blockers, err = skillDigestFreshnessBlockersWithSummary(root, change, skillName, summary)
			require.NoError(t, err)
			assert.Contains(t, blockers, "required_skill_stale:"+skillName+":"+changeYAMLRel)
		})
	}
}

func TestDefaultContentDigestKeepsRawChangeYAMLHash(t *testing.T) {
	t.Parallel()

	const skillName = "custom-runtime-review"
	root, change := createReviewInputDigestFixture(t)
	changeYAMLRel := filepath.ToSlash(filepath.Join("artifacts", "changes", change.Slug, "change.yaml"))
	summary := digestPolicyExecutionSummary(change, []string{changeYAMLRel})
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))
	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC),
		RunVersion: 1,
	}
	require.NoError(t, StampEvidenceDigestForSkill(root, change, skillName, record, summary))

	change.RecordEvidenceRef(skillName, filepath.ToSlash(filepath.Join("artifacts", "changes", change.Slug, "verification", skillName+".yaml")))
	require.NoError(t, state.SaveChange(root, change))

	blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, skillName, summary)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:"+skillName+":"+changeYAMLRel)
}

func TestStoredReviewDigestStalesWhenInputContentChanges(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	reviewablePath := filepath.Join(root, "reviewable.go")
	require.NoError(t, os.WriteFile(reviewablePath, []byte("package main\n"), 0o644))
	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  verdictAt,
		RunVersion: 1,
	}
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillIndependentReview, record, summary))
	require.NoError(t, os.WriteFile(reviewablePath, []byte("package main\n\nconst reviewDigestChanged = true\n"), 0o644))

	blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillIndependentReview, summary)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:independent-review:reviewable.go")
}

func TestStoredReviewDigestStalesWhenInputFileIsDeleted(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	deletedPath := filepath.Join(root, "tracked.go")
	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  verdictAt,
		RunVersion: 1,
	}
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillIndependentReview, record, summary))
	require.NoError(t, os.Remove(deletedPath))

	blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillIndependentReview, summary)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:independent-review:tracked.go")
}

func TestGatePlanningSkillRecordsPreservesStaleDigestArtifactName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("gate-plan-stale-digest")
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())

	rec := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC),
	}
	writeVerificationForTest(t, root, change.Slug, SkillPlanAudit, rec)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillPlanAudit, rec, nil))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte("# Decision\nchanged after plan audit\n"), 0o644))

	_, blockers, err := gatePlanningSkillRecords(root, change, model.PlanSubStepAudit)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:plan-audit:decision.md")
}

func TestExecutionSummaryFreshnessUsesTaskPlanHashNotTaskMTime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("execution-summary-digest")
	change.CurrentState = model.StateS4Verify
	require.NoError(t, state.SaveChange(root, change))
	tasksPath := writeTasksAndMaterializeWavePlan(t, root, change, uncheckedDigestTasks())
	plan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)

	capturedAt := time.Date(2026, 6, 4, 2, 0, 0, 0, time.UTC)
	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		TasksPlanHash:     plan.TasksPlanHash,
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:      "t-01",
			Verdict:     model.TaskVerdictPass,
			TaskKind:    model.TaskKindTest,
			CapturedAt:  capturedAt,
			EvidenceRef: "test:t-01",
		}},
	}
	state.ApplyExecutionSummaryFreshnessInputs(summary, change)
	summary.SyncDerivedFields()

	afterEvidence := capturedAt.Add(time.Hour)
	require.NoError(t, os.Chtimes(tasksPath, afterEvidence, afterEvidence))
	assert.Equal(t, ctxpack.EvidenceFreshnessFresh, state.ExecutionSummaryFreshness(root, change, summary))

	require.NoError(t, os.WriteFile(tasksPath, []byte(realChangedDigestTasks()), 0o644))
	assert.Equal(t, ctxpack.EvidenceFreshnessStale, state.ExecutionSummaryFreshness(root, change, summary))
}

func TestWaveOrchestrationInputDigestUsesRuntimeTaskEvidenceNotExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("wave-orchestration-digest")
	require.NoError(t, state.SaveChange(root, change))
	writeTasksAndMaterializeWavePlan(t, root, change, uncheckedDigestTasks())
	plan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)

	capturedAt := time.Date(2026, 6, 4, 3, 0, 0, 0, time.UTC)
	writeWaveDigestTaskEvidence(t, root, change, plan.TasksPlanHash, "test:first", capturedAt)
	tasks, issues, err := LoadExecutionTasksFromEvidence(root, change.Slug, 1)
	require.NoError(t, err)
	require.Empty(t, issues)
	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		TasksPlanHash:     plan.TasksPlanHash,
		Tasks:             tasks,
	}
	summary.SyncDerivedFields()
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	base, err := certifiedSkillInputDigest(root, change, SkillWaveOrchestration, summary)
	require.NoError(t, err)
	assert.Contains(t, base.Inputs, "wave-plan.yaml")
	assert.Contains(t, base.Inputs, "runtime_task_evidence")
	assert.NotContains(t, base.Inputs, "execution-summary.yaml")

	regeneratedSummary := *summary
	regeneratedSummary.CapturedAt = capturedAt.Add(time.Minute)
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, regeneratedSummary))
	unchanged, err := certifiedSkillInputDigest(root, change, SkillWaveOrchestration, &regeneratedSummary)
	require.NoError(t, err)
	fresh, changed := model.EvidenceFreshness(base, unchanged.Inputs)
	assert.True(t, fresh)
	assert.Empty(t, changed)

	writeWaveDigestTaskEvidence(t, root, change, plan.TasksPlanHash, "test:second", capturedAt.Add(2*time.Minute))
	updatedTasks, issues, err := LoadExecutionTasksFromEvidence(root, change.Slug, 1)
	require.NoError(t, err)
	require.Empty(t, issues)
	updatedSummary := *summary
	updatedSummary.Tasks = updatedTasks
	updated, err := certifiedSkillInputDigest(root, change, SkillWaveOrchestration, &updatedSummary)
	require.NoError(t, err)
	fresh, changed = model.EvidenceFreshness(base, updated.Inputs)
	require.False(t, fresh)
	assert.Contains(t, changed, "runtime_task_evidence")
}

func TestReviewInputDigestIncludesNonIgnoredUntrackedFiles(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})

	base, err := certifiedSkillInputDigest(root, change, SkillCodeQualityReview, summary)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "reviewable.go"), []byte("package main\n"), 0o644))
	current, err := certifiedSkillInputDigest(root, change, SkillCodeQualityReview, summary)
	require.NoError(t, err)

	fresh, changed := model.EvidenceFreshness(base, current.Inputs)
	require.False(t, fresh)
	assert.Contains(t, changed, "reviewable.go")
}

func TestReviewInputDigestUsesDeletedSentinelForDeletedDiffFiles(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})

	require.NoError(t, os.Remove(filepath.Join(root, "tracked.go")))
	current, err := certifiedSkillInputDigest(root, change, SkillSpecComplianceReview, summary)
	require.NoError(t, err)
	require.Contains(t, current.Inputs, "tracked.go")
	assert.True(t, deletedInputDigest(current.Inputs["tracked.go"], "tracked.go"))
}

func TestReviewInputDigestHashesDirectoryTargets(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	dir := filepath.Join(root, "generated")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "surface.txt"), []byte("v1\n"), 0o644))
	gitForReadinessOptimizationTests(t, root, "add", ".")
	gitForReadinessOptimizationTests(t, root, "commit", "-m", "add generated surface")
	summary := digestPolicyExecutionSummary(change, []string{"generated"})

	base, err := certifiedSkillInputDigest(root, change, SkillSpecComplianceReview, summary)
	require.NoError(t, err)
	require.Contains(t, base.Inputs, "generated")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "surface.txt"), []byte("v2\n"), 0o644))
	current, err := certifiedSkillInputDigest(root, change, SkillSpecComplianceReview, summary)
	require.NoError(t, err)
	fresh, changed := model.EvidenceFreshness(base, current.Inputs)
	require.False(t, fresh)
	assert.Contains(t, changed, "generated")
}

func writeWaveDigestTaskEvidence(
	t *testing.T,
	root string,
	change model.Change,
	tasksPlanHash string,
	evidenceRef string,
	capturedAt time.Time,
) {
	t.Helper()

	dir := state.EvidenceTasksDir(root, change.Slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	payload := TaskEvidencePayload{
		TaskID:            "t-01",
		RunSummaryVersion: 1,
		TaskKind:          model.TaskKindTest,
		Verdict:           model.TaskVerdictPass,
		ChangedFiles:      []string{"internal/model/evidence_digests_test.go"},
		TargetFiles:       []string{"internal/model/evidence_digests_test.go"},
		EvidenceRef:       evidenceRef,
		CapturedAt:        capturedAt.UTC().Format(time.RFC3339Nano),
		FreshnessInputs:   state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01", tasksPlanHash),
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "t-01.json"), append(raw, '\n'), 0o644))
}

func TestDiffClassReviewInputDigestIncludesSecurityAndIndependentReview(t *testing.T) {
	t.Parallel()

	for _, skillName := range []string{SkillSecurityReview, SkillIndependentReview} {
		t.Run(skillName, func(t *testing.T) {
			t.Parallel()
			root, change := createReviewInputDigestFixture(t)
			summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})

			base, err := certifiedSkillInputDigest(root, change, skillName, summary)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "reviewable.go"), []byte("package main\n"), 0o644))
			current, err := certifiedSkillInputDigest(root, change, skillName, summary)
			require.NoError(t, err)

			fresh, changed := model.EvidenceFreshness(base, current.Inputs)
			require.False(t, fresh)
			assert.Contains(t, changed, "reviewable.go")
		})
	}
}

func TestReviewInputDigestExcludesIgnoredAndRuntimeEvidence(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})

	base, err := certifiedSkillInputDigest(root, change, SkillSpecComplianceReview, summary)
	require.NoError(t, err)
	assert.NotContains(t, base.Inputs, "execution-summary.yaml")
	require.NoError(t, os.WriteFile(filepath.Join(root, "ignored.tmp"), []byte("ignored\n"), 0o644))
	runtimeDir := filepath.Join(state.ChangeDir(root, change.Slug), "evidence", "tasks")
	require.NoError(t, os.MkdirAll(runtimeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(runtimeDir, "t-01.json"), []byte("{}\n"), 0o644))
	regeneratedSummary := *summary
	regeneratedSummary.CapturedAt = regeneratedSummary.CapturedAt.Add(time.Hour)
	current, err := certifiedSkillInputDigest(root, change, SkillSpecComplianceReview, &regeneratedSummary)
	require.NoError(t, err)
	assert.NotContains(t, current.Inputs, "execution-summary.yaml")

	fresh, changed := model.EvidenceFreshness(base, current.Inputs)
	assert.True(t, fresh)
	assert.Empty(t, changed)
}

func TestReviewInputDigestExcludesGovernedChangeBundles(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	bundleRel := "artifacts/changes/" + change.Slug + "/assurance.md"
	summary := digestPolicyExecutionSummary(change, []string{bundleRel, "tracked.go"})

	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())
	base, err := certifiedSkillInputDigest(root, change, SkillSpecComplianceReview, summary)
	require.NoError(t, err)
	assert.NotContains(t, base.Inputs, bundleRel)

	change.CurrentState = model.StateS4Verify
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("# Assurance\nchanged after review\n"), 0o644))
	archivedPath := filepath.Join(root, "artifacts", "changes", "archived", "other-change", "change.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(archivedPath), 0o755))
	require.NoError(t, os.WriteFile(archivedPath, []byte("slug: other-change\n"), 0o644))

	current, err := certifiedSkillInputDigest(root, change, SkillSpecComplianceReview, summary)
	require.NoError(t, err)

	fresh, changed := model.EvidenceFreshness(base, current.Inputs)
	assert.True(t, fresh)
	assert.NotContains(t, current.Inputs, bundleRel)
	assert.NotContains(t, changed, "artifacts/changes/"+change.Slug+"/change.yaml")
	assert.NotContains(t, current.Inputs, "artifacts/changes/"+change.Slug+"/assurance.md")
	assert.NotContains(t, current.Inputs, "artifacts/changes/archived/other-change/change.yaml")
}

func TestGoalVerificationInputDigestIgnoresUnrelatedUntrackedUnlessSummarized(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	baseSummary := digestPolicyExecutionSummary(change, nil)
	base, err := certifiedSkillInputDigest(root, change, SkillGoalVerification, baseSummary)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(root, "scratch.go"), []byte("package main\n"), 0o644))
	current, err := certifiedSkillInputDigest(root, change, SkillGoalVerification, baseSummary)
	require.NoError(t, err)
	fresh, changed := model.EvidenceFreshness(base, current.Inputs)
	require.True(t, fresh)
	assert.Empty(t, changed)

	targetedSummary := digestPolicyExecutionSummary(change, []string{"scratch.go"})
	current, err = certifiedSkillInputDigest(root, change, SkillGoalVerification, targetedSummary)
	require.NoError(t, err)
	fresh, changed = model.EvidenceFreshness(base, current.Inputs)
	require.False(t, fresh)
	assert.Contains(t, changed, "scratch.go")
}

func TestGoalAndFinalCloseoutInputDigestExcludesExecutionSummaryMetadata(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())
	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})
	summary.TasksPlanHash = "semantic-task-plan-v1"
	state.ApplyExecutionSummaryFreshnessInputs(summary, change)
	summary.SyncDerivedFields()

	for _, skillName := range []string{SkillGoalVerification, SkillFinalCloseout} {
		t.Run(skillName, func(t *testing.T) {
			base, err := certifiedSkillInputDigest(root, change, skillName, summary)
			require.NoError(t, err)
			require.Contains(t, base.Inputs, "changed_target_files")
			require.Contains(t, base.Inputs, "tracked.go")
			assert.NotContains(t, base.Inputs, "execution-summary.yaml")
			assert.NotContains(t, base.Inputs, "run_summary_version")
			assert.NotContains(t, base.Inputs, "tasks_plan_hash")
			if skillName == SkillFinalCloseout {
				assert.Contains(t, base.Inputs, "assurance.md")
			}

			regeneratedSummary := *summary
			regeneratedSummary.CapturedAt = summary.CapturedAt.Add(time.Hour)
			current, err := certifiedSkillInputDigest(root, change, skillName, &regeneratedSummary)
			require.NoError(t, err)

			fresh, changed := model.EvidenceFreshness(base, current.Inputs)
			assert.True(t, fresh)
			assert.Empty(t, changed)
		})
	}
}

func TestGoalAndCloseoutInputDigestNamesDeletedSummarizedFiles(t *testing.T) {
	t.Parallel()

	for _, skillName := range []string{SkillGoalVerification, SkillFinalCloseout} {
		t.Run(skillName, func(t *testing.T) {
			t.Parallel()

			root, change := createReviewInputDigestFixture(t)
			summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})
			record := model.VerificationRecord{
				Verdict:    model.VerificationVerdictPass,
				Blockers:   []model.ReasonCode{},
				Timestamp:  time.Date(2026, 6, 4, 5, 0, 0, 0, time.UTC),
				RunVersion: 1,
			}
			require.NoError(t, StampEvidenceDigestForSkill(root, change, skillName, record, summary))

			require.NoError(t, os.Remove(filepath.Join(root, "tracked.go")))
			blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, skillName, summary)
			require.NoError(t, err)
			assert.Contains(t, blockers, "required_skill_stale:"+skillName+":tracked.go")
			assert.NotContains(t, blockers, "required_skill_stale:"+skillName+":input_digest_unavailable")

			current, err := certifiedSkillInputDigest(root, change, skillName, summary)
			require.NoError(t, err)
			assert.Contains(t, current.Inputs, "tracked.go")
			assert.True(t, deletedInputDigest(current.Inputs["tracked.go"], "tracked.go"))
		})
	}
}

func TestGoalVerificationInputDigestUsesDeletedSentinelForUnmatchedGlob(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	summary := digestPolicyExecutionSummary(change, []string{"obsolete/*.go"})

	current, err := certifiedSkillInputDigest(root, change, SkillGoalVerification, summary)
	require.NoError(t, err)
	require.Contains(t, current.Inputs, "obsolete/*.go")
	assert.True(t, deletedInputDigest(current.Inputs["obsolete/*.go"], "obsolete/*.go"))
}

func TestFinalCloseoutInputDigestIncludesAssuranceEvenWhenNotSummarized(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())
	baseSummary := digestPolicyExecutionSummary(change, nil)

	goalBase, err := certifiedSkillInputDigest(root, change, SkillGoalVerification, baseSummary)
	require.NoError(t, err)
	assert.NotContains(t, goalBase.Inputs, "assurance.md")

	closeoutBase, err := certifiedSkillInputDigest(root, change, SkillFinalCloseout, baseSummary)
	require.NoError(t, err)
	require.Contains(t, closeoutBase.Inputs, "assurance.md")

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("# Assurance\nchanged after closeout\n"), 0o644))

	goalCurrent, err := certifiedSkillInputDigest(root, change, SkillGoalVerification, baseSummary)
	require.NoError(t, err)
	fresh, changed := model.EvidenceFreshness(goalBase, goalCurrent.Inputs)
	assert.True(t, fresh)
	assert.Empty(t, changed)

	closeoutCurrent, err := certifiedSkillInputDigest(root, change, SkillFinalCloseout, baseSummary)
	require.NoError(t, err)
	fresh, changed = model.EvidenceFreshness(closeoutBase, closeoutCurrent.Inputs)
	require.False(t, fresh)
	assert.Contains(t, changed, "assurance.md")
}

func TestS4ShipGateReopensWhenStoredReviewDigestIsStale(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))
	change.CurrentState = model.StateS4Verify
	change.NeedsDiscovery = false
	change.WorkflowPreset = model.WorkflowPresetStandard
	require.NoError(t, state.SaveChange(root, change))
	bundleDir := writeDigestPlanningBundle(t, root, change, uncheckedDigestTasks())
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("# Intent\nINT-001: ship digest replacement.\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements
### Requirement: Review digest recovery
REQ-001: S4 ship gate approval reopens stale review digests. Traces to INT-001.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [x] `+"`t-01`"+` update digest input
  - depends_on: []
  - target_files: ["tracked.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(digestCloseoutAssurance()), 0o644))
	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})
	summary.Tasks[0].ChangedFiles = []string{"tracked.go"}
	summary.SyncDerivedFields()
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	originalAt := time.Date(2026, 6, 4, 6, 0, 0, 0, time.UTC)
	specReviewRecord := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  originalAt,
		RunVersion: 1,
		References: []string{"layer:R0=pass"},
	}
	codeReviewRecord := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  originalAt,
		RunVersion: 1,
		References: []string{"layer:IR1=pass"},
	}
	writeVerificationForTest(t, root, change.Slug, SkillSpecComplianceReview, specReviewRecord)
	writeVerificationForTest(t, root, change.Slug, SkillCodeQualityReview, codeReviewRecord)
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillSpecComplianceReview, specReviewRecord, summary))
	require.NoError(t, StampEvidenceDigestForSkill(root, change, SkillCodeQualityReview, codeReviewRecord, summary))

	require.NoError(t, os.WriteFile(filepath.Join(root, "tracked.go"), []byte("package main\n\nconst refreshedDigestInput = true\n"), 0o644))
	refreshedAt := originalAt.Add(time.Hour)
	specReviewRecord.Timestamp = refreshedAt
	codeReviewRecord.Timestamp = refreshedAt
	writeVerificationForTest(t, root, change.Slug, SkillSpecComplianceReview, specReviewRecord)
	writeVerificationForTest(t, root, change.Slug, SkillCodeQualityReview, codeReviewRecord)
	goalAt := refreshedAt.Add(time.Minute)
	writeVerificationForTest(t, root, change.Slug, SkillGoalVerification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  goalAt,
		RunVersion: 1,
		References: []string{"fresh:run_version=1"},
	})
	writeVerificationForTest(t, root, change.Slug, SkillFinalCloseout, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  goalAt.Add(time.Minute),
		RunVersion: 1,
		References: []string{
			"closeout:assurance_complete=pass",
			"closeout:goal_verification_reuse=pass",
			"closeout:goal_verification_reuse_run_version=1",
		},
	})

	advanced, err := AdvanceGoverned(root, change.Slug)
	require.NoError(t, err)
	require.Equal(t, "advanced", advanced.Action, "advanced=%+v", advanced)
	assert.Equal(t, model.StateS4Verify, advanced.FromState)
	assert.Equal(t, model.StateS3Review, advanced.ToState)
	assert.Equal(t, "stale_evidence_recovery_started", advanced.Reason)
	assert.True(t, advanced.RecoveryOnly)

	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	for _, skillName := range []string{SkillSpecComplianceReview, SkillCodeQualityReview} {
		assert.NotContains(t, digests.Skills, skillName, "%s digest should be pruned with stale review evidence", skillName)
	}
	assert.NotContains(t, digests.Skills, SkillGoalVerification)
	assert.NotContains(t, digests.Skills, SkillFinalCloseout)
}

func createReviewInputDigestFixture(t *testing.T) (string, model.Change) {
	t.Helper()

	root := t.TempDir()
	initGitWorkspaceForReadinessOptimizationTests(t, root)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.tmp\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tracked.go"), []byte("package main\n"), 0o644))
	gitForReadinessOptimizationTests(t, root, "add", ".")
	gitForReadinessOptimizationTests(t, root, "commit", "-m", "initial")

	change := model.NewChange("digest-policy")
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))
	gitForReadinessOptimizationTests(t, root, "add", ".")
	gitForReadinessOptimizationTests(t, root, "commit", "-m", "change")
	return root, change
}

func digestPolicyExecutionSummary(change model.Change, targetFiles []string) *model.ExecutionSummary {
	capturedAt := time.Date(2026, 6, 4, 4, 0, 0, 0, time.UTC)
	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:      "t-01",
			Verdict:     model.TaskVerdictPass,
			TaskKind:    model.TaskKindCode,
			TargetFiles: append([]string(nil), targetFiles...),
			CapturedAt:  capturedAt,
			EvidenceRef: "test:t-01",
		}},
	}
	state.ApplyExecutionSummaryFreshnessInputs(summary, change)
	summary.SyncDerivedFields()
	return summary
}

func writeDigestPlanningBundle(t *testing.T, root string, change model.Change, tasks string) string {
	t.Helper()

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	for name, content := range map[string]string{
		"intent.md":       "# Intent\nship digest freshness\n",
		"requirements.md": "# Requirements\nREQ-001 digest freshness\n",
		"research.md":     "# Research\ncurrent facts\n",
		"decision.md":     "# Decision\nuse digest inputs\n",
		"assurance.md":    "# Assurance\ncovered\n",
		"tasks.md":        tasks,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, name), []byte(content), 0o644))
	}
	return bundleDir
}

func uncheckedDigestTasks() string {
	return `# Tasks

- [ ] ` + "`t-01`" + ` prove digest freshness
  - target_files: ["internal/engine/progression/evidence_digests_test.go"]
  - task_kind: test
  - acceptance: digest contract is covered
`
}

func checkedDigestTasks() string {
	return `# Tasks

- [x] ` + "`t-01`" + ` prove digest freshness
  - target_files: ["internal/engine/progression/evidence_digests_test.go"]
  - task_kind: test
  - acceptance: digest contract is covered
`
}

func scopeOnlyDigestTasks() string {
	return `# Tasks

- [ ] ` + "`t-01`" + ` prove digest freshness
  - target_files: ["internal/engine/progression/stale_evidence_recovery.go"]
  - task_kind: test
  - acceptance: digest contract is covered
`
}

func realChangedDigestTasks() string {
	return `# Tasks

- [x] ` + "`t-01`" + ` prove changed digest freshness
  - target_files: ["internal/engine/progression/evidence_digests_test.go"]
  - task_kind: test
  - acceptance: digest contract is covered
`
}

func digestCloseoutAssurance() string {
	return `## Scope Summary
Digest closeout fixture.

## Verification Verdict
Verification records are fixture-authored.

## Evidence Index
Review, goal, and closeout records exist for run_version 1.

## Requirement Coverage
Digest replacement at S4 is covered.

## Residual Risks and Exceptions
No residual fixture risk.

## Rollback Readiness
Rollback is source-only in this fixture.

## Archive Decision
The fixture is ready to reach done-ready.
`
}

// deletedInputDigest reports whether digest matches the deleted-file input hash
// for rel. Test-only assertion helper for evidence-digest behavior.
func deletedInputDigest(digest, rel string) bool {
	if strings.TrimSpace(digest) == "" {
		return false
	}
	expected, err := deletedFileInputHash(rel)
	return err == nil && digest == expected
}

// TestWorkspacePathInputHashStableAcrossLineEndingRoundTrip is a Windows
// regression guard (REQ-002 scenario B / REQ-009). A workspace file is digested
// for goal-verification / review evidence via workspacePathInputHash (which
// hashes file content through model.ComputeFileContentHash). When the SAME
// logical content is digested again after conversion from LF to CRLF — the
// Windows `git core.autocrlf=true` checkout case — the recomputed evidence
// digest must EQUAL the recorded digest. CRLF is simulated OS-independently by
// writing bytes containing \r\n directly. If CRLF normalization were removed,
// the LF and CRLF digests would differ and this test would go RED.
func TestWorkspacePathInputHashStableAcrossLineEndingRoundTrip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "tracked.go")
	const rel = "tracked.go"

	// Digest the file while stored as LF (the committed form).
	require.NoError(t, os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644))
	lfDigest, err := workspacePathInputHash(path, rel)
	require.NoError(t, err)
	require.NotEmpty(t, lfDigest)

	// Re-materialize the SAME logical content with CRLF line endings (Windows
	// autocrlf checkout) and digest again. The raw bytes differ from the LF form.
	require.NoError(t, os.WriteFile(path, []byte("package main\r\n\r\nfunc main() {}\r\n"), 0o644))
	crlfBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(crlfBytes), "\r\n", "test must drive the real CRLF on-disk case")

	crlfDigest, err := workspacePathInputHash(path, rel)
	require.NoError(t, err)

	assert.Equal(t, lfDigest, crlfDigest,
		"evidence digest must be stable across an LF<->CRLF line-ending round-trip")
}
