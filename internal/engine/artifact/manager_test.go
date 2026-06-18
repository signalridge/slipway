package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaffoldGovernedBundleL2CreatesRequiredFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("my-change")
	err := ScaffoldGovernedBundleForChange(root, change, "")
	require.NoError(t, err)

	base := filepath.Join(root, "artifacts", "changes", "my-change")
	// The engine still scaffolds the artifacts whose bodies it owns.
	for _, file := range []string{
		"intent.md",
	} {
		_, err := os.Stat(ResolveArtifactPath(base, file))
		require.NoError(t, err, file)
	}

	// requirements.md/decision.md/tasks.md are authored directly by the host
	// skill via `slipway instructions`; the engine defers their creation so an
	// un-authored required artifact surfaces as missing/fail-closed rather than a
	// placeholder body the skill must overwrite (issue #119). assurance.md is
	// deferred for the same reason but authored later, at S3_REVIEW (issue #141):
	// the engine no longer seeds an early scaffold at S1_PLAN/bundle.
	for _, file := range []string{
		"requirements.md",
		"decision.md",
		"tasks.md",
		"assurance.md",
	} {
		_, err := os.Stat(ResolveArtifactPath(base, file))
		require.Error(t, err, file)
		assert.True(t, os.IsNotExist(err), file)
	}

	// status.md is no longer written — change.yaml in bundle is the authority.
	_, err = os.Stat(filepath.Join(base, "status.md"))
	require.True(t, os.IsNotExist(err), "status.md should not exist after scaffold")

	_, err = os.Stat(filepath.Join(base, "research.md"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

// assurance.md is deferred to S3_REVIEW authoring (issue #141): the engine must
// not seed an early scaffold at S1_PLAN/bundle, on any preset where it would
// otherwise be required. On light preset it is not required at all, so its absence
// is the unchanged no-op.
func TestScaffoldGovernedBundleDefersAssurance(t *testing.T) {
	t.Parallel()

	require.True(t, deferredToSkillAuthoring("assurance.md"),
		"assurance.md must be deferred to skill authoring")

	for _, preset := range []model.WorkflowPreset{
		model.WorkflowPresetStandard,
		model.WorkflowPresetStrict,
		model.WorkflowPresetLight,
	} {
		preset := preset
		t.Run(string(preset), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			change := model.NewChange("assurance-defer-" + string(preset))
			change.WorkflowPreset = preset
			require.NoError(t, ScaffoldGovernedBundleForChange(root, change, preset))

			base := filepath.Join(root, "artifacts", "changes", change.Slug)
			_, err := os.Stat(ResolveArtifactPath(base, "assurance.md"))
			require.Error(t, err, "assurance.md must not be scaffolded on %s preset", preset)
			assert.True(t, os.IsNotExist(err))
		})
	}
}

func TestScaffoldGovernedBundleNeedsDiscoveryDefersResearchAuthoring(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	worktreeRoot := t.TempDir()
	change := model.NewChange("my-change")
	change.NeedsDiscovery = true
	change.WorktreePath = worktreeRoot

	require.NoError(t, ScaffoldGovernedBundleForChange(root, change, ""))

	_, err := os.Stat(filepath.Join(worktreeRoot, "artifacts", "changes", "my-change", "research.md"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestScaffoldGovernedBundleNoDiscoverySkipsResearch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("my-change")

	require.NoError(t, ScaffoldGovernedBundleForChange(root, change, ""))

	_, err := os.Stat(filepath.Join(root, "artifacts", "changes", "my-change", "research.md"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestScaffoldGovernedBundleL1CreatesBundle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, ScaffoldGovernedBundleForChange(root, model.NewChange("my-change"), ""))

	_, err := os.Stat(filepath.Join(root, "artifacts", "changes", "my-change"))
	require.NoError(t, err)
}

func TestScaffoldGovernedBundleDiscoveryDefersResearch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	change := model.NewChange("discovery-change")
	change.NeedsDiscovery = true
	require.NoError(t, ScaffoldGovernedBundleForChange(root, change, ""))

	_, err := os.Stat(filepath.Join(root, "artifacts", "changes", change.Slug, "research.md"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

// TestArtifactTemplatesParseUnderEngineParsers pins the instructions exemplars
// (what `slipway instructions` serves) to the engine parsers that consume the
// authored artifacts, so the template and parser cannot drift — a drift would
// make skill-authored files unparseable (issue #119).
func TestArtifactTemplatesParseUnderEngineParsers(t *testing.T) {
	t.Parallel()
	reqTemplate, err := RenderArtifactExample("requirements.md")
	require.NoError(t, err)
	assert.Contains(t, reqTemplate, "### Requirement: <short requirement title>",
		"requirements instructions must keep the format marker the parser expects authors to fill")
	req := `# Requirements

## Requirements

### Requirement: Authenticated access
REQ-001: The system MUST reject protected-route requests that do not carry valid credentials.

#### Scenario: Missing credentials
GIVEN a request without valid credentials
WHEN it reaches a protected route
THEN the system returns 401 and does not serve the protected resource.
`
	reqBlocks := ParseRequirementBlocks(req)
	require.Len(t, reqBlocks, 1, "filled requirements example must parse into one block")
	assert.Equal(t, "Authenticated access", reqBlocks[0].Name)
	assert.Equal(t, "REQ-001", reqBlocks[0].StableID)
	assert.Empty(t, RequirementSubstanceBlockers(req),
		"filled requirements example must satisfy the requirements substance parser")

	tasks, err := RenderArtifactExample("tasks.md")
	require.NoError(t, err)
	_, err = wave.ParseTaskPlan(tasks)
	require.NoError(t, err, "tasks template must parse under the engine task-plan parser")
}

func TestTemplateRequiredSections(t *testing.T) {
	t.Parallel()
	assurance, err := TemplateContent("assurance.md")
	require.NoError(t, err)
	for _, heading := range requiredSectionsForArtifact("assurance.md") {
		assert.Contains(t, assurance, heading)
	}

	decision, err := TemplateContent("decision.md")
	require.NoError(t, err)
	for _, heading := range requiredSectionsForArtifact("decision.md") {
		assert.Contains(t, decision, heading)
	}

	research, err := TemplateContent("research.md")
	require.NoError(t, err)
	for _, heading := range requiredSectionsForArtifact("research.md") {
		assert.Contains(t, research, heading)
	}
}

func TestSchemaCarriesRequiredSectionsForValidatedArtifacts(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, requiredSectionsForArtifact("research.md"))
	assert.NotEmpty(t, requiredSectionsForArtifact("decision.md"))
	assert.NotEmpty(t, requiredSectionsForArtifact("assurance.md"))
}

func TestValidateResearchStructure(t *testing.T) {
	t.Parallel()
	valid := `## Alternatives Considered
Option A and Option B with tradeoffs.

## Unknowns
Unresolved technical questions.

## Assumptions
Assumptions to validate.

## Canonical References
Docs and specs that constrain this work.`

	blockers := ResearchStructureBlockers(valid)
	assert.Empty(t, blockers)

	template, err := RenderArtifactExample("research.md")
	require.NoError(t, err)
	blockers = ResearchStructureBlockers(template)
	require.Len(t, blockers, 1,
		"comment-only instructions template sections must fail closed at the structure layer")
	assert.Equal(t, "research_structure_invalid", blockers[0].Code)
	assert.Contains(t, blockers[0].Detail, "non-empty content")

	legacySeeded := `# Research

## Alternatives Considered
Pending investigation. Replace with concrete alternatives, supporting evidence, and the selected direction.

## Unknowns
- Pending investigation. List unknowns that must be resolved before planning.

## Assumptions
- Pending investigation. List assumptions only after identifying the evidence that supports them.

## Canonical References
- ` + "`artifacts/changes/example-change/intent.md`" + ` for the original request and intake context.
- ` + "`requirements.md`" + ` and ` + "`decision.md`" + ` in the same bundle once planning artifacts are refined.
- Existing code paths and tests related to the affected behavior in the repository.
`
	blockers = ResearchStructureBlockers(legacySeeded)
	require.Len(t, blockers, 4, "every legacy seeded research section must be flagged")
	assert.Contains(t, model.ReasonSpecs(blockers), "research_section_placeholder:## Alternatives Considered")
	assert.Contains(t, model.ReasonSpecs(blockers), "research_section_placeholder:## Unknowns")
	assert.Contains(t, model.ReasonSpecs(blockers), "research_section_placeholder:## Assumptions")
	assert.Contains(t, model.ReasonSpecs(blockers), "research_section_placeholder:## Canonical References")

	authoredFromTemplate := strings.NewReplacer(
		"<!-- Real alternatives with supporting evidence and the selected direction. -->",
		"Compared direct engine seeding with skill-authored artifacts; skill authoring preserves fail-closed ownership.",
		"<!-- Unknowns that must be resolved before planning, or \"None\". -->",
		"None; parser and readiness contracts were confirmed in tests.",
		"<!-- Assumptions with the evidence that supports them. -->",
		"Assumes `slipway instructions research` is the public authoring surface.",
		"<!-- file:path references used as planning authority. -->",
		"internal/tmpl/templates/artifacts/research.md\ninternal/engine/artifact/manager.go",
	).Replace(template)
	assert.Empty(t, ResearchStructureBlockers(authoredFromTemplate),
		"an authored research.md derived from the real instructions template must pass")

	// Missing a required section.
	missing := `## Alternatives Considered
Option A.

## Unknowns
Questions.`
	blockers = ResearchStructureBlockers(missing)
	require.Len(t, blockers, 1)
	assert.Equal(t, "research_structure_invalid", blockers[0].Code)

	// Reordered headings must fail.
	reordered := `## Unknowns
Questions.

## Alternatives Considered
Options.

## Assumptions
Assumptions.

## Canonical References
Refs.`
	blockers = ResearchStructureBlockers(reordered)
	require.Len(t, blockers, 1)
	assert.Equal(t, "research_structure_invalid", blockers[0].Code)
}

func TestValidateAssuranceStructure(t *testing.T) {
	t.Parallel()
	valid := `## Scope Summary
One

## Verification Verdict
Two

## Evidence Index
Three

## Requirement Coverage
Coverage mapping

## Residual Risks and Exceptions
Four

## Rollback Readiness
Rollback remains available.

## Archive Decision
Five`
	require.NoError(t, ValidateAssuranceStructure(valid))

	invalid := `## Scope Summary
One

## Verification Verdict
Two`
	require.Error(t, ValidateAssuranceStructure(invalid))

	emptySection := `## Scope Summary
One

## Verification Verdict

## Evidence Index
Three

## Requirement Coverage
Coverage mapping

## Residual Risks and Exceptions
Four

## Rollback Readiness
Rollback remains available.

## Archive Decision
Five`
	err := ValidateAssuranceStructure(emptySection)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty content")

	fullContent := `## Scope Summary
One

## Verification Verdict
Two

## Evidence Index
Three

## Requirement Coverage
Coverage mapping

## Residual Risks and Exceptions
Four

## Rollback Readiness
Rollback remains available.

## Archive Decision
Five`
	require.NoError(t, ValidateAssuranceStructure(fullContent))

	missingRollback := `## Scope Summary
One

## Verification Verdict
Two

## Evidence Index
Three

## Requirement Coverage
Coverage mapping

## Residual Risks and Exceptions
Four

## Archive Decision
Five`
	err = ValidateAssuranceStructure(missingRollback)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Rollback Readiness")

	// Reordered headings must fail.
	reordered := `## Archive Decision
Five

## Scope Summary
One

## Verification Verdict
Two

## Evidence Index
Three

## Requirement Coverage
Coverage mapping

## Residual Risks and Exceptions
Four

## Rollback Readiness
Rollback remains available.`
	err = ValidateAssuranceStructure(reordered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required heading")

	// Completely empty content must fail.
	require.Error(t, ValidateAssuranceStructure(""))

	// Missing individual headings each produce a specific error.
	requiredHeadings := []string{
		"## Scope Summary",
		"## Verification Verdict",
		"## Evidence Index",
		"## Requirement Coverage",
		"## Residual Risks and Exceptions",
		"## Rollback Readiness",
		"## Archive Decision",
	}
	for _, heading := range requiredHeadings {
		// Build content that omits exactly one heading.
		var sb []string
		for _, h := range requiredHeadings {
			if h == heading {
				continue
			}
			sb = append(sb, h+"\nContent for "+h)
		}
		content := strings.Join(sb, "\n\n")
		err := ValidateAssuranceStructure(content)
		require.Error(t, err, "expected error when missing %q", heading)
		assert.Contains(t, err.Error(), heading, "error should mention missing heading %q", heading)
	}
}

func TestAssuranceStructureBlockers(t *testing.T) {
	t.Parallel()
	valid := `## Scope Summary
One

## Verification Verdict
Two

## Evidence Index
Three

## Requirement Coverage
Coverage mapping

## Residual Risks and Exceptions
Four

## Rollback Readiness
Rollback remains available.

## Archive Decision
Five`
	assert.Empty(t, AssuranceStructureBlockers(valid))

	// Missing heading returns blocker.
	invalid := `## Scope Summary
One`
	blockers := AssuranceStructureBlockers(invalid)
	require.Len(t, blockers, 1)
	assert.Contains(t, blockers[0], "assurance_structure_invalid:")

	// Empty content returns blocker.
	blockers = AssuranceStructureBlockers("")
	require.Len(t, blockers, 1)
	assert.Contains(t, blockers[0], "assurance_structure_invalid:")
}

// TestAssuranceStructureBlockersRejectsScaffold covers issue #47: a structurally
// valid assurance.md whose sections still hold the template scaffold prose must
// be rejected, with one blocker naming each scaffold section.
func TestAssuranceStructureBlockersRejectsScaffold(t *testing.T) {
	t.Parallel()

	// The embedded template is itself the canonical all-scaffold document.
	scaffold, err := TemplateContent("assurance.md")
	require.NoError(t, err)

	blockers := AssuranceStructureBlockers(scaffold)
	require.NotEmpty(t, blockers, "all-scaffold assurance.md must be rejected")
	for _, heading := range requiredSectionsForArtifact("assurance.md") {
		assert.Contains(t, blockers, "assurance_section_placeholder:"+heading,
			"scaffold section %q must be flagged", heading)
	}
	// Structure itself is valid; the rejection is purely the placeholder floor.
	for _, b := range blockers {
		assert.NotContains(t, b, "assurance_structure_invalid")
	}
}

// TestAssuranceStructureBlockersPartialPlaceholder covers the partial case: only
// the still-scaffold section is named; authored sections pass.
func TestAssuranceStructureBlockersPartialPlaceholder(t *testing.T) {
	t.Parallel()

	// Every section authored except Scope Summary, which is left as the verbatim
	// template scaffold sentence.
	content := `## Scope Summary
Summarize delivered scope.

## Verification Verdict
All 42 tests pass; go build ./... is green.

## Evidence Index
- test: go test ./... (42/42)

## Requirement Coverage
REQ-001 -> t-01; REQ-002 -> t-02.

## Residual Risks and Exceptions
None; the change is behind the standard/strict gate only.

## Rollback Readiness
Single commit; git revert restores prior behavior.

## Archive Decision
Archived after a fresh validate --json freshness proof was captured before done.`

	blockers := AssuranceStructureBlockers(content)
	assert.Equal(t, []string{"assurance_section_placeholder:## Scope Summary"}, blockers)
}

// TestAssuranceStructureBlockersRejectsArchiveDecisionSeedSentence covers the
// legacy one-sentence Archive Decision scaffold from issue #47. The current
// template has grown additional guidance, but retaining only the original seed
// sentence is still placeholder content.
func TestAssuranceStructureBlockersRejectsArchiveDecisionSeedSentence(t *testing.T) {
	t.Parallel()

	content := `## Scope Summary
Added a template-derived placeholder floor to assurance validation.

## Verification Verdict
go test ./... passes (full suite green).

## Evidence Index
- test: go test ./...

## Requirement Coverage
REQ-001..006 each mapped to t-01..t-05.

## Residual Risks and Exceptions
None beyond the documented light-preset exclusion.

## Rollback Readiness
Revert the single feature commit.

## Archive Decision
Record archive readiness decision.`

	assert.Equal(t,
		[]string{"assurance_section_placeholder:## Archive Decision"},
		AssuranceStructureBlockers(content),
	)
}

// TestAssuranceStructureBlockersAuthoredPasses covers the fully-authored case.
func TestAssuranceStructureBlockersAuthoredPasses(t *testing.T) {
	t.Parallel()

	content := `## Scope Summary
Added a template-derived placeholder floor to assurance validation.

## Verification Verdict
go test ./... passes (full suite green).

## Evidence Index
- test: go test ./...

## Requirement Coverage
REQ-001..006 each mapped to t-01..t-05.

## Residual Risks and Exceptions
None beyond the documented light-preset exclusion.

## Rollback Readiness
Revert the single feature commit.

## Archive Decision
Ready to archive; validate --json freshness proof captured before done.`

	assert.Empty(t, AssuranceStructureBlockers(content))
}

// TestAssuranceSectionScaffoldDerivesFromTemplate is the template-drift safety
// check: the detector's per-section scaffold MUST equal the embedded template's
// section bodies, so detection follows the template instead of a hand-maintained
// phrase list.
func TestAssuranceSectionScaffoldDerivesFromTemplate(t *testing.T) {
	t.Parallel()

	scaffold := assuranceSectionScaffold()
	require.NotEmpty(t, scaffold)

	tmplContent, err := TemplateContent("assurance.md")
	require.NoError(t, err)

	for _, heading := range requiredSectionsForArtifact("assurance.md") {
		body := normalizeAssuranceBody(strings.Join(markdownSectionLines(tmplContent, heading), "\n"))
		require.NotEmpty(t, body, "template section %q must have scaffold body", heading)
		assert.Equal(t, body, scaffold[heading],
			"detector scaffold for %q must be derived from the template", heading)
	}
}

func TestScaffoldGovernedBundleMarkdownSizeBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("add-session-timeout-policy")
	change.NeedsDiscovery = true

	require.NoError(t, ScaffoldGovernedBundleForChange(root, change, ""))

	base := filepath.Join(root, "artifacts", "changes", change.Slug)
	entries, err := os.ReadDir(base)
	require.NoError(t, err)

	total := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(base, entry.Name()))
		require.NoError(t, err)
		total += len(raw)
	}

	assert.LessOrEqual(t, total, 15000, "fresh artifact bundle scaffold must stay structural, not analytical")
}

func TestParseDecisionLockedDecisionsIgnoresUnconfirmedSeededTextWithoutDraftComment(t *testing.T) {
	t.Parallel()

	content := `# Decision

## Alternatives Considered
### Approach A: Direct implementation
Implement update auth middleware timeout strategy with minimal structural changes.
- Pros: Straightforward, low risk
- Cons: May not address future extensibility

### Approach B: Refactor-first
Restructure affected code before implementing the change.
- Pros: Improved maintainability
- Cons: Larger scope, higher risk

### Selected Direction
Approach A. Confirm or replace this after research and user selection.

## Selected Approach
Start with the direct implementation path for update auth middleware timeout strategy. Revisit that choice only if code inspection shows the current boundaries cannot support the change safely. Confirm or replace this after research and user selection.

## Risk
Primary risk for update auth middleware timeout strategy is hidden coupling in the existing implementation. Mitigate with focused code inspection and regression coverage before broad structural changes. Confirm or replace this after risk review.
`

	assert.Nil(t, ParseDecisionLockedDecisions(content))
}

func TestParseDecisionLockedDecisionsRejectsDeadStatus(t *testing.T) {
	t.Parallel()

	content := `# Decision

## Status
Superseded

## Alternatives Considered
### Approach A
Keep the current parser.

### Approach B
Use the shared parsed decision contract.

### Selected Direction
Approach B because status handling must be shared.

## Selected Approach
Use the shared parsed decision contract.

## Interfaces and Data Flow
decision.md -> ParseDecisionContract -> callers.

## Rollout and Rollback
Revert this parser change if needed.

## Risk
Low risk.
`

	assert.Nil(t, ParseDecisionLockedDecisions(content))
}

func TestStalePropagationOrderBFS(t *testing.T) {
	t.Parallel()
	order, err := stalePropagationOrderFromGraph("intent.md", DefaultStaleGraph())
	require.NoError(t, err)
	require.NotEmpty(t, order)
	assert.NotContains(t, order, "intent.md")
	assert.Contains(t, order, "requirements.md")
	assert.Contains(t, order, "decision.md")
	assert.Contains(t, order, "tasks.md")
	assert.Contains(t, order, "assurance.md")
}

func TestScaffoldCustomSchemaWithExternalTemplate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create external template file.
	tmplDir := filepath.Join(root, "templates")
	require.NoError(t, os.MkdirAll(tmplDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmplDir, "custom-artifact.md"),
		[]byte("# Custom Artifact for {{.Slug}}\n"),
		0o644,
	))

	customSchema := []ArtifactSpec{
		{Name: "change.yaml"},
		{Name: "custom-artifact.md", Template: filepath.Join("templates", "custom-artifact.md")},
	}

	err := ScaffoldGovernedBundleForChange(root, model.NewChange("test-change"), "", customSchema)
	require.NoError(t, err)

	base := filepath.Join(root, "artifacts", "changes", "test-change")
	content, err := os.ReadFile(filepath.Join(base, "custom-artifact.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Custom Artifact for test-change\n", string(content))
}

func TestScaffoldCustomSchemaFallbackToStub(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Artifact with no template and no embedded match → minimal stub.
	customSchema := []ArtifactSpec{
		{Name: "my-widget.md"},
	}

	err := ScaffoldGovernedBundleForChange(root, model.NewChange("test-change"), "", customSchema)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", "test-change", "my-widget.md"))
	require.NoError(t, err)
	assert.Equal(t, "# my-widget\n", string(content))
}

func TestResolveSchemaCustomPropagatesTemplate(t *testing.T) {
	t.Parallel()
	defs := []model.ArtifactDefinition{
		{Name: "decision.md", Template: "templates/decision.md", DependsOn: []string{"intent.md"}},
		{Name: "intent.md"},
	}
	specs := ResolveSchema(model.ArtifactSchemaCustom, defs)
	require.Len(t, specs, 2)
	assert.Equal(t, "templates/decision.md", specs[0].Template)
	assert.Equal(t, "", specs[1].Template)
}

func TestPropagateStaleSkipsWhenContentHashMatches(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifactPath := filepath.Join(root, "intent.md")
	require.NoError(t, os.WriteFile(artifactPath, []byte("# Intent\nContent here"), 0o644))

	hash, err := model.ComputeFileContentHash(artifactPath)
	require.NoError(t, err)

	change := &model.Change{
		Slug: "test",
		Artifacts: map[string]model.ArtifactState{
			"intent":       {ID: "intent", Path: artifactPath, ContentHash: hash, State: model.ArtifactLifecycleDraft},
			"requirements": {ID: "requirements", State: model.ArtifactLifecycleDraft},
			"decision":     {ID: "decision", State: model.ArtifactLifecycleDraft},
			"tasks":        {ID: "tasks", State: model.ArtifactLifecycleDraft},
		},
	}

	// Content hasn't changed — propagation should be skipped.
	require.NoError(t, PropagateStale(change, "intent.md"))
	assert.Equal(t, model.ArtifactLifecycleDraft, change.Artifacts["requirements"].State)
	assert.Equal(t, model.ArtifactLifecycleDraft, change.Artifacts["decision"].State)
	assert.Equal(t, model.ArtifactLifecycleDraft, change.Artifacts["tasks"].State)
}

func TestPropagateStalePropagatestWhenContentHashDiffers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifactPath := filepath.Join(root, "intent.md")
	require.NoError(t, os.WriteFile(artifactPath, []byte("# Intent\nOriginal"), 0o644))

	// Store an old hash, then modify the file.
	change := &model.Change{
		Slug: "test",
		Artifacts: map[string]model.ArtifactState{
			"intent":       {ID: "intent", Path: artifactPath, ContentHash: "old-hash-not-matching", State: model.ArtifactLifecycleDraft},
			"requirements": {ID: "requirements", State: model.ArtifactLifecycleDraft},
			"tasks":        {ID: "tasks", State: model.ArtifactLifecycleDraft},
		},
	}

	require.NoError(t, PropagateStale(change, "intent.md"))
	assert.Equal(t, model.ArtifactLifecycleStale, change.Artifacts["requirements"].State)
	assert.Equal(t, model.ArtifactLifecycleStale, change.Artifacts["tasks"].State)
}

func TestReconcileFromFilesystemMissingFileResetsToDraft(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-change"
	base := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(base, 0o755))

	// Artifact file does NOT exist on disk.
	change := &model.Change{
		Slug: slug,
		Artifacts: map[string]model.ArtifactState{
			"intent": {
				ID:    "intent",
				State: model.ArtifactLifecycleApproved,
			},
		},
	}

	_, reconcileErr := ReconcileFromFilesystem(root, change)
	require.NoError(t, reconcileErr)
	assert.Equal(t, model.ArtifactLifecycleDraft, change.Artifacts["intent"].State)
}

