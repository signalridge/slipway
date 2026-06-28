package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	freshnesspkg "github.com/signalridge/slipway/internal/freshness"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/stringutil"
	"gopkg.in/yaml.v3"
)

const ExecutionSummaryFileName = "execution-summary.yaml"
const StaleExecutionEvidenceBlockerToken = "stale_execution_evidence"
const StalePlanningEvidenceBlockerToken = "stale_planning_evidence"
const TasksPlanChangedSinceTaskEvidenceBlockerToken = "tasks_plan_changed_since_task_evidence"
const planAuditFileName = "plan-audit.yaml"

// S3TaskPlanAmendmentDiagnostic explains why S3 task-plan edits stay in the
// review/fix loop instead of forcing S2 execution evidence replay.
const S3TaskPlanAmendmentDiagnostic = "s3_task_plan_amendment_review_required: tasks.md changed after S2 execution evidence; continue S3 review/fix without rerunning S2 solely for task-plan edits"

type ExecutionSummaryLoadError struct {
	Path string
	Err  error
}

func (e *ExecutionSummaryLoadError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExecutionSummaryLoadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func wrapExecutionSummaryLoadError(path string, err error) error {
	if err == nil {
		return nil
	}
	var loadErr *ExecutionSummaryLoadError
	if errors.As(err, &loadErr) {
		return err
	}
	return &ExecutionSummaryLoadError{
		Path: path,
		Err:  err,
	}
}

func ExecutionSummaryReady(summary *model.ExecutionSummary) bool {
	return summary != nil &&
		summary.RunSummaryVersion >= 1 &&
		(len(summary.Tasks) > 0 ||
			len(summary.OpenBlockers) > 0 ||
			len(summary.CompletedTasks) > 0 ||
			len(summary.NonPassTasks) > 0)
}

func ExecutionSummaryRelevantState(state model.WorkflowState) bool {
	switch state {
	case model.StateS2Implement, model.StateS3Review, model.StateDone:
		return true
	default:
		return false
	}
}

func ExecutionFreshnessInputBlocker(blocker model.ReasonCode) bool {
	switch strings.TrimSpace(blocker.Code) {
	case StalePlanningEvidenceBlockerToken,
		StaleExecutionEvidenceBlockerToken,
		TasksPlanChangedSinceTaskEvidenceBlockerToken:
		return true
	default:
		return false
	}
}

func StalePlanningEvidenceBlocker(blocker model.ReasonCode) bool {
	return strings.TrimSpace(blocker.Code) == StalePlanningEvidenceBlockerToken
}

// ExecutionSummaryPathForRead returns the authoritative read path candidate for
// execution-summary.yaml, preferring active worktree-owned and archived bundle
// locations over local fallback display paths.
func ExecutionSummaryPathForRead(root, slug string) string {
	return filepath.Join(verificationDirPathForRead(root, slug), ExecutionSummaryFileName)
}

func executionSummaryReadPath(root, slug string) (string, error) {
	dir, err := resolveExistingVerificationDir(root, slug)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ExecutionSummaryFileName), nil
}

func executionSummaryReadPathForChange(root string, change model.Change) (string, error) {
	dir, err := verificationDirForChange(root, change)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ExecutionSummaryFileName), nil
}

func LoadExecutionSummary(root, slug string) (model.ExecutionSummary, error) {
	displayPath := ExecutionSummaryPathForRead(root, slug)
	path, err := executionSummaryReadPath(root, slug)
	if err != nil {
		return model.ExecutionSummary{}, wrapExecutionSummaryLoadError(displayPath, err)
	}
	summary, err := loadExecutionSummaryFromPath(path)
	if err != nil {
		return model.ExecutionSummary{}, wrapExecutionSummaryLoadError(path, err)
	}
	return summary, nil
}

func LoadExecutionSummaryForChange(root string, change model.Change) (model.ExecutionSummary, error) {
	displayPath := ExecutionSummaryPathForRead(root, change.Slug)
	path, err := executionSummaryReadPathForChange(root, change)
	if err != nil {
		return model.ExecutionSummary{}, wrapExecutionSummaryLoadError(displayPath, err)
	}
	summary, err := loadExecutionSummaryFromPath(path)
	if err != nil {
		return model.ExecutionSummary{}, wrapExecutionSummaryLoadError(path, err)
	}
	return summary, nil
}

