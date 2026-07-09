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
// Cleanup operations: CleanedAtomicTemps, StaleLockCleaned, CleanedLockAnchors.
// Restore-to-contract: ConfigBackupPath (backs up corrupt config before
// restoring to .slipway.yaml).
// NonRepairableFindings require operator intervention (e.g. dual-active anomaly).
type repairSummary struct {
	CleanedAtomicTemps        []string                                 `json:"cleaned_atomic_temps,omitempty"`
	ConfigBackupPath          string                                   `json:"config_backup_path,omitempty"`
	StaleLockCleaned          bool                                     `json:"stale_lock_cleaned"`
	CleanedLockAnchors        []string                                 `json:"cleaned_lock_anchors,omitempty"`
	WorktreeScopeRepairs      []string                                 `json:"worktree_scope_repairs,omitempty"`
	MaterializedWavePlans     []string                                 `json:"materialized_wave_plans,omitempty"`
	RecoveredWaveRuns         []string                                 `json:"recovered_wave_runs,omitempty"`
	PrunedTaskEvidence        []string                                 `json:"pruned_task_evidence,omitempty"`
	RebuiltExecutionSummaries []string                                 `json:"rebuilt_execution_summaries,omitempty"`
	RemovedEmptyOrphanBundles []string                                 `json:"removed_empty_orphan_bundles,omitempty"`
	NonRepairableFindings     []string                                 `json:"non_repairable_findings,omitempty"`
	AppliedRepairs            []repairAppliedFinding                   `json:"applied_repairs,omitempty"`
	UnrepairedDrift           []repairDriftFinding                     `json:"unrepaired_drift,omitempty"`
	PathAuthority             map[string]*state.ExecutionPathAuthority `json:"path_authority,omitempty"`
	Mode                      string                                   `json:"mode,omitempty"`
}

type repairAppliedFinding struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

