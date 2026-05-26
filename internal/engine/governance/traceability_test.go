package governance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// resolveTestArtifact returns the path where the default resolver will look for requirements.md.
func resolveTestArtifact(bundleDir, slug string) string {
	return artifact.ResolveArtifactPath(bundleDir, slug, "requirements.md")
}

func traceabilityGapIDs(gaps []model.TraceabilityGap) []string {
	ids := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		ids = append(ids, gap.ID)
	}
	return ids
}

func TestTraceabilityCoherentBundle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "test-change"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
## Success Signals
INT-001: User can log in
## Open Questions
(none)
`)

	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
## ADDED Requirements
### Requirement: Login
REQ-001: The system must authenticate users via OAuth2. Traces to INT-001.
`)

	writeFile(t, filepath.Join(dir, "decision.md"), `# Decision
## Selected Approach
DEC-001: Use OAuth2 PKCE flow, implements REQ-001.
## Interfaces and Data Flow
## Rollout and Rollback
`)

	writeFile(t, filepath.Join(dir, "tasks.md"), `# Tasks
- [ ] Implement OAuth2 handler
  covers: [REQ-001]
`)

	writeFile(t, filepath.Join(dir, "assurance.md"), `# Assurance
## Requirement Coverage
REQ-001: verified via integration tests
## Verification Verdict
pass
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusOK, result.Status)
	assert.Empty(t, result.Gaps)
	assert.Contains(t, result.Links, model.TraceabilityLink{
		FromID:   "DEC-001",
		FromType: "decision",
		ToID:     "REQ-001",
		ToType:   "requirement",
	})
	assert.Contains(t, result.Message, "decisions")
	assert.Contains(t, result.Message, "assurance")
}

func TestTraceabilityStructuredDeltaSupportSections(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "structured-delta"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Original intent
`)
	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
## ADDED Requirements
### Requirement: A
REQ-001: Must do X. Traces to INT-001.
## NON-GOALS
- Do not change auth policy.
## DECISIONS
- DEC-001: Keep the current API shape for REQ-001.
## ROLLBACK
- Revert the config change.
`)

	result := EvaluateTraceability(TraceabilityInput{BundleDir: dir, Slug: slug})
	for _, gap := range result.Gaps {
		assert.NotContains(t, gap.ID, "requirements-delta-non-goals")
		assert.NotContains(t, gap.ID, "requirements-delta-decisions")
		assert.NotContains(t, gap.ID, "requirements-delta-rollback")
	}
}

func TestTraceabilityStructuredDeltaSupportSectionsMustBePopulated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "structured-delta-empty"

	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
## ADDED Requirements
### Requirement: A
REQ-001: Must do X.
## DECISIONS
- Keep current behavior without stable id.
## ROLLBACK
`)

	result := EvaluateTraceability(TraceabilityInput{BundleDir: dir, Slug: slug})
	assert.Contains(t, traceabilityGapIDs(result.Gaps), "requirements-delta-decisions-missing-ids")
	assert.Contains(t, traceabilityGapIDs(result.Gaps), "requirements-delta-rollback-empty")
}

func TestTraceabilitySuccessMessageOmitsOptionalArtifactsWhenAbsent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "core-success"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Original intent
`)

	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: A
REQ-001: Must do X. Traces to INT-001.
`)

	writeFile(t, filepath.Join(dir, "tasks.md"), `# Tasks
- [ ] `+"`t-01`"+` implement x
  covers: [REQ-001]
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir:  dir,
		Slug:       slug,
		SchemaName: model.ArtifactSchemaCore,
	})

	assert.Equal(t, model.TraceabilityStatusOK, result.Status)
	assert.Contains(t, result.Message, "intent")
	assert.Contains(t, result.Message, "requirements")
	assert.Contains(t, result.Message, "tasks")
	assert.NotContains(t, result.Message, "decisions")
	assert.NotContains(t, result.Message, "assurance")
}

func TestTraceabilityDecisionWithoutRequirementReference(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "decision-gap"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Original intent
`)

	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: A
REQ-001: Must do X. Traces to INT-001.
`)

	writeFile(t, filepath.Join(dir, "decision.md"), `# Decision
