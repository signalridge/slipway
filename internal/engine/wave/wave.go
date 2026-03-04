package wave

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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

type ExecutionOptions struct {
	Parallelization   bool `json:"parallelization"`
	MaxRetriesPerTask int  `json:"max_retries_per_task"`
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

type NodeExecutor func(node Node, attempt int) model.TaskRun

type ControlDecider func(checkpoint ControlCheckpoint) ControlDecision

type ExecutionResult struct {
	TaskRuns      map[string]model.TaskRun `json:"task_runs"`
	Checkpoint    *ControlCheckpoint       `json:"checkpoint,omitempty"`
	PivotRequired bool                     `json:"pivot_required,omitempty"`
	Aborted       bool                     `json:"aborted,omitempty"`
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
			TaskKind:    model.TaskKindCode,
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

func ExecutePlan(
	plan []Wave,
	runSummaryVersion int,
	options ExecutionOptions,
	execute NodeExecutor,
	decide ControlDecider,
) (ExecutionResult, error) {
	if runSummaryVersion < 1 {
		return ExecutionResult{}, fmt.Errorf("run_summary_version must be >= 1")
	}
	if options.MaxRetriesPerTask <= 0 {
		options.MaxRetriesPerTask = 2
	}

	result := ExecutionResult{
		TaskRuns: map[string]model.TaskRun{},
	}
	retryCounts := map[string]int{}

	for waveIndex := 0; waveIndex < len(plan); waveIndex++ {
		nodes := append([]Node(nil), plan[waveIndex].Nodes...)
		for {
			runs := executeWaveNodes(nodes, runSummaryVersion, options.Parallelization, execute, retryCounts)
			runs = applyPostWaveConflictResolution(runs)
			mergeTaskRuns(result.TaskRuns, runs)

			nonPassTaskIDs := collectNonPassTaskIDs(runs)
			if len(nonPassTaskIDs) == 0 {
				break
			}

			checkpoint := ControlCheckpoint{
				WaveIndex:      waveIndex,
				NonPassTaskIDs: nonPassTaskIDs,
				AllowedDecisions: []ControlDecision{
					ControlDecisionRetry,
					ControlDecisionSkip,
					ControlDecisionAbortWave,
					ControlDecisionPivot,
				},
			}
			decision := ControlDecisionAbortWave
			if decide != nil {
				decision = decide(checkpoint)
			}

			switch decision {
			case ControlDecisionRetry:
				retryNodes, exhausted := selectRetryNodes(nodes, nonPassTaskIDs, retryCounts, options.MaxRetriesPerTask)
				if exhausted {
					result.Checkpoint = &checkpoint
					return result, nil
				}
				nodes = retryNodes
				continue
			case ControlDecisionSkip:
				markSkippedRuns(result.TaskRuns, nonPassTaskIDs)
			case ControlDecisionAbortWave:
				result.Aborted = true
				result.Checkpoint = &checkpoint
				return result, nil
			case ControlDecisionPivot:
				result.PivotRequired = true
				result.Checkpoint = &checkpoint
				return result, nil
			default:
				return ExecutionResult{}, fmt.Errorf("invalid control decision %q", decision)
			}
			break
		}
	}

	return result, nil
}

func executeWaveNodes(
	nodes []Node,
	runSummaryVersion int,
	parallel bool,
	execute NodeExecutor,
	retryCounts map[string]int,
) []model.TaskRun {
	if len(nodes) == 0 {
		return nil
	}

	runNode := func(node Node) model.TaskRun {
		attempt := retryCounts[node.TaskID] + 1
		run := defaultTaskRun(node, runSummaryVersion)
		if execute != nil {
			executed := execute(node, attempt)
			if strings.TrimSpace(executed.TaskID) != "" {
				run.TaskID = executed.TaskID
			}
			run.ChangedFiles = append([]string(nil), executed.ChangedFiles...)
			run.TargetFiles = append([]string(nil), executed.TargetFiles...)
			run.EvidenceRef = executed.EvidenceRef
			run.Verdict = executed.Verdict
			run.Blockers = uniqueSorted(append(run.Blockers, executed.Blockers...))
		}
		if run.TaskKind == model.TaskKindOther && run.Verdict == model.TaskVerdictPass {
			run.Verdict = model.TaskVerdictIncomplete
			run.Blockers = uniqueSorted(append(run.Blockers, "manual_checkpoint_required"))
		}
		return run
	}

	if !parallel || len(nodes) == 1 {
		runs := make([]model.TaskRun, 0, len(nodes))
		for _, node := range nodes {
			runs = append(runs, runNode(node))
		}
		return runs
	}

	runs := make([]model.TaskRun, len(nodes))
	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			runs[idx] = runNode(nodes[idx])
		}(i)
	}
	wg.Wait()
	slices.SortFunc(runs, func(a, b model.TaskRun) int {
		if a.TaskID < b.TaskID {
			return -1
		}
		if a.TaskID > b.TaskID {
			return 1
		}
		return 0
	})
	return runs
}

