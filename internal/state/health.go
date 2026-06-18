package state

import (
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

// HealthFinding represents a single health diagnostic result.
// Repairable=true means the issue can be resolved by `slipway repair` (cleanup).
// Repairable=false means operator intervention is required (contract violation).
type HealthFinding struct {
	Severity             model.ReasonSeverity `json:"severity" yaml:"severity"`
	Category             string               `json:"category" yaml:"category"`
	Slug                 string               `json:"slug,omitempty" yaml:"slug,omitempty"`
	Message              string               `json:"message" yaml:"message"`
	Repairable           bool                 `json:"repairable" yaml:"repairable"`
	RepairHint           string               `json:"repair_hint,omitempty" yaml:"repair_hint,omitempty"`
	ActiveChangeBlocking bool                 `json:"active_change_blocking" yaml:"active_change_blocking"`
	ActiveChangeImpact   string               `json:"active_change_impact,omitempty" yaml:"active_change_impact,omitempty"`
	Reasons              []model.ReasonCode   `json:"reasons,omitempty" yaml:"reasons,omitempty"`
}

type HealthReport struct {
	Findings []HealthFinding `json:"findings,omitempty" yaml:"findings,omitempty"`
}

func CollectHealthReport(root string, activeSlugOpt ...string) (HealthReport, error) {
	findings := []HealthFinding{}
	activeSlug := ""
	if len(activeSlugOpt) > 0 {
		activeSlug = strings.TrimSpace(activeSlugOpt[0])
	}

	cfgPath := ConfigPath(root)
	if _, err := os.Stat(cfgPath); err == nil {
		if _, err := model.LoadConfig(cfgPath); err != nil {
			findings = append(findings, HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "config",
				Message:    "Config parsing failed",
				Repairable: true,
				RepairHint: "Run `slipway repair` to back up the broken config and restore deterministic defaults.",
				Reasons:    []model.ReasonCode{model.NewReasonCode("config_parse_failure", err.Error())},
			})
		}
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return HealthReport{}, err
	}

	orphanSlugs, err := OrphanBundleSlugs(root)
	if err != nil {
		return HealthReport{}, err
	}
	for _, slug := range orphanSlugs {
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityWarning,
			Category:   "bundle_integrity",
			Slug:       slug,
			Message:    "Bundle directory exists without change.yaml: " + slug,
			Repairable: false,
			RepairHint: "Inspect the orphan bundle directory and remove it manually if it contains no useful state.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("orphan_bundle_directory", slug)},
		})
	}

	changes, issues, err := ListChangesBestEffortWithIssues(root)
	if err != nil {
		return HealthReport{}, err
	}
	for _, issue := range issues {
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "bundle_integrity",
			Slug:       issue.Slug,
			Message:    "Change bundle authority is unreadable",
			Repairable: false,
			RepairHint: "Fix or replace the governed bundle change.yaml manually, then rerun health or repair.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("change_bundle_unreadable", issue.Err.Error())},
		})
	}
	hiddenScopeDiagnostics, err := hiddenBoundWorktreeDiagnostics(root)
	if err != nil {
		return HealthReport{}, err
	}
	findings = append(findings, hiddenScopeDiagnostics.Findings...)

	seenExecutionSummaryChecks := map[string]struct{}{}
	appendExecutionFindings := func(change model.Change) error {
		slug := strings.TrimSpace(change.Slug)
		if slug == "" {
			return nil
		}
		if _, seen := seenExecutionSummaryChecks[slug]; seen {
			return nil
		}
		seenExecutionSummaryChecks[slug] = struct{}{}
		finding := executionSummaryHealthFinding(root, change)
		if finding != nil {
			findings = append(findings, *finding)
		}
		executionFindings, err := executionContractHealthFindings(root, change)
		if err != nil {
			return err
		}
		findings = append(findings, executionFindings...)
		return nil
	}
	for _, change := range changes {
		if err := appendExecutionFindings(change); err != nil {
			return HealthReport{}, err
		}
	}
	for _, change := range hiddenScopeDiagnostics.Changes {
		if err := appendExecutionFindings(change); err != nil {
			return HealthReport{}, err
		}
	}

	activeCount := 0
	onlyActiveSlug := ""
	for _, change := range changes {
		if change.Status == model.ChangeStatusActive {
			activeCount++
			onlyActiveSlug = change.Slug
		}
		reasons, reasonErr := dedicatedWorktreeHealthReasons(root, change)
		if reasonErr != nil {
			return HealthReport{}, reasonErr
		}
		if len(reasons) > 0 {
			repairable, repairHint := worktreeScopeMetadataRepairability(reasons)
			message := "Dedicated worktree binding is invalid"
			if repairable {
				message = "Bound worktree scope is missing required workspace metadata"
			}
			findings = append(findings, HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "worktree",
				Slug:       change.Slug,
				Message:    message,
				Repairable: repairable,
				RepairHint: repairHint,
				Reasons:    reasons,
			})
		}
	}
	if activeCount > 1 {
		onlyActiveSlug = ""
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "active_change_selection",
			Message:    "Multiple active changes are present",
			Repairable: false,
			RepairHint: "Run `slipway status` to inspect active changes, then archive, cancel, or switch to an explicit `--change` workflow.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("multiple_active_changes", "")},
		})
	}
	if activeSlug == "" && activeCount == 1 {
		activeSlug = onlyActiveSlug
	}

	codebaseStats, err := collectCodebaseMapStats(root, nowUTC())
	if err != nil {
		return HealthReport{}, err
	}
	if codebaseStats.Freshness != "fresh" {
		findings = append(findings, HealthFinding{
			Severity:             model.ReasonSeverityWarning,
			Category:             "codebase_map",
			Message:              "Repo-scoped codebase map is missing, partial, or stale",
			Repairable:           true,
			RepairHint:           "Run `slipway codebase-map` to create or refresh the durable brownfield map.",
			ActiveChangeBlocking: false,
			ActiveChangeImpact:   "non_blocking_for_active_change",
			Reasons:              []model.ReasonCode{model.NewReasonCode("codebase_map_freshness_"+codebaseStats.Freshness, strings.Join(codebaseStats.MissingDocs, ","))},
		})
	}

	annotateActiveChangeImpact(findings, activeSlug)
	slices.SortFunc(findings, func(a, b HealthFinding) int {
		if a.Category != b.Category {
			return strings.Compare(a.Category, b.Category)
		}
		return strings.Compare(a.Slug, b.Slug)
	})

	return HealthReport{Findings: findings}, nil
}

