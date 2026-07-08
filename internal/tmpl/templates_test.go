package tmpl

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentReturnsGovernanceSkills(t *testing.T) {
	t.Parallel()
	// Static governance skills (loaded via Content)
	staticSkills := []string{
		"skills/research-orchestration/HOST_SKILL.md",
		"skills/plan-audit/HOST_SKILL.md",
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
		"skills/tdd-governance/HOST_SKILL.md.tmpl",
		"skills/spec-compliance-review/HOST_SKILL.md.tmpl",
		"skills/code-quality-review/HOST_SKILL.md.tmpl",
		"skills/independent-review/HOST_SKILL.md.tmpl",
		"skills/security-review/HOST_SKILL.md.tmpl",
		"skills/ship-verification/HOST_SKILL.md.tmpl",
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

func TestSkillSourceEntrypointsUseRoleSpecificNames(t *testing.T) {
	t.Parallel()

	err := fs.WalkDir(TemplateFS(), "skills", func(name string, d fs.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)
		if d.IsDir() {
			return nil
		}
		assert.NotContains(t,
			[]string{"SKILL.md", "SKILL.md.tmpl"},
			path.Base(name),
			"%s must use HOST_SKILL or CATALOG_SKILL source naming; generated adapter entries remain SKILL.md",
			name,
		)
		return nil
	})
	require.NoError(t, err)
}

func TestRequirementsQualityChecklistSidecarExistsAndIsReferenced(t *testing.T) {
	t.Parallel()

	checklist, err := Content("skills/_shared/references/checklist-quality.md")
	require.NoError(t, err)
	flatChecklist := strings.Join(strings.Fields(checklist), " ")
	assert.Contains(t, checklist, "Requirement-to-intent traceability")
	assert.Contains(t, checklist, "## Generated Skill Template Quality")
	assert.Contains(t, checklist, "Use this section only when editing generated Slipway skill templates under")
	assert.Contains(t, checklist, "`internal/tmpl/templates/skills/`; it is not a general prompt-writing manual.")
	assert.Contains(t, flatChecklist, "Start steps with familiar action words such as read, run, write, record, or stop unless a Slipway term is itself the contract.")
	assert.Contains(t, flatChecklist, "Keep context pointers reliable: say when the agent should read referenced material, and keep must-have contract details inline when a pointer would be easy to miss.")
	assert.Contains(t, checklist, "Make completion criteria checkable")
	assert.Contains(t, checklist, "Prune no-op prose")
	assert.Contains(t, flatChecklist, "preserving contract tokens such as `next_skill.name`, `verification_dir`, reason codes, command names, and evidence paths.")

	planAudit, err := Content("skills/plan-audit/HOST_SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, planAudit, "checklist-quality.md")

	specCompliance, err := Render("skills/spec-compliance-review/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:test",
		"Description": "test",
	})
	require.NoError(t, err)
	assert.Contains(t, specCompliance, "checklist-quality.md")
}

func TestWorkflowTemplatePinsRuntimeSessionHandoffContract(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/workflow/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway",
		"Description": "Governed workflow entry",
	})
	require.NoError(t, err)
	flat := strings.Join(strings.Fields(content), " ")

	assert.Contains(t, content, "## Session Continuity")
	// Slipway no longer measures host context; the host owns the timing decision.
	assert.Contains(t, flat, "Slipway does not measure your context window")
	assert.Contains(t, flat, "the timing decision is yours")
	// The handoff mechanism is present and host-invoked, but stays advisory.
	assert.Contains(t, flat, "slipway handoff write")
	assert.Contains(t, flat, "slipway handoff show")
	assert.Contains(t, flat, "You own when to call them.")
	assert.Contains(t, flat, "it is NOT lifecycle authority, governed evidence, freshness input, or a gate")
	// Lifecycle authority still comes only from status/next, never from the prose.
	assert.Contains(t, flat, "Governed continuity comes ONLY from `slipway status --json` and `slipway next --json`")
	assert.Contains(t, flat, "The host's own memory, compaction summary, or progress notes are informal context only and are NEVER governance authority.")
	assert.Contains(t, flat, "MUST run `slipway status` / `slipway next` and MUST NOT infer the governed position from a host summary")
}

func TestHandoffGuidanceDoesNotBecomeLifecycleAuthority(t *testing.T) {
	t.Parallel()

	workflow, err := Render("skills/workflow/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway",
		"Description": "Governed workflow entry",
	})
	require.NoError(t, err)
	runCommand, err := Render("commands/command-entry.md.tmpl", map[string]string{
		"CommandID":    "run",
		"ToolID":       "claude",
		"Trigger":      "/slipway:run",
		"Description":  "Advance governed execution until a skill, blocker, or done-ready outcome is surfaced",
		"BodyTemplate": "command-run-body",
		"Arguments":    "--json",
	})
	require.NoError(t, err)

	for name, content := range map[string]string{
		"workflow":    workflow,
		"run command": runCommand,
	} {
		flat := strings.Join(strings.Fields(content), " ")
		assert.NotContains(t, flat, "handoff.md is lifecycle authority", name)
		assert.NotContains(t, flat, "handoff.md is governed evidence", name)
		assert.NotContains(t, flat, "handoff.md is a freshness input", name)
		assert.NotContains(t, flat, "handoff.md is a gate", name)
		assert.NotContains(t, flat, "handoff.md selects the governed host skill", name)
		assert.NotContains(t, flat, "derive the governed host from the handoff", name)
		assert.NotContains(t, flat, "use handoff.md as evidence", name)
		assert.NotContains(t, flat, "use handoff.md for freshness", name)
	}

	flatRun := strings.Join(strings.Fields(runCommand), " ")
	// The run body surfaces the advisory handoff-write path (host owns the timing),
	// but must keep it explicitly advisory and must not resurrect the retired
	// context-pressure self-monitoring block.
	assert.Contains(t, flatRun, "slipway handoff write")
	assert.Contains(t, flatRun, "The handoff is advisory only; it does not replace `slipway status --json`, `slipway next --json`, lifecycle gates, freshness, or evidence checks.")
	assert.NotContains(t, flatRun, "## Context Self-Monitoring")
}

func TestDecisionTemplatePinsSupersessionGuidance(t *testing.T) {
	t.Parallel()

	content, err := Content("artifacts/decision.md")
	require.NoError(t, err)
	flat := strings.Join(strings.Fields(content), " ")

	assert.Contains(t, flat, "When a later decision replaces earlier guidance, keep the old guidance reviewable")
	assert.Contains(t, flat, "mark it as superseded by the concrete replacement decision or section in this file.")
	assert.NotContains(t, flat, "delete the old guidance")
	assert.NotContains(t, flat, "rewrite earlier guidance in place")
}

func TestPlanAuditTemplateDoesNotReintroduceLightPresetVerificationBlocker(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/plan-audit/HOST_SKILL.md")
	require.NoError(t, err)

	assert.Contains(t, content, "On light, only dimension #1")
	assert.NotContains(t, content, "Every task needs explicit per-task verification fields before execution begins.")
}

// TestCodebaseMapRelevanceGuidanceInSkills pins issue #80: the durable-map
// consumer skills carry the populated/partial relevance self-check, the stale
// "no whole-map advisory" prose is gone, and the reference defines staleness as a
// host-AI semantic relevance judgment rather than the rejected git-mtime/lockfile
// fingerprint heuristics. Guards against a future `init --refresh` regressing it.
func TestCodebaseMapRelevanceGuidanceInSkills(t *testing.T) {
	t.Parallel()

	// Collapse line-wrapping whitespace so multi-word phrases match regardless of
	// where the 79-column prose wrap falls.
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }

	for _, path := range []string{
		"skills/research-orchestration/HOST_SKILL.md",
		"skills/plan-audit/HOST_SKILL.md",
	} {
		content, err := Content(path)
		require.NoError(t, err, path)
		flat := norm(content)
		assert.Contains(t, flat, "not scope relevance", path)
		assert.Contains(t, flat, "Populated is not the same as relevant", path)
		assert.NotContains(t, flat, "no whole-map advisory", path)
		// The advisory fires for partial too, so the prose must name partial
		// explicitly and route it to per-doc inspection (engine code at
		// codebaseMapRelevanceAdvisory fires for populated AND partial).
		assert.Contains(t, flat, "fires for `populated` and `partial`", path)
		assert.Contains(t, flat, "For a `partial` map, also inspect", path)
	}

	// research-orchestration must not lump "stale" into the run-the-command path:
	// a semantically stale populated/partial map is re-authored inline, not
	// regenerated (the command only scaffolds a missing/non-durable set).
	research, err := Content("skills/research-orchestration/HOST_SKILL.md")
	require.NoError(t, err)
	flatResearch := norm(research)
	assert.Contains(t, flatResearch, "do not rerun")
	assert.NotContains(t, flatResearch, "missing or stale, run the")

	// wave-orchestration (rendered) is a durable-map consumer and must carry the
	// relevance self-check — the exact handoff issue #80 reproduces.
	wave, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:wave-orchestration",
		"Description": "test",
	})
	require.NoError(t, err)
	flatWave := norm(wave)
	assert.Contains(t, flatWave, "not scope relevance")
	assert.Contains(t, flatWave, "Populated is not the same as relevant")
	assert.Contains(t, flatWave, "fires for `populated` and `partial`")
	assert.Contains(t, flatWave, "For a `partial` map, also inspect")

	mapping, err := Content("skills/codebase-mapping/HOST_SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, norm(mapping), "re-author the change-relevant documents in place")

	// The reference defines staleness as host-AI semantic relevance, not the
	// rejected fingerprint heuristics, and no longer routes stale populated docs
	// to the `slipway codebase-map` no-op.
	ref, err := Content("skills/context-assembly/references/codebase-map.md")
	require.NoError(t, err)
	flatRef := norm(ref)
	assert.Contains(t, flatRef, "host-AI semantic relevance judgment")
	assert.NotContains(t, flatRef, "git mtime on the matching directory")
	assert.NotContains(t, flatRef, "do not match the lockfile")
}

