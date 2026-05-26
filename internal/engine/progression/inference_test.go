package progression

import (
	"context"
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
