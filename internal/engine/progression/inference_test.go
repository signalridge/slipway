package progression

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestInferDiscovery(t *testing.T) {
	t.Parallel()

	assert.False(t, InferDiscovery("fix login timeout", ""))
	assert.True(t, InferDiscovery("refactor auth middleware timeout strategy", model.GuardrailDomainAuthAuthZ))
	assert.True(t, InferDiscovery("not sure how to re-architect this service", ""))
}

func TestInferComplexity(t *testing.T) {
	t.Parallel()

	// Priority 1: guardrail domain → minimum "complex"
	assert.Equal(t, "complex", InferComplexity("add new feature", "auth_authz"))
	// Priority 1: guardrail domain + critical keyword → "critical"
	assert.Equal(t, "critical", InferComplexity("add auth login flow", "auth_authz"))
	assert.Equal(t, "critical", InferComplexity("update payment processing", "financial"))

	// Priority 2: explicit trivial signals (no guardrail domain)
	assert.Equal(t, "trivial", InferComplexity("trivial typo fix in readme", ""))
	assert.Equal(t, "trivial", InferComplexity("quick fix for button color", ""))
	assert.Equal(t, "trivial", InferComplexity("hello world example", ""))

	// Priority 3: complex keyword hints
	assert.Equal(t, "complex", InferComplexity("architectural overhaul of the routing subsystem", ""))
	assert.Equal(t, "complex", InferComplexity("multi-service integration for notifications", ""))
	assert.Equal(t, "complex", InferComplexity("cross-cutting concern: add tracing", ""))

	// Priority 4: default → "simple"
	assert.Equal(t, "simple", InferComplexity("add button to dashboard", ""))
	assert.Equal(t, "simple", InferComplexity("fix login timeout", ""))

	// Guardrail domain overrides trivial signal — "auth" triggers critical keyword within guardrail
	assert.Equal(t, "critical", InferComplexity("trivial auth change", "auth_authz"))
	// Guardrail domain without critical keyword → "complex" (not lowered by "trivial")
	assert.Equal(t, "complex", InferComplexity("trivial config change", "security_credentials"))
}