func TestShipVerificationTemplateRequiresAssuranceAttestationOnStandardStrict(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/ship-verification/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:ship-verification",
		"Description": "test",
	})
	require.NoError(t, err)

	// The attestation token keeps its `closeout:` prefix and is part of the
	// ship-verification references list, described as required on standard/strict;
	// the ship gate enforces it via the canonical ship_verification_assurance_attestation_missing
	// reason code. Light preset still omits it.
	assert.Contains(t, content, `- "closeout:assurance_complete=pass"`)
	assert.Contains(t, content, "`closeout:assurance_complete=pass` is REQUIRED")
	assert.Contains(t, content, "ship_verification_assurance_attestation_missing")
	assert.Contains(t, content, "On light preset, omit it")
}

func TestSpecComplianceReviewTemplateEmitsReviewContextOriginHandle(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/spec-compliance-review/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:spec-compliance-review",
		"Description": "test",
	})
	require.NoError(t, err)

	// All S3 reviewers record the same review-stage origin grammar. The handle
	// identifies the native subagent that performed this specific review.
	assert.Contains(t, content, "context_origin:stage=review=<handle>")
	assert.Contains(t, content, "MUST be DISTINCT")
	assertSelectedS3ReviewPeerSet(t, content)
	// The retired review_origin grammar must be gone from the review template.
	assert.NotContains(t, content, "review_origin:skill=")
	// The colliding next/handoff JSON name must never be emitted as the token.
	assert.NotContains(t, content, "review_context:skill=")
}

func TestCodeQualityReviewTemplateEmitsReviewContextOriginHandle(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/code-quality-review/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:code-quality-review",
		"Description": "test",
	})
	require.NoError(t, err)

	assert.Contains(t, content, "context_origin:stage=review=<handle>")
	assert.Contains(t, content, "MUST be DISTINCT")
	assertSelectedS3ReviewPeerSet(t, content)
	assert.NotContains(t, content, "review_origin:skill=")
	assert.NotContains(t, content, "review_context:skill=")
}

func TestCodeQualityReviewTemplateRequiresConfiguredGoLint(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/code-quality-review/HOST_SKILL.md.tmpl", nil)
	require.NoError(t, err)

	assert.Contains(t, content, "`golangci-lint run ./...`")
	assert.Contains(t, content, "treat new lint findings as blockers")
}

func assertSelectedS3ReviewPeerSet(t *testing.T, content string) {
	t.Helper()

	normalized := strings.Join(strings.Fields(content), " ")
	assert.Contains(
		t,
		normalized,
		"includes spec-compliance-review and independent-review",
	)
	assert.Contains(t, normalized, "adds code-quality-review when the workflow profile requires code-quality review")
	assert.Contains(t, normalized, "adds security-review when the security control is selected")
	// The merged terminal gate replaces the retired suite-result keystone; review
	// peers no longer consume or produce a shared suite-result.
	assert.NotContains(t, normalized, "goal-verification")
	assert.NotContains(t, content, "verification/suite-result.yaml")
}

func TestPromotedReviewTemplatesEmitReviewContextOriginHandle(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"skills/independent-review/HOST_SKILL.md.tmpl",
		"skills/security-review/HOST_SKILL.md.tmpl",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			content, err := Render(path, map[string]string{
				"ToolID":      "claude",
				"Trigger":     "/slipway:" + strings.TrimSuffix(strings.TrimPrefix(path, "skills/"), "/HOST_SKILL.md.tmpl"),
				"Description": "test",
			})
			require.NoError(t, err)

			assert.Contains(t, content, "S3_REVIEW")
			assert.Contains(t, content, "native subagent")
			assert.Contains(t, content, "SHARED change worktree")
			assert.Contains(t, content, "context_origin:stage=review=<handle>")
			assert.Contains(t, content, "MUST be DISTINCT")
			assert.NotContains(t, content, "verification/suite-result.yaml")
			assert.NotContains(t, content, "host-embedded")
			assert.NotContains(t, content, "base reader that both review hosts")
			assert.NotContains(t, content, "review_origin:skill=")
			assert.NotContains(t, content, "review_context:skill=")
		})
	}
}

func TestPlanAuditTemplateEmitsPlanAndAuditOriginHandles(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/plan-audit/HOST_SKILL.md")
	require.NoError(t, err)

	// plan-audit records the author/auditor pair tokens (NOT a
	// context_origin:stage= reference). The prose may name the stage grammar to
	// contrast it, but no emitted reference uses the stage form.
	assert.Contains(t, content, "plan_origin:<handle>")
	assert.Contains(t, content, "audit_origin:<handle>")
	assert.NotContains(t, content, `--reference "context_origin:stage=`)
	assert.NotContains(t, content, "review_origin:skill=")
}

func TestShipVerificationTemplateEmitsReviewContextOriginHandle(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/ship-verification/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:ship-verification",
		"Description": "test",
	})
	require.NoError(t, err)

	assert.Contains(t, content, "context_origin:stage=review=<handle>")
	// The retired suite-result keystone is gone; the authoritative suite runs here.
	assert.NotContains(t, content, "verification/suite-result.yaml")
	assert.NotContains(t, content, "context_origin:stage=goal=<handle>")
	assert.NotContains(t, content, "review_origin:skill=")
}

func TestShipVerificationTemplateRequiresConfiguredGoLint(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/ship-verification/HOST_SKILL.md.tmpl", nil)
	require.NoError(t, err)

	assert.Contains(t, content, "`golangci-lint run ./...`")
	assert.Contains(t, content, "block completion on new lint findings")
}

func TestShipVerificationTemplateDoesNotEmitRetiredContextOriginHandle(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/ship-verification/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:ship-verification",
		"Description": "test",
	})
	require.NoError(t, err)

	assert.Contains(t, content, "closeout:reviewer_independence=pass")
	assert.NotContains(t, content, "context_origin:stage=closeout=<handle>")
	assert.NotContains(t, content, "review_origin:skill=")
}

func TestShipVerificationTemplateRequiresReviewerIndependenceAndChainOrder(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/ship-verification/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:ship-verification",
		"Description": "test",
	})
	require.NoError(t, err)

	// Pattern-A presence attestation, now engine-consumed. The attestation token
	// keeps its `closeout:` prefix; the reason code is the canonical ship_verification_*.
	assert.Contains(t, content, `- "closeout:reviewer_independence=pass"`)
	assert.Contains(t, content, "ship_verification_reviewer_independence_missing")
	// Always-on terminal-ordering invariant with its own distinct reason code.
	assert.Contains(t, content, "ship-verification >= every selected S3 review peer")
	assert.Contains(t, content, "includes spec-compliance-review and independent-review")
	assert.Contains(t, content, "code-quality-review when the workflow profile requires code-quality review")
	assert.Contains(t, content, "adds security-review when the security control is selected")
	assert.Contains(t, content, "every selected S3 review peer match the current `run_version`")
	assert.NotContains(t, content, "goal-verification")
	assert.NotContains(t, content, "closeout >= goal-verification >= latest(selected S3 review set)")
	// The retired chain-order code is gone; the ordering invariant now fails closed
	// with its own ship_verification_ordering_invalid code, not the generic
	// ship_verification_evidence_missing.
	assert.Contains(t, content, "ship_verification_ordering_invalid")
	assert.NotContains(t, content, "ship_verification_evidence_missing")
	assert.NotContains(t, content, "closeout_chain_order_invalid")
	assert.Contains(t, content, "Advisory on light")
}

func TestShipVerificationTemplateDocumentsTerminalGateInvariant(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/ship-verification/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:ship-verification",
		"Description": "test",
	})
	require.NoError(t, err)

	assert.Contains(t, content, "At `S3_REVIEW` as the LAST step")
	assert.Contains(t, content, "It is the sole hard `G_ship`")
	assert.Contains(t, content, "after the selected adversarial review peers have")
	assert.NotContains(t, content, "closeout >= goal-verification >= latest(selected S3 review set)")
}

func TestWaveOrchestrationTemplateRequiresDegradedJustification(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:wave-orchestration",
		"Description": "test",
	})
	require.NoError(t, err)

	// A bare degraded_sequential must be paired with the tool-unavailable
	// justification token; unpaired is rejected on standard/strict, advisory on light.
	assert.Contains(t, content, "degraded_dispatch_justification:wave=<wave_index>:tool_unavailable=<detail>")
	assert.Contains(t, content, "degraded_dispatch_justification_missing")
	assert.Contains(t, content, "advisory on light")
}

func TestWaveOrchestrationTemplateDocumentsNoOpJustification(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:wave-orchestration",
		"Description": "test",
	})
	require.NoError(t, err)

	// The honest zero-change code path must be discoverable from the executor
	// result-JSON contract, not left to private host knowledge (#410).
	assert.Contains(t, content, "no_op_justification")
}

func TestCoreGovernanceSkillsUseWorkflowOutlineInsteadOfGraphviz(t *testing.T) {
	t.Parallel()
	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}

	// Static governance skills should now use short workflow outlines.
	for _, name := range []string{
		"skills/plan-audit/HOST_SKILL.md",
	} {
		content, err := Content(name)
		require.NoError(t, err, "failed to load %s", name)
		assert.Contains(t, content, "## Workflow Outline", "%s missing workflow outline section", name)
		assert.NotContains(t, content, "## Workflow Graph (Graphviz DOT)", "%s should not carry Graphviz workflow section", name)
		assert.NotContains(t, content, "```dot", "%s should not carry DOT code block", name)
	}

	// Templated governance skills should also use outlines.
	for _, name := range []string{
		"skills/ship-verification/HOST_SKILL.md.tmpl",
	} {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		assert.Contains(t, content, "## Workflow Outline", "%s missing workflow outline section", name)
		assert.NotContains(t, content, "## Workflow Graph (Graphviz DOT)", "%s should not carry Graphviz workflow section", name)
		assert.NotContains(t, content, "```dot", "%s should not carry DOT code block", name)
	}

	// Wave orchestration should likewise avoid inline DOT.
	data = map[string]string{"ToolID": "claude", "Trigger": "/slipway:wave-orchestration"}
	content, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err, "failed to render wave-orchestration/HOST_SKILL.md.tmpl")
	assert.Contains(t, content, "## Workflow Outline")
	assert.NotContains(t, content, "## Workflow Graph (Graphviz DOT)")
	assert.NotContains(t, content, "```dot")
}

