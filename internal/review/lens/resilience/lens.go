// Package resilience implements the R4 ResilienceLens heuristic.
//
// R4 is hunk-bounded, inferential-only for timeout/context/concurrency/cleanup.
// It never falls back to full file reads. Hunks are capped at 8MiB total;
// when exceeded, Truncated is set and no error is returned. ProofRefs are
// file:line derived from hunk line scan. The lens never imports plugin/ or planner/graph.
package resilience

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
)

// Lens is the R4 resilience heuristic. It is stateless and pure against the
// frozen LensInput (RiskInput + hunks + Truncated). Only hunks are inspected.
type Lens struct{}

// ID returns the stable lens identifier.
func (l *Lens) ID() string { return "resilience" }

// capBytes is the maximum total hunk bytes inspected (8MiB).
const capBytes = 8 << 20 // 8 MiB

// Analyze runs the R4 heuristic against the frozen input.
//
// Hunk-bound timeout/context/concurrency/cleanup patterns are inferential
// only. Total hunk size exceeding 8MiB sets Truncated without error.
// No full-file fallback is performed.
func (l *Lens) Analyze(_ context.Context, input lens.LensInput) (lens.LensResult, error) {
	result := lens.LensResult{
		LensID:    l.ID(),
		Findings:  nil,
		Evidence:  nil,
		Truncated: input.Truncated,
	}

	if input.Hunks == nil {
		input.Hunks = map[string][]byte{}
	}

	// Compute total hunk size and enforce 8MiB cap.
	total := 0
	for _, b := range input.Hunks {
		total += len(b)
	}
	if total > capBytes {
		result.Truncated = true
	}
	// Build capped view: sorted keys, accumulate until 8MiB.
	capped := make(map[string][]byte, len(input.Hunks))
	if total > capBytes {
		keys := make([]string, 0, len(input.Hunks))
		for k := range input.Hunks {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		remaining := capBytes
		for _, k := range keys {
			b := input.Hunks[k]
			if len(b) <= remaining {
				capped[k] = b
				remaining -= len(b)
			} else {
				// Truncate this hunk to remaining bytes.
				if remaining > 0 {
					capped[k] = b[:remaining]
				}
				remaining = 0
				break
			}
		}
	} else {
		for k, v := range input.Hunks {
			capped[k] = v
		}
	}

	// Stable order for deterministic findings.
	keys := make([]string, 0, len(capped))
	for k := range capped {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, p := range keys {
		if !strings.HasSuffix(strings.ToLower(p), ".go") {
			// Only Go hunks are inspected for resilience patterns; other files
			// are ignored to keep findings candidate-causal and precise.
			continue
		}
		content := string(capped[p])
		if strings.TrimSpace(content) == "" {
			continue
		}
		lines := strings.Split(content, "\n")
		for lineIdx, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// Strip diff prefix for scan but keep raw line for token search.
			scanLine := trimmed
			if len(scanLine) > 0 && (scanLine[0] == '+' || scanLine[0] == '-' || scanLine[0] == ' ') {
				scanLine = strings.TrimSpace(scanLine[1:])
			}
			combined := scanLine + " " + line

			// Timeout: http.Client without Timeout, or http.Get/Post without context.
			if isTimeoutHit(scanLine, combined) {
				f := makeFinding(l.ID(), p, lineIdx+1, "timeout", fmt.Sprintf("resilience: %s may lack timeout configuration — verify http.Client Timeout or context timeout", p))
				result.Findings = append(result.Findings, f)
				result.Evidence = append(result.Evidence, fmt.Sprintf("timeout pattern at %s:%d", p, lineIdx+1))
				continue
			}
			// Context: missing context propagation (context.Background/TODO without cancel, or missing context param)
			if isContextHit(scanLine, combined) {
				f := makeFinding(l.ID(), p, lineIdx+1, "context", fmt.Sprintf("resilience: %s may miss context cancellation propagation — verify context.Context usage", p))
				result.Findings = append(result.Findings, f)
				result.Evidence = append(result.Evidence, fmt.Sprintf("context pattern at %s:%d", p, lineIdx+1))
				continue
			}
			// Concurrency: goroutine without synchronization.
			if isConcurrencyHit(scanLine, combined) {
				f := makeFinding(l.ID(), p, lineIdx+1, "concurrency", fmt.Sprintf("resilience: %s uses concurrency without clear wait/cleanup — verify sync.WaitGroup or errgroup", p))
				result.Findings = append(result.Findings, f)
				result.Evidence = append(result.Evidence, fmt.Sprintf("concurrency pattern at %s:%d", p, lineIdx+1))
				continue
			}
			// Cleanup: resource acquisition without defer Close.
			if isCleanupHit(scanLine, combined) {
				f := makeFinding(l.ID(), p, lineIdx+1, "cleanup", fmt.Sprintf("resilience: %s acquires resource without visible defer cleanup — verify defer Close", p))
				result.Findings = append(result.Findings, f)
				result.Evidence = append(result.Evidence, fmt.Sprintf("cleanup pattern at %s:%d", p, lineIdx+1))
				continue
			}
		}
	}

	if len(result.Findings) == 0 && len(result.Evidence) == 0 {
		result.Evidence = []string{"resilience: no issues detected"}
	}

	return result, nil
}

func makeFinding(lensID, file string, line int, kind, msg string) lens.LensFinding {
	// ID uses R4 prefix with kind for traceability; unique per finding order
	// is handled by caller via sequential invocation, but we embed kind.
	// Generate deterministic ID via count placeholder; caller will uniquify.
	// For simplicity, use file+kind+line as suffix.
	// We keep a global counter via line to ensure uniqueness within file.
	id := fmt.Sprintf("R4-%s-%s-%03d", kind, sanitizeFileForID(file), line)
	proof := fmt.Sprintf("%s:%d", file, line)
	return lens.LensFinding{
		ID:        id,
		LensID:    lensID,
		Message:   msg,
		File:      file,
		Line:      line,
		ProofRefs: []string{proof},
		Class:     review.EvidenceInferential,
		Severity:  "warning",
	}
}

func sanitizeFileForID(p string) string {
	s := strings.ReplaceAll(p, "/", "-")
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, "_", "-")
	if len(s) > 20 {
		s = s[len(s)-20:]
	}
	return s
}

