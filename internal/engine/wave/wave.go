package wave

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/signalridge/slipway/internal/model"
)

func compareNodesByTaskID(a, b Node) int { return cmp.Compare(a.TaskID, b.TaskID) }

type Node struct {
	TaskID         string         `json:"task_id"`
	Objective      string         `json:"objective,omitempty"`
	DependsOn      []string       `json:"depends_on,omitempty"`
	TargetFiles    []string       `json:"target_files,omitempty"`
	TaskKind       model.TaskKind `json:"task_kind,omitempty"`
	CheckpointType string         `json:"checkpoint_type,omitempty"`
}

type Wave struct {
	Nodes []Node `json:"nodes"`
}

type TaskResult struct {
	TaskID       string   `json:"task_id"`
	ChangedFiles []string `json:"changed_files,omitempty"`
}

type ControlDecision string

const (
	ControlDecisionRetry     ControlDecision = "retry"
	ControlDecisionSkip      ControlDecision = "skip"
	ControlDecisionAbortWave ControlDecision = "abort_wave"
	ControlDecisionPivot     ControlDecision = "pivot"
)

type ControlCheckpoint struct {
	WaveIndex        int               `json:"wave_index"`
	NonPassTaskIDs   []string          `json:"non_pass_task_ids"`
	AllowedDecisions []ControlDecision `json:"allowed_decisions"`
}

type ExecutionResult struct {
	TaskResults   map[string]model.TaskRun `json:"task_results"`
	Checkpoint    *ControlCheckpoint       `json:"checkpoint,omitempty"`
	PivotRequired bool                     `json:"pivot_required,omitempty"`
	Aborted       bool                     `json:"aborted,omitempty"`
}

func PlanWaves(nodes []Node) ([]Wave, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	nodeByID := map[string]Node{}
	inDegree := map[string]int{}
	edges := map[string][]string{}
	for _, node := range nodes {
		if err := model.ValidateTaskID(node.TaskID); err != nil {
			return nil, err
		}
		if _, exists := nodeByID[node.TaskID]; exists {
			return nil, fmt.Errorf("duplicate task_id %q", node.TaskID)
		}
		nodeByID[node.TaskID] = node
		inDegree[node.TaskID] = 0
	}
	for _, node := range nodes {
		for _, dep := range node.DependsOn {
			if _, exists := nodeByID[dep]; !exists {
				return nil, fmt.Errorf("task %q depends on unknown task %q", node.TaskID, dep)
			}
			inDegree[node.TaskID]++
			edges[dep] = append(edges[dep], node.TaskID)
		}
	}

	processed := map[string]struct{}{}
	waves := []Wave{}
	for len(processed) < len(nodes) {
		layer := []Node{}
		for id, node := range nodeByID {
			if _, done := processed[id]; done {
				continue
			}
			if inDegree[id] == 0 {
				layer = append(layer, node)
			}
		}
		if len(layer) == 0 {
			return nil, fmt.Errorf("dependency cycle detected")
		}
		slices.SortFunc(layer, compareNodesByTaskID)

		waves = append(waves, splitLayerIntoWaves(layer)...)

		for _, node := range layer {
			processed[node.TaskID] = struct{}{}
			for _, next := range edges[node.TaskID] {
				inDegree[next]--
			}
		}
	}

	return waves, nil
}

func splitLayerIntoWaves(layer []Node) []Wave {
	result := []Wave{}
	packed := []Wave{}

	for _, node := range layer {
		if node.TaskKind == model.TaskKindOther || len(node.TargetFiles) == 0 {
			result = append(result, Wave{Nodes: []Node{node}})
			continue
		}

		placed := false
		for i := range packed {
			if hasStaticConflict(packed[i].Nodes, node) {
				continue
			}
			packed[i].Nodes = append(packed[i].Nodes, node)
			placed = true
			break
		}
		if !placed {
			packed = append(packed, Wave{Nodes: []Node{node}})
		}
	}

	for i := range packed {
		slices.SortFunc(packed[i].Nodes, compareNodesByTaskID)
	}
	return append(result, packed...)
}

func hasStaticConflict(existing []Node, candidate Node) bool {
	targets := map[string]struct{}{}
	for _, node := range existing {
		for _, file := range node.TargetFiles {
			targets[file] = struct{}{}
		}
	}
	for _, file := range candidate.TargetFiles {
		if _, exists := targets[file]; exists {
			return true
		}
	}
	return false
}
