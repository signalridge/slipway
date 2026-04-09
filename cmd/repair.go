package cmd

import (
	"fmt"
	"slices"
	"strings"
	"time"

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
	CleanedAtomicTemps    []string `json:"cleaned_atomic_temps,omitempty"`
	ConfigBackupPath      string   `json:"config_backup_path,omitempty"`
	StaleLockCleaned      bool     `json:"stale_lock_cleaned"`
	WorktreeScopeRepairs  []string `json:"worktree_scope_repairs,omitempty"`
	NonRepairableFindings []string `json:"non_repairable_findings,omitempty"`
}

func makeRepairCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Run safe local integrity and layout repairs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := repairRootFromWD()
			if err != nil {
				return err
			}
			return withWorkspaceRepairLock(root, func(staleLockCleaned bool) error {
				now := time.Now().UTC()
				summary := repairSummary{
					StaleLockCleaned: staleLockCleaned,
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

				return encodeJSONResponse(cmd, summary)
			})
		},
	}
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
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
