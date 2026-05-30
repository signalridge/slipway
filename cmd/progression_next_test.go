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
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionableSkillViewsOmitAlreadyPassingDisplaySkillWithoutBlocker(t *testing.T) {
	t.Parallel()

	change := model.NewChange("passing-display-skill")
	change.CurrentState = model.StateS4Verify
	readiness := progression.GovernanceReadiness{
		PassingSkills: map[string]model.VerificationRecord{
			progression.SkillGoalVerification: {
				Verdict:   model.VerificationVerdictPass,
				Timestamp: time.Now().UTC(),
			},
		},
	}

	assert.Nil(t, buildActionableNextSkillView(change, readiness))

	view := nextView{CurrentState: model.StateS4Verify}
	err := assembleSkillViewWithOptions(
		t.TempDir(),
		&view,
		changeRef{Slug: change.Slug},
		progression.AdvanceSummary{},
		&change,
		nil,
		readiness.PassingSkills,
		nil,
		false,
		assembleSkillViewOptions{},
	)
	require.NoError(t, err)
	assert.Nil(t, view.NextSkill)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "no_skill_required:S4_VERIFY")
}

func TestNextReturnsNextSkillForGovernedState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "add caching layer")

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

func TestNextS0ResearchActionDoesNotRequestResearchMarkdown(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createIntakeChangeFixture(t, root, "clarify workflow feedback")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.IntakeSubStep = model.IntakeSubStepResearch
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillIntakeClarification, view.NextSkill.Name)
		require.NotEmpty(t, view.RequiredActions)
		for _, action := range view.RequiredActions {
			if action.ControlID == string(model.ControlResearch) {
				assert.NotContains(t, action.Description, "complete research.md")
				assert.Contains(t, action.Description, "S0 intake research questions")
			}
		}
	})
}

func TestNextS1BundleSurfacesPlanAuditHandoff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L3", "audit bundle handoff")
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
		require.NotNil(t, view.NextSkill)
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

		slug := createGovernedRequest(t, root, "L2", "governance blocker preview")
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

		slug := createGovernedRequest(t, root, "L2", "recover from corrupt governance snapshot")
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

		slug := createGovernedRequest(t, root, "L2", "preview should expose plan recovery state")
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

	slug := createGovernedRequest(t, root, "L2", "light preset json autopass advisory")
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
	writePassingGoalVerificationEvidence(t, root, slug, 1)

	// Advancement is tested via buildNextView with preview=false (run path).
	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
	require.NoError(t, err)
	require.NotNil(t, view.Advanced)
	assert.Equal(t, "done_ready", view.Advanced.Action)
	require.Len(t, view.Advanced.AutoPassedStates, 1)
	assert.Equal(t, model.StateS4Verify, view.Advanced.AutoPassedStates[0].State)
	assert.Empty(t, view.AutoPassEligible)
	assert.Nil(t, view.NextSkill)

	updated, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS4Verify, updated.CurrentState)
	require.Len(t, updated.LastAutoPassedStates, 1)
}

func TestNextJSONNoAutoPassReportsEligibilityFromCurrentStateOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "light preset explicit no-auto-pass advisory")
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
	writePassingGoalVerificationEvidence(t, root, slug, 1)

	// Advancement with no-auto-pass is tested via buildNextView (run path).
	// autoSkipEvidence=false mirrors the original --json path.
	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, false, true)
	require.NoError(t, err)
	require.NotNil(t, view.Advanced)
	assert.Equal(t, "advanced", view.Advanced.Action)
	assert.Empty(t, view.Advanced.AutoPassedStates)
	require.Len(t, view.AutoPassEligible, 1)
	assert.Equal(t, model.StateS4Verify, view.AutoPassEligible[0].State)
	assert.Nil(t, view.NextSkill)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "no_skill_required:S4_VERIFY")

	updated, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS4Verify, updated.CurrentState)
	assert.Empty(t, updated.LastAutoPassedStates)
}

func TestNextDoesNotAutoPassLightPresetReviewWithoutExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "light preset review still requires execution authority")
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
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.NextSkill.Name)
	})
}

func TestNextDoesNotReturnDoneReadyWithoutGoalVerification(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "verify auto-pass still requires goal verification")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.WorkflowPreset = model.WorkflowPresetLight
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "intent.md", []byte("# Proposal")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "requirements.md", []byte("# Spec")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "decision.md", []byte("# Design")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte("- [ ] `t-01` verify\n  - wave: 1\n  - target_files: [\"cmd/done.go\"]\n  - task_kind: verification\n")))
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
		assert.Equal(t, model.StateS4Verify, view.CurrentState)
		if view.Advanced != nil {
			assert.Equal(t, "query", view.Advanced.Action, "query-first next JSON must stay read-only while surfacing missing goal-verification evidence")
		}
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillGoalVerification, view.NextSkill.Name)
	})
}

