package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

const sessionStartEntrySkill = "slipway"

// hookCommandName is the name of the parent command whose subtree is inlined
// into host automation. Errors from this subtree are swallowed to a silent
// exit 0 so a hook can never block the host (see isHookSubtreeCommand).
const hookCommandName = "hook"

func makeHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    hookCommandName,
		Short:  "Run internal Slipway hook helpers",
		Hidden: true,
		// Hooks are inlined into host automation and must stay inert under
		// version skew. If a generated config names a hook subcommand this
		// binary does not have, Cobra would otherwise print parent usage to the
		// host's hook output channel. Make the parent runnable so any
		// unresolved invocation returns an error that executeRootCommand
		// swallows to a silent exit 0 instead of emitting usage text.
		Args: cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("no hook subcommand resolved")
		},
	}
	cmd.AddCommand(makeSessionStartHookCmd())
	return cmd
}

func makeSessionStartHookCmd() *cobra.Command {
	var toolID string
	cmd := &cobra.Command{
		Use:    "session-start",
		Short:  "Emit SessionStart Slipway entry-skill routing pointer",
		Hidden: true,
		Args:   cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			// Fail silent: this hook is inlined into automatic host hooks and must
			// never surface a blocking or non-zero failure. Any internal error
			// (root lookup, JSON, write) is swallowed to a clean exit 0.
			_ = runSessionStartHook(cmd, toolID)
		},
	}
	cmd.Flags().StringVar(&toolID, "tool", "", "Host tool ID")
	return cmd
}

// runSessionStartHook emits the slipway_entry_skill routing pointer plus a
// static host-facing context note: Slipway does not watch context (the host owns
// the compact-vs-handoff decision) and the on-demand `slipway handoff` surface is
// an optional advisory narrative, not lifecycle authority. Both are static — the
// hook does NOT auto-inject per-session change-state (no active-worktree
// `next --json` view, no per-session continuity summary): governed continuity
// comes only from `slipway status --json` / `slipway next --json` over
// authoritative lifecycle state, which a resuming session runs itself. The only
// conditional output is a hard-error diagnostic when the project root cannot be
// resolved.
func runSessionStartHook(cmd *cobra.Command, toolID string) error {
	toolID = strings.TrimSpace(toolID)

	var diagnostics []string
	if _, err := projectRootFromCommand(cmd); err != nil {
		diagnostics = append(diagnostics, "hook_diagnostic: slipway root failed: "+normalizeHookDiagnostic(err.Error()))
	}
	return writeSessionStartHookOutput(cmd.OutOrStdout(), toolID, diagnostics)
}

func writeSessionStartHookOutput(w io.Writer, toolID string, diagnostics []string) error {
	if strings.EqualFold(strings.TrimSpace(toolID), "codex") {
		output := map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "SessionStart",
				"additionalContext": sessionStartAdditionalContext(toolID, diagnostics),
			},
		}
		encoded, err := json.Marshal(output)
		if err != nil {
			return nil
		}
		_, _ = w.Write(encoded)
		return nil
	}
	_, err := io.WriteString(w, sessionStartXMLContext(toolID, diagnostics))
	return err
}

func sessionStartXMLContext(toolID string, diagnostics []string) string {
	var b strings.Builder
	b.WriteString(`<slipway-session-start`)
	if strings.TrimSpace(toolID) != "" {
		b.WriteString(` tool="`)
		b.WriteString(escapeSessionStartAttr(toolID))
		b.WriteString(`"`)
	}
	b.WriteString(">\n")
	b.WriteString("slipway_entry_skill: This repository is governed by Slipway. To drive any non-trivial change through the governed lifecycle, load the \"")
	b.WriteString(sessionStartEntrySkill)
	b.WriteString("\" skill - the entry point that routes new/next/run/done.\n")
	b.WriteString("slipway_context_note: Slipway does not watch or measure your context window - you decide when to compact or start a fresh session using your host's own signal. The optional `slipway handoff write` / `slipway handoff show` surface records an advisory continuation narrative; it never replaces `slipway status` / `slipway next` as lifecycle authority.\n")
	for _, diagnostic := range diagnostics {
		diagnostic = strings.TrimSpace(diagnostic)
		if diagnostic == "" {
			continue
		}
		b.WriteString(diagnostic)
		b.WriteByte('\n')
	}
	b.WriteString("</slipway-session-start>\n")
	return b.String()
}

func sessionStartAdditionalContext(toolID string, diagnostics []string) string {
	xml := sessionStartXMLContext(toolID, diagnostics)
	xml = strings.TrimPrefix(xml, `<slipway-session-start tool="codex">`+"\n")
	xml = strings.TrimPrefix(xml, "<slipway-session-start>\n")
	xml = strings.TrimSuffix(xml, "</slipway-session-start>\n")
	return strings.TrimSpace(xml)
}

func normalizeHookDiagnostic(raw string) string {
	fields := strings.Fields(strings.ReplaceAll(raw, "\r", " "))
	return strings.Join(fields, " ")
}

func escapeSessionStartAttr(raw string) string {
	replacer := strings.NewReplacer(
		`&`, `&amp;`,
		`"`, `&quot;`,
		`<`, `&lt;`,
		`>`, `&gt;`,
	)
	return replacer.Replace(raw)
}