func TestReconcileFromFilesystemMissingUpstreamFileStalesDownstreamAndClearsHash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-change"
	base := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(base, 0o755))

	reqPath := ResolveArtifactPath(base, "requirements.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(reqPath), 0o755))
	require.NoError(t, os.WriteFile(reqPath, []byte("# Requirements\nCurrent content"), 0o644))

	reqHash, err := model.ComputeFileContentHash(reqPath)
	require.NoError(t, err)

	change := &model.Change{
		Slug: slug,
		Artifacts: map[string]model.ArtifactState{
			"intent": {
				ID:          "intent",
				State:       model.ArtifactLifecycleApproved,
				ContentHash: "old-intent-hash",
			},
			"requirements": {
				ID:          "requirements",
				State:       model.ArtifactLifecycleApproved,
				ContentHash: reqHash,
			},
		},
	}

	_, reconcileErr := ReconcileFromFilesystem(root, change)
	require.NoError(t, reconcileErr)

	assert.Equal(t, model.ArtifactLifecycleDraft, change.Artifacts["intent"].State)
	assert.Empty(t, change.Artifacts["intent"].ContentHash)
	assert.Equal(t, model.ArtifactLifecycleStale, change.Artifacts["requirements"].State)
}

func TestReconcileFromFilesystemFrozenNotOverridden(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-change"
	base := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(base, 0o755))

	// Artifact file does NOT exist, but state is frozen.
	change := &model.Change{
		Slug: slug,
		Artifacts: map[string]model.ArtifactState{
			"intent": {
				ID:    "intent",
				State: model.ArtifactLifecycleFrozen,
			},
		},
	}

	_, reconcileErr := ReconcileFromFilesystem(root, change)
	require.NoError(t, reconcileErr)
	assert.Equal(t, model.ArtifactLifecycleFrozen, change.Artifacts["intent"].State)
}

