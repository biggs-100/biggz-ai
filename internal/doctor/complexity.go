package doctor

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fzipp/gocyclo"
	"github.com/uudashr/gocognit"
)

const (
	ComplexityCheckID CheckID = "complexity"
	complexityTimeout         = 10 * time.Second
)

const (
	cyclomaticThreshold = 15
	cognitiveThreshold  = 20
)

var criticalRoots = []string{
	"internal/review",
	"internal/sdd",
	"internal/verification",
}

// ComplexityOffender mirrors the lens offender for JSON output.
type ComplexityOffender struct {
	Package    string `json:"package"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Function   string `json:"function"`
	Cyclomatic int    `json:"cyclomatic"`
	Cognitive  int    `json:"cognitive"`
}

// ComplexityDetails is the JSON details for ComplexityCheck.
type ComplexityDetails struct {
	Offenders            []ComplexityOffender `json:"offenders"`
	TestOffenders        []ComplexityOffender `json:"test_offenders,omitempty"`
	TotalFuncs           int                  `json:"total_funcs"`
	CyclomaticViolations int                  `json:"cyclomatic_violations"`
	CognitiveViolations  int                  `json:"cognitive_violations"`
}

// ComplexityCheck scans critical packages for cyclomatic/cognitive violations.
// It is CostQuick/read-only, panic-isolated via Runner and internal recover.
type ComplexityCheck struct {
	roots   []string
	timeout time.Duration
}

// NewComplexityCheck creates a ComplexityCheck with default roots and timeout.
func NewComplexityCheck() *ComplexityCheck {
	return &ComplexityCheck{
		roots:   criticalRoots,
		timeout: complexityTimeout,
	}
}

// NewComplexityCheckWithCustom creates a check with injected roots/timeout for testing.
func NewComplexityCheckWithCustom(roots []string, timeout time.Duration) *ComplexityCheck {
	if roots == nil {
		roots = criticalRoots
	}
	if timeout == 0 {
		timeout = complexityTimeout
	}
	return &ComplexityCheck{roots: roots, timeout: timeout}
}

// ID returns the check identifier.
func (c *ComplexityCheck) ID() CheckID { return ComplexityCheckID }

// Run executes the complexity scan with panic and timeout isolation.
func (c *ComplexityCheck) Run(ctx context.Context) (res *Result) {
	defer func() {
		if p := recover(); p != nil {
			res = &Result{
				ID:       ComplexityCheckID,
				Status:   StatusWarn,
				Message:  fmt.Sprintf("ComplexityCheck panicked: %v", p),
				Severity: SeverityWarning,
				Error:    fmt.Sprintf("panic: %v", p),
			}
		}
	}()

	// Enforce timeout
	runCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	type scanResult struct {
		offenders     []ComplexityOffender
		testOffenders []ComplexityOffender
		totalFuncs    int
		cycloCount    int
		cogCount      int
		err           error
	}
	ch := make(chan scanResult, 1)
	go func() {
		offs, testOffs, total, cyclo, cog, err := c.scan(runCtx)
		ch <- scanResult{offenders: offs, testOffenders: testOffs, totalFuncs: total, cycloCount: cyclo, cogCount: cog, err: err}
	}()

	select {
	case <-runCtx.Done():
		return &Result{
			ID:       ComplexityCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("ComplexityCheck timed out after %s: %v", c.timeout, runCtx.Err()),
			Severity: SeverityWarning,
			Error:    runCtx.Err().Error(),
		}
	case r := <-ch:
		if r.err != nil {
			return &Result{
				ID:       ComplexityCheckID,
				Status:   StatusWarn,
				Message:  fmt.Sprintf("ComplexityCheck scan error: %v", r.err),
				Severity: SeverityWarning,
				Error:    r.err.Error(),
			}
		}
		return c.buildResult(r.offenders, r.testOffenders, r.totalFuncs, r.cycloCount, r.cogCount)
	}
}

func (c *ComplexityCheck) buildResult(offenders, testOffenders []ComplexityOffender, totalFuncs, cycloCount, cogCount int) *Result {
	details := ComplexityDetails{
		Offenders:            offenders,
		TestOffenders:        testOffenders,
		TotalFuncs:           totalFuncs,
		CyclomaticViolations: cycloCount,
		CognitiveViolations:  cogCount,
	}
	// Sort already done in scan; ensure top offenders for message
	// Count blocking violations (non-test)
	blocking := len(offenders)
	testCount := len(testOffenders)

	if blocking > 0 {
		// Build table
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Complexity WARNING: %d blocking violation(s) in critical packages (cyclomatic >%d, cognitive >%d)\n", blocking, cyclomaticThreshold, cognitiveThreshold))
		sb.WriteString(fmt.Sprintf("Totals: %d functions scanned, %d cyclomatic violations, %d cognitive violations", totalFuncs, cycloCount, cogCount))
		if testCount > 0 {
			sb.WriteString(fmt.Sprintf(" (%d informational test file violations)", testCount))
		}
		sb.WriteString("\n")
		sb.WriteString("Top offenders (sorted by max complexity):\n")
		sb.WriteString(fmt.Sprintf("%-50s %-30s %6s %6s\n", "File:Line", "Function", "Cyclo", "Cognit"))
		sb.WriteString(strings.Repeat("-", 100) + "\n")
		limit := 10
		if len(offenders) < limit {
			limit = len(offenders)
		}
		for i := 0; i < limit; i++ {
			o := offenders[i]
			sb.WriteString(fmt.Sprintf("%-50s %-30s %6d %6d\n", fmt.Sprintf("%s:%d", o.File, o.Line), o.Function, o.Cyclomatic, o.Cognitive))
		}
		if testCount > 0 {
			sb.WriteString(fmt.Sprintf("\nInformational test offenders: %d (never block)\n", testCount))
			for _, o := range testOffenders {
				sb.WriteString(fmt.Sprintf("  %s:%d %s cyclo=%d cognit=%d\n", o.File, o.Line, o.Function, o.Cyclomatic, o.Cognitive))
			}
		}
		return &Result{
			ID:       ComplexityCheckID,
			Status:   StatusWarn,
			Message:  sb.String(),
			Severity: SeverityWarning,
			Details:  details,
		}
	}
	// No blocking violations
	msg := fmt.Sprintf("Complexity OK: 0 violations in critical packages (%d functions scanned, %d cyclomatic, %d cognitive)", totalFuncs, cycloCount, cogCount)
	if testCount > 0 {
		msg += fmt.Sprintf(" — %d informational test file violations", testCount)
		for _, o := range testOffenders {
			msg += fmt.Sprintf("\n  informational: %s:%d %s cyclo=%d cognit=%d", o.File, o.Line, o.Function, o.Cyclomatic, o.Cognitive)
		}
	}
	return &Result{
		ID:       ComplexityCheckID,
		Status:   StatusPass,
		Message:  msg,
		Severity: SeverityInfo,
		Details:  details,
	}
}

func (c *ComplexityCheck) scan(ctx context.Context) ([]ComplexityOffender, []ComplexityOffender, int, int, int, error) {
	var offenders []ComplexityOffender
	var testOffenders []ComplexityOffender
	totalFuncs := 0
	cycloCount := 0
	cogCount := 0

	for _, root := range c.roots {
		select {
		case <-ctx.Done():
			return offenders, testOffenders, totalFuncs, cycloCount, cogCount, ctx.Err()
		default:
		}
		// WalkDir
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				// Skip vendor, testdata, hidden
				base := filepath.Base(path)
				if base == "vendor" || base == "testdata" || strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(path), ".go") {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
			if err != nil {
				return nil
			}
			isTest := strings.HasSuffix(path, "_test.go")
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				totalFuncs++
				cyclo := gocyclo.Complexity(fn)
				cog := gocognit.Complexity(fn)
				isCycloViol := cyclo > cyclomaticThreshold
				isCogViol := cog > cognitiveThreshold
				if isCycloViol {
					cycloCount++
				}
				if isCogViol {
					cogCount++
				}
				if isCycloViol || isCogViol {
					off := ComplexityOffender{
						Package:    filepath.ToSlash(filepath.Dir(path)),
						File:       filepath.ToSlash(path),
						Line:       fset.Position(fn.Pos()).Line,
						Function:   funcNameForDoctor(fn),
						Cyclomatic: cyclo,
						Cognitive:  cog,
					}
					if isTest {
						testOffenders = append(testOffenders, off)
					} else {
						offenders = append(offenders, off)
					}
				}
			}
			return nil
		})
		if err != nil {
			// If context canceled, propagate
			if ctx.Err() != nil {
				return offenders, testOffenders, totalFuncs, cycloCount, cogCount, ctx.Err()
			}
			// Otherwise continue (e.g., root not found)
			continue
		}
	}

	// Sort offenders by max(cyclo,cog) descending
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
	sort.Slice(testOffenders, func(i, j int) bool {
		mi := testOffenders[i].Cyclomatic
		if testOffenders[i].Cognitive > mi {
			mi = testOffenders[i].Cognitive
		}
		mj := testOffenders[j].Cyclomatic
		if testOffenders[j].Cognitive > mj {
			mj = testOffenders[j].Cognitive
		}
		if mi != mj {
			return mi > mj
		}
		if testOffenders[i].File != testOffenders[j].File {
			return testOffenders[i].File < testOffenders[j].File
		}
		return testOffenders[i].Line < testOffenders[j].Line
	})

	// Deduplicate counts: cycloCount/cogCount currently counts per function violations, but spec wants per violation type totals.
	// We already incremented per function; adjust to ensure they match spec: totals are counts of violations by threshold (could double count same func for both).
	// Our counts already do that.

	return offenders, testOffenders, totalFuncs, cycloCount, cogCount, nil
}

func funcNameForDoctor(fn *ast.FuncDecl) string {
	if fn.Recv != nil && fn.Recv.NumFields() > 0 {
		typ := fn.Recv.List[0].Type
		switch t := typ.(type) {
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				return fmt.Sprintf("(%s).%s", "*"+ident.Name, fn.Name.Name)
			}
		case *ast.Ident:
			return fmt.Sprintf("(%s).%s", t.Name, fn.Name.Name)
		}
		return fmt.Sprintf("(%T).%s", typ, fn.Name.Name)
	}
	return fn.Name.Name
}

// Remedy returns nil — no automatic fix.
func (c *ComplexityCheck) Remedy() *Remedy { return nil }