func loadExecutionSummaryFromPath(path string) (model.ExecutionSummary, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		return model.ExecutionSummary{}, err
	}
	var summary model.ExecutionSummary
	if err := decodeExecutionSummaryStrict(raw, &summary); err != nil {
		return model.ExecutionSummary{}, fmt.Errorf("parse execution summary: %w", err)
	}
	summary.Normalize()
	if err := summary.Validate(); err != nil {
		return model.ExecutionSummary{}, fmt.Errorf("invalid execution summary: %w", err)
	}
	return summary, nil
}

func LoadOptionalExecutionSummary(root, slug string) (*model.ExecutionSummary, error) {
	summary, err := LoadExecutionSummary(root, slug)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &summary, nil
}

func LoadOptionalExecutionSummaryForChange(root string, change model.Change) (*model.ExecutionSummary, error) {
	summary, err := LoadExecutionSummaryForChange(root, change)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &summary, nil
}

func LoadOptionalRelevantExecutionSummary(root string, change model.Change) (*model.ExecutionSummary, error) {
	if !ExecutionSummaryRelevantState(change.CurrentState) {
		return nil, nil
	}
	return LoadOptionalExecutionSummaryForChange(root, change)
}

type RelevantExecutionSummaryContext struct {
	Summary          *model.ExecutionSummary
	Issues           []string
	Diagnostics      ExecutionFreshnessDiagnostics
	LatestRunVersion int
}

func LoadRelevantExecutionSummaryContext(root string, change model.Change) (RelevantExecutionSummaryContext, error) {
	summary, err := LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return RelevantExecutionSummaryContext{}, err
	}
	ctx := RelevantExecutionSummaryContext{
		Summary:     summary,
		Diagnostics: ExecutionSummaryFreshnessDiagnostics(root, change, summary),
	}
	ctx.Issues = collectExecutionSummaryIssuesFromDiagnostics(change, summary, ctx.Diagnostics)
	if ExecutionSummaryReady(summary) {
		ctx.LatestRunVersion = summary.RunSummaryVersion
	}
	return ctx, nil
}

type ExecutionFreshnessDiagnostics struct {
	Status                  string                         `json:"status" yaml:"status"`
	FirstStaleCause         *ExecutionFreshnessPair        `json:"first_stale_cause,omitempty" yaml:"first_stale_cause,omitempty"`
	StalePairs              []ExecutionFreshnessPair       `json:"stale_pairs,omitempty" yaml:"stale_pairs,omitempty"`
	DownstreamEvidenceChain []ExecutionFreshnessPair       `json:"downstream_evidence_chain,omitempty" yaml:"downstream_evidence_chain,omitempty"`
	TaskInputDiffs          []ExecutionTaskInputDifference `json:"task_input_diffs,omitempty" yaml:"task_input_diffs,omitempty"`
	PathAuthority           *ExecutionPathAuthority        `json:"path_authority,omitempty" yaml:"path_authority,omitempty"`
	NextAction              string                         `json:"next_action,omitempty" yaml:"next_action,omitempty"`
}