func annotateActiveChangeImpact(findings []HealthFinding, activeSlug string) {
	activeSlug = strings.TrimSpace(activeSlug)
	for i := range findings {
		if strings.TrimSpace(findings[i].ActiveChangeImpact) != "" {
			continue
		}
		if activeSlug != "" && strings.TrimSpace(findings[i].Slug) != "" && findings[i].Slug != activeSlug {
			findings[i].ActiveChangeBlocking = false
			findings[i].ActiveChangeImpact = "non_blocking_for_active_change"
			continue
		}
		if findings[i].Severity == model.ReasonSeverityError {
			findings[i].ActiveChangeBlocking = true
			findings[i].ActiveChangeImpact = "blocking_for_active_change"
			continue
		}
		findings[i].ActiveChangeBlocking = false
		findings[i].ActiveChangeImpact = "non_blocking_for_active_change"
	}
}

func executionSummaryHealthFinding(root string, change model.Change) *HealthFinding {
	if !ExecutionSummaryRelevantState(change.CurrentState) {
		return nil
	}
	if _, err := LoadOptionalRelevantExecutionSummary(root, change); err != nil {
		return &HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "execution_summary",
			Slug:       change.Slug,
			Message:    "Execution summary authority is unreadable",
			Repairable: false,
			RepairHint: "Fix or replace verification/execution-summary.yaml manually, then rerun health or repair.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("execution_summary_unreadable", err.Error())},
		}
	}
	return nil
}

