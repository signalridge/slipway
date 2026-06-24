package tmpl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThinHostShipVerificationDelegatesBulkyEvidence(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/ship-verification/SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:ship-verification",
		"Description": "test",
	})
	require.NoError(t, err)

	flat := thinHostFlatten(content)
	flatLower := strings.ToLower(flat)
	assert.Contains(t, content, "IRON LAW: NO SHIP WITHOUT FRESH, INDEPENDENT, 3-LEVEL TERMINAL VERIFICATION")
	assert.Contains(t, content, "<HARD-GATE>")
	assert.Contains(t, content, "run_version")
	assert.Contains(t, content, "fresh:command_ref")
	assert.Contains(t, content, "high_risk_check:<domain>.safety_baseline=pass")
	assert.Contains(t, flat, "isolated verifier context when supported; otherwise use a bounded structured-summary fallback")
	assert.Contains(t, flatLower, "the host owns the final verdict")
	assert.Contains(t, flat, "`high_risk_check:<domain>.safety_baseline=pass` must be paired with `fresh:command_ref=`")
	assert.Contains(t, flat, "prose-only delegated verdict")
	assert.Contains(t, flat, "private attestation")
	assert.Contains(t, flat, "missing, stale, inconclusive")
	assert.Contains(t, flat, "high_risk_check_missing")
}

func TestThinHostWorktreePreflightKeepsOnlyBoundedBaselineSummary(t *testing.T) {
	t.Parallel()

	content, err := Content("skills/worktree-preflight/SKILL.md")
	require.NoError(t, err)

	flat := thinHostFlatten(content)
	assert.Contains(t, content, "IRON LAW: NO DISCOVERY-REQUIRED GOVERNED EXECUTION WITHOUT A DEDICATED WORKTREE AND A VERIFIED BASELINE")
	assert.Contains(t, content, "<HARD-GATE>")
	assert.Contains(t, content, "run_version")
	assert.Contains(t, content, "worktree_path:")
	assert.Contains(t, content, "worktree_branch:")
	assert.Contains(t, content, "baseline_verify_cmd:")
	assert.Contains(t, flat, "host retains only the baseline command, exit code, bounded summary, and output reference")
	assert.Contains(t, flat, "Write the full baseline output to a referenceable artifact")
	assert.Contains(t, flat, "baseline_output_ref:")
	assert.Contains(t, flat, "isolated context when supported; otherwise bounded summary")
}

func TestThinHostWaveOrchestrationDelegatesCodebaseMapReads(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/wave-orchestration/SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:wave-orchestration",
		"Description": "test",
	})
	require.NoError(t, err)

	readContext := thinHostSectionBetween(content, "## Read Context", "## Read the Authoritative Wave Plan")
	flatContent := thinHostFlatten(content)
	flatReadContext := thinHostFlatten(readContext)

	assert.Contains(t, content, "IRON LAW: NO TASK EXECUTION WITHOUT A GOVERNED PLAN AND CONFLICT DETECTION")
	assert.Contains(t, content, "<HARD-GATE>")
	assert.Contains(t, content, "run_version")
	assert.Contains(t, content, "slipway evidence task")
	assert.NotContains(t, readContext, "If durable codebase mapping exists at `input_context.codebase_map_dir`, read:")
	assert.NotContains(t, flatReadContext, "- `STRUCTURE.md`")
	assert.NotContains(t, flatReadContext, "- `CONVENTIONS.md`")
	assert.NotContains(t, flatReadContext, "- `TESTING.md`")
	assert.NotContains(t, flatReadContext, "- `CONCERNS.md`")
	assert.Contains(t, flatContent, "keep the coordinator context to `input_context.wave_plan`")
	assert.Contains(t, flatContent, "pass `input_context.codebase_map_dir` and relevant `input_context.codebase_map_docs` paths")
	assert.Contains(t, flatContent, "executor-owned relevance/staleness self-check")
	assert.Contains(t, flatContent, "codebase-map relevance/staleness self-check")
	assert.Contains(t, flatContent, "input_context.codebase_map_doc_states")

	ref, err := Content("skills/wave-orchestration/references/executor-dispatch-reference.md")
	require.NoError(t, err)
	flatRef := thinHostFlatten(ref)
	assert.Contains(t, flatRef, "input_context.codebase_map_dir")
	assert.Contains(t, flatRef, "input_context.codebase_map_docs")
	assert.Contains(t, flatRef, "codebase_map_doc_states")
	assert.Contains(t, flatRef, "relevance/staleness self-check")
}