func TestContentReturnsTechniques(t *testing.T) {
	t.Parallel()
	techniques := []string{
		"skills/tdd/HOST_SKILL.md",
		"skills/codebase-mapping/HOST_SKILL.md",
		"skills/coding-discipline/HOST_SKILL.md",
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
		"skills/worktree-preflight/HOST_SKILL.md",
	}
	for _, name := range standalone {
		content, err := Content(name)
		require.NoError(t, err, "failed to load %s", name)
		assert.Contains(t, content, "## Purpose", "%s missing Purpose section", name)
	}
}

func TestCodebaseMappingTemplateDefinesDurableDocumentSet(t *testing.T) {
	t.Parallel()
	content, err := Content("skills/codebase-mapping/HOST_SKILL.md")
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
		"skills/research-orchestration/HOST_SKILL.md",
		"skills/plan-audit/HOST_SKILL.md",
	} {
		content, err := Content(name)
		require.NoError(t, err)
		assert.Contains(t, content, "input_context.codebase_map_dir", "%s missing durable codebase map reference", name)
	}

	// Templated governance skills
	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:wave-orchestration"}
	content, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, content, "input_context.codebase_map_dir", "wave-orchestration/HOST_SKILL.md.tmpl missing durable codebase map reference")
}

func TestPlanningAndDiscoveryTemplatesTreatNonDurableCodebaseMapAsAdvisory(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"skills/research-orchestration/HOST_SKILL.md",
		"skills/plan-audit/HOST_SKILL.md",
	} {
		content, err := Content(name)
		require.NoError(t, err)
		// The map-freshness status field is surfaced in the default handoff.
		assert.Contains(t, content, "codebase_map_status",
			"%s should reference the codebase_map_status freshness field", name)
		// scaffold-only/baseline maps must be treated as non-durable, not as
		// reviewed context.
		assert.Contains(t, content, "scaffold_only",
			"%s should call out the exact scaffold_only status value as non-durable", name)
		assert.NotContains(t, content, "`scaffold-only`",
			"%s must not document a hyphenated status value that callers cannot compare", name)
		assert.Contains(t, content, "non-durable",
			"%s should treat non-populated maps as non-durable context", name)
		// partial maps get no whole-map advisory, so consumers must inspect the
		// per-doc states to stay actionable (REQ-004).
		assert.Contains(t, content, "codebase_map_doc_states",
			"%s should direct consumers to inspect per-doc codebase_map_doc_states", name)
	}
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

func TestRenderTemplatedGovernanceSkillTemplates(t *testing.T) {
	t.Parallel()
	templates := []string{
		"skills/code-quality-review/HOST_SKILL.md.tmpl",
		"skills/independent-review/HOST_SKILL.md.tmpl",
		"skills/security-review/HOST_SKILL.md.tmpl",
		"skills/ship-verification/HOST_SKILL.md.tmpl",
		"skills/spec-compliance-review/HOST_SKILL.md.tmpl",
		"skills/tdd-governance/HOST_SKILL.md.tmpl",
		"skills/wave-orchestration/HOST_SKILL.md.tmpl",
	}
	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:do",
		"Description": "Example governance skill description",
	}
	for _, name := range templates {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		assert.NotContains(t, content, "{{.", "%s has unrendered template vars", name)
		assert.NotContains(t, content, "{{template", "%s has unrendered template directives", name)
	}
}

func TestTemplatedGovernanceSkillFrontmatterIncludesDescription(t *testing.T) {
	t.Parallel()
	templates := []string{
		"skills/code-quality-review/HOST_SKILL.md.tmpl",
		"skills/independent-review/HOST_SKILL.md.tmpl",
		"skills/security-review/HOST_SKILL.md.tmpl",
		"skills/ship-verification/HOST_SKILL.md.tmpl",
		"skills/spec-compliance-review/HOST_SKILL.md.tmpl",
		"skills/tdd-governance/HOST_SKILL.md.tmpl",
		"skills/wave-orchestration/HOST_SKILL.md.tmpl",
	}
	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:do",
		"Description": "Example governance skill description",
	}
	for _, name := range templates {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		parts := strings.SplitN(content, "---", 3)
		require.Len(t, parts, 3, "%s missing frontmatter delimiters", name)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name in frontmatter", name)
		assert.Contains(t, fm, "description:", "%s missing description in frontmatter", name)
	}
}

func TestRenderCommandEntryTemplate(t *testing.T) {
	t.Parallel()
	data := map[string]string{
		"CommandID":    "status",
		"ToolID":       "cursor",
		"Trigger":      "/slipway-status",
		"Description":  "Show lifecycle status and blockers",
		"BodyTemplate": "command-status-body",
		"Arguments":    "--json",
	}
	content, err := Render("commands/command-entry.md.tmpl", data)
	require.NoError(t, err, "failed to render command-entry.md.tmpl")
	assert.NotContains(t, content, "{{.", "command-entry has unrendered template vars")
	assert.Contains(t, content, "# Status", "command-entry should include body partial content")
}

func TestRenderSessionStartHookTemplate(t *testing.T) {
	t.Parallel()
	data := map[string]string{
		"ToolID":       "claude",
		"LaunchPrefix": "slipway",
		"ProbeBin":     "slipway",
	}
	content, err := Render("hooks/session-start.sh.tmpl", data)
	require.NoError(t, err, "failed to render session-start.sh.tmpl")
	assert.LessOrEqual(t, len([]byte(content)), 700, "session-start hook template must stay compact")
	assert.NotContains(t, content, "{{.", "session-start hook has unrendered template vars")
	assert.Contains(t, content, `slipway hook session-start --tool "claude"`)
	assert.NotContains(t, content, "slipway next --json")
	assert.NotContains(t, content, "slipway root")
	assert.NotContains(t, content, "slipway status --json")
	assert.NotContains(t, content, "--preview")
}

func TestRenderNextCommandEntryUsesQueryOnlyContract(t *testing.T) {
	t.Parallel()
	data := map[string]string{
		"CommandID":    "next",
		"ToolID":       "claude",
		"Trigger":      "/slipway:next",
		"Description":  "Query next governance step",
		"BodyTemplate": "command-next-body",
		"Arguments":    "--json",
	}
	content, err := Render("commands/command-entry.md.tmpl", data)
	require.NoError(t, err, "failed to render command-entry.md.tmpl for next")
	assert.LessOrEqual(t, len([]byte(content)), 3000, "generated slipway-next prompt must stay handoff-sized")
	assert.NotContains(t, content, "{{.", "next command entry has unrendered template vars")
	assert.Contains(t, content, "Query the next governed host without advancing lifecycle state.")
	assert.Contains(t, content, "`next_skill.name` is the authoritative governed-host handoff.")
	assert.Contains(t, content, "Treat the default JSON as an action contract")
	assert.Contains(t, content, "confirmation_requirement.next_action")
	assert.NotContains(t, content, "confirmation_requirement.resume_response_supported")
	assert.NotContains(t, content, "context_budget", "retired context-budget guard must not appear in the next prompt")
	assert.Contains(t, content, "slipway health --governance --json --change <slug>")
	assert.Contains(t, content, "Run `slipway run --json` when evidence is ready.")
	assert.Contains(t, content, "`next` is query-only")
	assert.Contains(t, content, "has no `--auto`/`--no-auto` flags")
	assert.Contains(t, content, "never mutates pending preset confirmations")
	assert.NotContains(t, content, "single-step progression command")
	assert.NotContains(t, content, "state progression context")
	assert.NotContains(t, content, "systematic-debugging")
	assert.NotContains(t, content, "Rationalization Red Flags")
}

func TestContentDoesNotExposeAgentDefinitions(t *testing.T) {
	t.Parallel()

	removedPath := path.Join("agents", "slipway-planner.md")
	_, err := Content(removedPath)
	require.Error(t, err)

	matches, err := fs.Glob(TemplateFS(), path.Join("agents", "*.md"))
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestGovernanceSkillFrontmatterMinimal(t *testing.T) {
	t.Parallel()
	// Static governance skills
	staticSkills := []string{
		"skills/worktree-preflight/HOST_SKILL.md",
		"skills/plan-audit/HOST_SKILL.md",
		"skills/research-orchestration/HOST_SKILL.md",
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
		assert.Contains(t, fm, "skill_id:", "%s missing skill_id in frontmatter", name)
		assert.Contains(t, fm, "description:", "%s missing description in frontmatter", name)
		assert.Contains(t, fm, "Use when ", "%s description must be trigger-oriented", name)
		assert.Contains(t, fm, "Triggers on", "%s description must describe trigger contract", name)
		for _, field := range routingFields {
			assert.NotContains(t, fm, field, "%s frontmatter contains routing field %s", name, field)
		}
	}
	// Templated governance skills (converted from static)
	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}
	for _, name := range []string{
		"skills/tdd-governance/HOST_SKILL.md.tmpl",
		"skills/spec-compliance-review/HOST_SKILL.md.tmpl",
		"skills/code-quality-review/HOST_SKILL.md.tmpl",
		"skills/ship-verification/HOST_SKILL.md.tmpl",
	} {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		parts := strings.SplitN(content, "---", 3)
		require.Len(t, parts, 3, "%s missing frontmatter delimiters", name)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name in frontmatter", name)
		assert.Contains(t, fm, "skill_id:", "%s missing skill_id in frontmatter", name)
		assert.Contains(t, fm, "description:", "%s missing description in frontmatter", name)
		assert.Contains(t, fm, "Use when ", "%s description must be trigger-oriented", name)
		assert.Contains(t, fm, "Triggers on", "%s description must describe trigger contract", name)
		for _, field := range routingFields {
			assert.NotContains(t, fm, field, "%s frontmatter contains routing field %s", name, field)
		}
	}
}

