package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/gate"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/engine/skill"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionableSkillViewsOmitAlreadyPassingDisplaySkillWithoutBlocker(t *testing.T) {
	t.Parallel()

	change := model.NewChange("passing-display-skill")
	change.CurrentState = model.StateS3Review
	passingRecord := model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Timestamp: time.Now().UTC(),
	}
	readiness := progression.GovernanceReadiness{
		PassingSkills: map[string]model.VerificationRecord{
			progression.SkillSpecComplianceReview: passingRecord,
			progression.SkillCodeQualityReview:    passingRecord,
			progression.SkillIndependentReview:    passingRecord,
			progression.SkillShipVerification:     passingRecord,
		},
	}

	assert.Nil(t, buildActionableNextSkillView(change, readiness))

	view := nextView{CurrentState: model.StateS3Review}
	err := assembleSkillViewWithOptions(
		t.TempDir(),
		&view,
		changeRef{Slug: change.Slug},
		progression.AdvanceSummary{},
		&change,
		nil,
		readiness.PassingSkills,
		nil,
		assembleSkillViewOptions{},
	)
	require.NoError(t, err)
	assert.Nil(t, view.NextSkill)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "no_skill_required:S3_REVIEW")
}

func TestNextReturnsNextSkillForGovernedState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "add caching layer")

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, "governed", view.ExecutionMode)
		// NextSkill should point to a governance skill or indicate ready for advance
		if view.NextSkill != nil {
			assert.NotEmpty(t, view.NextSkill.Name)
		}
	})
}

func TestNextS0ResearchActionReportsRunGuidanceWithoutPrematureScopeGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createIntakeChangeFixture(t, root, "clarify workflow feedback")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.IntakeSubStep = model.IntakeSubStepResearch
		require.NoError(t, state.SaveChange(root, change))
		writeSkillVerification(t, root, slug, progression.SkillIntakeClarification, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 0,
		})

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Nil(t, view.NextSkill)
		assert.NotContains(t, view.InputContext.GateStatus, string(gate.GateScope))
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "run_slipway_run_to_advance:S0_INTAKE")
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "no_skill_required:S0_INTAKE")
		assert.Equal(t, "run_slipway_run_to_advance", view.ConfirmationRequirement.Reason)
		require.NotEmpty(t, view.RequiredActions)
		for _, action := range view.RequiredActions {
			if action.ControlID == string(model.ControlResearch) {
				assert.NotContains(t, action.Description, "complete research.md")
				assert.Contains(t, action.Description, "S0 intake research questions")
			}
		}
	})
}

func TestNextS3ReviewWithPassingPeerEvidenceReportsGoalVerificationHandoff(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "s3 run guidance")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	writeShipReadyGovernedBundle(t, root, change)
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)

	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, Command: "run"})
	require.NoError(t, err)
	require.NotNil(t, view.NextSkill)
	assert.Equal(t, progression.SkillShipVerification, view.NextSkill.Name)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:ship-verification")
	assert.Equal(t, "skill_handoff:"+progression.SkillShipVerification, view.ConfirmationRequirement.Reason)
	for _, blocker := range view.Blockers {
		if blocker.Code == "no_skill_required" {
			assert.NotEqual(t, model.ReasonSeverityError, blocker.Severity)
		}
	}
}

func TestNextStalePlanningEvidenceReportsReviewAlignmentHandoff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug, _ := prepareStalePlanningRecoveryFixture(t, root, model.StateS3Review)

	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, Command: "run"})
	require.NoError(t, err)

	require.NotNil(t, view.NextSkill)
	assert.Equal(t, model.StateS3Review, view.CurrentState)
	assert.NotEqual(t, progression.SkillPlanAudit, view.NextSkill.Name)
	reasons := strings.Join(model.ReasonSpecs(view.Blockers), "\n")
	assert.NotContains(t, reasons, "required_skill_stale:plan-audit:")
	assert.NotContains(t, reasons, "required_skill_stale:wave-orchestration:")
	assert.NotContains(t, reasons, "required_skill_stale:intake-clarification:")
	assert.NotContains(t, reasons, state.StalePlanningEvidenceBlockerToken)
	assert.NotContains(t, reasons, "run_slipway_run_to_advance:"+string(model.StateS3Review))
	assert.Equal(t, "review_batch", view.ConfirmationRequirement.Reason)
	require.NotNil(t, view.FreshnessDiagnostics)
	assert.Equal(t, "fresh", view.FreshnessDiagnostics.Status)
	assert.Empty(t, view.FreshnessDiagnostics.StalePairs)
	assert.Contains(t, view.Warnings, state.S3TaskPlanAmendmentDiagnostic)

	loaded, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS3Review, loaded.CurrentState, "next must remain read-only")
	assert.Equal(t, model.PlanSubStepNone, loaded.PlanSubStep)
}

func TestStalePlanningReviewAlignmentActionContractStaysConsistentAcrossSurfaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug, _ := prepareStalePlanningRecoveryFixture(t, root, model.StateS3Review)
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	tasksPath := filepath.Join(bundlePath, "tasks.md")
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` verify recovered planning chain
  - depends_on: []
  - target_files: ["cmd/done.go"]
  - task_kind: verification
  - covers: [REQ-001]
`)))
	staleAt := time.Now().UTC().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(tasksPath, staleAt, staleAt))
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	require.NotEmpty(t, summary.Tasks)
	summary.Tasks[0].ChangedFiles = []string{"cmd/done.go"}
	summary.Tasks[0].TargetFiles = []string{"cmd/done.go"}
	writeExecutionSummary(t, root, slug, summary)

	assertReviewAlignmentHandoff := func(t *testing.T, surface, skillName, actionKind, actionReason string, selected []string) {
		t.Helper()
		assert.NotEmpty(t, skillName, surface)
		assert.NotEqual(t, progression.SkillPlanAudit, skillName, surface)
		assert.Contains(t, selected, skillName, surface)
		assert.Equal(t, "review_batch", actionKind, surface)
		assert.Equal(t, "review_batch", actionReason, surface)
	}

	nextCmd := commandForRoot(t, root, makeNextCmd())
	nextCmd.SetArgs([]string{"--json", "--change", slug})
	var nextOut bytes.Buffer
	nextCmd.SetOut(&nextOut)
	require.NoError(t, nextCmd.Execute())
	var nextHandoff nextHandoffView
	require.NoError(t, json.Unmarshal(nextOut.Bytes(), &nextHandoff))
	require.NotNil(t, nextHandoff.NextSkill)
	assertReviewAlignmentHandoff(
		t,
		"next handoff",
		nextHandoff.NextSkill.Name,
		nextHandoff.Confirmation.NextActionKind,
		nextHandoff.Confirmation.Reason,
		nextHandoff.NextSkill.SelectedReviewSkills,
	)

	nextDiagCmd := commandForRoot(t, root, makeNextCmd())
	nextDiagCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var nextDiagOut bytes.Buffer
	nextDiagCmd.SetOut(&nextDiagOut)
	require.NoError(t, nextDiagCmd.Execute())
	var nextDiag nextView
	require.NoError(t, json.Unmarshal(nextDiagOut.Bytes(), &nextDiag))
	require.NotNil(t, nextDiag.NextSkill)
	assertReviewAlignmentHandoff(
		t,
		"next diagnostics",
		nextDiag.NextSkill.Name,
		nextDiag.CurrentActionKind,
		nextDiag.ConfirmationRequirement.Reason,
		nextDiag.NextSkill.SelectedReviewSkills,
	)

	validateCmd := commandForRoot(t, root, makeValidateCmd())
	validateCmd.SetArgs([]string{"--change", slug})
	var validateOut bytes.Buffer
	validateCmd.SetOut(&validateOut)
	require.NoError(t, validateCmd.Execute())
	var validate validateView
	require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validate))
	require.NotNil(t, validate.ActionableNextSkill)
	assertReviewAlignmentHandoff(
		t,
		"validate",
		validate.ActionableNextSkill.Name,
		validate.CurrentActionKind,
		validate.CurrentActionKind,
		validate.ActionableNextSkill.SelectedReviewSkills,
	)

	statusCmd := commandForRoot(t, root, makeStatusCmd())
	statusCmd.SetArgs([]string{"--json", "--change", slug})
	var statusOut bytes.Buffer
	statusCmd.SetOut(&statusOut)
	require.NoError(t, statusCmd.Execute())
	var status statusView
	require.NoError(t, json.Unmarshal(statusOut.Bytes(), &status))
	require.NotNil(t, status.ActionableNextSkill)
	assert.Equal(t, "review_batch", status.CurrentActionKind)
	assert.Equal(t, "slipway run", status.CurrentActionCommand)
	assertReviewAlignmentHandoff(
		t,
		"status",
		status.ActionableNextSkill.Name,
		status.CurrentActionKind,
		status.CurrentActionKind,
		status.ActionableNextSkill.SelectedReviewSkills,
	)

	runCmd := commandForRoot(t, root, makeRunCmd())
	runCmd.SetArgs([]string{"--json", "--change", slug})
	var runOut bytes.Buffer
	runCmd.SetOut(&runOut)
	require.NoError(t, runCmd.Execute())
	var runHandoff nextHandoffView
	require.NoError(t, json.Unmarshal(runOut.Bytes(), &runHandoff))
	require.NotNil(t, runHandoff.NextSkill)
	assertReviewAlignmentHandoff(
		t,
		"run handoff",
		runHandoff.NextSkill.Name,
		runHandoff.Confirmation.NextActionKind,
		runHandoff.Confirmation.Reason,
		runHandoff.NextSkill.SelectedReviewSkills,
	)
	assert.Equal(t, nextDiag.NextSkill.Name, nextHandoff.NextSkill.Name)
	assert.Equal(t, nextDiag.NextSkill.Name, status.ActionableNextSkill.Name)
	assert.Equal(t, nextDiag.NextSkill.Name, validate.ActionableNextSkill.Name)
	assert.Equal(t, nextDiag.NextSkill.Name, runHandoff.NextSkill.Name)
}

func TestNextS0ConfirmWithoutApprovedSummaryDoesNotReportRunGuidance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createIntakeChangeFixture(t, root, "confirm requires approved summary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.IntakeSubStep = model.IntakeSubStepConfirm
		change.ComplexityLevel = "simple"
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "intent.md", []byte(`# Intent

## Summary
Clarified workflow diagnostic fix.

## In Scope
- Keep S0 confirm blocked until the user approves the summary.

## Out of Scope
- Do not advance planning before confirmation.

## Acceptance Signals
- next --json does not advertise slipway run before approval.

## Open Questions
<!-- none -->

## Approved Summary
<!-- pending user confirmation -->
`)))
		writeSkillVerification(t, root, slug, progression.SkillIntakeClarification, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 0,
		})

		view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, Command: "run"})
		require.NoError(t, err)
		assert.Nil(t, view.NextSkill)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "intake_confirmation_incomplete:intent.md requires non-empty 'Approved Summary'")
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "no_skill_required:S0_INTAKE")
		assert.NotContains(t, model.ReasonSpecs(view.Blockers), "run_slipway_run_to_advance:S0_INTAKE")
		assert.NotEqual(t, "run_slipway_run_to_advance", view.ConfirmationRequirement.Reason)
	})
}

func TestNextS1BundleSurfacesPlanAuditHandoff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelDiscovery, "audit bundle handoff")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.PlanSubStep = model.PlanSubStepBundle
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.NextSkill, "advanced=%+v blockers=%v warnings=%v", view.Advanced, model.ReasonSpecs(view.Blockers), view.Warnings)
		assert.Equal(t, progression.SkillPlanAudit, view.NextSkill.Name)
		assert.NotContains(t, model.ReasonSpecs(view.Blockers), "no_skill_required:S1_PLAN")
		assert.Contains(t, strings.Join(view.Warnings, "\n"), "S1_PLAN/bundle")
	})
}

func TestNextPreviewIncludesGovernanceSurfaceAndActionBlockers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "governance blocker preview")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepResearch
		change.ArtifactSchema = model.ArtifactSchemaCore
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

		governanceSignals, ok := payload["governance_signals"].(map[string]any)
		require.True(t, ok, "expected governance_signals in next output")
		// Post-simplification: only blast_radius and domains remain.
		assert.Equal(t, "low", governanceSignals["blast_radius"])

		// Post-simplification: exploration control derives from NeedsDiscovery,
		// not from confidence scores. An L2 change (NeedsDiscovery=false) with
		// low confidence no longer triggers exploration. This is intentional drift.
		// Verify governance surface and snapshot exist instead.
		requiredActions, _ := payload["required_actions"].([]any)
		foundResearch := false
		for _, raw := range requiredActions {
			actionMap, ok := raw.(map[string]any)
			if ok && actionMap["control_id"] == "research" {
				foundResearch = true
			}
		}
		assert.False(t, foundResearch, "L2 change (NeedsDiscovery=false) should not trigger research control after simplification")

		_, err = os.Stat(state.GovernanceSnapshotCachePath(root, slug))
		assert.True(t, os.IsNotExist(err), "next (query-only) should not persist governance snapshots")
	})
}

func TestNextDoesNotPersistArtifactReconcile(t *testing.T) {
	assertReadOnlyArtifactReconcileDoesNotPersist(
		t,
		"next query-only read-only reconcile",
		"fixed-hash-before-next-query",
		func(out *bytes.Buffer) error {
			cmd := makeNextCmd()
			cmd.SetOut(out)
			cmd.SetArgs([]string{"--json"})
			return cmd.Execute()
		},
	)
}

func TestNextPreviewIgnoresUnreadableGovernanceSnapshot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "recover from corrupt governance snapshot")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaCore
		require.NoError(t, state.SaveChange(root, change))

		snapshotPath := state.GovernanceSnapshotCachePath(root, slug)
		require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
		require.NoError(t, os.WriteFile(
			snapshotPath,
			[]byte("version: ["),
			0o644,
		))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

		_, ok := payload["governance_signals"].(map[string]any)
		require.True(t, ok, "expected governance_signals in next output")

		raw, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		assert.Equal(t, "version: [", string(raw), "next (query-only) should not repair or rewrite snapshot cache")
	})
}

func TestNextPreviewExposesPlanningRecoveryState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "preview should expose plan recovery state")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepValidate
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.PlanSubStepValidate, view.PlanSubStep)
		assert.Contains(t, view.PlanningNote, "recovery-only")
	})
}

func TestNextJSONAutoPassesByDefault(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "light preset json autopass advisory")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	writeShipReadyGovernedBundle(t, root, change)
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)
	writePassingShipVerificationEvidence(t, root, slug, 1)

	// Advancement is tested via buildNextView with preview=false (run path).
	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
	require.NoError(t, err)
	require.NotNil(t, view.Advanced)
	assert.Equal(t, "done_ready", view.Advanced.Action)
	assert.Empty(t, view.Advanced.AutoPassedStates)
	assert.Empty(t, view.AutoPassEligible)
	assert.Nil(t, view.NextSkill)

	updated, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS3Review, updated.CurrentState)
	assert.Empty(t, updated.LastAutoPassedStates)
}

func TestNextJSONNoAutoPassReportsEligibilityFromCurrentStateOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "light preset explicit no-auto-pass advisory")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	writeShipReadyGovernedBundle(t, root, change)
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)
	writePassingShipVerificationEvidence(t, root, slug, 1)

	// Advancement with no-auto-pass is tested via buildNextView (run path).
	// autoSkipEvidence=false mirrors the original --json path.
	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{SkipAutoPass: true, Command: "run"})
	require.NoError(t, err)
	require.NotNil(t, view.Advanced)
	assert.Equal(t, "done_ready", view.Advanced.Action)
	assert.Empty(t, view.Advanced.AutoPassedStates)
	assert.Empty(t, view.AutoPassEligible)
	assert.Nil(t, view.NextSkill)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "run_slipway_done_to_finalize")

	updated, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS3Review, updated.CurrentState)
	assert.Empty(t, updated.LastAutoPassedStates)
}

func TestNextDoesNotAutoPassLightPresetReviewWithoutExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "light preset review still requires execution authority")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.WorkflowPreset = model.WorkflowPresetLight
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		var buf bytes.Buffer
		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.StateS3Review, view.CurrentState)
		if view.Advanced != nil {
			assert.Equal(t, "query", view.Advanced.Action, "query-first next JSON must stay read-only while surfacing missing execution-summary blockers")
		}
		require.NotNil(t, view.NextSkill, "advanced=%+v blockers=%v warnings=%v", view.Advanced, model.ReasonSpecs(view.Blockers), view.Warnings)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.NextSkill.Name)
	})
}

