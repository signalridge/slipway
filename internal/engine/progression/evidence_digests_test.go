package progression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	ctxpack "github.com/signalridge/slipway/internal/engine/context"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanAuditInputDigestExcludesAssuranceAndIncludesSemanticTasks(t *testing.T) {
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

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("# Assurance\nchanged\n"), 0o644))
	changedAssurance, err := certifiedSkillInputDigest(root, change, SkillPlanAudit, nil)
	require.NoError(t, err)
	assert.NotContains(t, changedAssurance.Inputs, "assurance.md", "editing assurance.md must not introduce a plan-audit digest input")
	assert.Equal(t, first.Inputs, changedAssurance.Inputs, "assurance.md changes must not affect the plan-audit digest")
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
	require.NoError(t, stampEvidenceDigestForSkill(root, change, SkillPlanAudit, rec, nil))

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
	require.NoError(t, stampEvidenceDigestForSkill(root, change, SkillPlanAudit, rec, nil))

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
	require.NoError(t, stampEvidenceDigestForSkill(root, change, SkillPlanAudit, original, nil))

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
	assert.Empty(t, result.BackfilledSkills)
	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	stored := digests.Skills[SkillPlanAudit]
	assert.Equal(t, refreshedAt, stored.VerdictTimestamp)
	assert.Equal(t, refreshed.RunVersion, stored.RunVersion)
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
	require.NoError(t, stampEvidenceDigestForSkill(root, change, SkillPlanAudit, original, nil))

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

func TestDigestBackfillRefusesArtifactsChangedAfterVerdict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("legacy-digest-backfill")
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

	beforeVerdict := verdictAt.Add(-time.Minute)
	for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md", "assurance.md", "tasks.md"} {
		require.NoError(t, os.Chtimes(filepath.Join(bundleDir, rel), beforeVerdict, beforeVerdict))
	}
	_, blockers, err := EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	assert.NotContains(t, blockers, "required_skill_stale:plan-audit:legacy_artifact_changed_after_verdict")

	driftRoot := t.TempDir()
	driftChange := model.NewChange("legacy-digest-backfill-drift")
	driftChange.CurrentState = model.StateS1Plan
	driftChange.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(driftRoot, driftChange))
	driftBundleDir := writeDigestPlanningBundle(t, driftRoot, driftChange, uncheckedDigestTasks())
	writeVerificationForTest(t, driftRoot, driftChange.Slug, SkillPlanAudit, rec)

	afterVerdict := verdictAt.Add(time.Minute)
	require.NoError(t, os.Chtimes(filepath.Join(driftBundleDir, "decision.md"), afterVerdict, afterVerdict))
	_, blockers, err = EvaluateRequiredSkillsForChange(driftRoot, driftChange, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:plan-audit:decision.md")
}

func TestLegacyWaveDigestBackfillRefusesRuntimeTaskEvidenceChangedAfterVerdict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("legacy-wave-runtime-drift")
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

	blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillWaveOrchestration, record, summary)
	require.NoError(t, err)
	assert.Empty(t, blockers)

	afterVerdict := verdictAt.Add(time.Minute)
	require.NoError(t, os.Chtimes(taskEvidencePath, afterVerdict, afterVerdict))
	blockers, err = skillDigestFreshnessBlockersWithSummary(root, change, SkillWaveOrchestration, record, summary)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:wave-orchestration:runtime_task_evidence")
}

func TestDigestBackfillRecordsLifecycleEventOnlyForPreviouslyConsumedEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("legacy-digest-backfill-event")
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
	beforeVerdict := verdictAt.Add(-time.Minute)
	for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md", "assurance.md", "tasks.md"} {
		require.NoError(t, os.Chtimes(filepath.Join(bundleDir, rel), beforeVerdict, beforeVerdict))
	}
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
	assert.Equal(t, []string{SkillPlanAudit}, result.BackfilledSkills)

	before := change
	after := change
	after.CurrentState = model.StateS2Execute
	require.NoError(t, recordAdvanceLifecycleEvent(root, before, after, AdvanceSummary{
		Action:      "advanced",
		FromState:   before.CurrentState,
		ToState:     after.CurrentState,
		SideEffects: digestBackfilledSideEffects(result.BackfilledSkills),
	}, "run"))
	events, err := state.ReadLifecycleEvents(root, change)
	require.NoError(t, err)
	var found bool
	for _, event := range events {
		if event.EventType == digestBackfilledFromLegacyVerdictEvent && event.SkillID == SkillPlanAudit {
			found = true
			assert.Equal(t, "recorded", event.Result)
			assert.Equal(t, "legacy_verdict_digest_backfilled", event.Reason)
			assert.Contains(t, event.EvidenceRefs, "evidence-digests")
		}
	}
	assert.True(t, found, "expected digest backfill lifecycle event")

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
	assert.Empty(t, firstStamp.BackfilledSkills, "first acceptance stamp must not masquerade as legacy backfill")
}

