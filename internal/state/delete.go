package state

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

// DeleteOptions controls which targets `slipway delete` acts on.
type DeleteOptions struct {
	// RemoveWorktree also removes the bound git worktree (opt-in).
	RemoveWorktree bool
	// Archived operates on the archived terminal record instead of active state.
	Archived bool
	// Force overrides the uncommitted-tracked-changes refusal on worktree removal.
	Force bool
	// CurrentWorktree, when set, marks the worktree the command is running inside
	// so the plan refuses to remove it (the operator must run worktree removal
	// from the repo root or another worktree). Empty disables the guard.
	CurrentWorktree string
}

// DeleteMode describes what class of state a plan targets.
type DeleteMode string

const (
	// DeleteModeActive targets an active change whose change.yaml authority loads.
	DeleteModeActive DeleteMode = "active"
	// DeleteModeOrphan targets a partially-deleted bundle directory missing its
	// change.yaml authority (the stale-binding recovery case).
	DeleteModeOrphan DeleteMode = "orphan"
	// DeleteModeArchived targets an archived terminal record.
	DeleteModeArchived DeleteMode = "archived"
)

// DeleteTargetKind identifies a removable class of state.
type DeleteTargetKind string

const (
	DeleteTargetGovernedBundle DeleteTargetKind = "governed_bundle"
	DeleteTargetRuntimeBinding DeleteTargetKind = "runtime_binding"
	DeleteTargetWorktree       DeleteTargetKind = "worktree"
	DeleteTargetArchivedBundle DeleteTargetKind = "archived_bundle"
)

// DeleteAction is the planned disposition of a target.
type DeleteAction string

const (
	// DeleteActionDelete marks a present target that will be removed.
	DeleteActionDelete DeleteAction = "delete"
	// DeleteActionSkip marks an absent target (nothing to do).
	DeleteActionSkip DeleteAction = "skip"
	// DeleteActionRefused marks a present target blocked by a safety check.
	DeleteActionRefused DeleteAction = "refused"
)

// DeleteTarget is one planned removal, rendered for the dry-run plan and the
// post-run result so the operator always sees what was deleted, skipped, or
// refused.
type DeleteTarget struct {
	Kind   DeleteTargetKind `json:"kind"`
	Path   string           `json:"path"`
	Action DeleteAction     `json:"action"`
	Reason string           `json:"reason,omitempty"`

	// absPath is the on-disk path used during execution; it is intentionally not
	// serialized (Path carries the repo-relative display form).
	absPath string
}

// DeletePlan is the dry-run description of a `slipway delete` invocation. It is
// produced without mutating any state.
type DeletePlan struct {
	Slug    string         `json:"slug"`
	Mode    DeleteMode     `json:"mode"`
	Targets []DeleteTarget `json:"targets"`
}

// Deletions returns the targets that will be removed.
func (p DeletePlan) Deletions() []DeleteTarget {
	return p.targetsWithAction(DeleteActionDelete)
}

// Refusals returns the targets blocked by a safety check.
func (p DeletePlan) Refusals() []DeleteTarget {
	return p.targetsWithAction(DeleteActionRefused)
}

func (p DeletePlan) targetsWithAction(action DeleteAction) []DeleteTarget {
	var out []DeleteTarget
	for _, t := range p.Targets {
		if t.Action == action {
			out = append(out, t)
		}
	}
	return out
}

// HasRefusals reports whether any target is blocked by a safety check.
func (p DeletePlan) HasRefusals() bool {
	return len(p.Refusals()) > 0
}

// NothingToDelete reports whether no present target would be removed.
func (p DeletePlan) NothingToDelete() bool {
	return len(p.Deletions()) == 0
}

// DeleteResult reports what ExecuteDeletePlan actually removed.
type DeleteResult struct {
	Slug    string         `json:"slug"`
	Mode    DeleteMode     `json:"mode"`
	Removed []DeleteTarget `json:"removed,omitempty"`
	Skipped []DeleteTarget `json:"skipped,omitempty"`
}

var canonicalChangeSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ValidateChangeSlug ensures a user-provided change slug is a canonical single
// path segment before any lock, bundle, runtime, or archive path is derived from
// it.
func ValidateChangeSlug(slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return errors.New("slug is required")
	}
	if len(slug) > model.MaxSlugLength {
		return fmt.Errorf("slug exceeds maximum length %d", model.MaxSlugLength)
	}
	if filepath.IsAbs(slug) || slug == "." || slug == ".." || strings.ContainsAny(slug, `/\`) || filepath.Clean(slug) != slug {
		return errors.New("slug must be a single path segment")
	}
	if !canonicalChangeSlugPattern.MatchString(slug) {
		return errors.New("slug must use lowercase letters, digits, and single hyphen separators")
	}
	return nil
}

// BuildDeletePlan plans deletion of an active or orphaned governed change (or,
// with opts.Archived, an archived terminal record) identified by slug. It never
// mutates state.
func BuildDeletePlan(root, slug string, opts DeleteOptions) (DeletePlan, error) {
	slug = strings.TrimSpace(slug)
	if err := ValidateChangeSlug(slug); err != nil {
		return DeletePlan{}, err
	}
	if opts.Archived {
		return buildArchivedDeletePlan(root, slug, opts)
	}
	return buildActiveDeletePlan(root, slug, opts)
}

func buildActiveDeletePlan(root, slug string, opts DeleteOptions) (DeletePlan, error) {
	bundleDir, err := locateBundleDir(ActiveBundlesDir, root, slug)
	if err != nil {
		return DeletePlan{}, err
	}

	plan := DeletePlan{Slug: slug, Mode: DeleteModeActive}

	// Governed bundle.
	if bundleDir != "" {
		if !fileExists(filepath.Join(bundleDir, "change.yaml")) {
			plan.Mode = DeleteModeOrphan
		}
		plan.Targets = append(plan.Targets, deleteTarget(root, DeleteTargetGovernedBundle, bundleDir, DeleteActionDelete, ""))
	} else {
		plan.Targets = append(plan.Targets, DeleteTarget{Kind: DeleteTargetGovernedBundle, Path: DisplayPath(root, filepath.Join(ActiveBundlesDir(root), slug)), Action: DeleteActionSkip, Reason: "no governed bundle directory found"})
	}

	// Per-change runtime state / binding.
	runtimeDir := ChangeDir(root, slug)
	if dirExists(runtimeDir) {
		plan.Targets = append(plan.Targets, deleteTarget(root, DeleteTargetRuntimeBinding, runtimeDir, DeleteActionDelete, ""))
	} else {
		plan.Targets = append(plan.Targets, DeleteTarget{Kind: DeleteTargetRuntimeBinding, Path: DisplayPath(root, runtimeDir), Action: DeleteActionSkip, Reason: "no runtime binding present"})
	}

	// Optional worktree removal.
	plan.Targets = append(plan.Targets, planWorktreeTarget(root, slug, opts))

	return plan, nil
}

func buildArchivedDeletePlan(root, slug string, opts DeleteOptions) (DeletePlan, error) {
	archivedDir, err := locateBundleDir(ArchivedBundlesDir, root, slug)
	if err != nil {
		return DeletePlan{}, err
	}
	plan := DeletePlan{Slug: slug, Mode: DeleteModeArchived}
	if archivedDir != "" {
		plan.Targets = append(plan.Targets, deleteTarget(root, DeleteTargetArchivedBundle, archivedDir, DeleteActionDelete, ""))
	} else {
		plan.Targets = append(plan.Targets, DeleteTarget{Kind: DeleteTargetArchivedBundle, Path: DisplayPath(root, filepath.Join(ArchivedBundlesDir(root), slug)), Action: DeleteActionSkip, Reason: "no archived record found"})
	}
	if opts.RemoveWorktree || opts.Force {
		plan.Targets = append(plan.Targets, DeleteTarget{
			Kind:   DeleteTargetWorktree,
			Path:   "",
			Action: DeleteActionSkip,
			Reason: "--archived only purges archived records; --worktree/--force apply to active worktree removal",
		})
	}
	return plan, nil
}

func planWorktreeTarget(root, slug string, opts DeleteOptions) DeleteTarget {
	return planWorktreeTargetWithResolver(root, slug, opts, resolveChangeWorktreePath)
}

func planWorktreeTargetWithResolver(root, slug string, opts DeleteOptions, resolver func(string, string) (string, error)) DeleteTarget {
	worktreePath, err := resolver(root, slug)
	if err != nil {
		target := DeleteTarget{
			Kind:   DeleteTargetWorktree,
			Path:   "",
			Action: DeleteActionSkip,
			Reason: fmt.Sprintf("could not determine bound worktree: %v", err),
		}
		if opts.RemoveWorktree {
			target.Action = DeleteActionRefused
		}
		return target
	}
	if strings.TrimSpace(worktreePath) == "" {
		reason := "not requested (pass --worktree)"
		if opts.RemoveWorktree {
			reason = "no bound worktree"
		}
		return DeleteTarget{Kind: DeleteTargetWorktree, Path: "", Action: DeleteActionSkip, Reason: reason}
	}
	target := deleteTarget(root, DeleteTargetWorktree, worktreePath, DeleteActionDelete, "")
	if !opts.RemoveWorktree {
		target.Action = DeleteActionSkip
		target.Reason = "preserved (pass --worktree to remove)"
		return target
	}
	if isSameWorktree(opts.CurrentWorktree, worktreePath) {
		target.Action = DeleteActionRefused
		target.Reason = "cannot remove the worktree you are running inside; re-run from the repository root"
		return target
	}
	registered, rerr := registeredGitWorktree(root, worktreePath)
	if rerr != nil {
		target.Action = DeleteActionRefused
		target.Reason = fmt.Sprintf("cannot determine whether worktree is registered: %v", rerr)
		return target
	}
	if !registered {
		target.Action = DeleteActionSkip
		target.Reason = "bound worktree is no longer registered"
		return target
	}
	exists, eerr := worktreeDirExists(worktreePath)
	if eerr != nil {
		target.Action = DeleteActionRefused
		target.Reason = fmt.Sprintf("cannot determine whether worktree exists: %v", eerr)
		return target
	}
	if !exists {
		target.Reason = "bound worktree directory is already missing; git worktree metadata will be removed"
		return target
	}
	provisioned, perr := worktreeIsSlipwayProvisioned(root, slug, worktreePath)
	if perr != nil {
		target.Action = DeleteActionRefused
		target.Reason = fmt.Sprintf("cannot verify whether Slipway provisioned this worktree: %v", perr)
		return target
	}
	if !provisioned {
		target.Action = DeleteActionRefused
		target.Reason = "refusing to remove a worktree Slipway did not provision (not .worktrees/<slug> on feat/<slug>); preserve it or remove it yourself — Slipway never removes a worktree it did not provision"
		return target
	}
	refusal, derr := worktreeRemovalRefusalReason(worktreePath, slug, opts.Force)
	if derr != nil {
		target.Action = DeleteActionRefused
		target.Reason = fmt.Sprintf("cannot determine worktree cleanliness: %v", derr)
		return target
	}
	if refusal != "" {
		target.Action = DeleteActionRefused
		target.Reason = "worktree " + refusal
	}
	return target
}

// ExecuteDeletePlan performs the deletions described by plan. It is idempotent
// on already-missing targets and removes the worktree (when planned) before the
// in-worktree bundle so a worktree-resident bundle is not orphaned mid-run.
func ExecuteDeletePlan(root string, plan DeletePlan, opts DeleteOptions) (DeleteResult, error) {
	result := DeleteResult{Slug: plan.Slug, Mode: plan.Mode}

	// Order: worktree first (it may contain the governed bundle), then bundle,
	// then runtime binding last so the lock/binding teardown happens at the end.
	order := map[DeleteTargetKind]int{
		DeleteTargetWorktree:       0,
		DeleteTargetGovernedBundle: 1,
		DeleteTargetArchivedBundle: 1,
		DeleteTargetRuntimeBinding: 2,
	}
	ordered := append([]DeleteTarget(nil), plan.Targets...)
	stableSortTargets(ordered, order)

	for _, target := range ordered {
		if target.Action != DeleteActionDelete {
			result.Skipped = append(result.Skipped, target)
			continue
		}
		if err := executeDeleteTarget(root, plan.Slug, target, opts); err != nil {
			return result, err
		}
		result.Removed = append(result.Removed, target)
	}
	return result, nil
}

func executeDeleteTarget(root, slug string, target DeleteTarget, opts DeleteOptions) error {
	switch target.Kind {
	case DeleteTargetGovernedBundle, DeleteTargetArchivedBundle:
		if err := removeDirAll(target.absPath); err != nil {
			return fmt.Errorf("remove %s %q: %w", target.Kind, target.absPath, err)
		}
		if target.Kind == DeleteTargetArchivedBundle {
			if _, err := removeArchivedLocalRuntimeState(root, "", slug); err != nil {
				return fmt.Errorf("remove archived runtime residue for %q: %w", slug, err)
			}
		}
		return nil
	case DeleteTargetRuntimeBinding:
		if err := removePerChangeLocalRuntimeState(root, slug); err != nil {
			return fmt.Errorf("remove runtime binding for %q: %w", slug, err)
		}
		return nil
	case DeleteTargetWorktree:
		if err := RemoveChangeWorktree(root, slug, target.absPath, opts.Force); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unknown delete target kind %q", target.Kind)
	}
}

// RemoveChangeWorktree removes the git worktree at worktreePath. It refuses a
// worktree with uncommitted tracked changes, or untracked files outside known
// generated Slipway paths, unless force is set; the worktree's branch is never
// deleted. Git's own removal always runs with --force after this gate so expected
// generated files do not block abandoned-worktree cleanup.
func RemoveChangeWorktree(root, slug, worktreePath string, force bool) error {
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return errors.New("worktree path is required")
	}
	exists, err := worktreeDirExists(worktreePath)
	if err != nil {
		return fmt.Errorf("stat worktree %q: %w", worktreePath, err)
	}
	if exists {
		if refusal, err := worktreeRemovalRefusalReason(worktreePath, slug, force); err != nil {
			return err
		} else if refusal != "" {
			return fmt.Errorf("worktree %q %s", worktreePath, refusal)
		}
	}
	repoRoot, err := gitWorkspaceRoot(root)
	if err != nil {
		return fmt.Errorf("resolve git worktree root: %w", err)
	}
	out, err := exec.Command("git", "-C", repoRoot, "worktree", "remove", "--force", worktreePath).CombinedOutput() // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	if err != nil {
		return fmt.Errorf("git worktree remove failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	// Best-effort prune of administrative metadata for the removed worktree.
	_ = exec.Command("git", "-C", repoRoot, "worktree", "prune").Run() // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	invalidateWorktreeListCache(repoRoot)
	return nil
}

// worktreeIsSlipwayProvisioned reports whether the worktree at worktreePath is one
// Slipway itself provisioned for slug: positive proof is BOTH its default
// .worktrees/<slug> path AND its feat/<slug> branch. Anything else is treated as
// externally managed and must never be removed by `slipway delete --worktree`,
// even with --force (issue #285). A non-git context yields (false, nil).
func worktreeIsSlipwayProvisioned(root, slug, worktreePath string) (bool, error) {
	repoRoot, err := gitWorkspaceRoot(root)
	if err != nil {
		if gitCommandReportsNotRepository(err) {
			return false, nil
		}
		return false, err
	}
	defaultPath, err := NormalizePath(DefaultWorktreePath(repoRoot, slug))
	if err != nil {
		return false, err
	}
	normalized, err := NormalizePath(worktreePath)
	if err != nil {
		normalized = filepath.Clean(worktreePath)
	}
	if normalized != defaultPath {
		return false, nil
	}
	branch, err := resolveWorktreeActualBranch(worktreePath)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(branch) == DefaultWorktreeBranch(slug), nil
}

// resolveChangeWorktreePath returns the bound worktree path for slug, reading
// the git-local binding first and falling back to the default worktree location
// when it is a registered worktree. Returns "" when no bound worktree exists.
func resolveChangeWorktreePath(root, slug string) (string, error) {
	if binding, ok := readWorktreeBinding(root, slug); ok {
		return binding.WorktreePath, nil
	}
	repoRoot, err := gitWorkspaceRoot(root)
	if err != nil {
		if gitCommandReportsNotRepository(err) {
			return "", nil
		}
		return "", err
	}
	candidate, err := NormalizePath(DefaultWorktreePath(repoRoot, slug))
	if err != nil {
		return "", nil
	}
	registered, err := listGitWorktrees(repoRoot)
	if err != nil {
		return "", err
	}
	if _, ok := registered[candidate]; ok {
		return candidate, nil
	}
	return "", nil
}

func worktreeDirExists(worktreePath string) (bool, error) {
	info, err := os.Stat(worktreePath)
	if err == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("path exists but is not a directory")
		}
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func registeredGitWorktree(root, worktreePath string) (bool, error) {
	repoRoot, err := gitWorkspaceRoot(root)
	if err != nil {
		if gitCommandReportsNotRepository(err) {
			return false, nil
		}
		return false, err
	}
	normalized, err := NormalizePath(worktreePath)
	if err != nil {
		normalized = filepath.Clean(worktreePath)
	}
	registered, err := listGitWorktrees(repoRoot)
	if err != nil {
		return false, err
	}
	_, ok := registered[normalized]
	return ok, nil
}

// worktreeHasUncommittedTrackedChanges reports whether the worktree has staged
// or unstaged changes to tracked files. Untracked files (the governed bundle,
// build output) are intentionally NOT counted, so a freshly-provisioned
// governed worktree reads clean.
func worktreeHasUncommittedTrackedChanges(worktreePath string) (bool, error) {
	for _, args := range [][]string{
		{"-C", worktreePath, "diff", "--quiet"},
		{"-C", worktreePath, "diff", "--cached", "--quiet"},
	} {
		err := exec.Command("git", args...).Run() // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
		if err == nil {
			continue
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, fmt.Errorf("git %s in worktree %q: %w", strings.Join(args, " "), worktreePath, err)
	}
	return false, nil
}

func worktreeRemovalRefusalReason(worktreePath, slug string, force bool) (string, error) {
	if force {
		return "", nil
	}
	dirty, err := worktreeHasUncommittedTrackedChanges(worktreePath)
	if err != nil {
		return "", err
	}
	if dirty {
		return "has uncommitted tracked changes; pass --force to remove anyway", nil
	}
	unsafeUntracked, err := worktreeUnsafeUntrackedFiles(worktreePath, slug)
	if err != nil {
		return "", err
	}
	if len(unsafeUntracked) > 0 {
		return fmt.Sprintf(
			"has untracked files outside generated Slipway paths (%s); pass --force to remove anyway",
			formatPathSample(unsafeUntracked, 3),
		), nil
	}
	return "", nil
}

func worktreeUnsafeUntrackedFiles(worktreePath, slug string) ([]string, error) {
	// Include ignored files: `git worktree remove --force` deletes the whole
	// worktree, so ignored local files need the same fail-closed treatment as
	// ordinary untracked files unless they are known generated Slipway paths.
	out, err := exec.Command("git", "-C", worktreePath, "ls-files", "--others", "-z").Output() // #nosec G204 -- command and arguments are constructed by Slipway helpers and executed without shell interpolation.
	if err != nil {
		return nil, fmt.Errorf("git ls-files --others in worktree %q: %w", worktreePath, err)
	}
	var unsafe []string
	for _, raw := range bytes.Split(out, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		path := filepath.ToSlash(filepath.Clean(string(raw)))
		if safeGeneratedUntrackedPath(path, slug) {
			continue
		}
		unsafe = append(unsafe, path)
	}
	return unsafe, nil
}

func safeGeneratedUntrackedPath(path, slug string) bool {
	switch path {
	case ".gitignore", ".slipway.yaml":
		return true
	}
	for _, prefix := range []string{
		"artifacts/changes/" + slug + "/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func formatPathSample(paths []string, limit int) string {
	if len(paths) <= limit {
		return strings.Join(paths, ", ")
	}
	return strings.Join(paths[:limit], ", ") + fmt.Sprintf(", and %d more", len(paths)-limit)
}

// locateBundleDir finds an existing bundle directory for slug across every
// registered worktree scope, using dirFn to resolve active vs archived roots.
// Returns "" when no such directory exists.
func locateBundleDir(dirFn func(string) string, root, slug string) (string, error) {
	roots, err := allWorkspaceRoots(root)
	if err != nil {
		return "", err
	}
	for _, ws := range roots {
		dir := filepath.Join(dirFn(ws), slug)
		info, statErr := os.Stat(dir)
		if statErr == nil && info.IsDir() {
			return dir, nil
		}
		if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return "", statErr
		}
	}
	return "", nil
}

func deleteTarget(root string, kind DeleteTargetKind, absPath string, action DeleteAction, reason string) DeleteTarget {
	return DeleteTarget{
		Kind:    kind,
		Path:    DisplayPath(root, absPath),
		Action:  action,
		Reason:  reason,
		absPath: absPath,
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isSameWorktree reports whether two paths resolve to the same worktree root.
func isSameWorktree(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	na, err := NormalizePath(a)
	if err != nil {
		na = filepath.Clean(a)
	}
	nb, err := NormalizePath(b)
	if err != nil {
		nb = filepath.Clean(b)
	}
	return na == nb
}

// stableSortTargets orders targets by the supplied rank map while preserving
// the relative order of equal-rank entries.
func stableSortTargets(targets []DeleteTarget, rank map[DeleteTargetKind]int) {
	for i := 1; i < len(targets); i++ {
		for j := i; j > 0 && rank[targets[j].Kind] < rank[targets[j-1].Kind]; j-- {
			targets[j], targets[j-1] = targets[j-1], targets[j]
		}
	}
}
