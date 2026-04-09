package progression

import (
	"regexp"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

var schemaWordPattern = regexp.MustCompile(`\bschema\b`)

var discoverySignals = []string{
	"not sure",
	"investigate",
	"explore",
	"new project",
	"from scratch",
	"major refactor",
	"rewrite",
	"re-architect",
}

// InferGuardrailDomain infers a guardrail domain from description text using
// whole-word and multi-word phrase matching. When detailed is true, additional
// multi-word patterns are checked.
func InferGuardrailDomain(description string, detailed bool) string {
	text := strings.ToLower(description)
	switch {
	case containsWord(text, "auth"), containsWord(text, "oauth"), containsWord(text, "rbac"),
		containsPhrase(text, "authorization"), containsPhrase(text, "authentication"):
		return model.GuardrailDomainAuthAuthZ
	case containsWord(text, "credential"), containsWord(text, "secret"),
		containsPhrase(text, "api token"), containsPhrase(text, "access token"),
		containsPhrase(text, "api key"):
		return model.GuardrailDomainSecurityCredentials
	case containsWord(text, "pii"), containsWord(text, "privacy"):
		return model.GuardrailDomainPrivacyPII
	case containsWord(text, "payment"), containsWord(text, "billing"), containsWord(text, "financial"):
		return model.GuardrailDomainFinancialFlows
	case schemaWordPattern.MatchString(text),
		containsPhrase(text, "data migration"),
		containsPhrase(text, "database migration"):
		return model.GuardrailDomainSchemaDataMigration
	case detailed && (containsPhrase(text, "table migration") ||
		containsPhrase(text, "column migration") ||
		containsPhrase(text, "drop table") ||
		containsPhrase(text, "drop database")):
		return model.GuardrailDomainSchemaDataMigration
	case containsPhrase(text, "hard delete"),
		containsPhrase(text, "permanently delete"),
		containsPhrase(text, "irreversible operation"),
		containsWord(text, "delete"),
		containsWord(text, "irreversible"):
		return model.GuardrailDomainIrreversibleOps
	case containsPhrase(text, "api contract"), containsPhrase(text, "public api"):
		return model.GuardrailDomainExternalAPIContracts
	case detailed && (containsPhrase(text, "webhook contract") || containsPhrase(text, "external api contract")):
		return model.GuardrailDomainExternalAPIContracts
	default:
		return ""
	}
}

// InferDiscovery determines whether a change needs discovery based on its
// description and guardrail domain.
func InferDiscovery(description string, guardrailDomain string) bool {
	if strings.TrimSpace(guardrailDomain) != "" {
		return true
	}
	lowered := strings.ToLower(description)
	for _, signal := range discoverySignals {
		if strings.Contains(lowered, signal) {
			return true
		}
	}
	return false
}

// InferComplexity infers a complexity level from the description and guardrail domains.
// Priority: guardrail domain > explicit trivial signal > keyword hints > default "simple".
func InferComplexity(description string, guardrailDomain string) string {
	text := strings.ToLower(description)

	// Priority 1: guardrail domain present → minimum "complex"
	if strings.TrimSpace(guardrailDomain) != "" {
		// Critical keywords within guardrail domain elevate to "critical"
		for _, kw := range []string{"auth", "payment", "migration", "credential", "secret"} {
			if strings.Contains(text, kw) {
				return "critical"
			}
		}
		return "complex"
	}

	// Priority 2: explicit trivial signals
	for _, signal := range []string{"trivial", "poc", "hello world", "quick fix", "typo", "one-liner"} {
		if strings.Contains(text, signal) {
			return "trivial"
		}
	}

	// Priority 3: complex keyword hints (only elevate, never lower)
	for _, kw := range []string{"subsystem", "integration", "architectural", "multi-service", "cross-cutting", "major refactor"} {
		if strings.Contains(text, kw) {
			return "complex"
		}
	}

	// Priority 4: default
	return "simple"
}

func containsWord(text, word string) bool {
	idx := strings.Index(text, word)
	if idx < 0 {
		return false
	}
	if idx > 0 {
		c := text[idx-1]
		if isWordChar(c) {
			return false
		}
	}
	end := idx + len(word)
	if end < len(text) {
		c := text[end]
		if isWordChar(c) {
			return false
		}
	}
	return true
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func containsPhrase(text, phrase string) bool {
	return strings.Contains(text, phrase)
}