func isTimeoutHit(scanLine, combined string) bool {
	lower := strings.ToLower(scanLine)
	clower := strings.ToLower(combined)
	// http.Client literal without Timeout field in same line.
	if strings.Contains(lower, "http.client{") || strings.Contains(lower, "&http.client{") {
		if !strings.Contains(lower, "timeout") {
			return true
		}
	}
	// Direct http.Get/Post without context — suggest missing timeout.
	if strings.Contains(lower, "http.get(") || strings.Contains(lower, "http.post(") {
		return true
	}
	// net/http client without timeout elsewhere but this is hunk-bound heuristic:
	// any http.Client construction is flagged if not accompanied by Timeout.
	_ = clower
	return false
}

func isContextHit(scanLine, combined string) bool {
	lower := strings.ToLower(scanLine)
	// context.Background or context.TODO without WithCancel/WithTimeout nearby
	if strings.Contains(lower, "context.background(") || strings.Contains(lower, "context.todo(") {
		if !strings.Contains(lower, "withcancel") && !strings.Contains(lower, "withtimeout") && !strings.Contains(lower, "withdeadline") {
			return true
		}
	}
	_ = combined
	return false
}

func isConcurrencyHit(scanLine, combined string) bool {
	lower := strings.ToLower(scanLine)
	// go keyword launching goroutine
	trimmed := strings.TrimSpace(scanLine)
	if strings.HasPrefix(trimmed, "go ") || strings.Contains(lower, "\tgo ") || strings.Contains(lower, " go func") {
		// If line does not mention WaitGroup/errgroup/context, flag it.
		if !strings.Contains(lower, "waitgroup") && !strings.Contains(lower, "errgroup") && !strings.Contains(lower, "context") {
			return true
		}
	}
	_ = combined
	return false
}

func isCleanupHit(scanLine, combined string) bool {
	lower := strings.ToLower(scanLine)
	// Resource acquisition patterns without defer in same line
	if strings.Contains(lower, "os.open(") || strings.Contains(lower, "os.create(") || strings.Contains(lower, "os.openfile(") {
		if !strings.Contains(lower, "defer") && !strings.Contains(lower, "close(") {
			return true
		}
	}
	if strings.Contains(lower, "net.dial(") || strings.Contains(lower, "sql.open(") {
		if !strings.Contains(lower, "defer") {
			return true
		}
	}
	// Generic Closeable without defer
	if strings.Contains(lower, ".open(") && strings.Contains(lower, "file") {
		if !strings.Contains(lower, "defer") {
			return true
		}
	}
	_ = combined
	return false
}
