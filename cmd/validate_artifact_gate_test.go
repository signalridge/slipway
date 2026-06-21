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
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBlocksWhenGovernedBundleIsIncompleteAtSpecBundle(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "validate should gate incomplete bundle")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.Remove(artifact.ResolveArtifactPath(bundlePath, "decision.md")))

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "governed", view.ExecutionMode)
		assert.Equal(t, model.StateS1Plan, view.CurrentState)
		assert.False(t, view.CanAdvance)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "missing_required_artifact:decision.md")
		require.NotEmpty(t, view.Blockers)
		assert.Equal(t, "missing_required_artifact", view.Blockers[0].Code)
		require.NotNil(t, view.Recovery)
		assert.Contains(t, recoveryStepCodes(view.Recovery), "missing_required_artifact")
		assert.NotEmpty(t, view.Recovery.PrimaryCommand)
	})
}

func TestValidateReportsDeferredArtifactsMissingAfterRealScaffold(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := "real-scaffold-deferred-missing"
	change := model.NewChange(slug)
	change.Description = "validate real scaffold deferred artifacts"
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.QualityMode = model.QualityModeStandard
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	require.NoError(t, state.SaveChange(root, change))

	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, model.WorkflowPresetStandard))
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	for _, file := range []string{"requirements.md", "decision.md", "tasks.md"} {
		_, err := os.Stat(filepath.Join(bundlePath, file))
		require.ErrorIsf(t, err, os.ErrNotExist, "%s must be deferred to skill authoring", file)
	}

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	specs := model.ReasonSpecs(view.Blockers)
	assert.Contains(t, specs, "missing_required_artifact:requirements.md")
	assert.Contains(t, specs, "missing_required_artifact:decision.md")
	assert.Contains(t, specs, "missing_required_artifact:tasks.md")
	require.NotNil(t, view.Recovery)
	assert.Contains(t, recoveryStepCodes(view.Recovery), "missing_required_artifact")
}

func TestValidateAllowsJsonFalseAsDefaultJSONOutput(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetArgs([]string{"--json=false"})
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		assert.Empty(t, view.Mode, "default validate (no --focus) should omit mode")
	})
}

func TestValidatePreAuditDefaultViewOmitsShipGateDebt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate should omit ship gate debt before verify")

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	assert.NotContains(t, view.GateDetails, "G_ship")
	assert.NotContains(t, model.ReasonSpecs(view.Blockers), "plan_dimension_key_links_missing_target_files")
}

func TestNextBlocksWhenGovernedBundleIsIncompleteAtSpecBundle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "next should gate incomplete bundle")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.Remove(artifact.ResolveArtifactPath(bundlePath, "decision.md")))

	view, err := buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
	require.NoError(t, err)

	assert.Equal(t, "governed", view.ExecutionMode)
	assert.Equal(t, model.StateS1Plan, view.CurrentState)
	require.NotNil(t, view.Advanced)
	assert.Equal(t, "blocked", view.Advanced.Action)
	assert.Nil(t, view.NextSkill)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "missing_required_artifact:decision.md")
}

func TestValidateBlocksPlanAuditAdvanceWhenArtifactsAreMissingEvenIfSkillIsReady(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "validate should gate missing artifacts at plan audit")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		require.NoError(t, state.SaveChange(root, change))

		writeSkillVerification(t, root, slug, "plan-audit", model.VerificationRecord{
			Verdict:   model.VerificationVerdictPass,
			Blockers:  []model.ReasonCode{},
			Timestamp: time.Now().UTC(),
		})

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.Remove(artifact.ResolveArtifactPath(bundlePath, "decision.md")))

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "pass", view.SkillsReady["plan-audit"])
		assert.False(t, view.CanAdvance)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "missing_required_artifact:decision.md")
	})
}

func TestValidateBlocksPlanAuditWhenDecisionIsTemplateOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate should gate template decision at plan audit")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "requirements.md", []byte(`# Requirements

### Requirement: Decision substance gate
REQ-001: The system MUST block plan readiness when decision.md contains only template comments.

