package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
)

// repairSummary reports the results of bounded local integrity repairs.
// Cleanup operations: CleanedAtomicTemps, StaleLockCleaned.
// Restore-to-contract: ConfigBackupPath (backs up corrupt config before
// restoring to .slipway.yaml).
// NonRepairableFindings require operator intervention (e.g. dual-active anomaly).
type repairSummary struct {
	CleanedAtomicTemps        []string `json:"cleaned_atomic_temps,omitempty"`
	ConfigBackupPath          string   `json:"config_backup_path,omitempty"`
	StaleLockCleaned          bool     `json:"stale_lock_cleaned"`
	WorktreeScopeRepairs      []string `json:"worktree_scope_repairs,omitempty"`
	MaterializedWavePlans     []string `json:"materialized_wave_plans,omitempty"`
	RecoveredWaveRuns         []string `json:"recovered_wave_runs,omitempty"`
	ClearedCheckpoints        []string `json:"cleared_checkpoints,omitempty"`
	RepairedCheckpoints       []string `json:"repaired_checkpoints,omitempty"`
	PrunedTaskEvidence        []string `json:"pruned_task_evidence,omitempty"`
	RebuiltExecutionSummaries []string `json:"rebuilt_execution_summaries,omitempty"`
	NonRepairableFindings     []string `json:"non_repairable_findings,omitempty"`
	Mode                      string   `json:"mode,omitempty"`
}

