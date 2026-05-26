package fsutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrProjectRootNotFound = errors.New("project root not found")

const ProjectConfigFileName = ".slipway.yaml"

const (
	slipwayGitDirName = "slipway"
	scopeMarkerName   = "scope-root"
	scopesDirName     = "scopes"
)

// ResolveCanonicalScopeRoot maps a path inside a git worktree to its canonical
// slipway scope root in the main repository checkout. Unlike FindProjectRoot,
// it does not require slipway markers or config to exist yet, so it is safe to
// use during initialization.
func ResolveCanonicalScopeRoot(start string) (string, error) {
	if start == "" {
		start = "."
	}

	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(current); err == nil {
		current = resolved
	}

	info, err := os.Stat(current)
	if err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}

	workspaceInfo, err := resolveGitWorkspaceInfo(current)
	if err != nil {
		if gitCommandReportsNotRepository(err) {
			return "", fmt.Errorf(
				"%w: slipway requires a git repository; %q is not inside a git working tree",
				ErrProjectRootNotFound,
				start,
			)
		}
		return "", fmt.Errorf("%w: resolve git workspace info from %q: %v", ErrProjectRootNotFound, start, err)
	}
	mainRoot := filepath.Dir(workspaceInfo.commonDir)
	if resolved, err := filepath.EvalSymlinks(mainRoot); err == nil {
		mainRoot = resolved
	}

	scopeRel := scopeRelativePathWithinWorkspace(workspaceInfo.worktreeRoot, current)
	return canonicalScopeRoot(mainRoot, scopeRel), nil
}

// FindProjectRoot resolves the canonical slipway scope root for the working
// directory. Explicit nested scopes are registered via git-internal scope
// markers; without a closer scope marker, the canonical main repository root is
// returned.
func FindProjectRoot(start string) (string, error) {
	if start == "" {
		start = "."
	}

	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(current); err == nil {
		current = resolved
	}

	info, err := os.Stat(current)
	if err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}

	workspaceInfo, err := resolveGitWorkspaceInfo(current)
	if err != nil {
		if gitCommandReportsNotRepository(err) {
			return "", fmt.Errorf(
				"%w: slipway requires a git repository; %q is not inside a git working tree",
				ErrProjectRootNotFound,
				start,
			)
		}
		return "", fmt.Errorf("%w: resolve git workspace info from %q: %v", ErrProjectRootNotFound, start, err)
	}
	mainRoot := filepath.Dir(workspaceInfo.commonDir)
	if resolved, err := filepath.EvalSymlinks(mainRoot); err == nil {
		mainRoot = resolved
	}

	for dir := current; ; {
		scopeRel := scopeRelativePathWithinWorkspace(workspaceInfo.worktreeRoot, dir)
		if (markerExists(scopeMarkerPath(workspaceInfo.localGitDir, scopeRel)) || markerExists(scopeMarkerPath(workspaceInfo.commonDir, scopeRel))) &&
			configExists(canonicalScopeRoot(mainRoot, scopeRel)) {
			return canonicalScopeRoot(mainRoot, scopeRel), nil
		}
		if samePath(dir, workspaceInfo.worktreeRoot) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return mainRoot, nil
}

type gitWorkspaceInfo struct {
	worktreeRoot string
	commonDir    string
	localGitDir  string
}

func resolveGitWorkspaceInfo(dir string) (gitWorkspaceInfo, error) {
	lines, err := runGitRevParseLines(
		dir,
		"resolve git workspace info",
		"--path-format=absolute",
		"--show-toplevel",
		"--git-common-dir",
		"--git-dir",
	)
	if err != nil {
		return gitWorkspaceInfo{}, err
	}
	if len(lines) != 3 {
		return gitWorkspaceInfo{}, fmt.Errorf("expected 3 git workspace fields, got %d", len(lines))
	}

	worktreeRoot, err := normalizeGitPath(dir, lines[0], "git workspace root")
	if err != nil {
		return gitWorkspaceInfo{}, err
	}
	commonDir, err := normalizeGitPath(dir, lines[1], "git-common-dir")
	if err != nil {
		return gitWorkspaceInfo{}, err
	}
	localGitDir, err := normalizeGitPath(dir, lines[2], "git-dir")
	if err != nil {
		return gitWorkspaceInfo{}, err
	}

	return gitWorkspaceInfo{
		worktreeRoot: worktreeRoot,
		commonDir:    commonDir,
		localGitDir:  localGitDir,
	}, nil
}

func normalizeGitPath(baseDir, value, label string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("empty %s", label)
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(baseDir, value)
	}
	value = filepath.Clean(value)
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		value = resolved
	}
	return value, nil
}

func runGitRevParse(dir, label string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", dir, "rev-parse"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			return "", fmt.Errorf("%s: %w", label, err)
		}
		return "", fmt.Errorf("%s: %w (%s)", label, err, message)
	}
	return strings.TrimSpace(string(out)), nil
}

func runGitRevParseLines(dir, label string, args ...string) ([]string, error) {
	output, err := runGitRevParse(dir, label, args...)
	if err != nil {
		return nil, err
	}
	lines := []string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func gitCommandReportsNotRepository(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not a git repository")
}

func scopeRelativePathWithinWorkspace(worktreeRoot, dir string) string {
	rel, err := filepath.Rel(worktreeRoot, dir)
	if err != nil {
		return ""
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" {
		return ""
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return rel
}

func scopeMarkerPath(gitDir, scopeRel string) string {
	base := filepath.Join(gitDir, slipwayGitDirName)
	if scopeRel != "" {
		base = filepath.Join(base, scopesDirName, scopeRel)
	}
	return filepath.Join(base, scopeMarkerName)
}

func canonicalScopeRoot(mainRoot, scopeRel string) string {
	if scopeRel == "" {
		return mainRoot
	}
	return filepath.Clean(filepath.Join(mainRoot, scopeRel))
}

func markerExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func configExists(root string) bool {
	info, err := os.Stat(filepath.Join(root, ProjectConfigFileName))
	return err == nil && !info.IsDir()
}

// samePath compares already-normalized workspace paths. Callers are expected to
// pass values that have already gone through filepath.EvalSymlinks or
// NormalizePath.
func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
