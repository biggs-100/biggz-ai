// Package lens defines the hybrid facade lens contracts for biggz-ai.
//
// Lenses are sequential pipeline stages (no DAG) living in
// internal/review/lens — never in plugin/. Each lens is a durable reviewer
// slot ordered by PlanLenses (risk→resilience→readability→reliability).
// Input reuses the single DeriveRiskInput derivation (paths, changed lines,
// diff summary, hunks) and is capped at 8MiB (R4) with Truncated flag.
//
// Build-time Registry (registry.go) holds native + ExternalLensAdapter
// entries; duplicates last-win, unknowns skipped at cmd/biggz init.
package lens

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/biggs-100/biggz-ai/internal/review"
)

// LensResultDomain is the content-addressed hash domain for lens results.
// Mirrors review.LensResultDomain ("biggz-ai.lens-result/v1") for native
// heuristics; ExternalLensAdapter preserves the capture-result payload hash
// with the same prefix without changing capture.go/ledger.go schema.
const LensResultDomain = "biggz-ai.lens-result/v1"

// LensInput is the single derivation input for every lens.
// It embeds the frozen RiskInput from DeriveRiskInput (Paths, ChangedLines,
// DiffSummary, BaseTree) plus hunk-bounded diff content. Hunks are
// repository-relative path → raw hunk bytes, capped at 8MiB total for R4
// with Truncated=true when truncated. Repo is the repository root or origin.
type LensInput struct {
	review.RiskInput
	Hunks     map[string][]byte `json:"hunks,omitempty"`
	Truncated bool              `json:"truncated"`
	Repo      string            `json:"repo,omitempty"`
}

// LensFinding is one structured observation emitted by a lens.
// Findings default to inferential; only concrete ProofRefs may be
// deterministic. Class uses review.EvidenceClass values.
type LensFinding struct {
	ID        string               `json:"id"`
	LensID    string               `json:"lens_id"`
	Message   string               `json:"message"`
	File      string               `json:"file,omitempty"`
	Line      int                  `json:"line,omitempty"`
	ProofRefs []string             `json:"proof_refs,omitempty"`
	Class     review.EvidenceClass `json:"class"`
	Severity  string               `json:"severity,omitempty"`
}

// LensResult is the canonical result of one lens execution.
type LensResult struct {
	LensID     string        `json:"lens_id"`
	Findings   []LensFinding `json:"findings"`
	Evidence   []string      `json:"evidence"`
	ResultHash string        `json:"result_hash,omitempty"`
	Truncated  bool          `json:"truncated"`
}

// Lens is the minimal seam for native heuristics and ExternalLensAdapter.
// ID returns the stable lens identifier (risk, resilience, readability,
// reliability). Analyze is pure against the frozen LensInput.
type Lens interface {
	ID() string
	Analyze(ctx context.Context, input LensInput) (LensResult, error)
}

// HunkCapBytes is the maximum total hunk bytes inspected (8MiB).
const HunkCapBytes = 8 << 20

// NewLensInput builds a LensInput from the single DeriveRiskInput derivation
// plus hunk-bounded diff content. Hunks are capped at HunkCapBytes total;
// when exceeded, the hunks are truncated and Truncated is set. No per-lens
// diff is performed — the single RiskInput is reused.
func NewLensInput(riskInput review.RiskInput, hunks map[string][]byte, truncated bool, repo string) LensInput {
	total := 0
	for _, b := range hunks {
		total += len(b)
	}
	if total > HunkCapBytes {
		truncated = true
	}
	capped := hunks
	if total > HunkCapBytes {
		// Cap in stable sorted order to keep determinism.
		keys := make([]string, 0, len(hunks))
		for k := range hunks {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		capped = make(map[string][]byte, len(hunks))
		remaining := HunkCapBytes
		for _, k := range keys {
			b := hunks[k]
			if len(b) <= remaining {
				capped[k] = b
				remaining -= len(b)
			} else if remaining > 0 {
				capped[k] = b[:remaining]
				remaining = 0
				break
			} else {
				break
			}
		}
	}
	return LensInput{RiskInput: riskInput, Hunks: capped, Truncated: truncated, Repo: repo}
}

// LensResultHash derives the content-addressed hash for a lens result
// under LensResultDomain. Payload is {lens, findings, evidence} with
// stable JSON field order. Callers should set ResultHash to the returned
// value before persisting.
func LensResultHash(result LensResult) string {
	payload, _ := json.Marshal(struct {
		Lens     string        `json:"lens"`
		Findings []LensFinding `json:"findings"`
		Evidence []string      `json:"evidence"`
	}{
		Lens:     result.LensID,
		Findings: result.Findings,
		Evidence: result.Evidence,
	})
	sum := sha256.Sum256(append([]byte(LensResultDomain+"\x00"), payload...))
	return "sha256:" + hex.EncodeToString(sum[:])
}
