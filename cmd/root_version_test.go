package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootVersionFlagPrintsReleaseMetadata(t *testing.T) {
	oldVersion := version
	oldCommit := commit
	oldDate := date
	t.Cleanup(func() {
		version = oldVersion
		commit = oldCommit
		date = oldDate
	})

	version = "1.2.3"
	commit = "abc123"
	date = "2026-05-25T07:00:00Z"

	root := newRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"--version"})

	require.NoError(t, root.Execute())
	assert.Equal(t, "slipway 1.2.3\n  commit: abc123\n  built:  2026-05-25T07:00:00Z\n", out.String())
	assert.Empty(t, errOut.String())
}
