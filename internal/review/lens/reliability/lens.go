// Package reliability implements the R3 ReliabilityLens heuristic.
//
// R3 emits:
//   - inferential finding when a changed Go file lacks a sibling _test.go
//   - inferential finding on error-handling token hits within hunks
//
// It does NOT emit volume findings (no ChangedLines threshold). Findings are
// inferential only with ProofRefs file:line derived from hunk scan or file
// start. The lens is hunk-bound: error token scan uses Hunks map only, never
// falls back to reading full repository files. Missing-test check uses Paths,
// DiffSummary, Hunks key set, and optionally verifies sibling existence on disk
// when Repo is available, but does not read file content beyond the hunk.
// Truncated propagates without error. The lens never imports plugin/ or planner/graph.
package reliability

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
)

// Lens is the R3 reliability heuristic. It is stateless and pure against the
// frozen LensInput (RiskInput + hunks + Truncated).
type Lens struct{}

// ID returns the stable lens identifier.
func (l *Lens) ID() string { return "reliability" }

// ReliabilityPromptData is the inventory for r3-reliability.md template.
type ReliabilityPromptData struct {
	Repo         string
	ChangedLines int
	Paths        []string
	Diff         string
	Truncated    bool
	BaseTree     string
	Hunks        string
	Shared       string
}