func executionContractHealthFindings(root string, change model.Change) ([]HealthFinding, error) {
	findings := []HealthFinding{}

	findings = append(findings, runtimeStateHealthFindings(change)...)

	if change.ActiveCheckpoint != nil && change.CurrentState != model.StateS2Implement {
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "execution_checkpoint",
			Slug:       change.Slug,
			Message:    "Active checkpoint exists outside S2_IMPLEMENT",
			Repairable: true,
			RepairHint: "Run `slipway repair` to clear the stale checkpoint and rewrite execution state.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("stale_checkpoint_state", string(change.CurrentState))},
		})
	}
	if change.ActiveCheckpoint != nil && change.CurrentState == model.StateS2Implement {
		if staleAfter := checkpointStaleAfter(root); staleAfter > 0 &&
			!change.ActiveCheckpoint.PausedAt.IsZero() &&
			nowUTC().Sub(change.ActiveCheckpoint.PausedAt) > staleAfter {
			findings = append(findings, HealthFinding{
				Severity:   model.ReasonSeverityWarning,
				Category:   "execution_checkpoint",
				Slug:       change.Slug,
				Message:    "Active checkpoint has exceeded the stale threshold",
				Repairable: true,
				RepairHint: "Run `slipway repair` to clear the stale checkpoint before resuming execution.",
				Reasons:    []model.ReasonCode{model.NewReasonCode("checkpoint_stale", change.ActiveCheckpoint.PausedAt.UTC().Format(time.RFC3339))},
			})
		}
	}

	if !relevantWaveExecutionState(change.CurrentState) {
		return findings, nil
	}

	summary, summaryErr := LoadOptionalRelevantExecutionSummary(root, change)
	var plan *model.WavePlan
	if change.CurrentState == model.StateS2Implement {
		derived, _, err := MaterializeWavePlanTransactionOpAt(root, change, nowUTC())
		if err != nil {
			findings = append(findings, HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "wave_execution",
				Slug:       change.Slug,
				Message:    "Current tasks.md cannot be converted into a wave plan",
				Repairable: false,
				RepairHint: "Update tasks.md so task IDs, dependencies, and target files form a schedulable plan.",
				Reasons:    []model.ReasonCode{model.NewReasonCode("wave_plan_load_failed", err.Error())},
			})
			return findings, nil
		}
		plan = &derived
	} else {
		loadedPlan, err := LoadOptionalWavePlanForChange(root, change)
		if err != nil {
			findings = append(findings, HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "wave_execution",
				Slug:       change.Slug,
				Message:    "Derived wave plan is unreadable",
				Repairable: true,
				RepairHint: wavePlanRepairHint(),
				Reasons:    []model.ReasonCode{model.NewReasonCode("wave_plan_unreadable", err.Error())},
			})
			return findings, nil
		}
		if loadedPlan == nil {
			findings = append(findings, HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "wave_execution",
				Slug:       change.Slug,
				Message:    "Derived wave plan is missing",
				Repairable: true,
				RepairHint: wavePlanRepairHint(),
				Reasons:    []model.ReasonCode{model.NewReasonCode("wave_plan_missing", change.Slug)},
			})
			return findings, nil
		}
		plan = loadedPlan

		plan.Normalize()
		planHash := strings.TrimSpace(plan.EffectiveStructuralHash)
		if planHash == "" {
			planHash = strings.TrimSpace(plan.TasksPlanStructuralHash)
		}
		if planHash == "" {
			planHash = strings.TrimSpace(plan.TasksPlanHash)
		}
		if currentHash, err := CurrentTasksPlanStructuralState(root, change); err == nil &&
			planHash != "" &&
			currentHash != planHash {
			findings = append(findings, HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "wave_execution",
				Slug:       change.Slug,
				Message:    "Derived wave plan is stale against tasks.md",
				Repairable: true,
				RepairHint: wavePlanRepairHint(),
				Reasons:    []model.ReasonCode{model.NewReasonCode("wave_plan_drift", currentHash)},
			})
			return findings, nil
		}
	}

	if change.ActiveCheckpoint != nil {
		expectedWaveIndex := plan.WaveIndexForTask(change.ActiveCheckpoint.PausedTaskID)
		switch {
		case expectedWaveIndex == 0:
			findings = append(findings, HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "execution_checkpoint",
				Slug:       change.Slug,
				Message:    "Checkpoint task is not present in the current wave plan",
				Repairable: true,
				RepairHint: "Run `slipway repair` to clear the stale checkpoint before resuming execution.",
				Reasons:    []model.ReasonCode{model.NewReasonCode("checkpoint_task_missing_from_wave_plan", change.ActiveCheckpoint.PausedTaskID)},
			})
		case change.ActiveCheckpoint.PausedWaveIndex != expectedWaveIndex:
			findings = append(findings, HealthFinding{
				Severity:   model.ReasonSeverityWarning,
				Category:   "execution_checkpoint",
				Slug:       change.Slug,
				Message:    "Checkpoint wave index does not match the current wave plan",
				Repairable: true,
				RepairHint: "Run `slipway repair` to rewrite the checkpoint wave index.",
				Reasons:    []model.ReasonCode{model.NewReasonCode("checkpoint_wave_index_drift", fmt.Sprintf("%d", expectedWaveIndex))},
			})
		}
	}

	if summaryErr != nil || !ExecutionSummaryReady(summary) {
		return findings, nil
	}
	for _, blocker := range summary.OpenBlockers {
		if blocker.Code != "session_isolation_warning" {
			continue
		}
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityWarning,
			Category:   "execution_session",
			Slug:       change.Slug,
			Message:    "Session isolation warning detected in task evidence",
			Repairable: false,
			RepairHint: "Re-run wave orchestration so each task writes isolated session-backed evidence.",
			Reasons:    []model.ReasonCode{blocker},
		})
	}

	runs, err := LoadOptionalWaveRuns(root, change.Slug, summary.RunSummaryVersion)
	if err != nil {
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "wave_execution",
			Slug:       change.Slug,
			Message:    "Wave run evidence is unreadable",
			Repairable: true,
			RepairHint: "Run `slipway repair` to reconstruct wave runs from execution evidence or the execution summary.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("wave_runs_unreadable", err.Error())},
		})
		return findings, nil
	}
	if len(runs) == 0 {
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "wave_execution",
			Slug:       change.Slug,
			Message:    "Wave runs are missing for the latest execution summary",
			Repairable: true,
			RepairHint: "Run `slipway repair` to reconstruct wave runs before resuming or reviewing execution.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("wave_runs_missing", fmt.Sprintf("run_summary_version=%d", summary.RunSummaryVersion))},
		})
	} else if len(runs) < len(plan.Waves) {
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "wave_execution",
			Slug:       change.Slug,
			Message:    "Wave run evidence is incomplete for the current wave plan",
			Repairable: true,
			RepairHint: "Run `slipway repair` to reconstruct missing wave runs from the execution summary.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("wave_runs_incomplete", fmt.Sprintf("%d/%d", len(runs), len(plan.Waves)))},
		})
	}
	if linkageIssues := WaveTaskLinkageIssues(*plan, runs); len(linkageIssues) > 0 {
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "wave_execution",
			Slug:       change.Slug,
			Message:    "Wave run task linkage does not match wave-plan.yaml",
			Repairable: true,
			RepairHint: "Run `slipway repair` to reconstruct wave runs from the execution summary.",
			Reasons: []model.ReasonCode{
				model.NewReasonCode("wave_task_linkage_mismatch", strings.Join(linkageIssues, "; ")),
			},
		})
	}

	orphaned, taskEvidenceIssues, err := orphanTaskEvidence(root, change.Slug, summary.RunSummaryVersion, PlannedTaskIDSet(*plan))
	if err != nil {
		return nil, err
	}
	if len(taskEvidenceIssues) > 0 {
		reasons := make([]model.ReasonCode, 0, len(taskEvidenceIssues))
		for _, issue := range taskEvidenceIssues {
			reasons = append(reasons, model.NewReasonCode("task_evidence_unreadable", issue.message(root)))
		}
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "execution_evidence",
			Slug:       change.Slug,
			Message:    "Task evidence is unreadable",
			Repairable: false,
			RepairHint: "Regenerate execution evidence for the affected tasks before relying on health or repair diagnostics.",
			Reasons:    model.NormalizeReasonCodes(reasons),
		})
	}
	if len(orphaned) > 0 {
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityWarning,
			Category:   "execution_evidence",
			Slug:       change.Slug,
			Message:    "Orphan task evidence exists outside the current wave plan",
			Repairable: true,
			RepairHint: "Run `slipway repair` to prune orphan task evidence files.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("orphan_task_evidence", strings.Join(orphaned, ","))},
		})
	}

	return findings, nil
}

