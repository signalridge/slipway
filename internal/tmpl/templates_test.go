package tmpl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentReturnsGovernanceSkills(t *testing.T) {
	t.Parallel()
	// Static governance skills (loaded via Content)
	staticSkills := []string{
		"skills/research-orchestration/SKILL.md",
		"skills/plan-audit/SKILL.md",
		"skills/tdd-governance/SKILL.md",
	}
	for _, name := range staticSkills {
		content, err := Content(name)
		require.NoError(t, err, "failed to load %s", name)
		assert.Contains(t, content, "## Purpose", "%s missing Purpose section", name)
		assert.Contains(t, content, "## DO NOT SKIP", "%s missing DO NOT SKIP section", name)
		assert.Contains(t, content, "<HARD-GATE>", "%s missing HARD-GATE tag", name)
		assert.NotContains(t, content, "TodoWrite", "%s still references TodoWrite", name)
	}
	// Templated governance skills (loaded via Render)
	templatedSkills := []string{
		"skills/spec-compliance-review/SKILL.md.tmpl",
		"skills/code-quality-review/SKILL.md.tmpl",
		"skills/goal-verification/SKILL.md.tmpl",
		"skills/final-closeout/SKILL.md.tmpl",
	}
	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}
	for _, name := range templatedSkills {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		assert.Contains(t, content, "## Purpose", "%s missing Purpose section", name)
		assert.Contains(t, content, "## DO NOT SKIP", "%s missing DO NOT SKIP section", name)
		assert.Contains(t, content, "<HARD-GATE>", "%s missing HARD-GATE tag", name)
		assert.NotContains(t, content, "TodoWrite", "%s still references TodoWrite", name)
	}
}

func TestRequirementsQualityChecklistSidecarExistsAndIsReferenced(t *testing.T) {
	t.Parallel()

	checklist, err := Content("skills/checklist-quality.md")
	require.NoError(t, err)
	assert.Contains(t, checklist, "Requirement-to-intent traceability")

	planAudit, err := Content("skills/plan-audit/SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, planAudit, "checklist-quality.md")

	specCompliance, err := Render("skills/spec-compliance-review/SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:test",
		"Description": "test",
	})
	require.NoError(t, err)
	assert.Contains(t, specCompliance, "checklist-quality.md")
}

func TestPlanAuditTemplateDoesNotReintroduceLightPresetVerificationBlocker(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/plan-audit/SKILL.md")
	require.NoError(t, err)

	assert.Contains(t, content, "On light preset, dimension #1 (coverage) failures are advisory warnings; all other dimension failures remain blockers.")
	assert.NotContains(t, content, "Every task needs explicit per-task verification fields before execution begins.")
}

func TestFinalCloseoutTemplateKeepsAssuranceReferenceConditional(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/final-closeout/SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:final-closeout",
		"Description": "test",
	})
	require.NoError(t, err)

	assert.Contains(t, content, "On standard/strict preset, also add `closeout:assurance_complete=pass`.")
	assert.NotContains(t, content, "\n  - \"closeout:assurance_complete=pass\"\n")
}

func TestCoreGovernanceSkillsIncludeGraphvizWorkflow(t *testing.T) {
	t.Parallel()
	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}

	// Static governance skills with Graphviz
	for _, name := range []string{
		"skills/plan-audit/SKILL.md",
	} {
		content, err := Content(name)
		require.NoError(t, err, "failed to load %s", name)
		assert.Contains(t, content, "## Workflow Graph (Graphviz DOT)", "%s missing Graphviz workflow section", name)
		assert.Contains(t, content, "```dot", "%s missing DOT code block", name)
		assert.Contains(t, content, "digraph", "%s missing Graphviz digraph definition", name)
	}

	// Templated governance skills with Graphviz
	for _, name := range []string{
		"skills/goal-verification/SKILL.md.tmpl",
	} {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		assert.Contains(t, content, "## Workflow Graph (Graphviz DOT)", "%s missing Graphviz workflow section", name)
		assert.Contains(t, content, "```dot", "%s missing DOT code block", name)
		assert.Contains(t, content, "digraph", "%s missing Graphviz digraph definition", name)
	}

	// Templated governance skills with Graphviz (wave-orchestration)
	data = map[string]string{"ToolID": "claude", "Trigger": "/slipway:wave-orchestration"}
	content, err := Render("skills/wave-orchestration/SKILL.md.tmpl", data)
	require.NoError(t, err, "failed to render wave-orchestration/SKILL.md.tmpl")
	assert.Contains(t, content, "## Workflow Graph (Graphviz DOT)")
	assert.Contains(t, content, "```dot")
	assert.Contains(t, content, "digraph")
}

