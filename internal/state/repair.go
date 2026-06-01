package state

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"gopkg.in/yaml.v3"
)

func RepairCorruptConfig(root string, now time.Time) (string, error) {
	configPath := ConfigPath(root)
	raw, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return repairMissingConfig(configPath)
		}
		return "", err
	}
	if _, err := model.ParseConfigYAML(raw); err == nil {
		return "", nil
	}

	backupPath := filepath.Join(
		ConfigBackupDir(root),
		fmt.Sprintf("slipway.yaml.broken.%s.yaml", now.UTC().Format("20060102T150405Z")),
	)
	if err := fsutil.WriteFileAtomic(backupPath, raw, 0o644); err != nil {
		return "", err
	}
	if err := model.SaveConfig(configPath, model.DefaultConfig()); err != nil {
		return "", err
	}
	return backupPath, nil
}

func repairMissingConfig(configPath string) (string, error) {
	return "", model.SaveConfig(configPath, model.DefaultConfig())
}

// RepairArchivedTerminalStatus repairs archive residue left behind by an
// interrupted terminal archive rewrite. Archived bundles must be terminal,
// frozen, scrubbed of runtime-only refs, and detached from git-local runtime state.
func RepairArchivedTerminalStatus(root, slug string) (bool, error) {
	change, candidate, err := loadArchivedChangeWithCandidate(root, slug)
	if err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, err
	}

	repaired := false
	before := cloneChangeForRepairComparison(change)
	archiveDir := filepath.Dir(candidate.Path)

	if change.Status != model.ChangeStatusDone && change.Status != model.ChangeStatusCancelled {
		if change.CurrentState == model.StateDone {
			change.Status = model.ChangeStatusDone
		} else {
			change.Status = model.ChangeStatusCancelled
		}
	}

	change.Artifacts = FreezeArtifacts(change.Artifacts)
	scrubChangeRuntimeEvidenceRefs(&change)
	sanitizeArchivedChangeSnapshot(&change)
	change.Normalize()

	if !reflect.DeepEqual(before, change) {
		raw, err := yaml.Marshal(change)
		if err != nil {
			return false, err
		}
		if err := fsutil.WriteFileAtomic(filepath.Join(archiveDir, "change.yaml"), raw, 0o644); err != nil {
			return false, err
		}
		repaired = true
	}

	if err := scrubArchivedExecutionSummaryRuntimeEvidenceRefsAt(root, slug, archiveDir); err != nil {
		return false, err
	}

	cleaned, err := removeArchivedLocalRuntimeState(root, candidate.WorkspaceRoot, slug)
	if err != nil {
		return false, err
	}
	if cleaned {
		repaired = true
	}

	return repaired, nil
}

func cloneChangeForRepairComparison(change model.Change) model.Change {
	clone := change
	if change.Artifacts != nil {
		clone.Artifacts = make(map[string]model.ArtifactState, len(change.Artifacts))
		for key, artifact := range change.Artifacts {
			clone.Artifacts[key] = artifact
		}
	}
	if change.EvidenceRefs != nil {
		clone.EvidenceRefs = make(map[string]string, len(change.EvidenceRefs))
		for key, ref := range change.EvidenceRefs {
			clone.EvidenceRefs[key] = ref
		}
	}
	return clone
}

func removeArchivedLocalRuntimeState(root, workspaceRoot, slug string) (bool, error) {
	removed := false
	for _, runtimeRoot := range archiveRuntimeCleanupRoots(root, workspaceRoot) {
		if !hasPerChangeLocalRuntimeState(runtimeRoot, slug) {
			continue
		}
		if err := removePerChangeLocalRuntimeState(runtimeRoot, slug); err != nil {
			return false, err
		}
		removed = true
	}
	return removed, nil
}

func archiveRuntimeCleanupRoots(root, workspaceRoot string) []string {
	candidates := []string{root, workspaceRoot}
	seen := map[string]struct{}{}
	roots := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		normalized, err := NormalizePath(candidate)
		if err != nil {
			normalized = filepath.Clean(candidate)
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		roots = append(roots, normalized)
	}
	return roots
}

func ListArchivedChangeSlugs(root string) ([]string, error) {
	workspaceRoots, err := allWorkspaceRoots(root)
	if err != nil {
		return nil, err
	}
	unique := map[string]struct{}{}
	for _, workspaceRoot := range workspaceRoots {
		slugs, err := listSubdirs(ArchivedBundlesDir(workspaceRoot))
		if err != nil {
			return nil, err
		}
		for _, slug := range slugs {
			unique[slug] = struct{}{}
		}
	}
	slugs := make([]string, 0, len(unique))
	for slug := range unique {
		slugs = append(slugs, slug)
	}
	slices.Sort(slugs)
	return slugs, nil
}

