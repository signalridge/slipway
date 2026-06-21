package progression

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSteadyStateFreshnessPathsDoNotConsumeFileModTime(t *testing.T) {
	t.Parallel()

	repoRoot := repositoryRootForFreshnessGuard(t)
	for _, rel := range freshnessGuardProductionFiles(t, repoRoot) {
		file := parseFreshnessGuardFile(t, repoRoot, rel)
		allowedFunctions := []string(nil)
		if rel == "internal/engine/progression/evidence_digests.go" {
			allowedFunctions = []string{
				"digestInputsChangedAfterVerdict",
				"digestInputChangedAfterVerdict",
				"digestInputPathChangedAfterVerdict",
			}
		}
		if disallowed := disallowedSelectorCalls(
			file,
			[]selectorCallMatch{{selector: "ModTime"}},
			allowedFunctions...,
		); len(disallowed) > 0 {
			t.Fatalf("%s must not consume file mtimes on steady-state freshness paths: %v", rel, disallowed)
		}
	}
}

func TestSteadyStateFreshnessPathsDoNotConsumeWallClockNow(t *testing.T) {
	t.Parallel()

	repoRoot := repositoryRootForFreshnessGuard(t)
	for _, rel := range freshnessGuardProductionFiles(t, repoRoot) {
		file := parseFreshnessGuardFile(t, repoRoot, rel)
		allowedFunctions := []string(nil)
		switch rel {
		case "internal/engine/progression/wave_sync.go":
			allowedFunctions = []string{"BuildExecutionSummary"}
		case "internal/state/wave_execution.go":
			allowedFunctions = []string{"MaterializeWavePlan"}
		}
		if disallowed := disallowedSelectorCalls(
			file,
			[]selectorCallMatch{{receiverSuffix: "time", selector: "Now"}},
			allowedFunctions...,
		); len(disallowed) > 0 {
			t.Fatalf("%s must not consume wall-clock time on steady-state freshness paths: %v", rel, disallowed)
		}
	}
}

func TestGenericEvidenceFreshnessDoesNotUseTimestampOrdering(t *testing.T) {
	t.Parallel()

	repoRoot := repositoryRootForFreshnessGuard(t)

	// The execution-summary freshness path must not resurrect an artifact-clock
	// baseline helper.
	summaryFile := parseFreshnessGuardFile(t, repoRoot, "internal/state/execution_summary.go")
	if hasFunctionDecl(summaryFile, "latestExecutionRelevantUpdateAt") {
		t.Fatalf("execution-summary freshness must not keep artifact-clock baseline helpers")
	}

	// The generic evidence-freshness evaluator (EvaluateEvidenceFreshness and its
	// EvidenceFreshnessInput) is structural-only: it compares expected vs current
	// structural inputs and must never reintroduce the removed timestamp fields
	// (EvidenceTimestamp / LatestRelevantUpdateAt) or timestamp ordering. Importing
	// "time" is the precondition for any of those, so forbid it outright at this
	// boundary.
	contextFile := parseFreshnessGuardFile(t, repoRoot, "internal/engine/context/context.go")
	if fileImportsPackage(contextFile, "time") {
		t.Fatalf("internal/engine/context/context.go must not import \"time\": " +
			"generic evidence freshness must stay structural and must not reintroduce timestamp fields or ordering")
	}
}

func TestAuthorityTimestampOrderingIsLimitedToProofOrderingGates(t *testing.T) {
	t.Parallel()

	repoRoot := repositoryRootForFreshnessGuard(t)
	file := parseFreshnessGuardFile(t, repoRoot, "internal/engine/progression/authority.go")
	disallowed := disallowedSelectorCalls(
		file,
		[]selectorCallMatch{{selector: "Before"}, {selector: "After"}},
		"proofReuseEdgeBlockers",
		"closeoutChainOrderBlockers",
	)
	if len(disallowed) > 0 {
		t.Fatalf("authority timestamp ordering must stay limited to proof-ordering gates: %v", disallowed)
	}
}

func repositoryRootForFreshnessGuard(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func freshnessGuardProductionFiles(t *testing.T, repoRoot string) []string {
	t.Helper()

	files := []string{
		"internal/engine/context/context.go",
		"internal/state/execution_repair.go",
		"internal/state/execution_summary.go",
		"internal/state/wave_execution.go",
	}
	progressDir := filepath.Join(repoRoot, "internal", "engine", "progression")
	err := filepath.WalkDir(progressDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk progression files: %v", err)
	}
	return files
}

type selectorCallMatch struct {
	receiverSuffix string
	selector       string
}

func parseFreshnessGuardFile(t *testing.T, repoRoot string, rel string) *ast.File {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(repoRoot, rel), nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	return file
}

func disallowedSelectorCalls(file *ast.File, matches []selectorCallMatch, allowedFunctions ...string) []string {
	allowed := make(map[string]struct{}, len(allowedFunctions))
	for _, fn := range allowedFunctions {
		allowed[fn] = struct{}{}
	}

	var disallowed []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if _, ok := allowed[fn.Name.Name]; ok {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			for _, match := range matches {
				if !match.matches(sel) {
					continue
				}
				disallowed = append(disallowed, fn.Name.Name+" uses "+selectorPath(sel))
				return false
			}
			return true
		})
	}
	return disallowed
}

func (match selectorCallMatch) matches(sel *ast.SelectorExpr) bool {
	if sel.Sel.Name != match.selector {
		return false
	}
	if match.receiverSuffix == "" {
		return true
	}
	return strings.HasSuffix(selectorPath(sel.X), match.receiverSuffix)
}

func selectorPath(expr ast.Expr) string {
	switch n := expr.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.SelectorExpr:
		prefix := selectorPath(n.X)
		if prefix == "" {
			return n.Sel.Name
		}
		return prefix + "." + n.Sel.Name
	default:
		return ""
	}
}

func hasFunctionDecl(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return true
		}
	}
	return false
}

func fileImportsPackage(file *ast.File, importPath string) bool {
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		if strings.Trim(imp.Path.Value, "`\"") == importPath {
			return true
		}
	}
	return false
}