func TestNextDoesNotReturnDoneReadyWithoutGoalVerification(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "done-ready still requires goal verification")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.WorkflowPreset = model.WorkflowPresetLight
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "intent.md", []byte("# Proposal")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "requirements.md", []byte("# Spec")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "decision.md", []byte("# Design")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte("- [ ] `t-01` verify\n  - target_files: [\"cmd/done.go\"]\n  - task_kind: verification\n")))
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)

		var buf bytes.Buffer
		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.StateS3Review, view.CurrentState)
		if view.Advanced != nil {
			assert.Equal(t, "query", view.Advanced.Action, "query-first next JSON must stay read-only while surfacing missing ship-verification evidence")
		}
		require.NotNil(t, view.NextSkill, "advanced=%+v blockers=%v warnings=%v", view.Advanced, model.ReasonSpecs(view.Blockers), view.Warnings)
		assert.Equal(t, progression.SkillShipVerification, view.NextSkill.Name)
	})
}

// TestNextS3ShipVerificationSurfacesSubagentDelegationAcrossCapabilityStates
// pins the host subagent-delegation contract for the terminal ship-verification
// gate once the S3 review batch has cleared (#369). ship-verification is the
// single always-required terminal S3 gate; it REQUIRES dispatching a fresh
// verifier subagent but is not a catalog-registered skill, so the contract comes
// from the built-in subagent-dispatch lever. Without this, a host that cleared
// the four-reviewer batch via the named fallback hit the SAME dead-end one step
// later at ship-verification. "unknown" (host declared nothing) stays continuable
// on the skill_handoff boundary while riding a
// subagent_dispatch_authorization_required prerequisite with an enriched,
// named-fallback next_action; "unavailable" (host declared other capabilities but
// not subagent) fails closed as a first-class host_capability_unavailable blocker;
// "available" is unchanged; an explicit fallback clears the blocker without a
// bypass. This test exercises the public next path with ship-verification as the
// selected terminal skill so the prerequisite genuinely reaches the surface.
func TestNextS3ShipVerificationSurfacesSubagentDelegationAcrossCapabilityStates(t *testing.T) {
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	writeTestSubagentConfig(t, root, func(cfg *model.Config) {
		cfg.Subagents.Verify = model.SubagentSlot{
			Type:                model.SubagentTypeNative,
			Name:                "ship-verifier",
			SessionInstructions: "Verify terminal readiness without modifying files.",
			Timeout:             "30m",
		}
	})

	slug := createGovernedRequest(t, root, levelNonDiscovery, "ship verification subagent delegation")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	writeShipReadyGovernedBundle(t, root, change)
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)

	const subagentBlocker = "subagent_dispatch_authorization_required:ship-verification:subagent"
	const unavailableBlocker = "host_capability_unavailable:ship-verification:subagent"

	t.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "")

	// unknown: host declared nothing -> continuable skill_handoff riding the prerequisite.
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "")
	unknown, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, Command: "run"})
	require.NoError(t, err)
	require.NotNil(t, unknown.NextSkill)
	assert.Equal(t, progression.SkillShipVerification, unknown.NextSkill.Name)
	assertSubagentDirective(
		t,
		unknown.NextSkill.Subagent,
		model.SubagentTypeNative,
		"ship-verifier",
		"Verify terminal readiness without modifying files.",
		"30m",
		true,
		"deny",
	)
	unknownCap := requireHostCapabilityForSkill(t, unknown.HostCapabilities, progression.SkillShipVerification)
	assert.Equal(t, "unknown", unknownCap.Availability)
	assert.False(t, unknownCap.FallbackSelected)
	unknownSpecs := model.ReasonSpecs(unknown.Blockers)
	assert.Contains(t, unknownSpecs, subagentBlocker)
	assert.NotContains(t, unknownSpecs, unavailableBlocker)
	// Continuable: stays the ship-verification skill_handoff boundary, not a dead-end.
	assert.Equal(t, "skill_handoff:"+progression.SkillShipVerification, unknown.ConfirmationRequirement.Reason)
	assert.Contains(t, unknown.ConfirmationRequirement.NextAction, "Host subagent delegation is a prerequisite")
	assert.Contains(t, unknown.ConfirmationRequirement.NextAction, "same_context_degraded")

	// unavailable: host declared other capabilities but not subagent -> fails closed.
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "none")
	unavailable, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, Command: "run"})
	require.NoError(t, err)
	unavailableCap := requireHostCapabilityForSkill(t, unavailable.HostCapabilities, progression.SkillShipVerification)
	assert.Equal(t, "unavailable", unavailableCap.Availability)
	unavailableSpecs := model.ReasonSpecs(unavailable.Blockers)
	assert.Contains(t, unavailableSpecs, unavailableBlocker)
	assert.NotContains(t, unavailableSpecs, subagentBlocker)
	assert.Equal(t, "blocked_by_governance", unavailable.ConfirmationRequirement.Reason)

	// available: declared subagent -> no new blocker, identical to baseline.
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "subagent")
	available, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, Command: "run"})
	require.NoError(t, err)
	availableCap := requireHostCapabilityForSkill(t, available.HostCapabilities, progression.SkillShipVerification)
	assert.Equal(t, "available", availableCap.Availability)
	availableSpecs := model.ReasonSpecs(available.Blockers)
	assert.NotContains(t, availableSpecs, unavailableBlocker)
	assert.NotContains(t, availableSpecs, subagentBlocker)
	assert.Equal(t, "skill_handoff:"+progression.SkillShipVerification, available.ConfirmationRequirement.Reason)

	// unavailable + named fallback clears the blocker and restores the handoff,
	// without bypassing the gate (fresh ship-verification evidence is still owed).
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "none")
	t.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "manual_ship_verification")
	fallback, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, Command: "run"})
	require.NoError(t, err)
	fallbackCap := requireHostCapabilityForSkill(t, fallback.HostCapabilities, progression.SkillShipVerification)
	assert.True(t, fallbackCap.FallbackSelected)
	assert.Equal(t, "manual_ship_verification", fallbackCap.FallbackMode)
	fallbackSpecs := model.ReasonSpecs(fallback.Blockers)
	assert.NotContains(t, fallbackSpecs, unavailableBlocker)
	assert.NotContains(t, fallbackSpecs, subagentBlocker)
	assert.Equal(t, "skill_handoff:"+progression.SkillShipVerification, fallback.ConfirmationRequirement.Reason)
}

func TestNextS3ShipVerificationProjectsDefaultReadOnlyBoundary(t *testing.T) {
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "ship verification default subagent boundary")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	writeShipReadyGovernedBundle(t, root, change)
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)

	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "subagent")
	t.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "")

	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, Command: "run"})
	require.NoError(t, err)
	require.NotNil(t, view.NextSkill)
	assert.Equal(t, progression.SkillShipVerification, view.NextSkill.Name)
	assertSubagentDirective(
		t,
		view.NextSkill.Subagent,
		model.SubagentTypeNative,
		"",
		"",
		"",
		true,
		"deny",
	)
}

func TestNextDoesNotAutoPassStrictPresetReview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "strict preset review")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.WorkflowPreset = model.WorkflowPresetStrict
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		var buf bytes.Buffer
		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.NextSkill, "advanced=%+v blockers=%v warnings=%v", view.Advanced, model.ReasonSpecs(view.Blockers), view.Warnings)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.NextSkill.Name)
		if view.Advanced != nil {
			assert.Empty(t, view.Advanced.AutoPassedStates)
		}
	})
}

func TestNextJSONShipVerificationHintsDropRetiredFreshEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "ship verification hint contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "intent.md", []byte("# Intent")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "requirements.md", []byte("# Requirements")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "decision.md", []byte("# Decision")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte("- [ ] `t-01` verify\n  - target_files: [\"cmd/next.go\"]\n  - task_kind: verification\n")))
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)

		var buf bytes.Buffer
		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillShipVerification, view.NextSkill.Name)
		require.Len(t, view.NextSkill.TechniqueHints, 1)
		assert.Equal(t, "skill:coverage-analysis", view.NextSkill.TechniqueHints[0].Name)
		for _, hint := range view.NextSkill.TechniqueHints {
			assert.NotEqual(t, "skill:fresh-verification-evidence", hint.Name)
		}

		var handoffBuf bytes.Buffer
		handoffCmd := commandForRoot(t, root, makeNextCmd())
		handoffCmd.SetOut(&handoffBuf)
		handoffCmd.SetArgs([]string{"--json", "--change", slug})
		require.NoError(t, handoffCmd.Execute())

		var handoff nextHandoffView
		require.NoError(t, json.Unmarshal(handoffBuf.Bytes(), &handoff))
		require.NotNil(t, handoff.NextSkill)
		require.Len(t, handoff.NextSkill.TechniqueHints, 1)
		assert.Equal(t, "skill:coverage-analysis", handoff.NextSkill.TechniqueHints[0].Name)
		require.NotNil(t, handoff.NextSkill.SkillConstraints)
		assert.Nil(t, handoff.NextSkill.SkillConstraints.LockedDecisions)
	})
}

func TestRunJSONShipVerificationDropsRetiredFreshEvidenceHint(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "ship verification run hint contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.QualityMode = model.QualityModeFull
		require.NoError(t, state.SaveChange(root, change))

		markChangeReadyForDone(t, root, &change)
		require.NoError(t, os.Remove(state.VerificationFilePath(root, slug, progression.SkillShipVerification)))
		writeAssuranceMD(t, root, slug, validAssuranceContent())
		// Refresh the summary after bundle mutations so ship-verification readiness
		// uses the latest evidence window.
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		var buf bytes.Buffer
		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.NextSkill, "advanced=%+v blockers=%v warnings=%v", view.Advanced, model.ReasonSpecs(view.Blockers), view.Warnings)
		assert.Equal(t, progression.SkillShipVerification, view.NextSkill.Name)
		// ship-verification carries only the coverage-analysis host hint; the
		// retired fresh-verification-evidence hint must not appear.
		require.Len(t, view.NextSkill.TechniqueHints, 1)
		assert.Equal(t, "skill:coverage-analysis", view.NextSkill.TechniqueHints[0].Name)
		for _, hint := range view.NextSkill.TechniqueHints {
			assert.NotEqual(t, "skill:fresh-verification-evidence", hint.Name)
		}
	})
}

func TestAssembleSkillViewShipVerificationDropsRetiredFreshEvidenceHint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "ship verification hint contract")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.QualityMode = model.QualityModeFull
	require.NoError(t, state.SaveChange(root, change))

	view := &nextView{
		CurrentState: change.CurrentState,
		InputContext: nextContext{},
	}
	err = assembleSkillView(
		root,
		view,
		changeRef{Slug: slug},
		progression.AdvanceSummary{Action: "query", FromState: model.StateS3Review},
		&change,
		nil,
		passingSelectedReviewEvidenceForNextSkillTests(1),
		nil,
		true,
	)
	require.NoError(t, err)
	require.NotNil(t, view.NextSkill)
	assert.Equal(t, progression.SkillShipVerification, view.NextSkill.Name)
	require.Len(t, view.NextSkill.TechniqueHints, 1)
	assert.Equal(t, "skill:coverage-analysis", view.NextSkill.TechniqueHints[0].Name)
	for _, hint := range view.NextSkill.TechniqueHints {
		assert.NotEqual(t, "skill:fresh-verification-evidence", hint.Name)
	}
}

func passingSelectedReviewEvidenceForNextSkillTests(runVersion int) map[string]model.VerificationRecord {
	now := time.Now().UTC()
	out := map[string]model.VerificationRecord{}
	for _, skillName := range []string{
		progression.SkillSpecComplianceReview,
		progression.SkillCodeQualityReview,
		progression.SkillIndependentReview,
	} {
		out[skillName] = model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  now,
			RunVersion: runVersion,
		}
	}
	return out
}

func reviewBatchSkillNames(batch *reviewBatchView) []string {
	if batch == nil {
		return nil
	}
	names := make([]string, 0, len(batch.Skills))
	for _, skill := range batch.Skills {
		names = append(names, skill.Name)
	}
	return names
}

func TestWriteNextHumanShowsPlanningSubStepAndRecoveryNote(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeNextHuman(&buf, nextView{
		Slug:            "req-validate",
		CurrentState:    model.StateS1Plan,
		PlanSubStep:     model.PlanSubStepValidate,
		PlanningNote:    "This is a recovery-only planning state entered after post-audit machine validation failed.",
		Phase:           model.PhasePlanning,
		ExecutionMode:   "governed",
		LifecycleStatus: "active",
	})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Change: req-validate (S1_PLAN/validate)")
	assert.Contains(t, buf.String(), "Planning Note: This is a recovery-only planning state entered after post-audit machine validation failed.")
}

func TestWriteNextHumanShowsReviewLayerRequirements(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeNextHuman(&buf, nextView{
		Slug:            "review-layers",
		CurrentState:    model.StateS3Review,
		Phase:           model.PhaseReviewing,
		ExecutionMode:   "governed",
		LifecycleStatus: "active",
		NextSkill: &nextSkillView{
			Name:            progression.SkillCodeQualityReview,
			VerificationDir: "artifacts/changes/review-layers/verification",
			State:           "missing",
			RequiredTokens:  []string{"layer:IR1=pass", "layer:IR3=pass"},
			ReviewContext: &reviewContextView{
				RequiredImplementationLayers: []string{"IR1", "IR3"},
			},
		},
	})
	require.NoError(t, err)
	rendered := buf.String()
	assert.Contains(t, rendered, "Required Tokens: layer:IR1=pass, layer:IR3=pass")
	assert.Contains(t, rendered, "Required Implementation Layers: IR1, IR3")
}

func TestNextReturnsSkillNameWithoutToolSpecificRuntimeFields(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "test agent hint")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Set to plan audit state where plan-audit skill runs
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNil(t, view.NextSkill)
		assert.Equal(t, "plan-audit", view.NextSkill.Name)
	})
}

func TestNextReturnsReviewContextForArtifactReview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "refactor service module")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Set to review state with guardrail domain
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNil(t, view.NextSkill)
		assert.Equal(t, "spec-compliance-review", view.NextSkill.Name)
		assert.ElementsMatch(t, []string{
			progression.SkillSpecComplianceReview,
			progression.SkillCodeQualityReview,
			progression.SkillIndependentReview,
			progression.SkillSecurityReview,
		}, view.NextSkill.SelectedReviewSkills)
		require.NotNil(t, view.NextSkill.ReviewContext)
		assert.Contains(t, view.NextSkill.ReviewContext.RequiredArtifactLayers, "R0")
		assert.Contains(t, view.NextSkill.ReviewContext.RequiredArtifactLayers, "R3")
		assert.Empty(t, view.NextSkill.ReviewContext.RequiredImplementationLayers)
		assert.Contains(t, view.NextSkill.RequiredTokens, "layer:R0=pass")
		assert.Contains(t, view.NextSkill.RequiredTokens, "layer:R3=pass")
		assert.NotContains(t, view.NextSkill.RequiredTokens, "layer:IR1=pass")
		require.NotNil(t, view.ReviewBatch)
		assert.Equal(t, "parallel", view.ReviewBatch.Mode)
		assert.Equal(t, string(model.StateS3Review), view.ReviewBatch.State)
		assert.ElementsMatch(t, []string{
			progression.SkillSpecComplianceReview,
			progression.SkillCodeQualityReview,
			progression.SkillIndependentReview,
			progression.SkillSecurityReview,
		}, reviewBatchSkillNames(view.ReviewBatch))
		assert.Equal(t, "review_batch", view.ConfirmationRequirement.Reason)
		assert.Equal(t, "review_batch", view.ConfirmationRequirement.NextActionKind)
	})
}

func TestReviewRequiredTokensUseArtifactScopedSpecLayers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		change := model.NewChange("manifest-only-review")
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.GuardrailDomain = "external_api_contracts"
		projection := &progression.ArtifactProjection{
			Nodes: []progression.ArtifactProjectionNode{{
				Name:  "change.yaml",
				State: string(model.ArtifactLifecycleDraft),
			}},
		}

		view := nextView{
			Slug:         change.Slug,
			CurrentState: change.CurrentState,
		}
		err := assembleSkillViewWithOptions(
			root,
			&view,
			changeRef{Slug: change.Slug},
			progression.AdvanceSummary{},
			&change,
			nil,
			nil,
			projection,
			assembleSkillViewOptions{
				IncludeReviewContext: true,
				IncludeContextBudget: true,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.NextSkill.Name)
		require.NotNil(t, view.NextSkill.ReviewContext)
		assert.ElementsMatch(t, []string{"R0"}, view.NextSkill.ReviewContext.RequiredArtifactLayers)
		assert.Empty(t, view.NextSkill.ReviewContext.RequiredImplementationLayers)
		assert.ElementsMatch(t, []string{"layer:R0=pass"}, view.NextSkill.RequiredTokens)

		actionable := buildActionableNextSkillView(change, progression.GovernanceReadiness{
			ArtifactProjection: projection,
		})
		require.NotNil(t, actionable)
		assert.Equal(t, progression.SkillSpecComplianceReview, actionable.Name)
		assert.ElementsMatch(t, []string{"layer:R0=pass"}, actionable.RequiredTokens)
	})
}