func TestContentReturnsTechniques(t *testing.T) {
	t.Parallel()
	techniques := []string{
		"skills/tdd/SKILL.md",
		"skills/systematic-debugging/SKILL.md",
		"skills/code-review-protocol/SKILL.md",
		"skills/codebase-mapping/SKILL.md",
	}
	for _, name := range techniques {
		content, err := Content(name)
		require.NoError(t, err, "failed to load %s", name)
		assert.Contains(t, content, "## Purpose", "%s missing Purpose section", name)
	}
}

func TestContentReturnsStandaloneSkills(t *testing.T) {
	t.Parallel()
	standalone := []string{
		"skills/worktree-preflight/SKILL.md",
	}
	for _, name := range standalone {
		content, err := Content(name)
		require.NoError(t, err, "failed to load %s", name)
		assert.Contains(t, content, "## Purpose", "%s missing Purpose section", name)
	}
}

func TestCodebaseMappingTemplateDefinesDurableDocumentSet(t *testing.T) {
	t.Parallel()
	content, err := Content("skills/codebase-mapping/SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, content, "input_context.codebase_map_dir")
	assert.Contains(t, content, "artifacts/codebase/STACK.md")
	assert.Contains(t, content, "artifacts/codebase/ARCHITECTURE.md")
	assert.Contains(t, content, "artifacts/codebase/TESTING.md")
	assert.Contains(t, content, "artifacts/codebase/CONCERNS.md")
}

func TestPlanningAndDiscoveryTemplatesConsumeDurableCodebaseMap(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"skills/research-orchestration/SKILL.md",
		"skills/plan-audit/SKILL.md",
	} {
		content, err := Content(name)
		require.NoError(t, err)
		assert.Contains(t, content, "input_context.codebase_map_dir", "%s missing durable codebase map reference", name)
	}

	// Templated governance skills
	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:wave-orchestration"}
	content, err := Render("skills/wave-orchestration/SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, content, "input_context.codebase_map_dir", "wave-orchestration/SKILL.md.tmpl missing durable codebase map reference")
}

func TestContentReturnsArtifactTemplates(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"assurance.md",
		"decision.md",
		"intent.md",
		"research.md",
		"requirements.md",
		"tasks.md",
	} {
		path := "artifacts/" + name
		content, err := Content(path)
		require.NoError(t, err, "failed to load %s", path)
		assert.NotEmpty(t, content, "%s is empty", path)
	}
}

func TestRenderAdapterSkillTemplates(t *testing.T) {
	t.Parallel()
	templates := []string{
		"skills/init/SKILL.md.tmpl",
		"skills/new/SKILL.md.tmpl",
		"skills/next/SKILL.md.tmpl",
		"skills/status/SKILL.md.tmpl",
		"skills/done/SKILL.md.tmpl",
		"skills/cancel/SKILL.md.tmpl",
		"skills/preset/SKILL.md.tmpl",
		"skills/review/SKILL.md.tmpl",
		"skills/run/SKILL.md.tmpl",
		"skills/validate/SKILL.md.tmpl",
		"skills/validate-requirements/SKILL.md.tmpl",
		"skills/pivot/SKILL.md.tmpl",
		"skills/abort/SKILL.md.tmpl",
		"skills/repair/SKILL.md.tmpl",
		"skills/checkpoint/SKILL.md.tmpl",
		"skills/wave-orchestration/SKILL.md.tmpl",
	}
	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:do",
		"Description": "Example adapter skill description",
	}
	for _, name := range templates {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		assert.NotContains(t, content, "{{.", "%s has unrendered template vars", name)
	}
}

