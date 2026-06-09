package state

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"gopkg.in/yaml.v3"
)

const WavePlanFileName = "wave-plan.yaml"

func WavePlanPathForRead(root, slug string) string {
	return filepath.Join(verificationDirPathForRead(root, slug), WavePlanFileName)
}

func wavePlanReadPathForChange(root string, change model.Change) (string, error) {
	dir, err := verificationDirForChange(root, change)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, WavePlanFileName), nil
}

func LoadWavePlanForChange(root string, change model.Change) (model.WavePlan, error) {
	displayPath := WavePlanPathForRead(root, change.Slug)
	path, err := wavePlanReadPathForChange(root, change)
	if err != nil {
		return model.WavePlan{}, wrapExecutionSummaryLoadError(displayPath, err)
	}
	return loadWavePlanFromPath(path)
}

func loadWavePlanFromPath(path string) (model.WavePlan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.WavePlan{}, err
	}
	var plan model.WavePlan
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&plan); err != nil {
		return model.WavePlan{}, fmt.Errorf("parse wave plan: %w", err)
	}
	plan.Normalize()
	if err := plan.Validate(); err != nil {
		return model.WavePlan{}, fmt.Errorf("invalid wave plan: %w", err)
	}
	return plan, nil
}

func LoadOptionalWavePlanForChange(root string, change model.Change) (*model.WavePlan, error) {
	plan, err := LoadWavePlanForChange(root, change)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

func SaveWavePlan(root, slug string, plan model.WavePlan) error {
	plan.Normalize()
	if err := plan.Validate(); err != nil {
		return err
	}
	dir, err := resolveVerificationDirForWrite(root, slug)
	if err != nil {
		return fmt.Errorf("resolve wave plan dir for %q: %w", slug, err)
	}
	path := filepath.Join(dir, WavePlanFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := yaml.Marshal(plan)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o644)
}

func MaterializeWavePlan(root string, change model.Change) (model.WavePlan, error) {
	return MaterializeWavePlanAt(root, change, time.Now().UTC())
}

func MaterializeWavePlanAt(root string, change model.Change, generatedAt time.Time) (model.WavePlan, error) {
	hashes, nodes, err := currentTaskPlanHashesAndNodes(root, change)
	if err != nil {
		return model.WavePlan{}, err
	}
	waves, err := wave.PlanWaves(nodes)
	if err != nil {
		return model.WavePlan{}, err
	}
	plan := model.WavePlan{
		Version:                 model.WavePlanVersion,
		GeneratedAt:             generatedAt.UTC(),
		TasksPlanHash:           hashes.Structural,
		TasksPlanStructuralHash: hashes.Structural,
		TasksPlanScopeHash:      hashes.Scope,
		TasksPlanSemanticHash:   hashes.Semantic,
		EffectiveStructuralHash: hashes.Structural,
		TotalTasks:              len(nodes),
		Waves:                   make([]model.WavePlanWave, len(waves)),
	}
	// Forced within-wave parallelism is the default; a project opts out with
	// execution.parallelization: off. A missing/unreadable config defaults to
	// forced.
	forcedParallel := true
	if cfg, cfgErr := model.LoadConfig(ConfigPath(root)); cfgErr == nil {
		forcedParallel = cfg.Execution.ForcedParallel()
	}
	for i, plannedWave := range waves {
		tasks := make([]model.WavePlanTask, len(plannedWave.Nodes))
		for j, node := range plannedWave.Nodes {
			tasks[j] = model.WavePlanTask{
				TaskID:      node.TaskID,
				Objective:   node.Objective,
				DependsOn:   append([]string(nil), node.DependsOn...),
				TargetFiles: append([]string(nil), node.TargetFiles...),
				TaskKind:    node.TaskKind,
			}
		}
		// Mark a multi-task wave parallel so the host dispatches it concurrently
		// by default: wave planning already guarantees these tasks are
		// dependency-free and file-disjoint. The flag is derived here and is not
		// part of the freshness hashes above (which derive from tasks.md).
		plan.Waves[i] = model.WavePlanWave{
			WaveIndex: i + 1,
			Parallel:  forcedParallel && len(tasks) > 1,
			Tasks:     tasks,
		}
	}
	if err := SaveWavePlan(root, change.Slug, plan); err != nil {
		return model.WavePlan{}, err
	}
	return plan, nil
}

func CurrentTasksPlanStructuralState(root string, change model.Change) (string, error) {
	hashes, _, err := currentTaskPlanHashesAndNodes(root, change)
	return hashes.Structural, err
}

func CurrentTasksPlanScopeState(root string, change model.Change) (string, error) {
	hashes, _, err := currentTaskPlanHashesAndNodes(root, change)
	return hashes.Scope, err
}

func currentTaskPlanNodes(root string, change model.Change) (string, []wave.Node, error) {
	hashes, nodes, err := currentTaskPlanHashesAndNodes(root, change)
	return hashes.Semantic, nodes, err
}

type currentTaskPlanHashes struct {
	Semantic   string
	Structural string
	Scope      string
}

func currentTaskPlanHashesAndNodes(root string, change model.Change) (currentTaskPlanHashes, []wave.Node, error) {
	bundleDir, err := GovernedBundleDir(root, change)
	if err != nil {
		return currentTaskPlanHashes{}, nil, err
	}
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		return currentTaskPlanHashes{}, nil, err
	}
	content := string(raw)
	semanticHash, err := wave.TaskPlanSemanticHash(content)
	if err != nil {
		return currentTaskPlanHashes{}, nil, err
	}
	structuralHash, err := wave.TaskPlanStructuralHash(content)
	if err != nil {
		return currentTaskPlanHashes{}, nil, err
	}
	scopeHash, err := wave.TaskPlanScopeHash(content)
	if err != nil {
		return currentTaskPlanHashes{}, nil, err
	}
	taskPlan, err := wave.ParseTaskPlan(content)
	if err != nil {
		return currentTaskPlanHashes{}, nil, err
	}
	return currentTaskPlanHashes{
		Semantic:   semanticHash,
		Structural: structuralHash,
		Scope:      scopeHash,
	}, taskPlan.Nodes(), nil
}

