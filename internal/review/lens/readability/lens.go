// Package readability implements the R2 ReadabilityLens heuristic.
//
// R2 emits:
//   - deterministic finding on go/parser.ParseFile failure for changed .go files
//   - inferential finding when DiffSummary[path] > 400 (any file) or > 200 (Go)
//
// It does NOT check mixedCase+underscores. ProofRefs are file:line derived
// from parser error position or hunk start. Truncated propagates from
// LensInput (8MiB cap). EvidenceClass is deterministic for parser failures,
// inferential for thresholds. The lens never imports plugin/ or planner/graph.
package readability

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
)

// Lens is the R2 readability heuristic. It is stateless and pure against the
// frozen LensInput (RiskInput + hunks + Truncated).
type Lens struct{}

// ID returns the stable lens identifier.
func (l *Lens) ID() string { return "readability" }

// Analyze runs the R2 heuristic against the frozen input.
//
// Parser failures are deterministic with concrete ProofRefs; threshold
// findings are inferential. Hunks are used as file source when present;
// otherwise the Repo root is consulted when available. Truncated is
// propagated to the result without error.
func (l *Lens) Analyze(_ context.Context, input lens.LensInput) (lens.LensResult, error) {
	result := lens.LensResult{
		LensID:    l.ID(),
		Findings:  nil,
		Evidence:  nil,
		Truncated: input.Truncated,
	}

	// Stable order for deterministic IDs and snapshot tests.
	paths := sortedKeys(input.DiffSummary)
	// Also consider Paths that are not in DiffSummary (e.g., empty diff
	// but hunks present). Merge for parser check.
	pathSet := make(map[string]struct{}, len(paths)+len(input.Paths))
	for _, p := range paths {
		pathSet[p] = struct{}{}
	}
	for _, p := range input.Paths {
		if _, ok := pathSet[p]; !ok {
			paths = append(paths, p)
			pathSet[p] = struct{}{}
		}
	}
	sort.Strings(paths)

	// Threshold findings: DiffSummary-based, inferential, ProofRefs file:1.
	thresholdFindings := make([]lens.LensFinding, 0)
	for _, p := range paths {
		lines, hasSummary := input.DiffSummary[p]
		if !hasSummary {
			continue
		}
		isGo := strings.HasSuffix(strings.ToLower(p), ".go")
		threshold := 0
		exceeds := false
		if lines > 400 {
			threshold = 400
			exceeds = true
		} else if isGo && lines > 200 {
			threshold = 200
			exceeds = true
		}
		if !exceeds {
			continue
		}
		idx := len(thresholdFindings) + 1
		// IDs are R2-prefixed to bind to the readability lens.
		id := fmt.Sprintf("R2-threshold-%03d", idx)
		// Deduplicate: only one threshold finding per path (above logic).
		msg := fmt.Sprintf("readability: %s has %d changed lines — exceeds %d-line readability boundary", p, lines, threshold)
		proof := fmt.Sprintf("%s:1", p)
		finding := lens.LensFinding{
			ID:        id,
			LensID:    l.ID(),
			Message:   msg,
			File:      p,
			Line:      1,
			ProofRefs: []string{proof},
			Class:     review.EvidenceInferential,
			Severity:  "info",
		}
		thresholdFindings = append(thresholdFindings, finding)
		result.Evidence = append(result.Evidence, fmt.Sprintf("%s: %d lines (threshold %d)", p, lines, threshold))
	}

	// Parser findings: deterministic, ProofRefs file:line from parser error.
	parserFindings := make([]lens.LensFinding, 0)
	for _, p := range paths {
		if !strings.HasSuffix(strings.ToLower(p), ".go") {
			continue
		}
		src, ok := input.Hunks[p]
		if !ok || len(src) == 0 {
			// Fallback to Repo file when Hunks absent but Repo available.
			if input.Repo != "" {
				// #nosec G304 -- Repo is the frozen input root, path is repo-relative.
				b, err := readRepoFile(input.Repo, p)
				if err != nil || len(b) == 0 {
					continue
				}
				src = b
			} else {
				continue
			}
		}
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, p, src, parser.AllErrors)
		if err == nil {
			continue
		}
		line := extractParserLine(p, src, fset, err)
		if line <= 0 {
			line = 1
		}
		proof := fmt.Sprintf("%s:%d", p, line)
		idx := len(parserFindings) + 1
		id := fmt.Sprintf("R2-parser-%03d", idx)
		msg := fmt.Sprintf("readability: %s fails go/parser: %v", p, err)
		finding := lens.LensFinding{
			ID:        id,
			LensID:    l.ID(),
			Message:   msg,
			File:      p,
			Line:      line,
			ProofRefs: []string{proof},
			Class:     review.EvidenceDeterministic,
			Severity:  "warning",
		}
		parserFindings = append(parserFindings, finding)
		result.Evidence = append(result.Evidence, fmt.Sprintf("parser failure %s: %v", proof, err))
	}

	// Complexity findings: hunk-bounded, inferential, via DeriveRiskInput, no second diff.
	complexityFindings := make([]lens.LensFinding, 0)
	offenders, cWarnings := offendersFromHunks(input)
	// Surface warnings (rename/no-map, repo path fallback) as evidence, never block.
	for _, w := range cWarnings {
		result.Evidence = append(result.Evidence, w)
	}
	cycloIdx := 1
	cognitIdx := 1
	for _, o := range offenders {
		isTest := isTestFile(o.File)
		sev := "warning"
		if isTest {
			sev = "info"
		}
		if o.Cyclomatic > CyclomaticThreshold {
			id := fmt.Sprintf("R2-CYCLO-%03d", cycloIdx)
			cycloIdx++
			msg := fmt.Sprintf("readability: %s in %s:%d has cyclomatic %d >%d", o.Function, o.File, o.Line, o.Cyclomatic, CyclomaticThreshold)
			proof := fmt.Sprintf("%s:%d: %s %d >%d", o.File, o.Line, o.Function, o.Cyclomatic, CyclomaticThreshold)
			finding := lens.LensFinding{
				ID:        id,
				LensID:    l.ID(),
				Message:   msg,
				File:      o.File,
				Line:      o.Line,
				ProofRefs: []string{proof},
				Class:     review.EvidenceInferential,
				Severity:  sev,
			}
			if isTest {
				finding.Message += " (informational test file)"
			}
			complexityFindings = append(complexityFindings, finding)
			result.Evidence = append(result.Evidence, proof)
		}
		if o.Cognitive > CognitiveThreshold {
			id := fmt.Sprintf("R2-COGNIT-%03d", cognitIdx)
			cognitIdx++
			msg := fmt.Sprintf("readability: %s in %s:%d has cognitive %d >%d", o.Function, o.File, o.Line, o.Cognitive, CognitiveThreshold)
			proof := fmt.Sprintf("%s:%d: %s %d >%d", o.File, o.Line, o.Function, o.Cognitive, CognitiveThreshold)
			finding := lens.LensFinding{
				ID:        id,
				LensID:    l.ID(),
				Message:   msg,
				File:      o.File,
				Line:      o.Line,
				ProofRefs: []string{proof},
				Class:     review.EvidenceInferential,
				Severity:  sev,
			}
			if isTest {
				finding.Message += " (informational test file)"
			}
			complexityFindings = append(complexityFindings, finding)
			result.Evidence = append(result.Evidence, proof)
		}
	}

	// Merge findings: parser (deterministic) first for stable order, then thresholds, then complexity.
	result.Findings = append(parserFindings, thresholdFindings...)
	result.Findings = append(result.Findings, complexityFindings...)

	// Ensure Evidence is concrete: if no findings, provide one concrete entry
	// to satisfy downstream hash stability (optional).
	if len(result.Findings) == 0 && len(result.Evidence) == 0 {
		result.Evidence = []string{"readability: no issues detected"}
	}

	return result, nil
}

