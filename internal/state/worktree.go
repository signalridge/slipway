package state

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/stringutil"
)

const (
	WorktreeReasonMetadataRequired  = "dedicated_worktree_metadata_required"
	WorktreeReasonDedicatedRequired = "dedicated_worktree_required"
	WorktreeReasonPathInvalid       = "dedicated_worktree_path_invalid"
	WorktreeReasonBranchMismatch    = "dedicated_worktree_branch_mismatch"
)

const (
	worktreeRefPathPrefix     = "worktree_path:"
	worktreeRefBranchPrefix   = "worktree_branch:"
	worktreeRefBaselinePrefix = "baseline_verify_cmd:"
)

type worktreeListProbe struct {
	Path       string
	Exists     bool
	ModTime    time.Time
	EntryNames []string
}

type worktreeListCacheEntry struct {
	Probe     worktreeListProbe
	Worktrees map[string]struct{}
}

var worktreeListCache = struct {
	mu      sync.Mutex
	entries map[string]worktreeListCacheEntry
}{
	entries: map[string]worktreeListCacheEntry{},
}

type worktreeBindingResolution int

const (
	worktreeBindingUnresolved worktreeBindingResolution = iota
	worktreeBindingFromRuntime
	worktreeBindingFromLocation
)

// HydrateWorktreeBinding resolves change.WorktreePath for an active governed
// bundle that was loaded from a tracked change.yaml.
//
// The absolute worktree path is never persisted to tracked change.yaml; the
// WorktreePath field is yaml:"-", so a tracked bundle that still carries
// worktree_path is rejected by strict decoding before it ever reaches here.
// Resolution authority, in order:
//
//  1. The git-local worktree-binding record (writeWorktreeBinding), which keeps
//     a change's binding unambiguous even when a stale copy of the bundle exists
//     in another workspace.
//  2. Fallback: the bundle's own location. SaveChange always writes the bundle
//     under changeWorkspaceRoot(root, change), which equals the bound worktree,
//     so a bundle's location is a faithful, machine-local encoding of the
//     binding. This makes resolution self-healing when the runtime record is
//     missing (e.g. a fresh clone, or git-local state cleared).
//
// Callers must skip archived bundles, which are portable, terminal, and
// intentionally unbound.
func HydrateWorktreeBinding(projectRoot, workspaceRoot string, change *model.Change) worktreeBindingResolution {
	if change == nil {
		return worktreeBindingUnresolved
	}
	if binding, ok := readWorktreeBinding(projectRoot, change.Slug); ok {
		change.WorktreePath = binding.WorktreePath
		return worktreeBindingFromRuntime
	}
	if inferWorktreeBindingFromLocation(projectRoot, workspaceRoot, change) {
		return worktreeBindingFromLocation
	}
	return worktreeBindingUnresolved
}

// inferWorktreeBindingFromLocation reconstructs the bound worktree from the
// workspace root where the bundle physically lives. A bundle in the project's
// own worktree (or a non-git context) is treated as unbound.
func inferWorktreeBindingFromLocation(projectRoot, workspaceRoot string, change *model.Change) bool {
	change.WorktreePath = ""

	worktreeRoot, err := gitWorkspaceRoot(workspaceRoot)
	if err != nil {
		return false
	}
	projectWorktreeRoot, err := gitWorkspaceRoot(projectRoot)
	if err != nil {
		return false
	}
	normalizedWorktree, err := NormalizePath(worktreeRoot)
	if err != nil {
		return false
	}
	normalizedProject, err := NormalizePath(projectWorktreeRoot)
	if err != nil {
		return false
	}
	if normalizedWorktree == normalizedProject {
		// Bundle lives in the project worktree itself, so it is unbound.
		return false
	}
	change.WorktreePath = normalizedWorktree
	return true
}

func PersistScopeWorktreeMetadata(change *model.Change, worktreePath, worktreeBranch string) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	if strings.TrimSpace(worktreePath) == "" {
		return fmt.Errorf("worktree_path is required")
	}
	if strings.TrimSpace(worktreeBranch) == "" {
		return fmt.Errorf("worktree_branch is required")
	}
	normalized, err := NormalizePath(worktreePath)
	if err != nil {
		return fmt.Errorf("normalize worktree path: %w", err)
	}
	change.WorktreePath = normalized
	change.WorktreeBranch = worktreeBranch
	return nil
}