type repairDriftFinding struct {
	Reason     string `json:"reason"`
	Target     string `json:"target"`
	NextAction string `json:"next_action"`
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

				repairCfg := loadRepairLockConfigAtRoot(root)
				tempStaleAfter := time.Duration(repairCfg.Execution.LockStaleAfterSeconds) * time.Second
				cleaned, err := fsutil.CleanupAtomicTempArtifactsOlderThan(root, tempStaleAfter, now)
				if err != nil {
					return err
				}
				summary.CleanedAtomicTemps = cleaned

				backupPath, configRepairErr := state.RepairCorruptConfig(root, now)
				applyConfigRepairResult(&summary, backupPath, configRepairErr)
				worktreeScopeRepairs, err := state.RepairBoundWorktreeScopeMetadata(root)
				if err != nil {
					return err
				}
				summary.WorktreeScopeRepairs = worktreeScopeRepairs
				removedEmptyOrphans, err := state.RepairEmptyOrphanBundleDirs(root)
				if err != nil {
					return err
				}
				summary.RemovedEmptyOrphanBundles = removedEmptyOrphans

				cfg, cfgErr := loadConfigAtRoot(root)
				if cfgErr != nil {
					// The config is still unreadable after the RepairCorruptConfig
					// attempt above. Hard-returning here would discard the whole
					// summary — including any config-repair finding just recorded —
					// and leave the operator with a bare config_parse_failure that
					// says to run `slipway repair` right after they ran it (REQ-002).
					// Surface the unreadable config as a non-repairable finding and
					// fall back to default lock settings (as loadRepairLockConfigAtRoot
					// already does) so the remaining repairs and the summary complete.
					summary.NonRepairableFindings = append(
						summary.NonRepairableFindings,
						fmt.Sprintf(
							"config unreadable after repair; correct or remove %s manually: %v",
							state.ConfigPath(root), cfgErr,
						),
					)
					cfg = model.DefaultConfig()
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
				summary.CleanedLockAnchors = cleanupUnheldLockAnchors(root, staleLockPaths)

				execRepair, err := state.RepairExecutionStateAt(root, now)
				if err != nil {
					return err
				}
				summary.MaterializedWavePlans = execRepair.MaterializedWavePlans
				summary.RecoveredWaveRuns = execRepair.RecoveredWaveRuns
				summary.PrunedTaskEvidence = execRepair.PrunedTaskEvidence
				summary.NonRepairableFindings = append(summary.NonRepairableFindings, execRepair.NonRepairableFindings...)

				rebuiltSummaries, rebuildFindings, err := rebuildExecutionSummaries(root, now)
				if err != nil {
					return err
				}
				summary.RebuiltExecutionSummaries = rebuiltSummaries
				summary.NonRepairableFindings = append(summary.NonRepairableFindings, rebuildFindings...)
				summary.NonRepairableFindings = dropRepairedExecutionSummaryFindings(summary.NonRepairableFindings, rebuiltSummaries)

				for _, slug := range archivedSlugs {
					if _, err := state.RepairArchivedTerminalStatus(root, slug); err != nil {
						if state.IsMissingBundleAuthority(err) {
							summary.NonRepairableFindings = append(
								summary.NonRepairableFindings,
								fmt.Sprintf("%s: archived change authority missing: %v", slug, err),
							)
							continue
						}
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
					case "bundle_integrity", "execution_summary", "runtime_hygiene":
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
						slugs := make([]string, 0, len(unique))
						for slug := range unique {
							slugs = append(slugs, slug)
						}
						slices.Sort(slugs)
						summary.NonRepairableFindings = append(
							summary.NonRepairableFindings,
							fmt.Sprintf("multiple active changes are active: %s", strings.Join(slugs, ", ")),
						)
					}
				}

				slices.Sort(summary.NonRepairableFindings)
				summary.NonRepairableFindings = slices.Compact(summary.NonRepairableFindings)
				summary.AppliedRepairs = buildAppliedRepairFindings(summary)
				freshnessDrift := buildFreshnessRepairDriftFindings(root, allChanges)
				digestDrift := buildGovernanceDigestDriftFindings(root, allChanges)
				summary.UnrepairedDrift = normalizeRepairDriftFindings(append(append(buildUnrepairedDriftFindings(summary.NonRepairableFindings), freshnessDrift...), digestDrift...))
				summary.PathAuthority = buildRepairPathAuthority(root, allChanges, summary)
				applyRepairInvocationWorkspacePath(cmd, root, &summary)
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
	cmd.Flags().StringVar(&focus, "focus", "", "Repair focus alias (repair exposes no public focuses; uses the default local-integrity route)")
	cmd.Flags().BoolVar(&listFocuses, "list-focuses", false, "List public --focus aliases for this command and exit")
	cmd.Flags().StringVar(&discoveryFormat, "format", "text", "Output format for --list-focuses: text|json")
	return cmd
}

// applyConfigRepairResult records the outcome of state.RepairCorruptConfig on
// the summary. A successful repair records the backup path (empty when the
// config was already valid or was recreated from missing). A non-nil error is a
// config-repair FAILURE that must surface as a non-repairable finding for the
// operator rather than being swallowed as success: the backup or rewrite did
// not complete, so the on-disk config is still broken.
func applyConfigRepairResult(summary *repairSummary, backupPath string, err error) {
	if err != nil {
		summary.NonRepairableFindings = append(
			summary.NonRepairableFindings,
			fmt.Sprintf("config repair failed: %v", err),
		)
		return
	}
	summary.ConfigBackupPath = backupPath
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
	writeRepairSection("Cleaned lock anchors", summary.CleanedLockAnchors)
	writeRepairSection("Worktree scope repairs", summary.WorktreeScopeRepairs)
	writeRepairSection("Materialized wave plans", summary.MaterializedWavePlans)
	writeRepairSection("Recovered wave runs", summary.RecoveredWaveRuns)
	writeRepairSection("Pruned task evidence", summary.PrunedTaskEvidence)
	writeRepairSection("Rebuilt execution summaries", summary.RebuiltExecutionSummaries)
	writeRepairSection("Removed empty orphan bundles", summary.RemovedEmptyOrphanBundles)
	writeRepairSection("Non-repairable findings", summary.NonRepairableFindings)
	writeRepairSection("Applied repairs", repairAppliedFindingStrings(summary.AppliedRepairs))
	writeRepairSection("Unrepaired drift", repairDriftFindingStrings(summary.UnrepairedDrift))

	if len(summary.CleanedAtomicTemps) == 0 &&
		strings.TrimSpace(summary.ConfigBackupPath) == "" &&
		!summary.StaleLockCleaned &&
		len(summary.CleanedLockAnchors) == 0 &&
		len(summary.WorktreeScopeRepairs) == 0 &&
		len(summary.MaterializedWavePlans) == 0 &&
		len(summary.RecoveredWaveRuns) == 0 &&
		len(summary.PrunedTaskEvidence) == 0 &&
		len(summary.RebuiltExecutionSummaries) == 0 &&
		len(summary.RemovedEmptyOrphanBundles) == 0 &&
		len(summary.NonRepairableFindings) == 0 &&
		len(summary.AppliedRepairs) == 0 &&
		len(summary.UnrepairedDrift) == 0 {
		writer.Writef("No repairs were needed\n")
	}

	return writer.Err()
}

func buildAppliedRepairFindings(summary repairSummary) []repairAppliedFinding {
	findings := []repairAppliedFinding{}
	appendItems := func(kind string, targets []string) {
		for _, target := range targets {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}
			findings = append(findings, repairAppliedFinding{Kind: kind, Target: target})
		}
	}
	appendItems("cleaned_atomic_temp", summary.CleanedAtomicTemps)
	appendItems("cleaned_lock_anchor", summary.CleanedLockAnchors)
	appendItems("worktree_scope_repair", summary.WorktreeScopeRepairs)
	appendItems("materialized_wave_plan", summary.MaterializedWavePlans)
	appendItems("recovered_wave_run", summary.RecoveredWaveRuns)
	appendItems("pruned_task_evidence", summary.PrunedTaskEvidence)
	appendItems("rebuilt_execution_summary", summary.RebuiltExecutionSummaries)
	appendItems("empty_orphan_bundle", summary.RemovedEmptyOrphanBundles)
	if summary.StaleLockCleaned {
		findings = append(findings, repairAppliedFinding{Kind: "stale_lock_cleaned", Target: "workspace"})
	}
	if strings.TrimSpace(summary.ConfigBackupPath) != "" {
		findings = append(findings, repairAppliedFinding{Kind: "config_backup", Target: summary.ConfigBackupPath})
	}
	slices.SortFunc(findings, func(a, b repairAppliedFinding) int {
		if a.Kind != b.Kind {
			return strings.Compare(a.Kind, b.Kind)
		}
		return strings.Compare(a.Target, b.Target)
	})
	return findings
}

