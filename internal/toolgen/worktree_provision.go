package toolgen

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ProvisionWorktreeHostSurfaces materializes every host-adapter surface that
// exists in the repository root into a freshly created (or reused) worktree.
//
// Git worktrees share only the .git directory; the host-adapter trees
// (.claude, .cursor, .codex, .opencode, .gemini) are gitignored, so a new
// worktree starts with none of the skills, hooks, settings, or bundled skill
// references that an agent — including an isolated subagent — needs. This helper
// copies the third-party content over (skip-if-exists so worktree-local edits
// are never clobbered) and then regenerates the slipway-owned surfaces from the
// worktree's own source via Generate, so a worktree whose source differs from
// main carries its own slipway-* surfaces rather than a stale copy of main's
// generated output.
//
// Detection is by directory existence rather than the generated sentinel: any
// adapter root present in the repository is provisioned, even one a user created
// without ever running init.
//
// It lives in the surface-renderer layer (toolgen) and is injected into the
// authority layer (internal/state) so that state never imports a surface
// renderer. Provisioning is fail-closed: any copy or regeneration error is
// returned so the caller can refuse to bind a silently degraded worktree.
func ProvisionWorktreeHostSurfaces(repoRoot, worktreeRoot string) error {
	var present []string
	for _, cfg := range Registry() {
		toolRel := ToolRootPath(cfg)
		if toolRel == "" {
			continue
		}
		src := filepath.Join(repoRoot, toolRel)
		info, err := os.Stat(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat host-adapter surface %s: %w", toolRel, err)
		}
		if !info.IsDir() {
			continue
		}
		dst := filepath.Join(worktreeRoot, toolRel)
		if err := copyHostAdapterTree(src, dst); err != nil {
			return fmt.Errorf("provision host-adapter surface %s into worktree: %w", toolRel, err)
		}
		present = append(present, cfg.ID)
	}
	if len(present) == 0 {
		return nil
	}
	if err := Generate(worktreeRoot, present, true); err != nil {
		return fmt.Errorf("regenerate slipway-owned worktree surfaces: %w", err)
	}
	return nil
}

// copyHostAdapterTree recursively copies a host-adapter directory into the
// worktree without clobbering files that already exist at the destination.
//
// It excludes paths that must not be carried verbatim into a worktree:
//   - the nested "worktrees/" subtree, which holds linked git worktrees and
//     would cause recursion and bloat,
//   - lock files (e.g. scheduled_tasks.lock), which are host-process state, and
//   - the generated ".adapter-generated" sentinel, since Generate writes a fresh
//     one for the worktree.
func copyHostAdapterTree(srcRoot, dstRoot string) error {
	return filepath.WalkDir(srcRoot, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, srcPath)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dstRoot, 0o755) // #nosec G301 -- host-adapter surfaces are user-facing skill/command trees where searchable mode is intentional.
		}
		if excludeFromHostAdapterCopy(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dstPath := filepath.Join(dstRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755) // #nosec G301 -- host-adapter surfaces are user-facing skill/command trees where searchable mode is intentional.
		}
		return copyHostAdapterFileNoClobber(srcPath, dstPath, d)
	})
}

// excludeFromHostAdapterCopy reports whether a path (relative to the tool root)
// must be skipped during the copy step.
func excludeFromHostAdapterCopy(rel string, d fs.DirEntry) bool {
	if rel == "worktrees" || strings.HasPrefix(rel, "worktrees"+string(filepath.Separator)) {
		return true
	}
	if !d.IsDir() {
		base := filepath.Base(rel)
		if strings.HasSuffix(base, ".lock") {
			return true
		}
		if base == ".adapter-generated" {
			return true
		}
	}
	return false
}

// copyHostAdapterFileNoClobber copies a single file into the worktree, preserving
// its mode bits (hooks must stay executable) and never overwriting an existing
// destination so worktree-local edits survive.
func copyHostAdapterFileNoClobber(srcPath, dstPath string, d fs.DirEntry) error {
	if _, err := os.Lstat(dstPath); err == nil {
		return nil // skip-if-exists: never clobber worktree-local edits
	} else if !os.IsNotExist(err) {
		return err
	}
	info, err := d.Info()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil { // #nosec G301 -- host-adapter surfaces are user-facing skill/command trees where searchable mode is intentional.
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(srcPath)
		if err != nil {
			return err
		}
		return os.Symlink(target, dstPath)
	}
	if !info.Mode().IsRegular() {
		return nil // skip sockets, devices, and other irregular files
	}
	data, err := os.ReadFile(srcPath) // #nosec G304 -- srcPath is a host-adapter surface under the repository root being provisioned.
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, info.Mode().Perm()) // #nosec G306 -- preserves the source mode so adapter hooks remain executable.
}
