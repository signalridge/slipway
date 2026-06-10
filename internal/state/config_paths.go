package state

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/signalridge/slipway/internal/fsutil"
)

// ConfigPath returns the tracked repo-level slipway config path.
func ConfigPath(root string) string {
	return filepath.Join(root, fsutil.ProjectConfigFileName)
}

func scopeConfigExists(root string) bool {
	info, err := os.Stat(ConfigPath(root))
	return err == nil && !info.IsDir()
}

func scopeMarkerExists(root string) bool {
	info, err := os.Stat(WorkspaceScopeMarkerPath(root))
	return err == nil && !info.IsDir()
}

func workspaceScopeVisible(root string) bool {
	return scopeConfigExists(root) && scopeMarkerExists(root)
}

func EnsureWorkspaceScopeConfig(projectRoot, workspaceRoot string) error {
	targetRoot, err := scopeRootInWorkspace(projectRoot, workspaceRoot)
	if err != nil {
		return err
	}
	if scopeConfigExists(targetRoot) {
		return nil
	}

	sourcePath := ConfigPath(projectRoot)
	raw, err := os.ReadFile(sourcePath) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ConfigPath(targetRoot)), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	return fsutil.WriteFileAtomic(ConfigPath(targetRoot), raw, 0o644)
}

// EnsureWorkspaceScopeMarker seeds the current scope marker into a sibling
// worktree-local git namespace when the marker is missing there.
func EnsureWorkspaceScopeMarker(projectRoot, workspaceRoot string) error {
	targetRoot, err := scopeRootInWorkspace(projectRoot, workspaceRoot)
	if err != nil {
		return err
	}
	return ensureScopeMarkerFile(WorkspaceScopeMarkerPath(targetRoot))
}
