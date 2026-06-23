package progression

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/governance"
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

const taskEvidenceResultSchema = "task_id,verdict,evidence_ref,changed_files,blockers,session_id"

func taskEvidenceActionDetail(root, slug string, runSummaryVersion int) string {
	parts := []string{}
	if runSummaryVersion > 0 {
		parts = append(parts, fmt.Sprintf("run_summary_version=%d", runSummaryVersion))
	}
	if strings.TrimSpace(root) != "" && strings.TrimSpace(slug) != "" {
		parts = append(parts, "task_evidence_path="+state.DisplayPath(root, state.EvidenceTasksDir(root, slug)))
	}
	parts = append(parts, "record_command=slipway evidence task --result-file <path> --json")
	parts = append(parts, "result_schema="+taskEvidenceResultSchema)
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

	wavePlan, err := currentWavePlanForExecution(root, change, mutate)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return WaveSyncResult{
				Blockers: []model.ReasonCode{model.NewReasonCode("wave_plan_missing", change.Slug)},
			}, nil
		}
		return WaveSyncResult{}, err
	}
	tasksPlanHash := wavePlanStructuralHash(wavePlan)
	wavePlan = state.ApplyEffectiveParallel(wavePlan, state.EffectiveForcedParallel(root))
	dispatchModes, err := model.WaveDispatchModesFromVerification(record)
	if err != nil {
		return WaveSyncResult{}, err
	}
	waveRuns, err := state.BuildWaveRuns(wavePlan, record.RunVersion, tasks, dispatchModes)
	if err != nil {
		return WaveSyncResult{}, err
	}

	planDriftBlockers := taskEvidencePlanHashBlockers(tasksPlanHash, tasks)
	executionSummary := BuildExecutionSummary(record.RunVersion, tasks, record.Timestamp, &record)
	executionSummary.TasksPlanHash = tasksPlanHash
	if len(parseIssues) > 0 || len(planDriftBlockers) > 0 {
		executionSummary.OpenBlockers = model.NormalizeReasonCodes(append(executionSummary.OpenBlockers, model.ReasonCodesFromSpecs(append(parseIssues, planDriftBlockers...))...))
		executionSummary.SyncDerivedFields()
	}
	// Make incomplete-execution blockers durable in the summary's OpenBlockers so
	// read-only readiness (validate/next/status) surfaces them too. Once a
	// "ready" summary exists, refineS2WaveExecutionSkillBlockers short-circuits
	// the preview path, so a returned-only blocker would vanish from read-only
	// surfaces after the partial summary is written (issue #95 REQ-001).
	incompleteBlockers := IncompleteExecutionTaskBlockers(wavePlan, executionSummary.TaskRunMap())
	if len(incompleteBlockers) > 0 {
		executionSummary.OpenBlockers = model.NormalizeReasonCodes(append(executionSummary.OpenBlockers, incompleteBlockers...))
		executionSummary.SyncDerivedFields()
	}
	// Resolve the preset policy once here so the preset-sensitive degraded-dispatch
	// justification safety net fails closed on standard/strict and stays advisory
	// on light, without changing SyncGovernedWaveExecution's signature or its call
	// sites. Both the mutate and preview paths flow through this point, so a single
	// resolution covers them. authority.go in this same package already imports
	// governance, so this adds no new cross-package edge. Fail closed on a
	// resolution error.
	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return WaveSyncResult{}, err
	}
	enforced := policy.EffectivePreset != model.WorkflowPresetLight
	// Shared-worktree fail-closed safety nets, aggregated at the single governed
	// wave-execution assembly point so every gate flows out through one path.
	// Each turns already-recorded host evidence (target_files/changed_files, and
	// the plan's per-wave Parallel flag) into a hard blocker. Mirrors the
	// incompleteBlockers durability contract: surfaced in OpenBlockers so
	// read-only readiness reports them. Extended by later safety-net gates
	// (C2/C3).
	var safetyNetBlockers []model.ReasonCode
	safetyNetBlockers = append(safetyNetBlockers, TaskChangedFileScopeEscapeBlockers(wavePlan, tasks)...)
	safetyNetBlockers = append(safetyNetBlockers, ParallelWaveChangedFileOverlapBlockers(wavePlan, tasks)...)
	safetyNetBlockers = append(safetyNetBlockers, DispatchEvidenceBlockers(wavePlan, tasks, dispatchModes, model.DegradedDispatchJustificationsFromVerification(record), enforced)...)
	safetyNetBlockers = append(safetyNetBlockers, ExecutorAgentBlockers(wavePlan, tasks, dispatchModes, model.ExecutorAgentHandlesFromVerification(record))...)
	if len(safetyNetBlockers) > 0 {
		executionSummary.OpenBlockers = model.NormalizeReasonCodes(append(executionSummary.OpenBlockers, safetyNetBlockers...))
		executionSummary.SyncDerivedFields()
	}
	if !mutate {
		runs := executionSummary.TaskRunMap()
		blockers := model.ReasonCodesFromSpecs(parseIssues)
		blockers = append(blockers, model.ReasonCodesFromSpecs(planDriftBlockers)...)
		blockers = append(blockers, CollectNonPassTaskBlockers(runs)...)
		blockers = append(blockers, incompleteBlockers...)
		blockers = append(blockers, safetyNetBlockers...)
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
	blockers = append(blockers, safetyNetBlockers...)
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
	raw, err := os.ReadFile(tasksPath) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
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

func currentWavePlanForExecution(root string, change model.Change, mutate bool) (model.WavePlan, error) {
	if mutate {
		return state.MaterializeWavePlan(root, change)
	}
	plan, _, err := state.MaterializeWavePlanTransactionOpAt(root, change, time.Unix(0, 0).UTC())
	return plan, err
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

// ResumeWaveIndexFromTaskEvidence derives the current incomplete wave index from
// the per-task evidence on disk, without requiring a materialized
// execution-summary.yaml. It is the resume-index authority for the documented
// per-task-evidence flow BETWEEN waves, before wave-orchestration skill evidence
// has been recorded (issue #227a): in that window no run summary exists yet, so
// callers that fall back to state.ResumeWaveIndex(plan, nil) always see wave 1
// and a next-wave task is wrongly rejected as not-in-current-wave. Reconstructing
// the runs from recorded task evidence lets a fully-passed early wave count as
// complete so its successor becomes the current incomplete wave.
//
// The boolean result reports whether usable task evidence was found and applied.
// When false, the caller must keep its own default (wave 1) — there is no
// evidence yet to derive from, the recorded files do not resolve to a single run
// version, or none of them loaded cleanly; the returned index is 0 and carries no
// meaning. When true, the index is the authoritative current incomplete wave (0
// means every planned wave has passed, exactly like state.ResumeWaveIndex). A
// non-nil error is only returned for a genuine filesystem/parse failure that must
// fail closed.
func ResumeWaveIndexFromTaskEvidence(root string, change model.Change, plan model.WavePlan) (int, bool, error) {
	runVersion, err := singleTaskEvidenceRunVersion(root, change.Slug)
	if err != nil {
		return 0, false, err
	}
	if runVersion < 1 {
		return 0, false, nil
	}
	tasks, issues, err := LoadExecutionTasksFromEvidence(root, change.Slug, runVersion)
	if err != nil {
		return 0, false, err
	}
	if len(issues) > 0 || len(tasks) == 0 {
		return 0, false, nil
	}
	plan = state.ApplyEffectiveParallel(plan, state.EffectiveForcedParallel(root))
	runs, err := state.BuildWaveRuns(plan, runVersion, tasks, nil)
	if err != nil {
		return 0, false, err
	}
	return state.ResumeWaveIndex(plan, runs), true, nil
}

// singleTaskEvidenceRunVersion returns the run version shared by every recorded
// task evidence file, or 0 when none are recorded, an unreadable/malformed file is
// present, or the recorded files span more than one run version (all ambiguous —
// caller keeps the wave-1 default rather than guessing). Only a directory-level
// read failure fails closed via an error.
func singleTaskEvidenceRunVersion(root, slug string) (int, error) {
	dir := state.EvidenceTasksDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	versions := map[int]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		version, err := taskEvidenceRunVersion(filepath.Join(dir, entry.Name()))
		if err != nil {
			// A single unreadable/malformed task-evidence file must not make
			// resume-index derivation harder-failing than the read-only sibling
			// surfaces, which soft-tolerate the same file:
			// LoadExecutionTasksFromEvidence records it as an issue and continues.
			// Skip it for version detection; the len(issues) > 0 check in
			// ResumeWaveIndexFromTaskEvidence still forces derived=false, so the
			// caller keeps the safe, more-restrictive wave-1 default rather than
			// aborting (issue #227a review).
			continue
		}
		versions[version] = struct{}{}
	}
	if len(versions) != 1 {
		return 0, nil
	}
	for version := range versions {
		return version, nil
	}
	return 0, nil
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
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
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
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
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
// tasks would let wave-orchestration "pass" and advance S2_IMPLEMENT -> S3_REVIEW
// while later planned tasks were never executed (issue #95). The remedy is to
// execute and record the named task, or update tasks.md if the plan no longer
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
		blockers = append(blockers, model.NewReasonCode(IncompleteExecutionTaskBlockerCode, taskID))
	}
	return model.NormalizeReasonCodes(blockers)
}

