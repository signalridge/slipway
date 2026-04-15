package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/capability"
)

func TestValidateRouteMode_EmptyAllowed(t *testing.T) {
	for _, cmd := range []string{"review", "validate", "repair"} {
		if err := validateRouteMode(cmd, ""); err != nil {
			t.Fatalf("%s: empty mode should be allowed: %v", cmd, err)
		}
	}
}

func TestValidateRouteMode_AcceptsValid(t *testing.T) {
	reg := capability.DefaultRegistry()
	for _, cmd := range []string{"review", "validate", "repair"} {
		modes := capability.ValidModesForCommand(reg, cmd)
		if len(modes) == 0 {
			t.Fatalf("%s: expected at least one valid mode in registry", cmd)
		}
		if err := validateRouteMode(cmd, modes[0]); err != nil {
			t.Fatalf("%s: mode %q should be accepted: %v", cmd, modes[0], err)
		}
	}
}

func TestValidateRouteMode_AcceptsRouteOnlyOverride(t *testing.T) {
	if err := validateRouteMode("review", "second-opinion"); err != nil {
		t.Fatalf("review: route-only mode should be accepted: %v", err)
	}
}

func TestValidateRouteMode_RejectsUnknown(t *testing.T) {
	for _, cmd := range []string{"review", "validate", "repair"} {
		err := validateRouteMode(cmd, "does-not-exist")
		if err == nil {
			t.Fatalf("%s: expected rejection for unknown mode", cmd)
		}
		cliErr := asCLIError(err)
		if cliErr == nil {
			t.Fatalf("%s: expected CLI error, got %T", cmd, err)
		}
		if cliErr.ErrorCode != "unknown_route_mode" {
			t.Fatalf("%s: unexpected error code %q", cmd, cliErr.ErrorCode)
		}
		if !strings.Contains(cliErr.Remediation, "Use one of:") {
			t.Fatalf("%s: remediation should list allowed values, got %q", cmd, cliErr.Remediation)
		}
	}
}

func TestValidateRouteView_EmptyAllowed(t *testing.T) {
	for _, cmd := range []string{"status", "health"} {
		if err := validateRouteView(cmd, ""); err != nil {
			t.Fatalf("%s: empty view should be allowed: %v", cmd, err)
		}
	}
}

func TestValidateRouteView_AcceptsValid(t *testing.T) {
	reg := capability.DefaultRegistry()
	for _, cmd := range []string{"status", "health"} {
		views := capability.ValidViewsForCommand(reg, cmd)
		if len(views) == 0 {
			t.Skipf("%s: no views registered yet", cmd)
		}
		if err := validateRouteView(cmd, views[0]); err != nil {
			t.Fatalf("%s: view %q should be accepted: %v", cmd, views[0], err)
		}
	}
}

func TestValidateRouteView_AcceptsRouteOnlyOverride(t *testing.T) {
	for _, tc := range []struct {
		command string
		view    string
	}{
		{command: "status", view: "review-queue"},
		{command: "status", view: "observability-query"},
		{command: "health", view: "observability-query"},
	} {
		if err := validateRouteView(tc.command, tc.view); err != nil {
			t.Fatalf("%s: route-only view %q should be accepted: %v", tc.command, tc.view, err)
		}
	}
}

func TestValidateRouteView_RejectsUnknown(t *testing.T) {
	for _, cmd := range []string{"status", "health"} {
		err := validateRouteView(cmd, "does-not-exist")
		if err == nil {
			t.Fatalf("%s: expected rejection for unknown view", cmd)
		}
		cliErr := asCLIError(err)
		if cliErr == nil {
			t.Fatalf("%s: expected CLI error, got %T", cmd, err)
		}
		if cliErr.ErrorCode != "unknown_route_view" {
			t.Fatalf("%s: unexpected error code %q", cmd, cliErr.ErrorCode)
		}
	}
}

func TestResolveEffectiveRouteMode_PrecedenceAndFallback(t *testing.T) {
	t.Run("explicit override wins", func(t *testing.T) {
		got := resolveEffectiveRouteMode("review", "security-review")
		if got != "security-review" {
			t.Fatalf("expected explicit mode override, got %q", got)
		}
	})

	t.Run("empty explicit falls back to resolver auto route", func(t *testing.T) {
		got := resolveEffectiveRouteMode("review", "")
		if got == "" {
			t.Fatal("expected resolver-selected review mode")
		}
	})

	t.Run("unknown command falls back to empty", func(t *testing.T) {
		got := resolveEffectiveRouteMode("next", "")
		if got != "" {
			t.Fatalf("expected empty mode fallback, got %q", got)
		}
	})
}

