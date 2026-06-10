package bootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/signalridge/slipway/internal/toolgen"
)

func InitWorkspace(root string, tools []string, refresh bool, toolsSpecified ...bool) error {
	explicitTools := len(toolsSpecified) > 0 && toolsSpecified[0]
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
	if _, err := state.EnsureLocalStateGitIgnore(scopeRoot); err != nil {
		return err
	}

	// When --refresh is set and --tools was not explicitly provided, auto-detect
	// existing sentinelized adapters.
	if len(tools) == 0 && refresh && !explicitTools {
		detectedTools := toolgen.DetectExistingTools(workspaceRoot)
		if len(detectedTools) == 0 {
			return newInitUsageError(
				"init_refresh_no_sentinelized_tools",
				"no sentinelized adapters detected; rerun with --tools <tool> --refresh or --tools all --refresh",
				"Rerun with `slipway init --tools <tool> --refresh` or `slipway init --tools all --refresh`.",
				map[string]any{
					"tool":           "",
					"refresh":        refresh,
					"workspace_root": workspaceRoot,
					"detected_tools": []string{},
				},
			)
		}
		tools = detectedTools
	}

	// Non-refresh: check for pre-sentinel dirty tree.
	if !refresh && len(tools) > 0 {
		for _, toolID := range tools {
			cfg, ok := toolgen.LookupTool(toolID)
			if !ok {
				continue
			}
			if toolgen.HasWorkspaceLocalSurfaces(workspaceRoot, cfg) && !toolgen.HasSentinel(workspaceRoot, cfg) {
				return newInitUsageError(
					"init_missing_sentinel_existing_tree",
					fmt.Sprintf("workspace has existing Slipway surfaces for %s without a sentinel; rerun with --tools %s --refresh", toolID, toolID),
					fmt.Sprintf("Rerun with `slipway init --tools %s --refresh` to regenerate the workspace-local adapter tree.", toolID),
					map[string]any{
						"tool":           toolID,
						"refresh":        refresh,
						"workspace_root": workspaceRoot,
					},
				)
			}
		}
	}

	if len(tools) == 0 {
		return nil
	}
	return toolgen.Generate(workspaceRoot, tools, refresh)
}

func requireGitRepository(root string) error {
	if _, err := state.ResolveGitWorkspaceRoot(root); err != nil {
		return fmt.Errorf("slipway requires a git repository: %s is not inside a git working tree", root)
	}
	return nil
}

func ensureGitRuntimeMarker(root string) error {
	return os.MkdirAll(state.GitRuntimeDir(root), 0o755) // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
}

func ensureWorkspaceScopeVisibility(scopeRoot, workspaceRoot string, refresh bool) error {
	if scopeRoot == "" || workspaceRoot == "" || scopeRoot == workspaceRoot {
		return nil
	}

	sourcePath := state.ConfigPath(scopeRoot)
	targetPath := state.ConfigPath(workspaceRoot)
	sourceRaw, err := os.ReadFile(sourcePath) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		return err
	}
	targetRaw, err := os.ReadFile(targetPath) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	switch {
	case err == nil:
		if refresh || !bytes.Equal(targetRaw, sourceRaw) {
			if err := fsutil.WriteFileAtomic(targetPath, sourceRaw, 0o644); err != nil {
				return err
			}
		}
	case errors.Is(err, fs.ErrNotExist):
		if err := fsutil.WriteFileAtomic(targetPath, sourceRaw, 0o644); err != nil {
			return err
		}
	default:
		return err
	}

	markerPath := state.WorkspaceScopeMarkerPath(workspaceRoot)
	if fileExists(markerPath) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	return os.WriteFile(markerPath, []byte("slipway scope marker\n"), 0o644) // #nosec G306 -- file is a user-facing project or governance artifact where operator-readable mode is intentional.
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
