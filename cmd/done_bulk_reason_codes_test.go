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
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		blocked := model.NewChange("bulk-ship-blocked-reasons")
		blocked.CurrentState = model.StateS4Verify
		blocked.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, blocked))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, blocked, ""))
		writePassingExecutionSummary(t, root, blocked.Slug, 1, "t-01")
		writePassingWaveEvidence(t, root, blocked.Slug, 1)
		writePassingGoalVerificationEvidence(t, root, blocked.Slug, 1)
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
	})
}

func TestDoneAllReadyPreservesSpecificReadinessArtifactBlockers(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		blocked := model.NewChange("bulk-ship-blocked-artifact-reason")
		blocked.CurrentState = model.StateS4Verify
		blocked.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, blocked))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChangeWithPreset(root, blocked, ""))
		writePassingExecutionSummary(t, root, blocked.Slug, 1, "t-01")
		writePassingWaveEvidence(t, root, blocked.Slug, 1)
		writePassingReviewEvidencePack(t, root, blocked.Slug, 1)
		writePassingGoalVerificationEvidence(t, root, blocked.Slug, 1)
		writeAssuranceMD(t, root, blocked.Slug, validAssuranceContent())

		require.NoError(t, os.Remove(filepath.Join(root, "artifacts", "changes", blocked.Slug, "decision.md")))

		view := archiveAllDoneReady(root)
		require.Len(t, view.Skipped, 1)
		assert.Equal(t, "ship_gate_blocked", view.Skipped[0].Reason)
		assert.Contains(t,
			model.ReasonSpecs(view.Skipped[0].ReasonCodes),
			"missing_required_artifact:decision.md",
		)
	})
}
