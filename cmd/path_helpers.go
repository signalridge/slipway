package cmd

import (
	"path/filepath"
	"strings"
)

func resolveInputContextPath(projectRoot, workspaceRoot, target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if filepath.IsAbs(target) || strings.HasPrefix(target, "/") || strings.HasPrefix(target, `\`) {
		return filepath.ToSlash(filepath.Clean(target))
	}
	base := strings.TrimSpace(workspaceRoot)
	if base == "" {
		base = projectRoot
	}
	return filepath.ToSlash(filepath.Join(base, target))
}