func TestNextDoesNotAutoPassStrictPresetReview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "strict preset review")
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
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.NextSkill.Name)
		if view.Advanced != nil {
			assert.Empty(t, view.Advanced.AutoPassedStates)
		}
	})
}

func TestNextJSONGoalVerificationHintsDropRetiredFreshEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "goal verification hint contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.MkdirAll(bundlePath, 0o755))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "intent.md", []byte("# Intent")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "requirements.md", []byte("# Requirements")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "decision.md", []byte("# Decision")))
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte("- [ ] `t-01` verify\n  - wave: 1\n  - target_files: [\"cmd/next.go\"]\n  - task_kind: verification\n")))
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
		assert.Equal(t, progression.SkillGoalVerification, view.NextSkill.Name)
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

func TestRunJSONFinalCloseoutDropsRetiredFreshEvidenceHint(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "final closeout run hint contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.QualityMode = model.QualityModeFull
		require.NoError(t, state.SaveChange(root, change))

		markChangeReadyForDone(t, root, &change)
		writeAssuranceMD(t, root, slug, validAssuranceContent())
		// Refresh the summary after bundle mutations so full-closeout readiness
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
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillFinalCloseout, view.NextSkill.Name)
		assert.Empty(t, view.NextSkill.TechniqueHints)
	})
}

func TestAssembleSkillViewFinalCloseoutDropsRetiredFreshEvidenceHint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "final closeout hint contract")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS4Verify
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
		progression.AdvanceSummary{Action: "query", FromState: model.StateS4Verify},
		&change,
		nil,
		map[string]model.VerificationRecord{
			progression.SkillGoalVerification: {
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
	assert.Equal(t, progression.SkillFinalCloseout, view.NextSkill.Name)
	assert.Empty(t, view.NextSkill.TechniqueHints)
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

		slug := createGovernedRequest(t, root, "L2", "test agent hint")
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

		slug := createGovernedRequest(t, root, "L2", "refactor service module")
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
		foundCatalogHint := false
		for _, hint := range view.NextSkill.TechniqueHints {
			if hint.Name == "skill:independent-review" {
				foundCatalogHint = true
				break
			}
		}
		assert.True(t, foundCatalogHint, "expected resolver hint without changing governed next host")
		require.NotNil(t, view.NextSkill.ReviewContext)
		assert.Contains(t, view.NextSkill.ReviewContext.RequiredArtifactLayers, "R0")
		assert.Contains(t, view.NextSkill.ReviewContext.RequiredArtifactLayers, "R3")
		assert.Empty(t, view.NextSkill.ReviewContext.RequiredImplementationLayers)
		assert.Contains(t, view.NextSkill.RequiredTokens, "layer:R0=pass")
		assert.Contains(t, view.NextSkill.RequiredTokens, "layer:R3=pass")
		assert.NotContains(t, view.NextSkill.RequiredTokens, "layer:IR1=pass")
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
			false,
			handoffSkillViewOptions,
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

		slug := createGovernedRequest(t, root, "L2", "json evidence status surface")
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
		assert.Equal(t, progression.SkillSpecComplianceReview, view.NextSkill.DisplayName)
		assert.Equal(t, progression.SkillCodeQualityReview, view.NextSkill.BlockingName)
		assert.Contains(t, view.NextSkill.ResolutionReason, "blocking skill")
		assert.Contains(t, view.NextSkill.RequiredTokens, "layer:IR1=pass")
		assert.NotContains(t, view.NextSkill.RequiredTokens, "layer:R0=pass")

		statusBySkill := map[string]skillEvidenceEntry{}
		for _, entry := range view.SkillEvidence {
			statusBySkill[entry.SkillName] = entry
		}
		require.Contains(t, statusBySkill, progression.SkillSpecComplianceReview)
		require.Contains(t, statusBySkill, progression.SkillCodeQualityReview)
		assert.True(t, statusBySkill[progression.SkillSpecComplianceReview].HasEvidence)
		assert.Equal(t, "passing", statusBySkill[progression.SkillSpecComplianceReview].Status)
		assert.Equal(t, model.VerificationVerdictPass, statusBySkill[progression.SkillSpecComplianceReview].Verdict)
		assert.False(t, statusBySkill[progression.SkillCodeQualityReview].HasEvidence)
		assert.Equal(t, "missing", statusBySkill[progression.SkillCodeQualityReview].Status)
	})
}

