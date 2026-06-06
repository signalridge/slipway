package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/capability"
)

func TestValidateFocus_EmptyAllowed(t *testing.T) {
	t.Parallel()

	for _, cmd := range []string{"review", "validate", "repair"} {
		if err := validateFocus(cmd, ""); err != nil {
			t.Fatalf("%s: empty focus should be allowed: %v", cmd, err)
		}
	}
}

func TestValidateFocus_AcceptsPublicAliases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		command string
		alias   string
	}{
		{"review", "sast"},
		{"review", "calibration"},
		{"validate", "sast"},
		{"validate", "property"},
		{"validate", "mutation"},
	}
	for _, tc := range cases {
		if err := validateFocus(tc.command, tc.alias); err != nil {
			t.Fatalf("%s: focus %q should be accepted: %v", tc.command, tc.alias, err)
		}
	}
}

func TestValidateFocus_RejectsPrimaryName(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	err := validateFocus("review", "second-opinion")
	if err == nil {
		t.Fatal("expected rejection for legacy `second-opinion`")
	}
	cliErr := asCLIError(err)
	if cliErr == nil || cliErr.ErrorCode != "unknown_route_mode" {
		t.Fatalf("expected unknown_route_mode, got %v", err)
	}
}

func TestValidateFocus_RejectsSastForRepairAndRedirects(t *testing.T) {
	t.Parallel()

	// repair no longer advertises SAST (issue #88): the false-promise focus was
	// removed, and selecting it must redirect to the surfaces that actually
	// hydrate SAST guidance rather than silently no-op.
	err := validateFocus("repair", "sast")
	if err == nil {
		t.Fatal("expected rejection for `repair --focus sast`")
	}
	cliErr := asCLIError(err)
	if cliErr == nil || cliErr.ErrorCode != "unknown_route_mode" {
		t.Fatalf("expected unknown_route_mode, got %v", err)
	}
	if !strings.Contains(cliErr.Remediation, "slipway review --focus sast") ||
		!strings.Contains(cliErr.Remediation, "slipway validate --focus sast") {
		t.Fatalf("expected redirect to review/validate sast focus, got remediation %q", cliErr.Remediation)
	}
}

func TestValidateFocus_RejectsUnknownWithRemediation(t *testing.T) {
	t.Parallel()

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

func TestResolveEffectiveFocus_PrecedenceAndFallback(t *testing.T) {
	t.Run("explicit alias wins and returns public name", func(t *testing.T) {
		t.Parallel()

		got := resolveEffectiveFocus("review", "sast")
		if got != "sast" {
			t.Fatalf("expected public alias `sast`, got %q", got)
		}
	})

	t.Run("empty explicit falls back to resolver primary", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

		got := resolveEffectiveFocus("next", "")
		if got != "" {
			t.Fatalf("expected empty fallback, got %q", got)
		}
	})
}

func TestResolveEffectiveFocusHydrate_ExplicitAliasShortCircuits(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	got := resolveEffectiveFocusHydrate("review", "")
	want := capability.Resolve(capability.DefaultRegistry(), capability.Signals{Command: "review"}).HydrateReferences
	if !equalStrings(got, want) {
		t.Fatalf("expected resolver hydrate fallback %v, got %v", want, got)
	}
}

func TestResolveEffectiveFocusHydrate_UnknownAliasReturnsNil(t *testing.T) {
	t.Parallel()

	got := resolveEffectiveFocusHydrate("review", "does-not-exist")
	if got != nil {
		t.Fatalf("expected nil for unknown focus, got %v", got)
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
