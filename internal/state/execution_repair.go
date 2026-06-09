package state

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

	"github.com/signalridge/slipway/internal/model"
)

type ExecutionRepairResult struct {
	MaterializedWavePlans []string
	RecoveredWaveRuns     []string
	ClearedCheckpoints    []string
	RepairedCheckpoints   []string
	PrunedTaskEvidence    []string
	NonRepairableFindings []string
}

func RepairExecutionState(root string, now time.Time, staleAfter time.Duration) (ExecutionRepairResult, error) {
	allChanges, _, err := ListChangesBestEffortWithIssues(root)
	if err != nil {
		return ExecutionRepairResult{}, err
	}

	result := ExecutionRepairResult{}
	for _, change := range allChanges {
		if change.Status != model.ChangeStatusActive {
			continue
		}

		changed := false

		var summary *model.ExecutionSummary
		var summaryErr error
		if relevantWaveExecutionState(change.CurrentState) {
			summary, summaryErr = LoadOptionalRelevantExecutionSummary(root, change)

			// Wave-plan materialization and checkpoint repair must run even
			// when the execution summary is unreadable, so a wedged or stale
			// checkpoint can still self-heal. A corrupt summary is treated as
			// absent for plan reconstruction so it cannot block plan repair;
			// only the summary-dependent recovery/prune below is gated on a
			// readable, ready summary.
			summaryForPlan := summary
			if summaryErr != nil {
				summaryForPlan = nil
			}

			plan, planChanged, blockedReason, err := ensureWavePlan(root, change, summaryForPlan)
			if err != nil {
				result.NonRepairableFindings = append(result.NonRepairableFindings, fmt.Sprintf("%s: %v", change.Slug, err))
			} else if strings.TrimSpace(blockedReason) != "" {
				result.NonRepairableFindings = append(
					result.NonRepairableFindings,
					fmt.Sprintf("%s: wave plan repair blocked: %s. %s", change.Slug, blockedReason, wavePlanRepairHint()),
				)
			} else if planChanged {
				result.MaterializedWavePlans = append(result.MaterializedWavePlans, change.Slug)
			}

			if repaired, cleared := repairCheckpointAgainstWavePlan(&change, plan, now, staleAfter); repaired {
				changed = true
				if cleared {
					result.ClearedCheckpoints = append(result.ClearedCheckpoints, change.Slug)
				} else {
					result.RepairedCheckpoints = append(result.RepairedCheckpoints, change.Slug)
				}
			}

			if summaryErr != nil {
				result.NonRepairableFindings = append(result.NonRepairableFindings, fmt.Sprintf("%s: execution summary unreadable: %v", change.Slug, summaryErr))
			} else if plan != nil && ExecutionSummaryReady(summary) {
				recovered, recoveryErr := recoverWaveRunsFromSummary(root, change.Slug, *plan, *summary)
				if recoveryErr != nil {
					result.NonRepairableFindings = append(result.NonRepairableFindings, fmt.Sprintf("%s: wave run recovery failed: %v", change.Slug, recoveryErr))
				} else if recovered {
					result.RecoveredWaveRuns = append(result.RecoveredWaveRuns, change.Slug)
				}

				pruned, taskEvidenceIssues, pruneErr := pruneOrphanTaskEvidence(root, change.Slug, summary.RunSummaryVersion, PlannedTaskIDSet(*plan))
				if pruneErr != nil {
					result.NonRepairableFindings = append(result.NonRepairableFindings, fmt.Sprintf("%s: prune orphan task evidence: %v", change.Slug, pruneErr))
				}
				for _, issue := range taskEvidenceIssues {
					result.NonRepairableFindings = append(result.NonRepairableFindings, fmt.Sprintf("%s: task evidence unreadable: %s", change.Slug, issue.message(root)))
				}
				if len(pruned) > 0 {
					result.PrunedTaskEvidence = append(result.PrunedTaskEvidence, pruned...)
				}
			}
		}

		if changed {
			if err := SaveChange(root, change); err != nil {
				return ExecutionRepairResult{}, err
			}
		}
	}

	slices.Sort(result.MaterializedWavePlans)
	slices.Sort(result.RecoveredWaveRuns)
	slices.Sort(result.ClearedCheckpoints)
	slices.Sort(result.RepairedCheckpoints)
	slices.Sort(result.PrunedTaskEvidence)
	slices.Sort(result.NonRepairableFindings)
	result.MaterializedWavePlans = slices.Compact(result.MaterializedWavePlans)
	result.RecoveredWaveRuns = slices.Compact(result.RecoveredWaveRuns)
	result.ClearedCheckpoints = slices.Compact(result.ClearedCheckpoints)
	result.RepairedCheckpoints = slices.Compact(result.RepairedCheckpoints)
	result.PrunedTaskEvidence = slices.Compact(result.PrunedTaskEvidence)
	result.NonRepairableFindings = slices.Compact(result.NonRepairableFindings)
	return result, nil
}

