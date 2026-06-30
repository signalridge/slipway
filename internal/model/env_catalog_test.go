package model

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// envLiteral matches environment-variable names Slipway treats as public
// runtime/config surface: SLIPWAY_* names, ambient token fallbacks, and ambient
// username fallbacks. Scanning string literals (rather than only
// os.Getenv("...") call sites) catches vars read via const identifiers and
// simple fallback slices.
var envLiteral = regexp.MustCompile(`"(SLIPWAY_[A-Z0-9_]+|[A-Z][A-Z0-9_]*_TOKEN|GH_TOKEN|USER|USERNAME)"`)

// repoRootForTest resolves the repository root from this test's package
// directory (internal/model => ../..). Walking from here keeps the source scan
// deterministic regardless of the caller's working directory.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

// scanEnvUsages walks the repo's Go source (excluding _test.go files and
// env_catalog.go itself) and returns every distinct env name Slipway exposes as
// runtime/config surface. env_catalog.go is excluded so the catalog cannot
// trivially "cover itself": the test proves every use site elsewhere is
// documented.
func scanEnvUsages(t *testing.T, root string) map[string]string {
	t.Helper()
	skipDirs := map[string]bool{
		".git": true, ".worktrees": true, "artifacts": true,
		"node_modules": true, "vendor": true, "testdata": true,
	}
	found := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		if name == "env_catalog.go" {
			return nil
		}
		data, readErr := os.ReadFile(path) // #nosec G304 -- test-only scan of in-repo source.
		if readErr != nil {
			return readErr
		}
		for _, m := range envLiteral.FindAllStringSubmatch(string(data), -1) {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			if _, ok := found[m[1]]; !ok {
				found[m[1]] = rel
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo source: %v", err)
	}
	return found
}

// TestEnvCatalogCoversEveryGetenv is the drift guard for the runtime env
// surface: every SLIPWAY_* env var plus ambient token/username fallback read
// anywhere in source must have an EnvCatalog() entry. Reading a new env var
// without cataloguing it FAILS here and names the missing variable and the file
// that reads it — the env-side mirror of
// TestConfigCatalogCoversEveryStructLeaf.
func TestEnvCatalogCoversEveryGetenv(t *testing.T) {
	have := map[string]bool{}
	for _, entry := range EnvCatalog() {
		have[entry.Name] = true
	}
	usages := scanEnvUsages(t, repoRootForTest(t))
	if len(usages) == 0 {
		t.Fatal("scanned zero env usages; the source scan is broken")
	}
	names := make([]string, 0, len(usages))
	for name := range usages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !have[name] {
			t.Errorf("env var %q (read in %s) has no EnvCatalog() entry; add one to internal/model/env_catalog.go", name, usages[name])
		}
	}
}

// TestEnvCatalogEntriesAreWellFormed asserts each entry carries the required
// metadata, uses a known scope, and that the env/file relationship is coherent:
// repo-policy entries point at a real ConfigCatalog() file key, secret entries
// set Secret and carry no file key, and runtime-host entries carry no file key.
func TestEnvCatalogEntriesAreWellFormed(t *testing.T) {
	fileKeys := map[string]bool{}
	for _, entry := range ConfigCatalog() {
		fileKeys[entry.Name] = true
	}
	seen := map[string]bool{}
	for _, entry := range EnvCatalog() {
		if strings.TrimSpace(entry.Name) == "" {
			t.Errorf("env catalog entry has empty name: %+v", entry)
		}
		if seen[entry.Name] {
			t.Errorf("duplicate env catalog entry for %q", entry.Name)
		}
		seen[entry.Name] = true
		if strings.TrimSpace(entry.Description) == "" {
			t.Errorf("env catalog entry %q has empty description", entry.Name)
		}
		switch entry.Scope {
		case EnvScopeRepoPolicy:
			if entry.FileConfigKey == "" {
				t.Errorf("repo-policy env %q must carry a file_config_key", entry.Name)
			} else if !fileKeys[entry.FileConfigKey] {
				t.Errorf("repo-policy env %q references file_config_key %q with no ConfigCatalog() entry", entry.Name, entry.FileConfigKey)
			}
			if entry.Secret {
				t.Errorf("repo-policy env %q must not be marked secret", entry.Name)
			}
		case EnvScopeRuntimeHost:
			if entry.FileConfigKey != "" {
				t.Errorf("runtime-host env %q must not carry a file_config_key", entry.Name)
			}
		case EnvScopeSecret:
			if !entry.Secret {
				t.Errorf("secret env %q must set Secret=true", entry.Name)
			}
			if entry.FileConfigKey != "" {
				t.Errorf("secret env %q must not carry a file_config_key (secrets stay env-only)", entry.Name)
			}
		default:
			t.Errorf("env catalog entry %q has unknown scope %q", entry.Name, entry.Scope)
		}
	}
}

// TestEnvCatalogSorted asserts EnvCatalog() returns entries in name order so the
// discovery surface is stable.
func TestEnvCatalogSorted(t *testing.T) {
	entries := EnvCatalog()
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Name > entries[i].Name {
			t.Errorf("env catalog not sorted: %q before %q", entries[i-1].Name, entries[i].Name)
		}
	}
}
