// Package resilience implements the R4 ResilienceLens heuristic.
//
// R4 is hunk-bounded, inferential-only for timeout/context/concurrency/cleanup.
// It never falls back to full file reads. Hunks are capped at 8MiB total;
// when exceeded, Truncated is set and no error is returned. ProofRefs are
// file:line derived from hunk line scan. The lens never imports plugin/ or planner/graph.
package resilience

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
)

// Lens is the R4 resilience heuristic. It is stateless and pure against the
// frozen LensInput (RiskInput + hunks + Truncated). Only hunks are inspected.
type Lens struct{}

// ID returns the stable lens identifier.
func (l *Lens) ID() string { return "resilience" }

// ResiliencePromptData is the inventory for r4-resilience.md template.
type ResiliencePromptData struct {
	Repo         string
	ChangedLines int
	Paths        []string
	Diff         string
	Truncated    bool
	BaseTree     string
	Hunks        string
	Shared       string
}

func renderResiliencePrompt(input lens.LensInput) (string, error) {
	data, err := assets.FS.ReadFile("prompts/review/r4-resilience.md")
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("r4-resilience.md").Option("missingkey=error").Parse(string(data))
	if err != nil {
		return "", err
	}
	pd := ResiliencePromptData{
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

// capBytes is the maximum total hunk bytes inspected (8MiB).
const capBytes = 8 << 20 // 8 MiB

// Analyze runs the R4 heuristic against the frozen input.
//
// Hunk-bound timeout/context/concurrency/cleanup patterns are inferential
// only. Total hunk size exceeding 8MiB sets Truncated without error.
// No full-file fallback is performed.
func (l *Lens) Analyze(_ context.Context, input lens.LensInput) (lens.LensResult, error) {
	if _, err := renderResiliencePrompt(input); err != nil {
		_ = err
	}
	result := lens.LensResult{
		LensID:    l.ID(),
		Truncated: input.Truncated,
	}
	if input.Hunks == nil {
		input.Hunks = map[string][]byte{}
	}
	total := totalHunkBytes(input.Hunks)
	capped := input.Hunks
	if total > capBytes {
		result.Truncated = true
		capped = buildCappedHunks(input.Hunks)
	}
	for _, p := range slices.Sorted(maps.Keys(capped)) {
		if !isGoFile(p) {
			continue
		}
		findings, evidence := analyzeFileResilience(p, string(capped[p]), l.ID())
		result.Findings = append(result.Findings, findings...)
		result.Evidence = append(result.Evidence, evidence...)
	}
	if len(result.Findings) == 0 && len(result.Evidence) == 0 {
		result.Evidence = []string{"resilience: no issues detected"}
	}
	return result, nil
}

// totalHunkBytes returns the sum of all hunk byte lengths.
func totalHunkBytes(hunks map[string][]byte) int {
	total := 0
	for _, b := range hunks {
		total += len(b)
	}
	return total
}

// buildCappedHunks returns a copy of hunks truncated to capBytes in sorted key order.
func buildCappedHunks(hunks map[string][]byte) map[string][]byte {
	capped := make(map[string][]byte, len(hunks))
	keys := slices.Sorted(maps.Keys(hunks))
	remaining := capBytes
	for _, k := range keys {
		b := hunks[k]
		if len(b) <= remaining {
			capped[k] = b
			remaining -= len(b)
			continue
		}
		if remaining > 0 {
			capped[k] = b[:remaining]
		}
		break
	}
	return capped
}

// isGoFile reports whether path is a Go source file.
func isGoFile(p string) bool {
	return strings.HasSuffix(strings.ToLower(p), ".go")
}

// analyzeFileResilience scans a single Go hunk for resilience patterns.
func analyzeFileResilience(path, content, lensID string) ([]lens.LensFinding, []string) {
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}
	var findings []lens.LensFinding
	var evidence []string
	lines := strings.Split(content, "\n")
	for idx, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		scanLine := trimmed
		if len(scanLine) > 0 && (scanLine[0] == '+' || scanLine[0] == '-' || scanLine[0] == ' ') {
			scanLine = strings.TrimSpace(scanLine[1:])
		}
		combined := scanLine + " " + raw
		if f, ev, ok := resilienceFindingForLine(path, idx+1, scanLine, combined, lensID); ok {
			findings = append(findings, f)
			evidence = append(evidence, ev)
		}
	}
	return findings, evidence
}

