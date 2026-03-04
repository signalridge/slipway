package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	ctxpack "github.com/signalridge/speclane/internal/engine/context"
	"github.com/signalridge/speclane/internal/fsutil"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
)

func projectRootFromWD() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := fsutil.FindProjectRoot(wd)
	if err != nil {
		return "", err
	}
	return root, nil
}

func loadConfigAtRoot(root string) (model.Config, error) {
	cfgPath := filepath.Join(root, ".spln", "config.yaml")
	cfg, err := model.LoadConfig(cfgPath)
	if err != nil {
		return model.Config{}, newCLIError(
			categoryStateIntegrity,
			"config_parse_failure",
			fmt.Sprintf("failed to load .spln/config.yaml: %v", err),
			"Run `spln repair` to back up broken config and rewrite deterministic defaults.",
			"",
			map[string]any{"path": cfgPath},
		)
	}
	return cfg, nil
}

func ensureRequestScopedActive(root string) (state.ActiveResolution, error) {
	resolution, err := state.ResolveActiveRequest(root)
	if err != nil {
		if errors.Is(err, state.ErrNoActiveRequest) {
			return state.ActiveResolution{}, fmt.Errorf("no active request; create one with `spln new`")
		}
		if errors.Is(err, state.ErrMultipleActiveRequests) || errors.Is(err, state.ErrSameRequestDualActive) {
			return state.ActiveResolution{}, fmt.Errorf("active request context is ambiguous; run `spln status` and `spln repair`")
		}
		return state.ActiveResolution{}, err
	}
	return resolution, nil
}

func projectNextReadyActions(currentState model.WorkflowState) []string {
	actions := []string{}
	if currentState == model.StateDone {
		return actions
	}
	actions = append(actions, "do")
	if currentState == model.StateS8Verify {
		actions = append(actions, "done")
	}
	if currentState == model.StateS6RunWaves || currentState == model.StateS7Review || currentState == model.StateS8Verify {
		actions = append(actions, "pivot")
	}
	actions = append(actions, "cancel")
	return actions
}

func withWorkspaceStateLock(root string, commandName string, run func() error) error {
	cfg, err := loadConfigAtRoot(root)
	if err != nil {
		return err
	}

	lockPath := filepath.Join(root, ".spln", "state.lock")
	lock := fsutil.NewStateLock(lockPath)
	timeout := time.Duration(cfg.Execution.LockWaitTimeoutSeconds) * time.Second
	held, err := lock.Acquire(context.Background(), timeout, "spln "+commandName)
	if err != nil {
		if errors.Is(err, fsutil.ErrLockTimeout) {
			return newCLIError(
				categoryPrecondition,
				"state_lock_timeout",
				fmt.Sprintf("state lock timeout while running `%s`", commandName),
				"Run `spln repair` to clear stale lock artifacts or retry after lock holder exits.",
				"",
				map[string]any{
					"lock_path": lockPath,
					"command":   commandName,
				},
			)
		}
		return err
	}
	defer func() {
		_ = held.Release()
	}()

	return run()
}

func withWorkspaceRepairLock(root string, run func(staleLockCleaned bool) error) error {
	cfg, err := loadConfigAtRoot(root)
	if err != nil {
		return err
	}
	lockPath := filepath.Join(root, ".spln", "state.lock")
	lock := fsutil.NewStateLock(lockPath)
	timeout := time.Duration(cfg.Execution.LockWaitTimeoutSeconds) * time.Second

	held, err := lock.Acquire(context.Background(), timeout, "spln repair")
	staleLockCleaned := false
	if err != nil {
		if !errors.Is(err, fsutil.ErrLockTimeout) {
			return err
		}

		staleAfter := time.Duration(cfg.Execution.LockStaleAfterSeconds) * time.Second
		staleLockCleaned, err = lock.CleanupStale(staleAfter, time.Now().UTC(), isPIDAlive)
		if err != nil {
			return err
		}
		if !staleLockCleaned {
			return newCLIError(
				categoryPrecondition,
				"state_lock_timeout",
				"state lock timeout while running `repair`",
				"Retry after lock holder exits.",
				"",
				map[string]any{
					"lock_path": lockPath,
					"command":   "repair",
				},
			)
		}

		held, err = lock.Acquire(context.Background(), timeout, "spln repair")
		if err != nil {
			if errors.Is(err, fsutil.ErrLockTimeout) {
				return newCLIError(
					categoryPrecondition,
					"state_lock_timeout",
					"state lock timeout while running `repair`",
					"Retry after lock holder exits.",
					"",
					map[string]any{
						"lock_path": lockPath,
						"command":   "repair",
					},
				)
			}
			return err
		}
	}
	defer func() {
		_ = held.Release()
	}()

	return run(staleLockCleaned)
}

