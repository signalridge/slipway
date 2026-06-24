package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoneAllReadyPreservesShipGateReasonCodes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	blocked := model.NewChange("bulk-ship-blocked-reasons")
	blocked.CurrentState = model.StateS3Review
	blocked.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, blocked))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, blocked, ""))
	// requirements.md/decision.md/tasks.md are authored by the host skill, not
	// scaffolded (issue #119); write a real bundle so the ship gate reaches the
	// missing-review-skill check instead of failing closed on a missing artifact.
	writeShipReadyGovernedBundle(t, root, blocked)
	writePassingExecutionSummary(t, root, blocked.Slug, 1, "t-01")
	writePassingWaveEvidence(t, root, blocked.Slug, 1)
	writePassingShipVerificationEvidence(t, root, blocked.Slug, 1)
	writeAssuranceMD(t, root, blocked.Slug, validAssuranceContent())

	view := archiveAllDoneReady(root)
	require.Len(t, view.Skipped, 1)
	assert.Equal(t, "ship_gate_blocked", view.Skipped[0].Reason)
	assert.Contains(t,
		model.ReasonSpecs(view.Skipped[0].ReasonCodes),
		"required_skill_missing:code-quality-review",
	)
	assert.Contains(t,
		model.ReasonSpecs(view.Skipped[0].ReasonCodes),
		"required_skill_missing:spec-compliance-review",
	)
}

func TestDoneAllReadyPreservesSpecificReadinessArtifactBlockers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	blocked := model.NewChange("bulk-ship-blocked-artifact-reason")
	blocked.CurrentState = model.StateS3Review
	blocked.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, blocked))
	require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, blocked, ""))
	// requirements.md/decision.md/tasks.md are authored by the host skill, not
	// scaffolded (issue #119). Write a real bundle so requirements.md/tasks.md are
	// present; removing decision.md below then isolates the specific missing
	// artifact blocker this test asserts is preserved.
	writeShipReadyGovernedBundle(t, root, blocked)
	writePassingExecutionSummary(t, root, blocked.Slug, 1, "t-01")
	writePassingWaveEvidence(t, root, blocked.Slug, 1)
	writePassingReviewEvidencePack(t, root, blocked.Slug, 1)
	writePassingShipVerificationEvidence(t, root, blocked.Slug, 1)
	writeAssuranceMD(t, root, blocked.Slug, validAssuranceContent())

	require.NoError(t, os.Remove(filepath.Join(root, "artifacts", "changes", blocked.Slug, "decision.md")))

	view := archiveAllDoneReady(root)
	require.Len(t, view.Skipped, 1)
	assert.Equal(t, "ship_gate_blocked", view.Skipped[0].Reason)
	assert.Contains(t,
		model.ReasonSpecs(view.Skipped[0].ReasonCodes),
		"missing_required_artifact:decision.md",
	)
}