func TestNextJSONReportsActionableRequiredSkillAfterPassingReviewEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "json evidence status surface")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
		})

		var buf bytes.Buffer
		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, view.NextSkill.Name)
		assert.Empty(t, view.NextSkill.DisplayName)
		assert.Empty(t, view.NextSkill.BlockingName)
		assert.NotContains(t, view.NextSkill.ResolutionReason, "before")
		assert.ElementsMatch(t, []string{
			progression.SkillSpecComplianceReview,
			progression.SkillCodeQualityReview,
			progression.SkillIndependentReview,
		}, view.NextSkill.SelectedReviewSkills)
		assert.Contains(t, view.NextSkill.RequiredTokens, "layer:IR1=pass")
		assert.NotContains(t, view.NextSkill.RequiredTokens, "layer:R0=pass")

		statusBySkill := map[string]skillEvidenceEntry{}
		for _, entry := range view.SkillEvidence {
			statusBySkill[entry.SkillName] = entry
		}
		require.Contains(t, statusBySkill, progression.SkillSpecComplianceReview)
		require.Contains(t, statusBySkill, progression.SkillCodeQualityReview)
		require.Contains(t, statusBySkill, progression.SkillIndependentReview)
		assert.True(t, statusBySkill[progression.SkillSpecComplianceReview].HasEvidence)
		assert.Equal(t, "passing", statusBySkill[progression.SkillSpecComplianceReview].Status)
		assert.Equal(t, model.VerificationVerdictPass, statusBySkill[progression.SkillSpecComplianceReview].Verdict)
		assert.False(t, statusBySkill[progression.SkillCodeQualityReview].HasEvidence)
		assert.Equal(t, "missing", statusBySkill[progression.SkillCodeQualityReview].Status)
		assert.False(t, statusBySkill[progression.SkillIndependentReview].HasEvidence)
		assert.Equal(t, "missing", statusBySkill[progression.SkillIndependentReview].Status)
	})
}

func TestRequiredSkillStaleIsActionableAndSetExtractsSkillNames(t *testing.T) {
	t.Parallel()

	// required_skill_stale is now an actionable required-skill blocker, so the
	// stale skill is routed/surfaced as the next skill.
	assert.True(t, isRequiredSkillBlocker("required_skill_stale"))

	blockers := []model.ReasonCode{
		model.NewReasonCode("required_skill_stale", "plan-audit:assurance.md"),
		model.NewReasonCode("required_skill_stale", "ship-verification:run_version"),
		model.NewReasonCode("required_skill_missing", "wave-orchestration"),
	}
	stale := requiredSkillStaleSet(blockers)
	assert.True(t, stale["plan-audit"], "skill name is the first segment of the detail")
	assert.True(t, stale["ship-verification"])
	assert.NotContains(t, stale, "wave-orchestration", "non-stale blockers are excluded")
}

func TestBuildRequiredSkillEvidenceMarksDigestDriftedSkillStale(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "stale evidence status surface")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		// Precomputed path: plan-audit has a passing verdict but a digest-drift
		// blocker, so the evidence view must report "stale" (not "passing").
		passing := map[string]model.VerificationRecord{
			progression.SkillPlanAudit: {Verdict: model.VerificationVerdictPass},
		}
		stale := map[string]bool{progression.SkillPlanAudit: true}
		evidence, err := buildRequiredSkillEvidence(root, change, model.StateS1Plan, nil, passing, stale, skill.ReviewSkillSelection{})
		require.NoError(t, err)

		byName := map[string]skillEvidenceEntry{}
		for _, e := range evidence {
			byName[e.SkillName] = e
		}
		require.Contains(t, byName, progression.SkillPlanAudit)
		assert.Equal(t, "stale", byName[progression.SkillPlanAudit].Status)
		assert.True(t, byName[progression.SkillPlanAudit].HasEvidence)
	})
}

func TestBuildRequiredSkillEvidenceNonPrecomputedMarksDigestDriftedSkillStale(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "non-precomputed stale evidence status")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		// A passing plan-audit verdict on disk; the non-precomputed path reads it.
		writeSkillVerification(t, root, slug, progression.SkillPlanAudit, model.VerificationRecord{
			Verdict:   model.VerificationVerdictPass,
			Blockers:  []model.ReasonCode{},
			Timestamp: time.Now().UTC(),
		})

		// precomputedPassingSkills == nil forces the non-precomputed path.
		stale := map[string]bool{progression.SkillPlanAudit: true}
		evidence, err := buildRequiredSkillEvidence(root, change, model.StateS1Plan, nil, nil, stale, skill.ReviewSkillSelection{})
		require.NoError(t, err)

		byName := map[string]skillEvidenceEntry{}
		for _, e := range evidence {
			byName[e.SkillName] = e
		}
		require.Contains(t, byName, progression.SkillPlanAudit)
		assert.Equal(t, "stale", byName[progression.SkillPlanAudit].Status, "digest drift is stale even when the recorded verdict passes")
	})
}

func TestReviewStateActionableNextSkillConsistentAcrossCommandSurfaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "consistent review next skill")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.GuardrailDomain = "external_api_contracts"
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
			References: []string{
				"layer:R0=pass",
				"layer:R3=pass",
				"layer:IR1=pass",
				"layer:IR3=pass",
			},
		})
		selectedReviewSkills := []string{
			progression.SkillSpecComplianceReview,
			progression.SkillCodeQualityReview,
			progression.SkillIndependentReview,
			progression.SkillSecurityReview,
		}

		nextCmd := commandForRoot(t, root, makeNextCmd())
		nextCmd.SetArgs([]string{"--json", "--change", slug})
		var nextOut bytes.Buffer
		nextCmd.SetOut(&nextOut)
		require.NoError(t, nextCmd.Execute())
		var handoff nextHandoffView
		require.NoError(t, json.Unmarshal(nextOut.Bytes(), &handoff))
		require.NotNil(t, handoff.NextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, handoff.NextSkill.Name)
		assert.ElementsMatch(t, selectedReviewSkills, handoff.NextSkill.SelectedReviewSkills)
		assert.Empty(t, handoff.NextSkill.DisplayName)
		assert.Empty(t, handoff.NextSkill.BlockingName)
		require.NotNil(t, handoff.NextSkill.ReviewContext)
		assert.Empty(t, handoff.NextSkill.ReviewContext.RequiredArtifactLayers)
		assert.Contains(t, handoff.NextSkill.ReviewContext.RequiredImplementationLayers, "IR1")
		assert.Contains(t, handoff.NextSkill.ReviewContext.RequiredImplementationLayers, "IR3")
		assert.Contains(t, handoff.NextSkill.RequiredTokens, "layer:IR1=pass")
		assert.Contains(t, handoff.NextSkill.RequiredTokens, "layer:IR3=pass")
		assert.NotContains(t, handoff.NextSkill.RequiredTokens, "layer:R0=pass")
		require.NotNil(t, handoff.ReviewBatch)
		assert.Equal(t, "parallel", handoff.ReviewBatch.Mode)
		assert.ElementsMatch(t, []string{
			progression.SkillCodeQualityReview,
			progression.SkillIndependentReview,
			progression.SkillSecurityReview,
		}, reviewBatchSkillNames(handoff.ReviewBatch))

		nextDiagCmd := commandForRoot(t, root, makeNextCmd())
		nextDiagCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var nextDiagOut bytes.Buffer
		nextDiagCmd.SetOut(&nextDiagOut)
		require.NoError(t, nextDiagCmd.Execute())
		var nextDiag nextView
		require.NoError(t, json.Unmarshal(nextDiagOut.Bytes(), &nextDiag))
		require.NotNil(t, nextDiag.NextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, nextDiag.NextSkill.Name)
		assert.ElementsMatch(t, selectedReviewSkills, nextDiag.NextSkill.SelectedReviewSkills)
		assert.Contains(t, nextDiag.NextSkill.RequiredTokens, "layer:IR1=pass")
		require.NotNil(t, nextDiag.ReviewBatch)
		assert.ElementsMatch(t, []string{
			progression.SkillCodeQualityReview,
			progression.SkillIndependentReview,
			progression.SkillSecurityReview,
		}, reviewBatchSkillNames(nextDiag.ReviewBatch))
		assert.Equal(t, "review_batch", nextDiag.ConfirmationRequirement.Reason)
		assert.Equal(t, "review_batch", nextDiag.CurrentActionKind)

		validateCmd := commandForRoot(t, root, makeValidateCmd())
		validateCmd.SetArgs([]string{"--change", slug})
		var validateOut bytes.Buffer
		validateCmd.SetOut(&validateOut)
		require.NoError(t, validateCmd.Execute())
		var validate validateView
		require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validate))
		require.NotNil(t, validate.ActionableNextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, validate.ActionableNextSkill.Name)
		assert.ElementsMatch(t, selectedReviewSkills, validate.ActionableNextSkill.SelectedReviewSkills)
		assert.Empty(t, validate.ActionableNextSkill.DisplayName)
		assert.Empty(t, validate.ActionableNextSkill.BlockingName)
		assert.Contains(t, validate.ActionableNextSkill.RequiredTokens, "layer:IR1=pass")
		assert.Contains(t, validate.ActionableNextSkill.RequiredTokens, "layer:IR3=pass")
		assert.NotContains(t, validate.ActionableNextSkill.RequiredTokens, "layer:R0=pass")
		assert.NotContains(t, validate.ActionableNextSkill.RequiredTokens, "layer:R3=pass")
		assert.Equal(t, "review_batch", validate.CurrentActionKind)
		assert.Equal(t, "slipway run", validate.CurrentActionCommand)
		assert.Equal(t, "fresh", validate.ExecutionEvidenceFreshness)
		assert.Equal(t, "stale", validate.GovernanceEvidenceFreshness)
		assert.Equal(t, "blocked", validate.OverallReadinessFreshness)

		statusCmd := commandForRoot(t, root, makeStatusCmd())
		statusCmd.SetArgs([]string{"--json", "--change", slug})
		var statusOut bytes.Buffer
		statusCmd.SetOut(&statusOut)
		require.NoError(t, statusCmd.Execute())
		var status statusView
		require.NoError(t, json.Unmarshal(statusOut.Bytes(), &status))
		require.NotNil(t, status.ActionableNextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, status.ActionableNextSkill.Name)
		assert.ElementsMatch(t, selectedReviewSkills, status.ActionableNextSkill.SelectedReviewSkills)
		assert.Contains(t, status.ActionableNextSkill.RequiredTokens, "layer:IR1=pass")
		assert.Equal(t, "review_batch", status.CurrentActionKind)
		assert.Equal(t, "slipway run", status.CurrentActionCommand)
		assert.Equal(t, "fresh", status.ExecutionEvidenceFreshness)
		assert.Equal(t, "stale", status.GovernanceEvidenceFreshness)
		assert.Equal(t, "blocked", status.OverallReadinessFreshness)

		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var runOut bytes.Buffer
		runCmd.SetOut(&runOut)
		require.NoError(t, runCmd.Execute())
		var runView nextView
		require.NoError(t, json.Unmarshal(runOut.Bytes(), &runView))
		require.NotNil(t, runView.NextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, runView.NextSkill.Name)
		assert.Empty(t, runView.NextSkill.DisplayName)
		assert.Empty(t, runView.NextSkill.BlockingName)
		assert.ElementsMatch(t, selectedReviewSkills, runView.NextSkill.SelectedReviewSkills)
		assert.Contains(t, runView.NextSkill.RequiredTokens, "layer:IR1=pass")
		require.NotNil(t, runView.ReviewBatch)
		assert.ElementsMatch(t, []string{
			progression.SkillCodeQualityReview,
			progression.SkillIndependentReview,
			progression.SkillSecurityReview,
		}, reviewBatchSkillNames(runView.ReviewBatch))
		assert.Equal(t, "review_batch", runView.ConfirmationRequirement.Reason)
		assert.Equal(t, "review_batch", runView.CurrentActionKind)
	})
}

// TestReviewBatchHostSubagentDelegationSurfacedAcrossCapabilityStates pins the
// host subagent-delegation contract for the S3 review batch across the three
// capability states (#369). "available" is unchanged (see the sibling test).
// "unknown" (the host declared nothing) stays continuable: it keeps the normal
// review_batch handoff boundary instead of escalating to a governance dead-end,
// but rides a subagent_dispatch_authorization_required prerequisite and an
// enriched next_action that names delegation plus the named fallback. "unavailable"
// (the host declared other capabilities but not subagent) fails closed as a
// first-class host_capability_unavailable blocker until an explicit fallback is
// selected. Either way the surface is actionable, never a silent dead-end.
func TestReviewBatchHostSubagentDelegationSurfacedAcrossCapabilityStates(t *testing.T) {
	root, slug := prepareReviewBatchHostCapabilityFixture(t)

	originalCapabilities, hadCapabilities := os.LookupEnv("SLIPWAY_HOST_CAPABILITIES")
	originalFallbacks, hadFallbacks := os.LookupEnv("SLIPWAY_HOST_CAPABILITY_FALLBACKS")
	t.Cleanup(func() {
		if hadCapabilities {
			require.NoError(t, os.Setenv("SLIPWAY_HOST_CAPABILITIES", originalCapabilities))
		} else {
			require.NoError(t, os.Unsetenv("SLIPWAY_HOST_CAPABILITIES"))
		}
		if hadFallbacks {
			require.NoError(t, os.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", originalFallbacks))
		} else {
			require.NoError(t, os.Unsetenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS"))
		}
	})

	const subagentBlocker = "subagent_dispatch_authorization_required:independent-review:subagent"
	const unavailableBlocker = "host_capability_unavailable:independent-review:subagent"

	// --- unknown: host declared nothing -> continuable, riding prerequisite ---
	require.NoError(t, os.Unsetenv("SLIPWAY_HOST_CAPABILITIES"))
	require.NoError(t, os.Unsetenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS"))

	unknownCmd := commandForRoot(t, root, makeNextCmd())
	unknownCmd.SetArgs([]string{"--json", "--change", slug})
	var unknownOut bytes.Buffer
	unknownCmd.SetOut(&unknownOut)
	require.NoError(t, unknownCmd.Execute())
	var unknownHandoff nextHandoffView
	require.NoError(t, json.Unmarshal(unknownOut.Bytes(), &unknownHandoff))
	unknownCapability := requireIndependentReviewHostCapability(t, unknownHandoff.HostCapabilities)
	assert.Equal(t, "unknown", unknownCapability.Availability)
	assert.False(t, unknownCapability.FallbackSelected)
	unknownSpecs := model.ReasonSpecs(unknownHandoff.Blockers)
	assert.Contains(t, unknownSpecs, subagentBlocker)
	assert.NotContains(t, unknownSpecs, unavailableBlocker)
	// Continuable: stays the review_batch boundary, not a blocked_by_governance dead-end.
	assert.Equal(t, "review_batch", unknownHandoff.Confirmation.Reason)
	assert.Equal(t, "review_batch", unknownHandoff.Confirmation.NextActionKind)
	assert.Contains(t, unknownHandoff.Confirmation.NextAction, "Host subagent delegation is a prerequisite")
	assert.Contains(t, unknownHandoff.Confirmation.NextAction, "same_context_degraded")
	assert.Contains(t, unknownHandoff.Confirmation.NextAction, "context_origin:stage=review=<handle>")
	assert.Contains(t, unknownHandoff.Confirmation.NextAction, "fallback:<mode>")
	assert.Equal(t, "blocked", unknownHandoff.OverallReadinessFreshness)
	require.NotNil(t, unknownHandoff.Recovery)
	assert.NotEmpty(t, unknownHandoff.Recovery.Steps)

	// validate surfaces the same prerequisite and stays fail-closed (cannot advance).
	validateCmd := commandForRoot(t, root, makeValidateCmd())
	validateCmd.SetArgs([]string{"--change", slug})
	var validateOut bytes.Buffer
	validateCmd.SetOut(&validateOut)
	require.NoError(t, validateCmd.Execute())
	var validate validateView
	require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validate))
	validateCapability := requireIndependentReviewHostCapability(t, validate.HostCapabilities)
	assert.Equal(t, "unknown", validateCapability.Availability)
	assert.Contains(t, model.ReasonSpecs(validate.Blockers), subagentBlocker)
	assert.False(t, validate.CanAdvance)

	// run shares the same view and never advances past S3.
	runCmd := commandForRoot(t, root, makeRunCmd())
	runCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var runOut bytes.Buffer
	runCmd.SetOut(&runOut)
	require.NoError(t, runCmd.Execute())
	var runView nextView
	require.NoError(t, json.Unmarshal(runOut.Bytes(), &runView))
	runCapability := requireIndependentReviewHostCapability(t, runView.HostCapabilities)
	assert.Equal(t, "unknown", runCapability.Availability)
	assert.Contains(t, model.ReasonSpecs(runView.Blockers), subagentBlocker)
	assert.Equal(t, "review_batch", runView.ConfirmationRequirement.Reason)
	reloadedAfterRun, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS3Review, reloadedAfterRun.CurrentState)

	// --- unavailable: host declared other capabilities but not subagent ---
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "none")

	unavailableCmd := commandForRoot(t, root, makeNextCmd())
	unavailableCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var unavailableOut bytes.Buffer
	unavailableCmd.SetOut(&unavailableOut)
	require.NoError(t, unavailableCmd.Execute())
	var unavailableView nextView
	require.NoError(t, json.Unmarshal(unavailableOut.Bytes(), &unavailableView))
	unavailableCapability := requireIndependentReviewHostCapability(t, unavailableView.HostCapabilities)
	assert.Equal(t, "unavailable", unavailableCapability.Availability)
	assert.False(t, unavailableCapability.FallbackSelected)
	unavailableSpecs := model.ReasonSpecs(unavailableView.Blockers)
	assert.Contains(t, unavailableSpecs, unavailableBlocker)
	assert.NotContains(t, unavailableSpecs, subagentBlocker)
	// First-class blocker: escalates to a governance hard stop.
	assert.Equal(t, "blocked_by_governance", unavailableView.ConfirmationRequirement.Reason)

	// --- unavailable + generic same_context_degraded fallback clears the whole batch ---
	t.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "same_context_degraded")

	fallbackCmd := commandForRoot(t, root, makeNextCmd())
	fallbackCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var fallbackOut bytes.Buffer
	fallbackCmd.SetOut(&fallbackOut)
	require.NoError(t, fallbackCmd.Execute())
	var fallbackView nextView
	require.NoError(t, json.Unmarshal(fallbackOut.Bytes(), &fallbackView))
	fallbackCapability := requireIndependentReviewHostCapability(t, fallbackView.HostCapabilities)
	assert.Equal(t, "unavailable", fallbackCapability.Availability)
	assert.True(t, fallbackCapability.FallbackSelected)
	assert.Equal(t, "same_context_degraded", fallbackCapability.FallbackMode)
	fallbackSpecs := model.ReasonSpecs(fallbackView.Blockers)
	assert.NotContains(t, fallbackSpecs, unavailableBlocker)
	assert.NotContains(t, fallbackSpecs, subagentBlocker)
	assert.Equal(t, "review_batch", fallbackView.ConfirmationRequirement.Reason)
}