func relevantWaveExecutionState(state model.WorkflowState) bool {
	switch state {
	case model.StateS2Execute, model.StateS3Review, model.StateS4Verify:
		return true
	default:
		return false
	}
}

func ensureWavePlan(root string, change model.Change, summary *model.ExecutionSummary) (*model.WavePlan, bool, string, error) {
	plan, err := LoadOptionalWavePlanForChange(root, change)
	if err == nil && plan != nil {
		planChanged, blockedReason, err := wavePlanRepairDrift(root, change, *plan, summary)
		if err != nil {
			return nil, false, "", err
		}
		if strings.TrimSpace(blockedReason) != "" {
			return plan, false, blockedReason, nil
		}
		if planChanged {
			materialized, materializeErr := MaterializeWavePlan(root, change)
			if materializeErr != nil {
				return nil, false, "", fmt.Errorf("wave plan stale and could not be rematerialized: %w", materializeErr)
			}
			return &materialized, true, "", nil
		}
		return plan, false, "", nil
	}
	unreadable := err != nil
	if blockedReason, err := wavePlanRepairBlockedReason(root, change, summary); err != nil {
		return nil, false, "", err
	} else if strings.TrimSpace(blockedReason) != "" {
		return nil, false, blockedReason, nil
	}
	materialized, materializeErr := MaterializeWavePlan(root, change)
	if materializeErr != nil {
		if unreadable {
			return nil, false, "", fmt.Errorf("wave plan unreadable and could not be reconstructed: %w", materializeErr)
		}
		return nil, false, "", fmt.Errorf("wave plan missing and could not be materialized: %w", materializeErr)
	}
	return &materialized, true, "", nil
}

func wavePlanRepairHint() string {
	return "Run `slipway run` to reopen the owning planning stage or rebuild compatible generated evidence; restore the historical tasks.md only if recovering an old execution boundary."
}

func wavePlanRepairDrift(root string, change model.Change, plan model.WavePlan, summary *model.ExecutionSummary) (bool, string, error) {
	currentStructuralHash, err := CurrentTasksPlanStructuralState(root, change)
	if err != nil {
		return false, "", err
	}
	currentScopeHash, err := CurrentTasksPlanScopeState(root, change)
	if err != nil {
		return false, "", err
	}
	plan.Normalize()
	planStructuralHash := strings.TrimSpace(plan.EffectiveStructuralHash)
	if planStructuralHash == "" {
		planStructuralHash = strings.TrimSpace(plan.TasksPlanStructuralHash)
	}
	if planStructuralHash == "" {
		planStructuralHash = strings.TrimSpace(plan.TasksPlanHash)
	}
	if planStructuralHash != "" && currentStructuralHash != "" && planStructuralHash != currentStructuralHash {
		if ExecutionSummaryReady(summary) {
			return false, fmt.Sprintf("current tasks.md structural hash %q no longer matches wave plan hash %q", currentStructuralHash, planStructuralHash), nil
		}
		return true, "", nil
	}
	// Structure matched above; a scope drift (including a plan that predates the
	// scope-hash field, where planScopeHash is empty) rebuilds in place to refresh
	// target_files and backfill the scope hash rather than reusing a stale plan.
	planScopeHash := strings.TrimSpace(plan.TasksPlanScopeHash)
	if currentScopeHash != "" && planScopeHash != currentScopeHash {
		return true, "", nil
	}
	return false, "", nil
}

