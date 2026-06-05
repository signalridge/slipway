package tmpl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaveOrchestrationDispatchContractIsolatesTestAuthoring(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/wave-orchestration/SKILL.md.tmpl", map[string]string{
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

func TestTDDGovernanceNamesFrozenTestAuthoringCommitAsRedProof(t *testing.T) {
	t.Parallel()

	content, err := Render("skills/tdd-governance/SKILL.md.tmpl", map[string]string{
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
