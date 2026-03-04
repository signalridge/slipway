package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/signalridge/speclane/internal/fsutil"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type repairSummary struct {
	CleanedAtomicTemps    []string `json:"cleaned_atomic_temps,omitempty"`
	ConfigBackupPath      string   `json:"config_backup_path,omitempty"`
	StaleLockCleaned      bool     `json:"stale_lock_cleaned"`
	EvidenceDeleted       []string `json:"evidence_deleted,omitempty"`
	ArchiveRepairs        []string `json:"archive_repairs,omitempty"`
	GovernedCreateRepairs []string `json:"governed_create_repairs,omitempty"`
	DualActiveNormalized  bool     `json:"dual_active_normalized"`
	NonRepairableFindings []string `json:"non_repairable_findings,omitempty"`
}

func newRepairCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Run safe local integrity and layout repairs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
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

				cfg, err := loadConfigAtRoot(root)
				if err != nil {
					return err
				}
				gcResult, err := state.RunEvidenceRetentionGC(root, cfg.Execution.EvidenceRetentionDays, now)
				if err != nil {
					return err
				}
				summary.EvidenceDeleted = gcResult.DeletedPaths

				governedRepairs, err := state.RepairOrphanedGovernedAdmissions(root)
				if err != nil {
					return err
				}
				summary.GovernedCreateRepairs = governedRepairs

				activeRecords, err := state.DiscoverActiveRecords(root)
				if err != nil {
					return err
				}
				if len(activeRecords) > 1 {
					unique := map[string]struct{}{}
					for _, record := range activeRecords {
						unique[record.RequestID] = struct{}{}
					}
					if len(unique) == 1 {
						requestID := activeRecords[0].RequestID
						if admission, err := state.LoadAdmission(root, requestID); err == nil &&
							admission.AdmissionStatus == model.AdmissionStatusActive {
							admission.AdmissionStatus = model.AdmissionStatusSealedHandoff
							admission.CurrentState = model.StateS1Analyze
							admission.SealedAt = &now
							if err := state.SaveAdmission(root, admission); err != nil {
								return err
							}
							summary.DualActiveNormalized = true
						}
					} else {
						summary.NonRepairableFindings = append(
							summary.NonRepairableFindings,
							"different-request multi-active ambiguity requires operator intervention",
						)
					}
				}

				requestIDs := collectRequestIDs(root)
				for _, requestID := range requestIDs {
					repaired, err := state.RepairInterruptedTerminalArchive(root, requestID)
					if err != nil {
						return err
					}
					if repaired {
						summary.ArchiveRepairs = append(summary.ArchiveRepairs, requestID)
					}
				}
				slices.Sort(summary.ArchiveRepairs)
				slices.Sort(summary.GovernedCreateRepairs)
				slices.Sort(summary.NonRepairableFindings)

				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(summary)
			})
		},
	}
	return cmd
}

func collectRequestIDs(root string) []string {
	seen := map[string]struct{}{}
	for _, dir := range []string{
		filepath.Join(root, ".spln", "runtime", "admissions"),
		filepath.Join(root, ".spln", "runtime", "changes"),
	} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if filepath.Ext(name) != ".yaml" {
				continue
			}
			seen[name[:len(name)-len(".yaml")]] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for requestID := range seen {
		out = append(out, requestID)
	}
	slices.Sort(out)
	return out
}

func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}
