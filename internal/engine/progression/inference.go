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

// ValidGuardrailDomains returns the allowed guardrail_domain tokens, including
// the empty string which denotes "no sensitive domain". The slice is a copy, so
// callers may safely mutate it. It is the single source of truth shared by
// validation, CLI help, and error remediation so the accepted set never drifts.
func ValidGuardrailDomains() []string {
	return append([]string(nil), allowedGuardrailDomains...)
}

// ValidComplexityLevels returns the allowed complexity tokens in increasing
// order of risk. The slice is a copy.
func ValidComplexityLevels() []string {
	return append([]string(nil), allowedComplexityLevels...)
}

// ClassificationError describes why an intent classification is invalid. It
// carries the offending field, the allowed values, and a best-effort
// nearest-match suggestion so explicit callers can self-correct. Enum
// violations populate Allowed/Suggest; cross-field rule violations populate
// Reason instead. It is returned by ValidateClassification.
type ClassificationError struct {
	Field   string   // "guardrail_domain", "complexity", "needs_discovery"
	Value   string   // offending value (enum violations only)
	Allowed []string // allowed values for the field (enum violations only)
	Suggest string   // nearest allowed value, empty when nothing is close
	Reason  string   // human-readable explanation (cross-field violations)
}

func (e *ClassificationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Reason != "" {
		return e.Reason
	}
	msg := fmt.Sprintf("invalid %s %q", e.Field, e.Value)
	if e.Suggest != "" {
		msg += fmt.Sprintf(" (did you mean %q?)", e.Suggest)
	}
	return msg
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

// ValidateClassification reports whether a fully-resolved classification is
// acceptable. A nil error means valid. A non-nil error is always a
// *ClassificationError, so callers on explicit-classification surfaces can
// surface actionable remediation (valid set + suggestion) instead of silently
// safe-degrading.
func ValidateClassification(c IntentClassification) error {
	return validateClassification(c)
}

func validateClassification(c IntentClassification) error {
	domain := strings.TrimSpace(c.GuardrailDomain)
	if !slices.Contains(allowedGuardrailDomains, domain) {
		return &ClassificationError{
			Field:   "guardrail_domain",
			Value:   c.GuardrailDomain,
			Allowed: ValidGuardrailDomains(),
			Suggest: nearestAllowed(domain, allowedGuardrailDomains),
		}
	}
	complexity := strings.TrimSpace(c.Complexity)
	if !slices.Contains(allowedComplexityLevels, complexity) {
		return &ClassificationError{
			Field:   "complexity",
			Value:   c.Complexity,
			Allowed: ValidComplexityLevels(),
			Suggest: nearestAllowed(complexity, allowedComplexityLevels),
		}
	}
	if domain != "" {
		if !c.NeedsDiscovery {
			return &ClassificationError{
				Field:  "needs_discovery",
				Reason: fmt.Sprintf("guardrail_domain %q requires needs_discovery=true", domain),
			}
		}
		if complexity == "trivial" || complexity == "simple" {
			return &ClassificationError{
				Field:  "complexity",
				Reason: fmt.Sprintf("guardrail_domain %q requires complexity >= complex, got %q", domain, complexity),
			}
		}
	}
	return nil
}

// nearestAllowed returns the allowed value most similar to value, or "" when
// nothing is close enough to suggest. It combines shared-token overlap (so a
// concept word like "data-integrity" maps to "schema_data_migration") with
// Levenshtein edit distance (so a typo like "auth_autz" maps to "auth_authz").
// The empty candidate is never suggested.
func nearestAllowed(value string, allowed []string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	valueTokens := tokenize(value)
	best := ""
	bestShared := 0
	bestDist := 1 << 30
	for _, cand := range allowed {
		if cand == "" {
			continue
		}
		shared := sharedTokenCount(valueTokens, tokenize(cand))
		dist := levenshtein(value, strings.ToLower(cand))
		if shared > bestShared || (shared == bestShared && dist < bestDist) {
			best, bestShared, bestDist = cand, shared, dist
		}
	}
	if bestShared >= 1 || bestDist <= 3 {
		return best
	}
	return ""
}

// tokenize splits s into a set of lowercase alphanumeric tokens, treating any
// other character (underscore, hyphen, space) as a separator.
func tokenize(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, tok := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		out[tok] = struct{}{}
	}
	return out
}

func sharedTokenCount(a, b map[string]struct{}) int {
	n := 0
	for tok := range a {
		if _, ok := b[tok]; ok {
			n++
		}
	}
	return n
}

// levenshtein returns the edit distance between a and b.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

func SafeDegradeClassification() IntentClassification {
	return IntentClassification{
		GuardrailDomain: "",
		NeedsDiscovery:  true,
		Complexity:      "complex",
	}
}