// TestReviewBatchSurfacesSubagentDelegationForEveryPendingReviewer pins #369:
// every pending S3 reviewer that REQUIRES a fresh subagent surfaces the
// subagent-delegation prerequisite under the "unknown" host-capability state,
// not just independent-review. spec-compliance-review already has evidence in
// the fixture, so it is not pending and carries no requirement.
func TestReviewBatchSurfacesSubagentDelegationForEveryPendingReviewer(t *testing.T) {
	root, slug := prepareReviewBatchHostCapabilityFixture(t)
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "")
	t.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "")

	view := runNextDiagnostics(t, root, slug)
	specs := model.ReasonSpecs(view.Blockers)
	require.NotNil(t, view.NextSkill)
	assertSubagentDirective(
		t,
		view.NextSkill.Subagent,
		model.SubagentTypeNative,
		"",
		"",
		"",
		true,
		"deny",
	)
	require.NotNil(t, view.ReviewBatch)
	assertSubagentDirective(
		t,
		view.ReviewBatch.Subagent,
		model.SubagentTypeNative,
		"",
		"",
		"",
		true,
		"deny",
	)
	for _, reviewer := range []string{
		progression.SkillCodeQualityReview,
		progression.SkillIndependentReview,
		progression.SkillSecurityReview,
	} {
		capability := requireHostCapabilityForSkill(t, view.HostCapabilities, reviewer)
		assert.Equal(t, "unknown", capability.Availability, reviewer)
		assert.False(t, capability.FallbackSelected, reviewer)
		assert.NotEmpty(t, capability.Remediation, reviewer)
		assert.Contains(t, specs, "subagent_dispatch_authorization_required:"+reviewer+":subagent")
	}
	assert.Equal(t, "review_batch", view.ConfirmationRequirement.Reason)
	assert.Contains(t, view.ConfirmationRequirement.NextAction, "Host subagent delegation is a prerequisite")
}

func TestReviewBatchProjectsConfiguredSubagentDirective(t *testing.T) {
	root, slug := prepareReviewBatchHostCapabilityFixture(t)
	writeTestSubagentConfig(t, root, func(cfg *model.Config) {
		cfg.Subagents.Review = model.SubagentSlot{
			Type:                model.SubagentTypeSkills,
			Name:                "sliphub",
			SessionInstructions: "Run the selected read-only reviewers in parallel and return separate findings.",
			Timeout:             "45m",
		}
	})
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "subagent")
	t.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "")

	view := runNextDiagnostics(t, root, slug)

	require.NotNil(t, view.NextSkill)
	assert.Equal(t, progression.SkillCodeQualityReview, view.NextSkill.Name)
	assertSubagentDirective(
		t,
		view.NextSkill.Subagent,
		model.SubagentTypeSkills,
		"sliphub",
		"Run the selected read-only reviewers in parallel and return separate findings.",
		"45m",
		true,
		"deny",
	)
	require.NotNil(t, view.ReviewBatch)
	assertSubagentDirective(
		t,
		view.ReviewBatch.Subagent,
		model.SubagentTypeSkills,
		"sliphub",
		"Run the selected read-only reviewers in parallel and return separate findings.",
		"45m",
		true,
		"deny",
	)
}

func TestReviewBatchHostCapabilityAvailableDoesNotBlockCommandSurfaces(t *testing.T) {
	root, slug := prepareReviewBatchHostCapabilityFixture(t)
	t.Setenv("SLIPWAY_HOST_CAPABILITIES", "delegation")
	t.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "")

	nextCmd := commandForRoot(t, root, makeNextCmd())
	nextCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var nextOut bytes.Buffer
	nextCmd.SetOut(&nextOut)
	require.NoError(t, nextCmd.Execute())
	var nextOutputView nextView
	require.NoError(t, json.Unmarshal(nextOut.Bytes(), &nextOutputView))
	nextCapability := requireIndependentReviewHostCapability(t, nextOutputView.HostCapabilities)
	assert.Equal(t, "available", nextCapability.Availability)
	assert.False(t, nextCapability.FallbackSelected)
	assert.NotContains(t, model.ReasonSpecs(nextOutputView.Blockers), "host_capability_unavailable:independent-review:subagent")
	assert.Equal(t, "review_batch", nextOutputView.ConfirmationRequirement.Reason)

	validateCmd := commandForRoot(t, root, makeValidateCmd())
	validateCmd.SetArgs([]string{"--change", slug})
	var validateOut bytes.Buffer
	validateCmd.SetOut(&validateOut)
	require.NoError(t, validateCmd.Execute())
	var validate validateView
	require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validate))
	validateCapability := requireIndependentReviewHostCapability(t, validate.HostCapabilities)
	assert.Equal(t, "available", validateCapability.Availability)
	assert.False(t, validateCapability.FallbackSelected)
	assert.NotContains(t, model.ReasonSpecs(validate.Blockers), "host_capability_unavailable:independent-review:subagent")

	runCmd := commandForRoot(t, root, makeRunCmd())
	runCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var runOut bytes.Buffer
	runCmd.SetOut(&runOut)
	require.NoError(t, runCmd.Execute())
	var runView nextView
	require.NoError(t, json.Unmarshal(runOut.Bytes(), &runView))
	runCapability := requireIndependentReviewHostCapability(t, runView.HostCapabilities)
	assert.Equal(t, "available", runCapability.Availability)
	assert.False(t, runCapability.FallbackSelected)
	assert.NotContains(t, model.ReasonSpecs(runView.Blockers), "host_capability_unavailable:independent-review:subagent")
	assert.Equal(t, "review_batch", runView.ConfirmationRequirement.Reason)
}

func prepareReviewBatchHostCapabilityFixture(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "review host capability contract")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = "external_api_contracts"
	require.NoError(t, state.SaveChange(root, change))
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{
			"layer:R0=pass",
			"layer:R3=pass",
			"layer:IR1=pass",
			"layer:IR3=pass",
		},
	})
	return root, slug
}

func requireIndependentReviewHostCapability(t *testing.T, capabilities []hostCapabilityView) hostCapabilityView {
	t.Helper()

	for _, capabilityView := range capabilities {
		if capabilityView.SkillName == progression.SkillIndependentReview {
			assert.Equal(t, "subagent", capabilityView.Capability)
			assert.True(t, capabilityView.Required)
			assert.NotEmpty(t, capabilityView.EvidenceRequirement)
			assert.NotEmpty(t, capabilityView.Remediation)
			return capabilityView
		}
	}
	require.Fail(t, "missing independent-review host capability", "capabilities: %#v", capabilities)
	return hostCapabilityView{}
}

func TestReviewStateDocsProfileSkipsCodeQualityAcrossCommandSurfaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "docs review next skill")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.WorkflowProfile = model.WorkflowProfileDocs
		change.GuardrailDomain = "external_api_contracts"
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
			References: []string{
				"layer:R0=pass",
				"layer:R3=pass",
			},
		})
		selectedReviewSkills := []string{
			progression.SkillSpecComplianceReview,
			progression.SkillIndependentReview,
			progression.SkillSecurityReview,
		}
		pendingReviewSkills := []string{
			progression.SkillIndependentReview,
			progression.SkillSecurityReview,
		}

		nextCmd := commandForRoot(t, root, makeNextCmd())
		nextCmd.SetArgs([]string{"--json", "--change", slug})
		var nextOut bytes.Buffer
		nextCmd.SetOut(&nextOut)
		require.NoError(t, nextCmd.Execute())
		var handoff nextHandoffView
		require.NoError(t, json.Unmarshal(nextOut.Bytes(), &handoff))
		require.NotNil(t, handoff.NextSkill)
		assert.Equal(t, progression.SkillIndependentReview, handoff.NextSkill.Name)
		assert.ElementsMatch(t, selectedReviewSkills, handoff.NextSkill.SelectedReviewSkills)
		assert.NotContains(t, handoff.NextSkill.SelectedReviewSkills, progression.SkillCodeQualityReview)
		require.NotNil(t, handoff.ReviewBatch)
		assert.ElementsMatch(t, pendingReviewSkills, reviewBatchSkillNames(handoff.ReviewBatch))

		nextDiagCmd := commandForRoot(t, root, makeNextCmd())
		nextDiagCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var nextDiagOut bytes.Buffer
		nextDiagCmd.SetOut(&nextDiagOut)
		require.NoError(t, nextDiagCmd.Execute())
		var nextDiag nextView
		require.NoError(t, json.Unmarshal(nextDiagOut.Bytes(), &nextDiag))
		require.NotNil(t, nextDiag.NextSkill)
		assert.Equal(t, progression.SkillIndependentReview, nextDiag.NextSkill.Name)
		assert.ElementsMatch(t, selectedReviewSkills, nextDiag.NextSkill.SelectedReviewSkills)
		require.NotNil(t, nextDiag.ReviewBatch)
		assert.ElementsMatch(t, pendingReviewSkills, reviewBatchSkillNames(nextDiag.ReviewBatch))

		validateCmd := commandForRoot(t, root, makeValidateCmd())
		validateCmd.SetArgs([]string{"--change", slug})
		var validateOut bytes.Buffer
		validateCmd.SetOut(&validateOut)
		require.NoError(t, validateCmd.Execute())
		var validate validateView
		require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validate))
		assert.ElementsMatch(t, selectedReviewSkills, validate.SelectedReviewSkills)
		require.NotNil(t, validate.ActionableNextSkill)
		assert.Equal(t, progression.SkillIndependentReview, validate.ActionableNextSkill.Name)
		assert.ElementsMatch(t, selectedReviewSkills, validate.ActionableNextSkill.SelectedReviewSkills)

		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var runOut bytes.Buffer
		runCmd.SetOut(&runOut)
		require.NoError(t, runCmd.Execute())
		var runView nextView
		require.NoError(t, json.Unmarshal(runOut.Bytes(), &runView))
		require.NotNil(t, runView.NextSkill)
		assert.Equal(t, progression.SkillIndependentReview, runView.NextSkill.Name)
		assert.ElementsMatch(t, selectedReviewSkills, runView.NextSkill.SelectedReviewSkills)
		require.NotNil(t, runView.ReviewBatch)
		assert.ElementsMatch(t, pendingReviewSkills, reviewBatchSkillNames(runView.ReviewBatch))
	})
}

func TestRunJSONRoutesToShipVerificationAfterReviewSetWithoutDisplayPromotion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("ship-verification-terminal")
	change.QualityMode = model.QualityModeStandard
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone

	view := nextView{
		Slug:         change.Slug,
		CurrentState: model.StateS3Review,
		InputContext: nextContext{WorkspaceRoot: root},
	}
	err := assembleSkillViewWithOptions(
		root,
		&view,
		changeRef{Slug: change.Slug},
		progression.AdvanceSummary{Action: "blocked", FromState: model.StateS3Review, Blockers: []model.ReasonCode{model.NewReasonCode("ship_gate_blocked", "assurance.md")}},
		&change,
		nil,
		passingSelectedReviewEvidenceForNextSkillTests(1),
		nil,
		assembleSkillViewOptions{
			IncludeReviewContext: true,
			IncludeContextBudget: true,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, view.NextSkill)
	// The merge collapsed the goal->closeout two-skill display promotion into the
	// single terminal ship-verification skill; there is no DisplayName promotion.
	assert.Equal(t, progression.SkillShipVerification, view.NextSkill.Name)
	assert.Empty(t, view.NextSkill.DisplayName)
	assert.Empty(t, view.NextSkill.BlockingName)
}

func TestDiagnosticCommandsExposePathAuthorityWhenFreshnessUnknown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "path authority should not depend on execution freshness")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		require.NoError(t, state.SaveChange(root, change))

		nextCmd := commandForRoot(t, root, makeNextCmd())
		nextCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var nextOut bytes.Buffer
		nextCmd.SetOut(&nextOut)
		require.NoError(t, nextCmd.Execute())
		var nextDiag nextView
		require.NoError(t, json.Unmarshal(nextOut.Bytes(), &nextDiag))
		require.NotNil(t, nextDiag.FreshnessDiagnostics)
		assert.Equal(t, "unknown", nextDiag.FreshnessDiagnostics.Status)
		require.NotNil(t, nextDiag.FreshnessDiagnostics.PathAuthority)
		runtimePath := nextDiag.FreshnessDiagnostics.PathAuthority.RuntimeEvidencePath
		assert.True(t, filepath.IsAbs(runtimePath))
		assert.True(t, strings.HasSuffix(runtimePath, "/.git/slipway/runtime/changes/"+slug), runtimePath)

		validateCmd := commandForRoot(t, root, makeValidateCmd())
		validateCmd.SetArgs([]string{"--change", slug})
		var validateOut bytes.Buffer
		validateCmd.SetOut(&validateOut)
		require.NoError(t, validateCmd.Execute())
		var validate validateView
		require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validate))
		require.NotNil(t, validate.FreshnessDiagnostics)
		assert.Equal(t, "unknown", validate.FreshnessDiagnostics.Status)
		require.NotNil(t, validate.FreshnessDiagnostics.PathAuthority)

		statusCmd := commandForRoot(t, root, makeStatusCmd())
		statusCmd.SetArgs([]string{"--json", "--change", slug})
		var statusOut bytes.Buffer
		statusCmd.SetOut(&statusOut)
		require.NoError(t, statusCmd.Execute())
		var status statusView
		require.NoError(t, json.Unmarshal(statusOut.Bytes(), &status))
		require.NotNil(t, status.FreshnessDiagnostics)
		assert.Equal(t, "unknown", status.FreshnessDiagnostics.Status)
		require.NotNil(t, status.FreshnessDiagnostics.PathAuthority)
	})
}

func TestNextNoReviewContextForNonReviewState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "add pagination")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// At plan audit state — no review context
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNil(t, view.NextSkill)
		assert.Equal(t, "plan-audit", view.NextSkill.Name)
		assert.Nil(t, view.NextSkill.ReviewContext)
	})
}

func TestAssembleSkillViewReusesPrecomputedEvidenceMap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "reuse precomputed next evidence")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	verificationDir := state.VerificationDir(root, slug)
	require.NoError(t, os.MkdirAll(verificationDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(verificationDir, "broken.yaml"),
		[]byte("not valid yaml: [[["),
		0o644,
	))

	view := &nextView{
		CurrentState: change.CurrentState,
		InputContext: nextContext{},
	}
	err = assembleSkillView(
		root,
		view,
		changeRef{Slug: slug},
		progression.AdvanceSummary{},
		&change,
		nil,
		map[string]model.VerificationRecord{
			progression.SkillSpecComplianceReview: {
				Verdict:   model.VerificationVerdictPass,
				Blockers:  []model.ReasonCode{},
				Timestamp: time.Now().UTC(),
			},
		},
		nil,
		true,
	)
	require.NoError(t, err)
	require.NotNil(t, view.NextSkill)
	assert.Equal(t, progression.SkillCodeQualityReview, view.NextSkill.Name)
}

func TestNextBlocksWithoutPlanAuditEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "test gplan blocking")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	// Move to S1_PLAN/audit without evidence
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	// Advancement blocking is tested via buildNextView (run path).
	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
	require.NoError(t, err)

	require.NotNil(t, view.Advanced)
	assert.Equal(t, "blocked", view.Advanced.Action)
	assert.Equal(t, model.StateS1Plan, view.Advanced.FromState)
	assert.False(t, view.Advanced.RecoveryOnly)
	assert.Equal(t, model.StateS1Plan, view.CurrentState)
	assert.NotEmpty(t, view.Blockers)
}

func TestShouldExposeAdvancedSummaryToCaller(t *testing.T) {
	t.Parallel()

	assert.True(t, shouldExposeAdvancedSummaryToCaller(progression.AdvanceSummary{
		Action: "query",
	}))
	assert.True(t, shouldExposeAdvancedSummaryToCaller(progression.AdvanceSummary{
		Action:    "blocked",
		FromState: model.StateS1Plan,
	}))
	assert.True(t, shouldExposeAdvancedSummaryToCaller(progression.AdvanceSummary{
		Action:       "blocked",
		FromState:    model.StateS1Plan,
		ToSubStep:    string(model.PlanSubStepValidate),
		RecoveryOnly: true,
	}))
	assert.True(t, shouldExposeAdvancedSummaryToCaller(progression.AdvanceSummary{
		Action:    "advanced",
		FromState: model.StateS1Plan,
		ToState:   model.StateS2Implement,
	}))
}

func TestNextPreviewFailsWhenSkillEvidenceEvaluationFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review malformed skill registry handling")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		skillPath := filepath.Join(root, ".codex", "skills", "slipway-code-quality-review", "SKILL.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
		require.NoError(t, os.WriteFile(skillPath, []byte(strings.TrimSpace(`
---
name: code-quality-review
description: [
---
`)), 0o644))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "skill_registry_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
		assert.Equal(t, exitCodeStateIntegrity, cliErr.ExitCode)
		assert.Contains(t, err.Error(), "evaluate next skill evidence")
		assert.Contains(t, err.Error(), "parse skill frontmatter")
	})
}

func TestNextAdvancesWithPlanAuditEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "test gplan passing")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	// Create required L2 artifacts
	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte("# Proposal"), 0o644))
	require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "requirements.md", []byte(`# Requirements

### Requirement: Plan Audit
REQ-001: The plan audit path MUST advance only when the task checklist is valid.

#### Scenario: Advance only on a valid checklist
GIVEN a governed change with passing plan-audit evidence and a valid task checklist
WHEN next evaluates planning readiness
THEN the change advances to S2_IMPLEMENT.
`)))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`
- [ ] `+"`t-01`"+` implement plan audit checks
  - target_files: ["internal/engine/example.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))

	// Write plan-audit evidence with correct planning input hash.
	writeSkillVerification(t, root, slug, "plan-audit", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		References: planAuditOriginReferences(),
	})

	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
	require.NoError(t, err)

	require.NotNil(t, view.Advanced)
	assert.Equal(t, "advanced", view.Advanced.Action)
	assert.Equal(t, model.StateS1Plan, view.Advanced.FromState)
	// Audit clean path: post-audit machine validation runs inline.
	// If validation passes, it advances to S2_IMPLEMENT.
	// If it fails, it persists at S1_PLAN/validate.
	// This test provides sufficient artifacts for the clean path.
	assert.Equal(t, model.StateS2Implement, view.Advanced.ToState)
	assert.Equal(t, model.StateS2Implement, view.CurrentState)
}

func TestNextReadOnlyReportsRunGuidanceAfterPassingPlanAudit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "report S1 audit advance guidance")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))
	writeShipReadyGovernedBundle(t, root, change)
	writeSkillVerification(t, root, slug, "plan-audit", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		References: planAuditOriginReferences(),
	})

	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, AutoSkipEvidence: true, Command: "run"})
	require.NoError(t, err)

	assert.Nil(t, view.NextSkill)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "run_slipway_run_to_advance:S1_PLAN")
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "no_skill_required:S1_PLAN")
	assert.Equal(t, "run_slipway_run_to_advance", view.ConfirmationRequirement.Reason)
	assert.Equal(t, "slipway run", view.ConfirmationRequirement.NextCommand)

	reloaded, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS1Plan, reloaded.CurrentState)
	assert.Equal(t, model.PlanSubStepAudit, reloaded.PlanSubStep)
}

func TestNextBlocksWhenBundleMissingArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "test bundle missing")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	// Remove scaffolded artifact files but keep change.yaml (bundle
	// authority). LoadChange is fail-closed — removing change.yaml
	// would trigger a different error path.
	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	entries, _ := os.ReadDir(bundlePath)
	for _, e := range entries {
		if e.Name() != "change.yaml" {
			_ = os.RemoveAll(filepath.Join(bundlePath, e.Name()))
		}
	}

	writeSkillVerification(t, root, slug, "plan-audit", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	})

	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
	require.NoError(t, err)

	// Bundle precondition blocks before the audit->validate recovery path.
	require.NotNil(t, view.Advanced)
	assert.Equal(t, "blocked", view.Advanced.Action)
	assert.Empty(t, view.Advanced.FromSubStep)
	assert.Empty(t, view.Advanced.ToSubStep)
	assert.False(t, view.Advanced.RecoveryOnly)
	assert.Empty(t, view.Advanced.Reason)
	assert.NotEmpty(t, view.Blockers)
}

func TestNextBlocksOnInvalidBoundWorktreeBeforeBundleChecks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	initGitRepoForWorktreeTests(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "bundle invalid bound worktree")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	change.WorktreePath = root
	change.WorktreeBranch = currentGitBranch(t, root)
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
	require.NoError(t, err)

	require.NotNil(t, view.Advanced)
	assert.Equal(t, "blocked", view.Advanced.Action)
	assert.Nil(t, view.NextSkill)
	requireBlockerContains(t, view.Blockers, state.WorktreeReasonDedicatedRequired)
}

func TestNextBlocksWhenTasksChecklistMissingTargetFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "tasks checklist validation")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte("# Proposal"), 0o644))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "requirements.md", []byte("# Spec")))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "decision.md"), []byte("# Design"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "assurance.md"), []byte("# Assurance"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`
- [ ] `+"`t-01`"+` do something
`), 0o644))

		writeSkillVerification(t, root, slug, "plan-audit", model.VerificationRecord{
			Verdict:   model.VerificationVerdictPass,
			Blockers:  []model.ReasonCode{},
			Timestamp: time.Now().UTC(),
		})

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.StateS1Plan, view.CurrentState)
		joined := strings.Join(model.ReasonSpecs(view.Blockers), "\n")
		// target_files is still a hard blocker.
		assert.Contains(t, joined, "plan_dimension_key_links_missing_target_files:t-01")
		warnings := strings.Join(view.Warnings, "\n")
		assert.Contains(t, warnings, "plan_dimension_context_missing_task_kind_warning:t-01")
	})
}

func TestNextBlocksWhenTasksChecklistHasDependencyCycle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "tasks checklist dependency cycle")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte("# Proposal"), 0o644))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "requirements.md", []byte("# Spec")))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "decision.md"), []byte("# Design"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "assurance.md"), []byte("# Assurance"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`
- [ ] `+"`t-01`"+` first task
  - task_kind: code
  - target_files: ["internal/a.go"]
  - depends_on: ["t-02"]

- [ ] `+"`t-02`"+` second task
  - task_kind: code
  - target_files: ["internal/b.go"]
  - depends_on: ["t-01"]
`), 0o644))

		writeSkillVerification(t, root, slug, "plan-audit", model.VerificationRecord{
			Verdict:   model.VerificationVerdictPass,
			Blockers:  []model.ReasonCode{},
			Timestamp: time.Now().UTC(),
		})

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.StateS1Plan, view.CurrentState)
		assert.Contains(t, strings.Join(model.ReasonSpecs(view.Blockers), "\n"), "plan_dimension_dependency_cycle_detected")
	})
}

func TestNextPreviewIncludesTaskChecklistBlockers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "preview tasks checklist blockers")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.WorkflowPreset = model.WorkflowPresetLight
		change.ArtifactSchema = model.ArtifactSchemaCore
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte("# Intent"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "requirements.md"), []byte(`## Requirements

### Requirement: Auth
REQ-001: The system must authenticate requests.
`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` implement auth flow
  - depends_on: [t-99]
  - target_files: [cmd/next.go]
`), 0o644))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "plan_dimension_dependency_unknown:t-01->t-99")
	})
}

func TestNextPreviewIncludesAssuranceContractBlockersAtReview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "preview assurance contract blocker")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeAssuranceMD(t, root, slug, "## Scope Summary\nIncomplete\n")

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Contains(t, strings.Join(model.ReasonSpecs(view.Blockers), "\n"), "assurance_structure_invalid:")
	})
}

func TestNextReturnsDoneReadyWithoutNextSkillAfterGovernedShipPasses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "done ready contract")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.WorkflowPreset = model.WorkflowPresetLight
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	writeShipReadyGovernedBundle(t, root, change)
	writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
	// Refresh the execution summary after mutating bundle artifacts so the
	// evidence timestamp remains newer than the governed tasks plan.
	writePassingExecutionSummary(t, root, slug, 1, "t-01")

	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)
	writePassingShipVerificationEvidence(t, root, slug, 1)

	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
	require.NoError(t, err)

	require.NotNil(t, view.Advanced)
	assert.Equal(t, "done_ready", view.Advanced.Action)
	assert.Equal(t, model.StateS3Review, view.CurrentState)
	assert.Nil(t, view.NextSkill)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "run_slipway_done_to_finalize")
	// ship-verification is the single always-required terminal gate; once it
	// passes there is no optional-closeout advisory to surface.
	assert.NotContains(t, strings.Join(view.Warnings, "\n"), "optional_closeout_available")

	cmd := commandForRoot(t, root, makeNextCmd())
	cmd.SetArgs([]string{"--json", "--change", slug})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())
	var handoff nextHandoffView
	require.NoError(t, json.Unmarshal(out.Bytes(), &handoff))
	assert.Nil(t, handoff.NextSkill)
	assert.Contains(t, model.ReasonSpecs(handoff.Blockers), "run_slipway_done_to_finalize")
	assert.NotContains(t, model.ReasonSpecs(handoff.Blockers), "no_skill_required:S3_REVIEW")
}

func TestNextReturnsDoneReadyWithShipVerificationAttestationForStandardRequestPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "standard request done ready contract")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.QualityMode = model.QualityModeStandard
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	writeShipReadyGovernedBundle(t, root, change)
	writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
	writePassingExecutionSummary(t, root, slug, 1, "t-01")

	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)
	writePassingShipVerificationEvidence(t, root, slug, 1)

	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
	require.NoError(t, err)

	require.NotNil(t, view.Advanced)
	assert.Equal(t, "done_ready", view.Advanced.Action)
	assert.Equal(t, model.StateS3Review, view.CurrentState)
	assert.Nil(t, view.NextSkill)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "run_slipway_done_to_finalize")
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "ship_gate_blocked:required_skill_missing:ship-verification")
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "ship_verification_assurance_attestation_missing")
}

func TestNextDiagnosticsSkillEvidenceRoutesToShipVerificationAfterReviewSet(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "standard ship-verification diagnostics contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.WorkflowPreset = model.WorkflowPresetStandard
		change.QualityMode = model.QualityModeStandard
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		writeShipReadyGovernedBundle(t, root, change)
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		// Deliberately omit ship-verification so it is the next actionable skill.

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill, "advanced=%+v blockers=%v warnings=%v", view.Advanced, model.ReasonSpecs(view.Blockers), view.Warnings)
		assert.Equal(t, progression.SkillShipVerification, view.NextSkill.Name)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:ship-verification")

		statusBySkill := map[string]skillEvidenceEntry{}
		for _, entry := range view.SkillEvidence {
			statusBySkill[entry.SkillName] = entry
		}
		require.Contains(t, statusBySkill, progression.SkillSpecComplianceReview)
		require.Contains(t, statusBySkill, progression.SkillShipVerification)
		assert.True(t, statusBySkill[progression.SkillSpecComplianceReview].HasEvidence)
		assert.Equal(t, "passing", statusBySkill[progression.SkillSpecComplianceReview].Status)
		assert.False(t, statusBySkill[progression.SkillShipVerification].HasEvidence)
		assert.Equal(t, "missing", statusBySkill[progression.SkillShipVerification].Status)
	})
}

func TestCommandDiagnosticsSkillEvidenceRespectsAutoSkipEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "command diagnostics skill evidence boundary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.WorkflowPreset = model.WorkflowPresetStandard
		change.QualityMode = model.QualityModeStandard
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writeShipReadyGovernedBundle(t, root, change)
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)

		nextCmd := commandForRoot(t, root, makeNextCmd())
		nextCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var nextOut bytes.Buffer
		nextCmd.SetOut(&nextOut)
		require.NoError(t, nextCmd.Execute())

		var nextRaw map[string]any
		require.NoError(t, json.Unmarshal(nextOut.Bytes(), &nextRaw))
		assert.Contains(t, nextRaw, "skill_evidence")

		var nextDiagnostics nextView
		require.NoError(t, json.Unmarshal(nextOut.Bytes(), &nextDiagnostics))
		assert.NotEmpty(t, nextDiagnostics.SkillEvidence)

		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var runOut bytes.Buffer
		runCmd.SetOut(&runOut)
		require.NoError(t, runCmd.Execute())

		var runRaw map[string]any
		require.NoError(t, json.Unmarshal(runOut.Bytes(), &runRaw))
		assert.NotContains(t, runRaw, "skill_evidence")

		stageSlug, _ := createEvidenceTaskFixture(t, root)
		implementCmd := commandForRoot(t, root, makeImplementCmd())
		implementCmd.SetArgs([]string{"--json", "--diagnostics", "--change", stageSlug})
		var implementOut bytes.Buffer
		implementCmd.SetOut(&implementOut)
		require.NoError(t, implementCmd.Execute())

		var implementRaw map[string]any
		require.NoError(t, json.Unmarshal(implementOut.Bytes(), &implementRaw))
		assert.NotContains(t, implementRaw, "skill_evidence")

		var implementView nextView
		require.NoError(t, json.Unmarshal(implementOut.Bytes(), &implementView))
		require.NotNil(t, implementView.NextSkill)
		assert.Equal(t, progression.SkillWaveOrchestration, implementView.NextSkill.Name)
	})
}

func TestNextJSONDefaultIsHandoffOnlyAndDiagnosticsKeepsFullSurface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff-only done ready contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		writeShipReadyGovernedBundle(t, root, change)
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingShipVerificationEvidence(t, root, slug, 1)

		diagnosticsCmd := commandForRoot(t, root, makeNextCmd())
		diagnosticsCmd.SetArgs([]string{"--json", "--diagnostics"})
		var diagnosticsBuf bytes.Buffer
		diagnosticsCmd.SetOut(&diagnosticsBuf)
		require.NoError(t, diagnosticsCmd.Execute())

		var diagnosticsView nextView
		require.NoError(t, json.Unmarshal(diagnosticsBuf.Bytes(), &diagnosticsView))

		handoffCmd := commandForRoot(t, root, makeNextCmd())
		handoffCmd.SetArgs([]string{"--json"})
		var handoffBuf bytes.Buffer
		handoffCmd.SetOut(&handoffBuf)
		require.NoError(t, handoffCmd.Execute())

		var handoffView nextHandoffView
		require.NoError(t, json.Unmarshal(handoffBuf.Bytes(), &handoffView))
		var raw map[string]any
		require.NoError(t, json.Unmarshal(handoffBuf.Bytes(), &raw))

		assert.Equal(t, diagnosticsView.CurrentState, handoffView.CurrentState)
		assert.Equal(t, diagnosticsView.Blockers, handoffView.Blockers)
		if diagnosticsView.NextSkill == nil {
			assert.Nil(t, handoffView.NextSkill)
		} else {
			require.NotNil(t, handoffView.NextSkill)
			assert.Equal(t, diagnosticsView.NextSkill.Name, handoffView.NextSkill.Name)
			assert.Equal(t, diagnosticsView.NextSkill.State, handoffView.NextSkill.State)
		}
		assert.Equal(t, diagnosticsView.InputContext.WorkspaceRoot, handoffView.InputContext.WorkspaceRoot)
		assert.NotContains(t, raw, "context_budget")
		assert.NotContains(t, raw, "constraints")
		assert.NotContains(t, raw, "governance_signals")
		assert.NotContains(t, raw, "active_controls")
		assert.NotContains(t, raw, "required_actions")
		assert.NotContains(t, raw, "freshness_diagnostics")
		input, ok := raw["input_context"].(map[string]any)
		require.True(t, ok)
		assert.NotContains(t, input, "handoff_context")
		assert.NotContains(t, input, "gate_status")
		assert.NotContains(t, input, "artifact_status")
	})
}

func TestNextJSONDefaultOmitsFreshnessDiagnosticsWhenDiagnosticsViewHasThem(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff suppresses freshness diagnostics")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundleDir, err := state.GovernedBundleDir(root, change)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(bundleDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [x] `+"`t-01`"+` handoff suppresses freshness diagnostics
  - target_files: ["cmd/next.go"]
  - task_kind: code
`), 0o644))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)

		requirementsPath := filepath.Join(bundleDir, "requirements.md")
		require.NoError(t, os.WriteFile(requirementsPath, []byte("# Requirements\n\nREQ-001: changed after execution.\n"), 0o644))
		updatedAt := time.Now().UTC().Add(time.Minute)
		require.NoError(t, os.Chtimes(requirementsPath, updatedAt, updatedAt))

		diagnosticsCmd := commandForRoot(t, root, makeNextCmd())
		diagnosticsCmd.SetArgs([]string{"--json", "--diagnostics"})
		var diagnosticsBuf bytes.Buffer
		diagnosticsCmd.SetOut(&diagnosticsBuf)
		require.NoError(t, diagnosticsCmd.Execute())

		var diagnosticsRaw map[string]any
		require.NoError(t, json.Unmarshal(diagnosticsBuf.Bytes(), &diagnosticsRaw))
		assert.Contains(t, diagnosticsRaw, "freshness_diagnostics")

		handoffCmd := commandForRoot(t, root, makeNextCmd())
		handoffCmd.SetArgs([]string{"--json"})
		var handoffBuf bytes.Buffer
		handoffCmd.SetOut(&handoffBuf)
		require.NoError(t, handoffCmd.Execute())

		var handoffRaw map[string]any
		require.NoError(t, json.Unmarshal(handoffBuf.Bytes(), &handoffRaw))
		assert.NotContains(t, handoffRaw, "freshness_diagnostics")
	})
}

