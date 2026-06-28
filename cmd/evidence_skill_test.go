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

func TestEvidenceSkillRecordsPlanAuditVerification(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill records plan audit")
		change := setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)

		notesRel := filepath.ToSlash(filepath.Join("artifacts", "changes", slug, "verification", "plan-audit-notes.md"))
		notesPath := filepath.Join(root, filepath.FromSlash(notesRel))
		require.NoError(t, os.MkdirAll(filepath.Dir(notesPath), 0o755))
		require.NoError(t, os.WriteFile(notesPath, []byte("Plan audit passed.\n"), 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillPlanAudit,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "plan-audit:pass",
			"--notes-file", notesRel,
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		expectedPath := state.DisplayPath(
			root,
			filepath.Join(root, "artifacts", "changes", slug, "verification", "plan-audit.yaml"),
		)
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, progression.SkillPlanAudit, view.SkillName)
		assert.Equal(t, model.VerificationVerdictPass, view.Verdict)
		assert.Equal(t, 0, view.RunVersion)
		require.NotNil(t, view.InvocationRoute)
		assert.Equal(t, "unbound_active", view.InvocationRoute.Kind)
		assert.Equal(t, slug, view.InvocationRoute.ChangeSlug)
		assert.True(t, view.InvocationRoute.LocalLifecycleExecutionAllowed)
		assert.True(t, view.InvocationRoute.EffectiveLifecycleExecutionAllowed)
		assert.Equal(t, "slipway next --change "+slug, view.InvocationRoute.NextCommand)
		assert.Equal(t, expectedPath, view.Path)
		assert.True(t, view.Recorded)

		rec, err := state.LoadVerification(root, slug, progression.SkillPlanAudit)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Empty(t, rec.Blockers)
		assert.False(t, rec.Timestamp.IsZero())
		assert.Equal(t, 0, rec.RunVersion)
		assert.Equal(t, []string{"plan-audit:pass"}, rec.References)
		assert.Equal(t, "Plan audit passed.", rec.Notes)

		digests, err := state.LoadEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		require.Contains(t, digests.Skills, progression.SkillPlanAudit)
		assert.NotEmpty(t, digests.Skills[progression.SkillPlanAudit].Inputs["tasks.md"])

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, change.CurrentState, reloaded.CurrentState)
		assert.Equal(t, expectedPath, reloaded.EvidenceRefs[progression.SkillPlanAudit])

		events, err := state.ReadLifecycleEvents(root, reloaded)
		require.NoError(t, err)
		require.NotEmpty(t, events)
		assert.Equal(t, "skill.evidence_recorded", events[len(events)-1].EventType)
		assert.Equal(t, "recorded", events[len(events)-1].Result)
	})
}

func TestEvidenceSkillChangeFlagRejectsMissingSlugWithoutDiagnosticsFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	cmd := commandForRoot(t, root, makeEvidenceCmd())
	cmd.SetArgs([]string{
		"skill",
		"--json",
		"--change", "definitely-not-a-change",
		"--skill", progression.SkillPlanAudit,
		"--verdict", model.VerificationVerdictPass,
		"--notes", "will not be recorded",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cliErr := asCLIError(cmd.Execute())
	require.NotNil(t, cliErr)
	assert.Equal(t, "change_not_found", cliErr.ErrorCode)
	assert.Equal(t, "definitely-not-a-change", cliErr.Slug)
	assert.NotContains(t, out.String(), "no active change or ambiguous")
}

func TestEvidenceSkillFailsClosedWithoutActiveChange(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	cmd := commandForRoot(t, root, makeEvidenceCmd())
	cmd.SetArgs([]string{
		"skill",
		"--json",
		"--skill", progression.SkillPlanAudit,
		"--verdict", model.VerificationVerdictPass,
		"--notes", "will not be recorded",
	})
	cliErr := asCLIError(cmd.Execute())
	require.NotNil(t, cliErr)
	assert.Equal(t, "no_active_change", cliErr.ErrorCode)
}

func TestEvidenceSkillChangeFlagRejectsArchivedTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence archived target")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	require.NoError(t, state.SaveChange(root, change))
	_, err = state.ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	cmd := commandForRoot(t, root, makeEvidenceCmd())
	cmd.SetArgs([]string{
		"skill",
		"--json",
		"--change", slug,
		"--skill", progression.SkillPlanAudit,
		"--verdict", model.VerificationVerdictPass,
		"--notes", "will not be recorded",
	})
	cliErr := asCLIError(cmd.Execute())
	require.NotNil(t, cliErr)
	assert.Equal(t, "archived_change_not_validatable", cliErr.ErrorCode)
	assert.Equal(t, slug, cliErr.Slug)
}

