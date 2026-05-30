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

	ctxpack "github.com/signalridge/slipway/internal/engine/context"
	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/stringutil"
	"gopkg.in/yaml.v3"
)

const ExecutionSummaryFileName = "execution-summary.yaml"
const StaleExecutionEvidenceBlockerToken = "stale_execution_evidence"
const StalePlanningEvidenceBlockerToken = "stale_planning_evidence"
const planAuditFileName = "plan-audit.yaml"

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
	case model.StateS2Execute, model.StateS3Review, model.StateS4Verify, model.StateDone:
		return true
	default:
		return false
	}
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
	raw, err := os.ReadFile(path)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := yaml.Marshal(summary)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o644)
}

func RemoveExecutionSummary(root, slug string) error {
	dir, err := resolveVerificationDirForWrite(root, slug)
	if err != nil {
		return fmt.Errorf("resolve execution summary dir for %q: %w", slug, err)
	}
	path := filepath.Join(dir, ExecutionSummaryFileName)
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
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
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	return decoder.Decode(summary)
}

func collectExecutionSummaryIssuesFromDiagnostics(change model.Change, summary *model.ExecutionSummary, diagnostics ExecutionFreshnessDiagnostics) []string {
	if !ExecutionSummaryRelevantState(change.CurrentState) || !ExecutionSummaryReady(summary) || strings.TrimSpace(change.Slug) == "" {
		return nil
	}

	blockers := make([]string, 0, len(summary.OpenBlockers)+1)
	blockers = append(blockers, model.ReasonSpecs(summary.OpenBlockers)...)
	if diagnostics.Status == string(ctxpack.EvidenceFreshnessStale) {
		hasPlanningDrift := false
		for _, pair := range diagnostics.StalePairs {
			if pair.Reason == StalePlanningEvidenceBlockerToken {
				hasPlanningDrift = true
				break
			}
		}
		if hasPlanningDrift {
			blockers = append(blockers, StalePlanningEvidenceBlockerToken)
		} else {
			blockers = append(blockers, StaleExecutionEvidenceBlockerToken)
		}
	}
	return stringutil.UniqueSorted(blockers)
}

func ExpectedExecutionTaskFreshnessInputs(change model.Change, runSummaryVersion int, taskID string) model.ExecutionTaskFreshnessInputs {
	return model.ExecutionTaskFreshnessInputs{
		ChangeID:          strings.TrimSpace(change.Slug),
		RunSummaryVersion: runSummaryVersion,
		TaskID:            strings.TrimSpace(taskID),
		GuardrailDomain:   strings.TrimSpace(change.GuardrailDomain),
	}.Normalized()
}

func ApplyExecutionSummaryFreshnessInputs(summary *model.ExecutionSummary, change model.Change) {
	if summary == nil || !ExecutionSummaryReady(summary) {
		return
	}
	for i := range summary.Tasks {
		summary.Tasks[i].FreshnessInputs = ExpectedExecutionTaskFreshnessInputs(change, summary.RunSummaryVersion, summary.Tasks[i].TaskID)
	}
}

func ExecutionSummaryFreshness(root string, change model.Change, summary *model.ExecutionSummary) ctxpack.EvidenceFreshness {
	if !ExecutionSummaryReady(summary) || strings.TrimSpace(change.Slug) == "" {
		return ctxpack.EvidenceFreshnessUnknown
	}

	evidenceTimestamp := summary.CapturedAt.UTC()
	latestRelevantUpdateAt := latestExecutionRelevantUpdateAt(root, change, summary)
	evidenceArtifact := executionSummaryEvidenceArtifact(root, change)
	freshness, _, _ := executionSummaryFreshnessEvaluation(root, change, summary, evidenceArtifact, latestRelevantUpdateAt, evidenceTimestamp)
	return freshness
}

