package readability

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/review/lens"
	"github.com/fzipp/gocyclo"
	"github.com/uudashr/gocognit"
)

// Thresholds fixed per spec.
const (
	CyclomaticThreshold = 15
	CognitiveThreshold  = 20
)

// CriticalPackages are the only packages subject to blocking complexity gates.
var CriticalPackages = []string{
	"internal/review",
	"internal/sdd",
	"internal/verification",
}

// Offender is one function exceeding a threshold.
type Offender struct {
	Package    string `json:"package"`
	File       string `json:"file"`
	Function   string `json:"function"`
	Line       int    `json:"line"`
	Cyclomatic int    `json:"cyclomatic"`
	Cognitive  int    `json:"cognitive"`
}

// isCriticalPackage reports whether path is under a critical package.
func isCriticalPackage(path string) bool {
	for _, pkg := range CriticalPackages {
		if path == pkg || strings.HasPrefix(path, pkg+"/") {
			return true
		}
	}
	return false
}

// isTestFile reports whether path is a *_test.go file.
func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

// packageForPath derives a package label from path (dir).
func packageForPath(path string) string {
	dir := filepath.Dir(path)
	// Normalize to slash form for JSON stability.
	return filepath.ToSlash(dir)
}

// funcName returns display name for a FuncDecl, including receiver if present.
func funcName(fn *ast.FuncDecl) string {
	if fn.Recv != nil && fn.Recv.NumFields() > 0 {
		typ := fn.Recv.List[0].Type
		return fmt.Sprintf("(%s).%s", recvString(typ), fn.Name.Name) //lint:ignore no-fmtSprintf
	}
	return fn.Name.Name
}

func recvString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + recvString(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return recvString(t.X)
	case *ast.IndexListExpr:
		return recvString(t.X)
	default:
		return fmt.Sprintf("%T", expr) //lint:ignore no-fmtSprintf
	}
}

// lineRange is an inclusive line interval.
type lineRange struct {
	start, end int
}

func overlaps(funcStart, funcEnd int, ranges []lineRange) bool {
	if len(ranges) == 0 {
		return false
	}
	for _, r := range ranges {
		if funcEnd < r.start || funcStart > r.end {
			continue
		}
		return true
	}
	return false
}

// parseHunkHeader parses a single hunk header line and extracts the new-file range.
// Header example: "@@ -10,7 +10,7 @@ func Foo()".
func parseHunkHeader(line string) (lineRange, bool) {
	if !strings.HasPrefix(line, "@@") {
		return lineRange{}, false
	}
	parts := strings.Fields(line)
	for _, p := range parts {
		if !strings.HasPrefix(p, "+") {
			continue
		}
		s := strings.TrimPrefix(p, "+")
		seg := strings.Split(s, ",")
		startStr := seg[0]
		start, err := strconv.Atoi(startStr)
		if err != nil || start <= 0 {
			return lineRange{}, false
		}
		count := 1
		if len(seg) > 1 {
			if c, err := strconv.Atoi(seg[1]); err == nil {
				count = c
			}
		}
		if count == 0 {
			return lineRange{}, false
		}
		end := start + count - 1
		if end < start {
			end = start
		}
		return lineRange{start: start, end: end}, true
	}
	return lineRange{}, false
}

// parseHunkHeaders extracts changed line ranges (new file) from unified diff hunk headers.
// Headers look like "@@ -oldStart,oldLen +newStart,newLen @@".
func parseHunkHeaders(diff string) []lineRange {
	var out []lineRange
	for line := range strings.SplitSeq(diff, "\n") {
		if r, ok := parseHunkHeader(line); ok {
			out = append(out, r)
		}
	}
	return out
}

// isThresholdOffender reports whether cyclomatic or cognitive exceeds thresholds.
func isThresholdOffender(cyclo, cog int) bool {
	return cyclo > CyclomaticThreshold || cog > CognitiveThreshold
}

// offendersInHunk scans a single file content and returns offenders whose
// function ranges overlap the changed intervals.
func offendersInHunk(path string, content []byte, changedRanges []lineRange) []Offender {
	var offenders []Offender
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return offenders
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		if !overlaps(start, end, changedRanges) {
			continue
		}
		cyclo := gocyclo.Complexity(fn)
		cog := gocognit.Complexity(fn)
		if !isThresholdOffender(cyclo, cog) {
			continue
		}
		offenders = append(offenders, Offender{
			Package:    packageForPath(path),
			File:       path,
			Function:   funcName(fn),
			Line:       start,
			Cyclomatic: cyclo,
			Cognitive:  cog,
		})
	}
	return offenders
}

