package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestAuthorityPackagesDoNotImportSurfaceRenderers(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	authorityPackages := []string{
		"internal/model",
		"internal/state",
	}
	forbiddenImports := []string{
		"github.com/signalridge/slipway/cmd",
		"github.com/signalridge/slipway/internal/tmpl",
		"github.com/signalridge/slipway/internal/toolgen",
	}
	packageForbiddenImports := map[string][]string{
		"internal/state": {
			"github.com/signalridge/slipway/internal/engine",
		},
	}

	for _, relDir := range authorityPackages {
		relDir := relDir
		t.Run(relDir, func(t *testing.T) {
			t.Parallel()

			err := filepath.WalkDir(filepath.Join(root, relDir), func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					if strings.HasPrefix(entry.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}

				file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
				if err != nil {
					return err
				}
				effectiveForbiddenImports := append([]string(nil), forbiddenImports...)
				effectiveForbiddenImports = append(effectiveForbiddenImports, packageForbiddenImports[relDir]...)
				for _, imported := range file.Imports {
					importPath, err := strconv.Unquote(imported.Path.Value)
					if err != nil {
						return err
					}
					for _, forbidden := range effectiveForbiddenImports {
						if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
							t.Errorf("%s imports forbidden surface package %q", displayPath(root, path), importPath)
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func displayPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
