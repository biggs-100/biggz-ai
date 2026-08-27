package review

import (
	"fmt"
	"time"
)

// ─── Result Dispositions ─────────────────────────────────────────────────────
//
// ResultDisposition records an audited reviewer-result refusal or reopening.
// This provides an audit trail for result lifecycle management.

// DispositionAction describes the action taken on a result.
type DispositionAction string

const (
	DispositionRefused  DispositionAction = "refused"
	DispositionReopened DispositionAction = "reopened"
	DispositionAccepted DispositionAction = "accepted"
)

// ResultDisposition records a single disposition event for a review result.
type ResultDisposition struct {
	Schema    string            `json:"schema"` // "biggz.review-disposition/v1"
	LineageID string            `json:"lineage_id"`
	Action    DispositionAction `json:"action"`
	Reason    string            `json:"reason"`
	Actor     string            `json:"actor"`
	ResultID  string            `json:"result_id,omitempty"` // the result being acted on
	Timestamp string            `json:"timestamp"`
}

// DispositionJournal is an append-only journal of result dispositions.
type DispositionJournal struct {
	Schema       string              `json:"schema"`
	LineageID    string              `json:"lineage_id"`
	Dispositions []ResultDisposition `json:"dispositions"`
}

// NewDispositionJournal creates a new journal for a lineage.
func NewDispositionJournal(lineageID string) *DispositionJournal {
	return &DispositionJournal{
		Schema:       "biggz.review-disposition-journal/v1",
		LineageID:    lineageID,
		Dispositions: make([]ResultDisposition, 0),
	}
}

// Record appends a new disposition entry.
func (j *DispositionJournal) Record(action DispositionAction, reason, actor, resultID string) error {
	if reason == "" {
		return fmt.Errorf("disposition reason is required")
	}
	if actor == "" {
		return fmt.Errorf("disposition actor is required")
	}
	j.Dispositions = append(j.Dispositions, ResultDisposition{
		Schema:    "biggz.review-disposition/v1",
		LineageID: j.LineageID,
		Action:    action,
		Reason:    reason,
		Actor:     actor,
		ResultID:  resultID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

// Summary returns a summary of dispositions by action.
func (j *DispositionJournal) Summary() map[DispositionAction]int {
	summary := make(map[DispositionAction]int)
	for _, d := range j.Dispositions {
		summary[d.Action]++
	}
	return summary
}