func TestReviewStateActionableNextSkillConsistentAcrossCommandSurfaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "consistent review next skill")
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

		nextCmd := commandForRoot(t, root, makeNextCmd())
		nextCmd.SetArgs([]string{"--json", "--change", slug})
		var nextOut bytes.Buffer
		nextCmd.SetOut(&nextOut)
		require.NoError(t, nextCmd.Execute())
		var handoff nextHandoffView
		require.NoError(t, json.Unmarshal(nextOut.Bytes(), &handoff))
		require.NotNil(t, handoff.NextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, handoff.NextSkill.Name)
		require.NotNil(t, handoff.NextSkill.ReviewContext)
		assert.Empty(t, handoff.NextSkill.ReviewContext.RequiredArtifactLayers)
		assert.Contains(t, handoff.NextSkill.ReviewContext.RequiredImplementationLayers, "IR1")
		assert.Contains(t, handoff.NextSkill.ReviewContext.RequiredImplementationLayers, "IR3")
		assert.Contains(t, handoff.NextSkill.RequiredTokens, "layer:IR1=pass")
		assert.Contains(t, handoff.NextSkill.RequiredTokens, "layer:IR3=pass")
		assert.NotContains(t, handoff.NextSkill.RequiredTokens, "layer:R0=pass")

		nextDiagCmd := commandForRoot(t, root, makeNextCmd())
		nextDiagCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var nextDiagOut bytes.Buffer
		nextDiagCmd.SetOut(&nextDiagOut)
		require.NoError(t, nextDiagCmd.Execute())
		var nextDiag nextView
		require.NoError(t, json.Unmarshal(nextDiagOut.Bytes(), &nextDiag))
		require.NotNil(t, nextDiag.NextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, nextDiag.NextSkill.Name)
		assert.Contains(t, nextDiag.NextSkill.RequiredTokens, "layer:IR1=pass")

		validateCmd := commandForRoot(t, root, makeValidateCmd())
		validateCmd.SetArgs([]string{"--json", "--change", slug})
		var validateOut bytes.Buffer
		validateCmd.SetOut(&validateOut)
		require.NoError(t, validateCmd.Execute())
		var validate validateView
		require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validate))
		require.NotNil(t, validate.ActionableNextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, validate.ActionableNextSkill.Name)
		assert.Contains(t, validate.ActionableNextSkill.RequiredTokens, "layer:IR1=pass")
		assert.Contains(t, validate.ActionableNextSkill.RequiredTokens, "layer:IR3=pass")
		assert.NotContains(t, validate.ActionableNextSkill.RequiredTokens, "layer:R0=pass")
		assert.NotContains(t, validate.ActionableNextSkill.RequiredTokens, "layer:R3=pass")

		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var runOut bytes.Buffer
		runCmd.SetOut(&runOut)
		require.NoError(t, runCmd.Execute())
		var runView nextView
		require.NoError(t, json.Unmarshal(runOut.Bytes(), &runView))
		require.NotNil(t, runView.NextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, runView.NextSkill.Name)
		assert.Equal(t, progression.SkillSpecComplianceReview, runView.NextSkill.DisplayName)
		assert.Equal(t, progression.SkillCodeQualityReview, runView.NextSkill.BlockingName)
		assert.Contains(t, runView.NextSkill.ResolutionReason, "display skill")
		assert.Contains(t, runView.NextSkill.RequiredTokens, "layer:IR1=pass")
	})
}

func TestRunJSONDoesNotMarkOptionalFinalCloseoutAsBlocking(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("optional-final-closeout")
	change.QualityMode = model.QualityModeStandard
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone

	view := nextView{
		Slug:         change.Slug,
		CurrentState: model.StateS4Verify,
		InputContext: nextContext{WorkspaceRoot: root},
	}
	err := assembleSkillViewWithOptions(
		root,
		&view,
		changeRef{Slug: change.Slug},
		progression.AdvanceSummary{Action: "blocked", FromState: model.StateS4Verify, Blockers: []model.ReasonCode{model.NewReasonCode("ship_gate_blocked", "assurance.md")}},
		&change,
		nil,
		map[string]model.VerificationRecord{
			progression.SkillGoalVerification: {
				Verdict:    model.VerificationVerdictPass,
				Blockers:   []model.ReasonCode{},
				Timestamp:  time.Now().UTC(),
				RunVersion: 1,
			},
		},
		nil,
		true,
		handoffSkillViewOptions,
	)
	require.NoError(t, err)
	require.NotNil(t, view.NextSkill)
	assert.Equal(t, progression.SkillFinalCloseout, view.NextSkill.Name)
	assert.Equal(t, progression.SkillGoalVerification, view.NextSkill.DisplayName)
	assert.Empty(t, view.NextSkill.BlockingName)
	assert.Contains(t, view.NextSkill.ResolutionReason, "passing goal-verification")
	assert.NotContains(t, view.NextSkill.ResolutionReason, "blocking skill")
}

func TestDiagnosticCommandsExposePathAuthorityWhenFreshnessUnknown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "path authority should not depend on execution freshness")
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
		assert.Contains(t, nextDiag.FreshnessDiagnostics.PathAuthority.RuntimeEvidencePath, ".git/slipway/runtime/changes/"+slug)

		validateCmd := commandForRoot(t, root, makeValidateCmd())
		validateCmd.SetArgs([]string{"--json", "--change", slug})
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

		slug := createGovernedRequest(t, root, "L2", "add pagination")
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

	slug := createGovernedRequest(t, root, "L2", "reuse precomputed next evidence")
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

	slug := createGovernedRequest(t, root, "L2", "test gplan blocking")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	// Move to S1_PLAN/audit without evidence
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	// Advancement blocking is tested via buildNextView (run path).
	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
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
		ToState:   model.StateS2Execute,
	}))
}

