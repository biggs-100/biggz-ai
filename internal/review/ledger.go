package review

import (
	"fmt"
	"time"
)

// LedgerEntry records a single operation in the review audit trail.
type LedgerEntry struct {
	ID        string    `json:"id"`
	ReviewID  string    `json:"review_id"`
	Operation string    `json:"operation"` // started, lens_completed, policy_evaluated, correction_applied, completed, failed, archived
	Detail    string    `json:"detail,omitempty"`
	Status    string    `json:"status,omitempty"` // state after this operation
	Actor     string    `json:"actor,omitempty"`  // system, user, lens:risk, etc.
	Timestamp time.Time `json:"timestamp"`
}

// Ledger is an ordered, append-only audit trail for a review.
type Ledger struct {
	entries []LedgerEntry
}

// NewLedger creates an empty ledger.
func NewLedger() *Ledger {
	return &Ledger{entries: make([]LedgerEntry, 0)}
}

// Append adds an entry to the ledger.
func (l *Ledger) Append(entry LedgerEntry) {
	entry.ID = fmt.Sprintf("ledger-%d", len(l.entries)+1)
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	l.entries = append(l.entries, entry)
}

// AppendOp is a convenience method to add an operation entry.
func (l *Ledger) AppendOp(reviewID, operation, detail, status, actor string) {
	l.Append(LedgerEntry{
		ReviewID:  reviewID,
		Operation: operation,
		Detail:    detail,
		Status:    status,
		Actor:     actor,
	})
}

// Entries returns all ledger entries in order.
func (l *Ledger) Entries() []LedgerEntry {
	result := make([]LedgerEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

// FilterByOperation returns entries matching the given operation.
func (l *Ledger) FilterByOperation(op string) []LedgerEntry {
	var result []LedgerEntry
	for _, e := range l.entries {
		if e.Operation == op {
			result = append(result, e)
		}
	}
	return result
}

// Summary returns a human-readable summary of the ledger.
func (l *Ledger) Summary() string {
	if len(l.entries) == 0 {
		return "No entries."
	}
	ops := make(map[string]int)
	for _, e := range l.entries {
		ops[e.Operation]++
	}
	summary := ""
	for op, count := range ops {
		summary += fmt.Sprintf("  %s: %d\n", op, count)
	}
	return summary
}
