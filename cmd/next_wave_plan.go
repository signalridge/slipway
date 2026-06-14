package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// buildWavePlan returns the authoritative persisted wave plan during governed
// execution, and falls back to a derived preview from tasks.md before the
// authoritative plan exists.
func buildWavePlan(root string, change *model.Change, artifactBundle string) *wavePlanView {
	if change != nil && change.CurrentState == model.StateS2Execute {
		return authoritativeWavePlanView(root, *change)
	}
	return derivedWavePlanView(root, artifactBundle)
}

func authoritativeWavePlanView(root string, change model.Change) *wavePlanView {
	plan, err := state.LoadOptionalWavePlanForChange(root, change)
	switch {
	case err == nil && plan != nil:
		return wavePlanViewFromModel(
			root,
			*plan,
			state.EffectiveForcedParallel(root),
			sourceTargetFilesByTaskForChange(root, change),
		)
	case err == nil:
		return &wavePlanView{ParseError: "authoritative wave-plan.yaml is missing; run `slipway repair`"}
	default:
		return &wavePlanView{ParseError: fmt.Sprintf("failed to load authoritative wave-plan.yaml: %v", err)}
	}
}

// derivedWavePlanView parses tasks.md and computes dependency-ordered waves.
// Returns nil if tasks.md is missing or empty (not an error — plan may not exist yet).
func derivedWavePlanView(root, artifactBundle string) *wavePlanView {
	if artifactBundle == "" {
		return nil
	}
	tasksPath := filepath.Join(resolveInputContextPath(root, root, artifactBundle), "tasks.md")
	content, err := os.ReadFile(tasksPath) // #nosec G304 -- path is resolved from CLI/project authority before this read.
	if err != nil {
		return nil
	}

	plan, err := wave.ParseTaskPlan(string(content))
	if err != nil {
		return &wavePlanView{ParseError: err.Error()}
	}
	nodes := plan.Nodes()
	if len(nodes) == 0 {
		return nil
	}
	rawTargetFilesByTask := sourceTargetFilesByTask(string(content))

	waves, err := wave.PlanWaves(nodes)
	if err != nil {
		return &wavePlanView{
			TotalTasks: len(nodes),
			ParseError: err.Error(),
		}
	}

	forcedParallel := state.EffectiveForcedParallel(root)
	planView := &wavePlanView{
		WaveCount: len(waves),
		Waves:     make([]waveView, len(waves)),
	}

	for i, w := range waves {
		tasks := make([]waveTaskView, len(w.Nodes))
		for j, n := range w.Nodes {
			tasks[j] = waveTaskView{
				TaskID:      n.TaskID,
				Objective:   n.Objective,
				DependsOn:   n.DependsOn,
				TargetFiles: n.TargetFiles,
				TaskKind:    string(n.TaskKind),
			}
		}
		planView.Waves[i] = waveView{
			WaveIndex: i + 1,
			Parallel:  forcedParallel && len(w.Nodes) > 1,
			Tasks:     tasks,
		}
		planView.TotalTasks += len(w.Nodes)
	}

	// View-only narrowing advisories (REQ-006): the derived path already holds
	// the planner nodes, but parsing normalized target_files may strip explicit
	// directory markers such as "internal/newpkg/". Rehydrate equivalent raw
	// targets from tasks.md before analysis so derived and authoritative views
	// preserve the same directory intent. Excluded from wave-plan.yaml and
	// freshness hashes by construction (view layer only).
	planView.Advisories = wave.AnalyzeWaveNarrowingCauses(
		analyzerNodesFromSourceTargets(root, nodes, rawTargetFilesByTask),
	)

	return planView
}

func wavePlanViewFromModel(
	root string,
	plan model.WavePlan,
	forcedParallel bool,
	sourceTargetFilesByTask ...map[string][]string,
) *wavePlanView {
	plan = state.ApplyEffectiveParallel(plan, forcedParallel)
	if len(plan.Waves) == 0 {
		return nil
	}

	view := &wavePlanView{
		TotalTasks: plan.TotalTasks,
		WaveCount:  len(plan.Waves),
		Waves:      make([]waveView, len(plan.Waves)),
	}
	// Rebuild planner nodes from the persisted plan so the view-only narrowing
	// advisories (REQ-006) can be computed on the authoritative from-model path
	// exactly as on the derived path.
	analyzerNodes := make([]wave.Node, 0, plan.TotalTasks)
	for i, plannedWave := range plan.Waves {
		tasks := make([]waveTaskView, len(plannedWave.Tasks))
		for j, task := range plannedWave.Tasks {
			tasks[j] = waveTaskView{
				TaskID:      task.TaskID,
				Objective:   task.Objective,
				DependsOn:   append([]string(nil), task.DependsOn...),
				TargetFiles: append([]string(nil), task.TargetFiles...),
				TaskKind:    string(task.TaskKind),
			}
			node := wave.Node{
				TaskID:    task.TaskID,
				Objective: task.Objective,
				DependsOn: append([]string(nil), task.DependsOn...),
				TargetFiles: analyzerTargetFilesForSource(
					root,
					task.TaskID,
					task.TargetFiles,
					sourceTargetFilesByTask...,
				),
				TaskKind: task.TaskKind,
			}
			analyzerNodes = append(analyzerNodes, node)
		}
		view.Waves[i] = waveView{
			WaveIndex: plannedWave.WaveIndex,
			Parallel:  plannedWave.Parallel,
			Tasks:     tasks,
		}
	}
	view.Advisories = wave.AnalyzeWaveNarrowingCauses(analyzerNodes)
	return view
}