// IncompleteExecutionTaskBlockerCode marks a planned wave-plan task that has no
// recorded run at the active run_summary_version (issue #95). It is also the
// engine's durable signal that a task was folded into tasks.md at S3_REVIEW and
// still needs its evidence before the in-place convergence completes.
const IncompleteExecutionTaskBlockerCode = "incomplete_execution_task"

// ExecutionSummaryHasIncompleteTask reports whether the persisted execution
// summary still carries an incomplete_execution_task blocker — the engine's
// durable signal that a task folded into tasks.md at S3_REVIEW has not yet had
// its evidence recorded. It scopes the S3 wave-orchestration re-record window:
// the host may re-attest the wave run in place only while genuine convergence
// work remains, and the window closes once the rebuilt summary reflects the
// folded task.
func ExecutionSummaryHasIncompleteTask(summary *model.ExecutionSummary) bool {
	if summary == nil {
		return false
	}
	for _, blocker := range summary.OpenBlockers {
		if strings.TrimSpace(blocker.Code) == IncompleteExecutionTaskBlockerCode {
			return true
		}
	}
	return false
}

// TaskChangedFileScopeEscapeBlockers reports, per task, every recorded changed
// file that escapes that task's planned target_files. Under shared-worktree
// fan-out, accurate target_files plus exhaustive changed_files are the safety
// model: a task that writes a path it never declared can collide with a peer
// executor that legitimately owns it. Coverage uses wave.TargetCoversPath, the
// same directional scope predicate the wave planner's conflict detection relies
// on, so "covers" and "conflicts" share one implementation (REQ-002). The
// remedy is to fix target_files and re-record evidence; S3 review owns final
// plan/code alignment.
func TaskChangedFileScopeEscapeBlockers(plan model.WavePlan, tasks []model.ExecutionTaskSummary) []model.ReasonCode {
	if len(tasks) == 0 {
		return nil
	}
	// Coverage is judged against the plan's target_files, never the host-recorded
	// evidence target_files: a task could otherwise widen its own evidence targets
	// to cover any path it wrote and silence the audit. The plan is the authority
	// for what a task is allowed to touch (REQ-002).
	plannedTargets := make(map[string][]string)
	for _, planWave := range plan.Waves {
		for _, planTask := range planWave.Tasks {
			plannedTargets[planTask.TaskID] = planTask.TargetFiles
		}
	}
	blockers := []model.ReasonCode{}
	for _, task := range tasks {
		targets, planned := plannedTargets[task.TaskID]
		if !planned {
			// Evidence for a task absent from the wave plan is an orphan, owned by a
			// dedicated gate; this audit only adjudicates planned tasks.
			continue
		}
		for _, changedFile := range task.ChangedFiles {
			if strings.TrimSpace(changedFile) == "" {
				continue
			}
			if wave.TargetCoversPath(targets, changedFile) {
				continue
			}
			blockers = append(blockers, model.NewReasonCode(
				"task_changed_file_scope_escape",
				task.TaskID+":"+changedFile,
			))
		}
	}
	return model.NormalizeReasonCodes(blockers)
}

