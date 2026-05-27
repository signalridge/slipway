package capability

import (
	"sort"
	"strings"
)

// SurfaceClass classifies a public surface record. Two classes remain:
// primary (the default route for a command) and explicit_focus (opt-in
// selection via --focus).
type SurfaceClass string

const (
	SurfacePrimary       SurfaceClass = "primary"
	SurfaceExplicitFocus SurfaceClass = "explicit_focus"
)

// SurfaceRecord is a single public-surface entry. Every record in this plan
// family is skill-backed (`BackingID` names a catalog skill). If a later
// plan introduces command-owned diagnostics, it must widen the schema there
// rather than pre-allocating a second backing kind in PR-1.
type SurfaceRecord struct {
	Command    string
	Class      SurfaceClass
	PublicName string
	BackingID  string
	Summary    string
}

// surfacePolicy is the checked-in catalog of public surfaces. Primary routes
// mirror route-surface plan §5.1; explicit focuses mirror §5.3 after the
// suggested/view cleanup.
var surfacePolicy = []SurfaceRecord{
	// §5.1 Primary routes — one per command surface.
	// status and validate no longer have primary routes; their default output
	// is neutral and state-focused. Expert posture is available via --focus.
	{
		Command: "review", Class: SurfacePrimary,
		PublicName: "independent-review", BackingID: "independent-review",
		Summary: "Stable default review contract with explicit verdict.",
	},
	{
		Command: "repair", Class: SurfacePrimary,
		PublicName: "root-cause-tracing", BackingID: "root-cause-tracing",
		Summary: "Default repair posture: trace the root cause before fixing.",
	},
	{
		Command: "health", Class: SurfacePrimary,
		PublicName: "incident", BackingID: "incident-response",
		Summary: "Change-scoped incident-response diagnostic view.",
	},

	// §5.3 Explicit focuses — small opt-in set.
	//
	// After Wave-3 view→focus unification, status/health "incident" is
	// exposed as an explicit focus so `--focus incident` validates.
	{
		Command: "status", Class: SurfaceExplicitFocus,
		PublicName: "incident", BackingID: "incident-response",
		Summary: "Incident-response diagnostic focus for status.",
	},
	{
		Command: "health", Class: SurfaceExplicitFocus,
		PublicName: "incident", BackingID: "incident-response",
		Summary: "Incident-response diagnostic focus for health.",
	},
	{
		Command: "review", Class: SurfaceExplicitFocus,
		PublicName: "sast", BackingID: "sast-orchestration",
		Summary: "Run SAST tooling (CodeQL/Semgrep) with SARIF triage.",
	},
	{
		Command: "validate", Class: SurfaceExplicitFocus,
		PublicName: "spec-trace", BackingID: "spec-trace",
		Summary: "Code-to-artifact verification trace.",
	},
	{
		Command: "validate", Class: SurfaceExplicitFocus,
		PublicName: "sast", BackingID: "sast-orchestration",
		Summary: "Run SAST tooling (CodeQL/Semgrep) with SARIF triage.",
	},
	{
		Command: "repair", Class: SurfaceExplicitFocus,
		PublicName: "sast", BackingID: "sast-orchestration",
		Summary: "Run SAST tooling (CodeQL/Semgrep) with SARIF triage.",
	},
	{
		Command: "review", Class: SurfaceExplicitFocus,
		PublicName: "calibration", BackingID: "multi-reviewer-calibration",
		Summary: "Calibrate multiple reviewers on severity and scope before merging.",
	},
	{
		Command: "validate", Class: SurfaceExplicitFocus,
		PublicName: "property", BackingID: "property-testing",
		Summary: "Write property-based tests that specify invariants.",
	},
	{
		Command: "validate", Class: SurfaceExplicitFocus,
		PublicName: "mutation", BackingID: "mutation-testing",
		Summary: "Score test strength with mutation testing.",
	},
}

// PrimaryForCommand returns the primary surface record for a command
// surface, if any. For `status` / `health`, the command layer is responsible
// for gating use on an active/selected change target before consulting the
// primary view (route-surface plan §4.2, §6 acceptance).
func PrimaryForCommand(command string) (SurfaceRecord, bool) {
	command = strings.TrimSpace(command)
	for _, s := range surfacePolicy {
		if s.Class == SurfacePrimary && s.Command == command {
			return s, true
		}
	}
	return SurfaceRecord{}, false
}

// ExplicitFocusesForCommand returns the explicit-focus records registered
// for a command surface, sorted by public name for stable discovery output.
func ExplicitFocusesForCommand(command string) []SurfaceRecord {
	command = strings.TrimSpace(command)
	var out []SurfaceRecord
	for _, s := range surfacePolicy {
		if s.Class == SurfaceExplicitFocus && s.Command == command {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PublicName < out[j].PublicName })
	return out
}

// ExplicitFocusSurfaces returns every public explicit-focus surface, sorted by
// command and public alias for deterministic generated documentation.
func ExplicitFocusSurfaces() []SurfaceRecord {
	var out []SurfaceRecord
	for _, s := range surfacePolicy {
		if s.Class == SurfaceExplicitFocus {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Command != out[j].Command {
			return out[i].Command < out[j].Command
		}
		return out[i].PublicName < out[j].PublicName
	})
	return out
}

// LookupFocus resolves a `--focus <alias>` selector for a command surface.
func LookupFocus(command, alias string) (SurfaceRecord, bool) {
	command = strings.TrimSpace(command)
	alias = strings.TrimSpace(alias)
	for _, s := range surfacePolicy {
		if s.Class == SurfaceExplicitFocus && s.Command == command && s.PublicName == alias {
			return s, true
		}
	}
	return SurfaceRecord{}, false
}

// ExplicitFocusBackingIDs returns the set of catalog skill IDs that back
// any public `--focus` alias. Resolver uses this set to suppress hydrate
// emission on implicit paths (route-surface plan §6: explicit-focus skills
// hydrate only under explicit selection).
func ExplicitFocusBackingIDs() map[string]struct{} {
	out := make(map[string]struct{})
	for _, s := range surfacePolicy {
		if s.Class == SurfaceExplicitFocus {
			out[s.BackingID] = struct{}{}
		}
	}
	return out
}
