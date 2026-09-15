// Package gitexec is the compiled source guard that the CI step builds from this
// repository and runs directly.
//
// It replaces shell pipelines that could report success without scanning anything:
// a missing scanner, an empty target list or a truncated pipe all exited zero. Here
// the exit status is the verdict — the guard self-checks against a checked-in fixture
// set, censuses its targets, classifies git spawn sites by parsing the code, and
// reconciles the result against a frozen baseline.
//
// Scope: *.go outside internal/git, Go tests, testdata, e2e and openspec, plus the
// non-Go host assets under internal/assets/pi. Two rule classes:
//
//	go-git-spawn       migration debt that must route through internal/git
//	declared-boundary  declared permanent exceptions, never counted as debt
package gitexec

import (
	"cmp"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Rule names the baseline class a finding belongs to.
type Rule string

const (
	// RuleGoGitSpawn is a Go site spawning git directly or through a helper; it is
	// debt that must migrate to internal/git.
	RuleGoGitSpawn Rule = "go-git-spawn"
	// RuleDeclaredBoundary is a declared, permanent exception: the injectable
	// internal/doctor seams that simulate git, and the host assets outside the Go
	// binary. It never counts as debt.
	RuleDeclaredBoundary Rule = "declared-boundary"
)

// ExpectedFixtureFindings is the positive control: the checked-in fixture set holds
// exactly this many sites, so a scanner that stops seeing its targets cannot pass.
const ExpectedFixtureFindings = 9

const hostDirPrefix = "internal/assets/pi"

// ErrNoTargets is returned when a scan resolves zero .go targets: a scan that cannot
// prove it scanned something is a failure, never a clean tree.
var ErrNoTargets = errors.New("scan resolved zero .go targets")

// Finding is one git spawn site, reported as rule, path and line.
type Finding struct {
	Rule Rule
	Path string
	Line int
}

// String renders the finding in baseline format: rule<TAB>path<TAB>line.
func (f Finding) String() string {
	return fmt.Sprintf("%s\t%s\t%d", f.Rule, f.Path, f.Line)
}

// ScanResult is the outcome of one scan.
type ScanResult struct {
	Findings    []Finding
	GoTargets   int
	HostTargets int
}

// Scanner walks one root and classifies the git spawn sites it can prove.
type Scanner struct {
	Root string
}

// Scan classifies every spawn site under Root. Unreadable or unparsable targets and
// a scan that resolves no Go target are errors, never silently skipped files.
func (s Scanner) Scan() (ScanResult, error) {
	var res ScanResult
	err := filepath.WalkDir(s.Root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(s.Root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if skipDir(rel, entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		var found []Finding
		switch {
		case isGoTarget(rel):
			res.GoTargets++
			found, err = scanGo(path, rel)
		case isHostTarget(rel):
			res.HostTargets++
			found, err = scanHost(path, rel)
		default:
			return nil
		}
		if err != nil {
			return err
		}
		res.Findings = append(res.Findings, found...)
		return nil
	})
	if err != nil {
		return res, err
	}
	if res.GoTargets == 0 {
		return res, fmt.Errorf("%w: %s", ErrNoTargets, s.Root)
	}
	sortFindings(res.Findings)
	return res, nil
}

// SelfCheck runs the positive control: the fixture set must resolve targets and
// report exactly ExpectedFixtureFindings sites. Zero findings are never a pass.
func SelfCheck(fixtures string) (ScanResult, error) {
	res, err := (Scanner{Root: fixtures}).Scan()
	if err != nil {
		return res, err
	}
	if len(res.Findings) != ExpectedFixtureFindings {
		return res, fmt.Errorf("self-check: %s reported %d sites, want %d", fixtures, len(res.Findings), ExpectedFixtureFindings)
	}
	return res, nil
}

func skipDir(rel, name string) bool {
	switch name {
	case "testdata", "node_modules", ".git":
		return true
	}
	return rel == "e2e" || rel == "openspec" || rel == "internal/git"
}

func isGoTarget(rel string) bool {
	return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
}

func isHostTarget(rel string) bool {
	if !strings.HasPrefix(rel, hostDirPrefix+"/") {
		return false
	}
	return strings.HasSuffix(rel, ".ts") || strings.HasSuffix(rel, ".js")
}

// sortFindings orders findings canonically: debt before boundary, then path, then line.
func sortFindings(findings []Finding) {
	slices.SortFunc(findings, compareFindings)
}

func compareFindings(a, b Finding) int {
	if c := cmp.Compare(ruleRank(a.Rule), ruleRank(b.Rule)); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Path, b.Path); c != 0 {
		return c
	}
	return cmp.Compare(a.Line, b.Line)
}

func ruleRank(rule Rule) int {
	if rule == RuleGoGitSpawn {
		return 0
	}
	return 1
}

// scanGo parses one Go file and returns its git spawn sites.
func scanGo(path, rel string) ([]Finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	execPkgs := execPackageNames(file)
	var out []Finding
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		lit, callee, ok := commandLiteral(call, execPkgs)
		if !ok {
			return true
		}
		out = append(out, Finding{Rule: ruleFor(rel, callee), Path: rel, Line: fset.Position(lit.Pos()).Line})
		return true
	})
	return out, nil
}