func TestNextPreviewFailsWhenSkillEvidenceEvaluationFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "review malformed skill registry handling")
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

	slug := createGovernedRequest(t, root, "L2", "test gplan passing")
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
REQ-001: The plan audit path must advance only when the task checklist is valid.
`)))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`
- [ ] `+"`t-01`"+` implement plan audit checks
  - wave: 1
  - target_files: ["internal/engine/example.go"]
  - task_kind: code
  - covers: [REQ-001]
`), 0o644))

	// Write plan-audit evidence with correct planning input hash.
	writeSkillVerification(t, root, slug, "plan-audit", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	})

	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
	require.NoError(t, err)

	require.NotNil(t, view.Advanced)
	assert.Equal(t, "advanced", view.Advanced.Action)
	assert.Equal(t, model.StateS1Plan, view.Advanced.FromState)
	// Audit clean path: post-audit machine validation runs inline.
	// If validation passes, it advances to S2_EXECUTE.
	// If it fails, it persists at S1_PLAN/validate.
	// This test provides sufficient artifacts for the clean path.
	assert.Equal(t, model.StateS2Execute, view.Advanced.ToState)
	assert.Equal(t, model.StateS2Execute, view.CurrentState)
}

func TestNextBlocksWhenBundleMissingArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "test bundle missing")
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

	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
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

	slug := createGovernedRequest(t, root, "L2", "bundle invalid bound worktree")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	change.WorktreePath = root
	change.WorktreeBranch = currentGitBranch(t, root)
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
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

		slug := createGovernedRequest(t, root, "L2", "tasks checklist validation")
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

		slug := createGovernedRequest(t, root, "L2", "tasks checklist dependency cycle")
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

		slug := createGovernedRequest(t, root, "L2", "preview tasks checklist blockers")
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

		slug := createGovernedRequest(t, root, "L2", "preview assurance contract blocker")
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

func TestRunRejectsResumeResponseWithoutCheckpoint(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		_ = createGovernedRequest(t, root, "L2", "test no checkpoint")

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--resume-response", "approved"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err := cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "no_active_checkpoint", cliErr.ErrorCode)
		assert.Equal(t, categoryPrecondition, cliErr.Category)
		assert.Equal(t, exitCodePrecondition, cliErr.ExitCode)
	})
}

func TestNextReturnsDoneReadyWithoutNextSkillAfterGovernedShipPasses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "done ready contract")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	writePassingExecutionSummary(t, root, slug, 1, "t-01")

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	writeShipReadyGovernedBundle(t, root, change)
	writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
	// Refresh the execution summary after mutating bundle artifacts so the
	// evidence timestamp remains newer than the governed tasks plan.
	writePassingExecutionSummary(t, root, slug, 1, "t-01")

	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)
	writePassingGoalVerificationEvidence(t, root, slug, 1)

	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
	require.NoError(t, err)

	require.NotNil(t, view.Advanced)
	assert.Equal(t, "done_ready", view.Advanced.Action)
	assert.Equal(t, model.StateS4Verify, view.CurrentState)
	assert.Nil(t, view.NextSkill)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "run_slipway_done_to_finalize")
	assert.Contains(t, view.Warnings, "optional_closeout_available: final-closeout evidence is missing or stale; run final-closeout before `slipway done` only if refreshed closeout evidence is desired")

	cmd := commandForRoot(t, root, makeNextCmd())
	cmd.SetArgs([]string{"--json", "--change", slug})
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())
	var handoff nextHandoffView
	require.NoError(t, json.Unmarshal(out.Bytes(), &handoff))
	assert.Nil(t, handoff.NextSkill)
	assert.Contains(t, model.ReasonSpecs(handoff.Blockers), "run_slipway_done_to_finalize")
	assert.NotContains(t, model.ReasonSpecs(handoff.Blockers), "no_skill_required:S4_VERIFY")
}

func TestNextReturnsDoneReadyWithoutFinalCloseoutRequirementForStandardRequestPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, "L2", "standard request done ready contract")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.QualityMode = model.QualityModeStandard
	change.CurrentState = model.StateS4Verify
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
	writePassingGoalVerificationEvidence(t, root, slug, 1)

	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
	require.NoError(t, err)

	require.NotNil(t, view.Advanced)
	assert.Equal(t, "done_ready", view.Advanced.Action)
	assert.Equal(t, model.StateS4Verify, view.CurrentState)
	assert.Nil(t, view.NextSkill)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "run_slipway_done_to_finalize")
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "ship_gate_blocked:required_skill_missing:final-closeout")
}

func TestNextJSONDefaultIsHandoffOnlyAndDiagnosticsKeepsFullSurface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "handoff-only done ready contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS4Verify
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
		writePassingGoalVerificationEvidence(t, root, slug, 1)

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

		slug := createGovernedRequest(t, root, "L2", "handoff suppresses freshness diagnostics")
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
  - wave: 1
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

func TestNextHandoffSourceViewDoesNotBuildDiagnosticSurfaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "handoff source stays narrow")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		view, err := buildNextHandoffSourceView(root, changeRef{Slug: slug}, "", true, false, false)
		require.NoError(t, err)

		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.NextSkill.Name)
		assert.NotNil(t, view.NextSkill.SkillConstraints)
		assert.NotEmpty(t, view.NextSkill.TechniqueHints)
		require.NotNil(t, view.NextSkill.ReviewContext)
		assert.Contains(t, view.NextSkill.ReviewContext.RequiredArtifactLayers, "R0")
		assert.Empty(t, view.NextSkill.ReviewContext.RequiredImplementationLayers)
		require.NotNil(t, view.ContextBudget)
		assert.Equal(t, "ok", view.ContextBudget.GuardAction)
		assert.Nil(t, view.Constraints)
		assert.Nil(t, view.GovernanceSignals)
		assert.Empty(t, view.ActiveControls)
		assert.Empty(t, view.RequiredActions)
		assert.Empty(t, view.SkillEvidence)
		assert.Empty(t, view.ArtifactAmendments)
		assert.Nil(t, view.FreshnessDiagnostics)
		require.NotNil(t, view.InputContext.HandoffContext)
		assert.NotEmpty(t, view.InputContext.HandoffContext.ChangeAuthority)
		assert.Empty(t, view.InputContext.HandoffContext.PolicyPacks)
		assert.Empty(t, view.InputContext.HandoffContext.ReadRefs)
		assert.Nil(t, view.InputContext.GateStatus)
		assert.Nil(t, view.InputContext.ArtifactStatus)
		assert.Nil(t, view.InputContext.WavePlan)
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

	doneReady := deriveConfirmationRequirement(nextView{
		Blockers: []model.ReasonCode{model.NewReasonCode("run_slipway_done_to_finalize", "")},
	})
	assert.False(t, doneReady.Required)
	assert.Equal(t, "command_required", doneReady.Boundary)
	assert.False(t, doneReady.FreshConfirmationRequired)
	assert.True(t, doneReady.PriorAuthorizationSufficient)
	assert.Equal(t, "run_slipway_done_to_finalize", doneReady.Reason)
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
	require.Equal(t, "hard_stop", payload["confirmation_requirement"].(map[string]any)["boundary"])
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
		CurrentState:    model.StateS2Execute,
		LifecycleStatus: string(model.ChangeStatusActive),
		NextSkill: &nextSkillView{
			Name:            progression.SkillWaveOrchestration,
			VerificationDir: "artifacts/changes/budget-stop/verification/wave-orchestration",
			State:           string(model.StateS2Execute),
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

func TestRunRequiresResumeResponseForActiveCheckpoint(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "test checkpoint requires response")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:   "task-01",
			CheckpointType: "human_verify",
		}
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--resume-response")
		assert.Contains(t, err.Error(), "task-01")
	})
}

func TestRunDoesNotRequireResumeAfterAbortWithoutWaveBackedState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "abort without wave-backed state should not require resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
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

		slug := createGovernedRequest(t, root, "L2", "abort with wave-backed state should require resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` preserve completed first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` continue next wave after abort
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		change, err = state.LoadChange(root, slug)
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

func TestRunResumesCheckpointWithValidResponse(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "test checkpoint resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "task-02",
			PausedWaveIndex: 2,
			CheckpointType:  "human_verify",
		}
		require.NoError(t, state.SaveChange(root, change))
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-01`"+` first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` checkpointed second wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--resume-response", "verified ok"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		// Resume checkpoint should carry the response payload
		require.NotNil(t, view.InputContext.ResumeCheckpoint)
		assert.Equal(t, "task-02", view.InputContext.ResumeCheckpoint.PausedTaskID)
		assert.Equal(t, "human_verify", view.InputContext.ResumeCheckpoint.CheckpointType)
		assert.Equal(t, "verified ok", view.InputContext.ResumeCheckpoint.UserResponsePayload)

		// Active checkpoint should be cleared from change state
		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Nil(t, change.ActiveCheckpoint)
	})
}

