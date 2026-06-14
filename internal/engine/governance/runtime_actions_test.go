package governance

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRuntimeRequiredActionsUsesEvidenceReadinessAndRollbackDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitRepoForRuntimeActionsTests(t, root)
	change := model.NewChange("runtime-actions")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = filepath.Join(t.TempDir(), change.Slug)
	change.WorktreeBranch = "feat/" + change.Slug
	runGitForRuntimeActionsTests(t, root, "worktree", "add", change.WorktreePath, "-b", change.WorktreeBranch)
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("# Intent\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Keep the current path.
### Option B
Refactor later.

## Selected Approach
Option A.

## Interfaces and Data Flow
Stable.

## Rollout and Rollback
Rollback is documented here.

## Risk
Contained.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Scope Summary
Done.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
REQ-001: verified

## Residual Risks and Exceptions
Pending.

## Archive Decision
Pending.
`), 0o644))
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 2,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "task-a",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"internal/runtime_actions.go"},
				CapturedAt:   time.Now().UTC(),
			},
		},
	}))

	writeGovernanceVerification(t, root, change.Slug, skillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 2,
		References: []string{"run_summary_version=2"},
	})
	writeGovernanceVerification(t, root, change.Slug, skillSpecComplianceReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Add(time.Second),
		RunVersion: 1, // stale
		References: []string{"layer:R0=pass"},
	})
	writeGovernanceVerification(t, root, change.Slug, skillCodeQualityReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Add(2 * time.Second),
		RunVersion: 2,
		References: []string{"layer:QUALITY=pass"},
	})

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{
			BlastRadius: model.SignalLevelMedium,
		},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlDomainReview, model.ControlModeBlocking, model.ControlScopeReview),
			makeControl(model.ControlIndependentReview, model.ControlModeBlocking, model.ControlScopeReview),
			makeControl(model.ControlWorktreeIsolation, model.ControlModeBlocking, model.ControlScopeExecution),
			makeControl(model.ControlRollbackRequired, model.ControlModeAdvisory, model.ControlScopeRelease),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	byID := map[model.ControlID]RequiredAction{}
	for _, action := range actions {
		byID[action.ControlID] = action
	}

	assert.False(t, byID[model.ControlDomainReview].Satisfied, "review evidence with stale run summary should not satisfy domain-review")
	assert.Empty(t, byID[model.ControlDomainReview].SatisfiedBy, "stale review evidence must not be reported as satisfying domain-review")
	assert.Contains(t, byID[model.ControlDomainReview].Description, "run_version_mismatch")
	assert.True(t, byID[model.ControlIndependentReview].Satisfied, "independent reviewer evidence with current run summary should satisfy independent-review")
	assert.True(t, byID[model.ControlWorktreeIsolation].Satisfied, "worktree-isolation is always satisfied after worktree-preflight removal")
	assert.False(t, byID[model.ControlRollbackRequired].Satisfied, "rollback-required must require explicit rollback documentation in assurance.md, not just requirement coverage")
}

func TestResolveRuntimeRequiredActionsExplainsDomainReviewSatisfiedBySpecCompliance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitRepoForRuntimeActionsTests(t, root)
	change := model.NewChange("runtime-actions-domain-review")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 3,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "task-a",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"internal/engine/governance/runtime_actions.go"},
				CapturedAt:   time.Now().UTC(),
			},
		},
	}))
	writeGovernanceVerification(t, root, change.Slug, skillSpecComplianceReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 3,
		References: []string{"layer:R0=pass"},
	})

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlDomainReview, model.ControlModeBlocking, model.ControlScopeReview),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	require.Len(t, actions, 1)
	action := actions[0]
	require.True(t, action.Satisfied)
	require.Len(t, action.SatisfiedBy, 1)
	assert.Equal(t, "skill_evidence", action.SatisfiedBy[0].Kind)
	assert.Equal(t, skillSpecComplianceReview, action.SatisfiedBy[0].Name)
	assert.Equal(t, "artifacts/changes/"+change.Slug+"/verification/spec-compliance-review.yaml", action.SatisfiedBy[0].EvidenceRef)
	assert.Equal(t, "spec-compliance-review provides the domain-aware review evidence for domain-review", action.SatisfiedBy[0].Reason)
}

func TestResolveRuntimeRequiredActionsRejectsTemplateOnlyRollbackSections(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("runtime-actions-template-rollback")
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Keep the current path.
### Option B
Refactor later.

## Selected Approach
Option A.

## Interfaces and Data Flow
Stable.

## Rollout and Rollback
Describe rollout sequencing, safeguards, and how the change would be rolled back if verification fails.

## Risk
Contained.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Scope Summary
Done.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
Pending.

## Residual Risks and Exceptions
Pending.

## Rollback Readiness
Summarize rollback constraints, prerequisites, and verification status when rollback planning is required.

## Archive Decision
Pending.
`), 0o644))

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{
			BlastRadius: model.SignalLevelLow,
		},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlRollbackRequired, model.ControlModeAdvisory, model.ControlScopeRelease),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	require.Len(t, actions, 1)
	assert.False(t, actions[0].Satisfied, "template placeholder headings must not satisfy rollback-required")
}

