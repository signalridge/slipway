package state

import (
	"errors"
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
	Severity   model.ReasonSeverity `json:"severity" yaml:"severity"`
	Category   string               `json:"category" yaml:"category"`
	Slug       string               `json:"slug,omitempty" yaml:"slug,omitempty"`
	Message    string               `json:"message" yaml:"message"`
	Repairable bool                 `json:"repairable" yaml:"repairable"`
	RepairHint string               `json:"repair_hint,omitempty" yaml:"repair_hint,omitempty"`
	Reasons    []model.ReasonCode   `json:"reasons,omitempty" yaml:"reasons,omitempty"`
}

type HealthReport struct {
	Findings []HealthFinding `json:"findings,omitempty" yaml:"findings,omitempty"`
}

func CollectHealthReport(root string) (HealthReport, error) {
	findings := []HealthFinding{}

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
	appendExecutionSummaryFinding := func(change model.Change) error {
		slug := strings.TrimSpace(change.Slug)
		if slug == "" {
			return nil
		}
		if _, seen := seenExecutionSummaryChecks[slug]; seen {
			return nil
		}
		seenExecutionSummaryChecks[slug] = struct{}{}
		finding, err := executionSummaryHealthFinding(root, change)
		if err != nil {
			return err
		}
		if finding != nil {
			findings = append(findings, *finding)
		}
		return nil
	}
	for _, change := range changes {
		if err := appendExecutionSummaryFinding(change); err != nil {
			return HealthReport{}, err
		}
	}
	for _, change := range hiddenScopeDiagnostics.Changes {
		if err := appendExecutionSummaryFinding(change); err != nil {
			return HealthReport{}, err
		}
	}

	activeCount := 0
	for _, change := range changes {
		if change.Status == model.ChangeStatusActive {
			activeCount++
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
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityError,
			Category:   "active_change_selection",
			Message:    "Multiple active changes are present",
			Repairable: false,
			RepairHint: "Run `slipway status` to inspect active changes, then archive, cancel, or switch to an explicit `--change` workflow.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("multiple_active_changes", "")},
		})
	}

	codebaseStats, err := collectCodebaseMapStats(root, nowUTC())
	if err != nil {
		return HealthReport{}, err
	}
	if codebaseStats.Freshness != "fresh" {
		findings = append(findings, HealthFinding{
			Severity:   model.ReasonSeverityWarning,
			Category:   "codebase_map",
			Message:    "Repo-scoped codebase map is missing, partial, or stale",
			Repairable: true,
			RepairHint: "Run `slipway codebase-map` to create or refresh the durable brownfield map.",
			Reasons:    []model.ReasonCode{model.NewReasonCode("codebase_map_freshness_"+codebaseStats.Freshness, strings.Join(codebaseStats.MissingDocs, ","))},
		})
	}

	slices.SortFunc(findings, func(a, b HealthFinding) int {
		if a.Category != b.Category {
			return strings.Compare(a.Category, b.Category)
		}
		return strings.Compare(a.Slug, b.Slug)
	})

	return HealthReport{Findings: findings}, nil
}

func executionSummaryHealthFinding(root string, change model.Change) (*HealthFinding, error) {
	if !ExecutionSummaryRelevantState(change.CurrentState) {
		return nil, nil
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
		}, nil
	}
	return nil, nil
}

func dedicatedWorktreeHealthReasons(root string, change model.Change) ([]model.ReasonCode, error) {
	switch change.CurrentState {
	case model.StateS1Plan, model.StateS2Execute, model.StateS3Review, model.StateS4Verify:
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

func nowUTC() (now time.Time) {
	return time.Now().UTC()
}