func TestReconcileFromFilesystemHashMatchNoChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-change"
	base := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(base, 0o755))

	// Create the artifact file.
	content := []byte("# Proposal\nSome content here")
	artifactPath := filepath.Join(base, "intent.md")
	require.NoError(t, os.WriteFile(artifactPath, content, 0o644))

	hash, err := model.ComputeFileContentHash(artifactPath)
	require.NoError(t, err)

	change := &model.Change{
		Slug: slug,
		Artifacts: map[string]model.ArtifactState{
			"intent": {
				ID:          "intent",
				State:       model.ArtifactLifecycleApproved,
				ContentHash: hash,
			},
		},
	}

	_, reconcileErr := ReconcileFromFilesystem(root, change)
	require.NoError(t, reconcileErr)
	assert.Equal(t, model.ArtifactLifecycleApproved, change.Artifacts["intent"].State)
	assert.Equal(t, hash, change.Artifacts["intent"].ContentHash)
}

func TestReconcileFromFilesystemHashMismatchPropagatesToDownstream(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-change"
	base := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(base, 0o755))

	// Create the intent file with new content (hash will differ from stored).
	artifactPath := filepath.Join(base, "intent.md")
	require.NoError(t, os.WriteFile(artifactPath, []byte("# Intent\nModified content"), 0o644))

	// Create downstream artifact files so they are not reset to draft by file-missing rule.
	reqPath := ResolveArtifactPath(base, "requirements.md")
	decisionPath := filepath.Join(base, "decision.md")
	tasksPath := filepath.Join(base, "tasks.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(reqPath), 0o755))
	require.NoError(t, os.WriteFile(reqPath, []byte("# Requirements"), 0o644))
	require.NoError(t, os.WriteFile(decisionPath, []byte("# Decision"), 0o644))
	require.NoError(t, os.WriteFile(tasksPath, []byte("# Tasks"), 0o644))

	reqHash, err := model.ComputeFileContentHash(reqPath)
	require.NoError(t, err)
	decisionHash, err := model.ComputeFileContentHash(decisionPath)
	require.NoError(t, err)
	tasksHash, err := model.ComputeFileContentHash(tasksPath)
	require.NoError(t, err)

	change := &model.Change{
		Slug: slug,
		Artifacts: map[string]model.ArtifactState{
			"intent": {
				ID:          "intent",
				State:       model.ArtifactLifecycleApproved,
				ContentHash: "old-hash-that-does-not-match",
			},
			"requirements": {
				ID:          "requirements",
				State:       model.ArtifactLifecycleDraft,
				ContentHash: reqHash,
			},
			"decision": {
				ID:          "decision",
				State:       model.ArtifactLifecycleApproved,
				ContentHash: decisionHash,
			},
			"tasks": {
				ID:          "tasks",
				State:       model.ArtifactLifecycleDraft,
				ContentHash: tasksHash,
			},
		},
	}

	_, reconcileErr := ReconcileFromFilesystem(root, change)
	require.NoError(t, reconcileErr)

	// intent state should be unchanged (hash mismatch does NOT change lifecycle).
	assert.Equal(t, model.ArtifactLifecycleApproved, change.Artifacts["intent"].State)

	// The stored hash should be updated to the new disk hash.
	newHash, err := model.ComputeFileContentHash(artifactPath)
	require.NoError(t, err)
	assert.Equal(t, newHash, change.Artifacts["intent"].ContentHash)

	// Downstream artifacts should be marked stale.
	assert.Equal(t, model.ArtifactLifecycleStale, change.Artifacts["requirements"].State)
	assert.Equal(t, model.ArtifactLifecycleStale, change.Artifacts["decision"].State)
	assert.Equal(t, model.ArtifactLifecycleStale, change.Artifacts["tasks"].State)
}

