package review

import (
	"path/filepath"
	"strings"
)

type ReviewLayer string

const (
	LayerR0  ReviewLayer = "R0"
	LayerR1  ReviewLayer = "R1"
	LayerR2  ReviewLayer = "R2"
	LayerR3  ReviewLayer = "R3"
	LayerIR1 ReviewLayer = "IR1"
	LayerIR2 ReviewLayer = "IR2"
	LayerIR3 ReviewLayer = "IR3"
)

// RequiredArtifactLayers returns the mandatory artifact review layers.
func RequiredArtifactLayers(guardrailDomain, artifactName string) []ReviewLayer {
	if isManifestArtifact(artifactName) {
		return []ReviewLayer{LayerR0}
	}
	if strings.TrimSpace(guardrailDomain) != "" {
		return []ReviewLayer{LayerR0, LayerR3}
	}
	return []ReviewLayer{LayerR0}
}

// RequiredImplementationLayers returns the mandatory implementation review layers.
func RequiredImplementationLayers(guardrailDomain string) []ReviewLayer {
	if strings.TrimSpace(guardrailDomain) != "" {
		return []ReviewLayer{LayerIR1, LayerIR3}
	}
	return []ReviewLayer{LayerIR1}
}

// OptionalLayers returns optional MVP depth layers that must never be implicit blockers.
func OptionalLayers() []ReviewLayer {
	return []ReviewLayer{LayerR1, LayerR2, LayerIR2}
}

func isManifestArtifact(name string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	return base == "change.yaml"
}
