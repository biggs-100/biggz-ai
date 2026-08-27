package readability

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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

// parseHunkHeaders extracts changed line ranges (new file) from unified diff hunk headers.
// Headers look like "@@ -oldStart,oldLen +newStart,newLen @@".
func parseHunkHeaders(diff string) []lineRange {
	var out []lineRange
	lines := strings.Split(diff, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "@@") {
			continue
		}
		// Example: @@ -10,7 +10,7 @@ func Foo() {
		parts := strings.Fields(l)
		for _, p := range parts {
			if strings.HasPrefix(p, "+") {
				s := strings.TrimPrefix(p, "+")
				// s is "10,7" or "10"
				seg := strings.Split(s, ",")
				startStr := seg[0]
				start, err := strconv.Atoi(startStr)
				if err != nil || start <= 0 {
					continue
				}
				count := 1
				if len(seg) > 1 {
					c, err := strconv.Atoi(seg[1])
					if err == nil {
						count = c
					}
				}
				if count == 0 {
					// Deletion-only hunk (no new lines) -> no overlap
					continue
				}
				end := start + count - 1
				if end < start {
					end = start
				}
				out = append(out, lineRange{start: start, end: end})
				break
			}
		}
	}
	return out
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

// offendersFromHunks derives complexity offenders that intersect the PR hunk set.
// It reuses the single DeriveRiskInput derivation (Paths/DiffSummary/Hunks/Repo) and
// does NOT run a second git diff. It is hunk-bounded: only functions whose
// line range overlaps a changed hunk interval are considered. Critical-package
// and *_test.go filtering is applied; test files are collected but flagged for
// informational treatment by the caller. Warnings include repo-path fallback and
// rename/no-map cases.
func offendersFromHunks(input lens.LensInput) ([]Offender, []string) {
	var warnings []string
	var offenders []Offender

	// Git repo path selection: absolute preferred, relative fallback warns (threat matrix)
	if input.Repo != "" && !filepath.IsAbs(input.Repo) {
		warnings = append(warnings, fmt.Sprintf("warning: repo path %q is relative, using fallback", input.Repo)) //lint:ignore no-fmtSprintf
	}

	// Build set of candidate paths: Paths ∪ DiffSummary keys ∪ Hunks keys
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
	sort.Strings(paths)

	for _, p := range paths {
		if !isCriticalPackage(p) {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(p), ".go") {
			continue
		}
		// Determine if file is considered changed (has diff entry or in Paths)
		_, inSummary := input.DiffSummary[p]
		inPaths := false
		for _, cp := range input.Paths {
			if cp == p {
				inPaths = true
				break
			}
		}
		inHunks := false
		hunkBytes, hasHunk := input.Hunks[p]
		if hasHunk {
			inHunks = true
		}
		// If file has no evidence of being changed, skip (grandfather: legacy not in hunk)
		if !inSummary && !inPaths && !inHunks {
			continue
		}
		// If rename or ambiguous diff (e.g., hunk indicates rename but no mappable func), spec says warn and not block.
		// Detect rename: if diff summary missing but path appears only via Hunks with diff that has no parseable hunks?
		// We emit warning when Hunks contains diff-like content but we cannot map to changed funcs.
		var content []byte
		var changedRanges []lineRange
		isDiff := hasHunk && bytes.Contains(hunkBytes, []byte("@@"))
		if isDiff {
			changedRanges = parseHunkHeaders(string(hunkBytes))
			if len(changedRanges) == 0 {
				warnings = append(warnings, fmt.Sprintf("warning: file %s has diff but no mappable hunk ranges (rename or ambiguous diff)", p)) //lint:ignore no-fmtSprintf
				continue
			}
			// Diff hunk != source; need repo file for parsing
			if input.Repo != "" {
				repoPath, warn := resolveRepoPath(input.Repo, p)
				if warn != "" {
					warnings = append(warnings, warn)
				}
				b, err := os.ReadFile(repoPath)
				if err != nil {
					// Fallback: try relative
					b2, err2 := os.ReadFile(p)
					if err2 != nil {
						warnings = append(warnings, fmt.Sprintf("warning: cannot read %s for diff-based complexity: %v", p, err)) //lint:ignore no-fmtSprintf
						continue
					}
					content = b2
					if !filepath.IsAbs(input.Repo) {
						// already warned
					} else {
						warnings = append(warnings, fmt.Sprintf("warning: fallback relative read for %s", p)) //lint:ignore no-fmtSprintf
					}
				} else {
					content = b
				}
			} else {
				// No repo, cannot map diff without source; warn fallback
				warnings = append(warnings, fmt.Sprintf("warning: no repo to resolve diff for %s", p)) //lint:ignore no-fmtSprintf
				continue
			}
		} else if hasHunk && len(hunkBytes) > 0 {
			// Hunk is treated as file content (full file) or partial snippet
			// Try to parse as file; if fails, try repo fallback
			content = hunkBytes
			// Consider whole file as changed interval if file is in changed set.
			// For full-file hunks, we treat every line as changed (conservative) but still bounded at file level.
			// To enable function-level hunk-bounded when Hunks is full file, we need more info; we treat as changed if file is in Paths/DiffSummary.
			if inSummary || inPaths {
				changedRanges = []lineRange{{start: 1, end: 1000000}}
			} else {
				continue
			}
			// Validate it parses as Go; if not, fallback to repo
			fsetTmp := token.NewFileSet()
			if _, err := parser.ParseFile(fsetTmp, p, content, parser.ParseComments); err != nil && input.Repo != "" {
				repoPath, warn := resolveRepoPath(input.Repo, p)
				if warn != "" {
					warnings = append(warnings, warn)
				}
				if b, err2 := os.ReadFile(repoPath); err2 == nil {
					content = b
				}
			}
		} else {
			// No hunk content, try reading from repo or working dir
			if input.Repo != "" {
				repoPath, warn := resolveRepoPath(input.Repo, p)
				if warn != "" {
					warnings = append(warnings, warn)
				}
				b, err := os.ReadFile(repoPath)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("warning: cannot read %s: %v", p, err)) //lint:ignore no-fmtSprintf
					continue
				}
				content = b
			} else {
				b, err := os.ReadFile(p)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("warning: cannot read %s: %v", p, err)) //lint:ignore no-fmtSprintf
					continue
				}
				content = b
			}
			if inSummary || inPaths {
				changedRanges = []lineRange{{start: 1, end: 1000000}}
			} else {
				continue
			}
		}

		if len(content) == 0 {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, content, parser.ParseComments)
		if err != nil {
			// Parser failure is handled by lens separately; not complexity
			continue
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
			if cyclo > CyclomaticThreshold || cog > CognitiveThreshold {
				// For test files, still collect but caller will treat as informational
				offenders = append(offenders, Offender{
					Package:    packageForPath(p),
					File:       p,
					Function:   funcName(fn),
					Line:       start,
					Cyclomatic: cyclo,
					Cognitive:  cog,
				})
			}
		}
	}

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

	return offenders, warnings
}
