package toolgen

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
)

var createSymlink = os.Symlink

// ProvisionWorktreeHostSurfaces materializes every host-adapter surface that
// exists in the repository root into a freshly created (or reused) worktree.
//
// Git worktrees share only the .git directory; the host-adapter trees
// (.claude, .cursor, .codex, .opencode, .gemini) are gitignored, so a new
// worktree starts with none of the skills, hooks, settings, or bundled skill
// references that an agent — including an isolated subagent — needs. This helper
// copies the third-party content over (skip-if-exists so worktree-local edits
// are never clobbered) and then regenerates the slipway-owned surfaces with the
// running Slipway binary's embedded templates via GenerateWorktreeLocal, so the
// worktree carries freshly generated slipway-* surfaces rather than a stale copy
// of the source adapter's generated output. Host-global cleanup, such as pruning
// retired Codex prompts under $CODEX_HOME / ~/.codex, is deliberately skipped:
// that state is shared across every checkout and must not be mutated when
// provisioning one worktree.
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
		if err := copyHostAdapterTree(cfg, src, dst); err != nil {
			return fmt.Errorf("provision host-adapter surface %s into worktree: %w", toolRel, err)
		}
		present = append(present, cfg.ID)
	}
	if len(present) == 0 {
		return nil
	}
	if err := GenerateWorktreeLocal(worktreeRoot, present, true); err != nil {
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
//   - lock files (e.g. scheduled_tasks.lock), which are host-process state,
//   - the generated ".adapter-generated" sentinel, since Generate writes a fresh
//     one for the worktree, and
//   - slipway-owned skill directories, which are regenerated from the running
//     binary; copying them risks carrying a stale managed skill the generator no
//     longer produces, which would never be pruned on a first-time provision
//     (where the sentinel the prune relies on is absent).
func copyHostAdapterTree(cfg ToolConfig, srcRoot, dstRoot string) error {
	skillsRel := adapterSkillsRel(cfg)
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
		if excludeFromHostAdapterCopy(rel, d, skillsRel) {
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
func excludeFromHostAdapterCopy(rel string, d fs.DirEntry, skillsRel string) bool {
	if rel == "worktrees" || strings.HasPrefix(rel, "worktrees"+string(filepath.Separator)) {
		return true
	}
	if d.IsDir() && isSlipwayOwnedSkillDir(rel, skillsRel) {
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

// adapterSkillsRel returns the adapter's skills directory relative to its tool
// root (e.g. ".claude/skills" under ".claude" -> "skills"). It returns "" when
// the skills directory is absent or not under the tool root, which disables
// slipway-owned skill-dir exclusion for that adapter.
func adapterSkillsRel(cfg ToolConfig) string {
	toolRoot := ToolRootPath(cfg)
	if toolRoot == "" || cfg.SkillsDir == "" {
		return ""
	}
	rel, err := filepath.Rel(toolRoot, cfg.SkillsDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return rel
}

// isSlipwayOwnedSkillDir reports whether rel is a slipway-owned skill directory
// directly under the adapter's skills root. These are regenerated, never copied.
// The match is by naming convention (the reserved "slipway-" prefix and the
// workflow entry skill) so it also catches stale managed skills the current
// generator no longer emits.
func isSlipwayOwnedSkillDir(rel, skillsRel string) bool {
	if skillsRel == "" {
		return false
	}
	parent, name := filepath.Split(filepath.Clean(rel))
	if filepath.Clean(parent) != filepath.Clean(skillsRel) {
		return false
	}
	return strings.HasPrefix(name, "slipway-") || name == workflowEntryPublicName
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
		if symErr := createSymlink(target, dstPath); symErr != nil {
			// On Windows, os.Symlink fails without SeCreateSymbolicLinkPrivilege
			// (Developer Mode). Rather than abort the whole copy, dereference the
			// link and materialize the target's content at the destination.
			return dereferenceSymlinkCopy(srcPath, dstPath, target, symErr)
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return nil // skip sockets, devices, and other irregular files
	}
	data, err := os.ReadFile(srcPath) // #nosec G304 -- srcPath is a host-adapter surface under the repository root being provisioned.
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, info.Mode().Perm()) // #nosec G306 G703 -- preserves the source mode so adapter hooks remain executable; dstPath is filepath.Join(worktreeRoot, toolRel, rel) where rel comes from filepath.Rel against the walked source tree, so the write stays within the worktree's host-adapter surface.
}

// dereferenceSymlinkCopy materializes a symlink target's content at dstPath when
// os.Symlink fails (e.g. Windows without SeCreateSymbolicLinkPrivilege). Host
// adapter trees commonly contain third-party skill symlinks that point outside
// the repository, so the top-level target is allowed to resolve outside srcRoot.
// Nested symlinks inside a dereferenced directory are constrained to that
// directory's real root so one accepted skill link cannot pull in arbitrary
// additional filesystem content.
func dereferenceSymlinkCopy(srcPath, dstPath, linkTarget string, symErr error) error {
	resolved, info, statErr := fsutil.ResolveSymlinkTarget(srcPath, linkTarget)
	if statErr != nil {
		// Dangling/broken link or unreadable: preserve the original failure.
		return symErr
	}
	if info.IsDir() {
		if fsutil.SymlinkTargetContainsLink(resolved, srcPath) {
			return fmt.Errorf("refuse dereference symlink %q into %q: target directory %q contains the link", srcPath, dstPath, resolved)
		}
		if err := copyDereferencedDir(resolved, dstPath, resolved); err != nil {
			return fmt.Errorf("dereference symlink %q into %q: %w", srcPath, dstPath, err)
		}
		return nil
	}
	data, err := os.ReadFile(resolved) // #nosec G304 -- resolved is the explicit target of a host-adapter symlink being materialized.
	if err != nil {
		return fmt.Errorf("dereference symlink %q into %q: %w", srcPath, dstPath, err)
	}
	if err := os.WriteFile(dstPath, data, info.Mode().Perm()); err != nil { // #nosec G306 -- preserves the dereferenced target's mode; dstPath stays within the worktree's host-adapter surface.
		return fmt.Errorf("dereference symlink %q into %q: %w", srcPath, dstPath, err)
	}
	return nil
}

// copyDereferencedDir recursively copies the real contents of a directory that a
// symlink pointed at into dstRoot, preserving file modes. Nested symlinks are
// materialized only when their targets resolve within allowedRoot.
func copyDereferencedDir(srcRoot, dstRoot, allowedRoot string) error {
	return copyDereferencedDirWithin(srcRoot, dstRoot, allowedRoot, make(map[string]struct{}))
}

func copyDereferencedDirWithin(srcRoot, dstRoot, allowedRoot string, active map[string]struct{}) error {
	realRoot, err := fsutil.RealExistingPath(srcRoot)
	if err != nil {
		return err
	}
	if _, ok := active[realRoot]; ok {
		return fmt.Errorf("refuse dereference directory %q into %q: symlink cycle through %q", srcRoot, dstRoot, realRoot)
	}
	active[realRoot] = struct{}{}
	defer delete(active, realRoot)

	return filepath.WalkDir(srcRoot, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, srcPath)
		if err != nil {
			return err
		}
		dstPath := dstRoot
		if rel != "." {
			dstPath = filepath.Join(dstRoot, rel)
		}
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755) // #nosec G301 -- host-adapter surfaces are user-facing skill/command trees where searchable mode is intentional.
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return dereferenceNestedSymlinkCopy(allowedRoot, srcPath, dstPath, active)
		}
		if !info.Mode().IsRegular() {
			return nil // skip sockets, devices, and other irregular files
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil { // #nosec G301 -- host-adapter surfaces are user-facing skill/command trees where searchable mode is intentional.
			return err
		}
		data, err := os.ReadFile(srcPath) // #nosec G304 -- srcPath is under the accepted dereferenced host-adapter symlink target.
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode().Perm()) // #nosec G306 -- preserves the source mode; dstPath stays within the worktree's host-adapter surface.
	})
}

func dereferenceNestedSymlinkCopy(allowedRoot, srcPath, dstPath string, active map[string]struct{}) error {
	linkTarget, err := os.Readlink(srcPath)
	if err != nil {
		return err
	}
	resolved, info, err := fsutil.ResolveSymlinkTargetWithin(allowedRoot, srcPath, linkTarget)
	if err != nil {
		if errors.Is(err, fsutil.ErrSymlinkTargetOutsideRoot) {
			return fmt.Errorf("refuse dereference nested symlink %q into %q: %w", srcPath, dstPath, err)
		}
		return fmt.Errorf("dereference nested symlink %q into %q: %w", srcPath, dstPath, err)
	}
	if info.IsDir() {
		if fsutil.SymlinkTargetContainsLink(resolved, srcPath) {
			return fmt.Errorf("refuse dereference nested symlink %q into %q: target directory %q contains the link", srcPath, dstPath, resolved)
		}
		if err := copyDereferencedDirWithin(resolved, dstPath, allowedRoot, active); err != nil {
			return fmt.Errorf("dereference nested symlink %q into %q: %w", srcPath, dstPath, err)
		}
		return nil
	}
	data, err := os.ReadFile(resolved) // #nosec G304 -- resolved is bounded to the accepted dereferenced directory root.
	if err != nil {
		return fmt.Errorf("dereference nested symlink %q into %q: %w", srcPath, dstPath, err)
	}
	if err := os.WriteFile(dstPath, data, info.Mode().Perm()); err != nil { // #nosec G306 -- preserves the dereferenced target's mode; dstPath stays within the worktree's host-adapter surface.
		return fmt.Errorf("dereference nested symlink %q into %q: %w", srcPath, dstPath, err)
	}
	return nil
}