func TestRunRejectsResumeResponseWhenWaveArtifactsAreMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "run resume-response should fail closed when wave artifacts are missing")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "task-02",
			PausedWaveIndex: 2,
			CheckpointType:  string(model.CheckpointHumanVerify),
		}
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending checkpointed wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--resume-response", "verified ok", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "wave_runs_missing", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	})
}

func TestRunRejectsResumeResponseWhenWavePlanIsMissingBeforeExecutionSummaryReady(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "run resume-response should fail closed when pre-summary wave plan is missing")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "task-02",
			PausedWaveIndex: 2,
			CheckpointType:  string(model.CheckpointHumanVerify),
		}
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--resume-response", "verified ok", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "wave_plan_missing", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)

		after, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		require.NotNil(t, after.ActiveCheckpoint)
		assert.Equal(t, "task-02", after.ActiveCheckpoint.PausedTaskID)
	})
}

func TestNextRejectsCheckpointContextWhenWaveArtifactsAreMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "next should fail closed when checkpoint wave artifacts are missing")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "task-02",
			PausedWaveIndex: 2,
			CheckpointType:  string(model.CheckpointHumanVerify),
		}
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/next.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending checkpointed wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "wave_runs_missing", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	})
}

func TestNextRejectsCheckpointContextWhenWavePlanIsMissingBeforeExecutionSummaryReady(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "next should fail closed when pre-summary checkpoint wave plan is missing")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "task-02",
			PausedWaveIndex: 2,
			CheckpointType:  string(model.CheckpointHumanVerify),
		}
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "wave_plan_missing", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)

		after, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		require.NotNil(t, after.ActiveCheckpoint)
		assert.Equal(t, "task-02", after.ActiveCheckpoint.PausedTaskID)
	})
}