func TestGovernanceTemplatedSkillFrontmatterMinimal(t *testing.T) {
	t.Parallel()
	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:wave-orchestration"}
	content, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err)
	parts := strings.SplitN(content, "---", 3)
	require.Len(t, parts, 3, "missing frontmatter delimiters")
	fm := parts[1]
	assert.Contains(t, fm, "name:", "missing name in frontmatter")
	assert.Contains(t, fm, "skill_id:", "missing skill_id in frontmatter")
	assert.Contains(t, fm, "description:", "missing description in frontmatter")
	assert.Contains(t, fm, "Use when ", "description must be trigger-oriented")
	assert.Contains(t, fm, "Triggers on", "description must describe trigger contract")
	assert.Contains(t, fm, "tool:", "missing tool in frontmatter")
	for _, field := range []string{"required_levels:", "state:", "type:", "mitigation_target:", "run_summary_bound:"} {
		assert.NotContains(t, fm, field, "frontmatter contains routing field %s", field)
	}
}

func TestTechniqueSkillFrontmatterMinimal(t *testing.T) {
	t.Parallel()
	techniques := []string{
		"skills/tdd/HOST_SKILL.md",
		"skills/codebase-mapping/HOST_SKILL.md",
		"skills/coding-discipline/HOST_SKILL.md",
	}
	for _, name := range techniques {
		content, err := Content(name)
		require.NoError(t, err, "failed to load %s", name)
		parts := strings.SplitN(content, "---", 3)
		require.Len(t, parts, 3, "%s missing frontmatter delimiters", name)
		fm := parts[1]
		assert.Contains(t, fm, "name:", "%s missing name in frontmatter", name)
		assert.Contains(t, fm, "skill_id:", "%s missing skill_id in frontmatter", name)
		assert.Contains(t, fm, "description:", "%s missing description in frontmatter", name)
		assert.Contains(t, fm, "Use when ", "%s description must be trigger-oriented", name)
		assert.Contains(t, fm, "Triggers on", "%s description must describe trigger contract", name)
		assert.NotContains(t, fm, "type:", "%s frontmatter contains type field", name)
	}
}

func TestStandaloneSkillFrontmatterMinimal(t *testing.T) {
	t.Parallel()
	content, err := Content("skills/worktree-preflight/HOST_SKILL.md")
	require.NoError(t, err, "failed to load worktree-preflight")
	parts := strings.SplitN(content, "---", 3)
	require.Len(t, parts, 3, "skills/worktree-preflight/HOST_SKILL.md missing frontmatter delimiters")
	fm := parts[1]
	assert.Contains(t, fm, "name:", "skills/worktree-preflight/HOST_SKILL.md missing name in frontmatter")
	assert.Contains(t, fm, "skill_id:", "skills/worktree-preflight/HOST_SKILL.md missing skill_id in frontmatter")
	assert.Contains(t, fm, "description:", "skills/worktree-preflight/HOST_SKILL.md missing description in frontmatter")
	assert.Contains(t, fm, "Use when ", "skills/worktree-preflight/HOST_SKILL.md description must be trigger-oriented")
	assert.Contains(t, fm, "Triggers on", "skills/worktree-preflight/HOST_SKILL.md description must describe trigger contract")
}

func TestEntrySurfaceTemplatesAvoidPlanOnlyVocabulary(t *testing.T) {
	t.Parallel()

	newCmd, err := Render("commands/command-entry.md.tmpl", map[string]string{
		"CommandID":    "new",
		"ToolID":       "claude",
		"Trigger":      "/slipway:new",
		"Description":  "Create a governed change with intake-first workflow",
		"BodyTemplate": "command-new-body",
		"Arguments":    "--json",
	})
	require.NoError(t, err)

	for name, content := range map[string]string{
		"new": newCmd,
	} {
		assert.NotContains(t, content, "plan-only", "%s template reintroduced retired plan-only wording", name)
	}
}