## Selected Approach
DEC-001: Use approach A without requirement linkage.
## Interfaces and Data Flow
Stable.
## Rollout and Rollback
Safe.
## Risk
Contained.
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir:  dir,
		Slug:       slug,
		SchemaName: model.ArtifactSchemaExpanded,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	assert.Contains(t, result.Gaps, model.TraceabilityGap{
		ID:       "DEC-001",
		Type:     "decision",
		Issue:    "decision has no linked requirement reference",
		Blocking: true,
	})
}

func TestTraceabilityRequirementWithoutIntent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "test-change"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Original intent
`)

	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
## ADDED Requirements
### Requirement: A
REQ-001: Must do X. Traces to INT-001.
### Requirement: B
REQ-002: Must do Y.
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	var gapIDs []string
	for _, g := range result.Gaps {
		gapIDs = append(gapIDs, g.ID)
	}
	assert.Contains(t, gapIDs, "REQ-002")
}

func TestTraceabilityRequirementWithUnknownIntentReferenceFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "unknown-intent-ref"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
## Summary
No structured intent IDs yet.
`)

	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: Session timeout
REQ-001: The system MUST support the requested change described as: session timeout. Traces to INT-001.
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	assert.Contains(t, result.Gaps, model.TraceabilityGap{
		ID:       "REQ-001",
		Type:     "requirement",
		Issue:    "requirement references unknown intent ID(s): INT-001",
		Blocking: true,
	})
}

func TestTraceabilityFailsWhenRequirementBlocksHaveNoStableREQIDs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "requirement-id-gap"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Original intent
`)

	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: Auth
The system must authenticate requests. Traces to INT-001.
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	var issues []string
	for _, g := range result.Gaps {
		issues = append(issues, g.Issue)
	}
	assert.Contains(t, issues, "requirements artifact has no stable REQ-* IDs")
}

func TestTraceabilityRequirementWithoutCoveringTask(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "task-gap"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Original intent
`)

	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
## ADDED Requirements
### Requirement: A
REQ-001: Must do X. Traces to INT-001.
### Requirement: B
REQ-002: Must do Y. Traces to INT-001.
`)

	writeFile(t, filepath.Join(dir, "tasks.md"), `# Tasks
- [ ] `+"`t-01`"+` implement x
  covers: [REQ-001]
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	var issues []string
	var gapIDs []string
	for _, g := range result.Gaps {
		issues = append(issues, g.Issue)
		gapIDs = append(gapIDs, g.ID)
	}
	assert.Contains(t, issues, "requirement has no covering task")
	assert.Contains(t, gapIDs, "REQ-002")
}

func TestTraceabilityRequirementDeltaSectionsMustContainBlocks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "delta-structure"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Original intent
`)
	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
## ADDED Requirements
REQ-001: Must do X. Traces to INT-001.
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	var gapIDs []string
	for _, g := range result.Gaps {
		gapIDs = append(gapIDs, g.ID)
	}
	assert.Contains(t, gapIDs, "requirements-delta-added-no-blocks")
}

func TestTraceabilityCoreSchemaDowngradesMissingIntentReferenceToWarning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "core-change"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Original intent
`)

	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: A
REQ-001: Must do X.
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir:  dir,
		Slug:       slug,
		SchemaName: model.ArtifactSchemaCore,
	})

	assert.Equal(t, model.TraceabilityStatusWarning, result.Status)
	require.Len(t, result.Gaps, 1)
	assert.Equal(t, "REQ-001", result.Gaps[0].ID)
	assert.False(t, result.Gaps[0].Blocking)
}

func TestTraceabilityAssuranceNoREQIDs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "test-change"

	writeFile(t, filepath.Join(dir, "intent.md"), `INT-001: Intent`)
	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: Something
REQ-001: Something. INT-001
`)
	writeFile(t, filepath.Join(dir, "assurance.md"), `# Assurance
All tests pass.
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	var issues []string
	for _, g := range result.Gaps {
		issues = append(issues, g.Issue)
	}
	assert.Contains(t, issues, "assurance verifies no requirement IDs")
}

