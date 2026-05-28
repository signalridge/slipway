package state

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
)

const (
	localStateGitIgnoreStart = "# Slipway local state (managed)"
	localStateGitIgnoreEnd   = "# End Slipway local state"
)

var localStateGitIgnorePatterns = []string{
	"/artifacts/codebase/",
	"/artifacts/changes/**/evidence/",
	"/artifacts/changes/**/events/",
	"/artifacts/changes/**/verification/",
	"/.worktrees/",
}

type GitIgnoreUpdate struct {
	Path    string
	Changed bool
}

func LocalStateGitIgnoreBlock() string {
	var b strings.Builder
	b.WriteString(localStateGitIgnoreStart)
	b.WriteByte('\n')
	for _, pattern := range localStateGitIgnorePatterns {
		b.WriteString(pattern)
		b.WriteByte('\n')
	}
	b.WriteString(localStateGitIgnoreEnd)
	b.WriteByte('\n')
	return b.String()
}

func EnsureLocalStateGitIgnore(root string) (GitIgnoreUpdate, error) {
	workspaceRoot, err := NormalizePath(root)
	if err != nil {
		workspaceRoot = filepath.Clean(root)
	}
	path := filepath.Join(workspaceRoot, ".gitignore")
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return GitIgnoreUpdate{}, err
	}

	next, err := ensureLocalStateGitIgnoreBlock(string(current))
	if err != nil {
		return GitIgnoreUpdate{Path: path}, err
	}
	update := GitIgnoreUpdate{Path: path, Changed: next != string(current)}
	if !update.Changed {
		return update, nil
	}
	if err := fsutil.WriteFileAtomic(path, []byte(next), 0o644); err != nil {
		return GitIgnoreUpdate{}, err
	}
	return update, nil
}

func ensureLocalStateGitIgnoreBlock(current string) (string, error) {
	block := LocalStateGitIgnoreBlock()
	start := strings.Index(current, localStateGitIgnoreStart)
	if start >= 0 {
		end := strings.Index(current[start:], localStateGitIgnoreEnd)
		if end < 0 {
			return "", fmt.Errorf(
				".gitignore contains the Slipway start marker %q without a matching end marker %q; refusing to overwrite (run `slipway repair` to reset the managed block)",
				localStateGitIgnoreStart,
				localStateGitIgnoreEnd,
			)
		}
		end += start + len(localStateGitIgnoreEnd)
		if end < len(current) && current[end] == '\r' {
			end++
		}
		if end < len(current) && current[end] == '\n' {
			end++
		}
		return current[:start] + block + current[end:], nil
	}

	if strings.TrimSpace(current) == "" {
		return block, nil
	}
	if !strings.HasSuffix(current, "\n") {
		current += "\n"
	}
	return current + "\n" + block, nil
}
