package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductionDependenciesFollowSoftAutopilotArchitecture(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	allowedInternal := map[string]map[string]bool{
		"cmd":         {"adapter": true, "autopilot": true, "recoverycmd": true},
		"autopilot":   {"runstore": true, "jsonstrict": true},
		"runstore":    {"fsutil": true, "jsonstrict": true},
		"adapter":     {"fsutil": true, "tmpl": true, "jsonstrict": true},
		"recoverycmd": {},
		"tmpl":        {},
		"fsutil":      {},
		"jsonstrict":  {},
	}
	for source, allowed := range allowedInternal {
		directory := filepath.Join(root, source)
		if source != "cmd" {
			directory = filepath.Join(root, "internal", source)
		}
		entries, err := os.ReadDir(directory)
		require.NoError(t, err)
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			require.NoError(t, err)
			for _, spec := range file.Imports {
				importPath, err := strconv.Unquote(spec.Path.Value)
				require.NoError(t, err)
				const prefix = "github.com/signalridge/slipway/internal/"
				if !strings.HasPrefix(importPath, prefix) {
					continue
				}
				target := strings.Split(strings.TrimPrefix(importPath, prefix), "/")[0]
				assert.True(t, allowed[target], "%s must not import internal/%s", path, target)
			}
		}
	}
}

func TestRetiredArchitecturePackagesAreAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, name := range []string{"engine", "model", "state", "freshness", "wave", "bootstrap", "perfbaseline", "coverage", "covergate", "toolgen"} {
		_, err := os.Stat(filepath.Join(root, "internal", name))
		assert.ErrorIs(t, err, os.ErrNotExist, name)
	}
}

// TestRuntimeAndGeneratedCapabilitiesExcludeRetiredGovernanceSurfaces enforces
// issue #434 §15/§9.4 structurally: retired governance concepts (evidence,
// gates, freshness, done-ready/ship-ready, lifecycle) must not reappear as a
// public CLI command or as a machine-protocol JSON field/enum value. This is a
// structural surface check, not a vocabulary ban: ordinary English usage of
// these words in comments, error prose, or documentation must not fail CI, so
// the assertion targets exact command/field shapes instead of grepping prose.
func TestRuntimeAndGeneratedCapabilitiesExcludeRetiredGovernanceSurfaces(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// The retired concepts and the substrings that would betray their return
	// as machine fields or enum values. We match on quoted JSON-ish tokens and
	// CLI command names rather than bare words so legitimate prose is unaffected.
	retiredField := regexp.MustCompile(`"(?:evidence|gate|gates|freshness|done_ready|ship_ready|done_ready_gate|ship_ready_gate|lifecycle)"`)
	retiredCommand := regexp.MustCompile(`(?m)^\s*Use:\s+"(?:evidence|gate|freshness|lifecycle|done-ready|ship-ready)"`)

	targets := []string{
		filepath.Join(root, "cmd"),
		filepath.Join(root, "internal", "adapter"),
		filepath.Join(root, "internal", "autopilot"),
		filepath.Join(root, "internal", "runstore"),
		filepath.Join(root, "internal", "tmpl", "templates"),
	}
	for _, target := range targets {
		err := filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if filepath.Ext(path) != ".go" && filepath.Ext(path) != ".md" && filepath.Ext(path) != ".tmpl" {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			assert.Emptyf(t, retiredField.Find(content), "%s: retired governance term reappeared as a quoted JSON field/enum token", path)
			assert.Emptyf(t, retiredCommand.Find(content), "%s: retired governance term reappeared as a cobra command Use", path)
			return nil
		})
		require.NoError(t, err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}