func TestEvidenceSkillAllowsStaleResearchRestampFromAuditSubstep(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelDiscovery, "evidence skill restamps stale research")
		change := setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)
		writeMinimalGovernedBundle(t, root, change)
		writeSkillVerification(t, root, slug, progression.SkillResearchOrchestration, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 0,
			References: []string{"research:pass"},
			Notes:      "Original research certification.",
		})
		refreshPassingSkillDigestsForTest(t, root, slug, progression.SkillResearchOrchestration)

		researchPath := filepath.Join(root, "artifacts", "changes", slug, "research.md")
		raw, err := os.ReadFile(researchPath)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(researchPath, append(raw, []byte("\nAdditional current research.\n")...), 0o644))

		target, ok, err := progression.StaleEvidenceRepairAvailable(root, change, nil)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, progression.SkillResearchOrchestration, target.SkillName)

		readiness, err := progression.EvaluateGovernanceReadiness(root, change, progression.GovernanceReadinessOptions{
			IncludeGateEvaluations: true,
		})
		require.NoError(t, err)
		require.True(t,
			hasRecoverableRequiredSkillStaleForSkill(readinessBlockers(readiness), progression.SkillResearchOrchestration),
			"public readiness blockers must identify the same stale required skill",
		)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillResearchOrchestration,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "research:pass",
			"--notes", "Research re-certified after current artifact updates.",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, progression.SkillResearchOrchestration, view.SkillName)
		assert.True(t, view.Recorded)

		rec, err := state.LoadVerification(root, slug, progression.SkillResearchOrchestration)
		require.NoError(t, err)
		assert.Equal(t, "Research re-certified after current artifact updates.", rec.Notes)

		target, ok, err = progression.StaleEvidenceRepairAvailable(root, change, nil)
		require.NoError(t, err)
		assert.False(t, ok, "research restamp should clear the stale evidence repair target")
		assert.Empty(t, target.SkillName)
	})
}

func TestEvidenceSkillRejectsNonStaleResearchRestampFromAuditSubstep(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelDiscovery, "evidence skill rejects non-stale research restamp")
		change := setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)
		writeMinimalGovernedBundle(t, root, change)
		writeSkillVerification(t, root, slug, progression.SkillResearchOrchestration, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 0,
			References: []string{"research:pass"},
			Notes:      "Original current research certification.",
		})
		refreshPassingSkillDigestsForTest(t, root, slug, progression.SkillResearchOrchestration)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillResearchOrchestration,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "research:pass",
			"--notes", "Unexpected research overwrite.",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_wrong_plan_substep", cliErr.ErrorCode)

		rec, err := state.LoadVerification(root, slug, progression.SkillResearchOrchestration)
		require.NoError(t, err)
		assert.Equal(t, "Original current research certification.", rec.Notes)
	})
}

