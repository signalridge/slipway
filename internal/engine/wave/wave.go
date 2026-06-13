package wave

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

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

// PlanWaves computes the wave assignment for the given tasks from their
// declared depends_on edges and their target_files. Nothing is declared by
// the author: wave(task) = 1 for roots, otherwise max(wave of each
// dependency) + 1 before conflict adjustment. Waves are then filled in task-ID
// order from tasks whose dependencies are already in earlier waves, deferring
// any task that conflicts with an already accepted task in the current wave.
// Conflicts are a same-wave-only concern because waves run sequentially. Depth
// is minimal for pure dependency constraints; conflict bumping is deterministic
// greedy placement, not a depth-optimal schedule (that is graph-coloring-hard).
// The plan is deterministic across input orderings.
func PlanWaves(nodes []Node) ([]Wave, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	nodeByID := map[string]Node{}
	for _, node := range nodes {
		if err := model.ValidateTaskID(node.TaskID); err != nil {
			return nil, err
		}
		if _, exists := nodeByID[node.TaskID]; exists {
			return nil, fmt.Errorf("duplicate task_id %q", node.TaskID)
		}
		nodeByID[node.TaskID] = node
	}

	for _, node := range nodes {
		for _, dep := range node.DependsOn {
			if _, exists := nodeByID[dep]; !exists {
				return nil, fmt.Errorf("task %q depends on unknown task %q", node.TaskID, dep)
			}
		}
	}

	ordered, err := topologicalTaskOrder(nodeByID)
	if err != nil {
		return nil, err
	}

	taskIDs := append([]string(nil), ordered...)
	slices.Sort(taskIDs)
	assignedWave := map[string]int{}
	remaining := map[string]struct{}{}
	for _, taskID := range taskIDs {
		remaining[taskID] = struct{}{}
	}

	waves := []Wave{}
	for waveIndex := 1; len(remaining) > 0; waveIndex++ {
		layer := []Node{}
		for _, taskID := range taskIDs {
			if _, pending := remaining[taskID]; !pending {
				continue
			}
			node := nodeByID[taskID]
			if !dependenciesAssignedBeforeWave(node, assignedWave, waveIndex) {
				continue
			}
			if nodeConflictsWithWave(layer, node) {
				continue
			}
			layer = append(layer, node)
			assignedWave[taskID] = waveIndex
			delete(remaining, taskID)
		}

		if len(layer) == 0 {
			return nil, fmt.Errorf("no schedulable tasks for wave %d", waveIndex)
		}
		// Internal invariant: conflict-driven placement above must have
		// produced conflict-free waves; fail closed if it ever does not.
		if err := validateWaveStaticConflicts(waveIndex, layer); err != nil {
			return nil, err
		}
		waves = append(waves, Wave{Nodes: layer})
	}
	return waves, nil
}

func dependenciesAssignedBeforeWave(node Node, assignedWave map[string]int, waveIndex int) bool {
	for _, dep := range node.DependsOn {
		depWave, ok := assignedWave[dep]
		if !ok || depWave >= waveIndex {
			return false
		}
	}
	return true
}