func runtimeStateHealthFindings(change model.Change) []HealthFinding {
	findings := []HealthFinding{}

	// Check for interrupted execution from the change itself.
	if !change.InterruptedExecutionAt.IsZero() &&
		change.Status == model.ChangeStatusActive &&
		change.CurrentState == model.StateS2Implement {
		interruptedAt := change.InterruptedExecutionAt.UTC().Format(time.RFC3339)
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityWarning,
			Category:   "execution_session",
			Slug:       change.Slug,
			Message:    "Governed execution was interrupted at " + interruptedAt,
			Repairable: false,
			RepairHint: "Run `slipway status` to inspect the interrupted execution context, then resume with `slipway run --resume` when ready.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("execution_interrupted", interruptedAt)},
		})
	}
	return findings
}

func checkpointStaleAfter(root string) time.Duration {
	cfg, err := model.LoadConfig(ConfigPath(root))
	if err != nil || cfg.Execution.LockStaleAfterSeconds <= 0 {
		return 0
	}
	return time.Duration(cfg.Execution.LockStaleAfterSeconds) * time.Second
}

func dedicatedWorktreeHealthReasons(root string, change model.Change) ([]model.ReasonCode, error) {
	switch change.CurrentState {
	case model.StateS1Plan, model.StateS2Implement, model.StateS3Review:
	default:
		return nil, nil
	}
	validation, err := ValidateChangeWorktree(root, change)
	if err != nil {
		return nil, err
	}
	reasons := append([]model.ReasonCode(nil), validation.Blockers...)
	scopeReasons, scopeErr := boundWorktreeScopeMetadataReasons(root, change)
	if scopeErr != nil {
		return nil, scopeErr
	}
	return append(reasons, scopeReasons...), nil
}