// resolveRepoPath joins repo root with rel path and reports a warning when repo is relative.
// It mimics git -C handling: absolute repo is preferred, relative fallback warns.
func resolveRepoPath(repo, rel string) (string, string) {
	if repo == "" {
		return rel, ""
	}
	if !filepath.IsAbs(repo) {
		warn := fmt.Sprintf("warning: repo path %q is relative, using fallback join for %q", repo, rel) //lint:ignore no-fmtSprintf
		return filepath.Join(repo, rel), warn
	}
	return filepath.Join(repo, rel), ""
}

// findFuncAtLine returns the function containing targetLine in src, if any.
func findFuncAtLine(path string, src []byte, targetLine int) (string, int, bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return "", 0, false
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		if targetLine >= start && targetLine <= end {
			return funcName(fn), start, true
		}
	}
	return "", 0, false
}



// resolveContentForPath resolves file content and changed ranges for a path.
// Returns content, ranges, warnings, and whether the file should be skipped.
func resolveContentForPath(path string, input lens.LensInput) ([]byte, []lineRange, []string, bool) {
	hunkBytes, hasHunk := input.Hunks[path]
	_, inSummary := input.DiffSummary[path]
	inPaths := slices.Contains(input.Paths, path)
	isDiff := hasHunk && bytes.Contains(hunkBytes, []byte("@@"))

	if isDiff {
		return resolveDiffContent(path, string(hunkBytes), input.Repo)
	}
	if hasHunk && len(hunkBytes) > 0 {
		return resolveHunkBytesContent(path, hunkBytes, inSummary, inPaths, input.Repo)
	}
	return resolvePlainContent(path, inSummary, inPaths, input.Repo)
}

func resolveDiffContent(path, diffContent, repo string) ([]byte, []lineRange, []string, bool) {
	var warnings []string
	changedRanges := parseHunkHeaders(diffContent)
	if len(changedRanges) == 0 {
		warnings = append(warnings, fmt.Sprintf("warning: file %s has diff but no mappable hunk ranges (rename or ambiguous diff)", path)) //lint:ignore no-fmtSprintf
		return nil, nil, warnings, true
	}
	if repo != "" {
		repoPath, warn := resolveRepoPath(repo, path)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		b, err := os.ReadFile(repoPath)
		if err != nil {
			b2, err2 := os.ReadFile(path)
			if err2 != nil {
				warnings = append(warnings, fmt.Sprintf("warning: cannot read %s for diff-based complexity: %v", path, err)) //lint:ignore no-fmtSprintf
				return nil, nil, warnings, true
			}
			if filepath.IsAbs(repo) {
				warnings = append(warnings, fmt.Sprintf("warning: fallback relative read for %s", path)) //lint:ignore no-fmtSprintf
			}
			return b2, changedRanges, warnings, false
		}
		return b, changedRanges, warnings, false
	}
	warnings = append(warnings, fmt.Sprintf("warning: no repo to resolve diff for %s", path)) //lint:ignore no-fmtSprintf
	return nil, nil, warnings, true
}

func resolveHunkBytesContent(path string, hunkBytes []byte, inSummary, inPaths bool, repo string) ([]byte, []lineRange, []string, bool) {
	var warnings []string
	content := hunkBytes
	var changedRanges []lineRange
	if inSummary || inPaths {
		changedRanges = []lineRange{{start: 1, end: 1000000}}
	} else {
		return nil, nil, warnings, true
	}
	fsetTmp := token.NewFileSet()
	if _, err := parser.ParseFile(fsetTmp, path, content, parser.ParseComments); err != nil && repo != "" {
		repoPath, warn := resolveRepoPath(repo, path)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		if b, err2 := os.ReadFile(repoPath); err2 == nil {
			content = b
		}
	}
	return content, changedRanges, warnings, false
}

func resolvePlainContent(path string, inSummary, inPaths bool, repo string) ([]byte, []lineRange, []string, bool) {
	var warnings []string
	var content []byte
	if repo != "" {
		repoPath, warn := resolveRepoPath(repo, path)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		b, err := os.ReadFile(repoPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("warning: cannot read %s: %v", path, err)) //lint:ignore no-fmtSprintf
			return nil, nil, warnings, true
		}
		content = b
	} else {
		b, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("warning: cannot read %s: %v", path, err)) //lint:ignore no-fmtSprintf
			return nil, nil, warnings, true
		}
		content = b
	}
	if inSummary || inPaths {
		return content, []lineRange{{start: 1, end: 1000000}}, warnings, false
	}
	return nil, nil, warnings, true
}

