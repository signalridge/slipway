package wave

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

func compareNodesByTaskID(a, b Node) int { return cmp.Compare(a.TaskID, b.TaskID) }

type Node struct {
	TaskID         string         `json:"task_id"`
	Objective      string         `json:"objective,omitempty"`
	WaveIndex      int            `json:"wave,omitempty"`
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
	declaredWaves := map[int][]Node{}
	maxWaveIndex := 0
	for _, node := range nodes {
		if err := model.ValidateTaskID(node.TaskID); err != nil {
			return nil, err
		}
		if _, exists := nodeByID[node.TaskID]; exists {
			return nil, fmt.Errorf("duplicate task_id %q", node.TaskID)
		}
		if node.WaveIndex < 1 {
			return nil, fmt.Errorf("task %q missing required wave declaration", node.TaskID)
		}
		nodeByID[node.TaskID] = node
		declaredWaves[node.WaveIndex] = append(declaredWaves[node.WaveIndex], node)
		if node.WaveIndex > maxWaveIndex {
			maxWaveIndex = node.WaveIndex
		}
	}

	for waveIndex := 1; waveIndex <= maxWaveIndex; waveIndex++ {
		layer, exists := declaredWaves[waveIndex]
		if !exists || len(layer) == 0 {
			return nil, fmt.Errorf("wave %d missing from declared wave plan", waveIndex)
		}
	}

	for _, node := range nodes {
		for _, dep := range node.DependsOn {
			dependency, exists := nodeByID[dep]
			if !exists {
				return nil, fmt.Errorf("task %q depends on unknown task %q", node.TaskID, dep)
			}
			if dependency.WaveIndex >= node.WaveIndex {
				return nil, fmt.Errorf("task %q depends on %q in same or later wave", node.TaskID, dep)
			}
		}
	}

	waves := make([]Wave, 0, maxWaveIndex)
	for waveIndex := 1; waveIndex <= maxWaveIndex; waveIndex++ {
		layer := append([]Node(nil), declaredWaves[waveIndex]...)
		slices.SortFunc(layer, compareNodesByTaskID)
		if err := validateWaveStaticConflicts(waveIndex, layer); err != nil {
			return nil, err
		}
		waves = append(waves, Wave{Nodes: layer})
	}
	return waves, nil
}

func validateWaveStaticConflicts(waveIndex int, nodes []Node) error {
	type targetOwner struct {
		target string
		taskID string
	}

	targetOwners := []targetOwner{}
	for _, node := range nodes {
		for _, file := range node.TargetFiles {
			target := normalizeTargetFileForConflict(file)
			for _, existing := range targetOwners {
				if existing.taskID == node.TaskID {
					continue
				}
				if targetFilesConflict(existing.target, target) {
					return fmt.Errorf("wave %d has static target conflict: %q targets %q and %q targets %q", waveIndex, existing.taskID, existing.target, node.TaskID, target)
				}
			}
			targetOwners = append(targetOwners, targetOwner{target: target, taskID: node.TaskID})
		}
	}
	return nil
}

func targetFilesConflict(left, right string) bool {
	return left == right || targetFileContains(left, right) || targetFileContains(right, left)
}

func targetFileContains(parent, child string) bool {
	if parent == "" || child == "" {
		return false
	}
	if parent == "." || parent == "/" {
		return child != parent
	}
	return strings.HasPrefix(child, parent+"/")
}

func normalizeTargetFileForConflict(file string) string {
	file = strings.TrimSpace(file)
	if file == "" {
		return ""
	}
	return strings.ToLower(filepath.ToSlash(filepath.Clean(file)))
}
