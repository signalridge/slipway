package tmpl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThinHostGoalVerificationDelegatesBulkyEvidence(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/goal-verification/SKILL.md.tmpl", map[string]string{
		"ToolID":      "claude",
		"Trigger":     "/slipway:goal-verification",
		"Description": "test",
	})
	require.NoError(t, err)

	flat := thinHostFlatten(content)
	flatLower := strings.ToLower(flat)
	assert.Contains(t, content, "IRON LAW: NO COMPLETION CLAIMS WITHOUT FRESH 3-LEVEL VERIFICATION")
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
	assert.Contains(t, flatContent, "PR #112")
	assert.Contains(t, flatContent, "input_context.codebase_map_doc_states")

	ref, err := Content("skills/wave-orchestration/references/executor-dispatch-reference.md")
	require.NoError(t, err)
	flatRef := thinHostFlatten(ref)
	assert.Contains(t, flatRef, "input_context.codebase_map_dir")
	assert.Contains(t, flatRef, "input_context.codebase_map_docs")
	assert.Contains(t, flatRef, "codebase_map_doc_states")
	assert.Contains(t, flatRef, "relevance/staleness self-check")
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