type DefaultWorktreeBinding struct {
	Path          string
	Branch        string
	Created       bool
	SkippedReason string
}

func DefaultWorktreePath(root, slug string) string {
	return filepath.Join(root, ".worktrees", slug)
}

func DefaultWorktreeBranch(slug string) string {
	return "feat/" + slug
}

func EnsureDefaultWorktreeForChange(root string, change *model.Change) (DefaultWorktreeBinding, error) {
	if change == nil {
		return DefaultWorktreeBinding{}, fmt.Errorf("change is required")
	}
	if strings.TrimSpace(change.WorktreePath) != "" {
		return DefaultWorktreeBinding{
			Path:   change.WorktreePath,
			Branch: change.WorktreeBranch,
		}, nil
	}
	// Every governed change gets a dedicated worktree by default so the main
	// checkout stays free for parallel work; `governance.auto_provision_worktree:
	// false` opts out. Discovery is no longer the gate — non-discovery changes
	// previously ran their entire lifecycle in the main checkout.
	autoProvision := true
	if cfg, cfgErr := model.LoadConfig(ConfigPath(root)); cfgErr != nil {
		if !os.IsNotExist(cfgErr) {
			return DefaultWorktreeBinding{}, cfgErr
		}
	} else {
		autoProvision = cfg.Governance.AutoProvisionWorktreeEnabled()
	}
	if !autoProvision {
		return DefaultWorktreeBinding{SkippedReason: "worktree_provisioning_disabled"}, nil
	}

	repoRoot, err := gitWorkspaceRoot(root)
	if err != nil {
		if gitCommandReportsNotRepository(err) {
			return DefaultWorktreeBinding{SkippedReason: "not_git_repository"}, nil
		}
		return DefaultWorktreeBinding{}, err
	}
	if !gitHasHead(repoRoot) {
		return DefaultWorktreeBinding{SkippedReason: "git_head_missing"}, nil
	}

	path := DefaultWorktreePath(repoRoot, change.Slug)
	branch := DefaultWorktreeBranch(change.Slug)
	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return DefaultWorktreeBinding{}, fmt.Errorf("normalize default worktree path: %w", err)
	}

	registered, err := listGitWorktrees(repoRoot)
	if err != nil {
		return DefaultWorktreeBinding{}, err
	}
	if _, ok := registered[normalizedPath]; ok {
		if err := PersistScopeWorktreeMetadata(change, normalizedPath, branch); err != nil {
			return DefaultWorktreeBinding{}, err
		}
		validation, err := ValidateChangeWorktree(repoRoot, *change)
		if err != nil {
			return DefaultWorktreeBinding{}, err
		}
		if len(validation.Blockers) > 0 {
			return DefaultWorktreeBinding{}, fmt.Errorf("default worktree validation failed: %s", strings.Join(model.ReasonSpecs(validation.Blockers), ", "))
		}
		return DefaultWorktreeBinding{Path: normalizedPath, Branch: branch}, nil
	}

	if entries, readErr := os.ReadDir(normalizedPath); readErr == nil && len(entries) > 0 {
		return DefaultWorktreeBinding{}, fmt.Errorf("default worktree path exists and is not empty: %s", normalizedPath)
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return DefaultWorktreeBinding{}, readErr
	}
	if err := os.MkdirAll(filepath.Dir(normalizedPath), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return DefaultWorktreeBinding{}, err
	}

	args := []string{"-C", repoRoot, "worktree", "add"}
	if gitBranchExists(repoRoot, branch) {
		args = append(args, normalizedPath, branch)
	} else {
		baseRef := strings.TrimSpace(change.BaseRef)
		if baseRef == "" {
			baseRef = "HEAD"
		}
		args = append(args, "-b", branch, normalizedPath, baseRef)
	}
	cmd := exec.Command("git", args...) // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return DefaultWorktreeBinding{}, fmt.Errorf("git worktree add failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	invalidateWorktreeListCache(repoRoot)

	if err := PersistScopeWorktreeMetadata(change, normalizedPath, branch); err != nil {
		return DefaultWorktreeBinding{}, err
	}
	return DefaultWorktreeBinding{
		Path:    normalizedPath,
		Branch:  branch,
		Created: true,
	}, nil
}

func gitBranchExists(repoRoot, branch string) bool {
	if strings.TrimSpace(branch) == "" {
		return false
	}
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch) // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	return cmd.Run() == nil
}

