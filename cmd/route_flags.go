package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/capability"
)

var routeOnlyModeOverrides = map[string][]string{
	"review": {"second-opinion"},
}

var routeOnlyViewOverrides = map[string][]string{
	"status": {"review-queue", "observability-query"},
	"health": {"observability-query"},
}

func allowedRouteModes(command string) []string {
	reg := capability.DefaultRegistry()
	modes := append([]string(nil), capability.ValidModesForCommand(reg, command)...)
	modes = append(modes, routeOnlyModeOverrides[command]...)
	slices.Sort(modes)
	return slices.Compact(modes)
}

func allowedRouteViews(command string) []string {
	reg := capability.DefaultRegistry()
	views := append([]string(nil), capability.ValidViewsForCommand(reg, command)...)
	views = append(views, routeOnlyViewOverrides[command]...)
	slices.Sort(views)
	return slices.Compact(views)
}

// validateRouteMode verifies that the user-supplied --mode value is a valid
// route for the given command. Returns a CLI usage error when the value is
// unknown so the user sees the allowed list. An empty mode is allowed
// (meaning "use the default route").
func validateRouteMode(command, mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return nil
	}
	modes := allowedRouteModes(command)
	for _, m := range modes {
		if m == mode {
			return nil
		}
	}
	return newInvalidUsageError(
		"unknown_route_mode",
		fmt.Sprintf("unknown --mode=%q for `%s`", mode, command),
		fmt.Sprintf("Use one of: %s", strings.Join(modes, ", ")),
		map[string]any{"command": command, "mode": mode, "allowed": modes},
	)
}

// validateRouteView verifies that the user-supplied --view value is a valid
// view for the given status/health command. Empty view means "use the
// default view".
func validateRouteView(command, view string) error {
	view = strings.TrimSpace(view)
	if view == "" {
		return nil
	}
	views := allowedRouteViews(command)
	for _, v := range views {
		if v == view {
			return nil
		}
	}
	return newInvalidUsageError(
		"unknown_route_view",
		fmt.Sprintf("unknown --view=%q for `%s`", view, command),
		fmt.Sprintf("Use one of: %s", strings.Join(views, ", ")),
		map[string]any{"command": command, "view": view, "allowed": views},
	)
}

// resolveEffectiveRouteMode returns the effective mode for a command surface.
// Precedence is explicit flag > resolver auto-route > empty.
func resolveEffectiveRouteMode(command, explicit string, signals ...capability.Signals) string {
	mode := strings.TrimSpace(explicit)
	if mode != "" {
		return mode
	}
	var sig capability.Signals
	if len(signals) > 0 {
		sig = signals[0]
	}
	if strings.TrimSpace(sig.Command) == "" {
		sig.Command = command
	}
	resolution := capability.Resolve(capability.DefaultRegistry(), sig)
	if resolution.Route == nil {
		return ""
	}
	return strings.TrimSpace(resolution.Route.Mode)
}

// resolveEffectiveRouteView returns the effective view for status/health.
// Precedence is explicit flag > resolver auto-route > empty.
func resolveEffectiveRouteView(command, explicit string, signals ...capability.Signals) string {
	view := strings.TrimSpace(explicit)
	if view != "" {
		return view
	}
	var sig capability.Signals
	if len(signals) > 0 {
		sig = signals[0]
	}
	if strings.TrimSpace(sig.Command) == "" {
		sig.Command = command
	}
	resolution := capability.Resolve(capability.DefaultRegistry(), sig)
	if resolution.Route == nil {
		return ""
	}
	return strings.TrimSpace(resolution.Route.View)
}

// resolveEffectiveRouteHydrate returns the hydrate references that apply to
// the effective --mode selection. Explicit manual modes short-circuit the
// resolver: the registry is the authority on that skill's hydrate keys and
// we never fall back to auto-route output on the explicit path. Empty
// explicit mode falls through to resolver output (union of route +
// supports) so auto-route callers see the same keys as Resolve().
func resolveEffectiveRouteHydrate(command, explicit string, signals ...capability.Signals) []string {
	reg := capability.DefaultRegistry()
	if mode := strings.TrimSpace(explicit); mode != "" {
		return capability.HydrateReferenceKeysForSkill(reg, mode)
	}
	var sig capability.Signals
	if len(signals) > 0 {
		sig = signals[0]
	}
	if strings.TrimSpace(sig.Command) == "" {
		sig.Command = command
	}
	return capability.Resolve(reg, sig).HydrateReferences
}

// resolveEffectiveViewHydrate mirrors resolveEffectiveRouteHydrate for the
// status/health --view surfaces.
func resolveEffectiveViewHydrate(command, explicit string, signals ...capability.Signals) []string {
	reg := capability.DefaultRegistry()
	if view := strings.TrimSpace(explicit); view != "" {
		return capability.HydrateReferenceKeysForSkill(reg, view)
	}
	var sig capability.Signals
	if len(signals) > 0 {
		sig = signals[0]
	}
	if strings.TrimSpace(sig.Command) == "" {
		sig.Command = command
	}
	return capability.Resolve(reg, sig).HydrateReferences
}
