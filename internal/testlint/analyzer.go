// Package testlint contains Slipway-specific test quality checks.
package testlint

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	sourceGrepMessage = "source-grep test reads .go files and asserts on source text; delete it and replace with behavior coverage"
	timingMessage     = "elapsed/timing assertion uses time.Since or measured duration comparison; replace it with deterministic synchronization or fake time"
)

// Analyzer reports tests that verify implementation text or wall-clock elapsed
// time instead of observable behavior.
var Analyzer = &analysis.Analyzer{
	Name: "testlint",
	Doc:  "reports source-grep and elapsed-time tests that should be deleted and replaced",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if !strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go") {
			continue
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !isTestFunc(fn) {
				continue
			}
			checkTestFunc(pass, fn)
		}
	}

	return nil, nil
}

func checkTestFunc(pass *analysis.Pass, fn *ast.FuncDecl) {
	sourceObjects := map[types.Object]struct{}{}
	timingObjects := map[types.Object]struct{}{}

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			collectAssign(pass, n.Lhs, n.Rhs, sourceObjects, timingObjects)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, 0, len(n.Names))
			for _, name := range n.Names {
				lhs = append(lhs, name)
			}
			collectAssign(pass, lhs, n.Values, sourceObjects, timingObjects)
		}

		return true
	})

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			if isStringsContains(pass, n) && len(n.Args) >= 1 && isSourceValue(pass, n.Args[0], sourceObjects) {
				pass.Reportf(n.Pos(), sourceGrepMessage)
			}
			if isTestifyTimingAssertion(pass, n, timingObjects) {
				pass.Reportf(n.Pos(), timingMessage)
			}
		case *ast.BinaryExpr:
			if isOrderingComparison(n.Op) && isTimingComparison(pass, n, timingObjects) {
				pass.Reportf(n.OpPos, timingMessage)
			}
		}

		return true
	})
}

func collectAssign(
	pass *analysis.Pass,
	lhs []ast.Expr,
	rhs []ast.Expr,
	sourceObjects map[types.Object]struct{},
	timingObjects map[types.Object]struct{},
) {
	if len(rhs) == 0 {
		return
	}

	for i, left := range lhs {
		ident, ok := left.(*ast.Ident)
		if !ok {
			continue
		}

		right := rhs[len(rhs)-1]
		if i < len(rhs) {
			right = rhs[i]
		}

		obj := objectForIdent(pass, ident)
		if obj == nil {
			continue
		}
		if readsGoFile(pass, right) || isSourceValue(pass, right, sourceObjects) {
			sourceObjects[obj] = struct{}{}
		}
		if containsTimeSince(pass, right) || isTimingValue(pass, right, timingObjects) {
			timingObjects[obj] = struct{}{}
		}
	}
}

func isTestFunc(fn *ast.FuncDecl) bool {
	return strings.HasPrefix(fn.Name.Name, "Test") && fn.Recv == nil
}

func isStringsContains(pass *analysis.Pass, call *ast.CallExpr) bool {
	return isSelectorFromPackage(pass, call.Fun, "strings", "Contains")
}

func readsGoFile(pass *analysis.Pass, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	if !isSelectorFromPackage(pass, call.Fun, "os", "ReadFile") &&
		!isSelectorFromPackage(pass, call.Fun, "io/ioutil", "ReadFile") {
		return false
	}

	return pathExprNamesGoFile(pass, call.Args[0])
}

func pathExprNamesGoFile(pass *analysis.Pass, expr ast.Expr) bool {
	if value, ok := stringConst(pass, expr); ok {
		return strings.HasSuffix(value, ".go")
	}

	call, ok := expr.(*ast.CallExpr)
	if !ok || !isSelectorFromPackage(pass, call.Fun, "path/filepath", "Join") {
		return false
	}
	for _, arg := range call.Args {
		if value, ok := stringConst(pass, arg); ok && strings.HasSuffix(value, ".go") {
			return true
		}
	}

	return false
}

func stringConst(pass *analysis.Pass, expr ast.Expr) (string, bool) {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		value, err := strconv.Unquote(lit.Value)
		return value, err == nil
	}

	tv, ok := pass.TypesInfo.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}

	return constant.StringVal(tv.Value), true
}

func isSourceValue(pass *analysis.Pass, expr ast.Expr, sourceObjects map[types.Object]struct{}) bool {
	switch n := expr.(type) {
	case *ast.Ident:
		_, ok := sourceObjects[objectForIdent(pass, n)]
		return ok
	case *ast.CallExpr:
		if isStringConversion(pass, n) && len(n.Args) == 1 {
			return isSourceValue(pass, n.Args[0], sourceObjects) || readsGoFile(pass, n.Args[0])
		}
		return readsGoFile(pass, n)
	case *ast.ParenExpr:
		return isSourceValue(pass, n.X, sourceObjects)
	}

	return false
}

func isStringConversion(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "string" {
		return false
	}

	typ := pass.TypesInfo.TypeOf(call.Fun)
	basic, ok := typ.(*types.Basic)
	return ok && basic.Kind() == types.String
}

func isOrderingComparison(op token.Token) bool {
	return op == token.LSS || op == token.LEQ || op == token.GTR || op == token.GEQ
}

func isTestifyTimingAssertion(
	pass *analysis.Pass,
	call *ast.CallExpr,
	timingObjects map[types.Object]struct{},
) bool {
	if !isTestifyOrderingAssertion(pass, call.Fun) {
		return false
	}
	for _, arg := range call.Args {
		if containsTimeSince(pass, arg) || isTimingValue(pass, arg, timingObjects) {
			return true
		}
	}
	return false
}

func isTestifyOrderingAssertion(pass *analysis.Pass, expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || !isTestifyOrderingAssertionName(sel.Sel.Name) {
		return false
	}
	obj, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || obj.Pkg() == nil {
		return false
	}
	switch obj.Pkg().Path() {
	case "github.com/stretchr/testify/assert", "github.com/stretchr/testify/require":
		return true
	default:
		return false
	}
}

func isTestifyOrderingAssertionName(name string) bool {
	switch name {
	case "Greater", "GreaterOrEqual", "Less", "LessOrEqual":
		return true
	default:
		return false
	}
}

func isTimingComparison(pass *analysis.Pass, expr *ast.BinaryExpr, timingObjects map[types.Object]struct{}) bool {
	return containsTimeSince(pass, expr.X) ||
		containsTimeSince(pass, expr.Y) ||
		isTimingValue(pass, expr.X, timingObjects) ||
		isTimingValue(pass, expr.Y, timingObjects)
}

func containsTimeSince(pass *analysis.Pass, expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if isSelectorFromPackage(pass, call.Fun, "time", "Since") {
			found = true
			return false
		}

		return true
	})

	return found
}

func isTimingValue(pass *analysis.Pass, expr ast.Expr, timingObjects map[types.Object]struct{}) bool {
	switch n := expr.(type) {
	case *ast.Ident:
		_, ok := timingObjects[objectForIdent(pass, n)]
		return ok
	case *ast.ParenExpr:
		return isTimingValue(pass, n.X, timingObjects)
	}

	return false
}

func isSelectorFromPackage(pass *analysis.Pass, expr ast.Expr, pkgPath string, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}

	obj, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || obj.Pkg() == nil {
		return false
	}

	return obj.Pkg().Path() == pkgPath
}

func objectForIdent(pass *analysis.Pass, ident *ast.Ident) types.Object {
	if obj := pass.TypesInfo.Defs[ident]; obj != nil {
		return obj
	}

	return pass.TypesInfo.Uses[ident]
}