func gitHasHead(repoRoot string) bool {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", "--quiet", "HEAD") // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	return cmd.Run() == nil
}

func ValidateChangeWorktree(root string, change model.Change) (model.WorktreeValidationResult, error) {
	result := model.WorktreeValidationResult{}
	worktreePath := strings.TrimSpace(change.WorktreePath)
	worktreeBranch := strings.TrimSpace(change.WorktreeBranch)

	if worktreePath == "" && worktreeBranch == "" {
		if !change.NeedsDiscovery {
			return result, nil
		}
		switch change.CurrentState {
		case model.StateS2Implement, model.StateS3Review:
			result.Blockers = []model.ReasonCode{model.NewReasonCode(WorktreeReasonMetadataRequired, "")}
		}
		return result, nil
	}

	if worktreePath == "" || worktreeBranch == "" {
		result.Blockers = []model.ReasonCode{model.NewReasonCode(WorktreeReasonMetadataRequired, "")}
		return result, nil
	}

	normalized, err := NormalizePath(worktreePath)
	if err != nil {
		result.Blockers = []model.ReasonCode{model.NewReasonCode(WorktreeReasonPathInvalid, "")}
		return result, nil
	}
	result.NormalizedPath = normalized
	result.NormalizedBranch = worktreeBranch

	reasons, err := ValidateDedicatedWorktreeAuthenticityReasons(root, worktreePath, worktreeBranch)
	if err != nil {
		return model.WorktreeValidationResult{}, err
	}
	result.Blockers = reasons
	return result, nil
}

func ValidateWorktreeAuthenticityReasons(repoRoot, worktreePath, expectedBranch string) ([]string, error) {
	reasons := []string{}
	if strings.TrimSpace(worktreePath) == "" || strings.TrimSpace(expectedBranch) == "" {
		reasons = append(reasons, WorktreeReasonMetadataRequired)
		return reasons, nil
	}

	normalizedPath, err := NormalizePath(worktreePath)
	if err != nil {
		reasons = append(reasons, WorktreeReasonPathInvalid)
		return reasons, nil
	}
	stat, err := os.Stat(normalizedPath)
	if err != nil {
		reasons = append(reasons, WorktreeReasonPathInvalid)
		return reasons, nil
	}
	if !stat.IsDir() {
		reasons = append(reasons, WorktreeReasonPathInvalid)
		return reasons, nil
	}

	registered, err := listGitWorktrees(repoRoot)
	if err != nil {
		return nil, err
	}
	if _, exists := registered[normalizedPath]; !exists {
		reasons = append(reasons, WorktreeReasonPathInvalid)
		return stringutil.UniqueSorted(reasons), nil
	}

	actualBranch, ok := gitBranchFromMetadata(normalizedPath)
	if !ok {
		cmd := exec.Command("git", "-C", normalizedPath, "rev-parse", "--abbrev-ref", "HEAD") // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("resolve worktree branch: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		actualBranch = strings.TrimSpace(string(out))
	}
	if actualBranch != expectedBranch {
		reasons = append(reasons, WorktreeReasonBranchMismatch)
	}
	return stringutil.UniqueSorted(reasons), nil
}

func ValidateDedicatedWorktreeAuthenticityReasons(repoRoot, worktreePath, expectedBranch string) ([]model.ReasonCode, error) {
	reasons, err := ValidateWorktreeAuthenticityReasons(repoRoot, worktreePath, expectedBranch)
	if err != nil {
		return nil, err
	}
	if len(reasons) > 0 {
		return reasonCodesFromWorktreeReasons(reasons), nil
	}

	workspaceRoot, err := gitWorkspaceRoot(repoRoot)
	if err != nil {
		return nil, err
	}
	normalizedRepoRoot, err := NormalizePath(workspaceRoot)
	if err != nil {
		return []model.ReasonCode{model.NewReasonCode(WorktreeReasonPathInvalid, "")}, nil
	}
	normalizedWorktreePath, err := NormalizePath(worktreePath)
	if err != nil {
		return []model.ReasonCode{model.NewReasonCode(WorktreeReasonPathInvalid, "")}, nil
	}
	if normalizedRepoRoot == normalizedWorktreePath {
		return []model.ReasonCode{model.NewReasonCode(WorktreeReasonDedicatedRequired, "")}, nil
	}
	return nil, nil
}

