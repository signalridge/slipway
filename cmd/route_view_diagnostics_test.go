package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoViewRequiresConcreteChangeTarget(t *testing.T) {
	assert.Equal(t, "incident", resolveEffectiveView("status", ""))
	assert.Equal(t, "incident", resolveEffectiveView("health", ""))
}

func TestDiagnosticsModeDoesNotAutoSelectViewWithoutChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		var statusOut bytes.Buffer
		statusCmd := makeStatusCmd()
		statusCmd.SetArgs([]string{"--json"})
		statusCmd.SetOut(&statusOut)
		require.NoError(t, statusCmd.Execute())

		var status statusView
		require.NoError(t, json.Unmarshal(statusOut.Bytes(), &status))
		assert.Equal(t, "diagnostics", status.ExecutionMode)
		assert.Empty(t, status.View)

		var healthOut bytes.Buffer
		healthCmd := makeHealthCmd()
		healthCmd.SetArgs([]string{"--json"})
		healthCmd.SetOut(&healthOut)
		require.NoError(t, healthCmd.Execute())

		var health healthView
		require.NoError(t, json.Unmarshal(healthOut.Bytes(), &health))
		assert.Equal(t, "diagnostics", health.ExecutionMode)
		assert.Empty(t, health.View)
	})
}

func TestStatusDiagnosticsDoesNotAutoSelectViewWithoutActiveChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view statusView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		assert.Empty(t, view.View)
	})
}

func TestStatusDiagnosticsPreservesExplicitViewWithoutActiveChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetArgs([]string{"--json", "--view", "incident"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view statusView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		assert.Equal(t, "incident", view.View)
	})
}

func TestHealthDiagnosticsDoesNotAutoSelectViewWithoutActiveChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		assert.Empty(t, view.View)
	})
}

func TestHealthDiagnosticsPreservesExplicitViewWithoutActiveChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		var out bytes.Buffer
		cmd := makeHealthCmd()
		cmd.SetArgs([]string{"--json", "--view", "incident"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view healthView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, "diagnostics", view.ExecutionMode)
		assert.Equal(t, "incident", view.View)
	})
}
