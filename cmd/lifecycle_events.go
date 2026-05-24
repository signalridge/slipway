package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

func appendCLILifecycleEvent(root string, change model.Change, event state.LifecycleEvent) error {
	event.ActorKind = "cli"
	if event.EvidenceRefs == nil {
		event.EvidenceRefs = commandLifecycleEvidenceRefs(change)
	}
	_, err := state.AppendLifecycleEvent(root, change, event)
	return err
}

func commandLifecycleEvidenceRefs(change model.Change) map[string]string {
	if len(change.EvidenceRefs) == 0 {
		return nil
	}
	out := make(map[string]string, len(change.EvidenceRefs))
	for key, value := range change.EvidenceRefs {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func lifecyclePIDDiagnostics(interrupted, forceKilled []int) []string {
	diagnostics := []string{}
	if len(interrupted) > 0 {
		diagnostics = append(diagnostics, fmt.Sprintf("interrupt_pids=%v", interrupted))
	}
	if len(forceKilled) > 0 {
		diagnostics = append(diagnostics, fmt.Sprintf("force_killed_pids=%v", forceKilled))
	}
	return diagnostics
}

func lifecycleRepairDiagnostics(summary repairSummary) []string {
	diagnostics := []string{}
	appendItems := func(label string, items []string) {
		if len(items) == 0 {
			return
		}
		values := append([]string(nil), items...)
		slices.Sort(values)
		diagnostics = append(diagnostics, fmt.Sprintf("%s=%s", label, strings.Join(values, ",")))
	}
	appendItems("cleaned_atomic_temps", summary.CleanedAtomicTemps)
	appendItems("worktree_scope_repairs", summary.WorktreeScopeRepairs)
	appendItems("materialized_wave_plans", summary.MaterializedWavePlans)
	appendItems("recovered_wave_runs", summary.RecoveredWaveRuns)
	appendItems("migrated_legacy_sidecars", summary.MigratedLegacySidecars)
	appendItems("cleared_checkpoints", summary.ClearedCheckpoints)
	appendItems("repaired_checkpoints", summary.RepairedCheckpoints)
	appendItems("pruned_task_evidence", summary.PrunedTaskEvidence)
	appendItems("rebuilt_execution_summaries", summary.RebuiltExecutionSummaries)
	appendItems("non_repairable_findings", summary.NonRepairableFindings)
	if strings.TrimSpace(summary.ConfigBackupPath) != "" {
		diagnostics = append(diagnostics, "config_backup_path="+strings.TrimSpace(summary.ConfigBackupPath))
	}
	if summary.StaleLockCleaned {
		diagnostics = append(diagnostics, "stale_lock_cleaned=true")
	}
	slices.Sort(diagnostics)
	return diagnostics
}

func repairSummaryHasLifecycleActivity(summary repairSummary) bool {
	return len(lifecycleRepairDiagnostics(summary)) > 0
}
