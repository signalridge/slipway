package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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
		"autopilot":   {"runstore": true, "fsutil": true, "jsonstrict": true},
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
		// Walk nested directories too: a sub-package inherits its parent's
		// allowance, so scanning only top-level files would let a wrapper
		// package under the source tree import a forbidden target unseen.
		require.NoError(t, filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, spec := range file.Imports {
				importPath, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					return err
				}
				const prefix = "github.com/signalridge/slipway/internal/"
				if !strings.HasPrefix(importPath, prefix) {
					continue
				}
				target := strings.Split(strings.TrimPrefix(importPath, prefix), "/")[0]
				if target == source {
					continue
				}
				assert.True(t, allowed[target], "%s must not import internal/%s", path, target)
			}
			return nil
		}))
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}