type ExecutionFreshnessPair struct {
	SourceArtifact     string `json:"source_artifact,omitempty" yaml:"source_artifact,omitempty"`
	EvidenceArtifact   string `json:"evidence_artifact,omitempty" yaml:"evidence_artifact,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	SourceUpdatedAt    string `json:"source_updated_at,omitempty" yaml:"source_updated_at,omitempty"`
	EvidenceCapturedAt string `json:"evidence_captured_at,omitempty" yaml:"evidence_captured_at,omitempty"`
	NextAction         string `json:"next_action,omitempty" yaml:"next_action,omitempty"`
}

type ExecutionTaskInputDifference struct {
	TaskID       string `json:"task_id" yaml:"task_id"`
	Field        string `json:"field" yaml:"field"`
	Expected     string `json:"expected" yaml:"expected"`
	Current      string `json:"current" yaml:"current"`
	EvidencePath string `json:"evidence_path,omitempty" yaml:"evidence_path,omitempty"`
	NextAction   string `json:"next_action,omitempty" yaml:"next_action,omitempty"`
}

type ExecutionPathAuthority struct {
	InvocationWorkspacePath string `json:"invocation_workspace_path,omitempty" yaml:"invocation_workspace_path,omitempty"`
	BoundWorkspacePath      string `json:"bound_workspace_path,omitempty" yaml:"bound_workspace_path,omitempty"`
	GovernedBundlePath      string `json:"governed_bundle_path,omitempty" yaml:"governed_bundle_path,omitempty"`
	VerificationPath        string `json:"verification_path,omitempty" yaml:"verification_path,omitempty"`
	ChangeAuthorityPath     string `json:"change_authority_path,omitempty" yaml:"change_authority_path,omitempty"`
	GitCommonDirPath        string `json:"git_common_dir_path,omitempty" yaml:"git_common_dir_path,omitempty"`
	RuntimeEvidencePath     string `json:"runtime_evidence_path,omitempty" yaml:"runtime_evidence_path,omitempty"`
	TaskEvidencePath        string `json:"task_evidence_path,omitempty" yaml:"task_evidence_path,omitempty"`
}

func SaveExecutionSummary(root, slug string, summary model.ExecutionSummary) error {
	summary.Normalize()
	if change, err := LoadChange(root, slug); err == nil {
		ApplyExecutionSummaryFreshnessInputs(&summary, change)
	}
	summary.SyncDerivedFields()
	if err := summary.Validate(); err != nil {
		return err
	}
	dir, err := resolveVerificationDirForWrite(root, slug)
	if err != nil {
		return fmt.Errorf("resolve execution summary dir for %q: %w", slug, err)
	}
	path := filepath.Join(dir, ExecutionSummaryFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	raw, err := yaml.Marshal(summary)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o644)
}

// LatestRelevantExecutionRunVersion returns the execution-summary run version
// only for states that are expected to surface execution evidence. Callers
// that already know the workflow state should prefer this helper.
func LatestRelevantExecutionRunVersion(root string, change model.Change) (int, error) {
	ctx, err := LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		return 0, err
	}
	return ctx.LatestRunVersion, nil
}

func decodeExecutionSummaryStrict(raw []byte, summary *model.ExecutionSummary) error {
	return decodeYAMLKnownFields(raw, summary)
}

func decodeYAMLKnownFields(raw []byte, out any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	return decoder.Decode(out)
}

func collectExecutionSummaryIssuesFromDiagnostics(change model.Change, summary *model.ExecutionSummary, diagnostics ExecutionFreshnessDiagnostics) []string {
	if !ExecutionSummaryRelevantState(change.CurrentState) || !ExecutionSummaryReady(summary) || strings.TrimSpace(change.Slug) == "" {
		return nil
	}

	blockers := make([]string, 0, len(summary.OpenBlockers)+1)
	blockers = append(blockers, model.ReasonSpecs(summary.OpenBlockers)...)
	if diagnostics.Status == string(freshnesspkg.EvidenceFreshnessStale) {
		if ExecutionFreshnessIsS3TaskPlanAmendment(change.CurrentState, diagnostics) {
			return stringutil.UniqueSorted(blockers)
		}
		blockers = append(blockers, executionFreshnessBlockerCode(diagnostics))
	}
	return stringutil.UniqueSorted(blockers)
}

// ProjectExecutionFreshnessForState maps raw execution-summary freshness into
// the lifecycle state's public gate meaning.
func ProjectExecutionFreshnessForState(
	workflowState model.WorkflowState,
	diagnostics ExecutionFreshnessDiagnostics,
) freshnesspkg.EvidenceFreshness {
	status := freshnesspkg.EvidenceFreshness(strings.TrimSpace(diagnostics.Status))
	if status == "" {
		status = freshnesspkg.EvidenceFreshnessUnknown
	}
	if ExecutionFreshnessIsS3TaskPlanAmendment(workflowState, diagnostics) {
		return freshnesspkg.EvidenceFreshnessFresh
	}
	return status
}

// ProjectExecutionFreshnessDiagnosticsForState hides raw S2 replay diagnostics
// when the active S3 review loop owns the same-intent task-plan amendment.
func ProjectExecutionFreshnessDiagnosticsForState(
	workflowState model.WorkflowState,
	diagnostics ExecutionFreshnessDiagnostics,
) ExecutionFreshnessDiagnostics {
	if !ExecutionFreshnessIsS3TaskPlanAmendment(workflowState, diagnostics) {
		return diagnostics
	}
	return ExecutionFreshnessDiagnostics{
		Status:        string(freshnesspkg.EvidenceFreshnessFresh),
		PathAuthority: diagnostics.PathAuthority,
	}
}

// ExecutionFreshnessIsS3TaskPlanAmendment reports whether raw task-plan drift
// should be handled as current S3 review/fix input instead of S2 replay input.
func ExecutionFreshnessIsS3TaskPlanAmendment(
	workflowState model.WorkflowState,
	diagnostics ExecutionFreshnessDiagnostics,
) bool {
	return workflowState == model.StateS3Review && ExecutionFreshnessIsTaskPlanOnlyDrift(diagnostics)
}

// ExecutionFreshnessIsTaskPlanOnlyDrift reports a stale execution summary whose
// only cause is the tasks.md -> wave-plan/execution-summary planning chain.
func ExecutionFreshnessIsTaskPlanOnlyDrift(diagnostics ExecutionFreshnessDiagnostics) bool {
	if strings.TrimSpace(diagnostics.Status) != string(freshnesspkg.EvidenceFreshnessStale) {
		return false
	}
	if len(diagnostics.TaskInputDiffs) > 0 || len(diagnostics.StalePairs) == 0 {
		return false
	}
	for _, pair := range diagnostics.StalePairs {
		if !executionFreshnessPairHasPlanningCause(pair) {
			return false
		}
	}
	return true
}

func executionFreshnessBlockerCode(diagnostics ExecutionFreshnessDiagnostics) string {
	if executionFreshnessHasPlanningCause(diagnostics) {
		return StalePlanningEvidenceBlockerToken
	}
	return StaleExecutionEvidenceBlockerToken
}

func executionFreshnessHasPlanningCause(diagnostics ExecutionFreshnessDiagnostics) bool {
	for _, pair := range diagnostics.StalePairs {
		if executionFreshnessPairHasPlanningCause(pair) {
			return true
		}
	}
	return false
}

func executionFreshnessPairHasPlanningCause(pair ExecutionFreshnessPair) bool {
	return strings.TrimSpace(pair.Reason) == StalePlanningEvidenceBlockerToken
}

func ExpectedExecutionTaskFreshnessInputs(change model.Change, runSummaryVersion int, taskID string, tasksPlanHash ...string) model.ExecutionTaskFreshnessInputs {
	planHash := ""
	if len(tasksPlanHash) > 0 {
		planHash = strings.TrimSpace(tasksPlanHash[0])
	}
	return model.ExecutionTaskFreshnessInputs{
		ChangeID:          strings.TrimSpace(change.Slug),
		RunSummaryVersion: runSummaryVersion,
		TaskID:            strings.TrimSpace(taskID),
		GuardrailDomain:   strings.TrimSpace(change.GuardrailDomain),
		TasksPlanHash:     planHash,
	}.Normalized()
}

func ApplyExecutionSummaryFreshnessInputs(summary *model.ExecutionSummary, change model.Change) {
	if summary == nil || !ExecutionSummaryReady(summary) {
		return
	}
	for i := range summary.Tasks {
		summary.Tasks[i].FreshnessInputs = ExpectedExecutionTaskFreshnessInputs(change, summary.RunSummaryVersion, summary.Tasks[i].TaskID, summary.TasksPlanHash)
	}
}

func executionSummaryFreshnessEvaluation(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	evidenceArtifact string,
) (freshnesspkg.EvidenceFreshness, []ExecutionTaskInputDifference, []ExecutionFreshnessPair) {
	taskInputDiffs := taskFreshnessInputDiffs(root, change, summary)
	planningPairs := stalePlanningPairs(root, change, summary, evidenceArtifact)
	if len(taskInputDiffs) > 0 || len(planningPairs) > 0 {
		return freshnesspkg.EvidenceFreshnessStale, taskInputDiffs, planningPairs
	}
	inputs := collectTaskEvidenceFreshnessInputs(change, summary)
	if len(inputs) == 0 {
		return freshnesspkg.EvidenceFreshnessUnknown, nil, nil
	}
	return freshnesspkg.EvaluateEvidenceFreshness(true, inputs), nil, nil
}

func executionSummaryEvidenceArtifact(root string, change model.Change) string {
	evidenceArtifact := ExecutionSummaryPathForRead(root, change.Slug)
	if path, err := executionSummaryReadPathForChange(root, change); err == nil {
		evidenceArtifact = DisplayPath(root, path)
	}
	return evidenceArtifact
}

func ExecutionSummaryFreshnessDiagnostics(root string, change model.Change, summary *model.ExecutionSummary) ExecutionFreshnessDiagnostics {
	diagnostics := ExecutionFreshnessDiagnostics{
		Status:        string(freshnesspkg.EvidenceFreshnessUnknown),
		PathAuthority: ExecutionPathAuthorityDiagnostics(root, change, 0),
	}
	if !ExecutionSummaryReady(summary) || strings.TrimSpace(change.Slug) == "" {
		return diagnostics
	}

	diagnostics.PathAuthority = ExecutionPathAuthorityDiagnostics(root, change, summary.RunSummaryVersion)
	evidenceArtifact := executionSummaryEvidenceArtifact(root, change)
	freshness, taskInputDiffs, planningPairs := executionSummaryFreshnessEvaluation(root, change, summary, evidenceArtifact)
	diagnostics.Status = string(freshness)

	diagnostics.TaskInputDiffs = taskInputDiffs
	taskInputPairs := []ExecutionFreshnessPair{}
	if len(diagnostics.TaskInputDiffs) > 0 {
		taskInputPairs = append(taskInputPairs, ExecutionFreshnessPair{
			EvidenceArtifact:   evidenceArtifact,
			Reason:             StaleExecutionEvidenceBlockerToken,
			EvidenceCapturedAt: formatFreshnessTime(summary.CapturedAt),
			NextAction:         "regenerate execution evidence with `slipway run` or rerun wave-orchestration for affected tasks",
		})
	}

	diagnostics.StalePairs = append(diagnostics.StalePairs, planningPairs...)
	diagnostics.StalePairs = append(diagnostics.StalePairs, taskInputPairs...)
	diagnostics.StalePairs = normalizeFreshnessPairs(diagnostics.StalePairs)
	if len(diagnostics.StalePairs) > 0 {
		diagnostics.FirstStaleCause = &diagnostics.StalePairs[0]
		if len(diagnostics.StalePairs) > 1 {
			diagnostics.DownstreamEvidenceChain = append([]ExecutionFreshnessPair(nil), diagnostics.StalePairs[1:]...)
		}
		diagnostics.NextAction = diagnostics.FirstStaleCause.NextAction
		if diagnostics.NextAction == "" {
			diagnostics.NextAction = "regenerate stale evidence from the named source artifact"
		}
		return diagnostics
	}

	if freshness == freshnesspkg.EvidenceFreshnessStale {
		diagnostics.StalePairs = append(diagnostics.StalePairs, ExecutionFreshnessPair{
			EvidenceArtifact:   evidenceArtifact,
			Reason:             StaleExecutionEvidenceBlockerToken,
			EvidenceCapturedAt: formatFreshnessTime(summary.CapturedAt),
			NextAction:         "rerun wave-orchestration so execution-summary.yaml is regenerated from current evidence",
		})
		diagnostics.FirstStaleCause = &diagnostics.StalePairs[0]
		diagnostics.NextAction = diagnostics.FirstStaleCause.NextAction
	}
	return diagnostics
}

func ExecutionPathAuthorityDiagnostics(root string, change model.Change, runSummaryVersion int) *ExecutionPathAuthority {
	slug := strings.TrimSpace(change.Slug)
	if slug == "" {
		return nil
	}
	out := &ExecutionPathAuthority{
		InvocationWorkspacePath: DisplayPath(root, root),
		GitCommonDirPath:        absolutePathForDiagnostics(GitCommonDir(root)),
		RuntimeEvidencePath:     absolutePathForDiagnostics(ChangeDir(root, slug)),
	}
	out.TaskEvidencePath = absolutePathForDiagnostics(EvidenceTasksDir(root, slug))
	if paths, err := ResolveChangePaths(root, change); err == nil {
		out.BoundWorkspacePath = DisplayPath(root, paths.WorkspaceRoot)
		out.GovernedBundlePath = DisplayPath(root, paths.GovernedBundleDir)
		out.VerificationPath = DisplayPath(root, filepath.Join(paths.GovernedBundleDir, "verification"))
		out.ChangeAuthorityPath = DisplayPath(root, filepath.Join(paths.GovernedBundleDir, "change.yaml"))
	}
	return out
}

func absolutePathForDiagnostics(path string) string {
	if normalized, err := NormalizePath(path); err == nil {
		return filepath.ToSlash(normalized)
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func taskFreshnessInputDiffs(root string, change model.Change, summary *model.ExecutionSummary) []ExecutionTaskInputDifference {
	if !ExecutionSummaryReady(summary) {
		return nil
	}
	diffs := []ExecutionTaskInputDifference{}
	for _, task := range summary.Tasks {
		expected := ExpectedExecutionTaskFreshnessInputs(change, summary.RunSummaryVersion, task.TaskID, summary.TasksPlanHash).FieldMap()
		current := task.FreshnessInputs.FieldMap()
		evidencePath := taskEvidenceDisplayPath(root, change.Slug, summary.RunSummaryVersion, task.TaskID)
		if evidencePath == "" {
			evidencePath = strings.TrimSpace(task.EvidenceRef)
		}
		if task.FreshnessInputs.IsZero() {
			current = map[string]string{}
		}
		fields := []string{"change_id", "run_summary_version", "task_id", "guardrail_domain", "tasks_plan_hash"}
		for _, field := range fields {
			if expected[field] == current[field] {
				continue
			}
			currentValue := current[field]
			if currentValue == "" && strings.TrimSpace(task.EvidenceInputHash) != "" {
				currentValue = "missing; legacy evidence_input_hash=" + strings.TrimSpace(task.EvidenceInputHash)
			}
			diffs = append(diffs, ExecutionTaskInputDifference{
				TaskID:       task.TaskID,
				Field:        field,
				Expected:     expected[field],
				Current:      currentValue,
				EvidencePath: evidencePath,
				NextAction:   "regenerate execution evidence; do not edit freshness fields or timestamps by hand",
			})
		}
		if capturedAt, ok, err := taskEvidenceCapturedAt(root, change.Slug, summary.RunSummaryVersion, task.TaskID); err != nil {
			diffs = append(diffs, ExecutionTaskInputDifference{
				TaskID:       task.TaskID,
				Field:        "captured_at",
				Expected:     "readable runtime task evidence captured_at",
				Current:      err.Error(),
				EvidencePath: evidencePath,
				NextAction:   "regenerate execution evidence; do not edit freshness fields or timestamps by hand",
			})
		} else if ok && !task.CapturedAt.UTC().Equal(capturedAt.UTC()) {
			diffs = append(diffs, ExecutionTaskInputDifference{
				TaskID:       task.TaskID,
				Field:        "captured_at",
				Expected:     formatFreshnessTime(capturedAt),
				Current:      formatFreshnessTime(task.CapturedAt),
				EvidencePath: evidencePath,
				NextAction:   "regenerate execution-summary.yaml from runtime task evidence; do not edit timestamps by hand",
			})
		}
	}
	return diffs
}

func taskEvidenceCapturedAt(root, slug string, runSummaryVersion int, taskID string) (time.Time, bool, error) {
	path := filepath.Join(EvidenceTasksDir(root, slug), strings.TrimSpace(taskID)+".json")
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return time.Time{}, false, nil
		}
		return time.Time{}, true, err
	}
	var payload struct {
		CapturedAt        string `json:"captured_at"`
		RunSummaryVersion int    `json:"run_summary_version"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return time.Time{}, true, fmt.Errorf("parse runtime task evidence: %w", err)
	}
	if payload.RunSummaryVersion != runSummaryVersion {
		// Evidence on disk belongs to a different run version; treat it as
		// absent for this summary so captured_at drift is not misattributed
		// across run versions (mirrors LoadExecutionTasksFromEvidence).
		return time.Time{}, false, nil
	}
	if strings.TrimSpace(payload.CapturedAt) == "" {
		return time.Time{}, true, fmt.Errorf("runtime task evidence captured_at is missing")
	}
	capturedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.CapturedAt))
	if err != nil {
		return time.Time{}, true, fmt.Errorf("runtime task evidence captured_at is invalid: %w", err)
	}
	return capturedAt.UTC(), true, nil
}

