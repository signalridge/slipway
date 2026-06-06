package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/signalridge/slipway/internal/engine/capability"
	"github.com/spf13/cobra"
)

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
	remediation := fmt.Sprintf("Use one of: %s", strings.Join(allowed, ", "))
	// `repair` deliberately drops the `sast` focus: repair never runs external
	// scanners. Redirect the operator to the surfaces that legitimately hydrate
	// SAST guidance instead of leaving a bare allowed-list.
	if command == "repair" && alias == "sast" {
		remediation = "repair performs local-state integrity only and does not run SAST; to run SAST analysis use `slipway review --focus sast` or `slipway validate --focus sast`."
	}
	return newInvalidUsageError(
		"unknown_route_mode",
		fmt.Sprintf("unknown --focus=%q for `%s`", alias, command),
		remediation,
		map[string]any{"command": command, "focus": alias, "allowed": allowed},
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