// ParallelWaveChangedFileOverlapBlockers reports, for plan-parallel waves only,
// any changed file that two or more tasks in the same wave both recorded. Tasks
// in a parallel wave may run concurrently in the shared worktree, so two
// executors writing the same path can clobber each other (REQ-003). Sequential
// waves cannot run concurrently, so a shared file across them is allowed and not
// reported. The plan's per-wave Parallel flag is authoritative here; a run that
// later declared degraded_sequential still leaves a plan-quality defect if the
// supposedly parallel-safe tasks wrote the same file. Callers pass the wave plan
// after ApplyEffectiveParallel so the flag reflects the current effective
// parallelization mode. Task ids in the detail are sorted and comma-joined for a
// deterministic blocker.
func ParallelWaveChangedFileOverlapBlockers(plan model.WavePlan, tasks []model.ExecutionTaskSummary) []model.ReasonCode {
	if len(tasks) == 0 || len(plan.Waves) == 0 {
		return nil
	}
	changedByTask := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		changedByTask[task.TaskID] = task.ChangedFiles
	}

	blockers := []model.ReasonCode{}
	for _, wavePlanWave := range plan.Waves {
		if !wavePlanWave.Parallel {
			continue
		}
		// canonical file identity -> task ids in this wave that recorded it.
		// Bucketing by wave.CanonicalConflictPath (not the raw recorded string)
		// collapses slash/backslash and case-only path aliases to one file, so the
		// audit uses the same "same file" notion as the planner's conflict
		// detection instead of trusting raw-string equality (REQ-003).
		owners := map[string][]string{}
		display := map[string]string{}
		for _, planTask := range wavePlanWave.Tasks {
			for _, changedFile := range changedByTask[planTask.TaskID] {
				if strings.TrimSpace(changedFile) == "" {
					continue
				}
				key := wave.CanonicalConflictPath(changedFile)
				if key == "" {
					continue
				}
				owners[key] = append(owners[key], planTask.TaskID)
				if existing, ok := display[key]; !ok || changedFile < existing {
					display[key] = changedFile
				}
			}
		}
		keys := make([]string, 0, len(owners))
		for key := range owners {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			taskIDs := stringutil.UniqueSorted(owners[key])
			if len(taskIDs) < 2 {
				continue
			}
			blockers = append(blockers, model.NewReasonCode(
				"parallel_wave_changed_file_overlap",
				fmt.Sprintf("%d:%s:%s", wavePlanWave.WaveIndex, display[key], strings.Join(taskIDs, ",")),
			))
		}
	}
	return model.NormalizeReasonCodes(blockers)
}