func TestWorkflowStateTemplatesAvoidRetiredIntakeVocabulary(t *testing.T) {
	t.Parallel()

	researchSkill, err := Content("skills/research-orchestration/HOST_SKILL.md")
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

func TestResearchOrchestrationUsesResearchArtifactSchemaHeadings(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/research-orchestration/HOST_SKILL.md")
	require.NoError(t, err)

	assert.Contains(t, content, "## Alternatives Considered")
	assert.Contains(t, content, "## Unknowns")
	assert.Contains(t, content, "## Assumptions")
	assert.Contains(t, content, "## Canonical References")
	assert.NotContains(t, content, "### Unknowns Resolved")
	assert.NotContains(t, content, "## Research Findings")
	assert.Contains(t, content, "research_section_placeholder")
}

func TestResearchOrchestrationStaleDiscoveryEvidenceRoutesToEvidenceSkill(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/research-orchestration/HOST_SKILL.md")
	require.NoError(t, err)

	start := strings.Index(content, "- **Stale**")
	require.NotEqual(t, -1, start)
	end := strings.Index(content[start:], "- **Missing**")
	require.NotEqual(t, -1, end)
	staleBullet := content[start : start+end]

	assert.Contains(t, staleBullet, "`required_skill_stale`")
	assert.Contains(t, staleBullet, "`slipway evidence skill --skill research-orchestration --verdict pass`")
	assert.NotContains(t, staleBullet, "`slipway run`")
	assert.Contains(t, staleBullet, "must never be restamped")
}

func TestPlanningSkillsFollowArtifactDependencyOrder(t *testing.T) {
	t.Parallel()

	research, err := Content("skills/research-orchestration/HOST_SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, research, "Record the selected approach in `research.md`")
	assert.Contains(t, research, "Do not author `decision.md` during research")
	assert.NotContains(t, research, "`slipway instructions decision`")
	assert.NotContains(t, research, "locked decision authored into `decision.md`")

	planAudit, err := Content("skills/plan-audit/HOST_SKILL.md")
	require.NoError(t, err)
	requirementsIdx := strings.Index(planAudit, "`slipway instructions requirements`")
	decisionIdx := strings.Index(planAudit, "`slipway instructions decision`")
	tasksIdx := strings.Index(planAudit, "`slipway instructions tasks`")
	require.NotEqual(t, -1, requirementsIdx)
	require.NotEqual(t, -1, decisionIdx)
	require.NotEqual(t, -1, tasksIdx)
	assert.Less(t, requirementsIdx, decisionIdx)
	assert.Less(t, decisionIdx, tasksIdx)
	assert.Contains(t, planAudit, "schema dependency order")
	assert.Contains(t, planAudit, "`decision_contract`")
	assert.Contains(t, strings.Join(strings.Fields(planAudit), " "), "Every task names concrete `target_files`")
	assert.NotContains(t, planAudit, "research-orchestration already locked")
}

func TestVerificationDoctrineDocumentsStringOnlyReferences(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/ship-verification/HOST_SKILL.md.tmpl", nil)
	require.NoError(t, err)

	assert.Contains(t, content, "YAML sequence of strings only")
	assert.Contains(t, content, "do not write structured maps under `references`")
}

func TestShipVerificationPlaceholderScanIsMacOSPortable(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/ship-verification/HOST_SKILL.md.tmpl", nil)
	require.NoError(t, err)

	// The prescribed scan must run on stock Windows: no perl, no BSD/macOS-incompatible
	// `grep -P`, and no hardcoded GNU `grep ... \|` alternation as the mandated path.
	assert.NotContains(t, content, "perl -0ne")
	assert.NotContains(t, content, "grep -P")
	assert.NotContains(t, content, `grep -E 'TODO`)
	assert.NotContains(t, content, `\{\s*\n\s*\}`)

	// Portable, tool-agnostic guidance: name the markers and the empty-body check,
	// and express the HOW via whatever code search is available (editor/agent search or rg).
	assert.Contains(t, content, "PLACEHOLDER")
	assert.Contains(t, content, "TODO")
	assert.Contains(t, content, "code-search capability is available")
	assert.Contains(t, content, "editor/agent search")
	assert.Contains(t, content, "ripgrep")
	assert.Contains(t, content, "rg")
}

func TestPlanAuditBlocksFutureLifecycleAcceptanceCriteria(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/plan-audit/HOST_SKILL.md")
	require.NoError(t, err)

	assert.Contains(t, content, "satisfiable during S2 implementation")
	assert.Contains(t, content, "future S3 review or closeout evidence")
	assert.NotContains(t, content, "S3 closeout")
}

func TestPlanAuditScopeControlFlagsSharedTypeBlastRadius(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/plan-audit/HOST_SKILL.md")
	require.NoError(t, err)
	flat := strings.Join(strings.Fields(content), " ")

	// Scope Control must catch a code task that changes a shared/widely-referenced
	// type whose forced consumer edits no task's target_files own — the plan that
	// passes plan-audit but the S2 integration gate later rejects (issue #277).
	assert.Contains(t, flat, "cover the full edit set the tasks' own objectives force")
	assert.Contains(t, flat, "shared or widely-referenced type or contract")
	assert.Contains(t, flat, "non-exhaustive/non-sealed enum")
	assert.Contains(t, flat, "owned by some task's `target_files`")
}

func TestNextCommandDocumentsWorktreeSkillCatalogFallback(t *testing.T) {
	t.Parallel()

	content, err := Render("commands/command-entry.md.tmpl", map[string]string{
		"CommandID":    "next",
		"ToolID":       "claude",
		"Trigger":      "/slipway:next",
		"Description":  "Query the next governed host",
		"BodyTemplate": "command-next-body",
		"Arguments":    "--json",
	})
	require.NoError(t, err)

	assert.Contains(t, content, "worktree-bound changes")
	assert.Contains(t, content, "source checkout or globally")
	assert.Contains(t, content, "next_skill.verification_dir")
}

func TestWorktreePreflightDocumentsRepoLocalDefaultPath(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/worktree-preflight/HOST_SKILL.md")
	require.NoError(t, err)

	assert.Contains(t, content, ".worktrees/<slug>")
	assert.Contains(t, content, "operator-supplied path")
	assert.Contains(t, content, "cheapest deterministic baseline command")
	assert.Contains(t, content, "final verification proves the completed change")
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
	content, err := Render("skills/spec-compliance-review/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, content, "<HARD-GATE>", "hard-gate partial should render into governance skill")
	assert.Contains(t, content, "Do not call an advancing command", "hard-gate partial content missing")
}

func TestGovernedHostTemplatesAdvanceWithRunAfterConfirmation(t *testing.T) {
	t.Parallel()

	staticSkills := []string{
		"skills/intake-clarification/HOST_SKILL.md",
		"skills/plan-audit/HOST_SKILL.md",
		"skills/research-orchestration/HOST_SKILL.md",
		"skills/worktree-preflight/HOST_SKILL.md",
	}
	forbiddenNextAdvanceFragments := []string{
		"After confirmation: `slipway next`",
		"```bash\nslipway next\n```",
		"Do not run `slipway next` until",
		"calling `slipway next`",
	}
	for _, name := range staticSkills {
		content, err := Content(name)
		require.NoError(t, err, "failed to load %s", name)
		for _, forbidden := range forbiddenNextAdvanceFragments {
			assert.NotContains(
				t,
				content,
				forbidden,
				"%s must not route advancement through read-only next",
				name,
			)
		}
		assert.Contains(t, content, "slipway run", "%s must name the advancing command", name)
	}

	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:test",
		"Description": "test",
	}
	templatedSkills := []string{
		"skills/wave-orchestration/HOST_SKILL.md.tmpl",
		"skills/spec-compliance-review/HOST_SKILL.md.tmpl",
		"skills/code-quality-review/HOST_SKILL.md.tmpl",
		"skills/independent-review/HOST_SKILL.md.tmpl",
		"skills/security-review/HOST_SKILL.md.tmpl",
		"skills/ship-verification/HOST_SKILL.md.tmpl",
	}
	for _, name := range templatedSkills {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		for _, forbidden := range forbiddenNextAdvanceFragments {
			assert.NotContains(
				t,
				content,
				forbidden,
				"%s must not route advancement through read-only next",
				name,
			)
		}
		assert.Contains(t, content, "slipway run", "%s must name the advancing command", name)
	}
}

func TestTDDGovernanceUsesHostOwnedTaskEvidenceContract(t *testing.T) {
	t.Parallel()

	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:tdd-governance",
		"Description": "test",
	}
	content, err := Render("skills/tdd-governance/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err)

	assert.Contains(t, content, "slipway evidence task --task-id",
		"tdd-governance should point agents at host-owned task evidence recording")
	assertNoLedgerTaskEvidenceFlags(t, content, "tdd-governance")
	assert.Contains(t, content, "separate evidence-note command")
	assert.NotContains(t, content, "recorded not-applicable via a `slipway evidence task` note")
	assert.NotContains(t, content, "rather than a TDD verdict")
}

func TestIncidentResponseDoesNotTriggerOnBareStatusHealth(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/incident-response/CATALOG_SKILL.md")
	require.NoError(t, err)

	assert.Contains(t, content, "status --focus incident")
	assert.Contains(t, content, "health --focus incident")
	assert.NotContains(t, content, `command: ["status", "health"]`)
	assert.NotContains(t, content, "status or health command invoked; incident may be in scope")
}

func TestRunSummaryBoundGovernedTemplatesDoNotUseLiteralRunVersion(t *testing.T) {
	t.Parallel()

	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:test",
		"Description": "test",
	}
	tests := []struct {
		name           string
		templatePath   string
		wantRunSources []string
	}{
		{
			name:           "wave orchestration",
			templatePath:   "skills/wave-orchestration/HOST_SKILL.md.tmpl",
			wantRunSources: []string{"run_version: <current wave-orchestration run_version>"},
		},
		{
			name:           "tdd governance",
			templatePath:   "skills/tdd-governance/HOST_SKILL.md.tmpl",
			wantRunSources: []string{"run_version: <current wave-orchestration run_version>"},
		},
		{
			name:           "spec compliance review",
			templatePath:   "skills/spec-compliance-review/HOST_SKILL.md.tmpl",
			wantRunSources: []string{"slipway evidence skill", "current `run_summary_version`"},
		},
		{
			name:           "code quality review",
			templatePath:   "skills/code-quality-review/HOST_SKILL.md.tmpl",
			wantRunSources: []string{"slipway evidence skill", "current `run_summary_version`"},
		},
		{
			name:           "ship verification",
			templatePath:   "skills/ship-verification/HOST_SKILL.md.tmpl",
			wantRunSources: []string{"run_version: <current run_summary_version from slipway status --json>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := Render(tt.templatePath, data)
			require.NoError(t, err, "failed to render %s", tt.templatePath)
			assert.NotContains(
				t,
				content,
				"\nrun_version: 1\n",
				"%s must not guide agents to copy a stale literal run_version",
				tt.templatePath,
			)
			for _, want := range tt.wantRunSources {
				assert.Contains(
					t,
					content,
					want,
					"%s must name the authoritative run_version source",
					tt.templatePath,
				)
			}
		})
	}
}

func TestPartialsDeduplicateGovernanceContent(t *testing.T) {
	t.Parallel()
	// Verify shared verification doctrine renders in ship-verification.
	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:ship-verification",
		"Description": "test",
	}
	content, err := Render("skills/ship-verification/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, content, `"should work"`, "banned-language partial should render into ship-verification")
	assert.Contains(t, content, "opinions, not evidence", "banned-language partial content missing")
	assert.Contains(t, content, "Treat stale or missing verification as a blocker",
		"verification-doctrine partial should render into ship-verification")
}

func TestS3ReviewTemplateContractsStayLiveAndSynchronized(t *testing.T) {
	t.Parallel()

	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:test",
		"Description": "test",
	}
	tests := []struct {
		name               string
		templatePath       string
		skillID            string
		verificationRecord string
		notesFile          string
		requiredReferences []string
	}{
		{
			name:               "code quality review",
			templatePath:       "skills/code-quality-review/HOST_SKILL.md.tmpl",
			skillID:            "code-quality-review",
			verificationRecord: "verification/code-quality-review.yaml",
			notesFile:          "artifacts/changes/{slug}/verification/code-quality-review-notes.md",
			requiredReferences: []string{
				`--reference "layer:IR1=pass"`,
				`--reference "context_origin:stage=review=<handle>"`,
			},
		},
		{
			name:               "independent review",
			templatePath:       "skills/independent-review/HOST_SKILL.md.tmpl",
			skillID:            "independent-review",
			verificationRecord: "verification/independent-review.yaml",
			notesFile:          "artifacts/changes/{slug}/verification/independent-review-notes.md",
			requiredReferences: []string{
				`--reference "context_origin:stage=review=<handle>"`,
			},
		},
		{
			name:               "security review",
			templatePath:       "skills/security-review/HOST_SKILL.md.tmpl",
			skillID:            "security-review",
			verificationRecord: "verification/security-review.yaml",
			notesFile:          "artifacts/changes/{slug}/verification/security-review-notes.md",
			requiredReferences: []string{
				`--reference "context_origin:stage=review=<handle>"`,
			},
		},
		{
			name:               "spec compliance review",
			templatePath:       "skills/spec-compliance-review/HOST_SKILL.md.tmpl",
			skillID:            "spec-compliance-review",
			verificationRecord: "verification/spec-compliance-review.yaml",
			notesFile:          "artifacts/changes/{slug}/verification/spec-compliance-review-notes.md",
			requiredReferences: []string{
				`--reference "layer:R0=pass"`,
				`--reference "scope_contract:pass"`,
				`--reference "negative_path:pass"`,
				`--reference "context_origin:stage=review=<handle>"`,
			},
		},
	}

	var diskHandoffContract string
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			content, err := Render(tt.templatePath, data)
			require.NoError(t, err, "failed to render %s", tt.templatePath)
			assert.Equal(t, 1, strings.Count(content, "## Disk-Handoff Contract"))
			assert.Equal(t, 1, strings.Count(content, "## Record Verification"))

			currentDiskHandoff := markdownSection(t, content, "## Disk-Handoff Contract")
			if diskHandoffContract == "" {
				diskHandoffContract = currentDiskHandoff
			}
			assert.Equal(t, diskHandoffContract, currentDiskHandoff,
				"%s disk-handoff contract drifted from the S3 review peer contract",
				tt.templatePath)

			recordVerification := markdownSection(t, content, "## Record Verification")
			assert.Contains(t, recordVerification,
				"Write bulky review notes to disk, then record the verdict through the CLI so\nSlipway owns the timestamp, `run_version`, freshness inputs, and digest stamp.")
			assert.Contains(t, recordVerification, "Do not hand-edit `"+tt.verificationRecord+"`.")
			assert.Contains(t, recordVerification, "slipway evidence skill")
			assert.Contains(t, recordVerification, "--skill "+tt.skillID)
			assert.Contains(t, recordVerification, "--verdict pass")
			assert.Contains(t, recordVerification, "--notes-file "+tt.notesFile)
			for _, requiredReference := range tt.requiredReferences {
				assert.Contains(t, recordVerification, requiredReference)
			}
			assert.Contains(t, content, `--reference "fallback:same_context_degraded"`)
			assert.Contains(t, content, "`context_origin:stage=review=<handle>`")
		})
	}
}

