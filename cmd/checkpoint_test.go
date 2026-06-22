package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandDoesNotExposeDeletedCommandSurfaces(t *testing.T) {
	t.Parallel()

	deleted := map[string]bool{
		"checkpoint": true,
		"learn":      true,
		"stats":      true,
	}
	for _, child := range newRootCmd().Commands() {
		assert.False(t, deleted[child.Name()], "deleted command %q must not be registered", child.Name())
	}
}

func TestDeletedCommandsAreUnknown(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"checkpoint", "learn", "stats"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			withCommandWorkspace(t, root, func() {
				stdout, stderr, err := runRootCommandIn(root, []string{name, "--json"})
				require.Error(t, err)
				assert.Empty(t, stdout)
				assert.Contains(t, stderr, "unknown command")
			})
		})
	}
}
