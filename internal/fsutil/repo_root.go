package fsutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRepository identifies the active worktree and its Git metadata locations.
// CommonDir is shared by linked worktrees; GitDir belongs to this worktree.
type GitRepository struct {
	WorktreeRoot string `json:"worktree_root"`
	GitDir       string `json:"git_dir"`
	CommonDir    string `json:"common_dir"`
}

// DiscoverGit resolves start to the containing Git worktree. It never falls
// back to start when Git metadata cannot be discovered.
func DiscoverGit(start string) (GitRepository, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return GitRepository{}, fmt.Errorf("get working directory: %w", err)
		}
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return GitRepository{}, fmt.Errorf("absolute repository path: %w", err)
	}
	if info, statErr := os.Stat(abs); statErr != nil {
		return GitRepository{}, fmt.Errorf("inspect repository path: %w", statErr)
	} else if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return GitRepository{}, fmt.Errorf("resolve repository path: %w", err)
	}

	queries := [][]string{
		{"rev-parse", "--path-format=absolute", "--show-toplevel"},
		{"rev-parse", "--path-format=absolute", "--git-dir"},
		{"rev-parse", "--path-format=absolute", "--git-common-dir"},
	}
	paths := make([]string, len(queries))
	for i, query := range queries {
		output, runErr := runGit(abs, query...)
		if runErr != nil {
			return GitRepository{}, runErr
		}
		path := trimGitOutputTerminator(output)
		if path == "" {
			return GitRepository{}, fmt.Errorf("git %s returned an empty path", strings.Join(query, " "))
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(abs, path)
		}
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			return GitRepository{}, fmt.Errorf("resolve git path %q: %w", path, resolveErr)
		}
		info, statErr := os.Stat(resolved)
		if statErr != nil {
			return GitRepository{}, fmt.Errorf("inspect git path %q: %w", resolved, statErr)
		}
		if !info.IsDir() {
			return GitRepository{}, fmt.Errorf("git path %q is not a directory", resolved)
		}
		paths[i] = filepath.Clean(resolved)
	}
	return GitRepository{WorktreeRoot: paths[0], GitDir: paths[1], CommonDir: paths[2]}, nil
}

func trimGitOutputTerminator(output string) string {
	if strings.HasSuffix(output, "\n") {
		output = strings.TrimSuffix(output, "\n")
		output = strings.TrimSuffix(output, "\r")
	}
	return output
}

func runGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...) // #nosec G204 -- fixed git executable with internal rev-parse arguments; no shell interpretation.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return stdout.String(), nil
}
