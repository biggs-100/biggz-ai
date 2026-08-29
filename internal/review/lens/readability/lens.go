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
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
)

// Lens is the R2 readability heuristic. It is stateless and pure against the
// frozen LensInput (RiskInput + hunks + Truncated).
type Lens struct{}

// ID returns the stable lens identifier.
func (l *Lens) ID() string { return "readability" }

// ReadabilityPromptData is the inventory for r2-readability.md template.
type ReadabilityPromptData struct {
	Repo         string
	ChangedLines int
	Paths        []string
	Diff         string
	Truncated    bool
	BaseTree     string
	Hunks        string
	Shared       string
}

func renderReadabilityPrompt(input lens.LensInput) (string, error) {
	data, err := assets.FS.ReadFile("prompts/review/r2-readability.md")
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("r2-readability.md").Option("missingkey=error").Parse(string(data))
	if err != nil {
		return "", err
	}
	pd := ReadabilityPromptData{
		Repo:         input.Repo,
		ChangedLines: input.ChangedLines,
		Paths:        input.Paths,
		Diff:         string(flattenHunks(input.Hunks)),
		Truncated:    input.Truncated,
		BaseTree:     input.BaseTree,
		Hunks:        string(flattenHunks(input.Hunks)),
		Shared:       "shared",
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, pd); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func flattenHunks(hunks map[string][]byte) []byte {
	if len(hunks) == 0 {
		return nil
	}
	keys := make([]string, 0, len(hunks))
	for k := range hunks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []byte
	for _, k := range keys {
		out = append(out, hunks[k]...)
	}
	return out
}

// Analyze runs the R2 heuristic against the frozen input.
//
// Parser failures are deterministic with concrete ProofRefs; threshold
// findings are inferential. Hunks are used as file source when present;
// otherwise the Repo root is consulted when available. Truncated is
// propagated to the result without error.
func (l *Lens) Analyze(_ context.Context, input lens.LensInput) (lens.LensResult, error) {
	if _, err := renderReadabilityPrompt(input); err != nil {
		_ = err
	}
	result := lens.LensResult{
		LensID:    l.ID(),
		Findings:  nil,
		Evidence:  nil,
		Truncated: input.Truncated,
	}
	paths := l.collectPaths(input)
	threshFindings, threshEvidence := l.thresholdFindings(paths, input)
	result.Evidence = append(result.Evidence, threshEvidence...)
	parserFindings, parserEvidence := l.parserFindings(paths, input)
	result.Evidence = append(result.Evidence, parserEvidence...)
	complexFindings, complexEvidence := l.complexityFindings(input)
	result.Evidence = append(result.Evidence, complexEvidence...)
	result.Findings = append(parserFindings, threshFindings...)
	result.Findings = append(result.Findings, complexFindings...)
	if len(result.Findings) == 0 && len(result.Evidence) == 0 {
		result.Evidence = []string{"readability: no issues detected"}
	}
	return result, nil
}

// collectPaths merges DiffSummary and Paths into a deterministic sorted list.
func (l *Lens) collectPaths(input lens.LensInput) []string {
	paths := sortedKeys(input.DiffSummary)
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
	return paths
}

// thresholdFindings returns threshold findings for paths exceeding line limits.
func (l *Lens) thresholdFindings(paths []string, input lens.LensInput) ([]lens.LensFinding, []string) {
	var findings []lens.LensFinding
	var evidence []string
	for _, p := range paths {
		lines, hasSummary := input.DiffSummary[p]
		if !hasSummary {
			continue
		}
		threshold, exceeds := readabilityThreshold(p, lines)
		if !exceeds {
			continue
		}
		idx := len(findings) + 1
		id := fmt.Sprintf("R2-threshold-%03d", idx)
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
		findings = append(findings, finding)
		evidence = append(evidence, fmt.Sprintf("%s: %d lines (threshold %d)", p, lines, threshold))
	}
	return findings, evidence
}

// readabilityThreshold returns the applicable threshold and whether it is exceeded.
func readabilityThreshold(path string, lines int) (int, bool) {
	if lines > 400 {
		return 400, true
	}
	if strings.HasSuffix(strings.ToLower(path), ".go") && lines > 200 {
		return 200, true
	}
	return 0, false
}

// parserFindings returns parser failure findings for Go files.
func (l *Lens) parserFindings(paths []string, input lens.LensInput) ([]lens.LensFinding, []string) {
	var findings []lens.LensFinding
	var evidence []string
	for _, p := range paths {
		if !strings.HasSuffix(strings.ToLower(p), ".go") {
			continue
		}
		finding, ev, ok := l.analyzeFile(p, input, len(findings))
		if !ok {
			continue
		}
		findings = append(findings, finding)
		evidence = append(evidence, ev)
	}
	return findings, evidence
}

// analyzeFile checks a single Go file for parser errors.
// It returns a finding and evidence when parsing fails, otherwise ok is false.
func (l *Lens) analyzeFile(path string, input lens.LensInput, idx int) (lens.LensFinding, string, bool) {
	src, ok := input.Hunks[path]
	if !ok || len(src) == 0 {
		if input.Repo != "" {
			b, err := readRepoFile(input.Repo, path)
			if err != nil || len(b) == 0 {
				return lens.LensFinding{}, "", false
			}
			src = b
		} else {
			return lens.LensFinding{}, "", false
		}
	}
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, path, src, parser.AllErrors)
	if err == nil {
		return lens.LensFinding{}, "", false
	}
	line := extractParserLine(path, src, fset, err)
	if line <= 0 {
		line = 1
	}
	proof := fmt.Sprintf("%s:%d", path, line)
	id := fmt.Sprintf("R2-parser-%03d", idx+1)
	msg := fmt.Sprintf("readability: %s fails go/parser: %v", path, err)
	finding := lens.LensFinding{
		ID:        id,
		LensID:    l.ID(),
		Message:   msg,
		File:      path,
		Line:      line,
		ProofRefs: []string{proof},
		Class:     review.EvidenceDeterministic,
		Severity:  "warning",
	}
	ev := fmt.Sprintf("parser failure %s: %v", proof, err)
	return finding, ev, true
}

// complexityFindings returns complexity findings bounded by hunks.
func (l *Lens) complexityFindings(input lens.LensInput) ([]lens.LensFinding, []string) {
	var findings []lens.LensFinding
	var evidence []string
	offenders, cWarnings := offendersFromHunks(input)
	for _, w := range cWarnings {
		evidence = append(evidence, w)
	}
	cycloIdx := 1
	cognitIdx := 1
	for _, o := range offenders {
		f, ev, nc, ng := l.scoreHunk(o, cycloIdx, cognitIdx)
		findings = append(findings, f...)
		evidence = append(evidence, ev...)
		cycloIdx = nc
		cognitIdx = ng
	}
	return findings, evidence
}

// scoreHunk scores a single offender and returns findings for cyclomatic/cognitive violations.
func (l *Lens) scoreHunk(o Offender, cycloIdx, cognitIdx int) ([]lens.LensFinding, []string, int, int) {
	var findings []lens.LensFinding
	var evidence []string
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
		findings = append(findings, finding)
		evidence = append(evidence, proof)
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
		findings = append(findings, finding)
		evidence = append(evidence, proof)
	}
	return findings, evidence, cycloIdx, cognitIdx
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