func defaultTaskRun(node Node, runSummaryVersion int) model.TaskRun {
	taskKind := node.TaskKind
	if taskKind == "" {
		taskKind = model.TaskKindCode
	}
	return model.TaskRun{
		TaskID:            node.TaskID,
		RunSummaryVersion: runSummaryVersion,
		TaskKind:          taskKind,
		Verdict:           model.TaskVerdictPass,
		TargetFiles:       append([]string(nil), node.TargetFiles...),
		ChangedFiles:      []string{},
		Blockers:          []string{},
	}
}

func applyPostWaveConflictResolution(runs []model.TaskRun) []model.TaskRun {
	if len(runs) < 2 {
		return runs
	}
	results := make([]TaskResult, 0, len(runs))
	for _, run := range runs {
		results = append(results, TaskResult{
			TaskID:       run.TaskID,
			ChangedFiles: append([]string(nil), run.ChangedFiles...),
		})
	}
	conflicts := DetectPostWaveFileOverlap(results)
	if len(conflicts) == 0 {
		return runs
	}

	ownerByFile := map[string]string{}
	conflictedTasks := map[string]struct{}{}
	for _, result := range results {
		for _, file := range result.ChangedFiles {
			if owner, ok := ownerByFile[file]; ok && owner != result.TaskID {
				conflictedTasks[owner] = struct{}{}
				conflictedTasks[result.TaskID] = struct{}{}
				continue
			}
			ownerByFile[file] = result.TaskID
		}
	}

	updated := make([]model.TaskRun, 0, len(runs))
	for _, run := range runs {
		if _, conflicted := conflictedTasks[run.TaskID]; conflicted {
			run.Verdict = model.TaskVerdictBlocked
			run.Blockers = uniqueSorted(append(run.Blockers, conflicts...))
		}
		updated = append(updated, run)
	}
	return updated
}

func collectNonPassTaskIDs(runs []model.TaskRun) []string {
	out := []string{}
	for _, run := range runs {
		if run.Verdict != model.TaskVerdictPass || len(run.Blockers) > 0 {
			out = append(out, run.TaskID)
		}
	}
	return uniqueSorted(out)
}

func mergeTaskRuns(target map[string]model.TaskRun, runs []model.TaskRun) {
	for _, run := range runs {
		key, err := model.BuildTaskRunKey(run.TaskID, run.RunSummaryVersion)
		if err != nil {
			continue
		}
		target[key] = run
	}
}

func selectRetryNodes(
	allNodes []Node,
	nonPassTaskIDs []string,
	retryCounts map[string]int,
	maxRetries int,
) ([]Node, bool) {
	retrySet := map[string]struct{}{}
	for _, taskID := range nonPassTaskIDs {
		retrySet[taskID] = struct{}{}
	}

	retryNodes := []Node{}
	exhausted := false
	for _, node := range allNodes {
		if _, shouldRetry := retrySet[node.TaskID]; !shouldRetry {
			continue
		}
		if retryCounts[node.TaskID] >= maxRetries {
			exhausted = true
			continue
		}
		retryCounts[node.TaskID]++
		retryNodes = append(retryNodes, node)
	}
	if exhausted || len(retryNodes) == 0 {
		return nil, true
	}
	return retryNodes, false
}

func markSkippedRuns(taskRuns map[string]model.TaskRun, taskIDs []string) {
	if len(taskIDs) == 0 {
		return
	}
	taskSet := map[string]struct{}{}
	for _, taskID := range taskIDs {
		taskSet[taskID] = struct{}{}
	}
	for key, run := range taskRuns {
		if _, ok := taskSet[run.TaskID]; !ok {
			continue
		}
		run.Verdict = model.TaskVerdictIncomplete
		run.Blockers = uniqueSorted(append(run.Blockers, "skipped_by_operator"))
		taskRuns[key] = run
	}
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
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
	defer func() {
		_ = f.Close()
	}()

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