func TestTraceabilityAssuranceMustCoverEveryRequirement(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "assurance-gap"

	writeFile(t, filepath.Join(dir, "intent.md"), `INT-001: Intent`)
	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: Something
REQ-001: Something. INT-001
### Requirement: Something Else
REQ-002: Something else. INT-001
`)
	writeFile(t, filepath.Join(dir, "tasks.md"), `# Tasks
- [ ] `+"`t-01`"+` first task
  covers: [REQ-001, REQ-002]
`)
	writeFile(t, filepath.Join(dir, "assurance.md"), `# Assurance
## Requirement Coverage
REQ-001: verified via tests
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	var issues []string
	var gapIDs []string
	for _, g := range result.Gaps {
		issues = append(issues, g.Issue)
		gapIDs = append(gapIDs, g.ID)
	}
	assert.Contains(t, issues, "requirement missing assurance coverage verdict")
	assert.Contains(t, gapIDs, "REQ-002")
}

func TestTraceabilityBlockingOpenQuestions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Something

## Open Questions
- What about edge cases?
`)

	slug := "test-change"
	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: Something
REQ-001: Something. INT-001
`)
	writeFile(t, filepath.Join(dir, "tasks.md"), `- [ ] Do thing
  covers: [REQ-001]
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	var issues []string
	for _, g := range result.Gaps {
		issues = append(issues, g.Issue)
	}
	assert.Contains(t, issues, "blocking open questions remain unresolved while downstream artifacts are ready")
}

func TestTraceabilityBlockingOpenQuestionProse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Something

## Open Questions
Need to decide whether OpenCode commands are flat or nested.
`)

	slug := "test-change"
	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: Something
REQ-001: Something. INT-001
`)
	writeFile(t, filepath.Join(dir, "tasks.md"), `- [ ] Do thing
  covers: [REQ-001]
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	var issues []string
	for _, g := range result.Gaps {
		issues = append(issues, g.Issue)
	}
	assert.Contains(t, issues, "blocking open questions remain unresolved while downstream artifacts are ready")
}

func TestTraceabilityResolvedOpenQuestionsDoNotBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "resolved-questions"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Something

## Open Questions
- [x] Edge case resolved
`)
	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: Something
REQ-001: Something. INT-001
`)
	writeFile(t, filepath.Join(dir, "tasks.md"), `# Tasks
- [ ] `+"`t-01`"+` do thing
  covers: [REQ-001]
`)
	writeFile(t, filepath.Join(dir, "assurance.md"), `# Assurance
## Requirement Coverage
REQ-001: verified
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusOK, result.Status)
	for _, g := range result.Gaps {
		assert.NotEqual(t, "intent-open-questions", g.ID)
	}
}

func TestTraceabilityUsesCanonicalOpenQuestionsSectionOverSummarySourceDocument(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "canonical-open-questions"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Something

## Summary
### Source Document

## Open Questions
- What about edge cases copied from the source document?

## In Scope
- keep canonical sections authoritative

## Out of Scope
- summary rewrites

## Acceptance Signals
- traceability stays clean

## Open Questions
(none)
`)
	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
### Requirement: Something
REQ-001: Something. Traces to INT-001.
`)
	writeFile(t, filepath.Join(dir, "tasks.md"), `# Tasks
- [ ] `+"`t-01`"+` do thing
  covers: [REQ-001]
`)
	writeFile(t, filepath.Join(dir, "assurance.md"), `# Assurance
## Requirement Coverage
REQ-001: verified
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusOK, result.Status)
	for _, g := range result.Gaps {
		assert.NotEqual(t, "intent-open-questions", g.ID)
	}
}

func TestExtractCoversRefsUsesStructuredTaskIDsAndStableFallback(t *testing.T) {
	t.Parallel()

	refs := extractCoversRefs(`# Tasks
- [ ] ` + "`t-01`" + ` investigate auth flow
  covers: [REQ-001]
- [ ] Investigate token refresh edge cases
  covers: [REQ-002]
`)

	require.Contains(t, refs, "t-01")
	require.Contains(t, refs, "Investigate token refresh edge cases")
	assert.Equal(t, []string{"REQ-001"}, refs["t-01"])
	assert.Equal(t, []string{"REQ-002"}, refs["Investigate token refresh edge cases"])
}