// topologicalTaskOrder returns every task ID in dependency order with
// task-ID-ordered tiebreaks (Kahn's algorithm popping the smallest ready
// ID), so wave assignment is deterministic regardless of input order.
// Duplicate depends_on entries are tolerated; an unresolvable remainder is a
// dependency cycle and fails the plan.
func topologicalTaskOrder(nodeByID map[string]Node) ([]string, error) {
	taskIDs := make([]string, 0, len(nodeByID))
	for taskID := range nodeByID {
		taskIDs = append(taskIDs, taskID)
	}
	slices.Sort(taskIDs)

	pendingDeps := map[string]int{}
	dependents := map[string][]string{}
	for _, taskID := range taskIDs {
		seenDeps := map[string]struct{}{}
		for _, dep := range nodeByID[taskID].DependsOn {
			if _, dup := seenDeps[dep]; dup {
				continue
			}
			seenDeps[dep] = struct{}{}
			pendingDeps[taskID]++
			dependents[dep] = append(dependents[dep], taskID)
		}
	}

	ready := make([]string, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		if pendingDeps[taskID] == 0 {
			ready = append(ready, taskID)
		}
	}

	ordered := make([]string, 0, len(taskIDs))
	for len(ready) > 0 {
		taskID := ready[0]
		ready = ready[1:]
		ordered = append(ordered, taskID)
		for _, dependent := range dependents[taskID] {
			pendingDeps[dependent]--
			if pendingDeps[dependent] == 0 {
				insertAt, _ := slices.BinarySearch(ready, dependent)
				ready = slices.Insert(ready, insertAt, dependent)
			}
		}
	}

	if len(ordered) != len(taskIDs) {
		stuck := make([]string, 0, len(taskIDs)-len(ordered))
		for _, taskID := range taskIDs {
			if pendingDeps[taskID] > 0 {
				stuck = append(stuck, taskID)
			}
		}
		return nil, fmt.Errorf("depends_on cycle detected among tasks %s; remove a circular depends_on reference so every task can be ordered", strings.Join(stuck, ", "))
	}
	return ordered, nil
}

// nodeConflictsWithWave reports whether the candidate's target files overlap
// any target file already owned by another task assigned to the wave.
func nodeConflictsWithWave(occupants []Node, candidate Node) bool {
	for _, candidateFile := range candidate.TargetFiles {
		candidateTarget := normalizeTargetFileForConflict(candidateFile)
		for _, occupant := range occupants {
			for _, occupantFile := range occupant.TargetFiles {
				if targetFilesConflict(normalizeTargetFileForConflict(occupantFile), candidateTarget) {
					return true
				}
			}
		}
	}
	return false
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
	if left == right || targetFileContains(left, right) || targetFileContains(right, left) {
		return true
	}
	return targetPatternConflicts(left, right)
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
	normalized := model.NormalizePublicPath(file)
	if normalized == "" {
		return ""
	}
	// Be conservative across case-insensitive developer filesystems: same-wave
	// targets that differ only by case must not be auto-parallelized.
	return strings.ToLower(normalized)
}

func targetPatternConflicts(left, right string) bool {
	leftPattern := targetHasPatternMeta(left)
	rightPattern := targetHasPatternMeta(right)
	switch {
	case leftPattern && rightPattern:
		return targetPatternPrefixesOverlap(left, right)
	case leftPattern:
		return targetPatternMatches(left, right)
	case rightPattern:
		return targetPatternMatches(right, left)
	default:
		return false
	}
}

func targetHasPatternMeta(target string) bool {
	return strings.ContainsAny(target, "*?[")
}

func targetPatternMatches(pattern, target string) bool {
	if pattern == "" || target == "" {
		return false
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return prefix == "" || target == prefix || targetFileContains(prefix, target)
	}
	if strings.Contains(pattern, "**") {
		return targetOverlapsPatternStaticPrefix(pattern, target)
	}
	matched, err := path.Match(pattern, target)
	if err != nil {
		return true
	}
	return matched
}

func targetPatternPrefixesOverlap(left, right string) bool {
	leftPrefix := targetPatternStaticPrefix(left)
	rightPrefix := targetPatternStaticPrefix(right)
	if leftPrefix == "" || rightPrefix == "" {
		return true
	}
	return leftPrefix == rightPrefix ||
		targetFileContains(leftPrefix, rightPrefix) ||
		targetFileContains(rightPrefix, leftPrefix)
}

func targetOverlapsPatternStaticPrefix(pattern, target string) bool {
	prefix := targetPatternStaticPrefix(pattern)
	return prefix == "" || target == prefix || targetFileContains(prefix, target)
}

func targetPatternStaticPrefix(pattern string) string {
	patternIndex := strings.IndexAny(pattern, "*?[")
	if patternIndex < 0 {
		return strings.TrimSuffix(pattern, "/")
	}
	prefix := pattern[:patternIndex]
	slashIndex := strings.LastIndex(prefix, "/")
	if slashIndex < 0 {
		return ""
	}
	return strings.TrimSuffix(prefix[:slashIndex+1], "/")
}
