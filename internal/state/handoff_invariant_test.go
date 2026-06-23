package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandoffIsNotGateEvidenceOrFreshnessInput(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	scanRoots := []string{
		filepath.Join(repoRoot, "internal", "engine"),
		filepath.Join(repoRoot, "internal", "model"),
		filepath.Join(repoRoot, "internal", "state"),
	}
	allowed := map[string]struct{}{
		filepath.Join(repoRoot, "internal", "state", "handoff.go"):                  {},
		filepath.Join(repoRoot, "internal", "state", "handoff_test.go"):             {},
		filepath.Join(repoRoot, "internal", "state", "handoff_invariant_test.go"):   {},
		filepath.Join(repoRoot, "internal", "state", "health.go"):                   {},
		filepath.Join(repoRoot, "internal", "state", "health_test.go"):              {},
		filepath.Join(repoRoot, "internal", "state", "local_runtime_paths.go"):      {},
		filepath.Join(repoRoot, "internal", "state", "local_runtime_paths_test.go"): {},
	}

	var offenders []string
	for _, root := range scanRoots {
		require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			require.NoError(t, err)
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if _, ok := allowed[path]; ok {
				return nil
			}
			raw, readErr := os.ReadFile(path) // #nosec G304 -- test scans repository source files.
			require.NoError(t, readErr)
			content := string(raw)
			if strings.Contains(content, "ChangeHandoffPath") ||
				strings.Contains(content, "runtime/changes/<slug>/handoff.md") ||
				strings.Contains(content, "/handoff.md") {
				offenders = append(offenders, filepath.ToSlash(path))
			}
			return nil
		}))
	}
	assert.Empty(t, offenders, "handoff must not become lifecycle gate, evidence, or freshness input")
}
