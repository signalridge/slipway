package cmd

import "github.com/signalridge/slipway/internal/adapter"

func resolveRoot(explicit string) (string, error) {
	repositoryRoot, err := adapter.ResolveRepositoryRoot(explicit)
	if err != nil {
		return "", newUsageError("git_repository_required", err.Error(), defaultErrorNext())
	}
	return repositoryRoot, nil
}