func wavePlanRepairBlockedReason(root string, change model.Change, summary *model.ExecutionSummary) (string, error) {
	if !ExecutionSummaryReady(summary) {
		return "", nil
	}

	currentHash, err := CurrentTasksPlanStructuralState(root, change)
	if err != nil {
		return "", err
	}
	_, nodes, err := currentTaskPlanNodes(root, change)
	if err != nil {
		return "", err
	}

	currentTaskIDs := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		currentTaskIDs[node.TaskID] = struct{}{}
	}

	missingTasks := make([]string, 0, len(summary.Tasks))
	for _, task := range summary.Tasks {
		if _, ok := currentTaskIDs[task.TaskID]; ok {
			continue
		}
		missingTasks = append(missingTasks, task.TaskID)
	}
	slices.Sort(missingTasks)
	if len(missingTasks) > 0 {
		return fmt.Sprintf("current tasks.md no longer contains executed tasks: %s", strings.Join(missingTasks, ", ")), nil
	}

	summaryHash := strings.TrimSpace(summary.TasksPlanHash)
	if summaryHash != "" && currentHash != "" && summaryHash != currentHash {
		return fmt.Sprintf("current tasks.md structural hash %q no longer matches execution summary hash %q", currentHash, summaryHash), nil
	}

	return "", nil
}

func repairCheckpointAgainstWavePlan(change *model.Change, plan *model.WavePlan, now time.Time, staleAfter time.Duration) (repaired bool, cleared bool) {
	if change == nil || change.ActiveCheckpoint == nil {
		return false, false
	}
	if change.CurrentState != model.StateS2Execute {
		change.ActiveCheckpoint = nil
		return true, true
	}
	if plan == nil {
		return false, false
	}
	if staleAfter > 0 && !change.ActiveCheckpoint.PausedAt.IsZero() && now.UTC().Sub(change.ActiveCheckpoint.PausedAt) > staleAfter {
		change.ActiveCheckpoint = nil
		return true, true
	}
	expectedWaveIndex := plan.WaveIndexForTask(change.ActiveCheckpoint.PausedTaskID)
	if expectedWaveIndex == 0 {
		change.ActiveCheckpoint = nil
		return true, true
	}
	if change.ActiveCheckpoint.PausedWaveIndex == expectedWaveIndex && !change.ActiveCheckpoint.PausedAt.IsZero() {
		return false, false
	}
	change.ActiveCheckpoint.PausedWaveIndex = expectedWaveIndex
	if change.ActiveCheckpoint.PausedAt.IsZero() {
		change.ActiveCheckpoint.PausedAt = now.UTC()
	}
	return true, false
}

func recoverWaveRunsFromSummary(root, slug string, plan model.WavePlan, summary model.ExecutionSummary) (bool, error) {
	existing, err := LoadOptionalWaveRuns(root, slug, summary.RunSummaryVersion)
	unreadable := err != nil
	dispatchModes, err := waveDispatchModesForSummary(root, slug, summary)
	if err != nil {
		return false, err
	}
	plan = ApplyEffectiveParallel(plan, EffectiveForcedParallel(root))
	recovered, err := BuildWaveRuns(plan, summary.RunSummaryVersion, summary.Tasks, dispatchModes)
	if err != nil {
		return false, err
	}
	if !unreadable && waveRunsEquivalent(existing, recovered) {
		return false, nil
	}
	if err := SaveWaveRuns(root, slug, summary.RunSummaryVersion, recovered); err != nil {
		return false, err
	}
	return true, nil
}