func TestTraceabilityEmptyBundle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      "empty",
	})

	// Empty bundle should be OK — no artifacts to check.
	assert.Equal(t, model.TraceabilityStatusOK, result.Status)
}

func TestDowngradeAuditGapsForLightPresetPreservesOnlyGovernanceAndStructuralBlockingGaps(t *testing.T) {
	t.Parallel()

	summary := model.TraceabilitySummary{
		Status:  model.TraceabilityStatusFail,
		Message: "6 blocking traceability gaps",
		Gaps: []model.TraceabilityGap{
			{ID: "requirements-no-blocks", Type: "requirement", Issue: "requirements artifact has no Requirement blocks (expected ### Requirement: <Name>)", Blocking: true},
			{ID: "requirements-stable-ids", Type: "requirement", Issue: "requirements artifact has no stable REQ-* IDs", Blocking: true},
			{ID: "REQ-001", Type: "requirement", Issue: "requirement has no upstream intent reference", Blocking: true},
			{ID: "REQ-002", Type: "requirement", Issue: "requirement has no covering task", Blocking: true},
			{ID: "DEC-001", Type: "decision", Issue: "decision has no linked requirement reference", Blocking: true},
			{ID: "intent-open-questions", Type: "intent", Issue: "blocking open questions remain unresolved while downstream artifacts are ready", Blocking: true},
		},
	}

	downgraded := downgradeAuditGapsForLightPreset(summary)
	blockingByID := make(map[string]bool, len(downgraded.Gaps))
	for _, gap := range downgraded.Gaps {
		blockingByID[gap.ID] = gap.Blocking
	}

	assert.True(t, blockingByID["requirements-no-blocks"])
	assert.True(t, blockingByID["requirements-stable-ids"])
	assert.True(t, blockingByID["intent-open-questions"])
	assert.False(t, blockingByID["DEC-001"])
	assert.False(t, blockingByID["REQ-001"])
	assert.False(t, blockingByID["REQ-002"])
	assert.Equal(t, model.TraceabilityStatusFail, downgraded.Status)
}

func TestTraceabilityRequirementsWithNoBlocksIsBlockingGap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "no-blocks"

	writeFile(t, filepath.Join(dir, "intent.md"), `# Intent
INT-001: Something
`)
	// requirements.md has content but no ### Requirement: blocks — sync would reject this.
	writeFile(t, resolveTestArtifact(dir, slug), `# Requirements
REQ-001: Something. INT-001
`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusFail, result.Status)
	var gapIDs []string
	for _, g := range result.Gaps {
		gapIDs = append(gapIDs, g.ID)
	}
	assert.Contains(t, gapIDs, "requirements-no-blocks",
		"requirements.md with no Requirement blocks should produce a blocking traceability gap")
}

func TestTraceabilityEmptyRequirementsFileDoesNotProduceNoBlocksGap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	slug := "missing-req"

	// requirements.md does not exist — should not add the no-blocks gap.
	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      slug,
	})

	assert.Equal(t, model.TraceabilityStatusOK, result.Status)
	for _, g := range result.Gaps {
		assert.NotEqual(t, "requirements-no-blocks", g.ID)
	}
}

func TestTraceabilityCustomResolver(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "my-intent.md"), `INT-001: Custom intent`)
	writeFile(t, filepath.Join(dir, "my-reqs.md"), `REQ-001: Custom req. INT-001`)

	result := EvaluateTraceability(TraceabilityInput{
		BundleDir: dir,
		Slug:      "custom",
		ArtifactResolver: func(name string) string {
			switch name {
			case "intent.md":
				return filepath.Join(dir, "my-intent.md")
			case "requirements.md":
				return filepath.Join(dir, "my-reqs.md")
			default:
				return filepath.Join(dir, name)
			}
		},
	})

	// Verify the custom resolver was used: INT-001 and REQ-001 should produce
	// a REQ→INT link, confirming the resolver fed the right files.
	assert.NotEmpty(t, result.Links, "expected at least one traceability link from custom-resolved artifacts")
}
