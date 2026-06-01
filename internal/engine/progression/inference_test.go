package progression

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type stubIntentClassifier struct {
	classification IntentClassification
	err            error
}

func (s stubIntentClassifier) Classify(_ context.Context, _ string) (IntentClassification, error) {
	if s.err != nil {
		return IntentClassification{}, s.err
	}
	return s.classification, nil
}

func TestValidateClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		classification IntentClassification
		wantErr        bool
	}{
		{
			name: "accepts low risk simple classification",
			classification: IntentClassification{
				GuardrailDomain: "",
				NeedsDiscovery:  false,
				Complexity:      "simple",
			},
		},
		{
			name: "rejects invalid guardrail domain",
			classification: IntentClassification{
				GuardrailDomain: "unknown_domain",
				NeedsDiscovery:  true,
				Complexity:      "complex",
			},
			wantErr: true,
		},
		{
			name: "rejects invalid complexity",
			classification: IntentClassification{
				GuardrailDomain: "",
				NeedsDiscovery:  false,
				Complexity:      "easy",
			},
			wantErr: true,
		},
		{
			name: "rejects guardrail without discovery",
			classification: IntentClassification{
				GuardrailDomain: "auth_authz",
				NeedsDiscovery:  false,
				Complexity:      "complex",
			},
			wantErr: true,
		},
		{
			name: "rejects guardrail with simple complexity",
			classification: IntentClassification{
				GuardrailDomain: "auth_authz",
				NeedsDiscovery:  true,
				Complexity:      "simple",
			},
			wantErr: true,
		},
		{
			name: "accepts guardrail discovery classification",
			classification: IntentClassification{
				GuardrailDomain: "auth_authz",
				NeedsDiscovery:  true,
				Complexity:      "critical",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateClassification(tc.classification)
			if tc.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidateClassificationReturnsTypedEnumError(t *testing.T) {
	t.Parallel()

	err := ValidateClassification(IntentClassification{
		GuardrailDomain: "data-integrity",
		NeedsDiscovery:  true,
		Complexity:      "critical",
	})

	var ce *ClassificationError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ClassificationError, got %T (%v)", err, err)
	}
	if ce.Field != "guardrail_domain" {
		t.Fatalf("expected field guardrail_domain, got %q", ce.Field)
	}
	if ce.Suggest != "schema_data_migration" {
		t.Fatalf("expected suggestion schema_data_migration, got %q", ce.Suggest)
	}
	if !slices.Contains(ce.Allowed, "schema_data_migration") || !slices.Contains(ce.Allowed, "") {
		t.Fatalf("expected allowed set to carry the valid tokens, got %v", ce.Allowed)
	}
}

func TestValidateClassificationCrossFieldErrorHasReasonNotAllowed(t *testing.T) {
	t.Parallel()

	err := ValidateClassification(IntentClassification{
		GuardrailDomain: "auth_authz",
		NeedsDiscovery:  false,
		Complexity:      "complex",
	})

	var ce *ClassificationError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ClassificationError, got %T (%v)", err, err)
	}
	if ce.Field != "needs_discovery" {
		t.Fatalf("expected field needs_discovery, got %q", ce.Field)
	}
	if len(ce.Allowed) != 0 {
		t.Fatalf("cross-field error should not carry an allowed set, got %v", ce.Allowed)
	}
	if ce.Reason == "" {
		t.Fatal("expected a human-readable reason on the cross-field error")
	}
}

func TestNearestAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		allowed []string
		want    string
	}{
		{
			name:    "concept word maps via shared token",
			value:   "data-integrity",
			allowed: allowedGuardrailDomains,
			want:    "schema_data_migration",
		},
		{
			name:    "typo maps via edit distance",
			value:   "auth_autz",
			allowed: allowedGuardrailDomains,
			want:    "auth_authz",
		},
		{
			name:    "complexity typo",
			value:   "critcal",
			allowed: allowedComplexityLevels,
			want:    "critical",
		},
		{
			name:    "no close match returns empty",
			value:   "zzzzzzzzzz",
			allowed: allowedGuardrailDomains,
			want:    "",
		},
		{
			name:    "never suggests the empty token",
			value:   "x",
			allowed: allowedGuardrailDomains,
			want:    "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := nearestAllowed(tc.value, tc.allowed); got != tc.want {
				t.Fatalf("nearestAllowed(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestValidValueAccessorsReturnIndependentCopies(t *testing.T) {
	t.Parallel()

	domains := ValidGuardrailDomains()
	if !slices.Equal(domains, allowedGuardrailDomains) {
		t.Fatalf("ValidGuardrailDomains mismatch: %v", domains)
	}
	domains[0] = "mutated"
	if allowedGuardrailDomains[0] == "mutated" {
		t.Fatal("ValidGuardrailDomains must return a copy, not the backing slice")
	}

	levels := ValidComplexityLevels()
	if !slices.Equal(levels, allowedComplexityLevels) {
		t.Fatalf("ValidComplexityLevels mismatch: %v", levels)
	}
}

func TestResolveIntentClassificationDegradesWithoutClassifier(t *testing.T) {
	t.Parallel()

	result := ResolveIntentClassification(context.Background(), "fix login timeout", nil)
	if !result.Degraded {
		t.Fatal("expected degraded result without classifier")
	}
	if result.DegradeReason != "no_classifier" {
		t.Fatalf("expected no_classifier degrade reason, got %q", result.DegradeReason)
	}
	if result.Classification != SafeDegradeClassification() {
		t.Fatalf("expected safe degrade classification, got %#v", result.Classification)
	}
}

func TestResolveIntentClassificationDegradesOnValidationFailure(t *testing.T) {
	t.Parallel()

	result := ResolveIntentClassification(context.Background(), "rotate auth keys", stubIntentClassifier{
		classification: IntentClassification{
			GuardrailDomain: "auth_authz",
			NeedsDiscovery:  false,
			Complexity:      "simple",
		},
	})
	if !result.Degraded {
		t.Fatal("expected degraded result for invalid classifier output")
	}
	if result.DegradeReason == "" {
		t.Fatal("expected degrade reason to be populated")
	}
	if result.Classification != SafeDegradeClassification() {
		t.Fatalf("expected safe degrade classification, got %#v", result.Classification)
	}
}