func TestWaveOrchestrationDispatchReferenceDefinesRuntimeAdapters(t *testing.T) {
	t.Parallel()

	ref, err := Content("skills/wave-orchestration/references/executor-dispatch-reference.md")
	require.NoError(t, err)
	flatRef := thinHostFlatten(ref)

	assert.Contains(t, flatRef, "spawn_agent")
	assert.Contains(t, flatRef, "tool_search")
	assert.Contains(t, flatRef, "fork_context: false")
	assert.Contains(t, flatRef, "collect agent IDs")
	assert.Contains(t, flatRef, "wait for all")
	assert.Contains(t, flatRef, "close each agent")
	assert.NotContains(t, flatRef, "codex -q --task")
	assert.Contains(t, flatRef, "dispatch_mode:wave=<wave_index>:degraded_sequential")

	for _, field := range []string{
		"`task_id`",
		"`verdict`",
		"`changed_files`",
		"`test_summary`",
		"`evidence_ref`",
		"`blockers`",
	} {
		assert.Contains(t, flatRef, field)
	}

	assert.Contains(t, flatRef, "capable runtime")
	assert.Contains(t, flatRef, "must not silently execute")
	assert.Contains(t, flatRef, "slipway evidence task")
	assert.Contains(t, flatRef, "must not self-stamp")
	assert.Contains(t, flatRef, "single worktree")
	assert.Contains(t, flatRef, "target-overlap preflight")
	assert.Contains(t, flatRef, "executor_agent:wave=<wave_index>:task=<task_id>:<handle>")
	assert.Contains(t, flatRef, "executor_dispatch_stalled")
	assert.Contains(t, flatRef, "executor_result_missing")
	assert.Contains(t, flatRef, "explicit user authorization")
	assert.Contains(t, flatRef, "post-result changed-file conflict")
	assert.Contains(t, flatRef, ".git/config.lock")
	assert.Contains(t, flatRef, "Do not wrap a spawner workflow inside another subagent")
}

func TestRemainingHeavyHostsUseDiskHandoffContract(t *testing.T) {
	t.Parallel()

	data := map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:test",
		"Description": "test",
	}
	hosts := []struct {
		name    string
		content string
		err     error
	}{
		{
			name:    "research-orchestration",
			content: mustContent(t, "skills/research-orchestration/SKILL.md"),
		},
		{
			name:    "plan-audit",
			content: mustContent(t, "skills/plan-audit/SKILL.md"),
		},
		{
			name:    "intake-clarification",
			content: mustContent(t, "skills/intake-clarification/SKILL.md"),
		},
		{
			name:    "spec-compliance-review",
			content: mustRender(t, "skills/spec-compliance-review/SKILL.md.tmpl", data),
		},
		{
			name:    "code-quality-review",
			content: mustRender(t, "skills/code-quality-review/SKILL.md.tmpl", data),
		},
	}

	for _, host := range hosts {
		t.Run(host.name, func(t *testing.T) {
			t.Parallel()

			flat := thinHostFlatten(host.content)
			assert.Contains(t, flat, "Disk-Handoff Contract")
			assert.Contains(t, flat, "writes bulky artifacts directly to disk under `artifacts/changes/<slug>/`")
			assert.Contains(t, flat, "returns only a short confirmation")
			assert.Contains(t, flat, "confirmation is a claim, not evidence")
			assert.Contains(t, flat, "Slipway CLI owns `run_version`, timestamps, freshness inputs, and verdict stamping")
			assert.Contains(t, flat, "required-reading paths")
			assert.Contains(t, flat, "record the verdict through `slipway evidence skill`")
			assert.Contains(t, flat, "Do not hand-edit")
			assert.NotContains(t, flat, `timestamp: "<ISO-8601-UTC>"`)
			assert.NotContains(t, flat, "run_version: 0")
			assert.NotContains(t, flat, "run_version: <run_summary_version")
		})
	}
}

func mustContent(t *testing.T, name string) string {
	t.Helper()

	content, err := Content(name)
	require.NoError(t, err)
	return content
}

func mustRender(t *testing.T, name string, data map[string]string) string {
	t.Helper()

	content, err := Render(name, data)
	require.NoError(t, err)
	return content
}

func thinHostFlatten(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func thinHostSectionBetween(s, start, end string) string {
	startIndex := strings.Index(s, start)
	if startIndex == -1 {
		return ""
	}
	section := s[startIndex+len(start):]
	endIndex := strings.Index(section, end)
	if endIndex == -1 {
		return section
	}
	return section[:endIndex]
}