func cleanupUnheldLockAnchors(root string, lockPaths []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(lockPaths))
	for _, lockPath := range lockPaths {
		lockPath = strings.TrimSpace(lockPath)
		if lockPath == "" {
			continue
		}
		if _, ok := seen[lockPath]; ok {
			continue
		}
		seen[lockPath] = struct{}{}
		unique = append(unique, lockPath)
	}
	slices.Sort(unique)

	cleanedPaths := []string{}
	for _, lockPath := range unique {
		// Best-effort hygiene: a single anchor's I/O error must not abort the
		// whole repair. This mirrors the tolerant CleanupStale loop above and the
		// helper's own best-effort contract, so skip a failing anchor and keep
		// cleaning the rest.
		cleaned, err := fsutil.NewStateLock(lockPath).CleanupUnheldAnchorWithoutMeta()
		if err != nil {
			continue
		}
		if cleaned {
			cleanedPaths = append(cleanedPaths, state.DisplayPath(root, lockPath))
		}
	}
	return cleanedPaths
}

func buildUnrepairedDriftFindings(findings []string) []repairDriftFinding {
	if len(findings) == 0 {
		return nil
	}
	out := make([]repairDriftFinding, 0, len(findings))
	for _, finding := range findings {
		finding = strings.TrimSpace(finding)
		if finding == "" {
			continue
		}
		target := "workspace"
		reason := finding
		if before, after, ok := strings.Cut(finding, ": "); ok {
			before = strings.TrimSpace(before)
			after = strings.TrimSpace(after)
			switch {
			case before == "bundle directory exists without change.yaml":
				target = filepath.ToSlash(filepath.Join("artifacts", "changes", after))
				reason = before
			case before != "" && !strings.Contains(before, " "):
				target = before
				reason = after
			case after != "":
				target = after
				reason = before
			}
		}
		out = append(out, repairDriftFinding{
			Reason:     reason,
			Target:     target,
			NextAction: repairDriftNextAction(reason, target),
		})
	}
	return out
}

