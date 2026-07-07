package model

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// publicEnvLiteral matches environment-variable names Slipway treats as public
// runtime/config surface when they appear as source string literals:
// SLIPWAY_* names, ambient token fallbacks, and ambient username fallbacks.
// Scanning string literals catches vars read via const identifiers and simple
// fallback slices.
var publicEnvLiteral = regexp.MustCompile(`^(SLIPWAY_[A-Z0-9_]+|[A-Z][A-Z0-9_]*_TOKEN|GH_TOKEN|USER|USERNAME)$`)

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
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		fileSet := token.NewFileSet()
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.BasicLit:
				if node.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(node.Value)
				if err == nil && publicEnvLiteral.MatchString(value) {
					recordEnvUsage(found, value, rel)
				}
			case *ast.CallExpr:
				if name, ok := directEnvReadLiteral(node); ok {
					recordEnvUsage(found, name, rel)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo source: %v", err)
	}
	return found
}

func recordEnvUsage(found map[string]string, name, rel string) {
	if _, ok := found[name]; !ok {
		found[name] = rel
	}
}

func directEnvReadLiteral(call *ast.CallExpr) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) != 1 {
		return "", false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "os" {
		return "", false
	}
	switch selector.Sel.Name {
	case "Getenv", "LookupEnv":
	default:
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	name, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	name = strings.TrimSpace(name)
	return name, name != ""
}

// TestEnvCatalogCoversPublicEnvLiterals is the drift guard for the runtime env
// surface: every public env-var literal in the SLIPWAY_ namespace plus ambient
// token/username fallback, and every direct os.Getenv/os.LookupEnv string
// argument, must have an EnvCatalog() entry. Reading a new env var through a
// string literal, shared const, or direct env call without cataloguing it FAILS
// here and names the missing variable and source file.
func TestEnvCatalogCoversPublicEnvLiterals(t *testing.T) {
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
		if strings.TrimSpace(entry.ValueSyntax) == "" {
			t.Errorf("env catalog entry %q has empty value_syntax", entry.Name)
		}
		if strings.TrimSpace(entry.UnsetBehavior) == "" {
			t.Errorf("env catalog entry %q has empty unset_behavior", entry.Name)
		}
		for _, accepted := range entry.AcceptedValues {
			if strings.TrimSpace(accepted.Value) == "" {
				t.Errorf("env catalog entry %q has accepted value with empty value", entry.Name)
			}
			if strings.TrimSpace(accepted.Description) == "" {
				t.Errorf("env catalog entry %q accepted value %q has empty description", entry.Name, accepted.Value)
			}
		}
		for _, example := range entry.Examples {
			if strings.TrimSpace(example) == "" {
				t.Errorf("env catalog entry %q has empty example", entry.Name)
			}
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

func TestEnvCatalogHostCapabilityWiringContract(t *testing.T) {
	entries := EnvCatalog()
	byName := map[string]EnvCatalogEntry{}
	for _, entry := range entries {
		byName[entry.Name] = entry
	}

	capabilities, ok := byName["SLIPWAY_HOST_CAPABILITIES"]
	if !ok {
		t.Fatal("SLIPWAY_HOST_CAPABILITIES missing from env catalog")
	}
	if capabilities.ValueSyntax == "" {
		t.Fatal("SLIPWAY_HOST_CAPABILITIES must describe token syntax")
	}
	if capabilities.UnsetBehavior == "" {
		t.Fatal("SLIPWAY_HOST_CAPABILITIES must describe unset behavior")
	}
	if !strings.Contains(capabilities.UnsetBehavior, "unrecognized") {
		t.Fatal("SLIPWAY_HOST_CAPABILITIES must describe closed-world handling for unrecognized tokens")
	}
	if len(capabilities.Examples) == 0 {
		t.Fatal("SLIPWAY_HOST_CAPABILITIES must include at least one host declaration example")
	}

	accepted := map[string]string{}
	for _, value := range capabilities.AcceptedValues {
		accepted[value.Value] = value.Description
	}
	for _, token := range []string{"subagent", "delegation", "none", "unavailable"} {
		if strings.TrimSpace(accepted[token]) == "" {
			t.Fatalf("SLIPWAY_HOST_CAPABILITIES accepted values missing %q with description: %#v", token, accepted)
		}
	}

	fallbacks, ok := byName["SLIPWAY_HOST_CAPABILITY_FALLBACKS"]
	if !ok {
		t.Fatal("SLIPWAY_HOST_CAPABILITY_FALLBACKS missing from env catalog")
	}
	if !strings.Contains(fallbacks.ValueSyntax, "case-insensitive") {
		t.Fatal("SLIPWAY_HOST_CAPABILITY_FALLBACKS must describe case-insensitive token matching")
	}
	fallbackTokens := map[string]bool{}
	for _, value := range fallbacks.AcceptedValues {
		fallbackTokens[value.Value] = true
	}
	for _, token := range []string{"same_context_degraded", "manual_plan_audit", "manual_security_review"} {
		if !fallbackTokens[token] {
			t.Fatalf("SLIPWAY_HOST_CAPABILITY_FALLBACKS accepted values missing %q: %#v", token, fallbackTokens)
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
