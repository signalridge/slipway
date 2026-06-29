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

// forceHandoffInteractive pins the handoff command's terminal probe so the
// non-interactive/interactive branches are deterministic regardless of the test
// runner's real stdin.
func forceHandoffInteractive(t *testing.T, interactive bool) {
	t.Helper()
	prev := handoffCommandIsTerminal
	handoffCommandIsTerminal = func(int) bool { return interactive }
	t.Cleanup(func() { handoffCommandIsTerminal = prev })
}

func TestHandoffCommandBareWriteAndShowBrief(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff command")
		forceHandoffInteractive(t, false)

		writeCmd := commandForRoot(t, root, makeHandoffCmd())
		writeCmd.SetIn(strings.NewReader("## Current Position\nDriving the headless-honesty fix.\n"))
		var writeOut bytes.Buffer
		writeCmd.SetOut(&writeOut)
		require.NoError(t, writeCmd.Execute())
		assert.Contains(t, writeOut.String(), "handoff_written:")

		path := state.ChangeHandoffPath(root, slug)
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "slipway:handoff-machine-header")
		assert.Contains(t, string(raw), "Driving the headless-honesty fix.")
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
		forceHandoffInteractive(t, false)

		cmd := commandForRoot(t, root, makeHandoffCmd())
		cmd.SetArgs([]string{"write", "--section", "Next Session Focus"})
		cmd.SetIn(strings.NewReader("Finish hook wiring."))
		require.NoError(t, cmd.Execute())

		// A subsequent bare write with a piped body merges its sections over the
		// existing narrative while preserving the previously recorded section, and
		// bumps the generation.
		refresh := commandForRoot(t, root, makeHandoffCmd())
		refresh.SetArgs([]string{"write"})
		refresh.SetIn(strings.NewReader("## Current Position\nRefining the fix.\n"))
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
		assert.Contains(t, view.Narrative, "Refining the fix.")
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

func TestHandoffBarePipedBodyPersistsAndReportsWritten(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff piped body")
		forceHandoffInteractive(t, false)

		cmd := commandForRoot(t, root, makeHandoffCmd())
		cmd.SetIn(strings.NewReader("## Current Position\nWorking the headless fix.\n## Risks And Blockers\nNone yet.\n"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())
		assert.Contains(t, out.String(), "handoff_written:")

		raw, err := os.ReadFile(state.ChangeHandoffPath(root, slug))
		require.NoError(t, err)
		assert.Contains(t, string(raw), "Working the headless fix.")
		assert.Contains(t, string(raw), "None yet.")
	})
}

func TestHandoffBareFreeformBodyRoutesToDefaultSection(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff freeform body")
		forceHandoffInteractive(t, false)

		cmd := commandForRoot(t, root, makeHandoffCmd())
		cmd.SetIn(strings.NewReader("No headers, just where we are.\n"))
		require.NoError(t, cmd.Execute())

		raw, err := os.ReadFile(state.ChangeHandoffPath(root, slug))
		require.NoError(t, err)
		// A free-form body with no recognizable section headers lands under the
		// default Current Position section so piped narrative is never dropped.
		assert.Contains(t, string(raw), "## Current Position")
		assert.Contains(t, string(raw), "No headers, just where we are.")
	})
}

func TestHandoffBarePipedBodyPreservesUnmatchedContentWithCanonicalSection(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff mixed body")
		forceHandoffInteractive(t, false)

		cmd := commandForRoot(t, root, makeHandoffCmd())
		cmd.SetIn(strings.NewReader("NOTE: blocked on review.\n\n## Operator Notes\nimportant detail\n\n## Next Session Focus\nFinish merge.\n"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())
		assert.Contains(t, out.String(), "handoff_written:")

		raw, err := os.ReadFile(state.ChangeHandoffPath(root, slug))
		require.NoError(t, err)
		assert.Contains(t, string(raw), "NOTE: blocked on review.")
		assert.Contains(t, string(raw), "## Operator Notes")
		assert.Contains(t, string(raw), "important detail")
		assert.Contains(t, string(raw), "Finish merge.")
	})
}

