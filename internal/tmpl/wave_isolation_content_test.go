package tmpl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaveOrchestrationDispatchContractIsolatesTestAuthoring(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "codex",
		"Trigger":     "/slipway:wave-orchestration",
		"Description": "test",
	})
	require.NoError(t, err)

	normalized := strings.ToLower(content)
	assert.Contains(t, normalized, "dispatch contract")
	assert.Contains(t, normalized, "test-authoring")
	assert.Contains(t, normalized, "spec")
	assert.Contains(t, normalized, "acceptance")
	assert.Contains(t, normalized, "public api")
	assert.Contains(t, normalized, "never the implementation")
	assert.Contains(t, normalized, "freeze")
	assert.Contains(t, normalized, "task_kind=test")
	assert.Contains(t, normalized, "task_kind=code")
	assert.Contains(t, normalized, "depends_on")
	assert.NotContains(t, normalized, "engine rejects")
	assert.NotContains(t, normalized, "engine-level rejection")
}

// TestWaveOrchestrationDocumentsEngineSafetyModelAndBlockers asserts the
// generated wave-orchestration host surface (rendered skill + executor-dispatch
// reference) declares the single shared-worktree target/changed-file safety
// model and names the four engine-enforced wave blockers, while making neither
// an engine-spawn claim nor using the forbidden rejection markers (REQ-007).
func TestWaveOrchestrationDocumentsEngineSafetyModelAndBlockers(t *testing.T) {
	t.Parallel()

	skill, err := Render("skills/wave-orchestration/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "codex",
		"Trigger":     "/slipway:wave-orchestration",
		"Description": "test",
	})
	require.NoError(t, err)

	ref, err := Content("skills/wave-orchestration/references/executor-dispatch-reference.md")
	require.NoError(t, err)

	// The four engine-enforced blocker reason codes must appear in both the
	// rendered skill and the executor-dispatch reference.
	blockerCodes := []string{
		"task_changed_file_scope_escape",
		"parallel_wave_changed_file_overlap",
		"dispatch_mode_absent_on_started_parallel_wave",
		"executor_agent_missing",
	}
	for _, surface := range []struct {
		name    string
		content string
	}{
		{"HOST_SKILL.md.tmpl", skill},
		{"executor-dispatch-reference.md", ref},
	} {
		lower := strings.ToLower(surface.content)
		for _, code := range blockerCodes {
			assert.Containsf(t, lower, code, "%s must name engine blocker %q", surface.name, code)
		}

		// Single shared-worktree safety model: accurate target_files + exhaustive
		// changed_files are the boundary; the engine records/gates but does not spawn.
		assert.Containsf(t, lower, "shared", "%s must describe the shared worktree safety model", surface.name)
		assert.Containsf(t, lower, "target_files", "%s must name target_files as the safety boundary", surface.name)
		assert.Containsf(t, lower, "changed_files", "%s must name changed_files as the safety boundary", surface.name)
		assert.Containsf(t, lower, "does not spawn", "%s must state the engine does not spawn agents", surface.name)
		assert.Containsf(t, lower, "fails closed", "%s must state the change fails closed on bad evidence", surface.name)

		// The forbidden rejection markers must not leak into either surface.
		assert.NotContainsf(t, lower, "engine rejects", "%s must avoid the forbidden marker", surface.name)
		assert.NotContainsf(t, lower, "engine-level rejection", "%s must avoid the forbidden marker", surface.name)
	}
}

func TestTDDGovernanceNamesFrozenTestAuthoringCommitAsRedProof(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/tdd-governance/HOST_SKILL.md.tmpl", map[string]string{
		"ToolID":      "codex",
		"Trigger":     "/slipway:tdd-governance",
		"Description": "test",
	})
	require.NoError(t, err)

	normalized := strings.ToLower(content)
	assert.Contains(t, normalized, "git history verification protocol")
	assert.Contains(t, normalized, "frozen")
	assert.Contains(t, normalized, "test-authoring commit")
	assert.Contains(t, normalized, "red proof")
}