func TestRunRejectsResumeWhenWaveRunsAreIncomplete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "run resume should fail closed when wave evidence is incomplete")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending second wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))

		plan, err := state.MaterializeWavePlan(root, change)
		require.NoError(t, err)
		summary, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		runs, err := state.BuildWaveRuns(plan, summary.RunSummaryVersion, summary.Tasks)
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

		slug := createGovernedRequest(t, root, "L2", "resume boundary")
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
		assert.Contains(t, cliErr.Message, "current_state=S3_REVIEW")
		assert.Equal(t, model.StateS3Review, cliErr.Details["current_state"])
		assert.Contains(t, cliErr.Remediation, "S2_EXECUTE")
	})
}

func TestRunRejectsInvalidAllowedResponse(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "test allowed responses")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:     "task-02",
			PausedWaveIndex:  2,
			CheckpointType:   "decision",
			AllowedResponses: []string{"approve", "reject", "defer"},
		}
		require.NoError(t, state.SaveChange(root, change))
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-01`"+` first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` decision checkpoint
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		// Invalid response
		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json", "--resume-response", "maybe"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maybe")
		assert.Contains(t, err.Error(), "approve")

		// Valid response (case-insensitive)
		cmd2 := commandForRoot(t, root, makeRunCmd())
		cmd2.SetArgs([]string{"--json", "--resume-response", "Approve"})
		buf.Reset()
		cmd2.SetOut(&buf)
		require.NoError(t, cmd2.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.InputContext.ResumeCheckpoint)
		assert.Equal(t, "Approve", view.InputContext.ResumeCheckpoint.UserResponsePayload)
	})
}

func TestValidateResumeResponseUnit(t *testing.T) {
	t.Parallel()
	t.Run("empty response rejected", func(t *testing.T) {
		cp := &model.ActiveCheckpoint{
			PausedTaskID:   "task-x",
			CheckpointType: "human_verify",
		}
		err := validateResumeResponse(cp, "")
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_required", cliErr.ErrorCode)
		assert.Equal(t, categoryInvalidUsage, cliErr.Category)
		assert.Equal(t, exitCodeInvalidUsage, cliErr.ExitCode)
	})

	t.Run("free-form response accepted when no allowed list", func(t *testing.T) {
		cp := &model.ActiveCheckpoint{
			PausedTaskID:   "task-x",
			CheckpointType: "human_verify",
		}
		err := validateResumeResponse(cp, "looks good")
		require.NoError(t, err)
	})

	t.Run("response must match allowed list", func(t *testing.T) {
		cp := &model.ActiveCheckpoint{
			PausedTaskID:     "task-x",
			CheckpointType:   "decision",
			AllowedResponses: []string{"yes", "no"},
		}
		require.NoError(t, validateResumeResponse(cp, "yes"))
		require.NoError(t, validateResumeResponse(cp, "YES"))
		err := validateResumeResponse(cp, "maybe")
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "resume_response_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryInvalidUsage, cliErr.Category)
		assert.Equal(t, exitCodeInvalidUsage, cliErr.ExitCode)
	})

	t.Run("decision checkpoints require configured allowed responses", func(t *testing.T) {
		cp := &model.ActiveCheckpoint{
			PausedTaskID:   "task-x",
			CheckpointType: "decision",
		}
		err := validateResumeResponse(cp, "yes")
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "checkpoint_config_invalid", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
		assert.Equal(t, exitCodeStateIntegrity, cliErr.ExitCode)
	})
}

func TestShouldStopRunLoopOnlyForPendingCheckpoint(t *testing.T) {
	t.Parallel()
	t.Run("informational resume progress does not stop run", func(t *testing.T) {
		view := nextView{
			CurrentState: model.StateS2Execute,
			Advanced:     &progression.AdvanceSummary{Action: "advanced"},
			InputContext: nextContext{
				ResumeCheckpoint: &resumeCheckpoint{
					RunSummaryVersion: 1,
					CompletedTaskIDs:  []string{"task-01"},
					ResumeWaveIndex:   2,
				},
			},
		}
		assert.False(t, shouldStopRunLoop(view))
	})

	t.Run("checkpoint response payload does not stop run", func(t *testing.T) {
		view := nextView{
			CurrentState: model.StateS2Execute,
			Advanced:     &progression.AdvanceSummary{Action: "advanced"},
			InputContext: nextContext{
				ResumeCheckpoint: &resumeCheckpoint{
					PausedTaskID:        "task-02",
					PausedWaveIndex:     2,
					CheckpointType:      string(model.CheckpointHumanVerify),
					UserResponsePayload: "approved",
				},
			},
		}
		assert.False(t, shouldStopRunLoop(view))
	})

	t.Run("pending checkpoint still stops run", func(t *testing.T) {
		view := nextView{
			CurrentState: model.StateS2Execute,
			Advanced:     &progression.AdvanceSummary{Action: "advanced"},
			InputContext: nextContext{
				ResumeCheckpoint: &resumeCheckpoint{
					PausedTaskID:    "task-02",
					PausedWaveIndex: 2,
					CheckpointType:  string(model.CheckpointHumanVerify),
				},
			},
		}
		assert.True(t, shouldStopRunLoop(view))
	})
}

func TestNextIncludesFreshnessInResumeCheckpoint(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "test freshness in checkpoint")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Set to wave execution state with persisted execution summary
		// and some completed tasks to trigger resume checkpoint.
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` refresh checkpoint freshness
  - wave: 1
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

		require.NotNil(t, view.InputContext.ResumeCheckpoint)
		assert.NotEmpty(t, view.InputContext.ResumeCheckpoint.Freshness,
			"resume checkpoint should include freshness field")
		assert.Contains(t, []string{"fresh", "stale", "unknown"},
			view.InputContext.ResumeCheckpoint.Freshness)
	})
}