func executionSummaryFreshnessEvaluation(
	root string,
	change model.Change,
	summary *model.ExecutionSummary,
	evidenceArtifact string,
	latestRelevantUpdateAt time.Time,
	evidenceTimestamp time.Time,
) (ctxpack.EvidenceFreshness, []ExecutionTaskInputDifference, []ExecutionFreshnessPair) {
	taskInputDiffs := taskFreshnessInputDiffs(root, change, summary)
	planningPairs := stalePlanningPairs(root, change, summary, evidenceArtifact)
	if len(taskInputDiffs) > 0 || len(planningPairs) > 0 {
		return ctxpack.EvidenceFreshnessStale, taskInputDiffs, planningPairs
	}
	inputs := collectTaskEvidenceFreshnessInputs(change, summary, latestRelevantUpdateAt, evidenceTimestamp)
	if len(inputs) == 0 {
		inputs = []ctxpack.EvidenceFreshnessInput{{
			EvidenceTimestamp:      evidenceTimestamp,
			LatestRelevantUpdateAt: latestRelevantUpdateAt,
		}}
	}
	return ctxpack.EvaluateEvidenceFreshness(true, inputs), nil, nil
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
		Status:        string(ctxpack.EvidenceFreshnessUnknown),
		PathAuthority: ExecutionPathAuthorityDiagnostics(root, change, 0),
	}
	if !ExecutionSummaryReady(summary) || strings.TrimSpace(change.Slug) == "" {
		return diagnostics
	}

	diagnostics.PathAuthority = ExecutionPathAuthorityDiagnostics(root, change, summary.RunSummaryVersion)
	evidenceTimestamp := summary.CapturedAt.UTC()
	latestRelevantUpdateAt := latestExecutionRelevantUpdateAt(root, change, summary)
	evidenceArtifact := executionSummaryEvidenceArtifact(root, change)
	freshness, taskInputDiffs, planningPairs := executionSummaryFreshnessEvaluation(root, change, summary, evidenceArtifact, latestRelevantUpdateAt, evidenceTimestamp)
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

	if freshness == ctxpack.EvidenceFreshnessStale {
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
		GitCommonDirPath:        DisplayPath(root, GitCommonDir(root)),
		RuntimeEvidencePath:     DisplayPath(root, ChangeDir(root, slug)),
	}
	out.TaskEvidencePath = DisplayPath(root, EvidenceTasksDir(root, slug))
	if paths, err := ResolveChangePaths(root, change); err == nil {
		out.BoundWorkspacePath = DisplayPath(root, paths.WorkspaceRoot)
		out.GovernedBundlePath = DisplayPath(root, paths.GovernedBundleDir)
		out.VerificationPath = DisplayPath(root, filepath.Join(paths.GovernedBundleDir, "verification"))
		out.ChangeAuthorityPath = DisplayPath(root, filepath.Join(paths.GovernedBundleDir, "change.yaml"))
	}
	return out
}