#### Scenario: Template decision blocks plan readiness
GIVEN decision.md contains only generated guidance comments
WHEN validation previews plan readiness
THEN G_plan remains blocked with a decision contract blocker.
`)))
	templateDecision, err := artifact.RenderArtifactExample("decision.md")
	require.NoError(t, err)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "decision.md", []byte(templateDecision)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` Enforce decision substance in plan readiness
  - depends_on: []
  - target_files: ["internal/engine/progression/readiness.go"]
  - task_kind: code
  - covers: [REQ-001]
`)))
	writeSkillVerification(t, root, slug, "plan-audit", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	})

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)

	assert.False(t, view.CanAdvance)
	requireReasonSpecPrefix(t, model.ReasonSpecs(view.Blockers), "decision_structure_invalid:")
	require.Contains(t, view.GateDetails, "G_plan")
	assert.Equal(t, model.GateStatusBlocked, view.GateDetails["G_plan"].Status)
	requireReasonSpecPrefix(t, model.ReasonSpecs(view.GateDetails["G_plan"].ReasonCodes),
		"decision_structure_invalid:")
}

func requireReasonSpecPrefix(t *testing.T, specs []string, prefix string) {
	t.Helper()
	for _, spec := range specs {
		if strings.HasPrefix(spec, prefix) {
			return
		}
	}
	require.Failf(t, "missing reason prefix", "expected prefix %q in %v", prefix, specs)
}

func TestValidateUsesFilesystemArtifactReadinessWithoutPersistingReconcile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate should use filesystem artifact readiness")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	change.Artifacts["decision"] = model.ArtifactState{
		ID:          "decision",
		Path:        "decision.md",
		State:       model.ArtifactLifecycleApproved,
		ContentHash: "stale-approved-hash",
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.Remove(artifact.ResolveArtifactPath(bundlePath, "decision.md")))

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "missing_required_artifact:decision.md")

	after, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	decision, ok := after.Artifacts["decision"]
	require.True(t, ok)
	assert.Equal(t, model.ArtifactLifecycleApproved, decision.State)
	assert.Equal(t, "stale-approved-hash", decision.ContentHash)
}

func TestValidateExposesArtifactAmendmentsWithoutPersistingReconcile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate should expose artifact amendments")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
	intentPath := artifact.ResolveArtifactPath(bundlePath, "intent.md")
	oldContent := []byte("# Intent\nFrozen validate baseline\n")
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

	require.NoError(t, os.WriteFile(intentPath, []byte("# Intent\nValidate amended content\n"), 0o644))

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	require.Len(t, view.ArtifactAmendments, 1)
	assert.Equal(t, "intent", view.ArtifactAmendments[0].ArtifactID)
	assert.Equal(t, string(model.ArtifactLifecycleFrozen), view.ArtifactAmendments[0].FromState)
	assert.Equal(t, string(model.ArtifactLifecycleApproved), view.ArtifactAmendments[0].ToState)

	after, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	require.Contains(t, after.Artifacts, "intent")
	assert.Equal(t, model.ArtifactLifecycleFrozen, after.Artifacts["intent"].State)
	assert.Equal(t, oldHash, after.Artifacts["intent"].ContentHash)
}

func TestValidateOnlyRequiresActivePlanningSkillAtPlanAudit(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "validate should scope skill blockers to the active plan sub-step")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:plan-audit")
		assert.NotContains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:research-orchestration")
		assert.NotContains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:scope-confirmation")
	})
}

func TestValidateSkillsReadyScopesToActivePlanningSubStep(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelDiscovery, "validate should scope passing skills to the active plan sub-step")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepAudit
	require.NoError(t, state.SaveChange(root, change))

	writeSkillVerification(t, root, slug, "plan-audit", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC(),
	})
	writeSkillVerification(t, root, slug, "research-orchestration", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: time.Now().UTC().Add(time.Second),
	})

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	assert.Equal(t, "pass", view.SkillsReady["plan-audit"])
	assert.NotContains(t, view.SkillsReady, "research-orchestration")
}

func TestValidateExposesPlanningRecoveryState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate should expose plan recovery state")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepValidate
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.PlanSubStepValidate, view.PlanSubStep)
	assert.Contains(t, view.PlanningNote, "recovery-only")
}

func TestValidateDoesNotLeakBundleBlockersBeforeWorktreeBinding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelDiscovery, "validate should not leak bundle blockers before worktree")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	// At S2_IMPLEMENT with NeedsDiscovery=true and no worktree bound,
	// the worktree gate should be the primary blocker — not missing artifacts.
	change.CurrentState = model.StateS2Implement
	change.IntakeSubStep = ""
	change.PlanSubStep = model.PlanSubStepNone
	change.NeedsDiscovery = true
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "dedicated_worktree_metadata_required")
}

func TestValidateIncludesGovernanceActionBlockersAtReviewState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate governance action blockers")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = "auth_authz"
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	require.NoError(t, state.SaveChange(root, change))
	writeAuthReviewGovernedBundle(t, root, slug)

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	assert.False(t, view.CanAdvance)
}

func TestValidateIncludesTaskChecklistAdvisories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate task checklist advisories")
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
  - depends_on: []
  - target_files: [cmd/validate.go]
`), 0o644))

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	diagnostics := strings.Join(view.Diagnostics, "\n")
	assert.Contains(t, diagnostics, "plan_dimension_context_missing_task_kind_warning:t-01")
	assert.Contains(t, diagnostics, "plan_dimension_coverage_missing_requirement_warning:REQ-001")
}