func TestStampPassingSkillDigestsRefusesPreviouslyAcceptedResearchAfterInputDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("legacy-research-digest-drift")
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
	assert.Contains(t, result.Blockers, "required_skill_stale:research-orchestration:intent.md")

	digests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	if digests != nil {
		assert.NotContains(t, digests.Skills, SkillResearchOrchestration)
	}
}

func TestStampPassingSkillDigestsUsesStoredDigestForPreviouslyAcceptedPlanAuditCheckboxWriteback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("restamp-plan-audit-checkbox")
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
	require.NoError(t, stampEvidenceDigestForSkill(root, change, SkillPlanAudit, planRecord, nil))
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
	// Inputs settled before the verdict, so the orphan is Tier-0 healable.
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

	// New contract: a recorded orphan with inputs unchanged after the verdict is
	// Tier-0 healable, not a deadlock — it stays passing with no input_digest_missing.
	passing, blockers, err = EvaluateRequiredSkillsForChange(root, change, model.StateS1Plan, 0, false, model.PlanSubStepAudit)
	require.NoError(t, err)
	require.Empty(t, blockers)
	assert.Contains(t, passing, SkillPlanAudit)

	// Mutating advancement backfills the missing digest entry.
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
	assert.Empty(t, result.BackfilledSkills, "non-legacy backfill does not emit a legacy file-absent backfill event")
	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	assert.Contains(t, digests.Skills, SkillResearchOrchestration, "Tier-0 backfill stamps the previously-accepted orphan when inputs are unchanged after the verdict")

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
	require.NoError(t, stampEvidenceDigestForSkill(root, change, SkillResearchOrchestration, record, nil))

	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	require.Contains(t, digests.Skills[SkillResearchOrchestration].Inputs, "intent.md")
	require.Contains(t, digests.Skills[SkillResearchOrchestration].Inputs, "research.md")

	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "research.md"), []byte("# Research\nchanged after research verdict\n"), 0o644))
	blockers, err := skillDigestFreshnessBlockers(root, change, SkillResearchOrchestration, record)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:research-orchestration:research.md")
}

func TestLegacyDigestBackfillRefusesGoalInputChangedAfterVerdict(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	change.CurrentState = model.StateS4Verify
	require.NoError(t, state.SaveChange(root, change))
	summary := digestPolicyExecutionSummary(change, []string{"tracked.go"})
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	beforeVerdict := verdictAt.Add(-time.Minute)
	afterVerdict := verdictAt.Add(time.Minute)
	require.NoError(t, os.Chtimes(state.ExecutionSummaryPathForRead(root, change.Slug), beforeVerdict, beforeVerdict))
	require.NoError(t, os.Chtimes(filepath.Join(root, "tracked.go"), afterVerdict, afterVerdict))

	blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillGoalVerification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  verdictAt,
		RunVersion: 1,
	}, summary)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:goal-verification:tracked.go")
}

func TestLegacyDigestBackfillRefusesReviewInputChangedAfterVerdict(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	summary := digestPolicyExecutionSummary(change, nil)
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	reviewablePath := filepath.Join(root, "reviewable.go")
	require.NoError(t, os.WriteFile(reviewablePath, []byte("package main\n"), 0o644))
	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	beforeVerdict := verdictAt.Add(-time.Minute)
	afterVerdict := verdictAt.Add(time.Minute)
	require.NoError(t, os.Chtimes(state.ExecutionSummaryPathForRead(root, change.Slug), beforeVerdict, beforeVerdict))
	require.NoError(t, os.Chtimes(reviewablePath, afterVerdict, afterVerdict))

	blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillIndependentReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  verdictAt,
		RunVersion: 1,
	}, summary)
	require.NoError(t, err)
	assert.Contains(t, blockers, "required_skill_stale:independent-review:reviewable.go")
}