func WaveEvidenceDir(root, slug string) string {
	return filepath.Join(ChangeDir(root, slug), "evidence", "waves")
}

func LoadWaveRuns(root, slug string, runVersion int) ([]model.WaveRun, error) {
	if runVersion < 1 {
		return nil, fmt.Errorf("run_version must be >= 1")
	}
	dir := WaveEvidenceDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	slices.Sort(paths)
	runs := make([]model.WaveRun, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		evidenceRunVersion, err := waveEvidenceRunVersion(raw)
		if err != nil {
			return nil, fmt.Errorf("classify wave run %q: %w", path, err)
		}
		if evidenceRunVersion != runVersion {
			continue
		}
		var run model.WaveRun
		decoder := yaml.NewDecoder(bytes.NewReader(raw))
		decoder.KnownFields(true)
		if err := decoder.Decode(&run); err != nil {
			return nil, fmt.Errorf("parse wave run %q: %w", path, err)
		}
		run.Normalize()
		runs = append(runs, run)
	}
	for i := range runs {
		if err := runs[i].Validate(i + 1); err != nil {
			return nil, err
		}
	}
	return runs, nil
}

func waveEvidenceRunVersion(raw []byte) (int, error) {
	var payload struct {
		RunSummaryVersion int `yaml:"run_summary_version"`
	}
	if err := yaml.Unmarshal(raw, &payload); err != nil {
		return 0, fmt.Errorf("parse wave run: %w", err)
	}
	if payload.RunSummaryVersion < 1 {
		return 0, fmt.Errorf("run_summary_version is required")
	}
	return payload.RunSummaryVersion, nil
}

