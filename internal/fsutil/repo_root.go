package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FindRepoRoot walks upward from start until it finds the Go module root.
// An empty start preserves the tiny-command behavior of starting at the
// process working directory.
func FindRepoRoot(start string) (string, error) {
	dir := start
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	} else {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", fmt.Errorf("resolve start path: %w", err)
		}
		dir = abs
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find repository root containing go.mod")
		}
		dir = parent
	}
}
