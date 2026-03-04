package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrProjectRootNotFound = errors.New("project root not found")

// FindProjectRoot searches upward for a directory that contains .spln/.
func FindProjectRoot(start string) (string, error) {
	if start == "" {
		start = "."
	}

	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(current)
	if err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}

	for {
		splnPath := filepath.Join(current, ".spln")
		if stat, err := os.Stat(splnPath); err == nil && stat.IsDir() {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("%w: no .spln found from %q", ErrProjectRootNotFound, start)
		}
		current = parent
	}
}
