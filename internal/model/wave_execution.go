package model

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

const WavePlanVersion = 1

type WavePlan struct {
	Version                 int            `yaml:"version" json:"version"`
	GeneratedAt             time.Time      `yaml:"generated_at" json:"generated_at"`
	TasksPlanHash           string         `yaml:"tasks_plan_hash,omitempty" json:"tasks_plan_hash,omitempty"`
	TasksPlanStructuralHash string         `yaml:"tasks_plan_structural_hash,omitempty" json:"tasks_plan_structural_hash,omitempty"`
	TasksPlanScopeHash      string         `yaml:"tasks_plan_scope_hash,omitempty" json:"tasks_plan_scope_hash,omitempty"`
	TasksPlanSemanticHash   string         `yaml:"tasks_plan_semantic_hash,omitempty" json:"tasks_plan_semantic_hash,omitempty"`
	EffectiveStructuralHash string         `yaml:"effective_structural_hash,omitempty" json:"effective_structural_hash,omitempty"`
	TotalTasks              int            `yaml:"total_tasks" json:"total_tasks"`
	Waves                   []WavePlanWave `yaml:"waves,omitempty" json:"waves,omitempty"`
}

type WavePlanWave struct {
	WaveIndex int `yaml:"wave_index" json:"wave_index"`
	// Parallel marks a wave whose tasks are dependency-free and file-disjoint
	// (guaranteed by wave planning), so the host is expected to dispatch them
	// concurrently by default. Derived at materialization from the task count and
	// the effective parallelization mode; it is intentionally excluded from the
	// wave-plan freshness hashes.
	Parallel bool           `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	Tasks    []WavePlanTask `yaml:"tasks,omitempty" json:"tasks,omitempty"`
}

type WavePlanTask struct {
	TaskID      string   `yaml:"task_id" json:"task_id"`
	Objective   string   `yaml:"objective,omitempty" json:"objective,omitempty"`
	DependsOn   []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	TargetFiles []string `yaml:"target_files,omitempty" json:"target_files,omitempty"`
	TaskKind    TaskKind `yaml:"task_kind,omitempty" json:"task_kind,omitempty"`
}

func (p *WavePlan) Normalize() {
	if p.Version == 0 {
		p.Version = WavePlanVersion
	}
	if p.Waves == nil {
		p.Waves = []WavePlanWave{}
	}
	if !p.GeneratedAt.IsZero() {
		p.GeneratedAt = p.GeneratedAt.Round(0).UTC()
	}
	p.TasksPlanHash = strings.TrimSpace(p.TasksPlanHash)
	p.TasksPlanStructuralHash = strings.TrimSpace(p.TasksPlanStructuralHash)
	p.TasksPlanScopeHash = strings.TrimSpace(p.TasksPlanScopeHash)
	p.TasksPlanSemanticHash = strings.TrimSpace(p.TasksPlanSemanticHash)
	p.EffectiveStructuralHash = strings.TrimSpace(p.EffectiveStructuralHash)
	if p.TasksPlanStructuralHash == "" {
		p.TasksPlanStructuralHash = p.TasksPlanHash
	}
	if p.EffectiveStructuralHash == "" {
		p.EffectiveStructuralHash = p.TasksPlanStructuralHash
	}
	if p.TasksPlanHash == "" {
		p.TasksPlanHash = p.EffectiveStructuralHash
	}
	total := 0
	for i := range p.Waves {
		p.Waves[i].Normalize(i + 1)
		total += len(p.Waves[i].Tasks)
	}
	if p.TotalTasks == 0 {
		p.TotalTasks = total
	}
}

func (p WavePlan) Validate() error {
	if p.Version != WavePlanVersion {
		return fmt.Errorf("version must be %d", WavePlanVersion)
	}
	if p.GeneratedAt.IsZero() {
		return fmt.Errorf("generated_at is required")
	}
	if p.TotalTasks < 0 {
		return fmt.Errorf("total_tasks must be >= 0")
	}
	seen := map[string]struct{}{}
	total := 0
	for i, wave := range p.Waves {
		if err := wave.Validate(i+1, seen); err != nil {
			return fmt.Errorf("waves[%d]: %w", i, err)
		}
		total += len(wave.Tasks)
	}
	if total != p.TotalTasks {
		return fmt.Errorf("total_tasks must match planned tasks (%d != %d)", p.TotalTasks, total)
	}
	return nil
}

func (p WavePlan) TaskIDs() []string {
	out := make([]string, 0, p.TotalTasks)
	for _, wave := range p.Waves {
		for _, task := range wave.Tasks {
			out = append(out, task.TaskID)
		}
	}
	return out
}

func (p WavePlan) WaveIndexForTask(taskID string) int {
	needle := strings.TrimSpace(taskID)
	if needle == "" {
		return 0
	}
	for _, wave := range p.Waves {
		for _, task := range wave.Tasks {
			if task.TaskID == needle {
				return wave.WaveIndex
			}
		}
	}
	return 0
}

func (w *WavePlanWave) Normalize(index int) {
	if w.WaveIndex == 0 {
		w.WaveIndex = index
	}
	if w.Tasks == nil {
		w.Tasks = []WavePlanTask{}
	}
	for i := range w.Tasks {
		w.Tasks[i].Normalize()
	}
	slices.SortFunc(w.Tasks, func(a, b WavePlanTask) int {
		return strings.Compare(a.TaskID, b.TaskID)
	})
}

func (w WavePlanWave) Validate(expectedIndex int, seen map[string]struct{}) error {
	if w.WaveIndex != expectedIndex {
		return fmt.Errorf("wave_index must be %d", expectedIndex)
	}
	if w.Parallel && len(w.Tasks) < 2 {
		return fmt.Errorf("parallel wave must contain at least 2 tasks")
	}
	for i, task := range w.Tasks {
		if err := task.Validate(); err != nil {
			return fmt.Errorf("tasks[%d]: %w", i, err)
		}
		if _, exists := seen[task.TaskID]; exists {
			return fmt.Errorf("duplicate task_id %q", task.TaskID)
		}
		seen[task.TaskID] = struct{}{}
	}
	return nil
}

func (t *WavePlanTask) Normalize() {
	if t.DependsOn == nil {
		t.DependsOn = []string{}
	}
	if t.TargetFiles == nil {
		t.TargetFiles = []string{}
	}
	slices.Sort(t.DependsOn)
	slices.Sort(t.TargetFiles)
}

func (t WavePlanTask) Validate() error {
	if err := ValidateTaskID(t.TaskID); err != nil {
		return err
	}
	if t.TaskKind != "" && !t.TaskKind.IsValid() {
		return fmt.Errorf("invalid task_kind %q", t.TaskKind)
	}
	return nil
}

type TaskRunRef struct {
	TaskID            string `yaml:"task_id" json:"task_id"`
	RunSummaryVersion int    `yaml:"run_summary_version" json:"run_summary_version"`
}

func (r TaskRunRef) Validate() error {
	if err := ValidateTaskID(r.TaskID); err != nil {
		return err
	}
	if r.RunSummaryVersion < 1 {
		return fmt.Errorf("run_summary_version must be >= 1")
	}
	return nil
}

type WaveVerdict string

const (
	WaveVerdictPending WaveVerdict = "pending"
	WaveVerdictPass    WaveVerdict = "pass"
	WaveVerdictFail    WaveVerdict = "fail"
	WaveVerdictPartial WaveVerdict = "partial"
)

func (v WaveVerdict) IsValid() bool {
	switch v {
	case WaveVerdictPending, WaveVerdictPass, WaveVerdictFail, WaveVerdictPartial:
		return true
	default:
		return false
	}
}

// WaveDispatchMode records how a wave's tasks were actually dispatched, so a
// host that could not run a parallel-eligible wave concurrently records the
// degradation instead of losing it silently.
type WaveDispatchMode string

const (
	WaveDispatchParallel           WaveDispatchMode = "parallel"
	WaveDispatchDegradedSequential WaveDispatchMode = "degraded_sequential"

	WaveDispatchReferencePrefix = "dispatch_mode:wave="
)

func (m WaveDispatchMode) IsValid() bool {
	switch m {
	case WaveDispatchParallel, WaveDispatchDegradedSequential:
		return true
	default:
		return false
	}
}

// WaveDispatchModesFromVerification extracts structured per-wave dispatch
// evidence from wave-orchestration verification references and notes.
func WaveDispatchModesFromVerification(record VerificationRecord) (map[int]WaveDispatchMode, error) {
	modes := map[int]WaveDispatchMode{}
	for _, ref := range record.References {
		if err := collectWaveDispatchMode(modes, ref); err != nil {
			return nil, err
		}
	}
	for _, token := range dispatchModeTokens(record.Notes) {
		if err := collectWaveDispatchMode(modes, token); err != nil {
			return nil, err
		}
	}
	if len(modes) == 0 {
		return nil, nil
	}
	return modes, nil
}

func collectWaveDispatchMode(modes map[int]WaveDispatchMode, raw string) error {
	raw = strings.Trim(strings.TrimSpace(raw), "\"'`.,;()[]{}")
	if !strings.HasPrefix(raw, WaveDispatchReferencePrefix) {
		return nil
	}
	rest := strings.TrimPrefix(raw, WaveDispatchReferencePrefix)
	waveRaw, modeRaw, ok := strings.Cut(rest, ":")
	if !ok {
		return fmt.Errorf("invalid wave dispatch reference %q", raw)
	}
	waveIndex, err := strconv.Atoi(strings.TrimSpace(waveRaw))
	if err != nil || waveIndex < 1 {
		return fmt.Errorf("invalid wave dispatch reference %q: wave index must be >= 1", raw)
	}
	mode := WaveDispatchMode(strings.TrimSpace(modeRaw))
	if !mode.IsValid() {
		return fmt.Errorf("invalid wave dispatch reference %q: invalid dispatch_mode %q", raw, mode)
	}
	if existing, exists := modes[waveIndex]; exists && existing != mode {
		return fmt.Errorf("conflicting dispatch_mode for wave %d: %q and %q", waveIndex, existing, mode)
	}
	modes[waveIndex] = mode
	return nil
}