func TestAdapterSkillTemplateFrontmatterIncludesDescription(t *testing.T) {
	t.Parallel()
	templates := []string{
		"skills/init/SKILL.md.tmpl",
		"skills/new/SKILL.md.tmpl",
		"skills/next/SKILL.md.tmpl",
		"skills/status/SKILL.md.tmpl",
		"skills/done/SKILL.md.tmpl",
		"skills/cancel/SKILL.md.tmpl",
		"skills/preset/SKILL.md.tmpl",
		"skills/review/SKILL.md.tmpl",
		"skills/run/SKILL.md.tmpl",
		"skills/validate/SKILL.md.tmpl",
		"skills/validate-requirements/SKILL.md.tmpl",
		"skills/pivot/SKILL.md.tmpl",
		"skills/abort/SKILL.md.tmpl",
		"skills/repair/SKILL.md.tmpl",
		"skills/checkpoint/SKILL.md.tmpl",
	}
	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:do",
		"Description": "Example adapter skill description",
	}
	for _, name := range templates {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		parts := strings.SplitN(content, "---", 3)
		require.Len(t, parts, 3, "%s missing frontmatter delimiters", name)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name in frontmatter", name)
		assert.Contains(t, fm, "description:", "%s missing description in frontmatter", name)
		assert.Contains(t, fm, "tool:", "%s missing tool in frontmatter", name)
	}
}

func TestRenderCommandEntryTemplate(t *testing.T) {
	t.Parallel()
	data := map[string]string{
		"CommandID":   "status",
		"ToolID":      "cursor",
		"Trigger":     "/slipway-status",
		"Description": "Show lifecycle status and blockers",
		"SkillPath":   ".cursor/skills/slipway/status/SKILL.md",
		"Arguments":   "--json",
	}
	content, err := Render("commands/command-entry.md.tmpl", data)
	require.NoError(t, err, "failed to render command-entry.md.tmpl")
	assert.NotContains(t, content, "{{.", "command-entry has unrendered template vars")
}

func TestRenderSessionStartHookTemplate(t *testing.T) {
	t.Parallel()
	data := map[string]string{
		"ToolID": "claude",
	}
	content, err := Render("hooks/session-start.sh.tmpl", data)
	require.NoError(t, err, "failed to render session-start.sh.tmpl")
	assert.NotContains(t, content, "{{.", "session-start hook has unrendered template vars")
	assert.Contains(t, content, "slipway next --json --preview --hook-lite")
	assert.NotContains(t, content, "slipway next --preview --context-guard")
}

func TestContentReturnsAgentDefinitions(t *testing.T) {
	t.Parallel()
	for _, name := range AgentNames() {
		path := "agents/" + name + ".md"
		content, err := Content(path)
		require.NoError(t, err, "failed to load %s", path)
		assert.NotEmpty(t, content, "%s is empty", path)
		assert.Contains(t, content, "---", "%s missing frontmatter", path)
		assert.Contains(t, content, "name: "+name, "%s missing name field", path)
		assert.Contains(t, content, "agent_status:", "%s missing agent_status field", path)
	}
}

func TestPlannerAgentIncludesIntakeClarificationBinding(t *testing.T) {
	t.Parallel()

	content, err := Content("agents/slipway-planner.md")
	require.NoError(t, err)
	assert.Contains(t, content, "intake-clarification")
}

func TestAgentNamesMatchesFiles(t *testing.T) {
	t.Parallel()
	names := AgentNames()
	assert.Len(t, names, 11, "expected 11 agent definitions")
	// Verify sorted
	for i := 1; i < len(names); i++ {
		assert.True(t, names[i-1] < names[i], "AgentNames not sorted: %s >= %s", names[i-1], names[i])
	}
}