func TestNextDoesNotBuildResumeCheckpointFromChecklistWithoutReadyExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "bundle checklist resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
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
  - wave: 1
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code

- [ ] `+"`t-02`"+` rerun verification
  - wave: 2
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

		assert.Nil(t, view.InputContext.ResumeCheckpoint)
	})
}

func TestNextDoesNotRetainResumeCheckpointWhenOnlyChecklistMarksTasksComplete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "bundle checkpoint without skip-safe tasks")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
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
  - wave: 1
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: verification

- [ ] `+"`t-next`"+` continue execution
  - wave: 2
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

		assert.Nil(t, view.InputContext.ResumeCheckpoint)
	})
}

func TestNextPreviewIncludesWavePlanTaskShape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "wave plan protocol version")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [ ] `+"`t-01`"+` execute schema-tightened wave task
  - wave: 1
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

func TestNextPreviewUsesAuthoritativeWavePlanDuringExecution(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "authoritative wave plan should win during execution")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [ ] `+"`t-01`"+` authoritative wave task
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [ ] `+"`t-01`"+` mutated tasks.md should not replace authoritative wave plan
  - wave: 1
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
		assert.Equal(t, "authoritative wave task", firstTask["objective"])
		assert.Equal(t, []any{"cmd/next.go"}, firstTask["target_files"])
	})
}

func TestNextPreviewIncludesActiveCheckpointBundle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "preview checkpoint bundle")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "task-09",
			PausedWaveIndex: 2,
			CheckpointType:  "human_verify",
		}
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

- [x] `+"`task-01`"+` preserve preview checkpoint context
  - wave: 1
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code

- [ ] `+"`task-09`"+` pending checkpoint task
  - wave: 2
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

		require.NotNil(t, view.InputContext.ResumeCheckpoint)
		assert.Equal(t, 3, view.InputContext.ResumeCheckpoint.RunSummaryVersion)
		assert.Equal(t, "task-09", view.InputContext.ResumeCheckpoint.PausedTaskID)
		assert.Equal(t, "human_verify", view.InputContext.ResumeCheckpoint.CheckpointType)
		assert.Equal(t, []string{"task-01"}, view.InputContext.ResumeCheckpoint.CompletedTaskIDs)
		assert.NotEmpty(t, view.InputContext.ResumeCheckpoint.Freshness)

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.NotNil(t, after.ActiveCheckpoint, "preview mode must not clear active checkpoint")
		assert.Equal(t, "task-09", after.ActiveCheckpoint.PausedTaskID)
	})
}