func makeRepairCmd() *cobra.Command {
	var jsonOutput bool
	var focus string
	var listFocuses bool
	var discoveryFormat string
	cmd := &cobra.Command{
		Use:   "repair",
		Short: desc("repair"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if listFocuses {
				return emitFocusDiscovery(cmd, "repair", discoveryFormat)
			}
			if err := validateFocus("repair", focus); err != nil {
				return err
			}
			effectiveMode := resolveEffectiveFocus("repair", focus)
			root, err := repairRootFromCommand(cmd)
			if err != nil {
				return err
			}
			return withWorkspaceRepairLock(root, func(staleLockCleaned bool) error {
				now := time.Now().UTC()
				summary := repairSummary{
					StaleLockCleaned: staleLockCleaned,
					Mode:             effectiveMode,
				}

				cleaned, err := fsutil.CleanupAtomicTempArtifacts(root)
				if err != nil {
					return err
				}
				summary.CleanedAtomicTemps = cleaned

				if backupPath, err := state.RepairCorruptConfig(root, now); err == nil {
					summary.ConfigBackupPath = backupPath
				}
				worktreeScopeRepairs, err := state.RepairBoundWorktreeScopeMetadata(root)
				if err != nil {
					return err
				}
				summary.WorktreeScopeRepairs = worktreeScopeRepairs

				cfg, err := loadConfigAtRoot(root)
				if err != nil {
					return err
				}

				allChanges, changeIssues, err := state.ListChangesBestEffortWithIssues(root)
				if err != nil {
					return err
				}
				orphanSlugs, err := state.OrphanBundleSlugs(root)
				if err != nil {
					return err
				}
				archivedSlugs, err := state.ListArchivedChangeSlugs(root)
				if err != nil {
					return err
				}

				// Clean stale per-change locks plus workspace-scoped creation lock.
				staleAfter := time.Duration(cfg.Execution.LockStaleAfterSeconds) * time.Second
				staleLockPaths := changeCreateLockPaths(root)
				lockSlugs := map[string]struct{}{}
				for _, ch := range allChanges {
					lockSlugs[ch.Slug] = struct{}{}
				}
				for _, issue := range changeIssues {
					lockSlugs[issue.Slug] = struct{}{}
				}
				for _, slug := range orphanSlugs {
					lockSlugs[slug] = struct{}{}
				}
				for slug := range lockSlugs {
					staleLockPaths = append(staleLockPaths, state.ChangeStateLockPath(root, slug))
				}
				for _, lockPath := range staleLockPaths {
					lock := fsutil.NewStateLock(lockPath)
					if cleaned, err := lock.CleanupStale(staleAfter, now, isPIDAlive); err == nil && cleaned {
						summary.StaleLockCleaned = true
					}
				}

				execRepair, err := state.RepairExecutionState(root, now, staleAfter)
				if err != nil {
					return err
				}
				summary.MaterializedWavePlans = execRepair.MaterializedWavePlans
				summary.RecoveredWaveRuns = execRepair.RecoveredWaveRuns
				summary.ClearedCheckpoints = execRepair.ClearedCheckpoints
				summary.RepairedCheckpoints = execRepair.RepairedCheckpoints
				summary.PrunedTaskEvidence = execRepair.PrunedTaskEvidence
				summary.NonRepairableFindings = append(summary.NonRepairableFindings, execRepair.NonRepairableFindings...)

				rebuiltSummaries, rebuildFindings, err := rebuildExecutionSummaries(root, now)
				if err != nil {
					return err
				}
				summary.RebuiltExecutionSummaries = rebuiltSummaries
				summary.NonRepairableFindings = append(summary.NonRepairableFindings, rebuildFindings...)

				for _, slug := range archivedSlugs {
					if _, err := state.RepairArchivedTerminalStatus(root, slug); err != nil {
						return err
					}
				}

				// Check for dual-active anomaly.
				for _, issue := range changeIssues {
					summary.NonRepairableFindings = append(
						summary.NonRepairableFindings,
						fmt.Sprintf("%s: change authority unreadable: %v", issue.Slug, issue.Err),
					)
				}
				for _, slug := range orphanSlugs {
					summary.NonRepairableFindings = append(
						summary.NonRepairableFindings,
						fmt.Sprintf("bundle directory exists without change.yaml: %s", slug),
					)
				}
				for _, ch := range allChanges {
					diagnostics := state.DiagnoseBundleConsistency(root, ch)
					for _, msg := range diagnostics.Errors {
						summary.NonRepairableFindings = append(
							summary.NonRepairableFindings,
							fmt.Sprintf("%s: %s", ch.Slug, msg),
						)
					}
				}
				healthReport, err := state.CollectHealthReport(root)
				if err != nil {
					return err
				}
				for _, finding := range healthReport.Findings {
					switch finding.Category {
					case "bundle_integrity", "execution_summary":
						if message := repairSummaryForHealthFinding(finding); message != "" {
							summary.NonRepairableFindings = append(summary.NonRepairableFindings, message)
						}
					}
				}
				var activeChanges []model.Change
				for _, ch := range allChanges {
					if ch.Status == model.ChangeStatusActive {
						activeChanges = append(activeChanges, ch)
					}
				}
				if len(activeChanges) > 1 {
					unique := map[string]struct{}{}
					for _, ch := range activeChanges {
						unique[ch.Slug] = struct{}{}
					}
					if len(unique) > 1 {
						summary.NonRepairableFindings = append(
							summary.NonRepairableFindings,
							"multiple active changes require operator intervention",
						)
					}
				}

				slices.Sort(summary.NonRepairableFindings)
				summary.NonRepairableFindings = slices.Compact(summary.NonRepairableFindings)
				if repairSummaryHasLifecycleActivity(summary) {
					for _, ch := range activeChanges {
						if err := appendCLILifecycleEvent(root, ch, state.LifecycleEvent{
							Command:     "repair",
							EventType:   "repair.applied",
							Action:      "inspected",
							Reason:      effectiveMode,
							Result:      "completed",
							BeforeState: ch.CurrentState,
							AfterState:  ch.CurrentState,
							Diagnostics: lifecycleRepairDiagnostics(summary),
						}); err != nil {
							return err
						}
					}
				}
				if jsonOutput {
					return encodeJSONResponse(cmd, summary)
				}
				return writeRepairText(cmd.OutOrStdout(), summary)
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
	cmd.Flags().StringVar(&focus, "focus", "", "Repair focus (e.g. sast)")
	cmd.Flags().BoolVar(&listFocuses, "list-focuses", false, "List public --focus aliases for this command and exit")
	cmd.Flags().StringVar(&discoveryFormat, "format", "text", "Output format for --list-focuses: text|json")
	return cmd
}

func writeRepairText(w io.Writer, summary repairSummary) error {
	writer := newFormatWriter(w)
	writer.Writef("Repair Summary\n")
	if strings.TrimSpace(summary.Mode) != "" {
		writer.Writef("Mode: %s\n", summary.Mode)
	}
	writeRepairSection := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		writer.Writef("%s:\n", title)
		for _, item := range items {
			writer.Writef("  - %s\n", item)
		}
	}

	if summary.StaleLockCleaned {
		writer.Writef("Stale locks cleaned\n")
	}
	if strings.TrimSpace(summary.ConfigBackupPath) != "" {
		writer.Writef("Config backup: %s\n", summary.ConfigBackupPath)
	}

	writeRepairSection("Cleaned atomic temp artifacts", summary.CleanedAtomicTemps)
	writeRepairSection("Worktree scope repairs", summary.WorktreeScopeRepairs)
	writeRepairSection("Materialized wave plans", summary.MaterializedWavePlans)
	writeRepairSection("Recovered wave runs", summary.RecoveredWaveRuns)
	writeRepairSection("Cleared checkpoints", summary.ClearedCheckpoints)
	writeRepairSection("Repaired checkpoints", summary.RepairedCheckpoints)
	writeRepairSection("Pruned task evidence", summary.PrunedTaskEvidence)
	writeRepairSection("Rebuilt execution summaries", summary.RebuiltExecutionSummaries)
	writeRepairSection("Non-repairable findings", summary.NonRepairableFindings)

	if len(summary.CleanedAtomicTemps) == 0 &&
		strings.TrimSpace(summary.ConfigBackupPath) == "" &&
		!summary.StaleLockCleaned &&
		len(summary.WorktreeScopeRepairs) == 0 &&
		len(summary.MaterializedWavePlans) == 0 &&
		len(summary.RecoveredWaveRuns) == 0 &&
		len(summary.ClearedCheckpoints) == 0 &&
		len(summary.RepairedCheckpoints) == 0 &&
		len(summary.PrunedTaskEvidence) == 0 &&
		len(summary.RebuiltExecutionSummaries) == 0 &&
		len(summary.NonRepairableFindings) == 0 {
		writer.Writef("No repairs were needed\n")
	}

	return writer.Err()
}

func repairSummaryForHealthFinding(finding state.HealthFinding) string {
	slug := strings.TrimSpace(finding.Slug)
	switch finding.Category {
	case "bundle_integrity":
		for _, reason := range finding.Reasons {
			if reason.Code != "change_bundle_unreadable" {
				continue
			}
			if slug == "" {
				return fmt.Sprintf("change authority unreadable: %s", reason.Detail)
			}
			return fmt.Sprintf("%s: change authority unreadable: %s", slug, reason.Detail)
		}
	case "execution_summary":
		if finding.Repairable {
			return ""
		}
		for _, reason := range finding.Reasons {
			if reason.Code != "execution_summary_unreadable" {
				continue
			}
			if slug == "" {
				return fmt.Sprintf("execution summary unreadable: %s", reason.Detail)
			}
			return fmt.Sprintf("%s: execution summary unreadable: %s", slug, reason.Detail)
		}
	}
	return ""
}

func rebuildExecutionSummaries(root string, now time.Time) ([]string, []string, error) {
	allChanges, _, err := state.ListChangesBestEffortWithIssues(root)
	if err != nil {
		return nil, nil, err
	}

	rebuilt := []string{}
	findings := []string{}
	for _, change := range allChanges {
		if change.Status != model.ChangeStatusActive || !state.ExecutionSummaryRelevantState(change.CurrentState) {
			continue
		}

		record, found, err := progression.LatestPassingWaveEvidence(root, change.Slug)
		if err != nil {
			findings = append(findings, fmt.Sprintf("%s: execution summary recovery preflight failed: %v", change.Slug, err))
			continue
		}
		if !found || record.RunVersion < 1 {
			continue
		}

		summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		if err == nil && state.ExecutionSummaryReady(summary) {
			continue
		}

		if err != nil {
			backupPath, backupErr := backupUnreadableExecutionSummary(root, change, now)
			if backupErr != nil {
				findings = append(findings, fmt.Sprintf("%s: backup unreadable execution summary: %v", change.Slug, backupErr))
				continue
			}
			if strings.TrimSpace(backupPath) == "" {
				findings = append(findings, fmt.Sprintf("%s: execution summary unreadable and could not be backed up", change.Slug))
				continue
			}
		}

		if _, syncErr := progression.SyncGovernedWaveExecution(root, change); syncErr != nil {
			findings = append(findings, fmt.Sprintf("%s: rebuild execution summary from task evidence: %v", change.Slug, syncErr))
			continue
		}

		rebuiltSummary, loadErr := state.LoadOptionalRelevantExecutionSummary(root, change)
		if loadErr != nil || !state.ExecutionSummaryReady(rebuiltSummary) {
			if loadErr != nil {
				findings = append(findings, fmt.Sprintf("%s: rebuilt execution summary still unreadable: %v", change.Slug, loadErr))
			} else {
				findings = append(findings, fmt.Sprintf("%s: rebuilt execution summary is still incomplete", change.Slug))
			}
			continue
		}
		rebuilt = append(rebuilt, change.Slug)
	}

	slices.Sort(rebuilt)
	slices.Sort(findings)
	return slices.Compact(rebuilt), slices.Compact(findings), nil
}

func backupUnreadableExecutionSummary(root string, change model.Change, now time.Time) (string, error) {
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return "", err
	}
	summaryPath := filepath.Join(paths.GovernedBundleDir, "verification", state.ExecutionSummaryFileName)
	if _, err := os.Stat(summaryPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	backupPath := filepath.Join(
		filepath.Dir(summaryPath),
		fmt.Sprintf("execution-summary.broken.%s.yaml", now.UTC().Format("20060102T150405Z")),
	)
	if err := os.Rename(summaryPath, backupPath); err != nil {
		return "", err
	}
	return backupPath, nil
}
