package state

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

// RelocateGovernedBundle moves a change bundle when its canonical bundle root changes.
func RelocateGovernedBundle(root string, fromChange, toChange model.Change) error {
	fromPaths, err := ResolveChangePaths(root, fromChange)
	if err != nil {
		return err
	}
	toPaths, err := ResolveChangePaths(root, toChange)
	if err != nil {
		return err
	}

	fromDir := filepath.Clean(fromPaths.GovernedBundleDir)
	toDir := filepath.Clean(toPaths.GovernedBundleDir)
	if fromDir == toDir {
		return nil
	}

	if _, err := os.Stat(fromDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	if info, err := os.Stat(toDir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("governed bundle target exists and is not a directory: %s", toDir)
		}
		entries, readErr := os.ReadDir(toDir)
		if readErr != nil {
			return readErr
		}
		if len(entries) > 0 {
			return fmt.Errorf("governed bundle target already exists: %s", toDir)
		}
		if err := os.Remove(toDir); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if strings.TrimSpace(toChange.WorktreePath) != "" {
		if err := EnsureWorkspaceScopeMarker(root, toChange.WorktreePath); err != nil {
			return err
		}
		if err := EnsureWorkspaceScopeConfig(root, toChange.WorktreePath); err != nil {
			return err
		}
	}

	if err := moveDirIfExists(fromDir, toDir); err != nil {
		return err
	}
	return SaveChange(root, toChange)
}
