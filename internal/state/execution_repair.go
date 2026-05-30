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

				pruned, pruneErr := pruneOrphanTaskEvidence(root, change.Slug, summary.RunSummaryVersion, PlannedTaskIDSet(*plan))
				if pruneErr != nil {
					result.NonRepairableFindings = append(result.NonRepairableFindings, fmt.Sprintf("%s: prune orphan task evidence: %v", change.Slug, pruneErr))
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
	return "Run `slipway pivot --rescope` or restore the historical tasks.md before rerunning `slipway repair`."
}

func wavePlanRepairBlockedReason(root string, change model.Change, summary *model.ExecutionSummary) (string, error) {
	if !ExecutionSummaryReady(summary) {
		return "", nil
	}

	currentHash, tasksUpdatedAt, nodes, err := currentTaskPlanNodes(root, change)
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
		return fmt.Sprintf("current tasks.md hash %q no longer matches execution summary hash %q", currentHash, summaryHash), nil
	}

	if summaryHash != "" || currentHash == "" || tasksUpdatedAt.IsZero() {
		return "", nil
	}

	staleTasks := make([]string, 0, len(summary.Tasks))
	for _, task := range summary.Tasks {
		capturedAt := task.CapturedAt.UTC()
		if capturedAt.IsZero() || capturedAt.Before(tasksUpdatedAt) {
			staleTasks = append(staleTasks, task.TaskID)
		}
	}
	slices.Sort(staleTasks)
	if len(staleTasks) > 0 {
		return fmt.Sprintf("current tasks.md changed after execution evidence was captured for: %s", strings.Join(staleTasks, ", ")), nil
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
	recovered, err := BuildWaveRuns(plan, summary.RunSummaryVersion, summary.Tasks)
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
			!slices.Equal(leftCopy[i].TaskRuns, rightCopy[i].TaskRuns) {
			return false
		}
	}
	return true
}

func orphanTaskEvidence(root, slug string, runVersion int, allowed map[string]struct{}) ([]string, error) {
	dir := EvidenceTasksDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	orphaned := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		evidenceRunVersion, err := flatTaskEvidenceRunVersion(raw)
		if err != nil {
			return nil, fmt.Errorf("classify task evidence %s: %w", DisplayPath(root, path), err)
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
	return orphaned, nil
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

func pruneOrphanTaskEvidence(root, slug string, runVersion int, allowed map[string]struct{}) ([]string, error) {
	paths, err := orphanTaskEvidence(root, slug, runVersion, allowed)
	if err != nil {
		return nil, err
	}
	pruned := []string{}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return nil, err
		}
		pruned = append(pruned, filepath.ToSlash(filepath.Join(slug, filepath.Base(path))))
	}
	return pruned, nil
}
