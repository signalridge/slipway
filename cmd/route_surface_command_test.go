package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrongSurfaceDiscoveryFlagsFailAtParseTime(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cases := []struct {
			args    []string
			wantMsg string
		}{
			{args: []string{"status", "--list-focuses"}, wantMsg: "unknown flag: --list-focuses"},
			{args: []string{"review", "--list-views"}, wantMsg: "unknown flag: --list-views"},
		}

		for _, tc := range cases {
			stdout, stderr, err := runRootCommand(tc.args)
			require.Error(t, err, "%v should fail at parse time", tc.args)
			assert.Empty(t, stdout)

			var payload CLIError
			require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
			assert.Equal(t, categoryInvalidUsage, payload.Category)
			assert.Contains(t, payload.Message, tc.wantMsg)
		}
	})
}

func TestLegacyRawModeFlagFailsAtParseTime(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cases := []struct {
			args []string
		}{
			{args: []string{"validate", "--mode", "property-testing"}},
			{args: []string{"review", "--mode", "multi-reviewer-calibration"}},
			{args: []string{"repair", "--mode", "sast-orchestration"}},
			{args: []string{"repair", "--mode", "ci-triage"}},
		}

		for _, tc := range cases {
			stdout, stderr, err := runRootCommand(tc.args)
			require.Error(t, err, "%v should fail at parse time", tc.args)
			assert.Empty(t, stdout)

			var payload CLIError
			require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
			assert.Equal(t, categoryInvalidUsage, payload.Category)
			assert.Contains(t, payload.Message, "unknown flag: --mode")
		}
	})
}

func TestStatusExplicitViewEmitsPublicAliasAndHydrateReferences(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "status route-surface command contract")

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json", "--change", slug, "--view", "incident"})
		require.NoError(t, cmd.Execute())

		var view statusView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "governed", view.ExecutionMode)
		assert.Equal(t, "incident", view.View)
		assert.Contains(t, view.HydrateReferences, "incident-response/incident-severity-matrix.md")
	})
}

func TestReviewFocusCalibrationEmitsPublicAliasAndHydrateReferences(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "review focus calibration route-surface contract")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		markChangeReadyForDone(t, root, &change)

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json", "--change", slug, "--focus", "calibration"})
		require.NoError(t, cmd.Execute())

		var view reviewView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "pass", view.Verdict)
		assert.Equal(t, string(model.StateS4Verify), view.CurrentState)
		assert.Equal(t, "calibration", view.Mode)
		assert.Contains(t, view.HydrateReferences, "multi-reviewer-calibration/review-dimensions.md")
		for _, ref := range view.HydrateReferences {
			assert.NotContains(t, ref, "ci-triage/")
			assert.NotContains(t, ref, "review-comment-triage/")
		}
	})
}

func TestRepairSuggestedCapabilitiesSerializeInTextAndJSON(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		slug := createGovernedRequest(t, root, "L2", "repair should suggest gha review for workflow changes")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeExecutionSummary(t, root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
			CapturedAt:        change.CreatedAt.UTC(),
			OverallVerdict:    model.ExecutionVerdictPass,
			CompletedTasks:    []string{"t-01"},
			Tasks: []model.ExecutionTaskSummary{
				{
					TaskID:       "t-01",
					Verdict:      model.TaskVerdictPass,
					TaskKind:     model.TaskKindCode,
					ChangedFiles: []string{".github/workflows/ci.yml"},
					CapturedAt:   change.CreatedAt.UTC(),
				},
			},
		})

		var jsonOut bytes.Buffer
		jsonCmd := makeRepairCmd()
		jsonCmd.SetOut(&jsonOut)
		jsonCmd.SetArgs([]string{"--json"})
		require.NoError(t, jsonCmd.Execute())

		var summary repairSummary
		require.NoError(t, json.Unmarshal(jsonOut.Bytes(), &summary))
		require.Len(t, summary.SuggestedCapabilities, 1)
		assert.Equal(t, "gha-security-review", summary.SuggestedCapabilities[0].Name)
		assert.NotEmpty(t, summary.SuggestedCapabilities[0].Reason)

		var textOut bytes.Buffer
		textCmd := makeRepairCmd()
		textCmd.SetOut(&textOut)
		require.NoError(t, textCmd.Execute())
		assert.Contains(t, textOut.String(), "Suggested:")
		assert.Contains(t, textOut.String(), "gha-security-review")
	})
}
