package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaffoldGovernedBundleL2CreatesRequiredFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("my-change")
	err := ScaffoldGovernedBundleForChangeWithPreset(root, change, "")
	require.NoError(t, err)

	base := filepath.Join(root, "artifacts", "changes", "my-change")
	for _, file := range []string{
		"intent.md",
		"requirements.md",
		"decision.md",
		"tasks.md",
		"assurance.md",
	} {
		_, err := os.Stat(ResolveArtifactPath(base, "my-change", file))
		require.NoError(t, err, file)
	}

	// status.md is no longer written — change.yaml in bundle is the authority.
	_, err = os.Stat(filepath.Join(base, "status.md"))
	require.True(t, os.IsNotExist(err), "status.md should not exist after scaffold")

	_, err = os.Stat(filepath.Join(base, "research.md"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestScaffoldGovernedBundleNeedsDiscoveryAddsResearch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	worktreeRoot := t.TempDir()
	change := model.NewChange("my-change")
	change.NeedsDiscovery = true
	change.WorktreePath = worktreeRoot

	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

	_, err := os.Stat(filepath.Join(worktreeRoot, "artifacts", "changes", "my-change", "research.md"))
	require.NoError(t, err)
}

func TestScaffoldGovernedBundleNoDiscoverySkipsResearch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("my-change")

	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

	_, err := os.Stat(filepath.Join(root, "artifacts", "changes", "my-change", "research.md"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestScaffoldGovernedBundleInjectsProjectContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := model.DefaultConfig()
	cfg.Context.TechStack = "Go, Cobra"
	cfg.Context.Conventions = "Deterministic CLI contracts"
	cfg.Context.TestCmd = "go test ./..."
	cfg.Context.BuildCmd = "go build ./..."
	cfg.Context.Languages = []string{"go", "yaml"}
	require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, model.NewChange("ctx-change"), ""))
	proposalPath := filepath.Join(root, "artifacts", "changes", "ctx-change", "intent.md")
	raw, err := os.ReadFile(proposalPath)
	require.NoError(t, err)
	content := string(raw)
	assert.Contains(t, content, "## Project Context")
	assert.Contains(t, content, "Go, Cobra")
	assert.Contains(t, content, "go test ./...")
	assert.Contains(t, content, "go, yaml")
}

func TestScaffoldGovernedBundleL1CreatesBundle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, model.NewChange("my-change"), ""))

	_, err := os.Stat(filepath.Join(root, "artifacts", "changes", "my-change"))
	require.NoError(t, err)
}

func TestScaffoldGovernedBundleDiscoveryCreatesResearch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	change := model.NewChange("discovery-change")
	change.NeedsDiscovery = true
	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

	_, err := os.Stat(filepath.Join(root, "artifacts", "changes", change.Slug, "research.md"))
	require.NoError(t, err)
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

func TestScaffoldGovernedBundleSeedsRequirementsDraft(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("add-session-timeout-policy")

	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

	raw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "requirements.md"))
	require.NoError(t, err)
	content := string(raw)

	require.Contains(t, content, "REQ-001")
	require.NotContains(t, content, "Add requirements using")
}

func TestScaffoldGovernedBundleSeedsDecisionDraft(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("add-session-timeout-policy")

	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

	raw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "decision.md"))
	require.NoError(t, err)
	content := string(raw)

	require.Contains(t, content, "Pending investigation")
	require.NotContains(t, content, "Approach A")
	require.NotContains(t, content, "| | | | |")
}

func TestScaffoldGovernedBundleSeedsDecisionSectionBodies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("add-session-timeout-policy")
	change.Description = "add session timeout policy"

	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

	raw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "decision.md"))
	require.NoError(t, err)
	content := string(raw)

	selected := strings.TrimSpace(strings.Join(markdownSectionLines(content, "Selected Approach"), "\n"))
	interfaces := strings.TrimSpace(strings.Join(markdownSectionLines(content, "Interfaces and Data Flow"), "\n"))
	rollback := strings.TrimSpace(strings.Join(markdownSectionLines(content, "Rollout and Rollback"), "\n"))
	risk := strings.TrimSpace(strings.Join(markdownSectionLines(content, "Risk"), "\n"))

	assert.Contains(t, selected, "Pending investigation")
	assert.NotContains(t, selected, "session timeout policy")
	assert.NotContains(t, selected, "Describe the chosen approach")

	assert.Contains(t, interfaces, "Pending investigation")
	assert.NotContains(t, interfaces, "session timeout policy")
	assert.NotContains(t, interfaces, "Pending — detail after approach is confirmed.")

	assert.Contains(t, rollback, "Pending investigation")
	assert.NotContains(t, rollback, "session timeout policy")
	assert.NotContains(t, rollback, "Pending — define after interfaces are finalized.")

	assert.Contains(t, risk, "Pending investigation")
	assert.NotContains(t, risk, "session timeout policy")
	assert.NotContains(t, risk, "Pending — assess after approach is confirmed.")
}