func TestNextJSONHandoffDoesNotBuildDiagnosticSurfaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff source stays narrow")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var raw map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &raw))
		assert.NotContains(t, raw, "freshness_diagnostics")
		assert.NotContains(t, raw, "constraints")
		assert.NotContains(t, raw, "governance_signals")
		assert.NotContains(t, raw, "active_controls")
		assert.NotContains(t, raw, "required_actions")
		assert.NotContains(t, raw, "skill_evidence")
		assert.NotContains(t, raw, "artifact_amendments")
		inputRaw, ok := raw["input_context"].(map[string]any)
		require.True(t, ok)
		assert.NotContains(t, inputRaw, "handoff_context")
		assert.NotContains(t, inputRaw, "gate_status")
		assert.NotContains(t, inputRaw, "artifact_status")
		assert.NotContains(t, inputRaw, "policy_packs")
		assert.NotContains(t, inputRaw, "read_refs")

		var handoff nextHandoffView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &handoff))
		assert.Equal(t, "next", handoff.Command)
		assert.Equal(t, slug, handoff.Slug)
		assert.Equal(t, model.StateS3Review, handoff.CurrentState)
		assert.Equal(t, governedExecutionMode, handoff.ExecutionMode)
		assert.Nil(t, handoff.ContextBudget)
		require.NotNil(t, handoff.NextSkill)
		assert.Equal(t, progression.SkillSpecComplianceReview, handoff.NextSkill.Name)
		assert.NotNil(t, handoff.NextSkill.SkillConstraints)
		assert.NotEmpty(t, handoff.NextSkill.TechniqueHints)
		require.NotNil(t, handoff.NextSkill.ReviewContext)
		assert.Contains(t, handoff.NextSkill.ReviewContext.RequiredArtifactLayers, "R0")
		assert.Empty(t, handoff.NextSkill.ReviewContext.RequiredImplementationLayers)
		assert.NotEmpty(t, handoff.InputContext.WorkspaceRoot)
		assert.NotEmpty(t, handoff.InputContext.ArtifactBundle)
		assert.Nil(t, handoff.InputContext.WavePlan)
	})
}

func TestNextHandoffViewOmitsHealthyBudget(t *testing.T) {
	t.Parallel()

	view := nextView{
		Slug:            "budget-ok",
		Phase:           model.PhasePlanning,
		ExecutionMode:   governedExecutionMode,
		CurrentState:    model.StateS1Plan,
		LifecycleStatus: string(model.ChangeStatusActive),
		InputContext: nextContext{
			WorkspaceRoot:  "/repo",
			ArtifactBundle: "artifacts/changes/budget-ok",
		},
		ContextBudget: &contextBudget{
			GuardAction:      "ok",
			RemainingPercent: 80.0,
			Breakdown: contextBudgetBreakdown{
				SkillPrompt:     1,
				ArtifactContext: 2,
				StateContext:    3,
			},
		},
		Blockers:                []model.ReasonCode{},
		ConfirmationRequirement: confirmationNoBoundary("no_confirmation_boundary"),
	}

	handoff := buildNextHandoffView(view)
	assert.Nil(t, handoff.ContextBudget)
	raw, err := json.Marshal(handoff)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "context_budget")
	assert.NotContains(t, string(raw), "breakdown")
}

func TestConfirmationRequirementDistinguishesHardStopFromCommandBoundary(t *testing.T) {
	t.Parallel()

	handoff := deriveConfirmationRequirement(nextView{
		NextSkill: &nextSkillView{Name: progression.SkillCodeQualityReview},
	})
	assert.True(t, handoff.Required)
	assert.Equal(t, "hard_stop", handoff.Boundary)
	assert.True(t, handoff.FreshConfirmationRequired)
	assert.False(t, handoff.PriorAuthorizationSufficient)
	assert.Equal(t, "skill_handoff:code-quality-review", handoff.Reason)
	assert.Equal(t, "run governance skill code-quality-review and record evidence", handoff.NextAction)
	assert.Equal(t, "skill_handoff", handoff.NextActionKind)
	assert.Empty(t, handoff.NextCommand)

	doneReady := deriveConfirmationRequirement(nextView{
		Blockers: []model.ReasonCode{model.NewReasonCode("run_slipway_done_to_finalize", "")},
	})
	assert.False(t, doneReady.Required)
	assert.Equal(t, "command_required", doneReady.Boundary)
	assert.False(t, doneReady.FreshConfirmationRequired)
	assert.True(t, doneReady.PriorAuthorizationSufficient)
	assert.Equal(t, "run_slipway_done_to_finalize", doneReady.Reason)
	assert.Equal(t, "run slipway done to finalize", doneReady.NextAction)
	assert.Equal(t, "command", doneReady.NextActionKind)
	assert.Equal(t, "slipway done", doneReady.NextCommand)
}

func TestNoSkillRequiredDiagnosticsUseAdvanceCommandBoundary(t *testing.T) {
	t.Parallel()

	t.Run("S1 injects advance recovery", func(t *testing.T) {
		change := model.NewChange("ready-no-skill")
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepValidate
		view := nextView{
			CurrentState: model.StateS1Plan,
			Blockers: []model.ReasonCode{
				model.NewReasonCode("no_skill_required", string(model.StateS1Plan)),
			},
		}

		applyReadyAdvanceDiagnostics(t.TempDir(), &change, &view)
		view.ConfirmationRequirement = deriveConfirmationRequirement(view)
		view.Recovery = model.BuildRecovery(view.Blockers)

		assert.Contains(t, model.ReasonSpecs(view.Blockers), "run_slipway_run_to_advance:S1_PLAN")
		assert.Equal(t, "run_slipway_run_to_advance", view.ConfirmationRequirement.Reason)
		assert.Equal(t, "run slipway run to advance", view.ConfirmationRequirement.NextAction)
		assert.Equal(t, "command", view.ConfirmationRequirement.NextActionKind)
		assert.Equal(t, "slipway run", view.ConfirmationRequirement.NextCommand)
		require.NotNil(t, view.Recovery)
		assert.Equal(t, "slipway run", view.Recovery.PrimaryCommand)
		assert.NotContains(t, view.ConfirmationRequirement.Reason, "blocked_by_governance")
		assert.NotContains(t, view.ConfirmationRequirement.NextAction, "resolve governance blockers")
	})

	t.Run("S2 no skill info stays command boundary", func(t *testing.T) {
		view := nextView{
			CurrentState: model.StateS2Implement,
			Blockers: []model.ReasonCode{
				model.NewReasonCode("no_skill_required", string(model.StateS2Implement)),
			},
		}

		req := deriveConfirmationRequirement(view)

		assert.Equal(t, "run_slipway_run_to_advance", req.Reason)
		assert.Equal(t, "run slipway run to advance", req.NextAction)
		assert.Equal(t, "command", req.NextActionKind)
		assert.Equal(t, "slipway run", req.NextCommand)
		assert.NotContains(t, req.Reason, "blocked_by_governance")
		assert.NotContains(t, req.NextAction, "resolve governance blockers")
	})
}

// TestNextDiagnosticsReadyS2ImplementSurfacesAdvanceNotGovernanceBlock is the
// REQ-003 end-to-end command contract: a governed change sitting at a ready
// S2_IMPLEMENT with passing fresh wave-orchestration evidence (no skill required)
// must surface an advance handoff through the real `next --json --diagnostics`
// surface, never the misleading blocked_by_governance dead-end. It exercises the
// full command rather than the deriveConfirmationRequirement helper directly.
func TestNextDiagnosticsReadyS2ImplementSurfacesAdvanceNotGovernanceBlock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		// Reach a genuinely-ready S2 implement state: a materialized single-task wave plan,
		// runtime task evidence, and a passing wave-orchestration verification
		// recorded through the public `slipway evidence skill` surface.
		slug, _ := createEvidenceTaskFixture(t, root)
		capturedAt := time.Now().UTC().Add(-time.Minute)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "go test ./cmd -run TestNextDiagnosticsReadyS2ImplementSurfacesAdvanceNotGovernanceBlock",
			"captured_at":   capturedAt.Format(time.RFC3339Nano),
		})
		evidenceCmd := commandForRoot(t, root, makeEvidenceCmd())
		evidenceCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "wave-orchestration:pass",
			"--notes", "Wave orchestration passed.",
		})
		var evidenceOut bytes.Buffer
		evidenceCmd.SetOut(&evidenceOut)
		require.NoError(t, evidenceCmd.Execute())

		// Run the real command surface under test.
		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		// The query-first surface stays read-only at a ready S2 implement boundary.
		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Implement, reloaded.CurrentState, "next --json --diagnostics must stay read-only")

		// The decoded JSON must not mention the governance-block dead-end anywhere.
		raw := buf.String()
		assert.NotContains(t, raw, "blocked_by_governance",
			"a ready no-skill S2 must not surface blocked_by_governance")
		assert.NotContains(t, raw, "resolve governance blockers",
			"a ready no-skill S2 must not tell the operator to resolve governance blockers")

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.StateS2Implement, view.CurrentState)
		assert.Nil(t, view.NextSkill, "a ready S2 advance state requires no skill handoff")

		// The confirmation surface routes the operator to advance with `slipway run`.
		assert.Equal(t, "run_slipway_run_to_advance", view.ConfirmationRequirement.Reason)
		assert.Equal(t, "slipway run", view.ConfirmationRequirement.NextCommand)
		assert.Equal(t, "command", view.ConfirmationRequirement.NextActionKind)
		assert.NotContains(t, view.ConfirmationRequirement.Reason, "blocked_by_governance")
		assert.NotContains(t, view.ConfirmationRequirement.NextAction, "resolve governance blockers")

		// The only blocker is the informational no-skill pacing cue, not an error.
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "no_skill_required:S2_IMPLEMENT")
		for _, blocker := range view.Blockers {
			assert.NotEqual(t, "blocked_by_governance", blocker.Code)
			if blocker.Code == "no_skill_required" {
				assert.NotEqual(t, model.ReasonSeverityError, blocker.Severity)
			}
		}
	})
}

func TestNextHandoffViewUsesStructuredConfirmationRequirement(t *testing.T) {
	t.Parallel()

	view := nextView{
		Slug:            "confirm",
		Phase:           model.PhaseReviewing,
		ExecutionMode:   governedExecutionMode,
		CurrentState:    model.StateS3Review,
		LifecycleStatus: string(model.ChangeStatusActive),
		NextSkill: &nextSkillView{
			Name:            progression.SkillCodeQualityReview,
			VerificationDir: "artifacts/changes/confirm/verification/code-quality-review",
			State:           string(model.StateS3Review),
		},
		InputContext: nextContext{WorkspaceRoot: "/repo"},
		Blockers:     []model.ReasonCode{},
	}
	view.ConfirmationRequirement = deriveConfirmationRequirement(view)

	raw, err := json.Marshal(buildNextHandoffView(view))
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.NotContains(t, payload, "confirmation_required")
	require.Contains(t, payload, "confirmation_requirement")
	confirmation := payload["confirmation_requirement"].(map[string]any)
	require.Equal(t, "hard_stop", confirmation["boundary"])
	assert.NotContains(t, confirmation, "resume_response_supported")
	assert.Equal(t, "run governance skill code-quality-review and record evidence", confirmation["next_action"])
	assert.Equal(t, "skill_handoff", confirmation["next_action_kind"])
}

func TestNextHandoffViewOutputsMinimalWarnBudget(t *testing.T) {
	t.Parallel()

	view := nextView{
		Slug:            "budget-warn",
		Phase:           model.PhasePlanning,
		ExecutionMode:   governedExecutionMode,
		CurrentState:    model.StateS1Plan,
		LifecycleStatus: string(model.ChangeStatusActive),
		InputContext: nextContext{
			WorkspaceRoot:  "/repo",
			ArtifactBundle: "artifacts/changes/budget-warn",
		},
		ContextBudget: &contextBudget{
			EstimatedTokens:      100,
			AssumedContextWindow: 200,
			UtilizationPercent:   50,
			RemainingPercent:     42.3,
			Health:               "degrading",
			QualityCurve:         "degrading",
			GuardAction:          "warn",
			Thresholds: contextBudgetThresholds{
				WarnBelowRemainingPercent: 50,
				StopBelowRemainingPercent: 35,
			},
			Breakdown: contextBudgetBreakdown{
				SkillPrompt:     1,
				ArtifactContext: 2,
				StateContext:    3,
			},
		},
		Blockers:                []model.ReasonCode{},
		ConfirmationRequirement: confirmationNoBoundary("no_confirmation_boundary"),
	}

	handoff := buildNextHandoffView(view)
	require.NotNil(t, handoff.ContextBudget)
	assert.Equal(t, "warn", handoff.ContextBudget.GuardAction)
	assert.Equal(t, 42.3, handoff.ContextBudget.RemainingPercent)
	raw, err := json.Marshal(handoff)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	budget, ok := payload["context_budget"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, budget, 2)
	assert.Contains(t, budget, "guard_action")
	assert.Contains(t, budget, "remaining_percent")
}

func TestNextHandoffViewStopBudgetKeepsRecoveryPathsWithoutDiagnostics(t *testing.T) {
	t.Parallel()

	view := nextView{
		Slug:            "budget-stop",
		Phase:           model.PhaseBuilding,
		ExecutionMode:   governedExecutionMode,
		CurrentState:    model.StateS2Implement,
		LifecycleStatus: string(model.ChangeStatusActive),
		NextSkill: &nextSkillView{
			Name:            progression.SkillWaveOrchestration,
			VerificationDir: "artifacts/changes/budget-stop/verification/wave-orchestration",
			State:           string(model.StateS2Implement),
		},
		InputContext: nextContext{
			WorkspaceRoot:  "/repo",
			ArtifactBundle: "artifacts/changes/budget-stop",
			HandoffContext: &handoffContextView{
				ChangeAuthority: "artifacts/changes/budget-stop/change.yaml",
				PolicyPacks: []handoffPolicyPack{{
					Name:          "local",
					Path:          ".slipway/policy.yaml",
					AdvisoryRules: []string{"keep out of default handoff"},
				}},
			},
			GateStatus:     map[string]string{"review": "blocked"},
			ArtifactStatus: map[string]string{"tasks.md": "missing"},
		},
		ContextBudget: &contextBudget{
			RemainingPercent: 0,
			GuardAction:      "stop",
			Breakdown: contextBudgetBreakdown{
				SkillPrompt:     1,
				ArtifactContext: 2,
				StateContext:    3,
			},
		},
		GovernanceSignals: &governanceSignalView{BlastRadius: "high"},
		ActiveControls: []governanceControlView{{
			ControlID: "domain-review",
			Mode:      "blocking",
			Scope:     "change",
		}},
		RequiredActions: []governanceActionView{{
			ControlID:   "domain-review",
			Description: "record domain review evidence",
		}},
		SkillEvidence:           []skillEvidenceEntry{{SkillName: "wave-orchestration"}},
		Blockers:                []model.ReasonCode{},
		ConfirmationRequirement: confirmationHardStop("skill_handoff:wave-orchestration"),
	}

	handoff := buildNextHandoffView(view)
	require.NotNil(t, handoff.ContextBudget)
	assert.Equal(t, "stop", handoff.ContextBudget.GuardAction)
	assert.Equal(t, "artifacts/changes/budget-stop/change.yaml", handoff.InputContext.ChangeAuthority)

	raw, err := json.Marshal(handoff)
	require.NoError(t, err)
	s := string(raw)
	assert.NotContains(t, s, "handoff_context")
	assert.NotContains(t, s, "policy_packs")
	assert.NotContains(t, s, "advisory_rules")
	assert.NotContains(t, s, "gate_status")
	assert.NotContains(t, s, "artifact_status")
	assert.NotContains(t, s, "governance_signals")
	assert.NotContains(t, s, "active_controls")
	assert.NotContains(t, s, "required_actions")
	assert.NotContains(t, s, "skill_evidence")
	assert.NotContains(t, s, "breakdown")
}