func TestRunCommandEntryContainsLoopBehavioralBlocks(t *testing.T) {
	t.Parallel()
	data := map[string]string{
		"CommandID":    "run",
		"ToolID":       "claude",
		"Trigger":      "/slipway:run",
		"Description":  "Advance governed execution until a skill, blocker, or done-ready outcome is surfaced",
		"BodyTemplate": "command-run-body",
		"Arguments":    "--json",
	}
	content, err := Render("commands/command-entry.md.tmpl", data)
	require.NoError(t, err)
	assert.LessOrEqual(t, len([]byte(content)), 6500, "generated slipway-run prompt must stay compact")

	assert.NotContains(t, content, "context_budget",
		"retired context-budget guard must not appear in the run prompt")
	assert.Contains(t, content, "Use `slipway run --json --diagnostics` only for full readiness fields",
		"run command missing diagnostics boundary guidance")
	assert.Contains(t, content, "`advanced` reports the mutation performed by this invocation",
		"run command missing advanced/blockers boundary guidance")
	assert.NotContains(t, content, "confirmation_requirement.resume_response_supported",
		"run command must not advertise deleted resume-response capability guidance")
	assert.Contains(t, content, "slipway health --governance --json --change <slug>",
		"run command missing governance health handoff")

	assert.Contains(t, content, "fresh reviewer agent",
		"run command missing fresh-reviewer pause mandate")
	assert.Contains(t, content, "`independent-review`",
		"run command missing independent-review handoff")
	assert.Contains(t, content, "`security-review` when selected",
		"run command missing selected security-review handoff")
	assert.Contains(t, content, "`ship-verification` is the last terminal step",
		"run command must classify ship-verification after selected peers")
	assert.NotContains(t, content, "`goal-verification`",
		"run command must not name the retired goal-verification peer")
	assert.NotContains(t, content, "`final-closeout`",
		"run command must not name the retired final-closeout step")

	assert.Contains(t, content, "Fresh Context Boundary Rule (HARD RULE)",
		"run command missing fresh-context boundary rule")
	assert.Contains(t, content, "plain operator confirmation",
		"run command must not require fresh subagents for plain lifecycle confirmations")
	assert.Contains(t, content, "does not by itself require spawning a subagent",
		"run command must distinguish confirmation from review handoff")
	assert.NotContains(t, content, "After any checkpoint pause, user intervention, or governed review handoff",
		"run command must not treat every user intervention as a fresh-subagent boundary")

	assert.Contains(t, content, "three consecutive skills fail",
		"run command missing 3-consecutive-failure exit rule")

	assert.NotContains(t, content, "user_response_payload",
		"run skill must not advertise deleted checkpoint response handoff guidance")
	assert.NotContains(t, content, "--resume-response",
		"run skill must not advertise deleted checkpoint resume flag")
	assert.NotContains(t, content, "auto mode auto-acknowledges",
		"run command must not describe deleted checkpoint auto-ack behavior")
	assert.NotContains(t, content, "auto auto-acknowledges")

	normalized := strings.Join(strings.Fields(content), " ")
	for _, phrase := range []string{
		"Under auto (`execution.auto` or `slipway run --auto`)",
		"`--auto`/`--no-auto`: override `execution.auto` for this run",
		"auto continues only routine `run_slipway_run_to_advance` command boundaries",
		"never skill/review handoffs",
		"Skill handoffs and review batches still stop the run loop for host work",
		"`evidence_continuation` instead of `hard_stop`",
		"Auto never crosses sensitive/guardrail confirmations",
		"`security-review` boundaries",
		"the intake Approved Summary",
		"evidence gates",
	} {
		assert.Contains(t, normalized, phrase,
			"run command auto-mode redline phrase missing")
	}
}

func TestStatusCommandEntryUsesGovernanceSummaryContract(t *testing.T) {
	t.Parallel()
	data := map[string]string{
		"CommandID":    "status",
		"ToolID":       "claude",
		"Trigger":      "/slipway:status",
		"Description":  "Show lifecycle status, blockers, and next actions",
		"BodyTemplate": "command-status-body",
		"Arguments":    "--json",
	}
	content, err := Render("commands/command-entry.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, content, "Treat `status --json` as a lifecycle/status contract")
	assert.Contains(t, content, "`governance_summary`")
	assert.Contains(t, content, "slipway health --governance --json --change <slug>")
	assert.NotContains(t, content, "consume full governance controls from `status --json`")
}

func TestEvidenceCommandEntryUsesHostOwnedTaskSurface(t *testing.T) {
	t.Parallel()

	content := renderPromptSurfaceForTest(t, "commands/command-entry.md.tmpl", "evidence", "command-evidence-body", "claude")
	assert.Contains(t, content, "slipway evidence task --task-id",
		"evidence command body should teach host-owned task evidence recording")
	assert.Contains(t, content, `--reference "context_origin:stage=review=<handle>"`,
		"evidence command body should teach selected-review context-origin evidence")
	assert.Contains(t, content, "artifacts/changes/<slug>/verification/<selected-review-skill>-notes.md",
		"evidence command body should teach selected-review notes-file convention")
	assert.Contains(t, content, `--reference "fallback:same_context_degraded"`,
		"evidence command body should teach degraded fallback as a structured reference")
	assert.Equal(t, 1, strings.Count(content, "slipway evidence task --help"),
		"evidence command body should have exactly one manual fallback breadcrumb")

	taskSection := content
	start := strings.Index(taskSection, "### `evidence task`")
	require.NotEqual(t, -1, start, "evidence command body missing evidence task flag section")
	taskSection = taskSection[start:]
	end := strings.Index(taskSection, "### `evidence skill`")
	require.NotEqual(t, -1, end, "evidence command body missing evidence skill flag section")
	taskSection = taskSection[:end]

	assertNoLedgerTaskEvidenceFlags(t, taskSection, "evidence command task section")
}

func TestEvidenceCommandContractScopesTaskLifecycleAndDropsSuiteResult(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name         string
		templateName string
		toolID       string
	}{
		{
			name:         "claude command surface",
			templateName: "commands/command-entry.md.tmpl",
			toolID:       "claude",
		},
		{
			name:         "codex command skill",
			templateName: "commands/command-skill.md.tmpl",
			toolID:       "codex",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := renderPromptSurfaceForTest(t, tt.templateName, "evidence", "command-evidence-body", tt.toolID)
			// The suite-result subcommand is retired; the authoritative suite and
			// SAST proof are produced by the terminal ship-verification gate.
			assert.NotContains(t, content, "slipway evidence suite-result",
				"the retired suite-result subcommand must not be advertised")
			assert.NotContains(t, content, "### `evidence suite-result`",
				"the retired suite-result flag section must be removed")

			contractStart := strings.Index(content, "## Contract")
			require.NotEqual(t, -1, contractStart, "evidence command surface missing Contract section")
			contract := content[contractStart:]
			flagsStart := strings.Index(contract, "## Flags")
			require.NotEqual(t, -1, flagsStart, "evidence command surface missing Flags section")
			contract = contract[:flagsStart]

			assert.Contains(t, contract, "`evidence task` is only valid for an active change in `S2_IMPLEMENT`",
				"the S2 lifecycle precondition must be scoped to evidence task")
			assert.NotContains(t, contract, "- Only valid for an active change in `S2_IMPLEMENT`",
				"the shared Contract section must not make the whole evidence command S2-only")
			assert.Contains(t, contract, "ship-verification",
				"the Contract section should point full-suite/SAST proof at ship-verification")
		})
	}
}

func TestTemplateFSExcludesTransientPythonArtifacts(t *testing.T) {
	t.Parallel()

	err := fs.WalkDir(TemplateFS(), ".", func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		name := d.Name()
		if d.IsDir() {
			assert.NotEqual(t, "__pycache__", name, "template FS must not embed python cache directory %s", path)
			return nil
		}
		assert.False(t,
			strings.HasSuffix(name, ".pyc") || strings.HasSuffix(name, ".pyo"),
			"template FS must not embed transient python bytecode %s",
			path,
		)
		return nil
	})
	require.NoError(t, err)
}

func TestWaveOrchestrationEvidenceTaskSurfaceUsesHostOwnedFlags(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":  "claude",
		"Trigger": "/slipway:wave-orchestration",
	})
	require.NoError(t, err)

	assert.Contains(t, content, "slipway evidence task --task-id",
		"wave-orchestration skill must teach host-owned task evidence recording")
	assertNoLedgerTaskEvidenceFlags(t, content, "wave-orchestration")
	assert.NotContains(t, content, "manual flag mode",
		"wave-orchestration skill must not teach manual task ledger mode as agent guidance")
	assert.Contains(t, content, "Do not hand-write files under",
		"wave-orchestration skill must forbid manual runtime task JSON edits")
}

func TestWaveOrchestrationSkillOmitsDeletedCheckpointResumeGuidance(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":  "claude",
		"Trigger": "/slipway:wave-orchestration",
	})
	require.NoError(t, err)

	assert.NotContains(t, content, "user_response_payload",
		"wave-orchestration skill must not advertise deleted checkpoint response guidance")
	assert.NotContains(t, content, "checkpoint_type",
		"wave-orchestration skill must not advertise deleted checkpoint type guidance")
	assert.Contains(t, content, "IRON LAW: NO TASK EXECUTION WITHOUT A GOVERNED PLAN AND CONFLICT DETECTION",
		"wave-orchestration skill missing top-level IRON LAW")
	assert.Contains(t, content, "slipway evidence task",
		"wave-orchestration skill must route task evidence through the supported CLI")
	assert.Contains(t, content, "slipway evidence task --task-id",
		"wave-orchestration skill must teach host-owned task evidence recording")
	assertNoLedgerTaskEvidenceFlags(t, content, "wave-orchestration")
	assert.NotContains(t, content, "result-file mode",
		"wave-orchestration skill must not teach result-file import as agent guidance")
	assert.Contains(t, content, "`run_summary_version`, `task_kind`, `target_files`,",
		"wave-orchestration skill must identify ledger-owned fields")
	assert.NotContains(t, content, `slipway evidence task --task-id "<task_id>" --run-summary-version`,
		"wave-orchestration skill must not teach the long manual evidence command as the default")
	assert.Contains(t, content, "Do not hand-write files under",
		"wave-orchestration skill must forbid manual runtime task JSON edits")
}

func TestVariantAnalysisSkillMakesReferenceShelfVisible(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/variant-analysis/CATALOG_SKILL.md")
	require.NoError(t, err)

	assert.Contains(t, content, "## Reference Shelf")
	assert.Contains(t, content, "references/methodology.md")
	assert.Contains(t, content, "references/query-patterns.md")
}

