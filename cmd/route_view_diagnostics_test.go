package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
