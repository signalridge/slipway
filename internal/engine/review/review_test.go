package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequiredLayersBaselineGoverned(t *testing.T) {
	t.Parallel()
	assert.Equal(
		t,
		[]ReviewLayer{LayerR0},
		RequiredArtifactLayers("", "decision.md"),
	)
	assert.Equal(
		t,
		[]ReviewLayer{LayerIR1},
		RequiredImplementationLayers(""),
	)
}

func TestRequiredLayersGuardrailPrecedence(t *testing.T) {
	t.Parallel()
	assert.Equal(
		t,
		[]ReviewLayer{LayerR0, LayerR3},
		RequiredArtifactLayers("security_credentials", "decision.md"),
	)
	assert.Equal(
		t,
		[]ReviewLayer{LayerIR1, LayerIR3},
		RequiredImplementationLayers("security_credentials"),
	)
}

func TestManifestReviewAlwaysR0Only(t *testing.T) {
	t.Parallel()
	assert.Equal(
		t,
		[]ReviewLayer{LayerR0},
		RequiredArtifactLayers("privacy_pii", "artifacts/changes/x/change.yaml"),
	)
}

func TestExecutableChangesUseSameReviewLayers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []ReviewLayer{LayerR0, LayerR3}, RequiredArtifactLayers("auth_authz", "decision.md"))
	assert.Equal(t, []ReviewLayer{LayerIR1, LayerIR3}, RequiredImplementationLayers("auth_authz"))
	assert.Equal(t, []ReviewLayer{LayerR1, LayerR2, LayerIR2}, OptionalLayers())
}
