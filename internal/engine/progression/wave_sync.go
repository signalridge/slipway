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
	TaskID            string                             `json:"task_id,omitempty"`
	RunSummaryVersion int                                `json:"run_summary_version,omitempty"`
	TaskKind          model.TaskKind                     `json:"task_kind,omitempty"`
	Verdict           model.TaskVerdict                  `json:"verdict,omitempty"`
	ChangedFiles      []string                           `json:"changed_files,omitempty"`
	TargetFiles       []string                           `json:"target_files,omitempty"`
	EvidenceRef       string                             `json:"evidence_ref,omitempty"`
	Blockers          []model.ReasonCode                 `json:"blockers,omitempty"`
	CapturedAt        string                             `json:"captured_at,omitempty"`
	FreshnessInputs   model.ExecutionTaskFreshnessInputs `json:"freshness_inputs,omitempty"`
	InputHash         string                             `json:"input_hash,omitempty"`
	SessionID         string                             `json:"session_id,omitempty"`
}

type TaskEvidenceRunVersionMismatchError struct {
	Expected int
	Got      int
}

const taskEvidenceRequiredFields = "task_id,run_summary_version,task_kind,verdict,evidence_ref,captured_at,freshness_inputs"

func taskEvidenceActionDetail(root, slug string, runSummaryVersion int) string {
	parts := []string{}
	if runSummaryVersion > 0 {
		parts = append(parts, fmt.Sprintf("run_summary_version=%d", runSummaryVersion))
	}
	if strings.TrimSpace(root) != "" && strings.TrimSpace(slug) != "" {
		parts = append(parts, "task_evidence_path="+state.DisplayPath(root, state.EvidenceTasksDir(root, slug)))
	}
	parts = append(parts, "record_command=slipway evidence task")
	parts = append(parts, "required_fields="+taskEvidenceRequiredFields)
	return strings.Join(parts, "; ")
}

func runSummaryMissingDetail(root, slug, skillName string) string {
	detail := skillName + ":run_summary_missing"
	if actionDetail := taskEvidenceActionDetail(root, slug, 0); actionDetail != "" {
		detail += "; " + actionDetail
	}
	return detail
}

func (e TaskEvidenceRunVersionMismatchError) Error() string {
	return fmt.Sprintf("run_summary_version mismatch: expected=%d got=%d", e.Expected, e.Got)
}

// PreviewGovernedWaveExecution reports the same blockers as wave execution sync
// without materializing execution summaries or mutating task checkboxes.
func PreviewGovernedWaveExecution(root string, change model.Change) (WaveSyncResult, error) {
	return evaluateGovernedWaveExecution(root, change, false)
}

// SyncGovernedWaveExecution synchronizes wave execution state for a governed change.
func SyncGovernedWaveExecution(root string, change model.Change) (WaveSyncResult, error) {
	return evaluateGovernedWaveExecution(root, change, true)
}

