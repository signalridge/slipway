package cmd

import (
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/toolgen"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSurfacedCommandsMatchToolgenDescriptions(t *testing.T) {
	t.Parallel()

	commands := map[string]func() *cobra.Command{
		"abort":                 makeAbortCmd,
		"cancel":                makeCancelCmd,
		"checkpoint":            makeCheckpointCmd,
		"codebase-map":          makeCodebaseMapCmd,
		"done":                  makeDoneCmd,
		"health":                makeHealthCmd,
		"init":                  makeInitCmd,
		"new":                   makeNewCmd,
		"next":                  makeNextCmd,
		"pivot":                 makePivotCmd,
		"preset":                makePresetCmd,
		"repair":                makeRepairCmd,
		"review":                makeReviewCmd,
		"run":                   makeRunCmd,
		"stats":                 makeStatsCmd,
		"status":                makeStatusCmd,
		"validate":              makeValidateCmd,
	}

	for id, factory := range commands {
		id := id
		factory := factory
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			cmd := factory()
			require.NotNil(t, cmd)
			assert.Equal(t, id, cmd.Name())
			assert.Equal(t, toolgen.CommandDescription(id), cmd.Short)
		})
	}
}

func TestValidationCommandLongHelpStartsWithToolgenDescription(t *testing.T) {
	t.Parallel()

	commands := map[string]func() *cobra.Command{
		"validate": makeValidateCmd,
	}

	for id, factory := range commands {
		id := id
		factory := factory
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			cmd := factory()
			require.NotNil(t, cmd)
			assert.True(
				t,
				strings.HasPrefix(strings.TrimSpace(cmd.Long), toolgen.CommandDescription(id)),
				"long help for %s must start with the registry description",
				id,
			)
		})
	}
}