func TestScaffoldGovernedBundleSeedsTasksDraft(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("add-session-timeout-policy")

	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

	raw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "tasks.md"))
	require.NoError(t, err)
	content := string(raw)

	require.Contains(t, content, "t-01")
	require.Contains(t, content, "Pending task objective")
	require.NotContains(t, content, "Define implementation tasks")
	require.NotContains(t, content, "Add tests for the implementation")
}

func TestScaffoldGovernedBundleSeedsResearchDraft(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("add-session-timeout-policy")
	change.NeedsDiscovery = true

	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

	raw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "research.md"))
	require.NoError(t, err)
	content := string(raw)

	require.Contains(t, content, "Pending investigation")
	require.NotContains(t, content, "Approach A")
	require.Contains(t, content, "## Alternatives Considered")
}

func TestScaffoldGovernedBundleSeedsResearchSectionBodies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("add-session-timeout-policy")
	change.Description = "add session timeout policy"
	change.NeedsDiscovery = true

	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

	raw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "research.md"))
	require.NoError(t, err)
	content := string(raw)

	assert.Contains(t, content, "## Unknowns")
	assert.Contains(t, content, "## Assumptions")
	assert.Contains(t, content, "## Canonical References")
	assert.NotContains(t, content, "Unresolved technical questions to investigate.")
	assert.NotContains(t, content, "Assumptions to validate during planning.")
	assert.NotContains(t, content, "Docs, specs, code paths, and external references that constrain this work.")
	assert.NotContains(t, content, "session timeout policy")
}

func TestScaffoldGovernedBundleMarkdownSizeBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("add-session-timeout-policy")
	change.NeedsDiscovery = true

	require.NoError(t, ScaffoldGovernedBundleForChangeWithPreset(root, change, ""))

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

func TestSeedRequirementsContainsGuardrailDomain(t *testing.T) {
	t.Parallel()
	data := templateData{
		Slug:            "auth-change",
		InitialRequest:  "update auth middleware",
		GuardrailDomain: "auth_authz",
	}
	result := seedRequirements(data)
	assert.Contains(t, result, "REQ-001")
	assert.Contains(t, result, "REQ-002")
	assert.Contains(t, result, "auth_authz")
}

func TestSeedRequirementsFromDocPreservesGuardrailDomain(t *testing.T) {
	t.Parallel()
	data := templateData{
		Slug:            "auth-change",
		InitialRequest:  "update auth middleware",
		GuardrailDomain: "auth_authz",
	}
	docs := DocSections{
		Scope: "- update auth middleware",
	}

	result := seededRequirementsContent(data, docSectionItems(docs.Scope))

	assert.Contains(t, result, "REQ-001")
	assert.Contains(t, result, "REQ-002")
	assert.Contains(t, result, "auth_authz")
}

func TestSeedRequirementsUsesConservativeFallbackForNounPhraseAndDoesNotInventIntentIDs(t *testing.T) {
	t.Parallel()
	data := templateData{
		Slug:           "session-timeout",
		InitialRequest: "session timeout",
	}

	result := seedRequirements(data)

	assert.Contains(t, result, "REQ-001")
	assert.Contains(t, result, "session timeout")
	assert.NotContains(t, result, "The system MUST session timeout.")
	assert.NotContains(t, result, "Traces to INT-001.")
}

func TestSeedTasksContainsOnlyStructuralPlaceholder(t *testing.T) {
	t.Parallel()
	data := templateData{
		Slug:           "my-change",
		InitialRequest: "fix login timeout",
	}
	result := seedTasks(data)
	assert.Contains(t, result, "t-01")
	assert.Contains(t, result, "Pending task objective")
	assert.Contains(t, result, "task_kind: investigation")
	assert.NotContains(t, result, "t-02")
	assert.NotContains(t, result, "task_kind: test")
}

func TestCapitalizeFirstHandlesUnicodePrefix(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Éclair", capitalizeFirst("éclair"))
}

func TestScaffoldWithDocSectionsEnrichesRequirements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("session-timeout")
	change.Description = "session timeout"

	docs := DocSections{
		Scope:      "- expire idle sessions after 15 minutes",
		Acceptance: "- sessions expire after 15 minutes of inactivity",
	}
	projectCtx := model.ProjectContext{}

	require.NoError(t, ScaffoldGovernedBundleForChangeWithContextAndDocs(root, change, "", projectCtx, docs))

	raw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "requirements.md"))
	require.NoError(t, err)
	content := string(raw)
	assert.Contains(t, content, "15 minutes")
	assert.Contains(t, content, "REQ-001")
}

func TestScaffoldWithDocSectionsCreatesRequirementBlockPerScopeItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("session-timeout")
	change.Description = "session timeout"

	docs := DocSections{
		Scope: strings.Join([]string{
			"- expire idle sessions after 15 minutes",
			"- preserve MFA enforcement for admin sessions",
		}, "\n"),
	}

	require.NoError(t, ScaffoldGovernedBundleForChangeWithContextAndDocs(root, change, "", model.ProjectContext{}, docs))

	raw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "requirements.md"))
	require.NoError(t, err)

	blocks := ParseRequirementBlocks(string(raw))
	require.Len(t, blocks, 2)
	assert.Equal(t, "REQ-001", blocks[0].StableID)
	assert.Equal(t, "REQ-002", blocks[1].StableID)
}

