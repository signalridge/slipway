package cmd

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

const sessionStartEntrySkill = "slipway"

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

// runSessionStartHook emits only the slipway_entry_skill routing pointer. It no
// longer auto-injects per-session change-state (no active-worktree `next --json`
// view, no bound-elsewhere handoff pointer, no handoff-brief summary): the
// per-change handoff is authored explicitly and read with `slipway handoff show
// --change <slug>`; lifecycle authority comes from `slipway status --json` /
// `slipway next --json`. The only conditional output is a hard-error diagnostic
// when the project root cannot be resolved.
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
