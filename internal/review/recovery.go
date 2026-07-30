package review

import (
	"fmt"
	"time"
)

// ─── Recovery Provenance ─────────────────────────────────────────────────────

// RecoveryProvenance tracks how a review lineage was recovered or inherited
// from a previous lineage. This ensures auditability when lineages are
// replaced or superseded.
type RecoveryProvenance struct {
	Schema          string `json:"schema"`           // "biggz.review-recovery/v1"
	PredecessorLineage string `json:"predecessor_lineage"`
	PredecessorRevision string `json:"predecessor_revision"`
	Disposition     string `json:"disposition"`      // "escalated", "invalidated", "superseded"
	Reason          string `json:"reason"`
	RecoveredBy     string `json:"recovered_by"`
	RecoveredAt     string `json:"recovered_at"`
	Authorization   string `json:"authorization,omitempty"` // maintainer authorization ref
}

// NewRecoveryProvenance creates a recovery record for a lineage.
func NewRecoveryProvenance(predecessorLineage, predecessorRevision, disposition, reason, recoveredBy string) *RecoveryProvenance {
	return &RecoveryProvenance{
		Schema:             "biggz.review-recovery/v1",
		PredecessorLineage: predecessorLineage,
		PredecessorRevision: predecessorRevision,
		Disposition:        disposition,
		Reason:             reason,
		RecoveredBy:        recoveredBy,
		RecoveredAt:        time.Now().UTC().Format(time.RFC3339),
	}
}

// Validate checks that the recovery record is self-consistent.
func (rp *RecoveryProvenance) Validate() error {
	if rp.Schema != "biggz.review-recovery/v1" {
		return fmt.Errorf("invalid recovery schema: %s", rp.Schema)
	}
	if rp.PredecessorLineage == "" {
		return fmt.Errorf("predecessor lineage is required")
	}
	if rp.Reason == "" {
		return fmt.Errorf("recovery reason is required")
	}
	if rp.RecoveredBy == "" {
		return fmt.Errorf("recovered by is required")
	}
	validDispositions := map[string]bool{
		"escalated": true, "invalidated": true, "superseded": true,
	}
	if !validDispositions[rp.Disposition] {
		return fmt.Errorf("invalid recovery disposition: %s", rp.Disposition)
	}
	return nil
}