func taskEvidenceDisplayPath(root, slug string, runSummaryVersion int, taskID string) string {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(slug) == "" || runSummaryVersion < 1 || strings.TrimSpace(taskID) == "" {
		return ""
	}
	return DisplayPath(root, filepath.Join(EvidenceTasksDir(root, slug), strings.TrimSpace(taskID)+".json"))
}

func stalePlanningPairs(root string, change model.Change, summary *model.ExecutionSummary, evidenceArtifact string) []ExecutionFreshnessPair {
	if !ExecutionSummaryReady(summary) {
		return nil
	}
	bundleDir, err := GovernedBundleDir(root, change)
	if err != nil {
		return nil
	}

	sources := []stalePlanningSource{}
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	tasksPlanDrift := false
	if currentHash, err := CurrentTasksPlanStructuralState(root, change); err == nil {
		if strings.TrimSpace(summary.TasksPlanHash) != "" && currentHash != strings.TrimSpace(summary.TasksPlanHash) {
			tasksPlanDrift = true
			sources = append(sources, stalePlanningSource{
				path:       tasksPath,
				nextAction: "rerun wave-orchestration so Slipway derives the current wave projection and refreshes execution-summary.yaml",
			})
		}
	} else if strings.TrimSpace(summary.TasksPlanHash) != "" {
		tasksPlanDrift = true
		sources = append(sources, stalePlanningSource{
			path:       tasksPath,
			nextAction: "restore readable tasks.md, then rerun wave-orchestration to refresh execution-summary.yaml",
		})
	}
	if !tasksPlanDrift {
		currentScopeHash, currentScopeErr := CurrentTasksPlanScopeState(root, change)
		storedScopeHash, hasStoredScopeHash, storedScopeErr := storedTasksPlanScopeHash(root, change)
		if currentScopeErr == nil && storedScopeErr == nil && hasStoredScopeHash && currentScopeHash != storedScopeHash {
			sources = append(sources, stalePlanningSource{
				path:       tasksPath,
				nextAction: "rerun wave-orchestration so Slipway derives the current wave projection and refreshes execution-summary.yaml",
			})
		} else if currentScopeErr != nil && storedScopeErr == nil && hasStoredScopeHash {
			sources = append(sources, stalePlanningSource{
				path:       tasksPath,
				nextAction: "restore readable tasks.md, then rerun wave-orchestration to refresh execution-summary.yaml",
			})
		}
	}

	slices.SortFunc(sources, func(a, b stalePlanningSource) int {
		if !a.updatedAt.Equal(b.updatedAt) {
			if a.updatedAt.Before(b.updatedAt) {
				return -1
			}
			return 1
		}
		return strings.Compare(a.path, b.path)
	})

	pairs := []ExecutionFreshnessPair{}
	for _, source := range sources {
		pairs = append(pairs, stalePlanningEvidenceChain(root, bundleDir, evidenceArtifact, summary.CapturedAt.UTC(), source)...)
	}
	return pairs
}

