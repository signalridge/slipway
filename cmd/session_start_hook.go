package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

const sessionStartEntrySkill = "slipway"

func makeSessionStartHookCmd() *cobra.Command {
	var toolID string
	cmd := &cobra.Command{
		Use:    "session-start",
		Short:  "Emit SessionStart Slipway handoff context",
		Hidden: true,
		Args:   cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			// Fail silent: this hook is inlined into automatic host hooks and must
			// never surface a blocking or non-zero failure. Any internal error
			// (root lookup, state lock, JSON, write) is swallowed to a clean exit 0.
			_ = runSessionStartHook(cmd, toolID)
		},
	}
	cmd.Flags().StringVar(&toolID, "tool", "", "Host tool ID")
	return cmd
}

func runSessionStartHook(cmd *cobra.Command, toolID string) error {
	toolID = strings.TrimSpace(toolID)

	root, err := projectRootFromCommand(cmd)
	if err != nil {
		return writeSessionStartHookOutput(
			cmd.OutOrStdout(),
			toolID,
			"",
			"",
			[]string{"hook_diagnostic: slipway root failed: " + normalizeHookDiagnostic(err.Error())},
			"",
		)
	}

	var (
		diagnostics []string
		handoffInfo string
		changeSlug  string
	)
	nextJSON, changeSlug, err := sessionStartNextJSON(cmd, root)
	if err != nil {
		// A change bound to another worktree is an expected, informational state
		// when a session opens with no active change of its own: point the host
		// at the bound change instead of an alarming next-failed diagnostic, and
		// thread that change's slug so its handoff excerpt is surfaced below. The
		// bound change has no next --json here, so the handoff is the only
		// continuity signal the next agent gets.
		boundSlug, boundInfo := sessionStartBoundWorktreeHandoff(err)
		if boundInfo != "" {
			handoffInfo = boundInfo
			changeSlug = boundSlug
		} else {
			diagnostics = append(diagnostics, "hook_diagnostic: slipway next --json failed: "+normalizeHookDiagnostic(err.Error()))
		}
	}

	handoffSummary := sessionStartHandoffSummary(root, changeSlug)
	if nextJSON == "" && handoffInfo == "" && handoffSummary == "" && len(diagnostics) == 0 {
		return nil
	}
	return writeSessionStartHookOutput(cmd.OutOrStdout(), toolID, nextJSON, handoffInfo, diagnostics, handoffSummary)
}

// sessionStartBoundWorktreeHandoff renders the friendly informational line for
// the change_bound_to_other_worktree precondition and returns the bound change
// slug alongside it. When a session opens where no change is active for the
// current worktree while a change is bound elsewhere, the host is pointed at
// that change instead of seeing an alarming "next --json failed" diagnostic, and
// the returned slug lets the caller surface that change's handoff excerpt.
// Returns ("", "") for any other error or when the bound change details are
// incomplete, so the caller falls back to the diagnostic path.
func sessionStartBoundWorktreeHandoff(err error) (slug, info string) {
	cliErr := asCLIError(err)
	if cliErr == nil || cliErr.ErrorCode != "change_bound_to_other_worktree" {
		return "", ""
	}
	slug, worktreePath := firstBoundChange(cliErr.Details)
	if slug == "" || worktreePath == "" {
		return "", ""
	}
	return slug, fmt.Sprintf(
		"session_handoff_info: no active change in this worktree; active change %s is bound to %s; cd there or use --change %s to act",
		slug, worktreePath, slug,
	)
}

// firstBoundChange extracts the slug and worktree path of the first bound change
// from a change_bound_to_other_worktree error's details payload.
func firstBoundChange(details map[string]any) (slug, worktreePath string) {
	changes, ok := details["bound_changes"].([]map[string]string)
	if !ok || len(changes) == 0 {
		return "", ""
	}
	return changes[0]["slug"], changes[0]["worktree_path"]
}

func sessionStartNextJSON(cmd *cobra.Command, root string) (string, string, error) {
	ref, err := resolveActiveChangeRef(root, "")
	if err != nil {
		return "", "", err
	}

	auto, err := resolveEffectiveAuto(root, nil, false, false)
	if err != nil {
		return "", "", err
	}

	var out string
	err = withChangeStateLock(root, ref.Slug, "hook session-start", func() error {
		view, err := buildNextHandoffSourceView(root, ref, true, false, false, auto)
		if err != nil {
			return err
		}
		applyNextInvocationWorkspacePath(cmd, root, &view)
		raw, err := json.MarshalIndent(buildNextHandoffView(view), "", "  ")
		if err != nil {
			return err
		}
		out = string(raw)
		return nil
	})
	return out, ref.Slug, err
}

func sessionStartHandoffSummary(root, slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ""
	}
	if err := state.ValidateChangeSlug(slug); err != nil {
		return ""
	}
	handoffPath := state.ChangeHandoffPath(root, slug)
	if doc, ok, err := readHandoffDocument(root, slug); err == nil && ok {
		if excerpt := strings.TrimSpace(state.HandoffExcerpt(doc)); excerpt != "" {
			return excerpt
		}
	}
	present := "false"
	if _, err := os.Stat(handoffPath); err == nil {
		present = "true"
	}
	return fmt.Sprintf("session_handoff_present: %s\nsession_handoff_path: %s", present, handoffPath)
}

func writeSessionStartHookOutput(w io.Writer, toolID, nextJSON, handoffInfo string, diagnostics []string, handoffSummary string) error {
	if strings.EqualFold(strings.TrimSpace(toolID), "codex") {
		output := map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "SessionStart",
				"additionalContext": sessionStartAdditionalContext(toolID, nextJSON, handoffInfo, diagnostics, handoffSummary),
			},
		}
		encoded, err := json.Marshal(output)
		if err != nil {
			return nil
		}
		_, _ = w.Write(encoded)
		return nil
	}
	_, err := io.WriteString(w, sessionStartXMLContext(toolID, nextJSON, handoffInfo, diagnostics, handoffSummary))
	return err
}

func sessionStartXMLContext(toolID, nextJSON, handoffInfo string, diagnostics []string, handoffSummary string) string {
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
	if nextJSON != "" {
		b.WriteString(nextJSON)
		b.WriteByte('\n')
	}
	if strings.TrimSpace(handoffInfo) != "" {
		b.WriteString(handoffInfo)
		b.WriteByte('\n')
	}
	for _, diagnostic := range diagnostics {
		diagnostic = strings.TrimSpace(diagnostic)
		if diagnostic == "" {
			continue
		}
		b.WriteString(diagnostic)
		b.WriteByte('\n')
	}
	if strings.TrimSpace(handoffSummary) != "" {
		b.WriteString(handoffSummary)
		b.WriteByte('\n')
	}
	b.WriteString("</slipway-session-start>\n")
	return b.String()
}

func sessionStartAdditionalContext(toolID, nextJSON, handoffInfo string, diagnostics []string, handoffSummary string) string {
	xml := sessionStartXMLContext(toolID, nextJSON, handoffInfo, diagnostics, handoffSummary)
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