func TestResolveRuntimeRequiredActionsRejectsSeededDraftRollbackDecision(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("runtime-actions-seeded-rollback")
	change.Description = "update auth middleware timeout strategy"
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Keep the current path.
### Option B
Refactor later.

## Selected Approach
Option A.

## Interfaces and Data Flow
Stable.

## Rollout and Rollback
Rollback by restoring the previous behavior around update auth middleware timeout strategy and rerunning the normal verification flow before resuming ship readiness. Confirm or replace this after rollout planning.

## Risk
Contained.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte(`# Assurance
## Scope Summary
Done.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
Pending.

## Residual Risks and Exceptions
Pending.

## Rollback Readiness
Rollback owner, prerequisites, and verification checklist are documented here.

## Archive Decision
Pending.
`), 0o644))

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{
			BlastRadius: model.SignalLevelLow,
		},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlRollbackRequired, model.ControlModeAdvisory, model.ControlScopeRelease),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	require.Len(t, actions, 1)
	assert.False(t, actions[0].Satisfied, "seeded rollback draft text must not satisfy rollback-required until the decision section is explicitly confirmed")
}

func TestResolveRuntimeRequiredActionsRequiresBoundWorktreeMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("runtime-actions-worktree-metadata")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{
			BlastRadius: model.SignalLevelMedium,
		},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlWorktreeIsolation, model.ControlModeBlocking, model.ControlScopeExecution),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	require.Len(t, actions, 1)
	assert.False(t, actions[0].Satisfied, "worktree-isolation should remain unsatisfied until the change is bound to a dedicated worktree")
}

func TestResolveRuntimeRequiredActionsHandlesMissingVerification(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("runtime-actions-no-verification")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{
			BlastRadius: model.SignalLevelMedium,
		},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlDomainReview, model.ControlModeBlocking, model.ControlScopeReview),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	require.Len(t, actions, 1)
	assert.False(t, actions[0].Satisfied, "no verification records should leave domain-review unsatisfied")
}

func TestResolveRuntimeRequiredActionsFailsClosedWhenRunSummaryIsMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("runtime-actions-missing-summary")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeGovernanceVerification(t, root, change.Slug, skillSpecComplianceReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 5,
	})
	writeGovernanceVerification(t, root, change.Slug, skillCodeQualityReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 5,
	})

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{
			BlastRadius: model.SignalLevelMedium,
		},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlDomainReview, model.ControlModeBlocking, model.ControlScopeReview),
			makeControl(model.ControlIndependentReview, model.ControlModeBlocking, model.ControlScopeReview),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	byID := map[model.ControlID]RequiredAction{}
	for _, action := range actions {
		byID[action.ControlID] = action
	}

	assert.False(t, byID[model.ControlDomainReview].Satisfied)
	assert.Contains(t, byID[model.ControlDomainReview].Description, "run_summary_missing")
	assert.False(t, byID[model.ControlIndependentReview].Satisfied)
	assert.Contains(t, byID[model.ControlIndependentReview].Description, "run_summary_missing")
}

func TestResolveRuntimeRequiredActionsRejectsExecutionSummaryLevelBlockers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("runtime-actions-summary-blockers")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	const blocker = "session_isolation_warning:session_id=abc:shared_by=task-a,task-b"
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 3,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		OpenBlockers:      []model.ReasonCode{model.NewReasonCode("session_isolation_warning", "session_id=abc:shared_by=task-a,task-b")},
	}))

	writeGovernanceVerification(t, root, change.Slug, skillSpecComplianceReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 3,
	})
	writeGovernanceVerification(t, root, change.Slug, skillCodeQualityReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 3,
	})

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlDomainReview, model.ControlModeBlocking, model.ControlScopeReview),
			makeControl(model.ControlIndependentReview, model.ControlModeBlocking, model.ControlScopeReview),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	byID := map[model.ControlID]RequiredAction{}
	for _, action := range actions {
		byID[action.ControlID] = action
	}

	require.Contains(t, byID, model.ControlDomainReview)
	require.Contains(t, byID, model.ControlIndependentReview)
	assert.False(t, byID[model.ControlDomainReview].Satisfied)
	assert.Contains(t, byID[model.ControlDomainReview].Description, blocker)
	assert.False(t, byID[model.ControlIndependentReview].Satisfied)
	assert.Contains(t, byID[model.ControlIndependentReview].Description, blocker)
}