func buildFreshnessRepairDriftFindings(root string, changes []model.Change) []repairDriftFinding {
	out := []repairDriftFinding{}
	for _, change := range changes {
		if change.Status != model.ChangeStatusActive || !state.ExecutionSummaryRelevantState(change.CurrentState) {
			continue
		}
		summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		if err != nil || !state.ExecutionSummaryReady(summary) {
			continue
		}
		diagnostics := state.ExecutionSummaryFreshnessDiagnostics(root, change, summary)
		if diagnostics.Status != "stale" {
			continue
		}
		if state.ExecutionFreshnessIsS3TaskPlanAmendment(change.CurrentState, diagnostics) {
			continue
		}
		target := ""
		reason := state.StaleExecutionEvidenceBlockerToken
		nextAction := strings.TrimSpace(diagnostics.NextAction)
		if diagnostics.FirstStaleCause != nil {
			if diagnostics.FirstStaleCause.EvidenceArtifact != "" {
				target = diagnostics.FirstStaleCause.EvidenceArtifact
			}
			if diagnostics.FirstStaleCause.SourceArtifact != "" {
				target = diagnostics.FirstStaleCause.SourceArtifact
			}
			if diagnostics.FirstStaleCause.Reason != "" {
				reason = diagnostics.FirstStaleCause.Reason
			}
			if nextAction == "" {
				nextAction = diagnostics.FirstStaleCause.NextAction
			}
		}
		if (target == "" || reason == state.StaleExecutionEvidenceBlockerToken) && len(diagnostics.TaskInputDiffs) > 0 {
			for _, diff := range diagnostics.TaskInputDiffs {
				diffTarget := target
				if diff.EvidencePath != "" {
					diffTarget = diff.EvidencePath
				}
				diffReason := fmt.Sprintf("%s: task=%s field=%s expected=%s current=%s",
					state.StaleExecutionEvidenceBlockerToken,
					diff.TaskID,
					diff.Field,
					diff.Expected,
					diff.Current,
				)
				diffNextAction := nextAction
				if diff.NextAction != "" {
					diffNextAction = diff.NextAction
				}
				if diffTarget == "" && diagnostics.PathAuthority != nil {
					diffTarget = diagnostics.PathAuthority.VerificationPath
				}
				if diffNextAction == "" {
					diffNextAction = "regenerate execution-summary.yaml from current wave-backed task evidence"
				}
				out = append(out, repairDriftFinding{
					Reason:     diffReason,
					Target:     diffTarget,
					NextAction: diffNextAction,
				})
			}
			continue
		}
		if target == "" && diagnostics.PathAuthority != nil {
			target = diagnostics.PathAuthority.VerificationPath
		}
		if nextAction == "" {
			nextAction = "regenerate execution-summary.yaml from current wave-backed task evidence"
		}
		out = append(out, repairDriftFinding{
			Reason:     reason,
			Target:     target,
			NextAction: nextAction,
		})
	}
	return normalizeRepairDriftFindings(out)
}

func normalizeRepairDriftFindings(findings []repairDriftFinding) []repairDriftFinding {
	if len(findings) == 0 {
		return nil
	}
	normalized := make([]repairDriftFinding, 0, len(findings))
	for _, finding := range findings {
		finding.Reason = strings.TrimSpace(finding.Reason)
		finding.Target = strings.TrimSpace(finding.Target)
		finding.NextAction = strings.TrimSpace(finding.NextAction)
		if finding.Reason == "" {
			continue
		}
		if finding.Target == "" {
			finding.Target = "workspace"
		}
		if finding.NextAction == "" {
			finding.NextAction = repairDriftNextAction(finding.Reason, finding.Target)
		}
		normalized = append(normalized, finding)
	}
	if len(normalized) == 0 {
		return nil
	}
	slices.SortFunc(normalized, func(a, b repairDriftFinding) int {
		if a.Target != b.Target {
			return strings.Compare(a.Target, b.Target)
		}
		if a.Reason != b.Reason {
			return strings.Compare(a.Reason, b.Reason)
		}
		return strings.Compare(a.NextAction, b.NextAction)
	})
	return slices.CompactFunc(normalized, func(a, b repairDriftFinding) bool {
		return a.Target == b.Target && a.Reason == b.Reason && a.NextAction == b.NextAction
	})
}

