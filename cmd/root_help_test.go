package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootHelpUsesCurrentEntrySurfaceDescriptions(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeRootHelp(&buf))
	help := buf.String()

	assert.Contains(t, help, "Create a governed change with intake-first workflow")
	assert.NotContains(t, help, "quick")
	assert.NotContains(t, help, "Create a durable exploration bundle without opening governed change state")
	assert.Contains(t, help, "Create or refresh the durable repo-scoped codebase map")
	assert.Contains(t, help, "Show repo-wide governance freshness and workflow statistics")
	assert.Contains(t, help, "Show repo-local integrity and repairability findings")
	assert.Contains(t, help, "Finalize a done-ready change and archive it")
	assert.NotContains(t, help, "validate-requirements")
	assert.NotContains(t, help, "completed change")
	assert.NotContains(t, help, "Auto-classify advisory versus governed work")
}
