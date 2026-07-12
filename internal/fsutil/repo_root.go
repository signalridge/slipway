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
	if strings.TrimSpace(start) == "" {
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

	output, err := runGit(abs, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-dir", "--git-common-dir")
	if err != nil {
		return GitRepository{}, err
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		return GitRepository{}, fmt.Errorf("git rev-parse returned %d paths, want 3", len(lines))
	}
	for i := range lines {
		lines[i] = filepath.Clean(strings.TrimSpace(lines[i]))
		if !filepath.IsAbs(lines[i]) {
			lines[i] = filepath.Join(abs, lines[i])
		}
		resolved, resolveErr := filepath.EvalSymlinks(lines[i])
		if resolveErr == nil {
			lines[i] = resolved
		}
	}
	return GitRepository{WorktreeRoot: lines[0], GitDir: lines[1], CommonDir: lines[2]}, nil
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
