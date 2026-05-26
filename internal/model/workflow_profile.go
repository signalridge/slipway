package model

// WorkflowProfile classifies the shape of a governed change. It does not
// replace presets; presets tune strictness, while profile tunes which
// workflow-specific checks are relevant.
type WorkflowProfile string

const (
	WorkflowProfileCode     WorkflowProfile = "code"
	WorkflowProfileDocs     WorkflowProfile = "docs"
	WorkflowProfileResearch WorkflowProfile = "research"
	WorkflowProfileConfig   WorkflowProfile = "config"
	WorkflowProfileMeta     WorkflowProfile = "meta"
)

func (p WorkflowProfile) IsValid() bool {
	switch p {
	case "", WorkflowProfileCode, WorkflowProfileDocs, WorkflowProfileResearch, WorkflowProfileConfig, WorkflowProfileMeta:
		return true
	default:
		return false
	}
}

func (p WorkflowProfile) Effective() WorkflowProfile {
	if p == "" {
		return WorkflowProfileCode
	}
	return p
}

func (p WorkflowProfile) RequiresCodeQualityReview() bool {
	switch p.Effective() {
	case WorkflowProfileDocs, WorkflowProfileResearch:
		return false
	default:
		return true
	}
}

func DefaultArtifactSchemaForWorkflowProfile(profile WorkflowProfile, needsDiscovery bool, inferred ArtifactSchemaName) ArtifactSchemaName {
	if needsDiscovery {
		return ArtifactSchemaExpanded
	}
	switch profile.Effective() {
	case WorkflowProfileDocs:
		return ArtifactSchemaCore
	case WorkflowProfileResearch, WorkflowProfileConfig, WorkflowProfileMeta:
		return ArtifactSchemaExpanded
	default:
		if inferred != "" {
			return inferred
		}
		return ArtifactSchemaCore
	}
}