func TestCodingDisciplineSkillKeepsFourPrinciplesAndDesignStance(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/coding-discipline/HOST_SKILL.md")
	require.NoError(t, err)

	assert.Contains(t, content, "## Design Stance")
	assert.Contains(t, content, "It is not an")
	assert.Contains(t, content, "additional routed workflow or independent methodology.")
	assert.NotContains(t, content, "## Provenance")
	assert.NotContains(t, content, "multica-ai/andrej-karpathy-skills")
	assert.Contains(t, content, "## Think Before Coding")
	assert.Contains(t, content, "## Simplicity First")
	assert.Contains(t, content, "## Surgical Changes")
	assert.Contains(t, content, "## Goal-Driven Execution")
	assert.Contains(t, content, "`slipway-plan-audit`")
	assert.Contains(t, content, "`slipway-tdd-governance`")
	assert.Contains(t, content, "`slipway-ship-verification`")
	assert.Contains(t, content, "`slipway-independent-review`")
}

func TestGovernedHostTemplatesReferenceCodingDiscipline(t *testing.T) {
	t.Parallel()

	planAudit, err := Content("skills/plan-audit/HOST_SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, planAudit, "`slipway-coding-discipline`")

	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}
	for _, name := range []string{
		"skills/wave-orchestration/HOST_SKILL.md.tmpl",
		"skills/spec-compliance-review/HOST_SKILL.md.tmpl",
		"skills/code-quality-review/HOST_SKILL.md.tmpl",
	} {
		content, err := Render(name, data)
		require.NoError(t, err, "failed to render %s", name)
		assert.Contains(t, content, "`slipway-coding-discipline`", "%s must reference slipway-coding-discipline", name)
	}
}

func TestReviewTemplatesRequireNegativePathAndToolchainEvidence(t *testing.T) {
	t.Parallel()

	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}

	specCompliance, err := Render("skills/spec-compliance-review/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, specCompliance, "requirement-named negative/error paths")
	assert.Contains(t, specCompliance, "negative_path:pass")

	codeQuality, err := Render("skills/code-quality-review/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, codeQuality, "dependency/toolchain compatibility")
	assert.Contains(t, codeQuality, "MSRV")
	assert.Contains(t, codeQuality, "toolchain_compat:pass")
}

// TestSpecComplianceReviewGuardsAgainstTestPresenceOverTrust pins the issue #44
// review-fidelity guidance: spec-compliance-review must require that cited tests
// exercise the literal requirement clause and must block when the implementation
// narrows a clause without tightening the requirement prose, so review evidence
// cannot over-trust the mere presence of (narrower) tests.
func TestSpecComplianceReviewGuardsAgainstTestPresenceOverTrust(t *testing.T) {
	t.Parallel()

	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}

	specCompliance, err := Render("skills/spec-compliance-review/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, specCompliance, "Test presence is not")
	assert.Contains(t, specCompliance, "satisfied only in appearance")

	specTrace, err := Content("skills/spec-trace/CHECKLIST.tmpl")
	require.NoError(t, err)
	assert.Contains(t, specTrace, "exercises the literal clause it maps to")
}

// TestSpecTraceRecordsUncheckableCoverageGaps pins issue #157: spec-trace must
// provide a per-item uncertain status instead of forcing "could not check"
// mappings into covered/skipped/drift, and spec-compliance-review must not pass
// unresolved uncertainty as full bidirectional alignment.
func TestSpecTraceRecordsUncheckableCoverageGaps(t *testing.T) {
	t.Parallel()

	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}

	specTrace, err := Content("skills/spec-trace/CATALOG_SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, specTrace, "status: covered | skipped | drift | ambiguous | uncheckable")
	assert.Contains(t, specTrace, "reason: \"<why this mapping is ambiguous or uncheckable>\"")
	assert.Contains(t, specTrace, "coverage_gaps:")

	checklist, err := Content("skills/spec-trace/CHECKLIST.tmpl")
	require.NoError(t, err)
	assert.Contains(t, checklist, "`ambiguous`")
	assert.Contains(t, checklist, "`uncheckable`")
	assert.Contains(t, checklist, "must include a reason")
	assert.Contains(t, checklist, "coverage gaps")

	specCompliance, err := Render("skills/spec-compliance-review/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err)
	normalizedSpecCompliance := strings.Join(strings.Fields(specCompliance), " ")
	assert.Contains(t, specCompliance, "ambiguous` or `uncheckable`")
	assert.Contains(t, specCompliance, "must not be treated as full bidirectional alignment")
	assert.Contains(t, normalizedSpecCompliance, "block or request changes")
}

// TestSpecComplianceReviewTreatsPendingDecisionsAsAdvisory pins issue #140:
// the Decision Fidelity Check must enforce fidelity only against
// locked_decisions and treat pending_decisions (a recommended-but-unconfirmed
// approach) as advisory, never raising decision_fidelity:violated against it.
func TestSpecComplianceReviewTreatsPendingDecisionsAsAdvisory(t *testing.T) {
	t.Parallel()

	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}

	specCompliance, err := Render("skills/spec-compliance-review/HOST_SKILL.md.tmpl", data)
	require.NoError(t, err)
	assert.Contains(t, specCompliance, "skill_constraints.pending_decisions")
	assert.Contains(t, specCompliance, "Enforce fidelity ONLY against `skill_constraints.locked_decisions`")
	assert.Contains(t, specCompliance, "Do not raise `decision_fidelity:violated`")
	assert.Contains(t, specCompliance, "against a pending decision")
	assert.Contains(t, specCompliance, "treat it as\nadvisory context, not a contract")
}

func TestRootCauseTracingAbsorbsSystematicDebuggingDoctrine(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/root-cause-tracing/CATALOG_SKILL.md")
	require.NoError(t, err)

	assert.Contains(t, content, "Capture the exact symptom")
	assert.Contains(t, content, "Compare with a working case")
	assert.Contains(t, content, "write the failing regression test")
	assert.Contains(t, content, "references/root-cause-tracing.md")
	assert.Contains(t, content, "references/hypothesis-testing.md")
}

func TestPromptSurfaceTemplateContracts(t *testing.T) {
	t.Parallel()

	t.Run("wrapper renders prompt body", func(t *testing.T) {
		content := renderPromptSurfaceForTest(t, "commands/command-entry.md.tmpl", "status", "command-status-body", "cursor")
		assert.NotContains(t, content, "{{.", "wrapper render must not leak template variables")
		assert.Contains(t, content, "# Status")
	})

	t.Run("fix body distinguishes reexecution mode", func(t *testing.T) {
		content := renderPromptSurfaceForTest(t, "commands/command-entry.md.tmpl", "fix", "command-fix-body", "claude")
		assert.Contains(t, content, "Ordinary `fix` discovery does not advance lifecycle state")
		assert.Contains(t, content, "S3 task-plan amendments")
		assert.Contains(t, content, "--start-reexecution")
	})

	t.Run("fix body permits multiple fix-stage context origins per reviewer", func(t *testing.T) {
		content := renderPromptSurfaceForTest(t, "commands/command-entry.md.tmpl", "fix", "command-fix-body", "claude")
		// A reviewer's evidence may accumulate more than one fix-stage origin
		// handle (one per fresh-context repair subagent / batch) without
		// invalidating the single review-stage handle.
		assert.Contains(t, content, "A reviewer's evidence may accumulate multiple `context_origin:stage=fix=<repair-subagent-handle>` references")
		assert.Contains(t, content, "one per fresh-context repair subagent / batch")
		assert.Contains(t, content, "without invalidating the single `context_origin:stage=review` handle")
	})

	t.Run("fix and review bodies prefer configured subagent directive", func(t *testing.T) {
		fix := renderPromptSurfaceForTest(t, "commands/command-entry.md.tmpl", "fix", "command-fix-body", "claude")
		assert.Contains(t, fix, "Each repair batch MUST use `contract.subagent` when present")
		assert.Contains(t, fix, "`contract.subagent.engine_boundary`")
		assert.Contains(t, fix, "not a provider capability description")

		review := renderPromptSurfaceForTest(t, "commands/command-entry.md.tmpl", "review", "command-review-body", "claude")
		assert.Contains(t, review, "use `contract.subagent` when present")
		assert.Contains(t, review, "native fresh-context repair subagent")
	})

	t.Run("every prompt surface has matching body partial", func(t *testing.T) {
		partials := promptSurfaceBodyTemplates(t)
		require.Len(t, partials, 22)

		for _, bodyTemplate := range partials {
			commandID := strings.TrimSuffix(strings.TrimPrefix(bodyTemplate, "command-"), "-body")
			t.Run(commandID, func(t *testing.T) {
				content := renderPromptSurfaceForTest(t, "commands/command-entry.md.tmpl", commandID, bodyTemplate, "claude")
				assert.NotContains(t, content, "{{.", "%s must render through the generic wrapper", bodyTemplate)
				assert.Contains(t, content, `surface: "adapter"`)
			})
		}
	})

	t.Run("wave orchestration continues across wave boundaries", func(t *testing.T) {
		content, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", map[string]string{
			"ToolID":  "claude",
			"Trigger": "/slipway:wave-orchestration",
		})
		require.NoError(t, err)
		assert.Contains(t, content, "Do not ask the operator to run `slipway run` merely to cross a")
		assert.Contains(t, content, "Do not call an advancing command between\nwave boundaries")
		assert.Contains(t, content, "Natural execution stop points:")
	})

	t.Run("include helper renders", func(t *testing.T) {
		templateFS := fstest.MapFS{
			"templates/main.tmpl":           &fstest.MapFile{Data: []byte(`before {{ include "demo" . }} after`)},
			"templates/_partials/demo.tmpl": &fstest.MapFile{Data: []byte(`{{ define "demo" }}HELLO {{ .Value }}{{ end }}`)},
		}
		content, err := renderFS(templateFS, "main.tmpl", map[string]string{"Value": "world"})
		require.NoError(t, err)
		assert.Equal(t, "before HELLO world after", content)
	})

	t.Run("include helper nil guard fails closed", func(t *testing.T) {
		var tmplSet *template.Template
		var includeStack []string
		include := newIncludeFunc(&tmplSet, &includeStack)

		_, err := include("demo", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "template set not initialized")
	})

	t.Run("include helper missing template fails", func(t *testing.T) {
		templateFS := fstest.MapFS{
			"templates/main.tmpl": &fstest.MapFile{Data: []byte(`{{ include "missing" . }}`)},
		}
		_, err := renderFS(templateFS, "main.tmpl", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `include "missing": template not found`)
	})

	t.Run("include helper cycle fails", func(t *testing.T) {
		templateFS := fstest.MapFS{
			"templates/main.tmpl":        &fstest.MapFile{Data: []byte(`{{ include "alpha" . }}`)},
			"templates/_partials/a.tmpl": &fstest.MapFile{Data: []byte(`{{ define "alpha" }}{{ include "beta" . }}{{ end }}`)},
			"templates/_partials/b.tmpl": &fstest.MapFile{Data: []byte(`{{ define "beta" }}{{ include "alpha" . }}{{ end }}`)},
		}
		_, err := renderFS(templateFS, "main.tmpl", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cyclic include detected")
	})

	t.Run("include helper depth fails", func(t *testing.T) {
		templateFS := fstest.MapFS{
			"templates/main.tmpl": &fstest.MapFile{Data: []byte(`{{ include "node-00" . }}`)},
		}
		for i := 0; i <= maxIncludeDepth; i++ {
			name := fmt.Sprintf("node-%02d", i)
			body := "leaf"
			if i < maxIncludeDepth {
				body = fmt.Sprintf(`{{ include "node-%02d" . }}`, i+1)
			}
			templateFS[path.Join("templates", "_partials", name+".tmpl")] = &fstest.MapFile{
				Data: []byte(fmt.Sprintf(`{{ define %q }}%s{{ end }}`, name, body)),
			}
		}
		_, err := renderFS(templateFS, "main.tmpl", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nesting depth")
	})

	t.Run("next dispatch content preserved", func(t *testing.T) {
		content := renderPromptSurfaceForTest(t, "commands/command-entry.md.tmpl", "next", "command-next-body", "claude")
		normalized := strings.Join(strings.Fields(content), " ")
		assert.Contains(t, content, "`next_skill.name` is the authoritative governed-host handoff.")
		assert.Contains(t, content, "`slipway run --json`")
		assert.Contains(t, normalized, "`evidence_continuation` with prior authorization sufficient")
		assert.Contains(t, normalized, "run/stage loops still stop for the host to run the skill or review and record evidence")
		assert.Contains(t, normalized, "`security-review` boundaries")
		assert.Contains(t, normalized, "the intake Approved Summary")
		assert.NotContains(t, content, "Tool-Specific Dispatch")
	})

	t.Run("stage commands describe config auto without override flags", func(t *testing.T) {
		for commandID, bodyTemplate := range map[string]string{
			"intake":    "command-intake-body",
			"plan":      "command-plan-body",
			"implement": "command-implement-body",
		} {
			commandID, bodyTemplate := commandID, bodyTemplate
			t.Run(commandID, func(t *testing.T) {
				content := renderPromptSurfaceForTest(t, "commands/command-entry.md.tmpl", commandID, bodyTemplate, "claude")
				assert.Contains(t, content, "Config-level `execution.auto` applies to this stage command")
				assert.Contains(t, content, "there are no per-stage auto override flags")
				assert.Contains(t, content, "`evidence_continuation`")
				assert.Contains(t, content, "skill/review handoffs still stop the loop for host work")
				assert.Contains(t, content, "sensitive/guardrail confirmations")
				assert.NotContains(t, content, "- `--auto`")
				assert.NotContains(t, content, "- `--no-auto`")
			})
		}
	})

	t.Run("codex command skill renders body without prompt transport", func(t *testing.T) {
		content := renderPromptSurfaceForTest(t, "commands/command-skill.md.tmpl", "new", "command-new-body", "codex")
		// The skill surface carries the body partial and skill frontmatter,
		// not the retired global-prompt transport markers.
		assert.Contains(t, content, "# Create Governed Change")
		assert.Contains(t, content, "slipway new")
		assert.Contains(t, content, `surface: "skill"`)
		assert.Contains(t, content, "## Arguments")
		assert.Contains(t, content, "```text\n--json\n```")
		assert.NotContains(t, content, "argument-hint:")
		assert.NotContains(t, content, "$ARGUMENTS")
	})
}

func TestSubagentHostTemplatesExplainEngineBoundaryAndSessionInstructions(t *testing.T) {
	t.Parallel()

	data := map[string]string{"ToolID": "claude", "Trigger": "/slipway:test", "Description": "test"}
	renderedTemplates := []string{
		"skills/wave-orchestration/HOST_SKILL.md.tmpl",
		"skills/spec-compliance-review/HOST_SKILL.md.tmpl",
		"skills/code-quality-review/HOST_SKILL.md.tmpl",
		"skills/independent-review/HOST_SKILL.md.tmpl",
		"skills/security-review/HOST_SKILL.md.tmpl",
		"skills/ship-verification/HOST_SKILL.md.tmpl",
		"skills/workflow/command-reference.md.tmpl",
	}
	for _, name := range renderedTemplates {
		t.Run(name, func(t *testing.T) {
			content, err := Render(name, data)
			require.NoError(t, err)
			assert.Contains(t, content, "`engine_boundary`")
			assert.Contains(t, content, "not a provider capability")
			assert.Contains(t, content, "`session_instructions`")
		})
	}

	readOnlyTemplates := []struct {
		name string
		read func() (string, error)
	}{
		{
			name: "skills/plan-audit/HOST_SKILL.md",
			read: func() (string, error) {
				return Content("skills/plan-audit/HOST_SKILL.md")
			},
		},
		{
			name: "skills/independent-review/CATALOG_SKILL.md",
			read: func() (string, error) {
				return Content("skills/independent-review/CATALOG_SKILL.md")
			},
		},
		{
			name: "skills/security-review/CATALOG_SKILL.md",
			read: func() (string, error) {
				return Content("skills/security-review/CATALOG_SKILL.md")
			},
		},
		{
			name: "skills/spec-compliance-review/HOST_SKILL.md.tmpl",
			read: func() (string, error) {
				return Render("skills/spec-compliance-review/HOST_SKILL.md.tmpl", data)
			},
		},
		{
			name: "skills/code-quality-review/HOST_SKILL.md.tmpl",
			read: func() (string, error) {
				return Render("skills/code-quality-review/HOST_SKILL.md.tmpl", data)
			},
		},
		{
			name: "skills/ship-verification/HOST_SKILL.md.tmpl",
			read: func() (string, error) {
				return Render("skills/ship-verification/HOST_SKILL.md.tmpl", data)
			},
		},
	}
	for _, item := range readOnlyTemplates {
		t.Run(item.name, func(t *testing.T) {
			content, err := item.read()
			require.NoError(t, err)
			flat := strings.Join(strings.Fields(content), " ")
			assert.Contains(t, content, "`engine_boundary`")
			assert.Contains(t, content, "read-only")
			assert.Contains(t, flat, "do not modify files")
			assert.Contains(t, content, "`session_instructions`")
		})
	}
}

func TestSubagentReferenceDocsDescribeBoundaryAndRetiredKeys(t *testing.T) {
	t.Parallel()

	repoRoot := templatesTestRepoRoot(t)
	docs := []string{
		"docs/reference/subagents.md",
		"docs/zh/reference/subagents.md",
		"docs/ja/reference/subagents.md",
	}
	requiredSlots := []string{
		"`default`",
		"`plan_audit`",
		"`executor`",
		"`review`",
		"`fix`",
		"`verify`",
	}
	retiredKeys := []string{
		"`subagent_provider_profiles`",
		"`allowed_skills`",
		"`allowed_mcp_servers`",
		"`tool_policy`",
		"`profile`",
		"`prompt`",
	}
	for _, docPath := range docs {
		t.Run(docPath, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(repoRoot, docPath))
			require.NoError(t, err)
			content := string(data)

			for _, slot := range requiredSlots {
				assert.Contains(t, content, slot)
			}
			assert.Contains(t, content, "`engine_boundary`")
			assert.Contains(t, content, "`read_only")
			assert.Contains(t, content, "`mutation_policy")
			for _, key := range retiredKeys {
				assert.NotContains(t, content, key)
			}
		})
	}
}

func templatesTestRepoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func renderPromptSurfaceForTest(t *testing.T, templateName, commandID, bodyTemplate, toolID string) string {
	t.Helper()

	content, err := Render(templateName, map[string]any{
		"CommandID":    commandID,
		"ToolID":       toolID,
		"Trigger":      "/slipway:" + commandID,
		"Class":        "query",
		"Tier":         "core",
		"Surface":      "adapter",
		"Description":  "test description",
		"BodyTemplate": bodyTemplate,
		"Arguments":    "--json",
	})
	require.NoError(t, err)
	return content
}

func assertNoLedgerTaskEvidenceFlags(t *testing.T, content, surface string) {
	t.Helper()

	for _, forbidden := range []string{
		"--run-summary-version",
		"--task-kind",
		"--target-file",
		"--captured-at",
	} {
		assert.NotContains(t, content, forbidden,
			"%s must not teach ledger-owned evidence task flag %s",
			surface, forbidden)
	}
}

func markdownSection(t *testing.T, content, heading string) string {
	t.Helper()

	start := strings.Index(content, heading)
	require.NotEqual(t, -1, start, "%s section missing", heading)

	section := content[start:]
	afterHeading := section[len(heading):]
	if next := strings.Index(afterHeading, "\n## "); next >= 0 {
		section = section[:len(heading)+next]
	}
	return strings.TrimSpace(section)
}

func promptSurfaceBodyTemplates(t *testing.T) []string {
	t.Helper()

	entries, err := fs.ReadDir(TemplateFS(), "_partials")
	require.NoError(t, err)

	var partials []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "command-") || !strings.HasSuffix(name, "-body.tmpl") {
			continue
		}
		partials = append(partials, strings.TrimSuffix(name, ".tmpl"))
	}
	slices.Sort(partials)
	return partials
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