func TestReconcileFromFilesystemFrozenAmendableUnreadableArtifactReturnsError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-change"
	base := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(base, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(base, "intent.md"), 0o755))

	change := &model.Change{
		Slug:         slug,
		CurrentState: model.StateS2Implement,
		Artifacts: map[string]model.ArtifactState{
			"intent": {
				ID:          "intent",
				State:       model.ArtifactLifecycleFrozen,
				ContentHash: "frozen-hash",
				Path:        filepath.Join(base, "intent.md"),
			},
		},
	}

	_, err := ReconcileFromFilesystem(root, change)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intent.md")
}

func TestStalePropagationOrderUnknownArtifactError(t *testing.T) {
	t.Parallel()
	_, err := stalePropagationOrderFromGraph("nonexistent-artifact.md", DefaultStaleGraph())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownArtifact))
}

// TestReconcileFromFilesystemCRLFRematerializationIsNotStale is a Windows
// regression guard (REQ-002 scenario A / REQ-009). A governed text artifact is
// frozen with its content hash recorded while stored as LF, then re-read from
// disk with CRLF line endings (the Windows `git core.autocrlf=true` checkout
// case). Reconciliation runs in an amendment-eligible state (S2_IMPLEMENT), where
// a real content change would unfreeze the artifact to approved and record an
// AmendmentEvent. Because ComputeFileContentHash CRLF-normalizes text, the
// CRLF re-materialization must hash identically to the LF original, so
// reconciliation reports the artifact as UNCHANGED: it stays frozen, keeps its
// recorded hash, and produces no amendment. If CRLF normalization were removed,
// the LF-recorded hash would differ from the CRLF disk hash and this test would
// go RED (an amendment would fire and the state would flip to approved).
func TestReconcileFromFilesystemCRLFRematerializationIsNotStale(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "test-change"
	base := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(base, 0o755))

	artifactPath := filepath.Join(base, "intent.md")
	// Record the artifact while stored as LF (the original committed form).
	require.NoError(t, os.WriteFile(artifactPath, []byte("# Intent\nship the change\nverify it\n"), 0o644))
	lfHash, err := model.ComputeFileContentHash(artifactPath)
	require.NoError(t, err)

	// Re-materialize the SAME logical content on disk with CRLF line endings, as
	// a Windows autocrlf checkout would. The raw bytes differ from the LF form.
	require.NoError(t, os.WriteFile(artifactPath, []byte("# Intent\r\nship the change\r\nverify it\r\n"), 0o644))
	crlfBytes, err := os.ReadFile(artifactPath)
	require.NoError(t, err)
	require.Contains(t, string(crlfBytes), "\r\n", "test must drive the real CRLF on-disk case")

	// The normalized content hash is line-ending invariant: the regression guard.
	crlfHash, err := model.ComputeFileContentHash(artifactPath)
	require.NoError(t, err)
	require.Equal(t, lfHash, crlfHash,
		"CRLF re-materialization of identical text must hash equal to its LF form")

	change := &model.Change{
		Slug:         slug,
		CurrentState: model.StateS2Implement, // amendment-eligible: a real change would unfreeze.
		Artifacts: map[string]model.ArtifactState{
			"intent": {
				ID:          "intent",
				State:       model.ArtifactLifecycleFrozen,
				ContentHash: lfHash,
				Path:        artifactPath,
			},
		},
	}

	result, reconcileErr := ReconcileFromFilesystem(root, change)
	require.NoError(t, reconcileErr)

	// UNCHANGED: no auto-amendment for a line-ending-only re-materialization.
	assert.Empty(t, result.Amendments,
		"a CRLF-only re-materialization must not auto-amend a frozen artifact")
	assert.Equal(t, model.ArtifactLifecycleFrozen, change.Artifacts["intent"].State,
		"frozen artifact must remain frozen across a CRLF checkout")
	assert.Equal(t, lfHash, change.Artifacts["intent"].ContentHash,
		"recorded content hash must survive a CRLF round-trip unchanged")
}