// sortedKeys returns sorted keys of m for deterministic iteration.
func sortedKeys(m map[string]int) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// extractParserLine attempts to derive the line number of the first parser
// error from the FileSet or error string. It falls back to scanning the error
// text for ":line:".
func extractParserLine(_ string, _ []byte, fset *token.FileSet, err error) int {
	// Try FileSet positions: parser may have added a file to the set even on error.
	// We scan all files for the first position; simpler than parsing error type.
	if fset != nil {
		fset.Iterate(func(f *token.File) bool {
			// f.LineCount may be zero on error; ignore
			_ = f
			return true
		})
	}
	// Fallback: parse error string like "foo.go:2:3: message".
	msg := err.Error()
	// Regex captures ":line:" after the file path.
	re := regexp.MustCompile(`:(\d+):`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) == 2 {
		var line int
		_, _ = fmt.Sscanf(matches[1], "%d", &line)
		if line > 0 {
			return line
		}
	}
	// Alternate: token position encoded as "1:10: ..."
	re2 := regexp.MustCompile(`^(\d+):\d+:`)
	m2 := re2.FindStringSubmatch(msg)
	if len(m2) == 2 {
		var line int
		_, _ = fmt.Sscanf(m2[1], "%d", &line)
		if line > 0 {
			return line
		}
	}
	return 1
}

// readRepoFile reads a repo-relative path under root.
func readRepoFile(root, rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(root, rel))
}