func worktreeScopeMetadataRepairability(reasons []model.ReasonCode) (bool, string) {
	if len(reasons) == 0 {
		return false, ""
	}
	for _, reason := range reasons {
		switch reason.Code {
		case "workspace_scope_config_missing", "workspace_scope_marker_missing":
			continue
		default:
			return false, "Rebind the change to a valid dedicated worktree, or archive/cancel the change explicitly before deleting the worktree."
		}
	}
	return true, "Run `slipway repair` to restore the bound worktree scope config and marker."
}

func boundWorktreeScopeMetadataReasons(root string, change model.Change) ([]model.ReasonCode, error) {
	worktreePath := strings.TrimSpace(change.WorktreePath)
	if worktreePath == "" {
		return nil, nil
	}
	workspaceRoot, err := scopeRootInWorkspace(root, worktreePath)
	if err != nil {
		return nil, err
	}
	reasons := []model.ReasonCode{}
	if !scopeConfigExists(workspaceRoot) {
		reasons = append(reasons, model.NewReasonCode("workspace_scope_config_missing", workspaceRoot))
	}
	if !scopeMarkerExists(workspaceRoot) {
		reasons = append(reasons, model.NewReasonCode("workspace_scope_marker_missing", workspaceRoot))
	}
	return reasons, nil
}

type hiddenBoundWorktreeScan struct {
	Findings []HealthFinding
	Changes  []model.Change
}

