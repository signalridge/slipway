package state

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type gitMetadataProbe struct {
	Path  string
	Kind  string
	Value string
}

type gitCommonDirProbe struct {
	GitMetadata gitMetadataProbe
	CommonDir   gitMetadataProbe
}

type gitCommonDirCacheEntry struct {
	Probe     gitCommonDirProbe
	CommonDir string
}

var gitCommonDirCache = struct {
	mu      sync.Mutex
	entries map[string]gitCommonDirCacheEntry
}{
	entries: map[string]gitCommonDirCacheEntry{},
}

var gitCommonDirResolver = resolveGitCommonDir

const scopeMarkerFileName = "scope-root"

// GitStateDir returns the shared git-internal slipway namespace.
func GitStateDir(root string) string {
	return gitStateDirForBase(GitCommonDir(root), root)
}

// workspaceGitDir returns the worktree-local git dir for the given root.
func workspaceGitDir(root string) string {
	normalizedRoot, err := NormalizePath(root)
	if err != nil {
		normalizedRoot = filepath.Clean(root)
	}
	worktreeRoot, err := gitWorkspaceRoot(normalizedRoot)
	if err != nil {
		worktreeRoot = normalizedRoot
	}
	gitMetadataPath := filepath.Join(worktreeRoot, ".git")
	if gitDir := gitDirPathFromMetadata(worktreeRoot, gitMetadataPath); gitDir != "" {
		return gitDir
	}
	if info, err := os.Stat(gitMetadataPath); err == nil && info.IsDir() {
		return filepath.Clean(gitMetadataPath)
	}
	return filepath.Clean(gitMetadataPath)
}

// workspaceGitStateDir returns the worktree-local slipway namespace.
func workspaceGitStateDir(root string) string {
	return gitStateDirForBase(workspaceGitDir(root), root)
}

// ScopeMarkerPath returns the shared scope marker path under the git-common dir.
func ScopeMarkerPath(root string) string {
	return filepath.Join(GitStateDir(root), scopeMarkerFileName)
}

// WorkspaceScopeMarkerPath returns the worktree-local scope marker path.
func WorkspaceScopeMarkerPath(root string) string {
	return filepath.Join(workspaceGitStateDir(root), scopeMarkerFileName)
}

func gitStateDirForBase(baseDir, root string) string {
	base := filepath.Join(baseDir, "slipway")
	if scopeRel := scopeRelativePath(root); scopeRel != "" {
		return filepath.Join(base, "scopes", scopeRel)
	}
	return base
}

// GitCommonDir returns the git common dir shared by the repo and its worktrees.
// Init gates on git presence, so this should only be called inside a git repo.
// If git resolution fails (non-git context), returns the root itself to avoid
// creating a spurious .git directory; callers will write runtime state under
// the project root, which is wrong but visible rather than silently corrupting
// the git namespace.
func GitCommonDir(root string) string {
	normalizedRoot, err := NormalizePath(root)
	if err != nil {
		normalizedRoot = filepath.Clean(root)
	}
	probe := gitCommonDirProbeForRoot(normalizedRoot)

	gitCommonDirCache.mu.Lock()
	entry, ok := gitCommonDirCache.entries[normalizedRoot]
	gitCommonDirCache.mu.Unlock()
	if ok && gitCommonDirProbeMatches(entry.Probe, probe) {
		return entry.CommonDir
	}

	commonDir, err := gitCommonDirResolver(normalizedRoot)
	if err != nil || commonDir == "" {
		// Not a git repository. Return the root itself so downstream
		// paths don't create a fake .git directory. This is defensive
		// only — init and command entry points validate git presence.
		return normalizedRoot
	}
	if filepath.IsAbs(commonDir) {
		commonDir = filepath.Clean(commonDir)
	} else {
		commonDir = filepath.Clean(filepath.Join(normalizedRoot, commonDir))
	}

	gitCommonDirCache.mu.Lock()
	gitCommonDirCache.entries[normalizedRoot] = gitCommonDirCacheEntry{
		Probe:     probe,
		CommonDir: commonDir,
	}
	gitCommonDirCache.mu.Unlock()
	return commonDir
}

