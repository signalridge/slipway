package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/signalridge/slipway/internal/adapter"
)

func resolveRoot(explicit string) (string, error) {
	repositoryRoot, err := adapter.ResolveRepositoryRoot(explicit)
	if err != nil {
		return "", repositoryDiscoveryError(explicit, err)
	}
	return repositoryRoot, nil
}

func repositoryDiscoveryError(explicit string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		root := unresolvedRootHint(explicit)
		return newRuntimeError(
			"git_repository_unavailable",
			err.Error(),
			unavailableWorkspaceCommandNext(root, "retry-doctor", "slipway", "doctor", "--root", root, "--json"),
			nil,
		)
	}
	return newUsageError("git_repository_required", err.Error(), defaultErrorNext())
}

func unresolvedRootHint(explicit string) string {
	root := explicit
	if strings.TrimSpace(root) == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	if !utf8.ValidString(root) {
		return string(filepath.Separator)
	}
	if absolute, err := filepath.Abs(root); err == nil {
		return filepath.Clean(absolute)
	}
	return string(filepath.Separator)
}