// resolveWorktreeActualBranch returns the git branch the bound worktree is
// actually on (the same value ValidateWorktreeAuthenticityReasons compares
// against). It performs no mutation.
func resolveWorktreeActualBranch(worktreePath string) (string, error) {
	normalizedPath, err := NormalizePath(worktreePath)
	if err != nil {
		return "", err
	}
	if actualBranch, ok := gitBranchFromMetadata(normalizedPath); ok {
		return strings.TrimSpace(actualBranch), nil
	}
	cmd := exec.Command("git", "-C", normalizedPath, "rev-parse", "--abbrev-ref", "HEAD") // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve worktree branch: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// ReconcileWorktreeBranchBinding realigns a bound change's recorded
// WorktreeBranch to the worktree's actual git branch when the ONLY authenticity
// problem is a branch mismatch on an otherwise-valid dedicated worktree. It is a
// metadata reconciliation — no `git checkout`/HEAD move — routed through the
// canonical PersistScopeWorktreeMetadata setter, so worktree-preflight remains
// the initial binder and this only realigns an existing binding to reality.
//
// It returns (false, nil) and leaves the change untouched for any other
// condition (unbound, path invalid/unregistered, non-dedicated, or no mismatch),
// preserving the worktree gate's fail-closed behavior for those cases.
func ReconcileWorktreeBranchBinding(repoRoot string, change *model.Change) (bool, error) {
	if change == nil {
		return false, nil
	}
	worktreePath := strings.TrimSpace(change.WorktreePath)
	if worktreePath == "" || strings.TrimSpace(change.WorktreeBranch) == "" {
		return false, nil
	}
	reasons, err := ValidateDedicatedWorktreeAuthenticityReasons(repoRoot, worktreePath, change.WorktreeBranch)
	if err != nil {
		return false, err
	}
	if len(reasons) != 1 || strings.TrimSpace(reasons[0].Code) != WorktreeReasonBranchMismatch {
		return false, nil
	}
	actualBranch, err := resolveWorktreeActualBranch(worktreePath)
	if err != nil {
		return false, err
	}
	if actualBranch == "" || actualBranch == strings.TrimSpace(change.WorktreeBranch) {
		return false, nil
	}
	if err := PersistScopeWorktreeMetadata(change, change.WorktreePath, actualBranch); err != nil {
		return false, err
	}
	return true, nil
}

func reasonCodesFromWorktreeReasons(reasons []string) []model.ReasonCode {
	if len(reasons) == 0 {
		return nil
	}
	out := make([]model.ReasonCode, 0, len(reasons))
	for _, reason := range reasons {
		out = append(out, model.NewReasonCode(reason, ""))
	}
	return model.NormalizeReasonCodes(out)
}

func ParseWorktreePreflightReferences(references []string) (path string, branch string, baselineVerifyCmd string, reasons []string) {
	for _, ref := range references {
		switch {
		case strings.HasPrefix(ref, worktreeRefPathPrefix):
			path = strings.TrimSpace(strings.TrimPrefix(ref, worktreeRefPathPrefix))
		case strings.HasPrefix(ref, worktreeRefBranchPrefix):
			branch = strings.TrimSpace(strings.TrimPrefix(ref, worktreeRefBranchPrefix))
		case strings.HasPrefix(ref, worktreeRefBaselinePrefix):
			baselineVerifyCmd = strings.TrimSpace(strings.TrimPrefix(ref, worktreeRefBaselinePrefix))
		}
	}

	if path == "" {
		reasons = append(reasons, "missing worktree_path reference")
	}
	if branch == "" {
		reasons = append(reasons, "missing worktree_branch reference")
	}
	if baselineVerifyCmd == "" {
		reasons = append(reasons, "missing baseline_verify_cmd reference")
	}
	return path, branch, baselineVerifyCmd, stringutil.UniqueSorted(reasons)
}

func listGitWorktrees(repoRoot string) (map[string]struct{}, error) {
	return listGitWorktreesCachedWithLister(repoRoot, listGitWorktreesUncached)
}

func listGitWorktreesCachedWithLister(repoRoot string, lister func(string) (map[string]struct{}, error)) (map[string]struct{}, error) {
	normalizedRoot, err := NormalizePath(repoRoot)
	if err != nil {
		normalizedRoot = filepath.Clean(repoRoot)
	}
	probeBefore := worktreeListProbeForRepo(normalizedRoot)

	worktreeListCache.mu.Lock()
	entry, ok := worktreeListCache.entries[normalizedRoot]
	if ok && worktreeListProbeMatches(entry.Probe, probeBefore) {
		worktreeListCache.mu.Unlock()
		return cloneWorktreeSet(entry.Worktrees), nil
	}
	worktreeListCache.mu.Unlock()

	worktrees, err := lister(normalizedRoot)
	if err != nil {
		return nil, err
	}
	probeAfter := worktreeListProbeForRepo(normalizedRoot)

	worktreeListCache.mu.Lock()
	if entry, ok := worktreeListCache.entries[normalizedRoot]; ok && worktreeListProbeMatches(entry.Probe, probeAfter) {
		worktreeListCache.mu.Unlock()
		return cloneWorktreeSet(entry.Worktrees), nil
	}
	if worktreeListProbeMatches(probeBefore, probeAfter) {
		worktreeListCache.entries[normalizedRoot] = worktreeListCacheEntry{
			Probe:     probeAfter,
			Worktrees: cloneWorktreeSet(worktrees),
		}
	}
	worktreeListCache.mu.Unlock()
	return cloneWorktreeSet(worktrees), nil
}

func invalidateWorktreeListCache(repoRoot string) {
	normalizedRoot, err := NormalizePath(repoRoot)
	if err != nil {
		normalizedRoot = filepath.Clean(repoRoot)
	}
	worktreeListCache.mu.Lock()
	delete(worktreeListCache.entries, normalizedRoot)
	worktreeListCache.mu.Unlock()
}

func listGitWorktreesUncached(repoRoot string) (map[string]struct{}, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain") // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list git worktrees: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	worktrees := map[string]struct{}{}
	lines := bytes.Split(out, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("worktree ")) {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(string(line), "worktree "))
		normalized, err := NormalizePath(path)
		if err != nil {
			return nil, err
		}
		worktrees[normalized] = struct{}{}
	}
	return worktrees, nil
}

