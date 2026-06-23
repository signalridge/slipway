package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandoffCommandBareWriteAndShowBrief(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff command")

		writeCmd := commandForRoot(t, root, makeHandoffCmd())
		var writeOut bytes.Buffer
		writeCmd.SetOut(&writeOut)
		require.NoError(t, writeCmd.Execute())
		assert.Contains(t, writeOut.String(), "handoff_written:")

		path := state.ChangeHandoffPath(root, slug)
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "slipway:handoff-machine-header")
		assert.NotContains(t, string(raw), "current_state")
		assert.NotContains(t, string(raw), "next_skill")

		showCmd := commandForRoot(t, root, makeHandoffCmd())
		showCmd.SetArgs([]string{"show", "--brief"})
		var showOut bytes.Buffer
		showCmd.SetOut(&showOut)
		require.NoError(t, showCmd.Execute())
		assert.Contains(t, showOut.String(), "session_handoff: slug="+slug)
		assert.Contains(t, showOut.String(), "path="+path)
		assert.NotContains(t, showOut.String(), "current_state")
	})
}

func TestHandoffWriteSectionFromStdinPreservesOnRefresh(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createGovernedRequest(t, root, levelNonDiscovery, "handoff section")

		cmd := commandForRoot(t, root, makeHandoffCmd())
		cmd.SetArgs([]string{"write", "--section", "Next Session Focus"})
		cmd.SetIn(strings.NewReader("Finish hook wiring."))
		require.NoError(t, cmd.Execute())

		refresh := commandForRoot(t, root, makeHandoffCmd())
		refresh.SetArgs([]string{"write"})
		require.NoError(t, refresh.Execute())

		show := commandForRoot(t, root, makeHandoffCmd())
		show.SetArgs([]string{"show", "--json"})
		var out bytes.Buffer
		show.SetOut(&out)
		require.NoError(t, show.Execute())
		var view handoffShowView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, 2, view.Header.Generation)
		assert.Contains(t, view.Narrative, "Finish hook wiring.")
	})
}

func TestHandoffSubcommandsAcceptExplicitChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		targetSlug := createGovernedRequest(t, root, levelNonDiscovery, "targeted handoff")
		otherChange := model.NewChange("other-active-change")
		require.NoError(t, state.SaveChange(root, otherChange))

		writeCmd := commandForRoot(t, root, makeHandoffCmd())
		writeCmd.SetArgs([]string{"write", "--change", targetSlug, "--section", "Next Session Focus"})
		writeCmd.SetIn(strings.NewReader("Continue the targeted change."))
		var writeOut bytes.Buffer
		writeCmd.SetOut(&writeOut)
		require.NoError(t, writeCmd.Execute())
		assert.Contains(t, writeOut.String(), state.ChangeHandoffPath(root, targetSlug))

		showCmd := commandForRoot(t, root, makeHandoffCmd())
		showCmd.SetArgs([]string{"show", "--change", targetSlug, "--brief"})
		var showOut bytes.Buffer
		showCmd.SetOut(&showOut)
		require.NoError(t, showCmd.Execute())
		assert.Contains(t, showOut.String(), "session_handoff: slug="+targetSlug)
		assert.Contains(t, showOut.String(), "path="+state.ChangeHandoffPath(root, targetSlug))

		_, err := os.Stat(state.ChangeHandoffPath(root, otherChange.Slug))
		assert.True(t, os.IsNotExist(err), "explicit write must not fall back to the other active change")
	})
}

func TestHandoffRejectsInvalidExplicitChangeSlugBeforeRuntimeWrite(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		tests := []struct {
			name string
			slug string
		}{
			{name: "parent traversal", slug: "../x"},
			{name: "slash", slug: "bad/slug"},
			{name: "backslash", slug: `bad\slug`},
			{name: "dot", slug: "."},
			{name: "dot dot", slug: ".."},
			{name: "uppercase", slug: "Bad-Slug"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cmd := commandForRoot(t, root, makeHandoffCmd())
				cmd.SetArgs([]string{"write", "--change", tt.slug})
				err := cmd.Execute()

				cliErr := asCLIError(err)
				require.NotNil(t, cliErr)
				assert.Equal(t, "invalid_change_slug", cliErr.ErrorCode)
				assert.Equal(t, categoryInvalidUsage, cliErr.Category)
				assert.Equal(t, tt.slug, cliErr.Details["slug"])

				_, statErr := os.Stat(filepath.Join(state.GitRuntimeDir(root), "changes"))
				assert.True(t, os.IsNotExist(statErr), "invalid slug must not create the per-change runtime namespace")
				_, statErr = os.Stat(filepath.Join(state.GitRuntimeDir(root), "x", "handoff.md"))
				assert.True(t, os.IsNotExist(statErr), "parent traversal must not escape changes/<slug>")
			})
		}
	})
}

func TestHandoffNoopsWithoutActiveChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		cmd := commandForRoot(t, root, makeHandoffCmd())
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())
		assert.Empty(t, out.String())
	})
}
