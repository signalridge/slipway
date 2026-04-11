package progression

import (
	"encoding/json"
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
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/stringutil"
)

// TaskEvidencePayload is the parsed payload from a task evidence JSON file.
type TaskEvidencePayload struct {
	TaskRun *model.TaskRun `json:"task_run,omitempty"`

	TaskID            string             `json:"task_id,omitempty"`
	RunSummaryVersion int                `json:"run_summary_version,omitempty"`
	TaskKind          model.TaskKind     `json:"task_kind,omitempty"`
	Verdict           model.TaskVerdict  `json:"verdict,omitempty"`
	ChangedFiles      []string           `json:"changed_files,omitempty"`
	TargetFiles       []string           `json:"target_files,omitempty"`
	EvidenceRef       string             `json:"evidence_ref,omitempty"`
	Blockers          []model.ReasonCode `json:"blockers,omitempty"`
	CapturedAt        string             `json:"captured_at,omitempty"`
	InputHash         string             `json:"input_hash,omitempty"`
	SessionID         string             `json:"session_id,omitempty"`
}

// SyncGovernedWaveExecution synchronizes wave execution state for a governed change.
func SyncGovernedWaveExecution(root string, change model.Change) (WaveSyncResult, error) {
	record, found, err := LatestPassingWaveEvidence(root, change.Slug)
	if err != nil {
		return WaveSyncResult{}, err
	}
	if !found {
		return WaveSyncResult{}, nil
	}
	if record.RunVersion < 1 {
		return WaveSyncResult{
			Blockers: []model.ReasonCode{model.NewReasonCode("wave_orchestration_run_summary_version_invalid", "")},
		}, nil
	}

	tasks, parseIssues, err := LoadExecutionTasksFromEvidence(root, change.Slug, record.RunVersion)
	if err != nil {
		return WaveSyncResult{}, err
	}
	if len(tasks) == 0 {
		blockers := []model.ReasonCode{
			model.NewReasonCode("missing_task_evidence_for_run_summary", fmt.Sprintf("rv%d", record.RunVersion)),
		}
		blockers = append(blockers, model.ReasonCodesFromSpecs(parseIssues)...)
		return WaveSyncResult{Blockers: model.NormalizeReasonCodes(blockers)}, nil
	}

	wavePlan, err := state.LoadWavePlanForChange(root, change)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return WaveSyncResult{
				Blockers: []model.ReasonCode{model.NewReasonCode("wave_plan_missing", change.Slug)},
			}, nil
		}
		return WaveSyncResult{}, err
	}
	waveRuns, err := state.BuildWaveRuns(wavePlan, record.RunVersion, tasks)
	if err != nil {
		return WaveSyncResult{}, err
	}

	tasksPlanHash, tasksPlanUpdatedAt, err := state.CurrentTasksPlanState(root, change)
	if err != nil {
		return WaveSyncResult{}, err
	}
	existingSummary, err := state.LoadOptionalExecutionSummary(root, change.Slug)
	if err != nil {
		return WaveSyncResult{}, err
	}
	previousTasksPlanHash := ""
	if existingSummary != nil {
		previousTasksPlanHash = strings.TrimSpace(existingSummary.TasksPlanHash)
	}
	planDriftBlockers := tasksPlanChangedSinceTaskEvidenceBlockers(previousTasksPlanHash, tasks, tasksPlanHash, tasksPlanUpdatedAt)
	executionSummary := BuildExecutionSummary(record.RunVersion, tasks, record.Timestamp, &record)
	if len(planDriftBlockers) == 0 {
		executionSummary.TasksPlanHash = tasksPlanHash
	} else {
		executionSummary.TasksPlanHash = previousTasksPlanHash
	}
	if len(parseIssues) > 0 || len(planDriftBlockers) > 0 {
		executionSummary.OpenBlockers = model.NormalizeReasonCodes(append(executionSummary.OpenBlockers, model.ReasonCodesFromSpecs(append(parseIssues, planDriftBlockers...))...))
		executionSummary.SyncDerivedFields()
	}
	existingWaveRuns, err := state.LoadOptionalWaveRuns(root, change.Slug, record.RunVersion)
	if err != nil {
		return WaveSyncResult{}, err
	}
	wroteWaveRuns := !waveRunsEqual(existingWaveRuns, waveRuns)
	if wroteWaveRuns {
		if err := state.SaveWaveRuns(root, change.Slug, record.RunVersion, waveRuns); err != nil {
			return WaveSyncResult{}, err
		}
	}
	wroteExecutionSummary := existingSummary == nil || !existingSummary.Equal(executionSummary)
	if wroteExecutionSummary {
		if err := state.SaveExecutionSummary(root, change.Slug, executionSummary); err != nil {
			return WaveSyncResult{}, err
		}
	}

	runs := executionSummary.TaskRunMap()

	updated := wroteExecutionSummary || wroteWaveRuns
	if len(planDriftBlockers) == 0 {
		wroteChecklist, err := syncCompletedTaskCheckboxes(root, change, runs)
		if err != nil {
			return WaveSyncResult{}, err
		}
		if wroteChecklist {
			updated = true
		}
	}

	blockers := model.ReasonCodesFromSpecs(parseIssues)
	blockers = append(blockers, model.ReasonCodesFromSpecs(planDriftBlockers)...)
	blockers = append(blockers, CollectNonPassTaskBlockers(runs)...)
	return WaveSyncResult{
		Updated:  updated,
		Blockers: model.NormalizeReasonCodes(blockers),
	}, nil
}

