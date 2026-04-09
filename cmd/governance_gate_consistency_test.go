package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestGateStatusUsesPlanningEvidenceAcrossStatusValidateAndNext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, "L3", "gate status should stay stable outside planning")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		worktreeRoot := filepath.Join(t.TempDir(), change.Slug)
		branch := "feat/" + change.Slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch)
		normalizedWT, err := state.NormalizePath(worktreeRoot)
		require.NoError(t, err)
		changeBeforeWT := change
		change.CurrentState = model.StateS4Verify
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
REQ-001: Planning gate evidence must remain visible after execution.
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

		var statusOut bytes.Buffer
		statusCmd := makeStatusCmd()
		statusCmd.SetArgs([]string{"--json", "--change", slug})
		statusCmd.SetOut(&statusOut)
		require.NoError(t, statusCmd.Execute())

		var statusResp statusView
		require.NoError(t, json.Unmarshal(statusOut.Bytes(), &statusResp))
		assert.Equal(t, model.GateStatusApproved, statusResp.GateStatus["G_plan"].Status)
		assert.Equal(t, model.GateStatusApproved, statusResp.GateStatus["G_scope"].Status)

		var validateOut bytes.Buffer
		validateCmd := makeValidateCmd()
		validateCmd.SetArgs([]string{"--json", "--change", slug})
		validateCmd.SetOut(&validateOut)
		require.NoError(t, validateCmd.Execute())

		var validateResp validateView
		require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validateResp))
		assert.Equal(t, "approved", validateResp.GateStatus["G_plan"])
		assert.Equal(t, "approved", validateResp.GateStatus["G_scope"])

		var nextOut bytes.Buffer
		nextCmd := makeNextCmd()
		nextCmd.SetArgs([]string{"--json", "--change", slug})
		nextCmd.SetOut(&nextOut)
		require.NoError(t, nextCmd.Execute())

		var nextResp nextView
		require.NoError(t, json.Unmarshal(nextOut.Bytes(), &nextResp))
		assert.Equal(t, "approved", nextResp.InputContext.GateStatus["G_plan"])
		assert.Equal(t, "approved", nextResp.InputContext.GateStatus["G_scope"])
	})
}

func TestExecutionEvidenceBlockersStayConsistentAcrossStatusValidateAndNext(t *testing.T) {
	t.Run("missing execution summary", func(t *testing.T) {
		root := t.TempDir()
		withWorkspace(t, root, func() {
			require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

			slug := createGovernedRequest(t, root, "L2", "execution summary blockers should stay aligned")
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

			statusResp := runStatusViewForChange(t, root, slug)
			validateResp := runValidateViewForChange(t, root, slug)
			nextResp := runNextViewForChange(t, root, slug)

			for _, blockers := range [][]model.ReasonCode{
				statusResp.Blockers,
				validateResp.Blockers,
				nextResp.Blockers,
			} {
				requireBlockerContains(t, blockers, "required_skill_not_ready:spec-compliance-review:run_summary_missing")
				requireBlockerContains(t, blockers, "required_skill_not_ready:code-quality-review:run_summary_missing")
			}
			assert.Nil(t, nextResp.Advanced)
		})
	})

	t.Run("stale execution evidence", func(t *testing.T) {
		root := t.TempDir()
		withWorkspace(t, root, func() {
			require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

			slug := createGovernedRequest(t, root, "L2", "stale execution evidence should stay aligned")
			change, err := state.LoadChange(root, slug)
			require.NoError(t, err)
			change.CurrentState = model.StateS3Review
			change.PlanSubStep = model.PlanSubStepNone
			require.NoError(t, state.SaveChange(root, change))

			writePassingExecutionSummary(t, root, slug, 1, "t-01")
			bundlePath := filepath.Join(root, "artifacts", "changes", slug)
			require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte("# Intent\n\nUpdated after execution.\n"), 0o644))

			statusResp := runStatusViewForChange(t, root, slug)
			validateResp := runValidateViewForChange(t, root, slug)
			nextResp := runNextViewForChange(t, root, slug)

			for _, blockers := range [][]model.ReasonCode{
				statusResp.Blockers,
				validateResp.Blockers,
				nextResp.Blockers,
			} {
				requireBlockerContains(t, blockers, state.StaleExecutionEvidenceBlockerToken)
			}
			assert.Nil(t, nextResp.Advanced)
		})
	})
}

func TestReviewLayerBlockersStayConsistentAcrossStatusValidateNextAndReview(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review layer blockers should stay aligned")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))
		writeShipReadyGovernedBundle(t, root, change)
		writeAssuranceMD(t, root, slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
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

		statusResp := runStatusViewForChange(t, root, slug)
		validateResp := runValidateViewForChange(t, root, slug)
		nextResp := runNextViewForChange(t, root, slug)
		reviewResp := runReviewViewForChange(t, root, slug)

		for _, blockers := range [][]model.ReasonCode{
			statusResp.Blockers,
			validateResp.Blockers,
			nextResp.Blockers,
			reviewResp.Blockers,
		} {
			requireBlockerContains(t, blockers, "review_layer_missing:IR1")
		}
	})
}

