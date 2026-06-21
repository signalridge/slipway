package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGateStatusUsesPlanningEvidenceAcrossStatusValidateAndNext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	initGitRepoForWorktreeTests(t, root)

	slug := createGovernedRequest(t, root, levelDiscovery, "gate status should stay stable outside planning")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	worktreeRoot := filepath.Join(t.TempDir(), change.Slug)
	branch := "feat/" + change.Slug
	runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
	normalizedWT, err := state.NormalizePath(worktreeRoot)
	require.NoError(t, err)
	changeBeforeWT := change
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = normalizedWT
	change.WorktreeBranch = branch
	require.NoError(t, state.RelocateGovernedBundle(root, changeBeforeWT, change))
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(normalizedWT, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "intent.md", []byte(`# Intent
INT-001: validate gate status reuse
## Open Questions
(none)
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "requirements.md", []byte(`# Requirements
### Requirement: StableGateStatus
REQ-001: Planning gate evidence MUST remain visible after execution.

#### Scenario: Planning gate stays visible post-execution
GIVEN a change with planning-stage gate evidence
WHEN status, validate, and next read gate state after execution
THEN the planning gate remains visible and consistent.
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "decision.md", []byte(`# Decision
## Alternatives Considered
### Option A
Recompute gates from current state.
### Option B
Reuse planning-stage evidence for planning gates.

## Selected Approach
Use planning-stage evidence for planning gates.

## Interfaces and Data Flow
Planning evidence remains authoritative for G_plan and G_scope.

## Rollout and Rollback
Roll forward by reusing planning evidence everywhere.

## Risk
Low risk.
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` validate gate consistency
  - depends_on: []
  - target_files: ["cmd/status_view_build.go"]
  - task_kind: verification
  - covers: [REQ-001]
`)))
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "research.md", []byte(`## Alternatives Considered
### Option A
Keep gate state duplicated per surface.

## Unknowns
None.

## Assumptions
Planning evidence remains authoritative after S1.

## Canonical References
- docs/plans/2026-04-07-governance-authority-simplification-plan.md
`)))
	writeAssuranceMD(t, root, slug, validAssuranceContent())

	sourceAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	planAuditAt := sourceAt.Add(time.Hour)
	for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md", "tasks.md"} {
		path := filepath.Join(bundlePath, rel)
		require.NoError(t, os.Chtimes(path, sourceAt, sourceAt))
	}
	assurancePath := filepath.Join(bundlePath, "assurance.md")
	require.NoError(t, os.Chtimes(assurancePath, sourceAt, sourceAt))

	writeSkillVerification(t, root, slug, "plan-audit", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  planAuditAt,
		References: planAuditOriginReferences(),
	})
	writeSkillVerification(t, root, slug, "research-orchestration", model.VerificationRecord{
		Verdict:   model.VerificationVerdictPass,
		Blockers:  []model.ReasonCode{},
		Timestamp: planAuditAt,
	})

	statusResp, validateResp, nextResp := runReadOnlyGovernanceViewsForChange(t, root, slug)
	assert.Equal(t, model.GateStatusApproved, statusResp.GateStatus["G_plan"].Status)
	assert.Equal(t, model.GateStatusApproved, statusResp.GateStatus["G_scope"].Status)
	assert.Equal(t, "approved", validateResp.GateStatus["G_plan"])
	assert.Equal(t, "approved", validateResp.GateStatus["G_scope"])
	assert.Equal(t, "approved", nextResp.InputContext.GateStatus["G_plan"])
	assert.Equal(t, "approved", nextResp.InputContext.GateStatus["G_scope"])
}

func TestExecutionEvidenceBlockersStayConsistentAcrossStatusValidateAndNext(t *testing.T) {
	t.Run("missing execution summary", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "execution summary blockers should stay aligned")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
		})
		writeSkillVerification(t, root, slug, "code-quality-review", model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  time.Now().UTC().Add(time.Second),
			RunVersion: 1,
		})

		statusResp, validateResp, nextResp := runReadOnlyGovernanceViewsForChange(t, root, slug)

		for _, blockers := range [][]model.ReasonCode{
			statusResp.Blockers,
			validateResp.Blockers,
			nextResp.Blockers,
		} {
			requireBlockerContains(t, blockers, "required_skill_not_ready:spec-compliance-review:run_summary_missing")
			requireBlockerContains(t, blockers, "required_skill_not_ready:code-quality-review:run_summary_missing")
		}
		if nextResp.Advanced != nil {
			assert.Equal(t, "query", nextResp.Advanced.Action)
		}
	})

	t.Run("stale execution evidence", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ensureTestGitRepo(t, root)
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "stale execution evidence should stay aligned")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)
		change.GuardrailDomain = "schema_data_migration"
		require.NoError(t, state.SaveChange(root, change))

		statusResp, validateResp, nextResp := runReadOnlyGovernanceViewsForChange(t, root, slug)

		for _, blockers := range [][]model.ReasonCode{
			statusResp.Blockers,
			validateResp.Blockers,
			nextResp.Blockers,
		} {
			requireBlockerContains(t, blockers, state.StaleExecutionEvidenceBlockerToken)
		}
		if nextResp.Advanced != nil {
			assert.Equal(t, "query", nextResp.Advanced.Action)
		}
	})
}

func TestMissingReviewEvidenceBlockersIncludeSelectedReviewSetAcrossStatusValidateAndNext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "missing selected review evidence should stay aligned")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)

	statusResp, validateResp, nextResp := runReadOnlyGovernanceViewsForChange(t, root, slug)

	for _, blockers := range [][]model.ReasonCode{
		statusResp.Blockers,
		validateResp.Blockers,
		nextResp.Blockers,
	} {
		requireBlockerContains(t, blockers, "required_skill_missing:spec-compliance-review")
		requireBlockerContains(t, blockers, "required_skill_missing:code-quality-review")
		requireBlockerContains(t, blockers, "required_skill_missing:independent-review")
		require.NotContains(t, strings.Join(model.ReasonSpecs(blockers), "\n"), "required_skill_missing:security-review")
	}
	require.NotNil(t, nextResp.NextSkill)
	assert.ElementsMatch(t, []string{
		progression.SkillSpecComplianceReview,
		progression.SkillCodeQualityReview,
		progression.SkillIndependentReview,
		progression.SkillGoalVerification,
	}, nextResp.NextSkill.SelectedReviewSkills)
}

func TestReviewLayerBlockersStayConsistentAcrossStatusValidateNextAndReview(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "review layer blockers should stay aligned")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))
	writeShipReadyGovernedBundle(t, root, change)
	writeAssuranceMD(t, root, slug, validAssuranceContent())
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	materializeWaveExecutionForSummary(t, root, slug)
	writePassingWaveEvidence(t, root, slug, 1)
	writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{"layer:R0=pass"},
	})
	writeSkillVerification(t, root, slug, "code-quality-review", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC().Add(time.Second),
		RunVersion: 1,
	})

	statusResp, validateResp, nextResp := runReadOnlyGovernanceViewsForChange(t, root, slug)
	reviewResp := runReviewViewForChange(t, root, slug)

	for _, blockers := range [][]model.ReasonCode{
		statusResp.Blockers,
		validateResp.Blockers,
		nextResp.Blockers,
		reviewResp.Blockers,
	} {
		requireBlockerContains(t, blockers, "review_layer_missing:IR1")
	}
}

func TestSatisfiedDomainReviewAttributionStaysConsistentAcrossStatusValidateAndNext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "domain review attribution should stay aligned")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = string(model.GuardrailDomainAuthAuthZ)
	require.NoError(t, state.SaveChange(root, change))

	writeAuthReviewGovernedBundle(t, root, slug)
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	materializeWaveExecutionForSummary(t, root, slug)
	writePassingWaveEvidence(t, root, slug, 1)
	writeSkillVerification(t, root, slug, "spec-compliance-review", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
		References: []string{"layer:R0=pass"},
	})

	statusResp, validateResp, nextResp := runReadOnlyGovernanceViewsForChange(t, root, slug)

	requireDomainReviewSatisfiedBySpecCompliance(t, statusResp.RequiredActions)
	requireDomainReviewSatisfiedBySpecCompliance(t, validateResp.RequiredActions)
	requireDomainReviewSatisfiedBySpecCompliance(t, nextResp.RequiredActions)
}

func TestDoneShipGateReasonsStayConsistentWithSharedReadiness(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "done should reuse shared ship gate result")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeShipReadyGovernedBundle(t, root, change)
	writeAssuranceMD(t, root, slug, validAssuranceContent())
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writePassingReviewEvidencePack(t, root, slug, 1)

	readiness, err := progression.EvaluateGovernanceReadiness(
		root,
		change,
		progression.GovernanceReadinessOptions{
			IncludeShipSurface: true,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, readiness.ShipSurface)
	require.Equal(t, model.GateStatusBlocked, readiness.ShipSurface.Result.Status, "test setup must exercise a ship-gate blocker")

	shipEval, shipBlocked, err := refreshDoneShipGate(root, &change)
	require.NoError(t, err)
	require.True(t, shipBlocked)
	assert.ElementsMatch(t, model.ReasonSpecs(readiness.ShipSurface.Result.ReasonCodes), model.ReasonSpecs(shipEval.ReasonCodes))
	assert.ElementsMatch(t, readiness.ShipSurface.Result.ReasonCodes, shipEval.ReasonCodes)
}

func TestShipOnlyBlockersStayConsistentAcrossStatusValidateAndNext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "ship-only blockers should stay aligned")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.BaseRef = ""
	require.NoError(t, state.SaveChange(root, change))

	writeShipReadyGovernedBundle(t, root, change)
	writeAssuranceMD(t, root, slug, validAssuranceContent())
	writePassingExecutionSummary(t, root, slug, 1, "t-01")
	writePassingWaveEvidence(t, root, slug, 1)
	writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{})
	writePassingReviewEvidencePack(t, root, slug, 1)
	writePassingGoalVerificationEvidence(t, root, slug, 1)
	writePassingFinalCloseoutEvidence(t, root, slug, 1)

	statusResp, validateResp, nextResp := runReadOnlyGovernanceViewsForChange(t, root, slug)

	assert.Equal(t, model.GateStatusBlocked, statusResp.GateStatus["G_ship"].Status)
	assert.Equal(t, "blocked", validateResp.GateStatus["G_ship"])
	assert.Equal(t, "blocked", nextResp.InputContext.GateStatus["G_ship"])

	for _, blockers := range [][]model.ReasonCode{
		statusResp.Blockers,
		validateResp.Blockers,
		nextResp.Blockers,
	} {
		requireBlockerContains(t, blockers, "manifest_base_ref_missing")
	}
	assert.False(t, validateResp.CanAdvance)
	if nextResp.Advanced != nil {
		assert.Equal(t, "query", nextResp.Advanced.Action)
	}
}

func TestMissingArtifactBlockerStaysConsistentAcrossStatusValidateAndNextWithoutMutatingChangeAuthority(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "missing artifacts should block every read surface")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	require.NoError(t, state.SaveChange(root, change))

	require.NoError(t, os.Remove(filepath.Join(root, "artifacts", "changes", slug, "decision.md")))
	before := readGovernedChangeAuthorityBytes(t, root, slug)

	statusResp, validateResp, nextResp := runReadOnlyGovernanceViewsForChange(t, root, slug)
	requireBlockerContains(t, statusResp.Blockers, "missing_required_artifact:decision.md")
	requireChangeAuthorityStable(t, root, slug, before)

	requireBlockerContains(t, validateResp.Blockers, "missing_required_artifact:decision.md")
	requireChangeAuthorityStable(t, root, slug, before)

	requireBlockerContains(t, nextResp.Blockers, "missing_required_artifact:decision.md")
	if nextResp.Advanced != nil {
		assert.Equal(t, "query", nextResp.Advanced.Action)
	}
	requireChangeAuthorityStable(t, root, slug, before)

	after, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	assert.Equal(t, model.StateS1Plan, after.CurrentState)
	assert.Equal(t, model.PlanSubStepBundle, after.PlanSubStep)
}

func TestWorktreeBindingBlockerStaysConsistentAcrossStatusValidateAndNext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)
	initGitRepoForWorktreeTests(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "invalid worktree binding should stay aligned")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	change.ArtifactSchema = model.ArtifactSchemaExpanded
	change.WorktreePath = root
	change.WorktreeBranch = currentGitBranch(t, root)
	require.NoError(t, state.SaveChange(root, change))

	statusResp, validateResp, nextResp := runReadOnlyGovernanceViewsForChange(t, root, slug)

	for _, blockers := range [][]model.ReasonCode{
		statusResp.Blockers,
		validateResp.Blockers,
		nextResp.Blockers,
	} {
		requireBlockerContains(t, blockers, state.WorktreeReasonDedicatedRequired)
	}
	if nextResp.Advanced != nil {
		assert.Equal(t, "query", nextResp.Advanced.Action)
	}
}

func TestNextIncludesGovernanceActionBlockersFromReadiness(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "next should surface governance blockers")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = string(model.GuardrailDomainAuthAuthZ)
	require.NoError(t, state.SaveChange(root, change))
	writeAuthReviewGovernedBundle(t, root, slug)

	nextResp := runNextViewForChange(t, root, slug)
	requireBlockerContains(t, nextResp.Blockers, "governance_action_required:domain-review")
}

func TestGovernanceSurfaceUsesReadinessSnapshotWithinInvocation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "governance surface should reuse computed readiness snapshot")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.SuggestedWorkflowPreset = ""
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` medium blast radius
  - depends_on: []
  - target_files: ["a.go", "b.go", "c.go", "d.go", "e.go"]
  - task_kind: verification
`)))

	readiness, err := progression.EvaluateGovernanceReadiness(root, change, progression.GovernanceReadinessOptions{})
	require.NoError(t, err)

	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` low blast radius
  - depends_on: []
  - target_files: ["a.go"]
  - task_kind: verification
`)))

	view := statusView{}
	applyGovernanceSurfaceToStatus(readiness, &view)
	require.NotNil(t, view.GovernanceSignals)
	assert.Equal(t, "medium", view.GovernanceSignals.BlastRadius)
}

func runReadOnlyGovernanceViewsForChange(t *testing.T, root, slug string) (statusView, validateView, nextView) {
	t.Helper()

	var (
		statusResp   statusView
		validateResp validateView
		nextResp     nextView
	)
	errCh := make(chan error, 3)
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		change, err := state.LoadChange(root, slug)
		if err != nil {
			errCh <- err
			return
		}
		view, err := buildStatusViewFromChange(root, change)
		if err != nil {
			errCh <- err
			return
		}
		statusResp = view
	}()

	go func() {
		defer wg.Done()
		view, err := buildValidateViewForSlug(root, slug)
		if err != nil {
			errCh <- err
			return
		}
		validateResp = view
	}()

	go func() {
		defer wg.Done()
		view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, Command: "run"})
		if err != nil {
			errCh <- err
			return
		}
		nextResp = view
	}()

	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	return statusResp, validateResp, nextResp
}

func runNextViewForChange(t *testing.T, root, slug string, extraArgs ...string) nextView {
	t.Helper()

	require.Empty(t, extraArgs, "runNextViewForChange only supports default next --json semantics")
	view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, Command: "run"})
	require.NoError(t, err)
	return view
}

func runReviewViewForChange(t *testing.T, root, slug string) reviewView {
	t.Helper()

	view, err := buildReviewViewForSlug(root, slug, true, "", nil)
	require.NoError(t, err)
	return view
}

func readGovernedChangeAuthorityBytes(t *testing.T, root, slug string) []byte {
	t.Helper()

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	paths, err := state.ResolveChangePaths(root, change)
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(paths.GovernedBundleDir, "change.yaml"))
	require.NoError(t, err)
	return raw
}

func requireChangeAuthorityStable(t *testing.T, root, slug string, before []byte) {
	t.Helper()
	assert.Equal(t, string(before), string(readGovernedChangeAuthorityBytes(t, root, slug)))
}

func requireDomainReviewSatisfiedBySpecCompliance(t *testing.T, actions []governanceActionView) {
	t.Helper()

	for _, action := range actions {
		if action.ControlID != "domain-review" {
			continue
		}
		require.True(t, action.Satisfied)
		require.Len(t, action.SatisfiedBy, 1)
		assert.Equal(t, "skill_evidence", action.SatisfiedBy[0].Kind)
		assert.Equal(t, "spec-compliance-review", action.SatisfiedBy[0].Name)
		assert.Contains(t, action.SatisfiedBy[0].EvidenceRef, "verification/spec-compliance-review.yaml")
		assert.Equal(t, "spec-compliance-review provides the domain-aware review evidence for domain-review", action.SatisfiedBy[0].Reason)
		return
	}
	t.Fatalf("domain-review action not found in %#v", actions)
}
