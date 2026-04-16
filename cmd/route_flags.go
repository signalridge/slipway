package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/signalridge/slipway/internal/engine/capability"
	"github.com/signalridge/slipway/internal/model"
	"github.com/spf13/cobra"
)

// suggestedCapabilityView is the stable JSON/text shape for entries in a
// routed command's `suggested_capabilities[]` channel (route-surface plan
// §4.4). Score is intentionally dropped from the public view for stability.
type suggestedCapabilityView struct {
	Name    string `json:"name"`
	Summary string `json:"summary,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

// validateFocus verifies that the user-supplied --focus alias resolves via
// surface policy for the given command. An empty alias is allowed (meaning
// "use the primary route").
func validateFocus(command, alias string) error {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return nil
	}
	if _, ok := capability.LookupFocus(command, alias); ok {
		return nil
	}
	allowed := focusAliases(command)
	return newInvalidUsageError(
		"unknown_route_mode",
		fmt.Sprintf("unknown --focus=%q for `%s`", alias, command),
		fmt.Sprintf("Use one of: %s", strings.Join(allowed, ", ")),
		map[string]any{"command": command, "focus": alias, "allowed": allowed},
	)
}

// validateViewAlias verifies that the user-supplied --view alias resolves via
// surface policy for the given status/health command. An empty alias is
// allowed.
func validateViewAlias(command, alias string) error {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return nil
	}
	if _, ok := capability.LookupView(command, alias); ok {
		return nil
	}
	allowed := viewAliases(command)
	return newInvalidUsageError(
		"unknown_route_view",
		fmt.Sprintf("unknown --view=%q for `%s`", alias, command),
		fmt.Sprintf("Use one of: %s", strings.Join(allowed, ", ")),
		map[string]any{"command": command, "view": alias, "allowed": allowed},
	)
}

// focusAliases returns the sorted list of public focus aliases for the given
// command (surface policy §5.3).
func focusAliases(command string) []string {
	recs := capability.ExplicitFocusesForCommand(command)
	out := make([]string, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.PublicName)
	}
	sort.Strings(out)
	return out
}

// viewAliases returns the sorted list of public view aliases for a status/
// health command (surface policy §5.4).
func viewAliases(command string) []string {
	recs := capability.ViewsForCommand(command)
	out := make([]string, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.PublicName)
	}
	sort.Strings(out)
	return out
}

// resolveEffectiveFocus returns the effective focus alias for a command
// surface. Precedence: explicit --focus alias > resolver auto route > empty.
func resolveEffectiveFocus(command, alias string) string {
	alias = strings.TrimSpace(alias)
	if alias != "" {
		if rec, ok := capability.LookupFocus(command, alias); ok {
			return rec.PublicName
		}
	}
	resolution := capability.Resolve(capability.DefaultRegistry(), capability.Signals{
		Command: command,
		Focus:   alias,
	})
	if resolution.Route == nil {
		return ""
	}
	return strings.TrimSpace(resolution.Route.Mode)
}

// resolveEffectiveView returns the effective view alias for status/health.
// Precedence: explicit --view alias > primary surface view > empty.
func resolveEffectiveView(command, alias string) string {
	alias = strings.TrimSpace(alias)
	if alias != "" {
		if rec, ok := capability.LookupView(command, alias); ok {
			return rec.PublicName
		}
	}
	if rec, ok := capability.PrimaryForCommand(command); ok {
		return rec.PublicName
	}
	return ""
}

// resolveEffectiveFocusHydrate returns the hydrate reference keys for the
// effective --focus selection. Explicit aliases short-circuit to the backing
// skill's hydrate keys; empty falls through to resolver output (union of
// route + supports).
func resolveEffectiveFocusHydrate(command, alias string) []string {
	reg := capability.DefaultRegistry()
	alias = strings.TrimSpace(alias)
	if alias != "" {
		if rec, ok := capability.LookupFocus(command, alias); ok {
			return capability.HydrateReferenceKeysForSkill(reg, rec.BackingID)
		}
		return nil
	}
	return capability.Resolve(reg, capability.Signals{Command: command}).HydrateReferences
}

// resolveEffectiveViewHydrate mirrors resolveEffectiveFocusHydrate for the
// status/health --view surfaces.
func resolveEffectiveViewHydrate(command, alias string) []string {
	reg := capability.DefaultRegistry()
	alias = strings.TrimSpace(alias)
	if alias != "" {
		if rec, ok := capability.LookupView(command, alias); ok {
			return capability.HydrateReferenceKeysForSkill(reg, rec.BackingID)
		}
		return nil
	}
	return capability.Resolve(reg, capability.Signals{Command: command}).HydrateReferences
}

// writeSuggestedBlock emits the `Suggested:` text block for routed command
// text output. When the list is empty, no lines are written.
func writeSuggestedBlock(writer *formatWriter, suggestions []suggestedCapabilityView) {
	if len(suggestions) == 0 {
		return
	}
	writer.Writef("Suggested:\n")
	for _, s := range suggestions {
		line := "  - " + s.Name
		reason := strings.TrimSpace(s.Reason)
		if reason == "" {
			reason = strings.TrimSpace(s.Summary)
		}
		if reason != "" {
			line += " — " + reason
		}
		writer.Writef("%s\n", line)
	}
}

// focusDiscoveryEntry is the JSON shape for `--list-focuses` output.
type focusDiscoveryEntry struct {
	Name    string `json:"name"`
	Summary string `json:"summary,omitempty"`
}

// focusDiscoveryOutput is the top-level JSON shape for `--list-focuses`.
type focusDiscoveryOutput struct {
	Command string                `json:"command"`
	Focuses []focusDiscoveryEntry `json:"focuses"`
}

// viewDiscoveryOutput is the top-level JSON shape for `--list-views`.
type viewDiscoveryOutput struct {
	Command string                `json:"command"`
	Views   []focusDiscoveryEntry `json:"views"`
}

// emitFocusDiscovery writes the `--list-focuses` response for a command,
// short-circuiting normal execution before any workspace access.
func emitFocusDiscovery(cmd *cobra.Command, command, format string) error {
	records := capability.ExplicitFocusesForCommand(command)
	entries := make([]focusDiscoveryEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, focusDiscoveryEntry{Name: r.PublicName, Summary: r.Summary})
	}
	return emitDiscovery(cmd, format, focusDiscoveryOutput{Command: command, Focuses: entries}, entries)
}

// emitViewDiscovery writes the `--list-views` response for a command.
func emitViewDiscovery(cmd *cobra.Command, command, format string) error {
	records := capability.ViewsForCommand(command)
	entries := make([]focusDiscoveryEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, focusDiscoveryEntry{Name: r.PublicName, Summary: r.Summary})
	}
	return emitDiscovery(cmd, format, viewDiscoveryOutput{Command: command, Views: entries}, entries)
}

func emitDiscovery(cmd *cobra.Command, format string, jsonPayload any, entries []focusDiscoveryEntry) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		w := cmd.OutOrStdout()
		for _, e := range entries {
			if _, err := fmt.Fprintf(w, "%s\t%s\n", e.Name, e.Summary); err != nil {
				return err
			}
		}
		return nil
	case "json":
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(jsonPayload)
	default:
		return newInvalidUsageError(
			"invalid_format",
			fmt.Sprintf("invalid --format %q; expected text|json", format),
			"Use --format text or --format json.",
			nil,
		)
	}
}

// buildSuggestedCapabilities projects the resolver's SuggestedCapabilities
// onto the stable public view. Score is intentionally dropped.
func buildSuggestedCapabilities(sig capability.Signals) []suggestedCapabilityView {
	sig.Command = strings.TrimSpace(sig.Command)
	sig.Focus = strings.TrimSpace(sig.Focus)
	sig.View = strings.TrimSpace(sig.View)
	sig.UserText = strings.TrimSpace(sig.UserText)
	sig.Blockers = uniqueSortedNonEmpty(sig.Blockers)
	sig.ChangedFiles = uniqueSortedNonEmpty(sig.ChangedFiles)
	sig.Paths = uniqueSortedNonEmpty(sig.Paths)

	resolution := capability.Resolve(capability.DefaultRegistry(), sig)
	if len(resolution.SuggestedCapabilities) == 0 {
		return nil
	}
	out := make([]suggestedCapabilityView, 0, len(resolution.SuggestedCapabilities))
	for _, s := range resolution.SuggestedCapabilities {
		out = append(out, suggestedCapabilityView{
			Name:    s.Name,
			Summary: s.Summary,
			Reason:  s.Reason,
			Kind:    s.Kind,
		})
	}
	return out
}

func suggestedCapabilitySignalsForChange(
	command, focus string,
	change model.Change,
	summary *model.ExecutionSummary,
	blockers []model.ReasonCode,
) capability.Signals {
	sig := capability.Signals{
		Command:  command,
		Focus:    strings.TrimSpace(focus),
		UserText: strings.TrimSpace(change.Description),
	}
	sig.ChangedFiles = executionSummaryChangedFiles(summary)
	sig.Blockers = uniqueSortedNonEmpty(append(executionSummaryBlockerSpecs(summary), model.ReasonSpecs(blockers)...))
	return sig
}

func executionSummaryChangedFiles(summary *model.ExecutionSummary) []string {
	if summary == nil {
		return nil
	}
	var out []string
	for _, task := range summary.Tasks {
		out = append(out, task.ChangedFiles...)
	}
	return uniqueSortedNonEmpty(out)
}

func executionSummaryBlockerSpecs(summary *model.ExecutionSummary) []string {
	if summary == nil {
		return nil
	}
	out := model.ReasonSpecs(summary.OpenBlockers)
	for _, task := range summary.Tasks {
		out = append(out, model.ReasonSpecs(task.Blockers)...)
	}
	return uniqueSortedNonEmpty(out)
}

func uniqueSortedNonEmpty(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}
