package state

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
	LatestRunVersion int
}

func LoadRelevantExecutionSummaryContext(root string, change model.Change) (RelevantExecutionSummaryContext, error) {
	summary, err := LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return RelevantExecutionSummaryContext{}, err
	}
	ctx := RelevantExecutionSummaryContext{
		Summary: summary,
		Issues:  collectExecutionSummaryIssues(root, change, summary),
	}
	if ExecutionSummaryReady(summary) {
		ctx.LatestRunVersion = summary.RunSummaryVersion
	}
	return ctx, nil
}

func SaveExecutionSummary(root, slug string, summary model.ExecutionSummary) error {
	summary.Normalize()
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

func collectExecutionSummaryIssues(root string, change model.Change, summary *model.ExecutionSummary) []string {
	if !ExecutionSummaryRelevantState(change.CurrentState) || !ExecutionSummaryReady(summary) || strings.TrimSpace(change.Slug) == "" {
		return nil
	}

	blockers := make([]string, 0, len(summary.OpenBlockers)+1)
	blockers = append(blockers, model.ReasonSpecs(summary.OpenBlockers)...)
	if ExecutionSummaryFreshness(root, change, summary) == ctxpack.EvidenceFreshnessStale {
		if planningInputsChangedAfterExecution(root, change, summary) {
			blockers = append(blockers, StalePlanningEvidenceBlockerToken)
		} else {
			blockers = append(blockers, StaleExecutionEvidenceBlockerToken)
		}
	}
	return stringutil.UniqueSorted(blockers)
}

func ExecutionSummaryFreshness(root string, change model.Change, summary *model.ExecutionSummary) ctxpack.EvidenceFreshness {
	if !ExecutionSummaryReady(summary) || strings.TrimSpace(change.Slug) == "" {
		return ctxpack.EvidenceFreshnessUnknown
	}

	evidenceTimestamp := summary.CapturedAt.UTC()
	latestRelevantUpdateAt := latestExecutionRelevantUpdateAt(root, change, summary)
	inputs := collectTaskEvidenceFreshnessInputs(change, summary, latestRelevantUpdateAt, evidenceTimestamp)
	if len(inputs) == 0 {
		inputs = []ctxpack.EvidenceFreshnessInput{{
			EvidenceTimestamp:      evidenceTimestamp,
			LatestRelevantUpdateAt: latestRelevantUpdateAt,
		}}
	}
	return ctxpack.EvaluateEvidenceFreshness(true, inputs)
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

func planningInputsChangedAfterExecution(root string, change model.Change, summary *model.ExecutionSummary) bool {
	if !ExecutionSummaryReady(summary) || strings.TrimSpace(root) == "" || strings.TrimSpace(change.Slug) == "" {
		return false
	}
	bundleDir, err := GovernedBundleDir(root, change)
	if err != nil {
		return false
	}
	latestEvidenceAt := summary.LatestRelevantUpdateAt().UTC()
	if captured := summary.CapturedAt.UTC(); captured.After(latestEvidenceAt) {
		latestEvidenceAt = captured
	}

	for _, path := range []string{
		filepath.Join(bundleDir, "intent.md"),
		filepath.Join(bundleDir, "requirements.md"),
		filepath.Join(bundleDir, "research.md"),
		filepath.Join(bundleDir, "decision.md"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().UTC().After(latestEvidenceAt) {
			return true
		}
	}

	tasksPath := filepath.Join(bundleDir, "tasks.md")
	if strings.TrimSpace(summary.TasksPlanHash) != "" {
		return isTasksPlanFreshnessRelevant(tasksPath, summary)
	}
	info, err := os.Stat(tasksPath)
	return err == nil && info.ModTime().UTC().After(latestEvidenceAt)
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
		currentHash, hashErr := ComputeTaskEvidenceInputHash(
			change.Slug,
			summary.RunSummaryVersion,
			task.TaskID,
			change.GuardrailDomain,
		)
		if hashErr != nil {
			inputs = append(inputs, ctxpack.EvidenceFreshnessInput{
				EvidenceInputHash:      "hash_error",
				CurrentInputHash:       "force_stale",
				LatestRelevantUpdateAt: latestRelevantUpdateAt,
			})
			continue
		}

		evidenceTs := task.CapturedAt.UTC()
		if evidenceTs.IsZero() {
			evidenceTs = defaultEvidenceTimestamp
		}
		inputs = append(inputs, ctxpack.EvidenceFreshnessInput{
			EvidenceInputHash:      strings.TrimSpace(task.EvidenceInputHash),
			CurrentInputHash:       currentHash,
			EvidenceTimestamp:      evidenceTs,
			LatestRelevantUpdateAt: latestRelevantUpdateAt,
		})
	}
	return inputs
}

func ComputeTaskEvidenceInputHash(
	slug string,
	runSummaryVersion int,
	taskID string,
	guardrailDomain string,
) (string, error) {
	return model.ComputeInputHash(map[string]any{
		"change_id":           slug,
		"run_summary_version": runSummaryVersion,
		"task_id":             taskID,
		"guardrail_domain":    strings.TrimSpace(guardrailDomain),
	})
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