func syncCompletedTaskCheckboxes(root string, change model.Change, runs map[string]model.TaskRun) (bool, error) {
	if len(runs) == 0 {
		return false, nil
	}
	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return false, err
	}
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if _, err := wave.ParseTaskPlan(string(raw)); err != nil {
		return false, err
	}

	completed := make(map[string]bool)
	for _, run := range runs {
		if run.Verdict == model.TaskVerdictPass && len(run.Blockers) == 0 {
			completed[run.TaskID] = true
		}
	}
	updatedContent, changed, err := wave.ApplyCompletedTaskCheckboxes(string(raw), completed)
	if err != nil || !changed {
		return false, err
	}
	if err := fsutil.WriteFileAtomic(tasksPath, []byte(updatedContent), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// LatestPassingWaveEvidence returns the latest passing wave-orchestration verification record.
func LatestPassingWaveEvidence(root, slug string) (model.VerificationRecord, bool, error) {
	rec, err := state.LoadVerification(root, slug, SkillWaveOrchestration)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return model.VerificationRecord{}, false, nil
		}
		return model.VerificationRecord{}, false, fmt.Errorf("load wave-orchestration verification: %w", err)
	}
	if !rec.IsPassing() || rec.RunVersion < 1 {
		return model.VerificationRecord{}, false, nil
	}
	return rec, true, nil
}

// LoadExecutionTasksFromEvidence loads task execution summaries from evidence files for a specific version.
func LoadExecutionTasksFromEvidence(root, slug string, runSummaryVersion int) ([]model.ExecutionTaskSummary, []string, error) {
	if runSummaryVersion < 1 {
		return nil, nil, fmt.Errorf("run_summary_version must be >= 1")
	}
	dir := state.EvidenceTasksDir(root, slug, runSummaryVersion)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []model.ExecutionTaskSummary{}, nil, nil
		}
		return nil, nil, err
	}

	type candidate struct {
		task      model.ExecutionTaskSummary
		at        time.Time
		sessionID string
	}
	latest := map[string]candidate{}
	issues := []string{}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		task, capturedAt, sessionID, err := ParseTaskEvidence(root, path, runSummaryVersion)
		if err != nil {
			issues = append(issues, "task_evidence_invalid:"+entry.Name()+":"+err.Error())
			continue
		}

		existing, exists := latest[task.TaskID]
		if exists && !capturedAt.After(existing.at) {
			continue
		}
		latest[task.TaskID] = candidate{task: task, at: capturedAt, sessionID: sessionID}
	}

	sessionToTasks := map[string][]string{}
	for taskID, entry := range latest {
		if entry.sessionID != "" {
			sessionToTasks[entry.sessionID] = append(sessionToTasks[entry.sessionID], taskID)
		}
	}
	for sid, tasks := range sessionToTasks {
		if len(tasks) > 1 {
			slices.Sort(tasks)
			issues = append(issues, "session_isolation_warning:session_id="+sid+":shared_by="+strings.Join(tasks, ","))
		}
	}

	tasks := make([]model.ExecutionTaskSummary, 0, len(latest))
	for _, entry := range latest {
		tasks = append(tasks, entry.task)
	}
	slices.SortFunc(tasks, func(a, b model.ExecutionTaskSummary) int {
		return strings.Compare(a.TaskID, b.TaskID)
	})
	return tasks, stringutil.UniqueSorted(issues), nil
}

