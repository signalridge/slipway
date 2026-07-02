package capability

import (
	"testing"

	"github.com/signalridge/slipway/internal/tmpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecKittyScopeContractAbsorptionAppearsInReviewGuidance(t *testing.T) {
	t.Parallel()

	body, err := tmpl.Render("skills/spec-compliance-review/HOST_SKILL.md.tmpl", nil)
	require.NoError(t, err)

	assert.Contains(t, body, "Scope Contract Evidence")
	assert.Contains(t, body, "scope_contract.status=pass")
	assert.Contains(t, body, "scope_contract:fail:<reason>")
}

func TestSpecTraceReportSchemaIncludesScopeContractEvidence(t *testing.T) {
	t.Parallel()

	body, err := tmpl.Content("skills/spec-trace/CATALOG_SKILL.md")
	require.NoError(t, err)

	assert.Contains(t, body, "scope_contract:")
	assert.Contains(t, body, "scope_contract:pass or scope_contract:fail:<reason>")
}

func TestShipVerificationRequiresScopeContractStatement(t *testing.T) {
	t.Parallel()

	body, err := tmpl.Render("skills/ship-verification/HOST_SKILL.md.tmpl", nil)
	require.NoError(t, err)

	assert.Contains(t, body, "Scope Contract: pass|fail|not_applicable")
	assert.Contains(t, body, "scope_contract:pass")
	assert.Contains(t, body, "Scope Contract evidence exists and reports fail")
}