func TestRunDoesNotRequireResumeAfterAbortWithoutWaveBackedState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "abort without wave-backed state should not require resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		abortCmd := commandForRoot(t, root, makeAbortCmd())
		abortCmd.SetArgs([]string{"--json", "--change", slug})
		var abortOut bytes.Buffer
		abortCmd.SetOut(&abortOut)
		require.NoError(t, abortCmd.Execute())

		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--change", slug})
		var runOut bytes.Buffer
		runCmd.SetOut(&runOut)
		runCmd.SetErr(&runOut)
		require.NoError(t, runCmd.Execute())

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, after.InterruptedExecutionAt.IsZero())
	})
}

func TestRunRequiresExplicitResumeAfterAbortWithWaveBackedState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "abort with wave-backed state should require resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` preserve completed first wave
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` continue next wave after abort
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		_, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		materializeWaveExecutionForSummary(t, root, slug)

		abortCmd := commandForRoot(t, root, makeAbortCmd())
		abortCmd.SetArgs([]string{"--json", "--change", slug})
		var abortOut bytes.Buffer
		abortCmd.SetOut(&abortOut)
		require.NoError(t, abortCmd.Execute())

		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--change", slug})
		var blockedOut bytes.Buffer
		runCmd.SetOut(&blockedOut)
		runCmd.SetErr(&blockedOut)
		err = runCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_required", cliErr.ErrorCode)

		implementCmd := commandForRoot(t, root, makeImplementCmd())
		implementCmd.SetArgs([]string{"--json", "--change", slug})
		var implementOut bytes.Buffer
		implementCmd.SetOut(&implementOut)
		implementCmd.SetErr(&implementOut)
		err = implementCmd.Execute()
		require.Error(t, err)
		cliErr = asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_required", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, "`slipway implement --resume`")
		assert.NotContains(t, cliErr.Remediation, "`slipway run --resume`")

		resumeCmd := commandForRoot(t, root, makeRunCmd())
		resumeCmd.SetArgs([]string{"--json", "--resume", "--change", slug})
		var resumeOut bytes.Buffer
		resumeCmd.SetOut(&resumeOut)
		require.NoError(t, resumeCmd.Execute())

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.True(t, after.InterruptedExecutionAt.IsZero())
	})
}

func TestRunDoesNotRequireResumeWhenPlanningEvidenceIsStale(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "stale planning should not resume old wave")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` preserve completed first wave
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` continue next wave after abort
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		materializeWaveExecutionForSummary(t, root, slug)

		abortCmd := commandForRoot(t, root, makeAbortCmd())
		abortCmd.SetArgs([]string{"--json", "--change", slug})
		var abortOut bytes.Buffer
		abortCmd.SetOut(&abortOut)
		require.NoError(t, abortCmd.Execute())

		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` preserve completed first wave
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` continue next wave after abort
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-03`"+` changed planning evidence after abort
  - depends_on: ["task-02"]
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)))

		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var runOut bytes.Buffer
		runCmd.SetOut(&runOut)
		runCmd.SetErr(&runOut)
		require.NoError(t, runCmd.Execute())
		assert.NotContains(t, runOut.String(), "resume_required")
		assert.Contains(t, runOut.String(), "stale_planning_evidence")
	})
}

func TestRunRejectsResumeWhenWaveRunsAreIncomplete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "run resume should fail closed when wave evidence is incomplete")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending second wave
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))

		plan, err := state.MaterializeWavePlan(root, change)
		require.NoError(t, err)
		summary, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		runs, err := state.BuildWaveRuns(plan, summary.RunSummaryVersion, summary.Tasks, nil)
		require.NoError(t, err)
		require.Len(t, runs, 2, "expected one persisted run per planned wave")
		require.NoError(t, state.SaveWaveRuns(root, slug, summary.RunSummaryVersion, runs[:1]))

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--resume", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "wave_runs_incomplete", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	})
}

func TestRunResumeUnavailableExplainsLifecycleBoundary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "resume boundary")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--resume", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err = cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_unavailable", cliErr.ErrorCode)
		assert.Equal(t, model.StateS3Review, cliErr.Details["current_state"])
		assert.Contains(t, cliErr.Remediation, "S2_IMPLEMENT")
	})
}

func TestShouldStopRunLoopDoesNotStopForExecutionResumeContext(t *testing.T) {
	t.Parallel()

	view := nextView{
		CurrentState: model.StateS2Implement,
		Advanced:     &progression.AdvanceSummary{Action: "advanced"},
		InputContext: nextContext{
			ExecutionResume: &executionResumeContext{
				RunSummaryVersion: 1,
				CompletedTaskIDs:  []string{"task-01"},
				ResumeWaveIndex:   2,
			},
		},
	}

	assert.False(t, shouldStopRunLoop(view))
}

func TestNextIncludesFreshnessInExecutionResume(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "test freshness in execution resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Set to wave execution state with persisted execution summary
		// and some completed tasks to trigger execution resume context.
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` refresh execution resume freshness
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code
`)))
		materializeWaveExecutionForSummary(t, root, slug)

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNil(t, view.InputContext.ExecutionResume)
		assert.NotEmpty(t, view.InputContext.ExecutionResume.Freshness,
			"execution resume should include freshness field")
		assert.Contains(t, []string{"fresh", "stale", "unknown"},
			view.InputContext.ExecutionResume.Freshness)
	})
}

func TestNextDoesNotBuildExecutionResumeFromChecklistWithoutReadyExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "bundle checklist resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        time.Now().UTC(),
			OverallVerdict:    model.ExecutionVerdictPass,
			Tasks:             []model.ExecutionTaskSummary{},
		})

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [x] `+"`t-01`"+` implement bundle-first resume
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code

- [ ] `+"`t-02`"+` rerun verification
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: verification
`)))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())
		assert.NotContains(t, buf.String(), "\"task_policies\"")

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		assert.Nil(t, view.InputContext.ExecutionResume)
	})
}

func TestNextDoesNotRetainExecutionResumeWhenOnlyChecklistMarksTasksComplete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "bundle resume without skip-safe tasks")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        time.Now().UTC(),
			OverallVerdict:    model.ExecutionVerdictPass,
			Tasks:             []model.ExecutionTaskSummary{},
		})

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [x] `+"`t-verify`"+` rerun verification
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: verification

- [ ] `+"`t-next`"+` continue execution
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code
`)))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		assert.Nil(t, view.InputContext.ExecutionResume)
	})
}

func TestNextPreviewIncludesWavePlanTaskShape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		writeTestSubagentConfig(t, root, func(cfg *model.Config) {
			cfg.Subagents.Executor = model.SubagentSlot{
				Type:                model.SubagentTypeMCP,
				Name:                "slipway-executor-hub",
				SessionInstructions: "Execute the planned wave tasks and record task evidence.",
				Timeout:             "60m",
			}
		})

		slug := createGovernedRequest(t, root, levelNonDiscovery, "wave plan protocol version")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [ ] `+"`t-01`"+` execute schema-tightened wave task
  - depends_on: []
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

		inputContext, ok := payload["input_context"].(map[string]any)
		require.True(t, ok, "expected input_context in next output")

		wavePlan, ok := inputContext["wave_plan"].(map[string]any)
		require.True(t, ok, "expected wave_plan in next output")

		executorSubagent, ok := wavePlan["executor_subagent"].(map[string]any)
		require.True(t, ok, "expected configured executor_subagent in wave_plan")
		assert.Equal(t, "mcp", executorSubagent["type"])
		assert.Equal(t, "slipway-executor-hub", executorSubagent["name"])
		assert.Equal(t, "Execute the planned wave tasks and record task evidence.", executorSubagent["session_instructions"])
		assert.Equal(t, "60m", executorSubagent["timeout"])
		assert.NotContains(t, executorSubagent, "capabilities")
		engineBoundary, ok := executorSubagent["engine_boundary"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, false, engineBoundary["read_only"])
		assert.Equal(t, "allow", engineBoundary["mutation_policy"])

		rawWaves, ok := wavePlan["waves"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, rawWaves)

		firstWave, ok := rawWaves[0].(map[string]any)
		require.True(t, ok)
		rawTasks, ok := firstWave["tasks"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, rawTasks)

		firstTask, ok := rawTasks[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "t-01", firstTask["task_id"])
		assert.Equal(t, "execute schema-tightened wave task", firstTask["objective"])
		assert.Equal(t, "code", firstTask["task_kind"])
		assert.Equal(t, []any{"cmd/next.go"}, firstTask["target_files"])
	})
}

func TestNextHandoffJSONIncludesWavePlanParallelSignal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		config       string
		wantParallel bool
	}{
		{
			name:         "default forced parallelization",
			wantParallel: true,
		},
		{
			name:         "parallelization off",
			config:       "execution:\n  parallelization: off\n",
			wantParallel: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			withCommandWorkspace(t, root, func() {
				initTestWorkspace(t, root)
				if tt.config != "" {
					require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(tt.config), 0o644))
				}

				slug := createGovernedRequest(t, root, levelNonDiscovery, tt.name)
				change, err := state.LoadChange(root, slug)
				require.NoError(t, err)

				change.CurrentState = model.StateS2Implement
				change.PlanSubStep = model.PlanSubStepNone
				require.NoError(t, state.SaveChange(root, change))

				bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
				require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [ ] `+"`t-01`"+` first handoff wave task
  - depends_on: []
  - target_files: ["cmd/next.go"]
  - task_kind: code
- [ ] `+"`t-02`"+` second handoff wave task
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
				_, err = state.MaterializeWavePlan(root, change)
				require.NoError(t, err)

				cmd := commandForRoot(t, root, makeNextCmd())
				cmd.SetArgs([]string{"--json", "--change", slug})
				var buf bytes.Buffer
				cmd.SetOut(&buf)
				require.NoError(t, cmd.Execute())

				var payload map[string]any
				require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

				inputContext, ok := payload["input_context"].(map[string]any)
				require.True(t, ok, "expected input_context in next output")

				wavePlan, ok := inputContext["wave_plan"].(map[string]any)
				require.True(t, ok, "expected wave_plan in handoff next output")

				rawWaves, ok := wavePlan["waves"].([]any)
				require.True(t, ok)
				require.NotEmpty(t, rawWaves)

				firstWave, ok := rawWaves[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, tt.wantParallel, firstWave["parallel"])
			})
		})
	}
}

func TestNextPreviewUsesCurrentTasksDuringS2Implementation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "current tasks should drive S2 wave preview")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [ ] `+"`t-01`"+` initial wave task before S2 amendment
  - depends_on: []
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [ ] `+"`t-01`"+` amended S2 task should drive live wave preview
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

		inputContext, ok := payload["input_context"].(map[string]any)
		require.True(t, ok, "expected input_context in next output")

		wavePlan, ok := inputContext["wave_plan"].(map[string]any)
		require.True(t, ok, "expected wave_plan in next output")

		rawWaves, ok := wavePlan["waves"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, rawWaves)

		firstWave, ok := rawWaves[0].(map[string]any)
		require.True(t, ok)
		rawTasks, ok := firstWave["tasks"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, rawTasks)

		firstTask, ok := rawTasks[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "amended S2 task should drive live wave preview", firstTask["objective"])
		assert.Equal(t, []any{"cmd/run.go"}, firstTask["target_files"])
	})
}

func TestNextPreviewIncludesExecutionResumeContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "preview execution resume context")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 3,
			CapturedAt:        time.Now().UTC(),
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"task-01"},
			Tasks: []model.ExecutionTaskSummary{
				{
					TaskID:     "task-01",
					Verdict:    model.TaskVerdictPass,
					TaskKind:   model.TaskKindCode,
					CapturedAt: time.Now().UTC(),
				},
			},
		})
		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`# Tasks

- [x] `+"`task-01`"+` preserve preview resume context
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code

- [ ] `+"`task-09`"+` pending resume task
  - depends_on: ["task-01"]
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code
`)))
		materializeWaveExecutionForSummary(t, root, slug)

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNil(t, view.InputContext.ExecutionResume)
		assert.Equal(t, 3, view.InputContext.ExecutionResume.RunSummaryVersion)
		assert.Equal(t, []string{"task-01"}, view.InputContext.ExecutionResume.CompletedTaskIDs)
		assert.Equal(t, 2, view.InputContext.ExecutionResume.ResumeWaveIndex)
		assert.NotEmpty(t, view.InputContext.ExecutionResume.Freshness)

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Implement, after.CurrentState)
	})
}

func TestNextExecutionResumeFreshnessTurnsStaleAfterInputUpdate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "freshness stale after input update")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-01.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(taskEvidencePath), 0o755))
		taskEvidence := map[string]any{
			"task_id":             "task-01",
			"run_summary_version": 1,
			"task_kind":           "code",
			"verdict":             "pass",
			"evidence_ref":        "test:task-01",
			"captured_at":         time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano),
			"freshness_inputs":    state.ExpectedExecutionTaskFreshnessInputs(change, 1, "task-01"),
		}
		rawTaskEvidence, err := json.Marshal(taskEvidence)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(taskEvidencePath, rawTaskEvidence, 0o644))

		oldTS := time.Now().Add(-1 * time.Minute).UTC()
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        oldTS,
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"task-01"},
			Tasks: []model.ExecutionTaskSummary{
				{
					TaskID:          "task-01",
					Verdict:         model.TaskVerdictPass,
					TaskKind:        model.TaskKindCode,
					EvidenceRef:     taskEvidencePath,
					FreshnessInputs: state.ExpectedExecutionTaskFreshnessInputs(change, 1, "task-01"),
					CapturedAt:      oldTS,
				},
			},
		})
		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` preserve stale freshness on execution resume
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code
`)))

		// Simulate a relevant input update after evidence was captured.
		require.NoError(t, state.SaveChange(root, change))
		materializeWaveExecutionForSummary(t, root, slug)

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.InputContext.ExecutionResume)
		assert.Equal(t, "stale", view.InputContext.ExecutionResume.Freshness)
	})
}

func TestNextPreviewAdvancesAfterPassingResearchVerification(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, levelDiscovery, "passing research verification should advance to next skill")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		worktreeRoot := filepath.Join(t.TempDir(), change.Slug)
		branch := "feat/" + change.Slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
		change.WorktreePath = worktreeRoot
		change.WorktreeBranch = branch
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

		bundlePath := filepath.Join(worktreeRoot, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "research.md", []byte(`
## Alternatives Considered
Confirmed current CLI behavior and artifact ownership boundaries.

## Unknowns
None.

## Assumptions
The research-orchestration skill authored this artifact before recording pass evidence.

## Canonical References
- artifacts/changes/`+change.Slug+`/research.md
`)))

		writeSkillVerification(t, root, slug, progression.SkillResearchOrchestration, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			References: []string{"research:complete"},
		})

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Nil(t, view.NextSkill)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "no_skill_required:S1_PLAN")
	})
}

func TestCheckGovernedBundleReadyL2(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-bundle"
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))

	change := model.Change{Slug: slug, ArtifactSchema: model.ArtifactSchemaExpanded}

	// Missing files — the planning bundle is not ready.
	assert.False(t, progression.CheckGovernedBundleReady(root, change))

	// L2 expanded schema bundle-readiness set: change.yaml, intent.md,
	// requirements.md, decision.md, tasks.md. assurance.md is deferred to
	// S3_REVIEW authoring and owned by the assurance contract gate (issue #141),
	// so it is NOT part of the generic bundle-readiness existence check.
	l2Required := []string{"change.yaml", "intent.md", "requirements.md", "decision.md", "tasks.md"}
	for i, f := range l2Required {
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, f, []byte("x")))
		if i < len(l2Required)-1 {
			assert.False(t, progression.CheckGovernedBundleReady(root, change), "should be false with only %d of %d files", i+1, len(l2Required))
		}
	}
	assert.True(t, progression.CheckGovernedBundleReady(root, change))
}