// ParseTaskEvidence parses a single task evidence file into an execution task summary.
func ParseTaskEvidence(root, path string, expectedRunSummaryVersion int) (model.ExecutionTaskSummary, time.Time, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.ExecutionTaskSummary{}, time.Time{}, "", err
	}
	payload := TaskEvidencePayload{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return model.ExecutionTaskSummary{}, time.Time{}, "", err
	}

	run := model.TaskRun{}
	if payload.TaskRun != nil {
		run = *payload.TaskRun
	} else {
		run = model.TaskRun{
			TaskID:            strings.TrimSpace(payload.TaskID),
			RunSummaryVersion: payload.RunSummaryVersion,
			TaskKind:          payload.TaskKind,
			Verdict:           payload.Verdict,
			ChangedFiles:      append([]string(nil), payload.ChangedFiles...),
			TargetFiles:       append([]string(nil), payload.TargetFiles...),
			EvidenceRef:       strings.TrimSpace(payload.EvidenceRef),
			Blockers:          append([]model.ReasonCode(nil), payload.Blockers...),
		}
	}

	if run.TaskID == "" {
		run.TaskID = deriveTaskIDFromEvidenceFilename(filepath.Base(path))
	}
	if run.TaskID == "" {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("task_id is required")
	}
	if run.RunSummaryVersion == 0 {
		run.RunSummaryVersion = expectedRunSummaryVersion
	}
	if run.RunSummaryVersion != expectedRunSummaryVersion {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf(
			"run_summary_version mismatch: expected=%d got=%d",
			expectedRunSummaryVersion,
			run.RunSummaryVersion,
		)
	}
	if run.TaskKind == "" || !run.TaskKind.IsValid() {
		if run.TaskKind != "" && !run.TaskKind.IsValid() {
			run.Blockers = append(run.Blockers, model.NewReasonCode("invalid_task_kind", string(run.TaskKind)))
		}
		run.TaskKind = model.TaskKindCode
	}
	if !run.Verdict.IsValid() {
		run.Blockers = append(run.Blockers, model.NewReasonCode("invalid_or_missing_verdict", ""))
		run.Verdict = model.TaskVerdictIncomplete
	}
	run.Blockers = model.NormalizeReasonCodes(run.Blockers)
	if strings.TrimSpace(run.EvidenceRef) == "" {
		rel, relErr := filepath.Rel(root, path)
		if relErr == nil {
			run.EvidenceRef = filepath.ToSlash(rel)
		}
	}
	if err := run.Validate(); err != nil {
		return model.ExecutionTaskSummary{}, time.Time{}, "", err
	}

	capturedAt := time.Time{}
	if strings.TrimSpace(payload.CapturedAt) != "" {
		if ts, err := time.Parse(time.RFC3339Nano, payload.CapturedAt); err == nil {
			capturedAt = ts.UTC()
		}
	}
	if capturedAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			capturedAt = info.ModTime().UTC()
		}
	}
	task := model.ExecutionTaskSummary{
		TaskID:            run.TaskID,
		Verdict:           run.Verdict,
		TaskKind:          run.TaskKind,
		ChangedFiles:      append([]string(nil), run.ChangedFiles...),
		TargetFiles:       append([]string(nil), run.TargetFiles...),
		EvidenceRef:       strings.TrimSpace(run.EvidenceRef),
		EvidenceInputHash: strings.TrimSpace(payload.InputHash),
		Blockers:          append([]model.ReasonCode(nil), run.Blockers...),
		CapturedAt:        capturedAt,
	}
	task.Normalize()
	if err := task.Validate(); err != nil {
		return model.ExecutionTaskSummary{}, time.Time{}, "", err
	}
	return task, capturedAt, strings.TrimSpace(payload.SessionID), nil
}

// deriveTaskIDFromEvidenceFilename extracts a task ID from an evidence filename.
func deriveTaskIDFromEvidenceFilename(fileName string) string {
	base := strings.TrimSuffix(strings.TrimSpace(fileName), filepath.Ext(fileName))
	if idx := strings.Index(base, "--"); idx > 0 {
		base = base[:idx]
	}
	return strings.TrimSpace(base)
}