func storedTasksPlanScopeHash(root string, change model.Change) (string, bool, error) {
	plan, err := LoadWavePlanForChange(root, change)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	hash := strings.TrimSpace(plan.TasksPlanScopeHash)
	return hash, hash != "", nil
}

type stalePlanningSource struct {
	path       string
	updatedAt  time.Time
	nextAction string
}

func stalePlanningEvidenceChain(root, bundleDir, executionSummaryArtifact string, executionSummaryCapturedAt time.Time, source stalePlanningSource) []ExecutionFreshnessPair {
	stagePaths := []string{}
	for _, rel := range []string{
		filepath.Join("verification", WavePlanFileName),
	} {
		path := filepath.Join(bundleDir, rel)
		if _, err := os.Stat(path); err == nil {
			stagePaths = append(stagePaths, path)
		}
	}

	sourceArtifact := DisplayPath(root, source.path)
	sourceUpdatedAt := source.updatedAt
	if len(stagePaths) == 0 {
		return []ExecutionFreshnessPair{{
			SourceArtifact:     sourceArtifact,
			EvidenceArtifact:   executionSummaryArtifact,
			Reason:             StalePlanningEvidenceBlockerToken,
			SourceUpdatedAt:    formatFreshnessTime(sourceUpdatedAt),
			EvidenceCapturedAt: formatFreshnessTime(executionSummaryCapturedAt),
			NextAction:         source.nextAction,
		}}
	}

	pairs := []ExecutionFreshnessPair{}
	for _, stagePath := range stagePaths {
		stageCapturedAt := stageEvidenceCapturedAt(stagePath)
		pairs = append(pairs, ExecutionFreshnessPair{
			SourceArtifact:     sourceArtifact,
			EvidenceArtifact:   DisplayPath(root, stagePath),
			Reason:             StalePlanningEvidenceBlockerToken,
			SourceUpdatedAt:    formatFreshnessTime(sourceUpdatedAt),
			EvidenceCapturedAt: formatFreshnessTime(stageCapturedAt),
			NextAction:         nextActionForPlanningStage(stagePath),
		})
		sourceArtifact = DisplayPath(root, stagePath)
		sourceUpdatedAt = stageCapturedAt
	}
	pairs = append(pairs, ExecutionFreshnessPair{
		SourceArtifact:     sourceArtifact,
		EvidenceArtifact:   executionSummaryArtifact,
		Reason:             StalePlanningEvidenceBlockerToken,
		SourceUpdatedAt:    formatFreshnessTime(sourceUpdatedAt),
		EvidenceCapturedAt: formatFreshnessTime(executionSummaryCapturedAt),
		NextAction:         "rerun wave-orchestration before repairing downstream execution evidence",
	})
	return pairs
}