func buildRepairPathAuthority(root string, changes []model.Change, summary repairSummary) map[string]*state.ExecutionPathAuthority {
	if len(changes) == 0 {
		return nil
	}
	changesBySlug := map[string]model.Change{}
	for _, change := range changes {
		if strings.TrimSpace(change.Slug) == "" {
			continue
		}
		changesBySlug[change.Slug] = change
	}
	targetSlugs := map[string]struct{}{}
	addTarget := func(target string) {
		slug := repairTargetSlug(target)
		if slug == "" {
			return
		}
		if _, ok := changesBySlug[slug]; !ok {
			return
		}
		targetSlugs[slug] = struct{}{}
	}
	addTargets := func(targets []string) {
		for _, target := range targets {
			addTarget(target)
		}
	}

	addTargets(summary.MaterializedWavePlans)
	addTargets(summary.RecoveredWaveRuns)
	addTargets(summary.PrunedTaskEvidence)
	addTargets(summary.RebuiltExecutionSummaries)
	for _, finding := range summary.UnrepairedDrift {
		addTarget(finding.Target)
		addTarget(finding.Reason)
		for _, change := range changesBySlug {
			if repairFindingMentionsSlug(finding, change.Slug) {
				addTarget(change.Slug)
			}
		}
	}
	if len(targetSlugs) == 0 {
		return nil
	}

	out := map[string]*state.ExecutionPathAuthority{}
	for slug := range targetSlugs {
		change := changesBySlug[slug]
		out[slug] = state.ExecutionPathAuthorityDiagnostics(root, change, 0)
	}
	return out
}

func repairTargetSlug(target string) string {
	target = filepath.ToSlash(strings.TrimSpace(target))
	target = strings.TrimPrefix(target, "./")
	if target == "" {
		return ""
	}
	parts := strings.Split(target, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "artifacts" && parts[i+1] == "changes" {
			return strings.TrimSpace(parts[i+2])
		}
	}
	return strings.TrimSpace(parts[0])
}

func repairFindingMentionsSlug(finding repairDriftFinding, slug string) bool {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return false
	}
	haystack := filepath.ToSlash(finding.Target + "\n" + finding.Reason + "\n" + finding.NextAction)
	return strings.Contains(haystack, "/"+slug+"/") ||
		strings.Contains(haystack, "changes/"+slug) ||
		strings.Contains(haystack, slug+":") ||
		haystack == slug
}

func repairDriftNextAction(reason, target string) string {
	lower := strings.ToLower(reason)
	switch {
	case strings.Contains(lower, "evidence digest"), strings.Contains(lower, "required_skill_stale"):
		return governanceDigestEvidenceNextAction(target)
	case strings.Contains(lower, "unknown metadata key"), strings.Contains(lower, "wave_plan_load_failed"):
		// tasks.md is unparseable (an unsupported/unknown task metadata key, or a
		// wave-plan derivation failure), so the wave plan cannot be rebuilt and
		// advancing would just fail again: the operator must fix the governed
		// tasks.md first. This is checked before the "wave plan" case so a
		// rematerialization error that wraps the unknown-key cause still routes to
		// fixing tasks.md rather than to a rebuild that cannot succeed.
		return "edit tasks.md to fix or remove the unsupported metadata key, then re-run `slipway repair` / `slipway validate`"
	case strings.Contains(lower, "wave plan"):
		return "run `slipway repair` to rebuild wave-plan.yaml from current tasks.md, then run `slipway run` to refresh affected execution evidence"
	case strings.Contains(lower, "execution summary"):
		return "regenerate execution-summary.yaml from current wave-backed task evidence"
	case strings.Contains(lower, "change authority"), strings.Contains(lower, "change.yaml"):
		return "repair or replace the authoritative change.yaml before continuing"
	case strings.Contains(lower, "multiple active changes"):
		return "run `slipway status` to inspect, then resolve one with `slipway cancel --change <slug>` or `slipway done --change <slug>`"
	default:
		return "run `slipway run` to repair the current lifecycle evidence and continue alignment"
	}
}