func resolveGitCommonDir(normalizedRoot string) (string, error) {
	if commonDir, ok := gitCommonDirFromMetadata(normalizedRoot); ok {
		return commonDir, nil
	}

	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = normalizedRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes.TrimSpace(out))), nil
}

func gitCommonDirProbeForRoot(normalizedRoot string) gitCommonDirProbe {
	_, gitMetadataPath, ok := findGitMetadata(normalizedRoot)
	if !ok {
		gitMetadataPath = filepath.Join(normalizedRoot, ".git")
	}
	probe := gitCommonDirProbe{
		GitMetadata: gitMetadataProbeForPath(gitMetadataPath),
	}
	if gitDir := gitDirPathFromMetadata(filepath.Dir(gitMetadataPath), gitMetadataPath); gitDir != "" {
		probe.CommonDir = gitMetadataProbeForPath(filepath.Join(gitDir, "commondir"))
	}
	return probe
}

func gitCommonDirFromMetadata(normalizedRoot string) (string, bool) {
	worktreeRoot, gitMetadataPath, ok := findGitMetadata(normalizedRoot)
	if !ok {
		return "", false
	}
	gitDir := gitDirPathFromMetadata(worktreeRoot, gitMetadataPath)
	if gitDir == "" {
		return "", false
	}
	if !gitDirLooksLikeWorktreeMetadata(gitDir) {
		return "", false
	}

	commondirPath := filepath.Join(gitDir, "commondir")
	raw, err := os.ReadFile(commondirPath) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		if os.IsNotExist(err) {
			return filepath.Clean(gitDir), true
		}
		return "", false
	}

	commonDir := strings.TrimSpace(string(raw))
	if commonDir == "" {
		return filepath.Clean(gitDir), true
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(gitDir, commonDir)
	}
	return filepath.Clean(commonDir), true
}

func gitBranchFromMetadata(normalizedRoot string) (string, bool) {
	worktreeRoot, gitMetadataPath, ok := findGitMetadata(normalizedRoot)
	if !ok {
		return "", false
	}
	gitDir := gitDirPathFromMetadata(worktreeRoot, gitMetadataPath)
	if gitDir == "" {
		return "", false
	}

	raw, err := os.ReadFile(filepath.Join(gitDir, "HEAD")) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		return "", false
	}
	head := strings.TrimSpace(string(raw))
	if head == "" {
		return "", false
	}
	if !strings.HasPrefix(head, "ref:") {
		return "HEAD", true
	}
	ref := strings.TrimSpace(strings.TrimPrefix(head, "ref:"))
	const branchRefPrefix = "refs/heads/"
	if branch, ok := strings.CutPrefix(ref, branchRefPrefix); ok && branch != "" {
		return branch, true
	}
	return "", false
}

func gitDirLooksLikeWorktreeMetadata(gitDir string) bool {
	if _, err := os.Stat(filepath.Join(gitDir, "HEAD")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(gitDir, "config")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(gitDir, "commondir")); err == nil {
		return true
	}
	return false
}

func findGitMetadata(normalizedRoot string) (string, string, bool) {
	dir := normalizedRoot
	for {
		gitMetadataPath := filepath.Join(dir, ".git")
		if gitDirPathFromMetadata(dir, gitMetadataPath) != "" {
			return dir, gitMetadataPath, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
		dir = parent
	}
}

func gitMetadataProbeForPath(path string) gitMetadataProbe {
	probe := gitMetadataProbe{Path: path, Kind: "missing"}
	info, err := os.Stat(path)
	if err != nil {
		return probe
	}
	if info.IsDir() {
		probe.Kind = "dir"
		return probe
	}

	probe.Kind = "file"
	raw, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		return probe
	}
	probe.Value = strings.TrimSpace(string(raw))
	return probe
}