func renderReliabilityPrompt(input lens.LensInput) (string, error) {
	data, err := assets.FS.ReadFile("prompts/review/r3-reliability.md")
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("r3-reliability.md").Option("missingkey=error").Parse(string(data))
	if err != nil {
		return "", err
	}
	pd := ReliabilityPromptData{
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
	keys := slices.Sorted(maps.Keys(hunks))
	var out []byte
	for _, k := range keys {
		out = append(out, hunks[k]...)
	}
	return out
}

// Analyze runs the R3 heuristic against the frozen input.
//
// Missing-test findings are inferential with ProofRefs file:1.
// Error-token findings are inferential with ProofRefs file:line derived from
// hunk line scan. No volume findings are emitted. Hunks are the sole source
// for error-token scan; no full-file fallback is performed.
func (l *Lens) Analyze(_ context.Context, input lens.LensInput) (lens.LensResult, error) {
	if _, err := renderReliabilityPrompt(input); err != nil {
		_ = err
	}
	result := lens.LensResult{
		LensID:    l.ID(),
		Truncated: input.Truncated,
	}
	allPaths := collectAllPaths(input)
	goFiles, testBases := splitGoFiles(allPaths)
	missingFindings, missingEvidence := findMissingTests(goFiles, testBases, input.Repo, l.ID())
	result.Evidence = append(result.Evidence, missingEvidence...)
	errorTokens := reliabilityErrorTokens()
	hunkKeys := slices.Sorted(maps.Keys(input.Hunks))
	var tokenFindings []lens.LensFinding
	var tokenEvidence []string
	for _, p := range hunkKeys {
		if !isGoFile(p) {
			continue
		}
		hunk := input.Hunks[p]
		if len(hunk) == 0 {
			continue
		}
		findings, evidence := analyzeFileReliability(p, hunk, l.ID(), len(tokenFindings), errorTokens)
		tokenFindings = append(tokenFindings, findings...)
		tokenEvidence = append(tokenEvidence, evidence...)
	}
	result.Evidence = append(result.Evidence, tokenEvidence...)
	result.Findings = append(missingFindings, tokenFindings...)
	if len(result.Findings) == 0 && len(result.Evidence) == 0 {
		result.Evidence = []string{"reliability: no issues detected"}
	}
	return result, nil
}

// collectAllPaths merges DiffSummary, Paths, and Hunks keys into a sorted list.
func collectAllPaths(input lens.LensInput) []string {
	pathSet := make(map[string]struct{})
	for p := range input.DiffSummary {
		pathSet[p] = struct{}{}
	}
	for _, p := range input.Paths {
		pathSet[p] = struct{}{}
	}
	for p := range input.Hunks {
		pathSet[p] = struct{}{}
	}
	return slices.Sorted(maps.Keys(pathSet))
}

// splitGoFiles separates Go source files from test bases.
func splitGoFiles(allPaths []string) ([]string, map[string]struct{}) {
	testBases := make(map[string]struct{})
	var goFiles []string
	for _, p := range allPaths {
		lower := strings.ToLower(p)
		if strings.HasSuffix(lower, "_test.go") {
			base := strings.TrimSuffix(p, "_test.go")
			testBases[strings.ToLower(base)] = struct{}{}
			continue
		}
		if strings.HasSuffix(lower, ".go") {
			goFiles = append(goFiles, p)
		}
	}
	slices.Sort(goFiles)
	return goFiles, testBases
}

// findMissingTests returns findings for Go files without sibling _test.go.
func findMissingTests(goFiles []string, testBases map[string]struct{}, repo, lensID string) ([]lens.LensFinding, []string) {
	var findings []lens.LensFinding
	var evidence []string
	for idx, p := range goFiles {
		if checkCoverage(p, testBases, repo) {
			continue
		}
		id := fmt.Sprintf("R3-missing-test-%03d", idx+1)                                         //lint:ignore no-fmtSprintf
		msg := fmt.Sprintf("reliability: %s has no sibling _test.go — consider adding tests", p) //lint:ignore no-fmtSprintf
		proof := fmt.Sprintf("%s:1", p)                                                          //lint:ignore no-fmtSprintf
		finding := lens.LensFinding{
			ID:        id,
			LensID:    lensID,
			Message:   msg,
			File:      p,
			Line:      1,
			ProofRefs: []string{proof},
			Class:     review.EvidenceInferential,
			Severity:  "warning",
		}
		findings = append(findings, finding)
		evidence = append(evidence, fmt.Sprintf("missing sibling test for %s", p)) //lint:ignore no-fmtSprintf
	}
	return findings, evidence
}

// checkCoverage reports whether path has a sibling test file.
func checkCoverage(path string, testBases map[string]struct{}, repo string) bool {
	base := strings.TrimSuffix(path, ".go")
	if _, has := testBases[strings.ToLower(base)]; has {
		return true
	}
	if repo != "" {
		testPath := base + "_test.go"
		full := filepath.Join(repo, testPath)
		if _, err := os.Stat(full); err == nil {
			return true
		}
	}
	return false
}

// reliabilityErrorTokens returns the error-handling tokens scanned in hunks.
func reliabilityErrorTokens() []string {
	return []string{
		"panic(",
		"log.Fatal",
		"log.Fatalf",
		"os.Exit",
		"errors.New",
		"fmt.Errorf",
		"err :=",
		"err : =",
		"err=",
		"if err != nil",
		"if err ==",
	}
}

// isGoFile reports whether path is a Go source file.
func isGoFile(p string) bool {
	return strings.HasSuffix(strings.ToLower(p), ".go")
}

// analyzeFileReliability scans a single hunk for error-handling tokens.
func analyzeFileReliability(path string, hunk []byte, lensID string, startIdx int, errorTokens []string) ([]lens.LensFinding, []string) {
	var findings []lens.LensFinding
	var evidence []string
	lines := strings.Split(string(hunk), "\n")
	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		content := trimmed
		if len(content) > 0 && (content[0] == '+' || content[0] == '-' || content[0] == ' ') {
			content = strings.TrimSpace(content[1:])
		}
		hit := checkErrorHandling(content, line, errorTokens)
		if hit == "" {
			continue
		}
		lineNum := lineIdx + 1
		proof := fmt.Sprintf("%s:%d", path, lineNum)                                                              //lint:ignore no-fmtSprintf
		id := fmt.Sprintf("R3-error-token-%03d", startIdx+len(findings)+1)                                         //lint:ignore no-fmtSprintf
		msg := fmt.Sprintf("reliability: %s contains error-handling token %q — verify error handling", path, hit) //lint:ignore no-fmtSprintf
		finding := lens.LensFinding{
			ID:        id,
			LensID:    lensID,
			Message:   msg,
			File:      path,
			Line:      lineNum,
			ProofRefs: []string{proof},
			Class:     review.EvidenceInferential,
			Severity:  "warning",
		}
		findings = append(findings, finding)
		evidence = append(evidence, fmt.Sprintf("error token %q at %s", hit, proof)) //lint:ignore no-fmtSprintf
	}
	return findings, evidence
}

// checkErrorHandling returns the first error token found in content/rawLine, or "".
func checkErrorHandling(content, rawLine string, errorTokens []string) string {
	for _, tok := range errorTokens {
		normalized := strings.TrimSpace(tok)
		if strings.Contains(content, normalized) || strings.Contains(rawLine, normalized) {
			return normalized
		}
	}
	return ""
}