func TestEvidenceSkillAllowsStaleUpstreamIntakeRecertAtS1Plan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill recerts stale intake at S1")
		// S0-owned intake-clarification certifies intent.md at S0_INTAKE, then the
		// change advances into S1_PLAN. Keep the substep before audit so a fresh
		// plan-audit cannot supersede the intake drift for this test.
		change := setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepNone)
		writeMinimalGovernedBundle(t, root, change)
		writeSkillVerification(t, root, slug, progression.SkillIntakeClarification, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 0,
			References: []string{"intake:pass"},
			Notes:      "Original intake clarification.",
		})
		refreshPassingSkillDigestsForTest(t, root, slug, progression.SkillIntakeClarification)

		// Mutate the certified intent.md body after the change advanced past S0, so
		// the upstream intake-clarification evidence goes stale with no S0 reopen
		// path. The change must land under a hashed heading (the Intent body), not
		// the Open Questions section, which the intake digest intentionally ignores.
		intentPath := filepath.Join(root, "artifacts", "changes", slug, "intent.md")
		require.NoError(t, os.WriteFile(intentPath, []byte(`# Intent
INT-001: test fixture intent, clarified scope after planning began.

## Open Questions
(none)
`), 0o644))

		// The engine flags the stale upstream skill as a recoverable repair target.
		// (Readiness blockers intentionally omit intake-clarification staleness today
		// — that view-path vs advance-path divergence is a tracked follow-up — so the
		// in-place re-cert exception keys off the repair target, not readiness.)
		target, ok, err := progression.StaleEvidenceRepairAvailable(root, change, nil)
		require.NoError(t, err)
		require.True(t, ok, "engine must flag the stale upstream intake skill as a recoverable repair target")
		require.Equal(t, progression.SkillIntakeClarification, target.SkillName)

		refreshRequired, err := staleEvidenceSkillRefreshRequired(root, change, progression.SkillIntakeClarification)
		require.NoError(t, err)
		require.True(t, refreshRequired, "the wrong-state guard must treat the flagged stale intake skill as refresh-required")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillIntakeClarification,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "intake:pass",
			"--notes", "Intake re-certified after current intent updates.",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, progression.SkillIntakeClarification, view.SkillName)
		assert.True(t, view.Recorded)

		rec, err := state.LoadVerification(root, slug, progression.SkillIntakeClarification)
		require.NoError(t, err)
		assert.Equal(t, "Intake re-certified after current intent updates.", rec.Notes)

		target, ok, err = progression.StaleEvidenceRepairAvailable(root, change, nil)
		require.NoError(t, err)
		assert.False(t, ok, "intake re-cert should clear the stale evidence repair target")
		assert.Empty(t, target.SkillName)
	})
}

func TestEvidenceSkillRejectsNonStaleWrongStateSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill rejects non-stale wrong state")
		// An S3-owned review skill at S1_PLAN is at the wrong state and is not a
		// recoverable stale-repair target, so the wrong-state guard must still fail
		// closed even with the in-place re-cert exception present.
		setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", model.VerificationVerdictPass,
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_wrong_state", cliErr.ErrorCode)
		assert.Equal(t, progression.SkillSpecComplianceReview, cliErr.Details["skill"])
		assert.Equal(t, string(model.StateS3Review), cliErr.Details["required_state"])
		assert.Equal(t, string(model.StateS1Plan), cliErr.Details["current_state"])
	})
}

func TestEvidenceSkillNotesFileUsesBoundWorktreeWorkspace(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill bound notes file")
		change := setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)

		worktreeRoot := filepath.Join(t.TempDir(), slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		notesRel := filepath.ToSlash(filepath.Join("artifacts", "changes", slug, "verification", "plan-audit-notes.md"))
		notesPath := filepath.Join(worktreeRoot, filepath.FromSlash(notesRel))
		require.NoError(t, os.MkdirAll(filepath.Dir(notesPath), 0o755))
		require.NoError(t, os.WriteFile(notesPath, []byte("Bound worktree plan audit passed.\n"), 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillPlanAudit,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "plan-audit:pass",
			"--notes-file", notesRel,
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		expectedPath := state.DisplayPath(
			root,
			filepath.Join(worktreeRoot, "artifacts", "changes", slug, "verification", "plan-audit.yaml"),
		)
		assert.Equal(t, expectedPath, view.Path)

		rec, err := state.LoadVerification(root, slug, progression.SkillPlanAudit)
		require.NoError(t, err)
		assert.Equal(t, "Bound worktree plan audit passed.", rec.Notes)
	})
}

func TestEvidenceSkillFailOverwritesPlanAuditAndPrunesDigest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill fail prunes digest")
		change := setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)

		passCmd := commandForRoot(t, root, makeEvidenceCmd())
		passCmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillPlanAudit,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "plan-audit:pass",
			"--notes", "Plan audit passed.",
		})
		require.NoError(t, passCmd.Execute())

		digests, err := state.LoadEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		require.Contains(t, digests.Skills, progression.SkillPlanAudit)

		failCmd := commandForRoot(t, root, makeEvidenceCmd())
		failCmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillPlanAudit,
			"--verdict", model.VerificationVerdictFail,
			"--blocker", "required_skill_missing:plan-audit",
			"--notes", "Plan audit now fails.",
		})
		require.NoError(t, failCmd.Execute())

		rec, err := state.LoadVerification(root, slug, progression.SkillPlanAudit)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictFail, rec.Verdict)

		digests, err = state.LoadEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		assert.NotContains(t, digests.Skills, progression.SkillPlanAudit)
	})
}

