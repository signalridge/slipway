package cmd

import (
	"bytes"
	"testing"

	"github.com/signalridge/slipway/internal/toolgen"
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
	// The config public surface must be discoverable from the root help, not only
	// `slipway help config`; this is the whole point of a discoverability change.
	assert.Contains(t, help, "config")
	assert.Contains(t, help, desc("config"))
}

func TestRootHelpGroupsUseRegistryDescriptions(t *testing.T) {
	t.Parallel()

	groupTiers := map[string]string{
		"Core lifecycle": "core",
		"Discovery":      "discovery",
		"Situational":    "situational",
		"Helpers":        "helpers",
		"Diagnostics":    "diagnostics",
		"Setup":          "setup",
	}
	seen := map[string]string{}
	for _, group := range helpGroups {
		wantTier, ok := groupTiers[group.Title]
		require.Truef(t, ok, "root help group %q must declare its registry tier", group.Title)
		for _, entry := range group.Commands {
			if previousGroup, exists := seen[entry.Name]; exists {
				t.Fatalf("root help entry %q appears in both %q and %q", entry.Name, previousGroup, group.Title)
			}
			seen[entry.Name] = group.Title

			description := desc(entry.Name)
			require.NotEmptyf(t, description, "root help entry %q must be registered in toolgen commandRegistry", entry.Name)
			assert.Equal(t, description, entry.Description, "root help entry %q must use the registry description", entry.Name)

			var registryDef *toolgen.CommandDef
			for _, def := range toolgen.CommandDefinitions() {
				if def.ID == entry.Name {
					def := def
					registryDef = &def
					break
				}
			}
			require.NotNilf(t, registryDef, "root help entry %q must be registered in toolgen commandRegistry", entry.Name)
			assert.Equalf(t, wantTier, registryDef.Tier,
				"root help entry %q is grouped under %q but commandRegistry assigns tier %q",
				entry.Name, group.Title, registryDef.Tier)
		}
	}

	for _, def := range toolgen.CommandDefinitions() {
		assert.Containsf(t, seen, def.ID,
			"commandRegistry entry %q with tier %q must appear in root helpGroups",
			def.ID, def.Tier)
	}
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
