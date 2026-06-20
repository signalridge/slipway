package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixJSONSurfacesReviewFindingRepairContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "fix should surface review findings")

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
			Verdict:   model.VerificationVerdictFail,
			Blockers:  []model.ReasonCode{model.NewReasonCode("review_layer_failed", "R1")},
			Timestamp: time.Now().UTC(),
		})

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		var view fixView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, string(model.StateS3Review), view.CurrentState)
		assert.Equal(t, "s3-review-repair:"+slug, view.Contract.RepairBatchID)
		assert.True(t, view.Contract.CollectAllSelectedReviewFindingsFirst)
		assert.True(t, view.Contract.RequiresFreshContext)
		assert.Contains(t, view.Contract.FindingCollection, "Collect all selected S3 reviewer findings first")
		assert.Contains(t, view.Contract.RepairBrief, "One repair brief")
		assert.Equal(t, model.ContextOriginReferencePrefix+model.StageContextFix+"=<repair-subagent-handle>", view.Contract.ContextReference)
		assert.Contains(t, view.Contract.Prohibited, "Do not repair individual review findings before collecting the selected review batch findings.")
		require.NotEmpty(t, view.RepairTargets)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.RepairTargets[0].Reviewer)
		assert.Equal(t, "review_finding", view.RepairTargets[0].Kind)
	})
}

func TestFixRejectsNonReviewState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "fix should reject non review state")

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		err := cmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		assert.Equal(t, "fix_state_invalid", cliErr.ErrorCode)
		assert.Equal(t, "slipway plan", cliErr.Details["next_command"])
	})
}