// buildGovernanceDigestDriftFindings surfaces governance-skill evidence-digest
// drift (`required_skill_stale`) as a non-repairable finding. repair does not
// mutate engine-owned digests; the owning skill must be rerun and recertified.
func buildGovernanceDigestDriftFindings(root string, changes []model.Change) []repairDriftFinding {
	out := []repairDriftFinding{}
	for _, change := range changes {
		if change.Status != model.ChangeStatusActive {
			continue
		}
		readiness, err := progression.EvaluateGovernanceReadiness(root, change, progression.GovernanceReadinessOptions{})
		if err != nil {
			// repair is best-effort diagnostics; skip a change whose readiness
			// cannot be evaluated rather than failing the whole repair pass.
			continue
		}
		staleSkills := []string{}
		seen := map[string]bool{}
		for _, blocker := range readiness.Blockers {
			if strings.TrimSpace(blocker.Code) != "required_skill_stale" {
				continue
			}
			skillName := blockerSkillName(blocker)
			if skillName == "" || seen[skillName] {
				continue
			}
			seen[skillName] = true
			staleSkills = append(staleSkills, skillName)
		}
		slices.Sort(staleSkills)
		for _, skillName := range staleSkills {
			out = append(out, repairDriftFinding{
				Reason:     fmt.Sprintf("%s: evidence digest for governance skill %q is stale", change.Slug, skillName),
				Target:     skillName,
				NextAction: governanceDigestEvidenceNextAction(skillName),
			})
		}
	}
	return out
}

func governanceDigestEvidenceNextAction(skillName string) string {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		return "re-run the stale governance skill against current inputs, then record fresh evidence with `slipway evidence skill --skill <skill> --verdict pass`"
	}
	return fmt.Sprintf("re-run the %s governance skill against current inputs, then record fresh evidence with `slipway evidence skill --skill %s --verdict pass`", skillName, skillName)
}

func repairAppliedFindingStrings(findings []repairAppliedFinding) []string {
	if len(findings) == 0 {
		return nil
	}
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		out = append(out, finding.Kind+": "+finding.Target)
	}
	return out
}

func repairDriftFindingStrings(findings []repairDriftFinding) []string {
	if len(findings) == 0 {
		return nil
	}
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		target := finding.Target
		if target == "" {
			target = "workspace"
		}
		out = append(out, target+": "+finding.Reason+"; next_action="+finding.NextAction)
	}
	return out
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