// offendersFromHunks derives complexity offenders that intersect the PR hunk set.
// It reuses the single DeriveRiskInput derivation (Paths/DiffSummary/Hunks/Repo) and
// does NOT run a second git diff. It is hunk-bounded: only functions whose
// line range overlaps a changed hunk interval are considered. Critical-package
// and *_test.go filtering is applied; test files are collected but flagged for
// informational treatment by the caller. Warnings include repo-path fallback and
// rename/no-map cases.
func collectCandidatePaths(input lens.LensInput) []string {
	pathSet := make(map[string]struct{})
	for _, p := range input.Paths {
		pathSet[p] = struct{}{}
	}
	for p := range input.DiffSummary {
		pathSet[p] = struct{}{}
	}
	for p := range input.Hunks {
		pathSet[p] = struct{}{}
	}
	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	return paths
}

func sortOffenders(offenders []Offender) {
	sort.Slice(offenders, func(i, j int) bool {
		mi := offenders[i].Cyclomatic
		if offenders[i].Cognitive > mi {
			mi = offenders[i].Cognitive
		}
		mj := offenders[j].Cyclomatic
		if offenders[j].Cognitive > mj {
			mj = offenders[j].Cognitive
		}
		if mi != mj {
			return mi > mj
		}
		if offenders[i].File != offenders[j].File {
			return offenders[i].File < offenders[j].File
		}
		return offenders[i].Line < offenders[j].Line
	})
}

// OffendersForFileDiffs runs the hunk-bounded complexity scan over per-file
// unified diffs (git diff per-file sections, see SplitFileDiffs) for repoRoot.
// Only functions overlapping changed hunks in critical packages are reported,
// so pre-existing violations elsewhere can never fail a diff-based gate.
// Content is read from the working tree; test-file filtering is left to the
// caller (test offenders are returned like any other).
func OffendersForFileDiffs(repoRoot string, hunks map[string][]byte) ([]Offender, []string) {
	return offendersFromHunks(lens.LensInput{Repo: repoRoot, Hunks: hunks})
}

// SplitFileDiffs splits a unified diff (git diff output) into per-file
// sections keyed by new path. Deleted files (new path /dev/null) are
// skipped: there is no content left to scan.
func SplitFileDiffs(diff string) map[string][]byte {
	out := make(map[string][]byte)
	var cur []string
	curPath := ""
	flush := func() {
		if curPath != "" && len(cur) > 0 {
			out[curPath] = []byte(strings.Join(cur, "\n"))
		}
		cur = nil
		curPath = ""
	}
	for line := range strings.SplitSeq(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			curPath = newPathFromDiffHeader(strings.TrimPrefix(line, "diff --git "))
			if curPath == "" || curPath == "dev/null" || curPath == "/dev/null" {
				curPath = ""
				continue
			}
		}
		if curPath != "" {
			cur = append(cur, line)
		}
	}
	flush()
	return out
}

// newPathFromDiffHeader extracts the new (b/) path from a diff --git header.
// Handles quoted paths with spaces ("a/pa th" "b/pa th"); returns ""
// when unparseable.
func newPathFromDiffHeader(rest string) string {
	s := strings.TrimSpace(rest)
	var b string
	if strings.HasPrefix(s, `"`) {
		end := strings.Index(s[1:], `"`)
		if end < 0 {
			return ""
		}
		b = strings.TrimSpace(s[1+end+1:])
	} else {
		fields := strings.Fields(s)
		if len(fields) < 2 {
			return ""
		}
		b = fields[1]
	}
	b = strings.Trim(b, `"`)
	if !strings.HasPrefix(b, "b/") {
		return ""
	}
	return strings.TrimPrefix(b, "b/")
}

func offendersFromHunks(input lens.LensInput) ([]Offender, []string) {
	var warnings []string
	if input.Repo != "" && !filepath.IsAbs(input.Repo) {
		warnings = append(warnings, fmt.Sprintf("warning: repo path %q is relative, using fallback", input.Repo)) //lint:ignore no-fmtSprintf
	}
	paths := collectCandidatePaths(input)
	var offenders []Offender
	for _, p := range paths {
		if !isCriticalPackage(p) || !strings.HasSuffix(strings.ToLower(p), ".go") {
			continue
		}
		_, inSummary := input.DiffSummary[p]
		inPaths := slices.Contains(input.Paths, p)
		_, inHunks := input.Hunks[p]
		if !inSummary && !inPaths && !inHunks {
			continue
		}
		content, changedRanges, warns, skip := resolveContentForPath(p, input)
		warnings = append(warnings, warns...)
		if skip || len(content) == 0 {
			continue
		}
		offs := offendersInHunk(p, content, changedRanges)
		offenders = append(offenders, offs...)
	}
	sortOffenders(offenders)
	return offenders, warnings
}
