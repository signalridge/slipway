package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrongSurfaceDiscoveryFlagsFailAtParseTime(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cases := []struct {
			args []string
		}{
			{args: []string{"review", "--list-views"}},
		}

		for _, tc := range cases {
			stdout, stderr, err := runRootCommand(tc.args)
			require.Error(t, err, "%v should fail at parse time", tc.args)
			assert.Empty(t, stdout)

			var payload CLIError
			require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
			assert.Equal(t, categoryInvalidUsage, payload.Category)
			assert.Equal(t, "invalid_usage", payload.ErrorCode)
		}
	})
}

func TestLegacyRawModeFlagFailsAtParseTime(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

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
			assert.Equal(t, "invalid_usage", payload.ErrorCode)
		}
	})
}

func TestStatusExplicitViewEmitsPublicAliasAndHydrateReferences(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "status route-surface command contract")

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json", "--change", slug, "--focus", "incident"})
		require.NoError(t, cmd.Execute())

		var view statusView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "governed", view.ExecutionMode)
		assert.Equal(t, "incident", view.Mode)
		assert.Contains(t, view.HydrateReferences, "incident-response/incident-severity-matrix.md")
	})
}

func TestReviewFocusCalibrationEmitsPublicAliasAndHydrateReferences(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review focus calibration route-surface contract")
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
		assert.Equal(t, string(model.StateS3Review), view.CurrentState)
		assert.Equal(t, "calibration", view.Mode)
		assert.Contains(t, view.HydrateReferences, "multi-reviewer-calibration/review-dimensions.md")
		for _, ref := range view.HydrateReferences {
			assert.NotContains(t, ref, "ci-triage/")
			assert.NotContains(t, ref, "review-comment-triage/")
		}
	})
}