func TestScaffoldWithDocSectionsTreatsWrappedProseScopeAsSingleRequirement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("cache-timeout-update")
	change.Description = "cache timeout update"

	docs := DocSections{
		Scope: "expire idle cache entries after 15 minutes while preserving middleware contracts\nfor existing auth flows.",
	}

	require.NoError(t, ScaffoldGovernedBundleForChangeWithContextAndDocs(root, change, "", model.ProjectContext{}, docs))

	requirementsRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "requirements.md"))
	require.NoError(t, err)
	blocks := ParseRequirementBlocks(string(requirementsRaw))
	require.Len(t, blocks, 1)
	assert.NotContains(t, string(requirementsRaw), "REQ-002: The system MUST for existing auth flows.")

	tasksRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "tasks.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(tasksRaw), "`t-02` For existing auth flows.")
}

func TestScaffoldWithDocSectionsTreatsMultiParagraphScopeAsSingleRequirement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("session-timeout")
	change.Description = "session timeout"

	docs := DocSections{
		Scope: strings.Join([]string{
			"expire idle sessions after 15 minutes while keeping middleware contracts stable.",
			"",
			"Preserve MFA enforcement for admin sessions throughout the change.",
		}, "\n"),
	}

	require.NoError(t, ScaffoldGovernedBundleForChangeWithContextAndDocs(root, change, "", model.ProjectContext{}, docs))

	requirementsRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "requirements.md"))
	require.NoError(t, err)
	blocks := ParseRequirementBlocks(string(requirementsRaw))
	require.Len(t, blocks, 1)
	assert.Contains(t, strings.ToLower(string(requirementsRaw)), "preserve mfa enforcement")

	tasksRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "tasks.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(tasksRaw), "`t-02` Preserve MFA enforcement for admin sessions throughout the change.")
}

func TestScaffoldWithDocSectionsTreatsNestedScopeBulletsAsSingleTopLevelRequirement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("session-timeout")
	change.Description = "session timeout"

	docs := DocSections{
		Scope: strings.Join([]string{
			"- update session timeout handling",
			"  - expire idle sessions after 15 minutes",
			"  - preserve MFA enforcement for admin sessions",
		}, "\n"),
	}

	require.NoError(t, ScaffoldGovernedBundleForChangeWithContextAndDocs(root, change, "", model.ProjectContext{}, docs))

	requirementsRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "requirements.md"))
	require.NoError(t, err)
	blocks := ParseRequirementBlocks(string(requirementsRaw))
	require.Len(t, blocks, 1)
	assert.Contains(t, string(requirementsRaw), "15 minutes")
	assert.Contains(t, strings.ToLower(string(requirementsRaw)), "preserve mfa enforcement")

	tasksRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "tasks.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(tasksRaw), "`t-02` Expire idle sessions after 15 minutes")
	assert.NotContains(t, string(tasksRaw), "`t-03` Preserve MFA enforcement for admin sessions")
}

func TestScaffoldWithDocSectionsPreservesConstraintPreambleAlongsideListItems(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("session-timeout")
	change.Description = "session timeout"

	docs := DocSections{
		Constraints: strings.Join([]string{
			"Preserve the existing session revocation semantics.",
			"- keep existing middleware contract",
		}, "\n"),
	}

	require.NoError(t, ScaffoldGovernedBundleForChangeWithContextAndDocs(root, change, "", model.ProjectContext{}, docs))

	decisionRaw, err := os.ReadFile(filepath.Join(root, "artifacts", "changes", change.Slug, "decision.md"))
	require.NoError(t, err)
	selectedApproach := strings.TrimSpace(strings.Join(markdownSectionLines(string(decisionRaw), "Selected Approach"), "\n"))

	assert.Contains(t, selectedApproach, "Preserve the existing session revocation semantics.")
	assert.Contains(t, selectedApproach, "keep existing middleware contract")
	assert.Contains(t, selectedApproach, "This direction must continue honoring the documented constraints:")
}

func TestParseDecisionLockedDecisionsIgnoresScaffoldDraftSelectedDirection(t *testing.T) {
	t.Parallel()

	data := templateData{InitialRequest: "update auth middleware timeout strategy"}
	content := `# Decision

## Alternatives Considered
` + seedDecision(data) + `

## Selected Approach
` + seedDecisionApproach(data) + `

## Risk
` + seedDecisionRisk(data) + `
`

	assert.Nil(t, ParseDecisionLockedDecisions(content))
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

	err := ScaffoldGovernedBundleForChangeWithPreset(root, model.NewChange("test-change"), "", customSchema)
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

	err := ScaffoldGovernedBundleForChangeWithPreset(root, model.NewChange("test-change"), "", customSchema)
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

	reqPath := ResolveArtifactPath(base, slug, "requirements.md")
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
	reqPath := ResolveArtifactPath(base, slug, "requirements.md")
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
		CurrentState: model.StateS2Execute,
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
