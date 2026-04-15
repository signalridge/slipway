package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/capability"
)

func TestValidateFocus_EmptyAllowed(t *testing.T) {
	for _, cmd := range []string{"review", "validate", "repair"} {
		if err := validateFocus(cmd, ""); err != nil {
			t.Fatalf("%s: empty focus should be allowed: %v", cmd, err)
		}
	}
}

func TestValidateFocus_AcceptsPublicAliases(t *testing.T) {
	cases := []struct {
		command string
		alias   string
	}{
		{"review", "sast"},
		{"review", "calibration"},
		{"validate", "sast"},
		{"validate", "property"},
		{"validate", "mutation"},
		{"repair", "sast"},
	}
	for _, tc := range cases {
		if err := validateFocus(tc.command, tc.alias); err != nil {
			t.Fatalf("%s: focus %q should be accepted: %v", tc.command, tc.alias, err)
		}
	}
}

func TestValidateFocus_RejectsPrimaryName(t *testing.T) {
	// The primary route "independent-review" is NOT an explicit focus alias.
	err := validateFocus("review", "independent-review")
	if err == nil {
		t.Fatal("expected rejection for primary route name")
	}
	cliErr := asCLIError(err)
	if cliErr == nil || cliErr.ErrorCode != "unknown_route_mode" {
		t.Fatalf("expected unknown_route_mode, got %v", err)
	}
}

func TestValidateFocus_RejectsLegacySecondOpinion(t *testing.T) {
	err := validateFocus("review", "second-opinion")
	if err == nil {
		t.Fatal("expected rejection for legacy `second-opinion`")
	}
	cliErr := asCLIError(err)
	if cliErr == nil || cliErr.ErrorCode != "unknown_route_mode" {
		t.Fatalf("expected unknown_route_mode, got %v", err)
	}
}

