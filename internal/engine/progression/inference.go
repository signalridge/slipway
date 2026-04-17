package progression

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

type IntentClassification struct {
	GuardrailDomain string `json:"guardrail_domain"`
	NeedsDiscovery  bool   `json:"needs_discovery"`
	Complexity      string `json:"complexity"`
}

type IntentClassifier interface {
	Classify(ctx context.Context, inferenceText string) (IntentClassification, error)
}

var allowedGuardrailDomains = []string{
	"",
	model.GuardrailDomainAuthAuthZ,
	model.GuardrailDomainSecurityCredentials,
	model.GuardrailDomainPrivacyPII,
	model.GuardrailDomainFinancialFlows,
	model.GuardrailDomainSchemaDataMigration,
	model.GuardrailDomainIrreversibleOps,
	model.GuardrailDomainExternalAPIContracts,
}

var allowedComplexityLevels = []string{
	"trivial",
	"simple",
	"complex",
	"critical",
}

func GuardrailDomainEnumValues() []string {
	return append([]string(nil), allowedGuardrailDomains...)
}

func ComplexityEnumValues() []string {
	return append([]string(nil), allowedComplexityLevels...)
}

// InferenceResult holds a resolved intent classification with degradation
// metadata. When Degraded is true, the classification fell back to
// SafeDegradeClassification and DegradeReason explains why.
type InferenceResult struct {
	Classification IntentClassification
	Degraded       bool
	DegradeReason  string
}

func ResolveIntentClassification(
	ctx context.Context,
	inferenceText string,
	classifier IntentClassifier,
) InferenceResult {
	if strings.TrimSpace(inferenceText) == "" {
		return InferenceResult{SafeDegradeClassification(), true, "empty_inference_text"}
	}
	if classifier == nil {
		return InferenceResult{SafeDegradeClassification(), true, "no_classifier"}
	}
	classification, err := classifier.Classify(ctx, inferenceText)
	if err != nil {
		return InferenceResult{SafeDegradeClassification(), true, fmt.Sprintf("classifier_error: %s", err)}
	}
	if err := validateClassification(classification); err != nil {
		return InferenceResult{SafeDegradeClassification(), true, fmt.Sprintf("validation_failed: %s", err)}
	}
	return InferenceResult{classification, false, ""}
}

func validateClassification(c IntentClassification) error {
	if !slices.Contains(allowedGuardrailDomains, strings.TrimSpace(c.GuardrailDomain)) {
		return fmt.Errorf("invalid guardrail_domain %q", c.GuardrailDomain)
	}
	if !slices.Contains(allowedComplexityLevels, strings.TrimSpace(c.Complexity)) {
		return fmt.Errorf("invalid complexity %q", c.Complexity)
	}
	if strings.TrimSpace(c.GuardrailDomain) != "" {
		if !c.NeedsDiscovery {
			return fmt.Errorf("guardrail_domain %q requires needs_discovery=true", c.GuardrailDomain)
		}
		if c.Complexity == "trivial" || c.Complexity == "simple" {
			return fmt.Errorf("guardrail_domain %q requires complexity >= complex, got %q", c.GuardrailDomain, c.Complexity)
		}
	}
	return nil
}

func SafeDegradeClassification() IntentClassification {
	return IntentClassification{
		GuardrailDomain: "",
		NeedsDiscovery:  true,
		Complexity:      "complex",
	}
}
