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
// are never clobbered) and then regenerates the slipway-owned surfaces with the
// running Slipway binary's embedded templates via GenerateWorktreeLocal, so the
// worktree carries freshly generated slipway-* surfaces rather than a stale copy
// of the source adapter's generated output. Host-global outputs (Codex prompts
// under $CODEX_HOME / ~/.codex) are deliberately skipped: they are shared across
// every checkout and must not be rewritten when provisioning one worktree.
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