// ResolveGitWorkspaceRoot returns the git worktree root for root.
func ResolveGitWorkspaceRoot(root string) (string, error) {
	return gitWorkspaceRoot(root)
}

func gitWorkspaceRoot(root string) (string, error) {
	normalizedRoot, err := NormalizePath(root)
	if err != nil {
		normalizedRoot = filepath.Clean(root)
	}
	if worktreeRoot, gitMetadataPath, ok := findGitMetadata(normalizedRoot); ok {
		if gitDir := gitDirPathFromMetadata(worktreeRoot, gitMetadataPath); gitDir != "" && gitDirLooksLikeWorktreeMetadata(gitDir) {
			return worktreeRoot, nil
		}
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = normalizedRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	worktreeRoot := strings.TrimSpace(string(out))
	if worktreeRoot == "" {
		return normalizedRoot, nil
	}
	if !filepath.IsAbs(worktreeRoot) {
		worktreeRoot = filepath.Join(normalizedRoot, worktreeRoot)
	}
	return filepath.Clean(worktreeRoot), nil
}

func gitCommandReportsNotRepository(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not a git repository")
}

func worktreeListProbeForRepo(repoRoot string) worktreeListProbe {
	path := filepath.Join(GitCommonDir(repoRoot), "worktrees")
	info, err := os.Stat(path)
	if err != nil {
		return worktreeListProbe{Path: path}
	}
	probe := worktreeListProbe{
		Path:    path,
		Exists:  true,
		ModTime: info.ModTime().UTC(),
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return probe
	}
	probe.EntryNames = make([]string, 0, len(entries))
	for _, entry := range entries {
		probe.EntryNames = append(probe.EntryNames, entry.Name())
	}
	slices.Sort(probe.EntryNames)
	return probe
}

func worktreeListProbeMatches(a, b worktreeListProbe) bool {
	if a.Path != b.Path || a.Exists != b.Exists {
		return false
	}
	if !a.Exists {
		return true
	}
	return a.ModTime.Equal(b.ModTime) && slices.Equal(a.EntryNames, b.EntryNames)
}

func cloneWorktreeSet(in map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for path := range in {
		out[path] = struct{}{}
	}
	return out
}

// NormalizePath resolves a path to its canonical absolute form with symlink resolution.
// Used for worktree path comparison across the codebase.
func NormalizePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	return filepath.Clean(abs), nil
}