func hasPerChangeLocalRuntimeState(root, slug string) bool {
	for _, path := range []string{
		ChangeDir(root, slug),
		filepath.Dir(TaskPIDFilePath(root, slug)),
		filepath.Dir(GovernanceSnapshotCachePath(root, slug)),
	} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// RepairBoundWorktreeScopeMetadata re-seeds missing scope metadata for active
// change bundles living in linked worktrees. The canonical root config remains
// authoritative; these files exist only so sibling workspace discovery can
// find the bound scope again.
func RepairBoundWorktreeScopeMetadata(root string) ([]string, error) {
	workspaceRoots, err := allWorkspaceRoots(root)
	if err != nil {
		return nil, err
	}
	if len(workspaceRoots) <= 1 {
		return nil, nil
	}

	repaired := make([]string, 0)
	for _, workspaceRoot := range workspaceRoots[1:] {
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
			change, err := loadChangeCandidate(BundleChangeFilePath(workspaceRoot, entry.Name()))
			if err != nil {
				continue
			}
			HydrateWorktreeBinding(root, workspaceRoot, &change)
			if strings.TrimSpace(change.WorktreePath) == "" {
				continue
			}
			boundWorkspace, err := scopeRootInWorkspace(root, change.WorktreePath)
			if err != nil {
				return nil, err
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

			restored := false
			if !scopeConfigExists(workspaceRoot) {
				if err := EnsureWorkspaceScopeConfig(root, change.WorktreePath); err != nil {
					return nil, err
				}
				restored = true
			}
			if !scopeMarkerExists(workspaceRoot) {
				if err := EnsureWorkspaceScopeMarker(root, change.WorktreePath); err != nil {
					return nil, err
				}
				restored = true
			}
			if restored {
				repaired = append(repaired, change.Slug)
			}
		}
	}
	slices.Sort(repaired)
	return slices.Compact(repaired), nil
}

func isNotExist(err error) bool {
	return err != nil && errors.Is(err, fs.ErrNotExist)
}

// BundleConsistencyResult reports three-file partial write diagnostics.
type BundleConsistencyResult struct {
	Slug        string   `json:"slug"`
	Errors      []string `json:"errors,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	Diagnostics []string `json:"diagnostics,omitempty"`
}

// DiagnoseBundleConsistency checks the governed bundle for file consistency:
// change.yaml (single authority), tasks.md, and assurance.md.
func DiagnoseBundleConsistency(root string, change model.Change) BundleConsistencyResult {
	result := BundleConsistencyResult{Slug: change.Slug}

	paths, err := ResolveChangePaths(root, change)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("resolve paths: %v", err))
		return result
	}

	bundleDir := paths.GovernedBundleDir
	if _, err := os.Stat(bundleDir); isNotExist(err) {
		// No governed bundle — nothing to diagnose.
		return result
	}

	changeYamlPath := filepath.Join(bundleDir, "change.yaml")
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	assurancePath := filepath.Join(bundleDir, "assurance.md")

	changeYamlExists := fileExists(changeYamlPath)
	tasksExists := fileExists(tasksPath)
	assuranceExists := fileExists(assurancePath)

	// change.yaml must exist if bundle exists.
	if !changeYamlExists {
		result.Errors = append(result.Errors,
			"change.yaml missing in governed bundle — authority file required; run repair to regenerate")
	} else if raw, err := os.ReadFile(changeYamlPath); err != nil {
		result.Errors = append(result.Errors,
			fmt.Sprintf("change.yaml unreadable in governed bundle: %v", err))
	} else {
		var probe model.Change
		if err := yaml.Unmarshal(raw, &probe); err != nil {
			result.Errors = append(result.Errors,
				fmt.Sprintf("change.yaml corrupt in governed bundle — YAML parse error: %v; run repair to regenerate", err))
		}
	}

	// tasks.md: required for execution.
	if !tasksExists && changeYamlExists {
		result.Errors = append(result.Errors,
			"tasks.md missing in governed bundle — execution target file required")
	}

	// assurance.md remains optional on the light effective preset.
	// Standard/strict keep the earlier "required later, hard-required in review/done" behavior.
	assuranceRequired := bundleConsistencyRequiresAssurance(root, change)
	if assuranceRequired && !assuranceExists && changeYamlExists {
		if change.CurrentState == model.StateS3Review || change.CurrentState == model.StateS4Verify || change.CurrentState == model.StateDone {
			result.Errors = append(result.Errors,
				"assurance.md missing in governed bundle — required for review/verify/done phase")
		} else {
			result.Warnings = append(result.Warnings,
				"assurance.md missing in governed bundle — will be required for review phase")
		}
	}

	return result
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func bundleConsistencyRequiresAssurance(root string, change model.Change) bool {
	cfgPath, err := ConfigPathForChange(root, change)
	if err != nil {
		return true
	}
	cfg, err := model.LoadConfig(cfgPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return true
		}
		cfg = model.DefaultConfig()
	}

	effective := change.ConfirmedWorkflowPreset()
	if change.WorkflowPresetConfirmationPending() && change.SuggestedWorkflowPreset.IsValid() {
		effective = change.SuggestedWorkflowPreset
	}
	if !effective.IsValid() {
		effective = model.WorkflowPresetStandard
	}
	if cfg.Governance.MinPreset.IsValid() && effective.Rank() < cfg.Governance.MinPreset.Rank() {
		effective = cfg.Governance.MinPreset
	}
	if strings.TrimSpace(change.GuardrailDomain) != "" && effective.Rank() < model.WorkflowPresetStandard.Rank() {
		effective = model.WorkflowPresetStandard
	}
	return effective != model.WorkflowPresetLight
}
