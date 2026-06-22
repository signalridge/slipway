package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootHelpUsesCurrentEntrySurfaceDescriptions(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeRootHelp(&buf))
	help := buf.String()

	assert.Contains(t, help, "Create a governed change with intake-first workflow")
	assert.Contains(t, help, "Complete intake clarification and authorization")
	assert.Contains(t, help, "Author or amend the governed plan artifacts")
	assert.Contains(t, help, "Execute governed implementation waves")
	assert.Contains(t, help, "Run review convergence")
	assert.Contains(t, help, "Dispatch fresh-context fixes for S3 review findings")
	assert.NotContains(t, help, "quick")
	assert.NotContains(t, help, "Create a durable exploration bundle without opening governed change state")
	assert.Contains(t, help, "Create or refresh the durable repo-scoped codebase map")
	assert.NotContains(t, help, "Show repo-wide governance freshness and workflow statistics")
	assert.NotContains(t, help, "\n    stats")
	assert.Contains(t, help, "Show repo-local integrity and repairability findings")
	// Issue #91 (P2b): the new public authoring surface must be discoverable from
	// the main `slipway help` path, not only docs/toolgen.
	assert.Contains(t, help, "instructions")
	assert.Contains(t, help, "Show the authoring contract")
	assert.Contains(t, help, "Finalize a done-ready change and archive it")
	assert.NotContains(t, help, "completed change")
	assert.NotContains(t, help, "Auto-classify advisory versus governed work")
}

func TestProgressionCommandsDoNotExposeQuickBypass(t *testing.T) {
	t.Parallel()

	for name, makeCmd := range map[string]func() *cobra.Command{
		"next": makeNextCmd,
		"run":  makeRunCmd,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := makeCmd()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs([]string{"--help"})

			require.NoError(t, cmd.Execute())
			assert.NotContains(t, buf.String(), "--quick")
		})
	}
}
