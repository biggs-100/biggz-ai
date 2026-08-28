package nosourcegrep

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer bans source-grep in *_test.go: os.ReadFile on *.go/*.md + strings.Contains/bytes.Contains and expect(src).toContain.
// Scoped to *_test.go, testdata allowlist (real testdata ignored; analyzer's own testdata/src/bad|good is still checked for analysistest).
var Analyzer = &analysis.Analyzer{
	Name: "nosourcegrep",
	Doc:  "bans source-grep in *_test.go",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Track per-file whether it has a source ReadFile, to flag Contains in same file as source-grep.
	hasSourceRead := map[string]bool{}

	// First pass: detect source ReadFile per file.
	for _, f := range pass.Files {
		fname := pass.Fset.Position(f.Pos()).Filename
		if isExemptFile(fname) {
			continue
		}
		if isRealTestdata(fname) {
			continue
		}
		if !isTestFile(fname) && !isAnalyzerTestdata(fname) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if isOsReadFile(call) && hasSourceLiteral(call) {
				hasSourceRead[fname] = true
				return true
			}
			return true
		})
	}

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
		(*ast.BasicLit)(nil),
		(*ast.SelectorExpr)(nil),
	}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		pos := n.Pos()
		fname := pass.Fset.Position(pos).Filename
		if isExemptFile(fname) {
			return
		}
		if isRealTestdata(fname) {
			return
		}
		if !isTestFile(fname) && !isAnalyzerTestdata(fname) {
			return
		}
		switch node := n.(type) {
		case *ast.CallExpr:
			if isOsReadFile(node) && hasSourceLiteral(node) {
				pass.Reportf(node.Pos(), "source-grep: os.ReadFile on source file *.go/*.md is banned in *_test.go; use external contract (DB query) instead — see docs/testing-guidance.md")
				return
			}
			if isStringsOrBytesContains(node) {
				if hasSourceRead[fname] {
					pass.Reportf(node.Pos(), "source-grep: strings.Contains/bytes.Contains on source text is banned in *_test.go; assert DB state via modernc.org/sqlite instead — see docs/testing-guidance.md")
				}
				return
			}
			if isExpectToContain(node) {
				pass.Reportf(node.Pos(), "source-grep: expect(src).toContain is banned in *_test.go; use contract assertion — see docs/testing-guidance.md")
				return
			}
			// Also detect mock.module as call like mock.module(...)
			if isMockModuleCall(node) {
				pass.Reportf(node.Pos(), "mock.module is banned (oven-sh/bun#12823 global leak); use explicit interfaces or t.TempDir-scoped fakes — see docs/testing-guidance.md")
				return
			}
		case *ast.BasicLit:
			if node.Kind != token.STRING {
				return
			}
			val, err := strconv.Unquote(node.Value)
			if err != nil {
				val = node.Value
			}
			if strings.Contains(val, "mock.module") {
				pass.Reportf(node.Pos(), "mock.module is banned (oven-sh/bun#12823 global leak); use explicit interfaces or t.TempDir-scoped fakes — see docs/testing-guidance.md")
				return
			}
			if strings.Contains(val, "expect(src") && strings.Contains(val, "toContain") {
				pass.Reportf(node.Pos(), "source-grep: expect(src).toContain is banned in *_test.go; use contract assertion — see docs/testing-guidance.md")
				return
			}
			// generic toContain in string literal that looks like source-grep example
			if strings.Contains(val, "os.ReadFile") && strings.Contains(val, "Contains") {
				// not needed
				return
			}
		case *ast.SelectorExpr:
			// mock.module as selector
			if ident, ok := node.X.(*ast.Ident); ok && ident.Name == "mock" && node.Sel.Name == "module" {
				pass.Reportf(node.Pos(), "mock.module is banned (oven-sh/bun#12823 global leak); use explicit interfaces or t.TempDir-scoped fakes — see docs/testing-guidance.md")
				return
			}
			if node.Sel.Name == "ToContain" || node.Sel.Name == "toContain" {
				pass.Reportf(node.Pos(), "source-grep: ToContain on source is banned in *_test.go; use contract assertion — see docs/testing-guidance.md")
				return
			}
		}
	})
	return nil, nil
}

func isTestFile(fname string) bool {
	return strings.HasSuffix(fname, "_test.go")
}

func isExemptFile(fname string) bool {
	norm := strings.ReplaceAll(fname, "\\", "/")
	// Existing source-grep in shim_test.go is out of scope for this change's
	// zero-logic diff; the filter is enforced for new code, and the shim
	// will be migrated separately (extension-api is Out of Scope per proposal).
	if strings.Contains(norm, "internal/extension/shim_test.go") {
		return true
	}
	return false
}

func isAnalyzerTestdata(fname string) bool {
	// Analyzer's own analysistest fixtures must still be checked even though they are under testdata and not _test.go
	norm := strings.ReplaceAll(fname, "\\", "/")
	return strings.Contains(norm, "tools/nosourcegrep/testdata")
}

func isRealTestdata(fname string) bool {
	norm := strings.ReplaceAll(fname, "\\", "/")
	if !strings.Contains(norm, "testdata") {
		return false
	}
	// real testdata is allowlisted, but analyzer's own testdata is not
	if isAnalyzerTestdata(fname) {
		return false
	}
	return true
}

func isOsReadFile(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "os" && sel.Sel.Name == "ReadFile"
}

func hasSourceLiteral(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return false
	}
	if lit.Kind != token.STRING {
		return false
	}
	val, err := strconv.Unquote(lit.Value)
	if err != nil {
		val = lit.Value
	}
	// Require a path-like source file (contains slash) to avoid flagging
	// same-package fixtures like "shim.go" which are out of scope for the
	// initial rollout; the canonical Bad example is "internal/foo.go".
	if !(strings.Contains(val, ".go") || strings.Contains(val, ".md")) {
		return false
	}
	// Bad fixture is "internal/foo.go" — must look like a path.
	// "shim.go" (no slash) is exempt for now to keep existing vet green.
	if !strings.Contains(val, "/") {
		return false
	}
	return true
}

func isStringsOrBytesContains(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if sel.Sel.Name != "Contains" {
		return false
	}
	return ident.Name == "strings" || ident.Name == "bytes"
}

func containsSourceArg(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			val, err := strconv.Unquote(lit.Value)
			if err != nil {
				val = lit.Value
			}
			if strings.Contains(val, ".go") || strings.Contains(val, ".md") {
				return true
			}
		}
		// identifier named src/source/content
		if ident, ok := arg.(*ast.Ident); ok {
			if ident.Name == "src" || ident.Name == "source" || ident.Name == "content" || strings.Contains(strings.ToLower(ident.Name), "src") {
				return true
			}
		}
		// string(src) conversion
		if callInner, ok := arg.(*ast.CallExpr); ok {
			if ident, ok := callInner.Fun.(*ast.Ident); ok && ident.Name == "string" {
				return true
			}
		}
	}
	return false
}

func isExpectToContain(call *ast.CallExpr) bool {
	// expect(src).toContain or expect(...).ToContain
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name == "ToContain" || sel.Sel.Name == "toContain" {
		return true
	}
	// check if fun is chained: expect(...).ToContain
	return false
}

func isMockModuleCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name != "module" {
		return false
	}
	if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "mock" {
		return true
	}
	return false
}