func TestDoneShipGateReasonsStayConsistentWithSharedReadiness(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "done should reuse shared ship gate result")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
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
	})
}

func TestShipOnlyBlockersStayConsistentAcrossStatusValidateAndNext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "ship-only blockers should stay aligned")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS4Verify
		change.PlanSubStep = model.PlanSubStepNone
		change.BaseRef = ""
		require.NoError(t, state.SaveChange(root, change))

		writeShipReadyGovernedBundle(t, root, change)
		writeAssuranceMD(t, root, slug, validAssuranceContent())
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		writePassingReviewEvidencePack(t, root, slug, 1)
		writePassingGoalVerificationEvidence(t, root, slug, 1)

		statusResp := runStatusViewForChange(t, root, slug)
		validateResp := runValidateViewForChange(t, root, slug)
		nextResp := runNextViewForChange(t, root, slug)

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
		assert.Nil(t, nextResp.Advanced)
	})
}

func TestMissingArtifactBlockerStaysConsistentAcrossStatusValidateAndNextWithoutMutatingChangeAuthority(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "missing artifacts should block every read surface")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		require.NoError(t, state.SaveChange(root, change))

		require.NoError(t, os.Remove(filepath.Join(root, "artifacts", "changes", slug, "decision.md")))
		before := readGovernedChangeAuthorityBytes(t, root, slug)

		statusResp := runStatusViewForChange(t, root, slug)
		requireBlockerContains(t, statusResp.Blockers, "missing_required_artifact:decision.md")
		requireChangeAuthorityStable(t, root, slug, before)

		validateResp := runValidateViewForChange(t, root, slug)
		requireBlockerContains(t, validateResp.Blockers, "missing_required_artifact:decision.md")
		requireChangeAuthorityStable(t, root, slug, before)

		nextResp := runNextViewForChange(t, root, slug)
		requireBlockerContains(t, nextResp.Blockers, "missing_required_artifact:decision.md")
		assert.Nil(t, nextResp.Advanced)
		requireChangeAuthorityStable(t, root, slug, before)

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS1Plan, after.CurrentState)
		assert.Equal(t, model.PlanSubStepBundle, after.PlanSubStep)
	})
}

func TestWorktreeBindingBlockerStaysConsistentAcrossStatusValidateAndNext(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		initGitRepoForWorktreeTests(t, root)

		slug := createGovernedRequest(t, root, "L2", "invalid worktree binding should stay aligned")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.ArtifactSchema = model.ArtifactSchemaExpanded
		change.WorktreePath = root
		change.WorktreeBranch = currentGitBranch(t, root)
		require.NoError(t, state.SaveChange(root, change))

		statusResp := runStatusViewForChange(t, root, slug)
		validateResp := runValidateViewForChange(t, root, slug)
		nextResp := runNextViewForChange(t, root, slug)

		for _, blockers := range [][]model.ReasonCode{
			statusResp.Blockers,
			validateResp.Blockers,
			nextResp.Blockers,
		} {
			requireBlockerContains(t, blockers, state.WorktreeReasonDedicatedRequired)
		}
		assert.Nil(t, nextResp.Advanced)
	})
}

func TestNextIncludesGovernanceActionBlockersFromReadiness(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "next should surface governance blockers")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.GuardrailDomain = string(model.GuardrailDomainAuthAuthZ)
		require.NoError(t, state.SaveChange(root, change))
		writeAuthReviewGovernedBundle(t, root, slug)

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var nextResp nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &nextResp))
		requireBlockerContains(t, nextResp.Blockers, "governance_action_required:domain-review")
		assert.NotEmpty(t, nextResp.RequiredActions)
	})
}

func TestGovernanceSurfaceUsesReadinessSnapshotWithinInvocation(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "governance surface should reuse computed readiness snapshot")
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
	})
}

func runStatusViewForChange(t *testing.T, root, slug string) statusView {
	t.Helper()

	var out bytes.Buffer
	cmd := makeStatusCmd()
	cmd.SetArgs([]string{"--json", "--change", slug})
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())

	var view statusView
	require.NoError(t, json.Unmarshal(out.Bytes(), &view))
	return view
}

func runValidateViewForChange(t *testing.T, root, slug string) validateView {
	t.Helper()

	var out bytes.Buffer
	cmd := makeValidateCmd()
	cmd.SetArgs([]string{"--json", "--change", slug})
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())

	var view validateView
	require.NoError(t, json.Unmarshal(out.Bytes(), &view))
	return view
}

func runNextViewForChange(t *testing.T, root, slug string, extraArgs ...string) nextView {
	t.Helper()

	args := []string{"--json", "--change", slug}
	args = append(args, extraArgs...)

	var out bytes.Buffer
	cmd := makeNextCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())

	var view nextView
	require.NoError(t, json.Unmarshal(out.Bytes(), &view))
	return view
}

func runReviewViewForChange(t *testing.T, root, slug string) reviewView {
	t.Helper()

	var out bytes.Buffer
	cmd := makeReviewCmd()
	cmd.SetArgs([]string{"--json", "--change", slug})
	cmd.SetOut(&out)
	require.NoError(t, cmd.Execute())

	var view reviewView
	require.NoError(t, json.Unmarshal(out.Bytes(), &view))
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