func TestValidateIncludesTaskChecklistBlockers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate task checklist blockers")
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
  - target_files: [cmd/validate.go]
`), 0o644))

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "plan_dimension_dependency_unknown:t-01->t-99")
	require.NotNil(t, view.Recovery)
	assert.Contains(t, recoveryStepCodes(view.Recovery), "plan_dimension_dependency_unknown")
	assert.NotEmpty(t, view.Recovery.PrimaryCommand)
}

func TestValidateAtShipGateRequiresReviewEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "validate review evidence at ship gate")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "pass", view.SkillsReady["goal-verification"])
		assert.False(t, view.CanAdvance)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:spec-compliance-review")
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:code-quality-review")
		require.NotEmpty(t, view.Blockers)
		// Don't assume ordering — governance_action_required may appear before
		// required_skill_missing when blast radius degrades to medium (Plan B).
		var hasSkillMissing bool
		for _, r := range view.Blockers {
			if r.Code == "required_skill_missing" {
				hasSkillMissing = true
				break
			}
		}
		assert.True(t, hasSkillMissing, "expected required_skill_missing in blockers, got %v", view.Blockers)
	})
}

func TestValidateBlocksWhenExecutionEvidenceIsStale(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate should block stale evidence")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS2Implement
	change.Artifacts = map[string]model.ArtifactState{}
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

	// Write execution summary first.
	writePassingExecutionSummary(t, root, slug, 1, "t-01")

	// Modify the semantic task plan after execution to trigger planning staleness.
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` validate stale planning evidence
  - depends_on: []
  - target_files: ["cmd/validate.go"]
  - task_kind: verification
  - covers: [REQ-001]
`), 0o644))

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)
	assert.Contains(t, model.ReasonSpecs(view.Blockers), "stale_planning_evidence")
	assert.False(t, view.CanAdvance)
}

func TestValidateTreatsS3TaskPlanDriftAsReviewInput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug, _ := prepareStalePlanningRecoveryFixture(t, root, model.StateS3Review)

	view, err := buildValidateViewForSlug(root, slug)
	require.NoError(t, err)

	reasons := strings.Join(model.ReasonSpecs(view.Blockers), "\n")
	assert.NotContains(t, reasons, state.StalePlanningEvidenceBlockerToken)
	assert.NotContains(t, reasons, state.StaleExecutionEvidenceBlockerToken)
	assert.Equal(t, "fresh", view.EvidenceFreshness)
	require.NotNil(t, view.FreshnessDiagnostics)
	assert.Equal(t, "fresh", view.FreshnessDiagnostics.Status)
	assert.Empty(t, view.FreshnessDiagnostics.StalePairs)
	assert.Contains(t, view.Diagnostics, state.S3TaskPlanAmendmentDiagnostic)

	for _, action := range view.RequiredActions {
		assert.NotContains(t, action.Description, state.StalePlanningEvidenceBlockerToken)
		assert.NotContains(t, action.Description, state.StaleExecutionEvidenceBlockerToken)
	}
}
