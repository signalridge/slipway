package wave

import (
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTaskPlan_CheckboxNativeCapturesCompletionState(t *testing.T) {
	md := `# Tasks

## Task List

- [x] ` + "`t-01`" + ` Setup database schema
  - depends_on: []
  - target_files: ["internal/db/schema.go", "internal/db/migrate.go"]
  - task_kind: code

- [ ] ` + "`t-02`" + ` Implement API handlers
  - depends_on: ["t-01"]
  - target_files: ["internal/api/handler.go"]
  - task_kind: code

- [ ] ` + "`t-03`" + ` Write documentation
  - depends_on: ["t-01", "t-02"]
  - target_files: ["docs/api.md"]
  - task_kind: doc
`

	plan, err := ParseTaskPlan(md)
	require.NoError(t, err)
	require.Len(t, plan.Tasks, 3)
	assert.Equal(t, TaskPlanFormatCheckboxMarkdown, plan.Format)
	assert.True(t, plan.Tasks[0].Completed)
	assert.False(t, plan.Tasks[1].Completed)

	nodes := plan.Nodes()
	require.Len(t, nodes, 3)

	assert.Equal(t, "t-01", nodes[0].TaskID)
	assert.Empty(t, nodes[0].DependsOn)
	assert.Equal(t, []string{"internal/db/schema.go", "internal/db/migrate.go"}, nodes[0].TargetFiles)
	assert.Equal(t, model.TaskKindCode, nodes[0].TaskKind)

	assert.Equal(t, "t-02", nodes[1].TaskID)
	assert.Equal(t, []string{"t-01"}, nodes[1].DependsOn)
	assert.Equal(t, model.TaskKindCode, nodes[1].TaskKind)

	assert.Equal(t, "t-03", nodes[2].TaskID)
	assert.Equal(t, []string{"t-01", "t-02"}, nodes[2].DependsOn)
	assert.Equal(t, model.TaskKindDoc, nodes[2].TaskKind)
}

func TestParseTaskPlan_CommaSeparatedDeps(t *testing.T) {
	md := `# Tasks

- [ ] ` + "`t-01`" + ` First
  - depends_on: t-02, t-03
  - target_files: src/main.go
  - task_kind: code
`

	plan, err := ParseTaskPlan(md)
	require.NoError(t, err)
	nodes := plan.Nodes()
	require.Len(t, nodes, 1)
	assert.Equal(t, []string{"t-02", "t-03"}, nodes[0].DependsOn)
	assert.Equal(t, []string{"src/main.go"}, nodes[0].TargetFiles)
}

func TestParseTaskPlan_EmptyContent(t *testing.T) {
	plan, err := ParseTaskPlan("")
	require.NoError(t, err)
	nodes := plan.Nodes()
	assert.Empty(t, nodes)
}

func TestParseTaskPlan_NoTasks(t *testing.T) {
	md := `# Tasks

## Task List

- [ ] Define implementation tasks.
`
	plan, err := ParseTaskPlan(md)
	require.NoError(t, err)
	nodes := plan.Nodes()
	assert.Empty(t, nodes)
}

func TestParseTaskPlan_RejectsRetiredManualGateMetadata(t *testing.T) {
	retiredKey := strings.Join([]string{"checkpoint", "type"}, "_")
	md := `# Tasks

- [ ] ` + "`t-01`" + ` Manual task
  - task_kind: other
  - ` + retiredKey + `: human_verify
`

	_, err := ParseTaskPlan(md)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown metadata key")
	assert.Contains(t, err.Error(), retiredKey)
}

func TestParseTaskPlan_AcceptsEvidenceAndAcceptanceMetadata(t *testing.T) {
	md := `# Tasks

- [ ] ` + "`t-01`" + ` Parser-compatible audit task
  - target_files: ["internal/engine/wave/parse.go"]
  - task_kind: code
  - evidence: verdict
  - acceptance: parser accepts evidence and acceptance metadata
`

	plan, err := ParseTaskPlan(md)
	require.NoError(t, err)
	require.Len(t, plan.Tasks, 1)
	assert.Equal(t, "verdict", plan.Tasks[0].Evidence)
	assert.Equal(t, "parser accepts evidence and acceptance metadata", plan.Tasks[0].Acceptance)
}

func TestTaskPlanSemanticHashIncludesEvidenceAndAcceptance(t *testing.T) {
	base := `# Tasks

- [ ] ` + "`t-01`" + ` Parser-compatible audit task
  - target_files: ["internal/engine/wave/parse.go"]
  - task_kind: code
  - evidence: verdict
  - acceptance: first acceptance
`
	changed := `# Tasks

- [ ] ` + "`t-01`" + ` Parser-compatible audit task
  - target_files: ["internal/engine/wave/parse.go"]
  - task_kind: code
  - evidence: artifact
  - acceptance: second acceptance
`

	baseHash, err := TaskPlanSemanticHash(base)
	require.NoError(t, err)
	changedHash, err := TaskPlanSemanticHash(changed)
	require.NoError(t, err)
	assert.NotEqual(t, baseHash, changedHash)
}

func TestParseTaskPlan_RejectsDuplicateTaskID(t *testing.T) {
	md := `# Tasks

- [ ] ` + "`t-01`" + ` First task
  - task_kind: code
  - target_files: ["a.go"]

- [ ] ` + "`t-01`" + ` Duplicate task
  - task_kind: code
  - target_files: ["b.go"]
`

	_, err := ParseTaskPlan(md)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate task_id")
	assert.Contains(t, err.Error(), "t-01")
}

func TestParseTaskPlan_RejectsDuplicateMetadataKey(t *testing.T) {
	md := `# Tasks

- [ ] ` + "`t-01`" + ` Task with dup key
  - task_kind: code
  - target_files: ["a.go"]
  - task_kind: test
`

	_, err := ParseTaskPlan(md)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate metadata key")
	assert.Contains(t, err.Error(), "task_kind")
}

func TestParseTaskPlan_RejectsUnknownMetadataKey(t *testing.T) {
	md := `# Tasks

- [ ] ` + "`t-01`" + ` Task with unknown key
  - task_kind: code
  - target_files: ["a.go"]
  - some_random_key: value
`

	_, err := ParseTaskPlan(md)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown metadata key")
	assert.Contains(t, err.Error(), "some_random_key")
}

// TestParseTaskPlan_RejectsRetiredWaveKey pins the wave-key retirement
// contract: ParseTaskPlan stays a pure format parser, and a task carrying a
// `- wave:` metadata line fails parsing with a dedicated retirement error.
//
// Asserted error-message contract (the implementation must satisfy ALL):
//  1. contains the offending task ID (here "t-02"),
//  2. contains the word "wave" (case-insensitive),
//  3. contains at least ONE of "delete", "remove", or "depends_on"
//     (case-insensitive) — i.e. the remediation: delete the wave: line and
//     declare real depends_on for intentional ordering, because the engine
//     now assigns waves from depends_on and target_files.
func TestParseTaskPlan_RejectsRetiredWaveKey(t *testing.T) {
	md := `# Tasks

- [ ] ` + "`t-01`" + ` Healthy task without wave metadata
  - depends_on: []
  - target_files: ["a.go"]
  - task_kind: code

- [ ] ` + "`t-02`" + ` Task still carrying retired wave metadata
  - wave: 2
  - depends_on: ["t-01"]
  - target_files: ["b.go"]
  - task_kind: code
`

	_, err := ParseTaskPlan(md)
	require.Error(t, err, "wave: metadata is retired and must fail parsing")

	msg := err.Error()
	lowered := strings.ToLower(msg)
	assert.Contains(t, msg, "t-02", "retirement error must name the offending task ID")
	assert.Contains(t, lowered, "wave", "retirement error must name the retired wave key")
	hasRemediation := strings.Contains(lowered, "delete") ||
		strings.Contains(lowered, "remove") ||
		strings.Contains(lowered, "depends_on")
	assert.True(t, hasRemediation,
		"retirement error must carry remediation containing one of %q, %q, %q; got: %q",
		"delete", "remove", "depends_on", msg)
}

func TestParseTaskPlan_IntegrationWithPlanWaves(t *testing.T) {
	md := `# Tasks

- [ ] ` + "`t-01`" + ` Foundation
  - target_files: ["a.go"]
  - task_kind: code

- [ ] ` + "`t-02`" + ` Depends on t-01
  - depends_on: ["t-01"]
  - target_files: ["b.go"]
  - task_kind: code

- [ ] ` + "`t-03`" + ` Independent
  - target_files: ["c.go"]
  - task_kind: code
`

	plan, err := ParseTaskPlan(md)
	require.NoError(t, err)
	nodes := plan.Nodes()
	require.Len(t, nodes, 3)

	waves, err := PlanWaves(nodes)
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"t-01", "t-03"},
		{"t-02"},
	}, waveTaskIDGroups(waves))
}