// WaveTaskLinkageIssues reports wave-run/task-plan mismatches that can arise
// when persisted wave evidence is edited or partially corrupted. Missing task
// refs are allowed for pending/partial waves; cross-wave refs are not.
func WaveTaskLinkageIssues(plan model.WavePlan, runs []model.WaveRun) []string {
	if len(runs) == 0 {
		return nil
	}

	issues := make([]string, 0)
	for _, run := range runs {
		if run.WaveIndex < 1 || run.WaveIndex > len(plan.Waves) {
			continue
		}

		expectedWave := plan.Waves[run.WaveIndex-1]
		expected := make(map[string]struct{}, len(expectedWave.Tasks))
		for _, task := range expectedWave.Tasks {
			expected[strings.TrimSpace(task.TaskID)] = struct{}{}
		}

		seen := make(map[string]int, len(run.TaskRuns))
		unexpected := make([]string, 0)
		duplicates := make([]string, 0)
		for _, ref := range run.TaskRuns {
			taskID := strings.TrimSpace(ref.TaskID)
			if taskID == "" {
				continue
			}
			seen[taskID]++
			if seen[taskID] == 2 {
				duplicates = append(duplicates, taskID)
			}
			if _, ok := expected[taskID]; !ok {
				unexpected = append(unexpected, taskID)
			}
		}

		slices.Sort(unexpected)
		slices.Sort(duplicates)
		unexpected = slices.Compact(unexpected)
		duplicates = slices.Compact(duplicates)

		parts := make([]string, 0, 3)
		if len(unexpected) > 0 {
			parts = append(parts, "unexpected="+strings.Join(unexpected, ","))
		}
		if len(duplicates) > 0 {
			parts = append(parts, "duplicate="+strings.Join(duplicates, ","))
		}
		if run.Verdict == model.WaveVerdictPass && len(run.TaskRuns) != len(expectedWave.Tasks) {
			parts = append(parts, fmt.Sprintf("pass_count=%d/%d", len(run.TaskRuns), len(expectedWave.Tasks)))
		}
		if run.Verdict == model.WaveVerdictPending && len(run.TaskRuns) > 0 {
			parts = append(parts, fmt.Sprintf("pending_count=%d", len(run.TaskRuns)))
		}

		if len(parts) > 0 {
			issues = append(issues, fmt.Sprintf("wave %d: %s", run.WaveIndex, strings.Join(parts, " ")))
		}
	}

	slices.Sort(issues)
	return slices.Compact(issues)
}

func LoadOptionalWaveRuns(root, slug string, runVersion int) ([]model.WaveRun, error) {
	runs, err := LoadWaveRuns(root, slug, runVersion)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return runs, nil
}

