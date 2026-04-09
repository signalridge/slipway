package bootstrap

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/toolgen"
)

func InitWorkspace(root string, tools []string, refresh bool) error {
	if err := requireGitRepository(root); err != nil {
		return err
	}

	workspaceRoot, err := state.NormalizePath(root)
	if err != nil {
		workspaceRoot = filepath.Clean(root)
	}
	scopeRoot, err := fsutil.ResolveCanonicalScopeRoot(workspaceRoot)
	if err != nil {
		return err
	}

	configPath := state.ConfigPath(scopeRoot)
	if _, err := os.Stat(configPath); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := model.SaveConfig(configPath, model.DefaultConfig()); err != nil {
			return err
		}
	}
	if err := state.EnsureScopeMarker(scopeRoot); err != nil {
		return err
	}
	if err := ensureGitRuntimeMarker(scopeRoot); err != nil {
		return err
	}
	if err := ensureWorkspaceScopeVisibility(scopeRoot, workspaceRoot, refresh); err != nil {
		return err
	}

	// When --refresh is set and no --tools specified, auto-detect existing
	// adapter directories so the user doesn't need to re-specify tools.
	if len(tools) == 0 && refresh {
		tools = toolgen.DetectExistingTools(workspaceRoot)
	}

	if len(tools) == 0 {
		return nil
	}
	return toolgen.Generate(workspaceRoot, tools, refresh)
}

func requireGitRepository(root string) error {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("slipway requires a git repository: %s is not inside a git working tree", root)
	}
	return nil
}

func ensureGitRuntimeMarker(root string) error {
	return os.MkdirAll(state.GitRuntimeDir(root), 0o755)
}

func ensureWorkspaceScopeVisibility(scopeRoot, workspaceRoot string, refresh bool) error {
	if scopeRoot == "" || workspaceRoot == "" || scopeRoot == workspaceRoot {
		return nil
	}

	sourcePath := state.ConfigPath(scopeRoot)
	targetPath := state.ConfigPath(workspaceRoot)
	if refresh || !fileExists(targetPath) {
		raw, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		if err := fsutil.WriteFileAtomic(targetPath, raw, 0o644); err != nil {
			return err
		}
	}

	markerPath := state.WorkspaceScopeMarkerPath(workspaceRoot)
	if fileExists(markerPath) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(markerPath, []byte("slipway scope marker\n"), 0o644)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