func TestCheckGovernedBundleReadyL3(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-bundle-l3"
	worktreeRoot := t.TempDir()
	initGitRepoForWorktreeTests(t, root)
	branch := "feat/" + slug
	runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
	bundlePath := filepath.Join(worktreeRoot, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))

	change := model.Change{
		Slug:           slug,
		NeedsDiscovery: true,
		WorktreePath:   worktreeRoot,
		WorktreeBranch: branch,
		ArtifactSchema: model.ArtifactSchemaExpanded,
	}

	// Discovery-mode expanded schema adds research.md to the base set.
	// assurance.md is deferred (issue #141) and not part of the bundle-readiness
	// set, so research.md is the discovery differentiator under test.
	baseFiles := []string{"change.yaml", "intent.md", "requirements.md", "decision.md", "tasks.md"}
	for _, f := range baseFiles {
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, f, []byte("x")))
	}
	assert.False(t, progression.CheckGovernedBundleReady(root, change))

	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "research.md"), []byte(`## Alternatives Considered
### Option A
Tradeoff A

### Option B
Tradeoff B

## Unknowns
Two

## Assumptions
Three

## Canonical References
Docs`), 0o644))
	assert.True(t, progression.CheckGovernedBundleReady(root, change))
}

// After issue #141, assurance.md is deferred to S3_REVIEW authoring and its
// existence is owned by AssuranceContractBlockers, not the generic bundle
// readiness check. A planning bundle with no assurance.md is therefore ready —
// the absence must not strand the change before review.
func TestCheckGovernedBundleReadyDoesNotRequireAssuranceArtifact(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-bundle-defers-assurance"
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))

	change := model.Change{Slug: slug, ArtifactSchema: model.ArtifactSchemaExpanded}
	for _, f := range []string{"change.yaml", "intent.md", "requirements.md", "decision.md", "tasks.md"} {
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, f, []byte("x")))
	}
	// No assurance.md on disk, yet the bundle is ready.
	assert.True(t, progression.CheckGovernedBundleReady(root, change))
}

func TestCheckGovernedBundleReadyRejectsMissingDesignDependency(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-bundle-missing-design"
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))

	change := model.Change{Slug: slug, ArtifactSchema: model.ArtifactSchemaExpanded}
	for _, f := range []string{"change.yaml", "intent.md", "requirements.md", "tasks.md", "assurance.md"} {
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, f, []byte("x")))
	}
	assert.False(t, progression.CheckGovernedBundleReady(root, change))
}

func TestNextPreviewDoesNotAdvanceState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "preview should be read-only")

		changeBefore, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.Equal(t, model.StateS1Plan, changeBefore.CurrentState)

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.Advanced)
		assert.Equal(t, "query", view.Advanced.Action, "query-first next JSON must surface the read-only action explicitly")

		changeAfter, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, changeBefore.CurrentState, changeAfter.CurrentState)
	})
}

func TestNextPreviewDoesNotAppendLifecycleEvents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "preview should not append lifecycle events")

		changeBefore, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		eventLogPath, err := state.LifecycleEventLogPath(root, changeBefore)
		require.NoError(t, err)
		beforeRaw, beforeErr := os.ReadFile(eventLogPath)
		if beforeErr != nil && !os.IsNotExist(beforeErr) {
			require.NoError(t, beforeErr)
		}

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		afterRaw, afterErr := os.ReadFile(eventLogPath)
		if os.IsNotExist(beforeErr) {
			assert.True(t, os.IsNotExist(afterErr), "next query must not create lifecycle event log")
			return
		}
		require.NoError(t, afterErr)
		assert.Equal(t, string(beforeRaw), string(afterRaw), "next query must not append lifecycle events")
	})
}

func TestNextPreviewExposesArtifactAmendmentsWithoutPersistingReconcile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "preview should expose artifact amendments without persisting")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		intentPath := artifact.ResolveArtifactPath(bundlePath, "intent.md")
		oldContent := []byte("# Intent\nOriginal frozen content\n")
		require.NoError(t, os.WriteFile(intentPath, oldContent, 0o644))
		oldHash, err := model.ComputeFileContentHash(intentPath)
		require.NoError(t, err)
		change.Artifacts["intent"] = model.ArtifactState{
			ID:          "intent",
			Path:        intentPath,
			State:       model.ArtifactLifecycleFrozen,
			ContentHash: oldHash,
			UpdatedAt:   time.Now().UTC(),
		}
		require.NoError(t, state.SaveChange(root, change))

		require.NoError(t, os.WriteFile(intentPath, []byte("# Intent\nAmended content\n"), 0o644))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.Len(t, view.ArtifactAmendments, 1)
		assert.Equal(t, "intent", view.ArtifactAmendments[0].ArtifactID)
		assert.Equal(t, string(model.ArtifactLifecycleFrozen), view.ArtifactAmendments[0].FromState)
		assert.Equal(t, string(model.ArtifactLifecycleApproved), view.ArtifactAmendments[0].ToState)

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.ArtifactLifecycleFrozen, after.Artifacts["intent"].State)
		assert.Equal(t, oldHash, after.Artifacts["intent"].ContentHash)
	})
}

func TestRunIncludesTransitionTrace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "run transition trace")

		// createGovernedRequest runs request + one next, leaving governed lane at S1.
		changeBefore, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.Equal(t, model.StateS1Plan, changeBefore.CurrentState)

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotEmpty(t, view.AutoTransitions)

		hasWorktreeAdvance := false
		for _, step := range view.AutoTransitions {
			if step.FromState == model.StateS1Plan && step.Action == "advanced" {
				hasWorktreeAdvance = true
				break
			}
		}
		assert.True(t, hasWorktreeAdvance, "expected auto_transitions to include S1_PLAN advancement")
	})
}

func TestNextContextBudgetHardStopAddsWarning(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		// Force a tiny context window so the estimated budget trips the hard stop.
		t.Setenv("SLIPWAY_CONTEXT_WINDOW_TOKENS", "1")
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "context hard stop")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.ContextBudget)
		assert.Equal(t, "stop", view.ContextBudget.GuardAction)
		found := false
		for _, w := range view.Warnings {
			if strings.Contains(w, "stop threshold") {
				found = true
				break
			}
		}
		assert.True(t, found, "stop action should produce a warning, not a blocker")
	})
}

func TestNextBlocksWhenGovernedChangeHasNoFrozenSchema(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "document diagnostics warning")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.ArtifactSchema = ""
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepValidate
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "artifact_schema_missing")
		assert.Equal(t, slug, view.Slug)
	})
}

func TestNextGovernedMaterializesExecutionSummaryAndRuntimeSummaryDuringImplementation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, levelNonDiscovery, "materialize run summary")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeTaskEvidenceFile(t, root, slug, 1, "task-a", map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"blockers":            []string{},
		"evidence_ref":        "test:task-a",
		"captured_at":         time.Now().UTC().Format(time.RFC3339Nano),
	})
	writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{"task:evidence"},
	})
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-a`"+` materialize run summary
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	_, err = buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
	require.NoError(t, err)

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.RunSummaryVersion)
	require.Len(t, summary.Tasks, 1)
	assert.Equal(t, "task-a", summary.Tasks[0].TaskID)
	assert.Equal(t, model.TaskVerdictPass, summary.Tasks[0].Verdict)
}

func TestNextGovernedBlocksWithoutTaskEvidenceForWaveRunSummaryDuringImplementation(t *testing.T) {
	t.Parallel()
	root, slug := prepareMissingTaskEvidenceForWaveRunSummaryFixture(t)

	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
	require.NoError(t, err)

	var missingEvidenceBlocker string
	for _, blocker := range model.ReasonSpecs(view.Blockers) {
		if strings.HasPrefix(blocker, "missing_task_evidence_for_run_summary:run_summary_version=1") {
			missingEvidenceBlocker = blocker
			break
		}
	}
	require.NotEmpty(t, missingEvidenceBlocker)
	assert.Contains(t, missingEvidenceBlocker, ".git/slipway/runtime/changes/"+slug+"/evidence/tasks")
	assert.Contains(t, missingEvidenceBlocker, "record_command=slipway evidence task --result-file <path> --json")
	assert.Contains(t, missingEvidenceBlocker, "result_schema=task_id,verdict,evidence_ref,changed_files,blockers,session_id")
	assert.NotContains(t, missingEvidenceBlocker, "required_fields=task_id,run_summary_version,task_kind")
	assert.Equal(t, model.StateS2Implement, view.CurrentState)
}

func TestReadOnlyS2DiagnosticsKeepSingleRunSummaryMissingForAbsentTaskEvidence(t *testing.T) {
	t.Parallel()
	root, slug := prepareMissingTaskEvidenceForWaveRunSummaryFixture(t)

	nextCmd := commandForRoot(t, root, makeNextCmd())
	nextCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var nextOut bytes.Buffer
	nextCmd.SetOut(&nextOut)
	require.NoError(t, nextCmd.Execute())
	var nextDiag nextView
	require.NoError(t, json.Unmarshal(nextOut.Bytes(), &nextDiag))
	assertSingleRunSummaryMissingTaskEvidenceBlocker(t, "next", nextDiag.Blockers, slug)

	validateCmd := commandForRoot(t, root, makeValidateCmd())
	validateCmd.SetArgs([]string{"--change", slug})
	var validateOut bytes.Buffer
	validateCmd.SetOut(&validateOut)
	require.NoError(t, validateCmd.Execute())
	var validate validateView
	require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validate))
	assertSingleRunSummaryMissingTaskEvidenceBlocker(t, "validate", validate.Blockers, slug)

	statusCmd := commandForRoot(t, root, makeStatusCmd())
	statusCmd.SetArgs([]string{"--json", "--change", slug})
	var statusOut bytes.Buffer
	statusCmd.SetOut(&statusOut)
	require.NoError(t, statusCmd.Execute())
	var status statusView
	require.NoError(t, json.Unmarshal(statusOut.Bytes(), &status))
	assertSingleRunSummaryMissingTaskEvidenceBlocker(t, "status", status.Blockers, slug)
}

func TestReadOnlyS2DiagnosticsUseTaskEvidenceDriftInsteadOfRunSummaryMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, levelNonDiscovery, "surface stale task evidence diagnostics")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-a`"+` Initial objective
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	taskCapturedAt := time.Now().UTC().Add(-2 * time.Minute)
	writeTaskEvidenceFile(t, root, slug, 1, "task-a", map[string]any{
		"changed_files": []string{"cmd/next.go"},
		"captured_at":   taskCapturedAt.Format(time.RFC3339Nano),
	})
	writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  taskCapturedAt.Add(time.Minute),
		RunVersion: 1,
	})
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-a`"+` Updated objective
  - target_files: ["cmd/status.go"]
  - task_kind: code
`)))

	nextCmd := commandForRoot(t, root, makeNextCmd())
	nextCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
	var nextOut bytes.Buffer
	nextCmd.SetOut(&nextOut)
	require.NoError(t, nextCmd.Execute())
	var nextDiag nextView
	require.NoError(t, json.Unmarshal(nextOut.Bytes(), &nextDiag))
	assertReadOnlyS2TaskEvidenceDriftBlockers(t, "next", nextDiag.Blockers)

	validateCmd := commandForRoot(t, root, makeValidateCmd())
	validateCmd.SetArgs([]string{"--change", slug})
	var validateOut bytes.Buffer
	validateCmd.SetOut(&validateOut)
	require.NoError(t, validateCmd.Execute())
	var validate validateView
	require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validate))
	assertReadOnlyS2TaskEvidenceDriftBlockers(t, "validate", validate.Blockers)

	statusCmd := commandForRoot(t, root, makeStatusCmd())
	statusCmd.SetArgs([]string{"--json", "--change", slug})
	var statusOut bytes.Buffer
	statusCmd.SetOut(&statusOut)
	require.NoError(t, statusCmd.Execute())
	var status statusView
	require.NoError(t, json.Unmarshal(statusOut.Bytes(), &status))
	assertReadOnlyS2TaskEvidenceDriftBlockers(t, "status", status.Blockers)

	_, err = os.Stat(state.ExecutionSummaryPathForRead(root, slug))
	assert.True(t, os.IsNotExist(err), "read-only surfaces must not materialize execution-summary.yaml")
}

func assertReadOnlyS2TaskEvidenceDriftBlockers(t *testing.T, surface string, blockers []model.ReasonCode) {
	t.Helper()
	specs := model.ReasonSpecs(blockers)
	assert.Contains(t, specs, "tasks_plan_changed_since_task_evidence:task-a", surface)
	for _, spec := range specs {
		assert.NotContains(t, spec, "wave-orchestration:run_summary_missing", surface)
	}
}

func assertSingleRunSummaryMissingTaskEvidenceBlocker(t *testing.T, surface string, blockers []model.ReasonCode, slug string) {
	t.Helper()
	specs := model.ReasonSpecs(blockers)
	matches := []string{}
	for _, spec := range specs {
		assert.NotContains(t, spec, "missing_task_evidence_for_run_summary", surface)
		if strings.HasPrefix(spec, "required_skill_not_ready:wave-orchestration:run_summary_missing") {
			matches = append(matches, spec)
		}
	}
	require.Len(t, matches, 1, surface)
	missingEvidenceBlocker := matches[0]
	assert.Contains(t, missingEvidenceBlocker, "run_summary_version=1", surface)
	assert.Contains(t, missingEvidenceBlocker, ".git/slipway/runtime/changes/"+slug+"/evidence/tasks", surface)
	assert.Contains(t, missingEvidenceBlocker, "record_command=slipway evidence task --result-file <path> --json", surface)
	assert.Contains(t, missingEvidenceBlocker, "result_schema=task_id,verdict,evidence_ref,changed_files,blockers,session_id", surface)
	assert.NotContains(t, missingEvidenceBlocker, "required_fields=task_id,run_summary_version,task_kind", surface)
}

func prepareMissingTaskEvidenceForWaveRunSummaryFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, levelNonDiscovery, "missing task evidence should block")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
	})
	return root, slug
}

func prepareStalePlanningRecoveryFixture(t *testing.T, root string, currentState model.WorkflowState) (string, model.Change) {
	t.Helper()
	slug, change := prepareStalePlanningRecoveryBaseFixture(t, root, currentState)

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	staleTasksPath := filepath.Join(bundlePath, "tasks.md")
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` verify recovered planning chain
  - depends_on: []
  - target_files: ["cmd/next.go", "cmd/run.go"]
  - task_kind: verification
  - covers: [REQ-001]
`)))
	staleAt := time.Now().UTC().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(staleTasksPath, staleAt, staleAt))

	return slug, change
}

func prepareStalePlanningRecoveryBaseFixture(t *testing.T, root string, currentState model.WorkflowState) (string, model.Change) {
	t.Helper()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "stale planning recovery")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = currentState
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeShipReadyGovernedBundle(t, root, change)
	writeSkillVerification(t, root, slug, progression.SkillPlanAudit, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Add(-2 * time.Second),
		RunVersion: 0,
		References: []string{"plan-audit:fixture"},
	})
	refreshPassingSkillDigestsForTest(t, root, slug, progression.SkillPlanAudit)
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
		"changed_files": []string{"cmd/done.go"},
	})
	writePassingReviewEvidencePack(t, root, slug, 1)
	writePassingShipVerificationEvidence(t, root, slug, 1)

	return slug, change
}

func writeTaskEvidenceFile(t *testing.T, root, slug string, runSummaryVersion int, taskID string, payload map[string]any) {
	t.Helper()
	if _, ok := payload["task_id"]; !ok {
		payload["task_id"] = taskID
	}
	if _, ok := payload["run_summary_version"]; !ok {
		payload["run_summary_version"] = runSummaryVersion
	}
	if _, ok := payload["task_kind"]; !ok {
		payload["task_kind"] = "code"
	}
	if _, ok := payload["verdict"]; !ok {
		payload["verdict"] = "pass"
	}
	if _, ok := payload["evidence_ref"]; !ok {
		payload["evidence_ref"] = "test:" + taskID
	}
	if _, ok := payload["captured_at"]; !ok {
		capturedAt := time.Now().UTC()
		if summary, err := state.LoadExecutionSummary(root, slug); err == nil {
			for _, task := range summary.Tasks {
				if task.TaskID == taskID && !task.CapturedAt.IsZero() {
					capturedAt = task.CapturedAt.UTC()
					break
				}
			}
		}
		payload["captured_at"] = capturedAt.Format(time.RFC3339Nano)
	}
	if _, ok := payload["freshness_inputs"]; !ok {
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		tasksPlanHash := ""
		if wavePlan, err := state.LoadWavePlanForChange(root, change); err == nil {
			tasksPlanHash = wavePlan.TasksPlanHash
		}
		payload["freshness_inputs"] = state.ExpectedExecutionTaskFreshnessInputs(change, runSummaryVersion, taskID, tasksPlanHash)
	}
	dir := state.EvidenceTasksDir(root, slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, taskID+".json")
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o644))
}

func writeBundleArtifactFile(bundlePath, slug, artifactName string, content []byte) error {
	path := artifact.ResolveArtifactPath(bundlePath, artifactName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}