func nextActionForPlanningStage(path string) string {
	switch filepath.Base(path) {
	case planAuditFileName:
		return "rerun plan-audit to refresh stale plan review evidence"
	case WavePlanFileName:
		return "rerun wave-orchestration so Slipway refreshes the task-derived wave projection before rerunning execution"
	default:
		return "regenerate stale planning evidence before repairing downstream execution evidence"
	}
}

// stageEvidenceCapturedAt returns the captured timestamp embedded in a
// planning-stage evidence record when that timestamp is part of the evidence
// contract. wave-plan.yaml's generated_at is display/audit metadata only, so it
// intentionally does not participate in freshness diagnostics.
//
// The embedded record timestamp — not the file mtime — is the semantic capture
// time the staleness comparison treats as the evidence age. A record can be
// rewritten after capture (e.g. adding DEC->REQ trace references) without
// re-running the stage; that drifts the file mtime ahead of the real capture
// time and makes a genuinely stale pair read as fresh. Reading the mtime here
// is exactly the bug, so it is never consulted: an unreadable record or one
// without an embedded timestamp reports a zero (unknown) capture time.
func stageEvidenceCapturedAt(path string) time.Time {
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		return time.Time{}
	}
	var record struct {
		Timestamp time.Time `yaml:"timestamp"`
	}
	if err := yaml.Unmarshal(raw, &record); err != nil {
		return time.Time{}
	}
	switch filepath.Base(path) {
	case planAuditFileName:
		return record.Timestamp.UTC()
	default:
		return time.Time{}
	}
}

