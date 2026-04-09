package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextReturnsNextSkillForGovernedState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "add caching layer")

		cmd := makeNextCmd()
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
			assert.NotEmpty(t, view.NextSkill.PromptPath)
		}
	})
}

func TestNextPreviewIncludesGovernanceSurfaceAndActionBlockers(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "governance blocker preview")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepResearch
		change.ArtifactSchema = model.ArtifactSchemaCore
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview", "--change", slug})
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
		assert.True(t, os.IsNotExist(err), "next --preview should not persist governance snapshots")
	})
}

func TestNextDoesNotPersistArtifactReconcile(t *testing.T) {
	assertReadOnlyArtifactReconcileDoesNotPersist(
		t,
		"next preview read-only reconcile",
		"fixed-hash-before-next-preview",
		func(out *bytes.Buffer) error {
			cmd := makeNextCmd()
			cmd.SetOut(out)
			cmd.SetArgs([]string{"--json", "--preview"})
			return cmd.Execute()
		},
	)
}

func TestNextPreviewIgnoresUnreadableGovernanceSnapshot(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))

		_, ok := payload["governance_signals"].(map[string]any)
		require.True(t, ok, "expected governance_signals in next output")

		raw, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		assert.Equal(t, "version: [", string(raw), "next --preview should not repair or rewrite snapshot cache")
	})
}

func TestNextPreviewExposesPlanningRecoveryState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "preview should expose plan recovery state")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepValidate
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.PlanSubStepValidate, view.PlanSubStep)
		assert.Contains(t, view.PlanningNote, "recovery-only")
	})
}

func TestNextAutoPassesReviewAndVerifyForLightPreset(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "light preset autopass")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.WorkflowPreset = model.WorkflowPresetLight
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writeShipReadyGovernedBundle(t, root, change)
		writePassingExecutionSummary(t, root, slug, 1, "task-a")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)

		var buf bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.Advanced)
		assert.Equal(t, "done_ready", view.Advanced.Action)
		require.Len(t, view.Advanced.AutoPassedStates, 2)
		assert.Equal(t, model.StateS3Review, view.Advanced.AutoPassedStates[0].State)
		assert.Equal(t, model.StateS4Verify, view.Advanced.AutoPassedStates[1].State)

		updated, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.Len(t, updated.LastAutoPassedStates, 2)
		assert.Equal(t, model.StateS4Verify, updated.CurrentState)
	})
}

func TestNextDoesNotAutoPassLightPresetReviewWithoutExecutionSummary(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "light preset review still requires execution authority")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.WorkflowPreset = model.WorkflowPresetLight
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		var buf bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.StateS3Review, view.CurrentState)
		assert.Nil(t, view.Advanced, "review auto-pass must not bypass missing execution summary")
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.NextSkill.Name)
	})
}

func TestNextDoesNotReturnDoneReadyWithoutGoalVerification(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte("- [ ] `t-01` verify\n  - target_files: [\"cmd/done.go\"]\n  - task_kind: verification\n")))
		writeAssuranceMD(t, root, change.Slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)

		var buf bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, model.StateS4Verify, view.CurrentState)
		assert.Nil(t, view.Advanced, "verify auto-pass must not bypass missing goal-verification evidence")
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillGoalVerification, view.NextSkill.Name)
	})
}

func TestNextDoesNotAutoPassStrictPresetReview(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "strict preset review")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.WorkflowPreset = model.WorkflowPresetStrict
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		var buf bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--change", slug})
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

func TestValidateNextFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		auto           bool
		preview        bool
		resumeResponse string
		contextGuard   bool
		wantMessage    string
	}{
		{
			name:        "auto conflicts with preview",
			auto:        true,
			preview:     true,
			wantMessage: "--auto cannot be used with --preview",
		},
		{
			name:           "auto conflicts with resume response",
			auto:           true,
			resumeResponse: "approved",
			wantMessage:    "--auto cannot be used with --resume-response",
		},
		{
			name:         "auto conflicts with context guard",
			auto:         true,
			contextGuard: true,
			wantMessage:  "--auto cannot be used with --context-guard",
		},
		{
			name:         "context guard requires preview",
			contextGuard: true,
			wantMessage:  "--context-guard requires --preview",
		},
		{
			name:         "context guard with preview is valid",
			preview:      true,
			contextGuard: true,
		},
		{
			name:           "resume response without auto is valid",
			resumeResponse: "approved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNextFlags(tt.auto, tt.preview, tt.resumeResponse, tt.contextGuard)
			if tt.wantMessage == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			var cliErr *CLIError
			require.ErrorAs(t, err, &cliErr)
			assert.Equal(t, "flag_conflict", cliErr.ErrorCode)
			assert.Equal(t, tt.wantMessage, cliErr.Message)
		})
	}
}

func TestNextIncludesAgentHint(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "test agent hint")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Set to plan audit state where plan-audit skill runs
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNil(t, view.NextSkill)
		assert.Equal(t, "plan-audit", view.NextSkill.Name)
		assert.Equal(t, "slipway-auditor", view.NextSkill.AgentHint)
	})
}

func TestNextReturnsReviewContextForArtifactReview(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "refactor service module")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Set to review state with guardrail domain
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.GuardrailDomain = "auth_authz"
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNil(t, view.NextSkill)
		assert.Equal(t, "spec-compliance-review", view.NextSkill.Name)
		require.NotNil(t, view.NextSkill.ReviewContext)
		assert.Contains(t, view.NextSkill.ReviewContext.RequiredArtifactLayers, "R0")
		assert.Contains(t, view.NextSkill.ReviewContext.RequiredArtifactLayers, "R3")
		assert.Contains(t, view.NextSkill.ReviewContext.RequiredImplementationLayers, "IR1")
		assert.Contains(t, view.NextSkill.ReviewContext.RequiredImplementationLayers, "IR3")
	})
}

func TestNextNoReviewContextForNonReviewState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "add pagination")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// At plan audit state — no review context
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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
		)
		require.NoError(t, err)
		require.NotNil(t, view.NextSkill)
		assert.Equal(t, progression.SkillCodeQualityReview, view.NextSkill.Name)
	})
}

func TestNextBlocksWithoutPlanAuditEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "test gplan blocking")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Move to S1_PLAN/audit without evidence
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		// Should not have advanced — evidence missing
		assert.Nil(t, view.Advanced)
		assert.Equal(t, model.StateS1Plan, view.CurrentState)
		assert.NotEmpty(t, view.Blockers)
	})
}

func TestNextPreviewFailsWhenSkillEvidenceEvaluationFails(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review malformed skill registry handling")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		skillPath := filepath.Join(root, ".codex", "skills", "slipway", "code-quality-review", "SKILL.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
		require.NoError(t, os.WriteFile(skillPath, []byte(strings.TrimSpace(`
---
name: code-quality-review
description: [
---
`)), 0o644))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview"})
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNil(t, view.Advanced)
		assert.Equal(t, "advanced", view.Advanced.Action)
		assert.Equal(t, model.StateS1Plan, view.Advanced.FromState)
		// Audit clean path: post-audit machine validation runs inline.
		// If validation passes, it advances to S2_EXECUTE.
		// If it fails, it persists at S1_PLAN/validate.
		// This test provides sufficient artifacts for the clean path.
		assert.Equal(t, model.StateS2Execute, view.Advanced.ToState)
		assert.Equal(t, model.StateS2Execute, view.CurrentState)
	})
}

func TestNextBlocksWhenBundleMissingArtifacts(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		// G_plan gate should block — bundle artifacts missing
		assert.Nil(t, view.Advanced)
		assert.NotEmpty(t, view.Blockers)
	})
}

