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
	if filepath.IsAbs(target) {
		return target
	}
	base := strings.TrimSpace(workspaceRoot)
	if base == "" {
		base = projectRoot
	}
	return filepath.Join(base, target)
}