func dispatchModeTokens(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case ' ', '\t', '\r', '\n', ',', ';', '|':
			return true
		default:
			return false
		}
	})
}

type WaveRun struct {
	WaveIndex         int              `yaml:"wave_index" json:"wave_index"`
	RunSummaryVersion int              `yaml:"run_summary_version" json:"run_summary_version"`
	StartedAt         time.Time        `yaml:"started_at,omitempty" json:"started_at,omitempty"`
	CompletedAt       time.Time        `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`
	TaskRuns          []TaskRunRef     `yaml:"task_runs,omitempty" json:"task_runs,omitempty"`
	Verdict           WaveVerdict      `yaml:"verdict" json:"verdict"`
	DispatchMode      WaveDispatchMode `yaml:"dispatch_mode,omitempty" json:"dispatch_mode,omitempty"`
}

func (r *WaveRun) Normalize() {
	if !r.StartedAt.IsZero() {
		r.StartedAt = r.StartedAt.Round(0).UTC()
	}
	if !r.CompletedAt.IsZero() {
		r.CompletedAt = r.CompletedAt.Round(0).UTC()
	}
	if r.TaskRuns == nil {
		r.TaskRuns = []TaskRunRef{}
	}
	slices.SortFunc(r.TaskRuns, func(a, b TaskRunRef) int {
		if a.TaskID != b.TaskID {
			return strings.Compare(a.TaskID, b.TaskID)
		}
		return a.RunSummaryVersion - b.RunSummaryVersion
	})
}

func (r WaveRun) Validate(expectedIndex int) error {
	if r.WaveIndex != expectedIndex {
		return fmt.Errorf("wave_index must be %d", expectedIndex)
	}
	if r.RunSummaryVersion < 1 {
		return fmt.Errorf("run_summary_version must be >= 1")
	}
	if !r.Verdict.IsValid() {
		return fmt.Errorf("invalid verdict %q", r.Verdict)
	}
	if r.DispatchMode != "" && !r.DispatchMode.IsValid() {
		return fmt.Errorf("invalid dispatch_mode %q", r.DispatchMode)
	}
	for i, ref := range r.TaskRuns {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("task_runs[%d]: %w", i, err)
		}
	}
	return nil
}
