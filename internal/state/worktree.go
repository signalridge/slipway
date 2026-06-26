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
	// Heads fingerprints each linked worktree's HEAD ("<entry>\x00<head>", sorted).
	// The cached records carry each worktree's Branch, but a branch switch rewrites
	// .git/worktrees/<entry>/HEAD WITHOUT changing the parent dir's mtime or entry
	// set — so without this, branch-sensitive matching (FindSlugWorktreeMatch, #285)
	// could read a stale branch from the cache and misjudge SlipwayManaged.
	Heads []string
}

type worktreeListCacheEntry struct {
	Probe   worktreeListProbe
	Records []gitWorktreeRecord
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
		baseRef, err := validateWorktreeBaseRef(repoRoot, change.BaseRef)
		if err != nil {
			return DefaultWorktreeBinding{}, err
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

func validateWorktreeBaseRef(repoRoot, raw string) (string, error) {
	baseRef := strings.TrimSpace(raw)
	if baseRef == "" {
		baseRef = "HEAD"
	}
	if strings.HasPrefix(baseRef, "-") {
		return "", fmt.Errorf("invalid base_ref %q: value must not start with '-' because it would be parsed as a git option; repair the change authority to use HEAD, a branch, a tag, or a commit SHA", baseRef)
	}
	if strings.ContainsAny(baseRef, "\x00\r\n") {
		return "", fmt.Errorf("invalid base_ref %q: value must be a single git ref or commit-ish; repair the change authority to use HEAD, a branch, a tag, or a commit SHA", baseRef)
	}
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", "--quiet", baseRef+"^{commit}") // #nosec G204 -- baseRef is validated as data and passed as one argv element without shell interpolation.
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("invalid base_ref %q: git cannot resolve it as a commit; repair the change authority to use HEAD, a branch, a tag, or a commit SHA", baseRef)
	}
	return baseRef, nil
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

// listGitWorktrees returns the set of normalized worktree paths registered in
// the repository. It projects the cached path+branch records to a path set so a
// single porcelain listing serves both path and branch lookups per repo probe.
func listGitWorktrees(repoRoot string) (map[string]struct{}, error) {
	records, err := listGitWorktreeRecords(repoRoot)
	if err != nil {
		return nil, err
	}
	return worktreeRecordPathSet(records)
}

func worktreeRecordPathSet(records []gitWorktreeRecord) (map[string]struct{}, error) {
	set := make(map[string]struct{}, len(records))
	for _, rec := range records {
		normalized, err := NormalizePath(rec.Path)
		if err != nil {
			return nil, err
		}
		set[normalized] = struct{}{}
	}
	return set, nil
}

func listGitWorktreeRecordsCachedWithLister(repoRoot string, lister func(string) ([]gitWorktreeRecord, error)) ([]gitWorktreeRecord, error) {
	normalizedRoot, err := NormalizePath(repoRoot)
	if err != nil {
		normalizedRoot = filepath.Clean(repoRoot)
	}
	probeBefore := worktreeListProbeForRepo(normalizedRoot)

	worktreeListCache.mu.Lock()
	entry, ok := worktreeListCache.entries[normalizedRoot]
	if ok && worktreeListProbeMatches(entry.Probe, probeBefore) {
		worktreeListCache.mu.Unlock()
		return cloneWorktreeRecords(entry.Records), nil
	}
	worktreeListCache.mu.Unlock()

	records, err := lister(normalizedRoot)
	if err != nil {
		return nil, err
	}
	probeAfter := worktreeListProbeForRepo(normalizedRoot)

	worktreeListCache.mu.Lock()
	if entry, ok := worktreeListCache.entries[normalizedRoot]; ok && worktreeListProbeMatches(entry.Probe, probeAfter) {
		worktreeListCache.mu.Unlock()
		return cloneWorktreeRecords(entry.Records), nil
	}
	if worktreeListProbeMatches(probeBefore, probeAfter) {
		worktreeListCache.entries[normalizedRoot] = worktreeListCacheEntry{
			Probe:   probeAfter,
			Records: cloneWorktreeRecords(records),
		}
	}
	worktreeListCache.mu.Unlock()
	return cloneWorktreeRecords(records), nil
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

func listGitWorktreeRecordsUncached(repoRoot string) ([]gitWorktreeRecord, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain") // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list git worktrees: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return parseGitWorktreePorcelain(out), nil
}

// SlugWorktreeMatch describes a live git worktree (and its branch) whose
// identity corresponds to a change slug. It lets orphan-bundle recovery avoid
// recommending destructive cleanup of a slug whose live, possibly-unmerged work
// lives in a worktree Slipway does not manage.
type SlugWorktreeMatch struct {
	// WorktreePath is the normalized path of the matching live worktree.
	WorktreePath string
	// Branch is the worktree's checked-out branch (empty for a detached HEAD).
	Branch string
	// SlipwayManaged is true only when the match carries positive proof Slipway
	// itself provisioned the worktree: BOTH its default .worktrees/<slug> path and
	// its feat/<slug> branch. False marks an externally-managed worktree (e.g. one
	// a user placed at .worktrees/<slug> on a hand-named branch) that recovery must
	// never recommend removing.
	SlipwayManaged bool
}

// FindSlugWorktreeMatch reports whether any live git worktree corresponds to
// slug, matched either by Slipway's own worktree/branch convention or by a
// branch whose slugified identity equals slug. It returns the strongest match,
// preferring a Slipway-managed worktree when one exists. ok is false when no
// live worktree matches, or root is not a git repository.
func FindSlugWorktreeMatch(root, slug string) (match SlugWorktreeMatch, ok bool, err error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return SlugWorktreeMatch{}, false, nil
	}
	repoRoot, err := gitWorkspaceRoot(root)
	if err != nil {
		if gitCommandReportsNotRepository(err) {
			return SlugWorktreeMatch{}, false, nil
		}
		return SlugWorktreeMatch{}, false, err
	}
	records, err := listGitWorktreeRecords(repoRoot)
	if err != nil {
		return SlugWorktreeMatch{}, false, err
	}
	defaultPath, perr := NormalizePath(DefaultWorktreePath(repoRoot, slug))
	if perr != nil {
		defaultPath = filepath.Clean(DefaultWorktreePath(repoRoot, slug))
	}
	defaultBranch := DefaultWorktreeBranch(slug)
	normalizedRepoRoot, perr := NormalizePath(repoRoot)
	if perr != nil {
		normalizedRepoRoot = filepath.Clean(repoRoot)
	}

	var external *SlugWorktreeMatch
	for _, rec := range records {
		normPath, nerr := NormalizePath(rec.Path)
		if nerr != nil {
			normPath = filepath.Clean(rec.Path)
		}
		// Skip the checkout we resolved from: the workspace root is never the
		// slug's own dedicated worktree.
		if normPath == normalizedRepoRoot {
			continue
		}
		branch := strings.TrimSpace(rec.Branch)
		pathMatches := normPath == defaultPath
		branchMatches := branch == defaultBranch
		// A branch whose slugified identity equals slug also corresponds to the
		// slug, but only when the worktree actually carries a branch: a detached
		// HEAD has no branch and must not borrow SlugifyTitle's "change" fallback
		// to collide with a literal "change" slug.
		slugMatches := branch != "" && model.SlugifyTitle(branch) == slug
		if !pathMatches && !branchMatches && !slugMatches {
			continue
		}
		// Slipway-managed requires positive proof Slipway itself provisioned the
		// worktree: BOTH its default .worktrees/<slug> path AND its feat/<slug>
		// branch. A path or branch that merely coincides with the slug — e.g. an
		// external worktree a user placed at .worktrees/<slug> on a hand-named
		// branch (issue #285) — is NOT managed, and recovery must never recommend
		// removing it.
		managed := pathMatches && branchMatches
		candidate := SlugWorktreeMatch{WorktreePath: normPath, Branch: rec.Branch, SlipwayManaged: managed}
		if managed {
			// A Slipway-managed worktree is the authoritative match; existing
			// discard recovery already owns this case safely.
			return candidate, true, nil
		}
		if external == nil {
			c := candidate
			external = &c
		}
	}
	if external != nil {
		return *external, true, nil
	}
	return SlugWorktreeMatch{}, false, nil
}

type gitWorktreeRecord struct {
	Path   string
	Branch string
}

// listGitWorktreeRecords returns each registered worktree's path and checked-out
// branch (empty for a detached HEAD), cached behind the same repo probe as the
// rest of the worktree listing so callers can map a worktree back to a change
// slug without re-forking git.
func listGitWorktreeRecords(repoRoot string) ([]gitWorktreeRecord, error) {
	return listGitWorktreeRecordsCachedWithLister(repoRoot, listGitWorktreeRecordsUncached)
}

// parseGitWorktreePorcelain parses `git worktree list --porcelain` output into
// path+branch records. Paths are left as reported by git; callers normalize.
func parseGitWorktreePorcelain(out []byte) []gitWorktreeRecord {
	var records []gitWorktreeRecord
	var current *gitWorktreeRecord
	for _, raw := range bytes.Split(out, []byte("\n")) {
		line := string(raw)
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current != nil {
				records = append(records, *current)
			}
			current = &gitWorktreeRecord{Path: strings.TrimSpace(strings.TrimPrefix(line, "worktree "))}
		case strings.HasPrefix(line, "branch ") && current != nil:
			ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		}
	}
	if current != nil {
		records = append(records, *current)
	}
	return records
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
	probe.Heads = make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		probe.EntryNames = append(probe.EntryNames, name)
		// Read each linked worktree's HEAD so an in-place branch switch (which
		// rewrites HEAD but leaves the entry set and parent mtime untouched)
		// invalidates the cache. A missing/unreadable HEAD contributes empty
		// content; if it later appears the fingerprint changes and re-invalidates.
		head, _ := os.ReadFile(filepath.Join(path, name, "HEAD")) // #nosec G304 -- path is derived from the repo's own git metadata dir, not user input.
		probe.Heads = append(probe.Heads, name+"\x00"+strings.TrimSpace(string(head)))
	}
	slices.Sort(probe.EntryNames)
	slices.Sort(probe.Heads)
	return probe
}

func worktreeListProbeMatches(a, b worktreeListProbe) bool {
	if a.Path != b.Path || a.Exists != b.Exists {
		return false
	}
	if !a.Exists {
		return true
	}
	return a.ModTime.Equal(b.ModTime) && slices.Equal(a.EntryNames, b.EntryNames) && slices.Equal(a.Heads, b.Heads)
}

func cloneWorktreeRecords(in []gitWorktreeRecord) []gitWorktreeRecord {
	if in == nil {
		return nil
	}
	out := make([]gitWorktreeRecord, len(in))
	copy(out, in)
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