func evaluateGovernedWaveExecution(root string, change model.Change, mutate bool) (WaveSyncResult, error) {
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
			model.NewReasonCode("missing_task_evidence_for_run_summary", taskEvidenceActionDetail(root, change.Slug, record.RunVersion)),
		}
		blockers = append(blockers, model.ReasonCodesFromSpecs(parseIssues)...)
		return WaveSyncResult{Blockers: model.NormalizeReasonCodes(blockers)}, nil
	}
	if staleBlockers := waveRecordStaleTaskEvidenceBlockers(record, tasks); len(staleBlockers) > 0 {
		// Surface parse issues alongside staleness so invalid task evidence is
		// not masked when both conditions hold at the same time.
		staleBlockers = append(staleBlockers, model.ReasonCodesFromSpecs(parseIssues)...)
		return WaveSyncResult{Blockers: model.NormalizeReasonCodes(staleBlockers)}, nil
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
	tasksPlanHash, err := state.CurrentTasksPlanStructuralState(root, change)
	if err != nil {
		return WaveSyncResult{}, err
	}
	currentScopeHash, err := state.CurrentTasksPlanScopeState(root, change)
	if err != nil {
		return WaveSyncResult{}, err
	}
	previousTasksPlanHash := wavePlanStructuralHash(wavePlan)
	previousScopeHash := strings.TrimSpace(wavePlan.TasksPlanScopeHash)
	// Re-materialize the wave-plan in place when the structure is unchanged but the
	// scope (target_files) drifted. An empty previousScopeHash is treated as drift
	// too, so a plan that predates the scope-hash field backfills it on first touch
	// instead of carrying stale target_files until some later rebuild — the
	// structural-equality precondition keeps this a scope-compatible rebuild.
	if mutate && previousTasksPlanHash != "" && previousTasksPlanHash == strings.TrimSpace(tasksPlanHash) &&
		currentScopeHash != "" && previousScopeHash != currentScopeHash {
		materialized, err := state.MaterializeWavePlan(root, change)
		if err != nil {
			return WaveSyncResult{}, err
		}
		wavePlan = materialized
		previousTasksPlanHash = wavePlanStructuralHash(wavePlan)
	}
	wavePlan = state.ApplyEffectiveParallel(wavePlan, state.EffectiveForcedParallel(root))
	dispatchModes, err := model.WaveDispatchModesFromVerification(record)
	if err != nil {
		return WaveSyncResult{}, err
	}
	waveRuns, err := state.BuildWaveRuns(wavePlan, record.RunVersion, tasks, dispatchModes)
	if err != nil {
		return WaveSyncResult{}, err
	}

	planDriftBlockers := tasksPlanChangedSinceTaskEvidenceBlockers(previousTasksPlanHash, tasks, tasksPlanHash)
	planDriftBlockers = append(planDriftBlockers, taskEvidencePlanHashBlockers(previousTasksPlanHash, tasks)...)
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
	// Make incomplete-execution blockers durable in the summary's OpenBlockers so
	// read-only readiness (validate/next/status) surfaces them too. Once a
	// "ready" summary exists, refineS2WaveExecutionSkillBlockers short-circuits
	// the preview path, so a returned-only blocker would vanish from read-only
	// surfaces after the partial summary is written (issue #95 REQ-001).
	// Suppressed under plan-drift, which owns its own remediation.
	var incompleteBlockers []model.ReasonCode
	if len(planDriftBlockers) == 0 {
		incompleteBlockers = IncompleteExecutionTaskBlockers(wavePlan, executionSummary.TaskRunMap())
		if len(incompleteBlockers) > 0 {
			executionSummary.OpenBlockers = model.NormalizeReasonCodes(append(executionSummary.OpenBlockers, incompleteBlockers...))
			executionSummary.SyncDerivedFields()
		}
	}
	if !mutate {
		runs := executionSummary.TaskRunMap()
		blockers := model.ReasonCodesFromSpecs(parseIssues)
		blockers = append(blockers, model.ReasonCodesFromSpecs(planDriftBlockers)...)
		blockers = append(blockers, CollectNonPassTaskBlockers(runs)...)
		blockers = append(blockers, incompleteBlockers...)
		return WaveSyncResult{Blockers: model.NormalizeReasonCodes(blockers)}, nil
	}

	existingSummary, err := state.LoadOptionalExecutionSummary(root, change.Slug)
	if err != nil {
		return WaveSyncResult{}, err
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
	blockers = append(blockers, incompleteBlockers...)
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

func wavePlanStructuralHash(plan model.WavePlan) string {
	plan.Normalize()
	if strings.TrimSpace(plan.EffectiveStructuralHash) != "" {
		return strings.TrimSpace(plan.EffectiveStructuralHash)
	}
	if strings.TrimSpace(plan.TasksPlanStructuralHash) != "" {
		return strings.TrimSpace(plan.TasksPlanStructuralHash)
	}
	return strings.TrimSpace(plan.TasksPlanHash)
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

func waveRecordStaleTaskEvidenceBlockers(
	record model.VerificationRecord,
	tasks []model.ExecutionTaskSummary,
) []model.ReasonCode {
	if record.Timestamp.IsZero() || len(tasks) == 0 {
		return nil
	}
	recordedAt := record.Timestamp.UTC()
	blockers := []model.ReasonCode{}
	for _, task := range tasks {
		if task.CapturedAt.UTC().After(recordedAt) {
			blockers = append(blockers, model.NewReasonCode("wave_orchestration_stale_task_evidence", task.TaskID))
		}
	}
	return model.NormalizeReasonCodes(blockers)
}

// LoadExecutionTasksFromEvidence loads task execution summaries from evidence files for a specific version.
func LoadExecutionTasksFromEvidence(root, slug string, runSummaryVersion int) ([]model.ExecutionTaskSummary, []string, error) {
	if runSummaryVersion < 1 {
		return nil, nil, fmt.Errorf("run_summary_version must be >= 1")
	}
	dir := state.EvidenceTasksDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []model.ExecutionTaskSummary{}, nil, nil
		}
		return nil, nil, err
	}
	change, changeErr := state.LoadChange(root, slug)
	if changeErr != nil {
		change = model.NewChange(slug)
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
		evidenceRunVersion, err := taskEvidenceRunVersion(path)
		if err != nil {
			issues = append(issues, "task_evidence_invalid:"+entry.Name()+":"+err.Error())
			continue
		}
		if evidenceRunVersion != runSummaryVersion {
			continue
		}
		task, capturedAt, sessionID, err := ParseTaskEvidence(root, path, runSummaryVersion)
		if err != nil {
			var versionMismatch TaskEvidenceRunVersionMismatchError
			if errors.As(err, &versionMismatch) {
				continue
			}
			issues = append(issues, "task_evidence_invalid:"+entry.Name()+":"+err.Error())
			continue
		}
		if err := validateTaskEvidenceFreshnessInputs(change, runSummaryVersion, task); err != nil {
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

func taskEvidenceRunVersion(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var payload struct {
		RunSummaryVersion int `json:"run_summary_version"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, fmt.Errorf("parse task evidence: %w", err)
	}
	if payload.RunSummaryVersion < 1 {
		return 0, fmt.Errorf("run_summary_version is required")
	}
	return payload.RunSummaryVersion, nil
}

func validateTaskEvidenceFreshnessInputs(change model.Change, runSummaryVersion int, task model.ExecutionTaskSummary) error {
	if strings.TrimSpace(task.EvidenceInputHash) != "" {
		return fmt.Errorf("input_hash is not supported; use freshness_inputs")
	}
	if task.FreshnessInputs.IsZero() {
		return fmt.Errorf("freshness_inputs is required")
	}
	expected := state.ExpectedExecutionTaskFreshnessInputs(change, runSummaryVersion, task.TaskID)
	current := task.FreshnessInputs
	current.TasksPlanHash = ""
	if current.Equal(expected) {
		return nil
	}
	return fmt.Errorf("freshness_inputs mismatch: %s", taskFreshnessInputDiffSummary(expected, current))
}

func taskFreshnessInputDiffSummary(expected, current model.ExecutionTaskFreshnessInputs) string {
	expectedMap := expected.FieldMap()
	currentMap := current.FieldMap()
	fields := []string{"change_id", "run_summary_version", "task_id", "guardrail_domain"}
	diffs := make([]string, 0, len(fields))
	for _, field := range fields {
		if expectedMap[field] == currentMap[field] {
			continue
		}
		diffs = append(diffs, fmt.Sprintf("%s expected=%q got=%q", field, expectedMap[field], currentMap[field]))
	}
	return strings.Join(diffs, ", ")
}

// ParseTaskEvidence parses a single task evidence file into an execution task summary.
func ParseTaskEvidence(_ string, path string, expectedRunSummaryVersion int) (model.ExecutionTaskSummary, time.Time, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.ExecutionTaskSummary{}, time.Time{}, "", err
	}
	payload := TaskEvidencePayload{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return model.ExecutionTaskSummary{}, time.Time{}, "", err
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return model.ExecutionTaskSummary{}, time.Time{}, "", err
	}
	if _, ok := envelope["task_run"]; ok {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("task_run is not supported; use flat task evidence fields")
	}

	run := model.TaskRun{
		TaskID:            strings.TrimSpace(payload.TaskID),
		RunSummaryVersion: payload.RunSummaryVersion,
		TaskKind:          payload.TaskKind,
		Verdict:           payload.Verdict,
		ChangedFiles:      append([]string(nil), payload.ChangedFiles...),
		TargetFiles:       append([]string(nil), payload.TargetFiles...),
		EvidenceRef:       strings.TrimSpace(payload.EvidenceRef),
		Blockers:          append([]model.ReasonCode(nil), payload.Blockers...),
	}
	if run.TaskID == "" {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("task_id is required")
	}
	if run.RunSummaryVersion == 0 {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("run_summary_version is required")
	}
	if run.RunSummaryVersion != expectedRunSummaryVersion {
		return model.ExecutionTaskSummary{}, time.Time{}, "", TaskEvidenceRunVersionMismatchError{
			Expected: expectedRunSummaryVersion,
			Got:      run.RunSummaryVersion,
		}
	}
	if run.TaskKind == "" {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("task_kind is required")
	}
	if !run.TaskKind.IsValid() {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("invalid task_kind: %q", run.TaskKind)
	}
	if run.Verdict == "" {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("verdict is required")
	}
	if !run.Verdict.IsValid() {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("invalid task verdict: %q", run.Verdict)
	}
	run.Blockers = model.NormalizeReasonCodes(run.Blockers)
	if strings.TrimSpace(run.EvidenceRef) == "" {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("evidence_ref is required")
	}
	if err := run.Validate(); err != nil {
		return model.ExecutionTaskSummary{}, time.Time{}, "", err
	}

	capturedAtRaw := strings.TrimSpace(payload.CapturedAt)
	if capturedAtRaw == "" {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("captured_at is required")
	}
	capturedAt, err := time.Parse(time.RFC3339Nano, capturedAtRaw)
	if err != nil {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("captured_at must be RFC3339Nano: %w", err)
	}
	capturedAt = capturedAt.UTC()
	if strings.TrimSpace(payload.InputHash) != "" {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("input_hash is not supported; use freshness_inputs")
	}
	payload.FreshnessInputs.Normalize()
	if payload.FreshnessInputs.IsZero() {
		return model.ExecutionTaskSummary{}, time.Time{}, "", fmt.Errorf("freshness_inputs is required")
	}
	task := model.ExecutionTaskSummary{
		TaskID:          run.TaskID,
		Verdict:         run.Verdict,
		TaskKind:        run.TaskKind,
		ChangedFiles:    append([]string(nil), run.ChangedFiles...),
		TargetFiles:     append([]string(nil), run.TargetFiles...),
		EvidenceRef:     strings.TrimSpace(run.EvidenceRef),
		FreshnessInputs: payload.FreshnessInputs,
		Blockers:        append([]model.ReasonCode(nil), run.Blockers...),
		CapturedAt:      capturedAt,
	}
	task.Normalize()
	if err := task.Validate(); err != nil {
		return model.ExecutionTaskSummary{}, time.Time{}, "", err
	}
	return task, capturedAt, strings.TrimSpace(payload.SessionID), nil
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

// IncompleteExecutionTaskBlockers reports planned wave-plan tasks that have no
// recorded run at the active run_summary_version. The wave-plan is the authority
// for the full task set, so a task missing from runs has no task evidence at all
// — distinct from a recorded-but-failing task, which CollectNonPassTaskBlockers
// reports. Without this check a host that records evidence for only the early
// tasks would let wave-orchestration "pass" and advance S2_EXECUTE -> S3_REVIEW
// while later planned tasks were never executed (issue #95). The remedy is to
// execute and record the named task, or rescope tasks.md so the plan no longer
// claims it.
func IncompleteExecutionTaskBlockers(plan model.WavePlan, runs map[string]model.TaskRun) []model.ReasonCode {
	plannedIDs := plan.TaskIDs()
	if len(plannedIDs) == 0 {
		return nil
	}
	missing := make([]string, 0, len(plannedIDs))
	for _, taskID := range plannedIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		if _, ok := runs[taskID]; ok {
			continue
		}
		missing = append(missing, taskID)
	}
	if len(missing) == 0 {
		return nil
	}
	slices.Sort(missing)
	blockers := make([]model.ReasonCode, 0, len(missing))
	for _, taskID := range missing {
		blockers = append(blockers, model.NewReasonCode("incomplete_execution_task", taskID))
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
) []string {
	currentHash = strings.TrimSpace(currentHash)
	previousHash = strings.TrimSpace(previousHash)
	if currentHash == "" || previousHash == "" || len(tasks) == 0 {
		return nil
	}
	if previousHash == currentHash && previousHash != "" {
		return nil
	}

	staleTasks := make([]string, 0, len(tasks))
	for _, task := range tasks {
		staleTasks = append(staleTasks, task.TaskID)
	}
	slices.Sort(staleTasks)
	blockers := make([]string, 0, len(staleTasks))
	for _, taskID := range staleTasks {
		blockers = append(blockers, "tasks_plan_changed_since_task_evidence:"+taskID)
	}
	return blockers
}

func taskEvidencePlanHashBlockers(
	expectedPlanHash string,
	tasks []model.ExecutionTaskSummary,
) []string {
	expectedPlanHash = strings.TrimSpace(expectedPlanHash)
	if expectedPlanHash == "" || len(tasks) == 0 {
		return nil
	}
	staleTasks := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(task.FreshnessInputs.TasksPlanHash) == expectedPlanHash {
			continue
		}
		staleTasks = append(staleTasks, task.TaskID)
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
			l.DispatchMode != r.DispatchMode ||
			!slices.Equal(l.TaskRuns, r.TaskRuns) {
			return false
		}
	}
	return true
}
