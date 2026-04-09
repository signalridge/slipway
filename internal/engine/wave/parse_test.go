package wave

import (
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

func TestParseTaskPlan_CheckpointType(t *testing.T) {
	md := `# Tasks

- [ ] ` + "`t-01`" + ` Manual task
  - task_kind: other
  - checkpoint_type: human_verify
`

	plan, err := ParseTaskPlan(md)
	require.NoError(t, err)
	nodes := plan.Nodes()
	require.Len(t, nodes, 1)
	assert.Equal(t, "human_verify", nodes[0].CheckpointType)
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
	require.Len(t, waves, 2) // wave 1: t-01 + t-03, wave 2: t-02

	wave1IDs := make([]string, len(waves[0].Nodes))
	for i, n := range waves[0].Nodes {
		wave1IDs[i] = n.TaskID
	}
	assert.Contains(t, wave1IDs, "t-01")
	assert.Contains(t, wave1IDs, "t-03")

	assert.Len(t, waves[1].Nodes, 1)
	assert.Equal(t, "t-02", waves[1].Nodes[0].TaskID)
}