// CollectNonPassTaskBlockers returns blocker strings for tasks that don't have a pass verdict.
func CollectNonPassTaskBlockers(runs map[string]model.TaskRun) []model.ReasonCode {
	if len(runs) == 0 {
		return nil
	}
	taskIDs := make([]string, 0, len(runs))
	for taskID := range runs {
		taskIDs = append(taskIDs, taskID)
	}
	slices.Sort(taskIDs)

	blockers := []model.ReasonCode{}
	for _, taskID := range taskIDs {
		run := runs[taskID]
		if run.Verdict != model.TaskVerdictPass {
			blockers = append(blockers, model.NewReasonCode("non_pass_task", taskID))
		}
		for _, blocker := range run.Blockers {
			blockers = append(blockers, taskScopedReasonCode("task_blocker", taskID, blocker))
		}
	}
	return model.NormalizeReasonCodes(blockers)
}

// BuildExecutionSummary constructs the durable execution summary stored in verification/.
func BuildExecutionSummary(
	runSummaryVersion int,
	tasks []model.ExecutionTaskSummary,
	capturedAt time.Time,
	waveRecord *model.VerificationRecord,
) model.ExecutionSummary {
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}

	clonedTasks := make([]model.ExecutionTaskSummary, len(tasks))
	copy(clonedTasks, tasks)

	openBlockers := []model.ReasonCode{}
	for _, task := range clonedTasks {
		for _, blocker := range task.Blockers {
			openBlockers = append(openBlockers, taskScopedReasonCode("task", task.TaskID, blocker))
		}
	}
	if waveRecord != nil {
		openBlockers = append(openBlockers, waveRecord.Blockers...)
	}

	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: runSummaryVersion,
		CapturedAt:        capturedAt.UTC(),
		OpenBlockers:      model.NormalizeReasonCodes(openBlockers),
		Tasks:             clonedTasks,
	}
	summary.Normalize()
	summary.SyncDerivedFields()
	return summary
}

func taskScopedReasonCode(wrapperCode, taskID string, blocker model.ReasonCode) model.ReasonCode {
	blocker.Normalize()
	spec := strings.TrimSpace(blocker.Code)
	if detail := strings.TrimSpace(blocker.Detail); detail != "" {
		spec += ":" + detail
	}
	return model.NewReasonCode(wrapperCode, taskID+":"+spec)
}

// BuildResumeCompletedTasks returns a set of task IDs that have passing verdicts
// at the latest run summary version with "skip" resume policy.
func BuildResumeCompletedTasks(summary model.ExecutionSummary) map[string]bool {
	completed := make(map[string]bool)
	if summary.RunSummaryVersion < 1 {
		return completed
	}
	for _, task := range summary.Tasks {
		if task.Verdict == model.TaskVerdictPass && len(task.Blockers) == 0 && task.TaskKind.ShouldSkipOnResume() {
			completed[task.TaskID] = true
		}
	}
	return completed
}

func tasksPlanChangedSinceTaskEvidenceBlockers(
	previousHash string,
	tasks []model.ExecutionTaskSummary,
	currentHash string,
	tasksPlanUpdatedAt time.Time,
) []string {
	currentHash = strings.TrimSpace(currentHash)
	previousHash = strings.TrimSpace(previousHash)
	if currentHash == "" || tasksPlanUpdatedAt.IsZero() || len(tasks) == 0 {
		return nil
	}
	if previousHash == currentHash && previousHash != "" {
		return nil
	}

	staleTasks := make([]string, 0, len(tasks))
	for _, task := range tasks {
		capturedAt := task.CapturedAt.UTC()
		if capturedAt.IsZero() || capturedAt.Before(tasksPlanUpdatedAt) {
			staleTasks = append(staleTasks, task.TaskID)
		}
	}
	slices.Sort(staleTasks)
	blockers := make([]string, 0, len(staleTasks))
	for _, taskID := range staleTasks {
		blockers = append(blockers, "tasks_plan_changed_since_task_evidence:"+taskID)
	}
	return blockers
}

func waveRunsEqual(left, right []model.WaveRun) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		l := left[i]
		r := right[i]
		l.Normalize()
		r.Normalize()
		if l.WaveIndex != r.WaveIndex ||
			l.RunSummaryVersion != r.RunSummaryVersion ||
			!l.StartedAt.Equal(r.StartedAt) ||
			!l.CompletedAt.Equal(r.CompletedAt) ||
			l.Verdict != r.Verdict ||
			!slices.Equal(l.TaskRuns, r.TaskRuns) {
			return false
		}
	}
	return true
}