func TestLegacyDigestBackfillRefusesReviewDeletedFileAfterVerdict(t *testing.T) {
	t.Parallel()

	root, change := createReviewInputDigestFixture(t)
	summary := digestPolicyExecutionSummary(change, nil)
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, *summary))

	deletedPath := filepath.Join(root, "tracked.go")
	verdictAt := time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)
	beforeVerdict := verdictAt.Add(-time.Minute)
	require.NoError(t, os.Chtimes(state.ExecutionSummaryPathForRead(root, change.Slug), beforeVerdict, beforeVerdict))
	require.NoError(t, os.Chtimes(deletedPath, beforeVerdict, beforeVerdict))
	require.NoError(t, os.Remove(deletedPath))

	blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, SkillIndependentReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  verdictAt,
		RunVersion: 1,
	}, summary)
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
	require.NoError(t, stampEvidenceDigestForSkill(root, change, SkillPlanAudit, rec, nil))
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
			require.NoError(t, stampEvidenceDigestForSkill(root, change, skillName, record, summary))

			require.NoError(t, os.Remove(filepath.Join(root, "tracked.go")))
			blockers, err := skillDigestFreshnessBlockersWithSummary(root, change, skillName, record, summary)
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

func TestS4ShipGateApprovalRestampsReviewDigests(t *testing.T) {
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
### Requirement: Digest replacement
REQ-001: S4 ship gate approval restamps refreshed review digests. Traces to INT-001.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks
- [x] `+"`t-01`"+` update digest input
  - wave: 1
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
	require.NoError(t, stampEvidenceDigestForSkill(root, change, SkillSpecComplianceReview, specReviewRecord, summary))
	require.NoError(t, stampEvidenceDigestForSkill(root, change, SkillCodeQualityReview, codeReviewRecord, summary))

	require.NoError(t, os.WriteFile(filepath.Join(root, "tracked.go"), []byte("package main\n\nconst refreshedDigestInput = true\n"), 0o644))
	refreshedAt := originalAt.Add(time.Hour)
	specReviewRecord.Timestamp = refreshedAt
	codeReviewRecord.Timestamp = refreshedAt
	writeVerificationForTest(t, root, change.Slug, SkillSpecComplianceReview, specReviewRecord)
	writeVerificationForTest(t, root, change.Slug, SkillCodeQualityReview, codeReviewRecord)
	beforeRefreshed := refreshedAt.Add(-time.Minute)
	for _, skillName := range []string{SkillSpecComplianceReview, SkillCodeQualityReview} {
		inputPaths, err := digestInputArtifactPaths(root, change, skillName)
		require.NoError(t, err)
		for _, paths := range inputPaths {
			for _, path := range paths {
				require.NoError(t, os.Chtimes(path, beforeRefreshed, beforeRefreshed))
			}
		}
	}

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
	require.Equal(t, "done_ready", advanced.Action, "advanced=%+v", advanced)

	digests, err := state.LoadEvidenceDigestsForChange(root, change)
	require.NoError(t, err)
	for _, skillName := range []string{SkillSpecComplianceReview, SkillCodeQualityReview} {
		stored := digests.Skills[skillName]
		assert.Equal(t, refreshedAt, stored.VerdictTimestamp, "%s digest should be replaced at S4 acceptance", skillName)
		assert.Contains(t, stored.Inputs, "tracked.go")
		assert.NotContains(t, stored.Inputs, "artifacts/changes/"+change.Slug+"/change.yaml")
	}
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
  - wave: 1
  - target_files: ["internal/engine/progression/evidence_digests_test.go"]
  - task_kind: test
  - acceptance: digest contract is covered
`
}

func checkedDigestTasks() string {
	return `# Tasks

- [x] ` + "`t-01`" + ` prove digest freshness
  - wave: 1
  - target_files: ["internal/engine/progression/evidence_digests_test.go"]
  - task_kind: test
  - acceptance: digest contract is covered
`
}

func realChangedDigestTasks() string {
	return `# Tasks

- [x] ` + "`t-01`" + ` prove changed digest freshness
  - wave: 1
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