func TestHandoffWriteNonInteractiveEmptyBodyFailsLoudly(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff empty body")
		forceHandoffInteractive(t, false)

		cmd := commandForRoot(t, root, makeHandoffCmd())
		cmd.SetIn(strings.NewReader("   \n"))
		var out bytes.Buffer
		cmd.SetOut(&out)

		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "handoff_body_empty", cliErr.ErrorCode)
		assert.Equal(t, categoryInvalidUsage, cliErr.Category)
		assert.NotContains(t, out.String(), "handoff_written")
		_, statErr := os.Stat(state.ChangeHandoffPath(root, slug))
		assert.True(t, os.IsNotExist(statErr), "loud empty failure must not write a scaffold")
	})
}

func TestHandoffWriteSectionEmptyBodyFailsLoudly(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createGovernedRequest(t, root, levelNonDiscovery, "handoff section empty")
		forceHandoffInteractive(t, false)

		cmd := commandForRoot(t, root, makeHandoffCmd())
		cmd.SetArgs([]string{"write", "--section", "Risks And Blockers"})
		cmd.SetIn(strings.NewReader(""))
		var out bytes.Buffer
		cmd.SetOut(&out)

		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "handoff_body_empty", cliErr.ErrorCode)
		assert.Equal(t, "Risks And Blockers", cliErr.Details["section"])
		assert.NotContains(t, out.String(), "handoff_written")
	})
}

func TestHandoffWriteInteractiveSectionGuidesWithoutBlockingOrFalseSuccess(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff interactive section")
		forceHandoffInteractive(t, true)

		cmd := commandForRoot(t, root, makeHandoffCmd())
		cmd.SetArgs([]string{"write", "--section", "Next Session Focus"})
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		require.NoError(t, cmd.Execute())
		assert.NotContains(t, out.String(), "handoff_written")
		assert.Contains(t, errOut.String(), "section \"Next Session Focus\" needs a piped narrative")
		_, statErr := os.Stat(state.ChangeHandoffPath(root, slug))
		assert.True(t, os.IsNotExist(statErr), "interactive guidance must not write a scaffold")
	})
}

func TestHandoffWriteRejectsUnknownSection(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createGovernedRequest(t, root, levelNonDiscovery, "handoff unknown section")
		forceHandoffInteractive(t, false)

		cmd := commandForRoot(t, root, makeHandoffCmd())
		cmd.SetArgs([]string{"write", "--section", "Nonexistent Section"})
		cmd.SetIn(strings.NewReader("some content"))

		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "handoff_section_unknown", cliErr.ErrorCode)
		assert.Equal(t, categoryInvalidUsage, cliErr.Category)
		assert.Contains(t, cliErr.Remediation, "Current Position")
	})
}

func TestHandoffWriteInteractiveEmptyBodyGuidesWithoutFalseSuccess(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "handoff interactive empty")
		forceHandoffInteractive(t, true)

		cmd := commandForRoot(t, root, makeHandoffCmd())
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		require.NoError(t, cmd.Execute())
		assert.NotContains(t, out.String(), "handoff_written")
		assert.Contains(t, errOut.String(), "handoff not written")
		_, statErr := os.Stat(state.ChangeHandoffPath(root, slug))
		assert.True(t, os.IsNotExist(statErr), "interactive guidance must not write a scaffold")
	})
}

func TestHandoffShowEmptyHandoffPrintsNotice(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createGovernedRequest(t, root, levelNonDiscovery, "handoff show empty")

		show := commandForRoot(t, root, makeHandoffCmd())
		show.SetArgs([]string{"show"})
		var out bytes.Buffer
		show.SetOut(&out)
		require.NoError(t, show.Execute())
		assert.Contains(t, out.String(), "handoff is empty / all sections pending")
	})
}

func TestHandoffShowEmptyHandoffJSONFlagsEmpty(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createGovernedRequest(t, root, levelNonDiscovery, "handoff show empty json")

		show := commandForRoot(t, root, makeHandoffCmd())
		show.SetArgs([]string{"show", "--json"})
		var out bytes.Buffer
		show.SetOut(&out)
		require.NoError(t, show.Execute())
		var view handoffShowView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.True(t, view.Empty)
		assert.Contains(t, view.Notice, "pending")
	})
}