func TestNextIncludesActiveCheckpointWithoutRequiringResumeResponse(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "next should inspect active checkpoint without resume response")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:    "task-02",
			PausedWaveIndex: 2,
			CheckpointType:  "human_verify",
		}
		require.NoError(t, state.SaveChange(root, change))
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-01`"+` first wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` active checkpoint task
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.InputContext.ResumeCheckpoint)
		assert.Equal(t, "task-02", view.InputContext.ResumeCheckpoint.PausedTaskID)
		assert.Equal(t, "human_verify", view.InputContext.ResumeCheckpoint.CheckpointType)
		assert.Empty(t, view.InputContext.ResumeCheckpoint.UserResponsePayload)

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.NotNil(t, after.ActiveCheckpoint, "next inspection must not consume the active checkpoint")
		assert.Equal(t, "task-02", after.ActiveCheckpoint.PausedTaskID)
	})
}

func TestNextResumeCheckpointFreshnessTurnsStaleAfterInputUpdate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "freshness stale after input update")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
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
- [x] `+"`task-01`"+` preserve stale freshness on resume
  - wave: 1
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
		require.NotNil(t, view.InputContext.ResumeCheckpoint)
		assert.Equal(t, "stale", view.InputContext.ResumeCheckpoint.Freshness)
	})
}

func TestNextPreviewAdvancesAfterPassingResearchVerification(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, "L3", "passing research verification should advance to next skill")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		worktreeRoot := filepath.Join(t.TempDir(), change.Slug)
		branch := "feat/" + change.Slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
		change.WorktreePath = worktreeRoot
		change.WorktreeBranch = branch
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

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

	// Missing files — expanded schema requires all 6 for L2
	assert.False(t, progression.CheckGovernedBundleReady(root, change))

	// L2 expanded schema: change.yaml, intent.md, requirements.md, decision.md, tasks.md, assurance.md
	l2Required := []string{"change.yaml", "intent.md", "requirements.md", "decision.md", "tasks.md", "assurance.md"}
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

	// Discovery-mode expanded schema adds research.md to the base set
	l2Files := []string{"change.yaml", "intent.md", "requirements.md", "decision.md", "tasks.md", "assurance.md"}
	for _, f := range l2Files {
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

func TestCheckGovernedBundleReadyRequiresAssuranceArtifact(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-bundle-requires-assurance"
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))

	change := model.Change{Slug: slug, ArtifactSchema: model.ArtifactSchemaExpanded}
	for _, f := range []string{"change.yaml", "intent.md", "requirements.md", "decision.md", "tasks.md"} {
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, f, []byte("x")))
	}
	assert.False(t, progression.CheckGovernedBundleReady(root, change))
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
		slug := createGovernedRequest(t, root, "L2", "preview should be read-only")

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
		slug := createGovernedRequest(t, root, "L2", "preview should not append lifecycle events")

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
		slug := createGovernedRequest(t, root, "L2", "preview should expose artifact amendments without persisting")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		intentPath := artifact.ResolveArtifactPath(bundlePath, slug, "intent.md")
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
		slug := createGovernedRequest(t, root, "L2", "run transition trace")

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
		t.Setenv("SPECLANE_CONTEXT_WINDOW_TOKENS", "1")
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "context hard stop")
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
		slug := createGovernedRequest(t, root, "L2", "document diagnostics warning")

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

func TestNextS6GovernedMaterializesExecutionSummaryAndRuntimeSummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "materialize run summary")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{"task:evidence"},
	})
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
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-a`"+` materialize run summary
  - wave: 1
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	_, err = buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
	require.NoError(t, err)

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.RunSummaryVersion)
	require.Len(t, summary.Tasks, 1)
	assert.Equal(t, "task-a", summary.Tasks[0].TaskID)
	assert.Equal(t, model.TaskVerdictPass, summary.Tasks[0].Verdict)
}

func TestNextS6GovernedBlocksWithoutTaskEvidenceForWaveRunSummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	slug := createGovernedRequest(t, root, "L2", "missing task evidence should block")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeSkillVerification(t, root, slug, "wave-orchestration", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
	})

	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
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
	assert.Contains(t, missingEvidenceBlocker, "required_fields=task_id,run_summary_version,task_kind,verdict,evidence_ref,captured_at,freshness_inputs")
	assert.Equal(t, model.StateS2Execute, view.CurrentState)
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
		payload["captured_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if _, ok := payload["freshness_inputs"]; !ok {
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		payload["freshness_inputs"] = state.ExpectedExecutionTaskFreshnessInputs(change, runSummaryVersion, taskID)
	}
	dir := state.EvidenceTasksDir(root, slug)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, taskID+".json")
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o644))
}

func writeBundleArtifactFile(bundlePath, slug, artifactName string, content []byte) error {
	path := artifact.ResolveArtifactPath(bundlePath, slug, artifactName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}