func TestValidateFocus_RejectsUnknownWithRemediation(t *testing.T) {
	for _, cmd := range []string{"review", "validate", "repair"} {
		err := validateFocus(cmd, "does-not-exist")
		if err == nil {
			t.Fatalf("%s: expected rejection for unknown focus", cmd)
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

func TestValidateViewAlias_EmptyAllowed(t *testing.T) {
	for _, cmd := range []string{"status", "health"} {
		if err := validateViewAlias(cmd, ""); err != nil {
			t.Fatalf("%s: empty view should be allowed: %v", cmd, err)
		}
	}
}

func TestValidateViewAlias_AcceptsIncident(t *testing.T) {
	for _, cmd := range []string{"status", "health"} {
		if err := validateViewAlias(cmd, "incident"); err != nil {
			t.Fatalf("%s: view `incident` should be accepted: %v", cmd, err)
		}
	}
}

func TestValidateViewAlias_RejectsLegacyReviewQueue(t *testing.T) {
	err := validateViewAlias("status", "review-queue")
	if err == nil {
		t.Fatal("expected rejection for legacy `review-queue`")
	}
	cliErr := asCLIError(err)
	if cliErr == nil || cliErr.ErrorCode != "unknown_route_view" {
		t.Fatalf("expected unknown_route_view, got %v", err)
	}
}

func TestValidateViewAlias_RejectsLegacyObservabilityQuery(t *testing.T) {
	for _, cmd := range []string{"status", "health"} {
		err := validateViewAlias(cmd, "observability-query")
		if err == nil {
			t.Fatalf("%s: expected rejection for legacy `observability-query`", cmd)
		}
		cliErr := asCLIError(err)
		if cliErr == nil || cliErr.ErrorCode != "unknown_route_view" {
			t.Fatalf("%s: expected unknown_route_view, got %v", cmd, err)
		}
	}
}

func TestValidateViewAlias_RejectsUnknown(t *testing.T) {
	for _, cmd := range []string{"status", "health"} {
		err := validateViewAlias(cmd, "does-not-exist")
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

func TestResolveEffectiveFocus_PrecedenceAndFallback(t *testing.T) {
	t.Run("explicit alias wins and returns public name", func(t *testing.T) {
		got := resolveEffectiveFocus("review", "sast")
		if got != "sast" {
			t.Fatalf("expected public alias `sast`, got %q", got)
		}
	})

	t.Run("empty explicit falls back to resolver primary", func(t *testing.T) {
		got := resolveEffectiveFocus("review", "")
		if got == "" {
			t.Fatal("expected resolver-selected review mode")
		}
		// Primary for review is `independent-review`.
		if got != "independent-review" {
			t.Fatalf("expected `independent-review`, got %q", got)
		}
	})

	t.Run("unknown command falls back to empty", func(t *testing.T) {
		got := resolveEffectiveFocus("next", "")
		if got != "" {
			t.Fatalf("expected empty fallback, got %q", got)
		}
	})
}

func TestResolveEffectiveView_PrecedenceAndFallback(t *testing.T) {
	t.Run("explicit alias wins", func(t *testing.T) {
		got := resolveEffectiveView("status", "incident")
		if got != "incident" {
			t.Fatalf("expected public alias `incident`, got %q", got)
		}
	})

	t.Run("empty explicit falls back to resolver primary view", func(t *testing.T) {
		got := resolveEffectiveView("status", "")
		if got == "" {
			t.Fatal("expected resolver-selected status view")
		}
	})

	t.Run("unknown command falls back to empty", func(t *testing.T) {
		got := resolveEffectiveView("next", "")
		if got != "" {
			t.Fatalf("expected empty fallback, got %q", got)
		}
	})
}

func TestResolveEffectiveFocusHydrate_ExplicitAliasShortCircuits(t *testing.T) {
	// `sast` focus is backed by sast-orchestration.
	got := resolveEffectiveFocusHydrate("validate", "sast")
	if len(got) == 0 {
		t.Fatal("expected hydrate keys for sast focus")
	}
	for _, key := range got {
		if !strings.HasPrefix(key, "sast-orchestration/") {
			t.Fatalf("expected sast-orchestration/ prefix, got %q", key)
		}
	}
	if !slices.Contains(got, "sast-orchestration/codeql-ruleset-catalog.md") {
		t.Fatalf("expected sast-orchestration/codeql-ruleset-catalog.md in %v", got)
	}
}

func TestResolveEffectiveFocusHydrate_EmptyFallsBackToResolver(t *testing.T) {
	got := resolveEffectiveFocusHydrate("review", "")
	want := capability.Resolve(capability.DefaultRegistry(), capability.Signals{Command: "review"}).HydrateReferences
	if !equalStrings(got, want) {
		t.Fatalf("expected resolver hydrate fallback %v, got %v", want, got)
	}
}

func TestResolveEffectiveFocusHydrate_UnknownAliasReturnsNil(t *testing.T) {
	got := resolveEffectiveFocusHydrate("review", "does-not-exist")
	if got != nil {
		t.Fatalf("expected nil for unknown focus, got %v", got)
	}
}

func TestResolveEffectiveViewHydrate_IncidentShortCircuits(t *testing.T) {
	got := resolveEffectiveViewHydrate("status", "incident")
	if len(got) == 0 {
		t.Fatal("expected incident-response hydrate keys")
	}
	for _, key := range got {
		if !strings.HasPrefix(key, "incident-response/") {
			t.Fatalf("expected incident-response/ prefix, got %q", key)
		}
	}
	if !slices.Contains(got, "incident-response/incident-severity-matrix.md") {
		t.Fatalf("expected incident-severity-matrix key in %v", got)
	}
}

func TestResolveEffectiveViewHydrate_EmptyFallsBackToResolver(t *testing.T) {
	got := resolveEffectiveViewHydrate("status", "")
	want := capability.Resolve(capability.DefaultRegistry(), capability.Signals{Command: "status"}).HydrateReferences
	if !equalStrings(got, want) {
		t.Fatalf("expected resolver hydrate fallback %v, got %v", want, got)
	}
}

func TestResolveEffectiveViewHydrate_UnknownAliasReturnsNil(t *testing.T) {
	got := resolveEffectiveViewHydrate("status", "does-not-exist")
	if got != nil {
		t.Fatalf("expected nil for unknown view, got %v", got)
	}
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