func dropRepairedExecutionSummaryFindings(findings []string, rebuiltSlugs []string) []string {
	if len(findings) == 0 || len(rebuiltSlugs) == 0 {
		return findings
	}
	rebuilt := map[string]struct{}{}
	for _, slug := range rebuiltSlugs {
		slug = strings.TrimSpace(slug)
		if slug != "" {
			rebuilt[slug] = struct{}{}
		}
	}
	if len(rebuilt) == 0 {
		return findings
	}

	filtered := make([]string, 0, len(findings))
	for _, finding := range findings {
		slug, reason, ok := strings.Cut(strings.TrimSpace(finding), ": ")
		if !ok || !strings.Contains(reason, "execution summary unreadable") {
			filtered = append(filtered, finding)
			continue
		}
		if _, repaired := rebuilt[strings.TrimSpace(slug)]; repaired {
			continue
		}
		filtered = append(filtered, finding)
	}
	return filtered
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
			diagnostics := state.ExecutionSummaryFreshnessDiagnostics(root, change, summary)
			if diagnostics.Status != "stale" || executionFreshnessHasPlanningDrift(diagnostics) {
				continue
			}
		}
		_, parseIssues, taskEvidenceErr := progression.LoadExecutionTasksFromEvidence(root, change.Slug, record.RunVersion)
		if taskEvidenceErr != nil {
			findings = append(findings, fmt.Sprintf("%s: execution summary recovery preflight failed: %v", change.Slug, taskEvidenceErr))
			continue
		}
		if len(parseIssues) > 0 {
			findings = append(findings, fmt.Sprintf("%s: rebuild execution summary from task evidence: %s", change.Slug, strings.Join(model.ReasonSpecs(model.ReasonCodesFromSpecs(parseIssues)), ",")))
			continue
		}

		backupPath := ""
		if err != nil {
			var backupErr error
			backupPath, backupErr = backupUnreadableExecutionSummary(root, change, now)
			if backupErr != nil {
				findings = append(findings, fmt.Sprintf("%s: backup unreadable execution summary: %v", change.Slug, backupErr))
				continue
			}
			if strings.TrimSpace(backupPath) == "" {
				findings = append(findings, fmt.Sprintf("%s: execution summary unreadable and could not be backed up", change.Slug))
				continue
			}
		}

		syncResult, syncErr := progression.SyncGovernedWaveExecution(root, change)
		if syncErr != nil {
			restoreUnreadableExecutionSummaryBackup(root, change, backupPath)
			findings = append(findings, fmt.Sprintf("%s: rebuild execution summary from task evidence: %v", change.Slug, syncErr))
			continue
		}
		if len(syncResult.Blockers) > 0 {
			restoreUnreadableExecutionSummaryBackup(root, change, backupPath)
			findings = append(findings, fmt.Sprintf("%s: rebuild execution summary from task evidence: %s", change.Slug, strings.Join(model.ReasonSpecs(syncResult.Blockers), ",")))
			continue
		}

		rebuiltSummary, loadErr := state.LoadOptionalRelevantExecutionSummary(root, change)
		if loadErr != nil || !state.ExecutionSummaryReady(rebuiltSummary) {
			restoreUnreadableExecutionSummaryBackup(root, change, backupPath)
			if loadErr != nil {
				findings = append(findings, fmt.Sprintf("%s: rebuilt execution summary still unreadable: %v", change.Slug, loadErr))
			} else {
				findings = append(findings, fmt.Sprintf("%s: rebuilt execution summary is still incomplete", change.Slug))
			}
			continue
		}
		if diagnostics := state.ExecutionSummaryFreshnessDiagnostics(root, change, rebuiltSummary); diagnostics.Status == "stale" {
			findings = append(findings, fmt.Sprintf("%s: rebuilt execution summary is still stale: %s", change.Slug, executionFreshnessRepairReason(diagnostics)))
			continue
		}
		rebuilt = append(rebuilt, change.Slug)
	}

	slices.Sort(rebuilt)
	slices.Sort(findings)
	return slices.Compact(rebuilt), slices.Compact(findings), nil
}

func executionFreshnessHasPlanningDrift(diagnostics state.ExecutionFreshnessDiagnostics) bool {
	for _, pair := range diagnostics.StalePairs {
		if pair.Reason == state.StalePlanningEvidenceBlockerToken {
			return true
		}
	}
	return false
}

func executionFreshnessRepairReason(diagnostics state.ExecutionFreshnessDiagnostics) string {
	if diagnostics.FirstStaleCause != nil && strings.TrimSpace(diagnostics.FirstStaleCause.Reason) != "" {
		return diagnostics.FirstStaleCause.Reason
	}
	if len(diagnostics.TaskInputDiffs) > 0 {
		diff := diagnostics.TaskInputDiffs[0]
		return fmt.Sprintf("%s task=%s field=%s", state.StaleExecutionEvidenceBlockerToken, diff.TaskID, diff.Field)
	}
	return state.StaleExecutionEvidenceBlockerToken
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

func restoreUnreadableExecutionSummaryBackup(root string, change model.Change, backupPath string) {
	backupPath = strings.TrimSpace(backupPath)
	if backupPath == "" {
		return
	}
	paths, err := state.ResolveChangePaths(root, change)
	if err != nil {
		return
	}
	summaryPath := filepath.Join(paths.GovernedBundleDir, "verification", state.ExecutionSummaryFileName)
	_ = os.Remove(summaryPath)
	_ = os.Rename(backupPath, summaryPath)
}