func SaveWaveRuns(root, slug string, runVersion int, runs []model.WaveRun) error {
	if runVersion < 1 {
		return fmt.Errorf("run_version must be >= 1")
	}
	slices.SortFunc(runs, func(a, b model.WaveRun) int {
		return a.WaveIndex - b.WaveIndex
	})
	for i := range runs {
		runs[i].Normalize()
		if err := runs[i].Validate(i + 1); err != nil {
			return err
		}
	}
	dir := WaveEvidenceDir(root, slug)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if len(runs) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, run := range runs {
		raw, err := yaml.Marshal(run)
		if err != nil {
			return err
		}
		path := filepath.Join(dir, fmt.Sprintf("wave-%02d.yaml", run.WaveIndex))
		if err := fsutil.WriteFileAtomic(path, raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func ResetWaveExecution(root, slug string) error {
	for _, path := range []string{
		WaveEvidenceDir(root, slug),
		filepath.Join(verificationDirPathForRead(root, slug), WavePlanFileName),
	} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func BuildWaveRuns(plan model.WavePlan, runSummaryVersion int, tasks []model.ExecutionTaskSummary) ([]model.WaveRun, error) {
	if runSummaryVersion < 1 {
		return nil, fmt.Errorf("run_summary_version must be >= 1")
	}
	plan.Normalize()
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	taskByID := make(map[string]model.ExecutionTaskSummary, len(tasks))
	for _, task := range tasks {
		task.Normalize()
		taskByID[task.TaskID] = task
	}
	runs := make([]model.WaveRun, len(plan.Waves))
	for i, plannedWave := range plan.Waves {
		refs := make([]model.TaskRunRef, 0, len(plannedWave.Tasks))
		present := make([]model.ExecutionTaskSummary, 0, len(plannedWave.Tasks))
		for _, task := range plannedWave.Tasks {
			summary, ok := taskByID[task.TaskID]
			if !ok {
				continue
			}
			refs = append(refs, model.TaskRunRef{
				TaskID:            summary.TaskID,
				RunSummaryVersion: runSummaryVersion,
			})
			present = append(present, summary)
		}
		runs[i] = model.WaveRun{
			WaveIndex:         plannedWave.WaveIndex,
			RunSummaryVersion: runSummaryVersion,
			StartedAt:         waveStartedAt(present),
			CompletedAt:       waveCompletedAt(len(plannedWave.Tasks), present),
			TaskRuns:          refs,
			Verdict:           determineWaveVerdict(len(plannedWave.Tasks), present),
		}
		// A wave planned for forced parallel dispatch records its dispatch mode as
		// evidence. The host surfaces a blocker when it has to degrade a parallel
		// wave to sequential, so the loss stays visible.
		if plannedWave.Parallel {
			runs[i].DispatchMode = model.WaveDispatchParallel
		}
	}
	return runs, nil
}

func ResumeWaveIndex(plan model.WavePlan, runs []model.WaveRun) int {
	if len(plan.Waves) == 0 {
		return 0
	}
	runByWave := make(map[int]model.WaveRun, len(runs))
	for _, run := range runs {
		runByWave[run.WaveIndex] = run
	}
	for _, plannedWave := range plan.Waves {
		run, ok := runByWave[plannedWave.WaveIndex]
		if !ok || run.Verdict != model.WaveVerdictPass {
			return plannedWave.WaveIndex
		}
	}
	return 0
}

func determineWaveVerdict(plannedCount int, tasks []model.ExecutionTaskSummary) model.WaveVerdict {
	if plannedCount == 0 || len(tasks) == 0 {
		return model.WaveVerdictPending
	}
	passCount := 0
	nonPassCount := 0
	for _, task := range tasks {
		if task.Verdict == model.TaskVerdictPass && len(task.Blockers) == 0 {
			passCount++
			continue
		}
		nonPassCount++
	}
	switch {
	case len(tasks) < plannedCount:
		return model.WaveVerdictPartial
	case nonPassCount == 0:
		return model.WaveVerdictPass
	case passCount == 0:
		return model.WaveVerdictFail
	default:
		return model.WaveVerdictPartial
	}
}

func waveStartedAt(tasks []model.ExecutionTaskSummary) time.Time {
	var started time.Time
	for _, task := range tasks {
		capturedAt := task.CapturedAt.UTC()
		if capturedAt.IsZero() {
			continue
		}
		if started.IsZero() || capturedAt.Before(started) {
			started = capturedAt
		}
	}
	return started
}

func waveCompletedAt(plannedCount int, tasks []model.ExecutionTaskSummary) time.Time {
	if plannedCount == 0 || len(tasks) < plannedCount {
		return time.Time{}
	}
	var completed time.Time
	for _, task := range tasks {
		capturedAt := task.CapturedAt.UTC()
		if capturedAt.IsZero() {
			return time.Time{}
		}
		if capturedAt.After(completed) {
			completed = capturedAt
		}
	}
	return completed
}

func PlannedTaskIDSet(plan model.WavePlan) map[string]struct{} {
	out := make(map[string]struct{}, plan.TotalTasks)
	for _, taskID := range plan.TaskIDs() {
		out[strings.TrimSpace(taskID)] = struct{}{}
	}
	return out
}
