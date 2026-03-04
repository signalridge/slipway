package wave

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
)

type Node struct {
	TaskID      string         `json:"task_id"`
	DependsOn   []string       `json:"depends_on,omitempty"`
	TargetFiles []string       `json:"target_files,omitempty"`
	TaskKind    model.TaskKind `json:"task_kind,omitempty"`
	Priority    int            `json:"priority,omitempty"`
	Weight      int            `json:"weight,omitempty"`
}

type Wave struct {
	Nodes []Node `json:"nodes"`
}

type TaskResult struct {
	TaskID       string   `json:"task_id"`
	ChangedFiles []string `json:"changed_files,omitempty"`
}

type RunSummary struct {
	RequestID         string    `json:"request_id"`
	RunSummaryVersion int       `json:"run_summary_version"`
	CompletedTasks    []string  `json:"completed_tasks"`
	NonPassTasks      []string  `json:"non_pass_tasks"`
	CarriedDebt       []string  `json:"carried_debt"`
	EvidenceSet       []string  `json:"evidence_set"`
	OpenBlockers      []string  `json:"open_blockers"`
	FrozenAt          time.Time `json:"frozen_at"`
}

func NormalizeL1Brief(requestID string, checklist []string) []Node {
	nodes := make([]Node, 0, len(checklist))
	short := requestID
	if len(short) > 8 {
		short = short[:8]
	}
	prevID := ""
	for i, item := range checklist {
		taskID := fmt.Sprintf("l1-%s-%02d", short, i+1)
		node := Node{
			TaskID:      taskID,
			TaskKind:    model.TaskKindImplementation,
			TargetFiles: []string{},
		}
		if prevID != "" {
			node.DependsOn = []string{prevID}
		}
		if strings.TrimSpace(item) == "" {
			item = taskID
		}
		_ = item // retained for future checklist metadata expansion.
		nodes = append(nodes, node)
		prevID = taskID
	}
	return nodes
}

func PlanWaves(nodes []Node) ([]Wave, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	nodeByID := map[string]Node{}
	inDegree := map[string]int{}
	edges := map[string][]string{}
	for _, node := range nodes {
		if strings.TrimSpace(node.TaskID) == "" {
			return nil, fmt.Errorf("task_id is required")
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
		slices.SortFunc(layer, func(a, b Node) int {
			if a.TaskID < b.TaskID {
				return -1
			}
			if a.TaskID > b.TaskID {
				return 1
			}
			return 0
		})

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
		slices.SortFunc(packed[i].Nodes, func(a, b Node) int {
			if a.TaskID < b.TaskID {
				return -1
			}
			if a.TaskID > b.TaskID {
				return 1
			}
			return 0
		})
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

func DetectPostWaveFileOverlap(results []TaskResult) []string {
	fileOwner := map[string]string{}
	conflicts := map[string]struct{}{}

	for _, result := range results {
		for _, file := range result.ChangedFiles {
			if owner, exists := fileOwner[file]; exists && owner != result.TaskID {
				conflicts["post_wave_file_conflict:"+file] = struct{}{}
				continue
			}
			fileOwner[file] = result.TaskID
		}
	}

	out := make([]string, 0, len(conflicts))
	for reason := range conflicts {
		out = append(out, reason)
	}
	slices.Sort(out)
	return out
}

func PersistFrozenRunSummary(root string, summary RunSummary) (string, error) {
	if !model.IsUUIDv7(summary.RequestID) {
		return "", fmt.Errorf("request_id must be UUIDv7")
	}
	if summary.RunSummaryVersion < 1 {
		return "", fmt.Errorf("run_summary_version must be >= 1")
	}
	if summary.FrozenAt.IsZero() {
		summary.FrozenAt = time.Now().UTC()
	}

	path := filepath.Join(
		root,
		".spln",
		"evidence",
		"runs",
		summary.RequestID,
		fmt.Sprintf("rv%d.json", summary.RunSummaryVersion),
	)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		return "", err
	}
	return path, nil
}

func NextRunSummaryVersion(latest int) int {
	if latest < 0 {
		return 1
	}
	return latest + 1
}

func UpdateFrozenRunSummaryPointer(root, requestID string, runSummaryVersion int) error {
	if runSummaryVersion < 1 {
		return fmt.Errorf("run_summary_version must be >= 1")
	}

	if change, err := state.LoadChange(root, requestID); err == nil {
		if runSummaryVersion > change.LatestFrozenRunSummaryVersion {
			change.LatestFrozenRunSummaryVersion = runSummaryVersion
			return state.SaveChange(root, change)
		}
		return nil
	}

	admission, err := state.LoadAdmission(root, requestID)
	if err != nil {
		return err
	}
	if runSummaryVersion > admission.LatestFrozenRunSummaryVersion {
		admission.LatestFrozenRunSummaryVersion = runSummaryVersion
		return state.SaveAdmission(root, admission)
	}
	return nil
}