func analyzerNodesFromSourceTargets(
	root string,
	nodes []wave.Node,
	sourceTargetFilesByTask map[string][]string,
) []wave.Node {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]wave.Node, len(nodes))
	for i, node := range nodes {
		out[i] = node
		out[i].DependsOn = append([]string(nil), node.DependsOn...)
		out[i].TargetFiles = analyzerTargetFilesForSource(
			root,
			node.TaskID,
			node.TargetFiles,
			sourceTargetFilesByTask,
		)
	}
	return out
}

func analyzerTargetFilesForSource(
	root string,
	taskID string,
	targetFiles []string,
	sourceTargetFilesByTask ...map[string][]string,
) []string {
	if len(sourceTargetFilesByTask) > 0 {
		if sourceTargets, ok := sourceTargetFilesByTask[0][taskID]; ok &&
			normalizedTargetFilesEqual(sourceTargets, targetFiles) {
			return analyzerTargetFilesFromModel(root, sourceTargets)
		}
	}
	return analyzerTargetFilesFromModel(root, targetFiles)
}

func analyzerTargetFilesFromModel(root string, targetFiles []string) []string {
	out := append([]string(nil), targetFiles...)
	if root == "" {
		return out
	}
	for i, target := range out {
		if targetIsExistingDirectory(root, target) && !strings.HasSuffix(target, "/") {
			out[i] = target + "/"
		}
	}
	return out
}

func targetIsExistingDirectory(root, target string) bool {
	normalized := model.NormalizePublicPath(target)
	if normalized == "" || model.PublicPathIsAbs(normalized) || model.PublicPathHasParentTraversal(normalized) {
		return false
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(normalized)))
	return err == nil && info.IsDir()
}

func sourceTargetFilesByTaskForChange(root string, change model.Change) map[string][]string {
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(bundleDir, "tasks.md")) // #nosec G304 -- path is resolved from governed change authority.
	if err != nil {
		return nil
	}
	return sourceTargetFilesByTask(string(raw))
}

func sourceTargetFilesByTask(content string) map[string][]string {
	result := map[string][]string{}
	var currentTaskID string
	inCodeBlock := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}
		if taskID, ok := taskIDFromCheckboxLine(line); ok {
			currentTaskID = taskID
			continue
		}
		if currentTaskID == "" {
			continue
		}
		key, value := metadataKeyValue(trimmed)
		if key != "target_files" {
			continue
		}
		result[currentTaskID] = parseRawStringList(value)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func taskIDFromCheckboxLine(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !(strings.HasPrefix(trimmed, "- [") || strings.HasPrefix(trimmed, "* [")) {
		return "", false
	}
	closing := strings.Index(trimmed, "]")
	if closing < 0 || closing+1 >= len(trimmed) {
		return "", false
	}
	rest := strings.TrimSpace(trimmed[closing+1:])
	if strings.HasPrefix(rest, "`") {
		if idx := strings.Index(rest[1:], "`"); idx >= 0 {
			return strings.TrimSpace(rest[1 : idx+1]), true
		}
	}
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "", true
	}
	return strings.Trim(parts[0], "`"), true
}

func metadataKeyValue(line string) (string, string) {
	switch {
	case strings.HasPrefix(line, "- "):
		line = strings.TrimPrefix(line, "- ")
	case strings.HasPrefix(line, "* "):
		line = strings.TrimPrefix(line, "* ")
	default:
		return "", ""
	}

	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "**") {
		line = strings.TrimPrefix(line, "**")
		if idx := strings.Index(line, "**"); idx > 0 {
			key := line[:idx]
			rest := strings.TrimPrefix(line[idx:], "**")
			line = key + rest
		}
	}
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", ""
	}
	key := strings.Trim(strings.TrimSpace(line[:idx]), "`")
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, " ", "_"), "-", "_"))
	return key, strings.TrimSpace(line[idx+1:])
}

func parseRawStringList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "[]" || strings.EqualFold(value, "none") {
		return nil
	}
	if strings.HasPrefix(value, "[") {
		var items []string
		if err := json.Unmarshal([]byte(value), &items); err == nil {
			return items
		}
		value = strings.Trim(value, "[]")
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), "`\"'")
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func normalizedTargetFilesEqual(sourceTargets, planTargets []string) bool {
	normalizedSource := normalizeTargetFileList(sourceTargets)
	normalizedPlan := normalizeTargetFileList(planTargets)
	return slices.Equal(normalizedSource, normalizedPlan)
}

func normalizeTargetFileList(targets []string) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		normalized := model.NormalizePublicPath(target)
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	slices.Sort(out)
	return out
}