func TestNextBlocksOnInvalidBoundWorktreeBeforeBundleChecks(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
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

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Nil(t, view.Advanced)
		assert.Nil(t, view.NextSkill)
		requireBlockerContains(t, view.Blockers, state.WorktreeReasonDedicatedRequired)
	})
}

func TestNextBlocksWhenTasksChecklistMissingTargetFiles(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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

		cmd := makeNextCmd()
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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

		cmd := makeNextCmd()
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "plan_dimension_dependency_unknown:t-01->t-99")
	})
}

func TestNextPreviewIncludesAssuranceContractBlockersAtReview(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "preview assurance contract blocker")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeAssuranceMD(t, root, slug, "## Scope Summary\nIncomplete\n")

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Contains(t, strings.Join(model.ReasonSpecs(view.Blockers), "\n"), "assurance_structure_invalid:")
	})
}

func TestNextRejectsResumeResponseWithoutCheckpoint(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		_ = createGovernedRequest(t, root, "L2", "test no checkpoint")

		cmd := makeNextCmd()
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNilf(t, view.Advanced, "raw next output: %s", buf.String())
		assert.Equal(t, "done_ready", view.Advanced.Action)
		assert.Equal(t, model.StateS4Verify, view.CurrentState)
		assert.Nil(t, view.NextSkill)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "run_slipway_done_to_finalize")
		assert.Contains(t, view.Warnings, "optional_closeout_available: final-closeout evidence is missing or stale; run final-closeout before `slipway done` only if refreshed closeout evidence is desired")
	})
}

func TestNextReturnsDoneReadyWithoutFinalCloseoutRequirementForStandardRequestPath(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNilf(t, view.Advanced, "raw next output: %s", buf.String())
		assert.Equal(t, "done_ready", view.Advanced.Action)
		assert.Equal(t, model.StateS4Verify, view.CurrentState)
		assert.Nil(t, view.NextSkill)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "run_slipway_done_to_finalize")
		assert.NotContains(t, model.ReasonSpecs(view.Blockers), "ship_gate_blocked:required_skill_missing:final-closeout")
	})
}

func TestNextRequiresResumeResponseForActiveCheckpoint(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--resume-response")
		assert.Contains(t, err.Error(), "task-01")
	})
}

func TestNextResumesCheckpointWithValidResponse(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "test checkpoint resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:   "task-02",
			CheckpointType: "human_verify",
		}
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
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

func TestNextRejectsInvalidAllowedResponse(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "test allowed responses")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:     "task-03",
			CheckpointType:   "decision",
			AllowedResponses: []string{"approve", "reject", "defer"},
		}
		require.NoError(t, state.SaveChange(root, change))

		// Invalid response
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--resume-response", "maybe"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maybe")
		assert.Contains(t, err.Error(), "approve")

		// Valid response (case-insensitive)
		cmd2 := makeNextCmd()
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

func TestNextIncludesFreshnessInResumeCheckpoint(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code
`)))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code

- [ ] `+"`t-02`"+` rerun verification
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: verification
`)))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview"})
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

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
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: verification