func TestResolveRuntimeRequiredActionsUsesAuthoritativeChangeVerificationsForHiddenSiblingWorktree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitRepoForRuntimeActionsTests(t, root)

	change := model.NewChange("runtime-actions-hidden-worktree")
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	worktreeRoot := filepath.Join(t.TempDir(), change.Slug)
	branch := "feat/" + change.Slug
	runGitForRuntimeActionsTests(t, root, "worktree", "add", worktreeRoot, "-b", branch)
	normalizedWT, err := state.NormalizePath(worktreeRoot)
	require.NoError(t, err)

	beforeWorktree := change
	change.WorktreePath = normalizedWT
	change.WorktreeBranch = branch
	require.NoError(t, state.RelocateGovernedBundle(root, beforeWorktree, change))
	require.NoError(t, state.SaveChange(root, change))

	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 2,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "task-a",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"internal/runtime_actions.go"},
				CapturedAt:   time.Now().UTC(),
			},
		},
	}))
	writeGovernanceVerification(t, root, change.Slug, skillSpecComplianceReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 2,
		References: []string{"layer:R0=pass"},
	})
	writeGovernanceVerification(t, root, change.Slug, skillCodeQualityReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Add(time.Second),
		RunVersion: 2,
		References: []string{"layer:QUALITY=pass"},
	})

	require.NoError(t, os.Remove(filepath.Join(normalizedWT, ".slipway.yaml")))

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlDomainReview, model.ControlModeBlocking, model.ControlScopeReview),
			makeControl(model.ControlIndependentReview, model.ControlModeBlocking, model.ControlScopeReview),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	byID := map[model.ControlID]RequiredAction{}
	for _, action := range actions {
		byID[action.ControlID] = action
	}

	require.Contains(t, byID, model.ControlDomainReview)
	require.Contains(t, byID, model.ControlIndependentReview)
	assert.True(t, byID[model.ControlDomainReview].Satisfied, "authoritative hidden-worktree verification should still satisfy domain-review")
	assert.True(t, byID[model.ControlIndependentReview].Satisfied, "authoritative hidden-worktree verification should still satisfy independent-review")
	assert.NotContains(t, byID[model.ControlDomainReview].Description, "runtime_verification_load_failed")
	assert.NotContains(t, byID[model.ControlIndependentReview].Description, "runtime_verification_load_failed")
}

func TestResolveRuntimeRequiredActionsFailsClosedWhenExecutionSummaryIsInvalid(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitRepoForRuntimeActionsTests(t, root)
	change := model.NewChange("runtime-actions-invalid-summary")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 4,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
	}))
	require.NoError(t, os.WriteFile(state.ExecutionSummaryPathForRead(root, change.Slug), []byte("version: ["), 0o644))

	writeGovernanceVerification(t, root, change.Slug, skillSpecComplianceReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 4,
	})
	writeGovernanceVerification(t, root, change.Slug, skillCodeQualityReview, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Timestamp:  time.Now().UTC(),
		RunVersion: 4,
	})

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlDomainReview, model.ControlModeBlocking, model.ControlScopeReview),
			makeControl(model.ControlIndependentReview, model.ControlModeBlocking, model.ControlScopeReview),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	byID := map[model.ControlID]RequiredAction{}
	for _, action := range actions {
		byID[action.ControlID] = action
	}

	require.Contains(t, byID, model.ControlDomainReview)
	require.Contains(t, byID, model.ControlIndependentReview)
	assert.False(t, byID[model.ControlDomainReview].Satisfied)
	assert.Contains(t, byID[model.ControlDomainReview].Description, "runtime_execution_summary_invalid:")
	assert.Contains(t, byID[model.ControlDomainReview].Description, "run_summary_missing")
	assert.False(t, byID[model.ControlIndependentReview].Satisfied)
	assert.Contains(t, byID[model.ControlIndependentReview].Description, "runtime_execution_summary_invalid:")
	assert.Contains(t, byID[model.ControlIndependentReview].Description, "run_summary_missing")
}

func TestExecutionScopeDoesNotBlockNonDiscoveryAtS1(t *testing.T) {
	t.Parallel()

	change := model.Change{
		CurrentState:   model.StateS1Plan,
		PlanSubStep:    model.PlanSubStepBundle,
		NeedsDiscovery: false,
	}

	// worktree-isolation is execution-scope; should NOT block at S1 for non-discovery.
	actions := []RequiredAction{
		{
			ControlID:   model.ControlWorktreeIsolation,
			Mode:        model.ControlModeBlocking,
			Scope:       model.ControlScopeExecution,
			Satisfied:   false,
			Description: "worktree preflight",
		},
	}

	blockers := RequiredActionBlockers(change, actions)
	assert.Empty(t, blockers, "execution-scope control must not block non-discovery change at S1_PLAN")
}