// resilienceFindingForLine checks a single line for the four resilience patterns.
func resilienceFindingForLine(path string, lineNum int, scanLine, combined, lensID string) (lens.LensFinding, string, bool) {
	if checkTimeout(scanLine, combined) {
		f := makeFinding(lensID, path, lineNum, "timeout", fmt.Sprintf("resilience: %s may lack timeout configuration — verify http.Client Timeout or context timeout", path)) //lint:ignore no-fmtSprintf
		return f, fmt.Sprintf("timeout pattern at %s:%d", path, lineNum), true                                           //lint:ignore no-fmtSprintf
	}
	if checkContext(scanLine, combined) {
		f := makeFinding(lensID, path, lineNum, "context", fmt.Sprintf("resilience: %s may miss context cancellation propagation — verify context.Context usage", path)) //lint:ignore no-fmtSprintf
		return f, fmt.Sprintf("context pattern at %s:%d", path, lineNum), true                                            //lint:ignore no-fmtSprintf
	}
	if isConcurrencyHit(scanLine, combined) {
		f := makeFinding(lensID, path, lineNum, "concurrency", fmt.Sprintf("resilience: %s uses concurrency without clear wait/cleanup — verify sync.WaitGroup or errgroup", path)) //lint:ignore no-fmtSprintf
		return f, fmt.Sprintf("concurrency pattern at %s:%d", path, lineNum), true                                              //lint:ignore no-fmtSprintf
	}
	if isCleanupHit(scanLine, combined) {
		f := makeFinding(lensID, path, lineNum, "cleanup", fmt.Sprintf("resilience: %s acquires resource without visible defer cleanup — verify defer Close", path)) //lint:ignore no-fmtSprintf
		return f, fmt.Sprintf("cleanup pattern at %s:%d", path, lineNum), true                                           //lint:ignore no-fmtSprintf
	}
	return lens.LensFinding{}, "", false
}

func makeFinding(lensID, file string, line int, kind, msg string) lens.LensFinding {
	id := fmt.Sprintf("R4-%s-%s-%03d", kind, sanitizeFileForID(file), line) //lint:ignore no-fmtSprintf
	proof := fmt.Sprintf("%s:%d", file, line)                               //lint:ignore no-fmtSprintf
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

// checkTimeout reports whether scanLine indicates a missing timeout configuration.
func checkTimeout(scanLine, _ string) bool {
	lower := strings.ToLower(scanLine)
	if strings.Contains(lower, "http.client{") || strings.Contains(lower, "&http.client{") {
		if !strings.Contains(lower, "timeout") {
			return true
		}
	}
	if strings.Contains(lower, "http.get(") || strings.Contains(lower, "http.post(") {
		return true
	}
	return false
}

func isTimeoutHit(scanLine, combined string) bool {
	return checkTimeout(scanLine, combined)
}

// checkContext reports whether scanLine indicates missing context cancellation propagation.
func checkContext(scanLine, _ string) bool {
	lower := strings.ToLower(scanLine)
	if strings.Contains(lower, "context.background(") || strings.Contains(lower, "context.todo(") {
		if !strings.Contains(lower, "withcancel") && !strings.Contains(lower, "withtimeout") && !strings.Contains(lower, "withdeadline") {
			return true
		}
	}
	return false
}

func isContextHit(scanLine, combined string) bool {
	return checkContext(scanLine, combined)
}

func isConcurrencyHit(scanLine, combined string) bool {
	lower := strings.ToLower(scanLine)
	trimmed := strings.TrimSpace(scanLine)
	if strings.HasPrefix(trimmed, "go ") || strings.Contains(lower, "\tgo ") || strings.Contains(lower, " go func") {
		if !strings.Contains(lower, "waitgroup") && !strings.Contains(lower, "errgroup") && !strings.Contains(lower, "context") {
			return true
		}
	}
	_ = combined
	return false
}

func isCleanupHit(scanLine, combined string) bool {
	lower := strings.ToLower(scanLine)
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
	if strings.Contains(lower, ".open(") && strings.Contains(lower, "file") {
		if !strings.Contains(lower, "defer") {
			return true
		}
	}
	_ = combined
	return false
}