- [ ] `+"`t-next`"+` continue execution
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code
`)))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		assert.Nil(t, view.InputContext.ResumeCheckpoint)
	})
}

func TestNextPreviewIncludesWavePlanTaskShape(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "wave plan protocol version")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [ ] `+"`t-01`"+` execute schema-tightened wave task
  - depends_on: []
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview", "--change", slug})
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

func TestNextPreviewIncludesActiveCheckpointBundle(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "preview checkpoint bundle")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:   "task-09",
			CheckpointType: "human_verify",
		}
		require.NoError(t, state.SaveChange(root, change))
		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 3,
			CapturedAt:        time.Now().UTC(),
			OverallVerdict:    model.ExecutionVerdictPass,
			Tasks:             []model.ExecutionTaskSummary{},
		})

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))

		require.NotNil(t, view.InputContext.ResumeCheckpoint)
		assert.Equal(t, 3, view.InputContext.ResumeCheckpoint.RunSummaryVersion)
		assert.Equal(t, "task-09", view.InputContext.ResumeCheckpoint.PausedTaskID)
		assert.Equal(t, "human_verify", view.InputContext.ResumeCheckpoint.CheckpointType)

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.NotNil(t, after.ActiveCheckpoint, "preview mode must not clear active checkpoint")
		assert.Equal(t, "task-09", after.ActiveCheckpoint.PausedTaskID)
	})
}

func TestNextResumeCheckpointFreshnessTurnsStaleAfterInputUpdate(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "freshness stale after input update")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone

		originalHash, err := state.ComputeTaskEvidenceInputHash(
			change.Slug,
			1,
			"task-01",
			change.GuardrailDomain,
		)
		require.NoError(t, err)

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug, 1), "task-01.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(taskEvidencePath), 0o755))
		taskEvidence := map[string]any{
			"task_id":             "task-01",
			"run_summary_version": 1,
			"input_hash":          originalHash,
			"captured_at":         time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano),
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
					TaskID:            "task-01",
					Verdict:           model.TaskVerdictPass,
					TaskKind:          model.TaskKindCode,
					EvidenceRef:       taskEvidencePath,
					EvidenceInputHash: originalHash,
					CapturedAt:        oldTS,
				},
			},
		})
		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, change.Slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` preserve stale freshness on resume
  - target_files: ["cmd/next_context_build.go"]
  - task_kind: code
`)))

		// Simulate a relevant input update after evidence was captured.
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview"})
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
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
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		require.NotNil(t, view.NextSkill)
		// In preview mode, skill resolution reflects the current substep.
		// At S1_PLAN/research, the skill is research-orchestration.
		assert.Equal(t, progression.SkillResearchOrchestration, view.NextSkill.Name)
	})
}

func TestCheckGovernedBundleReadyL2(t *testing.T) {
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "preview should be read-only")

		changeBefore, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.Equal(t, model.StateS1Plan, changeBefore.CurrentState)

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Nil(t, view.Advanced)

		changeAfter, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, changeBefore.CurrentState, changeAfter.CurrentState)
	})
}

func TestNextAutoIncludesTransitionTrace(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "auto transition trace")

		// createGovernedRequest runs request + one next, leaving governed lane at S1.
		changeBefore, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.Equal(t, model.StateS1Plan, changeBefore.CurrentState)

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--auto"})
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

func TestNextContextBudgetHardStopAddsBlocker(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		t.Setenv("SPECLANE_CONTEXT_WINDOW_TOKENS", "1")
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "context hard stop")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		require.NotNil(t, view.ContextBudget)
		assert.Equal(t, "stop", view.ContextBudget.GuardAction)
		assert.Contains(t, strings.Join(model.ReasonSpecs(view.Blockers), "\n"), "hard stop")
	})
}

func TestNextBlocksWhenGovernedChangeHasNoFrozenSchema(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "document diagnostics warning")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.ArtifactSchema = ""
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepValidate
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--preview"})
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
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
		})

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		summary, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		assert.Equal(t, 1, summary.RunSummaryVersion)
		require.Len(t, summary.Tasks, 1)
		assert.Equal(t, "task-a", summary.Tasks[0].TaskID)
		assert.Equal(t, model.TaskVerdictPass, summary.Tasks[0].Verdict)

	})
}

func TestNextS6GovernedBlocksWithoutTaskEvidenceForWaveRunSummary(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
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

		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "missing_task_evidence_for_run_summary:rv1")
		assert.Equal(t, model.StateS2Execute, view.CurrentState)
	})
}

func writeTaskEvidenceFile(t *testing.T, root, slug string, runSummaryVersion int, taskID string, payload map[string]any) {
	t.Helper()
	dir := state.EvidenceTasksDir(root, slug, runSummaryVersion)
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
