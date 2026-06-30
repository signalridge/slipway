package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

		cfg, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		cfg.Subagents.Fix = model.SubagentProfile{
			Model:             "fix-fast",
			AllowedSkills:     stringList("code-quality-review"),
			AllowedMCPServers: stringList("serena"),
		}
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

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
		require.NotNil(t, view.Contract.Subagent)
		assert.Equal(t, "fix-fast", view.Contract.Subagent.Model)
		assert.Equal(t, []string{"code-quality-review"}, subagentListValue(view.Contract.Subagent.AllowedSkills))
		assert.Equal(t, []string{"serena"}, subagentListValue(view.Contract.Subagent.AllowedMCPServers))
		require.NotEmpty(t, view.RepairTargets)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.RepairTargets[0].Reviewer)
		assert.Equal(t, "review_finding", view.RepairTargets[0].Kind)

		var raw map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &raw))
		rawContract, ok := raw["contract"].(map[string]any)
		require.True(t, ok, "raw JSON must include contract")
		rawSubagent, ok := rawContract["subagent"].(map[string]any)
		require.True(t, ok, "raw JSON must include contract.subagent")
		_, ok = rawSubagent["allowed_mcp_servers"]
		assert.True(t, ok, "raw JSON must include contract.subagent.allowed_mcp_servers")
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

func TestFixStartReexecutionAdvancesWavePlanRunVersionAndReopensS2(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:stale-task-evidence",
		})
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))
		writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
			Verdict:    model.VerificationVerdictFail,
			Blockers:   []model.ReasonCode{model.NewReasonCode("review_layer_failed", "R1")},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
		})

		cmd := commandForRoot(t, root, makeFixCmd())
		cmd.SetArgs([]string{"--json", "--change", slug, "--start-reexecution"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		require.NoError(t, cmd.Execute())

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS2Implement, reloaded.CurrentState)

		plan, err := state.LoadWavePlanForChange(root, reloaded)
		require.NoError(t, err)
		assert.Equal(t, 2, plan.RunSummaryVersion)

		_, err = os.Stat(filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json"))
		assert.True(t, os.IsNotExist(err), "starting a fresh execution run must clear stale task evidence")
	})
}