func taskFreshnessInputDiffs(root string, change model.Change, summary *model.ExecutionSummary) []ExecutionTaskInputDifference {
	if !ExecutionSummaryReady(summary) {
		return nil
	}
	diffs := []ExecutionTaskInputDifference{}
	for _, task := range summary.Tasks {
		expected := ExpectedExecutionTaskFreshnessInputs(change, summary.RunSummaryVersion, task.TaskID).FieldMap()
		current := task.FreshnessInputs.FieldMap()
		evidencePath := taskEvidenceDisplayPath(root, change.Slug, summary.RunSummaryVersion, task.TaskID)
		if evidencePath == "" {
			evidencePath = strings.TrimSpace(task.EvidenceRef)
		}
		if task.FreshnessInputs.IsZero() {
			current = map[string]string{}
		}
		fields := []string{"change_id", "run_summary_version", "task_id", "guardrail_domain"}
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
	raw, err := os.ReadFile(path)
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

	latestEvidenceAt := summary.LatestRelevantUpdateAt().UTC()
	if captured := summary.CapturedAt.UTC(); captured.After(latestEvidenceAt) {
		latestEvidenceAt = captured
	}

	sources := []stalePlanningSource{}
	appendIfNewer := func(rel string) {
		path := filepath.Join(bundleDir, rel)
		info, err := os.Stat(path)
		if err != nil {
			return
		}
		updatedAt := info.ModTime().UTC()
		if !updatedAt.After(latestEvidenceAt) {
			return
		}
		sources = append(sources, stalePlanningSource{
			path:       path,
			updatedAt:  updatedAt,
			nextAction: "rerun plan-audit and wave-orchestration before repairing downstream execution evidence",
		})
	}
	for _, rel := range []string{"intent.md", "requirements.md", "research.md", "decision.md"} {
		appendIfNewer(rel)
	}

	tasksPath := filepath.Join(bundleDir, "tasks.md")
	tasksPlanHashMismatch := false
	if currentHash, updatedAt, err := CurrentTasksPlanState(root, change); err == nil {
		if strings.TrimSpace(summary.TasksPlanHash) != "" && currentHash != strings.TrimSpace(summary.TasksPlanHash) {
			tasksPlanHashMismatch = true
			sources = append(sources, stalePlanningSource{
				path:       tasksPath,
				updatedAt:  updatedAt,
				nextAction: "regenerate wave-plan.yaml and execution-summary.yaml from the current tasks.md plan",
			})
		}
	}
	if !tasksPlanHashMismatch && isTasksPlanFreshnessRelevant(tasksPath, summary) {
		appendIfNewer("tasks.md")
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
		pairs = append(pairs, stalePlanningEvidenceChain(root, bundleDir, evidenceArtifact, latestEvidenceAt, source)...)
	}
	return pairs
}

type stalePlanningSource struct {
	path       string
	updatedAt  time.Time
	nextAction string
}

func stalePlanningEvidenceChain(root, bundleDir, executionSummaryArtifact string, executionSummaryCapturedAt time.Time, source stalePlanningSource) []ExecutionFreshnessPair {
	stagePaths := []string{}
	for _, rel := range []string{
		filepath.Join("verification", planAuditFileName),
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
		stageCapturedAt := fileModTimeUTC(stagePath)
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
		return "rerun plan-audit before rebuilding downstream wave and execution evidence"
	case WavePlanFileName:
		return "regenerate wave-plan.yaml from current planning evidence before rerunning execution"
	default:
		return "regenerate stale planning evidence before repairing downstream execution evidence"
	}
}

func fileModTimeUTC(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime().UTC()
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

func failClosedFreshnessUpdateAt(latest time.Time) time.Time {
	if latest.IsZero() {
		return time.Unix(0, 1).UTC()
	}
	return latest.Add(time.Nanosecond)
}

func latestExecutionRelevantUpdateAt(root string, change model.Change, summary *model.ExecutionSummary) time.Time {
	var latest time.Time
	if summary != nil {
		latest = summary.LatestRelevantUpdateAt().UTC()
		if captured := summary.CapturedAt.UTC(); captured.After(latest) {
			latest = captured
		}
	}
	if strings.TrimSpace(root) == "" || strings.TrimSpace(change.Slug) == "" {
		return latest
	}

	bundleDir, err := GovernedBundleDir(root, change)
	if err != nil {
		return latest
	}

	for _, path := range []string{
		filepath.Join(bundleDir, "intent.md"),
		filepath.Join(bundleDir, "requirements.md"),
		filepath.Join(bundleDir, "research.md"),
		filepath.Join(bundleDir, "decision.md"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return failClosedFreshnessUpdateAt(latest)
		}
		modTime := info.ModTime().UTC()
		if modTime.After(latest) {
			latest = modTime
		}
	}

	tasksPath := filepath.Join(bundleDir, "tasks.md")
	if isTasksPlanFreshnessRelevant(tasksPath, summary) {
		if info, err := os.Stat(tasksPath); err == nil {
			modTime := info.ModTime().UTC()
			if modTime.After(latest) {
				latest = modTime
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return failClosedFreshnessUpdateAt(latest)
		}
	}
	return latest
}

func collectTaskEvidenceFreshnessInputs(
	change model.Change,
	summary *model.ExecutionSummary,
	latestRelevantUpdateAt time.Time,
	defaultEvidenceTimestamp time.Time,
) []ctxpack.EvidenceFreshnessInput {
	if !ExecutionSummaryReady(summary) {
		return nil
	}

	inputs := []ctxpack.EvidenceFreshnessInput{}
	for _, task := range summary.Tasks {
		evidenceTs := task.CapturedAt.UTC()
		if evidenceTs.IsZero() {
			evidenceTs = defaultEvidenceTimestamp
		}
		expected := ExpectedExecutionTaskFreshnessInputs(change, summary.RunSummaryVersion, task.TaskID)
		inputs = append(inputs, ctxpack.EvidenceFreshnessInput{
			ExpectedStructuralInput: expected.FieldMap(),
			CurrentStructuralInput:  task.FreshnessInputs.FieldMap(),
			EvidenceTimestamp:       evidenceTs,
			LatestRelevantUpdateAt:  latestRelevantUpdateAt,
		})
	}
	return inputs
}

func isTasksPlanFreshnessRelevant(tasksPath string, summary *model.ExecutionSummary) bool {
	if summary == nil || strings.TrimSpace(summary.TasksPlanHash) == "" {
		return true
	}
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		return true
	}
	currentHash, err := wave.TaskPlanSemanticHash(string(raw))
	if err != nil {
		return true
	}
	return currentHash != strings.TrimSpace(summary.TasksPlanHash)
}