func gitDirPathFromMetadata(normalizedRoot, gitMetadataPath string) string {
	info, err := os.Stat(gitMetadataPath)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return filepath.Clean(gitMetadataPath)
	}

	raw, err := os.ReadFile(gitMetadataPath) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(line, "gitdir:") {
		return ""
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if gitDir == "" {
		return ""
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(normalizedRoot, gitDir)
	}
	return filepath.Clean(gitDir)
}

func gitCommonDirProbeMatches(a, b gitCommonDirProbe) bool {
	return a == b
}

func gitWorktreeRoot(root string) string {
	normalizedRoot, err := NormalizePath(root)
	if err != nil {
		normalizedRoot = filepath.Clean(root)
	}

	worktreeRoot, err := gitWorkspaceRoot(normalizedRoot)
	if err != nil {
		return normalizedRoot
	}
	return worktreeRoot
}

func scopeRelativePath(root string) string {
	normalizedRoot, err := NormalizePath(root)
	if err != nil {
		normalizedRoot = filepath.Clean(root)
	}

	worktreeRoot := gitWorktreeRoot(normalizedRoot)
	rel, err := filepath.Rel(worktreeRoot, normalizedRoot)
	if err != nil {
		return ""
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return ""
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return rel
}

func scopeRootInWorkspace(scopeRoot, workspaceRoot string) (string, error) {
	normalizedWorkspace, err := NormalizePath(workspaceRoot)
	if err != nil {
		return "", err
	}
	scopeRel := scopeRelativePath(scopeRoot)
	if scopeRel == "" {
		return normalizedWorkspace, nil
	}
	return NormalizePath(filepath.Join(normalizedWorkspace, scopeRel))
}

// EnsureScopeMarker registers the scope root in the shared git-common slipway namespace.
func EnsureScopeMarker(root string) error {
	return ensureScopeMarkerFile(ScopeMarkerPath(root))
}

func ensureScopeMarkerFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	return os.WriteFile(path, []byte("slipway scope marker\n"), 0o644) // #nosec G306 -- file is a user-facing project or governance artifact where operator-readable mode is intentional.
}

// GitRuntimeDir returns the git-internal directory for local runtime-only state.
func GitRuntimeDir(root string) string {
	return filepath.Join(GitStateDir(root), "runtime")
}

// ChangeHandoffPath returns the per-change advisory session handoff path.
func ChangeHandoffPath(root, slug string) string {
	return filepath.Join(ChangeDir(root, slug), "handoff.md")
}

// lockDir returns the git-internal directory for local lock files.
func lockDir(root string) string {
	return filepath.Join(GitStateDir(root), "locks")
}

// ChangeStateLockPath returns the per-change lock file path.
func ChangeStateLockPath(root, slug string) string {
	return filepath.Join(lockDir(root), "changes", slug+".lock")
}

// ChangeCreateLockPath returns the global create lock path.
func ChangeCreateLockPath(root string) string {
	return filepath.Join(lockDir(root), "change-create.lock")
}

// RepairLockPath returns the workspace repair lock path.
func RepairLockPath(root string) string {
	return filepath.Join(lockDir(root), "repair.lock")
}

// TaskPIDFilePath returns the git-internal task PID registry path.
func TaskPIDFilePath(root, slug string) string {
	return filepath.Join(GitStateDir(root), "processes", slug, "task_pids.json")
}

// GovernanceSnapshotCachePath returns the git-internal governance cache path.
func GovernanceSnapshotCachePath(root, slug string) string {
	return filepath.Join(GitStateDir(root), "cache", "changes", slug, "governance_snapshot.yaml")
}

// ConfigBackupDir returns the git-internal config backup directory used by repair.
func ConfigBackupDir(root string) string {
	return filepath.Join(GitStateDir(root), "repair-backups", "config")
}
