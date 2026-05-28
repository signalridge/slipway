package model

// ChangeSetup is the resolved output from resolveChangeSetup().
// It provides the direct model fields needed to create a governed change.
type ChangeSetup struct {
	GuardrailDomain string
	NeedsDiscovery  bool
	ArtifactSchema  ArtifactSchemaName
	InitialSubStep  PlanSubStep
	ComplexityLevel string
}

// WorktreeValidationResult holds the result of shared worktree validation.
type WorktreeValidationResult struct {
	NormalizedPath   string
	NormalizedBranch string
	Blockers         []ReasonCode
}

// PlanningValidationResult holds the result of planning readiness validation.
type PlanningValidationResult struct {
	Blockers    []ReasonCode
	Diagnostics []string
}
