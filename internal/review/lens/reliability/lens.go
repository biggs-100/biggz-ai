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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
)

// Lens is the R3 reliability heuristic. It is stateless and pure against the
// frozen LensInput (RiskInput + hunks + Truncated).
type Lens struct{}

// ID returns the stable lens identifier.
func (l *Lens) ID() string { return "reliability" }

// Analyze runs the R3 heuristic against the frozen input.
//
// Missing-test findings are inferential with ProofRefs file:1.
// Error-token findings are inferential with ProofRefs file:line derived from
// hunk line scan. No volume findings are emitted. Hunks are the sole source
// for error-token scan; no full-file fallback is performed.
func (l *Lens) Analyze(_ context.Context, input lens.LensInput) (lens.LensResult, error) {
	result := lens.LensResult{
		LensID:    l.ID(),
		Findings:  nil,
		Evidence:  nil,
		Truncated: input.Truncated,
	}

	// Collect candidate Go files from DiffSummary, Paths, and Hunks keys.
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
	allPaths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		allPaths = append(allPaths, p)
	}
	sort.Strings(allPaths)

	// Track which base names have a test file present in the change or on disk.
	goFiles := []string{}
	testBases := make(map[string]struct{})
	for _, p := range allPaths {
		lower := strings.ToLower(p)
		if strings.HasSuffix(lower, "_test.go") {
			base := strings.TrimSuffix(p, "_test.go")
			testBases[strings.ToLower(base)] = struct{}{}
		} else if strings.HasSuffix(lower, ".go") {
			goFiles = append(goFiles, p)
		}
	}
	sort.Strings(goFiles)

	// Missing-test findings: inferential, ProofRefs file:1.
	missingFindings := make([]lens.LensFinding, 0)
	for idx, p := range goFiles {
		base := strings.TrimSuffix(p, ".go")
		if _, hasTest := testBases[strings.ToLower(base)]; hasTest {
			continue
		}
		// Also check if sibling test file exists on disk when Repo is available.
		if input.Repo != "" {
			testPath := base + "_test.go"
			full := filepath.Join(input.Repo, testPath)
			if _, err := os.Stat(full); err == nil {
				continue
			}
		}
		id := fmt.Sprintf("R3-missing-test-%03d", idx+1)
		msg := fmt.Sprintf("reliability: %s has no sibling _test.go — consider adding tests", p)
		proof := fmt.Sprintf("%s:1", p)
		finding := lens.LensFinding{
			ID:        id,
			LensID:    l.ID(),
			Message:   msg,
			File:      p,
			Line:      1,
			ProofRefs: []string{proof},
			Class:     review.EvidenceInferential,
			Severity:  "warning",
		}
		missingFindings = append(missingFindings, finding)
		result.Evidence = append(result.Evidence, fmt.Sprintf("missing sibling test for %s", p))
	}

	// Error-token findings: inferential, hunk-bound scan.
	// Tokens that indicate error-handling concern when present in hunks.
	errorTokens := []string{
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

	tokenFindings := make([]lens.LensFinding, 0)
	// Stable hunk iteration order.
	hunkKeys := make([]string, 0, len(input.Hunks))
	for k := range input.Hunks {
		hunkKeys = append(hunkKeys, k)
	}
	sort.Strings(hunkKeys)

	for _, p := range hunkKeys {
		if !strings.HasSuffix(strings.ToLower(p), ".go") {
			continue
		}
		hunk := input.Hunks[p]
		if len(hunk) == 0 {
			continue
		}
		lines := strings.Split(string(hunk), "\n")
		for lineIdx, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// Strip unified diff prefix (+, -, space) for token scan.
			content := trimmed
			if len(content) > 0 && (content[0] == '+' || content[0] == '-' || content[0] == ' ') {
				content = strings.TrimSpace(content[1:])
			}
			// Also scan raw line for tokens that may appear with prefix.
			hit := ""
			for _, tok := range errorTokens {
				normalizedTok := strings.TrimSpace(tok)
				if strings.Contains(content, normalizedTok) || strings.Contains(line, normalizedTok) {
					hit = normalizedTok
					break
				}
			}
			if hit == "" {
				continue
			}
			// Avoid duplicate: if we already emitted for this file+line token, skip?
			// Emit one per line hit to keep ≥15 test coverage realistic.
			lineNum := lineIdx + 1
			proof := fmt.Sprintf("%s:%d", p, lineNum)
			id := fmt.Sprintf("R3-error-token-%03d", len(tokenFindings)+1)
			msg := fmt.Sprintf("reliability: %s contains error-handling token %q — verify error handling", p, hit)
			finding := lens.LensFinding{
				ID:        id,
				LensID:    l.ID(),
				Message:   msg,
				File:      p,
				Line:      lineNum,
				ProofRefs: []string{proof},
				Class:     review.EvidenceInferential,
				Severity:  "warning",
			}
			tokenFindings = append(tokenFindings, finding)
			result.Evidence = append(result.Evidence, fmt.Sprintf("error token %q at %s", hit, proof))
		}
	}

	result.Findings = append(missingFindings, tokenFindings...)

	if len(result.Findings) == 0 && len(result.Evidence) == 0 {
		result.Evidence = []string{"reliability: no issues detected"}
	}

	return result, nil
}