func TestResolveEffectiveRouteView_PrecedenceAndFallback(t *testing.T) {
	t.Run("explicit override wins", func(t *testing.T) {
		got := resolveEffectiveRouteView("status", "incident-response")
		if got != "incident-response" {
			t.Fatalf("expected explicit view override, got %q", got)
		}
	})

	t.Run("empty explicit falls back to resolver auto route", func(t *testing.T) {
		got := resolveEffectiveRouteView("status", "")
		if got == "" {
			t.Fatal("expected resolver-selected status view")
		}
	})

	t.Run("unknown command falls back to empty", func(t *testing.T) {
		got := resolveEffectiveRouteView("next", "")
		if got != "" {
			t.Fatalf("expected empty view fallback, got %q", got)
		}
	})
}

func TestResolveEffectiveRouteHydrate_ExplicitModeShortCircuitsResolver(t *testing.T) {
	for _, tc := range []struct {
		command    string
		mode       string
		wantPrefix string
		wantKey    string
	}{
		{command: "review", mode: "gha-security-review", wantPrefix: "gha-security-review/", wantKey: "gha-security-review/pwn-request.md"},
		{command: "validate", mode: "sast-orchestration", wantPrefix: "sast-orchestration/", wantKey: "sast-orchestration/codeql-ruleset-catalog.md"},
	} {
		got := resolveEffectiveRouteHydrate(tc.command, tc.mode)
		if len(got) == 0 {
			t.Fatalf("%s/%s: expected registry hydrate keys", tc.command, tc.mode)
		}
		for _, key := range got {
			if !strings.HasPrefix(key, tc.wantPrefix) {
				t.Fatalf("%s/%s: expected skill-relative hydrate key with prefix %q, got %q", tc.command, tc.mode, tc.wantPrefix, key)
			}
		}
		if !slices.Contains(got, tc.wantKey) {
			t.Fatalf("%s/%s: expected key %q in %v", tc.command, tc.mode, tc.wantKey, got)
		}
		normalized := normalizeHydrateKeys(got)
		if !sortedStrings(normalized) {
			t.Fatalf("%s/%s: expected normalizeHydrateKeys to stable-sort, got %v", tc.command, tc.mode, normalized)
		}
	}
}

func TestResolveEffectiveRouteHydrate_EmptyExplicitFallsBackToResolver(t *testing.T) {
	got := resolveEffectiveRouteHydrate("review", "")
	sig := capability.Signals{Command: "review"}
	want := capability.Resolve(capability.DefaultRegistry(), sig).HydrateReferences
	if !equalStrings(got, want) {
		t.Fatalf("expected resolver hydrate fallback %v, got %v", want, got)
	}
}

func TestResolveEffectiveRouteHydrate_ExplicitModeWithNoRefsDoesNotFallBack(t *testing.T) {
	got := resolveEffectiveRouteHydrate("repair", "review-comment-triage")
	if got != nil {
		t.Fatalf("expected nil for explicit zero-hydrate mode, got %v", got)
	}
}

func TestResolveEffectiveRouteHydrate_UnknownModeReturnsNil(t *testing.T) {
	got := resolveEffectiveRouteHydrate("review", "does-not-exist")
	if got != nil {
		t.Fatalf("expected nil for unknown mode, got %v", got)
	}
}

func TestResolveEffectiveViewHydrate_ExplicitViewShortCircuitsResolver(t *testing.T) {
	for _, tc := range []struct {
		command    string
		view       string
		wantPrefix string
		wantKey    string
	}{
		{command: "status", view: "supply-chain-audit", wantPrefix: "supply-chain-audit/", wantKey: "supply-chain-audit/results-template.md"},
		{command: "health", view: "incident-response", wantPrefix: "incident-response/", wantKey: "incident-response/incident-severity-matrix.md"},
	} {
		got := resolveEffectiveViewHydrate(tc.command, tc.view)
		if len(got) == 0 {
			t.Fatalf("%s/%s: expected registry hydrate keys", tc.command, tc.view)
		}
		for _, key := range got {
			if !strings.HasPrefix(key, tc.wantPrefix) {
				t.Fatalf("%s/%s: expected skill-relative hydrate key with prefix %q, got %q", tc.command, tc.view, tc.wantPrefix, key)
			}
		}
		if !slices.Contains(got, tc.wantKey) {
			t.Fatalf("%s/%s: expected key %q in %v", tc.command, tc.view, tc.wantKey, got)
		}
	}
}

func TestResolveEffectiveViewHydrate_EmptyExplicitFallsBackToResolver(t *testing.T) {
	got := resolveEffectiveViewHydrate("status", "")
	sig := capability.Signals{Command: "status"}
	want := capability.Resolve(capability.DefaultRegistry(), sig).HydrateReferences
	if !equalStrings(got, want) {
		t.Fatalf("expected resolver hydrate fallback %v, got %v", want, got)
	}
}

func TestResolveEffectiveViewHydrate_ExplicitViewWithNoRefsDoesNotFallBack(t *testing.T) {
	got := resolveEffectiveViewHydrate("status", "ci-triage")
	if got != nil {
		t.Fatalf("expected nil for explicit zero-hydrate view, got %v", got)
	}
}

func sortedStrings(ss []string) bool {
	for i := 1; i < len(ss); i++ {
		if ss[i-1] > ss[i] {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