func TestGovernanceSkillFrontmatterMinimal(t *testing.T) {
	t.Parallel()
	// Static governance skills
	staticSkills := []string{
		"skills/worktree-preflight/SKILL.md",
		"skills/plan-audit/SKILL.md",
		"skills/tdd-governance/SKILL.md",
		"skills/research-orchestration/SKILL.md",
	}
	routingFields := []string{
		"required_levels:", "state:", "type:", "skill_name:",
		"guardrail_required:", "closeout_conditional:",
		"reviewer_independent:", "run_summary_bound:", "mitigation_target:",
	}
	for _, name := range staticSkills {
		content, err := Content(name)
		require.NoError(t, err, "failed to load %s", name)
		parts := strings.SplitN(content, "---", 3)
		require.Len(t, parts, 3, "%s missing frontmatter delimiters", name)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name in frontmatter", name)
		assert.Contains(t, fm, "description:", "%s missing description in frontmatter", name)
		for _, field := range routingFields {
			assert.NotContains(t, fm, field, "%s frontmatter contains routing field %s", name, field)
		}
	}
	// Templated governance skills (converted from static)
	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}
	for _, name := range []string{
		"skills/spec-compliance-review/SKILL.md.tmpl",
		"skills/code-quality-review/SKILL.md.tmpl",
		"skills/goal-verification/SKILL.md.tmpl",
		"skills/final-closeout/SKILL.md.tmpl",
	} {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		parts := strings.SplitN(content, "---", 3)
		require.Len(t, parts, 3, "%s missing frontmatter delimiters", name)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name in frontmatter", name)
		assert.Contains(t, fm, "description:", "%s missing description in frontmatter", name)
		for _, field := range routingFields {
			assert.NotContains(t, fm, field, "%s frontmatter contains routing field %s", name, field)
		}
	}
}

func TestGovernanceTemplatedSkillFrontmatterMinimal(t *testing.T) {
	t.Parallel()
	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:wave-orchestration"}
	content, err := Render("skills/wave-orchestration/SKILL.md.tmpl", data)
	require.NoError(t, err)
	parts := strings.SplitN(content, "---", 3)
	require.Len(t, parts, 3, "missing frontmatter delimiters")
	fm := parts[1]
	assert.Contains(t, fm, "name:", "missing name in frontmatter")
	assert.Contains(t, fm, "description:", "missing description in frontmatter")
	assert.Contains(t, fm, "tool:", "missing tool in frontmatter")
	for _, field := range []string{"required_levels:", "state:", "type:", "mitigation_target:", "run_summary_bound:"} {
		assert.NotContains(t, fm, field, "frontmatter contains routing field %s", field)
	}
}

func TestTechniqueSkillFrontmatterMinimal(t *testing.T) {
	t.Parallel()
	techniques := []string{
		"skills/tdd/SKILL.md",
		"skills/systematic-debugging/SKILL.md",
		"skills/code-review-protocol/SKILL.md",
		"skills/codebase-mapping/SKILL.md",
	}
	for _, name := range techniques {
		content, err := Content(name)
		require.NoError(t, err, "failed to load %s", name)
		parts := strings.SplitN(content, "---", 3)
		require.Len(t, parts, 3, "%s missing frontmatter delimiters", name)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name in frontmatter", name)
		assert.Contains(t, fm, "description:", "%s missing description in frontmatter", name)
		assert.NotContains(t, fm, "type:", "%s frontmatter contains type field", name)
	}
}

func TestStandaloneSkillFrontmatterMinimal(t *testing.T) {
	t.Parallel()
	content, err := Content("skills/worktree-preflight/SKILL.md")
	require.NoError(t, err, "failed to load worktree-preflight")
	parts := strings.SplitN(content, "---", 3)
	require.Len(t, parts, 3, "skills/worktree-preflight/SKILL.md missing frontmatter delimiters")
	fm := parts[1]
	assert.Contains(t, fm, "name:", "skills/worktree-preflight/SKILL.md missing name in frontmatter")
	assert.Contains(t, fm, "description:", "skills/worktree-preflight/SKILL.md missing description in frontmatter")
}

func TestEntrySurfaceTemplatesAvoidPlanOnlyVocabulary(t *testing.T) {
	t.Parallel()

	newSkill, err := Render("skills/new/SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:new",
		"Description": "Create a governed change with intake-first workflow",
	})
	require.NoError(t, err)

	for name, content := range map[string]string{
		"new": newSkill,
	} {
		assert.NotContains(t, content, "plan-only", "%s template reintroduced retired plan-only wording", name)
	}
}