func normalizeFreshnessPairs(pairs []ExecutionFreshnessPair) []ExecutionFreshnessPair {
	if len(pairs) == 0 {
		return nil
	}
	out := make([]ExecutionFreshnessPair, 0, len(pairs))
	for _, pair := range pairs {
		duplicate := false
		for _, existing := range out {
			if sameFreshnessPair(existing, pair) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		out = append(out, pair)
	}
	return out
}

func sameFreshnessPair(a, b ExecutionFreshnessPair) bool {
	return a.SourceArtifact == b.SourceArtifact &&
		a.EvidenceArtifact == b.EvidenceArtifact &&
		a.Reason == b.Reason &&
		a.SourceUpdatedAt == b.SourceUpdatedAt &&
		a.EvidenceCapturedAt == b.EvidenceCapturedAt
}

func formatFreshnessTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func collectTaskEvidenceFreshnessInputs(
	change model.Change,
	summary *model.ExecutionSummary,
) []freshnesspkg.EvidenceFreshnessInput {
	if !ExecutionSummaryReady(summary) {
		return nil
	}

	inputs := []freshnesspkg.EvidenceFreshnessInput{}
	for _, task := range summary.Tasks {
		expected := ExpectedExecutionTaskFreshnessInputs(change, summary.RunSummaryVersion, task.TaskID, summary.TasksPlanHash)
		inputs = append(inputs, freshnesspkg.EvidenceFreshnessInput{
			ExpectedStructuralInput: expected.FieldMap(),
			CurrentStructuralInput:  task.FreshnessInputs.FieldMap(),
		})
	}
	return inputs
}
