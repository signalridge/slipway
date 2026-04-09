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
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBlocksWhenGovernedBundleIsIncompleteAtSpecBundle(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "validate should gate incomplete bundle")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.Remove(artifact.ResolveArtifactPath(bundlePath, change.Slug, "decision.md")))

		var out bytes.Buffer
		cmd := makeValidateCmd()
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
	})
}

func TestNextBlocksWhenGovernedBundleIsIncompleteAtSpecBundle(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "next should gate incomplete bundle")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", change.Slug)
		require.NoError(t, os.Remove(artifact.ResolveArtifactPath(bundlePath, change.Slug, "decision.md")))

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "governed", view.ExecutionMode)
		assert.Equal(t, model.StateS1Plan, view.CurrentState)
		assert.Nil(t, view.Advanced)
		assert.Nil(t, view.NextSkill)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "missing_required_artifact:decision.md")
	})
}

func TestValidateBlocksPlanAuditAdvanceWhenArtifactsAreMissingEvenIfSkillIsReady(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "validate should gate missing artifacts at plan audit")
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
		require.NoError(t, os.Remove(artifact.ResolveArtifactPath(bundlePath, change.Slug, "decision.md")))

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "pass", view.SkillsReady["plan-audit"])
		assert.False(t, view.CanAdvance)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "missing_required_artifact:decision.md")
	})
}

func TestValidateUsesFilesystemArtifactReadinessWithoutPersistingReconcile(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "validate should use filesystem artifact readiness")
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
		require.NoError(t, os.Remove(artifact.ResolveArtifactPath(bundlePath, change.Slug, "decision.md")))

		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "missing_required_artifact:decision.md")

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		decision, ok := after.Artifacts["decision"]
		require.True(t, ok)
		assert.Equal(t, model.ArtifactLifecycleApproved, decision.State)
		assert.Equal(t, "stale-approved-hash", decision.ContentHash)
	})
}

func TestValidateOnlyRequiresActivePlanningSkillAtPlanAudit(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "validate should scope skill blockers to the active plan sub-step")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--change", slug})
		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:plan-audit")
		assert.NotContains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:research-orchestration")
		assert.NotContains(t, model.ReasonSpecs(view.Blockers), "required_skill_missing:scope-confirmation")
	})
}

func TestValidateSkillsReadyScopesToActivePlanningSubStep(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L3", "validate should scope passing skills to the active plan sub-step")
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
	})
}

func TestValidateExposesPlanningRecoveryState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "validate should expose plan recovery state")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepValidate
		require.NoError(t, state.SaveChange(root, change))

		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.PlanSubStepValidate, view.PlanSubStep)
		assert.Contains(t, view.PlanningNote, "recovery-only")
	})
}

func TestValidateDoesNotLeakBundleBlockersBeforeWorktreeBinding(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L3", "validate should not leak bundle blockers before worktree")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// At S2_EXECUTE with NeedsDiscovery=true and no worktree bound,
		// the worktree gate should be the primary blocker — not missing artifacts.
		change.CurrentState = model.StateS2Execute
		change.IntakeSubStep = ""
		change.PlanSubStep = model.PlanSubStepNone
		change.NeedsDiscovery = true
		require.NoError(t, state.SaveChange(root, change))

		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "dedicated_worktree_metadata_required")
	})
}

func TestValidateIncludesGovernanceActionBlockersAtReviewState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "validate governance action blockers")
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
		requireBlockerContains(t, view.Blockers, "governance_action_required:domain-review")
	})
}

func TestValidateIncludesTaskChecklistAdvisories(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "validate task checklist advisories")
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
	})
}

func TestValidateIncludesTaskChecklistBlockers(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "validate task checklist blockers")
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
	})
}

func TestValidateAtShipGateRequiresReviewEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "validate review evidence at ship gate")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))
		writePassingGoalVerificationEvidence(t, root, slug, 1)

		var out bytes.Buffer
		cmd := makeValidateCmd()
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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "validate should block stale evidence")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.Artifacts = map[string]model.ArtifactState{}
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

		// Write execution summary first.
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		// Modify a bundle artifact after execution to trigger staleness.
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte("# Modified intent\n\nPost-execution change."), 0o644))

		view, err := buildValidateViewForSlug(root, slug)
		require.NoError(t, err)
		assert.Contains(t, model.ReasonSpecs(view.Blockers), "stale_execution_evidence")
		assert.False(t, view.CanAdvance)
	})
}
