package capability

import (
	"sort"
	"strings"
)

// SurfaceClass classifies a public surface record. Route-surface plan §4.2
// defines four exposure classes: primary, suggested, explicit-focus, and
// view. Only `primary`, `explicit_focus`, and `view` participate in surface
// policy lookup; `suggested` is kept descriptive here for completeness but
// runtime suggestions are still derived from BindingCommandAuto signals
// plus the surface policy's disjointness rule (§4.4).
type SurfaceClass string

const (
	SurfacePrimary       SurfaceClass = "primary"
	SurfaceSuggested     SurfaceClass = "suggested"
	SurfaceExplicitFocus SurfaceClass = "explicit_focus"
	SurfaceView          SurfaceClass = "view"
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
// mirror route-surface plan §5.1; explicit focuses mirror §5.3; views mirror
// §5.4.
var surfacePolicy = []SurfaceRecord{
	// §5.1 Primary routes — one per command surface.
	{
		Command: "review", Class: SurfacePrimary,
		PublicName: "independent-review", BackingID: "independent-review",
		Summary: "Stable default review contract with explicit verdict.",
	},
	{
		Command: "validate", Class: SurfacePrimary,
		PublicName: "spec-trace", BackingID: "spec-trace",
		Summary: "Default code-to-artifact verification trace.",
	},
	{
		Command: "repair", Class: SurfacePrimary,
		PublicName: "root-cause-tracing", BackingID: "root-cause-tracing",
		Summary: "Default repair posture: trace the root cause before fixing.",
	},
	{
		Command: "status", Class: SurfacePrimary,
		PublicName: "incident", BackingID: "incident-response",
		Summary: "Change-scoped incident-response diagnostic view.",
	},
	{
		Command: "health", Class: SurfacePrimary,
		PublicName: "incident", BackingID: "incident-response",
		Summary: "Change-scoped incident-response diagnostic view.",
	},

	// §5.3 Explicit focuses — small opt-in set.
	{
		Command: "review", Class: SurfaceExplicitFocus,
		PublicName: "sast", BackingID: "sast-orchestration",
		Summary: "Run SAST tooling (CodeQL/Semgrep) with SARIF triage.",
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

	// §5.4 Read-only views.
	{
		Command: "status", Class: SurfaceView,
		PublicName: "incident", BackingID: "incident-response",
		Summary: "Read-only incident-response diagnostic view.",
	},
	{
		Command: "health", Class: SurfaceView,
		PublicName: "incident", BackingID: "incident-response",
		Summary: "Read-only incident-response diagnostic view.",
	},
}

// AllSurfaces returns a defensive copy of the shipped surface policy in
// stable (command, class, public-name) order. Callers must not mutate the
// returned slice.
func AllSurfaces() []SurfaceRecord {
	out := make([]SurfaceRecord, len(surfacePolicy))
	copy(out, surfacePolicy)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Command != out[j].Command {
			return out[i].Command < out[j].Command
		}
		if out[i].Class != out[j].Class {
			return classOrder(out[i].Class) < classOrder(out[j].Class)
		}
		return out[i].PublicName < out[j].PublicName
	})
	return out
}

func classOrder(c SurfaceClass) int {
	switch c {
	case SurfacePrimary:
		return 0
	case SurfaceExplicitFocus:
		return 1
	case SurfaceView:
		return 2
	case SurfaceSuggested:
		return 3
	}
	return 4
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

// ViewsForCommand returns the view records registered for a command surface,
// sorted by public name.
func ViewsForCommand(command string) []SurfaceRecord {
	command = strings.TrimSpace(command)
	var out []SurfaceRecord
	for _, s := range surfacePolicy {
		if s.Class == SurfaceView && s.Command == command {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PublicName < out[j].PublicName })
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

// LookupView resolves a `--view <alias>` selector for a command surface.
func LookupView(command, alias string) (SurfaceRecord, bool) {
	command = strings.TrimSpace(command)
	alias = strings.TrimSpace(alias)
	for _, s := range surfacePolicy {
		if s.Class == SurfaceView && s.Command == command && s.PublicName == alias {
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