// DispatchEvidenceBlockers reports each started parallel wave that recorded no
// explicit, valid dispatch_mode token. The engine no longer infers parallel
// dispatch for such a wave (REQ-004): under shared-worktree fan-out a started
// parallel wave with no recorded dispatch evidence cannot be assumed to have run
// its executors in parallel, so it fails closed to a blocker rather than a silent
// assumption. A parallel_subagents token is explicit evidence and is accepted
// without blocking; non-parallel waves never require dispatch evidence. A wave is
// "started" once any of its planned tasks has recorded execution evidence — the
// same condition BuildWaveRuns uses before recording a dispatch mode at all.
// dispatchModes is the validity-filtered map from WaveDispatchModesFromVerification,
// so a wave's presence in it already means a single valid token was recorded.
//
// A bare self-asserted degraded_sequential token is no longer self-sufficient
// (REQ-004): on standard/strict (enforced) it is accepted only when paired with a
// genuine tool-unavailable justification for the same wave, carried by the
// additive degraded_dispatch_justification:wave=<n>:tool_unavailable=<detail>
// reference grammar (justifications is the wave-index set parsed from it). A
// justified degraded wave passes; a bare degraded wave fails closed. On light the
// degraded path is advisory (enforced=false), so no justification is required.
// The tightening stays inside the Parallel guard and leaves the non-degraded
// dispatch_mode_absent path unchanged.
func DispatchEvidenceBlockers(
	plan model.WavePlan,
	tasks []model.ExecutionTaskSummary,
	dispatchModes map[int]model.WaveDispatchMode,
	justifications map[int]struct{},
	enforced bool,
) []model.ReasonCode {
	if len(plan.Waves) == 0 {
		return nil
	}
	recorded := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		recorded[task.TaskID] = struct{}{}
	}
	blockers := []model.ReasonCode{}
	for _, wavePlanWave := range plan.Waves {
		if !wavePlanWave.Parallel {
			continue
		}
		if !waveStarted(wavePlanWave, recorded) {
			continue
		}
		mode, ok := dispatchModes[wavePlanWave.WaveIndex]
		if !ok || !mode.IsValid() {
			blockers = append(blockers, model.NewReasonCode(
				"dispatch_mode_absent_on_started_parallel_wave",
				strconv.Itoa(wavePlanWave.WaveIndex),
			))
			continue
		}
		if enforced && mode == model.WaveDispatchDegradedSequential {
			if _, justified := justifications[wavePlanWave.WaveIndex]; !justified {
				blockers = append(blockers, model.NewReasonCode(
					"degraded_dispatch_justification_missing",
					strconv.Itoa(wavePlanWave.WaveIndex),
				))
			}
		}
	}
	return model.NormalizeReasonCodes(blockers)
}