func projectFreshnessForLane(
	root string,
	requestID string,
	latestRunSummaryVersion int,
	level model.Level,
	levelSource model.LevelSource,
	routeSnapshot model.RouteSnapshot,
	taskRuns map[string]model.TaskRun,
	blockers []string,
	actionHistory []model.ActionEvent,
) string {
	if len(blockers) > 0 {
		return string(ctxpack.EvidenceFreshnessStale)
	}
	if latestRunSummaryVersion < 1 || requestID == "" {
		return string(ctxpack.EvidenceFreshnessUnknown)
	}

	runSummaryPath := filepath.Join(
		root,
		".spln",
		"evidence",
		"runs",
		requestID,
		fmt.Sprintf("rv%d.json", latestRunSummaryVersion),
	)
	info, err := os.Stat(runSummaryPath)
	if err != nil {
		return string(ctxpack.EvidenceFreshnessUnknown)
	}

	evidenceTimestamp := info.ModTime().UTC()
	latestRelevantUpdateAt := evidenceTimestamp
	for _, event := range actionHistory {
		if event.Action != "analyze" && event.Action != "pivot" {
			continue
		}
		ts := event.Timestamp.UTC()
		if ts.IsZero() {
			continue
		}
		if ts.After(latestRelevantUpdateAt) {
			latestRelevantUpdateAt = ts
		}
	}

	inputs := collectTaskEvidenceFreshnessInputs(
		requestID,
		latestRunSummaryVersion,
		level,
		levelSource,
		routeSnapshot,
		taskRuns,
		latestRelevantUpdateAt,
		evidenceTimestamp,
	)
	if len(inputs) == 0 {
		inputs = []ctxpack.EvidenceFreshnessInput{
			{
				EvidenceTimestamp:      evidenceTimestamp,
				LatestRelevantUpdateAt: latestRelevantUpdateAt,
			},
		}
	}

	freshness := ctxpack.EvaluateEvidenceFreshness(true, inputs)
	return string(freshness)
}

func collectTaskEvidenceFreshnessInputs(
	requestID string,
	latestRunSummaryVersion int,
	level model.Level,
	levelSource model.LevelSource,
	routeSnapshot model.RouteSnapshot,
	taskRuns map[string]model.TaskRun,
	latestRelevantUpdateAt time.Time,
	defaultEvidenceTimestamp time.Time,
) []ctxpack.EvidenceFreshnessInput {
	if latestRunSummaryVersion < 1 || len(taskRuns) == 0 {
		return nil
	}
	keys := make([]string, 0, len(taskRuns))
	for key := range taskRuns {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	inputs := []ctxpack.EvidenceFreshnessInput{}
	for _, key := range keys {
		run := taskRuns[key]
		if run.RunSummaryVersion != latestRunSummaryVersion {
			continue
		}
		if strings.TrimSpace(run.EvidenceRef) == "" {
			continue
		}

		currentHash, hashErr := computeTaskEvidenceInputHash(
			requestID,
			run.RunSummaryVersion,
			run.TaskID,
			level,
			levelSource,
			routeSnapshot,
		)
		if hashErr != nil {
			continue
		}

		record, evidenceTs := readTaskEvidenceRecord(run.EvidenceRef)
		if evidenceTs.IsZero() {
			evidenceTs = defaultEvidenceTimestamp
		}
		inputs = append(inputs, ctxpack.EvidenceFreshnessInput{
			EvidenceInputHash:      strings.TrimSpace(record.InputHash),
			CurrentInputHash:       currentHash,
			EvidenceTimestamp:      evidenceTs,
			LatestRelevantUpdateAt: latestRelevantUpdateAt,
		})
	}
	return inputs
}

type taskEvidenceRecord struct {
	TaskID            string `json:"task_id"`
	RunSummaryVersion int    `json:"run_summary_version"`
	InputHash         string `json:"input_hash,omitempty"`
	CapturedAt        string `json:"captured_at,omitempty"`
}

func readTaskEvidenceRecord(path string) (taskEvidenceRecord, time.Time) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return taskEvidenceRecord{}, time.Time{}
	}
	record := taskEvidenceRecord{}
	if err := json.Unmarshal(raw, &record); err != nil {
		return taskEvidenceRecord{}, time.Time{}
	}
	if strings.TrimSpace(record.CapturedAt) != "" {
		if ts, err := time.Parse(time.RFC3339Nano, record.CapturedAt); err == nil {
			return record, ts.UTC()
		}
	}
	if info, err := os.Stat(path); err == nil {
		return record, info.ModTime().UTC()
	}
	return record, time.Time{}
}

func computeTaskEvidenceInputHash(
	requestID string,
	runSummaryVersion int,
	taskID string,
	level model.Level,
	levelSource model.LevelSource,
	routeSnapshot model.RouteSnapshot,
) (string, error) {
	normalizedRoute := map[string]any{
		"guardrail_domain": strings.TrimSpace(routeSnapshot.GuardrailDomain),
		"scores": map[string]int{
			"novelty":            routeSnapshot.Scores.Novelty,
			"ambiguity":          routeSnapshot.Scores.Ambiguity,
			"impact":             routeSnapshot.Scores.Impact,
			"risk":               routeSnapshot.Scores.Risk,
			"reversibility_cost": routeSnapshot.Scores.ReversibilityCost,
		},
		"routing_rationale":  cloneSortedStrings(routeSnapshot.RoutingRationale),
		"blocking_conflicts": cloneSortedStrings(routeSnapshot.BlockingConflicts),
	}
	return model.ComputeInputHash(map[string]any{
		"request_id":          requestID,
		"run_summary_version": runSummaryVersion,
		"task_id":             taskID,
		"level":               level,
		"level_source":        levelSource,
		"route_snapshot":      normalizedRoute,
	})
}

func cloneSortedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := append([]string(nil), values...)
	slices.Sort(out)
	return out
}
