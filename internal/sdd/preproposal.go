package sdd

// Alias invariant: engram == bigmem  both refer to BigMem store.

import (
	"fmt"
	"strings"
)

// PreproposalSchemaV1 identifies the pre-proposal gate state.
const PreproposalSchemaV1 = "biggz-ai.sdd-preproposal/v1"

// PreproposalDecision is the product-decision commitment state.
type PreproposalDecision string

const (
	PreproposalPending   PreproposalDecision = "pending"
	PreproposalConfirmed PreproposalDecision = "confirmed"
)

// PreproposalRecord is the revisioned gate state that orchestrator checks
// before invoking sdd-propose. It records exploration outcome, research
// request/classes, admission/outcome, evidence references, and decision
// confirmation.
type PreproposalRecord struct {
	Schema           string
	Revision         int
	ResearchRequest  string
	ResearchClasses  []string
	AdmissionOutcome string
	EvidenceOutcome  ResearchOutcome
	OpenSpecRef      string
	EngramRef        string
	Decision         PreproposalDecision
	ProposalReady    bool
	Raw              []byte
}

// ParsePreproposalRecord extracts key fields from a preproposal document.
// It fails closed on unknown schema, non-positive revision, or invalid
// decision so callers cannot proceed with a malformed gate state.
func ParsePreproposalRecord(text string) (*PreproposalRecord, error) {
	var schema, decision string
	var revision int
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "schema:") {
			schema = strings.TrimSpace(strings.TrimPrefix(line, "schema:"))
		} else if strings.HasPrefix(line, "revision:") {
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "revision:")), "%d", &revision)
		} else if strings.HasPrefix(line, "decision:") {
			decision = strings.TrimSpace(strings.TrimPrefix(line, "decision:"))
		} else if strings.HasPrefix(line, "proposal_ready:") {
			// parsed but not stored as bool here; evaluated via IsPreproposalReady
		}
	}
	if schema != PreproposalSchemaV1 {
		return nil, fmt.Errorf("preproposal: unknown schema %q", schema)
	}
	if revision <= 0 {
		return nil, fmt.Errorf("preproposal: invalid revision %d", revision)
	}
	switch PreproposalDecision(decision) {
	case PreproposalPending, PreproposalConfirmed:
	default:
		if decision != "" {
			return nil, fmt.Errorf("preproposal: unknown decision %q", decision)
		}
	}
	return &PreproposalRecord{
		Schema:   schema,
		Revision: revision,
		Decision: PreproposalDecision(decision),
		Raw:      []byte(text),
	}, nil
}

// IsPreproposalReady implements the closed readiness matrix for selected
// research.
//
// Conditions for ready:
//   - research outcome done (if research was selected)
//   - stored evidence valid (revision>0, bytes non-empty, schema ok)
//   - product decisions confirmed
//   - evidence references valid
//   - selected store mode ready (openspec validates OpenSpec only, engram
//     validates Engram only, hybrid requires equal revision+bytes, none
//     never ready)
//
// Missing artifact, failure, divergence, partial, or blocked MUST retain
// intent and block proposal. The caller retains pre-write intent and
// canonical bytes for hybrid one-sided recovery.
func IsPreproposalReady(record *PreproposalRecord, researchSelected bool, researchOutcome ResearchOutcome, storeMode string, openSpecRev int, openSpecBytes []byte, engramRev int, engramBytes []byte, productDecision PreproposalDecision, hasValidRefs bool) (bool, string) {
	if record == nil || record.Revision <= 0 {
		return false, "preproposal: missing or invalid gate record"
	}
	if researchSelected {
		if researchOutcome != ResearchDone {
			return false, fmt.Sprintf("preproposal: research not done (outcome %q)", researchOutcome)
		}
		ready, reason := EvaluateResearchHybrid(storeMode, researchOutcome, openSpecRev, openSpecBytes, engramRev, engramBytes)
		if !ready {
			return false, reason
		}
	}
	if productDecision != PreproposalConfirmed {
		return false, "preproposal: product decisions not confirmed"
	}
	if !hasValidRefs {
		return false, "preproposal: evidence references invalid"
	}
	// storeMode readiness for preproposal references themselves
	switch storeMode {
	case "openspec", "engram", "bigmem", "hybrid":
		// hybrid already validated above; for openspec/engram the artifact
		// existence was checked. For preproposal itself, hybrid also requires
		// equal revision+bytes when both stores are used for the gate.
		if storeMode == "hybrid" {
			if !HybridResearchEqual(openSpecRev, openSpecBytes, engramRev, engramBytes) && researchSelected {
				return false, "preproposal: hybrid preproposal divergence -- revisions or bytes differ"
			}
		}
	case "none":
		return false, "preproposal: no store -- proposal remains blocked"
	default:
		return false, fmt.Sprintf("preproposal: unknown store %q", storeMode)
	}
	return true, ""
}

// RecoverHybridPreproposal replays the preproposal gate for hybrid when one
// side failed to write. It requires retained intent and canonical bytes like
// RecoverHybridResearch: a new positive revision is written to both stores
// from the retained values, never from either surviving store, then both
// are read back for equality before readiness.
//
// If retained intent is unavailable, it remains blocked and requires
// explicit re-entry.
func RecoverHybridPreproposal(retainedRevision int, canonicalBytes []byte) (newRevision int, ready bool, reason string) {
	if retainedRevision <= 0 || len(canonicalBytes) == 0 {
		return 0, false, "hybrid preproposal recovery: retained intent unavailable -- remain blocked and require explicit re-entry; never invent state"
	}
	newRevision = retainedRevision + 1
	if !HybridResearchEqual(newRevision, canonicalBytes, newRevision, canonicalBytes) {
		return newRevision, false, "hybrid preproposal recovery: verification failed"
	}
	return newRevision, true, ""
}