func TestWorkflowStateTemplatesAvoidRetiredIntakeVocabulary(t *testing.T) {
	t.Parallel()

	researchSkill, err := Content("skills/research-orchestration/SKILL.md")
	require.NoError(t, err)

	for name, content := range map[string]string{
		"research-orchestration": researchSkill,
	} {
		assert.NotContains(t, content, "unknowns from intake", "%s reintroduced retired intake wording", name)
		assert.NotContains(t, content, "requested at intake", "%s reintroduced retired intake wording", name)
		assert.NotContains(t, content, "flagged during intake", "%s reintroduced retired intake wording", name)
		assert.NotContains(t, content, "from intake", "%s should not describe live workflow inputs as intake-derived", name)
	}
}

func TestAgentDefinitionFrontmatterNoRoutingFields(t *testing.T) {
	t.Parallel()
	for _, name := range AgentNames() {
		path := "agents/" + name + ".md"
		content, err := Content(path)
		require.NoError(t, err, "failed to load %s", path)
		parts := strings.SplitN(content, "---", 3)
		require.Len(t, parts, 3, "%s missing frontmatter delimiters", path)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name in frontmatter", path)
		assert.Contains(t, fm, "description:", "%s missing description in frontmatter", path)
		assert.NotContains(t, fm, "state:", "%s frontmatter contains state field", path)
		assert.NotContains(t, fm, "type:", "%s frontmatter contains type field", path)
	}
}

func TestPartialsAreAvailableInRender(t *testing.T) {
	t.Parallel()
	// Render a governance skill that uses {{template "hard-gate"}} and verify
	// the partial content appears in the output.
	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:spec-compliance-review",
		"Description": "test",
	}
	content, err := Render("skills/spec-compliance-review/SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, content, "<HARD-GATE>", "hard-gate partial should render into governance skill")
	assert.Contains(t, content, "Do not call `slipway next` until the user approves.", "hard-gate partial content missing")
}

func TestPartialsDeduplicateGovernanceContent(t *testing.T) {
	t.Parallel()
	// Verify banned-language partial renders in goal-verification.
	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:goal-verification",
		"Description": "test",
	}
	content, err := Render("skills/goal-verification/SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, content, `"should work"`, "banned-language partial should render into goal-verification")
	assert.Contains(t, content, "opinions, not evidence", "banned-language partial content missing")

	// Verify the same partial renders identically in final-closeout.
	data["Trigger"] = "/slipway:final-closeout"
	content2, err := Render("skills/final-closeout/SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, content2, `"should work"`, "banned-language partial should render into final-closeout")
}

func TestRunSkillContainsLoopBehavioralBlocks(t *testing.T) {
	t.Parallel()
	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:run",
		"Description": "Advance governed execution until a skill, blocker, checkpoint, or done-ready outcome is surfaced",
	}
	content, err := Render("skills/run/SKILL.md.tmpl", data)
	require.NoError(t, err)

	assert.Contains(t, content, "context_budget.health",
		"run skill missing context self-monitoring block")

	assert.Contains(t, content, "fresh reviewer agent",
		"run skill missing fresh-reviewer pause mandate")

	assert.Contains(t, content, "Subagent Continuation Rule (HARD RULE)",
		"run skill missing subagent continuation hard rule")

	assert.Contains(t, content, "three consecutive skills fail",
		"run skill missing 3-consecutive-failure exit rule")

	assert.Contains(t, content, "user_response_payload",
		"run skill missing checkpoint response handoff guidance")
}

func TestWaveOrchestrationSkillIncludesCheckpointResponseGuidance(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/wave-orchestration/SKILL.md.tmpl", map[string]string{
		"ToolID":  "claude",
		"Trigger": "/slipway:wave-orchestration",
	})
	require.NoError(t, err)

	assert.Contains(t, content, "user_response_payload",
		"wave-orchestration skill missing checkpoint response guidance")
	assert.Contains(t, content, "checkpoint_type",
		"wave-orchestration skill missing checkpoint type guidance")
}

func TestContentNotFound(t *testing.T) {
	t.Parallel()
	_, err := Content("nonexistent.md")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "nonexistent.md"))
}

func TestRenderNotFound(t *testing.T) {
	t.Parallel()
	_, err := Render("nonexistent.md.tmpl", nil)
	require.Error(t, err)
}