func TestEvidenceSkillRecordsSelectedReviewPeerWithoutSpecPredecessor(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill records unordered review peer")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillCodeQualityReview,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "code-quality:pass",
			"--notes", "Quality review passed.",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, progression.SkillCodeQualityReview, view.SkillName)
		assert.Equal(t, 1, view.RunVersion)
		assert.True(t, view.Recorded)

		rec, err := state.LoadVerification(root, slug, progression.SkillCodeQualityReview)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Equal(t, 1, rec.RunVersion)

		_, err = os.Stat(filepath.Join(state.VerificationDir(root, slug), progression.SkillSpecComplianceReview+".yaml"))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestEvidenceSkillAllowsSelectedReviewerRestampForInvalidContextOrigin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill restamps invalid review origin")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingReviewEvidencePack(t, root, slug, 1)
		// Overwrite the spec-compliance-review record with a malformed
		// review-stage context-origin handle so the selected-reviewer restamp
		// path becomes available for a narrow, in-place replacement.
		writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
			References: []string{
				"layer:R0=pass",
				"layer:R3=pass",
				model.ContextOriginReferencePrefix + "goal=retired-goal-context",
			},
		})
		refreshPassingSkillDigestsForTest(t, root, slug, progression.SkillSpecComplianceReview)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "layer:R0=pass",
			"--reference", "layer:R3=pass",
			"--reference", model.ContextOriginReferencePrefix + model.StageContextReview + "=fresh-spec-review-context",
			"--notes", "Spec-compliance review rerun in fresh review context.",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, progression.SkillSpecComplianceReview, view.SkillName)
		assert.True(t, view.Recorded)

		rec, err := state.LoadVerification(root, slug, progression.SkillSpecComplianceReview)
		require.NoError(t, err)
		handle, ok := model.ReviewContextOriginHandleFromVerification(rec)
		require.True(t, ok)
		assert.Equal(t, "fresh-spec-review-context", handle.Handle)
		assert.NotContains(t, rec.References, model.ContextOriginReferencePrefix+"goal=retired-goal-context")
	})
}

func TestEvidenceSkillRejectsSelectedReviewerRestampWithValidContextOrigin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill rejects valid review origin overwrite")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingReviewEvidencePack(t, root, slug, 1)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "layer:R0=pass",
			"--reference", "layer:R3=pass",
			"--reference", model.ContextOriginReferencePrefix + model.StageContextReview + "=unexpected-overwrite-context",
			"--notes", "Unexpected spec-compliance review overwrite.",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_not_current", cliErr.ErrorCode)

		rec, err := state.LoadVerification(root, slug, progression.SkillSpecComplianceReview)
		require.NoError(t, err)
		handle, ok := model.ReviewContextOriginHandleFromVerification(rec)
		require.True(t, ok)
		assert.Equal(t, testSpecContextHandle, handle.Handle)
		assert.NotEqual(t, "Unexpected spec-compliance review overwrite.", rec.Notes)
	})
}

