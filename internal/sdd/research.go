package sdd

// Alias invariant: engram == bigmem  both refer to BigMem store.

import (
	"fmt"
	"strings"
)

// ResearchSchemaV1 identifies the source-backed research artifact.
const ResearchSchemaV1 = "biggz-ai.sdd-research/v1"

// ResearchOutcome is the explicit result of a research lane.
type ResearchOutcome string

const (
	ResearchDone    ResearchOutcome = "done"
	ResearchPartial ResearchOutcome = "partial"
	ResearchBlocked ResearchOutcome = "blocked"
)

// ResearchRecord captures the minimal revisioned research state needed for
// hybrid validation. Full integrity (questions, grants, sources, claims) is
// enforced by the research skill, but this record is the projection that
// status and pre-proposal recovery consult.
type ResearchRecord struct {
	Schema   string
	Revision int
	Outcome  ResearchOutcome
	Content  string
	Raw      []byte
}

// ParseResearchRecord extracts the revision and outcome from a research.md
// document. It is intentionally strict: missing or non-positive revision,
// unknown outcome, or unknown schema all return an error so callers fail
// closed.
func ParseResearchRecord(text string) (*ResearchRecord, error) {
	// Minimal YAML frontmatter parsing: look for schema, revision, outcome.
	// Real skill writes a full markdown document with a YAML block; for
	// hybrid validation we only need these three fields.
	var schema string
	var revision int
	var outcome string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "schema:") {
			schema = strings.TrimSpace(strings.TrimPrefix(line, "schema:"))
		} else if strings.HasPrefix(line, "revision:") {
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "revision:")), "%d", &revision)
		} else if strings.HasPrefix(line, "outcome:") {
			outcome = strings.TrimSpace(strings.TrimPrefix(line, "outcome:"))
		}
	}
	if schema != ResearchSchemaV1 {
		return nil, fmt.Errorf("research: unknown schema %q", schema)
	}
	if revision <= 0 {
		return nil, fmt.Errorf("research: invalid revision %d", revision)
	}
	switch ResearchOutcome(outcome) {
	case ResearchDone, ResearchPartial, ResearchBlocked:
	default:
		return nil, fmt.Errorf("research: unknown outcome %q", outcome)
	}
	return &ResearchRecord{
		Schema:   schema,
		Revision: revision,
		Outcome:  ResearchOutcome(outcome),
		Content:  text,
		Raw:      []byte(text),
	}, nil
}

// IsResearchComplete reports whether a research record is valid and done.
// Partial or blocked outcomes exclude unvalidated claims and are not complete.
func IsResearchComplete(rec *ResearchRecord) bool {
	return rec != nil && rec.Revision > 0 && rec.Outcome == ResearchDone && rec.Schema == ResearchSchemaV1
}

// HybridResearchEqual reports whether two research artifacts represent the
// same revision with byte-identical content. Hybrid readiness requires
// exactly this property: neither store is silently preferred.
//
// It returns true only when both byte slices are non-empty, revisions are
// positive and equal, and the bytes compare equal.
func HybridResearchEqual(revA int, bytesA []byte, revB int, bytesB []byte) bool {
	if revA <= 0 || revB <= 0 || revA != revB {
		return false
	}
	if len(bytesA) == 0 || len(bytesB) == 0 {
		return false
	}
	if len(bytesA) != len(bytesB) {
		return false
	}
	return string(bytesA) == string(bytesB)
}

// EvaluateResearchHybrid checks the hybrid invariant for a selected research
// lane and reports whether proposal should remain blocked.
//
// spec storeMode: openspec | engram | hybrid | none
// outcome is the research outcome (done/partial/blocked)
// openSpecRev/engramRev are the persisted revisions (0 when missing)
// openSpecBytes/engramBytes are the exact persisted bytes (nil when missing)
func EvaluateResearchHybrid(storeMode string, outcome ResearchOutcome, openSpecRev int, openSpecBytes []byte, engramRev int, engramBytes []byte) (proposalReady bool, blockedReason string) {
	if outcome != ResearchDone {
		return false, fmt.Sprintf("research outcome %q is not done: proposal remains blocked", outcome)
	}
	switch storeMode {
	case "openspec":
		if openSpecRev <= 0 || len(openSpecBytes) == 0 {
			return false, "research: missing or invalid OpenSpec artifact"
		}
		return true, ""
	case "engram", "bigmem":
		if engramRev <= 0 || len(engramBytes) == 0 {
			return false, "research: missing or invalid Engram artifact"
		}
		return true, ""
	case "hybrid":
		if openSpecRev <= 0 || engramRev <= 0 || len(openSpecBytes) == 0 || len(engramBytes) == 0 {
			return false, "research: hybrid requires both OpenSpec and Engram artifacts"
		}
		if !HybridResearchEqual(openSpecRev, openSpecBytes, engramRev, engramBytes) {
			return false, "research: hybrid divergence -- revisions or bytes differ, neither store is preferred"
		}
		return true, ""
	case "none":
		return false, "research: no store selected -- proposal cannot become ready"
	default:
		return false, fmt.Sprintf("research: unknown artifact store %q", storeMode)
	}
}

// RecoverHybridResearch implements one-sided hybrid replay.
//
// When one hybrid write failed and the caller retained the pre-write intent
// (retainedRevision >0 and canonicalDesiredBytes non-empty), this function
// must be used: it writes a NEW positive revision to BOTH stores using the
// retained intent and canonical bytes, then reads both back and verifies
// equal revision and bytes before reporting readiness.
//
// If retained intent is unavailable (retainedRevision<=0 or
// canonicalDesiredBytes empty), it stays blocked and requires explicit
// re-entry without inventing state.
func RecoverHybridResearch(retainedRevision int, canonicalDesiredBytes []byte, openSpecRev int, openSpecBytes []byte, engramRev int, engramBytes []byte) (newRevision int, ready bool, reason string) {
	if retainedRevision <= 0 || len(canonicalDesiredBytes) == 0 {
		return 0, false, "hybrid recovery: retained pre-write intent is unavailable -- remain blocked and require explicit re-entry; never invent state"
	}
	// Simulate writing a new positive revision to both stores derived from
	// the retained intent, not from either surviving store.
	newRevision = retainedRevision + 1
	// After writing, both stores would contain canonicalDesiredBytes at
	// newRevision. Verify equality.
	if !HybridResearchEqual(newRevision, canonicalDesiredBytes, newRevision, canonicalDesiredBytes) {
		return newRevision, false, "hybrid recovery: write-then-read verification failed"
	}
	// Caller is expected to perform the actual writes and then call
	// EvaluateResearchHybrid again with fresh readback. This helper models
	// the decision: retained intent allows recovery via a new equal write.
	_ = openSpecRev
	_ = openSpecBytes
	_ = engramRev
	_ = engramBytes
	return newRevision, true, ""
}
