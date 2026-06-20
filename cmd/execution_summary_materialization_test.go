package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The public per-task-evidence + wave-orchestration flow must materialize
// execution-summary.yaml by itself, so a downstream validate no longer blocks on
// run_summary_missing until an undocumented `slipway repair`.
//
// issue #228
func TestEvidenceSkillWaveOrchestrationMaterializesExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		// Before any evidence, the public flow has produced no run summary.
		summaryBefore, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		require.NoError(t, err)
		require.Nil(t, summaryBefore)

		// Step 1: record the per-task evidence through the public command.
		taskCmd := commandForRoot(t, root, makeEvidenceCmd())
		taskCmd.SetArgs([]string{
			"task",
			"--change", slug,
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:t-01",
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
		})
		require.NoError(t, taskCmd.Execute())

		// Recording task evidence alone does not write the summary; the
		// wave-orchestration skill step owns that.
		summaryMid, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		require.NoError(t, err)
		require.Nil(t, summaryMid, "task evidence alone must not materialize the run summary")

		// Step 2: record wave-orchestration skill evidence through the public
		// command. This is the owning public step that must now materialize the
		// summary.
		skillCmd := commandForRoot(t, root, makeEvidenceCmd())
		skillCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "wave-orchestration:pass",
			"--notes", "Wave orchestration passed.",
		})
		var out bytes.Buffer
		skillCmd.SetOut(&out)
		require.NoError(t, skillCmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.True(t, view.Recorded)
		assert.Equal(t, 1, view.RunVersion)

		// The summary is now materialized by the same public command — no repair.
		summaryAfter, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		require.NoError(t, err)
		require.NotNil(t, summaryAfter, "wave-orchestration skill evidence must materialize execution-summary.yaml")
		assert.Equal(t, 1, summaryAfter.RunSummaryVersion)
		require.Len(t, summaryAfter.Tasks, 1)
		assert.Equal(t, "t-01", summaryAfter.Tasks[0].TaskID)
		assert.True(t, state.ExecutionSummaryReady(summaryAfter))
	})
}