func hiddenBoundWorktreeDiagnostics(root string) (hiddenBoundWorktreeScan, error) {
	workspaceRoots, err := allWorkspaceRoots(root)
	if err != nil {
		return hiddenBoundWorktreeScan{}, err
	}
	if len(workspaceRoots) <= 1 {
		return hiddenBoundWorktreeScan{}, nil
	}

	diagnostics := hiddenBoundWorktreeScan{}
	for _, workspaceRoot := range workspaceRoots[1:] {
		if workspaceScopeVisible(workspaceRoot) {
			continue
		}

		entries, err := os.ReadDir(ActiveBundlesDir(workspaceRoot))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return hiddenBoundWorktreeScan{}, err
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == "archived" {
				continue
			}
			change, err := loadChangeCandidate(BundleChangeFilePath(workspaceRoot, entry.Name()))
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				diagnostics.Findings = append(diagnostics.Findings, HealthFinding{
					Severity:   model.ReasonSeverityError,
					Category:   "bundle_integrity",
					Slug:       entry.Name(),
					Message:    "Change bundle authority is unreadable",
					Repairable: false,
					RepairHint: "Fix or replace the governed bundle change.yaml manually, then rerun health or repair.",
					Reasons:    []model.ReasonCode{model.NewReasonCode("change_bundle_unreadable", err.Error())},
				})
				continue
			}
			HydrateWorktreeBinding(root, workspaceRoot, &change)
			boundWorkspace, err := scopeRootInWorkspace(root, change.WorktreePath)
			if err != nil {
				return hiddenBoundWorktreeScan{}, err
			}
			normalizedBoundWorkspace, err := NormalizePath(boundWorkspace)
			if err != nil {
				normalizedBoundWorkspace = filepath.Clean(boundWorkspace)
			}
			normalizedWorkspaceRoot, err := NormalizePath(workspaceRoot)
			if err != nil {
				normalizedWorkspaceRoot = filepath.Clean(workspaceRoot)
			}
			if normalizedBoundWorkspace != normalizedWorkspaceRoot {
				continue
			}
			diagnostics.Changes = append(diagnostics.Changes, change)
			reasons, err := boundWorktreeScopeMetadataReasons(root, change)
			if err != nil {
				return hiddenBoundWorktreeScan{}, err
			}
			if len(reasons) == 0 {
				continue
			}
			repairable, repairHint := worktreeScopeMetadataRepairability(reasons)
			diagnostics.Findings = append(diagnostics.Findings, HealthFinding{
				Severity:   model.ReasonSeverityError,
				Category:   "worktree",
				Slug:       change.Slug,
				Message:    "Bound worktree scope is missing required workspace metadata",
				Repairable: repairable,
				RepairHint: repairHint,
				Reasons:    reasons,
			})
		}
	}
	return diagnostics, nil
}

// OrphanBundleSlugs returns change slugs whose governed bundle directory exists
// but lacks the authoritative change.yaml file.
func OrphanBundleSlugs(root string) ([]string, error) {
	// Authority integrity scans must inspect every registered worktree scope,
	// not only currently visible ones. Hidden sibling worktrees can still hold
	// orphan governed bundles when local scope metadata is missing.
	workspaceRoots, err := allWorkspaceRoots(root)
	if err != nil {
		return nil, err
	}
	var orphans []string
	for _, workspaceRoot := range workspaceRoots {
		entries, err := os.ReadDir(ActiveBundlesDir(workspaceRoot))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == "archived" {
				continue
			}
			changeYaml := filepath.Join(ActiveBundlesDir(workspaceRoot), entry.Name(), "change.yaml")
			if _, err := os.Stat(changeYaml); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					hasFiles, emptyErr := orphanBundleDirHasFiles(filepath.Dir(changeYaml))
					if emptyErr != nil {
						return nil, emptyErr
					}
					if !hasFiles {
						continue
					}
					orphans = append(orphans, entry.Name())
					continue
				}
				return nil, err
			}
		}
	}
	slices.Sort(orphans)
	return slices.Compact(orphans), nil
}

// StaleRuntimeBindingSlugs returns change slugs whose git-local runtime binding
// remains after the active governed bundle directory has been removed entirely.
func StaleRuntimeBindingSlugs(root string) ([]string, error) {
	entries, err := os.ReadDir(ChangesDir(root))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var stale []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		if err := ValidateChangeSlug(slug); err != nil {
			continue
		}
		if !fileExists(WorktreeBindingPath(root, slug)) {
			continue
		}
		hasBundle, err := activeBundleDirExists(root, slug)
		if err != nil {
			return nil, err
		}
		if hasBundle {
			continue
		}
		stale = append(stale, slug)
	}
	slices.Sort(stale)
	return slices.Compact(stale), nil
}

func activeBundleDirExists(root, slug string) (bool, error) {
	workspaceRoots, err := allWorkspaceRoots(root)
	if err != nil {
		return false, err
	}
	for _, workspaceRoot := range workspaceRoots {
		info, err := os.Stat(filepath.Join(ActiveBundlesDir(workspaceRoot), slug))
		if err == nil {
			return info.IsDir(), nil
		}
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return false, err
	}
	return false, nil
}

func nowUTC() (now time.Time) {
	return time.Now().UTC()
}