// execPackageNames returns the local names bound to the os/exec import, so aliased
// spawns (osexec "os/exec") classify like the plain ones.
func execPackageNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != "os/exec" {
			continue
		}
		if imp.Name != nil {
			names[imp.Name.Name] = true
			continue
		}
		names["exec"] = true
	}
	return names
}

// lookupCallees name a command without executing it; they are not spawn sites.
var lookupCallees = map[string]bool{"LookPath": true, "lookPathFn": true}

// commandLiteral returns the "git" literal in command position, if any, plus the
// callee name. Known os/exec constructors are read at their exact command argument;
// any other callee is a wrapper or an injected seam, so its command name sits in one
// of the first two positional arguments (runCmd("git", …), helper(ctx, "git", …)).
func commandLiteral(call *ast.CallExpr, execPkgs map[string]bool) (*ast.BasicLit, string, bool) {
	name, recv := calleeName(call.Fun)
	if name == "" || lookupCallees[name] {
		return nil, "", false
	}
	if execPkgs[recv] {
		idx := 0
		switch name {
		case "Command":
			// The command name is the first argument.
		case "CommandContext":
			idx = 1
		default:
			return nil, "", false
		}
		if idx < len(call.Args) {
			lit, ok := gitLiteral(call.Args[idx])
			return lit, name, ok
		}
		return nil, "", false
	}
	for i := range min(len(call.Args), 2) {
		if lit, ok := gitLiteral(call.Args[i]); ok {
			return lit, name, true
		}
	}
	return nil, "", false
}

func gitLiteral(e ast.Expr) (*ast.BasicLit, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING || lit.Value != `"git"` {
		return nil, false
	}
	return lit, true
}

// calleeName returns the callee's final identifier plus, for selector expressions,
// the identifier it is selected from ("exec" in exec.Command, "c" in c.execFn).
func calleeName(fun ast.Expr) (name, recv string) {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name, ""
	case *ast.SelectorExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			return f.Sel.Name, id.Name
		}
		return f.Sel.Name, ""
	}
	return "", ""
}

// ruleFor classifies one site: internal/doctor's execFn seams inject a fake git to
// simulate present, absent and broken, so they are a declared boundary; every other
// Go site is migration debt.
func ruleFor(rel, callee string) Rule {
	if strings.HasPrefix(rel, "internal/doctor/") && callee == "execFn" {
		return RuleDeclaredBoundary
	}
	return RuleGoGitSpawn
}

// hostSpawn matches a host call whose first argument is the literal "git", including
// aliases (const run = execFileSync; run("git", …)).
var hostSpawn = regexp.MustCompile(`[\w.$]+\s*\(\s*["']git["']`)

// scanHost reports git spawns in the non-Go assets. Host assets are lexed, not
// parsed, so prose in comments and documentation-like files is never classified.
func scanHost(path, rel string) ([]Finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	var out []Finding
	line := 0
	for text := range strings.SplitSeq(string(src), "\n") {
		line++
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		if hostSpawn.MatchString(text) {
			out = append(out, Finding{Rule: RuleDeclaredBoundary, Path: rel, Line: line})
		}
	}
	return out, nil
}