func waveDispatchModesForSummary(
	root, slug string,
	summary model.ExecutionSummary,
) (map[int]model.WaveDispatchMode, error) {
	record, err := LoadVerification(root, slug, "wave-orchestration")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("load wave-orchestration verification for dispatch modes: %w", err)
	}
	if !record.IsPassing() || record.RunVersion != summary.RunSummaryVersion {
		return nil, nil
	}
	dispatchModes, err := model.WaveDispatchModesFromVerification(record)
	if err != nil {
		return nil, fmt.Errorf("parse wave-orchestration dispatch modes: %w", err)
	}
	return dispatchModes, nil
}

func waveRunsEquivalent(left, right []model.WaveRun) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]model.WaveRun(nil), left...)
	rightCopy := append([]model.WaveRun(nil), right...)
	slices.SortFunc(leftCopy, func(a, b model.WaveRun) int { return a.WaveIndex - b.WaveIndex })
	slices.SortFunc(rightCopy, func(a, b model.WaveRun) int { return a.WaveIndex - b.WaveIndex })
	for i := range leftCopy {
		leftCopy[i].Normalize()
		rightCopy[i].Normalize()
		if leftCopy[i].WaveIndex != rightCopy[i].WaveIndex ||
			leftCopy[i].RunSummaryVersion != rightCopy[i].RunSummaryVersion ||
			!leftCopy[i].StartedAt.Equal(rightCopy[i].StartedAt) ||
			!leftCopy[i].CompletedAt.Equal(rightCopy[i].CompletedAt) ||
			leftCopy[i].Verdict != rightCopy[i].Verdict ||
			leftCopy[i].DispatchMode != rightCopy[i].DispatchMode ||
			!slices.Equal(leftCopy[i].TaskRuns, rightCopy[i].TaskRuns) {
			return false
		}
	}
	return true
}

type taskEvidenceScanIssue struct {
	Path string
	Err  error
}

func (i taskEvidenceScanIssue) message(root string) string {
	if i.Err == nil {
		return DisplayPath(root, i.Path)
	}
	return fmt.Sprintf("%s: %v", DisplayPath(root, i.Path), i.Err)
}

func orphanTaskEvidence(root, slug string, runVersion int, allowed map[string]struct{}) ([]string, []taskEvidenceScanIssue, error) {
	dir := EvidenceTasksDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	orphaned := []string{}
	issues := []taskEvidenceScanIssue{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			issues = append(issues, taskEvidenceScanIssue{Path: path, Err: err})
			continue
		}
		evidenceRunVersion, err := flatTaskEvidenceRunVersion(raw)
		if err != nil {
			issues = append(issues, taskEvidenceScanIssue{Path: path, Err: err})
			continue
		}
		if evidenceRunVersion != runVersion {
			continue
		}
		taskID := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if _, ok := allowed[taskID]; ok {
			continue
		}
		orphaned = append(orphaned, path)
	}
	slices.Sort(orphaned)
	slices.SortFunc(issues, func(a, b taskEvidenceScanIssue) int {
		return strings.Compare(a.Path, b.Path)
	})
	return orphaned, issues, nil
}

func flatTaskEvidenceRunVersion(raw []byte) (int, error) {
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

func pruneOrphanTaskEvidence(root, slug string, runVersion int, allowed map[string]struct{}) ([]string, []taskEvidenceScanIssue, error) {
	paths, issues, err := orphanTaskEvidence(root, slug, runVersion, allowed)
	if err != nil {
		return nil, issues, err
	}
	pruned := []string{}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return nil, issues, err
		}
		pruned = append(pruned, filepath.ToSlash(filepath.Join(slug, filepath.Base(path))))
	}
	return pruned, issues, nil
}