func TestExplorationDoesNotBlockAtS1Plan(t *testing.T) {
	t.Parallel()

	change := model.Change{
		CurrentState: model.StateS1Plan,
		PlanSubStep:  model.PlanSubStepResearch,
	}

	actions := []RequiredAction{
		{
			ControlID:   model.ControlResearch,
			Mode:        model.ControlModeBlocking,
			Scope:       model.ControlScopeDiscovery,
			Satisfied:   false,
			Description: "complete research.md, resolve unknowns, and confirm scope via intake before continuing",
		},
	}

	blockers := RequiredActionBlockers(change, actions)
	assert.Empty(t, blockers, "exploration should surface at S1 but not hard-block before scope confirmation is even possible")
}

func TestResolveRuntimeRequiredActionsUsesScopeConfirmationEvidenceAtS1Plan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("runtime-actions-scope-confirmed")
	change.NeedsDiscovery = true
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("# Intent\nValidated scope\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "research.md"), []byte(`# Research
## Alternatives Considered
Option A vs Option B.
## Unknowns
None remaining.
## Assumptions
Standard deployment.
## Canonical References
Internal docs.
`), 0o644))

	writeGovernanceVerification(t, root, change.Slug, skillIntakeClarification, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		References: []string{"scope:confirmed"},
	})

	snap := model.GovernanceSnapshot{
		Version: model.GovernanceSnapshotVersion,
		Summary: model.SignalSummary{
			BlastRadius: model.SignalLevelLow,
		},
		Traceability: model.TraceabilitySummary{
			Status: model.TraceabilityStatusOK,
		},
		ActiveControls: []model.ControlActivation{
			makeControl(model.ControlResearch, model.ControlModeBlocking, model.ControlScopeDiscovery),
		},
		ComputedAt: time.Now().UTC(),
	}

	actions := ResolveRuntimeRequiredActions(root, change, snap)
	require.Len(t, actions, 1)
	assert.True(t, actions[0].Satisfied, "intake + scope + research.md structure should satisfy the research control")
	assert.Empty(t, RequiredActionBlockers(change, actions), "satisfied research action must not block S1_PLAN")
}

func TestExecutionScopeDoesNotBlockDiscoveryAtS1(t *testing.T) {
	t.Parallel()

	change := model.Change{
		CurrentState:   model.StateS1Plan,
		PlanSubStep:    model.PlanSubStepResearch,
		NeedsDiscovery: true,
	}

	actions := []RequiredAction{
		{
			ControlID:   model.ControlWorktreeIsolation,
			Mode:        model.ControlModeBlocking,
			Scope:       model.ControlScopeExecution,
			Satisfied:   false,
			Description: "worktree preflight",
		},
	}

	blockers := RequiredActionBlockers(change, actions)
	assert.Empty(t, blockers, "execution-scope control must not block at S1_PLAN; worktree gate is at S2_EXECUTE/preflight")
}

func TestExecutionScopeBlocksAtS2Execute(t *testing.T) {
	t.Parallel()

	change := model.Change{
		CurrentState:   model.StateS2Execute,
		NeedsDiscovery: false,
	}

	actions := []RequiredAction{
		{
			ControlID:   model.ControlWorktreeIsolation,
			Mode:        model.ControlModeBlocking,
			Scope:       model.ControlScopeExecution,
			Satisfied:   false,
			Description: "worktree preflight",
		},
	}

	blockers := RequiredActionBlockers(change, actions)
	assert.Len(t, blockers, 1, "execution-scope control should block at S3 regardless of discovery flag")
}

func writeGovernanceVerification(t *testing.T, root, slug, skillName string, rec model.VerificationRecord) {
	t.Helper()
	writeVerificationForTest(t, root, slug, skillName, rec)
}

func initGitRepoForRuntimeActionsTests(t *testing.T, root string) {
	t.Helper()
	runGitForRuntimeActionsTests(t, root, "init", "-b", "main")
	runGitForRuntimeActionsTests(t, root, "config", "user.email", "test@example.com")
	runGitForRuntimeActionsTests(t, root, "config", "user.name", "Slipway Test")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("runtime actions"), 0o644))
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), model.DefaultConfig()))
	runGitForRuntimeActionsTests(t, root, "add", ".")
	runGitForRuntimeActionsTests(t, root, "commit", "-m", "init")
}

func runGitForRuntimeActionsTests(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}
