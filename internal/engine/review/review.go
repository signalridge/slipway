package review

import (
	"path/filepath"
	"strings"

	"github.com/signalridge/speclane/internal/model"
)

type Layer string

const (
	LayerR0  Layer = "R0"
	LayerR1  Layer = "R1"
	LayerR2  Layer = "R2"
	LayerR3  Layer = "R3"
	LayerIR1 Layer = "IR1"
	LayerIR2 Layer = "IR2"
	LayerIR3 Layer = "IR3"
)

type MatrixRow struct {
	ScopeCondition string  `json:"scope_condition"`
	ReviewedUnit   string  `json:"reviewed_unit"`
	RequiredLayers []Layer `json:"required_layers"`
	Notes          string  `json:"notes"`
}

// RequiredArtifactLayers returns the mandatory artifact review layers.
func RequiredArtifactLayers(level model.Level, guardrailDomain, artifactName string) []Layer {
	if !isGoverned(level) {
		return nil
	}
	if isManifestArtifact(artifactName) {
		return []Layer{LayerR0}
	}
	if strings.TrimSpace(guardrailDomain) != "" {
		return []Layer{LayerR0, LayerR3}
	}
	return []Layer{LayerR0}
}

// RequiredImplementationLayers returns the mandatory implementation review layers.
func RequiredImplementationLayers(level model.Level, guardrailDomain string) []Layer {
	if !isGoverned(level) {
		return nil
	}
	if strings.TrimSpace(guardrailDomain) != "" {
		return []Layer{LayerIR1, LayerIR3}
	}
	return []Layer{LayerIR1}
}

// OptionalLayers returns optional MVP depth layers that must never be implicit blockers.
func OptionalLayers(level model.Level) []Layer {
	if !isGoverned(level) {
		return nil
	}
	return []Layer{LayerR1, LayerR2, LayerIR2}
}

// ConsolidatedMatrixReference exports the MVP review matrix for command/runtime consumers.
func ConsolidatedMatrixReference() []MatrixRow {
	return []MatrixRow{
		{
			ScopeCondition: "governed_baseline_no_guardrail",
			ReviewedUnit:   "changed_artifacts",
			RequiredLayers: []Layer{LayerR0},
			Notes:          "R1/R2/R3 optional only when explicitly requested",
		},
		{
			ScopeCondition: "governed_baseline_no_guardrail",
			ReviewedUnit:   "implementation_deltas",
			RequiredLayers: []Layer{LayerIR1},
			Notes:          "IR2 optional only when explicitly requested",
		},
		{
			ScopeCondition: "guardrail_sensitive_governed",
			ReviewedUnit:   "changed_artifacts",
			RequiredLayers: []Layer{LayerR0, LayerR3},
			Notes:          "guardrail precedence requires R3",
		},
		{
			ScopeCondition: "guardrail_sensitive_governed",
			ReviewedUnit:   "implementation_deltas",
			RequiredLayers: []Layer{LayerIR1, LayerIR3},
			Notes:          "guardrail precedence requires IR3",
		},
		{
			ScopeCondition: "any_governed_scope",
			ReviewedUnit:   "change.yaml",
			RequiredLayers: []Layer{LayerR0},
			Notes:          "manifest remains R0-only in MVP",
		},
		{
			ScopeCondition: "l3_discovery_scope_stage",
			ReviewedUnit:   "explore.md",
			RequiredLayers: nil,
			Notes:          "governed by S2/S3 and G_scope, outside ship-stage matrix",
		},
	}
}

func isGoverned(level model.Level) bool {
	return level == model.LevelL2 || level == model.LevelL3
}

func isManifestArtifact(name string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	return base == "change.yaml"
}