func TestEvidenceSkillRejectsUnselectedSecurityReview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill rejects unselected security")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillSecurityReview,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "security-review:pass",
			"--notes", "Security review passed.",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_not_current", cliErr.ErrorCode)
		assert.Equal(t, progression.SkillSecurityReview, cliErr.Details["skill"])

		_, err := os.Stat(filepath.Join(state.VerificationDir(root, slug), progression.SkillSecurityReview+".yaml"))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestEvidenceSkillRejectsShipVerificationBeforeReviewSet(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill rejects ship ordering")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{})
		// Deliberately omit the selected review set so ship-verification, the
		// terminal S3 skill, is not yet the current actionable skill.

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillShipVerification,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "verification:pass",
			"--notes", "Ship verification passed.",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_predecessor_required", cliErr.ErrorCode)
		assert.Equal(t, progression.SkillShipVerification, cliErr.Details["skill"])
		// ship-verification runs last; the first still-missing selected reviewer
		// must be recorded before it.
		assert.Equal(t, progression.SkillSpecComplianceReview, cliErr.Details["required_first"])

		_, err := os.Stat(filepath.Join(state.VerificationDir(root, slug), progression.SkillShipVerification+".yaml"))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestEvidenceSkillRejectsRunSummaryBoundWithoutExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill requires execution summary")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "spec-compliance:pass",
			"--notes", "Review passed.",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_run_summary_missing", cliErr.ErrorCode)
		assert.Equal(t, slug, cliErr.Slug)
		assert.Equal(t, progression.SkillSpecComplianceReview, cliErr.Details["skill"])

		_, err := os.Stat(filepath.Join(state.VerificationDir(root, slug), progression.SkillSpecComplianceReview+".yaml"))
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestEvidenceSkillRecordsWaveOrchestrationFromRuntimeTaskEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		capturedAt := time.Now().UTC().Add(-time.Minute)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "go test ./cmd -run TestEvidenceSkillRecordsWaveOrchestrationFromRuntimeTaskEvidence",
			"captured_at":   capturedAt.Format(time.RFC3339Nano),
		})

		summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		require.NoError(t, err)
		require.Nil(t, summary)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "wave-orchestration:pass",
			"--notes", "Wave orchestration passed.",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		expectedPath := state.DisplayPath(
			root,
			filepath.Join(root, "artifacts", "changes", slug, "verification", "wave-orchestration.yaml"),
		)
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, progression.SkillWaveOrchestration, view.SkillName)
		assert.Equal(t, model.VerificationVerdictPass, view.Verdict)
		assert.Equal(t, 1, view.RunVersion)
		assert.Equal(t, expectedPath, view.Path)
		assert.True(t, view.Recorded)

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Equal(t, 1, rec.RunVersion)
		assert.Equal(t, []string{"wave-orchestration:pass"}, rec.References)
		assert.Equal(t, "Wave orchestration passed.", rec.Notes)

		digests, err := state.LoadEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		require.Contains(t, digests.Skills, progression.SkillWaveOrchestration)
		assert.Contains(t, digests.Skills[progression.SkillWaveOrchestration].Inputs, "wave-plan.yaml")
		assert.Contains(t, digests.Skills[progression.SkillWaveOrchestration].Inputs, "runtime_task_evidence")
		assert.NotContains(t, digests.Skills[progression.SkillWaveOrchestration].Inputs, "execution-summary.yaml")

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Implement, reloaded.CurrentState)
		assert.Equal(t, expectedPath, reloaded.EvidenceRefs[progression.SkillWaveOrchestration])
	})
}

func TestEvidenceSkillRejectsWrongState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill wrong state")
		setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", model.VerificationVerdictPass,
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_wrong_state", cliErr.ErrorCode)
		assert.Equal(t, progression.SkillSpecComplianceReview, cliErr.Details["skill"])
		assert.Equal(t, string(model.StateS3Review), cliErr.Details["required_state"])
		assert.Equal(t, string(model.StateS1Plan), cliErr.Details["current_state"])
	})
}

func TestEvidenceSkillWrongStateForWaveOrchestrationInS3RoutesToReviewAndVerificationEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill wave wrong state in review")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", model.VerificationVerdictPass,
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_wrong_state", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, progression.SkillSpecComplianceReview)
		assert.Contains(t, cliErr.Remediation, progression.SkillCodeQualityReview)
		assert.Contains(t, cliErr.Remediation, progression.SkillIndependentReview)
		assert.Contains(t, cliErr.Remediation, progression.SkillShipVerification)
	})
}

func TestEvidenceSkillWrongStateForWaveEvidenceInS3RoutesToShipVerificationEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill wave wrong state in review")
		setEvidenceSkillChangeState(t, root, slug, model.StateS3Review, model.PlanSubStepNone)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", model.VerificationVerdictPass,
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_wrong_state", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, progression.SkillShipVerification)
	})
}

func TestEvidenceSkillRejectsNotesConflict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence skill notes conflict")
		setEvidenceSkillChangeState(t, root, slug, model.StateS1Plan, model.PlanSubStepAudit)

		notesPath := filepath.Join(root, "notes.md")
		require.NoError(t, os.WriteFile(notesPath, []byte("file notes\n"), 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--change", slug,
			"--skill", progression.SkillPlanAudit,
			"--verdict", model.VerificationVerdictPass,
			"--notes", "inline notes",
			"--notes-file", notesPath,
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_notes_conflict", cliErr.ErrorCode)
	})
}

func setEvidenceSkillChangeState(
	t *testing.T,
	root string,
	slug string,
	workflowState model.WorkflowState,
	planSubStep model.PlanSubStep,
) model.Change {
	t.Helper()

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = workflowState
	change.PlanSubStep = planSubStep
	if workflowState != model.StateS1Plan {
		change.PlanSubStep = model.PlanSubStepNone
	}
	require.NoError(t, state.SaveChange(root, change))
	return change
}
