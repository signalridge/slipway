package review

import (
	"testing"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestRequiredLayersBaselineGoverned(t *testing.T) {
	assert.Equal(
		t,
		[]Layer{LayerR0},
		RequiredArtifactLayers(model.LevelL2, "", "design.md"),
	)
	assert.Equal(
		t,
		[]Layer{LayerIR1},
		RequiredImplementationLayers(model.LevelL2, ""),
	)
}

func TestRequiredLayersGuardrailPrecedence(t *testing.T) {
	assert.Equal(
		t,
		[]Layer{LayerR0, LayerR3},
		RequiredArtifactLayers(model.LevelL3, "security_credentials", "design.md"),
	)
	assert.Equal(
		t,
		[]Layer{LayerIR1, LayerIR3},
		RequiredImplementationLayers(model.LevelL3, "security_credentials"),
	)
}

func TestManifestReviewAlwaysR0Only(t *testing.T) {
	assert.Equal(
		t,
		[]Layer{LayerR0},
		RequiredArtifactLayers(model.LevelL3, "privacy_pii", "aircraft/changes/x/change.yaml"),
	)
}

func TestL1HasNoMandatoryGovernanceLayers(t *testing.T) {
	assert.Nil(t, RequiredArtifactLayers(model.LevelL1, "auth_authz", "design.md"))
	assert.Nil(t, RequiredImplementationLayers(model.LevelL1, "auth_authz"))
	assert.Nil(t, OptionalLayers(model.LevelL1))
}

func TestConsolidatedMatrixIncludesSpecialCases(t *testing.T) {
	rows := ConsolidatedMatrixReference()
	assert.NotEmpty(t, rows)

	manifestFound := false
	exploreFound := false
	for _, row := range rows {
		if row.ReviewedUnit == "change.yaml" {
			manifestFound = true
			assert.Equal(t, []Layer{LayerR0}, row.RequiredLayers)
		}
		if row.ReviewedUnit == "explore.md" {
			exploreFound = true
		}
	}
	assert.True(t, manifestFound)
	assert.True(t, exploreFound)
}