func waveStarted(wavePlanWave model.WavePlanWave, recorded map[string]struct{}) bool {
	for _, planTask := range wavePlanWave.Tasks {
		if _, ok := recorded[planTask.TaskID]; ok {
			return true
		}
	}
	return false
}

// ExecutorAgentBlockers reports, for every wave dispatched parallel_subagents,
// each planned task that has no recorded executor_agent handle. A parallel_subagents
// dispatch claims each task ran in its own subagent within the shared worktree;
// without a handle for a planned task that claim is unverifiable, so the engine
// fails closed (REQ-005). Waves dispatched degraded_sequential and non-parallel
// waves require no handles and are skipped. dispatchModes and handles both come
// from the same wave-orchestration verification references, so the parallel_subagents
// claim and its handles are recorded together.
func ExecutorAgentBlockers(
	plan model.WavePlan,
	tasks []model.ExecutionTaskSummary,
	dispatchModes map[int]model.WaveDispatchMode,
	handles map[int]map[string]string,
) []model.ReasonCode {
	if len(plan.Waves) == 0 {
		return nil
	}
	recorded := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		recorded[task.TaskID] = struct{}{}
	}
	blockers := []model.ReasonCode{}
	for _, wavePlanWave := range plan.Waves {
		// REQ-005: only parallel waves require handles. A non-parallel wave never
		// requires them even if it carries a stale or contradictory
		// parallel_subagents token (e.g. recorded while parallel, then
		// parallelization turned off so ApplyEffectiveParallel cleared the flag),
		// so skip it before consulting the dispatch mode — mirroring the
		// DispatchEvidenceBlockers / ParallelWaveChangedFileOverlapBlockers guard.
		if !wavePlanWave.Parallel {
			continue
		}
		if dispatchModes[wavePlanWave.WaveIndex] != model.WaveDispatchParallel {
			continue
		}
		if !waveStarted(wavePlanWave, recorded) {
			continue
		}
		waveHandles := handles[wavePlanWave.WaveIndex]
		for _, planTask := range wavePlanWave.Tasks {
			if strings.TrimSpace(waveHandles[planTask.TaskID]) != "" {
				continue
			}
			blockers = append(blockers, model.NewReasonCode(
				"executor_agent_missing",
				fmt.Sprintf("%d:%s", wavePlanWave.WaveIndex, planTask.TaskID),
			))
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
