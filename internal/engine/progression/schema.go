package progression

import "github.com/signalridge/slipway/internal/model"

// ResolveFrozenArtifactSchema returns the effective artifact schema that should
// be frozen on a change once routing is known.
func ResolveFrozenArtifactSchema(
	current model.ArtifactSchemaName,
	configDefault model.ArtifactSchemaName,
	needsDiscovery bool,
) model.ArtifactSchemaName {
	schema := current
	if schema == "" {
		schema = configDefault
	}
	if schema == "" {
		schema = model.ArtifactSchemaExpanded
	}
	if needsDiscovery && schema == model.ArtifactSchemaCore {
		return model.ArtifactSchemaExpanded
	}
	return schema
}
